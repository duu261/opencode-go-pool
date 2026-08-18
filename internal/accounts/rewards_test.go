package accounts

import "testing"

func TestApplyReferralAwardsCreditsNewInviteeAndInviterOnce(t *testing.T) {
	parent := Account{APIKey: "parent-key", Label: "go-001"}
	child := Account{APIKey: "child-key", Label: "go-002", ReferredByAPIKey: parent.APIKey}

	next, err := ApplyReferralAwards([]Account{parent}, []Account{parent, child})
	if err != nil {
		t.Fatalf("ApplyReferralAwards() error = %v", err)
	}
	if next[0].ReferralCredits != 1 || next[1].ReferralCredits != 1 || !next[1].ReferralAwarded {
		t.Fatalf("first award = %#v, want one credit for inviter and invitee", next)
	}

	again, err := ApplyReferralAwards(next, next)
	if err != nil {
		t.Fatalf("ApplyReferralAwards() second error = %v", err)
	}
	if again[0].ReferralCredits != 1 || again[1].ReferralCredits != 1 || !again[1].ReferralAwarded {
		t.Fatalf("second award changed balances: %#v", again)
	}
}

func TestApplyReferralAwardsRejectsChangingAwardedInviter(t *testing.T) {
	parentA := Account{APIKey: "parent-a", Label: "go-001", ReferralCredits: 1}
	parentB := Account{APIKey: "parent-b", Label: "go-002"}
	child := Account{APIKey: "child-key", Label: "go-003", ReferredByAPIKey: parentA.APIKey, ReferralCredits: 1, ReferralAwarded: true}

	_, err := ApplyReferralAwards(
		[]Account{parentA, parentB, child},
		[]Account{parentA, parentB, Account{APIKey: child.APIKey, Label: child.Label, ReferredByAPIKey: parentB.APIKey, ReferralCredits: child.ReferralCredits, ReferralAwarded: true}},
	)
	if err == nil {
		t.Fatal("ApplyReferralAwards() error = nil, want awarded inviter to be immutable")
	}
}

func TestApplyReferralAwardsWithHistoryDoesNotReawardReaddedInvitee(t *testing.T) {
	parent := Account{APIKey: "parent-key", Label: "go-001"}
	child := Account{APIKey: "child-key", Label: "go-002", ReferredByAPIKey: parent.APIKey}
	awarded := map[string]struct{}{child.APIKey: {}}

	next, history, err := ApplyReferralAwardsWithHistory([]Account{parent}, []Account{parent, child}, awarded)
	if err != nil {
		t.Fatalf("ApplyReferralAwardsWithHistory() error = %v", err)
	}
	if next[0].ReferralCredits != 0 || next[1].ReferralCredits != 0 || !next[1].ReferralAwarded {
		t.Fatalf("re-added award changed balances: %#v", next)
	}
	if _, ok := history[child.APIKey]; !ok {
		t.Fatalf("history lost awarded key: %#v", history)
	}
}
