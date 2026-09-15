package services

import (
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"
)

func TestTicketCaseReadFiltersLegacyEvidenceAndRecordedState(t *testing.T) {
	db := setupSLATenantTestDB(t)
	now := time.Now()
	tickets := []models.Ticket{
		{TicketNo: "CASE-LEGACY-WAIT", TenantID: 1, Status: enums.TicketStatusPendingCustomerConfirm},
		{TicketNo: "CASE-LEGACY-CONFIRM", TenantID: 1, Status: enums.TicketStatusPendingCustomerConfirm, ResolvedAt: &now},
		{TicketNo: "CASE-LEGACY-TRIAGE", TenantID: 1, Status: enums.TicketStatusProcessing},
		{TicketNo: "CASE-LEGACY-ASSIGNED", TenantID: 1, Status: enums.TicketStatusProcessing, CurrentAssigneeID: 12},
		{TicketNo: "CASE-RESTORED", TenantID: 1, Status: enums.TicketStatusProcessing, CaseStatus: "restored", RestoredAt: &now},
		{TicketNo: "CASE-RECORDED-WAIT", TenantID: 1, Status: enums.TicketStatusPendingCustomerConfirm, CaseStatus: "waiting", ResolvedAt: &now},
		{TicketNo: "CASE-OTHER-TENANT", TenantID: 2, Status: enums.TicketStatusProcessing, CaseStatus: "waiting"},
	}
	if err := db.Create(&tickets).Error; err != nil {
		t.Fatal(err)
	}
	for _, ticket := range tickets[:len(tickets)-1] {
		var actual string
		if err := db.Model(&models.Ticket{}).Select(ticketEffectiveCaseStatusSQL).Where("id = ?", ticket.ID).Scan(&actual).Error; err != nil {
			t.Fatal(err)
		}
		if actual != models.EffectiveTicketCaseStatus(ticket) {
			t.Fatalf("%s SQL state = %s; read model = %s", ticket.TicketNo, actual, models.EffectiveTicketCaseStatus(ticket))
		}
	}
	rows := repositories.TicketRepository.Find(db, enterpriseTicketBaseCnd(1, EnterpriseTicketQuery{CaseStatus: "waiting"}))
	if len(rows) != 2 {
		t.Fatalf("waiting filter returned %d rows, want two tenant 1 waiting records", len(rows))
	}
	for _, row := range rows {
		if row.TenantID != 1 || models.EffectiveTicketCaseStatus(row) != "waiting" {
			t.Fatalf("incorrect waiting match: %+v", row)
		}
	}
	closureRows := repositories.TicketRepository.Find(db, enterpriseTicketStatusCnd(1, EnterpriseTicketQuery{}, "awaiting_customer"))
	if len(closureRows) != 1 || closureRows[0].ID != tickets[1].ID {
		t.Fatalf("customer confirmation queue must exclude waiting-for-information cases: %+v", closureRows)
	}
	if _, err := EnterpriseTicketService.List(1, EnterpriseTicketQuery{CaseStatus: "not-a-status"}); err == nil {
		t.Fatal("unknown case status should be rejected")
	}
	legacy := ticketCaseSummaryWithOwner(tickets[2], "")
	if legacy.CaseStatusRecorded || legacy.AcknowledgedAt != "" || legacy.RestoredAt != "" || legacy.CaseOwnerID != 0 {
		t.Fatalf("legacy data invented acknowledgement/restore/owner evidence: %+v", legacy)
	}
	progress := buildCustomerTicketProgressFallback(tickets[0], "", nil)
	if len(progress) != 1 || progress[0].Content != "正在等待补充资料或处理条件，问题仍未解决。" {
		t.Fatalf("legacy waiting case was described as repaired: %+v", progress)
	}
}

func TestTicketCaseReadOwnerRemainsVisibleAfterTechnicalTransfer(t *testing.T) {
	db := setupSLATenantTestDB(t)
	tickets := []models.Ticket{
		{TicketNo: "CASE-OWNER", TenantID: 1, Status: enums.TicketStatusProcessing, CaseOwnerID: 55, CurrentAssigneeID: 66, CurrentTeamID: 20},
		{TicketNo: "CASE-UNRELATED", TenantID: 1, Status: enums.TicketStatusProcessing, CaseOwnerID: 77, CurrentAssigneeID: 66, CurrentTeamID: 20},
		{TicketNo: "CASE-CROSS-TENANT", TenantID: 2, Status: enums.TicketStatusProcessing, CaseOwnerID: 55, CurrentAssigneeID: 66, CurrentTeamID: 20},
	}
	if err := db.Create(&tickets).Error; err != nil {
		t.Fatal(err)
	}
	query := EnterpriseTicketQuery{RestrictViewer: true, ViewerUserID: 55, ViewerTeamIDs: []int64{10}}
	rows := repositories.TicketRepository.Find(db, enterpriseTicketBaseCnd(1, query))
	if len(rows) != 1 || rows[0].ID != tickets[0].ID {
		t.Fatalf("owner list scope returned %+v", rows)
	}
	scope := enterpriseProductAccessScope{TenantID: 1, Restricted: true, UserID: 55, TeamIDs: []int64{10}}
	if !scope.canAccessTicket(&tickets[0]) || scope.canAccessTicket(&tickets[1]) || scope.canAccessTicket(&tickets[2]) {
		t.Fatal("case owner read access must survive technical transfer and stay within tenant")
	}
}

