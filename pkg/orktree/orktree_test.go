package orktree_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/patricklbell/orktree/pkg/orktree"
)

func TestInit_createsStateAndReturnsManager(t *testing.T) {
	dir := t.TempDir()
	mgr, err := orktree.Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if mgr.SourceRoot() != dir {
		t.Errorf("SourceRoot = %q, want %q", mgr.SourceRoot(), dir)
	}
}

func TestInit_idempotent(t *testing.T) {
	dir := t.TempDir()
	mgr1, err := orktree.Init(dir)
	if err != nil {
		t.Fatalf("first Init: %v", err)
	}
	mgr2, err := orktree.Init(dir)
	if err != nil {
		t.Fatalf("second Init: %v", err)
	}
	if mgr1.SourceRoot() != mgr2.SourceRoot() {
		t.Errorf("SourceRoot mismatch: %q vs %q", mgr1.SourceRoot(), mgr2.SourceRoot())
	}
}

func TestNewManager_failsIfNotInitialized(t *testing.T) {
	dir := t.TempDir()
	_, err := orktree.NewManager(dir)
	if err == nil {
		t.Fatal("expected error for uninitialized directory")
	}
}

func TestDiscover_fromSubdirectory(t *testing.T) {
	dir := t.TempDir()
	if _, err := orktree.Init(dir); err != nil {
		t.Fatalf("Init: %v", err)
	}
	sub := filepath.Join(dir, "pkg", "server")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	mgr, err := orktree.Discover(sub)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if mgr.SourceRoot() != dir {
		t.Errorf("SourceRoot = %q, want %q", mgr.SourceRoot(), dir)
	}
}

func TestList_emptyReturnsEmptySlice(t *testing.T) {
	dir := t.TempDir()
	mgr, err := orktree.Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	infos, err := mgr.List()
	if err != nil {
		t.Fatalf("List: %v", err)
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

func TestRemoveRefusedError_formatting(t *testing.T) {
	t.Run("all sections populated", func(t *testing.T) {
		e := &orktree.RemoveRefusedError{
			Branch:     "feature",
			Dependents: []string{"feature-v2", "feature-v3"},
			DirtyFiles: []string{"src/parser.go", "src/parser_test.go"},
			UnmergedCommits: []string{
				"a1b2c3d add parser error recovery",
				"d4e5f6a fix lexer edge case",
			},
		}
		msg := e.Error()
		for _, want := range []string{
			`refusing to remove "feature"`,
			"Dependent orktrees",
			"feature-v2",
			"feature-v3",
			"Uncommitted changes in overlay",
			"src/parser.go",
			"src/parser_test.go",
			"Unmerged commits (not in any other branch)",
			"a1b2c3d add parser error recovery",
			"d4e5f6a fix lexer edge case",
			"Use --force to remove anyway.",
		} {
			if !strings.Contains(msg, want) {
				t.Errorf("Error() missing %q\ngot:\n%s", want, msg)
			}
		}
	})

	t.Run("only dirty files", func(t *testing.T) {
		e := &orktree.RemoveRefusedError{
			Branch:     "wip",
			DirtyFiles: []string{"README.md"},
		}
		msg := e.Error()
		if strings.Contains(msg, "Dependent") {
			t.Errorf("should not contain Dependent section:\n%s", msg)
		}
		if strings.Contains(msg, "Unmerged commits") {
			t.Errorf("should not contain Unmerged commits section:\n%s", msg)
		}
		if !strings.Contains(msg, "README.md") {
			t.Errorf("missing dirty file:\n%s", msg)
		}
	})

	t.Run("truncation at 10 items", func(t *testing.T) {
		files := make([]string, 13)
		for i := range files {
			files[i] = "file" + strings.Repeat("x", i)
		}
		e := &orktree.RemoveRefusedError{
			Branch:     "big",
			DirtyFiles: files,
		}
		msg := e.Error()
		if !strings.Contains(msg, "... and 3 more") {
			t.Errorf("expected truncation message, got:\n%s", msg)
		}
	})
}
