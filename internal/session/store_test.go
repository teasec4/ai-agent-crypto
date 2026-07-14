package session

import "testing"

func TestApprovalChannelLifecycle(t *testing.T) {
	state := NewStore().Create()

	first := state.NewApprovalChannel()
	if first == nil {
		t.Fatal("expected first approval channel")
	}
	if second := state.NewApprovalChannel(); second != nil {
		t.Fatal("expected second approval channel to be rejected while first is active")
	}

	state.FinishApprovalChannel(first)
	second := state.NewApprovalChannel()
	if second == nil {
		t.Fatal("expected approval channel after finishing first stream")
	}

	state.FinishApprovalChannel(first)
	if third := state.NewApprovalChannel(); third != nil {
		t.Fatal("old stream must not clear the active channel for a newer stream")
	}

	state.FinishApprovalChannel(second)
	if third := state.NewApprovalChannel(); third == nil {
		t.Fatal("expected approval channel after finishing second stream")
	}
}
