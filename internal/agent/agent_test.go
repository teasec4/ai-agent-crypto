package agent

import (
	"errors"
	"strings"
	"testing"
	"time"

	"ai-agent/internal/llm"
	"ai-agent/internal/memory"
	"ai-agent/internal/retry"
	"ai-agent/internal/tools/registry"
)

type chatResult struct {
	reply string
	err   error
}

type stubLLM struct {
	results  []chatResult
	messages [][]llm.Message
	calls    int
}

func (s *stubLLM) Chat(messages []llm.Message) (string, error) {
	if s.calls >= len(s.results) {
		return "", errors.New("unexpected chat call")
	}
	s.messages = append(s.messages, messages)
	result := s.results[s.calls]
	s.calls++
	return result.reply, result.err
}

func TestRunReplansOnceWhenPlannerReturnsUnknown(t *testing.T) {
	llmClient := &stubLLM{results: []chatResult{
		{reply: `{"action":"unknown","parameters":{"reason":"no suitable tool"}}`},
		{reply: `{"action":"message","reply":"fallback answer"}`},
	}}
	ag := NewAgent(llmClient, registry.New())

	got := ag.Run("do something unsupported")

	if llmClient.calls != 2 {
		t.Fatalf("Chat calls = %d, want 2", llmClient.calls)
	}
	if got != "fallback answer" {
		t.Fatalf("Run() = %q, want %q", got, "fallback answer")
	}
}

func TestRunReturnsFriendlyMessageAfterRepeatedUnknown(t *testing.T) {
	llmClient := &stubLLM{results: []chatResult{
		{reply: `{"action":"unknown","parameters":{"reason":"no suitable tool"}}`},
		{reply: `{"action":"unknown","parameters":{"reason":"still no suitable tool"}}`},
	}}
	ag := NewAgent(llmClient, registry.New())

	got := ag.Run("do something unsupported")

	if llmClient.calls != 2 {
		t.Fatalf("Chat calls = %d, want 2", llmClient.calls)
	}
	if !strings.Contains(got, "Я не умею выполнять этот запрос") {
		t.Fatalf("Run() = %q, want friendly unsupported-action reply", got)
	}
}

func TestFinalizeRetriesTransientError(t *testing.T) {
	oldRetryCfg := retryCfg
	retryCfg = retry.Config{
		MaxAttempts: 2,
		BaseDelay:   time.Nanosecond,
		MaxDelay:    time.Nanosecond,
	}
	defer func() { retryCfg = oldRetryCfg }()

	llmClient := &stubLLM{results: []chatResult{
		{err: errors.New("API returned status 500: temporary")},
		{reply: "formatted result"},
	}}
	ag := &Agent{llmClient: llmClient}

	got, err := ag.finalize("price?", "get_crypto_price", "Bitcoin price: 1.00 USD")
	if err != nil {
		t.Fatalf("finalize returned error: %v", err)
	}
	if got != "formatted result" {
		t.Fatalf("finalize = %q, want %q", got, "formatted result")
	}
	if llmClient.calls != 2 {
		t.Fatalf("Chat calls = %d, want 2", llmClient.calls)
	}
}

func TestRunAddsLongTermMemoryToPlanningContext(t *testing.T) {
	store := memory.NewJSONStore(t.TempDir() + "/events.jsonl")
	if err := store.Append(memory.MemoryEvent{
		SessionID: "s1",
		Time:      time.Now().Add(-time.Minute),
		Type:      memory.EventToolResult,
		Action:    "git_context",
		Result:    "previous git context",
		Tags:      []string{"git"},
	}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	llmClient := &stubLLM{results: []chatResult{
		{reply: `{"action":"message","reply":"ok"}`},
	}}
	ag := NewAgentWithMemory(llmClient, registry.New(), store, "s1", memory.DefaultContextLimit)

	got := ag.Run("what is git status?")

	if got != "ok" {
		t.Fatalf("Run() = %q, want ok", got)
	}
	if len(llmClient.messages) != 1 {
		t.Fatalf("captured Chat calls = %d, want 1", len(llmClient.messages))
	}
	planningMessages := llmClient.messages[0]
	if len(planningMessages) < 2 {
		t.Fatalf("planning messages len = %d, want memory context and user message", len(planningMessages))
	}
	foundMemoryContext := false
	for _, msg := range planningMessages {
		if msg.Role == memory.RoleSystem && strings.Contains(msg.Content, "previous git context") {
			foundMemoryContext = true
			break
		}
	}
	if !foundMemoryContext {
		t.Fatalf("planning messages = %#v, want long-term memory context", planningMessages)
	}
}
