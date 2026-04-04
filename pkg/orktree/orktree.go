package orktree

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/patricklbell/orktree/internal/git"
	"github.com/patricklbell/orktree/internal/overlay"
	"github.com/patricklbell/orktree/internal/state"
)

type Index struct {
	state *state.State
}

func (i *Index) SourceRoot() string {
	return i.state.SourceRoot
}

func (i *Index) IsGitRepo() bool {
	return i.state.IsGitRepo
}

type Prerequisite struct {
	Name     string
	Fix      string
	OK       bool
	Optional bool // advisory check — not required for core functionality
}

func CheckEnvironmentPrerequisites() []Prerequisite {
	_, fuseOfsErr := exec.LookPath("fuse-overlayfs")
	_, gitErr := exec.LookPath("git")
	return []Prerequisite{
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
		{
			Name:     "user_allow_other (/etc/fuse.conf)",
			Fix:      "echo 'user_allow_other' | sudo tee -a /etc/fuse.conf   # needed for Docker bind-mounts",
			OK:       overlay.UserAllowOther(),
			Optional: true,
		},
	}
}

// read-only snapshot of an orktree's metadata
type OrktreeMetadata struct {
	ID             string
	Branch         string
	MergedPath     string
	Mounted        bool
	LowerOrktreeID string
	CreatedAt      time.Time
	UpperDirSize   int64 // bytes consumed in the CoW upper dir; -1 if unknown
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

// initialises new orktree index for sourceRoot: creates the sibling data directory
// and state.json. Idempotent — if state already exists it is loaded without modification.
func CreateIndex(sourceRoot string) (*Index, error) {
	if cfg, loadErr := state.Load(sourceRoot); loadErr == nil {
		return &Index{state: cfg}, nil
	}
	isGit := git.IsGitRepo(sourceRoot)
	cfg, err := state.Init(sourceRoot, isGit)
	if err != nil {
		return nil, err
	}
	return &Index{state: cfg}, nil
}

// Loads existing orktree state for sourceRoot.
// Returns an error if orktree has not been initialised.
func LoadIndex(sourceRoot string) (*Index, error) {
	cfg, err := state.Load(sourceRoot)
	if err != nil {
		return nil, err
	}
	return &Index{state: cfg}, nil
}

// Walks up from startDir to locate an initialised orktree,
func DiscoverIndex(startDir string) (*Index, error) {
	currDir := startDir
	for {
		// Case 1: source root — look for siblingIndexDir .orktree/state.json
		siblingIndexDir := filepath.Join(filepath.Dir(currDir), filepath.Base(currDir)+".orktree")
		if _, err := os.Stat(filepath.Join(siblingIndexDir, state.StateFile)); err == nil {
			cfg, err := state.Load(currDir)
			if err != nil {
				return nil, err
			}
			return &Index{state: cfg}, nil
		}

		// Case 2: inside a merged view — the .orktree suffix reveals the source root
		currBase := filepath.Base(currDir)
		if strings.HasSuffix(currBase, ".orktree") {
			if _, err := os.Stat(filepath.Join(currDir, state.StateFile)); err == nil {
				srcRoot := filepath.Join(filepath.Dir(currDir), strings.TrimSuffix(currBase, ".orktree"))
				cfg, err := state.Load(srcRoot)
				if err != nil {
					return nil, err
				}
				return &Index{state: cfg}, nil
			}
		}

		parentDir := filepath.Dir(currDir)
		if parentDir == currDir {
			break
		}
		currDir = parentDir
	}
	return nil, fmt.Errorf("no orktree workspace found in %s or any parent directory", startDir)
}

// ---------------------------------------------------------------------------
// Add
// ---------------------------------------------------------------------------

type AddOrktreeOptions struct {
	CommitIsh string   // optional: git ref or existing orktree name (auto-detected)
	ExtraArgs []string // forwarded to git worktree add after --
}

// AddOrktree creates a new orktree at the given path. The branch name is derived
// from the basename of path. If CommitIsh names an existing orktree, the new
// orktree is stacked on top of it; otherwise CommitIsh is treated as a git ref.
func (i *Index) AddOrktree(path string, opts AddOrktreeOptions) (OrktreeMetadata, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return OrktreeMetadata{}, fmt.Errorf("resolving path: %w", err)
	}

	// Reject duplicate merged paths.
	for _, existing := range i.state.Orktrees {
		if existing.MergedPath == absPath {
			return OrktreeMetadata{}, fmt.Errorf("an orktree already exists at %q", absPath)
		}
	}

	branch := filepath.Base(absPath)
	sourceRoot := i.state.SourceRoot

	// Register the orktree in state early so we have an ID for overlay dirs.
	w, err := state.NewOrktree(i.state, branch, absPath)
	if err != nil {
		return OrktreeMetadata{}, err
	}

	// On any failure below, clean up the state entry.
	cleanup := func() { state.RemoveOrktree(i.state, w.ID) } //nolint:errcheck

	var lowerDir string

	if i.state.IsGitRepo {
		lowerDir, err = i.setupGitForAdd(&w, branch, opts, absPath)
		if err != nil {
			cleanup()
			return OrktreeMetadata{}, err
		}
	} else {
		lowerDir = sourceRoot
	}

	w.LowerDir = lowerDir
	if err := state.UpdateOrktree(i.state, w); err != nil {
		cleanup()
		return OrktreeMetadata{}, err
	}

	upper, work := i.state.OverlayDirs(w)

	if err := overlay.Create(upper, work); err != nil {
		cleanup()
		return OrktreeMetadata{}, err
	}
	// Ensure the mount point exists. For git repos AddWorktreeNoCheckout already
	// creates it, but for non-git repos (and ExtraArgs paths) we must do it here.
	if err := os.MkdirAll(absPath, 0o755); err != nil {
		cleanup()
		return OrktreeMetadata{}, fmt.Errorf("creating mount point: %w", err)
	}
	if err := overlay.Mount(lowerDir, upper, work, absPath); err != nil {
		cleanup()
		return OrktreeMetadata{}, err
	}

	return i.createOrktreeInfo(w)
}

