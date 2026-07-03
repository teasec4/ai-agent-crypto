package planner

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"ai-agent/internal/llm"
	"ai-agent/internal/memory"
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
func (p *LLMPlanner) Plan(history []llm.Message) (PlanResult, error) {
	messages := p.buildMessages(history)

	p.logger.Debug("llm chat request",
		"messages", len(messages),
		"last_role", lastRole(history),
	)

	response, err := p.llmClient.Chat(messages)
	if err != nil {
		p.logger.Error("llm chat failed", "error", err.Error())
		return PlanResult{}, fmt.Errorf("llm planning failed: %w", err)
	}

	rawResponse := response
	response = cleanJSONResponse(response)

	p.logger.Debug("llm chat response",
		"raw_bytes", len(rawResponse),
		"cleaned_bytes", len(response),
		"is_json", looksLikeJSON(response),
	)

	var planResponse PlanResult
	if err := json.Unmarshal([]byte(response), &planResponse); err != nil {
		if reply, ok := plainReplyFallback(history, response); ok {
			p.logger.Info("planner fallback: plain reply accepted as answer",
				"reply_bytes", len(reply),
				"after_tool_obs", lastMessageIsToolObservation(history),
			)
			return PlanResult{
				Action:     ActionMessage,
				Parameters: map[string]interface{}{},
				Reply:      reply,
			}, nil
		}
		p.logger.Error("planner JSON parse failed",
			"response", truncateForLog(response, 200),
			"last_role", lastRole(history),
			"error", err.Error(),
		)
		return PlanResult{}, fmt.Errorf("failed to parse planner JSON %q: %w", response, err)
	}

	p.logger.Info("planner decision",
		"action", planResponse.Action,
		"params_count", len(planResponse.Parameters),
		"reply_bytes", len(planResponse.Reply),
		"last_role", lastRole(history),
	)

	if planResponse.Action == "" {
		p.logger.Error("planner returned empty action")
		return PlanResult{}, fmt.Errorf("planner returned empty action")
	}

	if planResponse.Action != ActionMessage &&
		planResponse.Action != ActionUnknown &&
		!p.registry.IsValid(planResponse.Action) {
		p.logger.Error("planner returned unknown action",
			"action", planResponse.Action,
		)
		return PlanResult{}, fmt.Errorf("planner returned unknown action %q", planResponse.Action)
	}

	if planResponse.Action == ActionMessage && strings.TrimSpace(planResponse.Reply) == "" {
		p.logger.Error("planner returned message action without reply")
		return PlanResult{}, fmt.Errorf("planner returned message action without reply")
	}

	if planResponse.Parameters == nil {
		planResponse.Parameters = make(map[string]interface{})
	}

	logAttrs := []any{"action", planResponse.Action}
	if planResponse.Action != ActionMessage && planResponse.Action != ActionUnknown {
		logAttrs = append(logAttrs, "params", planResponse.Parameters)
	}
	p.logger.Info("planner parsed plan", logAttrs...)

	return planResponse, nil
}

func cleanJSONResponse(response string) string {
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	return strings.TrimSpace(response)
}

// plainReplyFallback accepts a non-JSON LLM response as a direct answer
// only when the last message is a tool observation (the LLM is expected to
// respond in natural language to tool results).
func plainReplyFallback(history []llm.Message, response string) (string, bool) {
	response = strings.TrimSpace(response)
	if response == "" {
		return "", false
	}

	// Only accept plain text when the LLM is responding to tool output.
	// Otherwise the LLM is expected to return valid JSON.
	if !lastMessageIsToolObservation(history) {
		return "", false
	}

	return cleanReplyAfterTool(response), true
}

func lastMessageIsToolObservation(history []llm.Message) bool {
	for i := len(history) - 1; i >= 0; i-- {
		content := strings.TrimSpace(history[i].Content)
		if content == "" {
			continue
		}
		return strings.HasPrefix(content, memory.ToolObservationPrefix)
	}
	return false
}

func cleanReplyAfterTool(response string) string {
	if !strings.HasPrefix(response, memory.ToolObservationPrefix) {
		return response
	}

	parts := strings.SplitN(response, "\n\n", 2)
	if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
		return strings.TrimSpace(parts[1])
	}

	lines := strings.Split(response, "\n")
	if len(lines) > 1 && strings.TrimSpace(strings.Join(lines[1:], "\n")) != "" {
		return strings.TrimSpace(strings.Join(lines[1:], "\n"))
	}

	return response
}

// buildMessages constructs the LLM messages array for planning.
func (p *LLMPlanner) buildMessages(history []llm.Message) []llm.Message {
	toolList := p.registry.List()
	systemPrompt := fmt.Sprintf(`You are a planner for an AI agent. Your job is to analyze the user's request and decide the next action.

	Available actions:
	- "message": answer directly without a tool
	- "unknown": use only when no available tool or direct answer fits the request
	- any registered tool listed below

	%s

	Return ONLY valid JSON, no markdown.
	Messages that start with "Tool observation:" are tool results for you to use. Do not copy that prefix into your reply.

	For a tool call:
	{
	  "action": "tool_name",
	  "parameters": { "key": "value" }
	}

	For a direct answer:
	{
	  "action": "message",
	  "reply": "your answer"
	}

	For an unsupported or unclear request:
	{
	  "action": "unknown",
	  "parameters": { "reason": "why no available action fits" }
	}
	`, toolList)

	messages := []llm.Message{{Role: "system", Content: systemPrompt}}
	for _, msg := range history {
		// Skip the memory's default system prompt — our planner prompt replaces it.
		if msg.Role == "system" && msg.Content == memory.SystemDefaultPrompt {
			continue
		}
		messages = append(messages, msg)
	}
	return messages
}

func lastRole(history []llm.Message) string {
	if len(history) == 0 {
		return "none"
	}
	return history[len(history)-1].Role
}

func looksLikeJSON(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[")
}

func truncateForLog(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
