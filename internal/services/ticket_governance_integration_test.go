package services_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"testing"
	"time"

	"github.com/mlogclub/simple/sqls"
	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"
)

func TestConversationTicketDefaultsAndIndependentRepeatReport(t *testing.T) {
	t.Setenv("RHD_BOOTSTRAP_ADMIN_PASSWORD", "Governance-test-only-2026!")
	setupTicketTestDB(t)
	db := sqls.DB()
	tenant, product, _, device, _ := createTicketAfterSalesFixture(t, "conversation-default", enums.StatusOk)
	op := &dto.AuthPrincipal{TenantID: tenant.ID, Username: "AI"}
	conversation := models.Conversation{TenantID: tenant.ID, ProductID: product.ID, DeviceID: device.ID, Status: enums.IMConversationStatusAIServing, LastMessageSummary: "设备启动后持续报 E42，无法使用"}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	req := request.CreateTicketFromConversationRequest{ConversationID: conversation.ID, Title: "设备报错 E42", Description: conversation.LastMessageSummary}
	first, err := services.TicketService.CreateFromConversation(req, op)
	if err != nil {
		t.Fatal(err)
	}
	if first.CaseType != "user_case" || first.PriorityLevel != "p2" || first.Source != enums.TicketSourceConversation {
		t.Fatalf("unexpected conversation classification: %+v", first.TicketGovernance)
	}
	req.IdempotencyKey = "different-workflow-message"
	replay, err := services.TicketService.CreateFromConversation(req, op)
	if err != nil || replay.ID != first.ID {
		t.Fatal("same conversation duplicated ticket", err)
	}
	conversation.ID = 0
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	req.ConversationID, req.IdempotencyKey = conversation.ID, ""
	other, err := services.TicketService.CreateFromConversation(req, op)
	if err != nil || other.ID == first.ID {
		t.Fatal("same title in a new conversation was silently merged", err)
	}
	var relations int64
	if err := db.Model(&models.TicketRelation{}).Count(&relations).Error; err != nil || relations != 0 {
		t.Fatal("conversation automatically invented parent/duplicate relation", err)
	}
}

