package harness

import (
	"context"
	"log/slog"
	"os"
	"time"

	"ai-agent/internal/config"
	"ai-agent/internal/executor"
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
	autoApprove bool
	logger      *slog.Logger
}

type RunRequest struct {
	Task            string
	Memory          *memory.WorkMemory
	Workspace       string
	RequireApproval bool
	OnEvent         func(loop.SSEEvent)
	OnApproval      loop.ApprovalFn
}

type HarnessExecutionResult struct {
	LoopResult loop.LoopResult
	Task       string
}

func New(cfg *config.Config) *Harness {
	logger := newLogger(cfg)

	// нам не нужен ключ тут
	// cryptoTool.SetAPIKey(cfg.CoinGeckoApiKey)

	reg := registry.New(
		tools.NewCryptoTool(),
		tools.NewGitTool(),
		tools.NewListDirectoryTool(),
		tools.NewReadFileTool(),
		tools.NewFindFilesTool(),
		tools.NewSearchTextTool(),
		tools.NewReadProjectMemoryTool(),
		tools.NewProposeMemoryUpdateTool(),
		tools.NewCreateDirectoryTool(),
		tools.NewDeletePathTool(),
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
		llmClient:   llmClient,
		planner:     pl,
		executor:    ex,
		autoApprove: cfg.AllowAutoApprove,
		logger:      logger,
	}
}

func (h *Harness) buildLoopRequest(ctx context.Context, req RunRequest) loop.LoopRequest {
	if ctx == nil {
		ctx = context.Background()
	}
	return loop.LoopRequest{
		Context:       ctx,
		Memory:        req.Memory,
		Planner:       h.planner,
		Executor:      h.executor,
		AutoApprove:   h.autoApprove && !req.RequireApproval,
		Logger:        h.logger,
		Workspace:     req.Workspace,
		MaxIterations: loop.DefaultMaxIterations,
		Deadline:      time.Now().Add(defaultLoopTimeout),
		OnEvent:       req.OnEvent,
		OnApproval:    req.OnApproval,
		CompactMemory: func(ctx context.Context) {
			req.Memory.CompactIfNeeded(ctx, h.llmClient)
		},
	}
}

func (h *Harness) Run(ctx context.Context, req RunRequest) HarnessExecutionResult {
	h.logger.Info("processing user message",
		"task_bytes", len(req.Task),
		"memory_messages", req.Memory.Len(),
		"workspace", req.Workspace,
		"require_approval", req.RequireApproval,
		"streaming", req.OnEvent != nil,
	)

	h.closeDanglingUserTurn(req.Memory)
	req.Memory.AddUser(req.Task)

	loopReq := h.buildLoopRequest(ctx, req)
	result := loop.RunLoop(loopReq)

	h.logger.Info("loop finished",
		"stopped_by", result.StoppedBy,
		"iterations", result.Iterations,
		"answer_bytes", len(result.Answer),
	)

	return HarnessExecutionResult{
		LoopResult: result,
		Task:       req.Task,
	}
}

func (h *Harness) closeDanglingUserTurn(workMemory *memory.WorkMemory) {
	if workMemory == nil || len(workMemory.Messages) == 0 {
		return
	}
	last := workMemory.Messages[len(workMemory.Messages)-1]
	if last.Role != memory.RoleUser {
		return
	}
	h.logger.Warn("closing dangling user turn before starting a new task")
	workMemory.AddAssistant("Предыдущий запрос был остановлен без финального ответа. Перехожу к следующему запросу.")
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
