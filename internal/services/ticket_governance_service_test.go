package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/ticketpolicy"
	"remotehelpdesk/internal/repositories"
	"strings"
	"testing"
	"time"
)

func setupGovernance(t *testing.T) (*gorm.DB, *dto.AuthPrincipal, models.Ticket) {
	db, op, ticket := setupCaseLifecycle(t)
	if err := db.AutoMigrate(&models.TicketGovernanceOperation{}, &models.TicketRelation{}, &models.TicketPriorityProposal{}); err != nil {
		t.Fatal(err)
	}
	op.Permissions = append(op.Permissions, constants.PermissionTicketView.Code, constants.PermissionTicketUpdate.Code)
	return db, op, ticket
}
func governanceCommand(t *testing.T, db *gorm.DB, id int64, action string) TicketGovernanceCommand {
	t.Helper()
	item := repositories.TicketRepository.Get(db, id)
	return TicketGovernanceCommand{Action: action, ExpectedRevision: &item.GovernanceRevision, OperationKey: fmt.Sprintf("%s-%d-%d", action, id, item.GovernanceRevision), Reason: "已核实实际影响及调整依据", Category: "impact"}
}
func executeGovernance(t *testing.T, id int64, cmd TicketGovernanceCommand, op *dto.AuthPrincipal) *TicketGovernanceReceipt {
	t.Helper()
	result, err := ExecuteTicketGovernance(id, cmd, op)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func lowImpactFacts() ticketpolicy.Facts {
	return ticketpolicy.Facts{Impact: "low", Urgency: "low", Safety: "none", Reach: "low", Workaround: "verified", Evidence: "备用流程已验证覆盖所有受影响用户"}
}

func TestTicketGovernanceDefaultsDowngradeAndReceipts(t *testing.T) {
	db, op, ticket := setupGovernance(t)
	for _, typ := range ticketpolicy.Types {
		item := ticket
		f := ticketpolicy.Facts{}
		if typ == "known_error" {
			item.ProductModuleID = 101
			f = lowImpactFacts()
		}
		if err := initializeTicketGovernanceDB(db, &item, typ, f); err != nil {
			t.Fatal(err)
		}
		if item.PriorityLevel != ticketpolicy.Default().Defaults[typ] || item.PriorityCode != ticketpolicy.LegacyCode(item.PriorityLevel) || item.PriorityReviewRequired {
			t.Fatalf("%s: %+v", typ, item.TicketGovernance)
		}
	}
	cmd := governanceCommand(t, db, ticket.ID, "classify")
	cmd.CaseType = "incident"
	cmd.Facts = lowImpactFacts()
	original := executeGovernance(t, ticket.ID, cmd, op)
	stored := repositories.TicketRepository.Get(db, ticket.ID)
	if stored.PriorityLevel != "p1" || stored.PrioritySuggested != "p1" || stored.PriorityReviewRequired {
		t.Fatalf("classification default: %+v", stored.TicketGovernance)
	}
	override := governanceCommand(t, db, ticket.ID, "override")
	override.Priority = "p2"
	override.Category = ""
	executeGovernance(t, ticket.ID, override, op)
	replay := executeGovernance(t, ticket.ID, cmd, op)
	if *replay != *original {
		t.Fatal("unstable receipt")
	}
	cmd.Reason = "changed payload"
	if _, err := ExecuteTicketGovernance(ticket.ID, cmd, op); !errors.Is(err, ErrTicketCaseConflict) {
		t.Fatal("payload conflict accepted", err)
	}
	stale := override
	stale.OperationKey = "different-operation"
	if _, err := ExecuteTicketGovernance(ticket.ID, stale, op); !errors.Is(err, ErrTicketCaseConflict) {
		t.Fatal("stale write accepted", err)
	}
	update := governanceCommand(t, db, ticket.ID, "classify")
	update.CaseType = "service_request"
	executeGovernance(t, ticket.ID, update, op)
	stored = repositories.TicketRepository.Get(db, ticket.ID)
	if stored.PriorityLevel != "p2" || stored.PriorityReviewRequired || !stored.PriorityOverridden || stored.CaseType != "service_request" {
		t.Fatal("classification change lost manual decision")
	}

}

func TestTicketGovernancePermissionsProposalsAndRollback(t *testing.T) {
	db, op, ticket := setupGovernance(t)
	cmd := governanceCommand(t, db, ticket.ID, "classify")
	cmd.CaseType = "user_case"
	executeGovernance(t, ticket.ID, cmd, op)
	engineer := *op
	engineer.UserID = 10002
	engineer.Permissions = []string{constants.PermissionTicketView.Code, constants.PermissionTicketChangeStatus.Code}
	if err := db.Model(&models.Ticket{}).Where("id = ?", ticket.ID).Update("current_assignee_id", engineer.UserID).Error; err != nil {
		t.Fatal(err)
	}
	cmd = governanceCommand(t, db, ticket.ID, "override")
	cmd.Priority = "p1"
	if _, err := ExecuteTicketGovernance(ticket.ID, cmd, &engineer); err == nil {
		t.Fatal("engineer directly overrode priority")
	}
	// Historical pending proposals remain read-only, including after lifecycle changes.
	proposal := models.TicketPriorityProposal{TenantID: ticket.TenantID, TicketID: ticket.ID, Revision: 1, Priority: "p1", Category: "impact", Reason: "历史建议", ProposedBy: engineer.UserID, Status: "pending", CreatedAt: time.Now()}
	if err := db.Create(&proposal).Error; err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"propose", "approve", "reject", "assess", "confirm"} {
		retired := governanceCommand(t, db, ticket.ID, action)
		retired.Priority, retired.ProposalID = "p1", proposal.ID
		if _, err := ExecuteTicketGovernance(ticket.ID, retired, op); err == nil {
			t.Fatalf("retired action %s accepted", action)
		}
	}
	var retained models.TicketPriorityProposal
	db.First(&retained, proposal.ID)
	if retained.Status != "pending" || repositories.TicketRepository.Get(db, ticket.ID).PriorityLevel != "p2" {
		t.Fatal("historical suggestion changed data")
	}
	view, err := GetTicketGovernance(ticket.ID, &engineer)
	if err != nil || view.CanManage || view.CanPropose || len(view.Proposals) != 1 {
		t.Fatalf("incorrect read-only permissions: %+v %v", view, err)
	}
	// An engineer with existing edit permission can change the level directly.
	engineer.Permissions = append(engineer.Permissions, constants.PermissionTicketUpdate.Code)
	cmd = governanceCommand(t, db, ticket.ID, "override")
	cmd.Priority, cmd.Category = "p1", ""
	executeGovernance(t, ticket.ID, cmd, &engineer)
	foreign := *op
	foreign.TenantID = 92
	if _, err := ExecuteTicketGovernance(ticket.ID, governanceCommand(t, db, ticket.ID, "override"), &foreign); err == nil {
		t.Fatal("cross tenant mutation")
	}
	viewer := *op
	viewer.Permissions = []string{constants.PermissionTicketView.Code}
	if _, err := ExecuteTicketGovernance(ticket.ID, governanceCommand(t, db, ticket.ID, "override"), &viewer); err == nil {
		t.Fatal("viewer mutation")
	}
	failName := "governance_receipt_failure"
	if err := db.Callback().Create().Before("gorm:create").Register(failName, func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*models.TicketGovernanceOperation); ok {
			tx.AddError(errors.New("receipt write failed"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Create().Remove(failName) })
	before := repositories.TicketRepository.Get(db, ticket.ID)
	cmd = governanceCommand(t, db, ticket.ID, "override")
	cmd.Priority = "p4"
	if _, err := ExecuteTicketGovernance(ticket.ID, cmd, op); err == nil {
		t.Fatal("receipt failure accepted")
	}
	after := repositories.TicketRepository.Get(db, ticket.ID)
	if before.PriorityLevel != after.PriorityLevel || before.GovernanceRevision != after.GovernanceRevision {
		t.Fatal("receipt failure did not roll back")
	}
}

func TestTicketGovernanceRelationsCyclesPermissionsAndClosure(t *testing.T) {
	db, op, parent := setupGovernance(t)
	child := parent
	child.ID = 0
	child.TicketNo = "CHILD"
	child.CaseOwnerID = op.UserID
	if err := db.Create(&child).Error; err != nil {
		t.Fatal(err)
	}
	third := child
	third.ID = 0
	third.TicketNo = "THIRD"
	if err := db.Create(&third).Error; err != nil {
		t.Fatal(err)
	}
	link := governanceCommand(t, db, parent.ID, "link")
	link.TargetID = child.ID
	link.RelationKind = "parent"
	executeGovernance(t, parent.ID, link, op)
	cycle := governanceCommand(t, db, child.ID, "link")
	cycle.TargetID = parent.ID
	cycle.RelationKind = "parent"
	if _, err := ExecuteTicketGovernance(child.ID, cycle, op); err == nil {
		t.Fatal("cycle accepted")
	}
	secondParent := governanceCommand(t, db, third.ID, "link")
	secondParent.TargetID = child.ID
	secondParent.RelationKind = "parent"
	if _, err := ExecuteTicketGovernance(third.ID, secondParent, op); err == nil {
		t.Fatal("second parent accepted")
	}
	if err := repositories.TicketRepository.Updates(db, parent.ID, map[string]any{"case_status": "closed", "status": "closed"}); err == nil {
		t.Fatal("parent closed with open child")
	}
	if err := repositories.TicketRepository.Updates(db, parent.ID, map[string]any{"case_status": "restored"}); err != nil {
		t.Fatal(err)
	}
	if repositories.TicketRepository.Get(db, child.ID).CaseStatus != "new" {
		t.Fatal("parent changed child status")
	}

	view, err := GetTicketGovernance(parent.ID, op)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Relations) != 1 {
		t.Fatal(view.Relations)
	}
	unlink := governanceCommand(t, db, parent.ID, "unlink")
	unlink.RelationID = view.Relations[0].ID
	executeGovernance(t, parent.ID, unlink, op)
	if err := repositories.TicketRepository.Updates(db, parent.ID, map[string]any{"case_status": "closed", "status": "closed"}); err != nil {
		t.Fatal(err)
	}
	var removed models.TicketRelation
	if err := db.First(&removed, unlink.RelationID).Error; err != nil {
		t.Fatal(err)
	}
	if removed.RemovedAt == nil || removed.ActiveKey != nil || removed.ChildKey != nil || removed.RemovalReason == "" {
		t.Fatal("relationship history lost")
	}
	if repositories.TicketRepository.Get(db, child.ID).CaseOwnerID != op.UserID {
		t.Fatal("relation change lost child owner")
	}
}

