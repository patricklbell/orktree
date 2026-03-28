// orktree – git worktree + fuse-overlayfs manager.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
		fmt.Fprintln(os.Stderr, "warning: 'orktree new' is deprecated; use 'orktree switch' instead")
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
	case "completion":
		return cmdCompletion(args[1:])
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
		fmt.Println("Next: cd into your repo and run 'orktree init'.")
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
		case "--help", "-h":
			printUsage()
			return nil
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
	fmt.Printf("Initialized orktree at %s\n", state.SiblingDir(cfg.SourceRoot))
	fmt.Printf("  source   : %s\n", cfg.SourceRoot)
	if isGit {
		fmt.Printf("  git repo : yes (orktrees will be git-backed)\n")
	}
	fmt.Println()
	fmt.Println("Run 'orktree switch <branch>' to create your first orktree and enter it.")
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
	if args[0] == "--help" || args[0] == "-h" {
		printUsage()
		return nil
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
		case "--help", "-h":
			printUsage()
			return nil
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

	fmt.Fprintf(os.Stderr, "Created orktree %s (branch: %s)\n", w.ID, w.Branch)
	fmt.Fprintf(os.Stderr, "  path      : %s\n", merged)
	if w.LowerOrktreeBranch != "" {
		fmt.Fprintf(os.Stderr, "  based on  : %s (zero-cost stacking)\n", w.LowerOrktreeBranch)
	} else if w.GitTreePath != "" && w.LowerDir == cfg.SourceRoot {
		fmt.Fprintf(os.Stderr, "  based on  : source root (zero-cost)\n")
	} else if w.GitTreePath != "" {
		fmt.Fprintf(os.Stderr, "  git tree  : %s\n", w.GitTreePath)
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "Switch : orktree switch %s\n", w.Branch)
	fmt.Fprintf(os.Stderr, "Remove : orktree rm     %s\n", w.Branch)
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

func cmdLs(args []string) error {
	quiet := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--quiet", "-q":
			quiet = true
		case "--help", "-h":
			printUsage()
			return nil
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	cfg, err := loadFromCwd()
	if err != nil {
		return err
	}

	if len(cfg.Orktrees) == 0 {
		if !quiet {
			fmt.Println("No orktrees yet. Run 'orktree switch <branch>' to create one.")
		}
		return nil
	}

	if quiet {
		for _, w := range cfg.Orktrees {
			fmt.Println(w.Branch)
		}
		return nil
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "BRANCH\tSTATUS\tPATH")
	for _, w := range cfg.Orktrees {
		_, _, merged := cfg.OverlayDirs(w)
		mounted, _ := overlay.IsMounted(merged)

		status := "unmounted"
		if mounted {
			status = "mounted"
		}

		fmt.Fprintf(tw, "%s\t%s\t%s\n",
			w.Branch,
			status,
			merged,
		)
	}
	tw.Flush()
	return nil
}

// ---------------------------------------------------------------------------
// switch  (orktree switch <branch> [--from <base>] [--no-git])
//
// Ensures the orktree is mounted, auto-creating it if absent.
// Use "-" to return to the source root.
// ---------------------------------------------------------------------------

func cmdSwitch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: orktree switch <branch> [--from <base>] [--no-git]")
	}
	if args[0] == "--help" || args[0] == "-h" {
		printUsage()
		return nil
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
		case "--help", "-h":
			printUsage()
			return nil
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	// "-" means return to the source root
	if branch == "-" {
		if from != "" || noGit {
			return fmt.Errorf("'orktree switch -' returns to the source root and takes no flags")
		}
		cfg, err := loadFromCwd()
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Switched to source root\n")
		fmt.Fprintf(os.Stderr, "  path      : %s\n", cfg.SourceRoot)
		fmt.Println(cfg.SourceRoot)
		return nil
	}

	cfg, err := loadFromCwd()
	if err != nil {
		return err
	}

	w, err := state.FindOrktree(cfg, branch)
	if err != nil {
		fmt.Fprintf(os.Stderr, "No orktree for '%s' — creating it now...\n", branch)
		// Build args for cmdNew: branch [--from <base>] [--no-git]
		newArgs := []string{branch}
		if from != "" {
			newArgs = append(newArgs, "--from", from)
		}
		if noGit {
			newArgs = append(newArgs, "--no-git")
		}
		if err := cmdNew(newArgs); err != nil {
			return err
		}
		// Reload state after creation
		cfg, err = state.Load(cfg.SourceRoot)
		if err != nil {
			return err
		}
		w, err = state.FindOrktree(cfg, branch)
		if err != nil {
			return err
		}
	} else if from != "" || noGit {
		return fmt.Errorf("orktree %q already exists; --from and --no-git are only used during creation", branch)
	}

	if err := ensureMountedWithAncestors(cfg, w, make(map[string]bool)); err != nil {
		return err
	}

	_, _, merged := cfg.OverlayDirs(w)
	fmt.Fprintf(os.Stderr, "Switched to orktree %q\n", w.Branch)
	fmt.Fprintf(os.Stderr, "  path      : %s\n", merged)
	return nil
}

