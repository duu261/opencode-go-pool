package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/duu261/opencode-go-pool/internal/accounts"
)

func TestAccountRoutePutRejectsMissingAccountsArray(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	accountsPath := filepath.Join(dir, "accounts.yaml")
	if err := os.WriteFile(configPath, []byte("openai-compatibility: []\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := accounts.Save(accountsPath, []accounts.Account{{APIKey: "keep-me"}}); err != nil {
		t.Fatalf("save initial registry: %v", err)
	}

	previous := currentPluginConfig()
	setPluginConfig(pluginConfig{ConfigPath: configPath, AccountsPath: accountsPath, MaxConcurrency: 2, TimeoutSeconds: 5})
	defer setPluginConfig(previous)

	requestRaw, _ := json.Marshal(managementRequest{Method: http.MethodPut, Path: fullAccountsPath, Body: []byte(`{}`)})
	raw, err := handleMethod("management.handle", requestRaw)
	if err != nil {
		t.Fatalf("handleMethod() error = %v", err)
	}
	var envelope testEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	var response struct {
		StatusCode int `json:"StatusCode"`
	}
	if err := json.Unmarshal(envelope.Result, &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.StatusCode)
	}
	stored, err := accounts.Load(accountsPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(stored) != 1 || stored[0].APIKey != "keep-me" {
		t.Fatalf("registry was unexpectedly replaced: %#v", stored)
	}
}

func TestAccountRoutePutRejectsActiveReferralOnlyAccount(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	accountsPath := filepath.Join(dir, "accounts.yaml")
	const apiKey = "active-secret"
	configYAML := "openai-compatibility:\n  - name: Opencode-go\n    base-url: https://opencode.ai/zen/go/v1\n    api-key-entries:\n      - api-key: " + apiKey + "\n"
	if err := os.WriteFile(configPath, []byte(configYAML), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := accounts.Save(accountsPath, []accounts.Account{{APIKey: apiKey, ProviderName: "Opencode-go"}}); err != nil {
		t.Fatalf("save accounts: %v", err)
	}
	_, revision, err := accounts.LoadWithRevision(accountsPath)
	if err != nil {
		t.Fatalf("load revision: %v", err)
	}

	previous := currentPluginConfig()
	setPluginConfig(pluginConfig{ConfigPath: configPath, AccountsPath: accountsPath, MaxConcurrency: 1, TimeoutSeconds: 5})
	defer setPluginConfig(previous)
	body, _ := json.Marshal(map[string]any{"revision": revision, "accounts": []accounts.Account{{APIKey: apiKey, ProviderName: "Opencode-go", ReferralOnly: true}}})
	request, _ := json.Marshal(managementRequest{Method: http.MethodPut, Path: fullAccountsPath, Body: body})
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
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", response.StatusCode)
	}
}
