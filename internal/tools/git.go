package tools

// GitTool provides information about the current git repository.
type GitTool struct{}

// NewGitTool creates a new GitTool instance.
func NewGitTool() Tool {
	return &GitTool{}
}

// Name returns the tool name.
func (t *GitTool) Name() string {
	return "git_context"
}

// Description returns a human-readable description of the tool.
func (t *GitTool) Description() string {
	return "Get information about the current git repository (branch, status, recent commits, diff). Parameters: action (one of: status, branch, log, diff)"
}

func (t *GitTool) Run(params map[string]interface{}) (string, error) {
	return "", nil
}
