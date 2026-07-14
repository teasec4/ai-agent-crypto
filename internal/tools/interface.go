package tools

import (
	"context"

	"ai-agent/internal/approval"
)

// Parameter describes a single tool parameter for LLM tool calling.
type Parameter struct {
	Type        string     `json:"type"`
	Description string     `json:"description"`
	Items       *Parameter `json:"items,omitempty"` // for array type
}

// ToolSchema describes the parameters schema for a tool.
type ToolSchema struct {
	Type       string               `json:"type"`
	Properties map[string]Parameter `json:"properties"`
	Required   []string             `json:"required,omitempty"`
}

// Tool is the interface that all tools must implement.
type Tool interface {
	Name() string
	Description() string
	Schema() ToolSchema
	// Run executes the tool. workspace is the sandbox root (empty = cwd).
	// ctx carries deadline/cancellation from the agent loop.
	Run(ctx context.Context, workspace string, params map[string]interface{}) (string, error)
}

// ApprovalAwareTool can request user approval before execution.
// Preview/Summary/Risk are called with params only (no workspace needed).
type ApprovalAwareTool interface {
	RequiresApproval(params map[string]interface{}) bool
	Risk(params map[string]interface{}) approval.RiskLevel
	Preview(params map[string]interface{}) (string, error)
	Summary(params map[string]interface{}) string
}
