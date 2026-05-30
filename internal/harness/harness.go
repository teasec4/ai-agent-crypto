package harness

import (
	"ai-agent/internal/config"
	"ai-agent/internal/executor"
	"ai-agent/internal/guardrails"
	"ai-agent/internal/llm"
	"ai-agent/internal/loop"
	"ai-agent/internal/memory"
	"ai-agent/internal/planner"
	"ai-agent/internal/tools"
	"ai-agent/internal/tools/registry"
	"time"
)

type Harness struct {
	llmClient llm.LlmClient
	planner   *planner.LLMPlanner
	executor  *executor.ToolExecutor
	guardrail guardrails.GuardrailFn
}

type HarnessExecutionResult struct {
	LoopResult loop.LoopResult
	Task       string
}

func New(cfg *config.Config) *Harness {
	cryptoTool := tools.NewCryptoTool()
	cryptoTool.SetAPIKey(cfg.CoinGeckoApiKey)
	gitTool := tools.NewGitTool()
	helpTool := tools.NewHelpTool()

	reg := registry.New(cryptoTool, gitTool, helpTool)

	llmClient := llm.NewClientWithTimeout(
		cfg.OpenAIApiKey,
		cfg.LLMBaseURL,
		cfg.LLMModel,
		cfg.LLMTemperature,
		cfg.LLMMaxTokens,
		time.Duration(cfg.TimeoutSeconds)*time.Second,
	)

	return &Harness{
		llmClient: llmClient,
		planner:   planner.NewLLMPlanner(llmClient, reg),
		executor:  executor.New(reg),
		guardrail: guardrails.CombineGuardrails(
			guardrails.MaxIterations(loop.DefaultMaxIterations),
			guardrails.MaxMessages(loop.DefaultMaxMessages),
		),
	}
}

func (h *Harness) Run(task string) HarnessExecutionResult {
	workingMemory := memory.NewWorkMemory()
	workingMemory.CreateContext(task)

	result := loop.RunLoop(loop.LoopRequest{
		Memory:    workingMemory,
		Guardrail: h.guardrail,
		Planner:   h.planner,
		Executor:  h.executor,
		LLMClient: h.llmClient,
	})

	return HarnessExecutionResult{
		LoopResult: result,
		Task:       task,
	}
}
