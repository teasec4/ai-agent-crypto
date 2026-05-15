package main

import (
	"fmt"
	"log"

	"ai-agent/internal/agent"
	"ai-agent/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("error loading config: %v", err)
	}

	ag := agent.NewWithConfig(cfg)

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
