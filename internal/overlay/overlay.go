package overlay

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func Create(upper, work, merged string) error {
	for _, dir := range []string{upper, work, merged} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating overlay dir %s: %w", dir, err)
		}
	}
	return nil
}

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

func Unmount(merged string) error {
	var errBuf bytes.Buffer
	cmd := exec.Command("fusermount", "-u", merged)
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		// Lazy unmount detaches immediately; kernel cleans up
		// once the last reference (e.g. a shell cwd) drops.
		var lazyBuf bytes.Buffer
		lazyCmd := exec.Command("fusermount", "-uz", merged)
		lazyCmd.Stderr = &lazyBuf
		if lazyErr := lazyCmd.Run(); lazyErr != nil {
			return fmt.Errorf("unmounting overlay %s: %s (lazy: %v): %w", merged, strings.TrimSpace(errBuf.String()), lazyErr, err)
		}
	}
	return nil
}

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

// Reports whether the path represents overlayfs internal metadata for file deletion or directory opacity.
func IsOverlayWhiteout(relPath string) bool {
	return strings.HasPrefix(filepath.Base(relPath), ".wh.")
}

// Returns relative paths of files that genuinely differ from the corresponding file in lower.
// Files that were copied up by the overlay but reverted to identical content
// are excluded.  Overlayfs whiteout markers (deletions) are always reported.
// Only file content is compared; permission or mode changes alone are not detected.
//
// When limit > 0, at most limit paths are collected in files, but the walk
// continues so that total reflects the true number of dirty files.
// When limit <= 0, all dirty paths are collected and total == len(files).
func DirtyUpperFiles(upper, lower string, limit int) (files []string, total int, err error) {
	err = filepath.Walk(upper, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		base := filepath.Base(path)
		if base == ".git" {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}

		rel, _ := filepath.Rel(upper, path)

		isDirty := false

		// Overlayfs whiteout: character device with rdev 0.
		if info.Mode()&os.ModeCharDevice != 0 {
			if st, ok := info.Sys().(*syscall.Stat_t); ok && st.Rdev == 0 {
				isDirty = true
			}
		}

		// Opaque directory marker.
		if !isDirty && base == ".wh..wh..opq" {
			isDirty = true
		}

		// Compare with lower.
		if !isDirty {
			lowerPath := filepath.Join(lower, rel)
			equal, err := filesEqual(path, lowerPath)
			if err != nil || !equal {
				isDirty = true
			}
		}

		if isDirty {
			total++
			if limit <= 0 || len(files) < limit {
				files = append(files, rel)
			}
		}
		return nil
	})
	if err != nil {
		return nil, 0, fmt.Errorf("walking upper dir: %w", err)
	}
	return files, total, nil
}

func filesEqual(a, b string) (bool, error) {
	infoA, err := os.Lstat(a)
	if err != nil {
		return false, err
	}
	infoB, err := os.Lstat(b)
	if err != nil {
		return false, err
	}
	if infoA.Size() != infoB.Size() {
		return false, nil
	}
	fa, err := os.Open(a)
	if err != nil {
		return false, err
	}
	defer fa.Close()
	fb, err := os.Open(b)
	if err != nil {
		return false, err
	}
	defer fb.Close()

	const chunk = 32 * 1024
	bufA := make([]byte, chunk)
	bufB := make([]byte, chunk)
	for {
		nA, errA := io.ReadFull(fa, bufA)
		nB, errB := io.ReadFull(fb, bufB)
		if !bytes.Equal(bufA[:nA], bufB[:nB]) {
			return false, nil
		}
		if errors.Is(errA, io.EOF) && errors.Is(errB, io.EOF) {
			return true, nil
		}
		if errors.Is(errA, io.ErrUnexpectedEOF) && errors.Is(errB, io.ErrUnexpectedEOF) {
			return true, nil
		}
		if errA != nil || errB != nil {
			return false, nil
		}
	}
}