// ---------------------------------------------------------------------------
// path  (orktree path <branch>)
// ---------------------------------------------------------------------------

func cmdPath(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: orktree path <branch> [--from <base>] [--no-git]")
	}
	if args[0] == "--help" || args[0] == "-h" {
		printUsage()
		return nil
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
		case "--help", "-h":
			printUsage()
			return nil
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	cfg, err := loadFromCwd()
	if err != nil {
		return err
	}

	// "-" means source root — used by the shell wrapper for "orktree switch -"
	if branch == "-" {
		fmt.Println(cfg.SourceRoot)
		return nil
	}

	w, err := state.FindOrktree(cfg, branch)
	if err != nil {
		fmt.Fprintf(os.Stderr, "No orktree for '%s' — creating it now...\n", branch)
		newArgs := []string{branch}
		if from != "" {
			newArgs = append(newArgs, "--from", from)
		}
		if noGit {
			newArgs = append(newArgs, "--no-git")
		}
		if err := cmdNew(newArgs); err != nil {
			return err
		}
		cfg, err = state.Load(cfg.SourceRoot)
		if err != nil {
			return err
		}
		w, err = state.FindOrktree(cfg, branch)
		if err != nil {
			return err
		}
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

const shellWrapper = `orktree() {
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
		fmt.Print(shellWrapper)
		return nil
	case "fish":
		return fmt.Errorf("fish shell support is not yet implemented")
	default:
		return fmt.Errorf("unsupported shell %q; supported: bash, zsh", shell)
	}
}

// ---------------------------------------------------------------------------
// completion
// ---------------------------------------------------------------------------

const bashCompletionScript = `# Generated by orktree completion bash — do not edit.
# Regenerate: orktree completion bash > ~/.local/share/bash-completion/completions/orktree
orktree() {
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

_orktree_completion() {
  local cur prev words cword
  _init_completion 2>/dev/null || {
    COMPREPLY=(); return
  }

  if [[ $cword -eq 1 ]]; then
    COMPREPLY=($(compgen -W "check init switch sw ls list path p rm remove shell-init completion help" -- "$cur"))
    return
  fi

  case "${words[1]}" in
    switch|sw|path|p|new|n|rm|remove)
      if [[ "$cur" != -* ]]; then
        local branches
        branches=$(command orktree ls --quiet 2>/dev/null)
        COMPREPLY=($(compgen -W "$branches" -- "$cur"))
        return
      fi
      case "${words[1]}" in
        rm|remove) COMPREPLY=($(compgen -W "--force" -- "$cur")) ;;
        *)         COMPREPLY=($(compgen -W "--from --no-git" -- "$cur")) ;;
      esac
      ;;
    init)
      case "$prev" in
        --source|-s) _filedir -d; return ;;
      esac
      COMPREPLY=($(compgen -W "--source" -- "$cur"))
      ;;
    shell-init)
      case "$prev" in
        --shell) COMPREPLY=($(compgen -W "bash zsh" -- "$cur")); return ;;
      esac
      COMPREPLY=($(compgen -W "--shell" -- "$cur"))
      ;;
    completion)
      COMPREPLY=($(compgen -W "bash zsh install" -- "$cur"))
      ;;
  esac
}

