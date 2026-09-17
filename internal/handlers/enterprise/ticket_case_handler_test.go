package enterprise

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"remotehelpdesk/internal/handlers/dashboard"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"
)

type caseHTTPFixture struct {
	db        *gorm.DB
	router    *gin.Engine
	actors    map[string]*dto.AuthPrincipal
	ticket    models.Ticket
	owner     *dto.AuthPrincipal
	successor *dto.AuthPrincipal
}

func newCaseHTTPFixture(t *testing.T) *caseHTTPFixture {
	t.Helper()
	db := setupEnterpriseContractDB(t)
	db.Logger = db.Logger.LogMode(logger.Silent)
	if err := db.AutoMigrate(&models.TicketCaseOperation{}, &models.TicketDispatchAttempt{}, &models.AuthSubjectPermissionOverride{}, &models.TicketSupplierCollaboration{}, &models.TicketSupplierCollaborationParticipant{}, &models.PartnerAuthorizationScope{}, &models.TicketQualityClue{}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{9401, 9402} {
		if err := db.Create(&models.Tenant{ID: id, Name: fmt.Sprintf("Case HTTP fixture %d", id), Status: enums.StatusOk}).Error; err != nil {
			t.Fatal(err)
		}
	}
	f := &caseHTTPFixture{db: db, actors: map[string]*dto.AuthPrincipal{}}
	f.owner = f.member(t, "owner", 9401, []string{constants.PermissionTicketView.Code, constants.PermissionTicketChangeStatus.Code})
	f.successor = f.member(t, "successor", 9401, []string{constants.PermissionTicketView.Code, constants.PermissionTicketChangeStatus.Code})
	f.member(t, "viewer", 9401, []string{constants.PermissionTicketView.Code})
	f.member(t, "foreign", 9402, []string{constants.PermissionTicketView.Code, constants.PermissionTicketChangeStatus.Code})
	// The engineer already accepted the dispatch: there is no separate
	// support-agent reception step before case work can start.
	f.ticket = models.Ticket{TenantID: 9401, TicketNo: "CASE-HTTP", Title: "知识搜索无法使用", Description: "部分文档无法找到", Channel: "phone", CaseStatus: "new", Status: enums.TicketStatusProcessing, CurrentAssigneeID: f.owner.UserID, PriorityCode: "p2", AuditFields: models.AuditFields{CreatedAt: time.Now(), UpdatedAt: time.Now()}}
	if err := db.Create(&f.ticket).Error; err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	f.router = gin.New()
	// Only authentication is synthetic; requests go through the actual HTTP
	// handlers, permission checks, services and isolated SQLite persistence.
	f.router.Use(func(ctx *gin.Context) {
		actor := f.actors[ctx.GetHeader("X-Case-Fixture-Actor")]
		if actor != nil {
			ctx.Set("authPrincipal", actor)
			ctx.Set("middlewareAuthPrincipal", actor)
			ctx.Set("tenantId", fmt.Sprint(actor.TenantID))
		}
		ctx.Next()
	})
	f.router.POST("/api/enterprise/v1/tickets/:id/lifecycle", TicketCaseTransition)
	f.router.POST("/api/enterprise/v1/tickets/:id/_close", TicketClose)
	f.router.GET("/api/enterprise/v1/tickets", TicketList)
	f.router.GET("/api/enterprise/v1/tickets/:id", TicketGet)
	f.router.POST("/api/dashboard/ticket/create", dashboard.TicketPostCreate)
	return f
}

func (f *caseHTTPFixture) member(t *testing.T, name string, tenantID int64, permissions []string) *dto.AuthPrincipal {
	t.Helper()
	user := models.User{Username: "case-http-" + name, Nickname: name, Status: enums.StatusOk}
	if err := f.db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	member := models.TenantMember{TenantID: tenantID, UserID: user.ID, MemberNo: name, DisplayName: name, MemberType: "employee", Status: enums.StatusOk}
	if err := f.db.Create(&member).Error; err != nil {
		t.Fatal(err)
	}
	role := models.AuthRole{TenantID: tenantID, DomainType: models.DomainTypeEnterprise, Code: "case-http-" + name, Name: name, Status: enums.StatusOk}
	if err := f.db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&models.AuthRoleBinding{TenantID: tenantID, DomainType: models.DomainTypeEnterprise, RoleID: role.ID, SubjectType: models.SubjectTypeTenantMember, SubjectID: member.ID, Status: enums.StatusOk}).Error; err != nil {
		t.Fatal(err)
	}
	for _, permission := range permissions {
		if err := f.db.Create(&models.AuthRolePermission{TenantID: tenantID, RoleID: role.ID, PermissionCode: permission, Effect: "allow", Status: enums.StatusOk}).Error; err != nil {
			t.Fatal(err)
		}
	}
	actor := &dto.AuthPrincipal{TenantID: tenantID, UserID: user.ID, Username: user.Username, DomainType: models.DomainTypeEnterprise, Status: enums.StatusOk, Permissions: permissions}
	f.actors[name] = actor
	return actor
}

