package planner

// PlanResult is the output of the planning phase.
type PlanResult struct {
	Action     string                 `json:"action"`
	Parameters map[string]interface{} `json:"parameters"`
	Reasoning  string                 `json:"reasoning"`
	Done       bool                   `json:"done"`
}
