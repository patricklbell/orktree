// agentw – CLI tool for managing isolated container workspaces.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/tabwriter"

	"github.com/patricklbell/janus/internal/container"
	"github.com/patricklbell/janus/internal/overlay"
	"github.com/patricklbell/janus/internal/state"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "agentw: %v\n", err)
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
	case "workspace", "ws":
		return cmdWorkspace(args[1:])
	case "help", "--help", "-h":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q — run 'agentw help'", args[0])
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

	ws, err := state.Init(source, image)
	if err != nil {
		return err
	}
	fmt.Printf("Initialised workspace set %s\n", ws.ID)
	fmt.Printf("  source : %s\n", ws.SourceRoot)
	fmt.Printf("  image  : %s\n", ws.Image)
	fmt.Printf("  data   : %s\n", ws.DataDir)
	fmt.Println()
	fmt.Println("Create a workspace with: agentw workspace new")
	return nil
}

// ---------------------------------------------------------------------------
// workspace (aliased as ws)
// ---------------------------------------------------------------------------

func cmdWorkspace(args []string) error {
	if len(args) == 0 {
		printWorkspaceUsage()
		return nil
	}
	switch args[0] {
	case "new":
		return cmdWorkspaceNew(args[1:])
	case "ls", "list":
		return cmdWorkspaceLs(args[1:])
	case "enter":
		return cmdWorkspaceEnter(args[1:])
	case "exec":
		return cmdWorkspaceExec(args[1:])
	case "open":
		return cmdWorkspaceOpen(args[1:])
	case "rm", "remove":
		return cmdWorkspaceRm(args[1:])
	default:
		return fmt.Errorf("unknown workspace sub-command %q — run 'agentw workspace'", args[0])
	}
}

// ---------------------------------------------------------------------------
// workspace new
// ---------------------------------------------------------------------------

func cmdWorkspaceNew(args []string) error {
	name := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name", "-n":
			if i+1 >= len(args) {
				return fmt.Errorf("--name requires a value")
			}
			i++
			name = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	ws, err := loadFromCwd()
	if err != nil {
		return err
	}

	w, err := state.NewWorkspace(ws, name)
	if err != nil {
		return err
	}

	upper, work, merged := ws.OverlayDirs(w)

	if err := overlay.Create(upper, work, merged); err != nil {
		return err
	}

	if err := overlay.Mount(ws.SourceRoot, upper, work, merged); err != nil {
		return fmt.Errorf("%w\n(hint: overlay mounts require root — try running with sudo)", err)
	}

	cname := container.ContainerName(ws.ID, w.ID)
	if err := container.Start(cname, ws.Image, merged); err != nil {
		return err
	}

	w.ContainerID = cname
	if err := state.UpdateWorkspace(ws, w); err != nil {
		return err
	}

	fmt.Printf("Created workspace %s", w.ID)
	if w.Name != "" {
		fmt.Printf(" (%s)", w.Name)
	}
	fmt.Println()
	fmt.Printf("  merged path : %s\n", merged)
	fmt.Printf("  container   : %s\n", cname)
	fmt.Println()
	fmt.Printf("Enter  : agentw workspace enter %s\n", w.ID)
	fmt.Printf("Open   : agentw workspace open  %s\n", w.ID)
	fmt.Printf("Remove : agentw workspace rm    %s\n", w.ID)
	return nil
}

// ---------------------------------------------------------------------------
// workspace ls
// ---------------------------------------------------------------------------

func cmdWorkspaceLs(_ []string) error {
	ws, err := loadFromCwd()
	if err != nil {
		return err
	}

	if len(ws.Workspaces) == 0 {
		fmt.Println("No workspaces. Create one with: agentw workspace new")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSTATUS\tCONTAINER\tMERGED")
	for _, workspace := range ws.Workspaces {
		_, _, merged := ws.OverlayDirs(workspace)

		mounted, _ := overlay.IsMounted(merged)
		running, _ := container.IsRunning(workspace.ContainerID)

		status := "stopped"
		if mounted && running {
			status = "running"
		} else if mounted {
			status = "mounted"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			workspace.ID,
			workspace.Name,
			status,
			workspace.ContainerID,
			merged,
		)
	}
	w.Flush()
	return nil
}

// ---------------------------------------------------------------------------
// workspace enter
// ---------------------------------------------------------------------------

func cmdWorkspaceEnter(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agentw workspace enter <workspace_id>")
	}
	ws, err := loadFromCwd()
	if err != nil {
		return err
	}
	w, err := state.FindWorkspace(ws, args[0])
	if err != nil {
		return err
	}

	upper, work, merged := ws.OverlayDirs(w)
	if err := overlay.EnsureMounted(ws.SourceRoot, upper, work, merged); err != nil {
		return err
	}
	if err := container.EnsureRunning(w.ContainerID, ws.Image, merged); err != nil {
		return err
	}
	return container.Enter(w.ContainerID)
}

