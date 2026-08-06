package tui

import (
	"context"
	"fmt"
	"strings"

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
	result *db.QueryResult
	err    *errors.XError
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
	ta.Placeholder = "Ask AI to write a SQL query (e.g. 'Show top 10 users')...."
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
		ctx := context.Background()
		res, xe := app.Query(ctx, app.QueryRequest{
			Profile:          m.profile,
			SQL:              sqlStr,
			AllowPlaintext:   m.profile.AllowPlaintext,
			SkipHostKeyCheck: m.profile.SSHConfig != nil && m.profile.SSHConfig.SkipHostKey,
			UnsafeAllowWrite: m.unsafeAllowWrite,
		})
		return queryExecutedMsg{result: res, err: xe}
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
			statusLine := SuccessBadgeStyle.Render(fmt.Sprintf("✓ Execution Success (%d rows returned)", len(msg.result.Rows)))
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
		// 1. Ctrl+C always quits
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}

		// 2. When editing SQL in text area
		if m.editingSQL {
			switch msg.Type {
			case tea.KeyEnter:
				editedVal := strings.TrimSpace(m.textarea.Value())
				if editedVal != "" {
					m.currentSQL = editedVal
				}
				m.editingSQL = false
				m.textarea.Reset()
				m.textarea.Blur() // Blur textarea to avoid capturing next Enter key
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

		// 3. When in SQLReady state (SQL preview pending approval)
		if m.state == StateSQLReady {
			switch {
			case msg.Type == tea.KeyEnter: // Enter to Execute SQL
				m.state = StateExecuting
				m.textarea.Focus() // Restore focus for next prompt input
				execLine := ExecutingTagStyle.Render("⚡ Executing") + " " + SQLCodeStyle.Render(m.currentSQL)
				m.messages = append(m.messages, execLine)
				m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
				m.viewport.GotoBottom()
				return m, m.executeSQLCmd(m.currentSQL)

			case msg.String() == "e" || msg.String() == "E": // 'e' key to Edit SQL
				m.editingSQL = true
				m.textarea.Focus()
				m.textarea.SetValue(m.currentSQL)
				m.textarea.CursorEnd()
				return m, nil

			case msg.Type == tea.KeyEsc: // Esc to Cancel SQL preview
				m.state = StateIdle
				m.textarea.Focus()
				return m, nil
			}
		}

		// 4. General TUI keyhandlers
		switch msg.Type {
		case tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyCtrlE: // Ctrl+E to Expand/Collapse full vertical view
			if m.activeTable >= 0 && m.activeTable < len(m.tableStates) {
				ts := &m.tableStates[m.activeTable]
				ts.VerticalView = !ts.VerticalView
				m.renderTableState(m.activeTable, true)
			}

		case tea.KeyShiftTab: // Toggle Auto-Execute vs Manual-Approve mode
			m.autoExecute = !m.autoExecute

		case tea.KeyTab: // Toggle focus between tables in history
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

func (m Model) View() string {
	var sb strings.Builder

	// 1. Header Bar
	modeBadge := BadgeReadOnly.Render("READ-ONLY")
	if m.unsafeAllowWrite {
		modeBadge = BadgeReadWrite.Render("READ-WRITE")
	}
	execModeBadge := BadgeManualApprove.Render("MANUAL-APPROVE")
	if m.autoExecute {
		execModeBadge = BadgeAutoExec.Render("AUTO-EXECUTE")
	}
	header := fmt.Sprintf(" xsql AI | Profile: %s (%s) | %s | Mode: %s ", m.profileName, m.profile.DB, modeBadge, execModeBadge)
	sb.WriteString(HeaderStyle.Width(m.width).Render(header) + "\n\n")

	// 2. Main Viewport (Messages & Results)
	sb.WriteString(m.viewport.View() + "\n\n")

	// 3. State Status & SQL Preview Card
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
		preview := fmt.Sprintf("%s\n%s", SQLTitleStyle.Render("✨ SQL Preview (Enter: Execute | e: Edit SQL | Esc: Cancel):"), sqlContent)
		sb.WriteString(SQLBoxStyle.Width(m.width-4).Render(preview) + "\n")
	}

	// 4. Input Area & Footer Hints
	if m.editingSQL {
		sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(AccentColor).Render("✏️  Edit SQL (Enter: Apply | Esc: Cancel):") + "\n")
	}
	sb.WriteString(m.textarea.View() + "\n")

	execModeHint := "MANUAL"
	if m.autoExecute {
		execModeHint = "AUTO"
	}

	help := fmt.Sprintf("Enter: Send Prompt | Tab: Focus Table | ←/→: Cols | PgUp/PgDn: Rows | Ctrl+E: Expand/Collapse | Shift+Tab: Mode (%s) | Esc: Quit", execModeHint)
	if m.state == StateSQLReady {
		help = fmt.Sprintf("Enter: Execute SQL | e: Edit SQL | Esc: Cancel | Ctrl+E: Expand/Collapse | Shift+Tab: Mode (%s)", execModeHint)
	}
	sb.WriteString(HelpStyle.Render(help))

	return sb.String()
}
