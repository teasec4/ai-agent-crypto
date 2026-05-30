package main

import (
	"ai-agent/internal/config"
	"ai-agent/internal/harness"
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("error loading config: %v", err)
	}
	h := harness.New(cfg)
	session := h.NewSession()

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("AI Agent ready. Type your request (/reset to clear context, Ctrl+C to exit):")

	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "/reset" {
			session.Reset()
			fmt.Print("Agent: context reset.\n\n")
			continue
		}

		log.Println("user input:", input)
		result := session.Run(input)
		fmt.Printf("Agent: %s\n\n", result.LoopResult.Answer)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
}
