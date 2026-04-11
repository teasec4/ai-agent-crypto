package tools

import (
	"fmt"
	"strings"
	"time"
)

// GreetingTool handles greeting interactions
type GreetingTool struct{}

// NewGreetingTool creates a new GreetingTool instance
func NewGreetingTool() Tool {
	return &GreetingTool{}
}

// Run executes the greeting
func (t *GreetingTool) Run(params map[string]interface{}) (string, error) {
	name := ""
	if nameParam, ok := params["name"].(string); ok && nameParam != "" {
		name = strings.TrimSpace(nameParam)
	}

	// Get current time for appropriate greeting
	hour := time.Now().Hour()
	var timeGreeting string
	switch {
	case hour < 6:
		timeGreeting = "Доброй ночи"
	case hour < 12:
		timeGreeting = "Доброе утро"
	case hour < 18:
		timeGreeting = "Добрый день"
	default:
		timeGreeting = "Добрый вечер"
	}

	// Construct greeting message
	var greeting string
	if name != "" {
		greeting = fmt.Sprintf("%s, %s! Я ваш AI-ассистент. Чем могу помочь?", timeGreeting, name)
	} else {
		greeting = fmt.Sprintf("%s! Я ваш AI-ассистент. Чем могу помочь?", timeGreeting)
	}

	return greeting, nil
}
