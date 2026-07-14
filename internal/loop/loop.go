package loop

import (
	"context"
	"encoding/json"
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
				Answer:     fmt.Sprintf("Internal agent error: %v", r),
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
			answer := fmt.Sprintf("Достигнут лимит итераций (%d). Пожалуйста, уточните запрос.", maxIter)
			emit(req, SSEEvent{Type: EventDone, Answer: answer})
			return LoopResult{
				Answer:     answer,
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
			answer := fmt.Sprintf("Превышено время выполнения (%.0f сек). Пожалуйста, упростите запрос.", time.Since(startTime).Seconds())
			emit(req, SSEEvent{Type: EventDone, Answer: answer})
			return LoopResult{
				Answer:     answer,
				Iterations: len(trace),
				Trace:      trace,
				StoppedBy:  StoppedByError,
			}
		}

		logger.Debug("loop iteration",
			"iteration", iterationIndex,
			"messages", req.Memory.Len(),
		)

		// ---- Guardrail check ----
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
				emit(req, SSEEvent{Type: EventDone, Answer: checkErr.Error()})
				return LoopResult{
					Answer:     checkErr.Error(),
					Iterations: len(trace),
					Trace:      trace,
					StoppedBy:  StoppedByGuardrail,
				}
			}
		}

		emit(req, SSEEvent{Type: EventThinking})

		// ---- Plan step ----
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
			answer := fmt.Sprintf("Ошибка при построении плана: %v", err)
			emit(req, SSEEvent{Type: EventDone, Answer: answer})
			return LoopResult{
				Answer:     answer,
				Iterations: len(trace),
				Trace:      trace,
				StoppedBy:  StoppedByError,
			}
		}

		// ---- Handle plan result ----
		switch {
		case planResult.Action == planner.ActionMessage:
			logger.Info("loop finished with answer",
				"iterations", iterationIndex,
				"messages", req.Memory.Len(),
				"reply_len", len(planResult.Reply),
				"total_ms", time.Since(startTime).Milliseconds(),
			)
			req.Memory.AddAssistant(planResult.Reply)
			req.Memory.CompactIfNeeded(req.LLMClient)
			trace = append(trace, LoopIteration{
				Index:       iterationIndex,
				Outcome:     OutcomeAnswer,
				ContextSize: req.Memory.Len(),
			})
			emit(req, SSEEvent{Type: EventDone, Answer: planResult.Reply})
			return LoopResult{
				Answer:     planResult.Reply,
				Iterations: len(trace),
				Trace:      trace,
				StoppedBy:  StoppedBySuccess,
			}

		case planResult.Action == planner.ActionUnknown:
			reason, _ := planResult.Parameters["reason"].(string)
			logger.Warn("loop finished with unknown",
				"iteration", iterationIndex,
				"reason", reason,
			)
			answer := unsupportedActionReply(reason)
			req.Memory.AddAssistant(answer)
			req.Memory.CompactIfNeeded(req.LLMClient)
			trace = append(trace, LoopIteration{
				Index:       iterationIndex,
				Outcome:     OutcomeAnswer,
				ContextSize: req.Memory.Len(),
			})
			emit(req, SSEEvent{Type: EventDone, Answer: answer})
			return LoopResult{
				Answer:     answer,
				Iterations: len(trace),
				Trace:      trace,
				StoppedBy:  StoppedByModel,
			}

		default:
			// ---- Tool execution path (native tool calling) ----
			logger.Info("tool call from LLM",
				"iteration", iterationIndex,
				"tool", planResult.Action,
				"tool_call_id", planResult.ToolCallID,
			)

			// Check if the tool needs approval
			needsApproval := !req.AutoApprove && req.Executor.RequiresApproval(planResult)

			if needsApproval {
				pendingAction, err := req.Executor.PendingAction(id.New(), planResult)
				if err != nil {
					logger.Error("failed to build pending action",
						"iteration", iterationIndex,
						"action", planResult.Action,
						"error", err.Error(),
					)
					answer := fmt.Sprintf("Не удалось подготовить подтверждение: %v", err)
					emit(req, SSEEvent{Type: EventDone, Answer: answer})
					return LoopResult{
						Answer:     answer,
						Iterations: len(trace),
						Trace:      trace,
						StoppedBy:  StoppedByError,
					}
				}

				// Legacy: no callback → return PendingAction
				if req.OnApproval == nil {
					emit(req, SSEEvent{
						Type:   EventApprovalRequired,
						Tool:   planResult.Action,
						Args:   planResult.Parameters,
						Action: pendingAction,
					})
					logger.Info("approval required (legacy stop)",
						"iteration", iterationIndex,
						"action", planResult.Action,
						"risk", pendingAction.Risk,
					)
					trace = append(trace, LoopIteration{
						Index:       iterationIndex,
						Outcome:     OutcomeToolCalls,
						ToolEvents:  []ToolEvent{{Tool: planResult.Action, Args: planResult.Parameters}},
						ContextSize: req.Memory.Len(),
					})
					return LoopResult{
						Answer:        fmt.Sprintf("Нужно подтверждение перед выполнением %s.\nЧто будет сделано: %s\n\n%s", planResult.Action, pendingAction.Summary, pendingAction.Preview),
						Iterations:    len(trace),
						Trace:         trace,
						StoppedBy:     StoppedByApproval,
						PendingAction: pendingAction,
					}
				}

				// Callback path (SSE streaming) — wait with context+timeout
				logger.Info("approval required (callback)",
					"iteration", iterationIndex,
					"tool", planResult.Action,
					"risk", pendingAction.Risk,
				)

				// Build an approval channel from the callback
				type approvalResult struct {
					approved bool
				}
				ch := make(chan approvalResult, 1)
				go func() {
					ch <- approvalResult{approved: req.OnApproval(pendingAction)}
				}()

				var approved bool
				select {
				case res := <-ch:
					approved = res.approved
				case <-req.Context.Done():
					logger.Warn("approval cancelled by context",
						"iteration", iterationIndex,
						"tool", planResult.Action,
						"error", req.Context.Err(),
					)
					approved = false
				case <-time.After(5 * time.Minute):
					logger.Warn("approval timed out",
						"iteration", iterationIndex,
						"tool", planResult.Action,
					)
					approved = false
				}

				if !approved {
					logger.Info("user rejected action",
						"iteration", iterationIndex,
						"tool", planResult.Action,
					)
					// Add assistant tool call + tool result (rejected) so the LLM understands
					toolCallID := planResult.ToolCallID
					if toolCallID == "" {
						toolCallID = id.New()
					}
					argsJSON, _ := json.Marshal(planResult.Parameters)
					req.Memory.AddAssistantToolCall(toolCallID, planResult.Action, string(argsJSON))
					req.Memory.AddToolResult(toolCallID, fmt.Sprintf("Error: action rejected by user"))
					req.Memory.AddAssistant(fmt.Sprintf("Действие %s отклонено.", planResult.Action))
					req.Memory.CompactIfNeeded(req.LLMClient)
					trace = append(trace, LoopIteration{
						Index:       iterationIndex,
						Outcome:     OutcomeToolCalls,
						ToolEvents:  []ToolEvent{{Tool: planResult.Action, Args: planResult.Parameters, Error: "rejected by user"}},
						ContextSize: req.Memory.Len(),
					})
					continue
				}

				logger.Info("user approved action",
					"iteration", iterationIndex,
					"tool", planResult.Action,
				)
			}

			// ---- Execute the tool ----
			emit(req, SSEEvent{
				Type: EventToolStart,
				Tool: planResult.Action,
				Args: planResult.Parameters,
			})

			// Generate a tool_call_id if the LLM didn't provide one (legacy path)
			toolCallID := planResult.ToolCallID
			if toolCallID == "" {
				toolCallID = id.New()
			}

			// Add assistant's tool_call message to memory (for native tool calling)
			argsJSON, _ := json.Marshal(planResult.Parameters)
			req.Memory.AddAssistantToolCall(toolCallID, planResult.Action, string(argsJSON))

			toolStart := time.Now()
			var ctx context.Context = req.Context
			if ctx == nil {
				ctx = context.Background()
			}
			result, err := req.Executor.ExecuteWithWorkspace(ctx, planResult, req.Workspace)
			toolElapsed := time.Since(toolStart).Milliseconds()

			event := ToolEvent{
				Tool: planResult.Action,
				Args: planResult.Parameters,
			}
			if err != nil {
				event.Error = err.Error()
				req.Memory.AddToolResult(toolCallID, memory.FormatToolResult(planResult.Action, result, err, ""))
				logger.Warn("tool execution failed",
					"iteration", iterationIndex,
					"tool", planResult.Action,
					"tool_call_id", toolCallID,
					"elapsed_ms", toolElapsed,
					"error", err.Error(),
				)
				emit(req, SSEEvent{
					Type:   EventToolError,
					Tool:   planResult.Action,
					Args:   planResult.Parameters,
					Error:  err.Error(),
					Result: result,
				})
			} else {
				event.Result = result
				req.Memory.AddToolResult(toolCallID, fmt.Sprintf("Tool %s result:\n%s", planResult.Action, result))
				logger.Info("tool executed successfully",
					"iteration", iterationIndex,
					"tool", planResult.Action,
					"tool_call_id", toolCallID,
					"elapsed_ms", toolElapsed,
					"result_bytes", len(result),
				)
				emit(req, SSEEvent{
					Type:   EventToolDone,
					Tool:   planResult.Action,
					Args:   planResult.Parameters,
					Result: result,
				})
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

func emit(req LoopRequest, event SSEEvent) {
	if req.OnEvent != nil {
		req.OnEvent(event)
	}
}

func unsupportedActionReply(reason string) string {
	if reason != "" {
		return "Я не умею выполнить этот запрос доступными инструментами: " + reason
	}
	return "Я не умею выполнить этот запрос доступными инструментами."
}
