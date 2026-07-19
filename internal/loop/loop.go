package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

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
	if req.Context == nil {
		req.Context = context.Background()
	}
	if !req.Deadline.IsZero() {
		ctx, cancel := context.WithDeadline(req.Context, req.Deadline)
		defer cancel()
		req.Context = ctx
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
		if err := req.Context.Err(); err != nil {
			logger.Warn("loop stopped: context cancelled",
				"iteration", iterationIndex-1,
				"error", err,
			)
			answer := fmt.Sprintf("Выполнение остановлено: %v", err)
			req.Memory.AddAssistant(answer)
			compactMemory(req)
			emit(req, SSEEvent{Type: EventDone, Answer: answer})
			return LoopResult{
				Answer:     answer,
				Iterations: len(trace),
				Trace:      trace,
				StoppedBy:  StoppedByError,
			}
		}

		if iterationIndex > maxIter {
			logger.Warn("loop stopped: max iterations reached",
				"iteration", iterationIndex-1,
				"limit", maxIter,
			)
			answer := fmt.Sprintf("Достигнут лимит итераций (%d). Пожалуйста, уточните запрос.", maxIter)
			req.Memory.AddAssistant(answer)
			compactMemory(req)
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
			req.Memory.AddAssistant(answer)
			compactMemory(req)
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

		emit(req, SSEEvent{Type: EventThinking})

		// ---- Plan step ----
		planStart := time.Now()
		planResult, err := req.Planner.Plan(req.Context, req.Memory.Messages, req.Workspace)
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
			req.Memory.AddAssistant(answer)
			compactMemory(req)
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
			compactMemory(req)
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
			compactMemory(req)
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

			planResult = repairFileWritePlan(req, planResult, logger)

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
					toolCallID := planResult.ToolCallID
					if toolCallID == "" {
						toolCallID = id.New()
					}
					argsJSON, _ := json.Marshal(planResult.Parameters)
					req.Memory.AddAssistantToolCall(toolCallID, planResult.Action, string(argsJSON))
					req.Memory.AddToolResult(toolCallID, fmt.Sprintf("Tool %s parameters are invalid: %v. Choose valid parameters or a different tool.", planResult.Action, err))
					emit(req, SSEEvent{
						Type:  EventToolError,
						Tool:  planResult.Action,
						Args:  planResult.Parameters,
						Error: err.Error(),
					})
					compactMemory(req)
					trace = append(trace, LoopIteration{
						Index:       iterationIndex,
						Outcome:     OutcomeToolCalls,
						ToolEvents:  []ToolEvent{{Tool: planResult.Action, Args: planResult.Parameters, Error: err.Error()}},
						ContextSize: req.Memory.Len(),
					})
					continue
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

				// Callback path (SSE streaming / CLI) — caller owns waiting policy.
				logger.Info("approval required (callback)",
					"iteration", iterationIndex,
					"tool", planResult.Action,
					"risk", pendingAction.Risk,
				)

				approved := req.OnApproval(req.Context, pendingAction)

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
					compactMemory(req)
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
			result, err := req.Executor.ExecuteWithWorkspace(req.Context, planResult, req.Workspace)
			toolElapsed := time.Since(toolStart).Milliseconds()

			event := ToolEvent{
				Tool: planResult.Action,
				Args: planResult.Parameters,
			}
			if err != nil {
				event.Error = err.Error()
				req.Memory.AddToolResult(toolCallID, memory.FormatToolResult(planResult.Action, result, err))
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
			compactMemory(req)
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

func compactMemory(req LoopRequest) {
	if req.CompactMemory != nil {
		req.CompactMemory(req.Context)
	}
}

func repairFileWritePlan(req LoopRequest, planResult planner.PlanResult, logger *slog.Logger) planner.PlanResult {
	if planResult.Action != "edit_file" {
		return planResult
	}
	path, _ := planResult.Parameters["path"].(string)
	newText, hasNewText := planResult.Parameters["new_text"].(string)
	oldText, hasOldText := planResult.Parameters["old_text"].(string)
	if path == "" || !hasNewText || (hasOldText && oldText != "") {
		return planResult
	}

	empty, exists, err := workspaceFileIsEmpty(req.Workspace, path)
	if err != nil {
		logger.Warn("could not inspect file for edit_file repair",
			"path", path,
			"error", err.Error(),
		)
		return planResult
	}
	if exists && !empty {
		return planResult
	}

	params := map[string]interface{}{
		"path":      path,
		"content":   newText,
		"overwrite": exists,
	}
	logger.Info("repaired edit_file without old_text to write_file",
		"path", path,
		"file_exists", exists,
	)
	return planner.PlanResult{
		Action:     "write_file",
		Parameters: params,
		ToolCallID: planResult.ToolCallID,
	}
}

func workspaceFileIsEmpty(workspace, rawPath string) (empty bool, exists bool, err error) {
	fullPath, err := resolveWorkspacePath(workspace, rawPath)
	if err != nil {
		return false, false, err
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return true, false, nil
		}
		return false, false, err
	}
	if info.IsDir() {
		return false, true, fmt.Errorf("%q is a directory", rawPath)
	}
	return info.Size() == 0, true, nil
}

func resolveWorkspacePath(workspace, rawPath string) (string, error) {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		return "", fmt.Errorf("missing path")
	}
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	clean := filepath.Clean(path)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes workspace")
	}
	if isBlockedWorkspacePath(clean) {
		return "", fmt.Errorf("access to %q is blocked", clean)
	}

	root := workspace
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		root = cwd
	}
	root = filepath.Clean(root)
	if resolvedRoot, err := filepath.EvalSymlinks(root); err == nil {
		root = resolvedRoot
	}

	fullPath := filepath.Clean(filepath.Join(root, clean))
	if fullPath != root && !strings.HasPrefix(fullPath, root+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes workspace")
	}
	if realPath, err := filepath.EvalSymlinks(fullPath); err == nil {
		if realPath != root && !strings.HasPrefix(realPath, root+string(filepath.Separator)) {
			return "", fmt.Errorf("symlink %q points outside the workspace", clean)
		}
	}
	return fullPath, nil
}

func isBlockedWorkspacePath(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		switch part {
		case ".env", ".env.local", ".git":
			return true
		}
	}
	return false
}

func unsupportedActionReply(reason string) string {
	if reason != "" {
		return "Я не умею выполнить этот запрос доступными инструментами: " + reason
	}
	return "Я не умею выполнить этот запрос доступными инструментами."
}
