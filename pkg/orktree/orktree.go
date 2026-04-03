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
	ID                 string
	Name               string
	Branch             string
	MergedPath         string
	Mounted            bool
	LowerOrktreeBranch string
	CreatedAt          time.Time
	UpperDirSize       int64 // bytes consumed in the CoW upper dir; -1 if unknown
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

// controls orktree creation behaviour.
type CreateOrktreeOptions struct {
	From  string // base branch, git ref, or existing orktree
	NoGit bool   // skip git worktree registration (overlay-only)
	Name  string // human-visible label and directory name; defaults to branch when empty
}

// adds a state entry, sets up git (unless NoGit), and mounts the overlay.
// Returns info for the newly created orktree.
func (i *Index) CreateOrktree(branch string, opts CreateOrktreeOptions) (OrktreeMetadata, error) {
	if err := validateBranchName(branch); err != nil {
		return OrktreeMetadata{}, err
	}
	if opts.Name != "" {
		if err := validateName(opts.Name); err != nil {
			return OrktreeMetadata{}, err
		}
	}
	if _, err := state.FindOrktree(i.state, branch); err == nil {
		return OrktreeMetadata{}, fmt.Errorf("orktree %q already exists", branch)
	}
	// When a custom name is provided also guard against duplicate names.
	if opts.Name != "" {
		if _, err := state.FindOrktree(i.state, opts.Name); err == nil {
			return OrktreeMetadata{}, fmt.Errorf("an orktree named %q already exists", opts.Name)
		}
	}

	w, err := state.NewOrktree(i.state, branch, opts.Name)
	if err != nil {
		return OrktreeMetadata{}, err
	}

	upper, work, merged := i.state.OverlayDirs(w)

	var lowerDir string
	if i.state.IsGitRepo && !opts.NoGit {
		lowerDir, err = i.setupGitForOrktree(&w, branch, opts.From, upper)
		if err != nil {
			state.RemoveOrktree(i.state, w.ID) //nolint:errcheck
			return OrktreeMetadata{}, err
		}
		if err := state.UpdateOrktree(i.state, w); err != nil {
			return OrktreeMetadata{}, err
		}
	} else {
		lowerDir = i.state.SourceRoot
	}

	if err := overlay.Create(upper, work, merged); err != nil {
		state.RemoveOrktree(i.state, w.ID) //nolint:errcheck
		return OrktreeMetadata{}, err
	}
	if err := overlay.Mount(lowerDir, upper, work, merged); err != nil {
		state.RemoveOrktree(i.state, w.ID) //nolint:errcheck
		return OrktreeMetadata{}, err
	}

	return i.createOrktreeInfo(w)
}

