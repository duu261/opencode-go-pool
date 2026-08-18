package main

import (
	"testing"

	"github.com/duu261/opencode-go-pool/internal/opencode"
)

func TestQuotaStateDistinguishesPartialAndFullExhaustion(t *testing.T) {
	usage := &opencode.Usage{
		Rolling: opencode.Window{Percent: 0},
		Weekly:  opencode.Window{Percent: 100},
		Monthly: opencode.Window{Percent: 14},
	}
	if got := quotaState(usage, "ok"); got != "quota_limited" {
		t.Fatalf("quotaState() = %q, want quota_limited", got)
	}
	usage.Rolling.Percent = 100
	usage.Monthly.Percent = 100
	if got := quotaState(usage, "ok"); got != "quota_depleted" {
		t.Fatalf("quotaState() = %q, want quota_depleted", got)
	}
	if got := quotaState(nil, "unavailable"); got != "unavailable" {
		t.Fatalf("quotaState() unavailable = %q, want unavailable", got)
	}
}