// setupGitForAdd handles git branch creation, worktree registration, and .git
// seeding for AddOrktree. It returns the overlay lower directory and may update
// w.LowerOrktreeID.
func (i *Index) setupGitForAdd(w *state.Orktree, branch string, opts AddOrktreeOptions, absPath string) (string, error) {
	sourceRoot := i.state.SourceRoot
	upper, _ := i.state.OverlayDirs(*w)

	if opts.CommitIsh != "" {
		// Try to resolve CommitIsh as an existing orktree (stacking).
		parent, findErr := state.FindOrktree(i.state, opts.CommitIsh)
		if findErr == nil {
			mounted, _ := overlay.IsMounted(parent.MergedPath)
			if !mounted {
				return "", fmt.Errorf("orktree %q is not mounted; mount it first", opts.CommitIsh)
			}

			exists, err := git.BranchExists(sourceRoot, branch)
			if err != nil {
				return "", err
			}
			if !exists {
				if err := git.CreateBranch(sourceRoot, branch, parent.Branch); err != nil {
					return "", fmt.Errorf("creating branch: %w", err)
				}
			}

			if len(opts.ExtraArgs) > 0 {
				if err := git.AddWorktreeForward(sourceRoot, opts.ExtraArgs); err != nil {
					return "", fmt.Errorf("registering git worktree: %w", err)
				}
			} else {
				if err := git.AddWorktreeNoCheckout(sourceRoot, absPath, branch); err != nil {
					return "", fmt.Errorf("registering git worktree: %w", err)
				}
			}

			if err := seedGitFile(absPath, upper); err != nil {
				return "", err
			}
			if err := seedSubmoduleGitFiles(parent.MergedPath, upper); err != nil {
				return "", err
			}
			w.LowerOrktreeID = parent.ID
			return parent.MergedPath, nil
		}

		// CommitIsh is a git ref, not an existing orktree.
		exists, err := git.BranchExists(sourceRoot, branch)
		if err != nil {
			return "", err
		}
		if !exists {
			if err := git.CreateBranch(sourceRoot, branch, opts.CommitIsh); err != nil {
				return "", fmt.Errorf("creating branch: %w", err)
			}
		}

		if len(opts.ExtraArgs) > 0 {
			if err := git.AddWorktreeForward(sourceRoot, opts.ExtraArgs); err != nil {
				return "", fmt.Errorf("registering git worktree: %w", err)
			}
		} else {
			if err := git.AddWorktreeNoCheckout(sourceRoot, absPath, branch); err != nil {
				return "", fmt.Errorf("registering git worktree: %w", err)
			}
		}

		if err := seedGitFile(absPath, upper); err != nil {
			return "", err
		}
		if err := seedSubmoduleGitFiles(sourceRoot, upper); err != nil {
			return "", err
		}
		return sourceRoot, nil
	}

	// No CommitIsh — default to HEAD.
	exists, err := git.BranchExists(sourceRoot, branch)
	if err != nil {
		return "", err
	}
	if !exists {
		if err := git.CreateBranch(sourceRoot, branch, ""); err != nil {
			return "", fmt.Errorf("creating branch: %w", err)
		}
	}

	if len(opts.ExtraArgs) > 0 {
		if err := git.AddWorktreeForward(sourceRoot, opts.ExtraArgs); err != nil {
			return "", fmt.Errorf("registering git worktree: %w", err)
		}
	} else {
		if err := git.AddWorktreeNoCheckout(sourceRoot, absPath, branch); err != nil {
			return "", fmt.Errorf("registering git worktree: %w", err)
		}
	}

	if err := seedGitFile(absPath, upper); err != nil {
		return "", err
	}
	if err := seedSubmoduleGitFiles(sourceRoot, upper); err != nil {
		return "", err
	}
	return sourceRoot, nil
}

