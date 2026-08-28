package ai

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

	prompt := BuildSystemPrompt("mysql", schema, "res1: users")
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}

	defaultPrompt := BuildSystemPrompt("", nil, "")
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
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"parallel_tool_calls":false`) {
			t.Errorf("expected parallel tool calls to be disabled, got %s", body)
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

func TestChatCompletionParsesMultipleToolCalls(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respBody := `{
			"id":"chatcmpl-multi",
			"object":"chat.completion",
			"created":1677652288,
			"model":"gpt-4o",
			"choices":[{
				"index":0,
				"message":{"role":"assistant","content":null,"tool_calls":[
					{"id":"call_1","type":"function","function":{"name":"execute_sql","arguments":"{\"sql\":\"SELECT 1\",\"explanation\":\"one\"}"}},
					{"id":"call_2","type":"function","function":{"name":"execute_javascript","arguments":"{\"js_code\":\"console.log(1);\",\"explanation\":\"two\"}"}}
				]},
				"finish_reason":"tool_calls"
			}]
		}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(respBody))
	}))
	defer mockServer.Close()

	cfg := config.AIConfig{BaseURL: mockServer.URL, APIKey: "test-key", Model: "gpt-4o"}
	client := NewClient(cfg, mockServer.Client())
	resp, xe := client.ChatCompletion(context.Background(), []ChatMessage{{Role: "user", Content: "run two actions"}})
	if xe != nil {
		t.Fatalf("unexpected error for multiple tool calls: %v", xe)
	}
	if len(resp.Actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(resp.Actions))
	}
	if resp.Actions[0].Type != TypeSQL || resp.Actions[0].SQL != "SELECT 1" {
		t.Errorf("unexpected first action: %+v", resp.Actions[0])
	}
	if resp.Actions[1].Type != TypeJS || resp.Actions[1].JSCode != "console.log(1);" {
		t.Errorf("unexpected second action: %+v", resp.Actions[1])
	}
}

func TestBoundToolFeedback(t *testing.T) {
	short := "compact aggregate"
	if got := BoundToolFeedback(short); got != short {
		t.Fatalf("expected short feedback unchanged, got %q", got)
	}

	long := strings.Repeat("数", MaxToolFeedbackRunes+10)
	got := BoundToolFeedback(long)
	if !strings.Contains(got, "[truncated:") {
		t.Fatalf("expected truncation marker, got suffix %q", got[len(got)-64:])
	}
	if count := len([]rune(strings.Split(got, "\n...[truncated:")[0])); count != MaxToolFeedbackRunes {
		t.Fatalf("expected %d retained runes, got %d", MaxToolFeedbackRunes, count)
	}
}

func TestGenerateResponse_MockHTTP_JSToolCall(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respBody := `{
			"id": "chatcmpl-125",
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
								"id": "call_js123",
								"type": "function",
								"function": {
									"name": "execute_javascript",
									"arguments": "{\"js_code\":\"rows.filter(r => r.status === 'ONLINE');\",\"explanation\":\"Filters online servers.\"}"
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
		Provider: "openai",
		BaseURL:  mockServer.URL,
		APIKey:   "test-key",
	}

	client := NewClient(cfg, mockServer.Client())
	service := NewService(cfg, client)

	res, xe := service.GenerateResponse(context.Background(), "filter online servers", nil, "mysql", "res1 catalog")
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}

	if res.Type != TypeJS {
		t.Errorf("expected type JS, got %q", res.Type)
	}
	if res.JSCode != "rows.filter(r => r.status === 'ONLINE');" {
		t.Errorf("unexpected JS code: %q", res.JSCode)
	}
}

func TestGenerateResponse_MockHTTP_ExportReportToolCall(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respBody := `{
			"id": "chatcmpl-126",
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
								"id": "call_report123",
								"type": "function",
								"function": {
									"name": "export_report",
									"arguments": "{\"content\":\"# Analysis Report\\n\\nAll good.\",\"filepath\":\"~/Downloads/report.md\",\"explanation\":\"Exports daily markdown report.\"}"
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
		Provider: "openai",
		BaseURL:  mockServer.URL,
		APIKey:   "test-key",
	}

	client := NewClient(cfg, mockServer.Client())
	service := NewService(cfg, client)

	res, xe := service.GenerateResponse(context.Background(), "export report", nil, "mysql", "res1 catalog")
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}

	if res.Type != TypeReport {
		t.Errorf("expected type Report, got %q", res.Type)
	}
	if res.Content != "# Analysis Report\n\nAll good." {
		t.Errorf("unexpected content: %q", res.Content)
	}
	if res.FilePath != "~/Downloads/report.md" {
		t.Errorf("unexpected filepath: %q", res.FilePath)
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

func TestService_ChatCompletion(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respBody := `{
			"id": "chatcmpl-999",
			"object": "chat.completion",
			"created": 1677652288,
			"model": "gpt-4o",
			"choices": [
				{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "Hello from ChatCompletion"
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
	}

	client := NewClient(cfg, mockServer.Client())
	service := NewService(cfg, client)

	msgs := []ChatMessage{
		{Role: "user", Content: "Hello"},
	}

	res, xe := service.ChatCompletion(context.Background(), msgs)
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}
	if res.Explanation != "Hello from ChatCompletion" {
		t.Errorf("expected explanation 'Hello from ChatCompletion', got %q", res.Explanation)
	}
}
