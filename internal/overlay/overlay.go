// Package overlay manages overlayfs mounts for orktrees.
//
// Mounts are performed with fuse-overlayfs, a userspace implementation of
// overlayfs that requires no elevated privileges — only /dev/fuse access,
// which is granted by being in the fuse group (see orktree check).
package overlay

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Create creates the upper/work/merged directories for the worktree.
func Create(upper, work, merged string) error {
	for _, dir := range []string{upper, work, merged} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating overlay dir %s: %w", dir, err)
		}
	}
	return nil
}

// Mount mounts a fuse-overlayfs with the given source as lowerdir.
// Requires fuse-overlayfs to be installed and /dev/fuse to be accessible
// (add yourself to the fuse group — see orktree check).
func Mount(source, upper, work, merged string) error {
	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", source, upper, work)
	cmd := exec.Command("fuse-overlayfs", "-o", opts, merged)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mounting overlay: %s: %w", strings.TrimSpace(errBuf.String()), err)
	}
	return nil
}

// Unmount unmounts the merged directory.
func Unmount(merged string) error {
	var errBuf bytes.Buffer
	cmd := exec.Command("fusermount", "-u", merged)
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		// Lazy unmount detaches the mount immediately; the kernel cleans up
		// once the last reference (e.g. a shell whose cwd is inside) drops.
		var lazyBuf bytes.Buffer
		lazyCmd := exec.Command("fusermount", "-uz", merged)
		lazyCmd.Stderr = &lazyBuf
		if lazyErr := lazyCmd.Run(); lazyErr != nil {
			return fmt.Errorf("unmounting overlay %s: %s (lazy: %v): %w", merged, strings.TrimSpace(errBuf.String()), lazyErr, err)
		}
	}
	return nil
}

// IsMounted returns true if merged is currently a fuse-overlayfs (or kernel
// overlayfs) mount point.
func IsMounted(merged string) (bool, error) {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false, fmt.Errorf("reading /proc/mounts: %w", err)
	}
	// Each line: <device> <mountpoint> <type> <options> ...
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[1] != merged {
			continue
		}
		if fields[2] == "overlay" || fields[2] == "fuse.fuse-overlayfs" {
			return true, nil
		}
	}
	return false, nil
}

// EnsureMounted mounts the overlay if it is not already mounted.
func EnsureMounted(source, upper, work, merged string) error {
	ok, err := IsMounted(merged)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	return Mount(source, upper, work, merged)
}

// Remove unmounts (if mounted) and deletes the workspace overlay directories.
func Remove(upper, work, merged string) error {
	mounted, err := IsMounted(merged)
	if err != nil {
		return err
	}
	if mounted {
		if err := Unmount(merged); err != nil {
			return err
		}
	}
	// upper is <base>/upper → parent is <base>.
	parent := filepath.Dir(upper)
	if parent == "." || parent == "" {
		return errors.New("could not determine overlay parent directory")
	}
	return os.RemoveAll(parent)
}
