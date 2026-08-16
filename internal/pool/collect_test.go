package pool

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/duu261/opencode-go-quota/internal/cliproxyconfig"
)

func TestCollectMapsHealthyUnavailableAndDisabled(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch r.Header.Get("Authorization") {
		case "Bearer healthy-key":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"usage":{"rolling":{"status":"ok","percent":8,"resetsAt":"2026-08-16T19:50:55Z"},"weekly":{"status":"ok","percent":29,"resetsAt":"2026-08-17T00:00:00Z"},"monthly":{"status":"ok","percent":14,"resetsAt":"2026-09-13T21:51:50Z"}}}`))
		case "Bearer reseller-key":
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		default:
			http.Error(w, "unexpected key", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	credentials := []cliproxyconfig.Credential{
		{ProviderName: "direct", BaseURL: server.URL, KeyID: "key-healthy", APIKey: "healthy-key", Enabled: true},
		{ProviderName: "reseller", BaseURL: server.URL, KeyID: "key-reseller", APIKey: "reseller-key", Enabled: true},
		{ProviderName: "disabled", BaseURL: server.URL, KeyID: "key-disabled", APIKey: "must-not-be-sent", Enabled: false},
	}

	results := Collect(context.Background(), credentials, server.Client(), 2)
	if len(results) != 3 {
		t.Fatalf("results len = %d", len(results))
	}
	if results[0].Status != StatusHealthy || results[0].Usage == nil || results[0].Usage.Weekly.Percent != 29 {
		t.Fatalf("unexpected healthy result: %#v", results[0])
	}
	if results[1].Status != StatusUnavailable {
		t.Fatalf("unexpected unauthorized result: %#v", results[1])
	}
	if results[2].Status != StatusDisabled {
		t.Fatalf("unexpected disabled result: %#v", results[2])
	}
	for _, index := range []int{1, 2} {
		raw, _ := json.Marshal(results[index])
		if bytes.Contains(raw, []byte(`"usage"`)) {
			t.Fatalf("non-healthy result exposes zero-value usage: %s", raw)
		}
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}
}

func TestCollectNeverCopiesRawKeyIntoResult(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "failure", http.StatusBadGateway)
	}))
	defer server.Close()

	const secret = "raw-secret-must-not-escape"
	results := Collect(context.Background(), []cliproxyconfig.Credential{{
		ProviderName: "direct", BaseURL: server.URL, KeyID: "key-safe", APIKey: secret, Enabled: true,
	}}, server.Client(), 1)

	if len(results) != 1 || results[0].Status != StatusError {
		t.Fatalf("unexpected results: %#v", results)
	}
	if containsSecret(results[0], secret) {
		t.Fatalf("result leaks API key: %#v", results[0])
	}
}

func containsSecret(result Result, secret string) bool {
	raw, _ := json.Marshal(result)
	return bytes.Contains(raw, []byte(secret))
}
