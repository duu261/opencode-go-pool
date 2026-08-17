package accounts

import (
	"path/filepath"
	"testing"
)

func TestSavePreservesPasswordWhitespace(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "accounts.yaml")
	if err := Save(path, []Account{{APIKey: "key", Password: " leading and trailing "}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got) != 1 || got[0].Password != " leading and trailing " {
		t.Fatalf("password = %q, want whitespace preserved", got[0].Password)
	}
}
