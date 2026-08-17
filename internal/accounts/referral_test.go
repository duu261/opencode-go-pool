package accounts

import (
	"path/filepath"
	"testing"
)

func TestSaveRejectsMissingReferralParent(t *testing.T) {
	t.Parallel()

	err := Save(filepath.Join(t.TempDir(), "accounts.yaml"), []Account{{APIKey: "child", ReferredByAPIKey: "missing"}})
	if err == nil {
		t.Fatal("Save() error = nil, want missing-parent error")
	}
}

func TestSaveRejectsSelfReferral(t *testing.T) {
	t.Parallel()

	err := Save(filepath.Join(t.TempDir(), "accounts.yaml"), []Account{{APIKey: "self", ReferredByAPIKey: "self"}})
	if err == nil {
		t.Fatal("Save() error = nil, want self-referral error")
	}
}

func TestSaveAcceptsExistingReferralParent(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "accounts.yaml")
	err := Save(path, []Account{
		{APIKey: "parent", Label: "go-01"},
		{APIKey: "child", Label: "go-02", ReferredByAPIKey: "parent"},
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
}
