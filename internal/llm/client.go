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
	Model string
	Client  *http.Client
}

func NewClient(apiKey string, baseUrl string, model string) *Client {
	return &Client{
		APIKey:  apiKey,
		BaseURL: baseUrl,
		Model: model,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}


func (c *Client) Chat(prompt string) (string, error) {
	reqBody := Request{
		Model: c.Model,
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
		c.BaseURL,
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
