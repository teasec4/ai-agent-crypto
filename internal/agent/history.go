package agent

import "ai-agent/internal/llm"

const (
	roleUser      = "user"
	roleAssistant = "assistant"
	roleTool      = "tool"

	defaultCompactAt = 10

	// summarizePrompt instructs the LLM to condense older conversation history.
	summarizePrompt = `You are a conversation summarizer. Condense the following conversation into 1-2 sentences.

Preserve:
- The user's intents and what they asked about
- Key data retrieved from tools (prices, statuses, results, errors)
- The final answer or outcome given to the user
- Any decisions or conclusions reached

Discard:
- Verbose tool logs, repetitive formatting, truncation markers
- Redundant or obvious details

Output only the summary, no explanations.`

	compactKeepRatio = 2 // keep at most history.len()/compactKeepRatio messages after compaction
)

// history is a simple list of messages with compaction support.
type history struct {
	messages []llm.Message
}

func newHistory() *history {
	return &history{messages: make([]llm.Message, 0)}
}

func (h *history) addUser(content string) {
	h.messages = append(h.messages, llm.Message{Role: roleUser, Content: content})
}

func (h *history) addAssistant(content string) {
	h.messages = append(h.messages, llm.Message{Role: roleAssistant, Content: content})
}

func (h *history) len() int {
	return len(h.messages)
}
