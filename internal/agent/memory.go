package agent

import (
	"log"
	"strings"
	"time"

	"ai-agent/internal/memory"
	"ai-agent/internal/planner"
)

func (a *Agent) remember(event memory.MemoryEvent) {
	if a.longTermMemory == nil {
		return
	}
	if event.SessionID == "" {
		event.SessionID = a.sessionID
	}
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	}
	if err := a.longTermMemory.Append(event); err != nil {
		log.Printf("[Agent] Memory append failed: %v", err)
	}
}

func (a *Agent) rememberPlan(result planner.PlanResult) {
	content := result.Reply
	if result.Action == planner.ActionUnknown {
		content = unknownObservation(result.Parameters)
	}

	a.remember(memory.MemoryEvent{
		Type:    memory.EventPlanResult,
		Action:  result.Action,
		Params:  result.Parameters,
		Content: content,
		Tags:    inferEventTags(result.Action, content),
	})
}

func (a *Agent) rememberAssistant(reply string) {
	a.remember(memory.MemoryEvent{
		Type:    memory.EventAssistantMessage,
		Content: reply,
		Tags:    inferEventTags("", reply),
	})
}

func inferEventTags(action, text string) []string {
	text = strings.ToLower(action + " " + text)
	var tags []string
	if strings.Contains(text, "git") || strings.Contains(text, "repo") || strings.Contains(text, "commit") {
		tags = append(tags, "git")
	}
	if strings.Contains(text, "crypto") ||
		strings.Contains(text, "bitcoin") ||
		strings.Contains(text, "ethereum") ||
		strings.Contains(text, "solana") ||
		strings.Contains(text, "price") {
		tags = append(tags, "crypto")
	}
	if strings.Contains(text, "planner") || action == planner.ActionUnknown {
		tags = append(tags, "planner")
	}
	return tags
}