func TestTicketGovernanceExplicitDeadlineWithoutPolicy(t *testing.T) {
	db, _, _ := setupGovernance(t)
	if err := db.AutoMigrate(&SLAPolicy{}); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	explicit := start.Add(4 * time.Hour)
	ticket := models.Ticket{TenantID: 91, PriorityCode: "p2", SLADueAt: &explicit,
		AuditFields: models.AuditFields{CreatedAt: start}}
	if err := refreshGovernanceDeadlineDB(db, &ticket); err != nil || ticket.SLADueAt == nil || !ticket.SLADueAt.Equal(explicit) {
		t.Fatalf("new ticket lost explicit deadline without policy: %+v, %v", ticket.SLADueAt, err)
	}
	policy := SLAPolicy{ID: "configured", TenantID: "91", Priority: "p2", ResolutionMinutes: 30, Status: "active"}
	if err := db.Create(&policy).Error; err != nil {
		t.Fatal(err)
	}
	if err := refreshGovernanceDeadlineDB(db, &ticket); err != nil || ticket.SLADueAt == nil || !ticket.SLADueAt.Equal(start.Add(30*time.Minute)) {
		t.Fatalf("configured target must override manual deadline: %+v, %v", ticket.SLADueAt, err)
	}
	if err := db.Model(&policy).Update("resolution_minutes", 0).Error; err != nil {
		t.Fatal(err)
	}
	if err := refreshGovernanceDeadlineDB(db, &ticket); err != nil || ticket.SLADueAt != nil {
		t.Fatalf("explicitly disabled target must clear deadline: %+v, %v", ticket.SLADueAt, err)
	}
	ticket.ID, ticket.PriorityCode, ticket.SLADueAt = 999, "p3", &explicit
	if err := refreshGovernanceDeadlineDB(db, &ticket); err != nil || ticket.SLADueAt != nil {
		t.Fatalf("saved ticket must clear stale deadline after priority change: %+v, %v", ticket.SLADueAt, err)
	}
}

