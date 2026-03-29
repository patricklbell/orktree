package overlay

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCreate_createsDirectories(t *testing.T) {
	base := t.TempDir()
	upper := filepath.Join(base, "upper")
	work := filepath.Join(base, "work")
	merged := filepath.Join(base, "merged")

	if err := Create(upper, work, merged); err != nil {
		t.Fatalf("Create: %v", err)
	}

	for _, dir := range []string{upper, work, merged} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("directory %s not created: %v", dir, err)
		} else if !info.IsDir() {
			t.Errorf("%s is not a directory", dir)
		}
	}
}

func TestIsMounted_falseForPlainDir(t *testing.T) {
	dir := t.TempDir()

	mounted, err := IsMounted(dir)
	if err != nil {
		t.Fatalf("IsMounted: %v", err)
	}
	if mounted {
		t.Error("expected IsMounted to return false for a plain directory")
	}
}

func TestDirtyUpperFiles_identicalFileNotDirty(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()
	content := []byte("hello world")
	os.WriteFile(filepath.Join(upper, "file.txt"), content, 0o644)
	os.WriteFile(filepath.Join(lower, "file.txt"), content, 0o644)

	dirty, err := DirtyUpperFiles(upper, lower, 100)
	if err != nil {
		t.Fatalf("DirtyUpperFiles: %v", err)
	}
	if len(dirty) != 0 {
		t.Errorf("expected no dirty files, got %v", dirty)
	}
}

func TestDirtyUpperFiles_differentFileIsDirty(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()
	os.WriteFile(filepath.Join(upper, "file.txt"), []byte("modified"), 0o644)
	os.WriteFile(filepath.Join(lower, "file.txt"), []byte("original"), 0o644)

	dirty, err := DirtyUpperFiles(upper, lower, 100)
	if err != nil {
		t.Fatalf("DirtyUpperFiles: %v", err)
	}
	if len(dirty) != 1 || dirty[0] != "file.txt" {
		t.Errorf("expected [file.txt], got %v", dirty)
	}
}

func TestDirtyUpperFiles_newFileIsDirty(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()
	os.WriteFile(filepath.Join(upper, "newfile.txt"), []byte("new"), 0o644)

	dirty, err := DirtyUpperFiles(upper, lower, 100)
	if err != nil {
		t.Fatalf("DirtyUpperFiles: %v", err)
	}
	if len(dirty) != 1 || dirty[0] != "newfile.txt" {
		t.Errorf("expected [newfile.txt], got %v", dirty)
	}
}

func TestDirtyUpperFiles_gitDirSkipped(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()
	gitDir := filepath.Join(upper, ".git")
	os.MkdirAll(gitDir, 0o755)
	os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main"), 0o644)

	dirty, err := DirtyUpperFiles(upper, lower, 100)
	if err != nil {
		t.Fatalf("DirtyUpperFiles: %v", err)
	}
	if len(dirty) != 0 {
		t.Errorf("expected no dirty files, got %v", dirty)
	}
}

func TestDirtyUpperFiles_limitRespected(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("file%d.txt", i)
		os.WriteFile(filepath.Join(upper, name), []byte("new"), 0o644)
	}

	dirty, err := DirtyUpperFiles(upper, lower, 3)
	if err != nil {
		t.Fatalf("DirtyUpperFiles: %v", err)
	}
	if len(dirty) != 3 {
		t.Errorf("expected 3 dirty files, got %d", len(dirty))
	}
}

func TestDirtyUpperFiles_emptyUpper(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()

	dirty, err := DirtyUpperFiles(upper, lower, 100)
	if err != nil {
		t.Fatalf("DirtyUpperFiles: %v", err)
	}
	if len(dirty) != 0 {
		t.Errorf("expected no dirty files, got %v", dirty)
	}
}

func TestDirtyUpperFiles_sameSizeDifferentContent(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()
	os.WriteFile(filepath.Join(upper, "file.txt"), []byte("aaaa"), 0o644)
	os.WriteFile(filepath.Join(lower, "file.txt"), []byte("bbbb"), 0o644)

	dirty, err := DirtyUpperFiles(upper, lower, 100)
	if err != nil {
		t.Fatalf("DirtyUpperFiles: %v", err)
	}
	if len(dirty) != 1 || dirty[0] != "file.txt" {
		t.Errorf("expected [file.txt], got %v", dirty)
	}
}
