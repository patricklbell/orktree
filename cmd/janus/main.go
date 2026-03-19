// janus – container worktree manager.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/tabwriter"

	igit "github.com/patricklbell/janus/internal/git"
	"github.com/patricklbell/janus/internal/container"
	"github.com/patricklbell/janus/internal/overlay"
	"github.com/patricklbell/janus/internal/state"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "janus: %v\n", err)
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
	case "new", "n":
		return cmdNew(args[1:])
	case "ls", "list":
		return cmdLs(args[1:])
	case "switch", "sw":
		return cmdSwitch(args[1:])
	case "enter", "sh":
		return cmdEnter(args[1:])
	case "exec":
		return cmdExec(args[1:])
	case "rm", "remove":
		return cmdRm(args[1:])
	case "help", "--help", "-h":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q — run 'janus help'", args[0])
	}
}

// ---------------------------------------------------------------------------
// init
// ---------------------------------------------------------------------------

func cmdInit(args []string) error {
	source := "."
	image := state.DefaultImage

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--source", "-s":
			if i+1 >= len(args) {
				return fmt.Errorf("--source requires a value")
			}
			i++
			source = args[i]
		case "--image", "-i":
			if i+1 >= len(args) {
				return fmt.Errorf("--image requires a value")
			}
			i++
			image = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	abs, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	isGit := igit.IsGitRepo(abs)

	cfg, err := state.Init(source, image, isGit)
	if err != nil {
		return err
	}
	fmt.Printf("Initialized janus repo %s\n", cfg.ID)
	fmt.Printf("  source   : %s\n", cfg.SourceRoot)
	fmt.Printf("  image    : %s\n", cfg.Image)
	fmt.Printf("  data     : %s\n", cfg.DataDir)
	if isGit {
		fmt.Printf("  git repo : yes (worktrees will be git worktrees)\n")
	}
	fmt.Println()
	fmt.Println("Create a worktree with: janus new <branch>")
	return nil
}

// ---------------------------------------------------------------------------
// new  (janus new <branch> [--from <base>] [--no-git])
// ---------------------------------------------------------------------------

func cmdNew(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: janus new <branch> [--from <base>]")
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

	w, err := state.NewWorktree(cfg, branch)
	if err != nil {
		return err
	}

	// Git worktree setup.
	if cfg.IsGitRepo && !noGit {
		gitTreePath := cfg.GitWorktreeDir(w)
		exists, err := igit.BranchExists(cfg.SourceRoot, branch)
		if err != nil {
			return err
		}
		newBranch := !exists
		if err := igit.AddWorktree(cfg.SourceRoot, gitTreePath, branch, newBranch, from); err != nil {
			// Roll back state entry.
			state.RemoveWorktree(cfg, w.ID) //nolint:errcheck
			return fmt.Errorf("creating git worktree: %w", err)
		}
		w.GitWorktreePath = gitTreePath
		if err := state.UpdateWorktree(cfg, w); err != nil {
			return err
		}
	}

	upper, work, merged := cfg.OverlayDirs(w)
	lowerDir := cfg.LowerDir(w)

	if err := overlay.Create(upper, work, merged); err != nil {
		return err
	}

	if err := overlay.Mount(lowerDir, upper, work, merged); err != nil {
		return fmt.Errorf("%w\n(hint: overlay mounts require root — try running with sudo)", err)
	}

	cname := container.ContainerName(cfg.ID, w.ID)
	if err := container.Start(cname, cfg.Image, merged); err != nil {
		return err
	}

	w.ContainerID = cname
	if err := state.UpdateWorktree(cfg, w); err != nil {
		return err
	}

	fmt.Printf("Created worktree %s (branch: %s)\n", w.ID, w.Branch)
	fmt.Printf("  merged path : %s\n", merged)
	fmt.Printf("  container   : %s\n", cname)
	if w.GitWorktreePath != "" {
		fmt.Printf("  git worktree: %s\n", w.GitWorktreePath)
	}
	fmt.Println()
	fmt.Printf("Enter  : janus enter  %s\n", w.Branch)
	fmt.Printf("Switch : janus switch %s\n", w.Branch)
	fmt.Printf("Remove : janus rm     %s\n", w.Branch)
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

	if len(cfg.Worktrees) == 0 {
		fmt.Println("No worktrees. Create one with: janus new <branch>")
		return nil
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "BRANCH\tID\tSTATUS\tCONTAINER\tMERGED")
	for _, w := range cfg.Worktrees {
		_, _, merged := cfg.OverlayDirs(w)

		mounted, _ := overlay.IsMounted(merged)
		running, _ := container.IsRunning(w.ContainerID)

		status := "stopped"
		if mounted && running {
			status = "running"
		} else if mounted {
			status = "mounted"
		}

		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			w.Branch,
			w.ID,
			status,
			w.ContainerID,
			merged,
		)
	}
	tw.Flush()
	return nil
}

