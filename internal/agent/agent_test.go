package agent

import (
	"errors"
	"strings"
	"testing"
	"time"

	"ai-agent/internal/llm"
	"ai-agent/internal/retry"
	"ai-agent/internal/tools/registry"
)

type chatResult struct {
	reply string
	err   error
}

type stubLLM struct {
	results []chatResult
	calls   int
}

func (s *stubLLM) Chat(messages []llm.Message) (string, error) {
	if s.calls >= len(s.results) {
		return "", errors.New("unexpected chat call")
	}
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
	if !strings.Contains(got, "Я не умею выполнить этот запрос") {
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
