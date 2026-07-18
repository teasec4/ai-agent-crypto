package tools

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	defaultMaxReadBytes    = 80 * 1024
	defaultMaxListEntries  = 200
	defaultMaxFindMatches  = 200
	defaultMaxSearchHits   = 100
	defaultMaxSearchBytes  = 2 * 1024 * 1024
	defaultSearchLineLimit = 240
)

var blockedWorkspaceNames = map[string]bool{
	".env":       true,
	".env.local": true,
	".git":       true,
}

// resolvePath resolves a relative path inside root.
// It returns (resolved-full-path, relative-slash-path, error).
// It follows symlinks on root and prevents symlink traversal outside root.
func resolvePath(root, rawPath string) (full string, rel string, err error) {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		path = "."
	}
	if filepath.IsAbs(path) {
		return "", "", fmt.Errorf("absolute paths are not allowed")
	}

	clean := filepath.Clean(path)
	if clean == "." {
		rootResolved, err := resolveRoot(root)
		if err != nil {
			return "", "", err
		}
		return rootResolved, ".", nil
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("path escapes workspace")
	}
	if isBlockedWorkspacePath(clean) {
		return "", "", fmt.Errorf("access to %q is blocked", clean)
	}

	rootResolved, err := resolveRoot(root)
	if err != nil {
		return "", "", err
	}

	full = filepath.Join(rootResolved, clean)
	fullClean := filepath.Clean(full)

	if fullClean != rootResolved && !strings.HasPrefix(fullClean, rootResolved+string(filepath.Separator)) {
		return "", "", fmt.Errorf("path escapes workspace")
	}

	// Resolve symlinks on the final path as an extra safety check.
	if real, statErr := filepath.EvalSymlinks(fullClean); statErr == nil {
		if real != rootResolved && !strings.HasPrefix(real, rootResolved+string(filepath.Separator)) {
			return "", "", fmt.Errorf("symlink %q points outside the workspace", clean)
		}
	}

	return fullClean, filepath.ToSlash(clean), nil
}

// resolveRoot cleans and resolves symlinks on the workspace root.
func resolveRoot(root string) (string, error) {
	clean := filepath.Clean(root)
	if clean == "." {
		cwd, err := os.Getwd()
		if err != nil {
			return ".", nil
		}
		clean = cwd
	}
	resolved, err := filepath.EvalSymlinks(clean)
	if err != nil {
		return clean, nil // best-effort: use cleaned path if EvalSymlinks fails
	}
	return resolved, nil
}

func isBlockedWorkspacePath(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, part := range parts {
		if blockedWorkspaceNames[part] {
			return true
		}
	}
	return false
}

func getStringParam(params map[string]interface{}, key string, fallback string) string {
	if value, ok := params[key].(string); ok && strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func getIntParam(params map[string]interface{}, key string, fallback int, min int, max int) int {
	value, ok := params[key]
	if !ok || value == nil {
		return fallback
	}

	var parsed int
	switch v := value.(type) {
	case int:
		parsed = v
	case int64:
		parsed = int(v)
	case float64:
		parsed = int(v)
	case jsonNumber:
		if i, err := v.Int64(); err == nil {
			parsed = int(i)
		} else {
			return fallback
		}
	default:
		return fallback
	}

	if parsed < min {
		return min
	}
	if parsed > max {
		return max
	}
	return parsed
}

type jsonNumber interface {
	Int64() (int64, error)
}

// ---- helpers that used to be on workspaceTool but don't need a receiver ----

// getRoot returns the effective root directory.
func getRoot(workspace string) string {
	if workspace != "" {
		return workspace
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

// ------------------- ListDirectoryTool -------------------

type ListDirectoryTool struct{}

func NewListDirectoryTool() *ListDirectoryTool { return &ListDirectoryTool{} }

func (t *ListDirectoryTool) Name() string { return "list_directory" }
func (t *ListDirectoryTool) Description() string {
	return "List files and directories in the workspace."
}
func (t *ListDirectoryTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]Parameter{
			"path":        {Type: "string", Description: "Relative directory path (default: .)"},
			"max_entries": {Type: "integer", Description: "Max entries to return (default: 200)"},
		},
	}
}
func (t *ListDirectoryTool) Run(ctx context.Context, workspace string, params map[string]interface{}) (string, error) {
	path := getStringParam(params, "path", ".")
	maxEntries := getIntParam(params, "max_entries", defaultMaxListEntries, 1, 1000)

	root := getRoot(workspace)
	fullPath, relPath, err := resolvePath(root, path)
	if err != nil {
		return "", err
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to list %q: %w", relPath, err)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return entries[i].Name() < entries[j].Name()
	})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Directory: %s\n", relPath))
	count := 0
	for _, entry := range entries {
		if count >= maxEntries {
			remaining := len(entries) - count
			if remaining > 0 {
				sb.WriteString(fmt.Sprintf("... %d more entries omitted\n", remaining))
			}
			break
		}
		if isBlockedWorkspacePath(entry.Name()) {
			continue
		}
		marker := "file"
		if entry.IsDir() {
			marker = "dir"
		}
		sb.WriteString(fmt.Sprintf("- [%s] %s\n", marker, entry.Name()))
		count++
	}

	return strings.TrimRight(sb.String(), "\n"), nil
}

