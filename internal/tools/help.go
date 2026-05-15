package tools

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
