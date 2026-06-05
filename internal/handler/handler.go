package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"ai-agent/internal/harness"
)

type AgentHandler struct {
	harness *harness.Harness
}

type AskRequest struct {
	Message string `json:"message"`
}

type AskResponse struct {
	Answer     string `json:"answer"`
	Iterations int    `json:"iterations"`
	StoppedBy  string `json:"stoppedBy"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func NewAgentHandler(h *harness.Harness) *AgentHandler {
	return &AgentHandler{harness: h}
}

func (h *AgentHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.Health)
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

	result := h.harness.Run(message)
	writeJSON(w, http.StatusOK, AskResponse{
		Answer:     result.LoopResult.Answer,
		Iterations: result.LoopResult.Iterations,
		StoppedBy:  string(result.LoopResult.StoppedBy),
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
