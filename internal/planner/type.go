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
}
