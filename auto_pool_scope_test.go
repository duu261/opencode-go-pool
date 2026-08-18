package main

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestHandleSchedulerPickDelegatesForUnrelatedProvider(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "cliproxy.yaml")
	previous := currentPluginConfig()
	setPluginConfig(pluginConfig{ConfigPath: configPath, AccountsPath: filepath.Join(t.TempDir(), "accounts.yaml"), AutoPool: true})
	defer setPluginConfig(previous)

	raw, err := json.Marshal(schedulerPickRequest{Provider: "anthropic", Providers: []string{"openai-compatibility:opencode-go"}, Candidates: []schedulerCandidate{
		{ID: "claude-key", Provider: "anthropic"},
		{ID: "go-key", Provider: "openai-compatibility:opencode-go"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	responseRaw, err := handleSchedulerPick(raw)
	if err != nil {
		t.Fatalf("handleSchedulerPick() error = %v", err)
	}
	var envelope testEnvelope
	if err := json.Unmarshal(responseRaw, &envelope); err != nil {
		t.Fatal(err)
	}
	var response schedulerPickResponse
	if err := json.Unmarshal(envelope.Result, &response); err != nil {
		t.Fatal(err)
	}
	if response.Handled {
		t.Fatalf("handleSchedulerPick() handled unrelated provider: %#v", response)
	}
}
