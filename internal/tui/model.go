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

type Model struct {
	opts             config.Options
	aiService        *ai.Service
	profile          config.Profile
	profileName      string
	unsafeAllowWrite bool
	initialPrompt    string
	autoExecute      bool

	state        State
	schemaInfo   *db.SchemaInfo
	currentSQL   string
	explanation  string
	messages     []string
	lastResult   *db.QueryResult
	verticalView bool
	colOffset    int

	textarea textarea.Model
	viewport viewport.Model
	spinner  spinner.Model

	editingSQL bool
	width      int
	height     int
}

func NewModel(opts config.Options, resolved config.Resolved, aiService *ai.Service, initialPrompt string, unsafeAllowWrite bool) Model {
	ta := textarea.New()
	ta.Placeholder = "Ask AI to write a SQL query (e.g. 'Show top 10 users')..."
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
		colOffset:        0,
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

func (m *Model) renderLastResult() {
	if m.lastResult == nil || len(m.messages) == 0 {
		return
	}
	formatted := FormatTableResult(m.lastResult, m.colOffset, m.width)
	if m.verticalView {
		formatted = FormatVerticalResult(m.lastResult)
	}
	m.messages[len(m.messages)-1] = formatted
	m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
	m.viewport.GotoBottom()
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
			m.lastResult = msg.result
			m.colOffset = 0
			statusLine := SuccessBadgeStyle.Render(fmt.Sprintf("✓ Execution Success (%d rows returned)", len(msg.result.Rows)))
			m.messages = append(m.messages, statusLine)
			
			formatted := FormatTableResult(msg.result, m.colOffset, m.width)
			if m.verticalView {
				formatted = FormatVerticalResult(msg.result)
			}
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
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyLeft:
			if m.lastResult != nil && m.colOffset > 0 {
				m.colOffset--
				m.renderLastResult()
			}

		case tea.KeyRight:
			if m.lastResult != nil && m.colOffset < len(m.lastResult.Columns)-1 {
				m.colOffset++
				m.renderLastResult()
			}

		case tea.KeyShiftTab: // Toggle Auto-Execute vs Manual-Approve mode
			m.autoExecute = !m.autoExecute

		case tea.KeyCtrlV: // Toggle Vertical (psql \x) full untruncated view
			if m.lastResult != nil && len(m.messages) > 0 {
				m.verticalView = !m.verticalView
				m.renderLastResult()
			}

		case tea.KeyCtrlE: // Execute current SQL
			if m.state == StateSQLReady && m.currentSQL != "" {
				m.state = StateExecuting
				execLine := ExecutingTagStyle.Render("⚡ Executing") + " " + SQLCodeStyle.Render(m.currentSQL)
				m.messages = append(m.messages, execLine)
				m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
				m.viewport.GotoBottom()
				return m, m.executeSQLCmd(m.currentSQL)
			}

		case tea.KeyCtrlR: // Toggle Edit SQL mode
			if m.state == StateSQLReady {
				m.editingSQL = !m.editingSQL
				if m.editingSQL {
					m.textarea.SetValue(m.currentSQL)
				}
			}

		case tea.KeyEnter: // Send prompt or confirm edited SQL
			if m.editingSQL {
				m.currentSQL = m.textarea.Value()
				m.editingSQL = false
				m.textarea.Reset()
				return m, nil
			}

			prompt := strings.TrimSpace(m.textarea.Value())
			if prompt != "" && (m.state == StateIdle || m.state == StateSQLReady) {
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

	if !m.editingSQL {
		var taCmd tea.Cmd
		m.textarea, taCmd = m.textarea.Update(msg)
		cmds = append(cmds, taCmd)
	}

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
		preview := fmt.Sprintf("%s\n%s", SQLTitleStyle.Render("✨ SQL Preview (Press Ctrl+E to Execute, Ctrl+R to Edit):"), sqlContent)
		sb.WriteString(SQLBoxStyle.Width(m.width - 4).Render(preview) + "\n")
	}

	// 4. Input Area & Footer Hints
	if m.editingSQL {
		sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(AccentColor).Render("✏️  Edit SQL (Press Enter to Apply Changes):") + "\n")
	}
	sb.WriteString(m.textarea.View() + "\n")

	execModeHint := "MANUAL"
	if m.autoExecute {
		execModeHint = "AUTO"
	}
	help := fmt.Sprintf("Enter: Send | ←/→: Scroll Cols | Ctrl+E: Exec | Ctrl+R: Edit | Ctrl+V: Vertical View | Shift+Tab: Mode (%s) | Esc: Quit", execModeHint)
	sb.WriteString(HelpStyle.Render(help))

	return sb.String()
}