func TestTicketGovernancePrioritySLAUsesOriginalStart(t *testing.T) {
	db, op, ticket := setupGovernance(t)
	if err := db.AutoMigrate(&SLAPolicy{}); err != nil {
		t.Fatal(err)
	}
	start := time.Now().Add(-2 * time.Hour)
	db.Model(&models.Ticket{}).Where("id = ?", ticket.ID).Update("created_at", start)
	for _, p := range []SLAPolicy{{ID: "p0", TenantID: "91", Priority: "p0", ResolutionMinutes: 30, Status: "active"}, {ID: "p1", TenantID: "91", Priority: "p1", ResolutionMinutes: 240, Status: "active"}} {
		if err := db.Create(&p).Error; err != nil {
			t.Fatal(err)
		}
	}
	classify := governanceCommand(t, db, ticket.ID, "classify")
	classify.CaseType = "incident"
	executeGovernance(t, ticket.ID, classify, op)
	stored := repositories.TicketRepository.Get(db, ticket.ID)
	if stored.SLADueAt == nil || !stored.SLADueAt.Equal(start.Add(30*time.Minute)) {
		t.Fatal("priority SLA restarted")
	}
	preview, err := PreviewTicketPriorityChange(ticket.ID, "p2", op)
	if err != nil || preview.Deadline == nil || !preview.Deadline.Equal(start.Add(4*time.Hour)) {
		t.Fatal("incorrect deadline preview", err)
	}
	if !repositories.TicketRepository.Get(db, ticket.ID).SLADueAt.Equal(start.Add(30 * time.Minute)) {
		t.Fatal("preview mutated the deadline")
	}
	cmd := governanceCommand(t, db, ticket.ID, "override")
	cmd.Priority = "p2"
	executeGovernance(t, ticket.ID, cmd, op)
	stored = repositories.TicketRepository.Get(db, ticket.ID)
	if stored.SLADueAt == nil || !stored.SLADueAt.Equal(start.Add(4*time.Hour)) {
		t.Fatal("downgrade SLA restarted")
	}
	view, err := GetTicketGovernance(ticket.ID, op)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.History) != 2 {
		t.Fatal("SLA change history missing")
	}
}

