package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ai-agent/internal/approval"
)

const writePreviewLimit = 6000

// CreateDirectoryTool creates a directory inside the workspace after approval.
type CreateDirectoryTool struct{}

func NewCreateDirectoryTool() *CreateDirectoryTool { return &CreateDirectoryTool{} }

func (t *CreateDirectoryTool) Name() string { return "create_directory" }
func (t *CreateDirectoryTool) Description() string {
	return "Create a directory in the workspace."
}
func (t *CreateDirectoryTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]Parameter{
			"path": {Type: "string", Description: "Relative directory path (required)"},
		},
		Required: []string{"path"},
	}
}

func (t *CreateDirectoryTool) RequiresApproval(params map[string]interface{}) bool { return true }
func (t *CreateDirectoryTool) Risk(params map[string]interface{}) approval.RiskLevel {
	return approval.RiskWrite
}
func (t *CreateDirectoryTool) Summary(params map[string]interface{}) string {
	path := getStringParam(params, "path", "")
	return fmt.Sprintf("Create directory %s", path)
}
func (t *CreateDirectoryTool) Preview(params map[string]interface{}) (string, error) {
	path := getStringParam(params, "path", "")
	if path == "" {
		return "", fmt.Errorf("missing required parameter 'path'")
	}
	return fmt.Sprintf("Will create directory:\n%s", path), nil
}
func (t *CreateDirectoryTool) Run(ctx context.Context, workspace string, params map[string]interface{}) (string, error) {
	path := getStringParam(params, "path", "")
	if path == "" {
		return "", fmt.Errorf("missing required parameter 'path'")
	}
	root := getRoot(workspace)
	fullPath, relPath, err := resolvePath(root, path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %q: %w", relPath, err)
	}
	return fmt.Sprintf("Created directory %s", relPath), nil
}

// DeletePathTool deletes a file or directory inside the workspace after approval.
type DeletePathTool struct{}

func NewDeletePathTool() *DeletePathTool { return &DeletePathTool{} }

func (t *DeletePathTool) Name() string { return "delete_path" }
func (t *DeletePathTool) Description() string {
	return "Delete a file or directory from the workspace."
}
func (t *DeletePathTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]Parameter{
			"path": {Type: "string", Description: "Relative file or directory path to delete (required)"},
		},
		Required: []string{"path"},
	}
}

func (t *DeletePathTool) RequiresApproval(params map[string]interface{}) bool { return true }
func (t *DeletePathTool) Risk(params map[string]interface{}) approval.RiskLevel {
	return approval.RiskWrite
}
func (t *DeletePathTool) Summary(params map[string]interface{}) string {
	return fmt.Sprintf("Delete %s", getStringParam(params, "path", ""))
}
func (t *DeletePathTool) Preview(params map[string]interface{}) (string, error) {
	path := getStringParam(params, "path", "")
	if path == "" {
		return "", fmt.Errorf("missing required parameter 'path'")
	}
	clean := filepath.ToSlash(filepath.Clean(path))
	if clean == "." {
		return "", fmt.Errorf("deleting the workspace root is not allowed")
	}
	if filepath.IsAbs(path) || isPathTraversal(path) || isBlockedWorkspacePath(path) {
		return "", fmt.Errorf("invalid or blocked path %q", path)
	}
	return fmt.Sprintf("Will permanently delete:\n%s", path), nil
}
func (t *DeletePathTool) Run(ctx context.Context, workspace string, params map[string]interface{}) (string, error) {
	path := getStringParam(params, "path", "")
	if path == "" {
		return "", fmt.Errorf("missing required parameter 'path'")
	}
	root := getRoot(workspace)
	fullPath, relPath, err := resolvePath(root, path)
	if err != nil {
		return "", err
	}
	if relPath == "." {
		return "", fmt.Errorf("deleting the workspace root is not allowed")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	info, err := os.Lstat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("path %q does not exist", relPath)
		}
		return "", fmt.Errorf("failed to stat %q: %w", relPath, err)
	}
	if info.IsDir() {
		if err := os.RemoveAll(fullPath); err != nil {
			return "", fmt.Errorf("failed to delete directory %q: %w", relPath, err)
		}
		return fmt.Sprintf("Deleted directory %s", relPath), nil
	}
	if err := os.Remove(fullPath); err != nil {
		return "", fmt.Errorf("failed to delete file %q: %w", relPath, err)
	}
	return fmt.Sprintf("Deleted file %s", relPath), nil
}

// WriteFileTool writes a text file inside the workspace after approval.
type WriteFileTool struct{}

func NewWriteFileTool() *WriteFileTool { return &WriteFileTool{} }

func (t *WriteFileTool) Name() string { return "write_file" }
func (t *WriteFileTool) Description() string {
	return "Create a new text file, write to an empty file, or overwrite a whole text file in the workspace."
}
func (t *WriteFileTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]Parameter{
			"path":      {Type: "string", Description: "Relative file path (required)"},
			"content":   {Type: "string", Description: "File content (required)"},
			"overwrite": {Type: "boolean", Description: "Overwrite existing file or fill an empty existing file (default: false)"},
		},
		Required: []string{"path", "content"},
	}
}

