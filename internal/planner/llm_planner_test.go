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
}

func (c *captureLLMClient) Chat(ctx context.Context, messages []llm.Message, tools []llm.ToolDefinition) (*llm.ChatResponse, error) {
	c.messages = append([]llm.Message(nil), messages...)
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
