package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/zx06/xsql/internal/ai"
	"github.com/zx06/xsql/internal/app"
	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/export"
	"github.com/zx06/xsql/internal/js"
	"github.com/zx06/xsql/internal/session"
)

type State int

const (
	StateLoadingSchema State = iota
	StateIdle
	StateThinking
	StateSQLReady
	StateExecuting
	StateExportReady
)

// Msg types
type schemaLoadedMsg struct {
	schema *db.SchemaInfo
	err    *errors.XError
}

type aiResponseMsg struct {
	response *ai.AIResponse
	err      *errors.XError
}

type sqlGeneratedMsg = aiResponseMsg

type queryExecutedMsg struct {
	result   *db.QueryResult
	err      *errors.XError
	duration time.Duration
}

type TableState struct {
	Result       *db.QueryResult
	MsgIndex     int
	ColOffset    int
	RowOffset    int
	VerticalView bool
}

type ToolCallItem struct {
	ID              string
	Name            string
	Summary         string
	Detail          string
	Result          string
	RawOutput       string // Raw execution output/logs (never hidden!)
	TableStateIndex int    // -1 if no table attached
	MsgIndex        int
	IsExpanded      bool
}

type PendingExport struct {
	DatasetID string
	Format    string
	FilePath  string
	ToolIdx   int
}

type Model struct {
	opts             config.Options
	aiService        *ai.Service
	profile          config.Profile
	profileName      string
	allProfiles      map[string]config.Profile
	profileList      []string
	unsafeAllowWrite bool
	initialPrompt    string
	autoExecute      bool

	sessionStore  *session.SessionDataStore
	jsEngine      *js.JSEngine
	chatHistory   []ai.ChatMessage
	pendingExport *PendingExport
	jsRetryCount  int
	maxJSRetries  int
	lastCtrlCTime time.Time

	confirmOption int // 0: Confirm/Execute, 1: Adjust Prompt, 2: Cancel/Deny

	state         State
	schemaInfo    *db.SchemaInfo
	currentSQL    string
	explanation   string
	messages      []string
	tableStates   []TableState
	toolCalls     []ToolCallItem
	activeTable   int
	activeToolIdx int

	textarea textarea.Model
	viewport viewport.Model
	spinner  spinner.Model

	width  int
	height int
}

func NewModel(opts config.Options, resolved config.Resolved, aiService *ai.Service, initialPrompt string, unsafeAllowWrite bool) Model {
	ta := textarea.New()
	ta.Placeholder = "Ask AI to query database or perform data analysis..."
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.Focus()
	ta.CharLimit = 4000
	ta.SetWidth(80)
	ta.SetHeight(3)

	// Custom crisp styles for textarea
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()

	vp := viewport.New(80, 15)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(PrimaryColor)

	var pList []string
	if len(resolved.AllProfiles) > 0 {
		for name := range resolved.AllProfiles {
			pList = append(pList, name)
		}
		sort.Strings(pList)
	} else if resolved.ProfileName != "" {
		pList = []string{resolved.ProfileName}
	}

	return Model{
		opts:             opts,
		aiService:        aiService,
		profile:          resolved.Profile,
		profileName:      resolved.ProfileName,
		allProfiles:      resolved.AllProfiles,
		profileList:      pList,
		unsafeAllowWrite: unsafeAllowWrite || resolved.Profile.UnsafeAllowWrite,
		initialPrompt:    strings.TrimSpace(initialPrompt),
		autoExecute:      false,
		sessionStore:     session.NewSessionDataStore(),
		jsEngine:         js.NewJSEngine(1 * time.Minute),
		chatHistory:      []ai.ChatMessage{},
		jsRetryCount:     0,
		maxJSRetries:     3,
		confirmOption:    0,
		tableStates:      []TableState{},
		toolCalls:        []ToolCallItem{},
		activeTable:      -1,
		activeToolIdx:    -1,
		state:            StateLoadingSchema,
		textarea:         ta,
		viewport:         vp,
		spinner:          s,
		messages:         []string{},
		width:            80,
		height:           24,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadSchemaCmd(),
	)
}

func (m Model) loadSchemaCmd() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		info, xe := app.DumpSchema(ctx, app.SchemaDumpRequest{
			Profile:          m.profile,
			AllowPlaintext:   m.profile.AllowPlaintext,
			SkipHostKeyCheck: m.profile.SSHConfig != nil && m.profile.SSHConfig.SkipHostKey,
		})
		return schemaLoadedMsg{schema: info, err: xe}
	}
}

