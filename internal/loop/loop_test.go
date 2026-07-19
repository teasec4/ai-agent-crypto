package loop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-agent/internal/approval"
	"ai-agent/internal/executor"
	"ai-agent/internal/llm"
	"ai-agent/internal/memory"
	"ai-agent/internal/planner"
	"ai-agent/internal/tools"
	"ai-agent/internal/tools/registry"
)

type sequenceLLMClient struct {
	responses []*llm.ChatResponse
	calls     int
}

func (c *sequenceLLMClient) Chat(ctx context.Context, messages []llm.Message, tools []llm.ToolDefinition) (*llm.ChatResponse, error) {
	if c.calls >= len(c.responses) {
		return &llm.ChatResponse{Content: "done", FinishReason: "stop"}, nil
	}
	response := c.responses[c.calls]
	c.calls++
	return response, nil
}

func TestRunLoopContinuesAfterApprovalPreviewError(t *testing.T) {
	client := &sequenceLLMClient{
		responses: []*llm.ChatResponse{
			{
				FinishReason: "tool_calls",
				ToolCalls: []llm.ToolCall{{
					ID:   "call-1",
					Type: "function",
					Function: llm.FunctionCall{
						Name:      "edit_file",
						Arguments: `{"path":"note.txt","new_text":"hello"}`,
					},
				}},
			},
			{Content: "Не хватает текста для замены, поэтому я уточню действие.", FinishReason: "stop"},
		},
	}
	reg := registry.New(tools.NewEditFileTool())
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "note.txt"), []byte("existing text"), 0644); err != nil {
		t.Fatal(err)
	}
	workMemory := memory.NewDefaultWorkMemory()
	workMemory.AddUser("измени файл note.txt")

	var sawToolError bool
	result := RunLoop(LoopRequest{
		Context:       context.Background(),
		Memory:        workMemory,
		Planner:       planner.NewLLMPlanner(client, reg),
		Executor:      executor.New(reg),
		Workspace:     workspace,
		MaxIterations: 3,
		OnApproval: func(ctx context.Context, action *approval.PendingAction) bool {
			t.Fatalf("approval should not be requested when preview cannot be built")
			return false
		},
		OnEvent: func(event SSEEvent) {
			if event.Type == EventToolError && event.Tool == "edit_file" && strings.Contains(event.Error, "old_text") {
				sawToolError = true
			}
		},
	})

	if result.StoppedBy != StoppedBySuccess {
		t.Fatalf("expected success after retry, got %s: %s", result.StoppedBy, result.Answer)
	}
	if !sawToolError {
		t.Fatal("expected tool_error event for invalid edit_file parameters")
	}

	foundToolResult := false
	for _, msg := range workMemory.Messages {
		if msg.Role == memory.RoleTool && strings.Contains(msg.Content, "old_text") {
			foundToolResult = true
			break
		}
	}
	if !foundToolResult {
		t.Fatalf("expected memory to include tool parameter error, got %#v", workMemory.Messages)
	}
}

func TestRunLoopRepairsEditWithoutOldTextForEmptyFile(t *testing.T) {
	client := &sequenceLLMClient{
		responses: []*llm.ChatResponse{
			{
				FinishReason: "tool_calls",
				ToolCalls: []llm.ToolCall{{
					ID:   "call-1",
					Type: "function",
					Function: llm.FunctionCall{
						Name:      "edit_file",
						Arguments: `{"path":"empty.txt","new_text":"hello"}`,
					},
				}},
			},
			{Content: "Записал текст в пустой файл.", FinishReason: "stop"},
		},
	}
	reg := registry.New(tools.NewEditFileTool(), tools.NewWriteFileTool())
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "empty.txt"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	workMemory := memory.NewDefaultWorkMemory()
	workMemory.AddUser("запиши hello в empty.txt")

	var approvedTool string
	result := RunLoop(LoopRequest{
		Context:       context.Background(),
		Memory:        workMemory,
		Planner:       planner.NewLLMPlanner(client, reg),
		Executor:      executor.New(reg),
		Workspace:     workspace,
		MaxIterations: 3,
		OnApproval: func(ctx context.Context, action *approval.PendingAction) bool {
			approvedTool = action.Tool
			return true
		},
	})

	if result.StoppedBy != StoppedBySuccess {
		t.Fatalf("expected success, got %s: %s", result.StoppedBy, result.Answer)
	}
	if approvedTool != "write_file" {
		t.Fatalf("expected repaired approval for write_file, got %q", approvedTool)
	}
	data, err := os.ReadFile(filepath.Join(workspace, "empty.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("expected file content %q, got %q", "hello", string(data))
	}
}
