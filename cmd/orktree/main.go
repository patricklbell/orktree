// orktree – git worktree + fuse-overlayfs manager.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/tabwriter"

	igit "github.com/patricklbell/orktree/internal/git"
	"github.com/patricklbell/orktree/internal/overlay"
	"github.com/patricklbell/orktree/internal/state"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "orktree: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	switch args[0] {
	case "init":
		return cmdInit(args[1:])
	case "check":
		return cmdCheck(args[1:])
	case "new", "n":
		return cmdNew(args[1:])
	case "ls", "list":
		return cmdLs(args[1:])
	case "switch", "sw":
		return cmdSwitch(args[1:])
	case "rm", "remove":
		return cmdRm(args[1:])
	case "path", "p":
		return cmdPath(args[1:])
	case "shell-init":
		return cmdShellInit(args[1:])
	case "help", "--help", "-h":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q \xe2\x80\x94 run 'orktree help'", args[0])
	}
}

// ---------------------------------------------------------------------------
// check
// ---------------------------------------------------------------------------

// cmdCheck checks prerequisites and prints the exact one-time fix command for
// anything that is missing.  Everything here only needs to be done once;
// normal orktree commands need no sudo.
func cmdCheck(_ []string) error {
	ok := true

	check := func(label, fix string, pass bool) {
		if pass {
			fmt.Printf("  \xe2\x9c\x93  %-22s\n", label)
		} else {
			fmt.Printf("  \xe2\x9c\x97  %-22s  ->  %s\n", label, fix)
			ok = false
		}
	}

	fmt.Println("orktree prerequisites")
	fmt.Println()

	// fuse-overlayfs binary (rootless CoW overlay; no file duplication)
	_, fuseOfsErr := exec.LookPath("fuse-overlayfs")
	check("fuse-overlayfs",
		"sudo apt-get install fuse-overlayfs   # or: dnf / pacman / brew equivalent",
		fuseOfsErr == nil)

	// /dev/fuse access (needed by fuse-overlayfs; granted via the fuse group)
	check("fuse group (/dev/fuse)",
		"sudo usermod -aG fuse $USER",
		canAccessFuseDev())

	// git binary
	_, gitErr := exec.LookPath("git")
	check("git",
		"install git: https://git-scm.com/downloads",
		gitErr == nil)

	fmt.Println()
	if ok {
		fmt.Println("All prerequisites satisfied.")
	} else {
		fmt.Println("Run the fix commands above (log out and back in after any usermod), then re-run 'orktree check'.")
		return fmt.Errorf("prerequisites not met")
	}
	return nil
}

