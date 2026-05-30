package agent

import (
	"ai-agent/internal/config"
	"ai-agent/internal/harness"
)

type Agent struct {
	harness *harness.Harness
}

func NewAgent(cfg *config.Config) *Agent {
	return NewWithConfig(cfg)
}

func NewWithConfig(cfg *config.Config) *Agent {
	return &Agent{
		harness: harness.New(cfg),
	}
}

func (a *Agent) Run(input string) string {
	result := a.harness.Run(input)
	return result.LoopResult.Answer
}
