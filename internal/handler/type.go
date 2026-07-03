package handler

import (
	"ai-agent/internal/approval"
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

type ApprovalRequest struct {
	Approved bool `json:"approved"`
}

type SetWorkspaceRequest struct {
	Path string `json:"path"`
}

type SessionResponse struct {
	SessionID string `json:"sessionId"`
}

type SessionDetailResponse struct {
	ID             string                    `json:"id"`
	SessionID      string                    `json:"sessionId"`
	CreatedAt      string                    `json:"createdAt"`
	UpdatedAt      string                    `json:"updatedAt"`
	MessageCount   int                       `json:"messageCount"`
	Messages       []ChatMessageResponse     `json:"messages"`
	PendingActions []*approval.PendingAction `json:"pendingActions,omitempty"`
	Workspace      string                    `json:"workspace,omitempty"`
}

type ChatMessageResponse struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Text    string `json:"text"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
