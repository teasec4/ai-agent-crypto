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

type Session struct {
	harness *Harness
	memory  *memory.WorkMemory
}

func New(cfg *config.Config) *Harness {
	cryptoTool := tools.NewCryptoTool()


	reg := registry.New(cryptoTool)

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

func (h *Harness) NewSession() *Session {
	return &Session{
		harness: h,
		memory:  memory.NewDefaultWorkMemory(),
	}
}

func (h *Harness) Run(task string) HarnessExecutionResult {
	return h.NewSession().Run(task)
}

func (s *Session) Run(task string) HarnessExecutionResult {
	s.memory.AddUser(task)

	result := loop.RunLoop(loop.LoopRequest{
		Memory:    s.memory,
		Guardrail: s.harness.guardrail,
		Planner:   s.harness.planner,
		Executor:  s.harness.executor,
		LLMClient: s.harness.llmClient,
	})

	return HarnessExecutionResult{
		LoopResult: result,
		Task:       task,
	}
}

func (s *Session) Reset() {
	s.memory.Reset()
}
