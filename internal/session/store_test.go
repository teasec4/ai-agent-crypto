package session

import (
	"testing"
	"time"
)

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

func TestRunLifecycleAllowsOnlyOneActiveRun(t *testing.T) {
	state := NewStore().Create()

	if !state.TryStartRun() {
		t.Fatal("expected first run to start")
	}
	if state.TryStartRun() {
		t.Fatal("expected second run to be rejected while first is active")
	}

	state.FinishRun()
	if !state.TryStartRun() {
		t.Fatal("expected run to start after previous run finished")
	}
}

func TestCleanupExpiredDisabledWhenTTLIsZero(t *testing.T) {
	store := NewStore()
	state := store.Create()
	state.UpdatedAt = time.Now().Add(-24 * time.Hour)

	if removed := store.CleanupExpired(0); removed != 0 {
		t.Fatalf("expected cleanup to be disabled with ttl=0, removed %d", removed)
	}
	if _, ok := store.Get(state.ID); !ok {
		t.Fatal("expected old session to remain when ttl=0")
	}
}

func TestCleanupExpiredUsesProvidedTTL(t *testing.T) {
	store := NewStore()
	state := store.Create()
	state.UpdatedAt = time.Now().Add(-2 * time.Hour)

	if removed := store.CleanupExpired(time.Hour); removed != 1 {
		t.Fatalf("expected one expired session to be removed, removed %d", removed)
	}
	if _, ok := store.Get(state.ID); ok {
		t.Fatal("expected expired session to be removed")
	}
}
