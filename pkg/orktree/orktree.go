// Package orktree provides a public API for managing copy-on-write orktrees
// backed by fuse-overlayfs and git worktrees.
package orktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	igit "github.com/patricklbell/orktree/internal/git"
	"github.com/patricklbell/orktree/internal/overlay"
	"github.com/patricklbell/orktree/internal/state"
)

// Manager wraps all orchestration logic for a single orktree-managed
// repository. Obtain one via Init, NewManager, or Discover.
type Manager struct {
	cfg *state.Config
}

// RemoveRefusedError is returned by Remove when safety checks fail.
type RemoveRefusedError struct {
	Branch          string
	Dependents      []string // branch names of dependent orktrees
	UnmergedCommits []string // short commit descriptions
	DirtyFiles      []string // relative paths in upper dir
}

func (e *RemoveRefusedError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "refusing to remove %q — has unmerged work", e.Branch)

	if len(e.Dependents) > 0 {
		b.WriteString("\n\nDependent orktrees (stacked on this one):")
		cap := len(e.Dependents)
		if cap > 10 {
			cap = 10
		}
		for _, d := range e.Dependents[:cap] {
			fmt.Fprintf(&b, "\n  %s", d)
		}
		if len(e.Dependents) > 10 {
			fmt.Fprintf(&b, "\n  ... and %d more", len(e.Dependents)-10)
		}
	}

	if len(e.DirtyFiles) > 0 {
		b.WriteString("\n\nUncommitted changes in overlay:")
		cap := len(e.DirtyFiles)
		if cap > 10 {
			cap = 10
		}
		for _, f := range e.DirtyFiles[:cap] {
			fmt.Fprintf(&b, "\n  %s", f)
		}
		if len(e.DirtyFiles) > 10 {
			fmt.Fprintf(&b, "\n  ... and %d more", len(e.DirtyFiles)-10)
		}
	}

	if len(e.UnmergedCommits) > 0 {
		b.WriteString("\n\nUnmerged commits (not in any other branch):")
		cap := len(e.UnmergedCommits)
		if cap > 10 {
			cap = 10
		}
		for _, c := range e.UnmergedCommits[:cap] {
			fmt.Fprintf(&b, "\n  %s", c)
		}
		if len(e.UnmergedCommits) > 10 {
			fmt.Fprintf(&b, "\n  ... and %d more", len(e.UnmergedCommits)-10)
		}
	}

	b.WriteString("\n\nUse --force to remove anyway.")
	return b.String()
}

// OrktreeInfo is a read-only snapshot of an orktree's metadata and status.
type OrktreeInfo struct {
	ID                 string
	Branch             string
	MergedPath         string
	Mounted            bool
	LowerOrktreeBranch string
	CreatedAt          time.Time
	UpperDirSize       int64 // bytes consumed in the CoW upper dir; -1 if unknown
}

// CreateOpts controls orktree creation behaviour.
type CreateOpts struct {
	From  string // base branch, git ref, or existing orktree
	NoGit bool   // skip git worktree registration (overlay-only)
}

// Prereq describes a single prerequisite check result.
type Prereq struct {
	Name string
	Fix  string
	OK   bool
}

// ---------------------------------------------------------------------------
// Construction / discovery
// ---------------------------------------------------------------------------

// Init initialises orktree for sourceRoot: creates the sibling data directory
// and state.json, then returns a Manager. Idempotent — if state already
// exists it is loaded without modification.
func Init(sourceRoot string) (*Manager, error) {
	abs, err := filepath.Abs(sourceRoot)
	if err != nil {
		return nil, err
	}
	if cfg, loadErr := state.Load(abs); loadErr == nil {
		return &Manager{cfg: cfg}, nil
	}
	isGit := igit.IsGitRepo(abs)
	cfg, err := state.Init(abs, isGit)
	if err != nil {
		return nil, err
	}
	return &Manager{cfg: cfg}, nil
}

