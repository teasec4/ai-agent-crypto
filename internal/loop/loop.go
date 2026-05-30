package loop

import (
	"fmt"

	"ai-agent/internal/guardrails"
	"ai-agent/internal/planner"
)

const (
	DefaultMaxIterations = 5
	DefaultMaxMessages   = 50
)

func RunLoop(req LoopRequest) LoopResult {
	trace := []LoopIteration{}

	for {
		iterationIndex := len(trace) + 1

		if req.Guardrail != nil {
			checkErr := req.Guardrail(guardrails.GuardrailInput{
				Iteration: len(trace),
				Messages:  req.Memory.Messages,
			})
			if checkErr != nil {
				return LoopResult{
					Answer:     checkErr.Error(),
					Iterations: len(trace),
					Trace:      trace,
					StoppedBy:  StoppedByGuardrail,
				}
			}
		}

		planResult, err := req.Planner.Plan(req.Memory.Messages)
		if err != nil {
			trace = append(trace, LoopIteration{
				Index:       iterationIndex,
				Outcome:     OutcomeError,
				ContextSize: req.Memory.Len(),
			})
			return LoopResult{
				Answer:     fmt.Sprintf("Не удалось построить план: %v", err),
				Iterations: len(trace),
				Trace:      trace,
				StoppedBy:  StoppedByError,
			}
		}

		switch planResult.Action {
		case planner.ActionMessage:
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
			answer := unsupportedActionReply(planResult)
			req.Memory.AddAssistant(answer)
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
			result, err := req.Executor.Execute(planResult)
			event := ToolEvent{
				Tool: planResult.Action,
				Args: planResult.Parameters,
			}
			if err != nil {
				event.Error = err.Error()
				req.Memory.AddTool(fmt.Sprintf("Tool %s failed: %v", planResult.Action, err))
			} else {
				event.Result = result
				req.Memory.AddTool(fmt.Sprintf("Tool %s result: %s", planResult.Action, result))
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
