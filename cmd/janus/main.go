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
	// Core commands
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
	case "open":
		return cmdOpen(args[1:])
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
	fmt.Printf("Open   : janus open   %s\n", w.Branch)
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
// ---------------------------------------------------------------------------

func cmdSwitch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: janus switch <branch>")
	}
	branch := args[0]
	editor := ""
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--editor", "-e":
			if i+1 >= len(args) {
				return fmt.Errorf("--editor requires a value")
			}
			i++
			editor = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
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

	if editor == "" {
		editor = detectEditor()
	}

	fmt.Printf("Switching to worktree %q\n", w.Branch)
	fmt.Printf("  container : %s\n", w.ContainerID)
	fmt.Printf("  path      : %s\n", merged)

	if editor == "" {
		fmt.Println("(no editor detected; open the path above manually)")
		return nil
	}

	return openInEditor(editor, w.ContainerID, merged, true)
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
// open  (janus open <ref> [--editor <e>])
// ---------------------------------------------------------------------------

func cmdOpen(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: janus open <branch> [--editor code|vim|emacs]")
	}
	ref := args[0]
	editor := ""
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--editor", "-e":
			if i+1 >= len(args) {
				return fmt.Errorf("--editor requires a value")
			}
			i++
			editor = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
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

	_, _, merged := cfg.OverlayDirs(w)

	if editor == "" {
		editor = detectEditor()
	}
	if editor == "" {
		fmt.Printf("Worktree merged path: %s\n", merged)
		fmt.Println("(Could not detect editor; set --editor or open the path manually.)")
		return nil
	}

	fmt.Printf("Opening %s in %s\n", merged, editor)
	return openInEditor(editor, w.ContainerID, merged, false)
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

// detectEditor returns the first available editor in a priority list.
func detectEditor() string {
	for _, e := range []string{"code", "vim", "nano", "emacs"} {
		if _, err := exec.LookPath(e); err == nil {
			return e
		}
	}
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	return ""
}

// openInEditor opens the merged path in the given editor.
// When reuse is true and editor is VS Code, the existing window is reused;
// a Dev Containers URI is attempted first so the editor attaches to the
// running container.
func openInEditor(editor, containerID, merged string, reuse bool) error {
	if editor == "code" {
		if reuse && containerID != "" {
			// Try to attach VS Code to the running container via the Dev
			// Containers extension. The URI format is:
			//   vscode-remote://attached-container+<hex-container-name>/workspace
			uri := devContainersURI(containerID, "/workspace")
			cmd := exec.Command("code", "--folder-uri", uri)
			if err := cmd.Run(); err == nil {
				return nil
			}
			// Fall back: reuse the window and open the host-visible merged path.
			cmd = exec.Command("code", "--reuse-window", merged)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
		cmd := exec.Command("code", merged)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	cmd := exec.Command(editor, merged)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// devContainersURI returns a VS Code Remote URI that attaches to the given
// Docker container and opens the workspace path inside it.
// The container identifier is hex-encoded as required by the protocol.
func devContainersURI(containerName, workspacePath string) string {
	var hex []byte
	const hexChars = "0123456789abcdef"
	for _, b := range []byte(containerName) {
		hex = append(hex, hexChars[b>>4], hexChars[b&0xf])
	}
	return fmt.Sprintf("vscode-remote://attached-container+%s%s", hex, workspacePath)
}

// ---------------------------------------------------------------------------
// usage
// ---------------------------------------------------------------------------

func printUsage() {
	fmt.Print(`janus – container worktree manager

Usage:
  janus <command> [args...]

Commands:
  init   [--source <path>] [--image <image>]   Initialize janus in a directory
  new    <branch> [--from <base>]              Create worktree on branch
  ls                                           List worktrees
  switch <branch>                              Switch to worktree (start + open editor)
  enter  <branch>                              Open shell in worktree container
  exec   <branch> -- <cmd...>                  Run command in worktree container
  open   <branch> [--editor code|vim|emacs]    Open worktree path in editor
  rm     <branch> [--force]                    Remove worktree

Aliases:
  new → n    switch → sw    enter → sh    rm → remove    ls → list

<branch> can be the branch name, the full worktree ID, or a unique prefix.
`)
}
