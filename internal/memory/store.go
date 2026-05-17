package memory

import "time"

const (
	DefaultMemoryPath    = ".agent/memory/events.jsonl"
	DefaultSessionID     = "default"
	DefaultContextLimit  = 8
	DefaultTaggedLimit   = 3
	DefaultEventMaxChars = 240

	EventUserMessage      = "user_message"
	EventAssistantMessage = "assistant_message"
	EventPlanResult       = "plan_result"
	EventPlanError        = "plan_error"
	EventToolResult       = "tool_result"
	EventToolError        = "tool_error"
)

type Store interface {
	Append(event MemoryEvent) error
	Recent(sessionID string, limit int) ([]MemoryEvent, error)
	ByTag(tag string, limit int) ([]MemoryEvent, error)
}

type MemoryEvent struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Time      time.Time `json:"time"`
	Type      string    `json:"type"`
	Content   string    `json:"content,omitempty"`

	Action string                 `json:"action,omitempty"`
	Params map[string]interface{} `json:"params,omitempty"`
	Result string                 `json:"result,omitempty"`
	Error  string                 `json:"error,omitempty"`
	Tags   []string               `json:"tags,omitempty"`
}
