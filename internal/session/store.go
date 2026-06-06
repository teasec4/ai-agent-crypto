package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"ai-agent/internal/llm"
	"ai-agent/internal/memory"
)

type Store struct {
	mu       sync.RWMutex
	sessions map[string]*State
}

type State struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time

	mu     sync.Mutex
	memory *memory.WorkMemory
}

type Snapshot struct {
	ID           string        `json:"id"`
	CreatedAt    time.Time     `json:"createdAt"`
	UpdatedAt    time.Time     `json:"updatedAt"`
	MessageCount int           `json:"messageCount"`
	Messages     []llm.Message `json:"messages,omitempty"`
}

func NewStore() *Store {
	return &Store{
		sessions: make(map[string]*State),
	}
}

func (store *Store) Create() *State {
	now := time.Now()
	state := &State{
		ID:        newID(),
		CreatedAt: now,
		UpdatedAt: now,
		memory:    memory.NewDefaultWorkMemory(),
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	store.sessions[state.ID] = state

	return state
}

func (store *Store) Get(id string) (*State, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	state, ok := store.sessions[id]
	return state, ok
}

func (store *Store) List() []Snapshot {
	store.mu.RLock()
	sessions := make([]*State, 0, len(store.sessions))
	for _, state := range store.sessions {
		sessions = append(sessions, state)
	}
	store.mu.RUnlock()

	snapshots := make([]Snapshot, 0, len(sessions))
	for _, state := range sessions {
		snapshots = append(snapshots, state.Snapshot(false))
	}

	return snapshots
}

func (s *State) WithMemory(fn func(*memory.WorkMemory)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fn(s.memory)
	s.UpdatedAt = time.Now()
}

func (s *State) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.memory.Reset()
	s.UpdatedAt = time.Now()
}

func (s *State) Snapshot(includeMessages bool) Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot := Snapshot{
		ID:           s.ID,
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
		MessageCount: s.memory.Len(),
	}

	if includeMessages {
		snapshot.Messages = append([]llm.Message(nil), s.memory.Messages...)
	}

	return snapshot
}

func newID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}

	return hex.EncodeToString(bytes[:])
}
