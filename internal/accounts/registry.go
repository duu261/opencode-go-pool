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
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

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
	ReferralCredits  int    `json:"referral_credits,omitempty" yaml:"referral_credits,omitempty"`
	ReferralAwarded  bool   `json:"referral_awarded,omitempty" yaml:"referral_awarded,omitempty"`
	ManualHold       bool   `json:"manual_hold,omitempty" yaml:"manual_hold,omitempty"`
	ReferralOnly     bool   `json:"referral_only,omitempty" yaml:"referral_only,omitempty"`
	ExpiresAt        string `json:"expires_at,omitempty" yaml:"expires_at,omitempty"`
	Notes            string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type RegistryState struct {
	Accounts            []Account
	ReferralAwardedKeys map[string]struct{}
}

type registryFile struct {
	Accounts            []Account `yaml:"accounts"`
	ReferralAwardedKeys []string  `yaml:"referral_awarded_keys,omitempty"`
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
	state, revision, err := LoadStateWithRevision(path)
	return state.Accounts, revision, err
}

func LoadStateWithRevision(path string) (RegistryState, string, error) {
	path, err := normalizedPath(path)
	if err != nil {
		return RegistryState{}, "", err
	}
	lock := registryLock(path)
	lock.Lock()
	defer lock.Unlock()
	state, err := loadStateUnlocked(path)
	if err != nil {
		return RegistryState{}, "", err
	}
	return state, revision(state), nil
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
	current, err := loadStateUnlocked(path)
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
	current.Accounts = normalized
	if err := saveStateUnlocked(path, current); err != nil {
		return currentRevision, err
	}
	return revision(current), nil
}

func ReplaceState(path string, state RegistryState, expectedRevision string) (string, error) {
	path, err := normalizedPath(path)
	if err != nil {
		return "", err
	}
	lock := registryLock(path)
	lock.Lock()
	defer lock.Unlock()
	current, err := loadStateUnlocked(path)
	if err != nil {
		return "", err
	}
	currentRevision := revision(current)
	if expectedRevision == "" || expectedRevision != currentRevision {
		return currentRevision, ErrRevisionConflict
	}
	normalized, err := normalizeState(state)
	if err != nil {
		return currentRevision, err
	}
	if err := saveStateUnlocked(path, normalized); err != nil {
		return currentRevision, err
	}
	return revision(normalized), nil
}

func ApplyReferralAwards(current, next []Account) ([]Account, error) {
	awarded := map[string]struct{}{}
	result, _, err := ApplyReferralAwardsWithHistory(current, next, awarded)
	return result, err
}

func ApplyReferralAwardsWithHistory(current, next []Account, awardedKeys map[string]struct{}) ([]Account, map[string]struct{}, error) {
	current, err := normalize(current)
	if err != nil {
		return nil, nil, err
	}
	next, err = normalize(next)
	if err != nil {
		return nil, nil, err
	}
	awarded := cloneKeys(awardedKeys)

	currentByKey := make(map[string]Account, len(current))
	for _, account := range current {
		currentByKey[account.APIKey] = account
	}
	nextIndexByKey := make(map[string]int, len(next))
	for index, account := range next {
		nextIndexByKey[account.APIKey] = index
	}

	for index := range next {
		account := &next[index]
		previous, existed := currentByKey[account.APIKey]
		if existed {
			if previous.ReferralAwarded {
				if account.ReferredByAPIKey != previous.ReferredByAPIKey {
					return nil, nil, fmt.Errorf("account %q cannot change its inviter after reward", account.APIKey)
				}
				account.ReferralAwarded = true
			}
			continue
		}
		if account.ReferredByAPIKey == "" {
			continue
		}
		parentIndex, exists := nextIndexByKey[account.ReferredByAPIKey]
		if !exists {
			return nil, nil, fmt.Errorf("account %q referral parent is not registered", account.APIKey)
		}
		if _, alreadyAwarded := awarded[account.APIKey]; alreadyAwarded {
			account.ReferralAwarded = true
			continue
		}
		account.ReferralCredits++
		next[parentIndex].ReferralCredits++
		account.ReferralAwarded = true
		awarded[account.APIKey] = struct{}{}
	}
	return next, awarded, nil
}

