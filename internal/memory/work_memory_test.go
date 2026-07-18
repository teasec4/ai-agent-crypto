package memory

import (
	"context"
	"testing"

	"ai-agent/internal/llm"
)

type fakeSummaryClient struct{}

func (fakeSummaryClient) Chat(ctx context.Context, messages []llm.Message, tools []llm.ToolDefinition) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Content: "summary"}, nil
}

func TestCompactIfNeededKeepsToolResultWithAssistantToolCall(t *testing.T) {
	mem := NewDefaultWorkMemory()
	for i := 0; i < 14; i++ {
		mem.AddUser("message")
	}
	mem.AddAssistantToolCall("call-1", "read_file", `{"path":"go.mod"}`)
	mem.AddToolResult("call-1", "Tool read_file result: module test")
	for i := 0; i < 14; i++ {
		mem.AddUser("follow-up")
	}

	mem.CompactIfNeeded(context.Background(), fakeSummaryClient{})

	for i, msg := range mem.Messages {
		if msg.Role != RoleTool {
			continue
		}
		if i == 0 || len(mem.Messages[i-1].ToolCalls) == 0 {
			t.Fatalf("tool result at index %d is not paired with preceding assistant tool_call", i)
		}
		if mem.Messages[i-1].ToolCalls[0].ID != msg.ToolCallID {
			t.Fatalf("tool result id %q does not match preceding tool call id %q", msg.ToolCallID, mem.Messages[i-1].ToolCalls[0].ID)
		}
	}
}
