package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"ai-agent/internal/approval"
)

const (
	defaultCommandTimeoutSeconds = 60
	maxCommandTimeoutSeconds     = 300
	defaultCommandMaxBytes       = 120 * 1024
)

// CommandTool runs allowlisted non-interactive commands inside the workspace after approval.
type CommandTool struct{}

func NewCommandTool() *CommandTool { return &CommandTool{} }

func (t *CommandTool) Name() string {
	return "run_command"
}

func (t *CommandTool) Description() string {
	return "Run an allowlisted non-interactive command in the workspace."
}

func (t *CommandTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]Parameter{
			"command":         {Type: "string", Description: "Command to run (go, git, ls, pwd) (required)"},
			"args":            {Type: "array", Description: "Command arguments", Items: &Parameter{Type: "string"}},
			"cwd":             {Type: "string", Description: "Working directory (default: .)"},
			"timeout_seconds": {Type: "integer", Description: "Timeout in seconds (default: 60)"},
		},
		Required: []string{"command"},
	}
}

func (t *CommandTool) RequiresApproval(params map[string]interface{}) bool {
	return true
}

func (t *CommandTool) Risk(params map[string]interface{}) approval.RiskLevel {
	return approval.RiskExec
}

func (t *CommandTool) Summary(params map[string]interface{}) string {
	command := getStringParam(params, "command", "")
	args, _ := getStringSliceParam(params, "args")
	return fmt.Sprintf("Run command: %s", formatCommand(command, args))
}

func (t *CommandTool) Preview(params map[string]interface{}) (string, error) {
	command, args, cwd, timeoutSeconds, maxBytes, err := t.parseParams(params)
	if err != nil {
		return "", err
	}
	if err := validateAllowedCommand(command, args); err != nil {
		return "", err
	}

	return fmt.Sprintf("Will run command after approval:\nCommand: %s\nWorking directory: %s\nTimeout: %ds\nMax output: %d bytes", formatCommand(command, args), cwd, timeoutSeconds, maxBytes), nil
}

func (t *CommandTool) Run(ctx context.Context, ws string, params map[string]interface{}) (string, error) {
	command, args, cwd, timeoutSeconds, maxBytes, err := t.parseParams(params)
	if err != nil {
		return "", err
	}
	if err := validateAllowedCommand(command, args); err != nil {
		return "", err
	}

	root := getRoot(ws)
	fullCWD, relCWD, err := resolvePath(root, cwd)
	if err != nil {
		return "", err
	}

	subCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(subCtx, command, args...)
	cmd.Dir = fullCWD
	cmd.Env = append(cmd.Environ(), "GIT_PAGER=cat", "PAGER=cat")

	var stdout limitedBuffer
	var stderr limitedBuffer
	stdout.limit = maxBytes
	stderr.limit = maxBytes
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if subCtx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("command timed out after %d seconds", timeoutSeconds)
	}

	output := formatCommandOutput(formatCommand(command, args), relCWD, stdout.String(), stderr.String(), stdout.truncated || stderr.truncated)
	if err != nil {
		return output, fmt.Errorf("command failed: %w", err)
	}
	return output, nil
}