func (t *WriteFileTool) RequiresApproval(params map[string]interface{}) bool { return true }
func (t *WriteFileTool) Risk(params map[string]interface{}) approval.RiskLevel {
	return approval.RiskWrite
}
func (t *WriteFileTool) Summary(params map[string]interface{}) string {
	path := getStringParam(params, "path", "")
	if getBoolParam(params, "overwrite", false) {
		return fmt.Sprintf("Overwrite file %s", path)
	}
	return fmt.Sprintf("Create file %s", path)
}
func (t *WriteFileTool) Preview(params map[string]interface{}) (string, error) {
	path := getStringParam(params, "path", "")
	content, ok := params["content"].(string)
	if path == "" {
		return "", fmt.Errorf("missing required parameter 'path'")
	}
	if !ok {
		return "", fmt.Errorf("missing required parameter 'content'")
	}
	if looksBinary([]byte(content)) {
		return "", fmt.Errorf("content appears to be binary")
	}
	return fmt.Sprintf("Will write file:\n%s\n(%d bytes)", path, len(content)), nil
}
func (t *WriteFileTool) Run(ctx context.Context, workspace string, params map[string]interface{}) (string, error) {
	path := getStringParam(params, "path", "")
	content, ok := params["content"].(string)
	if path == "" {
		return "", fmt.Errorf("missing required parameter 'path'")
	}
	if !ok {
		return "", fmt.Errorf("missing required parameter 'content'")
	}

	root := getRoot(workspace)
	fullPath, relPath, err := resolvePath(root, path)
	if err != nil {
		return "", err
	}
	if looksBinary([]byte(content)) {
		return "", fmt.Errorf("content appears to be binary")
	}

	overwrite := getBoolParam(params, "overwrite", false)
	if _, err := os.Stat(fullPath); err == nil && !overwrite {
		return "", fmt.Errorf("file %q already exists; set overwrite=true to replace it", relPath)
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to stat %q: %w", relPath, err)
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create parent directories for %q: %w", relPath, err)
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write %q: %w", relPath, err)
	}
	return fmt.Sprintf("Wrote file %s (%d bytes)", relPath, len(content)), nil
}

// EditFileTool replaces text in an existing file after approval.
type EditFileTool struct{}

func NewEditFileTool() *EditFileTool { return &EditFileTool{} }

func (t *EditFileTool) Name() string { return "edit_file" }
func (t *EditFileTool) Description() string {
	return "Edit an existing non-empty text file by replacing exact known text. Use write_file for empty files or full replacement."
}
func (t *EditFileTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]Parameter{
			"path":        {Type: "string", Description: "Relative file path (required)"},
			"old_text":    {Type: "string", Description: "Exact existing text to be replaced (required; read the file first if unknown)"},
			"new_text":    {Type: "string", Description: "Replacement text (required)"},
			"replace_all": {Type: "boolean", Description: "Replace all occurrences (default: false)"},
		},
		Required: []string{"path", "old_text", "new_text"},
	}
}

func (t *EditFileTool) RequiresApproval(params map[string]interface{}) bool { return true }
func (t *EditFileTool) Risk(params map[string]interface{}) approval.RiskLevel {
	return approval.RiskWrite
}
func (t *EditFileTool) Summary(params map[string]interface{}) string {
	return fmt.Sprintf("Edit file %s", getStringParam(params, "path", ""))
}
func (t *EditFileTool) Preview(params map[string]interface{}) (string, error) {
	path := getStringParam(params, "path", "")
	oldText, oldOK := params["old_text"].(string)
	newText, newOK := params["new_text"].(string)
	if path == "" {
		return "", fmt.Errorf("missing required parameter 'path'")
	}
	if !oldOK || oldText == "" {
		return "", fmt.Errorf("missing required parameter 'old_text'")
	}
	if !newOK {
		return "", fmt.Errorf("missing required parameter 'new_text'")
	}
	return fmt.Sprintf("Will edit file %s\nReplace: %q → %q", path, oldText, newText), nil
}
func (t *EditFileTool) Run(ctx context.Context, workspace string, params map[string]interface{}) (string, error) {
	path := getStringParam(params, "path", "")
	oldText, oldOK := params["old_text"].(string)
	newText, newOK := params["new_text"].(string)
	if path == "" {
		return "", fmt.Errorf("missing required parameter 'path'")
	}
	if !oldOK || oldText == "" {
		return "", fmt.Errorf("missing required parameter 'old_text'")
	}
	if !newOK {
		return "", fmt.Errorf("missing required parameter 'new_text'")
	}

	root := getRoot(workspace)
	fullPath, relPath, err := resolvePath(root, path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read %q: %w", relPath, err)
	}
	if looksBinary(data) {
		return "", fmt.Errorf("%q appears to be a binary file", relPath)
	}

	replaceAll := getBoolParam(params, "replace_all", false)
	newContent, replacements, err := replaceContent(string(data), oldText, newText, replaceAll)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write %q: %w", relPath, err)
	}
	return fmt.Sprintf("Edited file %s (%d replacement(s))", relPath, replacements), nil
}

func replaceContent(content, oldText, newText string, replaceAll bool) (string, int, error) {
	count := strings.Count(content, oldText)
	if count == 0 {
		return "", 0, fmt.Errorf("old_text was not found")
	}
	if replaceAll {
		return strings.ReplaceAll(content, oldText, newText), count, nil
	}
	return strings.Replace(content, oldText, newText, 1), 1, nil
}

func getBoolParam(params map[string]interface{}, key string, fallback bool) bool {
	value, ok := params[key]
	if !ok || value == nil {
		return fallback
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "yes")
	default:
		return fallback
	}
}
