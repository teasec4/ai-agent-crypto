package memory

import (
	"strings"
	"testing"
	"time"
)

func TestLongTermMemoryAppendAndBuildContext(t *testing.T) {
	mem := NewLongTermMemory(NewJSONStore(t.TempDir() + "/events.jsonl"))
	before := time.Now()

	if err := mem.Append(MemoryEvent{
		SessionID: "s1",
		Time:      before.Add(-time.Minute),
		Type:      EventToolResult,
		Action:    "git_context",
		Result:    "working tree clean",
		Tags:      []string{"git"},
	}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	messages, err := mem.BuildContext(ContextRequest{
		SessionID: "s1",
		Input:     "git status",
		Before:    before,
	})
	if err != nil {
		t.Fatalf("BuildContext() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("BuildContext() len = %d, want 1", len(messages))
	}
	if !strings.Contains(messages[0].Content, "working tree clean") {
		t.Fatalf("BuildContext() = %q, want stored memory", messages[0].Content)
	}
}
