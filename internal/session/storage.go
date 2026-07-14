package session

import (
	"ai-agent/internal/llm"
	"time"
)

// Storage persists API sessions. It is intentionally small so it can be
// replaced later with sqlite, postgres, redis, etc.
type Storage interface {
	Load() ([]PersistedState, error)
	Save(states []PersistedState) error
}

type PersistedState struct {
	ID        string        `json:"id"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
	Messages  []llm.Message `json:"messages"`
	Workspace string        `json:"workspace,omitempty"`
}
