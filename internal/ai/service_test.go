package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/db"
)

func TestBuildSystemPrompt(t *testing.T) {
	schema := &db.SchemaInfo{
		Database: "testdb",
		Tables: []db.Table{
			{
				Name: "users",
				Columns: []db.Column{
					{Name: "id", Type: "int", PrimaryKey: true},
					{Name: "name", Type: "varchar"},
				},
			},
		},
	}

	prompt := BuildSystemPrompt("mysql", schema)
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
}

func TestGenerateSQL_MockHTTP(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}

		resp := ChatCompletionResponse{
			Choices: []ChatCompletionChoice{
				{
					Message: ChatMessage{
						Role:    "assistant",
						Content: `{"sql": "SELECT id, name FROM users;", "explanation": "Queries all users."}`,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	cfg := config.AIConfig{
		Provider: "openai",
		BaseURL:  mockServer.URL,
		APIKey:   "test-key",
		Model:    "gpt-4o",
	}

	client := NewClient(cfg, mockServer.Client())
	service := NewService(cfg, client)

	res, xe := service.GenerateSQL(context.Background(), "show users", nil, "mysql")
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}

	if res.SQL != "SELECT id, name FROM users;" {
		t.Errorf("expected SQL 'SELECT id, name FROM users;', got %q", res.SQL)
	}
	if res.Explanation != "Queries all users." {
		t.Errorf("expected explanation 'Queries all users.', got %q", res.Explanation)
	}
}

func TestParseSQLResponse_Fallback(t *testing.T) {
	resp, xe := parseSQLResponse("SELECT * FROM users")
	if xe != nil {
		t.Fatal(xe)
	}
	if resp.SQL != "SELECT * FROM users" {
		t.Errorf("expected raw SQL fallback, got %q", resp.SQL)
	}
}
