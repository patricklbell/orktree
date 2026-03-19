package state_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/patricklbell/janus/internal/state"
)

func TestInitCreatesStateFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := state.Init(dir, "ubuntu:24.04", false)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if cfg.SourceRoot != dir {
		t.Errorf("SourceRoot = %q, want %q", cfg.SourceRoot, dir)
	}
	if cfg.Image != "ubuntu:24.04" {
		t.Errorf("Image = %q, want %q", cfg.Image, "ubuntu:24.04")
	}

	path := state.StatePath(dir)
	if _, err := os.Stat(path); err != nil {
		t.Errorf("state file not created at %s: %v", path, err)
	}
}

func TestInitDefaultImage(t *testing.T) {
	dir := t.TempDir()
	cfg, err := state.Init(dir, state.DefaultImage, false)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if cfg.Image != state.DefaultImage {
		t.Errorf("Image = %q, want %q", cfg.Image, state.DefaultImage)
	}
}

func TestInitNoGitRequired(t *testing.T) {
	// Source directory is just a plain directory, no .git present.
	dir := t.TempDir()
	// Confirm there is no .git directory.
	if _, err := os.Stat(filepath.Join(dir, ".git")); !os.IsNotExist(err) {
		t.Skip("temp dir unexpectedly contains .git")
	}
	_, err := state.Init(dir, state.DefaultImage, false)
	if err != nil {
		t.Fatalf("Init should succeed for a non-git directory, got: %v", err)
	}
}

func TestInitGitFlag(t *testing.T) {
	dir := t.TempDir()
	cfg, err := state.Init(dir, state.DefaultImage, true)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if !cfg.IsGitRepo {
		t.Error("IsGitRepo should be true when passed true")
	}
}

func TestLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	cfg, err := state.Init(dir, "myimage:latest", false)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	loaded, err := state.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.ID != cfg.ID {
		t.Errorf("ID mismatch: got %q, want %q", loaded.ID, cfg.ID)
	}
	if loaded.SourceRoot != cfg.SourceRoot {
		t.Errorf("SourceRoot mismatch")
	}
}

func TestNewWorktree(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := state.Init(dir, state.DefaultImage, false)

	w, err := state.NewWorktree(cfg, "feature-x")
	if err != nil {
		t.Fatalf("NewWorktree: %v", err)
	}
	if w.ID == "" {
		t.Error("worktree ID should not be empty")
	}
	if w.Branch != "feature-x" {
		t.Errorf("Branch = %q, want %q", w.Branch, "feature-x")
	}
	if w.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if len(cfg.Worktrees) != 1 {
		t.Errorf("expected 1 worktree, got %d", len(cfg.Worktrees))
	}

	// Reload from disk and verify persistence.
	loaded, _ := state.Load(dir)
	if len(loaded.Worktrees) != 1 {
		t.Errorf("expected 1 persisted worktree, got %d", len(loaded.Worktrees))
	}
}

func TestFindWorktreeByBranch(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := state.Init(dir, state.DefaultImage, false)

	w1, _ := state.NewWorktree(cfg, "main")
	w2, _ := state.NewWorktree(cfg, "feature-y")

	// Find by exact branch name.
	found, err := state.FindWorktree(cfg, "main")
	if err != nil {
		t.Fatalf("FindWorktree by branch: %v", err)
	}
	if found.ID != w1.ID {
		t.Errorf("found wrong worktree by branch")
	}

	// Find by branch prefix.
	found, err = state.FindWorktree(cfg, "feature")
	if err != nil {
		t.Fatalf("FindWorktree by branch prefix: %v", err)
	}
	if found.ID != w2.ID {
		t.Errorf("found wrong worktree by branch prefix")
	}
}

func TestFindWorktreeByID(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := state.Init(dir, state.DefaultImage, false)

	w1, _ := state.NewWorktree(cfg, "alpha")
	_, _ = state.NewWorktree(cfg, "beta")

	// Find by exact ID.
	found, err := state.FindWorktree(cfg, w1.ID)
	if err != nil {
		t.Fatalf("FindWorktree by ID: %v", err)
	}
	if found.ID != w1.ID {
		t.Errorf("found wrong worktree by ID")
	}

	// Not found.
	_, err = state.FindWorktree(cfg, "zzznomatch")
	if err == nil {
		t.Error("expected error for no match")
	}
}

func TestFindWorktreeIDPrefixMatch(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := state.Init(dir, state.DefaultImage, false)

	w, _ := state.NewWorktree(cfg, "")
	if len(w.ID) < 2 {
		t.Skip("ID too short for prefix test")
	}
	prefix := w.ID[:2]

	found, err := state.FindWorktree(cfg, prefix)
	if err != nil {
		t.Fatalf("prefix search: %v", err)
	}
	if found.ID != w.ID {
		t.Errorf("found wrong worktree via ID prefix")
	}
}

