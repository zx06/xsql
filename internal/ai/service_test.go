package ai

import (
	"context"
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

	defaultPrompt := BuildSystemPrompt("", nil)
	if defaultPrompt == "" {
		t.Fatal("expected non-empty default prompt")
	}
}

func TestNewService_NilClient(t *testing.T) {
	cfg := config.AIConfig{
		Provider: "openai",
		APIKey:   "key",
	}
	svc := NewService(cfg, nil)
	if svc == nil || svc.client == nil {
		t.Fatal("expected non-nil Service and Client")
	}
}

func TestGenerateSQL_MockHTTP_ToolCall(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}

		respBody := `{
			"id": "chatcmpl-123",
			"object": "chat.completion",
			"created": 1677652288,
			"model": "gpt-4o",
			"choices": [
				{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": null,
						"tool_calls": [
							{
								"id": "call_abc123",
								"type": "function",
								"function": {
									"name": "execute_sql",
									"arguments": "{\"sql\":\"SELECT id, name FROM users;\",\"explanation\":\"Queries all users.\"}"
								}
							}
						]
					},
					"finish_reason": "tool_calls"
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(respBody))
	}))
	defer mockServer.Close()

	cfg := config.AIConfig{
		Provider:  "openai",
		BaseURL:   mockServer.URL,
		APIKey:    "test-key",
		Model:     "gpt-4o",
		MaxTokens: 100,
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

func TestGenerateSQL_MockHTTP_TextMessageFallback(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respBody := `{
			"id": "chatcmpl-124",
			"object": "chat.completion",
			"created": 1677652288,
			"model": "gpt-4o",
			"choices": [
				{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "I cannot answer this question based on the schema."
					},
					"finish_reason": "stop"
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(respBody))
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

	res, xe := service.GenerateSQL(context.Background(), "unknown table", nil, "mysql")
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}

	if res.SQL != "" {
		t.Errorf("expected empty SQL, got %q", res.SQL)
	}
	if res.Explanation != "I cannot answer this question based on the schema." {
		t.Errorf("expected explanation, got %q", res.Explanation)
	}
}

func TestGenerateSQL_MockHTTP_APIError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": {"message": "internal server error"}}`))
	}))
	defer mockServer.Close()

	cfg := config.AIConfig{
		Provider: "openai",
		BaseURL:  mockServer.URL,
		APIKey:   "test-key",
	}

	client := NewClient(cfg, mockServer.Client())
	service := NewService(cfg, client)

	_, xe := service.GenerateSQL(context.Background(), "test", nil, "mysql")
	if xe == nil {
		t.Fatal("expected error for HTTP 500")
	}
}
