package agent

type Agent struct{
	planner  Planner
    executor Executor
    state    *State
}

func NewAgent()*Agent{
	return &Agent{
		planner:  NewPlanner(),
        executor: NewExecutor(),
        state:    NewState(),
	}
}

func (a *Agent) Run(input string) string {
    plan := a.planner.Plan(input, a.state)
    result := a.executor.Execute(plan, a.state)
    return result
}