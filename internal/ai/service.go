package ai

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/errors"
)

const MaxToolFeedbackRunes = 4096

type ResponseType string

const (
	TypeSQL    ResponseType = "sql"
	TypeJS     ResponseType = "js"
	TypeExport ResponseType = "export"
	TypeReport ResponseType = "report"
	TypeText   ResponseType = "text"
)

type ToolAction struct {
	ID          string       `json:"id,omitempty"`
	Type        ResponseType `json:"type"`
	SQL         string       `json:"sql,omitempty"`
	JSCode      string       `json:"js_code,omitempty"`
	DatasetID   string       `json:"dataset_id,omitempty"`
	Format      string       `json:"format,omitempty"`
	FilePath    string       `json:"filepath,omitempty"`
	Content     string       `json:"content,omitempty"`
	Explanation string       `json:"explanation,omitempty"`
}

type AIResponse struct {
	Type        ResponseType `json:"type"`
	SQL         string       `json:"sql,omitempty"`
	JSCode      string       `json:"js_code,omitempty"`
	DatasetID   string       `json:"dataset_id,omitempty"`
	Format      string       `json:"format,omitempty"`
	FilePath    string       `json:"filepath,omitempty"`
	Content     string       `json:"content,omitempty"`
	Explanation string       `json:"explanation"`
	Actions     []ToolAction `json:"actions,omitempty"`
}

type SQLResponse = AIResponse

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

func (s *Service) GenerateSQL(ctx context.Context, userPrompt string, schemaInfo *db.SchemaInfo, dbType string) (*AIResponse, *errors.XError) {
	return s.GenerateResponse(ctx, userPrompt, schemaInfo, dbType, "")
}

func (s *Service) GenerateResponse(ctx context.Context, userPrompt string, schemaInfo *db.SchemaInfo, dbType string, catalog string) (*AIResponse, *errors.XError) {
	systemPrompt := BuildSystemPrompt(dbType, schemaInfo, catalog)

	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	return s.client.ChatCompletion(ctx, messages)
}

func (s *Service) ChatCompletion(ctx context.Context, messages []ChatMessage) (*AIResponse, *errors.XError) {
	return s.client.ChatCompletion(ctx, messages)
}

// BoundToolFeedback limits locally computed tool output before it is sent to
// the remote model. The complete output remains available in the local TUI.
func BoundToolFeedback(value string) string {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) <= MaxToolFeedbackRunes {
		return value
	}

	runes := []rune(value)
	return string(runes[:MaxToolFeedbackRunes]) + "\n...[truncated: complete result remains local]"
}
