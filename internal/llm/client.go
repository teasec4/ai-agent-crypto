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
	APIKey  string
	BaseURL string
	Client  *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		APIKey:  apiKey,
		BaseURL: "https://api.deepseek.com",
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Request struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Response struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *APIError `json:"error,omitempty"`
}

type APIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param"`
	Code    string `json:"code"`
}

func (c *Client) Chat(prompt string) (string, error) {
	reqBody := Request{
		Model: "deepseek-chat",
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("faild to marshal request: %w", err)
	}

	req, err := http.NewRequest(
		"POST",
		"https://api.deepseek.com/v1/chat/completions",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
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
