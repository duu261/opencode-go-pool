package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/duu261/opencode-go-quota/internal/accounts"
)

func TestAccountRouteMergesRegistryIdentityWithLiveQuota(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "Bearer active-secret" && auth != "Bearer parked-secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"usage":{"rolling":{"status":"ok","percent":12,"resetsAt":"2026-08-17T20:00:00Z"},"weekly":{"status":"ok","percent":34,"resetsAt":"2026-08-24T00:00:00Z"},"monthly":{"status":"ok","percent":56,"resetsAt":"2026-09-17T00:00:00Z"}}}`))
	}))
	defer server.Close()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	accountsPath := filepath.Join(dir, "accounts.yaml")
	configYAML := "openai-compatibility:\n  - name: opencode-canary\n    base-url: " + server.URL + "\n    api-key-entries:\n      - api-key: active-secret\n"
	if err := os.WriteFile(configPath, []byte(configYAML), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := accounts.Save(accountsPath, []accounts.Account{
		{APIKey: "active-secret", ProviderName: "opencode-canary", Label: "go-01", Email: "go01@example.com", Password: "pw-one", ReferralURL: "https://opencode.ai/go?ref=ONE"},
		{APIKey: "parked-secret", ProviderName: "opencode-canary", Label: "go-02", Email: "go02@example.com", Password: "pw-two", ReferredByAPIKey: "active-secret"},
	}); err != nil {
		t.Fatalf("save accounts: %v", err)
	}

	previous := currentPluginConfig()
	setPluginConfig(pluginConfig{ConfigPath: configPath, AccountsPath: accountsPath, ProviderNames: []string{"opencode-canary"}, MaxConcurrency: 2, TimeoutSeconds: 5})
	defer setPluginConfig(previous)

	raw, err := handleMethod("management.handle", []byte(`{"Method":"GET","Path":"/v0/management/plugins/opencode-go-quota/accounts"}`))
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
	var snapshot struct {
		Revision  string `json:"revision"`
		Providers []struct {
			Name string `json:"name"`
		} `json:"providers"`
		Accounts []struct {
			APIKey           string `json:"api_key"`
			Label            string `json:"label"`
			Email            string `json:"email"`
			Password         string `json:"password"`
			ReferredByAPIKey string `json:"referred_by_api_key"`
			PoolEnabled      bool   `json:"pool_enabled"`
			Status           string `json:"status"`
			Usage            *struct {
				Weekly struct {
					Percent float64 `json:"percent"`
				} `json:"weekly"`
			} `json:"usage"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(response.Body, &snapshot); err != nil {
		t.Fatalf("decode account snapshot: %v body=%s", err, response.Body)
	}
	if response.StatusCode != http.StatusOK || snapshot.Revision == "" || len(snapshot.Providers) != 1 || snapshot.Providers[0].Name != "opencode-canary" || len(snapshot.Accounts) != 2 {
		t.Fatalf("unexpected account response: status=%d accounts=%#v", response.StatusCode, snapshot.Accounts)
	}
	if first := snapshot.Accounts[0]; first.APIKey != "active-secret" || first.Label != "go-01" || first.Email != "go01@example.com" || first.Password != "pw-one" || !first.PoolEnabled || first.Status != "healthy" || first.Usage == nil || first.Usage.Weekly.Percent != 34 {
		t.Fatalf("unexpected active account: %#v", first)
	}
	if second := snapshot.Accounts[1]; second.APIKey != "parked-secret" || second.ReferredByAPIKey != "active-secret" || second.PoolEnabled || second.Status != "healthy" || second.Usage == nil || second.Usage.Weekly.Percent != 34 {
		t.Fatalf("unexpected parked account: %#v", second)
	}
}

func TestAccountRoutePutReplacesPlaintextRegistry(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	accountsPath := filepath.Join(dir, "accounts.yaml")
	configYAML := "openai-compatibility:\n  - name: Opencode-go\n    base-url: https://opencode.ai/zen/go\n    api-key-entries: []\n"
	if err := os.WriteFile(configPath, []byte(configYAML), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	previous := currentPluginConfig()
	setPluginConfig(pluginConfig{ConfigPath: configPath, AccountsPath: accountsPath, MaxConcurrency: 2, TimeoutSeconds: 5})
	defer setPluginConfig(previous)

	_, revision, err := accounts.LoadWithRevision(accountsPath)
	if err != nil {
		t.Fatalf("LoadWithRevision() error = %v", err)
	}
	body, _ := json.Marshal(map[string]any{
		"revision": revision,
		"accounts": []accounts.Account{{APIKey: "saved-secret", ProviderName: "Opencode-go", Label: "go-10", Email: "go10@example.com", Password: "pw"}},
	})
	request := managementRequest{Method: http.MethodPut, Path: fullAccountsPath, Body: body}
	requestRaw, _ := json.Marshal(request)
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
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	stored, err := accounts.Load(accountsPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(stored) != 1 || stored[0].APIKey != "saved-secret" || stored[0].Label != "go-10" || stored[0].Password != "pw" {
		t.Fatalf("stored accounts = %#v", stored)
	}
}