func TestTicketGovernanceClassificationDefaultsAndPinnedRules(t *testing.T) {
	db, op, ticket := setupGovernance(t)
	classify := governanceCommand(t, db, ticket.ID, "classify")
	classify.CaseType, classify.Facts = "incident", lowImpactFacts()
	executeGovernance(t, ticket.ID, classify, op)
	change := governanceCommand(t, db, ticket.ID, "classify")
	change.CaseType = "service_request"
	executeGovernance(t, ticket.ID, change, op)
	stored := repositories.TicketRepository.Get(db, ticket.ID)
	if stored.PriorityLevel != "p4" || stored.PriorityOverridden || stored.PriorityReviewRequired {
		t.Fatal("classification default did not follow corrected type")
	}
	pinned := ticketpolicy.Default()
	pinned.Version = "pinned-fixture"
	pinned.Defaults["user_case"] = "p3"
	raw, _ := json.Marshal(pinned)
	stored.PriorityPolicyJSON = string(raw)
	original, _, err := loadTicketPriorityPolicyDB(db, stored, false)
	if err != nil || original.Version != "pinned-fixture" {
		t.Fatal("lost pinned policy", err)
	}
	latest, _, err := loadTicketPriorityPolicyDB(db, stored, true)
	if err != nil || latest.Version == "pinned-fixture" {
		t.Fatal("explicit latest rules ignored", err)
	}
	// Historical p4 retains its own SLA compatibility key after classification.
	legacy := ticket
	legacy.ID = 0
	legacy.TicketNo = "LEGACY-P4"
	legacy.PriorityCode = "p4"
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	cmd := governanceCommand(t, db, legacy.ID, "classify")
	cmd.CaseType = "service_request"
	executeGovernance(t, legacy.ID, cmd, op)
	if repositories.TicketRepository.Get(db, legacy.ID).PriorityCode != "p4" {
		t.Fatal("historical SLA key rewritten")
	}
}

