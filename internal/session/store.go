package session

import (
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"ai-agent/internal/id"
	"ai-agent/internal/llm"
	"ai-agent/internal/memory"
)

const (
	writerRequestTTL   = time.Minute
	writerConnectGrace = 15 * time.Second
)

type Store struct {
	mu        sync.RWMutex
	persistMu sync.Mutex
	sessions  map[string]*State
	storage   Storage
}

type State struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time

	mu        sync.Mutex
	memory    *memory.WorkMemory
	workspace string
	onChange  func()

	runMu     sync.Mutex
	runActive bool

	// approvalCh is used by the SSE streaming loop to wait for user approval.
	approvalMu sync.Mutex
	approvalCh chan bool

	controlMu            sync.Mutex
	writerClientID       string
	connectedClientIDs   map[string]time.Time
	pendingWriterRequest *WriterRequest

	eventMu     sync.Mutex
	subscribers map[string]chan any
}

type ClientRole string

const (
	RoleWriter ClientRole = "writer"
	RoleViewer ClientRole = "viewer"
)

type ClientAccess struct {
	ClientID             string         `json:"clientId"`
	Role                 ClientRole     `json:"role"`
	WriterClientID       string         `json:"writerClientId,omitempty"`
	PendingWriterRequest *WriterRequest `json:"pendingWriterRequest,omitempty"`
}

