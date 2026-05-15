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
	"ai-agent/internal/tools"
	"ai-agent/internal/tools/registry"
)

const maxPlanAttempts = 3

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
	for attempt := 1; attempt <= maxPlanAttempts; attempt++ {
		planResult, err := a.planner.Plan("", a.memory.Messages)
		if err != nil {
			lastErr = err
			log.Printf("[Agent] Plan attempt %d failed: %v", attempt, err)
			continue
		}

		if planResult.Action == "message" {
			a.memory.AddAssistant(planResult.Reply)
			a.memory.CompactIfNeeded(a.llmClient)
			return planResult.Reply
		}

		log.Printf("[Agent] Executing tool: %s", planResult.Action)
		result, err := a.executor.Execute(planResult)
		if err != nil {
			lastErr = err
			a.memory.AddTool(fmt.Sprintf("Tool %s failed: %v", planResult.Action, err))
			log.Printf("[Agent] Tool error: %v", err)
			continue
		}

		log.Printf("[Agent] Tool result: %s", truncate(result, 200))
		a.memory.AddTool(fmt.Sprintf("Tool %s result: %s", planResult.Action, result))

		finalReply, err := a.finalize(input, planResult.Action, result)
		if err != nil {
			lastErr = err
			log.Printf("[Agent] Finalize failed: %v", err)
			finalReply = result
		}

		a.memory.AddAssistant(finalReply)
		a.memory.CompactIfNeeded(a.llmClient)
		return finalReply
	}

	if lastErr != nil {
		return fmt.Sprintf("Не удалось выполнить запрос: %v", lastErr)
	}
	return "Не удалось выполнить запрос."
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
