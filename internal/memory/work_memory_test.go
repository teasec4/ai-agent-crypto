package memory

import (
	"strings"
	"testing"
)

func TestAddToolStoresAssistantObservation(t *testing.T) {
	h := NewWorkMemory()

	h.AddTool("Tool git_context result: clean")

	if got := h.Messages[0].Role; got != RoleAssistant {
		t.Fatalf("AddTool role = %q, want %q", got, RoleAssistant)
	}
	if got := h.Messages[0].Content; !strings.HasPrefix(got, ToolObservationPrefix) {
		t.Fatalf("AddTool content = %q, want prefix %q", got, ToolObservationPrefix)
	}
}
