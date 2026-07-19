package workspacebrowse

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBrowseListsDirectoriesFirst(t *testing.T) {
	root := t.TempDir()

	if err := os.Mkdir(filepath.Join(root, "beta"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "alpha"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	listing, err := Browse(root, []Root{{Path: root, Name: "root", Kind: "test"}})
	if err != nil {
		t.Fatal(err)
	}

	expectedPath := normalizePath(root)
	if listing.Path != expectedPath {
		t.Fatalf("expected listing path %q, got %q", expectedPath, listing.Path)
	}
	if listing.ParentPath != "" {
		t.Fatalf("expected no parent path above root, got %q", listing.ParentPath)
	}
	if got := len(listing.Entries); got != 3 {
		t.Fatalf("expected 3 entries, got %d", got)
	}
	if listing.Entries[0].Name != "alpha" || !listing.Entries[0].IsDir {
		t.Fatalf("expected first entry to be alpha directory, got %#v", listing.Entries[0])
	}
	if listing.Entries[1].Name != "beta" || !listing.Entries[1].IsDir {
		t.Fatalf("expected second entry to be beta directory, got %#v", listing.Entries[1])
	}
	if listing.Entries[2].Name != "note.txt" || listing.Entries[2].IsDir {
		t.Fatalf("expected third entry to be note.txt file, got %#v", listing.Entries[2])
	}
}

func TestBrowseRejectsPathsOutsideRoots(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	_, err := Browse(outside, []Root{{Path: root, Name: "root", Kind: "test"}})
	if err == nil {
		t.Fatal("expected browse outside roots to fail")
	}
}

func TestRootsDeduplicatesRecentPaths(t *testing.T) {
	home := t.TempDir()
	oldHome, hadHome := os.LookupEnv("HOME")
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatal(err)
	}
	if hadHome {
		defer func() { _ = os.Setenv("HOME", oldHome) }()
	} else {
		defer func() { _ = os.Unsetenv("HOME") }()
	}

	roots, err := Roots([]string{home, home})
	if err != nil {
		t.Fatal(err)
	}

	if len(roots) == 0 {
		t.Fatal("expected at least one root")
	}
	seen := map[string]bool{}
	for _, root := range roots {
		if seen[root.Path] {
			t.Fatalf("duplicate root path returned: %q", root.Path)
		}
		seen[root.Path] = true
	}
}
