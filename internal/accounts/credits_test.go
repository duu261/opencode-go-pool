package accounts

import "testing"

func TestAdjustReferralCreditsRejectsNegativeBalance(t *testing.T) {
	accounts := []Account{{APIKey: "key", Label: "go-001"}}
	_, err := AdjustReferralCredits(accounts, "key", -1)
	if err == nil {
		t.Fatal("AdjustReferralCredits() error = nil, want negative balance rejection")
	}
}

func TestAdjustReferralCreditsAppliesExplicitCorrection(t *testing.T) {
	accounts := []Account{{APIKey: "key", Label: "go-001", ReferralCredits: 2}}
	next, err := AdjustReferralCredits(accounts, "key", -1)
	if err != nil {
		t.Fatalf("AdjustReferralCredits() error = %v", err)
	}
	if next[0].ReferralCredits != 1 {
		t.Fatalf("ReferralCredits = %d, want 1", next[0].ReferralCredits)
	}
}

func TestNormalizeRejectsNegativeReferralCredits(t *testing.T) {
	if _, err := normalize([]Account{{APIKey: "key", ReferralCredits: -1}}); err == nil {
		t.Fatal("normalize() error = nil, want negative referral credit rejection")
	}
}
