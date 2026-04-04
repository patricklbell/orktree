package overlay_test

import (
	"errors"
	"testing"

	"github.com/patricklbell/orktree/internal/overlay"
)

func TestCheckAvailable(t *testing.T) {
	// fuse-overlayfs may or may not be installed in the test environment.
	// We only check that the function returns either nil or ErrNotAvailable.
	err := overlay.CheckAvailable()
	if err != nil && !errors.Is(err, overlay.ErrNotAvailable) {
		t.Fatalf("CheckAvailable returned unexpected error: %v", err)
	}
}