func TestUpdateWorktree(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := state.Init(dir, state.DefaultImage, false)
	w, _ := state.NewWorktree(cfg, "old-branch")

	w.Branch = "new-branch"
	w.ContainerID = "container-abc"
	w.GitWorktreePath = "/some/path"
	if err := state.UpdateWorktree(cfg, w); err != nil {
		t.Fatalf("UpdateWorktree: %v", err)
	}

	loaded, _ := state.Load(dir)
	if loaded.Worktrees[0].Branch != "new-branch" {
		t.Errorf("Branch not updated")
	}
	if loaded.Worktrees[0].ContainerID != "container-abc" {
		t.Errorf("ContainerID not updated")
	}
	if loaded.Worktrees[0].GitWorktreePath != "/some/path" {
		t.Errorf("GitWorktreePath not updated")
	}
}

func TestRemoveWorktree(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := state.Init(dir, state.DefaultImage, false)
	w, _ := state.NewWorktree(cfg, "to-remove")

	if err := state.RemoveWorktree(cfg, w.ID); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}
	if len(cfg.Worktrees) != 0 {
		t.Errorf("expected 0 worktrees after remove, got %d", len(cfg.Worktrees))
	}

	loaded, _ := state.Load(dir)
	if len(loaded.Worktrees) != 0 {
		t.Errorf("expected 0 persisted worktrees after remove")
	}
}

func TestOverlayDirs(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := state.Init(dir, state.DefaultImage, false)
	w := state.Worktree{
		ID:        "abcdef",
		Branch:    "main",
		CreatedAt: time.Now(),
	}
	upper, work, merged := cfg.OverlayDirs(w)
	if upper == "" || work == "" || merged == "" {
		t.Error("OverlayDirs returned empty paths")
	}
	base := filepath.Join(cfg.DataDir, w.ID)
	if upper != filepath.Join(base, "upper") {
		t.Errorf("upper = %q, want %q", upper, filepath.Join(base, "upper"))
	}
	if work != filepath.Join(base, "work") {
		t.Errorf("work = %q", work)
	}
	if merged != filepath.Join(base, "merged") {
		t.Errorf("merged = %q", merged)
	}
}

func TestMountPath(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := state.Init(dir, state.DefaultImage, true)

	// Without git worktree path: falls back to SourceRoot.
	wNoGit := state.Worktree{ID: "aaa", Branch: "main", CreatedAt: time.Now()}
	if cfg.MountPath(wNoGit) != cfg.SourceRoot {
		t.Errorf("MountPath without GitWorktreePath should return SourceRoot")
	}

	// With git worktree path: uses that path.
	wGit := state.Worktree{ID: "bbb", Branch: "feature", GitWorktreePath: "/some/tree", CreatedAt: time.Now()}
	if cfg.MountPath(wGit) != "/some/tree" {
		t.Errorf("MountPath with GitWorktreePath should return that path")
	}
}

func TestGitWorktreeDir(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := state.Init(dir, state.DefaultImage, true)
	w := state.Worktree{ID: "abc123", Branch: "feat", CreatedAt: time.Now()}
	expected := filepath.Join(cfg.DataDir, "abc123", "tree")
	if cfg.GitWorktreeDir(w) != expected {
		t.Errorf("GitWorktreeDir = %q, want %q", cfg.GitWorktreeDir(w), expected)
	}
}

func TestLoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := state.Load(dir)
	if err == nil {
		t.Error("expected error when state file missing")
	}
}

func TestInitNonExistentSourceDir(t *testing.T) {
	_, err := state.Init("/nonexistent/path/to/nowhere", state.DefaultImage, false)
	if err == nil {
		t.Error("expected error for non-existent source directory")
	}
}

func TestMultipleWorktreesPreserved(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := state.Init(dir, state.DefaultImage, false)

	state.NewWorktree(cfg, "main")
	state.NewWorktree(cfg, "develop")
	state.NewWorktree(cfg, "feature-z")

	loaded, _ := state.Load(dir)
	if len(loaded.Worktrees) != 3 {
		t.Errorf("expected 3 worktrees, got %d", len(loaded.Worktrees))
	}
}

func TestStateFileInJanusDir(t *testing.T) {
	dir := t.TempDir()
	state.Init(dir, state.DefaultImage, false) //nolint:errcheck
	path := state.StatePath(dir)
	// Must be under .janus/ not .agentw/
	if filepath.Base(filepath.Dir(path)) != state.StateDir {
		t.Errorf("state file not inside %s: got %s", state.StateDir, path)
	}
	if state.StateDir != ".janus" {
		t.Errorf("StateDir = %q, want %q", state.StateDir, ".janus")
	}
}
