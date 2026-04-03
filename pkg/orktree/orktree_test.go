package orktree_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/patricklbell/orktree/pkg/orktree"
)

func TestCreateIndex(t *testing.T) {
	dir := t.TempDir()
	idx, err := orktree.CreateIndex(dir)
	if err != nil {
		t.Fatalf("CreateIndex: %v", err)
	}
	if idx.SourceRoot() != dir {
		t.Errorf("SourceRoot = %q, want %q", idx.SourceRoot(), dir)
	}
}

func TestCreateIndex_idempotent(t *testing.T) {
	dir := t.TempDir()
	idx1, err := orktree.CreateIndex(dir)
	if err != nil {
		t.Fatalf("first CreateIndex: %v", err)
	}
	idx2, err := orktree.CreateIndex(dir)
	if err != nil {
		t.Fatalf("second CreateIndex: %v", err)
	}
	if idx1.SourceRoot() != idx2.SourceRoot() {
		t.Errorf("SourceRoot mismatch: %q vs %q", idx1.SourceRoot(), idx2.SourceRoot())
	}
}

func TestLoadIndex_failsIfNotInitialized(t *testing.T) {
	dir := t.TempDir()
	_, err := orktree.LoadIndex(dir)
	if err == nil {
		t.Fatal("expected error for uninitialized directory")
	}
}

func TestDiscoverIndex_fromSubdirectory(t *testing.T) {
	dir := t.TempDir()
	if _, err := orktree.CreateIndex(dir); err != nil {
		t.Fatalf("CreateIndex: %v", err)
	}
	sub := filepath.Join(dir, "pkg", "server")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	idx, err := orktree.DiscoverIndex(sub)
	if err != nil {
		t.Fatalf("DiscoverIndex: %v", err)
	}
	if idx.SourceRoot() != dir {
		t.Errorf("SourceRoot = %q, want %q", idx.SourceRoot(), dir)
	}
}

func TestListOrktrees_empty(t *testing.T) {
	dir := t.TempDir()
	idx, err := orktree.CreateIndex(dir)
	if err != nil {
		t.Fatalf("CreateIndex: %v", err)
	}
	infos, err := idx.ListOrktrees()
	if err != nil {
		t.Fatalf("ListOrktrees: %v", err)
	}
	if len(infos) != 0 {
		t.Errorf("expected empty list, got %d items", len(infos))
	}
}

func TestRemoveCheck_IsClean(t *testing.T) {
	tests := []struct {
		name string
		rc   orktree.RemoveCheck
		want bool
	}{
		{"empty check is clean", orktree.RemoveCheck{}, true},
		{"has dependents", orktree.RemoveCheck{Dependents: []string{"a"}}, false},
		{"has tracked dirty", orktree.RemoveCheck{TrackedTotal: 1}, false},
		{"has untracked dirty", orktree.RemoveCheck{UntrackedTotal: 2}, false},
		{"has unmerged commits", orktree.RemoveCheck{UnmergedTotal: 1}, false},
		{"only ignored dirty is clean", orktree.RemoveCheck{IgnoredDirty: 5}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rc.IsClean(); got != tt.want {
				t.Errorf("IsClean() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemoveCheck_HasBlockers(t *testing.T) {
	tests := []struct {
		name string
		rc   orktree.RemoveCheck
		want bool
	}{
		{"no dependents", orktree.RemoveCheck{}, false},
		{"has dependents", orktree.RemoveCheck{Dependents: []string{"child"}}, true},
		{"dirty but no dependents", orktree.RemoveCheck{TrackedTotal: 5}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rc.HasBlockers(); got != tt.want {
				t.Errorf("HasBlockers() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Named orktrees
// ---------------------------------------------------------------------------

func TestCreateOrktree_named(t *testing.T) {
	dir := t.TempDir()
	idx, err := orktree.CreateIndex(dir)
	if err != nil {
		t.Fatalf("CreateIndex: %v", err)
	}

	info, err := idx.CreateOrktree("feature/PROJ-42-long-description",
		orktree.CreateOrktreeOptions{NoGit: true, Name: "proj-42"})
	if err != nil {
		t.Fatalf("CreateOrktree: %v", err)
	}
	t.Cleanup(func() { idx.RemoveOrktree(info.Name) }) //nolint:errcheck

	if info.Name != "proj-42" {
		t.Errorf("Name = %q, want %q", info.Name, "proj-42")
	}
	if info.Branch != "feature/PROJ-42-long-description" {
		t.Errorf("Branch = %q, want %q", info.Branch, "feature/PROJ-42-long-description")
	}

	sib := filepath.Join(filepath.Dir(dir), filepath.Base(dir)+".orktree")
	wantPath := filepath.Join(sib, "proj-42")
	if info.MergedPath != wantPath {
		t.Errorf("MergedPath = %q, want %q", info.MergedPath, wantPath)
	}
}

func TestCreateOrktree_named_defaults_to_branch(t *testing.T) {
	dir := t.TempDir()
	idx, err := orktree.CreateIndex(dir)
	if err != nil {
		t.Fatalf("CreateIndex: %v", err)
	}

	info, err := idx.CreateOrktree("my-branch", orktree.CreateOrktreeOptions{NoGit: true})
	if err != nil {
		t.Fatalf("CreateOrktree: %v", err)
	}
	t.Cleanup(func() { idx.RemoveOrktree(info.Name) }) //nolint:errcheck

	// When no Name is supplied the effective name falls back to the branch.
	if info.Name != "my-branch" {
		t.Errorf("Name = %q, want %q", info.Name, "my-branch")
	}
}

func TestFindOrktree_by_name(t *testing.T) {
	dir := t.TempDir()
	idx, err := orktree.CreateIndex(dir)
	if err != nil {
		t.Fatalf("CreateIndex: %v", err)
	}

	created, err := idx.CreateOrktree("feature/PROJ-99", orktree.CreateOrktreeOptions{NoGit: true, Name: "proj-99"})
	if err != nil {
		t.Fatalf("CreateOrktree: %v", err)
	}
	t.Cleanup(func() { idx.RemoveOrktree(created.Name) }) //nolint:errcheck

	// Can look up by custom name.
	meta, err := idx.FindOrktree("proj-99")
	if err != nil {
		t.Fatalf("FindOrktree by name: %v", err)
	}
	if meta.Branch != "feature/PROJ-99" {
		t.Errorf("Branch = %q, want %q", meta.Branch, "feature/PROJ-99")
	}
}

func TestCreateOrktree_duplicate_name_rejected(t *testing.T) {
	dir := t.TempDir()
	idx, err := orktree.CreateIndex(dir)
	if err != nil {
		t.Fatalf("CreateIndex: %v", err)
	}

	created, err := idx.CreateOrktree("branch-a", orktree.CreateOrktreeOptions{NoGit: true, Name: "shared-name"})
	if err != nil {
		t.Fatalf("first CreateOrktree: %v", err)
	}
	t.Cleanup(func() { idx.RemoveOrktree(created.Name) }) //nolint:errcheck

	// Second orktree with the same custom name must be rejected.
	_, err = idx.CreateOrktree("branch-b", orktree.CreateOrktreeOptions{NoGit: true, Name: "shared-name"})
	if err == nil {
		t.Fatal("expected error for duplicate name, got nil")
	}
}
