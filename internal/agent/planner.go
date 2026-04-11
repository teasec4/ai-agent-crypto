package agent

import (
	"ai-agent/internal/llm"
	"strings"
)

type Planner interface {
	Plan(input string, state *State) Plan
}

type Plan struct {
	Action string
}

type SimplePlanner struct{
	llm *llm.Client
}

func NewPlanner(llmClient *llm.Client) Planner {
	return &SimplePlanner{llm: llmClient}
}

func (p *SimplePlanner) Plan(input string, state *State) Plan {
	prompt := `
		Ты AI агент. Определи действие.
		Возможные действия:
		- get_btc_price
		- get_eth_price
		
		Ответь ТОЛЬКО названием действия.
		
		Запрос: ` + input

	resp, _ := p.llm.Chat(prompt)

	return Plan{
		Action: strings.TrimSpace(resp),
	}
}
