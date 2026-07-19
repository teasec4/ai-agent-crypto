package projectmemory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	RelativePath = ".agent/memory.md"
	MaxBytes     = 32 * 1024
)

// Read loads the editable project memory file from the workspace root.
func Read(workspace string) (string, error) {
	root := workspaceRoot(workspace)
	path := filepath.Join(root, RelativePath)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read project memory %q: %w", RelativePath, err)
	}
	if len(data) > MaxBytes {
		data = data[:MaxBytes]
	}
	return strings.TrimSpace(string(data)), nil
}

func workspaceRoot(workspace string) string {
	if strings.TrimSpace(workspace) != "" {
		return filepath.Clean(workspace)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}
