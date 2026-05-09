package main

import (
	"fmt"
	"log"

	"ai-agent/internal/agent"
	"ai-agent/internal/config"
	"ai-agent/internal/llm"
	"ai-agent/internal/tools"
	"ai-agent/internal/tools/registry"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("error loading config: %v", err)
	}

	// Create LLM client with config values
	llmClient := llm.NewClientWithOptions(
		cfg.OpenAIApiKey,
		cfg.LLMBaseURL,
		cfg.LLMModel,
		cfg.LLMTemperature,
		cfg.LLMMaxTokens,
	)

	// Create tools
	cryptoTool := tools.NewCryptoTool()
	gitTool := tools.NewGitTool()

	// Create registry
	reg := registry.New(cryptoTool, gitTool)

	// Create agent
	ag := agent.NewAgent(llmClient, reg)

	// Test queries
	queries := []string{
		"А сколько стоит Солана?",
		"Какая цена биткоина?",
		"Привет!",
		"Что ты умеешь?",
	}

	for _, query := range queries {
		fmt.Printf("\n=== Запрос: %s ===\n", query)
		result := ag.Run(query)
		fmt.Printf("Ответ: %s\n", result)
	}
}
