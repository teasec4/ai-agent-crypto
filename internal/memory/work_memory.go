package memory

import (
	"context"
	"fmt"
	"strings"

	"ai-agent/internal/llm"
)

const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
	RoleTool      = "tool"

	ToolObservationPrefix = "Tool observation: "

	SystemDefaultPrompt = "You are a helpful assistant with access to tools. " +
		"Use tools whenever they help you give a more accurate answer. " +
		"When you have enough information, respond directly and concisely."

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

Output only the summary, no explanations.`

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
	h.lastCompactedLen = 0
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

// AddAssistantToolCall adds an assistant message with a tool_call for native tool calling.
func (h *WorkMemory) AddAssistantToolCall(toolCallID, toolName, arguments string) {
	h.Messages = append(h.Messages, llm.Message{
		Role: RoleAssistant,
		ToolCalls: []llm.ToolCall{
			{
				ID:   toolCallID,
				Type: "function",
				Function: llm.FunctionCall{
					Name:      toolName,
					Arguments: arguments,
				},
			},
		},
	})
}

// AddToolResult adds a tool role message (native tool calling format).
func (h *WorkMemory) AddToolResult(toolCallID, content string) {
	h.Messages = append(h.Messages, llm.Message{
		Role:       RoleTool,
		ToolCallID: toolCallID,
		Content:    content,
	})
}

// AddTool adds a tool observation as a user message (legacy format).
// Kept for backward compatibility.
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
func (h *WorkMemory) CompactIfNeeded(ctx context.Context, llmClient llm.LlmClient) {
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

	// Split: old messages to summarize, recent messages to keep. Do not start
	// the retained native-tool history with a tool result; it must stay paired
	// with the preceding assistant tool_call.
	split := len(h.Messages) - keep
	for split > 1 && h.Messages[split].Role == RoleTool {
		split--
	}
	oldMessages := h.Messages[:split]
	recentMessages := h.Messages[split:]

	summary, err := h.summarize(ctx, llmClient, oldMessages)
	if err != nil {
		fallbackSummary := buildFallbackSummary(oldMessages)
		summaryMessage := llm.Message{
			Role:    RoleSystem,
			Content: fmt.Sprintf("Previous conversation summary: %s", fallbackSummary),
		}
		h.Messages = prependDefaultSystem(append([]llm.Message{summaryMessage}, recentMessages...))
		h.lastCompactedLen = currentLen
		return
	}

	summaryMessage := llm.Message{
		Role:    RoleSystem,
		Content: fmt.Sprintf("Previous conversation summary: %s", summary),
	}
	h.Messages = prependDefaultSystem(append([]llm.Message{summaryMessage}, recentMessages...))
	h.lastCompactedLen = currentLen
}

func (h *WorkMemory) summarize(ctx context.Context, llmClient llm.LlmClient, messages []llm.Message) (string, error) {
	var sb strings.Builder
	for _, m := range messages {
		sb.WriteString(fmt.Sprintf("%s: %s\n", m.Role, m.Content))
	}

	msgs := []llm.Message{
		{Role: "system", Content: SummarizePrompt},
		{Role: "user", Content: sb.String()},
	}

	resp, err := llmClient.Chat(ctx, msgs, nil)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}
func buildFallbackSummary(messages []llm.Message) string {
	var userMsgs []string
	var lastAssistant string
	var toolResults []string
	for _, m := range messages {
		switch m.Role {
		case RoleUser:
			content := strings.TrimSpace(m.Content)
			if !strings.HasPrefix(content, ToolObservationPrefix) {
				if len(content) > 300 {
					content = content[:300] + "..."
				}
				userMsgs = append(userMsgs, content)
			}
		case RoleAssistant:
			if len(m.ToolCalls) > 0 {
				for _, tc := range m.ToolCalls {
					toolResults = append(toolResults, fmt.Sprintf("called tool: %s(%s)", tc.Function.Name, tc.Function.Arguments))
				}
			}
			lastAssistant = strings.TrimSpace(m.Content)
		case RoleTool:
			content := strings.TrimSpace(m.Content)
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			toolResults = append(toolResults, fmt.Sprintf("tool result [%s]: %s", m.ToolCallID, content))
		}
	}

	var sb strings.Builder
	if len(userMsgs) > 0 {
		sb.WriteString("User requested: ")
		sb.WriteString(strings.Join(userMsgs, "; "))
	}
	for _, tr := range toolResults {
		if sb.Len() > 0 {
			sb.WriteString(". ")
		}
		sb.WriteString(tr)
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
