package main

import (
	"ai-agent/internal/agent"
	"ai-agent/internal/config"
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

	ag := agent.NewWithConfig(cfg)

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
