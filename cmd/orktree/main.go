// orktree – git worktree + fuse-overlayfs manager (CLI layer).
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"text/tabwriter"
	"unsafe"

	"github.com/patricklbell/orktree/pkg/orktree"
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
	case "add":
		return cmdAdd(args[1:])
	case "doctor", "doc":
		return cmdDoctor(args[1:])
	case "ls", "list":
		return cmdLs(args[1:])
	case "rm", "remove":
		return cmdRm(args[1:])
	case "path", "p":
		return cmdPath(args[1:])
	case "mount":
		return cmdMount(args[1:])
	case "unmount", "umount":
		return cmdUnmount(args[1:])
	case "move", "mv":
		return cmdMove(args[1:])
	case "help", "--help", "-h":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q \xe2\x80\x94 run 'orktree help'", args[0])
	}
}

func discoverFromCwd() (*orktree.Index, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	mgr, err := orktree.DiscoverIndex(cwd)
	if err == nil {
		return mgr, nil
	}

	// Auto-init: detect git repo root and initialize there.
	out, gitErr := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if gitErr != nil {
		return nil, fmt.Errorf("no orktree workspace found \xe2\x80\x94 are you inside a git repository?\nhint: run 'orktree add <path>' from your repo root to get started")
	}
	repoRoot := strings.TrimSpace(string(out))
	fmt.Fprintf(os.Stderr, "orktree: no workspace found \xe2\x80\x94 initializing at %s\n", repoRoot)
	return orktree.CreateIndex(repoRoot)
}

// ---------------------------------------------------------------------------
// add
// ---------------------------------------------------------------------------

func cmdAdd(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: orktree add <path> [<commit-ish>] [-- <git-flags>...]")
	}

	// Split on "--": everything before is orktree args, everything after is
	// forwarded to git worktree add.
	var orktreeArgs, extraArgs []string
	for i, a := range args {
		if a == "--" {
			orktreeArgs = args[:i]
			extraArgs = args[i+1:]
			break
		}
	}
	if orktreeArgs == nil {
		orktreeArgs = args
	}

	if len(orktreeArgs) == 0 {
		return fmt.Errorf("usage: orktree add <path> [<commit-ish>] [-- <git-flags>...]")
	}

	path := orktreeArgs[0]
	var commitIsh string
	if len(orktreeArgs) > 1 {
		commitIsh = orktreeArgs[1]
	}

	mgr, err := discoverFromCwd()
	if err != nil {
		return err
	}

	info, err := mgr.AddOrktree(path, orktree.AddOrktreeOptions{
		CommitIsh: commitIsh,
		ExtraArgs: extraArgs,
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Created orktree '%s' at %s\n", info.Branch, info.MergedPath)
	fmt.Println(info.MergedPath)
	return nil
}

// ---------------------------------------------------------------------------
// doctor
// ---------------------------------------------------------------------------

func cmdDoctor(_ []string) error {
	fmt.Println("orktree prerequisites")
	fmt.Println()

	prereqs := orktree.CheckEnvironmentPrerequisites()

	// Compute max name width for aligned output.
	maxWidth := 0
	for _, p := range prereqs {
		if len(p.Name) > maxWidth {
			maxWidth = len(p.Name)
		}
	}

	var required, optional []orktree.Prerequisite
	for _, p := range prereqs {
		if p.Optional {
			optional = append(optional, p)
		} else {
			required = append(required, p)
		}
	}

	ok := true
	fmtStr := fmt.Sprintf("  %%s  %%-%ds", maxWidth)
	for _, p := range required {
		if p.OK {
			fmt.Printf(fmtStr+"\n", "\xe2\x9c\x93", p.Name)
		} else {
			fmt.Printf(fmtStr+"  ->  %s\n", "\xe2\x9c\x97", p.Name, p.Fix)
			ok = false
		}
	}

	if len(optional) > 0 {
		fmt.Println()
		fmt.Println("Optional:")
		for _, p := range optional {
			if p.OK {
				fmt.Printf(fmtStr+"\n", "\xe2\x9c\x93", p.Name)
			} else {
				fmt.Printf(fmtStr+"  ->  %s\n", "\xe2\x9c\x97", p.Name, p.Fix)
			}
		}
	}

	fmt.Println()
	if ok {
		fmt.Println("All prerequisites satisfied.")
	} else {
		fmt.Println("Run the fix commands above (log out and back in after any usermod), then re-run 'orktree doctor'.")
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
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	mgr, err := discoverFromCwd()
	if err != nil {
		return err
	}

	infos, err := mgr.ListOrktrees()
	if err != nil {
		return err
	}

	if len(infos) == 0 {
		if !quiet {
			fmt.Println("No orktrees yet. Run 'orktree add <path>' to create one.")
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
	fmt.Fprintln(tw, "BRANCH\tSTATUS\tSIZE\tPATH")
	var total int64
	for _, info := range infos {
		status := "unmounted"
		if info.Mounted {
			status = "mounted"
		}
		size := "?"
		if info.UpperDirSize >= 0 {
			size = humanSize(info.UpperDirSize)
			total += info.UpperDirSize
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			info.Branch,
			status,
			size,
			info.MergedPath,
		)
	}
	fmt.Fprintf(tw, "\t\t\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\t\n")
	fmt.Fprintf(tw, "\ttotal\t%s\t\n", humanSize(total))
	tw.Flush() //nolint:errcheck
	return nil
}

// humanSize formats bytes as a human-readable string using base-1024
// thresholds with short suffixes (B, K, M, G).
func humanSize(b int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1fG", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1fM", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1fK", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%dB", b)
	}
}

// ---------------------------------------------------------------------------
// path
// ---------------------------------------------------------------------------

func cmdPath(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: orktree path <worktree>")
	}

	mgr, err := discoverFromCwd()
	if err != nil {
		return err
	}

	resolved, err := mgr.ResolveOrktreePath(args[0])
	if err != nil {
		return err
	}

	fmt.Println(resolved)
	return nil
}

// ---------------------------------------------------------------------------
// mount
// ---------------------------------------------------------------------------

func cmdMount(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: orktree mount <worktree>")
	}

	mgr, err := discoverFromCwd()
	if err != nil {
		return err
	}

	if err := mgr.MountOrktree(args[0]); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Mounted orktree '%s'\n", args[0])
	return nil
}

