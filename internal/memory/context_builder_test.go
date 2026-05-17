package memory

import (
	"strings"
	"testing"
	"time"
)

func TestContextBuilderBuildsSystemMemory(t *testing.T) {
	store := NewJSONStore(t.TempDir() + "/events.jsonl")
	before := time.Now()
	if err := store.Append(MemoryEvent{
		SessionID: "s1",
		Time:      before.Add(-time.Minute),
		Type:      EventToolResult,
		Action:    "git_context",
		Result:    "branch main is clean",
		Tags:      []string{"git"},
	}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if err := store.Append(MemoryEvent{
		SessionID: "s1",
		Time:      before.Add(time.Minute),
		Type:      EventUserMessage,
		Content:   "current request should be filtered",
	}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	builder := NewContextBuilder(store)
	messages, err := builder.Build(ContextRequest{
		SessionID: "s1",
		Input:     "what is git status?",
		Before:    before,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("Build() len = %d, want 1", len(messages))
	}
	if messages[0].Role != RoleSystem {
		t.Fatalf("Build()[0].Role = %q, want %q", messages[0].Role, RoleSystem)
	}
	if !strings.Contains(messages[0].Content, "branch main is clean") {
		t.Fatalf("memory context = %q, want previous git result", messages[0].Content)
	}
	if strings.Contains(messages[0].Content, "current request should be filtered") {
		t.Fatalf("memory context included current-run event: %q", messages[0].Content)
	}
}
