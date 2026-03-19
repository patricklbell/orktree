// Package state manages the persistent JSON metadata for agentw.
package state

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

const (
	StateDir  = ".agentw"
	StateFile = "state.json"

	DefaultImage = "ubuntu:24.04"
)

// WorkspaceSet is the top-level metadata stored in .agentw/state.json.
type WorkspaceSet struct {
	ID         string      `json:"workspace_set_id"`
	SourceRoot string      `json:"source_root"`
	Image      string      `json:"image"`
	DataDir    string      `json:"data_dir"`
	Workspaces []Workspace `json:"workspaces"`
}

// Workspace holds per-workspace metadata.
type Workspace struct {
	ID          string    `json:"id"`
	Name        string    `json:"name,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	ContainerID string    `json:"container_id,omitempty"`
}

// OverlayDirs returns the three overlay sub-directories for a workspace.
func (ws *WorkspaceSet) OverlayDirs(w Workspace) (upper, work, merged string) {
	base := filepath.Join(ws.DataDir, w.ID)
	return filepath.Join(base, "upper"),
		filepath.Join(base, "work"),
		filepath.Join(base, "merged")
}

// StatePath returns the path to the state file inside sourceRoot.
func StatePath(sourceRoot string) string {
	return filepath.Join(sourceRoot, StateDir, StateFile)
}

// Load reads the state file from the given source root.
func Load(sourceRoot string) (*WorkspaceSet, error) {
	data, err := os.ReadFile(StatePath(sourceRoot))
	if err != nil {
		return nil, fmt.Errorf("reading state: %w (did you run 'agentw init'?)", err)
	}
	var ws WorkspaceSet
	if err := json.Unmarshal(data, &ws); err != nil {
		return nil, fmt.Errorf("parsing state: %w", err)
	}
	return &ws, nil
}

// Save writes the state back to disk (atomic: write to temp then rename).
func Save(ws *WorkspaceSet) error {
	path := StatePath(ws.SourceRoot)
	data, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling state: %w", err)
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "state-*.json.tmp")
	if err != nil {
		return fmt.Errorf("creating temp state file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("writing state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("closing temp state file: %w", err)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("setting state file permissions: %w", err)
	}
	return os.Rename(tmpName, path)
}

// Init creates a new WorkspaceSet for the given source root.
// It derives the data directory from XDG_DATA_HOME (or ~/.local/share).
func Init(sourceRoot, image string) (*WorkspaceSet, error) {
	abs, err := filepath.Abs(sourceRoot)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("source directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("source path is not a directory: %s", abs)
	}

	id := wsID(abs)
	dataDir := dataHome(id)

	ws := &WorkspaceSet{
		ID:         id,
		SourceRoot: abs,
		Image:      image,
		DataDir:    dataDir,
		Workspaces: []Workspace{},
	}

	stateDir := filepath.Join(abs, StateDir)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating state dir: %w", err)
	}
	if err := Save(ws); err != nil {
		return nil, err
	}
	return ws, nil
}

// NewWorkspace adds a workspace entry to ws and saves state.
// It does NOT create overlay dirs or start a container.
func NewWorkspace(ws *WorkspaceSet, name string) (Workspace, error) {
	w := Workspace{
		ID:        newID(),
		Name:      name,
		CreatedAt: time.Now().UTC(),
	}
	ws.Workspaces = append(ws.Workspaces, w)
	return w, Save(ws)
}

// UpdateWorkspace replaces the workspace entry with matching ID and saves.
func UpdateWorkspace(ws *WorkspaceSet, w Workspace) error {
	for i, existing := range ws.Workspaces {
		if existing.ID == w.ID {
			ws.Workspaces[i] = w
			return Save(ws)
		}
	}
	return fmt.Errorf("workspace %q not found", w.ID)
}

// RemoveWorkspace removes the workspace entry with the given ID and saves.
func RemoveWorkspace(ws *WorkspaceSet, id string) error {
	for i, w := range ws.Workspaces {
		if w.ID == id {
			ws.Workspaces = append(ws.Workspaces[:i], ws.Workspaces[i+1:]...)
			return Save(ws)
		}
	}
	return fmt.Errorf("workspace %q not found", id)
}

// FindWorkspace returns the workspace with the given id or name prefix.
func FindWorkspace(ws *WorkspaceSet, ref string) (Workspace, error) {
	// exact ID match first
	for _, w := range ws.Workspaces {
		if w.ID == ref {
			return w, nil
		}
	}
	// Use a map to deduplicate matches (a workspace could match both by ID
	// prefix and by name).
	seen := make(map[string]Workspace)
	for _, w := range ws.Workspaces {
		if len(ref) > 0 && len(w.ID) >= len(ref) && w.ID[:len(ref)] == ref {
			seen[w.ID] = w
			continue
		}
		if w.Name == ref {
			seen[w.ID] = w
		}
	}
	switch len(seen) {
	case 0:
		return Workspace{}, fmt.Errorf("no workspace matching %q", ref)
	case 1:
		for _, w := range seen {
			return w, nil
		}
	}
	return Workspace{}, fmt.Errorf("ambiguous workspace reference %q (matches multiple)", ref)
}

// wsID returns a short hex string derived from the source root path.
func wsID(sourceRoot string) string {
	h := sha256.Sum256([]byte(sourceRoot))
	return fmt.Sprintf("%x", h[:8])
}

// newID returns a random 6-character hex id.
func newID() string {
	b := make([]byte, 3)
	rand.Read(b) //nolint:gosec // ids need not be cryptographically random
	return fmt.Sprintf("%x", b)
}

// dataHome returns the XDG data home path for the given workspace-set id.
func dataHome(id string) string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "agentw", id)
}
