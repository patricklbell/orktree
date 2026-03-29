// orktree – git worktree + fuse-overlayfs manager (CLI layer).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/patricklbell/orktree"
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

// discoverFromCwd locates the orktree Manager by walking up from cwd.
func discoverFromCwd() (*orktree.Manager, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return orktree.Discover(cwd)
}

// ---------------------------------------------------------------------------
// check
// ---------------------------------------------------------------------------

func cmdCheck(_ []string) error {
	fmt.Println("orktree prerequisites")
	fmt.Println()

	ok := true
	for _, p := range orktree.CheckPrerequisites() {
		if p.OK {
			fmt.Printf("  \xe2\x9c\x93  %-22s\n", p.Name)
		} else {
			fmt.Printf("  \xe2\x9c\x97  %-22s  ->  %s\n", p.Name, p.Fix)
			ok = false
		}
	}

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

	mgr, err := orktree.Init(source)
	if err != nil {
		return err
	}

	srcRoot := mgr.SourceRoot()
	sibDir := filepath.Join(filepath.Dir(srcRoot), filepath.Base(srcRoot)+".orktree")
	fmt.Printf("Initialized orktree at %s\n", sibDir)
	fmt.Printf("  source   : %s\n", srcRoot)
	if mgr.IsGitRepo() {
		fmt.Printf("  git repo : yes (orktrees will be git-backed)\n")
	}
	fmt.Println()
	fmt.Println("Run 'orktree switch <branch>' to create your first orktree and enter it.")
	return nil
}

// ---------------------------------------------------------------------------
// new  (deprecated — use 'orktree switch' instead)
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

	mgr, err := discoverFromCwd()
	if err != nil {
		return err
	}

	info, err := mgr.Create(branch, orktree.CreateOpts{From: from, NoGit: noGit})
	if err != nil {
		return fmt.Errorf("%w\n(hint: run 'orktree check' to check prerequisites)", err)
	}

	fmt.Fprintf(os.Stderr, "Created orktree %s (branch: %s)\n", info.ID, info.Branch)
	fmt.Fprintf(os.Stderr, "  path      : %s\n", info.MergedPath)
	if info.LowerOrktreeBranch != "" {
		fmt.Fprintf(os.Stderr, "  based on  : %s (zero-cost stacking)\n", info.LowerOrktreeBranch)
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "Switch : orktree switch %s\n", info.Branch)
	fmt.Fprintf(os.Stderr, "Remove : orktree rm     %s\n", info.Branch)
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

	mgr, err := discoverFromCwd()
	if err != nil {
		return err
	}

	infos, err := mgr.List()
	if err != nil {
		return err
	}

	if len(infos) == 0 {
		if !quiet {
			fmt.Println("No orktrees yet. Run 'orktree switch <branch>' to create one.")
		}
		return nil
	}

	if quiet {
		for _, info := range infos {
			fmt.Println(info.Branch)
		}
		return nil
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "BRANCH\tSTATUS\tPATH")
	for _, info := range infos {
		status := "unmounted"
		if info.Mounted {
			status = "mounted"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n",
			info.Branch,
			status,
			info.MergedPath,
		)
	}
	tw.Flush()
	return nil
}

// ---------------------------------------------------------------------------
// switch  (orktree switch <branch> [--from <base>] [--no-git])
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
		mgr, err := discoverFromCwd()
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Switched to source root\n")
		fmt.Fprintf(os.Stderr, "  path      : %s\n", mgr.SourceRoot())
		fmt.Println(mgr.SourceRoot())
		return nil
	}

	mgr, err := discoverFromCwd()
	if err != nil {
		return err
	}

	_, findErr := mgr.Find(branch)
	if findErr != nil {
		fmt.Fprintf(os.Stderr, "No orktree for '%s' \xe2\x80\x94 creating it now...\n", branch)
	}

	info, err := mgr.EnsureReady(branch, orktree.CreateOpts{From: from, NoGit: noGit})
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Switched to orktree %q\n", info.Branch)
	fmt.Fprintf(os.Stderr, "  path      : %s\n", info.MergedPath)
	return nil
}

// ---------------------------------------------------------------------------
// path  (orktree path <branch> [--from <base>] [--no-git])
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

	mgr, err := discoverFromCwd()
	if err != nil {
		return err
	}

	// "-" means source root — used by the shell wrapper for "orktree switch -"
	if branch == "-" {
		fmt.Println(mgr.SourceRoot())
		return nil
	}

	_, findErr := mgr.Find(branch)
	if findErr != nil {
		fmt.Fprintf(os.Stderr, "No orktree for '%s' \xe2\x80\x94 creating it now...\n", branch)
	}

	info, err := mgr.EnsureReady(branch, orktree.CreateOpts{From: from, NoGit: noGit})
	if err != nil {
		return err
	}

	fmt.Println(info.MergedPath)
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

	fmt.Fprintf(os.Stderr, "\xe2\x9c\x93  Installed %s completion to %s\n", shell, installPath)
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
// rm
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

	mgr, err := discoverFromCwd()
	if err != nil {
		return err
	}

	info, err := mgr.Find(ref)
	if err != nil {
		return err
	}

	if err := mgr.Remove(ref, force); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Removed orktree '%s'\n", info.Branch)
	return nil
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
