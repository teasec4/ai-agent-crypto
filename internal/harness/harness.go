package harness

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"ai-agent/internal/approval"
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

type AgentSession struct {
	harness       *Harness
	memory        *memory.WorkMemory
	pendingAction *approval.PendingAction
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
		logger: logger,
	}
}

func (h *Harness) NewAgentSession() *AgentSession {
	return &AgentSession{
		harness: h,
		memory:  memory.NewDefaultWorkMemory(),
	}
}

func (h *Harness) buildLoopRequest(workMemory *memory.WorkMemory, workspace string) loop.LoopRequest {
	return loop.LoopRequest{
		Memory:      workMemory,
		Guardrail:   h.guardrail,
		Planner:     h.planner,
		Executor:    h.executor,
		LLMClient:   h.llmClient,
		AutoApprove: h.autoApprove,
		Logger:      h.logger,
		Workspace:   workspace,
	}
}

func (h *Harness) RunWithMemory(task string, workMemory *memory.WorkMemory, workspace string) HarnessExecutionResult {
	h.logger.Info("processing user message",
		"task_bytes", len(task),
		"memory_messages", workMemory.Len(),
		"workspace", workspace,
	)

	workMemory.AddUser(task)

	result := loop.RunLoop(h.buildLoopRequest(workMemory, workspace))

	h.logger.Info("loop finished",
		"stopped_by", result.StoppedBy,
		"iterations", result.Iterations,
		"answer_bytes", len(result.Answer),
		"pending_action", result.PendingAction != nil,
	)

	return HarnessExecutionResult{
		LoopResult: result,
		Task:       task,
	}
}

func (s *AgentSession) Run(task string) HarnessExecutionResult {
	result := s.harness.RunWithMemory(task, s.memory, "")
	s.pendingAction = result.LoopResult.PendingAction
	return result
}

func (s *AgentSession) PendingAction() *approval.PendingAction {
	return s.pendingAction
}

func (s *AgentSession) Approve() HarnessExecutionResult {
	if s.pendingAction == nil {
		return HarnessExecutionResult{
			LoopResult: loop.LoopResult{
				Answer:    "Нет ожидающего действия для подтверждения.",
				StoppedBy: loop.StoppedByError,
			},
		}
	}
	action := s.pendingAction
	s.pendingAction = nil
	result := s.harness.ContinueWithApprovedAction(action, s.memory, "")
	s.pendingAction = result.LoopResult.PendingAction
	return result
}

func (s *AgentSession) Reject() HarnessExecutionResult {
	if s.pendingAction == nil {
		return HarnessExecutionResult{
			LoopResult: loop.LoopResult{
				Answer:    "Нет ожидающего действия для отклонения.",
				StoppedBy: loop.StoppedByError,
			},
		}
	}
	action := s.pendingAction
	s.pendingAction = nil
	return s.harness.RecordRejectedAction(action, s.memory)
}

func (h *Harness) ContinueWithApprovedAction(action *approval.PendingAction, workMemory *memory.WorkMemory, workspace string) HarnessExecutionResult {
	h.logger.Info("continuing with approved action",
		"tool", action.Tool,
		"approval_id", action.ID,
	)

	if h.guardrail != nil {
		if err := h.guardrail(guardrails.GuardrailInput{
			Iteration: 0,
			Messages:  workMemory.Messages,
		}); err != nil {
			h.logger.Warn("guardrail blocked approved action",
				"tool", action.Tool,
				"error", err.Error(),
			)
			return HarnessExecutionResult{
				LoopResult: loop.LoopResult{
					Answer:    fmt.Sprintf("Действие %s заблокировано: %v", action.Tool, err),
					Trace:     []loop.LoopIteration{},
					StoppedBy: loop.StoppedByGuardrail,
				},
				Task: fmt.Sprintf("blocked:%s", action.ID),
			}
		}
	}

	initialTrace := loop.LoopIteration{
		Index:   1,
		Outcome: loop.OutcomeToolCalls,
		ToolEvents: []loop.ToolEvent{{
			Tool: action.Tool,
			Args: action.Args,
		}},
		ContextSize: workMemory.Len(),
	}

	result, err := h.executor.ExecuteWithWorkspace(planner.PlanResult{
		Action:     action.Tool,
		Parameters: action.Args,
	}, workspace)
	if err != nil {
		initialTrace.ToolEvents[0].Error = err.Error()
		workMemory.AddTool(memory.FormatToolResult(action.Tool, result, err, "Approved "))
	} else {
		initialTrace.ToolEvents[0].Result = result
		workMemory.AddTool(memory.FormatToolResult(action.Tool, result, nil, "Approved "))
	}
	workMemory.CompactIfNeeded(h.llmClient)

	loopResult := loop.RunLoop(h.buildLoopRequest(workMemory, workspace))

	initialTrace.ContextSize = workMemory.Len()
	loopResult.Trace = prependApprovalTrace(initialTrace, loopResult.Trace)
	loopResult.Iterations = len(loopResult.Trace)

	return HarnessExecutionResult{
		LoopResult: loopResult,
		Task:       fmt.Sprintf("approved:%s", action.ID),
	}
}

func (h *Harness) RecordRejectedAction(action *approval.PendingAction, workMemory *memory.WorkMemory) HarnessExecutionResult {
	h.logger.Info("action rejected by user",
		"tool", action.Tool,
		"approval_id", action.ID,
	)
	answer := fmt.Sprintf("Действие %s отклонено пользователем.", action.Tool)
	workMemory.AddAssistant(answer)
	return HarnessExecutionResult{
		LoopResult: loop.LoopResult{
			Answer:     answer,
			Iterations: 0,
			Trace:      []loop.LoopIteration{},
			StoppedBy:  loop.StoppedByModel,
		},
		Task: fmt.Sprintf("rejected:%s", action.ID),
	}
}

func prependApprovalTrace(first loop.LoopIteration, trace []loop.LoopIteration) []loop.LoopIteration {
	result := make([]loop.LoopIteration, 0, 1+len(trace))
	first.Index = 1
	result = append(result, first)
	for _, item := range trace {
		item.Index++
		result = append(result, item)
	}
	return result
}

func (s *AgentSession) Reset() {
	s.memory.Reset()
	s.pendingAction = nil
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
