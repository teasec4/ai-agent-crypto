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
	registry  *registry.Registry
}

func NewLLMPlanner(llmClient llm.LlmClient, reg *registry.Registry) *LLMPlanner {
	return &LLMPlanner{
		llmClient: llmClient,
		registry:  reg,
	}
}

// Plan uses the LLM to determine the next action.
func (p *LLMPlanner) Plan(input string, history []llm.Message) PlanResult {
	messages := p.buildMessages(input, history)

	response, err := p.llmClient.Chat(messages)
	if err != nil {
		return p.fallbackPlan(input, fmt.Sprintf("LLM error: %v", err))
	}

	// Clean up markdown wrapping
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var planResponse PlanResult
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

	return planResponse
}

// buildMessages constructs the LLM messages array for planning.
// History messages are passed as-is (not serialised into system prompt)
// to avoid duplicating context and to present the LLM with a clean message chain.
func (p *LLMPlanner) buildMessages(input string, history []llm.Message) []llm.Message {
	toolList := p.registry.List()
	systemPrompt := fmt.Sprintf(`You are a planner for an AI agent. Your job is to analyse the user's request and decide:
1. Which tool/action to use
2. What parameters are needed
3. Whether the goal is reached (done = true)

%s

Return ONLY valid JSON (no markdown):
{
  "action": "tool_name",
  "parameters": { "key": "value" },
  "reasoning": "brief explanation",
  "done": false
}`, toolList)

	messages := []llm.Message{{Role: "system", Content: systemPrompt}}
	messages = append(messages, history...)
	messages = append(messages, llm.Message{Role: "user", Content: input})
	return messages
}

// fallbackPlan is used when the LLM call fails.
func (p *LLMPlanner) fallbackPlan(input string, reason string) PlanResult {
	return PlanResult{
		Action:     "unknown",
		Parameters: map[string]interface{}{"query": input},
		Reasoning:  "Fallback: " + reason,
		Done:       false,
	}
}
