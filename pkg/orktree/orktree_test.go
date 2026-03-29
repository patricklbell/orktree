package orktree_test

import (
	"os"
	"path/filepath"
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
		if got := orktree.IsOverlayWhiteout(tt.path); got != tt.want {
			t.Errorf("IsOverlayWhiteout(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

