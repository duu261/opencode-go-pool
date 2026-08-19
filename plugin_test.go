package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type testEnvelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result"`
}

func TestPluginRegistrationDeclaresManagementAPI(t *testing.T) {
	raw, err := handleMethod("plugin.register", nil)
	if err != nil {
		t.Fatalf("handleMethod() error = %v", err)
	}
	var envelope testEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if !envelope.OK {
		t.Fatal("registration envelope is not ok")
	}
	var registration struct {
		SchemaVersion uint32 `json:"schema_version"`
		Metadata      struct {
			Name string `json:"Name"`
		} `json:"metadata"`
		Capabilities struct {
			ManagementAPI bool `json:"management_api"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(envelope.Result, &registration); err != nil {
		t.Fatalf("decode registration: %v", err)
	}
	if registration.SchemaVersion != 3 || registration.Metadata.Name != pluginName || !registration.Capabilities.ManagementAPI {
		t.Fatalf("unexpected registration: %#v", registration)
	}
}

func TestManagementRegistrationExposesQuotaPage(t *testing.T) {
	raw, err := handleMethod("management.register", nil)
	if err != nil {
		t.Fatalf("handleMethod() error = %v", err)
	}
	var envelope testEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	var registration struct {
		Routes []struct {
			Method string `json:"Method"`
			Path   string `json:"Path"`
		} `json:"routes"`
		Resources []struct {
			Path string `json:"Path"`
			Menu string `json:"Menu"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(envelope.Result, &registration); err != nil {
		t.Fatalf("decode management registration: %v", err)
	}
	if len(registration.Resources) != 1 || registration.Resources[0].Path != "/status" || registration.Resources[0].Menu != "OpenCode Go Pool" {
		t.Fatalf("unexpected resources: %#v", registration.Resources)
	}
	if len(registration.Routes) != 4 || registration.Routes[0].Method != "GET" || registration.Routes[0].Path != "/plugins/opencode-go-quota/quotas" || registration.Routes[1].Method != "GET" || registration.Routes[1].Path != "/plugins/opencode-go-quota/accounts" || registration.Routes[2].Method != "PUT" || registration.Routes[2].Path != "/plugins/opencode-go-quota/accounts" || registration.Routes[3].Method != "POST" || registration.Routes[3].Path != "/plugins/opencode-go-quota/resume" {
		t.Fatalf("unexpected routes: %#v", registration.Routes)
	}
}

func TestManagementPageReusesManagementCenterLogin(t *testing.T) {
	raw, err := handleMethod("management.handle", []byte(`{"Method":"GET","Path":"/v0/resource/plugins/opencode-go-quota/status"}`))
	if err != nil {
		t.Fatalf("handleMethod() error = %v", err)
	}
	var envelope testEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	var response struct {
		StatusCode int                 `json:"StatusCode"`
		Headers    map[string][]string `json:"Headers"`
		Body       []byte              `json:"Body"`
	}
	if err := json.Unmarshal(envelope.Result, &response); err != nil {
		t.Fatalf("decode management response: %v", err)
	}
	if response.StatusCode != 200 {
		t.Fatalf("status = %d", response.StatusCode)
	}
	if !bytes.Contains(response.Body, []byte("OpenCode Go Pool")) || !bytes.Contains(response.Body, []byte("cli-proxy-auth")) || !bytes.Contains(response.Body, []byte("/v0/management/plugins/opencode-go-quota/accounts")) || !bytes.Contains(response.Body, []byte("/v0/management/plugins/opencode-go-quota/resume")) || !bytes.Contains(response.Body, []byte("/v0/management/openai-compatibility")) || !bytes.Contains(response.Body, []byte("Resume routing")) || !bytes.Contains(response.Body, []byte("Add account")) || !bytes.Contains(response.Body, []byte("Referral")) || !bytes.Contains(response.Body, []byte("Referred by account")) || !bytes.Contains(response.Body, []byte(`id="account-referred-by"`)) || !bytes.Contains(response.Body, []byte("Password")) || !bytes.Contains(response.Body, []byte(`id="account-password" type="password"`)) || !bytes.Contains(response.Body, []byte("select.disabled=index>=0")) || !bytes.Contains(response.Body, []byte("revision=snapshot.revision")) || !bytes.Contains(response.Body, []byte("JSON.stringify({revision,accounts:")) || !bytes.Contains(response.Body, []byte("add.disabled=openCodeProviders().length===0")) || !bytes.Contains(response.Body, []byte("No eligible OpenCode provider configured")) || !bytes.Contains(response.Body, []byte("Number(current[index].weight)===0")) || !bytes.Contains(response.Body, []byte("weight:1")) || !bytes.Contains(response.Body, []byte("navigator.locks.request")) || !bytes.Contains(response.Body, []byte("async function recover")) || !bytes.Contains(response.Body, []byte(".links .pill{margin-top:0}")) || !bytes.Contains(response.Body, []byte("5h")) || !bytes.Contains(response.Body, []byte("Weekly")) {
		t.Fatalf("unexpected page: %s", response.Body)
	}
	if bytes.Contains(response.Body, []byte("||openCodeProviders()[0]")) {
		t.Fatalf("provider targeting falls back to a different provider: %s", response.Body)
	}
	if bytes.Contains(response.Body, []byte("return matches.length?matches:providers")) {
		t.Fatalf("account creation falls back to unrelated providers: %s", response.Body)
	}
	if bytes.Contains(response.Body, []byte("account-enabled")) || bytes.Contains(response.Body, []byte("if(enabled!==")) {
		t.Fatalf("metadata save is coupled to a pool mutation: %s", response.Body)
	}
	if bytes.Contains(response.Body, []byte("claim_url")) || bytes.Contains(response.Body, []byte("account-claim")) {
		t.Fatalf("obsolete claim-link field remains: %s", response.Body)
	}
	if !bytes.Contains(response.Body, []byte("remove.disabled=account.pool_enabled")) || !bytes.Contains(response.Body, []byte("Disable account before deleting it")) {
		t.Fatalf("active account deletion is not blocked: %s", response.Body)
	}
	if bytes.Contains(response.Body, []byte(`placeholder="Management key"`)) || bytes.Contains(response.Body, []byte(`type="password" id="key"`)) {
		t.Fatalf("page asks for a second management key: %s", response.Body)
	}
	if bytes.Contains(response.Body, []byte("<form")) || bytes.Contains(response.Body, []byte(`type="submit"`)) {
		t.Fatalf("page has a native form-submission path: %s", response.Body)
	}
	if csp := response.Headers["Content-Security-Policy"]; len(csp) != 1 || !strings.Contains(csp[0], "form-action 'none'") {
		t.Fatalf("CSP does not block form submission: %#v", csp)
	}
}

func TestParsePluginConfigAppliesSafeDefaultsAndOverrides(t *testing.T) {
	config, err := parsePluginConfig([]byte("config_path: /tmp/cpa.yaml\naccounts_path: /tmp/accounts.yaml\nprovider_names: [opencode-canary]\nmax_concurrency: 7\ntimeout_seconds: 9\n"))
	if err != nil {
		t.Fatalf("parsePluginConfig() error = %v", err)
	}
	if config.ConfigPath != "/tmp/cpa.yaml" || config.AccountsPath != "/tmp/accounts.yaml" || len(config.ProviderNames) != 1 || config.ProviderNames[0] != "opencode-canary" || config.MaxConcurrency != 7 || config.TimeoutSeconds != 9 {
		t.Fatalf("unexpected config: %#v", config)
	}
}

func TestAuthenticatedQuotaRouteReturnsBulkResultsWithoutKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer direct-secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"usage":{"rolling":{"status":"ok","percent":8,"resetsAt":"2026-08-16T19:50:55Z"},"weekly":{"status":"ok","percent":29,"resetsAt":"2026-08-17T00:00:00Z"},"monthly":{"status":"ok","percent":14,"resetsAt":"2026-09-13T21:51:50Z"}}}`))
	}))
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configYAML := "openai-compatibility:\n  - name: opencode-canary\n    base-url: " + server.URL + "\n    api-key-entries:\n      - api-key: direct-secret\n"
	if err := os.WriteFile(configPath, []byte(configYAML), 0o600); err != nil {
		t.Fatalf("write CLIProxy config: %v", err)
	}

	previous := currentPluginConfig()
	setPluginConfig(pluginConfig{ConfigPath: configPath, ProviderNames: []string{"opencode-canary"}, MaxConcurrency: 2, TimeoutSeconds: 5})
	defer setPluginConfig(previous)

	raw, err := handleMethod("management.handle", []byte(`{"Method":"GET","Path":"/v0/management/plugins/opencode-go-quota/quotas"}`))
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
		t.Fatalf("decode management response: %v", err)
	}
	if response.StatusCode != 200 || !strings.Contains(string(response.Body), `"weekly":{"status":"ok","percent":29`) {
		t.Fatalf("unexpected quota response: status=%d body=%s", response.StatusCode, response.Body)
	}
	if bytes.Contains(response.Body, []byte("direct-secret")) {
		t.Fatalf("quota response leaks API key: %s", response.Body)
	}
}
