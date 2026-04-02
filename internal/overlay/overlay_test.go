package overlay

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestCreate_createsDirectories(t *testing.T) {
	base := t.TempDir()
	upper := filepath.Join(base, "upper")
	work := filepath.Join(base, "work")
	merged := filepath.Join(base, "merged")

	if err := Create(upper, work, merged); err != nil {
		t.Fatalf("Create: %v", err)
	}

	for _, dir := range []string{upper, work, merged} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("directory %s not created: %v", dir, err)
		} else if !info.IsDir() {
			t.Errorf("%s is not a directory", dir)
		}
	}
}

func TestIsMounted_falseForPlainDir(t *testing.T) {
	dir := t.TempDir()

	mounted, err := IsMounted(dir)
	if err != nil {
		t.Fatalf("IsMounted: %v", err)
	}
	if mounted {
		t.Error("expected IsMounted to return false for a plain directory")
	}
}

func TestDirtyUpperFiles_identicalFileNotDirty(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()
	content := []byte("hello world")
	os.WriteFile(filepath.Join(upper, "file.txt"), content, 0o644)
	os.WriteFile(filepath.Join(lower, "file.txt"), content, 0o644)

	dirty, total, err := DirtyUpperFiles(upper, lower, 100)
	if err != nil {
		t.Fatalf("DirtyUpperFiles: %v", err)
	}
	if len(dirty) != 0 {
		t.Errorf("expected no dirty files, got %v", dirty)
	}
	if total != 0 {
		t.Errorf("expected total 0, got %d", total)
	}
}

