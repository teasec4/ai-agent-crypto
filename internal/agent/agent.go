package agent

import "ai-agent/internal/llm"

type Agent struct{
	planner  Planner
    executor Executor
    state    *State
}

func NewAgent(apiKey string)*Agent{
	llmClient := llm.NewClient(apiKey)
	return &Agent{
		planner:  NewPlanner(llmClient),
        executor: NewExecutor(llmClient),
        state:    NewState(),
	}
}

func (a *Agent) Run(input string) string {
    plan := a.planner.Plan(input, a.state)
    result := a.executor.Execute(plan, a.state)
    return result
}