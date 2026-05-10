package agent

import (
	"log"

	"ai-agent/internal/executor"
	"ai-agent/internal/llm"
	"ai-agent/internal/planner"
	"ai-agent/internal/tools/registry"
)

type Agent struct {
	llmClient llm.LlmClient
	registry  *registry.Registry
	planner   *planner.LLMPlanner
}

func NewAgent(
	llmClient llm.LlmClient,
	reg *registry.Registry,
) *Agent {
	return &Agent{
		llmClient: llmClient,
		registry:  reg,
		planner:   planner.NewLLMPlanner(llmClient, reg),
	}
}

func (a *Agent) Run(input string) string {
	exctr := executor.New(a.registry)

	log.Printf("[Agent] 🤔 Planning...")
	planResult := a.planner.Plan(input)

	if planResult.Action == "message" {
		return planResult.Reply // ответила сама LLM
	}

	log.Printf("[Agent] 🛠️  Executing tool: %s", planResult.Action)
	result, err := exctr.Execute(planResult)
	if err != nil {
		log.Printf("[Agent] ❌ Tool error: %v", err)
		return ""
	}

	log.Printf("[Agent] 📥 Tool result: %s", truncate(result, 200))
	return result
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
