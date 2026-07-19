package harness

import (
	"context"
	"log/slog"
	"os"
	"time"

	"ai-agent/internal/config"
	"ai-agent/internal/executor"
	"ai-agent/internal/guardrails"
	"ai-agent/internal/llm"
	"ai-agent/internal/loop"
	"ai-agent/internal/memory"
	"ai-agent/internal/planner"
	"ai-agent/internal/tools"
	"ai-agent/internal/tools/registry"
)

const defaultLoopTimeout = 3 * time.Minute

type Harness struct {
	llmClient   llm.LlmClient
	planner     *planner.LLMPlanner
	executor    *executor.ToolExecutor
	guardrail   guardrails.GuardrailFn
	autoApprove bool
	logger      *slog.Logger
}

type HarnessExecutionResult struct {
	LoopResult loop.LoopResult
	Task       string
}

func New(cfg *config.Config) *Harness {
	logger := newLogger(cfg)

	cryptoTool := tools.NewCryptoTool()
	cryptoTool.SetAPIKey(cfg.CoinGeckoApiKey)

	reg := registry.New(
		cryptoTool,
		tools.NewGitTool(),
		tools.NewListDirectoryTool(),
		tools.NewReadFileTool(),
		tools.NewFindFilesTool(),
		tools.NewSearchTextTool(),
		tools.NewReadProjectMemoryTool(),
		tools.NewProposeMemoryUpdateTool(),
		tools.NewCreateDirectoryTool(),
		tools.NewWriteFileTool(),
		tools.NewEditFileTool(),
		tools.NewCommandTool(),
	)

	llmClient := llm.NewClientWithTimeout(
		cfg.OpenAIApiKey,
		cfg.LLMBaseURL,
		cfg.LLMModel,
		cfg.LLMTemperature,
		cfg.LLMMaxTokens,
		time.Duration(cfg.TimeoutSeconds)*time.Second,
	)

	pl := planner.NewLLMPlanner(llmClient, reg)
	pl.SetLogger(logger)

	ex := executor.New(reg)
	ex.SetLogger(logger)

	logger.Info("harness initialized",
		"model", cfg.LLMModel,
		"tools", reg.Count(),
	)

	return &Harness{
		llmClient: llmClient,
		planner:   pl,
		executor:  ex,
		guardrail: guardrails.CombineGuardrails(
			guardrails.MaxIterations(loop.DefaultMaxIterations),
		),
		autoApprove: cfg.AllowAutoApprove,
		logger:      logger,
	}
}

func (h *Harness) buildLoopRequest(ctx context.Context, workMemory *memory.WorkMemory, workspace string) loop.LoopRequest {
	if ctx == nil {
		ctx = context.Background()
	}
	return loop.LoopRequest{
		Context:       ctx,
		Memory:        workMemory,
		Guardrail:     h.guardrail,
		Planner:       h.planner,
		Executor:      h.executor,
		LLMClient:     h.llmClient,
		AutoApprove:   h.autoApprove,
		Logger:        h.logger,
		Workspace:     workspace,
		MaxIterations: loop.DefaultMaxIterations,
		Deadline:      time.Now().Add(defaultLoopTimeout),
	}
}

// RunWithMemory runs the agent loop with the configured approval policy.
// Used by the REST /ask endpoint.
func (h *Harness) RunWithMemory(ctx context.Context, task string, workMemory *memory.WorkMemory, workspace string) HarnessExecutionResult {
	h.logger.Info("processing user message",
		"task_bytes", len(task),
		"memory_messages", workMemory.Len(),
		"workspace", workspace,
	)

	workMemory.AddUser(task)

	req := h.buildLoopRequest(ctx, workMemory, workspace)
	result := loop.RunLoop(req)

	h.logger.Info("loop finished",
		"stopped_by", result.StoppedBy,
		"iterations", result.Iterations,
		"answer_bytes", len(result.Answer),
	)

	return HarnessExecutionResult{
		LoopResult: result,
		Task:       task,
	}
}

// RunWithMemoryStreaming runs the agent loop with SSE streaming and optional approval callback.
// onEvent receives lifecycle events (thinking, tool_start, tool_done, etc).
// onApproval is called when a tool requires user confirmation; it may block.
func (h *Harness) RunWithMemoryStreaming(
	ctx context.Context,
	task string,
	workMemory *memory.WorkMemory,
	workspace string,
	onEvent func(loop.SSEEvent),
	onApproval loop.ApprovalFn,
) HarnessExecutionResult {
	h.logger.Info("processing user message (streaming)",
		"task_bytes", len(task),
		"memory_messages", workMemory.Len(),
		"workspace", workspace,
	)

	workMemory.AddUser(task)

	req := h.buildLoopRequest(ctx, workMemory, workspace)
	req.AutoApprove = false
	req.OnEvent = onEvent
	req.OnApproval = onApproval
	result := loop.RunLoop(req)

	h.logger.Info("streaming loop finished",
		"stopped_by", result.StoppedBy,
		"iterations", result.Iterations,
		"answer_bytes", len(result.Answer),
	)

	return HarnessExecutionResult{
		LoopResult: result,
		Task:       task,
	}
}

func newLogger(cfg *config.Config) *slog.Logger {
	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}
