package planner

import (
	"encoding/json"
	"fmt"
	"strings"

	"ai-agent/internal/llm"
	"ai-agent/internal/memory"
	"ai-agent/internal/tools/registry"
)

// LLMPlanner uses an LLM to decide the next action.
type LLMPlanner struct {
	llmClient llm.LlmClient
	registry  *registry.Registry
	workingMemory memory.WorkMemory
}

func NewLLMPlanner(llmClient llm.LlmClient, reg *registry.Registry) *LLMPlanner {
	return &LLMPlanner{
		llmClient: llmClient,
		registry:  reg,
		workingMemory: *memory.NewWorkMemory(),
	}
}

// Plan uses the LLM to determine the next action.
func (p *LLMPlanner) Plan(input string,) PlanResult {
	messages := p.buildMessages(input, p.workingMemory.Messages)

	response, err := p.llmClient.Chat(messages)
	if err != nil {
		fmt.Sprintf("LLM error: %v", err)
		
	}

	// Clean up markdown wrapping
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var planResponse PlanResult
	if err := json.Unmarshal([]byte(response), &planResponse); err != nil {
		fmt.Sprintf("JSON parse error: %v", err)
	}

	// Validate the action
	if !p.registry.IsValid(planResponse.Action) {
		planResponse.Action = "message"
	}

	// if planResponse.Parameters == nil {
	// 	planResponse.Parameters = make(map[string]interface{})
	// }

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


%s

Return ONLY valid JSON (no markdown):
{
  "action": "tool_name",
  "parameters": { "key": "value" },
  
} or if you can answer by yourself return 

{
	"action": "message",
	"reply" : "[your answer]"
}

`, toolList)

	messages := []llm.Message{{Role: "system", Content: systemPrompt}}
	messages = append(messages, history...)
	messages = append(messages, llm.Message{Role: "user", Content: input})
	return messages
}

