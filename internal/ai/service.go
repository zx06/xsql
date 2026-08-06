package ai

import (
	"context"

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

	return s.client.ChatCompletion(ctx, messages)
}
