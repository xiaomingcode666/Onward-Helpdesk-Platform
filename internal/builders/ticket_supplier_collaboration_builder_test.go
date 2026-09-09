package builders

import (
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/services"
)

func TestBuildPartnerTicketSupplierCollaborationMasksUnauthorizedDiagnosis(t *testing.T) {
	item := services.TicketSupplierCollaborationAggregate{
		Collaboration: &models.TicketSupplierCollaboration{
			ID:             7,
			TicketID:       11,
			VisibilityJSON: `["ticket_summary"]`,
		},
		Ticket: &models.Ticket{
			TicketNo:         "T-001",
			Title:            "设备异常",
			FaultCode:        "SECRET-FAULT",
			SymptomSummary:   "敏感故障现象",
			DiagnosisSummary: "敏感诊断结论",
		},
	}

	result := BuildPartnerTicketSupplierCollaboration(item)
	if result.TicketNo != "T-001" || result.TicketTitle != "设备异常" {
		t.Fatalf("expected ticket summary to remain visible: %+v", result)
	}
	if result.FaultCode != "" || result.SymptomSummary != "" || result.DiagnosisSummary != "" {
		t.Fatalf("expected diagnosis fields to be masked: %+v", result)
	}
}

func TestBuildPartnerTicketSupplierCollaborationMasksCustomerEmailInTitle(t *testing.T) {
	item := services.TicketSupplierCollaborationAggregate{
		Collaboration: &models.TicketSupplierCollaboration{
			VisibilityJSON: `["ticket_summary"]`,
		},
		Ticket: &models.Ticket{Title: "1587237547@qq.com 售后请求"},
	}

	result := BuildPartnerTicketSupplierCollaboration(item)
	if result.TicketTitle != "15********@qq.com 售后请求" {
		t.Fatalf("expected customer email to be masked for partner: %q", result.TicketTitle)
	}
}

func TestBuildPartnerPortalMeetingsCountsFinishedMeetings(t *testing.T) {
	result := BuildPartnerPortalMeetings([]services.PartnerPortalMeetingAggregate{
		{
			Meeting: dto.EnterpriseMeetingListItemDTO{
				ID:               "active-meeting",
				Status:           "active",
				ParticipantCount: 2,
			},
		},
		{
			Meeting: dto.EnterpriseMeetingListItemDTO{
				ID:               "ended-meeting",
				Status:           "ended",
				ParticipantCount: 3,
			},
		},
		{
			Meeting: dto.EnterpriseMeetingListItemDTO{
				ID:     "waiting-meeting",
				Status: "waiting",
			},
		},
	})

	if result.Summary.Active != 1 || result.Summary.Waiting != 1 || result.Summary.Ended != 1 || result.Summary.Mine != 3 {
		t.Fatalf("unexpected meeting summary: %+v", result.Summary)
	}
	if result.Summary.ParticipantsOnline != 5 {
		t.Fatalf("expected five participants in meeting rows, got %d", result.Summary.ParticipantsOnline)
	}
}
