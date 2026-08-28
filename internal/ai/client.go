package ai

import (
	"context"
	"encoding/json"
	"fmt"
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

	exportToolDef := openai.ChatCompletionToolParam{
		Function: shared.FunctionDefinitionParam{
			Name:        "export_data",
			Description: openai.String("Export a cached session dataset (e.g. res1, res2) to a local file in CSV or JSON format after human confirmation."),
			Parameters: shared.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"dataset_id": map[string]interface{}{
						"type":        "string",
						"description": "The dataset ID from session catalog to export (e.g. 'res1').",
					},
					"format": map[string]interface{}{
						"type":        "string",
						"description": "Export file format: 'csv' or 'json'.",
						"enum":        []string{"csv", "json"},
					},
					"filepath": map[string]interface{}{
						"type":        "string",
						"description": "Target file path (e.g. 'result.csv', 'report.json').",
					},
					"explanation": map[string]interface{}{
						"type":        "string",
						"description": "Explanation of the data being exported.",
					},
				},
				"required": []string{"dataset_id", "format", "filepath", "explanation"},
			},
		},
	}

	reportToolDef := openai.ChatCompletionToolParam{
		Function: shared.FunctionDefinitionParam{
			Name:        "export_report",
			Description: openai.String("Export a comprehensive Markdown analysis report to a local file after human confirmation."),
			Parameters: shared.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"content": map[string]interface{}{
						"type":        "string",
						"description": "The complete Markdown text content of the analysis report.",
					},
					"filepath": map[string]interface{}{
						"type":        "string",
						"description": "Target file path (e.g. 'summary_report.md', '~/Downloads/report.md').",
					},
					"explanation": map[string]interface{}{
						"type":        "string",
						"description": "Brief explanation of the report being saved.",
					},
				},
				"required": []string{"content", "filepath", "explanation"},
			},
		},
	}

	model := c.cfg.Model
	if model == "" {
		model = "gpt-4o"
	}

	params := openai.ChatCompletionNewParams{
		Model:             shared.ChatModel(model),
		Messages:          sdkMessages,
		Tools:             []openai.ChatCompletionToolParam{sqlToolDef, jsToolDef, exportToolDef, reportToolDef},
		ParallelToolCalls: openai.Bool(false),
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

	if len(msg.ToolCalls) > 0 {
		actions := make([]ToolAction, 0, len(msg.ToolCalls))
		for _, toolCall := range msg.ToolCalls {
			switch toolCall.Function.Name {
			case "execute_sql":
				var raw struct {
					SQL         string `json:"sql"`
					Explanation string `json:"explanation"`
				}
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &raw); err != nil {
					return nil, invalidToolArguments(toolCall.Function.Name, err)
				}
				actions = append(actions, ToolAction{
					ID:          toolCall.ID,
					Type:        TypeSQL,
					SQL:         strings.TrimSpace(raw.SQL),
					Explanation: strings.TrimSpace(raw.Explanation),
				})

			case "execute_javascript":
				var raw struct {
					JSCode      string `json:"js_code"`
					Explanation string `json:"explanation"`
				}
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &raw); err != nil {
					return nil, invalidToolArguments(toolCall.Function.Name, err)
				}
				actions = append(actions, ToolAction{
					ID:          toolCall.ID,
					Type:        TypeJS,
					JSCode:      strings.TrimSpace(raw.JSCode),
					Explanation: strings.TrimSpace(raw.Explanation),
				})

			case "export_data":
				var raw struct {
					DatasetID   string `json:"dataset_id"`
					Format      string `json:"format"`
					FilePath    string `json:"filepath"`
					Explanation string `json:"explanation"`
				}
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &raw); err != nil {
					return nil, invalidToolArguments(toolCall.Function.Name, err)
				}
				actions = append(actions, ToolAction{
					ID:          toolCall.ID,
					Type:        TypeExport,
					DatasetID:   strings.TrimSpace(raw.DatasetID),
					Format:      strings.ToLower(strings.TrimSpace(raw.Format)),
					FilePath:    strings.TrimSpace(raw.FilePath),
					Explanation: strings.TrimSpace(raw.Explanation),
				})

			case "export_report":
				var raw struct {
					Content     string `json:"content"`
					FilePath    string `json:"filepath"`
					Explanation string `json:"explanation"`
				}
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &raw); err != nil {
					return nil, invalidToolArguments(toolCall.Function.Name, err)
				}
				actions = append(actions, ToolAction{
					ID:          toolCall.ID,
					Type:        TypeReport,
					Content:     strings.TrimSpace(raw.Content),
					FilePath:    strings.TrimSpace(raw.FilePath),
					Explanation: strings.TrimSpace(raw.Explanation),
				})

			default:
				return nil, errors.New(errors.CodeInternal, "AI provider returned an unsupported tool call", map[string]any{
					"tool": toolCall.Function.Name,
				})
			}
		}

		first := actions[0]
		return &AIResponse{
			Type:        first.Type,
			SQL:         first.SQL,
			JSCode:      first.JSCode,
			DatasetID:   first.DatasetID,
			Format:      first.Format,
			FilePath:    first.FilePath,
			Content:     first.Content,
			Explanation: first.Explanation,
			Actions:     actions,
		}, nil
	}

	content := strings.TrimSpace(msg.Content)
	return &AIResponse{
		Type:        TypeText,
		SQL:         "",
		Explanation: content,
	}, nil
}

func invalidToolArguments(toolName string, err error) *errors.XError {
	msg := fmt.Sprintf("Invalid JSON arguments for tool '%s': %v. Ensure all string properties (especially multiline JS code and Markdown reports) are properly JSON-escaped with valid '\\n' newlines and escaped quotes.", toolName, err)
	return errors.New(errors.CodeInternal, msg, map[string]any{
		"tool": toolName,
		"err":  err.Error(),
	})
}
