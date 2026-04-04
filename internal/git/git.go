package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func IsGitRepo(path string) bool {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--git-dir")
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// MainWorktreeRoot returns the root directory of the main git worktree that
// owns the linked worktree containing path. Returns "" if path is not inside
// a git repository, is not a linked worktree, or is already the main worktree.
func MainWorktreeRoot(path string) (string, error) {
	gitDir, err := exec.Command("git", "-C", path, "rev-parse", "--git-dir").Output()
	if err != nil {
		return "", nil // not a git repo
	}
	commonDir, err := exec.Command("git", "-C", path, "rev-parse", "--git-common-dir").Output()
	if err != nil {
		return "", nil
	}
	gitDirStr := strings.TrimSpace(string(gitDir))
	commonDirStr := strings.TrimSpace(string(commonDir))
	if gitDirStr == commonDirStr {
		return "", nil // main worktree or bare repo
	}
	// In a linked worktree, --git-common-dir is an absolute path to the main .git.
	if !filepath.IsAbs(commonDirStr) {
		return "", nil
	}
	return filepath.Dir(commonDirStr), nil
}

func CurrentBranch(repoRoot string) (string, error) {
	out, err := exec.Command("git", "-C", repoRoot, "symbolic-ref", "--short", "HEAD").Output()
	if err != nil {
		// Detached HEAD – return the short commit hash instead.
		out, err = exec.Command("git", "-C", repoRoot, "rev-parse", "--short", "HEAD").Output()
		if err != nil {
			return "", fmt.Errorf("determining current branch: %w", err)
		}
	}
	return strings.TrimSpace(string(out)), nil
}

func BranchExists(repoRoot, branch string) (bool, error) {
	err := exec.Command("git", "-C", repoRoot, "show-ref", "--verify", "--quiet",
		"refs/heads/"+branch).Run()
	if err == nil {
		return true, nil
	}
	// Exit code 1 means the ref does not exist.
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("checking branch %q: %w", branch, err)
}

func RemoveWorktree(repoRoot, worktreePath string) error {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "remove", "--force", worktreePath)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git worktree remove: %w\n%s", err, strings.TrimSpace(errBuf.String()))
	}
	return nil
}

func PruneWorktrees(repoRoot string) error {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "prune")
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git worktree prune: %w\n%s", err, strings.TrimSpace(errBuf.String()))
	}
	return nil
}

func CreateBranch(repoRoot, branch, from string) error {
	args := []string{"-C", repoRoot, "branch", branch}
	if from != "" {
		args = append(args, from)
	}
	cmd := exec.Command("git", args...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git branch: %w\n%s", err, strings.TrimSpace(errBuf.String()))
	}
	return nil
}

func UnmergedCommits(repoRoot, branch string, limit int) ([]string, error) {
	args := []string{
		"-C", repoRoot, "log",
		branch,
		"--not",
		"--exclude=refs/heads/" + branch,
		"--branches",
		"--oneline",
		fmt.Sprintf("--max-count=%d", limit),
	}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("checking unmerged commits on %q: %w", branch, err)
	}
	text := strings.TrimSpace(string(out))
	if text == "" {
		return nil, nil
	}
	return strings.Split(text, "\n"), nil
}

func UnmergedCommitCount(repoRoot, branch string) (int, error) {
	args := []string{
		"-C", repoRoot, "rev-list", "--count",
		branch,
		"--not",
		"--exclude=refs/heads/" + branch,
		"--branches",
	}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return 0, fmt.Errorf("counting unmerged commits on %q: %w", branch, err)
	}
	var n int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &n); err != nil {
		return 0, fmt.Errorf("parsing rev-list --count output: %w", err)
	}
	return n, nil
}

func CheckIgnored(repoRoot string, paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	cmd := exec.Command("git", "-C", repoRoot, "check-ignore", "--stdin")
	cmd.Stdin = strings.NewReader(strings.Join(paths, "\n") + "\n")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		// Exit 0: at least one path is ignored.
		return parseLines(stdout.String()), nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		// Exit 1: none of the paths are ignored.
		return nil, nil
	}
	return nil, fmt.Errorf("git check-ignore: %s: %w", strings.TrimSpace(stderr.String()), err)
}

func parseLines(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// Populates the worktree index via `git read-tree HEAD` so that `git status`
// reports no untracked files even though overlayfs makes all source files visible.
func AddWorktreeNoCheckout(repoRoot, worktreePath, branch string) error {
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		return fmt.Errorf("creating worktree path %s: %w", worktreePath, err)
	}
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "add", "--no-checkout", worktreePath, branch)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git worktree add --no-checkout: %w\n%s", err, strings.TrimSpace(errBuf.String()))
	}
	// Populate the index so it matches HEAD. Without this the worktree index is
	// empty and `git status` reports every file visible via overlayfs as
	// untracked.
	var rtBuf bytes.Buffer
	rtCmd := exec.Command("git", "-C", worktreePath, "read-tree", "HEAD")
	rtCmd.Stderr = &rtBuf
	if err := rtCmd.Run(); err != nil {
		return fmt.Errorf("git read-tree HEAD: %w\n%s", err, strings.TrimSpace(rtBuf.String()))
	}
	return nil
}

// MoveWorktree relocates a git worktree from worktreePath to newPath.
func MoveWorktree(repoRoot, worktreePath, newPath string) error {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "move", worktreePath, newPath)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git worktree move: %w\n%s", err, strings.TrimSpace(errBuf.String()))
	}
	return nil
}

// AddWorktreeForward runs `git worktree add --no-checkout <worktreePath> <args...>`
// and then populates the index via `git read-tree HEAD`. The caller's args are
// appended after the path, allowing flags like `-b <branch>` to override the
// default branch while orktree retains control of --no-checkout and the path.
func AddWorktreeForward(repoRoot, worktreePath string, args []string) error {
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		return fmt.Errorf("creating worktree path %s: %w", worktreePath, err)
	}
	cmdArgs := append([]string{"-C", repoRoot, "worktree", "add", "--no-checkout", worktreePath}, args...)
	cmd := exec.Command("git", cmdArgs...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git worktree add: %w\n%s", err, strings.TrimSpace(errBuf.String()))
	}
	var rtBuf bytes.Buffer
	rtCmd := exec.Command("git", "-C", worktreePath, "read-tree", "HEAD")
	rtCmd.Stderr = &rtBuf
	if err := rtCmd.Run(); err != nil {
		return fmt.Errorf("git read-tree HEAD: %w\n%s", err, strings.TrimSpace(rtBuf.String()))
	}
	return nil
}