// canAccessFuseDev reports whether /dev/fuse is accessible to the current
// process (open for read is sufficient to confirm group/world access).
func canAccessFuseDev() bool {
	f, err := os.Open("/dev/fuse")
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// ---------------------------------------------------------------------------
// init
// ---------------------------------------------------------------------------

func cmdInit(args []string) error {
	source := "."

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--source", "-s":
			if i+1 >= len(args) {
				return fmt.Errorf("--source requires a value")
			}
			i++
			source = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	abs, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	isGit := igit.IsGitRepo(abs)

	cfg, err := state.Init(source, isGit)
	if err != nil {
		return err
	}
	fmt.Printf("Initialized orktree repo %s\n", cfg.ID)
	fmt.Printf("  source   : %s\n", cfg.SourceRoot)
	fmt.Printf("  data     : %s\n", cfg.DataDir)
	if isGit {
		fmt.Printf("  git repo : yes (orktrees will be git-backed)\n")
	}
	fmt.Println()
	fmt.Println("Create an orktree with: orktree new <branch>")
	return nil
}

// ---------------------------------------------------------------------------
// new  (orktree new <branch> [--from <base>] [--no-git])
//
// Zero-cost paths (no git checkout):
//   - No --from, or --from equals the source root's current branch:
//     uses the source root as overlayfs lowerdir.
//   - --from <existing-orktree>: uses that orktree's merged path as lowerdir
//     (stacks a new CoW layer on top of the existing overlay).
//
// Conventional path (full git checkout):
//   - --from <git-ref> where <git-ref> is not an existing orktree and does
//     not match the source root's current branch.
// ---------------------------------------------------------------------------

func cmdNew(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: orktree new <branch> [--from <base>]")
	}
	branch := args[0]
	from := ""
	noGit := false

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--from", "-f":
			if i+1 >= len(args) {
				return fmt.Errorf("--from requires a value")
			}
			i++
			from = args[i]
		case "--no-git":
			noGit = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	cfg, err := loadFromCwd()
	if err != nil {
		return err
	}

	w, err := state.NewOrktree(cfg, branch)
	if err != nil {
		return err
	}

	upper, work, merged := cfg.OverlayDirs(w)

	var lowerDir string
	if cfg.IsGitRepo && !noGit {
		lowerDir, err = setupGitForOrktree(cfg, &w, branch, from, upper)
		if err != nil {
			state.RemoveOrktree(cfg, w.ID) //nolint:errcheck
			return err
		}
		if err := state.UpdateOrktree(cfg, w); err != nil {
			return err
		}
	} else {
		lowerDir = cfg.SourceRoot
	}

	if err := overlay.Create(upper, work, merged); err != nil {
		return err
	}
	if err := overlay.Mount(lowerDir, upper, work, merged); err != nil {
		return fmt.Errorf("%w\n(hint: run 'orktree check' to check prerequisites)", err)
	}

	fmt.Printf("Created orktree %s (branch: %s)\n", w.ID, w.Branch)
	fmt.Printf("  path      : %s\n", merged)
	if w.LowerOrktreeBranch != "" {
		fmt.Printf("  based on  : %s (zero-cost stacking)\n", w.LowerOrktreeBranch)
	} else if w.GitTreePath != "" && w.LowerDir == cfg.SourceRoot {
		fmt.Printf("  based on  : source root (zero-cost)\n")
	} else if w.GitTreePath != "" {
		fmt.Printf("  git tree  : %s\n", w.GitTreePath)
	}
	fmt.Println()
	fmt.Printf("Switch : orktree switch %s\n", w.Branch)
	fmt.Printf("Remove : orktree rm     %s\n", w.Branch)
	return nil
}

