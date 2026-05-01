package executor

import "ai-agent/internal/planner"

// Executor executes a plan using registered tools.
type Executor interface {
	// Execute runs the given plan and returns the result.
	Execute(plan planner.PlanResult) (string, error)
}