package agent

import (
	"fmt"
	"strings"

	"ai-agent/internal/executor"
	"ai-agent/internal/planner"
	"ai-agent/internal/tools/registry"
)

// Agent orchestrates the full loop: Plan → Act → Observe.
type Agent struct {
	planner       planner.Planner
	executor      executor.Executor
	state         *State
	registry      *registry.Registry
	maxIterations int
}

// NewAgent creates a new Agent with the given dependencies.
func NewAgent(
	llmClient interface {
		Chat(prompt string) (string, error)
	},
	reg *registry.Registry,
	maxIterations int,
) *Agent {
	return &Agent{
		planner:       planner.NewLLMPlanner(llmClient, reg),
		executor:      executor.New(reg),
		state:         NewState(),
		registry:      reg,
		maxIterations: maxIterations,
	}
}

// Run executes the full agent loop for a user input.
// It returns the final result after the loop completes.
func (a *Agent) Run(input string) string {
	a.state = NewState()

	toolList := a.registry.List()

	// We'll track the current plan and result across iterations.
	currentInput := input

	for i := 0; i < a.maxIterations; i++ {
		// --- Plan ---
		planResult := a.planner.Plan(currentInput, a.state.History, toolList)

		// If the planner says we're done, return immediately with reasoning.
		if planResult.Done {
			if planResult.Reasoning != "" {
				return planResult.Reasoning
			}
			return a.state.LastResult
		}

		// --- Act ---
		result, err := a.executor.Execute(planResult)
		if err != nil {
			errMsg := fmt.Sprintf("Error executing '%s': %v", planResult.Action, err)
			a.state = a.state.WithResult(currentInput, planResult.Action, errMsg)
			return errMsg
		}

		// --- Observe ---
		// Save the step
		a.state = a.state.WithResult(currentInput, planResult.Action, result)

		// Check if the result contains a final answer (i.e., no more actions needed)
		if a.isFinalResult(result) {
			return result
		}

		// Prepare the next input for the planner — include the last observation.
		currentInput = fmt.Sprintf(
			"Previous action: %s\nResult: %s\n\nOriginal request: %s\nContinue if needed.",
			planResult.Action,
			truncate(result, 200),
			input,
		)
	}

	// Max iterations reached — return last result with a note.
	return fmt.Sprintf("%s\n\n(I reached the maximum number of steps (%d). If you need more, please ask again.)",
		a.state.LastResult, a.maxIterations)
}

// isFinalResult checks if the result looks like a final answer.
// This is a heuristic — for now we assume a single tool call is sufficient.
func (a *Agent) isFinalResult(result string) bool {
	// If the result starts with an error or help message, it's final.
	if strings.HasPrefix(result, "I'm sorry") || strings.HasPrefix(result, "Error") {
		return true
	}
	// If we have performed more than 3 actions, it's probably final.
	if len(a.state.History) > 3 {
		return true
	}
	// For now, assume the first result is the final one (simple agent).
	// In a more advanced agent, you'd let the planner decide.
	return true
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
