package loop

import (
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"ai-agent/internal/guardrails"
	"ai-agent/internal/id"
	"ai-agent/internal/memory"
	"ai-agent/internal/planner"
)

const (
	DefaultMaxIterations = 5
)

func RunLoop(req LoopRequest) (res LoopResult) {
	logger := req.Logger
	if logger == nil {
		logger = slog.Default()
	}

	maxIter := req.MaxIterations
	if maxIter <= 0 {
		maxIter = DefaultMaxIterations
	}

	startTime := time.Now()
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			logger.Error("loop panic recovered",
				"panic", r,
				"stack", stack,
				"elapsed_ms", time.Since(startTime).Milliseconds(),
			)
			res = LoopResult{
				Answer:     fmt.Sprintf("Internal agent error: %v\n\n%s", r, stack),
				Iterations: 0,
				Trace:      []LoopIteration{},
				StoppedBy:  StoppedByError,
			}
		}
	}()

	trace := []LoopIteration{}

	for iterationIndex := 1; ; iterationIndex++ {
		if iterationIndex > maxIter {
			logger.Warn("loop stopped: max iterations reached",
				"iteration", iterationIndex-1,
				"limit", maxIter,
			)
			return LoopResult{
				Answer:     fmt.Sprintf("Достигнут лимит итераций (%d). Пожалуйста, уточните запрос.", maxIter),
				Iterations: len(trace),
				Trace:      trace,
				StoppedBy:  StoppedByGuardrail,
			}
		}

		if !req.Deadline.IsZero() && time.Now().After(req.Deadline) {
			logger.Warn("loop stopped: deadline exceeded",
				"iteration", iterationIndex-1,
				"elapsed_ms", time.Since(startTime).Milliseconds(),
			)
			return LoopResult{
				Answer:     fmt.Sprintf("Превышено время выполнения (%.0f сек). Пожалуйста, упростите запрос.", time.Since(startTime).Seconds()),
				Iterations: len(trace),
				Trace:      trace,
				StoppedBy:  StoppedByError,
			}
		}

		logger.Debug("loop iteration",
			"iteration", iterationIndex,
			"messages", req.Memory.Len(),
		)

		if req.Guardrail != nil {
			checkErr := req.Guardrail(guardrails.GuardrailInput{
				Iteration: iterationIndex,
				Messages:  req.Memory.Messages,
			})
			if checkErr != nil {
				logger.Warn("guardrail stopped loop",
					"iteration", iterationIndex,
					"error", checkErr.Error(),
					"messages", req.Memory.Len(),
				)
				return LoopResult{
					Answer:     checkErr.Error(),
					Iterations: len(trace),
					Trace:      trace,
					StoppedBy:  StoppedByGuardrail,
				}
			}
		}

		planStart := time.Now()
		planResult, err := req.Planner.Plan(req.Memory.Messages)
		planElapsed := time.Since(planStart).Milliseconds()

		if err != nil {
			logger.Error("planner failed",
				"iteration", iterationIndex,
				"error", err.Error(),
				"elapsed_ms", planElapsed,
			)
			trace = append(trace, LoopIteration{
				Index:       iterationIndex,
				Outcome:     OutcomeError,
				ContextSize: req.Memory.Len(),
			})
			return LoopResult{
				Answer:     fmt.Sprintf("Ошибка при построении плана: %v", err),
				Iterations: len(trace),
				Trace:      trace,
				StoppedBy:  StoppedByError,
			}
		}

		logger.Info("plan decided",
			"iteration", iterationIndex,
			"action", planResult.Action,
			"elapsed_ms", planElapsed,
		)

		switch planResult.Action {
		case planner.ActionMessage:
			logger.Info("loop finished with answer",
				"iterations", iterationIndex,
				"messages", req.Memory.Len(),
				"total_ms", time.Since(startTime).Milliseconds(),
			)
			req.Memory.AddAssistant(planResult.Reply)
			req.Memory.CompactIfNeeded(req.LLMClient)
			trace = append(trace, LoopIteration{
				Index:       iterationIndex,
				Outcome:     OutcomeAnswer,
				ContextSize: req.Memory.Len(),
			})
			return LoopResult{
				Answer:     planResult.Reply,
				Iterations: len(trace),
				Trace:      trace,
				StoppedBy:  StoppedBySuccess,
			}
		case planner.ActionUnknown:
			reason, _ := planResult.Parameters["reason"].(string)
			logger.Warn("loop finished with unknown",
				"iteration", iterationIndex,
				"reason", reason,
			)
			answer := unsupportedActionReply(planResult)
			req.Memory.AddAssistant(answer)
			req.Memory.CompactIfNeeded(req.LLMClient)
			trace = append(trace, LoopIteration{
				Index:       iterationIndex,
				Outcome:     OutcomeAnswer,
				ContextSize: req.Memory.Len(),
			})
			return LoopResult{
				Answer:     answer,
				Iterations: len(trace),
				Trace:      trace,
				StoppedBy:  StoppedByModel,
			}
		default:
			needsApproval := !req.AutoApprove && req.Executor.RequiresApproval(planResult)
			if needsApproval {
				pendingAction, err := req.Executor.PendingAction(id.New(), planResult)
				trace = append(trace, LoopIteration{
					Index:       iterationIndex,
					Outcome:     OutcomeToolCalls,
					ToolEvents:  []ToolEvent{{Tool: planResult.Action, Args: planResult.Parameters}},
					ContextSize: req.Memory.Len(),
				})
				if err != nil {
					logger.Error("failed to build pending action",
						"iteration", iterationIndex,
						"action", planResult.Action,
						"error", err.Error(),
					)
					return LoopResult{
						Answer:     fmt.Sprintf("Не удалось подготовить подтверждение: %v", err),
						Iterations: len(trace),
						Trace:      trace,
						StoppedBy:  StoppedByError,
					}
				}
				logger.Info("approval required",
					"iteration", iterationIndex,
					"action", planResult.Action,
					"risk", pendingAction.Risk,
					"summary", pendingAction.Summary,
				)
				return LoopResult{
					Answer:        fmt.Sprintf("Нужно подтверждение перед выполнением %s.\nЧто будет сделано: %s\n\n%s", planResult.Action, pendingAction.Summary, pendingAction.Preview),
					Iterations:    len(trace),
					Trace:         trace,
					StoppedBy:     StoppedByApproval,
					PendingAction: pendingAction,
				}
			}

			toolStart := time.Now()
			result, err := req.Executor.ExecuteWithWorkspace(planResult, req.Workspace)
			toolElapsed := time.Since(toolStart).Milliseconds()

			event := ToolEvent{
				Tool: planResult.Action,
				Args: planResult.Parameters,
			}
			if err != nil {
				event.Error = err.Error()
				req.Memory.AddTool(memory.FormatToolResult(planResult.Action, result, err, ""))
				logger.Warn("tool execution failed",
					"iteration", iterationIndex,
					"tool", planResult.Action,
					"elapsed_ms", toolElapsed,
					"error", err.Error(),
				)
			} else {
				event.Result = result
				req.Memory.AddTool(memory.FormatToolResult(planResult.Action, result, nil, ""))
				logger.Info("tool executed",
					"iteration", iterationIndex,
					"tool", planResult.Action,
					"elapsed_ms", toolElapsed,
					"result_bytes", len(result),
				)
			}
			req.Memory.CompactIfNeeded(req.LLMClient)
			trace = append(trace, LoopIteration{
				Index:       iterationIndex,
				Outcome:     OutcomeToolCalls,
				ToolEvents:  []ToolEvent{event},
				ContextSize: req.Memory.Len(),
			})
		}
	}
}

func unsupportedActionReply(planResult planner.PlanResult) string {
	if reason, ok := planResult.Parameters["reason"].(string); ok && reason != "" {
		return "Я не умею выполнить этот запрос доступными инструментами: " + reason
	}
	return "Я не умею выполнить этот запрос доступными инструментами."
}