// Finds or creates the orktree for branch and ensures it
// (and any ancestor overlays) are mounted. This is the method that the
// switch and path commands use.
func (i *Index) EnsureOrktree(branch string, opts CreateOrktreeOptions) (OrktreeMetadata, error) {
	w, err := state.FindOrktree(i.state, branch)
	if err != nil {
		return i.CreateOrktree(branch, opts)
	}
	if opts.From != "" || opts.NoGit || opts.Name != "" {
		return OrktreeMetadata{}, fmt.Errorf("orktree %q already exists; --from, --no-git, and --name are only used during creation", branch)
	}
	if err := i.ensureMountedWithAncestors(w, make(map[string]bool)); err != nil {
		return OrktreeMetadata{}, err
	}
	return i.createOrktreeInfo(w)
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
func (i *Index) ListOrktreesUpperDirFiles(branch string, limit int) (files []string, count int, err error) {
	w, err := state.FindOrktree(i.state, branch)
	if err != nil {
		return nil, 0, err
	}
	upper, _, _ := i.state.OverlayDirs(w)

	return overlay.DirtyUpperFiles(upper, i.state.MountPath(w), limit)
}

// ---------------------------------------------------------------------------
// Rename
// ---------------------------------------------------------------------------

// RenameOrktree changes the human-visible label (name) of the orktree identified
// by ref (branch, name, ID or prefix). The new name is validated, checked for
// conflicts, and the merged directory on disk is moved to its new location.
// The branch is left untouched.
func (i *Index) RenameOrktree(ref, newName string) error {
	if err := validateName(newName); err != nil {
		return err
	}
	if newName == "" {
		return fmt.Errorf("new name must not be empty")
	}

	w, err := state.FindOrktree(i.state, ref)
	if err != nil {
		return err
	}

	// Reject if another orktree already carries newName (exact match).
	for _, other := range i.state.Orktrees {
		if other.ID == w.ID {
			continue
		}
		if other.EffectiveName() == newName {
			return fmt.Errorf("an orktree named %q already exists", newName)
		}
	}

	oldMerged := func() string { _, _, m := i.state.OverlayDirs(w); return m }()

	// Build a temporary Orktree with the new name to derive the new merged path
	// without modifying state yet — so failures leave state intact.
	newW := w
	newW.Name = newName
	newMerged := func() string { _, _, m := i.state.OverlayDirs(newW); return m }()

	if err := os.MkdirAll(filepath.Dir(newMerged), 0o755); err != nil {
		return fmt.Errorf("creating parent directory for renamed orktree: %w", err)
	}
	if err := os.Rename(oldMerged, newMerged); err != nil {
		return fmt.Errorf("moving orktree directory: %w", err)
	}

	if err := state.RenameOrktree(i.state, w.ID, newName); err != nil {
		// Best-effort rollback: try to move the directory back.
		os.Rename(newMerged, oldMerged) //nolint:errcheck
		return fmt.Errorf("updating state: %w", err)
	}

	// Clean up empty ancestor directories left by the old merged path.
	cleanEmptyAncestors(filepath.Dir(oldMerged), state.SiblingDir(i.state.SourceRoot))

	return nil
}

// ---------------------------------------------------------------------------
// Remove
// ---------------------------------------------------------------------------

type RemoveCheck struct {
	Branch          string
	MergedPath      string
	Dependents      []string // branch names of stacked orktrees
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

// Read-only, does not modify
func (i *Index) CheckRemoveOrktree(branch string) (*RemoveCheck, error) {
	w, err := state.FindOrktree(i.state, branch)
	if err != nil {
		return nil, err
	}

	upper, _, merged := i.state.OverlayDirs(w)
	rc := &RemoveCheck{
		Branch:     w.Branch,
		MergedPath: merged,
	}

	// Dependents.
	for _, d := range state.Dependents(i.state, w.Branch) {
		rc.Dependents = append(rc.Dependents, d.Branch)
	}

	// Dirty files in the overlay upper dir.
	dirtyFiles, _, err := overlay.DirtyUpperFiles(upper, i.state.MountPath(w), 0)
	if err != nil {
		return nil, fmt.Errorf("assessing overlay changes: %w", err)
	}

	if w.GitTreePath != "" {
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

		lowerDir := i.state.MountPath(w)
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
	} else { // NoGit mode: treat all dirty files as untracked.
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

	upper, work, merged := i.state.OverlayDirs(w)

	if err := overlay.Remove(upper, work, merged); err != nil {
		return fmt.Errorf("removing overlay: %w", err)
	}

	cleanEmptyAncestors(merged, state.SiblingDir(i.state.SourceRoot))

	if w.GitTreePath != "" {
		if err := git.RemoveWorktree(i.state.SourceRoot, w.GitTreePath); err != nil {
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
	_, _, merged := i.state.OverlayDirs(w)
	return merged, nil
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

func validateBranchName(name string) error {
	if name == "" {
		return fmt.Errorf("branch name must not be empty")
	}
	if strings.ContainsRune(name, 0) {
		return fmt.Errorf("branch name contains invalid characters")
	}
	cleaned := filepath.Clean(name)
	if filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, "..") {
		return fmt.Errorf("branch name %q would escape the orktree directory", name)
	}
	return nil
}

// validateName checks that a custom orktree name is safe to use as a
// filesystem path component. The rules mirror validateBranchName — empty is
// allowed here because callers only invoke validateName when a non-empty name
// has been explicitly supplied.
func validateName(name string) error {
	if strings.ContainsRune(name, 0) {
		return fmt.Errorf("orktree name contains invalid characters")
	}
	cleaned := filepath.Clean(name)
	if filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, "..") {
		return fmt.Errorf("orktree name %q would escape the orktree directory", name)
	}
	return nil
}

func (i *Index) ensureMountedWithAncestors(w state.Orktree, visited map[string]bool) error {
	if visited[w.ID] {
		return fmt.Errorf("cycle detected in orktree parent chain at %q", w.Branch)
	}
	visited[w.ID] = true

	if w.LowerOrktreeBranch != "" {
		parent, err := state.FindOrktree(i.state, w.LowerOrktreeBranch)
		if err != nil {
			return err
		}
		if err := i.ensureMountedWithAncestors(parent, visited); err != nil {
			return err
		}
	}

	upper, work, merged := i.state.OverlayDirs(w)
	return overlay.EnsureMounted(i.state.MountPath(w), upper, work, merged)
}

// Returns the overlayfs lowerdir. Populates the git-related fields of *w.
func (i *Index) setupGitForOrktree(w *state.Orktree, branch, from, upper string) (string, error) {
	treeDir := i.state.GitTreeDir(*w)

	// --from refers to an existing orktree
	if from != "" {
		fromOrk, err := state.FindOrktree(i.state, from)
		if err == nil {
			_, _, fromMerged := i.state.OverlayDirs(fromOrk)
			mounted, _ := overlay.IsMounted(fromMerged)
			if !mounted {
				return "", fmt.Errorf("orktree %q is not mounted; mount it first", from)
			}

			exists, err := git.BranchExists(i.state.SourceRoot, branch)
			if err != nil {
				return "", err
			}
			if !exists {
				if err := git.CreateBranch(i.state.SourceRoot, branch, fromOrk.Branch); err != nil {
					return "", fmt.Errorf("creating branch: %w", err)
				}
			}
			if err := git.AddWorktreeNoCheckout(i.state.SourceRoot, treeDir, branch); err != nil {
				return "", fmt.Errorf("registering git worktree: %w", err)
			}
			if err := seedGitFile(treeDir, upper); err != nil {
				return "", err
			}
			if err := seedSubmoduleGitFiles(fromMerged, upper); err != nil {
				return "", err
			}
			w.GitTreePath = treeDir
			w.LowerDir = fromMerged
			w.LowerOrktreeBranch = fromOrk.Branch
			return fromMerged, nil
		}
	}

	// --from matches source root
	currentBranch, _ := git.CurrentBranch(i.state.SourceRoot)
	if from == "" || from == currentBranch {
		exists, err := git.BranchExists(i.state.SourceRoot, branch)
		if err != nil {
			return "", err
		}
		if !exists {
			if err := git.CreateBranch(i.state.SourceRoot, branch, from); err != nil {
				return "", fmt.Errorf("creating branch: %w", err)
			}
		}
		if err := git.AddWorktreeNoCheckout(i.state.SourceRoot, treeDir, branch); err != nil {
			return "", fmt.Errorf("registering git worktree: %w", err)
		}
		if err := seedGitFile(treeDir, upper); err != nil {
			return "", err
		}
		if err := seedSubmoduleGitFiles(i.state.SourceRoot, upper); err != nil {
			return "", err
		}
		w.GitTreePath = treeDir
		w.LowerDir = i.state.SourceRoot
		return i.state.SourceRoot, nil
	}

	// fallback: --from <git-ref> could not be matched to an existing fs
	exists, err := git.BranchExists(i.state.SourceRoot, branch)
	if err != nil {
		return "", err
	}
	newBranch := !exists
	if err := git.AddWorktree(i.state.SourceRoot, treeDir, branch, newBranch, from); err != nil {
		return "", fmt.Errorf("creating git worktree: %w", err)
	}
	w.GitTreePath = treeDir
	w.LowerDir = treeDir
	return treeDir, nil
}

// Copies the .git gitfile from the no-checkout worktree directory
// into upper. This ensures git commands inside the merged overlay path track the
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

// Removes empty directories from path up to (but not including) stopAt.
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

func canAccessFuseDev() bool {
	f, err := os.Open("/dev/fuse")
	if err != nil {
		return false
	}
	f.Close()
	return true
}

func (i *Index) createOrktreeInfo(w state.Orktree) (OrktreeMetadata, error) {
	upper, _, merged := i.state.OverlayDirs(w)
	mounted, err := overlay.IsMounted(merged)
	if err != nil {
		return OrktreeMetadata{}, fmt.Errorf("checking mount status: %w", err)
	}
	return OrktreeMetadata{
		ID:                 w.ID,
		Name:               w.EffectiveName(),
		Branch:             w.Branch,
		MergedPath:         merged,
		Mounted:            mounted,
		LowerOrktreeBranch: w.LowerOrktreeBranch,
		CreatedAt:          w.CreatedAt,
		UpperDirSize:       calculateDirectoryTotalBytes(upper),
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
