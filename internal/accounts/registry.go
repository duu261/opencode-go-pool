package accounts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"gopkg.in/yaml.v3"
)

const maxRegistryBytes = 8 << 20

var (
	ErrRevisionConflict = errors.New("account registry revision conflict")
	registryLocks       sync.Map
)

type Account struct {
	APIKey           string `json:"api_key" yaml:"api_key"`
	ProviderName     string `json:"provider_name,omitempty" yaml:"provider_name,omitempty"`
	Label            string `json:"label,omitempty" yaml:"label,omitempty"`
	Email            string `json:"email,omitempty" yaml:"email,omitempty"`
	Password         string `json:"password,omitempty" yaml:"password,omitempty"`
	ReferralURL      string `json:"referral_url,omitempty" yaml:"referral_url,omitempty"`
	ReferredByAPIKey string `json:"referred_by_api_key,omitempty" yaml:"referred_by_api_key,omitempty"`
	ExpiresAt        string `json:"expires_at,omitempty" yaml:"expires_at,omitempty"`
	Notes            string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type registryFile struct {
	Accounts []Account `yaml:"accounts"`
}

func Load(path string) ([]Account, error) {
	path, err := normalizedPath(path)
	if err != nil {
		return nil, err
	}
	lock := registryLock(path)
	lock.Lock()
	defer lock.Unlock()
	return loadUnlocked(path)
}

func LoadWithRevision(path string) ([]Account, string, error) {
	path, err := normalizedPath(path)
	if err != nil {
		return nil, "", err
	}
	lock := registryLock(path)
	lock.Lock()
	defer lock.Unlock()
	accounts, err := loadUnlocked(path)
	if err != nil {
		return nil, "", err
	}
	return accounts, revision(accounts), nil
}

func Save(path string, accounts []Account) error {
	path, err := normalizedPath(path)
	if err != nil {
		return err
	}
	lock := registryLock(path)
	lock.Lock()
	defer lock.Unlock()
	return saveUnlocked(path, accounts)
}

func Replace(path string, accounts []Account, expectedRevision string) (string, error) {
	path, err := normalizedPath(path)
	if err != nil {
		return "", err
	}
	lock := registryLock(path)
	lock.Lock()
	defer lock.Unlock()
	current, err := loadUnlocked(path)
	if err != nil {
		return "", err
	}
	currentRevision := revision(current)
	if expectedRevision == "" || expectedRevision != currentRevision {
		return currentRevision, ErrRevisionConflict
	}
	normalized, err := normalize(accounts)
	if err != nil {
		return currentRevision, err
	}
	if err := saveNormalizedUnlocked(path, normalized); err != nil {
		return currentRevision, err
	}
	return revision(normalized), nil
}

func loadUnlocked(path string) ([]Account, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if errors.Is(err, os.ErrNotExist) {
		return []Account{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open account registry: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat account registry: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("account registry must be a regular file")
	}
	if info.Mode().Perm() != 0o600 {
		if err := file.Chmod(0o600); err != nil {
			return nil, fmt.Errorf("secure account registry permissions: %w", err)
		}
	}

	var registry registryFile
	decoder := yaml.NewDecoder(io.LimitReader(file, maxRegistryBytes))
	if err := decoder.Decode(&registry); errors.Is(err, io.EOF) {
		return []Account{}, nil
	} else if err != nil {
		return nil, fmt.Errorf("decode account registry: %w", err)
	}
	accounts, err := normalize(registry.Accounts)
	if err != nil {
		return nil, err
	}
	return accounts, nil
}

func saveUnlocked(path string, accounts []Account) error {
	accounts, err := normalize(accounts)
	if err != nil {
		return err
	}
	return saveNormalizedUnlocked(path, accounts)
}

func saveNormalizedUnlocked(path string, accounts []Account) error {
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, ".opencode-accounts-*.tmp")
	if err != nil {
		return fmt.Errorf("create account registry temp file: %w", err)
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)

	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return fmt.Errorf("set account registry permissions: %w", err)
	}
	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)
	if err := encoder.Encode(registryFile{Accounts: accounts}); err != nil {
		file.Close()
		return fmt.Errorf("encode account registry: %w", err)
	}
	if err := encoder.Close(); err != nil {
		file.Close()
		return fmt.Errorf("close account registry encoder: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync account registry: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close account registry: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace account registry: %w", err)
	}
	return nil
}

func normalizedPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("account registry path is required")
	}
	return filepath.Clean(path), nil
}

func registryLock(path string) *sync.Mutex {
	value, _ := registryLocks.LoadOrStore(path, &sync.Mutex{})
	return value.(*sync.Mutex)
}

func revision(accounts []Account) string {
	raw, _ := json.Marshal(accounts)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func normalize(accounts []Account) ([]Account, error) {
	result := make([]Account, 0, len(accounts))
	seen := make(map[string]struct{}, len(accounts))
	for index, account := range accounts {
		account.APIKey = strings.TrimSpace(account.APIKey)
		if account.APIKey == "" {
			return nil, fmt.Errorf("account %d API key is required", index+1)
		}
		if _, exists := seen[account.APIKey]; exists {
			return nil, fmt.Errorf("duplicate account API key at item %d", index+1)
		}
		seen[account.APIKey] = struct{}{}
		account.ProviderName = strings.TrimSpace(account.ProviderName)
		account.Label = strings.TrimSpace(account.Label)
		account.Email = strings.TrimSpace(account.Email)
		account.ReferralURL = strings.TrimSpace(account.ReferralURL)
		account.ReferredByAPIKey = strings.TrimSpace(account.ReferredByAPIKey)
		account.ExpiresAt = strings.TrimSpace(account.ExpiresAt)
		account.Notes = strings.TrimSpace(account.Notes)
		result = append(result, account)
	}
	for index, account := range result {
		if account.ReferredByAPIKey == "" {
			continue
		}
		if account.ReferredByAPIKey == account.APIKey {
			return nil, fmt.Errorf("account %d cannot refer itself", index+1)
		}
		if _, exists := seen[account.ReferredByAPIKey]; !exists {
			return nil, fmt.Errorf("account %d referral parent is not registered", index+1)
		}
	}
	return result, nil
}
