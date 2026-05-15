package tools

import "fmt"

// HelpTool returns a short description of available agent capabilities.
type HelpTool struct{}

func NewHelpTool() Tool {
	return &HelpTool{}
}

func (t *HelpTool) Name() string {
	return "help"
}

func (t *HelpTool) Description() string {
	return "Explain what this agent can do and which tools are available. Parameters: none."
}

func (t *HelpTool) Run(params map[string]interface{}) (string, error) {
	return "I can answer directly, get cryptocurrency prices, and inspect git repository context.", nil
}

// UnknownTool is a fallback for unsupported actions.
type UnknownTool struct{}

func NewUnknownTool() Tool {
	return &UnknownTool{}
}

func (t *UnknownTool) Name() string {
	return "unknown"
}

func (t *UnknownTool) Description() string {
	return "Fallback action for unsupported or unclear requests. Parameters: reason (optional)."
}

func (t *UnknownTool) Run(params map[string]interface{}) (string, error) {
	if reason, ok := params["reason"].(string); ok && reason != "" {
		return "I could not choose a suitable tool: " + reason, nil
	}
	return "I could not choose a suitable tool for this request.", nil
}

func requireStringParam(params map[string]interface{}, name string) (string, error) {
	value, ok := params[name].(string)
	if !ok || value == "" {
		return "", fmt.Errorf("missing required parameter %q", name)
	}
	return value, nil
}
