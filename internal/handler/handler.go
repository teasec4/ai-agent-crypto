package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"ai-agent/internal/approval"
	"ai-agent/internal/harness"
	"ai-agent/internal/llm"
	"ai-agent/internal/loop"
	"ai-agent/internal/memory"
	"ai-agent/internal/session"
)

const (
	maxRequestBodySize = 1 << 20 // 1 MB
	approvalTimeout    = 10 * time.Minute
)

type AgentHandler struct {
	harness  *harness.Harness
	sessions *session.Store
	logger   *slog.Logger
}

func NewAgentHandler(h *harness.Harness, sessions *session.Store) *AgentHandler {
	return &AgentHandler{
		harness:  h,
		sessions: sessions,
		logger:   slog.Default(),
	}
}

func (h *AgentHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /sessions", h.ListSessions)
	mux.HandleFunc("GET /sessions/{sessionID}", h.GetSession)
	mux.HandleFunc("DELETE /sessions/{sessionID}", h.DeleteSession)
	mux.HandleFunc("POST /sessions/{sessionID}/workspace", h.SetWorkspace)
	mux.HandleFunc("POST /sessions", h.CreateSession)
	mux.HandleFunc("POST /ask", h.Ask)
	mux.HandleFunc("POST /chat/completion", h.Ask)

	// SSE streaming endpoint
	mux.HandleFunc("POST /sessions/{sessionID}/stream", h.StreamTask)
	mux.HandleFunc("POST /sessions/{sessionID}/approve", h.ApproveAction)
	mux.HandleFunc("POST /sessions/{sessionID}/reject", h.RejectAction)
}

// ---- Health ----

func (h *AgentHandler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---- REST /ask ----

func (h *AgentHandler) Ask(w http.ResponseWriter, r *http.Request) {
	var req AskRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
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
	if !state.TryStartRun() {
		writeJSON(w, http.StatusConflict, ErrorResponse{Error: "session already has an active agent run"})
		return
	}
	defer state.FinishRun()

	var result harness.HarnessExecutionResult
	sessionWorkspace := state.Workspace()
	state.WithMemory(func(workMemory *memory.WorkMemory) {
		result = h.harness.RunWithMemory(r.Context(), message, workMemory, sessionWorkspace)
	})

	writeJSON(w, http.StatusOK, AskResponse{
		SessionID:     state.ID,
		Answer:        result.LoopResult.Answer,
		Iterations:    result.LoopResult.Iterations,
		StoppedBy:     string(result.LoopResult.StoppedBy),
		Trace:         result.LoopResult.Trace,
		PendingAction: result.LoopResult.PendingAction,
	})
}

// ---- SSE streaming ----

func (h *AgentHandler) StreamTask(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.PathValue("sessionID"))
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "session ID is required"})
		return
	}

	state, ok := h.sessions.Get(sessionID)
	if !ok {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "session not found"})
		return
	}

	var req StreamRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	message := strings.TrimSpace(req.Message)
	if message == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "message is required"})
		return
	}
	if !state.TryStartRun() {
		writeJSON(w, http.StatusConflict, ErrorResponse{Error: "session already has an active agent run"})
		return
	}
	defer state.FinishRun()

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "streaming not supported"})
		return
	}

	// Create approval channel BEFORE starting the loop.
	approvalCh := state.NewApprovalChannel()
	if approvalCh == nil {
		h.logger.Warn("SSE: approval channel already active on this session", "session", sessionID)
		writeJSON(w, http.StatusConflict, ErrorResponse{Error: "an active SSE stream already exists on this session"})
		return
	}
	defer state.FinishApprovalChannel(approvalCh)

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	sessionWorkspace := state.Workspace()

	var result harness.HarnessExecutionResult
	state.WithMemory(func(workMemory *memory.WorkMemory) {
		result = h.harness.RunWithMemoryStreaming(
			r.Context(),
			message,
			workMemory,
			sessionWorkspace,
			func(event loop.SSEEvent) { writeSSE(w, flusher, event) },
			func(ctx context.Context, action *approval.PendingAction) bool {
				writeSSE(w, flusher, loop.SSEEvent{
					Type:   loop.EventApprovalRequired,
					Tool:   action.Tool,
					Args:   action.Args,
					Action: action,
				})
				var approved bool
				select {
				case approved = <-approvalCh:
				case <-ctx.Done():
					h.logger.Info("SSE: approval cancelled", "tool", action.Tool, "error", ctx.Err())
					return false
				case <-time.After(approvalTimeout):
					h.logger.Info("SSE: approval timed out", "tool", action.Tool)
					return false
				}
				if approved {
					h.logger.Info("SSE: user approved", "tool", action.Tool)
				} else {
					h.logger.Info("SSE: user rejected", "tool", action.Tool)
				}
				return approved
			},
		)
	})

	// Send close event — the loop already sent the final EventDone
	fmt.Fprintf(w, "event: close\ndata: {}\n\n")
	flusher.Flush()

	h.logger.Info("SSE stream finished",
		"session", sessionID,
		"stopped_by", result.LoopResult.StoppedBy,
		"iterations", result.LoopResult.Iterations,
	)
}

