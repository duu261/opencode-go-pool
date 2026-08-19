package main

import (
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/duu261/opencode-go-pool/internal/accounts"
	"github.com/duu261/opencode-go-pool/internal/cliproxyconfig"
)

type schedulerCandidate struct {
	ID         string            `json:"ID"`
	Provider   string            `json:"Provider"`
	Status     string            `json:"Status"`
	Attributes map[string]string `json:"Attributes"`
}

type schedulerPickRequest struct {
	Provider   string               `json:"Provider"`
	Providers  []string             `json:"Providers"`
	Model      string               `json:"Model"`
	Candidates []schedulerCandidate `json:"Candidates"`
}

type schedulerPickResponse struct {
	AuthID  string `json:"AuthID,omitempty"`
	Handled bool   `json:"Handled"`
}

type usageFailure struct {
	StatusCode int    `json:"StatusCode"`
	Body       string `json:"Body"`
}

type usageRecord struct {
	Provider string       `json:"Provider"`
	Model    string       `json:"Model"`
	AuthID   string       `json:"AuthID"`
	Failed   bool         `json:"Failed"`
	Failure  usageFailure `json:"Failure"`
}

type autoPoolState struct {
	mu        sync.RWMutex
	blocked   map[string]time.Time
	held      map[string]bool
	heldUntil time.Time
	cursor    uint64
}

var runtimeAutoPool = autoPoolState{blocked: make(map[string]time.Time), held: make(map[string]bool)}

var quotaResetPattern = regexp.MustCompile(`(?i)(?:resets?|try\s+again)\s+in\s+(\d+)\s*(min(?:ute)?s?|h(?:our)?s?|d(?:ay)?s?)`)

var accountExpiryLocation = time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)

func accountExpired(account accounts.Account, now time.Time) bool {
	date, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(account.ExpiresAt), accountExpiryLocation)
	if err != nil {
		return false
	}
	// ponytail: date-only expiry means usable through that Vietnam calendar day.
	return !now.Before(date.AddDate(0, 0, 1))
}

func eligibleAutoPoolCandidates(candidates []schedulerCandidate, blocked map[string]time.Time, held map[string]bool, now time.Time) []schedulerCandidate {
	eligible := make([]schedulerCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.ID) == "" || held[candidate.ID] {
			continue
		}
		if resetAt, exists := blocked[candidate.ID]; exists && resetAt.After(now) {
			continue
		}
		eligible = append(eligible, candidate)
	}
	return eligible
}

func selectAutoPoolCandidate(candidates []schedulerCandidate, blocked map[string]time.Time, held map[string]bool, now time.Time) (schedulerCandidate, bool) {
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.ID) == "" || held[candidate.ID] {
			continue
		}
		if resetAt, exists := blocked[candidate.ID]; exists && resetAt.After(now) {
			continue
		}
		return candidate, true
	}
	return schedulerCandidate{}, false
}

func rotateAutoPoolCandidates(candidates []schedulerCandidate, start int) []schedulerCandidate {
	if len(candidates) == 0 {
		return nil
	}
	start %= len(candidates)
	if start < 0 {
		start += len(candidates)
	}
	rotated := make([]schedulerCandidate, 0, len(candidates))
	rotated = append(rotated, candidates[start:]...)
	rotated = append(rotated, candidates[:start]...)
	return rotated
}

func parseQuotaReset(body string, now time.Time) (time.Time, bool) {
	match := quotaResetPattern.FindStringSubmatch(body)
	if len(match) != 3 {
		return time.Time{}, false
	}
	var amount int
	for _, char := range match[1] {
		amount = amount*10 + int(char-'0')
	}
	var duration time.Duration
	switch strings.ToLower(match[2][:1]) {
	case "m":
		duration = time.Duration(amount) * time.Minute
	case "h":
		duration = time.Duration(amount) * time.Hour
	case "d":
		duration = time.Duration(amount) * 24 * time.Hour
	default:
		return time.Time{}, false
	}
	return now.Add(duration), true
}

func (s *autoPoolState) nextCandidateOrder(candidates []schedulerCandidate) []schedulerCandidate {
	if len(candidates) == 0 {
		return nil
	}
	s.mu.Lock()
	start := int(s.cursor % uint64(len(candidates)))
	s.cursor++
	s.mu.Unlock()
	return rotateAutoPoolCandidates(candidates, start)
}

func (s *autoPoolState) markUsage(record usageRecord, now time.Time) {
	if s == nil || strings.TrimSpace(record.AuthID) == "" || !record.Failed || record.Failure.StatusCode < 429 {
		return
	}
	body := strings.ToLower(record.Failure.Body)
	if !strings.Contains(body, "usage limit") && !strings.Contains(body, "weekly") {
		return
	}
	resetAt, ok := parseQuotaReset(record.Failure.Body, now)
	if !ok {
		return
	}
	s.mu.Lock()
	if current, exists := s.blocked[record.AuthID]; !exists || resetAt.After(current) {
		s.blocked[record.AuthID] = resetAt
	}
	s.mu.Unlock()
}

