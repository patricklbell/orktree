// Package git provides helpers for git worktree operations used by janus.
package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// IsGitRepo returns true if the given path is inside a git repository.
func IsGitRepo(path string) bool {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--git-dir")
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// CurrentBranch returns the name of the currently checked-out branch.
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

// BranchExists reports whether a local branch with the given name exists.
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

// AddWorktree creates a git worktree at worktreePath on branch.
//
//   - If newBranch is true a new branch is created from from (or HEAD when from
//     is empty).
//   - If newBranch is false the branch must already exist.
func AddWorktree(repoRoot, worktreePath, branch string, newBranch bool, from string) error {
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		return fmt.Errorf("creating worktree path %s: %w", worktreePath, err)
	}

	var args []string
	if newBranch {
		args = []string{"-C", repoRoot, "worktree", "add", "-b", branch, worktreePath}
		if from != "" {
			args = append(args, from)
		}
	} else {
		args = []string{"-C", repoRoot, "worktree", "add", worktreePath, branch}
	}

	cmd := exec.Command("git", args...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git worktree add: %w\n%s", err, strings.TrimSpace(errBuf.String()))
	}
	return nil
}

// RemoveWorktree removes the git worktree rooted at worktreePath.
func RemoveWorktree(repoRoot, worktreePath string) error {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "remove", "--force", worktreePath)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git worktree remove: %w\n%s", err, strings.TrimSpace(errBuf.String()))
	}
	return nil
}

// PruneWorktrees runs `git worktree prune` to clean up stale entries.
func PruneWorktrees(repoRoot string) error {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "prune")
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git worktree prune: %w\n%s", err, strings.TrimSpace(errBuf.String()))
	}
	return nil
}
