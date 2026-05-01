package planner

import (
	"encoding/json"
	"fmt"
	"strings"

	"ai-agent/internal/llm"
	"ai-agent/internal/tools/registry"
)

// LLMPlanner uses an LLM to decide the next action.
type LLMPlanner struct {
	llmClient llm.LlmClient
	registry *registry.Registry
}

func NewLLMPlanner(llmClient llm.LlmClient, reg *registry.Registry) *LLMPlanner {
	return &LLMPlanner{
		llmClient:      llmClient,
		registry: reg,
	}
}

// Plan uses the LLM to determine the next action.
func (p *LLMPlanner) Plan(input string, history []HistoryEntry, toolList string) PlanResult {
	// Build the prompt
	prompt := p.buildPrompt(input, history, toolList)

	response, err := p.llmClient.Chat(prompt)
	if err != nil {
		return p.fallbackPlan(input, fmt.Sprintf("LLM error: %v", err))
	}

	// Clean up markdown wrapping
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var planResponse struct {
		Action     string                 `json:"action"`
		Parameters map[string]interface{} `json:"parameters"`
		Reasoning  string                 `json:"reasoning"`
		Done       bool                   `json:"done"`
	}

	if err := json.Unmarshal([]byte(response), &planResponse); err != nil {
		return p.fallbackPlan(input, fmt.Sprintf("JSON parse error: %v", err))
	}

	// Validate the action
	if !p.registry.IsValid(planResponse.Action) {
		planResponse.Action = "unknown"
		planResponse.Reasoning = "Action not recognised, falling back to unknown"
	}

	if planResponse.Parameters == nil {
		planResponse.Parameters = make(map[string]interface{})
	}

	return PlanResult{
		Action:     planResponse.Action,
		Parameters: planResponse.Parameters,
		Reasoning:  planResponse.Reasoning,
		Done:       planResponse.Done,
	}
}

// buildPrompt constructs the LLM prompt for planning.
func (p *LLMPlanner) buildPrompt(input string, history []HistoryEntry, toolList string) string {
	historyStr := p.formatHistory(history)

	return fmt.Sprintf(`
You are a planner for an AI agent. Your job is to analyse the user's request and decide:
1. Which tool/action to use
2. What parameters are needed
3. Whether the goal is reached (done = true)

%s

Previous steps:
%s

User request: "%s"

Return ONLY valid JSON (no markdown):
{
  "action": "tool_name",
  "parameters": { "key": "value" },
  "reasoning": "brief explanation",
  "done": false
}
`, toolList, historyStr, input)
}

// fallbackPlan is used when the LLM call fails.
func (p *LLMPlanner) fallbackPlan(input string, reason string) PlanResult {
	inputLower := strings.ToLower(input)

	if strings.Contains(inputLower, "price") || strings.Contains(inputLower, "цена") || strings.Contains(inputLower, "курс") {
		crypto := detectCryptocurrency(inputLower)
		return PlanResult{
			Action:     "get_crypto_price",
			Parameters: map[string]interface{}{"query": crypto, "currency": "usd"},
			Reasoning:  "Fallback: detected a price query. " + reason,
			Done:       false,
		}
	}

	return PlanResult{
		Action:     "unknown",
		Parameters: map[string]interface{}{"query": input},
		Reasoning:  "Fallback: could not determine action. " + reason,
		Done:       false,
	}
}

// formatHistory formats the history entries for the prompt.
func (p *LLMPlanner) formatHistory(history []HistoryEntry) string {
	if len(history) == 0 {
		return "No previous steps (this is the first request)."
	}

	var sb strings.Builder
	// Show only the last 3 entries
	start := len(history) - 3
	if start < 0 {
		start = 0
	}
	for i := start; i < len(history); i++ {
		entry := history[i]
		sb.WriteString(fmt.Sprintf("- Request: %s → Action: %s → Result: %s\n",
			entry.Query, entry.Plan.Action, truncate(entry.Result, 100)))
	}
	return sb.String()
}

// truncate truncates a string to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// detectCryptocurrency tries to detect a cryptocurrency name from the input.
func detectCryptocurrency(input string) string {
	known := map[string]string{
		"bitcoin":  "bitcoin",
		"биткоин":  "bitcoin",
		"btc":      "bitcoin",
		"ethereum": "ethereum",
		"эфир":     "ethereum",
		"eth":      "ethereum",
		"solan":    "solana",
		"солана":   "solana",
		"sol":      "solana",
		"cardano":  "cardano",
		"ada":      "cardano",
	}

	for substr, id := range known {
		if strings.Contains(input, substr) {
			return id
		}
	}
	return "bitcoin"
}