func TestTicketGovernanceAtomicChildCreationAndLegacyUpdate(t *testing.T) {
	t.Setenv("RHD_BOOTSTRAP_ADMIN_PASSWORD", "Governance-test-only-2026!")
	setupTicketTestDB(t)
	db := sqls.DB()
	tenant, _, _, _, _ := createTicketAfterSalesFixture(t, "governance-create", enums.StatusOk)
	op := createTestOperator(t, "governance-manager")
	op.TenantID = tenant.ID
	op.Permissions = []string{constants.PermissionTicketView.Code, constants.PermissionTicketCreate.Code, constants.PermissionTicketUpdate.Code}
	base := request.CreateTicketRequest{Title: "Synthetic governance case", Description: "Fixture only", Source: "manual", PriorityCode: "p4", TicketClassificationInput: dto.TicketClassificationInput{CaseType: "incident"}}
	parent, err := services.TicketService.CreateTicket(base, op)
	if err != nil {
		t.Fatal(err)
	}
	if parent.PriorityLevel != "p1" || parent.PriorityCode != "p0" {
		t.Fatal("client priority bypassed classification")
	}
	base.ParentTicketID, base.RelationReason, base.CaseType = parent.ID, "同一事件的客户跟进", "user_case"
	child, err := services.TicketService.CreateTicket(base, op)
	if err != nil {
		t.Fatal(err)
	}
	if child.PriorityLevel != "p2" {
		t.Fatal("child inherited parent priority")
	}
	view, err := services.GetTicketGovernance(parent.ID, op)
	if err != nil || len(view.Relations) != 1 {
		t.Fatalf("missing atomic relation: %+v %v", view, err)
	}
	var count int64
	db.Model(&models.Ticket{}).Count(&count)
	foreign := *parent
	foreign.ID = 0
	foreign.TenantID++
	foreign.TicketNo = "FOREIGN-GOVERNANCE"
	if err := db.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	base.ParentTicketID = foreign.ID
	if _, err := services.TicketService.CreateTicket(base, op); err == nil {
		t.Fatal("foreign parent accepted")
	}
	var after int64
	db.Model(&models.Ticket{}).Count(&after)
	if after != count+1 {
		t.Fatal("failed parent link left an orphan ticket")
	}
	future := time.Now().Add(100 * time.Hour)
	update := request.UpdateTicketRequest{TicketID: parent.ID, Title: parent.Title, Description: parent.Description, PriorityCode: "p4", SLADueAt: &future}
	if err := services.TicketService.UpdateTicket(update, op); err == nil {
		t.Fatal("legacy endpoint bypassed reason workflow")
	}
	update.PriorityCode = ""
	if err := services.TicketService.UpdateTicket(update, op); err != nil {
		t.Fatal(err)
	}
	stored := services.TicketService.Get(parent.ID)
	if stored.PriorityLevel != "p1" || stored.PriorityCode != "p0" || (stored.SLADueAt != nil && stored.SLADueAt.Equal(future)) {
		t.Fatal("legacy update overwrote governed priority/deadline")
	}
	aggregate, err := services.EnterpriseTicketService.GetAggregate(tenant.ID, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range aggregate.Timeline {
		if row.Type == "ticket_governance" {
			t.Fatal("internal relationship reasons leaked through generic timeline")
		}
	}
	if builders.BuildTicketProgress(&models.TicketProgress{EventType: "ticket_governance", Content: "internal relationship reason"}) != nil {
		t.Fatal("generic progress builder leaked governance")
	}
}

func TestTicketGovernanceCreateWithManualUrgencyIsAtomic(t *testing.T) {
	t.Setenv("RHD_BOOTSTRAP_ADMIN_PASSWORD", "Governance-test-only-2026!")
	setupTicketTestDB(t)
	db := sqls.DB()
	tenant, _, _, _, _ := createTicketAfterSalesFixture(t, "governance-create-override", enums.StatusOk)
	op := createTestOperator(t, "create-urgency-editor")
	op.TenantID = tenant.ID
	op.Permissions = []string{constants.PermissionTicketView.Code, constants.PermissionTicketCreate.Code, constants.PermissionTicketUpdate.Code}
	base := request.CreateTicketRequest{Title: "Create with chosen urgency", Description: "Synthetic test", Source: "manual", TicketClassificationInput: dto.TicketClassificationInput{CaseType: "user_case", PriorityLevel: "p1", PriorityReason: "关键业务中断，需要立即处理"}}
	policy := services.SLAPolicy{ID: "create-urgency-p1", TenantID: fmt.Sprint(tenant.ID), Priority: "p0", ResolutionMinutes: 60, Status: "active"}
	if err := db.Create(&policy).Error; err != nil {
		t.Fatal(err)
	}
	ticket, err := services.TicketService.CreateTicket(base, op)
	if err != nil {
		t.Fatal(err)
	}
	if ticket.PriorityLevel != "p1" || ticket.PriorityCode != "p0" || !ticket.PriorityOverridden || ticket.CaseType != "user_case" || ticket.PrioritySuggested != "p2" {
		t.Fatalf("manual urgency not persisted: %+v", ticket.TicketGovernance)
	}
	if ticket.SLADueAt == nil || !ticket.SLADueAt.Equal(ticket.CreatedAt.Add(time.Hour)) {
		t.Fatalf("wrong initial SLA: %v", ticket.SLADueAt)
	}
	view, err := services.GetTicketGovernance(ticket.ID, op)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, h := range view.History {
		if h.Action != "override" {
			continue
		}
		var details struct {
			Before models.TicketGovernance
			After  models.TicketGovernance
		}
		if err := json.Unmarshal(h.Details, &details); err != nil {
			t.Fatal(err)
		}
		if h.Reason != base.PriorityReason || h.ActorID != op.UserID || details.Before.PriorityLevel != "p2" || details.After.PriorityLevel != "p1" {
			t.Fatalf("audit missing reason/actor/levels: %+v", h)
		}
		found = true
	}
	if !found {
		t.Fatal("initial override audit missing")
	}
	var before int64
	db.Model(&models.Ticket{}).Count(&before)
	invalid := base
	for _, level := range []string{"p0", "p5", "P1"} {
		invalid.PriorityLevel = level
		if _, err := services.TicketService.CreateTicket(invalid, op); err == nil {
			t.Fatalf("invalid urgency %s accepted", level)
		}
	}
	invalid = base
	invalid.PriorityReason = "  "
	if _, err := services.TicketService.CreateTicket(invalid, op); err == nil {
		t.Fatal("missing reason accepted")
	}
	creator := *op
	creator.Roles = []string{}
	creator.Permissions = []string{constants.PermissionTicketView.Code, constants.PermissionTicketCreate.Code}
	if _, err := services.TicketService.CreateTicket(base, &creator); err == nil {
		t.Fatal("create-only actor overrode priority")
	}
	var after int64
	db.Model(&models.Ticket{}).Count(&after)
	if before != after {
		t.Fatal("rejected creation left tickets")
	}
	// Audit storage failure must roll back ticket creation as well.
	name := "initial_priority_audit_failure"
	if err := db.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
		if row, ok := tx.Statement.Dest.(*models.TicketProgress); ok && row.EventType == "ticket_governance" {
			tx.AddError(errors.New("audit unavailable"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Create().Remove(name) })
	if _, err := services.TicketService.CreateTicket(base, op); err == nil {
		t.Fatal("audit failure accepted")
	}
	db.Model(&models.Ticket{}).Count(&after)
	if before != after {
		t.Fatal("audit failure left an orphan ticket")
	}
}
