package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/errors"
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionRequest struct {
	Model     string        `json:"model"`
	Messages  []ChatMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens,omitempty"`
}

type ChatCompletionChoice struct {
	Message ChatMessage `json:"message"`
}

type ChatCompletionResponse struct {
	Choices []ChatCompletionChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

type Client struct {
	cfg        config.AIConfig
	httpClient *http.Client
}

func NewClient(cfg config.AIConfig, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		cfg:        cfg,
		httpClient: httpClient,
	}
}

func (c *Client) ChatCompletion(ctx context.Context, messages []ChatMessage) (string, *errors.XError) {
	baseURL := strings.TrimRight(c.cfg.BaseURL, "/")
	url := fmt.Sprintf("%s/chat/completions", baseURL)

	reqBody := ChatCompletionRequest{
		Model:     c.cfg.Model,
		Messages:  messages,
		MaxTokens: c.cfg.MaxTokens,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", errors.New(errors.CodeInternal, "failed to marshal AI request", map[string]any{"err": err.Error()})
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return "", errors.New(errors.CodeInternal, "failed to create AI HTTP request", map[string]any{"err": err.Error()})
	}

	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.cfg.APIKey))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", errors.New(errors.CodeDBConnectFailed, "failed to connect to AI service", map[string]any{"err": err.Error(), "url": url})
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.New(errors.CodeInternal, "failed to read AI response", map[string]any{"err": err.Error()})
	}

	if resp.StatusCode != http.StatusOK {
		return "", errors.New(errors.CodeDBExecFailed, "AI provider returned non-200 error", map[string]any{
			"status": resp.StatusCode,
			"body":   string(respBytes),
		})
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		return "", errors.New(errors.CodeInternal, "failed to parse AI response JSON", map[string]any{"err": err.Error()})
	}

	if chatResp.Error != nil {
		return "", errors.New(errors.CodeDBExecFailed, "AI provider returned error", map[string]any{
			"message": chatResp.Error.Message,
			"code":    chatResp.Error.Code,
		})
	}

	if len(chatResp.Choices) == 0 {
		return "", errors.New(errors.CodeInternal, "AI provider returned empty choices", nil)
	}

	return chatResp.Choices[0].Message.Content, nil
}
