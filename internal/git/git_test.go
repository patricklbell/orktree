package git

import (
	"os"
	"os/exec"
	"path/filepath"
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

func TestCheckIgnored(t *testing.T) {
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
	os.WriteFile(repo+"/.gitignore", []byte("*.log\nbuild/\n"), 0o644)

	ignored, err := CheckIgnored(repo, []string{"main.go", "debug.log", "build/out.bin", "README.md"})
	if err != nil {
		t.Fatalf("CheckIgnored: %v", err)
	}
	want := map[string]bool{"debug.log": true, "build/out.bin": true}
	if len(ignored) != len(want) {
		t.Fatalf("expected %d ignored paths, got %v", len(want), ignored)
	}
	for _, p := range ignored {
		if !want[p] {
			t.Errorf("unexpected ignored path: %q", p)
		}
	}
}

func TestCheckIgnored_emptyPaths(t *testing.T) {
	ignored, err := CheckIgnored("/tmp", nil)
	if err != nil {
		t.Fatalf("CheckIgnored: %v", err)
	}
	if ignored != nil {
		t.Errorf("expected nil, got %v", ignored)
	}
}

func TestCheckIgnored_noneIgnored(t *testing.T) {
	repo := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main", repo)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %s", out)
	}

	ignored, err := CheckIgnored(repo, []string{"main.go", "README.md"})
	if err != nil {
		t.Fatalf("CheckIgnored: %v", err)
	}
	if ignored != nil {
		t.Errorf("expected nil, got %v", ignored)
	}
}

func TestMoveWorktree(t *testing.T) {
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

	// Create a branch and worktree.
	if err := CreateBranch(repo, "feat", ""); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	oldPath := filepath.Join(t.TempDir(), "old-wt")
	if err := AddWorktreeNoCheckout(repo, oldPath, "feat"); err != nil {
		t.Fatalf("AddWorktreeNoCheckout: %v", err)
	}

	newPath := filepath.Join(t.TempDir(), "new-wt")
	if err := MoveWorktree(repo, oldPath, newPath); err != nil {
		t.Fatalf("MoveWorktree: %v", err)
	}

	// Old path should not exist, new path should.
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("old worktree path still exists after move")
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Errorf("new worktree path does not exist: %v", err)
	}
}
func TestMainWorktreeRoot(t *testing.T) {
	repo := t.TempDir()
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test",
	)
	for _, args := range [][]string{
		{"git", "init", "-b", "main", repo},
		{"git", "-C", repo, "commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", args, out)
		}
	}

	wtPath := filepath.Join(t.TempDir(), "my-worktree")
	if err := CreateBranch(repo, "feat", ""); err != nil {
		t.Fatalf("CreateBranch feat: %v", err)
	}
	if err := AddWorktreeNoCheckout(repo, wtPath, "feat"); err != nil {
		t.Fatalf("AddWorktreeNoCheckout: %v", err)
	}

	t.Run("returns_empty_from_main_worktree", func(t *testing.T) {
		root, err := MainWorktreeRoot(repo)
		if err != nil {
			t.Fatalf("MainWorktreeRoot: %v", err)
		}
		if root != "" {
			t.Errorf("expected empty string from main worktree, got %q", root)
		}
	})

	t.Run("returns_main_root_from_linked_worktree", func(t *testing.T) {
		root, err := MainWorktreeRoot(wtPath)
		if err != nil {
			t.Fatalf("MainWorktreeRoot: %v", err)
		}
		if root != repo {
			t.Errorf("MainWorktreeRoot = %q, want %q", root, repo)
		}
	})

	t.Run("returns_main_root_from_subdirectory_inside_linked_worktree", func(t *testing.T) {
		sub := filepath.Join(wtPath, "some", "subdir")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		root, err := MainWorktreeRoot(sub)
		if err != nil {
			t.Fatalf("MainWorktreeRoot: %v", err)
		}
		if root != repo {
			t.Errorf("MainWorktreeRoot = %q, want %q", root, repo)
		}
	})
}
