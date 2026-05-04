package tools

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type GrepTool struct {
	Root        string
	MaxResults  int
	MaxFileSize int64
}

func (t *GrepTool) Name() string {
	return "grep"
}

func (t *GrepTool) Description() string {
	return "Search for a string in files"
}

func (t GrepTool) Run(params map[string]interface{}) (string, error) {
	query, ok := params["query"].(string)
	if !ok || query == "" {
		return "", fmt.Errorf("query required")
	}
	var results []string
	err := filepath.WalkDir(t.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		info, _ := d.Info()
		if info.Size() > t.MaxFileSize {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if strings.Contains(line, query) {
				rel, _ := filepath.Rel(t.Root, path)
				results = append(results,
					fmt.Sprintf("%s:%d: %s", rel, lineNum, line))
				if len(results) >= t.MaxResults {
					return fmt.Errorf("limit reached")
				}
			}
		}
		return nil
	})
	if err != nil && err.Error() != "limit reached" {
		return "", err
	}
	return strings.Join(results, "\n"), nil

}
