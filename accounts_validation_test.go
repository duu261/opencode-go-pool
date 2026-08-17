package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/duu261/opencode-go-quota/internal/accounts"
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
