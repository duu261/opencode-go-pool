package cliproxyconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverFindsDirectOpenCodeKeysAndPreservesDisabledState(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `
openai-compatibility:
  - name: unrelated
    base-url: https://example.com/v1
    api-key-entries:
      - api-key: ignore-me
  - name: opencode-direct
    base-url: https://opencode.ai/zen/go/
    api-key-entries:
      - api-key: direct-one
      - api-key: direct-two
        weight: 0
      - api-key: ""
`)

	credentials, err := Discover(path, nil)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(credentials) != 2 {
		t.Fatalf("credentials len = %d, want 2: %#v", len(credentials), credentials)
	}
	if credentials[0].ProviderName != "opencode-direct" || credentials[0].BaseURL != "https://opencode.ai/zen/go/" || credentials[0].APIKey != "direct-one" || !credentials[0].Enabled {
		t.Fatalf("unexpected first credential: %#v", credentials[0])
	}
	if credentials[1].Enabled {
		t.Fatalf("weight=0 credential must be disabled: %#v", credentials[1])
	}
	for _, credential := range credentials {
		if credential.KeyID == "" || strings.Contains(credential.KeyID, credential.APIKey) {
			t.Fatalf("unsafe key ID: %#v", credential)
		}
	}
}

func TestDiscoverMatchesConfiguredProviderNameForCanaryEndpoint(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `
openai-compatibility:
  - name: opencode-canary
    base-url: http://127.0.0.1:18444/zen/go
    api-key-entries:
      - api-key: canary-key
`)

	credentials, err := Discover(path, []string{"OPENCODE-CANARY"})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(credentials) != 1 || credentials[0].APIKey != "canary-key" {
		t.Fatalf("unexpected credentials: %#v", credentials)
	}
}

func TestDiscoverRejectsPlainHTTPDirectProvider(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `
openai-compatibility:
  - name: unsafe-direct
    base-url: http://opencode.ai/zen/go
    api-key-entries:
      - api-key: must-not-leave-over-http
`)

	credentials, err := Discover(path, nil)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(credentials) != 0 {
		t.Fatalf("plain HTTP direct provider was accepted: %#v", credentials)
	}
}

func TestDiscoverFindsDirectOpenCodeCompatibleV1BaseURL(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `
openai-compatibility:
  - name: opencode-compatible
    base-url: https://opencode.ai/zen/go/v1
    api-key-entries:
      - api-key: compatible-key
`)

	credentials, err := Discover(path, nil)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(credentials) != 1 || credentials[0].BaseURL != "https://opencode.ai/zen/go" {
		t.Fatalf("compatible provider was not discovered: %#v", credentials)
	}
}

func TestDiscoverPreservesExplicitCustomV1BaseURL(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `
openai-compatibility:
  - name: custom-explicit
    base-url: http://127.0.0.1:18444/custom/v1
    api-key-entries:
      - api-key: explicit-key
`)

	credentials, err := Discover(path, []string{"custom-explicit"})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(credentials) != 1 || credentials[0].BaseURL != "http://127.0.0.1:18444/custom/v1" {
		t.Fatalf("explicit provider base URL changed: %#v", credentials)
	}
}

func TestDiscoverRejectsQueryOrFragmentOnAutomaticDirectProvider(t *testing.T) {
	t.Parallel()

	for _, baseURL := range []string{
		"https://opencode.ai/zen/go/v1?",
		"https://opencode.ai/zen/go/v1?project=wrong",
		"https://opencode.ai/zen/go/v1#fragment",
	} {
		t.Run(baseURL, func(t *testing.T) {
			path := writeConfig(t, "openai-compatibility:\n  - name: unsafe-direct\n    base-url: \""+baseURL+"\"\n    api-key-entries:\n      - api-key: must-not-be-used\n")
			credentials, err := Discover(path, nil)
			if err != nil {
				t.Fatalf("Discover() error = %v", err)
			}
			if len(credentials) != 0 {
				t.Fatalf("unsafe direct URL was accepted: %#v", credentials)
			}
		})
	}
}

func TestDiscoverRejectsPlainHTTPDirectV1Provider(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `
openai-compatibility:
  - name: unsafe-direct-v1
    base-url: http://opencode.ai/zen/go/v1
    api-key-entries:
      - api-key: must-not-leave-over-http
`)

	credentials, err := Discover(path, nil)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(credentials) != 0 {
		t.Fatalf("plain HTTP direct v1 provider was accepted: %#v", credentials)
	}
}

func TestDiscoverRejectsAnyExplicitPortOnAutomaticDirectProvider(t *testing.T) {
	t.Parallel()

	for _, baseURL := range []string{
		"https://opencode.ai:/zen/go/v1",
		"https://opencode.ai:443/zen/go/v1",
	} {
		t.Run(baseURL, func(t *testing.T) {
			path := writeConfig(t, "openai-compatibility:\n  - name: unsafe-port\n    base-url: \""+baseURL+"\"\n    api-key-entries:\n      - api-key: must-not-be-used\n")
			credentials, err := Discover(path, nil)
			if err != nil {
				t.Fatalf("Discover() error = %v", err)
			}
			if len(credentials) != 0 {
				t.Fatalf("explicit-port direct URL was accepted: %#v", credentials)
			}
		})
	}
}

func TestDiscoverDeduplicatesRepeatedKeys(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `
openai-compatibility:
  - name: first
    base-url: https://opencode.ai/zen/go
    api-key-entries:
      - api-key: same-key
  - name: second
    base-url: https://opencode.ai/zen/go
    api-key-entries:
      - api-key: same-key
`)

	credentials, err := Discover(path, nil)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(credentials) != 1 {
		t.Fatalf("credentials len = %d, want 1", len(credentials))
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
