package ai

import (
	"context"

	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/errors"
)

type ResponseType string

const (
	TypeSQL    ResponseType = "sql"
	TypeJS     ResponseType = "js"
	TypeExport ResponseType = "export"
	TypeText   ResponseType = "text"
)

type AIResponse struct {
	Type        ResponseType `json:"type"`
	SQL         string       `json:"sql,omitempty"`
	JSCode      string       `json:"js_code,omitempty"`
	DatasetID   string       `json:"dataset_id,omitempty"`
	Format      string       `json:"format,omitempty"`
	FilePath    string       `json:"filepath,omitempty"`
	Explanation string       `json:"explanation"`
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
