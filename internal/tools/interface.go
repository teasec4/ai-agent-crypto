package tools

import "ai-agent/internal/approval"

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
	// Name returns the unique name of the tool (e.g. "read_file").
	Name() string

	// Description returns a human-readable description.
	Description() string

	// Schema returns the JSON schema for the tool's parameters.
	Schema() ToolSchema

	// Run executes the tool with the given parameters and returns the result.
	Run(params map[string]interface{}) (string, error)
}

// ApprovalAwareTool can request user approval before execution.
type ApprovalAwareTool interface {
	RequiresApproval(params map[string]interface{}) bool
	Risk(params map[string]interface{}) approval.RiskLevel
	Preview(params map[string]interface{}) (string, error)
	Summary(params map[string]interface{}) string
}

// WorkspaceTool accepts a configurable workspace root path.
type WorkspaceTool interface {
	SetRoot(path string)
}
