package builders

import (
	"testing"
	"time"

	"remotehelpdesk/internal/models"
)

func TestBuildPlatformAuditLogDowngradesLegacySuccessfulRoutineIAMRisk(t *testing.T) {
	t.Parallel()

	cases := []struct {
		action string
		want   string
	}{
		{action: models.AuditActionTenantCreated, want: models.RiskLevelMedium},
		{action: "auth_policy.saved", want: models.RiskLevelMedium},
		{action: "auth_role.created", want: models.RiskLevelMedium},
		{action: "customer_user.authorized", want: models.RiskLevelMedium},
		{action: "partner_admin.invited", want: models.RiskLevelMedium},
		{action: "platform_staff.invited", want: models.RiskLevelMedium},
		{action: "tenant_member.invited", want: models.RiskLevelMedium},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.action, func(t *testing.T) {
			t.Parallel()

			result := BuildPlatformAuditLog(&models.AuthAuditLog{
				Action:     tt.action,
				RiskLevel:  models.RiskLevelHigh,
				Status:     models.AuditStatusSuccess,
				OccurredAt: time.Now(),
			})

			if result.RiskLevel != tt.want {
				t.Fatalf("risk level = %q, want %q", result.RiskLevel, tt.want)
			}
		})
	}
}

func TestBuildPlatformAuditLogKeepsHighRiskForSensitiveAndFailedEvents(t *testing.T) {
	t.Parallel()

	sensitive := BuildPlatformAuditLog(&models.AuthAuditLog{
		Action:     "platform_staff.tenant_granted",
		RiskLevel:  models.RiskLevelCritical,
		Status:     models.AuditStatusSuccess,
		OccurredAt: time.Now(),
	})
	if sensitive.RiskLevel != models.RiskLevelCritical {
		t.Fatalf("sensitive risk level = %q, want critical", sensitive.RiskLevel)
	}

	failed := BuildPlatformAuditLog(&models.AuthAuditLog{
		Action:     "tenant_member.invited",
		RiskLevel:  models.RiskLevelHigh,
		Status:     models.AuditStatusFailure,
		OccurredAt: time.Now(),
	})
	if failed.RiskLevel != models.RiskLevelHigh {
		t.Fatalf("failed risk level = %q, want high", failed.RiskLevel)
	}
}