// ---------------------------------------------------------------------------
// unmount
// ---------------------------------------------------------------------------

func cmdUnmount(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: orktree unmount <worktree>")
	}

	mgr, err := discoverFromCwd()
	if err != nil {
		return err
	}

	if err := mgr.UnmountOrktree(args[0]); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Unmounted orktree '%s'\n", args[0])
	return nil
}

// ---------------------------------------------------------------------------
// move
// ---------------------------------------------------------------------------

func cmdMove(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: orktree move <worktree> <new-path>")
	}

	mgr, err := discoverFromCwd()
	if err != nil {
		return err
	}

	if err := mgr.MoveOrktree(args[0], args[1]); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Moved orktree '%s' to %s\n", args[0], args[1])
	return nil
}

// ---------------------------------------------------------------------------
// rm
// ---------------------------------------------------------------------------

// isTerminal reports whether the given file descriptor refers to a terminal.
// Uses a raw TCGETS ioctl instead of golang.org/x/term to avoid an external
// dependency — orktree targets Linux only where TCGETS is stable.
func isTerminal(fd uintptr) bool {
	var termios syscall.Termios
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TCGETS, uintptr(unsafe.Pointer(&termios)))
	return err == 0
}

// formatAssessment renders a RemoveCheck as a human-readable categorized summary.
// Only non-empty sections are included; ignored files show only a count.
func formatAssessment(rc *orktree.RemoveCheck) string {
	var sections []string

	if rc.UnmergedTotal > 0 {
		var b strings.Builder
		b.WriteString("Commits only on this branch:")
		cap := rc.UnmergedTotal
		if cap > 10 {
			cap = 10
		}
		if cap > len(rc.UnmergedCommits) {
			cap = len(rc.UnmergedCommits)
		}
		for _, c := range rc.UnmergedCommits[:cap] {
			fmt.Fprintf(&b, "\n  %s", c)
		}
		if rc.UnmergedTotal > 10 {
			fmt.Fprintf(&b, "\n  ... and %d more", rc.UnmergedTotal-10)
		}
		sections = append(sections, b.String())
	}

	if rc.TrackedTotal > 0 {
		var b strings.Builder
		b.WriteString("Modified tracked files:")
		cap := rc.TrackedTotal
		if cap > 10 {
			cap = 10
		}
		if cap > len(rc.TrackedDirty) {
			cap = len(rc.TrackedDirty)
		}
		for _, f := range rc.TrackedDirty[:cap] {
			fmt.Fprintf(&b, "\n  %s", f)
		}
		if rc.TrackedTotal > 10 {
			fmt.Fprintf(&b, "\n  ... and %d more", rc.TrackedTotal-10)
		}
		sections = append(sections, b.String())
	}

	if rc.UntrackedTotal > 0 {
		var b strings.Builder
		b.WriteString("Untracked files:")
		cap := rc.UntrackedTotal
		if cap > 10 {
			cap = 10
		}
		if cap > len(rc.UntrackedDirty) {
			cap = len(rc.UntrackedDirty)
		}
		for _, f := range rc.UntrackedDirty[:cap] {
			fmt.Fprintf(&b, "\n  %s", f)
		}
		if rc.UntrackedTotal > 10 {
			fmt.Fprintf(&b, "\n  ... and %d more", rc.UntrackedTotal-10)
		}
		sections = append(sections, b.String())
	}

	if rc.IgnoredDirty > 0 {
		sections = append(sections, fmt.Sprintf("Ignored files: %d files", rc.IgnoredDirty))
	}

	return strings.Join(sections, "\n\n")
}

