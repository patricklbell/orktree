package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestCompletion_bash(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := run([]string{"completion", "bash"})
	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	if err != nil {
		t.Fatalf("completion bash: %v", err)
	}
	if !strings.Contains(out, "_orktree_completion") {
		t.Error("bash completion missing _orktree_completion function")
	}
	if !strings.Contains(out, "complete -F") {
		t.Error("bash completion missing complete -F directive")
	}
}
