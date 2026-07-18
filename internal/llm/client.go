package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type Client struct {
	Model       Model
	Temperature float64
	MaxTokens   int
	HTTPClient  *http.Client
	Logger      *slog.Logger
}

func NewClientWithTimeout(apiKey string, baseURL string, model string, temperature float64, maxTokens int, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Client{
		Model:       *NewModel(baseURL, apiKey, model),
		Temperature: temperature,
		MaxTokens:   maxTokens,
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
		Logger: slog.Default(),
	}
}

// Chat sends messages with optional tools and returns a parsed ChatResponse.
func (c *Client) Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (*ChatResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	reqBody := Request{
		Model:       c.Model.Model,
		Messages:    messages,
		Tools:       tools,
		Temperature: c.Temperature,
		MaxTokens:   c.MaxTokens,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	c.Logger.Debug("llm request",
		"model", c.Model.Model,
		"messages", len(messages),
		"tools", len(tools),
		"body_size", len(body),
	)

	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		c.Model.BaseURL,
		bytes.NewBuffer(body),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Model.ApiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var res Response
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Check for API error
	if res.Error != nil && res.Error.Message != "" {
		return nil, fmt.Errorf("API error: %s", res.Error.Message)
	}

	// Check if we have any choices
	if len(res.Choices) == 0 {
		return nil, fmt.Errorf("no choices in API response")
	}

	choice := res.Choices[0]
	chatResp := &ChatResponse{
		Content:      choice.Message.Content,
		FinishReason: choice.FinishReason,
		ToolCalls:    choice.Message.ToolCalls,
	}

	c.Logger.Debug("llm response",
		"finish_reason", choice.FinishReason,
		"content_len", len(choice.Message.Content),
		"tool_calls", len(choice.Message.ToolCalls),
	)

	return chatResp, nil
}
