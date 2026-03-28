package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/patricklbell/orktree/internal/state"
)

func TestCleanEmptyAncestors_removesIntermediateDirs(t *testing.T) {
	tmp := t.TempDir()
	// Simulate repo.orktree/feature/my-branch
	sib := tmp + ".orktree" // sibling dir
	leaf := filepath.Join(sib, "feature", "my-branch")
	if err := os.MkdirAll(leaf, 0o755); err != nil {
		t.Fatal(err)
	}

	cleanEmptyAncestors(leaf, sib)

	// feature/ should be gone
	if _, err := os.Stat(filepath.Join(sib, "feature")); !os.IsNotExist(err) {
		t.Error("expected feature/ to be removed")
	}
	// sib itself must remain
	if _, err := os.Stat(sib); err != nil {
		t.Errorf("sibling dir should still exist: %v", err)
	}
}

func TestCleanEmptyAncestors_stopsAtNonEmpty(t *testing.T) {
	tmp := t.TempDir()
	sib := tmp + ".orktree"
	leaf := filepath.Join(sib, "feature", "branch-a")
	other := filepath.Join(sib, "feature", "branch-b")
	if err := os.MkdirAll(leaf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}

	cleanEmptyAncestors(leaf, sib)

	// leaf is removed but feature/ stays (branch-b still there)
	if _, err := os.Stat(leaf); !os.IsNotExist(err) {
		t.Error("expected leaf to be removed")
	}
	if _, err := os.Stat(filepath.Join(sib, "feature")); err != nil {
		t.Error("feature/ should remain because branch-b is still there")
	}
}

func TestLoadFromCwd_fromSourceRoot(t *testing.T) {
	// Create a source root and init state
	src := t.TempDir()
	if _, err := state.Init(src, false); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Change to source root
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) })
	if err := os.Chdir(src); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadFromCwd()
	if err != nil {
		t.Fatalf("loadFromCwd: %v", err)
	}
	if cfg.SourceRoot != src {
		t.Errorf("SourceRoot = %q, want %q", cfg.SourceRoot, src)
	}
}

func TestLoadFromCwd_fromSubdir(t *testing.T) {
	src := t.TempDir()
	if _, err := state.Init(src, false); err != nil {
		t.Fatalf("Init: %v", err)
	}
	subdir := filepath.Join(src, "pkg", "server")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) })
	if err := os.Chdir(subdir); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadFromCwd()
	if err != nil {
		t.Fatalf("loadFromCwd from subdir: %v", err)
	}
	if cfg.SourceRoot != src {
		t.Errorf("SourceRoot = %q, want %q", cfg.SourceRoot, src)
	}
}

func TestLoadFromCwd_fromMergedView(t *testing.T) {
	src := t.TempDir()
	if _, err := state.Init(src, false); err != nil {
		t.Fatalf("Init: %v", err)
	}

	sib := state.SiblingDir(src)
	mergedPath := filepath.Join(sib, "mybranch")
	if err := os.MkdirAll(mergedPath, 0o755); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) })
	if err := os.Chdir(mergedPath); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadFromCwd()
	if err != nil {
		t.Fatalf("loadFromCwd from merged view: %v", err)
	}
	if cfg.SourceRoot != src {
		t.Errorf("SourceRoot = %q, want %q", cfg.SourceRoot, src)
	}
}

func TestLoadFromCwd_fromMergedViewSlashBranch(t *testing.T) {
	src := t.TempDir()
	if _, err := state.Init(src, false); err != nil {
		t.Fatalf("Init: %v", err)
	}

	sib := state.SiblingDir(src)
	// Branch "feature/my-branch" → nested dir; cd into a subdirectory of it
	mergedPath := filepath.Join(sib, "feature", "my-branch", "src")
	if err := os.MkdirAll(mergedPath, 0o755); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) })
	if err := os.Chdir(mergedPath); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadFromCwd()
	if err != nil {
		t.Fatalf("loadFromCwd from slash-branch merged view: %v", err)
	}
	if cfg.SourceRoot != src {
		t.Errorf("SourceRoot = %q, want %q", cfg.SourceRoot, src)
	}
}

func TestLoadFromCwd_noState(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	_, err := loadFromCwd()
	if err == nil {
		t.Fatal("expected error when no state exists")
	}
	if !strings.Contains(err.Error(), "no .orktree") {
		t.Errorf("unexpected error: %v", err)
	}
}
