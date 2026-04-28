package agent

import (
	"ai-agent/internal/planner"
)

// State holds the current state of the agent loop.
// State is a value object – it should not be mutated in place,
// but replaced with a new copy.
type State struct {
	LastAction string
	LastResult string
	LastQuery  string
	History    []planner.HistoryEntry
}

// NewState creates a new empty state.
func NewState() *State {
	return &State{
		History: make([]planner.HistoryEntry, 0),
	}
}

// WithResult returns a new State with the given step appended to history.
// The original State is not mutated.
func (s *State) WithResult(query, action, result string) *State {
	entry := planner.HistoryEntry{
		Query: query,
		Plan: planner.PlanResult{
			Action: action,
		},
		Result: result,
	}

	// Copy history and append
	newHistory := make([]planner.HistoryEntry, len(s.History)+1)
	copy(newHistory, s.History)
	newHistory[len(s.History)] = entry

	return &State{
		LastAction: action,
		LastResult: result,
		LastQuery:  query,
		History:    newHistory,
	}
}
