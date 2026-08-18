package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestManagementPageUsesDensePoolOperatorTable(t *testing.T) {
	raw, err := handleMethod("management.handle", []byte(`{"Method":"GET","Path":"/v0/resource/plugins/opencode-go-quota/status"}`))
	if err != nil {
		t.Fatalf("handleMethod() error = %v", err)
	}
	var envelope testEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	var response struct {
		Body []byte `json:"Body"`
	}
	if err := json.Unmarshal(envelope.Result, &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	for _, want := range []string{"account-table", "Attention", "Referral credits", "Monthly", "Credits", "Add reward", "Manual hold", "auto_state", "referral_credits", "referral_awarded", "manual_hold", "cell=text('div','','quota')", "copyField", "Show", "expiresInDays(31)", "Bulk add", "Paste TSV rows", "Import accepted accounts", "parseBulk", "duplicate in paste"} {
		if !bytes.Contains(response.Body, []byte(want)) {
			t.Fatalf("management page missing %q", want)
		}
	}
}
