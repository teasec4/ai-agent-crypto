package executor

import (
	"fmt"

	"ai-agent/internal/planner"
	"ai-agent/internal/tools/registry"
)

// Executor executes a plan using registered tools.
type Executor interface {
	// Execute runs the given plan and returns the result.
	Execute(plan planner.PlanResult) (string, error)
}

// ToolExecutor is the default implementation of Executor.
type ToolExecutor struct {
	registry *registry.Registry
}

// New creates a new ToolExecutor with the given tool registry.
func New(reg *registry.Registry) Executor {
	return &ToolExecutor{
		registry: reg,
	}
}

// Execute runs the plan's action using the appropriate tool.
func (e *ToolExecutor) Execute(plan planner.PlanResult) (string, error) {
	tool := e.registry.Get(plan.Action)
	if tool == nil {
		// Fallback to unknown tool
		tool = e.registry.Get("unknown")
		if tool == nil {
			return "", fmt.Errorf("no tool found for action '%s' and no 'unknown' fallback registered", plan.Action)
		}
	}

	result, err := tool.Run(plan.Parameters)
	if err != nil {
		return "", fmt.Errorf("tool '%s' returned error: %w", plan.Action, err)
	}

	return result, nil
}