func (m Model) runAgentStepCmd() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		catalog := m.sessionStore.GetCatalog()
		sysPrompt := ai.BuildSystemPrompt(m.profile.DB, m.schemaInfo, catalog)

		msgs := make([]ai.ChatMessage, 0, len(m.chatHistory)+1)
		msgs = append(msgs, ai.ChatMessage{Role: "system", Content: sysPrompt})
		for _, item := range m.chatHistory {
			if item.Role != "system" {
				msgs = append(msgs, item)
			}
		}

		resp, xe := m.aiService.ChatCompletion(ctx, msgs)
		return aiResponseMsg{response: resp, err: xe}
	}
}

func (m Model) generateSQLCmd(prompt string) tea.Cmd {
	return m.runAgentStepCmd()
}

func (m Model) executeSQLCmd(sqlStr string) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		ctx := context.Background()
		res, xe := app.Query(ctx, app.QueryRequest{
			Profile:          m.profile,
			SQL:              sqlStr,
			AllowPlaintext:   m.profile.AllowPlaintext,
			SkipHostKeyCheck: m.profile.SSHConfig != nil && m.profile.SSHConfig.SkipHostKey,
			UnsafeAllowWrite: m.unsafeAllowWrite,
		})
		elapsed := time.Since(start)
		return queryExecutedMsg{result: res, err: xe, duration: elapsed}
	}
}

func (m *Model) focusToolCall(idx int) {
	if len(m.toolCalls) == 0 {
		return
	}
	if idx < 0 {
		idx = len(m.toolCalls) - 1
	} else if idx >= len(m.toolCalls) {
		idx = 0
	}

	oldIdx := m.activeToolIdx
	m.activeToolIdx = idx

	// Sync embedded table focus if tool call has attached table
	if tc := m.toolCalls[idx]; tc.TableStateIndex >= 0 {
		m.activeTable = tc.TableStateIndex
	}

	if oldIdx >= 0 && oldIdx < len(m.toolCalls) {
		m.renderToolCall(oldIdx)
	}
	m.renderToolCall(m.activeToolIdx)
}

func (m *Model) renderTableState(idx int, isActive bool) {
	if idx < 0 || idx >= len(m.tableStates) {
		return
	}
	ts := &m.tableStates[idx]

	// Find associated ToolCallItem if embedded
	for i := range m.toolCalls {
		if m.toolCalls[i].TableStateIndex == idx {
			m.renderToolCall(i)
			return
		}
	}

	if ts.MsgIndex < 0 || ts.MsgIndex >= len(m.messages) {
		return
	}
	formatted := FormatTableResult(ts.Result, ts.ColOffset, ts.RowOffset, m.width, isActive)
	if ts.VerticalView {
		formatted = FormatVerticalResult(ts.Result)
	}
	m.messages[ts.MsgIndex] = formatted
	m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
}

