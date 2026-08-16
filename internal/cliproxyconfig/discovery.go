package cliproxyconfig

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

const maxConfigBytes = 8 << 20

type Credential struct {
	ProviderName string
	BaseURL      string
	KeyID        string
	APIKey       string
	Enabled      bool
}

type proxyConfig struct {
	OpenAICompatibility []compatProvider `yaml:"openai-compatibility"`
}

type compatProvider struct {
	Name          string         `yaml:"name"`
	BaseURL       string         `yaml:"base-url"`
	APIKeyEntries []compatAPIKey `yaml:"api-key-entries"`
}

type compatAPIKey struct {
	APIKey string `yaml:"api-key"`
	Weight *int   `yaml:"weight"`
}

func Discover(configPath string, providerNames []string) ([]Credential, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("open CLIProxy config: %w", err)
	}
	defer file.Close()

	var config proxyConfig
	decoder := yaml.NewDecoder(io.LimitReader(file, maxConfigBytes))
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("decode CLIProxy config: %w", err)
	}

	allowedNames := make(map[string]struct{}, len(providerNames))
	for _, name := range providerNames {
		if normalized := strings.ToLower(strings.TrimSpace(name)); normalized != "" {
			allowedNames[normalized] = struct{}{}
		}
	}

	credentials := make([]Credential, 0)
	seen := make(map[string]int)
	for _, provider := range config.OpenAICompatibility {
		if !isOpenCodeProvider(provider, allowedNames) {
			continue
		}
		for _, entry := range provider.APIKeyEntries {
			apiKey := strings.TrimSpace(entry.APIKey)
			if apiKey == "" {
				continue
			}
			enabled := entry.Weight == nil || *entry.Weight > 0
			if index, exists := seen[apiKey]; exists {
				credentials[index].Enabled = credentials[index].Enabled || enabled
				continue
			}
			seen[apiKey] = len(credentials)
			credentials = append(credentials, Credential{
				ProviderName: strings.TrimSpace(provider.Name),
				BaseURL:      strings.TrimSpace(provider.BaseURL),
				KeyID:        fingerprint(apiKey),
				APIKey:       apiKey,
				Enabled:      enabled,
			})
		}
	}
	return credentials, nil
}

func isOpenCodeProvider(provider compatProvider, allowedNames map[string]struct{}) bool {
	if _, ok := allowedNames[strings.ToLower(strings.TrimSpace(provider.Name))]; ok {
		return true
	}
	u, err := url.Parse(strings.TrimSpace(provider.BaseURL))
	if err != nil || !strings.EqualFold(u.Scheme, "https") || !strings.EqualFold(u.Hostname(), "opencode.ai") || u.Port() != "" {
		return false
	}
	return path.Clean("/"+strings.TrimSpace(u.Path)) == "/zen/go"
}

func fingerprint(apiKey string) string {
	digest := sha256.Sum256([]byte(apiKey))
	return "key-" + hex.EncodeToString(digest[:6])
}