func (t *CommandTool) parseParams(params map[string]interface{}) (string, []string, string, int, int, error) {
	command := strings.TrimSpace(getStringParam(params, "command", ""))
	if command == "" {
		return "", nil, "", 0, 0, fmt.Errorf("missing required parameter 'command'")
	}
	if command != filepath.Base(command) || strings.ContainsAny(command, `/\`) {
		return "", nil, "", 0, 0, fmt.Errorf("command must be a bare executable name, not a path")
	}

	args, err := getStringSliceParam(params, "args")
	if err != nil {
		return "", nil, "", 0, 0, err
	}
	cwd := getStringParam(params, "cwd", ".")
	timeoutSeconds := getIntParam(params, "timeout_seconds", defaultCommandTimeoutSeconds, 1, maxCommandTimeoutSeconds)
	maxBytes := getIntParam(params, "max_bytes", defaultCommandMaxBytes, 1, 512*1024)
	return command, args, cwd, timeoutSeconds, maxBytes, nil
}

func getStringSliceParam(params map[string]interface{}, key string) ([]string, error) {
	value, ok := params[key]
	if !ok || value == nil {
		return []string{}, nil
	}

	switch v := value.(type) {
	case []string:
		return append([]string(nil), v...), nil
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			arg, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("parameter %q must be an array of strings", key)
			}
			result = append(result, arg)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("parameter %q must be an array of strings", key)
	}
}

func validateAllowedCommand(command string, args []string) error {
	for _, arg := range args {
		if strings.ContainsAny(arg, "\x00\n\r") {
			return fmt.Errorf("command arguments must not contain control characters")
		}
	}

	switch command {
	case "pwd":
		if len(args) != 0 {
			return fmt.Errorf("pwd does not accept arguments")
		}
		return nil
	case "ls":
		return validateLSArgs(args)
	case "go":
		return validateGoArgs(args)
	case "git":
		return validateGitArgs(args)
	default:
		return fmt.Errorf("command %q is not allowlisted", command)
	}
}

func validateLSArgs(args []string) error {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			for _, ch := range strings.TrimPrefix(arg, "-") {
				if !strings.ContainsRune("lahAR", ch) {
					return fmt.Errorf("ls flag -%c is not allowed", ch)
				}
			}
			continue
		}
		if filepath.IsAbs(arg) || strings.Contains(filepath.Clean(arg), "..") || isBlockedWorkspacePath(arg) {
			return fmt.Errorf("ls path %q is invalid or blocked", arg)
		}
	}
	return nil
}

func validateGoArgs(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("go subcommand is required")
	}

	subcommand := args[0]
	switch subcommand {
	case "test", "vet", "fmt":
		return validateSafeGoPackageArgs(args[1:])
	case "version", "env":
		if len(args) > 2 {
			return fmt.Errorf("go %s accepts at most one argument in this tool", subcommand)
		}
		return validateNoShellLikeArgs(args[1:])
	default:
		return fmt.Errorf("go %s is not allowlisted", subcommand)
	}
}

func validateSafeGoPackageArgs(args []string) error {
	for _, arg := range args {
		if err := validateNoShellLikeArg(arg); err != nil {
			return err
		}
		if strings.HasPrefix(arg, "-") {
			safePrefixes := []string{"-run", "-count", "-timeout", "-v", "-race", "-cover"}
			allowed := false
			for _, prefix := range safePrefixes {
				if arg == prefix || strings.HasPrefix(arg, prefix+"=") {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("go flag %q is not allowed", arg)
			}
			continue
		}
		if filepath.IsAbs(arg) || strings.Contains(filepath.Clean(arg), "..") || isBlockedWorkspacePath(arg) {
			return fmt.Errorf("go package/path argument %q is invalid or blocked", arg)
		}
	}
	return nil
}

func validateGitArgs(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("git subcommand is required")
	}

	allowedReadOnly := map[string]bool{
		"status":    true,
		"diff":      true,
		"log":       true,
		"branch":    true,
		"show":      true,
		"rev-parse": true,
	}
	if !allowedReadOnly[args[0]] {
		return fmt.Errorf("git %s is not allowlisted", args[0])
	}
	return validateNoShellLikeArgs(args[1:])
}

func validateNoShellLikeArgs(args []string) error {
	for _, arg := range args {
		if err := validateNoShellLikeArg(arg); err != nil {
			return err
		}
	}
	return nil
}

func validateNoShellLikeArg(arg string) error {
	blocked := []string{";", "&&", "||", "|", ">", "<", "$", "`", "$(", "../"}
	for _, token := range blocked {
		if strings.Contains(arg, token) {
			return fmt.Errorf("argument %q contains blocked token %q", arg, token)
		}
	}
	if filepath.IsAbs(arg) && !strings.HasPrefix(arg, "--") {
		return fmt.Errorf("absolute path argument %q is not allowed", arg)
	}
	return nil
}

func formatCommand(command string, args []string) string {
	parts := append([]string{command}, args...)
	return strings.Join(parts, " ")
}

func formatCommandOutput(command string, cwd string, stdout string, stderr string, truncated bool) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Command: %s\n", command))
	sb.WriteString(fmt.Sprintf("Working directory: %s\n", cwd))
	if truncated {
		sb.WriteString("Note: output was truncated.\n")
	}
	sb.WriteString("--- stdout\n")
	if strings.TrimSpace(stdout) == "" {
		sb.WriteString("<empty>\n")
	} else {
		sb.WriteString(stdout)
		if !strings.HasSuffix(stdout, "\n") {
			sb.WriteString("\n")
		}
	}
	sb.WriteString("--- stderr\n")
	if strings.TrimSpace(stderr) == "" {
		sb.WriteString("<empty>")
	} else {
		sb.WriteString(stderr)
	}
	return sb.String()
}

type limitedBuffer struct {
	bytes.Buffer
	limit     int
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		b.truncated = true
		return len(p), nil
	}

	remaining := b.limit - b.Buffer.Len()
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		_, _ = b.Buffer.Write(p[:remaining])
		b.truncated = true
		return len(p), nil
	}
	_, _ = b.Buffer.Write(p)
	return len(p), nil
}