// ------------------- ReadFileTool -------------------

type ReadFileTool struct{}

func NewReadFileTool() *ReadFileTool { return &ReadFileTool{} }

func (t *ReadFileTool) Name() string { return "read_file" }
func (t *ReadFileTool) Description() string {
	return "Read a text file from the workspace."
}
func (t *ReadFileTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]Parameter{
			"path":      {Type: "string", Description: "Relative file path (required)"},
			"max_bytes": {Type: "integer", Description: "Max bytes to read (default: 81920)"},
		},
		Required: []string{"path"},
	}
}
func (t *ReadFileTool) Run(ctx context.Context, workspace string, params map[string]interface{}) (string, error) {
	path := getStringParam(params, "path", "")
	if path == "" {
		return "", fmt.Errorf("missing required parameter 'path'")
	}
	maxBytes := getIntParam(params, "max_bytes", defaultMaxReadBytes, 1, 512*1024)

	root := getRoot(workspace)
	fullPath, relPath, err := resolvePath(root, path)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat %q: %w", relPath, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%q is a directory; use list_directory", relPath)
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to open %q: %w", relPath, err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, int64(maxBytes)+1))
	if err != nil {
		return "", fmt.Errorf("failed to read %q: %w", relPath, err)
	}

	truncated := false
	if len(data) > maxBytes {
		data = data[:maxBytes]
		truncated = true
	}
	if looksBinary(data) {
		return "", fmt.Errorf("%q appears to be a binary file", relPath)
	}

	content := string(data)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("File: %s\n", relPath))
	if truncated {
		sb.WriteString(fmt.Sprintf("Note: truncated to %d bytes.\n", maxBytes))
	}
	sb.WriteString("---\n")
	for i, line := range strings.Split(content, "\n") {
		sb.WriteString(fmt.Sprintf("%5d\t%s\n", i+1, line))
	}

	return strings.TrimRight(sb.String(), "\n"), nil
}

func looksBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	check := len(data)
	if check > 8000 {
		check = 8000
	}
	for i := 0; i < check; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

// ------------------- FindFilesTool -------------------

type FindFilesTool struct{}

func NewFindFilesTool() *FindFilesTool { return &FindFilesTool{} }

func (t *FindFilesTool) Name() string { return "find_files" }
func (t *FindFilesTool) Description() string {
	return "Find workspace files by glob pattern."
}
func (t *FindFilesTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]Parameter{
			"pattern":     {Type: "string", Description: "Glob pattern (e.g. **/*.go) (required)"},
			"max_matches": {Type: "integer", Description: "Max matches to return (default: 200)"},
		},
		Required: []string{"pattern"},
	}
}
func (t *FindFilesTool) Run(ctx context.Context, workspace string, params map[string]interface{}) (string, error) {
	pattern := getStringParam(params, "pattern", "")
	if pattern == "" {
		return "", fmt.Errorf("missing required parameter 'pattern'")
	}
	if filepath.IsAbs(pattern) || isPathTraversal(pattern) || isBlockedWorkspacePath(pattern) {
		return "", fmt.Errorf("invalid or blocked pattern %q", pattern)
	}
	maxMatches := getIntParam(params, "max_matches", defaultMaxFindMatches, 1, 1000)

	root := getRoot(workspace)
	matches := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() && blockedWorkspaceNames[name] {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if isBlockedWorkspacePath(rel) {
			return nil
		}

		matched, err := doublestarMatch(filepath.ToSlash(pattern), rel)
		if err != nil {
			return err
		}
		if matched {
			matches = append(matches, rel)
			if len(matches) >= maxMatches {
				return errStopWalk
			}
		}
		return nil
	})
	if err != nil && err != errStopWalk {
		return "", fmt.Errorf("find failed: %w", err)
	}

	sort.Strings(matches)
	if len(matches) == 0 {
		return fmt.Sprintf("No files matched pattern %q", pattern), nil
	}
	return "Matched files:\n- " + strings.Join(matches, "\n- "), nil
}

