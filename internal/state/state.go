// Package state manages the persistent JSON metadata for orktree.
package state

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

const StateFile = "state.json"

// Config is the top-level metadata stored in <repo>.orktree/state.json.
type Config struct {
	SourceRoot string    `json:"source_root"`
	IsGitRepo  bool      `json:"is_git_repo"`
	Orktrees   []Orktree `json:"orktrees"`
}

// Orktree holds per-orktree metadata.
// An orktree is a git worktree registration paired with a fuse-overlayfs CoW mount.
type Orktree struct {
	// ID is a short random hex identifier.
	ID string `json:"id"`
	// Branch is the git branch name (or a human label when not in a git repo).
	Branch string `json:"branch"`
	// GitTreePath is the path to the registered git worktree directory.
	// Empty when the orktree is not git-backed.
	GitTreePath string `json:"git_tree_path,omitempty"`
	// LowerDir is the overlayfs lowerdir path.
	// For a conventional orktree this equals GitTreePath (the full checkout).
	// For a zero-cost orktree this is either the source root or the merged
	// path of a parent orktree — no separate checkout is needed.
	LowerDir string `json:"lower_dir,omitempty"`
	// LowerOrktreeBranch records the parent orktree branch when this orktree
	// was created zero-cost from another orktree.
	LowerOrktreeBranch string    `json:"lower_orktree_branch,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

// SiblingDir returns the path to the orktree data directory that sits next to
// sourceRoot: /path/to/myrepo → /path/to/myrepo.orktree
func SiblingDir(sourceRoot string) string {
	return filepath.Join(filepath.Dir(sourceRoot), filepath.Base(sourceRoot)+".orktree")
}

// StatePath returns the path to the state file in the sibling dir for sourceRoot.
func StatePath(sourceRoot string) string {
	return filepath.Join(SiblingDir(sourceRoot), StateFile)
}

// Load reads the state file from the given source root.
func Load(sourceRoot string) (*Config, error) {
	data, err := os.ReadFile(StatePath(sourceRoot))
	if err != nil {
		return nil, fmt.Errorf("reading state: %w (did you run 'orktree init'?)", err)
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
// It creates a sibling directory (<repo>.orktree/) next to the source root and
// writes a .gitignore there to prevent any parent git repo from tracking its contents.
func Init(sourceRoot string, isGitRepo bool) (*Config, error) {
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

	sib := SiblingDir(abs)
	if err := os.MkdirAll(sib, 0o755); err != nil {
		return nil, fmt.Errorf("creating sibling dir: %w", err)
	}

	// Prevent any parent git repo from accidentally tracking orktree internals.
	gitignorePath := filepath.Join(sib, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte("*\n"), 0o644); err != nil {
		return nil, fmt.Errorf("writing .gitignore: %w", err)
	}

	cfg := &Config{
		SourceRoot: abs,
		IsGitRepo:  isGitRepo,
		Orktrees:   []Orktree{},
	}

	if err := Save(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// NewOrktree adds an orktree entry to cfg and saves state.
func NewOrktree(cfg *Config, branch string) (Orktree, error) {
	w := Orktree{
		ID:        newID(),
		Branch:    branch,
		CreatedAt: time.Now().UTC(),
	}
	cfg.Orktrees = append(cfg.Orktrees, w)
	return w, Save(cfg)
}

// UpdateOrktree replaces the orktree entry with matching ID and saves.
func UpdateOrktree(cfg *Config, w Orktree) error {
	for i, existing := range cfg.Orktrees {
		if existing.ID == w.ID {
			cfg.Orktrees[i] = w
			return Save(cfg)
		}
	}
	return fmt.Errorf("orktree %q not found", w.ID)
}

// RemoveOrktree removes the orktree entry with the given ID and saves.
func RemoveOrktree(cfg *Config, id string) error {
	for i, w := range cfg.Orktrees {
		if w.ID == id {
			cfg.Orktrees = append(cfg.Orktrees[:i], cfg.Orktrees[i+1:]...)
			return Save(cfg)
		}
	}
	return fmt.Errorf("orktree %q not found", id)
}

// Dependents returns all orktrees whose LowerOrktreeBranch matches branch,
// i.e. orktrees that directly depend on the given branch as their overlay base.
func Dependents(cfg *Config, branch string) []Orktree {
	var deps []Orktree
	for _, w := range cfg.Orktrees {
		if w.LowerOrktreeBranch == branch {
			deps = append(deps, w)
		}
	}
	return deps
}

// FindOrktree returns the orktree matching ref by ID, branch name, or prefix.
func FindOrktree(cfg *Config, ref string) (Orktree, error) {
	// Exact ID match.
	for _, w := range cfg.Orktrees {
		if w.ID == ref {
			return w, nil
		}
	}
	// Exact branch match.
	for _, w := range cfg.Orktrees {
		if w.Branch == ref {
			return w, nil
		}
	}
	// Use a map to deduplicate (an orktree could match both by ID prefix and
	// branch prefix).
	seen := make(map[string]Orktree)
	for _, w := range cfg.Orktrees {
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
		return Orktree{}, fmt.Errorf("no orktree matching %q", ref)
	case 1:
		for _, w := range seen {
			return w, nil
		}
	}
	return Orktree{}, fmt.Errorf("ambiguous orktree reference %q (matches multiple)", ref)
}

// GitTreeDir returns the path where the registered git worktree directory for w
// is stored.  For zero-cost orktrees this directory contains only a .git gitfile
// (no checkout); for conventional orktrees it is the full checkout used as lowerdir.
func (c *Config) GitTreeDir(w Orktree) string {
	return filepath.Join(SiblingDir(c.SourceRoot), ".overlayfs", w.ID, "tree")
}

// OverlayDirs returns the upper, work, and merged directory paths for w.
// Branches containing "/" (e.g. "feature/my-branch") produce nested merged paths.
func (c *Config) OverlayDirs(w Orktree) (upper, work, merged string) {
	sib := SiblingDir(c.SourceRoot)
	hidden := filepath.Join(sib, ".overlayfs", w.ID)
	// filepath.FromSlash translates branches like "feature/my-branch" into
	// nested directories, matching how users expect them to appear.
	merged = filepath.Join(sib, filepath.FromSlash(w.Branch))
	return filepath.Join(hidden, "upper"), filepath.Join(hidden, "work"), merged
}

// MountPath returns the overlayfs lowerdir for w.
// When LowerDir is explicitly stored it is returned directly;
// otherwise falls back to GitTreePath then the source root.
func (c *Config) MountPath(w Orktree) string {
	if w.LowerDir != "" {
		return w.LowerDir
	}
	if w.GitTreePath != "" {
		return w.GitTreePath
	}
	return c.SourceRoot
}

// newID returns a random 6-character hex id.
func newID() string {
	b := make([]byte, 3)
	rand.Read(b) //nolint:gosec // ids need not be cryptographically random
	return fmt.Sprintf("%x", b)
}
