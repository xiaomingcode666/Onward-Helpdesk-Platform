package services

import (
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
)

func TestGovernedSLAPauseRejectsGenericPendingAndMissingEvidence(t *testing.T) {
	db := setupSLATenantTestDB(t)
	now := time.Now().UTC()
	ticket := models.Ticket{
		TicketNo: "SLA-PAUSE-GUARD", TenantID: 1, Status: enums.TicketStatusInProgress,
		CurrentAssigneeID: 41, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	approver := &dto.AuthPrincipal{TenantID: 1, UserID: 7, Username: "service-owner", Roles: []string{EnterpriseRoleServiceManager}}
	if _, err := SLAService.PauseSLAForTenant("1", formatID(ticket.ID), "waiting_customer"); err == nil {
		t.Fatal("legacy pause path bypassed governance")
	}

	if _, err := SLAService.PauseSLAForTenantWithApproval("1", formatID(ticket.ID), SLAPauseRequest{
		Reason: "generic_pending", OwnerID: 41, Evidence: "customer email",
	}, approver); err == nil {
		t.Fatal("generic pending paused an accountable SLA clock")
	}
	if active := SLAService.GetActivePauseForTenant("1", formatID(ticket.ID)); active != nil {
		t.Fatalf("generic pending created a pause: %+v", active)
	}
	if _, err := SLAService.PauseSLAForTenantWithApproval("1", formatID(ticket.ID), SLAPauseRequest{
		Reason: "waiting_customer", OwnerID: 41,
	}, approver); err == nil {
		t.Fatal("pause without evidence succeeded")
	}
}

func TestGovernedSLAPauseStoresOwnerEffectiveTimeEvidenceAndApprover(t *testing.T) {
	db := setupSLATenantTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	ticket := models.Ticket{
		TicketNo: "SLA-PAUSE-EVIDENCE", TenantID: 1, Status: enums.TicketStatusInProgress,
		CaseOwnerID: 51, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	approver := &dto.AuthPrincipal{TenantID: 1, UserID: 8, Username: "tenant-owner", Roles: []string{EnterpriseRoleOwner}}
	effectiveAt := now.Add(-time.Minute)
	record, err := SLAService.PauseSLAForTenantWithApproval("1", formatID(ticket.ID), SLAPauseRequest{
		Reason: "waiting_parts", EffectiveAt: &effectiveAt, Evidence: "supplier order PO-2026-001",
	}, approver)
	if err != nil {
		t.Fatalf("pause SLA: %v", err)
	}
	if record.OwnerID != 51 || record.EffectiveAt == nil || !record.EffectiveAt.Equal(effectiveAt) || record.Evidence != "supplier order PO-2026-001" {
		t.Fatalf("pause context not preserved: %+v", record)
	}
	if record.ApprovedBy != 8 || record.ApprovedByName != "tenant-owner" || record.ApprovedAt == nil || record.ApprovalStatus != "approved" {
		t.Fatalf("approval subject not preserved: %+v", record)
	}
	if !record.PausedAt.Equal(effectiveAt) {
		t.Fatalf("paused_at=%v want effective_at=%v", record.PausedAt, effectiveAt)
	}
}

func TestGovernedSLAPauseRequiresAuthorizedApprovalSubject(t *testing.T) {
	db := setupSLATenantTestDB(t)
	now := time.Now().UTC()
	ticket := models.Ticket{
		TicketNo: "SLA-PAUSE-APPROVER", TenantID: 1, Status: enums.TicketStatusInProgress,
		CurrentAssigneeID: 61, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	engineer := &dto.AuthPrincipal{TenantID: 1, UserID: 61, Username: "engineer", Roles: []string{EnterpriseRoleEngineer}}
	if _, err := SLAService.PauseSLAForTenantWithApproval("1", formatID(ticket.ID), SLAPauseRequest{
		Reason: "waiting_customer", Evidence: "customer requested a callback tomorrow",
	}, engineer); err == nil {
		t.Fatal("unapproved engineer paused SLA")
	}
}
