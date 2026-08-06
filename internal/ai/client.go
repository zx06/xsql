package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"

	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/errors"
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Client struct {
	cfg          config.AIConfig
	openaiClient openai.Client
}

func NewClient(cfg config.AIConfig, httpClient *http.Client) *Client {
	opts := []option.RequestOption{}

	if cfg.APIKey != "" {
		opts = append(opts, option.WithAPIKey(cfg.APIKey))
	}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(strings.TrimRight(cfg.BaseURL, "/")))
	}
	if httpClient != nil {
		opts = append(opts, option.WithHTTPClient(httpClient))
	}

	cli := openai.NewClient(opts...)
	return &Client{
		cfg:          cfg,
		openaiClient: cli,
	}
}

func (c *Client) ChatCompletion(ctx context.Context, messages []ChatMessage) (*AIResponse, *errors.XError) {
	sdkMessages := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))
	for _, m := range messages {
		switch m.Role {
		case "system":
			sdkMessages = append(sdkMessages, openai.SystemMessage(m.Content))
		case "user":
			sdkMessages = append(sdkMessages, openai.UserMessage(m.Content))
		case "assistant":
			sdkMessages = append(sdkMessages, openai.AssistantMessage(m.Content))
		default:
			sdkMessages = append(sdkMessages, openai.UserMessage(m.Content))
		}
	}

	sqlToolDef := openai.ChatCompletionToolParam{
		Function: shared.FunctionDefinitionParam{
			Name:        "execute_sql",
			Description: openai.String("Execute or present generated SQL query based on database schema and user intent"),
			Parameters: shared.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"sql": map[string]interface{}{
						"type":        "string",
						"description": "The generated SQL query statement.",
					},
					"explanation": map[string]interface{}{
						"type":        "string",
						"description": "Explanation of what the query does or why SQL generation failed.",
					},
				},
				"required": []string{"sql", "explanation"},
			},
		},
	}

	jsToolDef := openai.ChatCompletionToolParam{
		Function: shared.FunctionDefinitionParam{
			Name:        "execute_javascript",
			Description: openai.String("Execute JavaScript code in local goja VM sandbox to aggregate, transform, join, or format active session datasets (res1, res2, rows)."),
			Parameters: shared.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"js_code": map[string]interface{}{
						"type":        "string",
						"description": "The JavaScript code snippet to execute on active session datasets.",
					},
					"explanation": map[string]interface{}{
						"type":        "string",
						"description": "Explanation of what the JavaScript processing code does.",
					},
				},
				"required": []string{"js_code", "explanation"},
			},
		},
	}

	model := c.cfg.Model
	if model == "" {
		model = "gpt-4o"
	}

	params := openai.ChatCompletionNewParams{
		Model:    shared.ChatModel(model),
		Messages: sdkMessages,
		Tools:    []openai.ChatCompletionToolParam{sqlToolDef, jsToolDef},
	}
	if c.cfg.MaxTokens > 0 {
		params.MaxTokens = openai.Int(int64(c.cfg.MaxTokens))
	}

	resp, err := c.openaiClient.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, errors.New(errors.CodeDBExecFailed, "AI provider returned error", map[string]any{
			"err": err.Error(),
		})
	}

	if len(resp.Choices) == 0 {
		return nil, errors.New(errors.CodeInternal, "AI provider returned empty choices", nil)
	}

	choice := resp.Choices[0]
	msg := choice.Message

	for _, toolCall := range msg.ToolCalls {
		if toolCall.Function.Name == "execute_sql" {
			var raw struct {
				SQL         string `json:"sql"`
				Explanation string `json:"explanation"`
			}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &raw); err == nil {
				return &AIResponse{
					Type:        TypeSQL,
					SQL:         strings.TrimSpace(raw.SQL),
					Explanation: strings.TrimSpace(raw.Explanation),
				}, nil
			}
		} else if toolCall.Function.Name == "execute_javascript" {
			var raw struct {
				JSCode      string `json:"js_code"`
				Explanation string `json:"explanation"`
			}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &raw); err == nil {
				return &AIResponse{
					Type:        TypeJS,
					JSCode:      strings.TrimSpace(raw.JSCode),
					Explanation: strings.TrimSpace(raw.Explanation),
				}, nil
			}
		}
	}

	content := strings.TrimSpace(msg.Content)
	return &AIResponse{
		Type:        TypeText,
		SQL:         "",
		Explanation: content,
	}, nil
}
