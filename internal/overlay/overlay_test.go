package overlay

import (
	"os"
	"path/filepath"
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