var errStopWalk = fmt.Errorf("stop walk")

func doublestarMatch(pattern, name string) (bool, error) {
	if !strings.Contains(pattern, "**") {
		return filepath.Match(pattern, name)
	}

	segments := strings.Split(pattern, "/")
	nameSegments := strings.Split(name, "/")
	return matchPathSegments(segments, nameSegments)
}

func matchPathSegments(patternSegments, nameSegments []string) (bool, error) {
	if len(patternSegments) == 0 {
		return len(nameSegments) == 0, nil
	}

	segment := patternSegments[0]
	if segment == "**" {
		if len(patternSegments) == 1 {
			return true, nil
		}
		for i := 0; i <= len(nameSegments); i++ {
			matched, err := matchPathSegments(patternSegments[1:], nameSegments[i:])
			if err != nil || matched {
				return matched, err
			}
		}
		return false, nil
	}

	if len(nameSegments) == 0 {
		return false, nil
	}
	matched, err := filepath.Match(segment, nameSegments[0])
	if err != nil || !matched {
		return matched, err
	}
	return matchPathSegments(patternSegments[1:], nameSegments[1:])
}

// ------------------- SearchTextTool -------------------

type SearchTextTool struct{}

func NewSearchTextTool() *SearchTextTool { return &SearchTextTool{} }

func (t *SearchTextTool) Name() string { return "search_text" }
func (t *SearchTextTool) Description() string {
	return "Search text in workspace files."
}
func (t *SearchTextTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]Parameter{
			"query":    {Type: "string", Description: "Plain text substring to search (required)"},
			"path":     {Type: "string", Description: "Directory to search (default: .)"},
			"max_hits": {Type: "integer", Description: "Max results to return (default: 100)"},
		},
		Required: []string{"query"},
	}
}
func (t *SearchTextTool) Run(ctx context.Context, workspace string, params map[string]interface{}) (string, error) {
	query := getStringParam(params, "query", "")
	if query == "" {
		return "", fmt.Errorf("missing required parameter 'query'")
	}
	path := getStringParam(params, "path", ".")
	maxHits := getIntParam(params, "max_hits", defaultMaxSearchHits, 1, 500)

	root := getRoot(workspace)
	fullPath, relRoot, err := resolvePath(root, path)
	if err != nil {
		return "", err
	}

	hits := make([]string, 0)
	err = filepath.WalkDir(fullPath, func(path string, d fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			return nil
		}
		if d.IsDir() && blockedWorkspaceNames[d.Name()] {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if isBlockedWorkspacePath(rel) {
			return nil
		}

		info, err := d.Info()
		if err != nil || info.Size() > defaultMaxSearchBytes {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || looksBinary(data) {
			return nil
		}

		lines := strings.Split(string(data), "\n")
		for index, line := range lines {
			if !strings.Contains(line, query) {
				continue
			}
			trimmed := strings.TrimSpace(line)
			if len(trimmed) > defaultSearchLineLimit {
				trimmed = trimmed[:defaultSearchLineLimit] + "..."
			}
			hits = append(hits, fmt.Sprintf("%s:%d: %s", rel, index+1, trimmed))
			if len(hits) >= maxHits {
				return errStopWalk
			}
		}
		return nil
	})
	if err != nil && err != errStopWalk {
		return "", fmt.Errorf("search failed: %w", err)
	}

	if len(hits) == 0 {
		return fmt.Sprintf("No matches for %q under %s", query, relRoot), nil
	}
	return fmt.Sprintf("Search results for %q under %s:\n", query, relRoot) + strings.Join(hits, "\n"), nil
}
