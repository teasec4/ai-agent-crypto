package tools

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultGitTimeout       = 10 * time.Second
	defaultGitOutputMaxSize = 120 * 1024
)

// GitTool provides read-only local git repository context.
type GitTool struct {
	root string
}

func NewGitTool() *GitTool {
	root, err := os.Getwd()
	if err != nil {
		root = "."
	}
	return &GitTool{root: root}
}

func (t *GitTool) SetRoot(path string) { t.root = filepath.Clean(path) }

func (t *GitTool) Name() string {
	return "git_context"
}

func (t *GitTool) Description() string {
	return "Read local git repository context. Parameters: mode (branch, status, changed_files, diff, branch_diff, log), base (for branch_diff, default: main then master), limit (for log, default: 10), max_bytes (default: 122880). Read-only."
}

func (t *GitTool) Run(params map[string]interface{}) (string, error) {
	mode := getStringParam(params, "mode", "status")
	maxBytes := getIntParam(params, "max_bytes", defaultGitOutputMaxSize, 1, 512*1024)

	if err := t.ensureGitRepo(); err != nil {
		return "", err
	}

	var output string
	var err error
	switch mode {
	case "branch":
		output, err = t.runGit(maxBytes, "branch", "--show-current")
	case "status":
		output, err = t.runGit(maxBytes, "status", "--short", "--branch")
	case "changed_files":
		output, err = t.changedFiles(maxBytes)
	case "diff":
		output, err = t.diff(maxBytes)
	case "branch_diff":
		base := getStringParam(params, "base", "")
		output, err = t.branchDiff(base, maxBytes)
	case "log":
		limit := getIntParam(params, "limit", 10, 1, 50)
		output, err = t.runGit(maxBytes, "log", "--oneline", "--decorate", "-"+strconv.Itoa(limit))
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

func (t *GitTool) ensureGitRepo() error {
	_, err := t.runGit(4096, "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("workspace is not a git repository: %w", err)
	}
	return nil
}

func (t *GitTool) changedFiles(maxBytes int) (string, error) {
	status, err := t.runGit(maxBytes, "status", "--short")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(status) == "" {
		return "No changed files.", nil
	}
	return status, nil
}

func (t *GitTool) diff(maxBytes int) (string, error) {
	unstaged, err := t.runGit(maxBytes/2, "diff", "--no-ext-diff")
	if err != nil {
		return "", err
	}
	staged, err := t.runGit(maxBytes/2, "diff", "--cached", "--no-ext-diff")
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

func (t *GitTool) branchDiff(base string, maxBytes int) (string, error) {
	if strings.TrimSpace(base) == "" {
		base = t.defaultBaseBranch()
	}
	if base == "" {
		return "", fmt.Errorf("could not determine base branch; pass parameter 'base', e.g. main")
	}
	mergeBaseRange := base + "...HEAD"
	output, err := t.runGit(maxBytes, "diff", "--stat", mergeBaseRange)
	if err != nil {
		return "", err
	}
	fullDiff, err := t.runGit(maxBytes, "diff", "--no-ext-diff", mergeBaseRange)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(output) == "" && strings.TrimSpace(fullDiff) == "" {
		return fmt.Sprintf("No diff against %s.", base), nil
	}
	return fmt.Sprintf("Diff against %s:\n%s\n%s", base, output, fullDiff), nil
}

func (t *GitTool) defaultBaseBranch() string {
	for _, branch := range []string{"main", "master"} {
		if _, err := t.runGit(4096, "rev-parse", "--verify", branch); err == nil {
			return branch
		}
		if _, err := t.runGit(4096, "rev-parse", "--verify", "origin/"+branch); err == nil {
			return "origin/" + branch
		}
	}
	return ""
}

func (t *GitTool) runGit(maxBytes int, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = t.root

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
