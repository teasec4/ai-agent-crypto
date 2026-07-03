package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ai-agent/internal/approval"
)

const writePreviewLimit = 6000

// CreateDirectoryTool creates a directory inside the workspace after approval.
type CreateDirectoryTool struct {
	workspaceTool
}

func NewCreateDirectoryTool() *CreateDirectoryTool {
	return &CreateDirectoryTool{workspaceTool: newWorkspaceTool()}
}

func (t *CreateDirectoryTool) Name() string {
	return "create_directory"
}

func (t *CreateDirectoryTool) Description() string {
	return "Create a directory in the workspace. Parameters: path (relative directory path). Requires user approval."
}

func (t *CreateDirectoryTool) RequiresApproval(params map[string]interface{}) bool {
	return true
}

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
	_, relPath, err := t.resolvePath(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Will create directory:\n%s", relPath), nil
}

func (t *CreateDirectoryTool) Run(params map[string]interface{}) (string, error) {
	path := getStringParam(params, "path", "")
	if path == "" {
		return "", fmt.Errorf("missing required parameter 'path'")
	}
	fullPath, relPath, err := t.resolvePath(path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %q: %w", relPath, err)
	}
	return fmt.Sprintf("Created directory %s", relPath), nil
}

// WriteFileTool writes a text file inside the workspace after approval.
type WriteFileTool struct {
	workspaceTool
}

func NewWriteFileTool() *WriteFileTool {
	return &WriteFileTool{workspaceTool: newWorkspaceTool()}
}

func (t *WriteFileTool) Name() string {
	return "write_file"
}

func (t *WriteFileTool) Description() string {
	return "Create or overwrite a text file in the workspace. Parameters: path, content, overwrite (default false), create_parents (default false). Requires user approval."
}

func (t *WriteFileTool) RequiresApproval(params map[string]interface{}) bool {
	return true
}

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
	fullPath, relPath, err := t.resolvePath(path)
	if err != nil {
		return "", err
	}
	if looksBinary([]byte(content)) {
		return "", fmt.Errorf("content appears to be binary")
	}

	oldContent := ""
	if data, err := os.ReadFile(fullPath); err == nil {
		oldContent = string(data)
	}

	return buildTextPreview(relPath, oldContent, content), nil
}

func (t *WriteFileTool) Run(params map[string]interface{}) (string, error) {
	path := getStringParam(params, "path", "")
	content, ok := params["content"].(string)
	if path == "" {
		return "", fmt.Errorf("missing required parameter 'path'")
	}
	if !ok {
		return "", fmt.Errorf("missing required parameter 'content'")
	}
	fullPath, relPath, err := t.resolvePath(path)
	if err != nil {
		return "", err
	}
	if looksBinary([]byte(content)) {
		return "", fmt.Errorf("content appears to be binary")
	}

	overwrite := getBoolParam(params, "overwrite", false)
	createParents := getBoolParam(params, "create_parents", false)
	if _, err := os.Stat(fullPath); err == nil && !overwrite {
		return "", fmt.Errorf("file %q already exists; set overwrite=true to replace it", relPath)
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to stat %q: %w", relPath, err)
	}

	if createParents {
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return "", fmt.Errorf("failed to create parent directories for %q: %w", relPath, err)
		}
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write %q: %w", relPath, err)
	}
	return fmt.Sprintf("Wrote file %s (%d bytes)", relPath, len(content)), nil
}

// EditFileTool replaces text in an existing file after approval.
type EditFileTool struct {
	workspaceTool
}

func NewEditFileTool() *EditFileTool {
	return &EditFileTool{workspaceTool: newWorkspaceTool()}
}

func (t *EditFileTool) Name() string {
	return "edit_file"
}

func (t *EditFileTool) Description() string {
	return "Edit an existing text file by replacing old_text with new_text. Parameters: path, old_text, new_text, replace_all (default false). Requires user approval."
}

func (t *EditFileTool) RequiresApproval(params map[string]interface{}) bool {
	return true
}

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

	fullPath, relPath, err := t.resolvePath(path)
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

	oldContent := string(data)
	newContent, replacements, err := replaceContent(oldContent, oldText, newText, getBoolParam(params, "replace_all", false))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Replacements: %d\n%s", replacements, buildTextPreview(relPath, oldContent, newContent)), nil
}

func (t *EditFileTool) Run(params map[string]interface{}) (string, error) {
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

	fullPath, relPath, err := t.resolvePath(path)
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

	newContent, replacements, err := replaceContent(string(data), oldText, newText, getBoolParam(params, "replace_all", false))
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

func buildTextPreview(path string, oldContent string, newContent string) string {
	oldSnippet := trimPreview(oldContent)
	newSnippet := trimPreview(newContent)
	return fmt.Sprintf("File: %s\n--- before\n%s\n--- after\n%s", path, oldSnippet, newSnippet)
}

func trimPreview(content string) string {
	if content == "" {
		return "<empty or new file>"
	}
	if len(content) <= writePreviewLimit {
		return content
	}
	return content[:writePreviewLimit] + fmt.Sprintf("\n... truncated to %d bytes", writePreviewLimit)
}
