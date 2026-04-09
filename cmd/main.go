package main

import (
	"fmt"

	"ai-agent/internal/agent"
)

func main() {
	agent := agent.NewAgent()

	result := agent.Run("Стоит ли покупать BTC?")
	fmt.Println(result)
}