func TestTicketGovernanceClosedPriorityFilters(t *testing.T) {
	db, _, ticket := setupGovernance(t)
	for _, level := range []string{"p1", "p2", "p3", "p4"} {
		item := ticket
		item.ID = 0
		item.TicketNo = "CLOSED-" + level
		item.CaseStatus = "closed"
		item.Status = "closed"
		item.PriorityLevel = level
		item.PriorityCode = ticketpolicy.LegacyCode(level)
		if err := db.Create(&item).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, level := range []string{"p1", "p2", "p3", "p4"} {
		cnd := sqls.NewCnd().Eq("case_status", "closed")
		applyEnterpriseTicketPriorityCnd(cnd, level, time.Now())
		rows := repositories.TicketRepository.Find(db, cnd)
		if len(rows) != 1 || rows[0].PriorityLevel != level {
			t.Fatalf("closed %s filter failed: %+v", level, rows)
		}
	}
}

func TestTicketGovernanceRelationAuditRollbackAndDirectedCycles(t *testing.T) {
	db, op, a := setupGovernance(t)
	b := a
	b.ID = 0
	b.TicketNo = "RELATION-B"
	if err := db.Create(&b).Error; err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"parent"} {
		cmd := governanceCommand(t, db, a.ID, "link")
		cmd.TargetID = b.ID
		cmd.RelationKind = kind
		executeGovernance(t, a.ID, cmd, op)
		reverse := governanceCommand(t, db, b.ID, "link")
		reverse.TargetID = a.ID
		reverse.RelationKind = kind
		if _, err := ExecuteTicketGovernance(b.ID, reverse, op); err == nil {
			t.Fatalf("reverse %s accepted", kind)
		}
	}
	self := governanceCommand(t, db, a.ID, "link")
	self.TargetID = a.ID
	self.RelationKind = "parent"
	if _, err := ExecuteTicketGovernance(a.ID, self, op); err == nil {
		t.Fatal("self relation accepted")
	}
	view, err := GetTicketGovernance(a.ID, op)
	if err != nil {
		t.Fatal(err)
	}
	before := repositories.TicketRepository.Get(db, a.ID)
	name := "fail_relation_audit"
	if err := db.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
		if row, ok := tx.Statement.Dest.(*models.TicketProgress); ok && row.EventType == "ticket_governance" {
			tx.AddError(errors.New("audit unavailable"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Create().Remove(name) })
	unlink := governanceCommand(t, db, a.ID, "unlink")
	unlink.RelationID = view.Relations[0].ID
	if _, err := ExecuteTicketGovernance(a.ID, unlink, op); err == nil {
		t.Fatal("audit failure accepted")
	}
	var relation models.TicketRelation
	db.First(&relation, unlink.RelationID)
	if relation.ActiveKey == nil || repositories.TicketRepository.Get(db, a.ID).GovernanceRevision != before.GovernanceRevision {
		t.Fatal("relation audit failure did not roll back")
	}
}

func TestTicketGovernanceRelationsRespectBothProductScopes(t *testing.T) {
	db, manager, a := setupGovernance(t)
	if err := db.AutoMigrate(&models.AgentTeamMember{}, &models.Product{}); err != nil {
		t.Fatal(err)
	}
	b := a
	b.ID = 0
	b.TicketNo = "RESTRICTED"
	b.ProductID = 802
	if err := db.Create(&b).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&a).Update("product_id", 801).Error; err != nil {
		t.Fatal(err)
	}
	cmd := governanceCommand(t, db, a.ID, "link")
	cmd.TargetID = b.ID
	cmd.RelationKind = "parent"
	executeGovernance(t, a.ID, cmd, manager)
	engineer, _, _ := seedCaseOwnerMember(t, db, 91, "restricted-viewer", constants.PermissionTicketUpdate.Code)
	team := models.AgentTeam{TenantID: 91, ProductID: 801, Name: "Scoped team", TeamType: AgentTeamTypeProductRepair}
	if err := db.Create(&team).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.AgentTeamMember{TenantID: 91, TeamID: team.ID, UserID: engineer.ID}).Error; err != nil {
		t.Fatal(err)
	}
	op := caseOwnerPrincipal(engineer)
	op.Roles = []string{EnterpriseRoleEngineer}
	op.Permissions = append(op.Permissions, constants.PermissionTicketView.Code)
	view, err := GetTicketGovernance(a.ID, op)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Relations) != 0 || len(view.History) != 0 {
		t.Fatal("restricted endpoint leaked relation or audit reason")
	}
	cmd = governanceCommand(t, db, a.ID, "link")
	cmd.TargetID = b.ID
	cmd.RelationKind = "parent"
	if _, err := ExecuteTicketGovernance(a.ID, cmd, op); err == nil {
		t.Fatal("one-sided authority created relationship")
	}
}

func TestTicketGovernanceAcceptanceDeadlineKeepsAssignmentStart(t *testing.T) {
	db, op, ticket := setupGovernance(t)
	if err := db.AutoMigrate(&SLAPolicy{}); err != nil {
		t.Fatal(err)
	}
	start := time.Now().Add(-2 * time.Hour)
	assigned := time.Now().Add(-5 * time.Minute)
	oldDeadline := start.Add(30 * time.Minute)
	if err := db.Model(&ticket).Updates(map[string]any{"priority_code": "p0", "priority_level": "p1", "case_type": "incident", "created_at": start, "assigned_at": assigned, "accept_deadline_at": oldDeadline}).Error; err != nil {
		t.Fatal(err)
	}
	policy := SLAPolicy{ID: "accept-p1", TenantID: "91", Priority: "p1", AssignmentMinutes: 240, ResolutionMinutes: 480, Status: "active"}
	if err := db.Create(&policy).Error; err != nil {
		t.Fatal(err)
	}
	proposed := ticket
	proposed.PriorityCode = "p1"
	proposed.CreatedAt = start
	want := ticketAssignmentDeadlineDB(db, &proposed, assigned)
	cmd := governanceCommand(t, db, ticket.ID, "override")
	cmd.Priority = "p2"
	executeGovernance(t, ticket.ID, cmd, op)
	stored := repositories.TicketRepository.Get(db, ticket.ID)
	if stored.AcceptDeadlineAt == nil || !stored.AcceptDeadlineAt.Equal(want) || !stored.AssignedAt.Equal(assigned) {
		t.Fatal("priority adjustment restarted acceptance clock")
	}
}

