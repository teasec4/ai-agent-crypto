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
	helpTool := tools.NewHelpTool()
	unknownTool := tools.NewUnknownTool()
	
	// Create LLM client
	const baseUrlDeepSeek string = "https://api.deepseek.com/v1/chat/completions"
	const modelDeepSeek string = "deepseek-chat"
	llmClient := llm.NewClient(cfg.OpenAIApiKey, baseUrlDeepSeek, modelDeepSeek)

	// Create registry
	reg := registry.New(cryptoTool, gitTool, helpTool, unknownTool)

	// Create agent with max 5 iterations per request
	agent := agent.NewAgent(llmClient, reg, 5)

	// Interactive CLI
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("AI Agent ready. Type your request (Ctrl+C to exit):")

	for scanner.Scan() {
		input := scanner.Text()
		if input == "" {
			continue
		}
		log.Println("user input:", input)
		output := agent.Run(input)
		fmt.Printf("Agent: %s\n\n", output)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
}