func cmdRm(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: orktree rm <worktree>... [--force] [--ignore-untracked] [--ignore-tracked]")
	}

	var branches []string
	force := false
	ignoreUntracked := false
	ignoreTracked := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--force", "-f":
			force = true
		case "--ignore-untracked":
			ignoreUntracked = true
		case "--ignore-tracked":
			ignoreTracked = true
		default:
			if strings.HasPrefix(args[i], "-") {
				return fmt.Errorf("unknown flag %q", args[i])
			}
			branches = append(branches, args[i])
		}
	}

	if len(branches) == 0 {
		return fmt.Errorf("usage: orktree rm <worktree>... [--force] [--ignore-untracked] [--ignore-tracked]")
	}

	// --force implies both scoped ignore flags.
	if force {
		ignoreUntracked = true
		ignoreTracked = true
	}

	mgr, err := discoverFromCwd()
	if err != nil {
		return err
	}

	// Single branch: preserve original error propagation behavior.
	if len(branches) == 1 {
		return rmOne(mgr, branches[0], force, ignoreUntracked, ignoreTracked)
	}

	// Multiple branches: run all, collect errors.
	var errCount int
	for _, ref := range branches {
		if err := rmOne(mgr, ref, force, ignoreUntracked, ignoreTracked); err != nil {
			fmt.Fprintf(os.Stderr, "orktree: rm %s: %v\n", ref, err)
			errCount++
		}
	}
	if errCount > 0 {
		return fmt.Errorf("%d of %d orktree(s) could not be removed", errCount, len(branches))
	}
	return nil
}

func rmOne(mgr *orktree.Index, ref string, force, ignoreUntracked, ignoreTracked bool) error {
	info, err := mgr.FindOrktree(ref)
	if err != nil {
		return err
	}

	// Always check for dependents (even with --force).
	rc, err := mgr.CheckRemoveOrktree(ref)
	if err != nil {
		return err
	}

	if rc.HasBlockers() {
		fmt.Fprintf(os.Stderr, "cannot remove %q \u2014 %d other orktree(s) depend on it as a base:\n", rc.Branch, len(rc.Dependents))
		for _, d := range rc.Dependents {
			fmt.Fprintf(os.Stderr, "  %s\n", d)
		}
		fmt.Fprintln(os.Stderr, "remove the dependent orktrees first, or re-stack them with a different base")
		return fmt.Errorf("cannot remove %q \xe2\x80\x94 has dependents", rc.Branch)
	}

	// --force was passed: skip the safety assessment and remove immediately.
	// Dependents are still checked above — force does not bypass that.
	if force {
		if err := mgr.RemoveOrktree(ref); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Removed orktree '%s'\n", info.Branch)
		return nil
	}

	// Skip all prompts when the orktree is clean enough given the ignore flags.
	if rc.IsCleanWith(ignoreUntracked, ignoreTracked) {
		if err := mgr.RemoveOrktree(ref); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Removed orktree '%s'\n", info.Branch)
		return nil
	}

	// Print categorized assessment.
	assessment := formatAssessment(rc)
	fmt.Fprintln(os.Stderr, assessment)
	fmt.Fprintln(os.Stderr)

	// Default Yes only when no "major" concerns remain after applying ignore flags.
	// Unmerged commits and tracked changes (unless ignored) are major.
	majorRemaining := rc.UnmergedTotal > 0 || (!ignoreTracked && rc.TrackedTotal > 0)
	defaultYes := !majorRemaining

	tty := isTerminal(os.Stdin.Fd()) && isTerminal(os.Stderr.Fd())
	if !tty {
		fmt.Fprintln(os.Stderr, "pass --force to remove without confirmation")
		return fmt.Errorf("cannot confirm removal of %q \xe2\x80\x94 not a terminal", rc.Branch)
	}

	prompt := fmt.Sprintf("Remove orktree %q (%s)? Commits on this branch are preserved in git history.", rc.Branch, rc.MergedPath)
	if defaultYes {
		fmt.Fprintf(os.Stderr, "%s [Y/n] ", prompt)
	} else {
		fmt.Fprintf(os.Stderr, "%s [y/N] ", prompt)
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	answer := strings.TrimSpace(scanner.Text())

	confirmed := false
	switch strings.ToLower(answer) {
	case "y", "yes":
		confirmed = true
	case "":
		confirmed = defaultYes
	}

	if !confirmed {
		return fmt.Errorf("removal cancelled")
	}

	if err := mgr.RemoveOrktree(ref); err != nil {
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
  add     <path> [<commit-ish>] [-- <git-flags>...]      Create a new orktree
  rm      <worktree>... [--force] [--ignore-untracked] [--ignore-tracked]    Remove orktree(s)
  ls      [--quiet]                                        List orktrees
  path    <worktree>                                       Print workspace path
  mount   <worktree>                                       Mount overlay
  unmount <worktree>                                       Unmount overlay
  move    <worktree> <new-path>                            Move orktree
  doctor                                                   Diagnose issues
  help                                                     Show this help

Aliases:  p → path,  list → ls,  remove → rm,  umount → unmount,  mv → move

Getting started:
  cd /path/to/your/repo
  orktree add ../feature-x           # creates orktree in sibling directory
  orktree add ../stacked feature-x   # stack on existing orktree

Run 'man orktree <command>' for details on a specific command.
`)
}
