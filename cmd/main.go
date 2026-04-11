package main

import (
	"fmt"
	"log"

	"ai-agent/internal/agent"
	"ai-agent/internal/config"
)

func main() {
	config, err := config.Load()
	if err != nil {
		log.Fatalf("error - %v", err)
	}

	agent := agent.NewAgent(config)

	// Test multiple queries
	queries := []string{
		// "Какая цена Ethereum?",
		// "Какой курс биткоина?",
		"А сколько стоит Солана?",
	}

	for _, query := range queries {
		fmt.Printf("\nЗапрос: %s\n", query)
		result := agent.Run(query)
		fmt.Printf("Ответ: %s\n", result)
	}
}
