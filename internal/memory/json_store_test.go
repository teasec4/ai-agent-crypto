package memory

import (
	"path/filepath"
	"testing"
	"time"
)

func TestJSONStoreAppendRecentAndByTag(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.json")
	store := NewJSONStore(path)

	events := []MemoryEvent{
		{SessionID: "s1", Time: time.Now().Add(-3 * time.Minute), Type: EventUserMessage, Content: "hello", Tags: []string{"chat"}},
		{SessionID: "s1", Time: time.Now().Add(-2 * time.Minute), Type: EventToolResult, Action: "git_context", Result: "clean", Tags: []string{"git"}},
		{SessionID: "s2", Time: time.Now().Add(-1 * time.Minute), Type: EventUserMessage, Content: "other"},
	}
	for _, event := range events {
		if err := store.Append(event); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}

	recent, err := store.Recent("s1", 10)
	if err != nil {
		t.Fatalf("Recent() error = %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("Recent() len = %d, want 2", len(recent))
	}
	if recent[1].Action != "git_context" {
		t.Fatalf("Recent()[1].Action = %q, want git_context", recent[1].Action)
	}

	tagged, err := store.ByTag("git", 10)
	if err != nil {
		t.Fatalf("ByTag() error = %v", err)
	}
	if len(tagged) != 1 || tagged[0].Result != "clean" {
		t.Fatalf("ByTag() = %#v, want git result", tagged)
	}
}
