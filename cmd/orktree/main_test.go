package main

import (
	"strings"
	"testing"

	"github.com/patricklbell/orktree/pkg/orktree"
)

func TestHelp(t *testing.T) {
	err := run([]string{"help"})
	if err != nil {
		t.Fatalf("help: %v", err)
	}
}

func TestUnknownCommand(t *testing.T) {
	err := run([]string{"bogus"})
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
}

func TestHumanSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0B"},
		{512, "512B"},
		{1023, "1023B"},
		{1024, "1.0K"},
		{1536, "1.5K"},
		{1048576, "1.0M"},
		{1073741824, "1.0G"},
	}
	for _, tt := range tests {
		got := humanSize(tt.bytes)
		if got != tt.want {
			t.Errorf("humanSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestIsTerminal(t *testing.T) {
	// Verify isTerminal doesn't panic on common file descriptors.
	// In CI, none of these are typically a TTY.
	isTerminal(0) // stdin
	isTerminal(1) // stdout
	isTerminal(2) // stderr
}

func TestFormatAssessment_empty(t *testing.T) {
	rc := &orktree.RemoveCheck{Branch: "test", MergedPath: "/tmp/test"}
	got := formatAssessment(rc)
	if got != "" {
		t.Errorf("expected empty string for clean check, got %q", got)
	}
}

func TestFormatAssessment_allSections(t *testing.T) {
	rc := &orktree.RemoveCheck{
		Branch:          "feat",
		MergedPath:      "/tmp/feat",
		UnmergedCommits: []string{"abc1234 add parser", "def5678 fix lexer"},
		UnmergedTotal:   2,
		TrackedDirty:    []string{"src/parser.go", "src/lexer.go"},
		TrackedTotal:    2,
		UntrackedDirty:  []string{"notes.txt"},
		UntrackedTotal:  1,
		IgnoredDirty:    847,
	}
	got := formatAssessment(rc)

	for _, want := range []string{
		"Commits only on this branch:",
		"abc1234 add parser",
		"def5678 fix lexer",
		"Modified tracked files:",
		"src/parser.go",
		"Untracked files:",
		"notes.txt",
		"Ignored files: 847 files",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

func TestFormatAssessment_overflow(t *testing.T) {
	commits := make([]string, 10)
	for i := range commits {
		commits[i] = "abc1234 commit"
	}
	rc := &orktree.RemoveCheck{
		Branch:          "feat",
		MergedPath:      "/tmp/feat",
		UnmergedCommits: commits,
		UnmergedTotal:   15,
	}
	got := formatAssessment(rc)
	if !strings.Contains(got, "... and 5 more") {
		t.Errorf("expected overflow message, got:\n%s", got)
	}
}

func TestFormatAssessment_onlyIgnored(t *testing.T) {
	rc := &orktree.RemoveCheck{
		Branch:       "feat",
		MergedPath:   "/tmp/feat",
		IgnoredDirty: 42,
	}
	got := formatAssessment(rc)
	if !strings.Contains(got, "Ignored files: 42 files") {
		t.Errorf("expected ignored section, got:\n%s", got)
	}
	if strings.Contains(got, "Commits") || strings.Contains(got, "Modified") || strings.Contains(got, "Untracked") {
		t.Errorf("unexpected section in output:\n%s", got)
	}
}

func TestFormatAssessment_sectionSeparation(t *testing.T) {
	rc := &orktree.RemoveCheck{
		Branch:         "feat",
		MergedPath:     "/tmp/feat",
		TrackedDirty:   []string{"a.go"},
		TrackedTotal:   1,
		UntrackedDirty: []string{"b.txt"},
		UntrackedTotal: 1,
	}
	got := formatAssessment(rc)
	// Sections should be separated by a blank line.
	if !strings.Contains(got, "\n\n") {
		t.Errorf("expected blank line between sections, got:\n%s", got)
	}
}
