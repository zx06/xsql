//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zx06/xsql/internal/ai"
	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/tui"
)

func TestE2E_AI_Service_With_MockOpenAI(t *testing.T) {
	// 1. Setup Mock OpenAI Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-e2e-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req ai.ChatCompletionRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		// Assert System prompt contains schema context
		hasSystem := false
		for _, msg := range req.Messages {
			if msg.Role == "system" && strings.Contains(msg.Content, "DATABASE SCHEMA") {
				hasSystem = true
				break
			}
		}
		if !hasSystem {
			t.Errorf("system prompt missing schema context: %+v", req.Messages)
		}

		resp := ai.ChatCompletionResponse{
			Choices: []ai.ChatCompletionChoice{
				{
					Message: ai.ChatMessage{
						Role:    "assistant",
						Content: `{"sql": "SELECT COUNT(*) FROM users;", "explanation": "Returns total user count."}`,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// 2. Setup AI Config and Service
	aiCfg := config.AIConfig{
		Provider:  "openai",
		BaseURL:   server.URL,
		APIKey:    "test-e2e-key",
		Model:     "gpt-4o",
		MaxTokens: 2048,
	}

	client := ai.NewClient(aiCfg, server.Client())
	service := ai.NewService(aiCfg, client)

	mockSchema := &db.SchemaInfo{
		Database: "e2e_db",
		Tables: []db.Table{
			{
				Name: "users",
				Columns: []db.Column{
					{Name: "id", Type: "bigint", PrimaryKey: true},
				},
			},
		},
	}

	// 3. Test GenerateSQL
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sqlResp, xe := service.GenerateSQL(ctx, "How many users exist?", mockSchema, "mysql")
	if xe != nil {
		t.Fatalf("unexpected error generating SQL: %v", xe)
	}

	if sqlResp.SQL != "SELECT COUNT(*) FROM users;" {
		t.Errorf("expected SQL 'SELECT COUNT(*) FROM users;', got %q", sqlResp.SQL)
	}
	if sqlResp.Explanation != "Returns total user count." {
		t.Errorf("expected explanation 'Returns total user count.', got %q", sqlResp.Explanation)
	}
}

func TestE2E_AI_TUI_Terminal_Pipe(t *testing.T) {
	// Setup temporary xsql config file
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "xsql.yaml")
	cfgContent := `profiles:
  dev:
    db: mysql
    host: 127.0.0.1
    port: 3306
    user: root
    database: test
ai:
  base_url: "https://mock.api.com"
  model: "test-model"
  api_key: "test-key"
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0600); err != nil {
		t.Fatal(err)
	}

	resolved, xe := config.Resolve(config.Options{ConfigPath: cfgPath, CLIProfile: "dev", CLIProfileSet: true})
	if xe != nil {
		t.Fatalf("failed to resolve config: %v", xe)
	}

	aiService := ai.NewService(resolved.AI, nil)
	model := tui.NewModel(config.Options{}, resolved, aiService, "Show total users", false)

	inBuf := bytes.NewBufferString("\n") // Press Enter
	outBuf := &bytes.Buffer{}

	p := tea.NewProgram(model, tea.WithInput(inBuf), tea.WithOutput(outBuf))
	
	go func() {
		time.Sleep(100 * time.Millisecond)
		p.Quit()
	}()

	if _, err := p.Run(); err != nil {
		t.Fatalf("TUI program run failed: %v", err)
	}

	outputStr := outBuf.String()
	if !strings.Contains(outputStr, "xsql AI") {
		t.Errorf("expected TUI terminal output to contain header 'xsql AI', got:\n%s", outputStr)
	}
}
