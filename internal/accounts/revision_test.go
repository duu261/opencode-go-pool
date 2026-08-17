package accounts

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestReplaceRejectsStaleRevisionWithoutLosingNewerWrite(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "accounts.yaml")
	if err := Save(path, []Account{{APIKey: "initial"}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	_, revision, err := LoadWithRevision(path)
	if err != nil {
		t.Fatalf("LoadWithRevision() error = %v", err)
	}
	if revision == "" {
		t.Fatal("revision is empty")
	}

	newRevision, err := Replace(path, []Account{{APIKey: "first-writer"}}, revision)
	if err != nil {
		t.Fatalf("first Replace() error = %v", err)
	}
	if newRevision == revision {
		t.Fatal("revision did not change after replacement")
	}
	if _, err := Replace(path, []Account{{APIKey: "stale-writer"}}, revision); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale Replace() error = %v, want ErrRevisionConflict", err)
	}

	stored, storedRevision, err := LoadWithRevision(path)
	if err != nil {
		t.Fatalf("LoadWithRevision() error = %v", err)
	}
	if len(stored) != 1 || stored[0].APIKey != "first-writer" {
		t.Fatalf("stored accounts = %#v, newer write was lost", stored)
	}
	if storedRevision != newRevision {
		t.Fatalf("stored revision = %q, want %q", storedRevision, newRevision)
	}
}
