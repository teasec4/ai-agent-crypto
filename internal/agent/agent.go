package agent

import (
	"ai-agent/internal/config"
	"ai-agent/internal/llm"
)

type Agent struct {
	planner  Planner
	executor Executor
	state    *State
}

func NewAgent(cfg *config.Config) *Agent {
	llmClient := llm.NewClient(cfg.OpenAIApiKey)
	return &Agent{
		planner:  NewPlanner(),
		executor: NewExecutor(cfg, llmClient),
		state:    NewState(),
	}
}

func (a *Agent) Run(input string) string {
	plan := a.planner.Plan(input, a.state)
	result := a.executor.Execute(plan, a.state)
	return result
}