func TestTicketCaseReadLegacyQualityReviewUsesExistingMilestones(t *testing.T) {
	db := setupSLATenantTestDB(t)
	now := time.Now()
	tests := []struct {
		name              string
		handled, resolved *time.Time
		want              string
	}{
		{name: "review-after-closure", handled: &now, resolved: &now, want: "closed"},
		{name: "review-after-resolution", resolved: &now, want: "resolved"},
		{name: "review-during-investigation", want: "in_triage"},
	}
	for _, test := range tests {
		ticket := models.Ticket{TicketNo: test.name, TenantID: 1, Status: enums.TicketStatusQualityReview, HandledAt: test.handled, ResolvedAt: test.resolved}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatal(err)
		}
		if got := models.EffectiveTicketCaseStatus(ticket); got != test.want {
			t.Fatalf("%s projected state = %s, want %s", test.name, got, test.want)
		}
		rows := repositories.TicketRepository.Find(db, enterpriseTicketBaseCnd(1, EnterpriseTicketQuery{CaseStatus: test.want}).Eq("id", ticket.ID))
		if len(rows) != 1 || rows[0].ID != ticket.ID {
			t.Fatalf("%s is not visible under its lifecycle filter: %+v", test.name, rows)
		}
		if ticket.CaseStatus != "" || ticket.AcknowledgedAt != nil || ticket.RestoredAt != nil {
			t.Fatal("quality-review mapping must not rewrite historical evidence")
		}
	}
}

func TestTicketCaseReadRestoreDoesNotStopResolutionSLA(t *testing.T) {
	db := setupSLATenantTestDB(t)
	now := time.Now()
	started := now.Add(-30 * time.Minute)
	restored := started.Add(time.Minute)
	resolved := started.Add(2 * time.Minute)
	deadline := started.Add(10 * time.Minute)
	policy := SLAPolicy{ID: "case-restore-policy", TenantID: "1", Priority: "p2", ResolutionMinutes: 10, Status: "active", CreatedAt: started, UpdatedAt: started}
	if err := db.Create(&policy).Error; err != nil {
		t.Fatal(err)
	}
	tickets := []models.Ticket{
		{TicketNo: "CASE-RESTORE-SLA", TenantID: 1, Status: enums.TicketStatusProcessing, CaseStatus: "restored", PriorityCode: "p2", RestoredAt: &restored, SLADueAt: &deadline, AuditFields: models.AuditFields{CreatedAt: started, UpdatedAt: now}},
		{TicketNo: "CASE-RESOLVE-SLA", TenantID: 1, Status: enums.TicketStatusResolved, CaseStatus: "resolved", PriorityCode: "p2", RestoredAt: &restored, ResolvedAt: &resolved, SLADueAt: &deadline, AuditFields: models.AuditFields{CreatedAt: started, UpdatedAt: now}},
	}
	if err := db.Create(&tickets).Error; err != nil {
		t.Fatal(err)
	}
	if !enterpriseTicketSLABreached(tickets[0]) || enterpriseTicketSLABreached(tickets[1]) {
		t.Fatal("restored case must retain resolution clock; resolved evidence must stop it")
	}
	violations, err := SLAService.CheckSLAViolations()
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 1 || violations[0].TicketID != formatID(tickets[0].ID) || violations[0].ViolationType != "resolution" {
		t.Fatalf("resolution violations = %+v; want only restored case", violations)
	}
}

func TestTicketCaseReadAutoCloseSkipsWaitingBeforeCandidateLimit(t *testing.T) {
	db := setupSLATenantTestDB(t)
	now := time.Now()
	old := now.AddDate(0, 0, -10)
	recent := now.AddDate(0, 0, -8)
	tickets := []models.Ticket{
		{TicketNo: "CASE-WAIT-OLD", TenantID: 1, Status: enums.TicketStatusPendingCustomerConfirm, CaseStatus: "waiting", ResolvedAt: &old},
		{TicketNo: "CASE-RESTORED-OLD", TenantID: 1, Status: enums.TicketStatusResolved, CaseStatus: "restored", RestoredAt: &old, ResolvedAt: &old},
		{TicketNo: "CASE-CLOSURE-DUE", TenantID: 1, Status: enums.TicketStatusPendingCustomerConfirm, CaseStatus: "closure_pending", ResolvedAt: &recent},
		{TicketNo: "CASE-CLOSURE-OTHER", TenantID: 2, Status: enums.TicketStatusPendingCustomerConfirm, CaseStatus: "closure_pending", ResolvedAt: &old},
	}
	if err := db.Create(&tickets).Error; err != nil {
		t.Fatal(err)
	}
	rows := repositories.TicketRepository.FindDueForAutoClose(db, 1, now.AddDate(0, 0, -7), 1)
	if len(rows) != 1 || rows[0].ID != tickets[2].ID {
		t.Fatalf("auto-close candidates = %+v; expected only eligible closure-pending case", rows)
	}
	if TicketAutoCloseService.eligibleAt(tickets[0], now) || TicketAutoCloseService.eligibleAt(tickets[1], now) {
		t.Fatal("waiting/restored cases must not auto-close")
	}
}
