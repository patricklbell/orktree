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
	ws, err := state.Init(dir, "ubuntu:24.04")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if ws.SourceRoot != dir {
		t.Errorf("SourceRoot = %q, want %q", ws.SourceRoot, dir)
	}
	if ws.Image != "ubuntu:24.04" {
		t.Errorf("Image = %q, want %q", ws.Image, "ubuntu:24.04")
	}

	path := state.StatePath(dir)
	if _, err := os.Stat(path); err != nil {
		t.Errorf("state file not created at %s: %v", path, err)
	}
}

func TestInitDefaultImage(t *testing.T) {
	dir := t.TempDir()
	ws, err := state.Init(dir, state.DefaultImage)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if ws.Image != state.DefaultImage {
		t.Errorf("Image = %q, want %q", ws.Image, state.DefaultImage)
	}
}

func TestInitNoGitRequired(t *testing.T) {
	// Source directory is just a plain directory, no .git present.
	dir := t.TempDir()
	// Confirm there is no .git directory.
	if _, err := os.Stat(filepath.Join(dir, ".git")); !os.IsNotExist(err) {
		t.Skip("temp dir unexpectedly contains .git")
	}
	_, err := state.Init(dir, state.DefaultImage)
	if err != nil {
		t.Fatalf("Init should succeed for a non-git directory, got: %v", err)
	}
}

func TestLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	ws, err := state.Init(dir, "myimage:latest")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	loaded, err := state.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.ID != ws.ID {
		t.Errorf("ID mismatch: got %q, want %q", loaded.ID, ws.ID)
	}
	if loaded.SourceRoot != ws.SourceRoot {
		t.Errorf("SourceRoot mismatch")
	}
}

func TestNewWorkspace(t *testing.T) {
	dir := t.TempDir()
	ws, _ := state.Init(dir, state.DefaultImage)

	w, err := state.NewWorkspace(ws, "my-ws")
	if err != nil {
		t.Fatalf("NewWorkspace: %v", err)
	}
	if w.ID == "" {
		t.Error("workspace ID should not be empty")
	}
	if w.Name != "my-ws" {
		t.Errorf("Name = %q, want %q", w.Name, "my-ws")
	}
	if w.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if len(ws.Workspaces) != 1 {
		t.Errorf("expected 1 workspace, got %d", len(ws.Workspaces))
	}

	// Reload from disk and verify persistence.
	loaded, _ := state.Load(dir)
	if len(loaded.Workspaces) != 1 {
		t.Errorf("expected 1 persisted workspace, got %d", len(loaded.Workspaces))
	}
}

func TestFindWorkspace(t *testing.T) {
	dir := t.TempDir()
	ws, _ := state.Init(dir, state.DefaultImage)

	w1, _ := state.NewWorkspace(ws, "alpha")
	w2, _ := state.NewWorkspace(ws, "beta")

	// Find by exact ID.
	found, err := state.FindWorkspace(ws, w1.ID)
	if err != nil {
		t.Fatalf("FindWorkspace by ID: %v", err)
	}
	if found.ID != w1.ID {
		t.Errorf("found wrong workspace")
	}

	// Find by name.
	found, err = state.FindWorkspace(ws, "beta")
	if err != nil {
		t.Fatalf("FindWorkspace by name: %v", err)
	}
	if found.ID != w2.ID {
		t.Errorf("found wrong workspace by name")
	}

	// Not found.
	_, err = state.FindWorkspace(ws, "zzznomatch")
	if err == nil {
		t.Error("expected error for no match")
	}
}

func TestFindWorkspacePrefixMatch(t *testing.T) {
	dir := t.TempDir()
	ws, _ := state.Init(dir, state.DefaultImage)

	// Create a workspace and search by a prefix of its ID.
	w, _ := state.NewWorkspace(ws, "")
	if len(w.ID) < 2 {
		t.Skip("ID too short for prefix test")
	}
	prefix := w.ID[:2]

	found, err := state.FindWorkspace(ws, prefix)
	if err != nil {
		t.Fatalf("prefix search: %v", err)
	}
	if found.ID != w.ID {
		t.Errorf("found wrong workspace via prefix")
	}
}

func TestUpdateWorkspace(t *testing.T) {
	dir := t.TempDir()
	ws, _ := state.Init(dir, state.DefaultImage)
	w, _ := state.NewWorkspace(ws, "old-name")

	w.Name = "new-name"
	w.ContainerID = "container-abc"
	if err := state.UpdateWorkspace(ws, w); err != nil {
		t.Fatalf("UpdateWorkspace: %v", err)
	}

	loaded, _ := state.Load(dir)
	if loaded.Workspaces[0].Name != "new-name" {
		t.Errorf("Name not updated")
	}
	if loaded.Workspaces[0].ContainerID != "container-abc" {
		t.Errorf("ContainerID not updated")
	}
}

func TestRemoveWorkspace(t *testing.T) {
	dir := t.TempDir()
	ws, _ := state.Init(dir, state.DefaultImage)
	w, _ := state.NewWorkspace(ws, "to-remove")

	if err := state.RemoveWorkspace(ws, w.ID); err != nil {
		t.Fatalf("RemoveWorkspace: %v", err)
	}
	if len(ws.Workspaces) != 0 {
		t.Errorf("expected 0 workspaces after remove, got %d", len(ws.Workspaces))
	}

	loaded, _ := state.Load(dir)
	if len(loaded.Workspaces) != 0 {
		t.Errorf("expected 0 persisted workspaces after remove")
	}
}

func TestOverlayDirs(t *testing.T) {
	dir := t.TempDir()
	ws, _ := state.Init(dir, state.DefaultImage)
	w := state.Workspace{
		ID:        "abcdef",
		CreatedAt: time.Now(),
	}
	upper, work, merged := ws.OverlayDirs(w)
	if upper == "" || work == "" || merged == "" {
		t.Error("OverlayDirs returned empty paths")
	}
	// All should be under DataDir/workspaceID.
	base := filepath.Join(ws.DataDir, w.ID)
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

func TestLoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := state.Load(dir)
	if err == nil {
		t.Error("expected error when state file missing")
	}
}

func TestInitNonExistentSourceDir(t *testing.T) {
	_, err := state.Init("/nonexistent/path/to/nowhere", state.DefaultImage)
	if err == nil {
		t.Error("expected error for non-existent source directory")
	}
}

func TestMultipleWorkspacesPreserved(t *testing.T) {
	dir := t.TempDir()
	ws, _ := state.Init(dir, state.DefaultImage)

	state.NewWorkspace(ws, "a")
	state.NewWorkspace(ws, "b")
	state.NewWorkspace(ws, "c")

	loaded, _ := state.Load(dir)
	if len(loaded.Workspaces) != 3 {
		t.Errorf("expected 3 workspaces, got %d", len(loaded.Workspaces))
	}
}
