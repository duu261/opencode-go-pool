package accounts

import (
	"path/filepath"
	"testing"
)

func TestLoadStateWithRevisionBackfillsLegacyReferralAwards(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.yaml")
	parent := Account{APIKey: "parent-key", Label: "go-001", ReferralCredits: 1}
	child := Account{APIKey: "child-key", Label: "go-002", ReferredByAPIKey: parent.APIKey, ReferralCredits: 1, ReferralAwarded: true}
	if err := Save(path, []Account{parent, child}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	state, _, err := LoadStateWithRevision(path)
	if err != nil {
		t.Fatalf("LoadStateWithRevision() error = %v", err)
	}
	if _, ok := state.ReferralAwardedKeys[child.APIKey]; !ok {
		t.Fatalf("legacy award history = %#v, want %q", state.ReferralAwardedKeys, child.APIKey)
	}
}
