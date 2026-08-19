package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/duu261/opencode-go-pool/internal/accounts"
	"github.com/duu261/opencode-go-pool/internal/cliproxyconfig"
)

func TestAccountExpiredAfterDateEnds(t *testing.T) {
	account := accounts.Account{ExpiresAt: "2026-09-13"}
	if accountExpired(account, time.Date(2026, 9, 13, 16, 59, 59, 0, time.UTC)) {
		t.Fatal("account expired before its Vietnam calendar date ended")
	}
	if !accountExpired(account, time.Date(2026, 9, 13, 17, 0, 0, 0, time.UTC)) {
		t.Fatal("account remained eligible after its Vietnam expiry date")
	}
}

func TestSchedulerRejectsReferralOnlyPool(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	accountsPath := filepath.Join(dir, "accounts.yaml")
	const apiKey = "referral-only-secret"
	configYAML := "openai-compatibility:\n  - name: Opencode-go\n    base-url: https://opencode.ai/zen/go/v1\n    api-key-entries:\n      - api-key: " + apiKey + "\n"
	if err := os.WriteFile(configPath, []byte(configYAML), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := accounts.Save(accountsPath, []accounts.Account{{APIKey: apiKey, ProviderName: "Opencode-go", ReferralOnly: true}}); err != nil {
		t.Fatalf("save accounts: %v", err)
	}

	previousConfig := currentPluginConfig()
	setPluginConfig(pluginConfig{ConfigPath: configPath, AccountsPath: accountsPath, AutoPool: true, MaxConcurrency: 1, TimeoutSeconds: 5})
	defer setPluginConfig(previousConfig)
	resetHeldCacheForTest()

	authID := cliproxyconfig.RuntimeAuthID("Opencode-go", apiKey, "https://opencode.ai/zen/go/v1", "")
	request, err := json.Marshal(schedulerPickRequest{Provider: "openai-compatibility:opencode-go", Candidates: []schedulerCandidate{{
		ID: authID, Provider: "openai-compatibility:opencode-go", Attributes: map[string]string{"compat_name": "Opencode-go"},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := handleSchedulerPick(request)
	if err != nil {
		t.Fatalf("handleSchedulerPick() error = %v", err)
	}
	var envelope envelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.OK || envelope.Error == nil || envelope.Error.Code != "no_eligible_opencode_credentials" {
		t.Fatalf("scheduler response = %#v, want explicit no-eligible error", envelope)
	}
}

func TestResumeCooldownRouteClearsOneCredential(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	accountsPath := filepath.Join(dir, "accounts.yaml")
	const apiKey = "cooling-secret"
	configYAML := "openai-compatibility:\n  - name: Opencode-go\n    base-url: https://opencode.ai/zen/go/v1\n    api-key-entries:\n      - api-key: " + apiKey + "\n"
	if err := os.WriteFile(configPath, []byte(configYAML), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := accounts.Save(accountsPath, []accounts.Account{{APIKey: apiKey, ProviderName: "Opencode-go"}}); err != nil {
		t.Fatalf("save accounts: %v", err)
	}

	previousConfig := currentPluginConfig()
	setPluginConfig(pluginConfig{ConfigPath: configPath, AccountsPath: accountsPath, AutoPool: true, MaxConcurrency: 1, TimeoutSeconds: 5})
	defer setPluginConfig(previousConfig)
	authID := cliproxyconfig.RuntimeAuthID("Opencode-go", apiKey, "https://opencode.ai/zen/go/v1", "")
	runtimeAutoPool.mu.Lock()
	runtimeAutoPool.blocked[authID] = time.Now().Add(time.Hour)
	runtimeAutoPool.mu.Unlock()
	defer runtimeAutoPool.clear(authID)

	body, _ := json.Marshal(resumeCooldownRequest{KeyID: cliproxyconfig.KeyID(apiKey)})
	request, _ := json.Marshal(managementRequest{Method: http.MethodPost, Path: fullResumePath, Body: body})
	raw, err := handleMethod("management.handle", request)
	if err != nil {
		t.Fatalf("handleMethod() error = %v", err)
	}
	var result testEnvelope
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	var response struct {
		StatusCode int `json:"StatusCode"`
	}
	if err := json.Unmarshal(result.Result, &response); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	if state, _ := runtimeAutoPool.state(authID, time.Now()); state != "ready" {
		t.Fatalf("state = %q, want ready", state)
	}
}

func resetHeldCacheForTest() {
	runtimeAutoPool.mu.Lock()
	runtimeAutoPool.held = make(map[string]bool)
	runtimeAutoPool.heldUntil = time.Time{}
	runtimeAutoPool.mu.Unlock()
}
