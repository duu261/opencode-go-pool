package accounts

import (
	"os"
	"testing"
)

func TestLoadRejectsDirectoryWithoutChangingPermissions(t *testing.T) {
	t.Parallel()

	path := t.TempDir()
	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() before error = %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("Load() error = nil, want non-regular-file error")
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() after error = %v", err)
	}
	if after.Mode().Perm() != before.Mode().Perm() {
		t.Fatalf("directory permissions changed from %o to %o", before.Mode().Perm(), after.Mode().Perm())
	}
}
