package tools

import (
	"fmt"
	"os/exec"
	"strings"
)

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
	return "Get information about the current git repository (branch, status, recent commits, diff). Parameters: action (one of: status, branch, log, diff) — defaults to status."
}

// Run executes the requested git action.
func (t *GitTool) Run(params map[string]interface{}) (string, error) {
	action := "status"
	if a, ok := params["action"].(string); ok && a != "" {
		action = a
	}

	switch action {
	case "status":
		return t.gitStatus()
	case "branch":
		return t.gitBranch()
	case "log":
		limit := 5
		if l, ok := params["limit"].(float64); ok && l > 0 {
			limit = int(l)
		}
		return t.gitLog(limit)
	case "diff":
		return t.gitDiff()
	default:
		return "", fmt.Errorf("unknown git action: %s (supported: status, branch, log, diff)", action)
	}
}

func (t *GitTool) gitStatus() (string, error) {
	out, err := exec.Command("git", "status", "--short", "--branch").Output()
	if err != nil {
		return "", fmt.Errorf("git status failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (t *GitTool) gitBranch() (string, error) {
	out, err := exec.Command("git", "branch", "-a").Output()
	if err != nil {
		return "", fmt.Errorf("git branch failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (t *GitTool) gitLog(limit int) (string, error) {
	args := []string{"log", "--oneline", fmt.Sprintf("-%d", limit)}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", fmt.Errorf("git log failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (t *GitTool) gitDiff() (string, error) {
	out, err := exec.Command("git", "diff", "--stat").Output()
	if err != nil {
		return "", fmt.Errorf("git diff failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
