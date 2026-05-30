package loop

import (
	"testing"

	"ai-agent/internal/executor"
	"ai-agent/internal/guardrails"
	"ai-agent/internal/llm"
	"ai-agent/internal/memory"
	"ai-agent/internal/planner"
	"ai-agent/internal/tools/registry"
)

type stubLLM struct {
	replies []string
	calls   int
}

func (s *stubLLM) Chat(messages []llm.Message) (string, error) {
	reply := s.replies[s.calls]
	s.calls++
	return reply, nil
}

func TestRunLoopReturnsDirectAnswer(t *testing.T) {
	llmClient := &stubLLM{replies: []string{
		`{"action":"message","reply":"hello"}`,
	}}
	reg := registry.New()
	mem := memory.NewWorkMemory()
	mem.CreateContext("hi")

	result := RunLoop(LoopRequest{
		Memory:    mem,
		Guardrail: guardrails.MaxIterations(3),
		Planner:   planner.NewLLMPlanner(llmClient, reg),
		Executor:  executor.New(reg),
		LLMClient: llmClient,
	})

	if result.Answer != "hello" {
		t.Fatalf("Answer = %q, want hello", result.Answer)
	}
	if result.StoppedBy != StoppedBySuccess {
		t.Fatalf("StoppedBy = %q, want %q", result.StoppedBy, StoppedBySuccess)
	}
	if result.Iterations != 1 {
		t.Fatalf("Iterations = %d, want 1", result.Iterations)
	}
}

func TestRunLoopStopsByGuardrail(t *testing.T) {
	llmClient := &stubLLM{}
	reg := registry.New()
	mem := memory.NewWorkMemory()
	mem.CreateContext("hi")

	result := RunLoop(LoopRequest{
		Memory:    mem,
		Guardrail: guardrails.MaxIterations(0),
		Planner:   planner.NewLLMPlanner(llmClient, reg),
		Executor:  executor.New(reg),
		LLMClient: llmClient,
	})

	if result.StoppedBy != StoppedByGuardrail {
		t.Fatalf("StoppedBy = %q, want %q", result.StoppedBy, StoppedByGuardrail)
	}
}
