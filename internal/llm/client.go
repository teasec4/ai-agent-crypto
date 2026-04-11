package llm

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type Client struct{
	APIKey string
}

func NewClient(apiKey string) *Client{
	return &Client{
		APIKey: apiKey,
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
}

func (c *Client) Chat(prompt string) (string, error){
	reqBody := Request{
		Model: "deepseek-chat",
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
	}
	
	body, _ := json.Marshal(reqBody)
	
	req, _ := http.NewRequest(
		"POST",
		"https://api.deepseek.com/v1/chat/completions",
		bytes.NewBuffer(body),
	)
	
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	var res Response
	json.NewDecoder(resp.Body).Decode(&res)

	return res.Choices[0].Message.Content, nil
}