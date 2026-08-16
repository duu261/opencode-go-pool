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
	if registration.SchemaVersion != 1 || registration.Metadata.Name != pluginName || !registration.Capabilities.ManagementAPI {
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
	if len(registration.Resources) != 1 || registration.Resources[0].Path != "/status" || registration.Resources[0].Menu != "OpenCode Go Quota" {
		t.Fatalf("unexpected resources: %#v", registration.Resources)
	}
	if len(registration.Routes) != 1 || registration.Routes[0].Method != "GET" || registration.Routes[0].Path != "/plugins/opencode-go-quota/quotas" {
		t.Fatalf("unexpected routes: %#v", registration.Routes)
	}
}

func TestManagementPageRequiresKeyBeforeLoadingQuotaData(t *testing.T) {
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
	if !bytes.Contains(response.Body, []byte("OpenCode Go Quota")) || !bytes.Contains(response.Body, []byte("Management key")) || !bytes.Contains(response.Body, []byte("5h reset")) || !bytes.Contains(response.Body, []byte("Weekly reset")) || !bytes.Contains(response.Body, []byte("Monthly reset")) {
		t.Fatalf("unexpected page: %s", response.Body)
	}
	if bytes.Contains(response.Body, []byte("api-key-entries")) {
		t.Fatalf("public shell contains credential data: %s", response.Body)
	}
	if bytes.Contains(response.Body, []byte("<form")) || bytes.Contains(response.Body, []byte(`type="submit"`)) || !bytes.Contains(response.Body, []byte(`type="button"`)) {
		t.Fatalf("management key has a native form-submission path: %s", response.Body)
	}
	if csp := response.Headers["Content-Security-Policy"]; len(csp) != 1 || !strings.Contains(csp[0], "form-action 'none'") {
		t.Fatalf("CSP does not block form submission: %#v", csp)
	}
}

func TestParsePluginConfigAppliesSafeDefaultsAndOverrides(t *testing.T) {
	config, err := parsePluginConfig([]byte("config_path: /tmp/cpa.yaml\nprovider_names: [opencode-canary]\nmax_concurrency: 7\ntimeout_seconds: 9\n"))
	if err != nil {
		t.Fatalf("parsePluginConfig() error = %v", err)
	}
	if config.ConfigPath != "/tmp/cpa.yaml" || len(config.ProviderNames) != 1 || config.ProviderNames[0] != "opencode-canary" || config.MaxConcurrency != 7 || config.TimeoutSeconds != 9 {
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
