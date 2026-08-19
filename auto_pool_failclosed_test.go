package main

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestSchedulerFailsClosedWhenStateDiscoveryFails(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing-config.yaml")
	previous := currentPluginConfig()
	setPluginConfig(pluginConfig{ConfigPath: configPath, AccountsPath: filepath.Join(t.TempDir(), "missing-accounts.yaml"), AutoPool: true})
	defer setPluginConfig(previous)
	resetHeldCacheForTest()

	request, err := json.Marshal(schedulerPickRequest{Provider: "openai-compatibility:opencode-go", Candidates: []schedulerCandidate{{ID: "auth-1", Provider: "openai-compatibility:opencode-go"}}})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := handleSchedulerPick(request)
	if err != nil {
		t.Fatalf("handleSchedulerPick() error = %v", err)
	}
	var result envelope
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if result.OK || result.Error == nil || result.Error.Code != "opencode_pool_state_unavailable" {
		t.Fatalf("scheduler response = %#v, want fail-closed error", result)
	}
}

func TestHeldSnapshotFailsClosedWhenDiscoveryFails(t *testing.T) {
	state := &autoPoolState{blocked: map[string]time.Time{}, held: map[string]bool{}}
	config := pluginConfig{ConfigPath: filepath.Join(t.TempDir(), "missing-config.yaml"), AccountsPath: filepath.Join(t.TempDir(), "missing-accounts.yaml")}
	held, available := state.heldSnapshot(config, time.Now())
	if available || held != nil {
		t.Fatalf("heldSnapshot() = %#v, %v; want nil, false", held, available)
	}
}
