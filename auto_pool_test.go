package main

import (
	"testing"
	"time"
)

func TestSelectAutoPoolCandidateSkipsBlockedAndHeld(t *testing.T) {
	now := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	candidates := []schedulerCandidate{
		{ID: "blocked", Provider: "openai-compatibility:opencode-go"},
		{ID: "held", Provider: "openai-compatibility:opencode-go"},
		{ID: "healthy", Provider: "openai-compatibility:opencode-go"},
	}
	blocked := map[string]time.Time{"blocked": now.Add(time.Hour)}
	held := map[string]bool{"held": true}

	got, ok := selectAutoPoolCandidate(candidates, blocked, held, now)
	if !ok || got.ID != "healthy" {
		t.Fatalf("selectAutoPoolCandidate() = %#v, %v; want healthy, true", got, ok)
	}
}

func TestSelectAutoPoolCandidateDelegatesWhenAllBlocked(t *testing.T) {
	now := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	candidates := []schedulerCandidate{{ID: "blocked", Provider: "openai-compatibility:opencode-go"}}
	blocked := map[string]time.Time{"blocked": now.Add(time.Hour)}

	_, ok := selectAutoPoolCandidate(candidates, blocked, nil, now)
	if ok {
		t.Fatal("selectAutoPoolCandidate() handled all-blocked pool, want native fallback")
	}
}

func TestParseQuotaResetUnderstandsFiveHourAndWeeklyMessages(t *testing.T) {
	now := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		body string
		want time.Duration
	}{
		{body: "5-hour usage limit reached. Resets in 4min.", want: 4 * time.Minute},
		{body: "Weekly usage limit reached. Resets in 5 days.", want: 5 * 24 * time.Hour},
		{body: "Usage limit reached. Try again in 2 hours.", want: 2 * time.Hour},
	} {
		got, ok := parseQuotaReset(test.body, now)
		if !ok || !got.Equal(now.Add(test.want)) {
			t.Fatalf("parseQuotaReset(%q) = %v, %v; want %v, true", test.body, got, ok, now.Add(test.want))
		}
	}
}
