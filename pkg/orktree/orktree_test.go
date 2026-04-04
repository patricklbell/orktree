package orktree_test

import (
	"os"
	"os/exec"
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

func TestDiscoverIndex_fromLinkedWorktree(t *testing.T) {
	repo := t.TempDir()
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test",
	)
	for _, args := range [][]string{
		{"git", "init", "-b", "main", repo},
		{"git", "-C", repo, "commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", args, out)
		}
	}

	if _, err := orktree.CreateIndex(repo); err != nil {
		t.Fatalf("CreateIndex: %v", err)
	}

	// Create a git linked worktree (simulating an orktree merged view).
	wtPath := filepath.Join(t.TempDir(), "my-feature")
	for _, args := range [][]string{
		{"git", "-C", repo, "branch", "feat"},
		{"git", "-C", repo, "worktree", "add", "--no-checkout", wtPath, "feat"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", args, out)
		}
	}

	t.Run("from_worktree_root", func(t *testing.T) {
		idx, err := orktree.DiscoverIndex(wtPath)
		if err != nil {
			t.Fatalf("DiscoverIndex: %v", err)
		}
		if idx.SourceRoot() != repo {
			t.Errorf("SourceRoot = %q, want %q", idx.SourceRoot(), repo)
		}
	})

	t.Run("from_subdirectory_inside_worktree", func(t *testing.T) {
		sub := filepath.Join(wtPath, "pkg", "server")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		idx, err := orktree.DiscoverIndex(sub)
		if err != nil {
			t.Fatalf("DiscoverIndex: %v", err)
		}
		if idx.SourceRoot() != repo {
			t.Errorf("SourceRoot = %q, want %q", idx.SourceRoot(), repo)
		}
	})
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

// ---------------------------------------------------------------------------
// AddOrktree
// ---------------------------------------------------------------------------

func TestAddOrktree_nonGitRepo(t *testing.T) {
	dir := t.TempDir()
	idx, err := orktree.CreateIndex(dir)
	if err != nil {
		t.Fatalf("CreateIndex: %v", err)
	}

	mergedPath := filepath.Join(filepath.Dir(dir), filepath.Base(dir)+".orktree", "feature-x")
	info, err := idx.AddOrktree(mergedPath, orktree.AddOrktreeOptions{})
	if err != nil {
		t.Fatalf("AddOrktree: %v", err)
	}
	t.Cleanup(func() { idx.RemoveOrktree(info.ID) }) //nolint:errcheck

	if info.Branch != "feature-x" {
		t.Errorf("Branch = %q, want %q", info.Branch, "feature-x")
	}
	if info.MergedPath != mergedPath {
		t.Errorf("MergedPath = %q, want %q", info.MergedPath, mergedPath)
	}
}

func TestAddOrktree_duplicatePath_rejected(t *testing.T) {
	dir := t.TempDir()
	idx, err := orktree.CreateIndex(dir)
	if err != nil {
		t.Fatalf("CreateIndex: %v", err)
	}

	mergedPath := filepath.Join(filepath.Dir(dir), filepath.Base(dir)+".orktree", "dup-test")
	info, err := idx.AddOrktree(mergedPath, orktree.AddOrktreeOptions{})
	if err != nil {
		t.Fatalf("first AddOrktree: %v", err)
	}
	t.Cleanup(func() { idx.RemoveOrktree(info.ID) }) //nolint:errcheck

	// Second orktree at the same path must be rejected.
	_, err = idx.AddOrktree(mergedPath, orktree.AddOrktreeOptions{})
	if err == nil {
		t.Fatal("expected error for duplicate path, got nil")
	}
}

// ---------------------------------------------------------------------------
// CheckEnvironmentPrerequisites
// ---------------------------------------------------------------------------

func TestCheckEnvironmentPrerequisites_includes_fuse_overlayfs(t *testing.T) {
	prereqs := orktree.CheckEnvironmentPrerequisites()
	found := false
	for _, p := range prereqs {
		if p.Name == "fuse-overlayfs" {
			found = true
			break
		}
	}
	if !found {
		t.Error("CheckEnvironmentPrerequisites does not include a fuse-overlayfs check")
	}
}

// ---------------------------------------------------------------------------
// RemoveCheck
// ---------------------------------------------------------------------------

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

func TestRemoveCheck_IsCleanWith(t *testing.T) {
	tests := []struct {
		name            string
		rc              orktree.RemoveCheck
		ignoreUntracked bool
		ignoreTracked   bool
		want            bool
	}{
		{
			name: "empty_check_is_clean_with_any_flags",
			rc:   orktree.RemoveCheck{},
			want: true,
		},
		{
			name:            "empty_check_is_clean_with_both_flags",
			rc:              orktree.RemoveCheck{},
			ignoreUntracked: true,
			ignoreTracked:   true,
			want:            true,
		},
		{
			name: "has_dependents_never_clean",
			rc:   orktree.RemoveCheck{Dependents: []string{"child"}},
			want: false,
		},
		{
			name:            "has_dependents_never_clean_with_both_flags",
			rc:              orktree.RemoveCheck{Dependents: []string{"child"}},
			ignoreUntracked: true,
			ignoreTracked:   true,
			want:            false,
		},
		{
			name: "has_unmerged_commits_never_clean",
			rc:   orktree.RemoveCheck{UnmergedTotal: 3},
			want: false,
		},
		{
			name:            "has_unmerged_commits_never_clean_with_both_flags",
			rc:              orktree.RemoveCheck{UnmergedTotal: 3},
			ignoreUntracked: true,
			ignoreTracked:   true,
			want:            false,
		},
		{
			name:          "tracked_dirty_only_clean_when_ignore_tracked",
			rc:            orktree.RemoveCheck{TrackedTotal: 2},
			ignoreTracked: true,
			want:          true,
		},
		{
			name: "tracked_dirty_only_not_clean_when_not_ignored",
			rc:   orktree.RemoveCheck{TrackedTotal: 2},
			want: false,
		},
		{
			name:            "untracked_dirty_only_clean_when_ignore_untracked",
			rc:              orktree.RemoveCheck{UntrackedTotal: 4},
			ignoreUntracked: true,
			want:            true,
		},
		{
			name: "untracked_dirty_only_not_clean_when_not_ignored",
			rc:   orktree.RemoveCheck{UntrackedTotal: 4},
			want: false,
		},
		{
			name: "both_dirty_not_clean_with_no_flags",
			rc:   orktree.RemoveCheck{TrackedTotal: 1, UntrackedTotal: 2},
			want: false,
		},
		{
			name:            "both_dirty_not_clean_with_only_ignore_untracked",
			rc:              orktree.RemoveCheck{TrackedTotal: 1, UntrackedTotal: 2},
			ignoreUntracked: true,
			want:            false,
		},
		{
			name:          "both_dirty_not_clean_with_only_ignore_tracked",
			rc:            orktree.RemoveCheck{TrackedTotal: 1, UntrackedTotal: 2},
			ignoreTracked: true,
			want:          false,
		},
		{
			name:            "both_dirty_clean_with_both_flags",
			rc:              orktree.RemoveCheck{TrackedTotal: 1, UntrackedTotal: 2},
			ignoreUntracked: true,
			ignoreTracked:   true,
			want:            true,
		},
		{
			name: "only_ignored_dirty_is_always_clean",
			rc:   orktree.RemoveCheck{IgnoredDirty: 7},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rc.IsCleanWith(tt.ignoreUntracked, tt.ignoreTracked); got != tt.want {
				t.Errorf("IsCleanWith(%v, %v) = %v, want %v", tt.ignoreUntracked, tt.ignoreTracked, got, tt.want)
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