// ---------------------------------------------------------------------------
// workspace exec
// ---------------------------------------------------------------------------

func cmdWorkspaceExec(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: agentw workspace exec <workspace_id> -- <cmd...>")
	}
	wsID := args[0]
	cmdArgs := args[1:]
	// strip leading "--"
	if len(cmdArgs) > 0 && cmdArgs[0] == "--" {
		cmdArgs = cmdArgs[1:]
	}
	if len(cmdArgs) == 0 {
		return fmt.Errorf("no command provided")
	}

	ws, err := loadFromCwd()
	if err != nil {
		return err
	}
	w, err := state.FindWorkspace(ws, wsID)
	if err != nil {
		return err
	}

	upper, work, merged := ws.OverlayDirs(w)
	if err := overlay.EnsureMounted(ws.SourceRoot, upper, work, merged); err != nil {
		return err
	}
	if err := container.EnsureRunning(w.ContainerID, ws.Image, merged); err != nil {
		return err
	}
	return container.Exec(w.ContainerID, cmdArgs)
}

// ---------------------------------------------------------------------------
// workspace open
// ---------------------------------------------------------------------------

func cmdWorkspaceOpen(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agentw workspace open <workspace_id> [--editor <editor>]")
	}
	wsID := args[0]
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

	ws, err := loadFromCwd()
	if err != nil {
		return err
	}
	w, err := state.FindWorkspace(ws, wsID)
	if err != nil {
		return err
	}

	_, _, merged := ws.OverlayDirs(w)

	if editor == "" {
		editor = detectEditor()
	}
	if editor == "" {
		fmt.Printf("Workspace merged path: %s\n", merged)
		fmt.Println("(Could not detect editor; set --editor or open the path manually.)")
		return nil
	}

	fmt.Printf("Opening %s in %s\n", merged, editor)
	cmd := exec.Command(editor, merged)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ---------------------------------------------------------------------------
// workspace rm
// ---------------------------------------------------------------------------

func cmdWorkspaceRm(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agentw workspace rm <workspace_id> [--force]")
	}
	wsID := args[0]
	force := false
	for i := 1; i < len(args); i++ {
		if args[i] == "--force" || args[i] == "-f" {
			force = true
		}
	}

	ws, err := loadFromCwd()
	if err != nil {
		return err
	}
	w, err := state.FindWorkspace(ws, wsID)
	if err != nil {
		return err
	}

	upper, work, merged := ws.OverlayDirs(w)

	// Stop container
	if w.ContainerID != "" {
		if err := container.Stop(w.ContainerID); err != nil && !force {
			return fmt.Errorf("stopping container: %w (use --force to ignore)", err)
		}
	}

	// Remove overlay
	if err := overlay.Remove(upper, work, merged); err != nil && !force {
		return fmt.Errorf("removing overlay: %w (use --force to ignore)", err)
	}

	// Remove from state
	if err := state.RemoveWorkspace(ws, w.ID); err != nil {
		return err
	}

	fmt.Printf("Removed workspace %s\n", w.ID)
	return nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// loadFromCwd finds the source root by looking for .agentw/state.json
// starting from the current directory and walking up.
func loadFromCwd() (*state.WorkspaceSet, error) {
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
	return nil, fmt.Errorf("no .agentw/state.json found in %s or any parent directory (did you run 'agentw init'?)", cwd)
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

// ---------------------------------------------------------------------------
// usage
// ---------------------------------------------------------------------------

func printUsage() {
	fmt.Print(`agentw – container worktree manager

Usage:
  agentw init [--source <path>] [--image <image>]
  agentw workspace <sub-command> [args...]
  agentw ws        <sub-command> [args...]   (alias)

Commands:
  init      Initialise a workspace set for a source directory
  workspace Manage workspaces (aliases: ws)

Run 'agentw workspace' for workspace sub-commands.
`)
}

func printWorkspaceUsage() {
	fmt.Print(`agentw workspace – manage container workspaces

Sub-commands:
  new    [--name <name>]                  Create a new workspace
  ls                                      List all workspaces
  enter  <id>                             Open interactive shell
  exec   <id> -- <cmd...>                 Run command in workspace
  open   <id> [--editor code|vim|emacs]  Open workspace in editor
  rm     <id> [--force]                  Remove workspace

<id> can be the full workspace ID or a unique prefix.
`)
}
