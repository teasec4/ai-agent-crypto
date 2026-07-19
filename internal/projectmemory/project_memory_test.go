package projectmemory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadMissingMemoryReturnsEmpty(t *testing.T) {
	got, err := Read(t.TempDir())
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty memory for missing file, got %q", got)
	}
}

func TestReadLoadsWorkspaceMemory(t *testing.T) {
	dir := t.TempDir()
	memoryPath := filepath.Join(dir, RelativePath)
	if err := os.MkdirAll(filepath.Dir(memoryPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(memoryPath, []byte("\n# Memory\n- answer in Russian\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := Read(dir)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got != "# Memory\n- answer in Russian" {
		t.Fatalf("unexpected memory: %q", got)
	}
}

func TestReadTruncatesLargeMemory(t *testing.T) {
	dir := t.TempDir()
	memoryPath := filepath.Join(dir, RelativePath)
	if err := os.MkdirAll(filepath.Dir(memoryPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(memoryPath, []byte(strings.Repeat("x", MaxBytes+100)), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := Read(dir)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if len(got) != MaxBytes {
		t.Fatalf("expected %d bytes, got %d", MaxBytes, len(got))
	}
}
