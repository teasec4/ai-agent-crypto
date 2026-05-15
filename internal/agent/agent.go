package agent

import (
	"fmt"
	"log"
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

type Agent struct {
	llmClient llm.LlmClient
	planner   *planner.LLMPlanner
	executor  executor.Executor
	memory    *memory.WorkMemory
}

func NewAgent(
	llmClient llm.LlmClient,
	reg *registry.Registry,
) *Agent {
	return &Agent{
		llmClient: llmClient,
		planner:   planner.NewLLMPlanner(llmClient, reg),
		executor:  executor.New(reg),
		memory:    memory.NewWorkMemory(),
	}
}

// NewWithConfig creates a fully wired Agent from config.
// This is the single place for wiring — use it from all entry points.
func NewWithConfig(cfg *config.Config) *Agent {
	cryptoTool := tools.NewCryptoTool()
	cryptoTool.SetAPIKey(cfg.CoinGeckoApiKey)
	gitTool := tools.NewGitTool()
	helpTool := tools.NewHelpTool()
	unknownTool := tools.NewUnknownTool()

	llmClient := llm.NewClientWithTimeout(
		cfg.OpenAIApiKey,
		cfg.LLMBaseURL,
		cfg.LLMModel,
		cfg.LLMTemperature,
		cfg.LLMMaxTokens,
		time.Duration(cfg.TimeoutSeconds)*time.Second,
	)

	reg := registry.New(cryptoTool, gitTool, helpTool, unknownTool)
	return NewAgent(llmClient, reg)
}

func (a *Agent) Run(input string) string {
	log.Printf("[Agent] Planning...")
	a.memory.AddUser(input)

	var lastErr error
	for attempt := 1; attempt <= maxLoopAttempts; attempt++ {
		// Phase 1: Plan — with retry for transient LLM errors.
		// Retryable: network, timeout, 429, 5xx.
		// Non-retryable: bad JSON, validation — added to history, loop retries with context.
		planResult, err := a.plan(attempt)
		if err != nil {
			lastErr = err
			if retry.IsFatal(err) {
				return fmt.Sprintf("Ошибка: %v", err)
			}
			// Feed error back into history so the planner can adapt on next iteration
			a.memory.AddTool(fmt.Sprintf("Plan attempt %d failed: %v", attempt, err))
			log.Printf("[Agent] Plan attempt %d/%d failed (retryable): %v", attempt, maxLoopAttempts, err)
			continue
		}

		// Direct answer path: planner chose "message" action
		if planResult.Action == "message" {
			a.memory.AddAssistant(planResult.Reply)
			a.memory.CompactIfNeeded(a.llmClient)
			return planResult.Reply
		}

		// Phase 2: Act — execute the chosen tool (no retry, errors go to history)
		log.Printf("[Agent] Executing tool: %s", planResult.Action)
		result, toolErr := a.executor.Execute(planResult)
		if toolErr != nil {
			lastErr = toolErr
			a.memory.AddTool(fmt.Sprintf("Tool %s failed: %v", planResult.Action, toolErr))
			log.Printf("[Agent] Tool error: %v", toolErr)
			continue
		}

		log.Printf("[Agent] Tool result: %s", truncate(result, 200))
		a.memory.AddTool(fmt.Sprintf("Tool %s result: %s", planResult.Action, result))

		// Phase 3: Format — turn raw tool output into natural language
		finalReply, formatErr := a.finalize(input, planResult.Action, result)
		if formatErr != nil {
			log.Printf("[Agent] Finalize failed: %v — falling back to raw result", formatErr)
			finalReply = result
		}

		a.memory.AddAssistant(finalReply)
		a.memory.CompactIfNeeded(a.llmClient)
		return finalReply
	}

	if lastErr != nil {
		return fmt.Sprintf("Не удалось выполнить запрос после %d попыток: %v", maxLoopAttempts, lastErr)
	}
	return "Не удалось выполнить запрос."
}

// plan wraps the planner call with exponential backoff retry for transient LLM errors.
func (a *Agent) plan(attempt int) (planner.PlanResult, error) {
	var result planner.PlanResult
	var lastErr error

	err := retry.Do(retryCfg, func() error {
		var e error
		result, e = a.planner.Plan("", a.memory.Messages)
		lastErr = e
		return e
	})
	if err != nil {
		return planner.PlanResult{}, fmt.Errorf("plan: %w", lastErr)
	}
	return result, nil
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

	return a.llmClient.Chat(messages)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
