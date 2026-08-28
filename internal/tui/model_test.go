package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zx06/xsql/internal/ai"
	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/errors"
)

func TestTUI_Model_StateTransitions(t *testing.T) {
	resolved := config.Resolved{
		ProfileName: "dev",
		Profile: config.Profile{
			DB: "mysql",
		},
	}
	aiService := ai.NewService(config.AIConfig{}, nil)
	m := NewModel(config.Options{}, resolved, aiService, "", false)

	// Initial State should be StateLoadingSchema
	if m.state != StateLoadingSchema {
		t.Fatalf("expected initial state StateLoadingSchema, got %v", m.state)
	}

	// 1. Send schemaLoadedMsg -> transition to StateIdle
	updated, _ := m.Update(schemaLoadedMsg{
		schema: &db.SchemaInfo{Database: "testdb"},
	})
	m = updated.(Model)
	if m.state != StateIdle {
		t.Fatalf("expected state StateIdle, got %v", m.state)
	}

	// 2. Send aiResponseMsg -> transition to StateSQLReady
	updated, _ = m.Update(aiResponseMsg{
		response: &ai.AIResponse{
			Type:        ai.TypeSQL,
			SQL:         "SELECT * FROM users;",
			Explanation: "Returns all users.",
		},
	})
	m = updated.(Model)
	if m.state != StateSQLReady {
		t.Fatalf("expected state StateSQLReady, got %v", m.state)
	}
	if m.currentSQL != "SELECT * FROM users;" {
		t.Errorf("expected SQL 'SELECT * FROM users;', got %q", m.currentSQL)
	}

	// 3. Test View Output Rendering
	viewStr := m.View()
	if !strings.Contains(viewStr, "xsql AI") {
		t.Errorf("expected view to contain header 'xsql AI', got:\n%s", viewStr)
	}
	if !strings.Contains(viewStr, "READ-ONLY") {
		t.Errorf("expected view to contain READ-ONLY badge, got:\n%s", viewStr)
	}
	if !strings.Contains(viewStr, "SELECT * FROM users;") {
		t.Errorf("expected view to contain SQL Preview, got:\n%s", viewStr)
	}

	// 4. Test KeyMsg KeyEnter in StateSQLReady -> transition to StateExecuting
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.state != StateExecuting {
		t.Fatalf("expected state StateExecuting after KeyEnter, got %v", m.state)
	}
	if cmd == nil {
		t.Fatal("expected non-nil Cmd for executeSQLCmd")
	}

	// 5. Send queryExecutedMsg -> Agent loops back with runAgentStepCmd (StateThinking)
	updated, cmd = m.Update(queryExecutedMsg{
		result: &db.QueryResult{
			Columns: []string{"id", "name"},
			Rows:    []map[string]any{{"id": 1, "name": "Alice"}},
		},
	})
	m = updated.(Model)
	if m.state != StateThinking {
		t.Fatalf("expected state StateThinking while Agent processes tool result, got %v", m.state)
	}
	if cmd == nil {
		t.Fatal("expected non-nil Cmd for runAgentStepCmd after query execution")
	}

	// 6. Send final aiResponseMsg (TypeText) -> transition to StateIdle
	updated, _ = m.Update(aiResponseMsg{
		response: &ai.AIResponse{
			Type:        ai.TypeText,
			Explanation: "Found 1 user named Alice.",
		},
	})
	m = updated.(Model)
	if m.state != StateIdle {
		t.Fatalf("expected state StateIdle after final text response, got %v", m.state)
	}

	// View output should contain query result
	viewStr = m.View()
	if !strings.Contains(viewStr, "Alice") {
		t.Errorf("expected view to contain result 'Alice', got:\n%s", viewStr)
	}
}