func (f *caseHTTPFixture) request(t *testing.T, actor, method, endpoint string, body any) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, endpoint, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Case-Fixture-Actor", actor)
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	return rec
}

func (f *caseHTTPFixture) command(t *testing.T, actor, action, expected string, revision int64, key, reason string) *httptest.ResponseRecorder {
	t.Helper()
	return f.request(t, actor, http.MethodPost, fmt.Sprintf("/api/enterprise/v1/tickets/%d/lifecycle", f.ticket.ID), services.TicketCaseCommand{Action: action, Reason: reason, ExpectedStatus: expected, ExpectedRevision: &revision, IdempotencyKey: key})
}

func (f *caseHTTPFixture) aggregateAfterCommand(t *testing.T, rec *httptest.ResponseRecorder, status string, revision int64) dto.TicketAggregateDTO {
	t.Helper()
	var receipt services.TicketCaseCommandResult
	decodeEnterpriseData(t, rec, &receipt)
	if receipt.TicketID != f.ticket.ID || receipt.OperationKey == "" || receipt.Status != status || receipt.Revision != revision {
		t.Fatalf("unexpected stable command receipt: %+v", receipt)
	}
	return caseHTTPAggregate(t, f.request(t, "owner", http.MethodGet, fmt.Sprintf("/api/enterprise/v1/tickets/%d", f.ticket.ID), nil), status, revision)
}

func caseHTTPAggregate(t *testing.T, rec *httptest.ResponseRecorder, status string, revision int64) dto.TicketAggregateDTO {
	t.Helper()
	var aggregate dto.TicketAggregateDTO
	decodeEnterpriseData(t, rec, &aggregate)
	if aggregate.CaseLifecycle == nil || aggregate.CaseLifecycle.Status != status || aggregate.CaseLifecycle.Revision != revision || aggregate.Ticket.CaseStatus != status {
		t.Fatalf("unexpected lifecycle response: %+v", aggregate.CaseLifecycle)
	}
	return aggregate
}

func TestTicketCaseHTTPCompleteLifecycleAndConflicts(t *testing.T) {
	f := newCaseHTTPFixture(t)
	// The engineer already accepted the assignment, so triage starts the case
	// directly; no support-agent acknowledgement command exists any more.
	assertEnterpriseEnvelopeError(t, f.command(t, "owner", "acknowledge", "new", 0, "case-ack", "旧页面仍提交客服受理"))
	if current := repositories.TicketRepository.Get(f.db, f.ticket.ID); current == nil || current.CaseStatus != "new" {
		t.Fatalf("removed acknowledgement command changed the case: %+v", current)
	}
	inTriage := f.aggregateAfterCommand(t, f.command(t, "owner", "triage", "new", 0, "case-triage", "核对搜索范围"), "in_triage", 1)
	if inTriage.CaseLifecycle.AcknowledgedAt != "" {
		t.Fatalf("engineer triage must not fabricate a reception timestamp: %+v", inTriage.CaseLifecycle)
	}
	waiting := f.aggregateAfterCommand(t, f.command(t, "owner", "wait", "in_triage", 1, "case-wait", "等待客户提供文档名称"), "waiting", 2)
	if waiting.CaseLifecycle.WaitingReason == "" {
		t.Fatal("waiting reason missing from response")
	}
	closeRec := f.request(t, "owner", http.MethodPost, fmt.Sprintf("/api/enterprise/v1/tickets/%d/_close", f.ticket.ID), map[string]string{"resolution": "尝试跳过解决"})
	assertEnterpriseEnvelopeError(t, closeRec)
	resumed := f.aggregateAfterCommand(t, f.command(t, "owner", "resume", "waiting", 2, "case-resume", "客户已提供资料"), "in_triage", 3)
	if resumed.CaseLifecycle.WaitingReason != "" {
		t.Fatal("resuming must clear the waiting reason")
	}
	restored := f.aggregateAfterCommand(t, f.command(t, "owner", "restore", "in_triage", 3, "case-restore", "临时搜索入口已验证可用"), "restored", 4)
	var ticket models.Ticket
	if err := f.db.First(&ticket, f.ticket.ID).Error; err != nil {
		t.Fatal(err)
	}
	if restored.CaseLifecycle.RestoredAt == "" || ticket.ResolvedAt != nil {
		t.Fatal("restoring service must not record final resolution")
	}
	f.aggregateAfterCommand(t, f.command(t, "owner", "resolve", "restored", 4, "case-resolve", "索引已修复，使用三个故障样本验证通过"), "resolved", 5)
	f.aggregateAfterCommand(t, f.command(t, "owner", "request_closure", "resolved", 5, "case-closure", "请客户确认问题已解决"), "closure_pending", 6)
	if err := f.db.First(&ticket, f.ticket.ID).Error; err != nil {
		t.Fatal(err)
	}
	if ticket.ResolvedAt == nil || ticket.HandledAt != nil {
		t.Fatalf("closure pending must preserve resolution without closing: %+v", ticket)
	}
	var events, operations int64
	f.db.Model(&models.TicketProgress{}).Where("ticket_id = ? AND event_type = ? AND visible_to_customer = ?", ticket.ID, "case_status_changed", false).Count(&events)
	f.db.Model(&models.TicketCaseOperation{}).Where("ticket_id = ?", ticket.ID).Count(&operations)
	if events != 6 || operations != 6 {
		t.Fatalf("replay/conflicts created extra history: events=%d operations=%d", events, operations)
	}
	for _, actor := range []string{"viewer", "foreign"} {
		assertEnterpriseEnvelopeError(t, f.command(t, actor, "close", "closure_pending", 6, "denied-close-"+actor, "无权关闭"))
	}
	closeResult := f.command(t, "owner", "close", "closure_pending", 6, "case-close", "客户确认关闭")
	f.aggregateAfterCommand(t, closeResult, "closed", 7)
	var originalClose services.TicketCaseCommandResult
	decodeEnterpriseData(t, closeResult, &originalClose)
	f.aggregateAfterCommand(t, f.command(t, "owner", "reopen", "closed", 7, "case-reopen", "问题再次出现"), "in_triage", 8)
	var replayClose services.TicketCaseCommandResult
	decodeEnterpriseData(t, f.command(t, "owner", "close", "closure_pending", 6, "case-close", "客户确认关闭"), &replayClose)
	if replayClose != originalClose {
		t.Fatal("close retry did not return original receipt")
	}
	if rec := f.command(t, "owner", "close", "closure_pending", 6, "case-stale-close", "过期页面关闭"); rec.Code != http.StatusConflict {
		t.Fatalf("stale close must return HTTP 409: %d %s", rec.Code, rec.Body.String())
	}
	f.aggregateAfterCommand(t, f.command(t, "owner", "wait", "in_triage", 8, "case-wait-again", "等待客户选择"), "waiting", 9)
	f.aggregateAfterCommand(t, f.command(t, "owner", "cancel", "waiting", 9, "case-cancel", "客户撤销请求"), "cancelled", 10)
}