func TestDirtyUpperFiles_differentFileIsDirty(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()
	os.WriteFile(filepath.Join(upper, "file.txt"), []byte("modified"), 0o644)
	os.WriteFile(filepath.Join(lower, "file.txt"), []byte("original"), 0o644)

	dirty, total, err := DirtyUpperFiles(upper, lower, 100)
	if err != nil {
		t.Fatalf("DirtyUpperFiles: %v", err)
	}
	if len(dirty) != 1 || dirty[0] != "file.txt" {
		t.Errorf("expected [file.txt], got %v", dirty)
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
}

func TestDirtyUpperFiles_newFileIsDirty(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()
	os.WriteFile(filepath.Join(upper, "newfile.txt"), []byte("new"), 0o644)

	dirty, total, err := DirtyUpperFiles(upper, lower, 100)
	if err != nil {
		t.Fatalf("DirtyUpperFiles: %v", err)
	}
	if len(dirty) != 1 || dirty[0] != "newfile.txt" {
		t.Errorf("expected [newfile.txt], got %v", dirty)
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
}

func TestDirtyUpperFiles_gitDirSkipped(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()
	gitDir := filepath.Join(upper, ".git")
	os.MkdirAll(gitDir, 0o755)
	os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main"), 0o644)

	dirty, total, err := DirtyUpperFiles(upper, lower, 100)
	if err != nil {
		t.Fatalf("DirtyUpperFiles: %v", err)
	}
	if len(dirty) != 0 {
		t.Errorf("expected no dirty files, got %v", dirty)
	}
	if total != 0 {
		t.Errorf("expected total 0, got %d", total)
	}
}

func TestDirtyUpperFiles_limitRespected(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("file%d.txt", i)
		os.WriteFile(filepath.Join(upper, name), []byte("new"), 0o644)
	}

	dirty, total, err := DirtyUpperFiles(upper, lower, 3)
	if err != nil {
		t.Fatalf("DirtyUpperFiles: %v", err)
	}
	if len(dirty) != 3 {
		t.Errorf("expected 3 dirty files, got %d", len(dirty))
	}
	if total != 10 {
		t.Errorf("expected total 10, got %d", total)
	}
}

func TestDirtyUpperFiles_emptyUpper(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()

	dirty, total, err := DirtyUpperFiles(upper, lower, 100)
	if err != nil {
		t.Fatalf("DirtyUpperFiles: %v", err)
	}
	if len(dirty) != 0 {
		t.Errorf("expected no dirty files, got %v", dirty)
	}
	if total != 0 {
		t.Errorf("expected total 0, got %d", total)
	}
}

func TestUserAllowOther(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"uncommented", "user_allow_other\n", true},
		{"commented", "#user_allow_other\n", false},
		{"commented with space", "# user_allow_other\n", false},
		{"empty file", "", false},
		{"mixed lines", "# some comment\nuser_allow_other\n", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			path := filepath.Join(tmp, "fuse.conf")
			os.WriteFile(path, []byte(tt.content), 0o644)
			fuseConfPath = path
			defer func() { fuseConfPath = "/etc/fuse.conf" }()
			if got := UserAllowOther(); got != tt.want {
				t.Errorf("UserAllowOther() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserAllowOther_missingFile(t *testing.T) {
	orig := fuseConfPath
	fuseConfPath = "/nonexistent/fuse.conf"
	defer func() { fuseConfPath = orig }()
	if UserAllowOther() {
		t.Error("UserAllowOther() = true for missing file, want false")
	}
}

func TestDirtyUpperFiles_sameSizeDifferentContent(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()
	os.WriteFile(filepath.Join(upper, "file.txt"), []byte("aaaa"), 0o644)
	os.WriteFile(filepath.Join(lower, "file.txt"), []byte("bbbb"), 0o644)

	dirty, total, err := DirtyUpperFiles(upper, lower, 100)
	if err != nil {
		t.Fatalf("DirtyUpperFiles: %v", err)
	}
	if len(dirty) != 1 || dirty[0] != "file.txt" {
		t.Errorf("expected [file.txt], got %v", dirty)
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
}

func TestDirtyUpperFiles_totalAccurateWhenLimited(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("dirty%d.txt", i)
		os.WriteFile(filepath.Join(upper, name), []byte("new"), 0o644)
	}

	dirty, total, err := DirtyUpperFiles(upper, lower, 3)
	if err != nil {
		t.Fatalf("DirtyUpperFiles: %v", err)
	}
	if len(dirty) != 3 {
		t.Errorf("expected 3 files in slice, got %d", len(dirty))
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
}

func TestDirtyUpperFiles_noLimitCollectsAll(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()
	for i := 0; i < 7; i++ {
		name := fmt.Sprintf("file%d.txt", i)
		os.WriteFile(filepath.Join(upper, name), []byte("new"), 0o644)
	}

	dirty, total, err := DirtyUpperFiles(upper, lower, 0)
	if err != nil {
		t.Fatalf("DirtyUpperFiles: %v", err)
	}
	if len(dirty) != 7 {
		t.Errorf("expected 7 files, got %d", len(dirty))
	}
	if total != 7 {
		t.Errorf("expected total 7, got %d", total)
	}
}

func TestDirtyUpperFiles_whiteoutByNameExcluded(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()
	// .wh.<name> is the fuse-overlayfs naming convention for whiteouts (zero-size).
	os.WriteFile(filepath.Join(upper, ".wh.deleted.txt"), []byte{}, 0o644)
	os.WriteFile(filepath.Join(upper, "real.txt"), []byte("content"), 0o644)

	dirty, total, err := DirtyUpperFiles(upper, lower, 100)
	if err != nil {
		t.Fatalf("DirtyUpperFiles: %v", err)
	}
	if len(dirty) != 1 || dirty[0] != "real.txt" {
		t.Errorf("expected [real.txt], got %v", dirty)
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
}

func TestDirtyUpperFiles_nonEmptyWhPrefixFileNotWhiteout(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()
	// A user file named .wh.something with actual content is not a whiteout.
	os.WriteFile(filepath.Join(upper, ".wh.notes"), []byte("user data"), 0o644)

	dirty, total, err := DirtyUpperFiles(upper, lower, 100)
	if err != nil {
		t.Fatalf("DirtyUpperFiles: %v", err)
	}
	if len(dirty) != 1 || dirty[0] != ".wh.notes" {
		t.Errorf("expected [.wh.notes], got %v", dirty)
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
}

func TestDirtyUpperFiles_opaqueDirSentinelExcluded(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()
	opaqueDir := filepath.Join(upper, "replaced")
	os.MkdirAll(opaqueDir, 0o755)
	// The .wh..wh..opq sentinel itself must not appear in results.
	os.WriteFile(filepath.Join(opaqueDir, ".wh..wh..opq"), []byte{}, 0o644)
	os.WriteFile(filepath.Join(opaqueDir, "new.txt"), []byte("new"), 0o644)

	dirty, total, err := DirtyUpperFiles(upper, lower, 100)
	if err != nil {
		t.Fatalf("DirtyUpperFiles: %v", err)
	}
	// Only new.txt should appear, not .wh..wh..opq.
	if len(dirty) != 1 {
		t.Errorf("expected 1 dirty file, got %v", dirty)
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
}

func TestDirtyUpperFiles_opaqueDirMakesIdenticalFileDirty(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()

	// Lower has replaced/same.txt.
	os.MkdirAll(filepath.Join(lower, "replaced"), 0o755)
	os.WriteFile(filepath.Join(lower, "replaced", "same.txt"), []byte("content"), 0o644)

	// Upper has replaced/ with .wh..wh..opq → opaque dir.
	os.MkdirAll(filepath.Join(upper, "replaced"), 0o755)
	os.WriteFile(filepath.Join(upper, "replaced", ".wh..wh..opq"), []byte{}, 0o644)
	// same.txt has identical content to lower, but the directory is opaque
	// so it should still be reported as dirty.
	os.WriteFile(filepath.Join(upper, "replaced", "same.txt"), []byte("content"), 0o644)

	dirty, total, err := DirtyUpperFiles(upper, lower, 100)
	if err != nil {
		t.Fatalf("DirtyUpperFiles: %v", err)
	}
	if len(dirty) != 1 || dirty[0] != filepath.Join("replaced", "same.txt") {
		t.Errorf("expected [replaced/same.txt], got %v", dirty)
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
}

func TestDirtyUpperFiles_opaqueDirNestedFilesAllDirty(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()

	os.MkdirAll(filepath.Join(upper, "dir", "sub"), 0o755)
	os.WriteFile(filepath.Join(upper, "dir", ".wh..wh..opq"), []byte{}, 0o644)
	os.WriteFile(filepath.Join(upper, "dir", "a.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(upper, "dir", "sub", "b.txt"), []byte("b"), 0o644)

	// Lower has the same files with identical content.
	os.MkdirAll(filepath.Join(lower, "dir", "sub"), 0o755)
	os.WriteFile(filepath.Join(lower, "dir", "a.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(lower, "dir", "sub", "b.txt"), []byte("b"), 0o644)

	dirty, total, err := DirtyUpperFiles(upper, lower, 100)
	if err != nil {
		t.Fatalf("DirtyUpperFiles: %v", err)
	}
	// Both files are inside an opaque directory, so both are dirty
	// even though content matches lower.
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
	if len(dirty) != 2 {
		t.Errorf("expected 2 dirty files, got %v", dirty)
	}
}

// canSetUserXattr checks whether user xattrs are supported on the
// filesystem backing dir (tmpfs, ext4, etc.).
func canSetUserXattr(dir string) bool {
	p := filepath.Join(dir, ".xattr_probe")
	os.WriteFile(p, nil, 0o644)
	defer os.Remove(p)
	err := syscall.Setxattr(p, "user.test", []byte("1"), 0)
	return err == nil
}

func TestDirtyUpperFiles_xattrWhiteoutExcluded(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()
	if !canSetUserXattr(upper) {
		t.Skip("filesystem does not support user xattrs")
	}

	// Create a zero-size file with user.overlay.whiteout xattr.
	whPath := filepath.Join(upper, "gone.txt")
	os.WriteFile(whPath, nil, 0o644)
	if err := syscall.Setxattr(whPath, "user.overlay.whiteout", []byte("y"), 0); err != nil {
		t.Fatalf("setxattr: %v", err)
	}
	os.WriteFile(filepath.Join(upper, "real.txt"), []byte("hello"), 0o644)

	dirty, total, err := DirtyUpperFiles(upper, lower, 100)
	if err != nil {
		t.Fatalf("DirtyUpperFiles: %v", err)
	}
	if len(dirty) != 1 || dirty[0] != "real.txt" {
		t.Errorf("expected [real.txt], got %v", dirty)
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
}

func TestDirtyUpperFiles_xattrOpaqueDirMakesChildrenDirty(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()
	if !canSetUserXattr(upper) {
		t.Skip("filesystem does not support user xattrs")
	}

	opaqueDir := filepath.Join(upper, "replaced")
	os.MkdirAll(opaqueDir, 0o755)
	if err := syscall.Setxattr(opaqueDir, "user.overlay.opaque", []byte("y"), 0); err != nil {
		t.Fatalf("setxattr: %v", err)
	}
	os.WriteFile(filepath.Join(opaqueDir, "file.txt"), []byte("same"), 0o644)

	// Lower has the same file with identical content.
	os.MkdirAll(filepath.Join(lower, "replaced"), 0o755)
	os.WriteFile(filepath.Join(lower, "replaced", "file.txt"), []byte("same"), 0o644)

	dirty, total, err := DirtyUpperFiles(upper, lower, 100)
	if err != nil {
		t.Fatalf("DirtyUpperFiles: %v", err)
	}
	if len(dirty) != 1 || dirty[0] != filepath.Join("replaced", "file.txt") {
		t.Errorf("expected [replaced/file.txt], got %v", dirty)
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
}
