package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-agent/internal/projectmemory"
)

func TestReadProjectMemoryTool(t *testing.T) {
	dir := t.TempDir()
	memoryPath := filepath.Join(dir, projectmemory.RelativePath)
	if err := os.MkdirAll(filepath.Dir(memoryPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(memoryPath, []byte("## Decisions\n- Use SSE approval\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := NewReadProjectMemoryTool().Run(context.Background(), dir, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !strings.Contains(result, "Use SSE approval") {
		t.Fatalf("expected memory content in result, got %s", result)
	}
}

func TestProposeMemoryUpdateToolDoesNotWriteFile(t *testing.T) {
	dir := t.TempDir()
	result, err := NewProposeMemoryUpdateTool().Run(context.Background(), dir, map[string]interface{}{
		"section": "User Preferences",
		"entry":   "Answer in Russian.",
		"reason":  "User requested Russian responses.",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !strings.Contains(result, "Entry: - Answer in Russian.") {
		t.Fatalf("unexpected proposal: %s", result)
	}
	if _, err := os.Stat(filepath.Join(dir, projectmemory.RelativePath)); !os.IsNotExist(err) {
		t.Fatalf("proposal tool must not write memory file, stat err=%v", err)
	}
}