// ---------------------------------------------------------------------------
// Mount / Unmount
// ---------------------------------------------------------------------------

// MountOrktree ensures the orktree identified by ref (and all its ancestors)
// are mounted.
func (i *Index) MountOrktree(ref string) error {
	w, err := state.FindOrktree(i.state, ref)
	if err != nil {
		return err
	}
	return i.ensureMountedWithAncestors(w, make(map[string]bool))
}

// UnmountOrktree unmounts the overlay for the orktree identified by ref.
// No-op if not currently mounted.
func (i *Index) UnmountOrktree(ref string) error {
	w, err := state.FindOrktree(i.state, ref)
	if err != nil {
		return err
	}
	mounted, err := overlay.IsMounted(w.MergedPath)
	if err != nil {
		return fmt.Errorf("checking mount status: %w", err)
	}
	if !mounted {
		return nil
	}
	return overlay.Unmount(w.MergedPath)
}

// ---------------------------------------------------------------------------
// Move
// ---------------------------------------------------------------------------

// MoveOrktree relocates the orktree identified by ref to newPath. The overlay
// is unmounted from the old location, the git worktree is moved (if applicable),
// the state is updated, and the overlay is remounted at the new path.
func (i *Index) MoveOrktree(ref, newPath string) error {
	w, err := state.FindOrktree(i.state, ref)
	if err != nil {
		return err
	}

	absNewPath, err := filepath.Abs(newPath)
	if err != nil {
		return fmt.Errorf("resolving new path: %w", err)
	}

	oldMergedPath := w.MergedPath

	// Unmount overlay from old location.
	mounted, err := overlay.IsMounted(oldMergedPath)
	if err != nil {
		return fmt.Errorf("checking mount status: %w", err)
	}
	if mounted {
		if err := overlay.Unmount(oldMergedPath); err != nil {
			return fmt.Errorf("unmounting overlay: %w", err)
		}
	}

	// Move git worktree if applicable.
	if i.state.IsGitRepo {
		if err := git.MoveWorktree(i.state.SourceRoot, oldMergedPath, absNewPath); err != nil {
			return fmt.Errorf("moving git worktree: %w", err)
		}
	}

	// Update state.
	w.MergedPath = absNewPath
	if err := state.UpdateOrktree(i.state, w); err != nil {
		return err
	}

	// Remount overlay at new path.
	upper, work := i.state.OverlayDirs(w)
	lowerDir := w.LowerDir
	if lowerDir == "" {
		lowerDir = i.state.SourceRoot
	}
	return overlay.Mount(lowerDir, upper, work, absNewPath)
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func (i *Index) ListOrktrees() ([]OrktreeMetadata, error) {
	infos := make([]OrktreeMetadata, 0, len(i.state.Orktrees))
	for _, w := range i.state.Orktrees {
		info, err := i.createOrktreeInfo(w)
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}

// Paths of files in the overlay upper directory that
// genuinely differ from the lower layer. Files copied up by the overlay but
// reverted to identical content are excluded. Returns at most limit paths and
// the total count of dirty files (which may exceed len(files) when truncated).
func (i *Index) ListOrktreesUpperDirFiles(ref string, limit int) (files []string, count int, err error) {
	w, err := state.FindOrktree(i.state, ref)
	if err != nil {
		return nil, 0, err
	}
	upper, _ := i.state.OverlayDirs(w)

	lowerDir := w.LowerDir
	if lowerDir == "" {
		lowerDir = i.state.SourceRoot
	}
	return overlay.DirtyUpperFiles(upper, lowerDir, limit)
}

// ---------------------------------------------------------------------------
// Remove
// ---------------------------------------------------------------------------

type RemoveCheck struct {
	Branch          string
	MergedPath      string
	Dependents      []string // branch names of dependent orktrees
	UnmergedCommits []string // short commit descriptions (at most 10)
	UnmergedTotal   int      // true count of unmerged commits
	TrackedDirty    []string // modified/deleted tracked files (at most 10)
	UntrackedDirty  []string // new files not in .gitignore (at most 10)
	IgnoredDirty    int      // count of gitignored files (cache/build artifacts)
	TrackedTotal    int      // true count of tracked dirty files
	UntrackedTotal  int      // true count of untracked non-ignored files
}

// reports whether the orktree has no uncommitted work worth preserving.
// Ignored files (cache/build artifacts) are not considered.
func (c *RemoveCheck) IsClean() bool {
	return len(c.Dependents) == 0 && c.TrackedTotal == 0 && c.UntrackedTotal == 0 && c.UnmergedTotal == 0
}

// IsCleanWith reports whether the orktree has no uncommitted work that would
// block removal given the specified ignore flags. ignoreUntracked skips the
// untracked-files check; ignoreTracked skips the tracked-dirty-files check.
// Unmerged commits and dependents are never ignored.
func (c *RemoveCheck) IsCleanWith(ignoreUntracked, ignoreTracked bool) bool {
	if len(c.Dependents) > 0 || c.UnmergedTotal > 0 {
		return false
	}
	if !ignoreTracked && c.TrackedTotal > 0 {
		return false
	}
	if !ignoreUntracked && c.UntrackedTotal > 0 {
		return false
	}
	return true
}

// reports whether removal should be refused regardless of user confirmation.
// Currently this is true only when other orktrees depend on this one as a base layer.
func (c *RemoveCheck) HasBlockers() bool {
	return len(c.Dependents) > 0
}

// Read-only, does not modify state.
func (i *Index) CheckRemoveOrktree(ref string) (*RemoveCheck, error) {
	w, err := state.FindOrktree(i.state, ref)
	if err != nil {
		return nil, err
	}

	upper, _ := i.state.OverlayDirs(w)
	rc := &RemoveCheck{
		Branch:     w.Branch,
		MergedPath: w.MergedPath,
	}

	// Dependents — other orktrees stacked on this one.
	for _, d := range state.Dependents(i.state, w.ID) {
		rc.Dependents = append(rc.Dependents, d.Branch)
	}

	// Lower dir for dirty-file comparison.
	lowerDir := w.LowerDir
	if lowerDir == "" {
		lowerDir = i.state.SourceRoot
	}

	// Dirty files in the overlay upper dir.
	dirtyFiles, _, err := overlay.DirtyUpperFiles(upper, lowerDir, 0)
	if err != nil {
		return nil, fmt.Errorf("assessing overlay changes: %w", err)
	}

	if i.state.IsGitRepo {
		// Classify dirty files using git ignore rules.
		ignoredSet := make(map[string]bool)

		if len(dirtyFiles) > 0 {
			// Intentionally discarding error: if check-ignore fails we skip
			// classification and treat all files as non-ignored, producing
			// conservative over-warning rather than silent suppression.
			ignored, _ := git.CheckIgnored(i.state.SourceRoot, dirtyFiles)
			for _, p := range ignored {
				ignoredSet[p] = true
			}
		}

		for _, f := range dirtyFiles {
			if ignoredSet[f] {
				rc.IgnoredDirty++
				continue
			}
			if _, err := os.Stat(filepath.Join(lowerDir, f)); err == nil {
				rc.TrackedTotal++
				if len(rc.TrackedDirty) < 10 {
					rc.TrackedDirty = append(rc.TrackedDirty, f)
				}
			} else {
				rc.UntrackedTotal++
				if len(rc.UntrackedDirty) < 10 {
					rc.UntrackedDirty = append(rc.UntrackedDirty, f)
				}
			}
		}

		commits, _ := git.UnmergedCommits(i.state.SourceRoot, w.Branch, 10)
		rc.UnmergedCommits = commits
		if total, err := git.UnmergedCommitCount(i.state.SourceRoot, w.Branch); err == nil {
			rc.UnmergedTotal = total
		} else {
			rc.UnmergedTotal = len(commits)
		}
	} else { // non-git: treat all dirty files as untracked.
		rc.UntrackedTotal = len(dirtyFiles)
		if len(dirtyFiles) > 10 {
			rc.UntrackedDirty = dirtyFiles[:10]
		} else {
			rc.UntrackedDirty = dirtyFiles
		}
	}

	return rc, nil
}

// Unmounts the overlay, deregisters the git worktree, and deletes
// the orktree state entry unconditionally.
func (i *Index) RemoveOrktree(ref string) error {
	w, err := state.FindOrktree(i.state, ref)
	if err != nil {
		return err
	}

	upper, _ := i.state.OverlayDirs(w)

	if err := overlay.Remove(upper, w.MergedPath); err != nil {
		return fmt.Errorf("removing overlay: %w", err)
	}

	if i.state.IsGitRepo {
		if err := git.RemoveWorktree(i.state.SourceRoot, w.MergedPath); err != nil {
			return fmt.Errorf("removing git worktree: %w", err)
		}
		git.PruneWorktrees(i.state.SourceRoot) //nolint:errcheck
	}

	return state.RemoveOrktree(i.state, w.ID)
}

// ---------------------------------------------------------------------------
// Resolve path
// ---------------------------------------------------------------------------

func (i *Index) ResolveOrktreePath(ref string) (string, error) {
	w, err := state.FindOrktree(i.state, ref)
	if err != nil {
		return "", err
	}
	return w.MergedPath, nil
}

// ---------------------------------------------------------------------------
// Find
// ---------------------------------------------------------------------------

func (i *Index) FindOrktree(ref string) (OrktreeMetadata, error) {
	w, err := state.FindOrktree(i.state, ref)
	if err != nil {
		return OrktreeMetadata{}, err
	}
	return i.createOrktreeInfo(w)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func (i *Index) ensureMountedWithAncestors(w state.Orktree, visited map[string]bool) error {
	if visited[w.ID] {
		return fmt.Errorf("cycle detected in orktree parent chain at %q", w.Branch)
	}
	visited[w.ID] = true

	if w.LowerOrktreeID != "" {
		parent, err := state.FindOrktree(i.state, w.LowerOrktreeID)
		if err != nil {
			return err
		}
		if err := i.ensureMountedWithAncestors(parent, visited); err != nil {
			return err
		}
	}

	upper, work := i.state.OverlayDirs(w)
	lowerDir := w.LowerDir
	if lowerDir == "" {
		lowerDir = i.state.SourceRoot
	}
	return overlay.EnsureMounted(lowerDir, upper, work, w.MergedPath)
}

// Copies the .git gitfile from the worktree directory into upper. This ensures
// git commands inside the merged overlay path track the correct branch rather
// than the lowerdir's branch.
func seedGitFile(worktreePath, upper string) error {
	gitFileData, err := os.ReadFile(filepath.Join(worktreePath, ".git"))
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

// Seeds submodule .git gitfiles from lowerDir into upper, rewriting any relative
// gitdir paths to absolute ones. This is necessary because submodule .git files
// contain relative paths computed relative to the submodule's location in lowerDir.
// When accessed through the overlayfs merged view (which sits at a different
// directory depth), those relative paths would resolve to the wrong location.
func seedSubmoduleGitFiles(lowerDir, upper string) error {
	return filepath.WalkDir(lowerDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip .git directories — we only want .git gitfiles (submodule pointers),
			// not the actual git object stores.
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != ".git" {
			return nil
		}
		// Skip the root-level .git file (already handled by seedGitFile).
		if filepath.Dir(path) == lowerDir {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading submodule git pointer %s: %w", path, err)
		}
		content := strings.TrimSpace(string(data))
		if !strings.HasPrefix(content, "gitdir: ") {
			return nil
		}
		gitdirPath := strings.TrimPrefix(content, "gitdir: ")
		// Resolve relative gitdir paths to absolute so they remain valid from the
		// merged view, which lives at a different directory depth than lowerDir.
		if !filepath.IsAbs(gitdirPath) {
			gitdirPath = filepath.Clean(filepath.Join(filepath.Dir(path), gitdirPath))
		}
		rel, err := filepath.Rel(lowerDir, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(upper, rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("creating submodule dir in overlay upper: %w", err)
		}
		return os.WriteFile(dest, []byte("gitdir: "+gitdirPath+"\n"), 0o644)
	})
}

func canAccessFuseDev() bool {
	f, err := os.Open("/dev/fuse")
	if err != nil {
		return false
	}
	f.Close()
	return true
}

func (i *Index) createOrktreeInfo(w state.Orktree) (OrktreeMetadata, error) {
	upper, _ := i.state.OverlayDirs(w)
	mounted, err := overlay.IsMounted(w.MergedPath)
	if err != nil {
		return OrktreeMetadata{}, fmt.Errorf("checking mount status: %w", err)
	}
	return OrktreeMetadata{
		ID:             w.ID,
		Branch:         w.Branch,
		MergedPath:     w.MergedPath,
		Mounted:        mounted,
		LowerOrktreeID: w.LowerOrktreeID,
		CreatedAt:      w.CreatedAt,
		UpperDirSize:   calculateDirectoryTotalBytes(upper),
	}, nil
}

func calculateDirectoryTotalBytes(path string) int64 {
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
