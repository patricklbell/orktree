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
	case "doctor", "doc":
		return cmdDoctor(args[1:])
	case "ls", "list":
		return cmdLs(args[1:])
	case "switch", "sw":
		return cmdSwitch(args[1:])
	case "rm", "remove":
		return cmdRm(args[1:])
	case "path", "p":
		return cmdPath(args[1:])
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
		return nil, fmt.Errorf("no orktree workspace found \xe2\x80\x94 are you inside a git repository?\nhint: run 'orktree switch <branch>' from your repo root to get started")
	}
	repoRoot := strings.TrimSpace(string(out))
	fmt.Fprintf(os.Stderr, "orktree: no workspace found \xe2\x80\x94 initializing at %s\n", repoRoot)
	return orktree.CreateIndex(repoRoot)
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
	fmt.Fprintf(tw, "\t\t────\t\n")
	fmt.Fprintf(tw, "\ttotal\t%s\t\n", humanSize(total))
	tw.Flush()
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
// switch  (orktree switch <branch> [--from <base>] [--no-git])
// ---------------------------------------------------------------------------

func cmdSwitch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: orktree switch <branch> [--from <base>] [--no-git]")
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
		return nil
	}

	mgr, err := discoverFromCwd()
	if err != nil {
		return err
	}

	_, findErr := mgr.FindOrktree(branch)
	if findErr != nil {
		fmt.Fprintf(os.Stderr, "No orktree for '%s' \xe2\x80\x94 creating it now...\n", branch)
	}

	info, err := mgr.EnsureOrktree(branch, orktree.CreateOrktreeOptions{From: from, NoGit: noGit})
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

	mgr, err := discoverFromCwd()
	if err != nil {
		return err
	}

	// "-" means source root — used by the shell wrapper for "orktree switch -"
	if branch == "-" {
		fmt.Println(mgr.SourceRoot())
		return nil
	}

	_, findErr := mgr.FindOrktree(branch)
	if findErr != nil {
		fmt.Fprintf(os.Stderr, "No orktree for '%s' \xe2\x80\x94 creating it now...\n", branch)
	}

	info, err := mgr.EnsureOrktree(branch, orktree.CreateOrktreeOptions{From: from, NoGit: noGit})
	if err != nil {
		return err
	}

	fmt.Println(info.MergedPath)
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
		return fmt.Errorf("usage: orktree rm <branch> [--force]")
	}
	ref := args[0]
	force := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--force", "-f":
			force = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	mgr, err := discoverFromCwd()
	if err != nil {
		return err
	}

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
		return fmt.Errorf("cannot remove %q — has dependents", rc.Branch)
	}

	if force {
		if err := mgr.RemoveOrktree(ref); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Removed orktree '%s'\n", info.Branch)
		return nil
	}

	// Clean orktree — remove without prompt.
	if rc.IsClean() {
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

	// Determine default: Yes if only untracked/ignored files, No otherwise.
	onlyMinor := rc.UnmergedTotal == 0 && rc.TrackedTotal == 0
	defaultYes := onlyMinor

	tty := isTerminal(os.Stdin.Fd()) && isTerminal(os.Stderr.Fd())
	if !tty {
		fmt.Fprintln(os.Stderr, "pass --force to remove without confirmation")
		return fmt.Errorf("cannot confirm removal of %q — not a terminal", rc.Branch)
	}

	prompt := fmt.Sprintf("Remove orktree %q (%s)? This cannot be undone.", rc.Branch, rc.MergedPath)
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
  switch  <branch> [--from <b>] [--no-git]   Enter orktree (auto-creates)
  switch  -                                   Return to the source root
  ls      [--quiet]                           List orktrees with status and size
  path    <branch> [--from <b>] [--no-git]    Print workspace path (auto-creates)
  rm      <branch> [--force]                  Remove orktree
  doctor									  Runs the doctor to diagnose issues

Aliases:  sw → switch,  p → path,  list → ls,  remove → rm

Getting started:
  cd /path/to/your/repo
  orktree switch my-feature        # creates and enters orktree
  orktree switch -                 # returns to source root

Run 'man orktree <command>' for details on a specific command.
`)
}
