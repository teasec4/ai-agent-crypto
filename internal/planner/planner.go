package planner

import (
	"ai-agent/internal/llm"
	"ai-agent/internal/tools/registry"
)

// PlanResult is the output of the planning phase.
type PlanResult struct {
	Action      string
	Parameters  map[string]interface{}
	Reasoning   string
	Observation string // filled by Observe phase
	Done        bool   // true if the goal is reached
}

// Planner decides what action to take next.
type Planner interface {
	// Plan analyses the user input and current state, then returns the next action(s).
	Plan(input string, history []HistoryEntry, toolList string) PlanResult
}

// HistoryEntry stores a single step in the agent loop.
type HistoryEntry struct {
	Query  string
	Plan   PlanResult
	Result string
}

// NewPlanner creates the appropriate planner based on config.
func NewPlanner(llmCLient llm.LlmClient, registry *registry.Registry) Planner {
	return &LLMPlanner{
		llm:      llmCLient,
		registry: registry,
	}
}
