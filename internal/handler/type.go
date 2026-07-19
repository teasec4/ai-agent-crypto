package handler

import (
	"ai-agent/internal/approval"
	"ai-agent/internal/llm"
	"ai-agent/internal/loop"
)

type AskRequest struct {
	SessionID string `json:"sessionId,omitempty"`
	Message   string `json:"message"`
}

type AskResponse struct {
	SessionID     string                  `json:"sessionId"`
	Answer        string                  `json:"answer"`
	Iterations    int                     `json:"iterations"`
	StoppedBy     string                  `json:"stoppedBy"`
	Trace         []loop.LoopIteration    `json:"trace,omitempty"`
	PendingAction *approval.PendingAction `json:"pendingAction,omitempty"`
}

type SetWorkspaceRequest struct {
	Path string `json:"path"`
}

type SessionResponse struct {
	SessionID string `json:"sessionId"`
}

type SessionDetailResponse struct {
	ID           string                `json:"id"`
	SessionID    string                `json:"sessionId"`
	CreatedAt    string                `json:"createdAt"`
	UpdatedAt    string                `json:"updatedAt"`
	MessageCount int                   `json:"messageCount"`
	Messages     []ChatMessageResponse `json:"messages,omitempty"`
	Workspace    string                `json:"workspace,omitempty"`
}

type WorkspaceRootResponse struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type WorkspaceEntryResponse struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
}

type WorkspaceBrowseResponse struct {
	Path       string                   `json:"path"`
	ParentPath string                   `json:"parentPath,omitempty"`
	Roots      []WorkspaceRootResponse  `json:"roots,omitempty"`
	Entries    []WorkspaceEntryResponse `json:"entries"`
}

type ChatMessageResponse struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCalls  []llm.ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// SSE types

// StreamRequest is the body for POST /sessions/{sessionID}/stream.
type StreamRequest struct {
	Message string `json:"message"`
}
