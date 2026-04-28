package tools

import "fmt"

// HelpTool returns the list of available tools.
type HelpTool struct{}

// NewHelpTool creates a new HelpTool instance.
func NewHelpTool() Tool {
	return &HelpTool{}
}

func (t *HelpTool) Name() string {
	return "help"
}

func (t *HelpTool) Description() string {
	return "Show the list of available tools and how to use them."
}

func (t *HelpTool) Run(params map[string]interface{}) (string, error) {
	return "Send me a command and I will decide which tool to use.", nil
}

// UnknownTool handles unknown/unrecognised actions.
type UnknownTool struct{}

// NewUnknownTool creates a new UnknownTool instance.
func NewUnknownTool() Tool {
	return &UnknownTool{}
}

func (t *UnknownTool) Name() string {
	return "unknown"
}

func (t *UnknownTool) Description() string {
	return "Fallback for unrecognised queries. The agent does not know how to handle this."
}

func (t *UnknownTool) Run(params map[string]interface{}) (string, error) {
	query := "your query"
	if q, ok := params["query"].(string); ok && q != "" {
		query = q
	}
	return fmt.Sprintf("I'm sorry, I don't know how to handle '%s'. Here are the tools I have available:\n\n", query), nil
}
