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

type State struct {
	SourceRoot string    `json:"source_root"`
	IsGitRepo  bool      `json:"is_git_repo"`
	Orktrees   []Orktree `json:"orktrees"`
}

type Orktree struct {
	ID             string    `json:"id"`
	Branch         string    `json:"branch"`
	MergedPath     string    `json:"merged_path"`
	LowerDir       string    `json:"lower_dir,omitempty"`
	LowerOrktreeID string    `json:"lower_orktree_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// Path to the orktree data directory that sits next to
// sourceRoot: /path/to/myrepo → /path/to/myrepo.orktree
func SiblingDir(sourceRoot string) string {
	return filepath.Join(filepath.Dir(sourceRoot), filepath.Base(sourceRoot)+".orktree")
}

// Path to the state file in the sibling dir for sourceRoot.
func StatePath(sourceRoot string) string {
	return filepath.Join(SiblingDir(sourceRoot), StateFile)
}

// Load reads the state file from the given source root.
func Load(sourceRoot string) (*State, error) {
	data, err := os.ReadFile(StatePath(sourceRoot))
	if err != nil {
		return nil, fmt.Errorf("reading state: %w", err)
	}
	var cfg State
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing state: %w", err)
	}
	return &cfg, nil
}

// Save writes the state back to disk (atomic: write to temp then rename).
func Save(cfg *State) error {
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
func Init(sourceRoot string, isGitRepo bool) (*State, error) {
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

	cfg := &State{
		SourceRoot: abs,
		IsGitRepo:  isGitRepo,
		Orktrees:   []Orktree{},
	}

	if err := Save(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Adds an orktree entry to cfg and saves state.
// mergedPath is the absolute path where the user works (the overlayfs mount point).
func NewOrktree(cfg *State, branch, mergedPath string) (Orktree, error) {
	w := Orktree{
		ID:         newID(),
		Branch:     branch,
		MergedPath: mergedPath,
		CreatedAt:  time.Now().UTC(),
	}
	cfg.Orktrees = append(cfg.Orktrees, w)
	return w, Save(cfg)
}

// Replaces the orktree entry with matching ID and saves.
func UpdateOrktree(cfg *State, w Orktree) error {
	for i, existing := range cfg.Orktrees {
		if existing.ID == w.ID {
			cfg.Orktrees[i] = w
			return Save(cfg)
		}
	}
	return fmt.Errorf("orktree %q not found", w.ID)
}

// Removes the orktree entry with the given ID and saves.
func RemoveOrktree(cfg *State, id string) error {
	for i, w := range cfg.Orktrees {
		if w.ID == id {
			cfg.Orktrees = append(cfg.Orktrees[:i], cfg.Orktrees[i+1:]...)
			return Save(cfg)
		}
	}
	return fmt.Errorf("orktree %q not found", id)
}

// Orktrees whose LowerOrktreeID matches id,
// i.e. orktrees that directly depend on the given orktree as their overlay base.
func Dependents(cfg *State, id string) []Orktree {
	var deps []Orktree
	for _, w := range cfg.Orktrees {
		if w.LowerOrktreeID == id {
			deps = append(deps, w)
		}
	}
	return deps
}

// Orktree matching ref by ID, branch, basename of MergedPath, absolute
// MergedPath, or prefix of any of the first three.
func FindOrktree(cfg *State, ref string) (Orktree, error) {
	// 1. Exact ID match.
	for _, w := range cfg.Orktrees {
		if w.ID == ref {
			return w, nil
		}
	}
	// 2. Exact branch match.
	for _, w := range cfg.Orktrees {
		if w.Branch == ref {
			return w, nil
		}
	}
	// 3. Exact basename of MergedPath match.
	for _, w := range cfg.Orktrees {
		if w.MergedPath != "" && filepath.Base(w.MergedPath) == ref {
			return w, nil
		}
	}
	// 4. Full absolute path match against MergedPath.
	for _, w := range cfg.Orktrees {
		if w.MergedPath != "" && w.MergedPath == ref {
			return w, nil
		}
	}
	// 5–7. Prefix matches (deduplicated).
	// Use a map to deduplicate (an orktree could match by multiple prefix types).
	seen := make(map[string]Orktree)
	for _, w := range cfg.Orktrees {
		// 5. ID prefix match.
		if len(ref) > 0 && len(w.ID) >= len(ref) && w.ID[:len(ref)] == ref {
			seen[w.ID] = w
			continue
		}
		// 6. Branch prefix match.
		if len(ref) > 0 && len(w.Branch) >= len(ref) && w.Branch[:len(ref)] == ref {
			seen[w.ID] = w
			continue
		}
		// 7. Basename of MergedPath prefix match.
		if w.MergedPath != "" {
			base := filepath.Base(w.MergedPath)
			if len(ref) > 0 && len(base) >= len(ref) && base[:len(ref)] == ref {
				seen[w.ID] = w
			}
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

// Upper and work directory paths for w's overlay.
// The merged directory is w.MergedPath (set at creation time).
func (c *State) OverlayDirs(w Orktree) (upper, work string) {
	base := filepath.Join(SiblingDir(c.SourceRoot), ".overlayfs", w.ID)
	return filepath.Join(base, "upper"), filepath.Join(base, "work")
}

// Random 6-character hex id.
func newID() string {
	b := make([]byte, 3)
	rand.Read(b) //nolint:gosec // ids need not be cryptographically random
	return fmt.Sprintf("%x", b)
}
