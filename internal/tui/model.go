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
)

type State int

const (
	StateLoadingSchema State = iota
	StateIdle
	StateThinking
	StateSQLReady
	StateExecuting
)

// Msg types
type schemaLoadedMsg struct {
	schema *db.SchemaInfo
	err    *errors.XError
}

type sqlGeneratedMsg struct {
	response *ai.SQLResponse
	err      *errors.XError
}

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

type Model struct {
	opts             config.Options
	aiService        *ai.Service
	profile          config.Profile
	profileName      string
	unsafeAllowWrite bool
	initialPrompt    string
	autoExecute      bool

	state       State
	schemaInfo  *db.SchemaInfo
	currentSQL  string
	explanation string
	messages    []string
	tableStates []TableState
	activeTable int

	textarea textarea.Model
	viewport viewport.Model
	spinner  spinner.Model

	editingSQL bool
	width      int
	height     int
}

func NewModel(opts config.Options, resolved config.Resolved, aiService *ai.Service, initialPrompt string, unsafeAllowWrite bool) Model {
	ta := textarea.New()
	ta.Placeholder = "Ask AI to generate a SQL query (e.g. 'Show top 10 servers')...."
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
		tableStates:      []TableState{},
		activeTable:      -1,
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

func (m Model) generateSQLCmd(prompt string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		resp, xe := m.aiService.GenerateSQL(ctx, prompt, m.schemaInfo, m.profile.DB)
		return sqlGeneratedMsg{response: resp, err: xe}
	}
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

func (m *Model) renderTableState(idx int, isActive bool) {
	if idx < 0 || idx >= len(m.tableStates) {
		return
	}
	ts := &m.tableStates[idx]
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
			m.state = StateThinking
			m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
			m.viewport.GotoBottom()
			return m, m.generateSQLCmd(prompt)
		}
		m.state = StateIdle
		m.viewport.SetContent(strings.Join(m.messages, "\n\n"))

	case sqlGeneratedMsg:
		if msg.err != nil {
			m.messages = append(m.messages, ErrorMsgStyle.Render(fmt.Sprintf("AI Error: %v", msg.err)))
			m.state = StateIdle
		} else {
			m.currentSQL = msg.response.SQL
			m.explanation = msg.response.Explanation

			aiMsg := AITagStyle.Render("🤖 AI") + " " + AIResponseStyle.Render(msg.response.Explanation)
			m.messages = append(m.messages, aiMsg)

			if msg.response.SQL != "" {
				if m.autoExecute {
					m.state = StateExecuting
					execLine := ExecutingTagStyle.Render("⚡ Auto-Executing") + " " + SQLCodeStyle.Render(m.currentSQL)
					m.messages = append(m.messages, execLine)
					m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
					m.viewport.GotoBottom()
					return m, m.executeSQLCmd(m.currentSQL)
				}
				m.state = StateSQLReady
			} else {
				m.state = StateIdle
			}
		}
		m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
		m.viewport.GotoBottom()

	case queryExecutedMsg:
		if msg.err != nil {
			m.messages = append(m.messages, ErrorMsgStyle.Render(fmt.Sprintf("SQL Exec Error [%s]: %s", msg.err.Code, msg.err.Message)))
		} else if msg.result != nil {
			modelName := m.opts.CLIAIModel
			if modelName == "" {
				modelName = "gpt-4o"
			}
			durStr := msg.duration.Round(time.Millisecond).String()
			if msg.duration < time.Millisecond {
				durStr = fmt.Sprintf("%.2fms", float64(msg.duration.Microseconds())/1000.0)
			}
			metricsStr := fmt.Sprintf("⏱️ %s | 📊 %d rows | 🤖 %s", durStr, len(msg.result.Rows), modelName)
			statusLine := SuccessBadgeStyle.Render("✓ Execution Success") + " " + MetricsStyle.Render(metricsStr)
			m.messages = append(m.messages, statusLine)

			// Remove focus from previous active table
			if m.activeTable >= 0 && m.activeTable < len(m.tableStates) {
				m.renderTableState(m.activeTable, false)
			}

			ts := TableState{
				Result:       msg.result,
				MsgIndex:     len(m.messages),
				ColOffset:    0,
				RowOffset:    0,
				VerticalView: false,
			}
			m.tableStates = append(m.tableStates, ts)
			m.activeTable = len(m.tableStates) - 1

			formatted := FormatTableResult(msg.result, 0, 0, m.width, true)
			m.messages = append(m.messages, formatted)
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

		if m.state == StateSQLReady {
			switch {
			case msg.Type == tea.KeyEnter:
				m.state = StateExecuting
				m.textarea.Focus()
				execLine := ExecutingTagStyle.Render("⚡ Executing") + " " + SQLCodeStyle.Render(m.currentSQL)
				m.messages = append(m.messages, execLine)
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

		case tea.KeyCtrlE:
			if m.activeTable >= 0 && m.activeTable < len(m.tableStates) {
				ts := &m.tableStates[m.activeTable]
				ts.VerticalView = !ts.VerticalView
				m.renderTableState(m.activeTable, true)
			}

		case tea.KeyShiftTab:
			m.autoExecute = !m.autoExecute

		case tea.KeyTab:
			if len(m.tableStates) > 1 {
				oldIdx := m.activeTable
				m.activeTable = (m.activeTable + 1) % len(m.tableStates)
				m.renderTableState(oldIdx, false)
				m.renderTableState(m.activeTable, true)
				return m, nil
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
				m.textarea.Reset()
				m.state = StateThinking
				m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
				m.viewport.GotoBottom()
				return m, m.generateSQLCmd(prompt)
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

	// 3. State Status & SQL Preview Box
	switch m.state {
	case StateLoadingSchema:
		sb.WriteString(m.spinner.View() + " Loading database schema...\n")
	case StateThinking:
		sb.WriteString(m.spinner.View() + " AI is analyzing schema and generating SQL...\n")
	case StateExecuting:
		sb.WriteString(m.spinner.View() + " Executing SQL query...\n")
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

	var keybindings string
	if m.state == StateSQLReady {
		keybindings = renderKeybindingBadges([][2]string{
			{"Enter", "Execute"},
			{"e", "Edit SQL"},
			{"Esc", "Cancel"},
			{"Shift+Tab", "Mode (" + execModeHint + ")"},
		})
	} else {
		keybindings = renderKeybindingBadges([][2]string{
			{"Enter", "Send"},
			{"Tab", "Focus Table"},
			{"←/→", "Cols"},
			{"PgUp/PgDn", "Rows"},
			{"Ctrl+E", "Expand"},
			{"Shift+Tab", "Mode (" + execModeHint + ")"},
			{"Esc", "Quit"},
		})
	}

	sb.WriteString(keybindings + "\n")

	return sb.String()
}
