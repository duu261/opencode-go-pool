package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/duu261/opencode-go-pool/internal/accounts"
)

func TestAccountRouteRejectsUnrelatedProvider(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	accountsPath := filepath.Join(dir, "accounts.yaml")
	configYAML := "openai-compatibility:\n  - name: opencode-direct\n    base-url: https://opencode.ai/zen/go\n    api-key-entries: []\n  - name: unrelated\n    base-url: https://example.com/v1\n    api-key-entries: []\n"
	if err := os.WriteFile(configPath, []byte(configYAML), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	_, revision, err := accounts.LoadWithRevision(accountsPath)
	if err != nil {
		t.Fatalf("LoadWithRevision() error = %v", err)
	}

	previous := currentPluginConfig()
	setPluginConfig(pluginConfig{ConfigPath: configPath, AccountsPath: accountsPath, MaxConcurrency: 2, TimeoutSeconds: 5})
	defer setPluginConfig(previous)

	body, _ := json.Marshal(map[string]any{
		"revision": revision,
		"accounts": []accounts.Account{{APIKey: "wrong-secret", ProviderName: "unrelated"}},
	})
	requestRaw, _ := json.Marshal(managementRequest{Method: http.MethodPut, Path: fullAccountsPath, Body: body})
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
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", response.StatusCode)
	}
	stored, err := accounts.Load(accountsPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(stored) != 0 {
		t.Fatalf("unrelated provider account was stored: %#v", stored)
	}
}
