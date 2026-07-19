package workspacebrowse

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Root struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type Entry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
}

type Listing struct {
	Path       string  `json:"path"`
	ParentPath string  `json:"parentPath,omitempty"`
	Roots      []Root  `json:"roots,omitempty"`
	Entries    []Entry `json:"entries"`
}

func Roots(recentPaths []string) ([]Root, error) {
	roots := make([]Root, 0, 4+len(recentPaths))
	seen := map[string]bool{}

	add := func(path, name, kind string) {
		clean := normalizePath(path)
		if clean == "" || seen[clean] {
			return
		}
		seen[clean] = true
		roots = append(roots, Root{
			Path: clean,
			Name: name,
			Kind: kind,
		})
	}

	if cwd, err := os.Getwd(); err == nil {
		add(cwd, displayName(cwd, "Current folder"), "cwd")
	}
	if home, err := os.UserHomeDir(); err == nil {
		add(home, displayName(home, "Home"), "home")
	}
	for _, recent := range recentPaths {
		add(recent, displayName(recent, "Recent project"), "recent")
	}

	return roots, nil
}

func Browse(path string, roots []Root) (Listing, error) {
	if len(roots) == 0 {
		var err error
		roots, err = Roots(nil)
		if err != nil {
			return Listing{}, err
		}
	}

	clean := normalizePath(path)
	if clean == "" {
		if len(roots) == 0 {
			return Listing{}, fmt.Errorf("no workspace roots are available")
		}
		clean = roots[0].Path
	}

	if !isWithinAnyRoot(clean, roots) {
		return Listing{}, fmt.Errorf("directory %q is outside the exposed workspace roots", clean)
	}

	info, err := os.Stat(clean)
	if err != nil {
		return Listing{}, fmt.Errorf("failed to read directory %q: %w", clean, err)
	}
	if !info.IsDir() {
		return Listing{}, fmt.Errorf("%q is not a directory", clean)
	}

	entries, err := os.ReadDir(clean)
	if err != nil {
		return Listing{}, fmt.Errorf("failed to list directory %q: %w", clean, err)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})

	result := Listing{
		Path:    clean,
		Roots:   nil,
		Entries: make([]Entry, 0, len(entries)),
	}
	parent := filepath.Dir(clean)
	if parent != clean && isWithinAnyRoot(parent, roots) {
		result.ParentPath = parent
	}
	for _, entry := range entries {
		full := filepath.Join(clean, entry.Name())
		result.Entries = append(result.Entries, Entry{
			Name:  entry.Name(),
			Path:  full,
			IsDir: entry.IsDir(),
		})
	}
	return result, nil
}

func isWithinAnyRoot(path string, roots []Root) bool {
	clean := normalizePath(path)
	for _, root := range roots {
		if isWithinRoot(clean, root.Path) {
			return true
		}
	}
	return false
}

func isWithinRoot(path, root string) bool {
	cleanPath := normalizePath(path)
	cleanRoot := normalizePath(root)
	if cleanPath == "" || cleanRoot == "" {
		return false
	}
	return cleanPath == cleanRoot || strings.HasPrefix(cleanPath, cleanRoot+string(filepath.Separator))
}

func displayName(path, fallback string) string {
	if path == "" {
		return fallback
	}
	name := filepath.Base(path)
	if name == "." || name == string(filepath.Separator) || name == "" {
		return fallback
	}
	return name
}

func normalizePath(path string) string {
	clean := strings.TrimSpace(path)
	if clean == "" {
		return ""
	}
	if !filepath.IsAbs(clean) {
		abs, err := filepath.Abs(clean)
		if err != nil {
			return ""
		}
		clean = abs
	}
	clean = filepath.Clean(clean)
	if resolved, err := filepath.EvalSymlinks(clean); err == nil {
		clean = resolved
	}
	return clean
}
