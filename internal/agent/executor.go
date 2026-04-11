package agent

import (
	"ai-agent/internal/llm"
	"ai-agent/internal/tools"
	"fmt"
)

type Executor struct {
	tools map[string]tools.Tool
	llm *llm.Client
}

func NewExecutor(llmClient *llm.Client) Executor {
	return Executor{
		llm: llmClient,
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

	decisionPrompt := fmt.Sprintf(`
		Цена: %s
		Стоит ли покупать? Ответь кратко.
		`, result)

	decision, _ := e.llm.Chat(decisionPrompt)

	return decision
}