func TestTUI_Model_WriteGate(t *testing.T) {
	tests := []struct {
		name               string
		profileAllowsWrite bool
		cliAllowsWrite     bool
		wantWrite          bool
	}{
		{name: "neither enabled"},
		{name: "CLI only", cliAllowsWrite: true},
		{name: "profile only", profileAllowsWrite: true},
		{name: "both enabled", profileAllowsWrite: true, cliAllowsWrite: true, wantWrite: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := config.Resolved{
				ProfileName: "dev",
				Profile: config.Profile{
					DB:               "mysql",
					UnsafeAllowWrite: tt.profileAllowsWrite,
				},
			}
			m := NewModel(config.Options{}, resolved, ai.NewService(config.AIConfig{}, nil), "", tt.cliAllowsWrite)

			if m.unsafeAllowWrite != tt.wantWrite {
				t.Fatalf("unsafeAllowWrite = %v, want %v", m.unsafeAllowWrite, tt.wantWrite)
			}
			if tt.wantWrite && !strings.Contains(m.View(), "READ-WRITE") {
				t.Fatalf("expected READ-WRITE badge, got:\n%s", m.View())
			}
			if !tt.wantWrite && !strings.Contains(m.View(), "READ-ONLY") {
				t.Fatalf("expected READ-ONLY badge, got:\n%s", m.View())
			}
		})
	}
}

func TestFormatTableResult(t *testing.T) {
	res := &db.QueryResult{
		Columns: []string{"id", "username", "extra_json"},
		Rows: []map[string]any{
			{"id": 1, "username": "admin", "extra_json": "{\n  \"key\": \"very long value that exceeds column limit\"\n}"},
			{"id": 2, "username": "guest", "extra_json": nil},
		},
	}

	formatted := FormatTableResult(res, 0, 0, 80, true)
	if !strings.Contains(formatted, "admin") || !strings.Contains(formatted, "guest") {
		t.Errorf("formatted table result missing row data:\n%s", formatted)
	}
	if !strings.Contains(formatted, "NULL") {
		t.Errorf("expected NULL representation for nil value, got:\n%s", formatted)
	}
	if strings.Contains(formatted, "\n  \"key\"") {
		t.Errorf("expected newlines inside cells to be sanitized, got:\n%s", formatted)
	}

	vertFormatted := FormatVerticalResult(res)
	if !strings.Contains(vertFormatted, "Record 1 of 2") {
		t.Errorf("expected vertical view header, got:\n%s", vertFormatted)
	}
	if !strings.Contains(vertFormatted, "very long value that exceeds column limit") {
		t.Errorf("expected vertical view to display untruncated multiline text, got:\n%s", vertFormatted)
	}
}

func TestTUI_Model_InitialPromptAutoExecute(t *testing.T) {
	resolved := config.Resolved{
		ProfileName: "dev",
		Profile:     config.Profile{DB: "mysql"},
	}
	aiService := ai.NewService(config.AIConfig{}, nil)
	m := NewModel(config.Options{}, resolved, aiService, "Show total users", false)

	// When schemaLoadedMsg arrives, initialPrompt should automatically trigger StateThinking
	updated, cmd := m.Update(schemaLoadedMsg{
		schema: &db.SchemaInfo{Database: "testdb"},
	})
	m = updated.(Model)

	if m.state != StateThinking {
		t.Fatalf("expected state StateThinking when initialPrompt provided, got %v", m.state)
	}
	if cmd == nil {
		t.Fatal("expected non-nil Cmd for generateSQLCmd from initial prompt")
	}
}

func TestTUI_Model_ShiftTabAutoExecuteToggle(t *testing.T) {
	resolved := config.Resolved{
		ProfileName: "dev",
		Profile:     config.Profile{DB: "mysql"},
	}
	aiService := ai.NewService(config.AIConfig{}, nil)
	m := NewModel(config.Options{}, resolved, aiService, "", false)

	if m.autoExecute {
		t.Fatal("expected autoExecute to be false by default")
	}

	// Press Shift+Tab -> toggle to autoExecute = true
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = updated.(Model)
	if !m.autoExecute {
		t.Fatal("expected autoExecute to be true after Shift+Tab")
	}

	// Send aiResponseMsg -> should automatically transition to StateExecuting
	updated, cmd := m.Update(aiResponseMsg{
		response: &ai.AIResponse{
			Type:        ai.TypeSQL,
			SQL:         "SELECT * FROM users;",
			Explanation: "Returns users.",
		},
	})
	m = updated.(Model)
	if m.state != StateExecuting {
		t.Fatalf("expected state StateExecuting when autoExecute is true, got %v", m.state)
	}
	if cmd == nil {
		t.Fatal("expected non-nil executeSQLCmd for auto-execution")
	}
}