func TestTicketGovernancePostgresGraphSerialization(t *testing.T) {
	db := setupCaseOwnerPostgresTestDB(t)
	first := db.Begin()
	if first.Error != nil {
		t.Fatal(first.Error)
	}
	defer first.Rollback()
	if err := repositories.LockTicketGraphDB(first, 91); err != nil {
		t.Fatal(err)
	}
	second := db.Begin()
	if second.Error != nil {
		t.Fatal(second.Error)
	}
	defer second.Rollback()
	if err := repositories.LockTicketGraphDB(second, 91); err == nil {
		t.Fatal("concurrent graph writer was not excluded")
	}
	if err := first.Rollback().Error; err != nil {
		t.Fatal(err)
	}
	if err := repositories.LockTicketGraphDB(second, 91); err != nil {
		t.Fatal("graph lock not released", err)
	}
}

func TestTicketGovernanceSimpleDefaultsIgnoreLegacyAssessment(t *testing.T) {
	db, _, ticket := setupGovernance(t)
	facts := ticketpolicy.Facts{Safety: "critical", Impact: "high", Reach: "high", Workaround: "none"}
	if err := initializeTicketGovernanceDB(db, &ticket, "service_request", facts); err != nil {
		t.Fatal(err)
	}
	if ticket.PriorityLevel != "p4" || ticket.PriorityReviewRequired {
		t.Fatal("legacy facts changed classification default")
	}
}