// NewManager loads existing orktree state for sourceRoot.
// Returns an error if orktree has not been initialised.
func NewManager(sourceRoot string) (*Manager, error) {
	abs, err := filepath.Abs(sourceRoot)
	if err != nil {
		return nil, err
	}
	cfg, err := state.Load(abs)
	if err != nil {
		return nil, err
	}
	return &Manager{cfg: cfg}, nil
}

// Discover walks up from startDir to locate an initialised orktree,
// handling both source-root and merged-view working directories.
func Discover(startDir string) (*Manager, error) {
	abs, err := filepath.Abs(startDir)
	if err != nil {
		return nil, err
	}
	dir := abs
	for {
		// Case 1: dir is the source root — look for sibling .orktree/state.json
		sib := filepath.Join(filepath.Dir(dir), filepath.Base(dir)+".orktree")
		if _, err := os.Stat(filepath.Join(sib, state.StateFile)); err == nil {
			cfg, err := state.Load(dir)
			if err != nil {
				return nil, err
			}
			return &Manager{cfg: cfg}, nil
		}
		// Case 2: dir is inside a merged view — the .orktree suffix reveals the source root
		base := filepath.Base(dir)
		if strings.HasSuffix(base, ".orktree") {
			if _, err := os.Stat(filepath.Join(dir, state.StateFile)); err == nil {
				srcRoot := filepath.Join(filepath.Dir(dir), strings.TrimSuffix(base, ".orktree"))
				cfg, err := state.Load(srcRoot)
				if err != nil {
					return nil, err
				}
				return &Manager{cfg: cfg}, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil, fmt.Errorf("no orktree workspace found in %s or any parent directory", abs)
}

// CheckPrerequisites reports whether fuse-overlayfs, /dev/fuse, and git
// are available. Each entry contains a human-readable fix command when the
// prerequisite is not satisfied.
func CheckPrerequisites() []Prereq {
	_, fuseOfsErr := exec.LookPath("fuse-overlayfs")
	_, gitErr := exec.LookPath("git")
	return []Prereq{
		{
			Name: "fuse-overlayfs",
			Fix:  "sudo apt-get install fuse-overlayfs   # or: dnf / pacman / brew equivalent",
			OK:   fuseOfsErr == nil,
		},
		{
			Name: "fuse group (/dev/fuse)",
			Fix:  "sudo usermod -aG fuse $USER",
			OK:   canAccessFuseDev(),
		},
		{
			Name: "git",
			Fix:  "install git: https://git-scm.com/downloads",
			OK:   gitErr == nil,
		},
	}
}

// ---------------------------------------------------------------------------
// Accessors
// ---------------------------------------------------------------------------

// SourceRoot returns the absolute path of the source repository root.
func (m *Manager) SourceRoot() string {
	return m.cfg.SourceRoot
}

// IsGitRepo reports whether the managed source root is a git repository.
func (m *Manager) IsGitRepo() bool {
	return m.cfg.IsGitRepo
}

// ---------------------------------------------------------------------------
// Operations
// ---------------------------------------------------------------------------

// validateBranch rejects branch names that would escape the orktree directory
// or are otherwise invalid for use as filesystem paths.
func validateBranch(branch string) error {
	if branch == "" {
		return fmt.Errorf("branch name must not be empty")
	}
	if strings.ContainsRune(branch, 0) {
		return fmt.Errorf("branch name contains invalid characters")
	}
	cleaned := filepath.Clean(branch)
	if filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, "..") {
		return fmt.Errorf("branch name %q would escape the orktree directory", branch)
	}
	return nil
}

// Create creates a new orktree for branch: adds a state entry, sets up git
// (unless NoGit), and mounts the overlay. Returns info for the newly created
// orktree.
func (m *Manager) Create(branch string, opts CreateOpts) (OrktreeInfo, error) {
	if err := validateBranch(branch); err != nil {
		return OrktreeInfo{}, err
	}
	if _, err := state.FindOrktree(m.cfg, branch); err == nil {
		return OrktreeInfo{}, fmt.Errorf("orktree %q already exists", branch)
	}

	w, err := state.NewOrktree(m.cfg, branch)
	if err != nil {
		return OrktreeInfo{}, err
	}

	upper, work, merged := m.cfg.OverlayDirs(w)

	var lowerDir string
	if m.cfg.IsGitRepo && !opts.NoGit {
		lowerDir, err = m.setupGitForOrktree(&w, branch, opts.From, upper)
		if err != nil {
			state.RemoveOrktree(m.cfg, w.ID) //nolint:errcheck
			return OrktreeInfo{}, err
		}
		if err := state.UpdateOrktree(m.cfg, w); err != nil {
			return OrktreeInfo{}, err
		}
	} else {
		lowerDir = m.cfg.SourceRoot
	}

	if err := overlay.Create(upper, work, merged); err != nil {
		state.RemoveOrktree(m.cfg, w.ID) //nolint:errcheck
		return OrktreeInfo{}, err
	}
	if err := overlay.Mount(lowerDir, upper, work, merged); err != nil {
		state.RemoveOrktree(m.cfg, w.ID) //nolint:errcheck
		return OrktreeInfo{}, err
	}

	return m.orktreeInfo(w)
}

// EnsureReady finds or creates the orktree for branch and ensures it
// (and any ancestor overlays) are mounted. This is the method that the
// switch and path commands use.
func (m *Manager) EnsureReady(branch string, opts CreateOpts) (OrktreeInfo, error) {
	w, err := state.FindOrktree(m.cfg, branch)
	if err != nil {
		return m.Create(branch, opts)
	}
	if opts.From != "" || opts.NoGit {
		return OrktreeInfo{}, fmt.Errorf("orktree %q already exists; --from and --no-git are only used during creation", branch)
	}
	if err := m.ensureMountedWithAncestors(w, make(map[string]bool)); err != nil {
		return OrktreeInfo{}, err
	}
	return m.orktreeInfo(w)
}

// List returns info for every orktree, including mount status and upper
// directory size.
func (m *Manager) List() ([]OrktreeInfo, error) {
	infos := make([]OrktreeInfo, 0, len(m.cfg.Orktrees))
	for _, w := range m.cfg.Orktrees {
		info, err := m.orktreeInfo(w)
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}

// UpperDirFiles returns paths of files in the overlay upper directory that
// genuinely differ from the lower layer. Files copied up by the overlay but
// reverted to identical content are excluded. Returns at most limit paths and
// the total count of dirty files (which may exceed len(files) when truncated).
func (m *Manager) UpperDirFiles(ref string, limit int) ([]string, int, error) {
	w, err := state.FindOrktree(m.cfg, ref)
	if err != nil {
		return nil, 0, err
	}
	upper, _, _ := m.cfg.OverlayDirs(w)

	return overlay.DirtyUpperFiles(upper, m.cfg.MountPath(w), limit)
}

// Remove unmounts the overlay, deregisters the git worktree, and deletes
// the orktree state entry. When force is true, safety checks and errors
// from unmount and worktree removal are ignored.
func (m *Manager) Remove(ref string, force bool) error {
	w, err := state.FindOrktree(m.cfg, ref)
	if err != nil {
		return err
	}

	// Safety checks — skipped when --force is set.
	if !force && w.GitTreePath != "" {
		refused := &RemoveRefusedError{Branch: w.Branch}

		deps := state.Dependents(m.cfg, w.Branch)
		for _, d := range deps {
			refused.Dependents = append(refused.Dependents, d.Branch)
		}

		upper, _, _ := m.cfg.OverlayDirs(w)
		dirtyFiles, _, _ := overlay.DirtyUpperFiles(upper, m.cfg.MountPath(w), 0)
		refused.DirtyFiles = dirtyFiles

		commits, _ := igit.UnmergedCommits(m.cfg.SourceRoot, w.Branch, 10)
		refused.UnmergedCommits = commits

		if len(refused.Dependents) > 0 || len(refused.DirtyFiles) > 0 || len(refused.UnmergedCommits) > 0 {
			return refused
		}
	}

	upper, work, merged := m.cfg.OverlayDirs(w)

	if err := overlay.Remove(upper, work, merged); err != nil && !force {
		return fmt.Errorf("removing overlay: %w (use --force to ignore)", err)
	}

	cleanEmptyAncestors(merged, state.SiblingDir(m.cfg.SourceRoot))

	if w.GitTreePath != "" {
		if err := igit.RemoveWorktree(m.cfg.SourceRoot, w.GitTreePath); err != nil && !force {
			return fmt.Errorf("removing git worktree: %w (use --force to ignore)", err)
		}
		igit.PruneWorktrees(m.cfg.SourceRoot) //nolint:errcheck
	}

	return state.RemoveOrktree(m.cfg, w.ID)
}

// Path returns the merged overlay path for an existing orktree.
func (m *Manager) Path(ref string) (string, error) {
	w, err := state.FindOrktree(m.cfg, ref)
	if err != nil {
		return "", err
	}
	_, _, merged := m.cfg.OverlayDirs(w)
	return merged, nil
}

// Find returns info for the orktree matching ref (by ID, branch name,
// or unique prefix).
func (m *Manager) Find(ref string) (OrktreeInfo, error) {
	w, err := state.FindOrktree(m.cfg, ref)
	if err != nil {
		return OrktreeInfo{}, err
	}
	return m.orktreeInfo(w)
}

// ---------------------------------------------------------------------------
// Private helpers
// ---------------------------------------------------------------------------

func (m *Manager) ensureMountedWithAncestors(w state.Orktree, visited map[string]bool) error {
	if visited[w.ID] {
		return fmt.Errorf("cycle detected in orktree parent chain at %q", w.Branch)
	}
	visited[w.ID] = true

	if w.LowerOrktreeBranch != "" {
		parent, err := state.FindOrktree(m.cfg, w.LowerOrktreeBranch)
		if err != nil {
			return err
		}
		if err := m.ensureMountedWithAncestors(parent, visited); err != nil {
			return err
		}
	}

	upper, work, merged := m.cfg.OverlayDirs(w)
	return overlay.EnsureMounted(m.cfg.MountPath(w), upper, work, merged)
}

// setupGitForOrktree decides which git setup path to take and returns the
// overlayfs lowerdir. It populates the git-related fields of *w.
//
// Zero-cost paths skip a full checkout by reusing an existing filesystem as
// lowerdir and seeding only a .git gitfile into the overlay upper dir.
func (m *Manager) setupGitForOrktree(w *state.Orktree, branch, from, upper string) (string, error) {
	treeDir := m.cfg.GitTreeDir(*w)

	// ---- Zero-cost path A: --from refers to an existing orktree ---------
	if from != "" {
		fromOrk, err := state.FindOrktree(m.cfg, from)
		if err == nil {
			_, _, fromMerged := m.cfg.OverlayDirs(fromOrk)
			mounted, _ := overlay.IsMounted(fromMerged)
			if !mounted {
				return "", fmt.Errorf("orktree %q is not mounted; mount it first", from)
			}

			exists, err := igit.BranchExists(m.cfg.SourceRoot, branch)
			if err != nil {
				return "", err
			}
			if !exists {
				if err := igit.CreateBranch(m.cfg.SourceRoot, branch, fromOrk.Branch); err != nil {
					return "", fmt.Errorf("creating branch: %w", err)
				}
			}
			if err := igit.AddWorktreeNoCheckout(m.cfg.SourceRoot, treeDir, branch); err != nil {
				return "", fmt.Errorf("registering git worktree: %w", err)
			}
			if err := seedGitFile(treeDir, upper); err != nil {
				return "", err
			}
			w.GitTreePath = treeDir
			w.LowerDir = fromMerged
			w.LowerOrktreeBranch = fromOrk.Branch
			return fromMerged, nil
		}
	}

	// ---- Zero-cost path B: no --from, or --from matches source root -----
	currentBranch, _ := igit.CurrentBranch(m.cfg.SourceRoot)
	if from == "" || from == currentBranch {
		exists, err := igit.BranchExists(m.cfg.SourceRoot, branch)
		if err != nil {
			return "", err
		}
		if !exists {
			if err := igit.CreateBranch(m.cfg.SourceRoot, branch, from); err != nil {
				return "", fmt.Errorf("creating branch: %w", err)
			}
		}
		if err := igit.AddWorktreeNoCheckout(m.cfg.SourceRoot, treeDir, branch); err != nil {
			return "", fmt.Errorf("registering git worktree: %w", err)
		}
		if err := seedGitFile(treeDir, upper); err != nil {
			return "", err
		}
		w.GitTreePath = treeDir
		w.LowerDir = m.cfg.SourceRoot
		return m.cfg.SourceRoot, nil
	}

	// ---- Conventional path: --from <git-ref> not matching any orktree ---
	exists, err := igit.BranchExists(m.cfg.SourceRoot, branch)
	if err != nil {
		return "", err
	}
	newBranch := !exists
	if err := igit.AddWorktree(m.cfg.SourceRoot, treeDir, branch, newBranch, from); err != nil {
		return "", fmt.Errorf("creating git worktree: %w", err)
	}
	w.GitTreePath = treeDir
	w.LowerDir = treeDir
	return treeDir, nil
}

// seedGitFile copies the .git gitfile from the no-checkout worktree directory
// into upper/ so that git commands inside the merged overlay path track the
// correct branch rather than the lowerdir's branch.
func seedGitFile(treeDir, upper string) error {
	gitFileData, err := os.ReadFile(filepath.Join(treeDir, ".git"))
	if err != nil {
		return fmt.Errorf("reading git worktree pointer: %w", err)
	}
	if err := os.MkdirAll(upper, 0o755); err != nil {
		return fmt.Errorf("creating overlay upper dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(upper, ".git"), gitFileData, 0o644); err != nil {
		return fmt.Errorf("seeding .git into overlay upper: %w", err)
	}
	return nil
}

// cleanEmptyAncestors removes empty directories from path up to (but not
// including) stopAt.
func cleanEmptyAncestors(path, stopAt string) {
	for {
		parent := filepath.Dir(path)
		if parent == path || path == stopAt || !strings.HasPrefix(path, stopAt) {
			return
		}
		if err := os.Remove(path); err != nil {
			return // non-empty or other error — stop
		}
		path = parent
	}
}

// canAccessFuseDev reports whether /dev/fuse is accessible to the current process.
func canAccessFuseDev() bool {
	f, err := os.Open("/dev/fuse")
	if err != nil {
		return false
	}
	f.Close()
	return true
}

func (m *Manager) orktreeInfo(w state.Orktree) (OrktreeInfo, error) {
	upper, _, merged := m.cfg.OverlayDirs(w)
	mounted, err := overlay.IsMounted(merged)
	if err != nil {
		return OrktreeInfo{}, fmt.Errorf("checking mount status: %w", err)
	}
	return OrktreeInfo{
		ID:                 w.ID,
		Branch:             w.Branch,
		MergedPath:         merged,
		Mounted:            mounted,
		LowerOrktreeBranch: w.LowerOrktreeBranch,
		CreatedAt:          w.CreatedAt,
		UpperDirSize:       dirSize(upper),
	}, nil
}

func dirSize(path string) int64 {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	if err != nil {
		return -1
	}
	return size
}