func TestTUI_Model_ActionOptionsCardFlow(t *testing.T) {
	resolved := config.Resolved{
		ProfileName: "dev",
		Profile:     config.Profile{DB: "mysql"},
	}
	aiService := ai.NewService(config.AIConfig{}, nil)
	m := NewModel(config.Options{}, resolved, aiService, "", false)
	m.state = StateSQLReady
	m.currentSQL = "SELECT * FROM users LIMIT 10;"

	// 1. Press Down -> switches confirmOption to 1 (Adjust Prompt)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	if m.confirmOption != 1 {
		t.Fatalf("expected confirmOption to be 1 after KeyDown, got %d", m.confirmOption)
	}

	// 2. Press Enter -> Option 1 returns to StateIdle for adjusting prompt
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.state != StateIdle {
		t.Fatalf("expected state StateIdle after selecting Adjust Prompt option, got %v", m.state)
	}

	// 3. Reset to StateSQLReady and press Enter on Option 0 -> transitions to StateExecuting
	m.state = StateSQLReady
	m.confirmOption = 0
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.state != StateExecuting {
		t.Fatalf("expected state StateExecuting after confirming execution, got %v", m.state)
	}
	if cmd == nil {
		t.Fatal("expected non-nil executeSQLCmd for executing SQL")
	}
}

func TestTUI_Model_CtrlCTwiceToQuit(t *testing.T) {
	resolved := config.Resolved{ProfileName: "dev", Profile: config.Profile{DB: "mysql"}}
	aiService := ai.NewService(config.AIConfig{}, nil)
	m := NewModel(config.Options{}, resolved, aiService, "", false)

	// 1st Ctrl+C -> should NOT quit, but set warning line
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = updated.(Model)
	if cmd != nil {
		t.Fatal("expected 1st Ctrl+C to NOT return quit cmd")
	}

	// 2nd Ctrl+C immediately -> returns tea.Quit
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected 2nd Ctrl+C to return tea.Quit")
	}
}

func TestTUI_Model_EscClearsTextarea(t *testing.T) {
	resolved := config.Resolved{ProfileName: "dev", Profile: config.Profile{DB: "mysql"}}
	aiService := ai.NewService(config.AIConfig{}, nil)
	m := NewModel(config.Options{}, resolved, aiService, "", false)
	m.state = StateIdle
	m.textarea.SetValue("draft prompt to clear")

	// Press Esc in StateIdle -> clears prompt
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.textarea.Value() != "" {
		t.Fatalf("expected textarea to be cleared after Esc, got %q", m.textarea.Value())
	}
}

func TestTUI_Model_CtrlPProfileSwitching(t *testing.T) {
	allProfiles := map[string]config.Profile{
		"dev":  {DB: "mysql", UnsafeAllowWrite: true},
		"prod": {DB: "pg"},
	}
	resolved := config.Resolved{
		ProfileName: "dev",
		Profile:     allProfiles["dev"],
		AllProfiles: allProfiles,
	}
	aiService := ai.NewService(config.AIConfig{}, nil)
	m := NewModel(config.Options{}, resolved, aiService, "", true)
	if !m.unsafeAllowWrite {
		t.Fatal("expected write mode when both CLI and initial profile allow writes")
	}

	// Press Ctrl+P -> switches profile to 'prod'
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = updated.(Model)
	if m.profileName != "prod" {
		t.Fatalf("expected profileName to switch to 'prod', got %q", m.profileName)
	}
	if m.profile.DB != "pg" {
		t.Fatalf("expected profile DB to be 'pg', got %q", m.profile.DB)
	}
	if m.unsafeAllowWrite {
		t.Fatal("expected write mode to be disabled after switching to a profile that does not allow writes")
	}
	if cmd == nil {
		t.Fatal("expected loadSchemaCmd after switching profile")
	}
}

