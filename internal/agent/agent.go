package agent

import (
	"fmt"
	"log"
	"strings"

	"ai-agent/internal/executor"
	"ai-agent/internal/llm"
	"ai-agent/internal/planner"
	"ai-agent/internal/tools/registry"
)


// Agent orchestrates the full loop: Plan -> Act -> Observe.
type Agent struct {
	llmClient     llm.LlmClient
	registry      *registry.Registry
	maxIterations int
	history       *history
	compactAt     int // if history exceeds this, summarise before next Run
}

// NewAgent creates a new Agent with the given dependencies.
func NewAgent(
	llmClient llm.LlmClient,
	reg *registry.Registry,
	maxIterations int,
) *Agent {
	return &Agent{
		llmClient:     llmClient,
		registry:      reg,
		maxIterations: maxIterations,
		history:       newHistory(),
		compactAt:     defaultCompactAt,
	}
}

// Run executes the full agent loop for a user input.
// It returns the final result after the loop completes.
func (a *Agent) Run(input string) string {
	// Compact history if it's grown too large (before adding the new user message)
	a.maybeCompact()

	

	// Create fresh planner and executor for this run.
	plnr := planner.NewLLMPlanner(a.llmClient, a.registry)
	exctr := executor.New(a.registry)

	toolList := a.registry.List()

	currentInput := input
	lastResult := ""

	for i := 0; i < a.maxIterations; i++ {
		log.Printf("[Agent] 🔁 Iteration %d/%d", i+1, a.maxIterations)

		// --- Plan ---
		// Planner sees the full message chain: history messages are passed as-is
		log.Printf("[Agent] 🤔 Planning...")
		planResult := plnr.Plan(currentInput, a.history.messages, toolList)
		log.Printf("[Agent] 🎯 Plan: action=%q reasoning=%q done=%v params=%v",
			planResult.Action, planResult.Reasoning, planResult.Done, planResult.Parameters)

		// If the planner says we're done, format a natural response.
		if planResult.Done {
			final := a.formatResponse(input, lastResult)
			log.Printf("[Agent] ✅ Planner says done, returning: %q", final)
			a.history.addAssistant(final)
			return final
		}

		// --- Act ---
		log.Printf("[Agent] 🛠️  Executing tool: %s", planResult.Action)
		result, err := exctr.Execute(planResult)

		// Record the action result as an assistant message in history.
		// If the tool failed, the planner sees the error and can decide
		// to try a different approach instead of aborting.
		if err != nil {
			log.Printf("[Agent] ❌ Tool error: %v", err)
			a.history.messages = append(a.history.messages, llm.Message{
				Role:    roleTool,
				Content: fmt.Sprintf("Action: %s -> Error: %v", planResult.Action, err),
			})
		} else {
			log.Printf("[Agent] 📥 Tool result: %s", truncate(result, 200))

			// If a real tool (not fallback "unknown") executed successfully,
			// format the answer directly — no need for a second LLM call.
			if planResult.Action != "unknown" {
				final := a.formatResponse(input, result)
				log.Printf("[Agent] ✅ Tool completed successfully, returning: %q", final)
				a.history.addUser(input)
				a.history.addAssistant(final)
				return final
			}

			// "unknown" fallback: save to history so planner can retry
			lastResult = result
			a.history.messages = append(a.history.messages, llm.Message{
				Role:    roleTool,
				Content: fmt.Sprintf("Result of %s: %s", planResult.Action, result),
			})
		}

		// Next input for the planner is minimal — the tool result is already in history.
		// No need to duplicate it here (was the cause of context duplication).
		currentInput = "Continue. If the original request is fulfilled and no more tools are needed, respond with {\"done\": true}."
	}

	// Max iterations reached -- return the last result as final
	final := fmt.Sprintf("%s\n\n(I reached the maximum number of steps (%d). If you need more, please ask again.)",
		lastResult, a.maxIterations)
	// Add user message to history
	a.history.addUser(input)
	// Add an assistant message to history
	a.history.addAssistant(final)
	return final
}

// formatResponse sends a minimal request to format a natural final answer.
// It does NOT duplicate history — only the original input + raw tool result.
func (a *Agent) formatResponse(originalInput string, lastResult string) string {
	if lastResult == "" {
		return "I processed your request, but there are no results to show."
	}

	msgs := []llm.Message{
		{
			Role:    "system",
			Content: "You are a helpful AI assistant. Compose a natural, conversational response based on the tool results provided. Be concise but complete.",
		},
		{
			Role:    "user",
			Content: fmt.Sprintf("Original request: %s\n\nTool results:\n%s", originalInput, lastResult),
		},
	}

	response, err := a.llmClient.Chat(msgs)
	if err != nil {
		log.Printf("[Agent] ⚠️  formatResponse failed: %v — falling back to raw result", err)
		return lastResult
	}

	return response
}

// maybeCompact checks if the history exceeds compactAt.
// If so, asks the LLM to summarise older messages into a single system message.
// Falls back to simple trimming if the LLM call fails.
func (a *Agent) maybeCompact() {
	if a.history.len() < a.compactAt {
		return
	}

	log.Printf("[Agent] 📦 History length %d exceeds %d, compacting via LLM...", a.history.len(), a.compactAt)

	keep := a.history.len() / compactKeepRatio
	if keep < 1 {
		keep = 1
	}

	compactEnd := a.history.len() - keep
	if compactEnd <= 0 {
		return
	}

	// Build the text to summarise — only the older messages
	var sb strings.Builder
	for _, msg := range a.history.messages[:compactEnd] {
		sb.WriteString(fmt.Sprintf("%s: %s\n", truncateLabel(msg.Role), msg.Content))
	}

	summaryInput := sb.String()
	if summaryInput == "" {
		return
	}

	// Ask the LLM to summarise
	summary, err := a.llmClient.Chat([]llm.Message{
		{Role: "system", Content: summarizePrompt},
		{Role: "user", Content: summaryInput},
	})
	if err != nil {
		log.Printf("[Agent] ⚠️  Summarisation failed: %v — trimming instead", err)
		// Fallback: just keep the last `keep` messages
		a.history.messages = a.history.messages[a.history.len()-keep:]
		return
	}

	summary = strings.TrimSpace(summary)
	if summary == "" {
		// Empty summary — fallback to trim
		a.history.messages = a.history.messages[a.history.len()-keep:]
		return
	}

	log.Printf("[Agent] 📝 Summary: %s", truncate(summary, 120))

	// Replace old messages with a single system message containing the summary
	kept := a.history.messages[compactEnd:]
	a.history.messages = append([]llm.Message{{
		Role:    "system",
		Content: "Previous conversation summary: " + summary,
	}}, kept...)
}

func truncateLabel(role string) string {
	switch role {
	case "user":
		return "User"
	case "assistant":
		return "Assistant"
	case "system":
		return "[Context]"
	default:
		return role
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
