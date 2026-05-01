package planner

// PlanResult is the output of the planning phase.
type PlanResult struct {
	Action      string
	Parameters  map[string]interface{}
	Reasoning   string
	Observation string // filled by Observe phase
	Done        bool   // true if the goal is reached
}

// HistoryEntry stores a single step in the agent loop.
type HistoryEntry struct {
	Query  string
	Plan   PlanResult
	Result string
}