// ---------------------------------------------------------------------------
// switch  (janus switch <branch>)
//
// Ensures the worktree is running, then attempts to reopen VS Code with the
// Dev Containers "Attach to Running Container" feature.  No other editors
// support a true "reopen in container" workflow, so nothing is attempted for
// them.
// ---------------------------------------------------------------------------

func cmdSwitch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: janus switch <branch>")
	}
	branch := args[0]
	for i := 1; i < len(args); i++ {
		return fmt.Errorf("unknown flag %q", args[i])
	}

	cfg, err := loadFromCwd()
	if err != nil {
		return err
	}

	w, err := state.FindWorktree(cfg, branch)
	if err != nil {
		// Worktree doesn't exist yet — create it automatically.
		fmt.Printf("Worktree for branch %q not found; creating it...\n", branch)
		return cmdNew([]string{branch})
	}

	upper, work, merged := cfg.OverlayDirs(w)
	lowerDir := cfg.LowerDir(w)
	if err := overlay.EnsureMounted(lowerDir, upper, work, merged); err != nil {
		return err
	}
	if err := container.EnsureRunning(w.ContainerID, cfg.Image, merged); err != nil {
		return err
	}

	fmt.Printf("Switched to worktree %q\n", w.Branch)
	fmt.Printf("  container : %s\n", w.ContainerID)
	fmt.Printf("  path      : %s\n", merged)

	// Attempt VS Code Dev Containers "Attach to Running Container".
	// The URI reopens VS Code inside the container — no fallback to a simple
	// folder open, which the user can do themselves.
	if _, err := exec.LookPath("code"); err == nil {
		uri := attachedContainerURI(w.ContainerID, "/workspace")
		cmd := exec.Command("code", "--folder-uri", uri)
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "note: VS Code Dev Containers attach failed (%v)\n", err)
			fmt.Fprintf(os.Stderr, "      Ensure the Dev Containers extension is installed and the container is running.\n")
			fmt.Fprintf(os.Stderr, "      Container: %s\n", w.ContainerID)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// enter  (janus enter <ref>)
// ---------------------------------------------------------------------------

func cmdEnter(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: janus enter <branch>")
	}
	cfg, err := loadFromCwd()
	if err != nil {
		return err
	}
	w, err := state.FindWorktree(cfg, args[0])
	if err != nil {
		return err
	}

	upper, work, merged := cfg.OverlayDirs(w)
	lowerDir := cfg.LowerDir(w)
	if err := overlay.EnsureMounted(lowerDir, upper, work, merged); err != nil {
		return err
	}
	if err := container.EnsureRunning(w.ContainerID, cfg.Image, merged); err != nil {
		return err
	}
	return container.Enter(w.ContainerID)
}

// ---------------------------------------------------------------------------
// exec  (janus exec <ref> -- <cmd...>)
// ---------------------------------------------------------------------------

