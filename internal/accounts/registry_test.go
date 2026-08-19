package accounts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadRoundTripPlaintextAccountRegistry(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "opencode-accounts.yaml")
	want := []Account{{APIKey: "parent-secret", ProviderName: "Opencode-go", Label: "go-parent"}, {
		APIKey:           "sk-go-one",
		ProviderName:     "Opencode-go",
		Label:            "go-01",
		Email:            "go01@example.com",
		Password:         "disposable-password",
		ReferralURL:      "https://opencode.ai/go?ref=ABC123",
		ReferredByAPIKey: "parent-secret",
		ReferralOnly:     true,
		ExpiresAt:        "2026-09-17",
		Notes:            "Disposable account",
	}}

	if err := Save(path, want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("registry permissions = %o, want 600", perm)
	}
}

func TestLoadMissingRegistryReturnsEmpty(t *testing.T) {
	t.Parallel()

	got, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Load() = %#v, want empty", got)
	}
}

func TestLoadEmptyRegistryReturnsEmpty(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "empty.yaml")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Load() = %#v, want empty", got)
	}
}

func TestSaveRejectsInvalidExpiryDate(t *testing.T) {
	t.Parallel()

	err := Save(filepath.Join(t.TempDir(), "accounts.yaml"), []Account{{APIKey: "key", ExpiresAt: "never"}})
	if err == nil {
		t.Fatal("Save() error = nil, want invalid expires_at rejection")
	}
}

func TestSaveRejectsDuplicateAPIKeys(t *testing.T) {
	t.Parallel()

	err := Save(filepath.Join(t.TempDir(), "accounts.yaml"), []Account{
		{APIKey: "same-key", Label: "one"},
		{APIKey: "same-key", Label: "two"},
	})
	if err == nil {
		t.Fatal("Save() error = nil, want duplicate-key error")
	}
}