func TestTUI_Model_ProfileSwitchRejectsMissingSSHProxy(t *testing.T) {
	allProfiles := map[string]config.Profile{
		"dev":  {DB: "mysql"},
		"prod": {DB: "pg", SSHProxy: "missing"},
	}
	resolved := config.Resolved{
		ProfileName: "dev",
		Profile:     allProfiles["dev"],
		AllProfiles: allProfiles,
	}
	m := NewModel(config.Options{}, resolved, ai.NewService(config.AIConfig{}, nil), "", false)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = updated.(Model)
	if cmd != nil {
		t.Fatal("expected invalid profile switch to avoid loading schema")
	}
	if m.profileName != "dev" {
		t.Fatalf("expected active profile to remain dev, got %q", m.profileName)
	}
	if !strings.Contains(strings.Join(m.messages, "\n"), "ssh_proxy 'missing' not found") {
		t.Fatalf("expected missing ssh_proxy error, got %v", m.messages)
	}
}

func TestTUI_Model_ToolCallsAndTableRendering(t *testing.T) {
	resolved := config.Resolved{ProfileName: "dev", Profile: config.Profile{DB: "mysql"}}
	aiService := ai.NewService(config.AIConfig{}, nil)
	m := NewModel(config.Options{}, resolved, aiService, "", false)
	m.state = StateIdle
	m.messages = append(m.messages, "", "")

	tc := ToolCallItem{
		ID:              "tc_1",
		Name:            "execute_sql",
		Summary:         "SELECT * FROM users",
		Detail:          "SELECT * FROM users",
		Result:          "✓ Execution Success",
		TableStateIndex: -1,
		MsgIndex:        0,
		IsExpanded:      false,
	}
	m.toolCalls = append(m.toolCalls, tc)

	// Test focusToolCall & renderToolCall
	m.focusToolCall(0)
	if m.activeToolIdx != 0 {
		t.Fatalf("expected activeToolIdx 0, got %d", m.activeToolIdx)
	}

	// Toggle expanded via Ctrl+O
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = updated.(Model)
	if !m.toolCalls[0].IsExpanded {
		t.Fatal("expected toolCall to be expanded after Ctrl+O")
	}

	// Test WindowSizeMsg
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = updated.(Model)
	if m.width != 100 || m.height != 40 {
		t.Fatalf("expected width 100, height 40, got %d, %d", m.width, m.height)
	}
}

func TestTUI_Model_ExportFlow(t *testing.T) {
	resolved := config.Resolved{ProfileName: "dev", Profile: config.Profile{DB: "mysql"}}
	aiService := ai.NewService(config.AIConfig{}, nil)
	m := NewModel(config.Options{}, resolved, aiService, "", false)

	// Simulate aiResponseMsg returning TypeExport
	updated, _ := m.Update(aiResponseMsg{
		response: &ai.AIResponse{
			Type:      ai.TypeExport,
			DatasetID: "res1",
			Format:    "csv",
			FilePath:  "test.csv",
		},
	})
	m = updated.(Model)
	if m.state != StateExportReady {
		t.Fatalf("expected StateExportReady, got %v", m.state)
	}
	if m.pendingExport == nil {
		t.Fatal("expected pendingExport to be non-nil")
	}

	// Select option 3 (Deny / Cancel Export) via Key '3'
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m = updated.(Model)
	if m.state != StateThinking {
		t.Fatalf("expected StateThinking after denying export, got %v", m.state)
	}
	if cmd == nil {
		t.Fatal("expected runAgentStepCmd after export feedback")
	}

	// Test TypeReport flow
	tempDir := t.TempDir()
	reportPath := filepath.Join(tempDir, "report.md")
	updated, _ = m.Update(aiResponseMsg{
		response: &ai.AIResponse{
			Type:        ai.TypeReport,
			Content:     "# Sales Report\n\n- Q1: 100",
			FilePath:    reportPath,
			Explanation: "Export markdown report",
		},
	})
	m = updated.(Model)
	if m.state != StateExportReady {
		t.Fatalf("expected StateExportReady for TypeReport, got %v", m.state)
	}
	if m.pendingExport == nil || !m.pendingExport.IsReport {
		t.Fatal("expected pendingExport to be report")
	}

	// Confirm report save
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	content, err := os.ReadFile(reportPath)
	if err != nil || !strings.Contains(string(content), "Sales Report") {
		t.Fatalf("expected report file to be written, err: %v", err)
	}
}

