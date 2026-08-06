package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zx06/xsql/internal/ai"
	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/db"
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

	// 2. Send sqlGeneratedMsg -> transition to StateSQLReady
	updated, _ = m.Update(sqlGeneratedMsg{
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

	// Send sqlGeneratedMsg -> should automatically transition to StateExecuting
	updated, cmd := m.Update(sqlGeneratedMsg{
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
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
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
		"dev":  {DB: "mysql"},
		"prod": {DB: "pg"},
	}
	resolved := config.Resolved{
		ProfileName: "dev",
		Profile:     allProfiles["dev"],
		AllProfiles: allProfiles,
	}
	aiService := ai.NewService(config.AIConfig{}, nil)
	m := NewModel(config.Options{}, resolved, aiService, "", false)

	// Press Ctrl+P -> switches profile to 'prod'
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = updated.(Model)
	if m.profileName != "prod" {
		t.Fatalf("expected profileName to switch to 'prod', got %q", m.profileName)
	}
	if m.profile.DB != "pg" {
		t.Fatalf("expected profile DB to be 'pg', got %q", m.profile.DB)
	}
	if cmd == nil {
		t.Fatal("expected loadSchemaCmd after switching profile")
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
}
