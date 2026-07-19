package tools

import (
	"context"
	"fmt"
	"strings"

	"ai-agent/internal/projectmemory"
)

// ReadProjectMemoryTool reads durable project notes from .agent/memory.md.
type ReadProjectMemoryTool struct{}

func NewReadProjectMemoryTool() *ReadProjectMemoryTool { return &ReadProjectMemoryTool{} }

func (t *ReadProjectMemoryTool) Name() string { return "read_project_memory" }

func (t *ReadProjectMemoryTool) Description() string {
	return "Read durable project notes from .agent/memory.md in the workspace."
}

func (t *ReadProjectMemoryTool) Schema() ToolSchema {
	return ToolSchema{
		Type:       "object",
		Properties: map[string]Parameter{},
	}
}

func (t *ReadProjectMemoryTool) Run(ctx context.Context, workspace string, params map[string]interface{}) (string, error) {
	content, err := projectmemory.Read(workspace)
	if err != nil {
		return "", err
	}
	if content == "" {
		return fmt.Sprintf("Project memory file %s does not exist or is empty.", projectmemory.RelativePath), nil
	}
	return fmt.Sprintf("Project memory (%s):\n%s", projectmemory.RelativePath, content), nil
}

// ProposeMemoryUpdateTool formats a suggested memory entry without writing it.
type ProposeMemoryUpdateTool struct{}

func NewProposeMemoryUpdateTool() *ProposeMemoryUpdateTool {
	return &ProposeMemoryUpdateTool{}
}

func (t *ProposeMemoryUpdateTool) Name() string { return "propose_memory_update" }

func (t *ProposeMemoryUpdateTool) Description() string {
	return "Propose a concise update for .agent/memory.md without modifying files."
}

func (t *ProposeMemoryUpdateTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]Parameter{
			"section": {Type: "string", Description: "Memory section to update, e.g. Decisions, User Preferences, How to run"},
			"entry":   {Type: "string", Description: "Concise durable fact or preference to remember (required)"},
			"reason":  {Type: "string", Description: "Short reason why this belongs in project memory"},
		},
		Required: []string{"entry"},
	}
}

func (t *ProposeMemoryUpdateTool) Run(ctx context.Context, workspace string, params map[string]interface{}) (string, error) {
	entry := strings.TrimSpace(getStringParam(params, "entry", ""))
	if entry == "" {
		return "", fmt.Errorf("missing required parameter 'entry'")
	}
	section := strings.TrimSpace(getStringParam(params, "section", "Notes"))
	reason := strings.TrimSpace(getStringParam(params, "reason", ""))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Proposed update for %s\n", projectmemory.RelativePath))
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("Section: %s\n", section))
	sb.WriteString(fmt.Sprintf("Entry: - %s\n", entry))
	if reason != "" {
		sb.WriteString(fmt.Sprintf("Reason: %s\n", reason))
	}
	sb.WriteString("\nNo file was modified. Ask the user before applying this memory update.")
	return sb.String(), nil
}
