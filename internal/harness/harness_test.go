package harness

import (
	"fmt"
	"strings"
	"testing"

	"ai-agent/internal/executor"
	"ai-agent/internal/guardrails"
	"ai-agent/internal/llm"
	"ai-agent/internal/planner"
	"ai-agent/internal/tools/registry"
)

type captureLLM struct {
	replies []string
	calls   [][]llm.Message
}

func (s *captureLLM) Chat(messages []llm.Message) (string, error) {
	copied := append([]llm.Message(nil), messages...)
	s.calls = append(s.calls, copied)

	replyIndex := len(s.calls) - 1
	if replyIndex >= len(s.replies) {
		return "", fmt.Errorf("missing stub reply for call %d", replyIndex)
	}

	return s.replies[replyIndex], nil
}

func TestSessionKeepsMemoryBetweenRuns(t *testing.T) {
	llmClient := &captureLLM{replies: []string{
		`{"action":"message","reply":"first answer"}`,
		`{"action":"message","reply":"second answer"}`,
	}}
	session := newTestSession(llmClient)

	session.Run("first input")
	session.Run("second input")

	if len(llmClient.calls) != 2 {
		t.Fatalf("LLM calls = %d, want 2", len(llmClient.calls))
	}

	secondPlanningContext := llmClient.calls[1]
	for _, want := range []string{"first input", "first answer", "second input"} {
		if !messagesContain(secondPlanningContext, want) {
			t.Fatalf("second planning context does not contain %q: %#v", want, secondPlanningContext)
		}
	}
}

func TestSessionResetClearsMemory(t *testing.T) {
	llmClient := &captureLLM{replies: []string{
		`{"action":"message","reply":"first answer"}`,
		`{"action":"message","reply":"second answer"}`,
	}}
	session := newTestSession(llmClient)

	session.Run("first input")
	session.Reset()
	session.Run("second input")

	if len(llmClient.calls) != 2 {
		t.Fatalf("LLM calls = %d, want 2", len(llmClient.calls))
	}

	secondPlanningContext := llmClient.calls[1]
	if messagesContain(secondPlanningContext, "first input") || messagesContain(secondPlanningContext, "first answer") {
		t.Fatalf("reset context still contains previous turn: %#v", secondPlanningContext)
	}
	if !messagesContain(secondPlanningContext, "second input") {
		t.Fatalf("reset context does not contain current input: %#v", secondPlanningContext)
	}
}

func newTestSession(llmClient *captureLLM) *Session {
	reg := registry.New()
	h := &Harness{
		llmClient: llmClient,
		planner:   planner.NewLLMPlanner(llmClient, reg),
		executor:  executor.New(reg),
		guardrail: guardrails.MaxIterations(3),
	}

	return h.NewSession()
}

func messagesContain(messages []llm.Message, content string) bool {
	for _, msg := range messages {
		if strings.Contains(msg.Content, content) {
			return true
		}
	}
	return false
}