complete -F _orktree_completion orktree
`

const zshCompletionScript = `#compdef orktree

# Generated by orktree completion zsh — do not edit.
# Regenerate: orktree completion zsh > ~/.local/share/zsh/site-functions/_orktree

# Install the shell wrapper if not already defined (e.g. via shell-init).
if [[ -z "$functions[orktree]" ]]; then
orktree() {
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
fi

_orktree_branches() {
  local -a branches
  branches=(${(f)"$(command orktree ls --quiet 2>/dev/null)"})
  _describe 'branch' branches
}

_orktree() {
  local -a commands
  commands=(
    'check:Check prerequisites'
    'init:Initialise orktree in a directory'
    'switch:Enter orktree (auto-creates if absent)'
    'ls:List orktrees'
    'path:Print workspace path'
    'rm:Remove orktree'
    'shell-init:Emit shell integration snippet'
    'completion:Emit shell completion script'
    'help:Show help'
  )

  _arguments -C \
    '1: :->command' \
    '*: :->args'

  case $state in
    command) _describe 'command' commands ;;
    args)
      case ${words[2]} in
        switch|sw|path|p|rm|remove|new|n)
          case ${words[2]} in
            rm|remove) _arguments '--force[force removal]' '1:branch:_orktree_branches' ;;
            *)         _arguments '--from[base branch]:branch:_orktree_branches' '--no-git[skip git registration]' '1:branch:_orktree_branches' ;;
          esac
          ;;
        init)
          _arguments '--source[source directory]:dir:_files -/' ;;
        shell-init)
          _arguments '--shell[shell type]:shell:(bash zsh)' ;;
        completion)
          case ${words[3]} in
            install) _arguments '--shell[shell type]:shell:(bash zsh)' ;;
            *) _arguments '1:subcommand:(bash zsh install)' ;;
          esac
          ;;
      esac
      ;;
  esac
}

