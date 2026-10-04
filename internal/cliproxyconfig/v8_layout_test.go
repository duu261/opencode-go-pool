package cliproxyconfig

import (
	"strings"
	"testing"
)

// CLIProxyAPI v8 rewrites the config to api-keys.<provider>[].keys with
// config-version: 8. The plugin must read the same credentials as before.
func TestDiscoverReadsV8Layout(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `
api-keys:
    claude:
        - name: claude-1
          keys:
            - api-key: claude-key
    openai-compatibility:
        - name: Opencode-go
          base-url: https://opencode.ai/zen/go/v1
          headers:
            X-Opencode-Client: cli
          keys:
            - api-key: v8-one
            - api-key: v8-two
              weight: 0
            - api-key: v8-three
              proxy-url: http://proxy.local
        - name: unrelated
          base-url: https://example.com/v1
          keys:
            - api-key: other
config-version: 8
`)

	credentials, err := Discover(path, nil)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(credentials) != 3 {
		t.Fatalf("credentials len = %d, want 3: %#v", len(credentials), credentials)
	}
	first := credentials[0]
	if first.ProviderName != "Opencode-go" || first.APIKey != "v8-one" || !first.Enabled || first.BaseURL != "https://opencode.ai/zen/go" {
		t.Fatalf("unexpected first credential: %#v", first)
	}
	if credentials[1].Enabled {
		t.Fatalf("weight=0 credential must be disabled: %#v", credentials[1])
	}
	if credentials[2].ProxyURL != "http://proxy.local" {
		t.Fatalf("proxy URL lost: %#v", credentials[2])
	}
	// Auth IDs must keep using the raw configured base URL, as CLIProxyAPI does.
	want := RuntimeAuthID("Opencode-go", "v8-three", "https://opencode.ai/zen/go/v1", "http://proxy.local")
	if credentials[2].AuthID != want {
		t.Fatalf("AuthID = %q, want %q", credentials[2].AuthID, want)
	}

	providers, err := DiscoverProviders(path, nil)
	if err != nil {
		t.Fatalf("DiscoverProviders() error = %v", err)
	}
	if len(providers) != 1 || providers[0].Name != "Opencode-go" || providers[0].BaseURL != "https://opencode.ai/zen/go" {
		t.Fatalf("unexpected providers: %#v", providers)
	}
}

func TestDiscoverLegacyLayoutWithClientAPIKeyList(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `
api-keys:
  - client-key
openai-compatibility:
  - name: opencode
    base-url: https://opencode.ai/zen/go
    api-key-entries:
      - api-key: legacy-one
`)

	credentials, err := Discover(path, nil)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(credentials) != 1 || credentials[0].APIKey != "legacy-one" {
		t.Fatalf("unexpected credentials: %#v", credentials)
	}
}

func TestDiscoverV8PathReplacesLegacyList(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `
openai-compatibility:
  - name: stale
    base-url: https://opencode.ai/zen/go
    api-key-entries:
      - api-key: stale-key
api-keys:
  openai-compatibility:
    - name: current
      base-url: https://opencode.ai/zen/go
      keys:
        - api-key: current-key
config-version: 8
`)

	credentials, err := Discover(path, nil)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(credentials) != 1 || credentials[0].APIKey != "current-key" || credentials[0].ProviderName != "current" {
		t.Fatalf("v8 path must win over legacy list: %#v", credentials)
	}
}

func TestDiscoverV8EmptyListMeansNoCredentials(t *testing.T) {
	t.Parallel()

	for name, body := range map[string]string{
		"empty": "api-keys:\n  openai-compatibility: []\nopenai-compatibility:\n  - name: stale\n    base-url: https://opencode.ai/zen/go\n    api-key-entries:\n      - api-key: stale-key\n",
		"null":  "api-keys:\n  openai-compatibility:\nopenai-compatibility:\n  - name: stale\n    base-url: https://opencode.ai/zen/go\n    api-key-entries:\n      - api-key: stale-key\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, body)
			credentials, err := Discover(path, nil)
			if err != nil {
				t.Fatalf("Discover() error = %v", err)
			}
			if len(credentials) != 0 {
				t.Fatalf("present v8 path must replace legacy list: %#v", credentials)
			}
		})
	}
}

func TestDiscoverRejectsMalformedV8Group(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `
api-keys:
  openai-compatibility:
    name: not-a-list
`)

	_, err := Discover(path, nil)
	if err == nil || !strings.Contains(err.Error(), "api-keys.openai-compatibility") {
		t.Fatalf("Discover() error = %v, want malformed v8 error", err)
	}
}
