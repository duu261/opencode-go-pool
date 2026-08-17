package cliproxyconfig

import "testing"

func TestDiscoverProvidersIncludesOpenCodeProviderWithoutKeys(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `
openai-compatibility:
  - name: opencode-empty
    base-url: https://opencode.ai/zen/go/v1
    api-key-entries: []
  - name: unrelated
    base-url: https://example.com/v1
    api-key-entries: []
`)

	providers, err := DiscoverProviders(path, nil)
	if err != nil {
		t.Fatalf("DiscoverProviders() error = %v", err)
	}
	if len(providers) != 1 || providers[0].Name != "opencode-empty" || providers[0].BaseURL != "https://opencode.ai/zen/go" {
		t.Fatalf("providers = %#v", providers)
	}
}

func TestKeyIDDoesNotExposeAPIKey(t *testing.T) {
	t.Parallel()

	if got := KeyID("parked-secret"); got == "" || got == "parked-secret" {
		t.Fatalf("KeyID() = %q", got)
	}
}
