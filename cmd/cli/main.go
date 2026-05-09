package main

import (
	"ai-agent/internal/agent"
	"ai-agent/internal/config"
	"ai-agent/internal/llm"
	"ai-agent/internal/tools"
	"ai-agent/internal/tools/registry"
	"bufio"
	"fmt"
	"log"
	"os"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("error loading config: %v", err)
	}

	// Create tools
	cryptoTool := tools.NewCryptoTool()
	gitTool := tools.NewGitTool()

	// Create LLM client with config values
	llmClient := llm.NewClientWithOptions(
		cfg.OpenAIApiKey,
		cfg.LLMBaseURL,
		cfg.LLMModel,
		cfg.LLMTemperature,
		cfg.LLMMaxTokens,
	)

	// Create registry
	reg := registry.New(cryptoTool, gitTool)

	// Create agent
	ag := agent.NewAgent(llmClient, reg)

	// Interactive CLI
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("AI Agent ready. Type your request (Ctrl+C to exit):")

	for scanner.Scan() {
		input := scanner.Text()
		if input == "" {
			continue
		}
		log.Println("user input:", input)
		output := ag.Run(input)
		fmt.Printf("Agent: %s\n\n", output)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
}