func (m *Model) renderToolCall(idx int) {
	if idx < 0 || idx >= len(m.toolCalls) {
		return
	}
	tc := &m.toolCalls[idx]
	if tc.MsgIndex < 0 || tc.MsgIndex >= len(m.messages) {
		return
	}

	isActiveTool := (idx == m.activeToolIdx)
	activeMarker := ""
	if isActiveTool && len(m.toolCalls) > 1 {
		activeMarker = fmt.Sprintf(" 🎯[Tool %d/%d]", idx+1, len(m.toolCalls))
	}

	var sb strings.Builder
	if !tc.IsExpanded {
		badge := ToolCollapsedBadge.Render("▶ 🛠️ Tool: " + tc.Name + activeMarker)
		summary := MetricsStyle.Render(fmt.Sprintf("%s (Folded - Press Ctrl+O to unfold)", tc.Summary))
		sb.WriteString(fmt.Sprintf("%s %s", badge, summary))
	} else {
		badge := ToolExpandedBadge.Render("▼ 🛠️ Tool: " + tc.Name + activeMarker)

		detailCode := tc.Detail
		if tc.Name == "execute_sql" {
			detailCode = HighlightSQL(tc.Detail)
		} else if tc.Name == "execute_javascript" {
			detailCode = HighlightJS(tc.Detail)
		}

		detail := ToolDetailStyle.Render(detailCode)
		resText := MetricsStyle.Render(tc.Result)
		sb.WriteString(fmt.Sprintf("%s\n%s\n%s", badge, detail, resText))

		// Render Raw Output / Calculation Results if present (Never hide tool output!)
		if tc.RawOutput != "" {
			outTitle := lipgloss.NewStyle().Bold(true).Foreground(SecondaryColor).Render("📊 Raw Execution Output:")
			outBox := ToolDetailStyle.Render(tc.RawOutput)
			sb.WriteString(fmt.Sprintf("\n%s\n%s", outTitle, outBox))
		}

		// Render embedded Table Result inside container when unfolded
		if tc.TableStateIndex >= 0 && tc.TableStateIndex < len(m.tableStates) {
			ts := &m.tableStates[tc.TableStateIndex]
			tableStr := FormatTableResult(ts.Result, ts.ColOffset, ts.RowOffset, m.width, tc.TableStateIndex == m.activeTable)
			if ts.VerticalView {
				tableStr = FormatVerticalResult(ts.Result)
			}
			sb.WriteString("\n\n" + tableStr)
		}
	}

	m.messages[tc.MsgIndex] = sb.String()
	m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textarea.SetWidth(max(20, msg.Width-6))
		m.viewport.Width = max(20, msg.Width-4)
		m.viewport.Height = max(5, msg.Height-15)

	case schemaLoadedMsg:
		if msg.err != nil {
			m.messages = append(m.messages, ErrorMsgStyle.Render(fmt.Sprintf("Failed to load schema for profile '%s': %v", m.profileName, msg.err)))
		} else {
			m.schemaInfo = msg.schema
		}
		if m.initialPrompt != "" {
			prompt := m.initialPrompt
			m.initialPrompt = ""

			userHeader := UserTagStyle.Render("👤 YOU")
			var userLine string
			if strings.Contains(prompt, "\n") {
				userLine = fmt.Sprintf("%s\n%s", userHeader, prompt)
			} else {
				userLine = fmt.Sprintf("%s %s", userHeader, prompt)
			}

			m.messages = append(m.messages, userLine)
			m.chatHistory = append(m.chatHistory, ai.ChatMessage{Role: "user", Content: prompt})
			m.state = StateThinking
			m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
			m.viewport.GotoBottom()
			return m, m.runAgentStepCmd()
		}
		m.state = StateIdle
		m.viewport.SetContent(strings.Join(m.messages, "\n\n"))

	case aiResponseMsg:
		if msg.err != nil {
			m.messages = append(m.messages, ErrorMsgStyle.Render(fmt.Sprintf("AI Error: %v", msg.err)))
			m.state = StateIdle
		} else {
			m.explanation = msg.response.Explanation

			if msg.response.Type == ai.TypeJS && msg.response.JSCode != "" {
				m.chatHistory = append(m.chatHistory, ai.ChatMessage{
					Role:    "assistant",
					Content: fmt.Sprintf("Call tool 'execute_javascript':\n%s", msg.response.JSCode),
				})

				lineCount := len(strings.Split(msg.response.JSCode, "\n"))
				tc := ToolCallItem{
					ID:              fmt.Sprintf("tc_%d", len(m.toolCalls)+1),
					Name:            "execute_javascript",
					Summary:         fmt.Sprintf("Executing %d lines of JS data analysis", lineCount),
					Detail:          msg.response.JSCode,
					TableStateIndex: -1,
					MsgIndex:        len(m.messages),
					IsExpanded:      false,
				}

				m.messages = append(m.messages, "")
				m.toolCalls = append(m.toolCalls, tc)
				toolIdx := len(m.toolCalls) - 1
				m.focusToolCall(toolIdx)

				ctx := context.Background()
				jsRes, jsErr := m.jsEngine.Execute(ctx, msg.response.JSCode, m.sessionStore)
				if jsErr != nil {
					m.jsRetryCount++
					m.toolCalls[toolIdx].Result = fmt.Sprintf("❌ Failed: %v", jsErr.Message)
					m.renderToolCall(toolIdx)

					if m.jsRetryCount <= m.maxJSRetries {
						retryWarn := ErrorMsgStyle.Render(fmt.Sprintf("⚠️ JS Execution Failed (Attempt %d/%d): %v", m.jsRetryCount, m.maxJSRetries, jsErr.Message))
						m.messages = append(m.messages, retryWarn)

						m.chatHistory = append(m.chatHistory, ai.ChatMessage{
							Role:    "user",
							Content: fmt.Sprintf("Tool 'execute_javascript' failed with error: %s. Please fix the code and call 'execute_javascript' again.", jsErr.Message),
						})

						m.state = StateThinking
						m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
						m.viewport.GotoBottom()
						return m, m.runAgentStepCmd()
					}
					m.messages = append(m.messages, ErrorMsgStyle.Render(fmt.Sprintf("❌ JS Execution Error (after %d retries): %v", m.maxJSRetries, jsErr.Message)))
					m.jsRetryCount = 0
					m.state = StateIdle
				} else {
					m.jsRetryCount = 0
					m.toolCalls[toolIdx].Result = "✓ JavaScript executed successfully"
					m.toolCalls[toolIdx].RawOutput = jsRes.SummaryText // Display raw JS output inside tool call container!
					m.renderToolCall(toolIdx)

					m.chatHistory = append(m.chatHistory, ai.ChatMessage{
						Role:    "user",
						Content: fmt.Sprintf("Tool 'execute_javascript' executed successfully. Output:\n%s", jsRes.SummaryText),
					})

					m.state = StateThinking
					m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
					m.viewport.GotoBottom()
					return m, m.runAgentStepCmd()
				}
			} else if msg.response.Type == ai.TypeExport && msg.response.DatasetID != "" {
				m.chatHistory = append(m.chatHistory, ai.ChatMessage{
					Role:    "assistant",
					Content: fmt.Sprintf("Call tool 'export_data': dataset_id=%s, format=%s, filepath=%s", msg.response.DatasetID, msg.response.Format, msg.response.FilePath),
				})

				tc := ToolCallItem{
					ID:              fmt.Sprintf("tc_%d", len(m.toolCalls)+1),
					Name:            "export_data",
					Summary:         fmt.Sprintf("Export %s to %s (%s) [Pending User Confirmation]", msg.response.DatasetID, msg.response.FilePath, strings.ToUpper(msg.response.Format)),
					Detail:          fmt.Sprintf("Dataset: %s | FilePath: %s | Format: %s", msg.response.DatasetID, msg.response.FilePath, msg.response.Format),
					Result:          "⏳ Pending Human Confirmation",
					TableStateIndex: -1,
					MsgIndex:        len(m.messages),
					IsExpanded:      false,
				}
				m.messages = append(m.messages, "")
				m.toolCalls = append(m.toolCalls, tc)
				toolIdx := len(m.toolCalls) - 1
				m.focusToolCall(toolIdx)

				m.pendingExport = &PendingExport{
					DatasetID: msg.response.DatasetID,
					Format:    msg.response.Format,
					FilePath:  msg.response.FilePath,
					ToolIdx:   toolIdx,
				}
				m.confirmOption = 0
				m.state = StateExportReady
			} else if msg.response.Type == ai.TypeSQL && msg.response.SQL != "" {
				m.chatHistory = append(m.chatHistory, ai.ChatMessage{
					Role:    "assistant",
					Content: fmt.Sprintf("Call tool 'execute_sql': %s", msg.response.SQL),
				})

				m.currentSQL = msg.response.SQL

				tc := ToolCallItem{
					ID:              fmt.Sprintf("tc_%d", len(m.toolCalls)+1),
					Name:            "execute_sql",
					Summary:         m.currentSQL,
					Detail:          m.currentSQL,
					TableStateIndex: -1,
					MsgIndex:        len(m.messages),
					IsExpanded:      false,
				}
				m.messages = append(m.messages, "")
				m.toolCalls = append(m.toolCalls, tc)
				toolIdx := len(m.toolCalls) - 1
				m.focusToolCall(toolIdx)

				if m.autoExecute {
					m.state = StateExecuting
					m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
					m.viewport.GotoBottom()
					return m, m.executeSQLCmd(m.currentSQL)
				}
				m.confirmOption = 0
				m.state = StateSQLReady
			} else {
				// FINAL LLM AGENT OUTPUT (No Tool Call)
				exp := msg.response.Explanation

				// Defensive guard: If LLM mistakenly output raw JS code in text instead of tool call, intercept it!
				if strings.Contains(exp, "Call tool 'execute_javascript'") || (strings.Contains(exp, "var data =") && strings.Contains(exp, "stats")) {
					jsCode := exp
					if idx := strings.Index(exp, "var "); idx >= 0 {
						jsCode = exp[idx:]
					}
					msg.response.Type = ai.TypeJS
					msg.response.JSCode = jsCode
					return m.Update(msg)
				}

				m.chatHistory = append(m.chatHistory, ai.ChatMessage{
					Role:    "assistant",
					Content: exp,
				})

				if exp != "" {
					renderedMD := RenderMarkdown(exp, m.width)
					aiMsg := AITagStyle.Render("🤖 AI") + "\n" + renderedMD
					m.messages = append(m.messages, aiMsg)
				}
				m.state = StateIdle
			}
		}
		m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
		m.viewport.GotoBottom()

	case queryExecutedMsg:
		if msg.err != nil {
			errText := fmt.Sprintf("SQL Exec Error [%s]: %s", msg.err.Code, msg.err.Message)
			m.messages = append(m.messages, ErrorMsgStyle.Render(errText))

			m.chatHistory = append(m.chatHistory, ai.ChatMessage{
				Role:    "user",
				Content: fmt.Sprintf("Tool 'execute_sql' failed with error: %s", errText),
			})

			m.state = StateThinking
			m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
			m.viewport.GotoBottom()
			return m, m.runAgentStepCmd()
		} else if msg.result != nil {
			datasetID := m.sessionStore.Save(m.currentSQL, msg.result)

			modelName := m.opts.CLIAIModel
			if modelName == "" {
				modelName = "gpt-4o"
			}
			durStr := msg.duration.Round(time.Millisecond).String()
			if msg.duration < time.Millisecond {
				durStr = fmt.Sprintf("%.2fms", float64(msg.duration.Microseconds())/1000.0)
			}
			metricsStr := fmt.Sprintf("⏱️ %s | 📊 %d rows | 🤖 %s | 💾 %s", durStr, len(msg.result.Rows), modelName, datasetID)
			statusLine := SuccessBadgeStyle.Render("✓ Execution Success") + " " + MetricsStyle.Render(metricsStr)

			ts := TableState{
				Result:       msg.result,
				MsgIndex:     -1,
				ColOffset:    0,
				RowOffset:    0,
				VerticalView: false,
			}
			m.tableStates = append(m.tableStates, ts)
			tableIdx := len(m.tableStates) - 1

			// Attach TableStateIndex directly inside execute_sql ToolCallItem
			if len(m.toolCalls) > 0 {
				lastIdx := len(m.toolCalls) - 1
				if m.toolCalls[lastIdx].Name == "execute_sql" {
					m.toolCalls[lastIdx].Result = statusLine
					m.toolCalls[lastIdx].TableStateIndex = tableIdx
					m.focusToolCall(lastIdx)
				}
			}

			m.chatHistory = append(m.chatHistory, ai.ChatMessage{
				Role:    "user",
				Content: fmt.Sprintf("Tool 'execute_sql' executed successfully. Returned %d rows (columns: %v). Dataset saved as '%s'.", len(msg.result.Rows), msg.result.Columns, datasetID),
			})

			m.state = StateThinking
			m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
			m.viewport.GotoBottom()
			return m, m.runAgentStepCmd()
		}
		m.state = StateIdle
		m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
		m.viewport.GotoBottom()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case tea.KeyMsg:
		// CTRL+C TWICE TO QUIT MECHANISM (Like Claude Code / Aider)
		if msg.Type == tea.KeyCtrlC {
			if time.Since(m.lastCtrlCTime) < 2*time.Second {
				return m, tea.Quit
			}
			m.lastCtrlCTime = time.Now()
			warnMsg := WarningBadgeStyle.Render("⚠️ Press Ctrl+C again to exit xsql AI")
			m.messages = append(m.messages, warnMsg)
			m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
			m.viewport.GotoBottom()
			return m, nil
		}

		// UNIFIED HUMAN-IN-THE-LOOP INTERACTION FOR BOTH StateExportReady AND StateSQLReady!
		if m.state == StateExportReady && m.pendingExport != nil {
			switch msg.Type {
			case tea.KeyUp:
				m.confirmOption = (m.confirmOption - 1 + 3) % 3
				return m, nil
			case tea.KeyDown:
				m.confirmOption = (m.confirmOption + 1) % 3
				return m, nil
			}

			triggerOpt := -1
			if msg.Type == tea.KeyEnter {
				triggerOpt = m.confirmOption
			} else if msg.String() == "1" {
				triggerOpt = 0
			} else if msg.String() == "2" {
				triggerOpt = 1
			} else if msg.String() == "3" || msg.Type == tea.KeyEsc {
				triggerOpt = 2
			}

			switch triggerOpt {
			case 0:
				// Option 1: Confirm & Export
				datasetRes, exists := m.sessionStore.Get(m.pendingExport.DatasetID)
				if !exists || datasetRes == nil {
					m.toolCalls[m.pendingExport.ToolIdx].Result = fmt.Sprintf("❌ Export Failed: Dataset '%s' not found", m.pendingExport.DatasetID)
					m.renderToolCall(m.pendingExport.ToolIdx)
					m.chatHistory = append(m.chatHistory, ai.ChatMessage{
						Role:    "user",
						Content: fmt.Sprintf("Tool 'export_data' failed: dataset '%s' not found in session catalog.", m.pendingExport.DatasetID),
					})
				} else {
					outPath, xe := export.ExportQueryResult(datasetRes, export.ExportFormat(m.pendingExport.Format), m.pendingExport.FilePath)
					if xe != nil {
						m.toolCalls[m.pendingExport.ToolIdx].Result = fmt.Sprintf("❌ Export Failed: %v", xe.Message)
						m.renderToolCall(m.pendingExport.ToolIdx)
						m.chatHistory = append(m.chatHistory, ai.ChatMessage{
							Role:    "user",
							Content: fmt.Sprintf("Tool 'export_data' failed to write file: %v", xe.Message),
						})
					} else {
						m.toolCalls[m.pendingExport.ToolIdx].Result = fmt.Sprintf("✓ Exported dataset '%s' to '%s' (%s)", m.pendingExport.DatasetID, outPath, strings.ToUpper(m.pendingExport.Format))
						m.renderToolCall(m.pendingExport.ToolIdx)

						statusLine := SuccessBadgeStyle.Render("✓ File Exported Success") + " " + MetricsStyle.Render(fmt.Sprintf("Exported dataset '%s' to '%s'", m.pendingExport.DatasetID, outPath))
						m.messages = append(m.messages, statusLine)

						m.chatHistory = append(m.chatHistory, ai.ChatMessage{
							Role:    "user",
							Content: fmt.Sprintf("Tool 'export_data' executed successfully. Exported dataset '%s' to local file '%s'.", m.pendingExport.DatasetID, outPath),
						})
					}
				}
				m.pendingExport = nil
				m.state = StateThinking
				m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
				m.viewport.GotoBottom()
				return m, m.runAgentStepCmd()

			case 1:
				// Option 2: Adjust Prompt
				m.state = StateIdle
				m.pendingExport = nil
				m.textarea.Focus()
				return m, nil

			case 2:
				// Option 3: Deny / Cancel Export
				m.toolCalls[m.pendingExport.ToolIdx].Result = "🚫 Export Denied by User"
				m.renderToolCall(m.pendingExport.ToolIdx)

				m.chatHistory = append(m.chatHistory, ai.ChatMessage{
					Role:    "user",
					Content: "Tool 'export_data' was denied by user.",
				})
				m.pendingExport = nil
				m.state = StateThinking
				m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
				m.viewport.GotoBottom()
				return m, m.runAgentStepCmd()
			}
			return m, nil
		}

		if m.state == StateSQLReady {
			switch msg.Type {
			case tea.KeyUp:
				m.confirmOption = (m.confirmOption - 1 + 3) % 3
				return m, nil
			case tea.KeyDown:
				m.confirmOption = (m.confirmOption + 1) % 3
				return m, nil
			}

			triggerOpt := -1
			if msg.Type == tea.KeyEnter {
				triggerOpt = m.confirmOption
			} else if msg.String() == "1" {
				triggerOpt = 0
			} else if msg.String() == "2" {
				triggerOpt = 1
			} else if msg.String() == "3" || msg.Type == tea.KeyEsc {
				triggerOpt = 2
			}

			switch triggerOpt {
			case 0:
				// Option 1: Execute SQL
				m.state = StateExecuting
				m.textarea.Focus()
				m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
				m.viewport.GotoBottom()
				return m, m.executeSQLCmd(m.currentSQL)

			case 1:
				// Option 2: Adjust Prompt / Re-generate
				m.state = StateIdle
				m.textarea.Focus()
				return m, nil

			case 2:
				// Option 3: Cancel Execution
				m.state = StateIdle
				m.textarea.Focus()
				return m, nil
			}
		}

		// MULTI-LINE PROMPT INPUT SHORTCUT: Alt+Enter to insert soft newline into textarea
		if m.state == StateIdle && ((msg.Alt && msg.Type == tea.KeyEnter) || msg.String() == "alt+enter") {
			m.textarea.InsertString("\n")
			return m, nil
		}

		switch msg.Type {
		case tea.KeyEsc:
			// ESC CLEARS PROMPT INPUT BOX (No longer quits application)
			if m.state == StateIdle {
				m.textarea.Reset()
				return m, nil
			}

		case tea.KeyCtrlP:
			// PROFILE SWITCHING FEATURE: Cycle active profile & synchronize with AI agent context!
			if len(m.profileList) > 1 {
				currIdx := -1
				for i, name := range m.profileList {
					if name == m.profileName {
						currIdx = i
						break
					}
				}
				nextIdx := (currIdx + 1) % len(m.profileList)
				nextProfileName := m.profileList[nextIdx]

				newP, ok := m.allProfiles[nextProfileName]
				if ok {
					if newP.Port == 0 {
						switch newP.DB {
						case "mysql":
							newP.Port = 3306
						case "pg":
							newP.Port = 5432
						}
					}

					m.profileName = nextProfileName
					m.profile = newP
					m.unsafeAllowWrite = newP.UnsafeAllowWrite
					m.schemaInfo = nil
					m.state = StateLoadingSchema

					switchLine := SuccessBadgeStyle.Render(fmt.Sprintf("✓ Switched active profile to '%s' (%s)", m.profileName, m.profile.DB))
					m.messages = append(m.messages, switchLine)

					m.chatHistory = append(m.chatHistory, ai.ChatMessage{
						Role:    "user",
						Content: fmt.Sprintf("System Notice: Switched active database profile to '%s' (Database: %s). New database schema metadata is being loaded.", m.profileName, m.profile.DB),
					})

					m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
					m.viewport.GotoBottom()
					return m, m.loadSchemaCmd()
				}
			}

		case tea.KeyCtrlO:
			// Toggle folding/unfolding of currently active/focused ToolCallItem
			if len(m.toolCalls) > 0 {
				if m.activeToolIdx < 0 || m.activeToolIdx >= len(m.toolCalls) {
					m.activeToolIdx = len(m.toolCalls) - 1
				}
				m.toolCalls[m.activeToolIdx].IsExpanded = !m.toolCalls[m.activeToolIdx].IsExpanded
				m.renderToolCall(m.activeToolIdx)
				return m, nil
			}

		case tea.KeyTab:
			// Tab: Cycle focus between Tool Call Containers
			if len(m.toolCalls) > 0 {
				m.focusToolCall((m.activeToolIdx + 1) % len(m.toolCalls))
				return m, nil
			}

		case tea.KeyShiftTab:
			// Shift+Tab: Toggle AUTO-EXECUTE / MANUAL approval mode
			m.autoExecute = !m.autoExecute

		case tea.KeyCtrlE:
			if m.activeTable >= 0 && m.activeTable < len(m.tableStates) {
				ts := &m.tableStates[m.activeTable]
				ts.VerticalView = !ts.VerticalView
				m.renderTableState(m.activeTable, true)
			}

		case tea.KeyLeft:
			if m.activeTable >= 0 && m.activeTable < len(m.tableStates) {
				ts := &m.tableStates[m.activeTable]
				if ts.ColOffset > 0 {
					ts.ColOffset--
					m.renderTableState(m.activeTable, true)
				}
			}

		case tea.KeyRight:
			if m.activeTable >= 0 && m.activeTable < len(m.tableStates) {
				ts := &m.tableStates[m.activeTable]
				if ts.ColOffset < len(ts.Result.Columns)-1 {
					ts.ColOffset++
					m.renderTableState(m.activeTable, true)
				}
			}

		case tea.KeyPgUp:
			if m.activeTable >= 0 && m.activeTable < len(m.tableStates) {
				ts := &m.tableStates[m.activeTable]
				if ts.RowOffset >= PageRowSize {
					ts.RowOffset -= PageRowSize
					m.renderTableState(m.activeTable, true)
					return m, nil
				}
			}
			m.viewport.LineUp(6)
			return m, nil

		case tea.KeyPgDown:
			if m.activeTable >= 0 && m.activeTable < len(m.tableStates) {
				ts := &m.tableStates[m.activeTable]
				if ts.RowOffset+PageRowSize < len(ts.Result.Rows) {
					ts.RowOffset += PageRowSize
					m.renderTableState(m.activeTable, true)
					return m, nil
				}
			}
			m.viewport.LineDown(6)
			return m, nil

		case tea.KeyUp:
			m.viewport.LineUp(1)
			return m, nil

		case tea.KeyDown:
			m.viewport.LineDown(1)
			return m, nil

		case tea.KeyEnter:
			prompt := strings.TrimSpace(m.textarea.Value())
			if prompt != "" && m.state == StateIdle {
				userHeader := UserTagStyle.Render("👤 YOU")
				var userLine string
				if strings.Contains(prompt, "\n") {
					userLine = fmt.Sprintf("%s\n%s", userHeader, prompt)
				} else {
					userLine = fmt.Sprintf("%s %s", userHeader, prompt)
				}

				m.messages = append(m.messages, userLine)
				m.chatHistory = append(m.chatHistory, ai.ChatMessage{Role: "user", Content: prompt})
				m.textarea.Reset()
				m.state = StateThinking
				m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
				m.viewport.GotoBottom()
				return m, m.runAgentStepCmd()
			}
		}
	}

	var taCmd tea.Cmd
	m.textarea, taCmd = m.textarea.Update(msg)
	cmds = append(cmds, taCmd)

	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

