package agent

import (
	"fmt"
	"log"
	"strings"
	"time"

	"ai-agent/internal/config"
	"ai-agent/internal/executor"
	"ai-agent/internal/llm"
	"ai-agent/internal/memory"
	"ai-agent/internal/planner"
	"ai-agent/internal/retry"
	"ai-agent/internal/tools"
	"ai-agent/internal/tools/registry"
)

// retryCfg defines the retry policy for LLM calls (Plan, Finalize).
// Tool execution is NOT retried — tool errors feed back into the planner.
var retryCfg = retry.Config{
	MaxAttempts: 3,
	BaseDelay:   1 * time.Second,
	MaxDelay:    30 * time.Second,
}

// maxLoopAttempts caps the Plan→Act→Observe loop iterations.
// Each iteration includes retry-backed Plan + one tool execution.
const maxLoopAttempts = 5

const unsupportedActionReply = "Я не умею выполнять этот запрос доступными инструментами. Сейчас могу ответить напрямую, посмотреть git-контекст или получить цену криптовалюты."

type Agent struct {
	llmClient          llm.LlmClient
	planner            *planner.LLMPlanner
	executor           *executor.ToolExecutor
	memory             *memory.WorkMemory
	longTermMemory     *memory.LongTermMemory
	sessionID          string
	memoryContextLimit int
}

func NewAgent(
	llmClient llm.LlmClient,
	reg *registry.Registry,
) *Agent {
	return NewAgentWithMemory(llmClient, reg, nil, memory.DefaultSessionID, memory.DefaultContextLimit)
}

func NewAgentWithMemory(
	llmClient llm.LlmClient,
	reg *registry.Registry,
	store memory.Store,
	sessionID string,
	contextLimit int,
) *Agent {
	if sessionID == "" {
		sessionID = memory.DefaultSessionID
	}
	if contextLimit <= 0 {
		contextLimit = memory.DefaultContextLimit
	}

	return &Agent{
		llmClient:          llmClient,
		planner:            planner.NewLLMPlanner(llmClient, reg),
		executor:           executor.New(reg),
		memory:             memory.NewWorkMemory(),
		longTermMemory:     memory.NewLongTermMemory(store),
		sessionID:          sessionID,
		memoryContextLimit: contextLimit,
	}
}

// NewWithConfig creates a fully wired Agent from config.
// This is the single place for wiring — use it from all entry points.
func NewWithConfig(cfg *config.Config) *Agent {
	cryptoTool := tools.NewCryptoTool()
	cryptoTool.SetAPIKey(cfg.CoinGeckoApiKey)
	gitTool := tools.NewGitTool()
	helpTool := tools.NewHelpTool()

	llmClient := llm.NewClientWithTimeout(
		cfg.OpenAIApiKey,
		cfg.LLMBaseURL,
		cfg.LLMModel,
		cfg.LLMTemperature,
		cfg.LLMMaxTokens,
		time.Duration(cfg.TimeoutSeconds)*time.Second,
	)

	reg := registry.New(cryptoTool, gitTool, helpTool)
	store := memory.NewJSONStore(cfg.MemoryPath)
	return NewAgentWithMemory(llmClient, reg, store, cfg.MemorySessionID, cfg.MemoryContextLimit)
}

