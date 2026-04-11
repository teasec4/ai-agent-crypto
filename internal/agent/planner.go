package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"ai-agent/internal/llm"
)

type Planner interface {
	Plan(input string, state *State) Plan
}

type LLMPlanner struct {
	llm *llm.Client
}

func NewPlanner(llmClient *llm.Client) Planner {
	return &LLMPlanner{llm: llmClient}
}

func (p *LLMPlanner) Plan(input string, state *State) Plan {
	// Create a prompt for the LLM to analyze the user's intent
	prompt := fmt.Sprintf(`
Ты - планировщик для AI агента. Проанализируй запрос пользователя и определи:
1. Какое действие нужно выполнить
2. Какие параметры нужны для этого действия
3. Какой инструмент использовать

Доступные инструменты:
1. get_crypto_price - получить цену криптовалюты (параметры: query - название криптовалюты, currency - валюта)
2. greeting - приветствие (параметры: name - имя пользователя, если известно)
3. unknown - для неизвестных запросов

История предыдущих действий: %s

Запрос пользователя: "%s"

Верни ответ в формате JSON:
{
  "action": "название_действия",
  "parameters": {
    "param1": "value1",
    "param2": "value2"
  },
  "reasoning": "краткое объяснение выбора"
}

Примеры:
1. Запрос: "Какая цена биткоина?"
   Ответ: {"action": "get_crypto_price", "parameters": {"query": "bitcoin", "currency": "usd"}, "reasoning": "Пользователь спрашивает цену биткоина"}

2. Запрос: "Привет, меня зовут Иван"
   Ответ: {"action": "greeting", "parameters": {"name": "Иван"}, "reasoning": "Пользователь представляется"}

3. Запрос: "Что такое блокчейн?"
   Ответ: {"action": "unknown", "parameters": {"query": "Что такое блокчейн?"}, "reasoning": "Запрос не относится к доступным инструментам"}
`, formatHistory(state.History), input)

	// Get response from LLM
	fmt.Println(">>> Starting LLM call")
	response, err := p.llm.Chat(prompt)
	fmt.Println(">>> LLM call done, err:", err)
	if err != nil {
		// Fallback to simple planner if LLM fails
		fmt.Println(">>> LLM failed, using fallback")
		return p.fallbackPlan(input)
	}
	// markdown обертка
	response = strings.Trim(response, "`")
	response = strings.Replace(response, "```json", "", -1)
	response = strings.Replace(response, "```", "", -1)

	fmt.Fprintf(os.Stdout, "LLM Response: %s\n", response)

	// Parse JSON response
	var planResponse struct {
		Action     string                 `json:"action"`
		Parameters map[string]interface{} `json:"parameters"`
		Reasoning  string                 `json:"reasoning"`
	}

	if err := json.Unmarshal([]byte(response), &planResponse); err != nil {
		// If JSON parsing fails, use fallback
		return p.fallbackPlan(input)
	}

	// Validate action
	validActions := map[string]bool{
		"get_crypto_price": true,
		"greeting":         true,
		"unknown":          true,
	}

	if !validActions[planResponse.Action] {
		planResponse.Action = "unknown"
	}

	return Plan{
		Action:     planResponse.Action,
		Parameters: planResponse.Parameters,
		Input:      input,
	}
}

// Fallback planner for when LLM fails
func (p *LLMPlanner) fallbackPlan(input string) Plan {
	lower := strings.ToLower(input)

	if strings.Contains(lower, "биткоин") {
		return Plan{Action: "get_crypto_price", Parameters: map[string]interface{}{"query": "bitcoin"}, Input: input}
	}

	if strings.Contains(lower, "эфир") || strings.Contains(lower, "ethereum") {
		return Plan{Action: "get_crypto_price", Parameters: map[string]interface{}{"query": "ethereum"}, Input: input}
	}

	// Default to unknown
	return Plan{
		Action: "unknown",
		Parameters: map[string]interface{}{
			"query": input,
		},
		Input: input,
	}
}

// Helper function to format history for the prompt
func formatHistory(history []HistoryEntry) string {
	if len(history) == 0 {
		return "Нет истории"
	}

	var historyStr strings.Builder
	for i, entry := range history {
		if i > 2 { // Show only last 3 entries
			break
		}
		historyStr.WriteString(fmt.Sprintf("- Запрос: %s, Действие: %s\n", entry.Query, entry.Action))
	}
	return historyStr.String()
}
