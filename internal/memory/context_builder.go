package memory

import (
	"fmt"
	"strings"
	"time"

	"ai-agent/internal/llm"
)

type ContextBuilder struct {
	store Store
}

type ContextRequest struct {
	SessionID string
	Input     string
	Limit     int
	Before    time.Time
	Tags      []string
}

func NewContextBuilder(store Store) *ContextBuilder {
	return &ContextBuilder{store: store}
}

func (b *ContextBuilder) Build(req ContextRequest) ([]llm.Message, error) {
	if b == nil || b.store == nil {
		return nil, nil
	}
	if req.SessionID == "" {
		req.SessionID = DefaultSessionID
	}
	if req.Limit <= 0 {
		req.Limit = DefaultContextLimit
	}

	events, err := b.store.Recent(req.SessionID, req.Limit)
	if err != nil {
		return nil, err
	}

	tags := req.Tags
	if len(tags) == 0 {
		tags = inferTags(req.Input)
	}
	for _, tag := range tags {
		tagged, err := b.store.ByTag(tag, DefaultTaggedLimit)
		if err != nil {
			return nil, err
		}
		events = appendUniqueEvents(events, tagged...)
	}

	events = filterEventsForSession(events, req.SessionID)
	events = filterEventsBefore(events, req.Before)
	content := formatMemoryContext(events)
	if strings.TrimSpace(content) == "" {
		return nil, nil
	}

	return []llm.Message{
		{
			Role: RoleSystem,
			Content: "Relevant long-term memory:\n" + content +
				"\nUse this only when it helps. Do not invent details beyond it.",
		},
	}, nil
}

func formatMemoryContext(events []MemoryEvent) string {
	var sb strings.Builder
	for _, event := range events {
		sb.WriteString("- ")
		sb.WriteString(formatMemoryEvent(event))
		sb.WriteByte('\n')
	}
	return strings.TrimSpace(sb.String())
}

func formatMemoryEvent(event MemoryEvent) string {
	var parts []string
	if event.Type != "" {
		parts = append(parts, fmt.Sprintf("[%s]", event.Type))
	}
	if event.Action != "" {
		parts = append(parts, "action="+event.Action)
	}

	text := firstNonEmpty(event.Content, event.Result, event.Error)
	if text != "" {
		parts = append(parts, compactText(text, DefaultEventMaxChars))
	}

	return strings.Join(parts, " ")
}

func appendUniqueEvents(events []MemoryEvent, more ...MemoryEvent) []MemoryEvent {
	seen := make(map[string]bool, len(events)+len(more))
	for _, event := range events {
		seen[eventKey(event)] = true
	}
	for _, event := range more {
		key := eventKey(event)
		if seen[key] {
			continue
		}
		seen[key] = true
		events = append(events, event)
	}
	return events
}

func filterEventsBefore(events []MemoryEvent, before time.Time) []MemoryEvent {
	if before.IsZero() {
		return events
	}

	filtered := make([]MemoryEvent, 0, len(events))
	for _, event := range events {
		if event.Time.IsZero() || event.Time.Before(before) {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func filterEventsForSession(events []MemoryEvent, sessionID string) []MemoryEvent {
	if sessionID == "" {
		return events
	}

	filtered := make([]MemoryEvent, 0, len(events))
	for _, event := range events {
		if event.SessionID == sessionID {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func inferTags(input string) []string {
	input = strings.ToLower(input)
	var tags []string
	if strings.Contains(input, "git") || strings.Contains(input, "repo") || strings.Contains(input, "commit") {
		tags = append(tags, "git")
	}
	if strings.Contains(input, "crypto") || strings.Contains(input, "bitcoin") || strings.Contains(input, "solana") {
		tags = append(tags, "crypto")
	}
	return tags
}

func eventKey(event MemoryEvent) string {
	if event.ID != "" {
		return event.ID
	}
	return event.SessionID + "|" + event.Type + "|" + event.Action + "|" + event.Content + "|" + event.Result + "|" + event.Error
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func compactText(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
