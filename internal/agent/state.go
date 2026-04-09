package agent

type State struct {
    LastAction string
    LastResult string
}

func NewState() *State {
    return &State{}
}