package overlay

import (
	"bufio"
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

// fuseConfPath is the path to the FUSE configuration file.
// Tests override this to avoid depending on the system config.
var fuseConfPath = "/etc/fuse.conf"

// UserAllowOther reports whether /etc/fuse.conf has an uncommented
// user_allow_other directive, which permits non-root FUSE mounts
// to be accessed by other users (including the Docker daemon).
func UserAllowOther() bool {
	f, err := os.Open(fuseConfPath)
	if err != nil {
		return false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}
<<<<<<< HEAD
		if line == "user_allow_other" {
=======
		if strings.Contains(line, "user_allow_other") {
>>>>>>> 5342122 (cmd/switch: improve tty hint to reference docs instead of repo paths)
			return true
		}
	}
	return false
}

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
	if UserAllowOther() {
		opts += ",allow_other"
	}
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

// getOverlayXattr reads an overlayfs xattr, checking both the trusted.overlay.
// and user.overlay. namespaces (the latter is used by unprivileged fuse-overlayfs).
func getOverlayXattr(path, name string) (string, bool) {
	for _, prefix := range []string{"trusted.overlay.", "user.overlay."} {
		sz, err := syscall.Getxattr(path, prefix+name, nil)
		if err != nil || sz <= 0 {
			continue
		}
		buf := make([]byte, sz)
		sz, err = syscall.Getxattr(path, prefix+name, buf)
		if err != nil {
			continue
		}
		return string(buf[:sz]), true
	}
	return "", false
}

// isWhiteout reports whether a file in the overlay upper directory is a
// deletion marker (whiteout).  Three formats are recognised:
//   - character device with 0/0 device number (kernel overlayfs)
//   - zero-size regular file with the trusted.overlay.whiteout (or
//     user.overlay.whiteout) xattr (kernel overlayfs ≥ 5.8)
//   - zero-size regular file whose name starts with .wh.
//     (fuse-overlayfs naming convention; requiring zero-size avoids
//     misidentifying user files that happen to start with .wh.)
func isWhiteout(path string, info os.FileInfo) bool {
	if info.Mode()&os.ModeCharDevice != 0 {
		if st, ok := info.Sys().(*syscall.Stat_t); ok && st.Rdev == 0 {
			return true
		}
	}
	if info.Mode().IsRegular() && info.Size() == 0 {
		if _, ok := getOverlayXattr(path, "whiteout"); ok {
			return true
		}
	}
	// fuse-overlayfs whiteouts are always zero-size regular files.
	if info.Mode().IsRegular() && info.Size() == 0 && strings.HasPrefix(filepath.Base(path), ".wh.") {
		return true
	}
	return false
}

// isOpaqueDir reports whether a directory in the overlay upper directory
// replaces (rather than merges with) the corresponding directory in the lower
// layer.  Two formats are recognised:
//   - trusted.overlay.opaque (or user.overlay.opaque) xattr set to "y"
//     (kernel overlayfs)
//   - a zero-size .wh..wh..opq sentinel file inside the directory
//     (fuse-overlayfs; requiring zero-size avoids misidentifying a
//     user-created file with the same name)
func isOpaqueDir(path string) bool {
	if val, ok := getOverlayXattr(path, "opaque"); ok && val == "y" {
		return true
	}
	sentinel := filepath.Join(path, ".wh..wh..opq")
	if fi, err := os.Lstat(sentinel); err == nil && fi.Mode().IsRegular() && fi.Size() == 0 {
		return true
	}
	return false
}

func isInsideOpaqueDir(rel string, opaqueDirs []string) bool {
	for _, od := range opaqueDirs {
		if strings.HasPrefix(rel, od+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// Returns relative paths of files that differ from the corresponding
// file in lower.  Overlayfs internal metadata — whiteouts (deletion markers)
// and opaque directory sentinels — are excluded from the result.  Files that
// were copied up by the overlay but reverted to identical content are also
// excluded.  Only file content is compared; permission or mode changes alone
// are not detected.
//
// Opaque directories (where the upper completely replaces the lower) are
// detected via xattr (trusted.overlay.opaque / user.overlay.opaque = "y") or
// the .wh..wh..opq sentinel file.  All files inside an opaque directory are
// reported as dirty regardless of whether a matching file exists in lower,
// because the lower version is masked.
//
// When limit > 0, at most limit paths are collected in files, but the walk
// continues so that total reflects the true number of dirty files.
// When limit <= 0, all dirty paths are collected and total == len(files).
func DirtyUpperFiles(upper, lower string, limit int) (files []string, total int, err error) {
	var opaqueDirs []string

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

		rel, _ := filepath.Rel(upper, path)

		if info.IsDir() {
			if rel != "." && isOpaqueDir(path) {
				opaqueDirs = append(opaqueDirs, rel)
			}
			return nil
		}

		// Skip overlay internal metadata.
		if base == ".wh..wh..opq" {
			return nil
		}
		if isWhiteout(path, info) {
			return nil
		}

		isDirty := false

		// Files inside an opaque directory are always dirty — the lower
		// version of the parent directory is completely masked.
		if isInsideOpaqueDir(rel, opaqueDirs) {
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
