package main

import (
	"fmt"
	"log"

	"ai-agent/internal/agent"
	"ai-agent/internal/config"
	"ai-agent/internal/store"
)

func main() {
	config, err := config.Load()
	if err != nil{
		log.Fatalf("error - %v", err)
	}
	
	profile := store.NewProfile("Max", 1000)
	fmt.Printf("Profile name %s, balance %v $\n", profile.Name, profile.Balance)
	
	agent := agent.NewAgent(config.APIKey)

	result := agent.Run("Стоит ли покупать BTC?")
	fmt.Println(result)
}
