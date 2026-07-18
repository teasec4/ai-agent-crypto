package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"ai-agent/internal/llm"
	"ai-agent/internal/tools/registry"
)

// LLMPlanner uses an LLM to decide the next action.
type LLMPlanner struct {
	llmClient llm.LlmClient
	registry  *registry.Registry
	logger    *slog.Logger
}

func NewLLMPlanner(llmClient llm.LlmClient, reg *registry.Registry) *LLMPlanner {
	return &LLMPlanner{
		llmClient: llmClient,
		registry:  reg,
		logger:    slog.Default(),
	}
}

func (p *LLMPlanner) SetLogger(logger *slog.Logger) {
	p.logger = logger
}

// Plan uses the LLM to determine the next action.
// Tools are passed natively via the API's tools parameter.
func (p *LLMPlanner) Plan(ctx context.Context, history []llm.Message) (PlanResult, error) {
	tools := p.registry.ToolDefinitions()
	messages := p.buildMessages(history)

	p.logger.Debug("planner request",
		"messages", len(messages),
		"tools", len(tools),
		"last_role", lastRole(history),
	)

	chatResp, err := p.llmClient.Chat(ctx, messages, tools)
	if err != nil {
		p.logger.Error("planner: llm chat failed", "error", err.Error())
		return PlanResult{}, fmt.Errorf("llm planning failed: %w", err)
	}

	p.logger.Info("planner response",
		"finish_reason", chatResp.FinishReason,
		"tool_calls", len(chatResp.ToolCalls),
		"content_len", len(chatResp.Content),
	)

	// Handle tool calls from the LLM
	if len(chatResp.ToolCalls) > 0 {
		tc := chatResp.ToolCalls[0]
		params := make(map[string]interface{})
		if tc.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
				p.logger.Warn("planner: failed to parse tool arguments as JSON",
					"tool", tc.Function.Name,
					"arguments", truncateForLog(tc.Function.Arguments, 200),
					"error", err.Error(),
				)
			}
		}

		if !p.registry.IsValid(tc.Function.Name) {
			p.logger.Error("planner: LLM called unknown tool",
				"tool", tc.Function.Name,
			)
			return PlanResult{}, fmt.Errorf("llm called unknown tool %q", tc.Function.Name)
		}

		p.logger.Info("planner: tool call decided",
			"tool", tc.Function.Name,
			"tool_call_id", tc.ID,
			"params", params,
		)

		return PlanResult{
			Action:     tc.Function.Name,
			Parameters: params,
			ToolCallID: tc.ID,
		}, nil
	}

	// Handle text response
	content := strings.TrimSpace(chatResp.Content)

	// Detect "unknown" / refusal
	if content == "" || isRefusal(content) {
		p.logger.Warn("planner: LLM returned empty or refusal", "content", truncateForLog(content, 120))
		return PlanResult{
			Action: ActionUnknown,
			Parameters: map[string]interface{}{
				"reason": content,
			},
		}, nil
	}

	p.logger.Info("planner: message response",
		"reply_len", len(content),
	)

	return PlanResult{
		Action: ActionMessage,
		Reply:  content,
	}, nil
}

// buildMessages constructs the LLM messages array.
func (p *LLMPlanner) buildMessages(history []llm.Message) []llm.Message {
	systemPrompt := `You are an AI coding assistant with access to tools. When the user asks you to do something:

1. If you have all the information needed, answer directly.
2. If you need current information, read files, search code, check git status, run commands, or look up market prices — use the available tools before answering.
3. Always respond in the same language as the user.
4. After completing a task, summarize briefly in 1-2 sentences.
5. If a task is impossible with the available tools, say so clearly.
6. For cryptocurrency price/rank/market questions, always call get_crypto_price first. Do not refuse because a ticker looks unfamiliar or rare; try the tool with the user's symbol/name/id and let the tool resolve it. Only say you cannot find it after the tool fails.

You have access to the following tool categories:
- File operations: read, write, edit, search, list directories
- Git operations: status, diff, log, branch info
- Command execution: go, git, ls, pwd
- Crypto: cryptocurrency price lookups by symbol, ticker, name, or CoinGecko id`

	messages := []llm.Message{{Role: "system", Content: systemPrompt}}
	for _, msg := range history {
		// Skip the memory's default system prompt — our planner replaces it
		if msg.Role == "system" && strings.Contains(msg.Content, "helpful assistant") {
			continue
		}
		messages = append(messages, msg)
	}
	return messages
}

func isRefusal(content string) bool {
	lower := strings.ToLower(content)
	refusals := []string{
		"i cannot", "i can't", "i'm not able", "i am not able",
		"я не могу", "я не в состоянии", "недостаточно информации",
		"i don't have", "no tool",
	}
	for _, r := range refusals {
		if strings.Contains(lower, r) {
			return true
		}
	}
	// If it's very short and sounds like confusion
	if len(content) < 15 && strings.Contains(lower, "?") {
		return true
	}
	return false
}

func lastRole(history []llm.Message) string {
	if len(history) == 0 {
		return "none"
	}
	return history[len(history)-1].Role
}

func truncateForLog(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
