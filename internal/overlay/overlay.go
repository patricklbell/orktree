// Package overlay manages overlayfs mounts for janus worktrees.
package overlay

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Create creates the upper/work/merged directories under dataDir/workspaceID.
func Create(upper, work, merged string) error {
	for _, dir := range []string{upper, work, merged} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating overlay dir %s: %w", dir, err)
		}
	}
	return nil
}

// Mount mounts an overlayfs with the given source as lowerdir.
// Requires CAP_SYS_ADMIN (run janus with sudo or via a privileged helper).
func Mount(source, upper, work, merged string) error {
	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", source, upper, work)
	cmd := exec.Command("mount", "-t", "overlay", "overlay", "-o", opts, merged)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mounting overlay: %w", err)
	}
	return nil
}

// Unmount unmounts the merged directory.
func Unmount(merged string) error {
	cmd := exec.Command("umount", merged)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("unmounting overlay %s: %w", merged, err)
	}
	return nil
}

// IsMounted returns true if merged is currently an overlay mount point.
func IsMounted(merged string) (bool, error) {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false, fmt.Errorf("reading /proc/mounts: %w", err)
	}
	// Each line: <device> <mountpoint> <type> <options> ...
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[2] == "overlay" && fields[1] == merged {
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
