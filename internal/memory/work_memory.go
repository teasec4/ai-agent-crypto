package memory

import (
	"fmt"
	"strings"

	"ai-agent/internal/llm"
)

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

// WorkMemory holds the conversation history with compaction support.
type WorkMemory struct {
	Messages []llm.Message
}

func NewWorkMemory() *WorkMemory {
	return &WorkMemory{Messages: make([]llm.Message, 0)}
}

func (h *WorkMemory) AddUser(content string) {
	h.Messages = append(h.Messages, llm.Message{Role: RoleUser, Content: content})
}

func (h *WorkMemory) AddAssistant(content string) {
	if content == "" {
		return
	}
	h.Messages = append(h.Messages, llm.Message{Role: RoleAssistant, Content: content})
}

func (h *WorkMemory) AddTool(content string) {
	if content == "" {
		return
	}
	h.Messages = append(h.Messages, llm.Message{Role: RoleTool, Content: content})
}

// CompactIfNeeded checks if the history exceeds the threshold and compacts it.
// Uses LLM summarization to preserve context; falls back to simple trimming on error.
func (h *WorkMemory) CompactIfNeeded(llmClient llm.LlmClient) {
	if len(h.Messages) <= DefaultCompactAt {
		return
	}

	keep := len(h.Messages) / CompactKeepRatio
	if keep < 1 {
		keep = 1
	}

	// Split: old messages to summarize, recent messages to keep
	oldMessages := h.Messages[:len(h.Messages)-keep]
	recentMessages := h.Messages[len(h.Messages)-keep:]

	summary, err := h.summarize(llmClient, oldMessages)
	if err != nil {
		// Fallback: simple trim keeping only recent messages
		h.Messages = recentMessages
		return
	}

	// Replace old messages with a single system summary
	compacted := make([]llm.Message, 0, 1+len(recentMessages))
	compacted = append(compacted, llm.Message{
		Role:    "system",
		Content: fmt.Sprintf("Previous conversation summary: %s", summary),
	})
	compacted = append(compacted, recentMessages...)
	h.Messages = compacted
}

// summarize sends old messages to the LLM for condensation.
func (h *WorkMemory) summarize(llmClient llm.LlmClient, messages []llm.Message) (string, error) {
	var sb strings.Builder
	for _, m := range messages {
		sb.WriteString(fmt.Sprintf("%s: %s\n", m.Role, m.Content))
	}

	msgs := []llm.Message{
		{Role: "system", Content: SummarizePrompt},
		{Role: "user", Content: sb.String()},
	}

	return llmClient.Chat(msgs)
}

func (h *WorkMemory) Len() int {
	return len(h.Messages)
}
