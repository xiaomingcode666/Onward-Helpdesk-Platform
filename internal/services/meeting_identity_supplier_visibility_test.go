package services

import (
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
)

func TestSupplierVisibilityAllowsLegacyEmptyScope(t *testing.T) {
	legacy := &models.TicketSupplierCollaboration{}
	if !supplierVisibilityAllows(legacy, "meeting") {
		t.Fatal("legacy supplier collaboration should retain meeting visibility")
	}

	limited := &models.TicketSupplierCollaboration{VisibilityJSON: `["ticket_summary"]`}
	if supplierVisibilityAllows(limited, "meeting") {
		t.Fatal("ticket-only supplier collaboration must not expose meetings")
	}
}

func TestMeetingParticipantIdentityRecognizesPlatformRoles(t *testing.T) {
	operator := &dto.AuthPrincipal{Roles: []string{constants.RoleCodeSuperAdmin}}
	if !isPlatformMeetingOperator(operator) {
		t.Fatal("super admin must be recorded as a platform administrator")
	}

	participant := &models.MeetingParticipant{UserType: meetingParticipantTypePlatformAdmin}
	if got := meetingParticipantIdentityType(nil, 0, participant); got != meetingParticipantTypePlatformAdmin {
		t.Fatalf("explicit platform participant identity = %q", got)
	}
}
