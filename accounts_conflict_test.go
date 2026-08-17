package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/duu261/opencode-go-pool/internal/accounts"
)

func TestAccountRoutePutRejectsStaleRevision(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	accountsPath := filepath.Join(dir, "accounts.yaml")
	if err := os.WriteFile(configPath, []byte("openai-compatibility: []\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := accounts.Save(accountsPath, []accounts.Account{{APIKey: "initial"}}); err != nil {
		t.Fatalf("save initial registry: %v", err)
	}
	_, staleRevision, err := accounts.LoadWithRevision(accountsPath)
	if err != nil {
		t.Fatalf("LoadWithRevision() error = %v", err)
	}
	if _, err := accounts.Replace(accountsPath, []accounts.Account{{APIKey: "newer"}}, staleRevision); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}

	previous := currentPluginConfig()
	setPluginConfig(pluginConfig{ConfigPath: configPath, AccountsPath: accountsPath, MaxConcurrency: 2, TimeoutSeconds: 5})
	defer setPluginConfig(previous)

	body, _ := json.Marshal(map[string]any{
		"revision": staleRevision,
		"accounts": []accounts.Account{{APIKey: "stale"}},
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
		StatusCode int    `json:"StatusCode"`
		Body       []byte `json:"Body"`
	}
	if err := json.Unmarshal(envelope.Result, &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409 body=%s", response.StatusCode, response.Body)
	}
	stored, err := accounts.Load(accountsPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(stored) != 1 || stored[0].APIKey != "newer" {
		t.Fatalf("newer registry was overwritten: %#v", stored)
	}
}
