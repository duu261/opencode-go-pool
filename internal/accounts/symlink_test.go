package accounts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRejectsSymlinkWithoutChangingTargetPermissions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "target.yaml")
	link := filepath.Join(dir, "accounts.yaml")
	if err := os.WriteFile(target, []byte("accounts: []\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Chmod(target, 0o644); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	if _, err := Load(link); err == nil {
		t.Fatal("Load() error = nil, want symlink rejection")
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Fatalf("target permissions changed to %o", perm)
	}
}
