package agent

import "ai-agent/internal/tools"

type Executor struct {
	tools map[string]tools.Tool
}

func NewExecutor() Executor {
	return Executor{
		tools: map[string]tools.Tool{
			"get_btc_price": tools.NewBTCTool(),
		},
	}
}

func (e Executor) Execute(plan Plan, state *State) string {
	tool, ok := e.tools[plan.Action]
	if !ok {
		return "неизвестное действие"
	}

	result, _ := tool.Run()

	state.LastAction = plan.Action
	state.LastResult = result

	return result
}
