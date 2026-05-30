package loop

import (
	"ai-agent/internal/executor"
	"ai-agent/internal/guardrails"
	"ai-agent/internal/llm"
	"ai-agent/internal/memory"
	"ai-agent/internal/planner"
)

type ToolEvent struct {
	Tool   string         `json:"tool"`
	Args   map[string]any `json:"args"`
	Result string         `json:"result"`
	Error  string         `json:"error,omitempty"`
}

type Outcome string

const (
	OutcomeToolCalls Outcome = "tool_calls"
	OutcomeAnswer    Outcome = "answer"
	OutcomeError     Outcome = "error"
)

type LoopIteration struct {
	Index       int         `json:"index"`
	Outcome     Outcome     `json:"outcome"`
	ToolEvents  []ToolEvent `json:"toolEvents"`
	ContextSize int         `json:"contextSize"`
}

type StoppedBy string

const (
	StoppedByModel     StoppedBy = "model"
	StoppedByGuardrail StoppedBy = "guardrail"
	StoppedBySuccess   StoppedBy = "success"
	StoppedByError     StoppedBy = "error"
)

type LoopResult struct {
	Answer     string          `json:"answer"`
	Iterations int             `json:"iterations"`
	Trace      []LoopIteration `json:"trace"`
	StoppedBy  StoppedBy       `json:"stoppedBy"`
}

type LoopRequest struct {
	Memory    *memory.WorkMemory
	Guardrail guardrails.GuardrailFn
	Planner   *planner.LLMPlanner
	Executor  *executor.ToolExecutor
	LLMClient llm.LlmClient
}
