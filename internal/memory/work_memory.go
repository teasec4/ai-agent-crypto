package memory

import (
	"fmt"
	"strings"

	"ai-agent/internal/llm"
)

const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"

	ToolObservationPrefix = "Tool observation: "

	SystemDefaultPrompt = `
		You are a helpful assistant with access to tools.
		Use tools whenever they help you give a more accurate answer.
		When you have enough information, respond directly and concisely.
	`

	DefaultCompactAt = 30
	CompactMinGap    = 10 // minimum new messages before next compaction

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

		Output only the summary, no explanations.
	`

	CompactKeepRatio = 2 // keep at most len(Messages)/CompactKeepRatio messages after compaction
)

// WorkMemory holds the conversation history with compaction support.
type WorkMemory struct {
	Messages         []llm.Message
	lastCompactedLen int // message count when compaction last ran
}

func NewDefaultWorkMemory() *WorkMemory {
	return &WorkMemory{Messages: []llm.Message{defaultSystemMessage()}}
}

func (h *WorkMemory) Reset() {
	h.Messages = h.Messages[:0]
	h.Messages = append(h.Messages, defaultSystemMessage())
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
	h.Messages = append(h.Messages, llm.Message{Role: RoleUser, Content: ToolObservationPrefix + content})
}

// FormatToolResult builds a tool observation message for the conversation history.
func FormatToolResult(action string, result string, err error, prefix string) string {
	if err != nil {
		if result != "" {
			return fmt.Sprintf("%s%s output:\n%s\nError: %v", prefix, action, result, err)
		}
		return fmt.Sprintf("%s%s failed: %v", prefix, action, err)
	}
	return fmt.Sprintf("%s%s result: %s", prefix, action, result)
}

// CompactIfNeeded checks if the history exceeds the threshold and compacts it.
// Uses LLM summarization to preserve context; falls back to simple trimming on error.
// Respects CompactMinGap to avoid compacting too frequently.
func (h *WorkMemory) CompactIfNeeded(llmClient llm.LlmClient) {
	currentLen := len(h.Messages)
	if currentLen <= DefaultCompactAt {
		return
	}
	if h.lastCompactedLen > 0 && currentLen-h.lastCompactedLen < CompactMinGap {
		return
	}

	keep := currentLen / CompactKeepRatio
	if keep < 1 {
		keep = 1
	}

	// Split: old messages to summarize, recent messages to keep
	oldMessages := h.Messages[:len(h.Messages)-keep]
	recentMessages := h.Messages[len(h.Messages)-keep:]

	summary, err := h.summarize(llmClient, oldMessages)
	if err != nil {
		// Fallback: build a simple text summary from old messages instead of discarding them
		fallbackSummary := buildFallbackSummary(oldMessages)
		summaryMessage := llm.Message{
			Role:    RoleSystem,
			Content: fmt.Sprintf("Previous conversation summary: %s", fallbackSummary),
		}
		h.Messages = prependDefaultSystem(append([]llm.Message{summaryMessage}, recentMessages...))
		h.lastCompactedLen = currentLen
		return
	}

	// Replace old messages with a single system summary
	summaryMessage := llm.Message{
		Role:    RoleSystem,
		Content: fmt.Sprintf("Previous conversation summary: %s", summary),
	}
	h.Messages = prependDefaultSystem(append([]llm.Message{summaryMessage}, recentMessages...))
	h.lastCompactedLen = currentLen
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

// buildFallbackSummary creates a simple text summary when LLM summarization fails.
// It keeps the user intents and discards verbose tool outputs.
func buildFallbackSummary(messages []llm.Message) string {
	var userMsgs []string
	var lastAssistant string
	for _, m := range messages {
		switch m.Role {
		case RoleUser:
			content := strings.TrimSpace(m.Content)
			if !strings.HasPrefix(content, ToolObservationPrefix) {
				// Truncate long user messages
				if len(content) > 300 {
					content = content[:300] + "..."
				}
				userMsgs = append(userMsgs, content)
			}
		case RoleAssistant:
			lastAssistant = strings.TrimSpace(m.Content)
		}
	}

	var sb strings.Builder
	if len(userMsgs) > 0 {
		sb.WriteString("User requested: ")
		sb.WriteString(strings.Join(userMsgs, "; "))
	}
	if lastAssistant != "" {
		if sb.Len() > 0 {
			sb.WriteString(". ")
		}
		sb.WriteString("Last response: ")
		if len(lastAssistant) > 300 {
			lastAssistant = lastAssistant[:300] + "..."
		}
		sb.WriteString(lastAssistant)
	}
	if sb.Len() == 0 {
		return "Previous conversation (summary unavailable)"
	}
	return sb.String()
}

func (h *WorkMemory) Len() int {
	return len(h.Messages)
}

func prependDefaultSystem(messages []llm.Message) []llm.Message {
	result := make([]llm.Message, 0, 1+len(messages))
	result = append(result, defaultSystemMessage())
	for _, msg := range messages {
		if msg.Role == RoleSystem && msg.Content == SystemDefaultPrompt {
			continue
		}
		result = append(result, msg)
	}
	return result
}

func defaultSystemMessage() llm.Message {
	return llm.Message{Role: RoleSystem, Content: SystemDefaultPrompt}
}
