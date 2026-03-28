package state_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/patricklbell/orktree/internal/state"
)

func TestSiblingDir(t *testing.T) {
	got := state.SiblingDir("/projects/myrepo")
	want := "/projects/myrepo.orktree"
	if got != want {
		t.Errorf("SiblingDir = %q, want %q", got, want)
	}
}

func TestInitCreatesStateFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := state.Init(dir, false)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if cfg.SourceRoot != dir {
		t.Errorf("SourceRoot = %q, want %q", cfg.SourceRoot, dir)
	}

	path := state.StatePath(dir)
	if _, err := os.Stat(path); err != nil {
		t.Errorf("state file not created at %s: %v", path, err)
	}

	// Sibling dir must exist.
	sib := state.SiblingDir(dir)
	if _, err := os.Stat(sib); err != nil {
		t.Errorf("sibling dir not created at %s: %v", sib, err)
	}

	// .gitignore must exist and contain "*".
	gi, err := os.ReadFile(filepath.Join(sib, ".gitignore"))
	if err != nil {
		t.Fatalf(".gitignore not created: %v", err)
	}
	if !strings.Contains(string(gi), "*") {
		t.Errorf(".gitignore does not contain '*': %q", gi)
	}
}

func TestInitNoGitRequired(t *testing.T) {
	// Source directory is just a plain directory, no .git present.
	dir := t.TempDir()
	// Confirm there is no .git directory.
	if _, err := os.Stat(filepath.Join(dir, ".git")); !os.IsNotExist(err) {
		t.Skip("temp dir unexpectedly contains .git")
	}
	_, err := state.Init(dir, false)
	if err != nil {
		t.Fatalf("Init should succeed for a non-git directory, got: %v", err)
	}
}

func TestLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	cfg, err := state.Init(dir, false)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	loaded, err := state.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.SourceRoot != cfg.SourceRoot {
		t.Errorf("SourceRoot mismatch")
	}
}

func TestNewWorktree(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := state.Init(dir, false)

	w, err := state.NewOrktree(cfg, "feature-x")
	if err != nil {
		t.Fatalf("NewOrktree: %v", err)
	}
	if w.ID == "" {
		t.Error("orktree ID should not be empty")
	}
	if w.Branch != "feature-x" {
		t.Errorf("Branch = %q, want %q", w.Branch, "feature-x")
	}
	if w.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if len(cfg.Orktrees) != 1 {
		t.Errorf("expected 1 orktree, got %d", len(cfg.Orktrees))
	}

	// Reload from disk and verify persistence.
	loaded, _ := state.Load(dir)
	if len(loaded.Orktrees) != 1 {
		t.Errorf("expected 1 persisted orktree, got %d", len(loaded.Orktrees))
	}
}

func TestFindWorktreeByBranch(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := state.Init(dir, false)

	w1, _ := state.NewOrktree(cfg, "main")
	w2, _ := state.NewOrktree(cfg, "feature-y")

	// Find by exact branch name.
	found, err := state.FindOrktree(cfg, "main")
	if err != nil {
		t.Fatalf("FindOrktree by branch: %v", err)
	}
	if found.ID != w1.ID {
		t.Errorf("found wrong orktree by branch")
	}

	// Find by branch prefix.
	found, err = state.FindOrktree(cfg, "feature")
	if err != nil {
		t.Fatalf("FindOrktree by branch prefix: %v", err)
	}
	if found.ID != w2.ID {
		t.Errorf("found wrong orktree by branch prefix")
	}
}

func TestFindWorktreeByID(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := state.Init(dir, false)

	w1, _ := state.NewOrktree(cfg, "alpha")
	_, _ = state.NewOrktree(cfg, "beta")

	// Find by exact ID.
	found, err := state.FindOrktree(cfg, w1.ID)
	if err != nil {
		t.Fatalf("FindOrktree by ID: %v", err)
	}
	if found.ID != w1.ID {
		t.Errorf("found wrong orktree by ID")
	}

	// Not found.
	_, err = state.FindOrktree(cfg, "zzznomatch")
	if err == nil {
		t.Error("expected error for no match")
	}
}

func TestFindWorktreeIDPrefixMatch(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := state.Init(dir, false)

	w, _ := state.NewOrktree(cfg, "")
	if len(w.ID) < 2 {
		t.Skip("ID too short for prefix test")
	}
	prefix := w.ID[:2]

	found, err := state.FindOrktree(cfg, prefix)
	if err != nil {
		t.Fatalf("prefix search: %v", err)
	}
	if found.ID != w.ID {
		t.Errorf("found wrong orktree via ID prefix")
	}
}

func TestUpdateWorktree(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := state.Init(dir, false)
	w, _ := state.NewOrktree(cfg, "old-branch")

	w.Branch = "new-branch"
	if err := state.UpdateOrktree(cfg, w); err != nil {
		t.Fatalf("UpdateOrktree: %v", err)
	}

	loaded, _ := state.Load(dir)
	if loaded.Orktrees[0].Branch != "new-branch" {
		t.Errorf("Branch not updated")
	}
}

func TestDependents(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := state.Init(dir, false)

	base, _ := state.NewOrktree(cfg, "base")
	child1, _ := state.NewOrktree(cfg, "child1")
	child1.LowerOrktreeBranch = base.Branch
	state.UpdateOrktree(cfg, child1)

	child2, _ := state.NewOrktree(cfg, "child2")
	child2.LowerOrktreeBranch = base.Branch
	state.UpdateOrktree(cfg, child2)

	// unrelated has no parent
	state.NewOrktree(cfg, "unrelated")

	deps := state.Dependents(cfg, "base")
	if len(deps) != 2 {
		t.Fatalf("expected 2 dependents, got %d", len(deps))
	}
	got := map[string]bool{deps[0].Branch: true, deps[1].Branch: true}
	if !got["child1"] || !got["child2"] {
		t.Errorf("unexpected dependents: %v", deps)
	}

	// No dependents for unrelated.
	if deps := state.Dependents(cfg, "unrelated"); len(deps) != 0 {
		t.Errorf("expected 0 dependents for unrelated, got %d", len(deps))
	}
}

func TestRemoveWorktree(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := state.Init(dir, false)
	w, _ := state.NewOrktree(cfg, "to-remove")

	if err := state.RemoveOrktree(cfg, w.ID); err != nil {
		t.Fatalf("RemoveOrktree: %v", err)
	}
	if len(cfg.Orktrees) != 0 {
		t.Errorf("expected 0 orktrees after remove, got %d", len(cfg.Orktrees))
	}

	loaded, _ := state.Load(dir)
	if len(loaded.Orktrees) != 0 {
		t.Errorf("expected 0 persisted orktrees after remove")
	}
}

func TestOverlayDirs(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := state.Init(dir, false)
	sib := state.SiblingDir(dir)

	// Simple branch: merged path should be <sibling>/<branch>
	w1, _ := state.NewOrktree(cfg, "main")
	_, _, merged := cfg.OverlayDirs(w1)
	wantMerged := filepath.Join(sib, "main")
	if merged != wantMerged {
		t.Errorf("merged = %q, want %q", merged, wantMerged)
	}

	// Branch with slash: merged path should be nested
	w2, _ := state.NewOrktree(cfg, "feature/my-branch")
	_, _, merged2 := cfg.OverlayDirs(w2)
	wantMerged2 := filepath.Join(sib, "feature", "my-branch")
	if merged2 != wantMerged2 {
		t.Errorf("merged = %q, want %q", merged2, wantMerged2)
	}
}
