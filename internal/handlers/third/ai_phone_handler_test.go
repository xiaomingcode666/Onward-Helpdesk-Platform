package third

import "testing"

func TestAIPhoneTicketRequestValidate(t *testing.T) {
	valid := aiPhoneTicketRequest{
		SchemaVersion: 1,
		Provider:      "china-telecom",
		CallID:        "call-1",
		IntentID:      "intent-1",
		Language:      "zh-CN",
		WorkLanguage:  "zh-CN",
		Title:         "设备无法启动",
		Description:   "客户反馈设备无法启动",
		ProjectKey:    "support",
		TicketType:    "incident",
	}
	if err := valid.validate(); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	valid.SchemaVersion = 2
	if err := valid.validate(); err == nil {
		t.Fatal("unsupported schema version must be rejected")
	}
	valid.SchemaVersion = 1
	valid.Title = string(make([]rune, 20001))
	if err := valid.validate(); err == nil {
		t.Fatal("oversized title must be rejected")
	}
}

func TestAIPhoneCredentialAndLineBinding(t *testing.T) {
	t.Setenv("AI_PHONE_SERVICE_TOKEN", "12345678901234567890123456789012")
	t.Setenv("AI_PHONE_LINE_TENANTS", `{"line-1":42}`)

	if validAIPhoneServiceToken("Bearer 12345678901234567890123456789012") != true {
		t.Fatal("configured service token should be accepted")
	}
	if validAIPhoneServiceToken("Bearer wrong") {
		t.Fatal("wrong service token should be rejected")
	}
	if tenantID, ok := aiPhoneTenantForLine("line-1"); !ok || tenantID != 42 {
		t.Fatalf("line binding = (%d, %v), want (42, true)", tenantID, ok)
	}
	if _, ok := aiPhoneTenantForLine("line-2"); ok {
		t.Fatal("unbound line should be rejected")
	}
}

func TestAIPhoneServiceAccountIDIsConfigurationDriven(t *testing.T) {
	t.Setenv("AI_PHONE_SERVICE_ACCOUNT_ID", "27")
	if got := aiPhoneServiceAccountID(); got != 27 {
		t.Fatalf("service account id = %d, want 27", got)
	}
	t.Setenv("AI_PHONE_SERVICE_ACCOUNT_ID", "not-a-number")
	if got := aiPhoneServiceAccountID(); got != 0 {
		t.Fatalf("invalid service account id = %d, want 0", got)
	}
}