_orktree
`

func cmdCompletion(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: orktree completion <bash|zsh|install>")
	}
	switch args[0] {
	case "bash":
		fmt.Print(bashCompletionScript)
		return nil
	case "zsh":
		fmt.Print(zshCompletionScript)
		return nil
	case "install":
		return cmdCompletionInstall(args[1:])
	default:
		return fmt.Errorf("unknown completion target %q; use bash, zsh, or install", args[0])
	}
}

func cmdCompletionInstall(args []string) error {
	shell := os.Getenv("SHELL")
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

	if shell == "" {
		return fmt.Errorf("$SHELL is unset; use --shell bash or --shell zsh")
	}
	// Normalise: take only the base name of $SHELL.
	shell = filepath.Base(shell)

	xdgData := os.Getenv("XDG_DATA_HOME")
	if xdgData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("determining home directory: %w", err)
		}
		xdgData = filepath.Join(home, ".local", "share")
	}

	var installPath string
	var script string
	switch shell {
	case "bash":
		installPath = filepath.Join(xdgData, "bash-completion", "completions", "orktree")
		script = bashCompletionScript
	case "zsh":
		installPath = filepath.Join(xdgData, "zsh", "site-functions", "_orktree")
		script = zshCompletionScript
	default:
		return fmt.Errorf("unsupported shell %q; supported: bash, zsh", shell)
	}

	if err := os.MkdirAll(filepath.Dir(installPath), 0o755); err != nil {
		return fmt.Errorf("creating completion directory: %w", err)
	}
	// Atomic write: temp file then rename.
	tmp, err := os.CreateTemp(filepath.Dir(installPath), "orktree-completion-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(script); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("writing completion script: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Rename(tmpName, installPath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("installing completion script: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✓  Installed %s completion to %s\n", shell, installPath)
	fmt.Fprintln(os.Stderr, "   Active after opening a new terminal, or run:")
	fmt.Fprintf(os.Stderr, "     source %s\n", installPath)
	if shell == "bash" {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "   NOTE: The cd-on-switch feature requires one source per shell session.")
		fmt.Fprintln(os.Stderr, "   For reliable cd without pressing TAB first, also add to ~/.bashrc:")
		fmt.Fprintln(os.Stderr, `     eval "$(orktree shell-init)"`)
	} else if shell == "zsh" {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "   Ensure this directory is in fpath before compinit in ~/.zshrc:")
		fmt.Fprintf(os.Stderr, "     fpath=(%s $fpath)\n", filepath.Dir(installPath))
	}
	return nil
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
	if ref == "--help" || ref == "-h" {
		printUsage()
		return nil
	}
	force := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--force", "-f":
			force = true
		case "--help", "-h":
			printUsage()
			return nil
		default:
			return fmt.Errorf("unknown flag %q", args[i])
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

	// Clean up any empty directories left between merged and the sibling root
	// (relevant when the branch name contained "/" and produced nested dirs).
	cleanEmptyAncestors(merged, state.SiblingDir(cfg.SourceRoot))

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

	fmt.Fprintf(os.Stderr, "Removed orktree '%s'\n", w.Branch)
	return nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// loadFromCwd finds the source root by walking up from the current directory.
// Two cases are handled:
//   - cwd is inside a source root: look for a sibling <dir>.orktree/state.json
//   - cwd is inside a merged view: the ancestor ending in ".orktree" reveals the source root
func loadFromCwd() (*state.Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	dir := cwd
	for {
		// Case 1: dir is the source root — check for sibling .orktree/state.json
		sib := filepath.Join(filepath.Dir(dir), filepath.Base(dir)+".orktree")
		if _, err := os.Stat(filepath.Join(sib, state.StateFile)); err == nil {
			return state.Load(dir)
		}
		// Case 2: dir is inside a merged view — check if dir ends in ".orktree"
		// e.g. /projects/myrepo.orktree/feature/my-branch → source is /projects/myrepo
		base := filepath.Base(dir)
		if strings.HasSuffix(base, ".orktree") {
			if _, err := os.Stat(filepath.Join(dir, state.StateFile)); err == nil {
				srcRoot := filepath.Join(filepath.Dir(dir), strings.TrimSuffix(base, ".orktree"))
				return state.Load(srcRoot)
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil, fmt.Errorf("no .orktree state found in %s or any parent directory (did you run 'orktree init'?)", cwd)
}

// cleanEmptyAncestors removes empty directories from path up to (but not including) stopAt.
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

// ---------------------------------------------------------------------------
// usage
// ---------------------------------------------------------------------------

func printUsage() {
	fmt.Print(`orktree — isolated workspaces for parallel git branches
Context-switch instantly without stashing, committing, or copying files.

SHELL INTEGRATION (required for cd-on-switch):
  eval "$(orktree shell-init)"   # add to ~/.bashrc or ~/.zshrc
  orktree completion install     # or: install completion script (no bashrc edit needed)

Usage:
  orktree <command> [args...]

Commands:
  check                                       Check prerequisites
  init   [--source <path>]                    Initialise orktree in a directory
  switch <branch> [--from <b>] [--no-git]     Enter orktree (auto-creates if absent)
  switch -                                    Return to the source root
  ls     [--quiet]                            List orktrees
  path   <branch> [--from <b>] [--no-git]     Print workspace path (auto-creates if absent)
  rm     <branch> [--force]                   Remove orktree
  shell-init [--shell bash|zsh]               Emit shell integration snippet
  completion <bash|zsh|install>               Emit or install tab-completion script

Flags:
  --from <branch|ref>    Base orktree on this branch or git ref
  --no-git               Skip git worktree registration (overlay-only)
  --force                Force removal even if unmount fails

Aliases:
  sw → switch,  p → path,  remove → rm,  list → ls,  n → new (deprecated)
`)
}
