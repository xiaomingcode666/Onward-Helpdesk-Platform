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
	f.ticket = models.Ticket{TenantID: 9401, TicketNo: "CASE-HTTP", Title: "知识搜索无法使用", Description: "部分文档无法找到", Channel: "phone", CaseStatus: "new", Status: enums.TicketStatusPendingAcceptance, PriorityCode: "p2", AuditFields: models.AuditFields{CreatedAt: time.Now(), UpdatedAt: time.Now()}}
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
	f.router.POST("/api/enterprise/v1/tickets/:id/case-owner", TicketCaseOwnerTransfer)
	f.router.GET("/api/enterprise/v1/tickets/:id/case-owner-options", TicketCaseOwnerOptions)
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
	rec := f.command(t, "owner", "acknowledge", "new", 0, "case-ack", "客服确认收到问题")
	ack := f.aggregateAfterCommand(t, rec, "acknowledged", 1)
	var originalReceipt services.TicketCaseCommandResult
	decodeEnterpriseData(t, rec, &originalReceipt)
	if ack.CaseLifecycle.OwnerID != f.owner.UserID || ack.CaseLifecycle.AcknowledgedAt == "" || ack.Assignment.AcceptedAt != "" {
		t.Fatalf("reception must record case owner without inventing engineer acceptance: %+v", ack)
	}
	f.aggregateAfterCommand(t, f.command(t, "owner", "acknowledge", "new", 0, "case-ack", "客服确认收到问题"), "acknowledged", 1)
	for _, rec := range []*httptest.ResponseRecorder{
		f.command(t, "owner", "acknowledge", "new", 0, "case-ack", "不同的请求内容"),
		f.command(t, "owner", "triage", "new", 0, "case-stale", "开始排查"),
	} {
		if rec.Code != http.StatusConflict {
			t.Fatalf("stale/different-payload command status = %d: %s", rec.Code, rec.Body.String())
		}
	}
	f.aggregateAfterCommand(t, f.command(t, "owner", "triage", "acknowledged", 1, "case-triage", "核对搜索范围"), "in_triage", 2)
	var replayedReceipt services.TicketCaseCommandResult
	decodeEnterpriseData(t, f.command(t, "owner", "acknowledge", "new", 0, "case-ack", "客服确认收到问题"), &replayedReceipt)
	if replayedReceipt != originalReceipt {
		t.Fatalf("later transitions changed an earlier command receipt: %+v vs %+v", replayedReceipt, originalReceipt)
	}
	waiting := f.aggregateAfterCommand(t, f.command(t, "owner", "wait", "in_triage", 2, "case-wait", "等待客户提供文档名称"), "waiting", 3)
	if waiting.CaseLifecycle.WaitingReason == "" {
		t.Fatal("waiting reason missing from response")
	}
	closeRec := f.request(t, "owner", http.MethodPost, fmt.Sprintf("/api/enterprise/v1/tickets/%d/_close", f.ticket.ID), map[string]string{"resolution": "尝试跳过解决"})
	assertEnterpriseEnvelopeError(t, closeRec)
	resumed := f.aggregateAfterCommand(t, f.command(t, "owner", "resume", "waiting", 3, "case-resume", "客户已提供资料"), "in_triage", 4)
	if resumed.CaseLifecycle.WaitingReason != "" {
		t.Fatal("resuming must clear the waiting reason")
	}
	restored := f.aggregateAfterCommand(t, f.command(t, "owner", "restore", "in_triage", 4, "case-restore", "临时搜索入口已验证可用"), "restored", 5)
	var ticket models.Ticket
	if err := f.db.First(&ticket, f.ticket.ID).Error; err != nil {
		t.Fatal(err)
	}
	if restored.CaseLifecycle.RestoredAt == "" || ticket.ResolvedAt != nil {
		t.Fatal("restoring service must not record final resolution")
	}
	f.aggregateAfterCommand(t, f.command(t, "owner", "resolve", "restored", 5, "case-resolve", "索引已修复，使用三个故障样本验证通过"), "resolved", 6)
	f.aggregateAfterCommand(t, f.command(t, "owner", "request_closure", "resolved", 6, "case-closure", "请客户确认问题已解决"), "closure_pending", 7)
	if err := f.db.First(&ticket, f.ticket.ID).Error; err != nil {
		t.Fatal(err)
	}
	if ticket.ResolvedAt == nil || ticket.CaseOwnerID != f.owner.UserID || ticket.HandledAt != nil {
		t.Fatalf("closure pending must preserve resolution/owner without closing: %+v", ticket)
	}
	var events, operations int64
	f.db.Model(&models.TicketProgress{}).Where("ticket_id = ? AND event_type = ? AND visible_to_customer = ?", ticket.ID, "case_status_changed", false).Count(&events)
	f.db.Model(&models.TicketCaseOperation{}).Where("ticket_id = ?", ticket.ID).Count(&operations)
	if events != 7 || operations != 7 {
		t.Fatalf("replay/conflicts created extra history: events=%d operations=%d", events, operations)
	}
	for _, actor := range []string{"viewer", "foreign"} {
		assertEnterpriseEnvelopeError(t, f.command(t, actor, "close", "closure_pending", 7, "denied-close-"+actor, "无权关闭"))
	}
	closeResult := f.command(t, "owner", "close", "closure_pending", 7, "case-close", "客户确认关闭")
	f.aggregateAfterCommand(t, closeResult, "closed", 8)
	var originalClose services.TicketCaseCommandResult
	decodeEnterpriseData(t, closeResult, &originalClose)
	f.aggregateAfterCommand(t, f.command(t, "owner", "reopen", "closed", 8, "case-reopen", "问题再次出现"), "in_triage", 9)
	var replayClose services.TicketCaseCommandResult
	decodeEnterpriseData(t, f.command(t, "owner", "close", "closure_pending", 7, "case-close", "客户确认关闭"), &replayClose)
	if replayClose != originalClose {
		t.Fatal("close retry did not return original receipt")
	}
	if rec := f.command(t, "owner", "close", "closure_pending", 7, "case-stale-close", "过期页面关闭"); rec.Code != http.StatusConflict {
		t.Fatalf("stale close must return HTTP 409: %d %s", rec.Code, rec.Body.String())
	}
	f.aggregateAfterCommand(t, f.command(t, "owner", "wait", "in_triage", 9, "case-wait-again", "等待客户选择"), "waiting", 10)
	f.aggregateAfterCommand(t, f.command(t, "owner", "cancel", "waiting", 10, "case-cancel", "客户撤销请求"), "cancelled", 11)
}

