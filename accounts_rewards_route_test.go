package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/duu261/opencode-go-pool/internal/accounts"
)

func TestAccountRoutePutAwardsReferralCreditsForNewAccount(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	accountsPath := filepath.Join(dir, "accounts.yaml")
	if err := os.WriteFile(configPath, []byte("openai-compatibility:\n  - name: Opencode-go\n    base-url: https://opencode.ai/zen/go\n    api-key-entries: []\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	parent := accounts.Account{APIKey: "parent-key", ProviderName: "Opencode-go", Label: "go-001"}
	if err := accounts.Save(accountsPath, []accounts.Account{parent}); err != nil {
		t.Fatalf("save registry: %v", err)
	}
	_, revision, err := accounts.LoadWithRevision(accountsPath)
	if err != nil {
		t.Fatalf("LoadWithRevision() error = %v", err)
	}

	previous := currentPluginConfig()
	setPluginConfig(pluginConfig{ConfigPath: configPath, AccountsPath: accountsPath, MaxConcurrency: 2, TimeoutSeconds: 5})
	defer setPluginConfig(previous)

	child := accounts.Account{APIKey: "child-key", ProviderName: "Opencode-go", Label: "go-002", ReferredByAPIKey: parent.APIKey}
	body, _ := json.Marshal(map[string]any{"revision": revision, "accounts": []accounts.Account{parent, child}})
	requestRaw, _ := json.Marshal(managementRequest{Method: "PUT", Path: fullAccountsPath, Body: body})
	if _, err := handleMethod("management.handle", requestRaw); err != nil {
		t.Fatalf("handleMethod() error = %v", err)
	}

	stored, err := accounts.Load(accountsPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(stored) != 2 || stored[0].ReferralCredits != 1 || stored[1].ReferralCredits != 1 || !stored[1].ReferralAwarded {
		t.Fatalf("stored referral credits = %#v, want one credit on each account", stored)
	}
}
