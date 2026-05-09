package memory

import "ai-agent/internal/llm"

const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"

	DefaultCompactAt = 10

	// SummarizePrompt instructs the LLM to condense older conversation history.
	SummarizePrompt = `You are a conversation summarizer. Condense the following conversation into 1-2 sentences.

Preserve:
- The user's intents and what they asked about
- Key data retrieved from tools (prices, statuses, results, errors)
- The final answer or outcome given to the user
- Any decisions or conclusions reached

Discard:
- Verbose tool logs, repetitive formatting, truncation markers
- Redundant or obvious details

Output only the summary, no explanations.`

	CompactKeepRatio = 2 // keep at most len(Messages)/CompactKeepRatio messages after compaction
)

// history is a simple list of messages with compaction support.
type WorkMemory struct {
	Messages []llm.Message
}

func NewWorkMemory() *WorkMemory {
	return &WorkMemory{Messages: make([]llm.Message, 0)}
}

func (h *WorkMemory) addUser(content string) {
	h.Messages = append(h.Messages, llm.Message{Role: RoleUser, Content: content})
}

func (h *WorkMemory) addAssistant(content string) {
	h.Messages = append(h.Messages, llm.Message{Role: RoleAssistant, Content: content})
}

func (h *WorkMemory) len() int {
	return len(h.Messages)
}
