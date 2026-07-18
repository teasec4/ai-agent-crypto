package llm

import "context"

// LlmClient defines the interface for LLM communication.
type LlmClient interface {
	// Chat sends messages and optional tool definitions to the LLM.
	// Returns a ChatResponse which contains either text content or tool calls.
	Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (*ChatResponse, error)
}
