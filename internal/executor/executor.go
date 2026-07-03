package executor

import (
	"fmt"
	"log/slog"
	"time"

	"ai-agent/internal/approval"
	"ai-agent/internal/planner"
	"ai-agent/internal/tools"
	"ai-agent/internal/tools/registry"
)

// ToolExecutor is the default implementation of Executor.
type ToolExecutor struct {
	registry *registry.Registry
	logger   *slog.Logger
}

// New creates a new ToolExecutor with the given tool registry.
func New(reg *registry.Registry) *ToolExecutor {
	return &ToolExecutor{
		registry: reg,
		logger:   slog.Default(),
	}
}

func (e *ToolExecutor) SetLogger(logger *slog.Logger) {
	e.logger = logger
}

// ExecuteWithWorkspace runs the plan's action, configuring the tool's workspace if set.
func (e *ToolExecutor) ExecuteWithWorkspace(plan planner.PlanResult, workspace string) (string, error) {
	tool := e.registry.Get(plan.Action)
	if tool == nil {
		e.logger.Error("tool not found in registry", "action", plan.Action)
		return "", fmt.Errorf("no tool found for action %q", plan.Action)
	}

	if workspace != "" {
		if wt, ok := tool.(tools.WorkspaceTool); ok {
			wt.SetRoot(workspace)
		}
	}

	start := time.Now()
	result, err := tool.Run(plan.Parameters)
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		e.logger.Warn("tool run error",
			"tool", plan.Action,
			"elapsed_ms", elapsed,
			"error", err.Error(),
		)
		return result, fmt.Errorf("tool '%s' returned error: %w", plan.Action, err)
	}

	e.logger.Debug("tool run ok",
		"tool", plan.Action,
		"elapsed_ms", elapsed,
		"result_bytes", len(result),
	)
	return result, nil
}

func (e *ToolExecutor) RequiresApproval(plan planner.PlanResult) bool {
	tool := e.registry.Get(plan.Action)
	aware, ok := tool.(tools.ApprovalAwareTool)
	return ok && aware.RequiresApproval(plan.Parameters)
}

func (e *ToolExecutor) PendingAction(id string, plan planner.PlanResult) (*approval.PendingAction, error) {
	tool := e.registry.Get(plan.Action)
	if tool == nil {
		e.logger.Error("pending action: tool not found", "action", plan.Action)
		return nil, fmt.Errorf("no tool found for action %q", plan.Action)
	}

	aware, ok := tool.(tools.ApprovalAwareTool)
	if !ok || !aware.RequiresApproval(plan.Parameters) {
		return nil, fmt.Errorf("tool %q does not require approval", plan.Action)
	}

	preview, err := aware.Preview(plan.Parameters)
	if err != nil {
		e.logger.Error("pending action: preview failed",
			"tool", plan.Action,
			"error", err.Error(),
		)
		return nil, fmt.Errorf("failed to build preview for %q: %w", plan.Action, err)
	}

	return &approval.PendingAction{
		ID:        id,
		Tool:      plan.Action,
		Risk:      aware.Risk(plan.Parameters),
		Summary:   aware.Summary(plan.Parameters),
		Preview:   preview,
		Args:      map[string]any(plan.Parameters),
		CreatedAt: time.Now(),
	}, nil
}
