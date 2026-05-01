package planner

// Planner decides what action to take next.
type Planner interface {
	// Plan analyses the user input and current state, then returns the next action(s).
	Plan(input string, history []HistoryEntry, toolList string) PlanResult
}