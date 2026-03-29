// orktree – git worktree + fuse-overlayfs manager (CLI layer).
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
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
	case "check":
		return cmdCheck(args[1:])
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

// discoverFromCwd locates the orktree Manager by walking up from cwd.
// If no state is found, it auto-initializes at the git repo root.
func discoverFromCwd() (*orktree.Manager, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	mgr, err := orktree.Discover(cwd)
	if err == nil {
		return mgr, nil
	}

	// Auto-init: detect git repo root and initialize there.
	out, gitErr := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if gitErr != nil {
		return nil, fmt.Errorf("no orktree workspace found \xe2\x80\x94 are you inside a git repository?\nhint: run 'orktree switch <branch>' from your repo root to get started")
	}
	repoRoot := strings.TrimSpace(string(out))
	fmt.Fprintf(os.Stderr, "orktree: no workspace found \xe2\x80\x94 initializing at %s\n", repoRoot)
	return orktree.Init(repoRoot)
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
	} else {
		fmt.Println("Run the fix commands above (log out and back in after any usermod), then re-run 'orktree check'.")
		return fmt.Errorf("prerequisites not met")
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

Usage:
  orktree <command> [flags]

Commands:
  switch  <branch> [--from <b>] [--no-git]   Enter orktree (auto-creates)
  switch  -                                   Return to the source root
  ls      [--quiet]                           List orktrees with status and size
  path    <branch> [--from <b>] [--no-git]    Print workspace path (auto-creates)
  rm      <branch> [--force]                  Remove orktree
  shell-init [--shell bash|zsh]               Print shell cd-on-switch snippet

Aliases:  sw → switch,  p → path,  list → ls,  remove → rm

Getting started:
  cd /path/to/your/repo
  eval "$(orktree shell-init)"     # add to ~/.bashrc or ~/.zshrc
  orktree switch my-feature        # creates and enters orktree
  orktree switch -                 # returns to source root

Run 'orktree <command> --help' for details on a specific command.
`)
}
