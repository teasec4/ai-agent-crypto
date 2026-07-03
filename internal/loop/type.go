package loop

import (
	"log/slog"
	"time"

	"ai-agent/internal/approval"
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
	StoppedByApproval  StoppedBy = "approval_required"
)

type LoopResult struct {
	Answer        string                  `json:"answer"`
	Iterations    int                     `json:"iterations"`
	Trace         []LoopIteration         `json:"trace"`
	StoppedBy     StoppedBy               `json:"stoppedBy"`
	PendingAction *approval.PendingAction `json:"pendingAction,omitempty"`
}

type LoopRequest struct {
	Memory        *memory.WorkMemory
	Guardrail     guardrails.GuardrailFn
	Planner       *planner.LLMPlanner
	Executor      *executor.ToolExecutor
	LLMClient     llm.LlmClient
	AutoApprove   bool
	Logger        *slog.Logger
	Workspace     string
	MaxIterations int       // 0 means use DefaultMaxIterations
	Deadline      time.Time // zero means no deadline
}