func TestTUI_Model_FullCoverage(t *testing.T) {
	resolved := config.Resolved{
		ProfileName: "dev",
		Profile:     config.Profile{DB: "mysql", AllowPlaintext: true},
	}
	aiService := ai.NewService(config.AIConfig{}, nil)
	m := NewModel(config.Options{}, resolved, aiService, "", false)

	// 1. Test Init & loadSchemaCmd
	initCmd := m.Init()
	if initCmd == nil {
		t.Fatal("expected non-nil Init Cmd")
	}

	loadCmd := m.loadSchemaCmd()
	if loadCmd == nil {
		t.Fatal("expected non-nil loadSchemaCmd")
	}
	_ = loadCmd() // execute closure statements

	// 2. Test runAgentStepCmd & executeSQLCmd closures
	stepCmd := m.runAgentStepCmd()
	if stepCmd != nil {
		_ = stepCmd()
	}

	execCmd := m.executeSQLCmd("SELECT 1")
	if execCmd != nil {
		_ = execCmd()
	}

	// 3. Test JS response and raw output rendering
	updated, _ := m.Update(aiResponseMsg{
		response: &ai.AIResponse{
			Type:        ai.TypeJS,
			JSCode:      "var x = 1;",
			Explanation: "Run JS script",
		},
	})
	m = updated.(Model)

	// 4. Test queryExecutedMsg with TableResult and TableState
	res := &db.QueryResult{
		Columns: []string{"id", "name"},
		Rows:    []map[string]any{{"id": 1, "name": "Alice"}},
	}
	updated, _ = m.Update(queryExecutedMsg{
		result:   res,
		duration: 10 * time.Millisecond,
	})
	m = updated.(Model)

	// 5. Test Key Navigation & renderTableState
	m.tableStates = append(m.tableStates, TableState{
		Result:       res,
		MsgIndex:     0,
		VerticalView: false,
	})
	m.messages = []string{"msg0"}
	m.renderTableState(0, true)
	m.renderTableState(0, false)
	m.tableStates[0].VerticalView = true
	m.renderTableState(0, true)

	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlE})

	exportPath := filepath.Join(t.TempDir(), "test.csv")

	// 6. Test Export Option 1 (Confirm Export)
	m.state = StateExportReady
	m.pendingExport = &PendingExport{
		DatasetID: "res1",
		Format:    "csv",
		FilePath:  exportPath,
	}
	m.confirmOption = 0
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	// 7. Test Export Option 2 (Adjust Export Prompt)
	m.state = StateExportReady
	m.toolCalls = []ToolCallItem{{ID: "tc_1", Name: "export_data"}}
	m.pendingExport = &PendingExport{
		DatasetID: "res1",
		Format:    "csv",
		FilePath:  exportPath,
		ToolIdx:   0,
	}
	m.confirmOption = 1
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.state != StateIdle {
		t.Fatalf("expected StateIdle after Adjust Prompt option, got %v", m.state)
	}
}

