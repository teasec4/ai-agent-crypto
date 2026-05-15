package registry

import (
	"ai-agent/internal/tools"
	"fmt"
	"sort"
	"strings"
)

// Registry holds all available tools and provides lookup by name.
type Registry struct {
	tools map[string]tools.Tool
}

// New creates a new Registry and registers the given tools.
func New(toolList ...tools.Tool) *Registry {
	r := &Registry{
		tools: make(map[string]tools.Tool),
	}
	for _, t := range toolList {
		r.Register(t)
	}
	return r
}

// Register adds a tool to the registry.
func (r *Registry) Register(t tools.Tool) {
	r.tools[t.Name()] = t
}

// Get returns the tool by name, or nil if not found.
func (r *Registry) Get(name string) tools.Tool {
	return r.tools[name]
}

// List returns a formatted string of all registered tools with their descriptions.
// This should be included in the planner prompt.
func (r *Registry) List() string {
	var sb strings.Builder
	sb.WriteString("Available tools:\n")

	// Sort for deterministic output
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", name, r.tools[name].Description()))
	}
	return sb.String()
}

// IsValid returns true if a tool with the given name is registered.
func (r *Registry) IsValid(name string) bool {
	_, ok := r.tools[name]
	return ok
}