func cmdExec(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: janus exec <branch> -- <cmd...>")
	}
	ref := args[0]
	cmdArgs := args[1:]
	if len(cmdArgs) > 0 && cmdArgs[0] == "--" {
		cmdArgs = cmdArgs[1:]
	}
	if len(cmdArgs) == 0 {
		return fmt.Errorf("no command provided")
	}

	cfg, err := loadFromCwd()
	if err != nil {
		return err
	}
	w, err := state.FindWorktree(cfg, ref)
	if err != nil {
		return err
	}

	upper, work, merged := cfg.OverlayDirs(w)
	lowerDir := cfg.LowerDir(w)
	if err := overlay.EnsureMounted(lowerDir, upper, work, merged); err != nil {
		return err
	}
	if err := container.EnsureRunning(w.ContainerID, cfg.Image, merged); err != nil {
		return err
	}
	return container.Exec(w.ContainerID, cmdArgs)
}

// ---------------------------------------------------------------------------
// rm  (janus rm <ref> [--force])
// ---------------------------------------------------------------------------

func cmdRm(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: janus rm <branch> [--force]")
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
	w, err := state.FindWorktree(cfg, ref)
	if err != nil {
		return err
	}

	upper, work, merged := cfg.OverlayDirs(w)

	// Stop container.
	if w.ContainerID != "" {
		if err := container.Stop(w.ContainerID); err != nil && !force {
			return fmt.Errorf("stopping container: %w (use --force to ignore)", err)
		}
	}

	// Remove overlayfs.
	if err := overlay.Remove(upper, work, merged); err != nil && !force {
		return fmt.Errorf("removing overlay: %w (use --force to ignore)", err)
	}

	// Remove git worktree.
	if w.GitWorktreePath != "" {
		if err := igit.RemoveWorktree(cfg.SourceRoot, w.GitWorktreePath); err != nil && !force {
			return fmt.Errorf("removing git worktree: %w (use --force to ignore)", err)
		}
		igit.PruneWorktrees(cfg.SourceRoot) //nolint:errcheck
	}

	// Remove from state.
	if err := state.RemoveWorktree(cfg, w.ID); err != nil {
		return err
	}

	fmt.Printf("Removed worktree %s (branch: %s)\n", w.ID, w.Branch)
	return nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// loadFromCwd finds the source root by looking for .janus/state.json
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
	return nil, fmt.Errorf("no .janus/state.json found in %s or any parent directory (did you run 'janus init'?)", cwd)
}

// attachedContainerURI returns the VS Code Remote URI for attaching to a
// running Docker container via the Dev Containers extension.
//
// The protocol requires a hex-encoded JSON descriptor:
//
//	{"containerName":"/<name>"}
//
// Docker container names always have a leading "/" in the API, so we add one.
// Ref: https://code.visualstudio.com/docs/devcontainers/attach-container
func attachedContainerURI(containerName, workspacePath string) string {
	// Build the JSON identifier VS Code expects.
	jsonDesc := fmt.Sprintf(`{"containerName":"/%s"}`, containerName)
	// Hex-encode each byte of the JSON string.
	hexBuf := make([]byte, len(jsonDesc)*2)
	const hexChars = "0123456789abcdef"
	for i, b := range []byte(jsonDesc) {
		hexBuf[i*2] = hexChars[b>>4]
		hexBuf[i*2+1] = hexChars[b&0xf]
	}
	return fmt.Sprintf("vscode-remote://attached-container+%s%s", hexBuf, workspacePath)
}

// ---------------------------------------------------------------------------
// usage
// ---------------------------------------------------------------------------

func printUsage() {
	fmt.Print(`janus – container worktree manager

Usage:
  janus <command> [args...]

Commands:
  init   [--source <path>] [--image <image>]  Initialize janus in a directory
  new    <branch> [--from <base>]             Create worktree on branch
  ls                                          List worktrees
  switch <branch>                             Start worktree and reopen VS Code in container
  enter  <branch>                             Open shell in worktree container
  exec   <branch> -- <cmd...>                 Run command in worktree container
  rm     <branch> [--force]                   Remove worktree

Aliases:
  new → n    switch → sw    enter → sh    rm → remove    ls → list

<branch> can be the branch name, the full worktree ID, or a unique prefix.
`)
}
