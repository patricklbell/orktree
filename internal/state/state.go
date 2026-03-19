// Package state manages the persistent JSON metadata for janus.
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
	StateDir  = ".janus"
	StateFile = "state.json"

	DefaultImage = "ubuntu:24.04"
)

// Config is the top-level metadata stored in .janus/state.json.
type Config struct {
	ID        string     `json:"id"`
	SourceRoot string    `json:"source_root"`
	IsGitRepo bool       `json:"is_git_repo"`
	Image     string     `json:"image"`
	DataDir   string     `json:"data_dir"`
	Worktrees []Worktree `json:"worktrees"`
}

// Worktree holds per-worktree metadata.
type Worktree struct {
	// ID is a short random hex identifier.
	ID string `json:"id"`
	// Branch is the git branch name (or a human label when not in a git repo).
	Branch string `json:"branch"`
	// GitWorktreePath is the path to the git worktree checkout on the host.
	// It serves as the overlayfs lowerdir. Empty when not in a git repo.
	GitWorktreePath string    `json:"git_worktree_path,omitempty"`
	ContainerID     string    `json:"container_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// OverlayDirs returns the three overlay sub-directories for a worktree.
func (c *Config) OverlayDirs(w Worktree) (upper, work, merged string) {
	base := filepath.Join(c.DataDir, w.ID)
	return filepath.Join(base, "upper"),
		filepath.Join(base, "work"),
		filepath.Join(base, "merged")
}

// LowerDir returns the overlayfs lowerdir for the given worktree.
// When git integration is active and a worktree checkout exists, that path is
// used; otherwise the repo source root is used.
func (c *Config) LowerDir(w Worktree) string {
	if w.GitWorktreePath != "" {
		return w.GitWorktreePath
	}
	return c.SourceRoot
}

// GitWorktreeDir returns the expected path for the git worktree checkout of w.
func (c *Config) GitWorktreeDir(w Worktree) string {
	return filepath.Join(c.DataDir, w.ID, "tree")
}

// StatePath returns the path to the state file inside sourceRoot.
func StatePath(sourceRoot string) string {
	return filepath.Join(sourceRoot, StateDir, StateFile)
}

// Load reads the state file from the given source root.
func Load(sourceRoot string) (*Config, error) {
	data, err := os.ReadFile(StatePath(sourceRoot))
	if err != nil {
		return nil, fmt.Errorf("reading state: %w (did you run 'janus init'?)", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing state: %w", err)
	}
	return &cfg, nil
}

// Save writes the state back to disk (atomic: write to temp then rename).
func Save(cfg *Config) error {
	path := StatePath(cfg.SourceRoot)
	data, err := json.MarshalIndent(cfg, "", "  ")
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

// Init creates a new Config for the given source root.
// It derives the data directory from XDG_DATA_HOME (or ~/.local/share).
func Init(sourceRoot, image string, isGitRepo bool) (*Config, error) {
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

	id := repoID(abs)
	dataDir := dataHome(id)

	cfg := &Config{
		ID:        id,
		SourceRoot: abs,
		IsGitRepo: isGitRepo,
		Image:     image,
		DataDir:   dataDir,
		Worktrees: []Worktree{},
	}

	stateDir := filepath.Join(abs, StateDir)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating state dir: %w", err)
	}
	if err := Save(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// NewWorktree adds a worktree entry to cfg and saves state.
// It does NOT create overlay dirs, git worktrees, or start a container.
func NewWorktree(cfg *Config, branch string) (Worktree, error) {
	w := Worktree{
		ID:        newID(),
		Branch:    branch,
		CreatedAt: time.Now().UTC(),
	}
	cfg.Worktrees = append(cfg.Worktrees, w)
	return w, Save(cfg)
}

// UpdateWorktree replaces the worktree entry with matching ID and saves.
func UpdateWorktree(cfg *Config, w Worktree) error {
	for i, existing := range cfg.Worktrees {
		if existing.ID == w.ID {
			cfg.Worktrees[i] = w
			return Save(cfg)
		}
	}
	return fmt.Errorf("worktree %q not found", w.ID)
}

// RemoveWorktree removes the worktree entry with the given ID and saves.
func RemoveWorktree(cfg *Config, id string) error {
	for i, w := range cfg.Worktrees {
		if w.ID == id {
			cfg.Worktrees = append(cfg.Worktrees[:i], cfg.Worktrees[i+1:]...)
			return Save(cfg)
		}
	}
	return fmt.Errorf("worktree %q not found", id)
}

// FindWorktree returns the worktree matching ref by ID, branch name, or prefix.
func FindWorktree(cfg *Config, ref string) (Worktree, error) {
	// Exact ID match.
	for _, w := range cfg.Worktrees {
		if w.ID == ref {
			return w, nil
		}
	}
	// Exact branch match.
	for _, w := range cfg.Worktrees {
		if w.Branch == ref {
			return w, nil
		}
	}
	// Use a map to deduplicate (a worktree could match both by ID prefix and
	// branch prefix).
	seen := make(map[string]Worktree)
	for _, w := range cfg.Worktrees {
		if len(ref) > 0 && len(w.ID) >= len(ref) && w.ID[:len(ref)] == ref {
			seen[w.ID] = w
			continue
		}
		if len(ref) > 0 && len(w.Branch) >= len(ref) && w.Branch[:len(ref)] == ref {
			seen[w.ID] = w
		}
	}
	switch len(seen) {
	case 0:
		return Worktree{}, fmt.Errorf("no worktree matching %q", ref)
	case 1:
		for _, w := range seen {
			return w, nil
		}
	}
	return Worktree{}, fmt.Errorf("ambiguous worktree reference %q (matches multiple)", ref)
}

// repoID returns a short hex string derived from the source root path.
func repoID(sourceRoot string) string {
	h := sha256.Sum256([]byte(sourceRoot))
	return fmt.Sprintf("%x", h[:8])
}

// newID returns a random 6-character hex id.
func newID() string {
	b := make([]byte, 3)
	rand.Read(b) //nolint:gosec // ids need not be cryptographically random
	return fmt.Sprintf("%x", b)
}

// dataHome returns the XDG data home path for the given repo id.
func dataHome(id string) string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "janus", id)
}