func TestTicketCaseHTTPAuthorizationRestrictsCaseActions(t *testing.T) {
	f := newCaseHTTPFixture(t)
	for _, actor := range []string{"viewer", "foreign"} {
		assertEnterpriseEnvelopeError(t, f.command(t, actor, "triage", "new", 0, "denied-"+actor, "无权请求"))
	}
	// A member with status permission but no handling rights on this ticket may
	// only cancel; triage belongs to the engineer who took the case.
	outsider := f.member(t, "outsider", 9401, []string{constants.PermissionTicketView.Code, constants.PermissionTicketChangeStatus.Code})
	assertEnterpriseEnvelopeError(t, f.command(t, outsider.Username, "triage", "new", 0, "outsider-triage", "试图替工程师开始分析"))
	f.aggregateAfterCommand(t, f.command(t, "owner", "triage", "new", 0, "owner-triage", "工程师开始分析"), "in_triage", 1)
}

func TestTicketCaseHTTPCreateCannotFabricateLifecycleEvidence(t *testing.T) {
	f := newCaseHTTPFixture(t)
	if err := f.db.AutoMigrate(&models.TicketNoSequence{}, &models.TicketTag{}, &models.Tag{}, &models.TicketContextSnapshot{}); err != nil {
		t.Fatal(err)
	}
	f.owner.Permissions = append(f.owner.Permissions, constants.PermissionTicketCreate.Code)
	rec := f.request(t, "owner", http.MethodPost, "/api/dashboard/ticket/create", map[string]any{
		"title": "新问题", "description": "问题仍待确认", "tenantId": f.owner.TenantID,
		"source": "manual", "channel": "enterprise", "idempotencyKey": "create-case-evidence",
		"resolvedAt":  time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
		"case_status": "closed", "case_owner_id": f.successor.UserID,
		"acknowledged_at": time.Now().UTC().Format(time.RFC3339), "restored_at": time.Now().UTC().Format(time.RFC3339),
	})
	var created response.TicketResponse
	decodeEnterpriseData(t, rec, &created)
	var persisted models.Ticket
	if err := f.db.First(&persisted, created.ID).Error; err != nil {
		t.Fatal(err)
	}
	if created.CaseStatus != "new" || persisted.CaseStatus != "new" || persisted.CaseOwnerID != 0 || persisted.AcknowledgedAt != nil || persisted.RestoredAt != nil || persisted.ResolvedAt != nil || persisted.HandledAt != nil {
		t.Fatalf("create request injected lifecycle completion evidence: %+v", persisted)
	}
}
