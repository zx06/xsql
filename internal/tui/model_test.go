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

	// 4. Test KeyMsg Ctrl+E -> transition to StateExecuting
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	m = updated.(Model)
	if m.state != StateExecuting {
		t.Fatalf("expected state StateExecuting after Ctrl+E, got %v", m.state)
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
		Columns: []string{"id", "username"},
		Rows: []map[string]any{
			{"id": 1, "username": "admin"},
			{"id": 2, "username": "guest"},
		},
	}

	formatted := FormatTableResult(res)
	if !strings.Contains(formatted, "admin") || !strings.Contains(formatted, "guest") {
		t.Errorf("formatted table result missing row data:\n%s", formatted)
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
