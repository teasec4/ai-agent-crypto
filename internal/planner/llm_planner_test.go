package planner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-agent/internal/llm"
	"ai-agent/internal/projectmemory"
	"ai-agent/internal/tools"
	"ai-agent/internal/tools/registry"
)

type captureLLMClient struct {
	messages []llm.Message
	response *llm.ChatResponse
}

func (c *captureLLMClient) Chat(ctx context.Context, messages []llm.Message, tools []llm.ToolDefinition) (*llm.ChatResponse, error) {
	c.messages = append([]llm.Message(nil), messages...)
	if c.response != nil {
		return c.response, nil
	}
	return &llm.ChatResponse{Content: "ok", FinishReason: "stop"}, nil
}

func TestPlanIncludesProjectMemory(t *testing.T) {
	dir := t.TempDir()
	memoryPath := filepath.Join(dir, projectmemory.RelativePath)
	if err := os.MkdirAll(filepath.Dir(memoryPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(memoryPath, []byte("## User Preferences\n- Answer in Russian\n"), 0644); err != nil {
		t.Fatal(err)
	}

	client := &captureLLMClient{}
	planner := NewLLMPlanner(client, registry.New(tools.NewReadProjectMemoryTool()))
	_, err := planner.Plan(context.Background(), []llm.Message{{Role: "user", Content: "hello"}}, dir)
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	found := false
	for _, msg := range client.messages {
		if msg.Role == "system" && strings.Contains(msg.Content, "Project memory from .agent/memory.md") && strings.Contains(msg.Content, "Answer in Russian") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected project memory system message, got %#v", client.messages)
	}
}

func TestPlanUnknownToolReturnsActionUnknown(t *testing.T) {
	client := &captureLLMClient{
		response: &llm.ChatResponse{
			FinishReason: "tool_calls",
			ToolCalls: []llm.ToolCall{{
				ID:   "call-1",
				Type: "function",
				Function: llm.FunctionCall{
					Name:      "delete_directory",
					Arguments: `{"path":"test-folder"}`,
				},
			}},
		},
	}
	planner := NewLLMPlanner(client, registry.New(tools.NewReadFileTool()))

	result, err := planner.Plan(context.Background(), []llm.Message{{Role: "user", Content: "удали папку test-folder"}}, t.TempDir())
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if result.Action != ActionUnknown {
		t.Fatalf("expected ActionUnknown, got %q", result.Action)
	}
	reason, _ := result.Parameters["reason"].(string)
	if !strings.Contains(reason, "delete_directory") {
		t.Fatalf("expected reason to mention unknown tool, got %q", reason)
	}
}