func TestTUI_Model_ViewAndAllStatesCoverage(t *testing.T) {
	resolved := config.Resolved{
		ProfileName: "dev",
		Profile:     config.Profile{DB: "mysql", UnsafeAllowWrite: true},
	}
	aiService := ai.NewService(config.AIConfig{}, nil)
	m := NewModel(config.Options{}, resolved, aiService, "", true) // unsafeAllowWrite = true
	m.autoExecute = true

	// 1. Test View in StateLoadingSchema
	m.state = StateLoadingSchema
	if view := m.View(); !strings.Contains(view, "READ-WRITE") || !strings.Contains(view, "AUTO-EXEC") {
		t.Fatalf("expected badges in header view, got:\n%s", view)
	}

	// 2. Test View in all states
	states := []State{
		StateIdle, StateThinking, StateSQLReady, StateExecuting, StateExportReady,
	}
	for _, st := range states {
		m.state = st
		_ = m.View()
	}

	// 3. Test schemaLoadedMsg with error
	updated, _ := m.Update(schemaLoadedMsg{
		err: errors.New("XSQL_CFG_INVALID", "invalid config", nil),
	})
	m = updated.(Model)

	// 4. Test aiResponseMsg with error & retry behavior
	// Attempt 1: Should trigger retry and set state to StateThinking
	updated, cmd := m.Update(aiResponseMsg{
		err: errors.New("XSQL_AI_API_ERROR", "api failed", nil),
	})
	m = updated.(Model)
	if m.state != StateThinking {
		t.Fatalf("expected StateThinking on first AI error retry, got %v", m.state)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd to retry step")
	}

	// Attempt 2: Should still retry (StateThinking)
	updated, _ = m.Update(aiResponseMsg{
		err: errors.New("XSQL_AI_API_ERROR", "api failed again", nil),
	})
	m = updated.(Model)
	if m.state != StateThinking {
		t.Fatalf("expected StateThinking on second AI error retry, got %v", m.state)
	}

	// Attempt 3: Exceeds maxAIRetries (2), should transition to StateIdle
	updated, _ = m.Update(aiResponseMsg{
		err: errors.New("XSQL_AI_API_ERROR", "api failed third time", nil),
	})
	m = updated.(Model)
	if m.state != StateIdle {
		t.Fatalf("expected StateIdle after exhausting AI retries, got %v", m.state)
	}

	updated, _ = m.Update(aiResponseMsg{
		response: &ai.AIResponse{
			Type:        ai.TypeText,
			Explanation: "Here is text response",
		},
	})
	m = updated.(Model)
	if m.state != StateIdle {
		t.Fatalf("expected StateIdle after TypeText response, got %v", m.state)
	}

	// 5. Test queryExecutedMsg with error
	updated, _ = m.Update(queryExecutedMsg{
		err: errors.New("XSQL_SQL_SYNTAX_ERROR", "syntax error", nil),
	})
	m = updated.(Model)

	// 6. Test Option keys '1', '2', '3' in StateSQLReady
	m.state = StateSQLReady
	m.currentSQL = "SELECT 1;"

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m.state = StateSQLReady
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m.state = StateSQLReady
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})

	// 7. Test focusToolCall out of bounds & renderTableState invalid indices
	m.focusToolCall(-1)
	m.focusToolCall(999)
	m.renderTableState(-1, false)
	m.renderTableState(999, false)
}