func TestTicketGovernanceSingleConnectionPauseAndFailureRollback(t *testing.T) {
	db, op, ticket := setupGovernance(t)
	if err := db.AutoMigrate(&SLAPolicy{}, &SLAPauseRecord{}); err != nil {
		t.Fatal(err)
	}
	conn, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	conn.SetMaxOpenConns(1)
	start := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	if err := db.Model(&ticket).Updates(map[string]any{"case_type": "incident", "priority_level": "p1", "priority_code": "p0", "created_at": start}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&SLAPolicy{ID: "simple-p2", TenantID: "91", Priority: "p1", ResolutionMinutes: 240, Status: "active"}).Error; err != nil {
		t.Fatal(err)
	}
	resumed := start.Add(30 * time.Minute)
	for _, pause := range []SLAPauseRecord{
		{ID: "own-pause", TenantID: "91", TicketID: formatID(ticket.ID), PausedAt: start.Add(20 * time.Minute), ResumedAt: &resumed, Duration: 600},
		{ID: "foreign-pause", TenantID: "92", TicketID: formatID(ticket.ID), PausedAt: start, ResumedAt: &resumed, Duration: 99999},
	} {
		if err := db.Create(&pause).Error; err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	sqls.SetDB(db.WithContext(ctx))
	defer sqls.SetDB(db)
	cmd := governanceCommand(t, db, ticket.ID, "override")
	cmd.Priority, cmd.Category = "p2", ""
	executeGovernance(t, ticket.ID, cmd, op)
	stored := repositories.TicketRepository.Get(db, ticket.ID)
	if stored.SLADueAt == nil || !stored.SLADueAt.Equal(start.Add(250*time.Minute)) {
		t.Fatalf("pause/original start lost: %v", stored.SLADueAt)
	}
	name := "simple_pause_failure"
	if err := db.Callback().Query().Before("gorm:query").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == "sla_pause_records" {
			tx.AddError(errors.New("pause query unavailable"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Query().Remove(name) })
	if err := db.Create(&SLAPolicy{ID: "simple-p1", TenantID: "91", Priority: "p0", ResolutionMinutes: 60, Status: "active"}).Error; err != nil {
		t.Fatal(err)
	}
	cmd = governanceCommand(t, db, ticket.ID, "override")
	cmd.Priority = "p1"
	if _, err := ExecuteTicketGovernance(ticket.ID, cmd, op); err == nil {
		t.Fatal("pause query failure accepted")
	}
	after := repositories.TicketRepository.Get(db, ticket.ID)
	if after.PriorityLevel != stored.PriorityLevel || after.GovernanceRevision != stored.GovernanceRevision || !after.SLADueAt.Equal(*stored.SLADueAt) {
		t.Fatal("failed SLA query did not roll back")
	}
}

func TestTicketGovernanceSimpleReasonRequiredAndHistoricalRead(t *testing.T) {
	db, op, ticket := setupGovernance(t)
	cmd := governanceCommand(t, db, ticket.ID, "override")
	cmd.Priority, cmd.Reason = "p1", "  "
	if _, err := ExecuteTicketGovernance(ticket.ID, cmd, op); err == nil {
		t.Fatal("empty reason accepted")
	}
	if err := db.Model(&ticket).Updates(map[string]any{"priority_level": "p2", "priority_code": "p1", "priority_review_required": true, "priority_facts_json": `{"evidence":"historical evidence"}`}).Error; err != nil {
		t.Fatal(err)
	}
	view, err := GetTicketGovernance(ticket.ID, op)
	if err != nil || view.ReviewRequired || view.Priority != "p2" || view.Facts.Evidence != "historical evidence" {
		t.Fatalf("historical read mismatch: %+v %v", view, err)
	}
	stored := repositories.TicketRepository.Get(db, ticket.ID)
	if !stored.PriorityReviewRequired {
		t.Fatal("reading rewrote historical data")
	}
}

func TestTicketGovernanceKnownErrorMustLinkRequirements(t *testing.T) {
	db, op, ticket := setupGovernance(t)
	ticket.ProductModuleID = 101
	if err := db.Save(&ticket).Error; err != nil {
		t.Fatal(err)
	}

	validFacts := lowImpactFacts() // Workaround: "verified", Evidence: "...", Safety: "none", Impact: "low", Reach: "low"

	// 1. Missing or invalid Workaround
	{
		cmd := governanceCommand(t, db, ticket.ID, "classify")
		cmd.CaseType = "known_error"
		invalidFacts := validFacts
		invalidFacts.Workaround = "none"
		cmd.Facts = invalidFacts
		_, err := ExecuteTicketGovernance(ticket.ID, cmd, op)
		if err == nil || !strings.Contains(err.Error(), "临时解决办法") {
			t.Fatalf("expected workaround rejection, got %v", err)
		}

		// Also test missing evidence
		invalidFacts = validFacts
		invalidFacts.Evidence = ""
		cmd.Facts = invalidFacts
		_, err = ExecuteTicketGovernance(ticket.ID, cmd, op)
		if err == nil || !strings.Contains(err.Error(), "临时解决办法") {
			t.Fatalf("expected evidence rejection, got %v", err)
		}
	}

	// 2. Missing Component
	{
		noComponentTicket := ticket
		noComponentTicket.ID = 0
		noComponentTicket.TicketNo = "CASE-NO-COMP"
		noComponentTicket.ProductModuleID = 0
		noComponentTicket.ProductID = 0
		noComponentTicket.FaultCode = ""
		if err := db.Create(&noComponentTicket).Error; err != nil {
			t.Fatal(err)
		}
		cmd := governanceCommand(t, db, noComponentTicket.ID, "classify")
		cmd.CaseType = "known_error"
		cmd.Facts = validFacts
		_, err := ExecuteTicketGovernance(noComponentTicket.ID, cmd, op)
		if err == nil || !strings.Contains(err.Error(), "受影响组件") {
			t.Fatalf("expected component rejection, got %v", err)
		}
	}

	// 3. Missing Risk (Safety or Impact)
	{
		cmd := governanceCommand(t, db, ticket.ID, "classify")
		cmd.CaseType = "known_error"
		invalidFacts := validFacts
		invalidFacts.Impact = "unknown"
		cmd.Facts = invalidFacts
		_, err := ExecuteTicketGovernance(ticket.ID, cmd, op)
		if err == nil || !strings.Contains(err.Error(), "风险") {
			t.Fatalf("expected risk impact rejection, got %v", err)
		}

		invalidFacts = validFacts
		invalidFacts.Safety = "unknown"
		cmd.Facts = invalidFacts
		_, err = ExecuteTicketGovernance(ticket.ID, cmd, op)
		if err == nil || !strings.Contains(err.Error(), "风险") {
			t.Fatalf("expected risk safety rejection, got %v", err)
		}
	}

	// 4. Missing Reach (Scope)
	{
		cmd := governanceCommand(t, db, ticket.ID, "classify")
		cmd.CaseType = "known_error"
		invalidFacts := validFacts
		invalidFacts.Reach = "unknown"
		cmd.Facts = invalidFacts
		_, err := ExecuteTicketGovernance(ticket.ID, cmd, op)
		if err == nil || !strings.Contains(err.Error(), "适用范围") {
			t.Fatalf("expected reach rejection, got %v", err)
		}
	}

	// 5. Missing Permanent Fix Owner
	{
		noOwnerTicket := ticket
		noOwnerTicket.ID = 0
		noOwnerTicket.TicketNo = "CASE-NO-OWNER"
		noOwnerTicket.CurrentAssigneeID = 0
		noOwnerTicket.CaseOwnerID = 0
		noOwnerTicket.ProductModuleID = 101
		if err := db.Create(&noOwnerTicket).Error; err != nil {
			t.Fatal(err)
		}
		cmd := governanceCommand(t, db, noOwnerTicket.ID, "classify")
		cmd.CaseType = "known_error"
		cmd.Facts = validFacts
		_, err := ExecuteTicketGovernance(noOwnerTicket.ID, cmd, op)
		if err == nil || !strings.Contains(err.Error(), "永久修复责任人") {
			t.Fatalf("expected permanent fix owner rejection, got %v", err)
		}
	}

	// 6. All 5 satisfied -> success
	{
		cmd := governanceCommand(t, db, ticket.ID, "classify")
		cmd.CaseType = "known_error"
		cmd.Facts = validFacts
		receipt, err := ExecuteTicketGovernance(ticket.ID, cmd, op)
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		stored := repositories.TicketRepository.Get(db, ticket.ID)
		if stored.CaseType != "known_error" || stored.GovernanceRevision != receipt.Revision {
			t.Fatalf("unexpected stored governance state: %+v", stored)
		}
		var storedFacts ticketpolicy.Facts
		if err := json.Unmarshal([]byte(stored.PriorityFactsJSON), &storedFacts); err != nil || storedFacts.Workaround != "verified" {
			t.Fatalf("facts not preserved: %s", stored.PriorityFactsJSON)
		}
	}

	// 7. Ticket without component succeeds if ProductID or ProductModuleID is supplied in command
	{
		noCompTicket := ticket
		noCompTicket.ID = 0
		noCompTicket.TicketNo = "CASE-AUTO-ATTACH-COMP"
		noCompTicket.ProductModuleID = 0
		noCompTicket.ProductID = 0
		if err := db.Create(&noCompTicket).Error; err != nil {
			t.Fatal(err)
		}
		cmd := governanceCommand(t, db, noCompTicket.ID, "classify")
		cmd.CaseType = "known_error"
		cmd.Facts = validFacts
		cmd.ProductID = 88
		cmd.ProductModuleID = 99
		receipt, err := ExecuteTicketGovernance(noCompTicket.ID, cmd, op)
		if err != nil {
			t.Fatalf("expected success with supplied component, got %v", err)
		}
		stored := repositories.TicketRepository.Get(db, noCompTicket.ID)
		if stored.CaseType != "known_error" || stored.ProductID != 88 || stored.ProductModuleID != 99 || stored.GovernanceRevision != receipt.Revision {
			t.Fatalf("unexpected stored state: %+v", stored)
		}
	}

	// 8. Knowledge support ticket with Category or KnowledgeBaseID succeeds without ProductID
	{
		knowledgeTicket := ticket
		knowledgeTicket.ID = 0
		knowledgeTicket.TicketNo = "CASE-KNOWLEDGE-SUPPORT"
		knowledgeTicket.ProductModuleID = 0
		knowledgeTicket.ProductID = 0
		knowledgeTicket.TicketType = "知识库问答"
		knowledgeTicket.KnowledgeBaseID = 12
		if err := db.Create(&knowledgeTicket).Error; err != nil {
			t.Fatal(err)
		}
		cmd := governanceCommand(t, db, knowledgeTicket.ID, "classify")
		cmd.CaseType = "known_error"
		cmd.Facts = validFacts
		receipt, err := ExecuteTicketGovernance(knowledgeTicket.ID, cmd, op)
		if err != nil {
			t.Fatalf("expected success for knowledge support ticket, got %v", err)
		}
		stored := repositories.TicketRepository.Get(db, knowledgeTicket.ID)
		if stored.CaseType != "known_error" || stored.GovernanceRevision != receipt.Revision {
			t.Fatalf("unexpected stored state: %+v", stored)
		}
	}
}
