package ai

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/errors"
)

type SQLResponse struct {
	SQL         string `json:"sql"`
	Explanation string `json:"explanation"`
}

type Service struct {
	client *Client
}

func NewService(cfg config.AIConfig, client *Client) *Service {
	if client == nil {
		client = NewClient(cfg, nil)
	}
	return &Service{
		client: client,
	}
}

func (s *Service) GenerateSQL(ctx context.Context, userPrompt string, schemaInfo *db.SchemaInfo, dbType string) (*SQLResponse, *errors.XError) {
	systemPrompt := BuildSystemPrompt(dbType, schemaInfo)

	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	content, xe := s.client.ChatCompletion(ctx, messages)
	if xe != nil {
		return nil, xe
	}

	return parseSQLResponse(content)
}

var codeBlockRegex = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)\\s*```")

func parseSQLResponse(content string) (*SQLResponse, *errors.XError) {
	cleaned := strings.TrimSpace(content)
	if matches := codeBlockRegex.FindStringSubmatch(cleaned); len(matches) > 1 {
		cleaned = strings.TrimSpace(matches[1])
	}

	var resp SQLResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err == nil {
		resp.SQL = strings.TrimSpace(resp.SQL)
		resp.Explanation = strings.TrimSpace(resp.Explanation)
		return &resp, nil
	}

	// Fallback if AI returned raw SQL or raw text
	if strings.HasPrefix(strings.ToUpper(cleaned), "SELECT") ||
		strings.HasPrefix(strings.ToUpper(cleaned), "WITH") ||
		strings.HasPrefix(strings.ToUpper(cleaned), "SHOW") ||
		strings.HasPrefix(strings.ToUpper(cleaned), "EXPLAIN") {
		return &SQLResponse{
			SQL:         cleaned,
			Explanation: "Generated SQL based on request.",
		}, nil
	}

	return &SQLResponse{
		SQL:         "",
		Explanation: cleaned,
	}, nil
}
