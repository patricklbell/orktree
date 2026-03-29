package main

import (
	"testing"
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
