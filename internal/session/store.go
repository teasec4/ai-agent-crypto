package session

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"ai-agent/internal/id"
	"ai-agent/internal/llm"
	"ai-agent/internal/memory"
)

type Store struct {
	mu       sync.RWMutex
	sessions map[string]*State
	storage  Storage
}

type State struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time

	mu        sync.Mutex
	memory    *memory.WorkMemory
	workspace string
	onChange  func()

	// approvalCh is used by the SSE streaming loop to wait for user approval.
	approvalMu sync.Mutex
	approvalCh chan bool
}

type Snapshot struct {
	ID           string        `json:"id"`
	CreatedAt    time.Time     `json:"createdAt"`
	UpdatedAt    time.Time     `json:"updatedAt"`
	MessageCount int           `json:"messageCount"`
	Messages     []llm.Message `json:"messages,omitempty"`
	Workspace    string        `json:"workspace,omitempty"`
}

func NewStore() *Store {
	store, _ := NewStoreWithStorage(nil)
	return store
}

func NewStoreWithStorage(storage Storage) (*Store, error) {
	store := &Store{
		sessions: make(map[string]*State),
		storage:  storage,
	}

	if storage == nil {
		return store, nil
	}

	states, err := storage.Load()
	if err != nil {
		return nil, err
	}
	for _, persisted := range states {
		state := stateFromPersisted(persisted)
		store.attachOnChange(state)
		store.sessions[state.ID] = state
	}

	return store, nil
}

func (store *Store) Create() *State {
	now := time.Now()
	state := &State{
		ID:        id.New(),
		CreatedAt: now,
		UpdatedAt: now,
		memory:    memory.NewDefaultWorkMemory(),
	}
	store.attachOnChange(state)

	store.mu.Lock()
	store.sessions[state.ID] = state
	store.mu.Unlock()
	store.persist()

	return state
}

func (store *Store) Get(id string) (*State, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	state, ok := store.sessions[id]
	return state, ok
}

func (store *Store) Delete(id string) bool {
	store.mu.Lock()
	_, ok := store.sessions[id]
	if !ok {
		store.mu.Unlock()
		return false
	}
	delete(store.sessions, id)
	store.mu.Unlock()

	store.persist()
	return true
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

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].UpdatedAt.After(snapshots[j].UpdatedAt)
	})

	return snapshots
}

func (s *State) WithMemory(fn func(*memory.WorkMemory)) {
	s.mu.Lock()
	fn(s.memory)
	s.UpdatedAt = time.Now()
	s.mu.Unlock()
	s.changed()
}

func (s *State) Reset() {
	s.mu.Lock()
	s.memory.Reset()
	s.UpdatedAt = time.Now()
	s.mu.Unlock()
	s.changed()
}

func (s *State) SetWorkspace(path string) {
	s.mu.Lock()
	s.workspace = path
	s.UpdatedAt = time.Now()
	s.mu.Unlock()
	s.changed()
}

func (s *State) Workspace() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.workspace
}

func (s *State) Snapshot(includeMessages bool) Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot := Snapshot{
		ID:           s.ID,
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
		MessageCount: s.memory.Len(),
		Workspace:    s.workspace,
	}

	if includeMessages {
		snapshot.Messages = append([]llm.Message(nil), s.memory.Messages...)
	}

	return snapshot
}

func (store *Store) attachOnChange(state *State) {
	state.onChange = store.persist
}

func (store *Store) persist() {
	if store.storage == nil {
		return
	}
	if err := store.storage.Save(store.persistedStates()); err != nil {
		fmt.Printf("failed to persist sessions: %v\n", err)
	}
}

func (store *Store) persistedStates() []PersistedState {
	store.mu.RLock()
	sessions := make([]*State, 0, len(store.sessions))
	for _, state := range store.sessions {
		sessions = append(sessions, state)
	}
	store.mu.RUnlock()

	states := make([]PersistedState, 0, len(sessions))
	for _, state := range sessions {
		states = append(states, state.PersistedState())
	}
	sort.Slice(states, func(i, j int) bool {
		return states[i].UpdatedAt.After(states[j].UpdatedAt)
	})
	return states
}

func (s *State) PersistedState() PersistedState {
	s.mu.Lock()
	defer s.mu.Unlock()

	return PersistedState{
		ID:        s.ID,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
		Messages:  append([]llm.Message(nil), s.memory.Messages...),
		Workspace: s.workspace,
	}
}

func stateFromPersisted(persisted PersistedState) *State {
	messages := append([]llm.Message(nil), persisted.Messages...)
	if len(messages) == 0 {
		messages = memory.NewDefaultWorkMemory().Messages
	}

	return &State{
		ID:        persisted.ID,
		CreatedAt: persisted.CreatedAt,
		UpdatedAt: persisted.UpdatedAt,
		memory:    &memory.WorkMemory{Messages: messages},
		workspace: persisted.Workspace,
	}
}

func (s *State) changed() {
	if s.onChange != nil {
		s.onChange()
	}
}

// NewApprovalChannel creates a fresh buffered channel for SSE approval.
func (s *State) NewApprovalChannel() chan bool {
	s.approvalMu.Lock()
	defer s.approvalMu.Unlock()
	s.approvalCh = make(chan bool, 1)
	return s.approvalCh
}

// SignalApproval sends a value to the active approval channel, if any.
// Safe to call multiple times; if no channel is active the call is silently ignored.
func (s *State) SignalApproval(approved bool) {
	s.approvalMu.Lock()
	ch := s.approvalCh
	if ch == nil {
		s.approvalMu.Unlock()
		return
	}
	s.approvalCh = nil
	s.approvalMu.Unlock()

	select {
	case ch <- approved:
	default:
	}
}

const sessionTTL = 1 * time.Hour

func (store *Store) CleanupExpired() int {
	store.mu.Lock()
	cutoff := time.Now().Add(-sessionTTL)
	removed := 0
	for id, state := range store.sessions {
		state.mu.Lock()
		inactive := state.UpdatedAt.Before(cutoff)
		state.mu.Unlock()
		if inactive {
			delete(store.sessions, id)
			removed++
		}
	}
	store.mu.Unlock()

	if removed > 0 {
		store.persist()
	}
	return removed
}

func (store *Store) StartCleanup(interval time.Duration) (stop func()) {
	ticker := time.NewTicker(interval)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				if removed := store.CleanupExpired(); removed > 0 {
					fmt.Printf("session cleanup: removed %d expired sessions\n", removed)
				}
			case <-done:
				ticker.Stop()
				return
			}
		}
	}()
	return func() { close(done) }
}
