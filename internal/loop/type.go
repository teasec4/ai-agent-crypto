package loop

import (
	"context"
	"log/slog"
	"time"

	"ai-agent/internal/approval"
	"ai-agent/internal/executor"
	"ai-agent/internal/memory"
	"ai-agent/internal/planner"
)

// ---- streaming events ----

type SSEEventType string

const (
	EventThinking         SSEEventType = "thinking"
	EventToolStart        SSEEventType = "tool_start"
	EventToolDone         SSEEventType = "tool_done"
	EventToolError        SSEEventType = "tool_error"
	EventApprovalRequired SSEEventType = "approval_required"
	EventDone             SSEEventType = "done"
)

type SSEEvent struct {
	Type   SSEEventType            `json:"type"`
	Tool   string                  `json:"tool,omitempty"`
	Args   map[string]any          `json:"args,omitempty"`
	Result string                  `json:"result,omitempty"`
	Error  string                  `json:"error,omitempty"`
	Answer string                  `json:"answer,omitempty"`
	Action *approval.PendingAction `json:"action,omitempty"`
}

// ApprovalFn is called when a tool requires user approval.
type ApprovalFn func(ctx context.Context, action *approval.PendingAction) bool

// ---- trace types ----

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
	Context       context.Context
	Memory        *memory.WorkMemory
	Planner       *planner.LLMPlanner
	Executor      *executor.ToolExecutor
	AutoApprove   bool
	Logger        *slog.Logger
	Workspace     string
	MaxIterations int       // 0 means use DefaultMaxIterations
	Deadline      time.Time // zero means no deadline
	CompactMemory func(context.Context)

	// OnApproval is called when a tool requires user confirmation.
	// If nil and AutoApprove is false, the loop falls back to the legacy
	// StoppedByApproval behaviour (returning PendingAction).
	OnApproval ApprovalFn

	// OnEvent is called for every streaming event.
	// If nil, no events are emitted.
	OnEvent func(SSEEvent)
}