func TestModel_MultipleToolCalls_SequentialExecution(t *testing.T) {
	opts := config.Options{
		CLIAIModel: "test-model",
	}
	resolved := config.Resolved{
		Profile: config.Profile{
			DB: "mysql",
		},
		ProfileName: "default",
	}
	aiClient := ai.NewClient(config.AIConfig{Provider: "openai", APIKey: "test"}, nil)
	aiService := ai.NewService(config.AIConfig{Provider: "openai", APIKey: "test"}, aiClient)

	m := NewModel(opts, resolved, aiService, "", false)
	m.autoExecute = true // Auto-execute SQL so actions flow automatically

	// 1. Initial schema loaded
	updated, _ := m.Update(schemaLoadedMsg{
		schema: &db.SchemaInfo{Database: "testdb"},
	})
	m = updated.(Model)

	// 2. Receive aiResponseMsg with 2 actions: SQL + JS
	multiResp := &ai.AIResponse{
		Actions: []ai.ToolAction{
			{
				Type:        ai.TypeSQL,
				SQL:         "SELECT id, val FROM metrics;",
				Explanation: "Query metrics",
			},
			{
				Type:        ai.TypeJS,
				JSCode:      "var sum = res1.rows.reduce(function(acc, r) { return acc + r.val; }, 0); ({ sum: sum });",
				Explanation: "Sum metrics",
			},
		},
	}

	updated, cmd := m.Update(aiResponseMsg{response: multiResp})
	m = updated.(Model)
	if m.state != StateExecuting {
		t.Fatalf("expected StateExecuting after 1st SQL action, got %v", m.state)
	}
	if cmd == nil {
		t.Fatal("expected non-nil Cmd for executeSQLCmd")
	}

	// 3. Query finishes execution -> should trigger the next pending JS action!
	updated, cmd = m.Update(queryExecutedMsg{
		result: &db.QueryResult{
			Columns: []string{"id", "val"},
			Rows: []map[string]any{
				{"id": 1, "val": 10},
				{"id": 2, "val": 20},
			},
		},
	})
	m = updated.(Model)

	// Since JS executes synchronously and there are no more actions, state should transition to StateThinking with runAgentStepCmd
	if m.state != StateThinking {
		t.Fatalf("expected StateThinking after completing all actions in queue, got %v", m.state)
	}
	if cmd == nil {
		t.Fatal("expected runAgentStepCmd after all actions finish")
	}
	if len(m.toolCalls) != 2 {
		t.Fatalf("expected 2 tool calls rendered, got %d", len(m.toolCalls))
	}
	if m.toolCalls[0].Name != "execute_sql" || m.toolCalls[1].Name != "execute_javascript" {
		t.Errorf("unexpected tool call names: %s, %s", m.toolCalls[0].Name, m.toolCalls[1].Name)
	}

	// 4. Send final AI answer
	updated, _ = m.Update(aiResponseMsg{
		response: &ai.AIResponse{
			Type:        ai.TypeText,
			Explanation: "The total sum is 30.",
		},
	})
	m = updated.(Model)
	if m.state != StateIdle {
		t.Fatalf("expected StateIdle after final text response, got %v", m.state)
	}
}

func TestTUI_Model_ThemeAutoDetection(t *testing.T) {
	// Test Default Dark
	resolved := config.Resolved{
		ProfileName: "default",
		Profile:     config.Profile{DB: "mysql"},
	}
	t.Setenv("XSQL_THEME", "dark")
	mDark := NewModel(config.Options{}, resolved, nil, "", false)
	if !mDark.isDark {
		t.Fatal("expected isDark == true when XSQL_THEME=dark")
	}

	// Test Light Mode auto-detection via env var
	t.Setenv("XSQL_THEME", "light")
	mLight := NewModel(config.Options{}, resolved, nil, "", false)
	if mLight.isDark {
		t.Fatal("expected isDark == false when XSQL_THEME=light")
	}

	// Test COLORFGBG detection
	t.Setenv("XSQL_THEME", "")
	t.Setenv("COLORFGBG", "0;15") // Light background (bg=15)
	mLight2 := NewModel(config.Options{}, resolved, nil, "", false)
	if mLight2.isDark {
		t.Fatal("expected isDark == false when COLORFGBG='0;15'")
	}

	t.Setenv("COLORFGBG", "15;0") // Dark background (bg=0)
	mDark2 := NewModel(config.Options{}, resolved, nil, "", false)
	if !mDark2.isDark {
		t.Fatal("expected isDark == true when COLORFGBG='15;0'")
	}
}