// ---- Approval signals for active SSE stream ----

func (h *AgentHandler) ApproveAction(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.PathValue("sessionID"))
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "session ID is required"})
		return
	}

	state, ok := h.sessions.Get(sessionID)
	if !ok {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "session not found"})
		return
	}

	h.logger.Info("SSE approve signal received", "session", sessionID)
	state.SignalApproval(true)
	writeJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

func (h *AgentHandler) RejectAction(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.PathValue("sessionID"))
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "session ID is required"})
		return
	}

	state, ok := h.sessions.Get(sessionID)
	if !ok {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "session not found"})
		return
	}

	h.logger.Info("SSE reject signal received", "session", sessionID)
	state.SignalApproval(false)
	writeJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}

// ---- Session management ----

func (h *AgentHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	state := h.sessions.Create()
	writeJSON(w, http.StatusCreated, SessionResponse{SessionID: state.ID})
}

func (h *AgentHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.sessions.List())
}

func (h *AgentHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.PathValue("sessionID"))
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "session ID is required"})
		return
	}

	state, ok := h.sessions.Get(sessionID)
	if !ok {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "session not found"})
		return
	}

	snapshot := state.Snapshot(true)
	messages := visibleMessages(snapshot.Messages)
	writeJSON(w, http.StatusOK, SessionDetailResponse{
		ID:           snapshot.ID,
		SessionID:    snapshot.ID,
		CreatedAt:    snapshot.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:    snapshot.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		MessageCount: len(messages),
		Messages:     messages,
		Workspace:    snapshot.Workspace,
	})
}

func (h *AgentHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.PathValue("sessionID"))
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "session ID is required"})
		return
	}

	if !h.sessions.Delete(sessionID) {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "session not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *AgentHandler) SetWorkspace(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.PathValue("sessionID"))
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "session ID is required"})
		return
	}

	state, ok := h.sessions.Get(sessionID)
	if !ok {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "session not found"})
		return
	}

	var req SetWorkspaceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	path := strings.TrimSpace(req.Path)
	if path == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "path is required"})
		return
	}

	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "path must be an existing directory"})
		return
	}

	state.SetWorkspace(path)

	snapshot := state.Snapshot(false)
	writeJSON(w, http.StatusOK, SessionDetailResponse{
		ID:           snapshot.ID,
		SessionID:    snapshot.ID,
		CreatedAt:    snapshot.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:    snapshot.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		MessageCount: snapshot.MessageCount,
		Workspace:    snapshot.Workspace,
	})
}

// ---- helpers ----

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

func visibleMessages(messages []llm.Message) []ChatMessageResponse {
	result := make([]ChatMessageResponse, 0, len(messages))
	for _, message := range messages {
		if message.Role == memory.RoleSystem {
			continue
		}
		if strings.HasPrefix(message.Content, memory.ToolObservationPrefix) {
			continue
		}

		resp := ChatMessageResponse{
			Role:       message.Role,
			Content:    message.Content,
			ToolCallID: message.ToolCallID,
		}
		if len(message.ToolCalls) > 0 {
			resp.ToolCalls = message.ToolCalls
		}
		result = append(result, resp)
	}

	return result
}

// ---- SSE frame writer ----

func writeSSE(w http.ResponseWriter, flusher http.Flusher, event loop.SSEEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		slog.Warn("SSE marshal failed", "event_type", event.Type, "error", err)
		fmt.Fprintf(w, "event: error\ndata: {\"error\":\"marshal failed\"}\n\n")
		flusher.Flush()
		return
	}

	slog.Debug("SSE write", "event_type", event.Type, "bytes", len(data))
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
	flusher.Flush()
}

// ---- JSON decode/encode ----

func decodeJSON(r *http.Request, v any) error {
	defer func() { _ = r.Body.Close() }()
	r.Body = http.MaxBytesReader(nil, r.Body, maxRequestBodySize)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			return fmt.Errorf("request body too large (max %d bytes)", maxRequestBodySize)
		}
		return fmt.Errorf("invalid JSON request body: %v", err)
	}

	_, _ = io.Copy(io.Discard, r.Body)
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"failed to encode response"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}
