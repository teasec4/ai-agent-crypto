package registry

import (
	"fmt"
	"sort"
	"strings"

	"ai-agent/internal/llm"
	"ai-agent/internal/tools"
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

// ToolDefinitions returns the list of LLM tool definitions for all registered tools.
func (r *Registry) ToolDefinitions() []llm.ToolDefinition {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	defs := make([]llm.ToolDefinition, 0, len(names))
	for _, name := range names {
		t := r.tools[name]
		schema := t.Schema()
		defs = append(defs, llm.ToolDefinition{
			Type: "function",
			Function: llm.FunctionDefinition{
				Name:        name,
				Description: t.Description(),
				Parameters: &llm.JSONSchema{
					Type:       schema.Type,
					Properties: toolPropsToLLM(schema.Properties),
					Required:   schema.Required,
				},
			},
		})
	}
	return defs
}

func toolPropsToLLM(props map[string]tools.Parameter) map[string]llm.Property {
	result := make(map[string]llm.Property, len(props))
	for k, v := range props {
		p := llm.Property{
			Type:        v.Type,
			Description: v.Description,
		}
		if v.Items != nil {
			p.Items = &llm.Property{
				Type:        v.Items.Type,
				Description: v.Items.Description,
			}
		}
		result[k] = p
	}
	return result
}

// List returns a formatted string of all registered tools with their descriptions.
func (r *Registry) List() string {
	var sb strings.Builder
	sb.WriteString("Available tools:\n")

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

// Count returns the number of registered tools.
func (r *Registry) Count() int {
	return len(r.tools)
}
