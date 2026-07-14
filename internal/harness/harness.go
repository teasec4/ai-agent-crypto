package harness

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
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

type AgentSession struct {
	harness       *Harness
	memory        *memory.WorkMemory
	workspace     string
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

// RunWithMemory runs the agent loop with auto-approval for all actions.
// Used by the /ask REST endpoint and the legacy AgentSession.
func (h *Harness) RunWithMemory(task string, workMemory *memory.WorkMemory, workspace string) HarnessExecutionResult {
	h.logger.Info("processing user message",
		"task_bytes", len(task),
		"memory_messages", workMemory.Len(),
		"workspace", workspace,
	)

	workMemory.AddUser(task)

	req := h.buildLoopRequest(workMemory, workspace)
	req.AutoApprove = true // auto-approve for REST /ask endpoint
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

// RunWithMemoryStreaming runs the agent loop with SSE streaming and approval callback.
func (h *Harness) RunWithMemoryStreaming(
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

	req := h.buildLoopRequest(workMemory, workspace)
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

// ---- AgentSession (CLI) ----

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
	return s.harness.continueWithApprovedAction(action, s.memory, s.workspace)
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
	return s.harness.recordRejectedAction(action, s.memory)
}

// ---- continuation (for REST /approvals/{id}) ----

const (
	approvalContinuationLimit = 2
	approvalContinuationTO    = 1 * time.Minute
)

func (h *Harness) ContinueWithApprovedAction(action *approval.PendingAction, workMemory *memory.WorkMemory, workspace string) HarnessExecutionResult {
	return h.continueWithApprovedAction(action, workMemory, workspace)
}

func (h *Harness) continueWithApprovedAction(action *approval.PendingAction, workMemory *memory.WorkMemory, workspace string) HarnessExecutionResult {
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

	// Use native tool calling format for the result message
	toolCallID := "approve_" + action.ID
	argsJSON, _ := json.Marshal(action.Args)
	workMemory.AddAssistantToolCall(toolCallID, action.Tool, string(argsJSON))

	if err != nil {
		initialTrace.ToolEvents[0].Error = err.Error()
		workMemory.AddToolResult(toolCallID, fmt.Sprintf("Tool %s error:\n%s%v", action.Tool, result, err))
	} else {
		initialTrace.ToolEvents[0].Result = result
		workMemory.AddToolResult(toolCallID, fmt.Sprintf("Tool %s result:\n%s", action.Tool, result))
	}
	workMemory.CompactIfNeeded(h.llmClient)

	loopReq := h.buildLoopRequest(workMemory, workspace)
	loopReq.MaxIterations = approvalContinuationLimit
	loopReq.Deadline = time.Now().Add(approvalContinuationTO)
	loopReq.AutoApprove = true
	loopResult := loop.RunLoop(loopReq)

	if loopResult.StoppedBy == loop.StoppedByError && containsTimeExceeded(loopResult.Answer) {
		toolResult := initialTrace.ToolEvents[0]
		if toolResult.Error != "" {
			loopResult.Answer = fmt.Sprintf("%s — ошибка: %s\n\n%s", action.Tool, toolResult.Error, loopResult.Answer)
		} else {
			loopResult.Answer = fmt.Sprintf("%s выполнен успешно.\n\n%s", action.Tool, loopResult.Answer)
		}
	}

	initialTrace.ContextSize = workMemory.Len()
	loopResult.Trace = prependApprovalTrace(initialTrace, loopResult.Trace)
	loopResult.Iterations = len(loopResult.Trace)

	return HarnessExecutionResult{
		LoopResult: loopResult,
		Task:       fmt.Sprintf("approved:%s", action.ID),
	}
}

func (h *Harness) RecordRejectedAction(action *approval.PendingAction, workMemory *memory.WorkMemory) HarnessExecutionResult {
	return h.recordRejectedAction(action, workMemory)
}

func (h *Harness) recordRejectedAction(action *approval.PendingAction, workMemory *memory.WorkMemory) HarnessExecutionResult {
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

func containsTimeExceeded(s string) bool {
	return strings.Contains(s, "Превышено время выполнения")
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
