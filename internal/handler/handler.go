package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"ai-agent/internal/harness"
	"ai-agent/internal/memory"
	"ai-agent/internal/session"
)

type AgentHandler struct {
	harness  *harness.Harness
	sessions *session.Store
}

type AskRequest struct {
	SessionID string `json:"sessionId,omitempty"`
	Message   string `json:"message"`
}

type AskResponse struct {
	SessionID  string `json:"sessionId"`
	Answer     string `json:"answer"`
	Iterations int    `json:"iterations"`
	StoppedBy  string `json:"stoppedBy"`
}

type SessionResponse struct {
	SessionID string `json:"sessionId"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func NewAgentHandler(h *harness.Harness, sessions *session.Store) *AgentHandler {
	return &AgentHandler{
		harness:  h,
		sessions: sessions,
	}
}

func (h *AgentHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /sessions", h.ListSessions)
	mux.HandleFunc("POST /sessions", h.CreateSession)
	mux.HandleFunc("POST /ask", h.Ask)
}

func (h *AgentHandler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *AgentHandler) Ask(w http.ResponseWriter, r *http.Request) {
	var req AskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid JSON request body"})
		return
	}

	message := strings.TrimSpace(req.Message)
	if message == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "message is required"})
		return
	}

	sessionID := strings.TrimSpace(req.SessionID)
	state := h.findOrCreateSession(w, sessionID)
	if state == nil {
		return
	}

	var result harness.HarnessExecutionResult
	state.WithMemory(func(workMemory *memory.WorkMemory) {
		result = h.harness.RunWithMemory(message, workMemory)
	})

	writeJSON(w, http.StatusOK, AskResponse{
		SessionID:  state.ID,
		Answer:     result.LoopResult.Answer,
		Iterations: result.LoopResult.Iterations,
		StoppedBy:  string(result.LoopResult.StoppedBy),
	})
}

func (h *AgentHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	state := h.sessions.Create()
	writeJSON(w, http.StatusCreated, SessionResponse{SessionID: state.ID})
}

func (h *AgentHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.sessions.List())
}

func (h *AgentHandler) findOrCreateSession(w http.ResponseWriter, sessionID string) *session.State {
	if sessionID == "" {
		return h.sessions.Create()
	}

	state, ok := h.sessions.Get(sessionID)
	if !ok {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "session not found"})
		return nil
	}

	return state
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
