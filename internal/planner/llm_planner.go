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
func (p *LLMPlanner) Plan(history []llm.Message) (PlanResult, error) {
	messages := p.buildMessages(history)

	response, err := p.llmClient.Chat(messages)
	if err != nil {
		return PlanResult{}, fmt.Errorf("llm planning failed: %w", err)
	}

	response = cleanJSONResponse(response)

	var planResponse PlanResult
	if err := json.Unmarshal([]byte(response), &planResponse); err != nil {
		return PlanResult{}, fmt.Errorf("failed to parse planner JSON %q: %w", response, err)
	}

	if planResponse.Action == "" {
		return PlanResult{}, fmt.Errorf("planner returned empty action")
	}

	if planResponse.Action != ActionMessage &&
		planResponse.Action != ActionUnknown &&
		!p.registry.IsValid(planResponse.Action) {
		return PlanResult{}, fmt.Errorf("planner returned unknown action %q", planResponse.Action)
	}

	if planResponse.Action == ActionMessage && strings.TrimSpace(planResponse.Reply) == "" {
		return PlanResult{}, fmt.Errorf("planner returned message action without reply")
	}

	if planResponse.Parameters == nil {
		planResponse.Parameters = make(map[string]interface{})
	}

	return planResponse, nil
}

func cleanJSONResponse(response string) string {
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	return strings.TrimSpace(response)
}

// buildMessages constructs the LLM messages array for planning.
func (p *LLMPlanner) buildMessages(history []llm.Message) []llm.Message {
	toolList := p.registry.List()
	systemPrompt := fmt.Sprintf(`You are a planner for an AI agent. Your job is to analyze the user's request and decide the next action.

Available actions:
- "message": answer directly without a tool
- "unknown": use only when no available tool or direct answer fits the request
- any registered tool listed below

%s

Return ONLY valid JSON, no markdown.

For a tool call:
{
  "action": "tool_name",
  "parameters": { "key": "value" }
}

For a direct answer:
{
  "action": "message",
  "reply": "your answer"
}

For an unsupported or unclear request:
{
  "action": "unknown",
  "parameters": { "reason": "why no available action fits" }
}
`, toolList)

	messages := []llm.Message{{Role: "system", Content: systemPrompt}}
	messages = append(messages, history...)
	return messages
}
