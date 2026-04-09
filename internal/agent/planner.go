package agent

import "strings"

type Planner interface {
	Plan(input string, state *State) Plan
}

type Plan struct {
	Action string
}

type SimplePlanner struct{}

func NewPlanner() Planner {
	return &SimplePlanner{}
}

func (p *SimplePlanner) Plan(input string, state *State) Plan {
	// тут LLM или простая логика
	if strings.Contains(input, "BTC") {
		return Plan{Action: "get_btc_price"}
	}
	return Plan{Action: "unknown"}
}
