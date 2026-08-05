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
		response: &ai.SQLResponse{
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

	// 5. Send queryExecutedMsg -> transition to StateIdle
	updated, _ = m.Update(queryExecutedMsg{
		result: &db.QueryResult{
			Columns: []string{"id", "name"},
			Rows:    []map[string]any{{"id": 1, "name": "Alice"}},
		},
	})
	m = updated.(Model)
	if m.state != StateIdle {
		t.Fatalf("expected state StateIdle after query executed, got %v", m.state)
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
		response: &ai.SQLResponse{
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