// AdjustReferralCredits applies an explicit operator correction to one account.
func AdjustReferralCredits(accounts []Account, apiKey string, delta int) ([]Account, error) {
	accounts, err := normalize(accounts)
	if err != nil {
		return nil, err
	}
	apiKey = strings.TrimSpace(apiKey)
	for index := range accounts {
		if accounts[index].APIKey != apiKey {
			continue
		}
		next := accounts[index].ReferralCredits + delta
		if next < 0 {
			return nil, fmt.Errorf("account %q referral credits cannot be negative", apiKey)
		}
		accounts[index].ReferralCredits = next
		return accounts, nil
	}
	return nil, fmt.Errorf("account %q is not registered", apiKey)
}

func loadUnlocked(path string) ([]Account, error) {
	state, err := loadStateUnlocked(path)
	return state.Accounts, err
}

func loadStateUnlocked(path string) (RegistryState, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if errors.Is(err, os.ErrNotExist) {
		return RegistryState{Accounts: []Account{}, ReferralAwardedKeys: map[string]struct{}{}}, nil
	}
	if err != nil {
		return RegistryState{}, fmt.Errorf("open account registry: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return RegistryState{}, fmt.Errorf("stat account registry: %w", err)
	}
	if !info.Mode().IsRegular() {
		return RegistryState{}, errors.New("account registry must be a regular file")
	}
	if info.Mode().Perm() != 0o600 {
		if err := file.Chmod(0o600); err != nil {
			return RegistryState{}, fmt.Errorf("secure account registry permissions: %w", err)
		}
	}

	var registry registryFile
	decoder := yaml.NewDecoder(io.LimitReader(file, maxRegistryBytes))
	if err := decoder.Decode(&registry); errors.Is(err, io.EOF) {
		return RegistryState{Accounts: []Account{}, ReferralAwardedKeys: map[string]struct{}{}}, nil
	} else if err != nil {
		return RegistryState{}, fmt.Errorf("decode account registry: %w", err)
	}
	accounts, err := normalize(registry.Accounts)
	if err != nil {
		return RegistryState{}, err
	}
	awarded := make(map[string]struct{}, len(registry.ReferralAwardedKeys))
	for _, key := range registry.ReferralAwardedKeys {
		key = strings.TrimSpace(key)
		if key != "" {
			awarded[key] = struct{}{}
		}
	}
	for _, account := range accounts {
		if account.ReferralAwarded {
			awarded[account.APIKey] = struct{}{}
		}
	}
	return RegistryState{Accounts: accounts, ReferralAwardedKeys: awarded}, nil
}

func saveUnlocked(path string, accounts []Account) error {
	state, err := loadStateUnlocked(path)
	if err != nil {
		return err
	}
	state.Accounts, err = normalize(accounts)
	if err != nil {
		return err
	}
	return saveStateUnlocked(path, state)
}

func saveStateUnlocked(path string, state RegistryState) error {
	state, err := normalizeState(state)
	if err != nil {
		return err
	}
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
	if err := encoder.Encode(registryFile{Accounts: state.Accounts, ReferralAwardedKeys: sortedKeys(state.ReferralAwardedKeys)}); err != nil {
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

func normalizeState(state RegistryState) (RegistryState, error) {
	accounts, err := normalize(state.Accounts)
	if err != nil {
		return RegistryState{}, err
	}
	return RegistryState{Accounts: accounts, ReferralAwardedKeys: cloneKeys(state.ReferralAwardedKeys)}, nil
}

func cloneKeys(keys map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{}, len(keys))
	for key := range keys {
		key = strings.TrimSpace(key)
		if key != "" {
			result[key] = struct{}{}
		}
	}
	return result
}

func sortedKeys(keys map[string]struct{}) []string {
	result := make([]string, 0, len(keys))
	for key := range keys {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func revision(state RegistryState) string {
	raw, _ := json.Marshal(struct {
		Accounts            []Account `json:"accounts"`
		ReferralAwardedKeys []string  `json:"referral_awarded_keys,omitempty"`
	}{Accounts: state.Accounts, ReferralAwardedKeys: sortedKeys(state.ReferralAwardedKeys)})
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
		if account.ReferralCredits < 0 {
			return nil, fmt.Errorf("account %d referral credits cannot be negative", index+1)
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
		if account.ExpiresAt != "" {
			date, err := time.Parse("2006-01-02", account.ExpiresAt)
			if err != nil || date.Format("2006-01-02") != account.ExpiresAt {
				return nil, fmt.Errorf("account %d expires_at must be YYYY-MM-DD", index+1)
			}
		}
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
