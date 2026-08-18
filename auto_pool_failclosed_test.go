package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestHeldSnapshotFailsClosedWhenDiscoveryFails(t *testing.T) {
	state := &autoPoolState{blocked: map[string]time.Time{}, held: map[string]bool{}}
	config := pluginConfig{ConfigPath: filepath.Join(t.TempDir(), "missing-config.yaml"), AccountsPath: filepath.Join(t.TempDir(), "missing-accounts.yaml")}
	held, available := state.heldSnapshot(config, time.Now())
	if available || held != nil {
		t.Fatalf("heldSnapshot() = %#v, %v; want nil, false", held, available)
	}
}
