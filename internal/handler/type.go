package handler

import (
	"ai-agent/internal/approval"
	"ai-agent/internal/llm"
	"ai-agent/internal/loop"
	"ai-agent/internal/session"
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
	Path     string `json:"path"`
	ClientID string `json:"clientId,omitempty"`
}

type SessionResponse struct {
	SessionID string `json:"sessionId"`
}

type SessionDetailResponse struct {
	ID                   string                 `json:"id"`
	SessionID            string                 `json:"sessionId"`
	CreatedAt            string                 `json:"createdAt"`
	UpdatedAt            string                 `json:"updatedAt"`
	MessageCount         int                    `json:"messageCount"`
	Messages             []ChatMessageResponse  `json:"messages,omitempty"`
	Workspace            string                 `json:"workspace,omitempty"`
	WriterClientID       string                 `json:"writerClientId,omitempty"`
	PendingWriterRequest *session.WriterRequest `json:"pendingWriterRequest,omitempty"`
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

type SessionConnectRequest struct {
	ClientID string `json:"clientId,omitempty"`
}

type SessionConnectResponse struct {
	ClientID             string                 `json:"clientId"`
	Role                 session.ClientRole     `json:"role"`
	WriterClientID       string                 `json:"writerClientId,omitempty"`
	PendingWriterRequest *session.WriterRequest `json:"pendingWriterRequest,omitempty"`
	Session              SessionDetailResponse  `json:"session"`
}

type WriterRequestBody struct {
	ClientID string `json:"clientId"`
}

type WriterDecisionRequest struct {
	ClientID  string `json:"clientId"`
	RequestID string `json:"requestId,omitempty"`
}

type LiveEvent struct {
	Type                 string                  `json:"type"`
	ClientID             string                  `json:"clientId,omitempty"`
	Role                 string                  `json:"role,omitempty"`
	Content              string                  `json:"content,omitempty"`
	Tool                 string                  `json:"tool,omitempty"`
	Args                 map[string]any          `json:"args,omitempty"`
	Result               string                  `json:"result,omitempty"`
	Error                string                  `json:"error,omitempty"`
	Answer               string                  `json:"answer,omitempty"`
	Action               *approval.PendingAction `json:"action,omitempty"`
	WriterClientID       string                  `json:"writerClientId,omitempty"`
	PendingWriterRequest *session.WriterRequest  `json:"pendingWriterRequest,omitempty"`
	Approved             *bool                   `json:"approved,omitempty"`
}

// SSE types

// StreamRequest is the body for POST /sessions/{sessionID}/stream.
type StreamRequest struct {
	Message  string `json:"message"`
	ClientID string `json:"clientId,omitempty"`
}
