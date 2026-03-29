package git

import (
	"os"
	"os/exec"
	"testing"
)

func TestBranchExists_falseForMissing(t *testing.T) {
	repo := t.TempDir()
	cmd := exec.Command("git", "init", repo)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	exists, err := BranchExists(repo, "nonexistent-branch-xyz")
	if err != nil {
		t.Fatalf("BranchExists: %v", err)
	}
	if exists {
		t.Error("expected BranchExists to return false for a missing branch")
	}
}

func TestCurrentBranch(t *testing.T) {
	repo := t.TempDir()
	for _, args := range [][]string{
		{"git", "init", "-b", "main", repo},
		{"git", "-C", repo, "commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", args, out)
		}
	}

	branch, err := CurrentBranch(repo)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch != "main" {
		t.Errorf("CurrentBranch = %q, want %q", branch, "main")
	}
}
