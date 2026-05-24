package loop

type ToolEvent struct {
	Tool   string         `json:"tool"`
	Args   map[string]any `json:"args"`
	Result string         `json:"result"`
}

type Outcome string

const (
	OutcomeToolCalls Outcome = "tool_calls"
	OutcomeAnswer    Outcome = "answer"
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
)

type LoopResult struct {
	Answer    string          `json:"answer"`
	Iterations int            `json:"iterations"`
	Trace     []LoopIteration `json:"trace"`
	StoppedBy StoppedBy       `json:"stoppedBy"`
}