package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

const defaultCLIProxyConfigPath = "/CLIProxyAPI/config.yaml"
const defaultAccountsPath = "/CLIProxyAPI/opencode-accounts.yaml"

type lifecycleRequest struct {
	ConfigYAML []byte `json:"config_yaml"`
}

type pluginConfig struct {
	ConfigPath     string   `yaml:"config_path"`
	AccountsPath   string   `yaml:"accounts_path"`
	ProviderNames  []string `yaml:"provider_names"`
	MaxConcurrency int      `yaml:"max_concurrency"`
	TimeoutSeconds int      `yaml:"timeout_seconds"`
}

var runtimeConfig = struct {
	sync.RWMutex
	value pluginConfig
}{value: defaultPluginConfig()}

func defaultPluginConfig() pluginConfig {
	return pluginConfig{
		ConfigPath:     defaultCLIProxyConfigPath,
		AccountsPath:   defaultAccountsPath,
		MaxConcurrency: 4,
		TimeoutSeconds: 15,
	}
}

func parsePluginConfig(raw []byte) (pluginConfig, error) {
	config := defaultPluginConfig()
	if len(raw) > 0 {
		if err := yaml.Unmarshal(raw, &config); err != nil {
			return pluginConfig{}, fmt.Errorf("decode plugin config: %w", err)
		}
	}
	config.ConfigPath = strings.TrimSpace(config.ConfigPath)
	if config.ConfigPath == "" {
		config.ConfigPath = defaultCLIProxyConfigPath
	}
	config.AccountsPath = strings.TrimSpace(config.AccountsPath)
	if config.AccountsPath == "" {
		config.AccountsPath = defaultAccountsPath
	}
	if config.MaxConcurrency < 1 || config.MaxConcurrency > 16 {
		return pluginConfig{}, fmt.Errorf("max_concurrency must be between 1 and 16")
	}
	if config.TimeoutSeconds < 1 || config.TimeoutSeconds > 60 {
		return pluginConfig{}, fmt.Errorf("timeout_seconds must be between 1 and 60")
	}
	config.ProviderNames = normalizeProviderNames(config.ProviderNames)
	return config, nil
}

func configurePlugin(request []byte) error {
	var lifecycle lifecycleRequest
	if len(request) > 0 {
		if err := json.Unmarshal(request, &lifecycle); err != nil {
			return fmt.Errorf("decode plugin lifecycle request: %w", err)
		}
	}
	config, err := parsePluginConfig(lifecycle.ConfigYAML)
	if err != nil {
		return err
	}
	setPluginConfig(config)
	return nil
}

func currentPluginConfig() pluginConfig {
	runtimeConfig.RLock()
	defer runtimeConfig.RUnlock()
	config := runtimeConfig.value
	config.ProviderNames = append([]string(nil), config.ProviderNames...)
	return config
}

func setPluginConfig(config pluginConfig) {
	config.ProviderNames = append([]string(nil), config.ProviderNames...)
	runtimeConfig.Lock()
	runtimeConfig.value = config
	runtimeConfig.Unlock()
}

func normalizeProviderNames(names []string) []string {
	result := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		key := strings.ToLower(name)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, name)
	}
	return result
}