func TestTicketCaseHTTPAuthorizationAndExplicitOwnerTransfer(t *testing.T) {
	f := newCaseHTTPFixture(t)
	for _, actor := range []string{"viewer", "foreign"} {
		assertEnterpriseEnvelopeError(t, f.command(t, actor, "acknowledge", "new", 0, "denied-"+actor, "无权请求"))
	}
	f.aggregateAfterCommand(t, f.command(t, "owner", "acknowledge", "new", 0, "owner-ack", "客服受理"), "acknowledged", 1)
	// Model an existing technical assignment; the owner-transfer endpoint must
	// leave its engineer, technical group and acceptance timestamp untouched.
	acceptedAt := time.Now().Add(-time.Minute).UTC().Truncate(time.Second)
	if err := f.db.Model(&models.Ticket{}).Where("id = ?", f.ticket.ID).Updates(map[string]any{"case_status": "assigned", "status": enums.TicketStatusProcessing, "current_assignee_id": 7701, "current_team_id": 8801, "accepted_at": acceptedAt}).Error; err != nil {
		t.Fatal(err)
	}
	optionsRec := f.request(t, "owner", http.MethodGet, fmt.Sprintf("/api/enterprise/v1/tickets/%d/case-owner-options", f.ticket.ID), nil)
	var candidates []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	decodeEnterpriseData(t, optionsRec, &candidates)
	if len(candidates) != 2 {
		t.Fatalf("owner candidates must exclude foreign/read-only accounts: %+v", candidates)
	}
	allowed := map[int64]bool{f.owner.UserID: true, f.successor.UserID: true}
	for _, candidate := range candidates {
		if !allowed[candidate.ID] || candidate.Name == "" {
			t.Fatalf("invalid owner candidate: %+v", candidate)
		}
	}
	endpoint := fmt.Sprintf("/api/enterprise/v1/tickets/%d/case-owner", f.ticket.ID)
	transfer := map[string]any{"owner_id": f.successor.UserID, "expected_owner_id": f.owner.UserID, "reason": "交班后由下一位客服继续跟进", "idempotency_key": "owner-transfer-key"}
	assertEnterpriseEnvelopeError(t, f.request(t, "viewer", http.MethodPost, endpoint, transfer))
	assertEnterpriseEnvelopeError(t, f.request(t, "foreign", http.MethodPost, endpoint, transfer))
	transferRec := f.request(t, "owner", http.MethodPost, endpoint, transfer)
	transferred := f.aggregateAfterCommand(t, transferRec, "assigned", 2)
	var firstReceipt services.TicketCaseCommandResult
	decodeEnterpriseData(t, transferRec, &firstReceipt)
	if transferred.CaseLifecycle.OwnerID != f.successor.UserID || transferred.CaseLifecycle.OwnerName != "successor" {
		t.Fatalf("owner handoff not reflected in aggregate: %+v", transferred.CaseLifecycle)
	}
	var persisted models.Ticket
	if err := f.db.First(&persisted, f.ticket.ID).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.CurrentAssigneeID != 7701 || persisted.CurrentTeamID != 8801 || persisted.AcceptedAt == nil || !persisted.AcceptedAt.Equal(acceptedAt) {
		t.Fatalf("case owner handoff changed technical responsibility: %+v", persisted)
	}
	var repeatReceipt services.TicketCaseCommandResult
	decodeEnterpriseData(t, f.request(t, "owner", http.MethodPost, endpoint, transfer), &repeatReceipt)
	if repeatReceipt != firstReceipt {
		t.Fatalf("owner retry returned a different result: %+v vs %+v", repeatReceipt, firstReceipt)
	}
	transfer["reason"] = "同编号篡改原因"
	if rec := f.request(t, "owner", http.MethodPost, endpoint, transfer); rec.Code != http.StatusConflict {
		t.Fatalf("owner key reuse must return HTTP 409: %d %s", rec.Code, rec.Body.String())
	}
	listRec := f.request(t, "successor", http.MethodGet, "/api/enterprise/v1/tickets?case_status=assigned", nil)
	var list dto.EnterpriseListResponse[dto.EnterpriseTicketListItemDTO]
	decodeEnterpriseData(t, listRec, &list)
	if len(list.Items) != 1 || list.Items[0].CaseOwnerID != f.successor.UserID || list.Items[0].CaseStatus != "assigned" {
		t.Fatalf("exact lifecycle list did not expose transferred owner: %+v", list.Items)
	}
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
