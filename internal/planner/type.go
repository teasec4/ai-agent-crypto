package planner

const (
	ActionMessage = "message"
	ActionUnknown = "unknown"
)

// PlanResult is the output of the planning phase.
type PlanResult struct {
	Action     string                 `json:"action"`
	Parameters map[string]interface{} `json:"parameters"`
	Reply      string                 `json:"reply"`
	ToolCallID string                 `json:"tool_call_id,omitempty"` // ID from LLM tool_call for tool role pairing
}

// IsFinished returns true when the plan is a final answer (no tool calls).
func (p PlanResult) IsFinished() bool {
	return p.Reply != "" && p.Action != ActionUnknown
}
