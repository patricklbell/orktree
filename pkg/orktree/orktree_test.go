package orktree_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/patricklbell/orktree/internal/overlay"
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

func TestIsOverlayWhiteout(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{".wh..wh..opq", true},
		{".wh..opq", true},
		{".wh.deleted-file.txt", true},
		{"subdir/.wh.foo", true},
		{"normal-file.txt", false},
		{".whatsapp/config", false},
		{"src/main.go", false},
	}
	for _, tt := range tests {
		if got := overlay.IsOverlayWhiteout(tt.path); got != tt.want {
			t.Errorf("IsOverlayWhiteout(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
