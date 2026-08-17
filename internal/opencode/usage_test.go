package opencode

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientFetchReturnsUsageWindows(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/zen/go/v1/usage" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret-key" {
			t.Fatalf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"usage":{"rolling":{"status":"ok","percent":8,"resetsAt":"2026-08-16T19:50:55.700Z"},"weekly":{"status":"ok","percent":29,"resetsAt":"2026-08-17T00:00:00.700Z"},"monthly":{"status":"ok","percent":14,"resetsAt":"2026-09-13T21:51:50.700Z"}}}`))
	}))
	defer server.Close()

	client := Client{BaseURL: server.URL + "/zen/go", HTTPClient: server.Client()}
	usage, err := client.Fetch(context.Background(), "secret-key")
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if usage.Rolling.Percent != 8 || usage.Weekly.Percent != 29 || usage.Monthly.Percent != 14 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
	wantReset := time.Date(2026, 9, 13, 21, 51, 50, 700_000_000, time.UTC)
	if !usage.Monthly.ResetsAt.Equal(wantReset) {
		t.Fatalf("monthly reset = %s, want %s", usage.Monthly.ResetsAt, wantReset)
	}
}

func TestClientAlwaysAppendsUsagePathToProvidedBaseURL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/custom/v1/v1/usage" {
			t.Fatalf("path = %q, want /custom/v1/v1/usage", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"usage":{"rolling":{"status":"ok","percent":1,"resetsAt":"2026-08-16T19:50:55Z"},"weekly":{"status":"ok","percent":2,"resetsAt":"2026-08-17T00:00:00Z"},"monthly":{"status":"ok","percent":3,"resetsAt":"2026-09-13T21:51:50Z"}}}`))
	}))
	defer server.Close()

	client := Client{BaseURL: server.URL + "/custom/v1", HTTPClient: server.Client()}
	if _, err := client.Fetch(context.Background(), "secret-key"); err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
}

func TestClientFetchMapsUnauthorizedWithoutCallingItExhausted(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":{"message":"Unauthorized"}}`, http.StatusUnauthorized)
	}))
	defer server.Close()

	client := Client{BaseURL: server.URL, HTTPClient: server.Client()}
	_, err := client.Fetch(context.Background(), "reseller-key")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("Fetch() error = %v, want ErrUnauthorized", err)
	}
}