func (s *autoPoolState) blockedSnapshot() map[string]time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]time.Time, len(s.blocked))
	for key, value := range s.blocked {
		result[key] = value
	}
	return result
}

func (s *autoPoolState) clear(authID string) bool {
	authID = strings.TrimSpace(authID)
	if s == nil || authID == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.blocked[authID]; !exists {
		return false
	}
	delete(s.blocked, authID)
	return true
}

func (s *autoPoolState) state(authID string, now time.Time) (string, time.Time) {
	s.mu.RLock()
	resetAt, exists := s.blocked[authID]
	s.mu.RUnlock()
	if !exists || !resetAt.After(now) {
		return "ready", time.Time{}
	}
	return "cooling", resetAt
}

func autoStateView(authID string) (string, *time.Time) {
	state, resetAt := runtimeAutoPool.state(authID, time.Now())
	if resetAt.IsZero() {
		return state, nil
	}
	return state, &resetAt
}

func (s *autoPoolState) heldSnapshot(config pluginConfig, now time.Time) (map[string]bool, bool) {
	s.mu.RLock()
	if now.Before(s.heldUntil) {
		result := make(map[string]bool, len(s.held))
		for key, value := range s.held {
			result[key] = value
		}
		s.mu.RUnlock()
		return result, true
	}
	s.mu.RUnlock()

	registry, errRegistry := accounts.Load(config.AccountsPath)
	credentials, errCredentials := cliproxyconfig.Discover(config.ConfigPath, config.ProviderNames)
	if errRegistry != nil || errCredentials != nil {
		return nil, false
	}
	credentialByKey := make(map[string]cliproxyconfig.Credential, len(credentials))
	for _, credential := range credentials {
		credentialByKey[credential.APIKey] = credential
	}
	held := make(map[string]bool)
	for _, account := range registry {
		if !account.ManualHold && !account.ReferralOnly && !accountExpired(account, now) {
			continue
		}
		if credential, exists := credentialByKey[account.APIKey]; exists && credential.AuthID != "" {
			held[credential.AuthID] = true
		}
	}
	s.mu.Lock()
	s.held = held
	s.heldUntil = now.Add(time.Second)
	s.mu.Unlock()
	return held, true
}

func isOpenCodeProviderKey(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(value, "openai-compatibility:") && strings.Contains(value, "opencode")
}

func schedulerRequestTargetsOpenCode(request schedulerPickRequest) bool {
	if strings.TrimSpace(request.Provider) != "" {
		return isOpenCodeProviderKey(request.Provider)
	}
	for _, provider := range request.Providers {
		if isOpenCodeProviderKey(provider) {
			return true
		}
	}
	return false
}

func isOpenCodeCandidate(candidate schedulerCandidate) bool {
	provider := strings.ToLower(strings.TrimSpace(candidate.Provider))
	if !strings.HasPrefix(provider, "openai-compatibility:") {
		return false
	}
	compatName := strings.ToLower(strings.TrimSpace(candidate.Attributes["compat_name"]))
	baseURL := strings.ToLower(strings.TrimSpace(candidate.Attributes["base_url"]))
	return strings.Contains(provider, "opencode") || strings.Contains(compatName, "opencode") || strings.Contains(baseURL, "opencode.ai")
}

func handleSchedulerPick(raw []byte) ([]byte, error) {
	config := currentPluginConfig()
	if !config.AutoPool {
		return okEnvelope(schedulerPickResponse{Handled: false})
	}
	var request schedulerPickRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		return nil, err
	}
	if !schedulerRequestTargetsOpenCode(request) {
		return okEnvelope(schedulerPickResponse{Handled: false})
	}
	now := time.Now()
	candidates := make([]schedulerCandidate, 0, len(request.Candidates))
	for _, candidate := range request.Candidates {
		if isOpenCodeCandidate(candidate) {
			candidates = append(candidates, candidate)
		}
	}
	blocked := runtimeAutoPool.blockedSnapshot()
	held, holdsAvailable := runtimeAutoPool.heldSnapshot(config, now)
	if !holdsAvailable {
		return errorEnvelope("opencode_pool_state_unavailable", "OpenCode account state could not be loaded safely"), nil
	}
	eligible := eligibleAutoPoolCandidates(candidates, blocked, held, now)
	candidate, ok := selectAutoPoolCandidate(runtimeAutoPool.nextCandidateOrder(eligible), nil, nil, now)
	if !ok {
		return errorEnvelope("no_eligible_opencode_credentials", "all OpenCode credentials are cooling, held, expired, or referral-only"), nil
	}
	return okEnvelope(schedulerPickResponse{AuthID: candidate.ID, Handled: true})
}

func handleUsage(raw []byte) ([]byte, error) {
	var record usageRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return nil, err
	}
	runtimeAutoPool.markUsage(record, time.Now())
	return okEnvelope(map[string]any{})
}
