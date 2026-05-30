package memory

import (
	"strings"
	"testing"
)

func TestCreateContextAddsSystemAndUserMessages(t *testing.T) {
	mem := NewWorkMemory()

	mem.CreateContext("hello")

	if mem.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", mem.Len())
	}
	if mem.Messages[0].Role != RoleSystem {
		t.Fatalf("first role = %q, want %q", mem.Messages[0].Role, RoleSystem)
	}
	if mem.Messages[1].Role != RoleUser || mem.Messages[1].Content != "hello" {
		t.Fatalf("user message = %#v, want hello user message", mem.Messages[1])
	}
}

func TestAddToolStoresObservationAsAssistantMessage(t *testing.T) {
	mem := NewWorkMemory()

	mem.AddTool("price: 1")

	if mem.Messages[0].Role != RoleAssistant {
		t.Fatalf("tool role = %q, want %q", mem.Messages[0].Role, RoleAssistant)
	}
	if !strings.Contains(mem.Messages[0].Content, "Tool observation:") {
		t.Fatalf("tool content = %q, want observation prefix", mem.Messages[0].Content)
	}
}