func renderKeybindingBadges(items [][2]string) string {
	var parts []string
	for _, item := range items {
		keyBadge := KeyBadgeStyle.Render(item[0])
		label := KeyLabelStyle.Render(item[1])
		parts = append(parts, fmt.Sprintf("%s %s", keyBadge, label))
	}
	return strings.Join(parts, "  ")
}

func renderActionOptionsCard(title string, detailText string, options []string, activeOpt int, width int) string {
	var sb strings.Builder
	sb.WriteString(SQLTitleStyle.Render(title) + "\n")
	if detailText != "" {
		sb.WriteString(detailText + "\n\n")
	}
	sb.WriteString(lipgloss.NewStyle().Foreground(MutedColor).Render("Action Options (Use ↑/↓ or 1/2/3 to select, Enter to confirm):") + "\n")
	for i, opt := range options {
		if i == activeOpt {
			sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(PrimaryColor).Render(fmt.Sprintf(" ▶ [%d] %s", i+1, opt)) + "\n")
		} else {
			sb.WriteString(lipgloss.NewStyle().Foreground(MutedColor).Render(fmt.Sprintf("   [%d] %s", i+1, opt)) + "\n")
		}
	}
	return SQLBoxStyle.Width(width - 4).Render(sb.String())
}

func (m Model) View() string {
	var sb strings.Builder

	// 1. Full-Width Header Bar
	titlePill := HeaderTitleBadge.Render("xsql AI")
	profilePill := HeaderProfileBadge.Render(fmt.Sprintf("%s (%s)", m.profileName, m.profile.DB))

	modePill := BadgeReadOnly.Render("READ-ONLY")
	if m.unsafeAllowWrite {
		modePill = BadgeReadWrite.Render("READ-WRITE")
	}

	execPill := BadgeManualApprove.Render("MANUAL")
	if m.autoExecute {
		execPill = BadgeAutoExec.Render("AUTO-EXEC")
	}

	headerContent := fmt.Sprintf("%s %s %s %s", titlePill, profilePill, modePill, execPill)
	header := HeaderBarStyle.Width(m.width).Render(headerContent)
	sb.WriteString(header + "\n\n")

	// 2. Main Viewport
	sb.WriteString(m.viewport.View() + "\n\n")

	// 3. State Status & Unified SQL / Export Action Selection Card
	switch m.state {
	case StateLoadingSchema:
		sb.WriteString(m.spinner.View() + fmt.Sprintf(" Loading database schema for profile '%s'...\n", m.profileName))
	case StateThinking:
		sb.WriteString(m.spinner.View() + " AI is analyzing schema and executing tools...\n")
	case StateExecuting:
		sb.WriteString(m.spinner.View() + " Executing SQL query...\n")
	case StateExportReady:
		if m.pendingExport != nil {
			exportInfo := fmt.Sprintf("Dataset: %s | FilePath: %s | Format: %s", m.pendingExport.DatasetID, m.pendingExport.FilePath, strings.ToUpper(m.pendingExport.Format))
			card := renderActionOptionsCard(
				"✨ File Export Approval Required",
				SQLCodeStyle.Render(exportInfo),
				[]string{"Confirm & Export File", "Adjust Prompt / Change Options", "Deny & Cancel Export"},
				m.confirmOption,
				m.width,
			)
			sb.WriteString(card + "\n")
		}
	case StateSQLReady:
		sqlContent := HighlightSQL(m.currentSQL)
		if m.currentSQL == "" {
			sqlContent = lipgloss.NewStyle().Foreground(MutedColor).Italic(true).Render("(No SQL generated)")
		}
		card := renderActionOptionsCard(
			"✨ SQL Approval Required",
			sqlContent,
			[]string{"Execute SQL Query", "Adjust Prompt / Re-generate", "Cancel Execution"},
			m.confirmOption,
			m.width,
		)
		sb.WriteString(card + "\n")
	}

	// 4. Ultra-Minimal Prompt Input Area & Dynamic Divider Rules
	divLen := m.width - 4
	if divLen < 10 {
		divLen = 10
	}
	separator := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#CBD5E1", Dark: "#334155"}).Render(strings.Repeat("─", divLen))

	sb.WriteString(separator + "\n")
	sb.WriteString(m.textarea.View() + "\n")
	sb.WriteString(separator + "\n\n")

	execModeHint := "MANUAL"
	if m.autoExecute {
		execModeHint = "AUTO"
	}

	toolFoldState := "Folded"
	if m.activeToolIdx >= 0 && m.activeToolIdx < len(m.toolCalls) && m.toolCalls[m.activeToolIdx].IsExpanded {
		toolFoldState = "Unfolded"
	}

	toolNavHint := ""
	if len(m.toolCalls) > 1 {
		toolNavHint = fmt.Sprintf(" [%d/%d]", m.activeToolIdx+1, len(m.toolCalls))
	}

	var keybindings string
	if m.state == StateExportReady || m.state == StateSQLReady {
		keybindings = renderKeybindingBadges([][2]string{
			{"↑/↓", "Select Option"},
			{"Enter", "Confirm"},
			{"1/2/3", "Quick Select"},
			{"Esc", "Cancel"},
		})
	} else {
		profileBadge := ""
		if len(m.profileList) > 1 {
			profileBadge = fmt.Sprintf(" (%s)", m.profileName)
		}
		keybindings = renderKeybindingBadges([][2]string{
			{"Enter", "Send"},
			{"Alt+Enter", "Newline"},
			{"Ctrl+P", "Profile" + profileBadge},
			{"Tab", "Focus Tool" + toolNavHint},
			{"Ctrl+O", "Tools (" + toolFoldState + ")"},
			{"←/→", "Cols"},
			{"PgUp/PgDn", "Rows"},
			{"Ctrl+E", "Expand Table"},
			{"Shift+Tab", "Mode (" + execModeHint + ")"},
			{"Esc", "Clear"},
			{"Ctrl+C", "Quit (x2)"},
		})
	}

	sb.WriteString(keybindings + "\n")

	return sb.String()
}