type WriterRequest struct {
	ID           string    `json:"id"`
	FromClientID string    `json:"fromClientId"`
	ToClientID   string    `json:"toClientId,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	ExpiresAt    time.Time `json:"expiresAt"`
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
	state.initRuntimeFields()
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
	store.persistMu.Lock()
	defer store.persistMu.Unlock()
	if err := store.storage.Save(store.persistedStates()); err != nil {
		slog.Error("failed to persist sessions", "error", err)
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

	state := &State{
		ID:        persisted.ID,
		CreatedAt: persisted.CreatedAt,
		UpdatedAt: persisted.UpdatedAt,
		memory:    &memory.WorkMemory{Messages: messages},
		workspace: persisted.Workspace,
	}
	state.initRuntimeFields()
	return state
}

func (s *State) initRuntimeFields() {
	if s.connectedClientIDs == nil {
		s.connectedClientIDs = make(map[string]time.Time)
	}
	if s.subscribers == nil {
		s.subscribers = make(map[string]chan any)
	}
}

func (s *State) changed() {
	if s.onChange != nil {
		s.onChange()
	}
}

func (s *State) ConnectClient(clientID string) ClientAccess {
	if clientID == "" {
		clientID = id.New()
	}

	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	s.initRuntimeFields()
	now := time.Now()
	s.connectedClientIDs[clientID] = now
	s.expireWriterRequestLocked()
	if s.writerClientID == "" || !s.writerActiveLocked(now) {
		s.writerClientID = clientID
		s.pendingWriterRequest = nil
	}
	return s.accessLocked(clientID)
}

func (s *State) ClientAccess(clientID string) ClientAccess {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	s.expireWriterRequestLocked()
	return s.accessLocked(clientID)
}

func (s *State) IsWriter(clientID string) bool {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	return clientID != "" && s.writerClientID == clientID
}

func (s *State) RequestWriter(clientID string) (ClientAccess, error) {
	if clientID == "" {
		return ClientAccess{}, fmt.Errorf("clientId is required")
	}

	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	s.initRuntimeFields()
	now := time.Now()
	s.connectedClientIDs[clientID] = now
	s.expireWriterRequestLocked()

	if s.writerClientID == "" || s.writerClientID == clientID || !s.writerActiveLocked(now) {
		s.writerClientID = clientID
		s.pendingWriterRequest = nil
		return s.accessLocked(clientID), nil
	}
	if s.pendingWriterRequest != nil {
		if s.pendingWriterRequest.FromClientID == clientID {
			return s.accessLocked(clientID), nil
		}
		return ClientAccess{}, fmt.Errorf("writer request already pending")
	}
	s.pendingWriterRequest = &WriterRequest{
		ID:           id.New(),
		FromClientID: clientID,
		ToClientID:   s.writerClientID,
		CreatedAt:    now,
		ExpiresAt:    now.Add(writerRequestTTL),
	}
	return s.accessLocked(clientID), nil
}

func (s *State) ApproveWriterRequest(clientID, requestID string) (ClientAccess, error) {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	request, err := s.pendingRequestForWriterLocked(clientID, requestID)
	if err != nil {
		return ClientAccess{}, err
	}
	s.writerClientID = request.FromClientID
	s.pendingWriterRequest = nil
	return s.accessLocked(s.writerClientID), nil
}

func (s *State) RejectWriterRequest(clientID, requestID string) (ClientAccess, error) {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	_, err := s.pendingRequestForWriterLocked(clientID, requestID)
	if err != nil {
		return ClientAccess{}, err
	}
	s.pendingWriterRequest = nil
	return s.accessLocked(clientID), nil
}

func (s *State) ReleaseWriter(clientID string) (ClientAccess, error) {
	if clientID == "" {
		return ClientAccess{}, fmt.Errorf("clientId is required")
	}
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	if s.writerClientID != clientID {
		return ClientAccess{}, fmt.Errorf("client is not the writer")
	}
	if s.pendingWriterRequest != nil {
		s.writerClientID = s.pendingWriterRequest.FromClientID
		s.pendingWriterRequest = nil
	} else {
		s.writerClientID = s.nextSubscriberWriterLocked(clientID)
	}
	return s.accessLocked(clientID), nil
}

func (s *State) Subscribe(clientID string) (<-chan any, func() (ClientAccess, bool)) {
	if clientID == "" {
		clientID = id.New()
	}
	ch := make(chan any, 64)
	s.eventMu.Lock()
	s.initRuntimeFields()
	s.subscribers[clientID] = ch
	s.eventMu.Unlock()

	return ch, func() (ClientAccess, bool) {
		s.eventMu.Lock()
		if s.subscribers[clientID] == ch {
			delete(s.subscribers, clientID)
			close(ch)
		}
		s.eventMu.Unlock()
		return s.disconnectClient(clientID)
	}
}

func (s *State) disconnectClient(clientID string) (ClientAccess, bool) {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	delete(s.connectedClientIDs, clientID)

	writerChanged := false
	wasWriter := s.writerClientID == clientID && !s.hasSubscriber(clientID)
	if s.pendingWriterRequest != nil && s.pendingWriterRequest.FromClientID == clientID {
		s.pendingWriterRequest = nil
	}
	if s.pendingWriterRequest != nil && s.pendingWriterRequest.ToClientID == clientID {
		if wasWriter && s.clientActiveLocked(s.pendingWriterRequest.FromClientID, time.Now()) {
			s.writerClientID = s.pendingWriterRequest.FromClientID
			writerChanged = true
		}
		s.pendingWriterRequest = nil
	}
	if wasWriter && s.writerClientID == clientID {
		s.writerClientID = s.nextSubscriberWriterLocked(clientID)
		writerChanged = true
	}
	return s.accessLocked(clientID), writerChanged
}

func (s *State) Broadcast(event any) {
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	for _, ch := range s.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func (s *State) accessLocked(clientID string) ClientAccess {
	s.expireWriterRequestLocked()
	role := RoleViewer
	if clientID != "" && s.writerClientID == clientID {
		role = RoleWriter
	}
	return ClientAccess{
		ClientID:             clientID,
		Role:                 role,
		WriterClientID:       s.writerClientID,
		PendingWriterRequest: cloneWriterRequest(s.pendingWriterRequest),
	}
}

func (s *State) pendingRequestForWriterLocked(clientID, requestID string) (*WriterRequest, error) {
	s.expireWriterRequestLocked()
	if clientID == "" {
		return nil, fmt.Errorf("clientId is required")
	}
	if s.writerClientID != clientID {
		return nil, fmt.Errorf("client is not the writer")
	}
	if s.pendingWriterRequest == nil {
		return nil, fmt.Errorf("no writer request is pending")
	}
	if requestID != "" && s.pendingWriterRequest.ID != requestID {
		return nil, fmt.Errorf("writer request not found")
	}
	return cloneWriterRequest(s.pendingWriterRequest), nil
}

func (s *State) expireWriterRequestLocked() {
	if s.pendingWriterRequest != nil && time.Now().After(s.pendingWriterRequest.ExpiresAt) {
		s.pendingWriterRequest = nil
	}
}

func (s *State) writerActiveLocked(now time.Time) bool {
	if s.writerClientID == "" {
		return false
	}
	return s.clientActiveLocked(s.writerClientID, now)
}

func (s *State) clientActiveLocked(clientID string, now time.Time) bool {
	if clientID == "" {
		return false
	}
	if s.hasSubscriber(clientID) {
		return true
	}
	connectedAt, ok := s.connectedClientIDs[clientID]
	return ok && now.Sub(connectedAt) <= writerConnectGrace
}

func (s *State) nextSubscriberWriterLocked(excludeClientID string) string {
	s.eventMu.Lock()
	ids := make([]string, 0, len(s.subscribers))
	for clientID := range s.subscribers {
		if clientID != "" && clientID != excludeClientID {
			ids = append(ids, clientID)
		}
	}
	s.eventMu.Unlock()

	sort.Slice(ids, func(i, j int) bool {
		left, leftOK := s.connectedClientIDs[ids[i]]
		right, rightOK := s.connectedClientIDs[ids[j]]
		if leftOK != rightOK {
			return leftOK
		}
		if !left.Equal(right) {
			return left.Before(right)
		}
		return ids[i] < ids[j]
	})
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

func (s *State) hasSubscriber(clientID string) bool {
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	_, ok := s.subscribers[clientID]
	return ok
}

func cloneWriterRequest(req *WriterRequest) *WriterRequest {
	if req == nil {
		return nil
	}
	clone := *req
	return &clone
}

// TryStartRun marks the session as busy. Only one agent loop may mutate a
// session's memory at a time.
func (s *State) TryStartRun() bool {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	if s.runActive {
		return false
	}
	s.runActive = true
	return true
}

// FinishRun releases the session run lock.
func (s *State) FinishRun() {
	s.runMu.Lock()
	s.runActive = false
	s.runMu.Unlock()
}

// NewApprovalChannel creates a fresh buffered channel for SSE approval.
// Returns nil if a channel is already active (to prevent overwrite).
func (s *State) NewApprovalChannel() chan bool {
	s.approvalMu.Lock()
	defer s.approvalMu.Unlock()
	if s.approvalCh != nil {
		return nil
	}
	s.approvalCh = make(chan bool, 1)
	return s.approvalCh
}

// FinishApprovalChannel clears the active SSE approval channel if it still
// belongs to the caller's stream.
func (s *State) FinishApprovalChannel(ch chan bool) {
	s.approvalMu.Lock()
	defer s.approvalMu.Unlock()
	if s.approvalCh == ch {
		s.approvalCh = nil
	}
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

func (store *Store) CleanupExpired(ttl time.Duration) int {
	if ttl <= 0 {
		return 0
	}
	store.mu.Lock()
	cutoff := time.Now().Add(-ttl)
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

func (store *Store) StartCleanup(interval time.Duration, ttl time.Duration) (stop func()) {
	ticker := time.NewTicker(interval)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				if removed := store.CleanupExpired(ttl); removed > 0 {
					slog.Info("session cleanup removed expired sessions", "removed", removed)
				}
			case <-done:
				ticker.Stop()
				return
			}
		}
	}()
	return func() { close(done) }
}
