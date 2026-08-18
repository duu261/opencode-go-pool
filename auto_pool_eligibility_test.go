package main

import (
	"testing"
	"time"
)

func TestEligibleAutoPoolCandidatesFiltersBeforeRotation(t *testing.T) {
	now := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	candidates := []schedulerCandidate{{ID: "a"}, {ID: "blocked"}, {ID: "c"}}
	eligible := eligibleAutoPoolCandidates(candidates, map[string]time.Time{"blocked": now.Add(time.Hour)}, nil, now)
	if len(eligible) != 2 || eligible[0].ID != "a" || eligible[1].ID != "c" {
		t.Fatalf("eligibleAutoPoolCandidates() = %#v, want [a c]", eligible)
	}
}
