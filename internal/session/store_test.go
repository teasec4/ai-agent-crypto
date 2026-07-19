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

func TestWriterHandoffLifecycle(t *testing.T) {
	state := NewStore().Create()

	first := state.ConnectClient("client-a")
	if first.Role != RoleWriter {
		t.Fatalf("expected first client to be writer, got %q", first.Role)
	}

	second := state.ConnectClient("client-b")
	if second.Role != RoleViewer {
		t.Fatalf("expected second client to be viewer, got %q", second.Role)
	}
	if second.WriterClientID != "client-a" {
		t.Fatalf("expected client-a writer, got %q", second.WriterClientID)
	}

	requested, err := state.RequestWriter("client-b")
	if err != nil {
		t.Fatalf("request writer: %v", err)
	}
	if requested.PendingWriterRequest == nil {
		t.Fatal("expected pending writer request")
	}
	repeated, err := state.RequestWriter("client-b")
	if err != nil {
		t.Fatalf("repeat request writer should be idempotent: %v", err)
	}
	if repeated.PendingWriterRequest == nil || repeated.PendingWriterRequest.ID != requested.PendingWriterRequest.ID {
		t.Fatal("expected repeated request to return the same pending writer request")
	}

	approved, err := state.ApproveWriterRequest("client-a", requested.PendingWriterRequest.ID)
	if err != nil {
		t.Fatalf("approve writer request: %v", err)
	}
	if approved.WriterClientID != "client-b" {
		t.Fatalf("expected client-b writer after approve, got %q", approved.WriterClientID)
	}
	if !state.IsWriter("client-b") {
		t.Fatal("expected client-b to be writer")
	}
	if state.IsWriter("client-a") {
		t.Fatal("expected client-a to lose writer role")
	}
}

func TestRejectWriterHandoffKeepsCurrentWriter(t *testing.T) {
	state := NewStore().Create()
	state.ConnectClient("client-a")
	state.ConnectClient("client-b")

	requested, err := state.RequestWriter("client-b")
	if err != nil {
		t.Fatalf("request writer: %v", err)
	}
	if _, err := state.RejectWriterRequest("client-a", requested.PendingWriterRequest.ID); err != nil {
		t.Fatalf("reject writer request: %v", err)
	}
	if !state.IsWriter("client-a") {
		t.Fatal("expected client-a to remain writer")
	}
	if access := state.ClientAccess("client-b"); access.PendingWriterRequest != nil {
		t.Fatal("expected pending request to be cleared")
	}
}

func TestWriterIsReleasedWhenEventsSubscriptionCloses(t *testing.T) {
	state := NewStore().Create()

	first := state.ConnectClient("client-a")
	if first.Role != RoleWriter {
		t.Fatalf("expected first client to be writer, got %q", first.Role)
	}
	_, unsubscribe := state.Subscribe("client-a")
	unsubscribe()

	second := state.ConnectClient("client-b")
	if second.Role != RoleWriter {
		t.Fatalf("expected next active client to become writer, got %q", second.Role)
	}
	if second.WriterClientID != "client-b" {
		t.Fatalf("expected client-b writer, got %q", second.WriterClientID)
	}
}

func TestWriterTransfersToActiveViewerWhenSubscriptionCloses(t *testing.T) {
	state := NewStore().Create()

	state.ConnectClient("client-a")
	state.ConnectClient("client-b")
	_, unsubscribeA := state.Subscribe("client-a")
	state.Subscribe("client-b")

	access, changed := unsubscribeA()
	if !changed {
		t.Fatal("expected writer change when active writer disconnects")
	}
	if access.WriterClientID != "client-b" {
		t.Fatalf("expected client-b writer after disconnect, got %q", access.WriterClientID)
	}
	if !state.IsWriter("client-b") {
		t.Fatal("expected active viewer to become writer")
	}
}

func TestWriterTransfersToPendingRequesterWhenSubscriptionCloses(t *testing.T) {
	state := NewStore().Create()

	state.ConnectClient("client-a")
	state.ConnectClient("client-b")
	_, unsubscribeA := state.Subscribe("client-a")
	state.Subscribe("client-b")
	if _, err := state.RequestWriter("client-b"); err != nil {
		t.Fatalf("request writer: %v", err)
	}

	access, changed := unsubscribeA()
	if !changed {
		t.Fatal("expected writer change when active writer disconnects")
	}
	if access.WriterClientID != "client-b" {
		t.Fatalf("expected pending requester to become writer, got %q", access.WriterClientID)
	}
	if pending := state.ClientAccess("client-b").PendingWriterRequest; pending != nil {
		t.Fatal("expected pending request to be cleared after automatic transfer")
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
