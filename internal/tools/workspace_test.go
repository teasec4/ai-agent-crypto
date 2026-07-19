package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePath_Basic(t *testing.T) {
	dir := t.TempDir()
	full, rel, err := resolvePath(dir, ".")
	if err != nil {
		t.Fatalf("resolvePath('.') failed: %v", err)
	}
	if rel != "." {
		t.Fatalf("expected rel='.', got %q", rel)
	}
	want, _ := resolveRoot(dir)
	if full != want {
		t.Fatalf("expected full=%q, got %q", want, full)
	}
}

func TestResolvePath_Escapes(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		path string
	}{
		{".."},
		{"../"},
		{"../etc"},
		{"/etc"},
		{"/"},
	}
	for _, tt := range tests {
		_, _, err := resolvePath(dir, tt.path)
		if err == nil {
			t.Errorf("resolvePath(%q) should have failed", tt.path)
		}
	}
}

func TestResolvePath_Blocked(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)
	os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("test"), 0644)

	_, _, err := resolvePath(dir, ".git/config")
	if err == nil {
		t.Error("resolvePath('.git/config') should be blocked")
	}
}

func TestResolvePath_SymlinkPrevention(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "subdir")
	if err := os.Symlink(dir, link); err != nil {
		t.Skip("symlink not supported")
	}

	full, rel, err := resolvePath(dir, "subdir")
	if err != nil {
		t.Fatalf("resolvePath('subdir') should work: %v", err)
	}
	want, _ := resolveRoot(dir)
	want = filepath.Join(want, "subdir")
	if full != want {
		t.Fatalf("expected full=%q, got %q", want, full)
	}
	if rel != "subdir" {
		t.Fatalf("expected 'subdir', got %q", rel)
	}
}

func TestResolvePath_SymlinkOutsideWorkspace(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()

	link := filepath.Join(workspace, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skip("symlink not supported")
	}

	_, _, err := resolvePath(workspace, "escape")
	if err == nil {
		t.Error("symlink pointing outside workspace should be blocked")
	}
}

func TestCommandTool_Allowlist(t *testing.T) {
	tests := []struct {
		cmd  string
		args []string
		ok   bool
	}{
		{"ls", []string{"-la"}, true},
		{"ls", []string{"-lah", "."}, true},
		{"go", []string{"version"}, true},
		{"go", []string{"test", "-v", "./..."}, true},
		{"go", []string{"test", "-run", "TestX"}, true},
		{"git", []string{"status"}, true},
		{"git", []string{"diff", "--cached"}, true},
		{"git", []string{"push"}, false},
		{"pwd", []string{}, true},
		{"pwd", []string{"-P"}, false},
		{"rm", []string{"-rf", "/"}, false},
		{"bash", []string{"-c", "rm /"}, false},
		{"go", []string{"get", "./..."}, false},
	}

	for _, tt := range tests {
		err := validateAllowedCommand(tt.cmd, tt.args)
		if tt.ok && err != nil {
			t.Errorf("validateAllowedCommand(%q, %v) should pass, got: %v", tt.cmd, tt.args, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("validateAllowedCommand(%q, %v) should fail", tt.cmd, tt.args)
		}
	}
}

func TestReadFileTool_Run(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello world\nline 2"), 0644)

	tool := &ReadFileTool{}
	result, err := tool.Run(context.Background(), dir, map[string]interface{}{
		"path": "test.txt",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !strings.Contains(result, "hello world") {
		t.Errorf("expected 'hello world' in result, got:\n%s", result)
	}
}

func TestReadFileTool_Blocked(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)
	os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("test"), 0644)

	tool := &ReadFileTool{}
	_, err := tool.Run(context.Background(), dir, map[string]interface{}{
		"path": ".git/config",
	})
	if err == nil {
		t.Error("reading .git/config should be blocked")
	}
}

func TestDeletePathTool_RunDeletesDirectory(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "scratch", "nested")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "note.txt"), []byte("temporary"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := &DeletePathTool{}
	result, err := tool.Run(context.Background(), dir, map[string]interface{}{
		"path": "scratch",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !strings.Contains(result, "Deleted directory scratch") {
		t.Fatalf("unexpected result: %s", result)
	}
	if _, err := os.Stat(filepath.Join(dir, "scratch")); !os.IsNotExist(err) {
		t.Fatalf("expected scratch directory to be deleted, stat err=%v", err)
	}
}

func TestDeletePathTool_BlocksWorkspaceRoot(t *testing.T) {
	dir := t.TempDir()
	tool := &DeletePathTool{}

	_, err := tool.Run(context.Background(), dir, map[string]interface{}{
		"path": ".",
	})
	if err == nil {
		t.Fatal("deleting workspace root should fail")
	}
}
