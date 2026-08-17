package accounts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRepairsInsecureRegistryPermissions(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "accounts.yaml")
	if err := os.WriteFile(path, []byte("accounts: []\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("registry permissions = %o, want 600", perm)
	}
}