// setupGitForOrktree decides which git setup path to take and returns the
// overlayfs lowerdir.  It also populates the git-related fields of *w.
//
// Zero-cost paths skip a full git checkout by using an existing filesystem
// as lowerdir and seeding only a .git gitfile into the overlay upper dir.
func setupGitForOrktree(cfg *state.Config, w *state.Orktree, branch, from, upper string) (string, error) {
	treeDir := cfg.GitTreeDir(*w)
	// ---- Zero-cost path A: --from refers to an existing orktree ---------
	if from != "" {
		fromOrk, err := state.FindOrktree(cfg, from)
		if err == nil {
			_, _, fromMerged := cfg.OverlayDirs(fromOrk)
			mounted, _ := overlay.IsMounted(fromMerged)
			if !mounted {
				return "", fmt.Errorf("orktree %q is not mounted; run 'orktree switch %s' first", from, fromOrk.Branch)
			}

			// Create the new branch pointing at the from-orktree's branch.
			exists, err := igit.BranchExists(cfg.SourceRoot, branch)
			if err != nil {
				return "", err
			}
			if !exists {
				if err := igit.CreateBranch(cfg.SourceRoot, branch, fromOrk.Branch); err != nil {
					return "", fmt.Errorf("creating branch: %w", err)
				}
			}
			if err := igit.AddWorktreeNoCheckout(cfg.SourceRoot, treeDir, branch); err != nil {
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
	// The source root is already a checked-out tree; use it as lowerdir.
	currentBranch, _ := igit.CurrentBranch(cfg.SourceRoot)
	if from == "" || from == currentBranch {
		exists, err := igit.BranchExists(cfg.SourceRoot, branch)
		if err != nil {
			return "", err
		}
		if !exists {
			if err := igit.CreateBranch(cfg.SourceRoot, branch, from); err != nil {
				return "", fmt.Errorf("creating branch: %w", err)
			}
		}
		if err := igit.AddWorktreeNoCheckout(cfg.SourceRoot, treeDir, branch); err != nil {
			return "", fmt.Errorf("registering git worktree: %w", err)
		}
		if err := seedGitFile(treeDir, upper); err != nil {
			return "", err
		}
		w.GitTreePath = treeDir
		w.LowerDir = cfg.SourceRoot
		return cfg.SourceRoot, nil
	}

	// ---- Conventional path: --from <git-ref> not matching any orktree ---
	// Perform a full git worktree checkout so the new branch has its own tree.
	exists, err := igit.BranchExists(cfg.SourceRoot, branch)
	if err != nil {
		return "", err
	}
	newBranch := !exists
	if err := igit.AddWorktree(cfg.SourceRoot, treeDir, branch, newBranch, from); err != nil {
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

// ---------------------------------------------------------------------------
// ls
// ---------------------------------------------------------------------------

func cmdLs(_ []string) error {
	cfg, err := loadFromCwd()
	if err != nil {
		return err
	}

	if len(cfg.Orktrees) == 0 {
		fmt.Println("No orktrees. Create one with: orktree new <branch>")
		return nil
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "BRANCH\tID\tSTATUS\tPATH")
	for _, w := range cfg.Orktrees {
		_, _, merged := cfg.OverlayDirs(w)
		mounted, _ := overlay.IsMounted(merged)

		status := "stopped"
		if mounted {
			status = "mounted"
		}

		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			w.Branch,
			w.ID,
			status,
			merged,
		)
	}
	tw.Flush()
	return nil
}

// ---------------------------------------------------------------------------
// switch  (orktree switch <branch>)
//
// Ensures the orktree is mounted.
// ---------------------------------------------------------------------------

func cmdSwitch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: orktree switch <branch>")
	}
	branch := args[0]
	for i := 1; i < len(args); i++ {
		return fmt.Errorf("unknown flag %q", args[i])
	}

	cfg, err := loadFromCwd()
	if err != nil {
		return err
	}

	w, err := state.FindOrktree(cfg, branch)
	if err != nil {
		fmt.Printf("Orktree for branch %q not found; creating it...\n", branch)
		return cmdNew([]string{branch})
	}

	if err := ensureMountedWithAncestors(cfg, w, make(map[string]bool)); err != nil {
		return err
	}

	_, _, merged := cfg.OverlayDirs(w)
	fmt.Printf("Switched to orktree %q\n", w.Branch)
	fmt.Printf("  path      : %s\n", merged)
	return nil
}

// ---------------------------------------------------------------------------
// path  (orktree path <branch>)
// ---------------------------------------------------------------------------

func cmdPath(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: orktree path <branch>")
	}
	branch := args[0]

	cfg, err := loadFromCwd()
	if err != nil {
		return err
	}

	w, err := state.FindOrktree(cfg, branch)
	if err != nil {
		return err
	}

	if err := ensureMountedWithAncestors(cfg, w, make(map[string]bool)); err != nil {
		return err
	}

	_, _, merged := cfg.OverlayDirs(w)
	fmt.Println(merged)
	return nil
}

// ---------------------------------------------------------------------------
// shell-init
// ---------------------------------------------------------------------------

const bashZshInit = `orktree() {
  case "$1" in
    switch|sw)
      local _orktree_path
      _orktree_path="$(command orktree path "${@:2}")" && cd "$_orktree_path" || return $?
      ;;
    *)
      command orktree "$@"
      ;;
  esac
}
`

func cmdShellInit(args []string) error {
	shell := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--shell":
			if i+1 >= len(args) {
				return fmt.Errorf("--shell requires a value")
			}
			i++
			shell = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	switch shell {
	case "", "bash", "zsh":
		fmt.Print(bashZshInit)
		return nil
	case "fish":
		return fmt.Errorf("fish shell support is not yet implemented")
	default:
		return fmt.Errorf("unsupported shell %q; supported: bash, zsh", shell)
	}
}

// ---------------------------------------------------------------------------
// ensureMountedWithAncestors
// ---------------------------------------------------------------------------

func ensureMountedWithAncestors(cfg *state.Config, w state.Orktree, visited map[string]bool) error {
	if visited[w.ID] {
		return fmt.Errorf("cycle detected in orktree parent chain at %q", w.Branch)
	}
	visited[w.ID] = true

	if w.LowerOrktreeBranch != "" {
		parent, err := state.FindOrktree(cfg, w.LowerOrktreeBranch)
		if err != nil {
			return err
		}
		if err := ensureMountedWithAncestors(cfg, parent, visited); err != nil {
			return err
		}
	}

	upper, work, merged := cfg.OverlayDirs(w)
	return overlay.EnsureMounted(cfg.MountPath(w), upper, work, merged)
}

// ---------------------------------------------------------------------------
// rm  (orktree rm <ref> [--force])
// ---------------------------------------------------------------------------

func cmdRm(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: orktree rm <branch> [--force]")
	}
	ref := args[0]
	force := false
	for i := 1; i < len(args); i++ {
		if args[i] == "--force" || args[i] == "-f" {
			force = true
		}
	}

	cfg, err := loadFromCwd()
	if err != nil {
		return err
	}
	w, err := state.FindOrktree(cfg, ref)
	if err != nil {
		return err
	}

	upper, work, merged := cfg.OverlayDirs(w)

	// Unmount and remove the fuse-overlayfs (also deletes upper/work/merged).
	if err := overlay.Remove(upper, work, merged); err != nil && !force {
		return fmt.Errorf("removing overlay: %w (use --force to ignore)", err)
	}

	// Remove the git worktree registration (deregisters from git, removes tree dir).
	if w.GitTreePath != "" {
		if err := igit.RemoveWorktree(cfg.SourceRoot, w.GitTreePath); err != nil && !force {
			return fmt.Errorf("removing git worktree: %w (use --force to ignore)", err)
		}
		igit.PruneWorktrees(cfg.SourceRoot) //nolint:errcheck
	}

	if err := state.RemoveOrktree(cfg, w.ID); err != nil {
		return err
	}

	fmt.Printf("Removed orktree %s (branch: %s)\n", w.ID, w.Branch)
	return nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// loadFromCwd finds the source root by looking for .orktree/state.json
// starting from the current directory and walking up.
func loadFromCwd() (*state.Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	dir := cwd
	for {
		candidate := state.StatePath(dir)
		if _, err := os.Stat(candidate); err == nil {
			return state.Load(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil, fmt.Errorf("no .orktree/state.json found in %s or any parent directory (did you run 'orktree init'?)", cwd)
}

// ---------------------------------------------------------------------------
// usage
// ---------------------------------------------------------------------------

func printUsage() {
	fmt.Print(`orktree - git worktree + fuse-overlayfs manager

Each orktree is a git branch paired with a fuse-overlayfs CoW mount.
Only files you actually modify consume extra disk space.

Usage:
  orktree <command> [args...]

Commands:
  check                                          Check prerequisites (run once)
  init   [--source <path>]                       Initialize orktree in a directory
  new    <branch> [--from <base>] [--no-git]     Create orktree on <branch>
  ls                                             List orktrees
  switch <branch>                                Mount orktree, creating it if needed
  path   <branch>                                Print workspace path (mounts if needed)
  rm     <branch> [--force]                      Remove orktree
  shell-init [--shell bash|zsh]                  Print shell integration (eval in .bashrc/.zshrc)

Aliases:
  n      alias for new
  sw     alias for switch
  p      alias for path
  remove alias for rm
  list   alias for ls

Shell integration:
  eval "$(orktree shell-init)"    # add to ~/.bashrc or ~/.zshrc
  # then: orktree switch <branch>  will cd to the workspace automatically
`)
}
