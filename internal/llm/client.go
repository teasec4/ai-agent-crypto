package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	APIKey      string
	BaseURL     string
	Model       string
	HTTPClient  *http.Client
	Temperature float64
	MaxTokens   int
}

func NewClientWithTimeout(apiKey string, baseURL string, model string, temperature float64, maxTokens int, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Client{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   model,
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
		Temperature: temperature,
		MaxTokens:   maxTokens,
	}
}

func (c *Client) Chat(messages []Message) (string, error) {
	reqBody := Request{
		Model:       c.Model,
		Messages:    messages,
		Temperature: c.Temperature,
		MaxTokens:   c.MaxTokens,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(
		"POST",
		c.BaseURL,
		bytes.NewBuffer(body),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var res Response
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Check for API error
	if res.Error != nil && res.Error.Message != "" {
		return "", fmt.Errorf("API error: %s", res.Error.Message)
	}

	// Check if we have any choices
	if len(res.Choices) == 0 {
		return "", fmt.Errorf("no choices in API response")
	}

	return res.Choices[0].Message.Content, nil
}
