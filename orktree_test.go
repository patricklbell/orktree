package orktree_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/patricklbell/orktree"
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
