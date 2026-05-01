package llm

// LlmClient defines the interface for LLM communication.
type LlmClient interface {
	// Chat sends a list of messages (system/user/assistant/tool) and returns the assistant's reply.
	Chat(messages []Message) (string, error)
}
