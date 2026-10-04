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
	AuthID       string
	ProxyURL     string
	APIKey       string
	Enabled      bool
}

type Provider struct {
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
}

type proxyConfig struct {
	OpenAICompatibility []compatProvider `yaml:"openai-compatibility"`
	// APIKeys is a client-key list in the legacy layout and an upstream
	// provider map in the CLIProxyAPI v8 layout.
	APIKeys yaml.Node `yaml:"api-keys"`
}

type compatProvider struct {
	Name          string         `yaml:"name"`
	BaseURL       string         `yaml:"base-url"`
	APIKeyEntries []compatAPIKey `yaml:"api-key-entries"`
}

// v8CompatGroup is one api-keys.openai-compatibility entry in the
// CLIProxyAPI v8 layout; its keys list replaces api-key-entries.
type v8CompatGroup struct {
	Name    string         `yaml:"name"`
	BaseURL string         `yaml:"base-url"`
	Keys    []compatAPIKey `yaml:"keys"`
}

type compatAPIKey struct {
	APIKey   string `yaml:"api-key"`
	ProxyURL string `yaml:"proxy-url"`
	Weight   *int   `yaml:"weight"`
}

func Discover(configPath string, providerNames []string) ([]Credential, error) {
	config, err := loadConfig(configPath)
	if err != nil {
		return nil, err
	}
	allowedNames := allowedProviderNames(providerNames)
	credentials := make([]Credential, 0)
	seen := make(map[string]int)
	for _, provider := range config.OpenAICompatibility {
		usageBaseURL, ok := providerUsageBaseURL(provider, allowedNames)
		if !ok {
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
				BaseURL:      usageBaseURL,
				KeyID:        KeyID(apiKey),
				AuthID:       RuntimeAuthID(provider.Name, apiKey, provider.BaseURL, entry.ProxyURL),
				ProxyURL:     strings.TrimSpace(entry.ProxyURL),
				APIKey:       apiKey,
				Enabled:      enabled,
			})
		}
	}
	return credentials, nil
}

func DiscoverProviders(configPath string, providerNames []string) ([]Provider, error) {
	config, err := loadConfig(configPath)
	if err != nil {
		return nil, err
	}
	allowedNames := allowedProviderNames(providerNames)
	providers := make([]Provider, 0, len(config.OpenAICompatibility))
	for _, provider := range config.OpenAICompatibility {
		baseURL, ok := providerUsageBaseURL(provider, allowedNames)
		if !ok {
			continue
		}
		providers = append(providers, Provider{Name: strings.TrimSpace(provider.Name), BaseURL: baseURL})
	}
	return providers, nil
}

func loadConfig(configPath string) (proxyConfig, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return proxyConfig{}, fmt.Errorf("open CLIProxy config: %w", err)
	}
	defer file.Close()

	var config proxyConfig
	decoder := yaml.NewDecoder(io.LimitReader(file, maxConfigBytes))
	if err := decoder.Decode(&config); err != nil {
		return proxyConfig{}, fmt.Errorf("decode CLIProxy config: %w", err)
	}
	groups, err := v8CompatGroups(&config.APIKeys)
	if err != nil {
		return proxyConfig{}, fmt.Errorf("decode CLIProxy config: %w", err)
	}
	// CLIProxyAPI v8 lets api-keys.openai-compatibility replace the legacy
	// top-level list whenever the v8 path is present.
	if groups != nil {
		config.OpenAICompatibility = make([]compatProvider, 0, len(groups))
		for _, group := range groups {
			config.OpenAICompatibility = append(config.OpenAICompatibility, compatProvider{
				Name:          group.Name,
				BaseURL:       group.BaseURL,
				APIKeyEntries: group.Keys,
			})
		}
	}
	return config, nil
}

// v8CompatGroups returns nil when the v8 openai-compatibility path is absent,
// and a non-nil slice (possibly empty) when it is present.
func v8CompatGroups(apiKeys *yaml.Node) ([]v8CompatGroup, error) {
	if apiKeys.Kind != yaml.MappingNode {
		return nil, nil
	}
	for index := 0; index+1 < len(apiKeys.Content); index += 2 {
		if apiKeys.Content[index].Value != "openai-compatibility" {
			continue
		}
		value := apiKeys.Content[index+1]
		if value.Kind == yaml.ScalarNode && value.Tag == "!!null" {
			return []v8CompatGroup{}, nil
		}
		groups := make([]v8CompatGroup, 0)
		if err := value.Decode(&groups); err != nil {
			return nil, fmt.Errorf("api-keys.openai-compatibility: %w", err)
		}
		return groups, nil
	}
	return nil, nil
}

func allowedProviderNames(providerNames []string) map[string]struct{} {
	allowedNames := make(map[string]struct{}, len(providerNames))
	for _, name := range providerNames {
		if normalized := strings.ToLower(strings.TrimSpace(name)); normalized != "" {
			allowedNames[normalized] = struct{}{}
		}
	}
	return allowedNames
}

func providerUsageBaseURL(provider compatProvider, allowedNames map[string]struct{}) (string, bool) {
	baseURL := strings.TrimSpace(provider.BaseURL)
	if _, ok := allowedNames[strings.ToLower(strings.TrimSpace(provider.Name))]; ok {
		return baseURL, true
	}
	u, err := url.Parse(baseURL)
	if err != nil || !strings.EqualFold(u.Scheme, "https") || !strings.EqualFold(u.Host, "opencode.ai") || u.User != nil || u.ForceQuery || u.RawQuery != "" || u.Fragment != "" {
		return "", false
	}
	providerPath := path.Clean("/" + strings.TrimSpace(u.Path))
	switch providerPath {
	case "/zen/go":
		return baseURL, true
	case "/zen/go/v1":
		u.Path = strings.TrimSuffix(strings.TrimRight(u.Path, "/"), "/v1")
		u.RawPath = ""
		return u.String(), true
	default:
		return "", false
	}
}

func KeyID(apiKey string) string {
	digest := sha256.Sum256([]byte(apiKey))
	return "key-" + hex.EncodeToString(digest[:6])
}

// RuntimeAuthID mirrors CLIProxyAPI v7.2.130's stable OpenAI-compatible auth ID.
// It uses the raw configured base URL, not the normalized usage URL.
func RuntimeAuthID(providerName, apiKey, baseURL, proxyURL string) string {
	kind := "openai-compatibility:" + strings.ToLower(strings.TrimSpace(providerName))
	hasher := sha256.New()
	hasher.Write([]byte(kind))
	for _, part := range []string{apiKey, baseURL, proxyURL} {
		hasher.Write([]byte{0})
		hasher.Write([]byte(strings.TrimSpace(part)))
	}
	digest := hex.EncodeToString(hasher.Sum(nil))
	return kind + ":" + digest[:12]
}
