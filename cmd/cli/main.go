package main

import (
	"ai-agent/internal/agent"
	"ai-agent/internal/config"
	"bufio"
	"fmt"
	"log"
	"os"
)

func main(){
	config, err := config.Load()
	if err != nil {
		log.Fatalf("error - %v", err)
	}
	
	agent := agent.NewAgent(config)
	
	scanner := bufio.NewScanner(os.Stdin)
	
	for scanner.Scan(){
		input := scanner.Text()
		log.Println("user input: " + input)
		output := agent.Run(input)
		fmt.Printf("Llm output: %s\n", output)
	}
	
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
	
	
	
}