func (a *Agent) Run(input string) string {
	log.Printf("[Agent] Planning...")
	runStarted := time.Now().UTC()
	a.memory.AddUser(input)
	a.remember(memory.MemoryEvent{
		Type:    memory.EventUserMessage,
		Content: input,
		Tags:    inferEventTags("", input),
	})

	var lastErr error
	unknownAttempts := 0
	for attempt := 1; attempt <= maxLoopAttempts; attempt++ {
		// Phase 1: Plan — with retry for transient LLM errors.
		// Retryable: network, timeout, 429, 5xx.
		// Non-retryable: bad JSON, validation — added to history, loop retries with context.
		planResult, err := a.plan(runStarted, input)
		if err != nil {
			lastErr = err
			if retry.IsFatal(err) {
				a.remember(memory.MemoryEvent{
					Type:  memory.EventPlanError,
					Error: err.Error(),
					Tags:  []string{"planner"},
				})
				return fmt.Sprintf("Ошибка: %v", err)
			}
			// Feed error back into history so the planner can adapt on next iteration
			a.memory.AddTool(fmt.Sprintf("Plan attempt %d failed: %v", attempt, err))
			a.remember(memory.MemoryEvent{
				Type:  memory.EventPlanError,
				Error: err.Error(),
				Tags:  []string{"planner"},
			})
			log.Printf("[Agent] Plan attempt %d/%d failed (retryable): %v", attempt, maxLoopAttempts, err)
			continue
		}

		a.rememberPlan(planResult)

		// Direct answer path: planner chose "message" action
		if planResult.Action == planner.ActionMessage {
			a.memory.AddAssistant(planResult.Reply)
			a.memory.CompactIfNeeded(a.llmClient)
			a.rememberAssistant(planResult.Reply)
			return planResult.Reply
		}

		// Unknown is a planner fallback, not a real completed action.
		// Give the planner one chance to recover, then return a user-facing fallback.
		if planResult.Action == planner.ActionUnknown {
			unknownAttempts++
			result := unknownObservation(planResult.Parameters)
			lastErr = fmt.Errorf("planner returned unknown action: %s", result)
			a.memory.AddTool("Planner returned unknown: " + result)
			if unknownAttempts > 1 {
				a.memory.AddAssistant(unsupportedActionReply)
				a.memory.CompactIfNeeded(a.llmClient)
				a.rememberAssistant(unsupportedActionReply)
				return unsupportedActionReply
			}

			log.Printf("[Agent] Planner returned unknown; retrying with observation: %s", truncate(result, 200))
			continue
		}

		// Phase 2: Act — execute the chosen tool (no retry, errors go to history)
		log.Printf("[Agent] Executing tool: %s", planResult.Action)
		result, toolErr := a.executor.Execute(planResult)
		if toolErr != nil {
			lastErr = toolErr
			a.memory.AddTool(fmt.Sprintf("Tool %s failed: %v", planResult.Action, toolErr))
			a.remember(memory.MemoryEvent{
				Type:   memory.EventToolError,
				Action: planResult.Action,
				Params: planResult.Parameters,
				Error:  toolErr.Error(),
				Tags:   inferEventTags(planResult.Action, toolErr.Error()),
			})
			log.Printf("[Agent] Tool error: %v", toolErr)
			continue
		}

		log.Printf("[Agent] Tool result: %s", truncate(result, 200))
		a.memory.AddTool(fmt.Sprintf("Tool %s result: %s", planResult.Action, result))
		a.remember(memory.MemoryEvent{
			Type:   memory.EventToolResult,
			Action: planResult.Action,
			Params: planResult.Parameters,
			Result: result,
			Tags:   inferEventTags(planResult.Action, result),
		})

		// Phase 3: Format — turn raw tool output into natural language
		finalReply, formatErr := a.finalize(input, planResult.Action, result)
		if formatErr != nil {
			log.Printf("[Agent] Finalize failed: %v — falling back to raw result", formatErr)
			finalReply = result
		}

		a.memory.AddAssistant(finalReply)
		a.memory.CompactIfNeeded(a.llmClient)
		a.rememberAssistant(finalReply)
		return finalReply
	}

	if lastErr != nil {
		return fmt.Sprintf("Не удалось выполнить запрос после %d попыток: %v", maxLoopAttempts, lastErr)
	}
	return "Не удалось выполнить запрос."
}

// plan wraps the planner call with exponential backoff retry for transient LLM errors.
func (a *Agent) plan(before time.Time, input string) (planner.PlanResult, error) {
	var result planner.PlanResult
	var lastErr error

	err := retry.Do(retryCfg, func() error {
		var e error
		result, e = a.planner.Plan(a.planningMessages(before, input))
		lastErr = e
		return e
	})
	if err != nil {
		return planner.PlanResult{}, fmt.Errorf("plan: %w", lastErr)
	}
	return result, nil
}

func (a *Agent) planningMessages(before time.Time, input string) []llm.Message {
	if a.longTermMemory == nil {
		return a.memory.Messages
	}

	context, err := a.longTermMemory.BuildContext(memory.ContextRequest{
		SessionID: a.sessionID,
		Input:     input,
		Limit:     a.memoryContextLimit,
		Before:    before,
	})
	if err != nil {
		log.Printf("[Agent] Memory context unavailable: %v", err)
		return a.memory.Messages
	}
	if len(context) == 0 {
		return a.memory.Messages
	}

	messages := make([]llm.Message, 0, len(context)+len(a.memory.Messages))
	messages = append(messages, context...)
	messages = append(messages, a.memory.Messages...)
	return messages
}

func (a *Agent) finalize(input, action, toolResult string) (string, error) {
	messages := []llm.Message{
		{
			Role:    "system",
			Content: "You are an AI assistant. Use the tool result to answer the user naturally and concisely. Do not invent facts not present in the tool result.",
		},
		{
			Role:    "user",
			Content: fmt.Sprintf("User request: %s\nTool used: %s\nTool result: %s", input, action, toolResult),
		},
	}

	var reply string
	var lastErr error
	err := retry.Do(retryCfg, func() error {
		var e error
		reply, e = a.llmClient.Chat(messages)
		lastErr = e
		return e
	})
	if err != nil {
		return "", fmt.Errorf("finalize: %w", lastErr)
	}

	return reply, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func unknownObservation(params map[string]interface{}) string {
	if reason, ok := params["reason"].(string); ok && strings.TrimSpace(reason) != "" {
		return "no available action fits: " + strings.TrimSpace(reason)
	}
	return "no available action fits this request"
}
