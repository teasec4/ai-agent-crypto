package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	defaultGitTimeout       = 10 * time.Second
	defaultGitOutputMaxSize = 120 * 1024
)

// GitTool provides read-only local git repository context.
type GitTool struct{}

func NewGitTool() *GitTool { return &GitTool{} }

func (t *GitTool) Name() string { return "git_context" }
func (t *GitTool) Description() string {
	return "Read local git repository context."
}
func (t *GitTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]Parameter{
			"mode":      {Type: "string", Description: "Information mode: branch, status, diff, log, changed_files, branch_diff (required)"},
			"base":      {Type: "string", Description: "Base branch for branch_diff (default: main)"},
			"limit":     {Type: "integer", Description: "Log limit (default: 10)"},
			"max_bytes": {Type: "integer", Description: "Max output bytes (default: 122880)"},
		},
		Required: []string{"mode"},
	}
}
func (t *GitTool) Run(ctx context.Context, workspace string, params map[string]interface{}) (string, error) {
	mode := getStringParam(params, "mode", "status")
	maxBytes := getIntParam(params, "max_bytes", defaultGitOutputMaxSize, 1, 512*1024)
	root := getRoot(workspace)

	if err := t.ensureGitRepo(root); err != nil {
		return "", err
	}

	var output string
	var err error
	switch mode {
	case "branch":
		output, err = t.runGit(root, maxBytes, "branch", "--show-current")
	case "status":
		output, err = t.runGit(root, maxBytes, "status", "--short", "--branch")
	case "changed_files":
		output, err = t.changedFiles(root, maxBytes)
	case "diff":
		output, err = t.diff(root, maxBytes)
	case "branch_diff":
		base := getStringParam(params, "base", "")
		if base == "" {
			base = t.defaultBaseBranch(root)
		}
		output, err = t.branchDiff(root, base, maxBytes)
	case "log":
		limit := getIntParam(params, "limit", 10, 1, 50)
		output, err = t.runGit(root, maxBytes, "log", "--oneline", "--decorate", "-"+strconv.Itoa(limit))
	default:
		return "", fmt.Errorf("unsupported git_context mode %q", mode)
	}
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(output) == "" {
		output = "No output."
	}
	return fmt.Sprintf("git_context mode=%s\n---\n%s", mode, strings.TrimRight(output, "\n")), nil
}

func (t *GitTool) ensureGitRepo(root string) error {
	_, err := t.runGit(root, 4096, "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("workspace is not a git repository: %w", err)
	}
	return nil
}

func (t *GitTool) changedFiles(root string, maxBytes int) (string, error) {
	status, err := t.runGit(root, maxBytes, "status", "--short")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(status) == "" {
		return "No changed files.", nil
	}
	return status, nil
}

func (t *GitTool) diff(root string, maxBytes int) (string, error) {
	unstaged, err := t.runGit(root, maxBytes/2, "diff", "--no-ext-diff")
	if err != nil {
		return "", err
	}
	staged, err := t.runGit(root, maxBytes/2, "diff", "--cached", "--no-ext-diff")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(unstaged) == "" && strings.TrimSpace(staged) == "" {
		return "No working tree diff.", nil
	}

	var sb strings.Builder
	if strings.TrimSpace(staged) != "" {
		sb.WriteString("Staged diff:\n")
		sb.WriteString(staged)
		if !strings.HasSuffix(staged, "\n") {
			sb.WriteString("\n")
		}
	}
	if strings.TrimSpace(unstaged) != "" {
		sb.WriteString("Unstaged diff:\n")
		sb.WriteString(unstaged)
	}
	return sb.String(), nil
}

func (t *GitTool) branchDiff(root, base string, maxBytes int) (string, error) {
	if strings.TrimSpace(base) == "" {
		return "", fmt.Errorf("base branch is required; e.g. main")
	}
	mergeBaseRange := base + "...HEAD"
	output, err := t.runGit(root, maxBytes, "diff", "--stat", mergeBaseRange)
	if err != nil {
		return "", err
	}
	fullDiff, err := t.runGit(root, maxBytes, "diff", "--no-ext-diff", mergeBaseRange)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(output) == "" && strings.TrimSpace(fullDiff) == "" {
		return fmt.Sprintf("No diff against %s.", base), nil
	}
	return fmt.Sprintf("Diff against %s:\n%s\n%s", base, output, fullDiff), nil
}

func (t *GitTool) defaultBaseBranch(root string) string {
	for _, branch := range []string{"main", "master"} {
		if _, err := t.runGit(root, 4096, "rev-parse", "--verify", branch); err == nil {
			return branch
		}
		if _, err := t.runGit(root, 4096, "rev-parse", "--verify", "origin/"+branch); err == nil {
			return "origin/" + branch
		}
	}
	return ""
}

func (t *GitTool) runGit(root string, maxBytes int, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start git %s: %w", strings.Join(args, " "), err)
	}
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			message := strings.TrimSpace(stderr.String())
			if message == "" {
				message = err.Error()
			}
			return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), message)
		}
	case <-time.After(defaultGitTimeout):
		_ = cmd.Process.Kill()
		return "", fmt.Errorf("git %s timed out", strings.Join(args, " "))
	}

	output := stdout.String()
	if len(output) > maxBytes {
		output = output[:maxBytes] + fmt.Sprintf("\n... truncated to %d bytes", maxBytes)
	}
	return output, nil
}
