package agent

type State struct {
	LastAction string
	LastResult string
	LastQuery  string
	History    []HistoryEntry
}

type HistoryEntry struct {
	Query  string
	Action string
	Result string
	Time   string
}

func NewState() *State {
	return &State{
		History: make([]HistoryEntry, 0),
	}
}
