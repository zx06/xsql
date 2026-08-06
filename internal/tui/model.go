package tui

import (
	"context"
	"fmt"
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
	TableStateIndex int // -1 if no table attached
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
	unsafeAllowWrite bool
	initialPrompt    string
	autoExecute      bool

	sessionStore  *session.SessionDataStore
	jsEngine      *js.JSEngine
	chatHistory   []ai.ChatMessage
	pendingExport *PendingExport
	jsRetryCount  int
	maxJSRetries  int

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

	editingSQL bool
	width      int
	height     int
}

func NewModel(opts config.Options, resolved config.Resolved, aiService *ai.Service, initialPrompt string, unsafeAllowWrite bool) Model {
	ta := textarea.New()
	ta.Placeholder = "Ask AI to generate SQL or analyze datasets (e.g. 'Show top 10 servers')...."
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.Focus()
	ta.CharLimit = 1000
	ta.SetWidth(80)
	ta.SetHeight(2)

	vp := viewport.New(80, 15)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(PrimaryColor)

	return Model{
		opts:             opts,
		aiService:        aiService,
		profile:          resolved.Profile,
		profileName:      resolved.ProfileName,
		unsafeAllowWrite: unsafeAllowWrite || resolved.Profile.UnsafeAllowWrite,
		initialPrompt:    strings.TrimSpace(initialPrompt),
		autoExecute:      false,
		sessionStore:     session.NewSessionDataStore(),
		jsEngine:         js.NewJSEngine(1 * time.Minute),
		chatHistory:      []ai.ChatMessage{},
		jsRetryCount:     0,
		maxJSRetries:     3,
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
		summary := SQLCodeStyle.Render(tc.Summary)
		detail := ToolDetailStyle.Render(tc.Detail)
		resText := MetricsStyle.Render(tc.Result)
		sb.WriteString(fmt.Sprintf("%s %s\n%s\n%s", badge, summary, detail, resText))

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
		m.textarea.SetWidth(msg.Width - 4)
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = max(5, msg.Height-14)

	case schemaLoadedMsg:
		if msg.err != nil {
			m.messages = append(m.messages, ErrorMsgStyle.Render(fmt.Sprintf("Failed to load schema: %v", msg.err)))
		} else {
			m.schemaInfo = msg.schema
		}
		if m.initialPrompt != "" {
			prompt := m.initialPrompt
			m.initialPrompt = ""
			userLine := UserTagStyle.Render("👤 YOU") + " " + prompt
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
			} else if msg.response.Type == ai.TypeTable && msg.response.DatasetID != "" {
				m.chatHistory = append(m.chatHistory, ai.ChatMessage{
					Role:    "assistant",
					Content: fmt.Sprintf("Call tool 'render_table': dataset_id=%s, title=%s", msg.response.DatasetID, msg.response.Title),
				})

				datasetRes, exists := m.sessionStore.Get(msg.response.DatasetID)
				if !exists || datasetRes == nil {
					m.messages = append(m.messages, ErrorMsgStyle.Render(fmt.Sprintf("❌ Tool render_table failed: dataset '%s' not found", msg.response.DatasetID)))
					m.chatHistory = append(m.chatHistory, ai.ChatMessage{
						Role:    "user",
						Content: fmt.Sprintf("Tool 'render_table' failed: dataset '%s' not found in session catalog.", msg.response.DatasetID),
					})
					m.state = StateThinking
					m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
					m.viewport.GotoBottom()
					return m, m.runAgentStepCmd()
				}

				ts := TableState{
					Result:       datasetRes,
					MsgIndex:     -1,
					ColOffset:    0,
					RowOffset:    0,
					VerticalView: false,
				}
				m.tableStates = append(m.tableStates, ts)
				tableIdx := len(m.tableStates) - 1

				tc := ToolCallItem{
					ID:              fmt.Sprintf("tc_%d", len(m.toolCalls)+1),
					Name:            "render_table",
					Summary:         fmt.Sprintf("Rendered interactive table view for %s (%d rows)", msg.response.DatasetID, len(datasetRes.Rows)),
					Detail:          fmt.Sprintf("Dataset: %s | Title: %s", msg.response.DatasetID, msg.response.Title),
					Result:          fmt.Sprintf("✓ Table rendered (%d rows)", len(datasetRes.Rows)),
					TableStateIndex: tableIdx,
					MsgIndex:        len(m.messages),
					IsExpanded:      false,
				}
				m.messages = append(m.messages, "")
				m.toolCalls = append(m.toolCalls, tc)
				toolIdx := len(m.toolCalls) - 1
				m.focusToolCall(toolIdx)

				m.chatHistory = append(m.chatHistory, ai.ChatMessage{
					Role:    "user",
					Content: fmt.Sprintf("Tool 'render_table' executed successfully. Interactive table view for dataset '%s' rendered for user.", msg.response.DatasetID),
				})

				m.state = StateThinking
				m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
				m.viewport.GotoBottom()
				return m, m.runAgentStepCmd()
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
				m.state = StateSQLReady
			} else {
				// FINAL LLM AGENT OUTPUT (No Tool Call)
				m.chatHistory = append(m.chatHistory, ai.ChatMessage{
					Role:    "assistant",
					Content: msg.response.Explanation,
				})

				if msg.response.Explanation != "" {
					aiMsg := AITagStyle.Render("🤖 AI") + " " + AIResponseStyle.Render(msg.response.Explanation)
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
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}

		if m.editingSQL {
			switch msg.Type {
			case tea.KeyEnter:
				editedVal := strings.TrimSpace(m.textarea.Value())
				if editedVal != "" {
					m.currentSQL = editedVal
				}
				m.editingSQL = false
				m.textarea.Reset()
				m.textarea.Blur()
				m.state = StateSQLReady
				return m, nil

			case tea.KeyEsc:
				m.editingSQL = false
				m.textarea.Reset()
				m.textarea.Focus()
				return m, nil
			}
			var taCmd tea.Cmd
			m.textarea, taCmd = m.textarea.Update(msg)
			return m, taCmd
		}

		if m.state == StateExportReady && m.pendingExport != nil {
			switch msg.Type {
			case tea.KeyEnter:
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

			case tea.KeyEsc:
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
			switch {
			case msg.Type == tea.KeyEnter:
				m.state = StateExecuting
				m.textarea.Focus()
				m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
				m.viewport.GotoBottom()
				return m, m.executeSQLCmd(m.currentSQL)

			case msg.String() == "e" || msg.String() == "E":
				m.editingSQL = true
				m.textarea.Focus()
				m.textarea.SetValue(m.currentSQL)
				m.textarea.CursorEnd()
				return m, nil

			case msg.Type == tea.KeyEsc:
				m.state = StateIdle
				m.textarea.Focus()
				return m, nil
			}
		}

		switch msg.Type {
		case tea.KeyEsc:
			return m, tea.Quit

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

		case tea.KeyTab, tea.KeyCtrlN:
			// Tab / Ctrl+N: Navigate focus to NEXT Tool Call Container
			if len(m.toolCalls) > 0 {
				m.focusToolCall((m.activeToolIdx + 1) % len(m.toolCalls))
				return m, nil
			}

		case tea.KeyShiftTab, tea.KeyCtrlP:
			// Shift+Tab / Ctrl+P: Navigate focus to PREVIOUS Tool Call Container
			if len(m.toolCalls) > 0 {
				if m.activeToolIdx <= 0 {
					m.focusToolCall(len(m.toolCalls) - 1)
				} else {
					m.focusToolCall(m.activeToolIdx - 1)
				}
				return m, nil
			}

		case tea.KeyCtrlA:
			// Ctrl+A: Toggle Auto-Execute / Manual approval mode
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
				userLine := UserTagStyle.Render("👤 YOU") + " " + prompt
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

	// 3. State Status & SQL / Export Confirmation Box
	switch m.state {
	case StateLoadingSchema:
		sb.WriteString(m.spinner.View() + " Loading database schema...\n")
	case StateThinking:
		sb.WriteString(m.spinner.View() + " AI is analyzing schema and executing tools...\n")
	case StateExecuting:
		sb.WriteString(m.spinner.View() + " Executing SQL query...\n")
	case StateExportReady:
		if m.pendingExport != nil {
			exportInfo := fmt.Sprintf("Dataset: %s | Target: %s | Format: %s", m.pendingExport.DatasetID, m.pendingExport.FilePath, strings.ToUpper(m.pendingExport.Format))
			preview := fmt.Sprintf("%s\n%s", SQLTitleStyle.Render("✨ File Export Approval Required (Enter: Confirm Export | Esc: Deny):"), SQLCodeStyle.Render(exportInfo))
			sb.WriteString(SQLBoxStyle.Width(m.width-4).Render(preview) + "\n")
		}
	case StateSQLReady:
		sqlContent := SQLCodeStyle.Render(m.currentSQL)
		if m.currentSQL == "" {
			sqlContent = lipgloss.NewStyle().Foreground(MutedColor).Italic(true).Render("(No SQL generated)")
		}
		preview := fmt.Sprintf("%s\n%s", SQLTitleStyle.Render("✨ SQL Preview (Enter: Execute | e: Edit | Esc: Cancel):"), sqlContent)
		sb.WriteString(SQLBoxStyle.Width(m.width-4).Render(preview) + "\n")
	}

	// 4. Input Area & Footer Keybindings
	promptTitle := lipgloss.NewStyle().Bold(true).Foreground(PrimaryColor).Render("✦ Ask AI:")
	if m.editingSQL {
		promptTitle = lipgloss.NewStyle().Bold(true).Foreground(AccentColor).Render("✏️  Edit SQL (Enter: Apply | Esc: Cancel):")
	}
	sb.WriteString(promptTitle + "\n")
	sb.WriteString(m.textarea.View() + "\n\n")

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
	if m.state == StateExportReady {
		keybindings = renderKeybindingBadges([][2]string{
			{"Enter", "Confirm Export"},
			{"Esc", "Deny Export"},
		})
	} else if m.state == StateSQLReady {
		keybindings = renderKeybindingBadges([][2]string{
			{"Enter", "Execute"},
			{"e", "Edit SQL"},
			{"Esc", "Cancel"},
			{"Ctrl+A", "Mode (" + execModeHint + ")"},
			{"Ctrl+O", "Tool Details (" + toolFoldState + toolNavHint + ")"},
			{"Tab/Shift+Tab", "Nav Tools"},
		})
	} else {
		keybindings = renderKeybindingBadges([][2]string{
			{"Enter", "Send"},
			{"Tab/Shift+Tab", "Focus Tool" + toolNavHint},
			{"←/→", "Cols"},
			{"PgUp/PgDn", "Rows"},
			{"Ctrl+O", "Tools (" + toolFoldState + ")"},
			{"Ctrl+E", "Expand Table"},
			{"Ctrl+A", "Mode (" + execModeHint + ")"},
			{"Esc", "Quit"},
		})
	}

	sb.WriteString(keybindings + "\n")

	return sb.String()
}
