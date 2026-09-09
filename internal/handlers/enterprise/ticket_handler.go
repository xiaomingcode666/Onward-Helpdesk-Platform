package enterprise

import (
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/sqls"
)

func TicketList(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("page_size", ctx.DefaultQuery("limit", "20")))
	filter := parseEnterpriseFilter(ctx.Query("filter"))
	query := services.EnterpriseTicketQuery{
		Page:           page,
		PageSize:       pageSize,
		Status:         firstNonEmpty(ctx.Query("status"), filter["status"]),
		Priority:       firstNonEmpty(ctx.Query("priority"), filter["priority"]),
		Search:         firstNonEmpty(ctx.Query("search"), strings.TrimPrefix(filter["title"], "~")),
		Sort:           ctx.Query("sort"),
		ConversationID: firstNonZero(queryInt64(ctx, "conversation_id"), queryInt64(ctx, "conversationId")),
		ProductID:      firstNonZero(queryInt64(ctx, "product_id"), queryInt64(ctx, "productId")),
		DeviceID:       firstNonZero(queryInt64(ctx, "device_id"), queryInt64(ctx, "deviceId")),
		TeamID:         firstNonZero(queryInt64(ctx, "team_id"), queryInt64(ctx, "teamId")),
		SLABreached:    firstBoolPtr(queryBoolPtr(ctx, "sla_breached"), queryBoolPtr(ctx, "slaBreached")),
		SLARisk:        firstBoolPtr(queryBoolPtr(ctx, "sla_risk"), queryBoolPtr(ctx, "slaRisk")),
		Mine:           queryBool(ctx, "mine"),
	}
	result, err := services.EnterpriseTicketService.ListForOperator(tenantID, query, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func TicketSummary(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	filter := parseEnterpriseFilter(ctx.Query("filter"))
	query := services.EnterpriseTicketQuery{
		ProductID: firstNonZero(queryInt64(ctx, "product_id"), queryInt64(ctx, "productId")),
		DeviceID:  firstNonZero(queryInt64(ctx, "device_id"), queryInt64(ctx, "deviceId")),
		TeamID:    firstNonZero(queryInt64(ctx, "team_id"), queryInt64(ctx, "teamId")),
		Search:    firstNonEmpty(ctx.Query("search"), strings.TrimPrefix(filter["title"], "~")),
		Mine:      queryBool(ctx, "mine"),
	}
	result, err := services.EnterpriseTicketService.SummaryForOperator(tenantID, query, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func TicketGet(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ticketID, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	result, err := services.EnterpriseTicketService.GetAggregateForOperator(tenantID, ticketID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func TicketCustomerOptions(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketCreate); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "100"))
	items, err := services.EnterpriseTicketService.ListCustomerOptions(tenantID, ctx.Query("search"), limit)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, items)
}

func TicketCustomerInvitationDraftCreate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketCreate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	var req struct {
		Title       string `json:"title" binding:"required"`
		Description string `json:"description" binding:"required"`
		Priority    string `json:"priority"`
		DisplayName string `json:"displayName"`
		Email       string `json:"email" binding:"required"`
		CustomerOrg string `json:"customerOrg" binding:"required"`
		Locale      string `json:"locale"`
		Timezone    string `json:"timezone"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.CustomerRegistrationService.InviteCustomerWithDraftTicket(
		tenantID,
		request.EnterpriseCustomerUserInviteRequest{
			DisplayName: req.DisplayName, Email: req.Email, CustomerOrg: req.CustomerOrg,
			Locale: req.Locale, Timezone: req.Timezone,
		},
		request.CreateTicketRequest{
			Title: req.Title, Description: req.Description, Source: string(enums.TicketSourceManual),
			Channel: "enterprise", PriorityCode: enterpriseTicketPriorityCode(req.Priority), TenantID: tenantID,
			CurrentAssigneeID: invitedCustomerDraftAssigneeID(operator),
		},
		operator,
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordEnterpriseTicketAudit(ctx, operator, result.Ticket.TenantID, result.Ticket.ID, "ticket.invitation_draft_created", models.RiskLevelLow, map[string]any{
		"ticketNo": result.Ticket.TicketNo,
		"grantId":  result.Invitation.Grant.ID,
		"status":   result.Ticket.Status,
	})
	httpx.WriteJSON(ctx, &dto.EnterpriseTicketInvitationDraftDTO{
		Ticket: services.EnterpriseTicketService.BuildListItem(*result.Ticket),
		Invitation: &dto.EnterpriseIAMCustomerInviteDTO{
			GrantID: result.Invitation.Grant.ID, InviteCode: result.Invitation.InviteCode,
			RegistrationURL: result.Invitation.RegistrationURL, Email: result.Invitation.Grant.Email,
			CustomerOrgID: result.Invitation.CustomerOrg.ID, CustomerOrgName: result.Invitation.CustomerOrg.Name,
			ExpiresAt: result.Invitation.Grant.ExpiresAt.Format(time.RFC3339),
			EmailSent: result.Invitation.EmailSent, EmailError: result.Invitation.EmailError,
		},
	})
}

func invitedCustomerDraftAssigneeID(operator *dto.AuthPrincipal) int64 {
	if operator != nil && operator.HasRole(services.EnterpriseRoleEngineer) {
		return operator.UserID
	}
	return 0
}

func TicketCreate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketCreate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	var req struct {
		dto.TicketIntakeInput
		IdempotencyKey         string `json:"idempotency_key"`
		Title                  string `json:"title"`
		Description            string `json:"description"`
		Source                 string `json:"source"`
		Channel                string `json:"channel"`
		Priority               string `json:"priority"`
		PriorityCode           string `json:"priority_code"`
		PriorityCodeCamel      string `json:"priorityCode"`
		CustomerID             int64  `json:"customer_id"`
		CustomerIDCamel        int64  `json:"customerId"`
		ConversationID         int64  `json:"conversation_id"`
		ConversationIDCamel    int64  `json:"conversationId"`
		CurrentAssigneeID      int64  `json:"current_assignee_id"`
		CurrentAssigneeIDCamel int64  `json:"currentAssigneeId"`
		ProductID              int64  `json:"product_id"`
		ProductIDCamel         int64  `json:"productId"`
		ProductModelID         int64  `json:"product_model_id"`
		ProductModelIDCamel    int64  `json:"productModelId"`
		ProductModuleID        int64  `json:"product_module_id"`
		ProductModuleIDCamel   int64  `json:"productModuleId"`
		DeviceID               int64  `json:"device_id"`
		DeviceIDCamel          int64  `json:"deviceId"`
		ServiceCodeID          int64  `json:"service_code_id"`
		ServiceCodeIDCamel     int64  `json:"serviceCodeId"`
		ServiceRegion          string `json:"region_code"`
		ServiceRegionCamel     string `json:"serviceRegion"`
		FaultCode              string `json:"fault_code"`
		FaultCodeCamel         string `json:"faultCode"`
		SymptomSummary         string `json:"symptom_summary"`
		SymptomSummaryCamel    string `json:"symptomSummary"`
		DiagnosisSummary       string `json:"diagnosis_summary"`
		DiagnosisSummaryCamel  string `json:"diagnosisSummary"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	source := firstNonEmpty(req.Source, string(enums.TicketSourceManual))
	conversationID := firstNonZero(req.ConversationID, req.ConversationIDCamel)
	if conversationID > 0 && (req.Channel == "phone" || req.TicketIntakeInput != (dto.TicketIntakeInput{})) {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("manual intake cannot use conversation creation"))
		return
	}
	currentAssigneeID := firstNonZero(req.CurrentAssigneeID, req.CurrentAssigneeIDCamel)
	tenant := repositories.PlatformIAMRepository.GetTenant(sqls.DB(), tenantID)
	if conversationID <= 0 && currentAssigneeID <= 0 && tenant != nil && tenant.IsKnowledgeSupportScene() && operator.HasRole(services.EnterpriseRoleEngineer) {
		currentAssigneeID = operator.UserID
	}
	if !services.CanCreateTicketWithAssignee(operator, currentAssigneeID) {
		httpx.WriteJSON(ctx, errorsx.ForbiddenI18n("error.e0225"))
		return
	}
	var item *models.Ticket
	if conversationID > 0 {
		item, err = services.TicketService.CreateFromConversation(request.CreateTicketFromConversationRequest{
			ConversationID:    conversationID,
			Title:             req.Title,
			Description:       req.Description,
			PriorityCode:      enterpriseTicketPriorityCode(firstNonEmpty(req.PriorityCode, req.PriorityCodeCamel, req.Priority)),
			CurrentAssigneeID: currentAssigneeID,
			TenantID:          tenantID,
			ProductID:         firstNonZero(req.ProductID, req.ProductIDCamel),
			ProductModelID:    firstNonZero(req.ProductModelID, req.ProductModelIDCamel),
			ProductModuleID:   firstNonZero(req.ProductModuleID, req.ProductModuleIDCamel),
			DeviceID:          firstNonZero(req.DeviceID, req.DeviceIDCamel),
			ServiceCodeID:     firstNonZero(req.ServiceCodeID, req.ServiceCodeIDCamel),
			ServiceRegion:     firstNonEmpty(req.ServiceRegion, req.ServiceRegionCamel),
			FaultCode:         firstNonEmpty(req.FaultCode, req.FaultCodeCamel),
			SymptomSummary:    firstNonEmpty(req.SymptomSummary, req.SymptomSummaryCamel),
			DiagnosisSummary:  firstNonEmpty(req.DiagnosisSummary, req.DiagnosisSummaryCamel),
		}, operator)
	} else {
		item, err = services.TicketService.CreateTicket(request.CreateTicketRequest{
			TicketIntakeInput: req.TicketIntakeInput,
			IdempotencyKey:    req.IdempotencyKey,
			Title:             req.Title,
			Description:       req.Description,
			Source:            source,
			Channel:           req.Channel,
			PriorityCode:      enterpriseTicketPriorityCode(firstNonEmpty(req.PriorityCode, req.PriorityCodeCamel, req.Priority)),
			CustomerID:        firstNonZero(req.CustomerID, req.CustomerIDCamel),
			ConversationID:    conversationID,
			CurrentAssigneeID: currentAssigneeID,
			TenantID:          tenantID,
			ProductID:         firstNonZero(req.ProductID, req.ProductIDCamel),
			ProductModelID:    firstNonZero(req.ProductModelID, req.ProductModelIDCamel),
			ProductModuleID:   firstNonZero(req.ProductModuleID, req.ProductModuleIDCamel),
			DeviceID:          firstNonZero(req.DeviceID, req.DeviceIDCamel),
			ServiceCodeID:     firstNonZero(req.ServiceCodeID, req.ServiceCodeIDCamel),
			ServiceRegion:     firstNonEmpty(req.ServiceRegion, req.ServiceRegionCamel),
			FaultCode:         firstNonEmpty(req.FaultCode, req.FaultCodeCamel),
			SymptomSummary:    firstNonEmpty(req.SymptomSummary, req.SymptomSummaryCamel),
			DiagnosisSummary:  firstNonEmpty(req.DiagnosisSummary, req.DiagnosisSummaryCamel),
		}, operator)
	}
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordEnterpriseTicketAudit(ctx, operator, item.TenantID, item.ID, "ticket.created", models.RiskLevelLow, map[string]any{
		"ticketNo":         item.TicketNo,
		"source":           item.Source,
		"channel":          item.Channel,
		"source_record_id": item.SourceRecordID,
		"context_status":   item.ContextStatus,
		"status":           item.Status,
	})
	writeEnterpriseTicketListItem(ctx, item.ID)
}

func enterpriseTicketPriorityCode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "critical":
		return "p0"
	case "high":
		return "p1"
	case "medium":
		return "p2"
	case "low":
		return "p3"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func TicketAssign(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketAssign)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ticketID, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	var req struct {
		AssigneeID int64  `json:"assignee_id"`
		ToUserID   int64  `json:"to_user_id"`
		Note       string `json:"note"`
		Reason     string `json:"reason"`
	}
	if err := readOptionalJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	assigneeID := firstNonZero(req.AssigneeID, req.ToUserID)
	reason := firstNonEmpty(req.Note, req.Reason)
	if err := services.TicketLifecycleService.Assign(ticketID, assigneeID, reason, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordEnterpriseTicketAudit(ctx, operator, tenantID, ticketID, models.AuditActionTicketAssigned, models.RiskLevelMedium, map[string]any{
		"toUserId": assigneeID,
		"reason":   reason,
	})
	writeEnterpriseTicketListItem(ctx, ticketID)
}

func TicketTransfer(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketAssign)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ticketID, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	var req struct {
		ToUserID int64  `json:"to_user_id"`
		Reason   string `json:"reason"`
	}
	if err := readOptionalJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.TicketLifecycleService.Transfer(ticketID, req.ToUserID, req.Reason, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordEnterpriseTicketAudit(ctx, operator, tenantID, ticketID, "ticket.transferred", models.RiskLevelMedium, map[string]any{
		"toUserId": req.ToUserID,
		"reason":   req.Reason,
	})
	writeEnterpriseTicketListItem(ctx, ticketID)
}

func TicketAccept(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketChangeStatus)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ticketID, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	var req struct {
		AssigneeID int64 `json:"assignee_id"`
	}
	if err := readOptionalJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	assigneeID := req.AssigneeID
	if assigneeID == 0 {
		assigneeID = operator.UserID
	}
	if err := services.TicketLifecycleService.AcceptWithAudit(ticketID, assigneeID, operator, services.RecordAuditInput{
		TenantID:       tenantID,
		IPAddress:      ctx.ClientIP(),
		UserAgent:      ctx.Request.UserAgent(),
		RequestID:      httpx.GetRequestID(ctx),
		SupportGrantID: operator.SupportGrantID,
	}); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	writeEnterpriseTicketListItem(ctx, ticketID)
}

func TicketTakeover(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketChangeStatus)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ticketID, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	if err := services.TicketLifecycleService.TakeoverWithAudit(ticketID, operator, services.RecordAuditInput{
		TenantID:       tenantID,
		IPAddress:      ctx.ClientIP(),
		UserAgent:      ctx.Request.UserAgent(),
		RequestID:      httpx.GetRequestID(ctx),
		SupportGrantID: operator.SupportGrantID,
	}); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	writeEnterpriseTicketAggregate(ctx, tenantID, ticketID)
}

func TicketClose(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketChangeStatus)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ticketID, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	var req struct {
		Resolution string `json:"resolution"`
	}
	if err := readOptionalJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.TicketLifecycleService.Close(ticketID, req.Resolution, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordEnterpriseTicketAudit(ctx, operator, tenantID, ticketID, models.AuditActionTicketClosed, models.RiskLevelMedium, map[string]any{
		"resolution": req.Resolution,
	})
	writeEnterpriseTicketListItem(ctx, ticketID)
}

func TicketCancel(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketChangeStatus)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ticketID, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := readOptionalJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.TicketLifecycleService.Cancel(ticketID, req.Reason, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordEnterpriseTicketAudit(ctx, operator, tenantID, ticketID, "ticket.cancelled", models.RiskLevelMedium, map[string]any{
		"reason": req.Reason,
	})
	writeEnterpriseTicketListItem(ctx, ticketID)
}

func TicketReopen(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketChangeStatus)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ticketID, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := readOptionalJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.TicketLifecycleService.Reopen(ticketID, req.Reason, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordEnterpriseTicketAudit(ctx, operator, tenantID, ticketID, "ticket.reopened", models.RiskLevelMedium, map[string]any{
		"reason": req.Reason,
	})
	writeEnterpriseTicketListItem(ctx, ticketID)
}

func TicketActions(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ticketID, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	ticket := repositories.TicketRepository.Get(sqls.DB(), ticketID)
	if ticket == nil || ticket.TenantID != tenantID {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("ticket not found"))
		return
	}
	if _, err := services.EnterpriseTicketService.GetAggregateForOperator(tenantID, ticketID, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, services.EnterpriseTicketService.BuildActionsForOperator(ticket, operator))
}

func TicketProgressCreate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketProgress)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ticketID, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	var req struct {
		Content           string `json:"content"`
		VisibleToCustomer bool   `json:"visible_to_customer"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if _, err := services.TicketService.AddProgress(request.CreateTicketProgressRequest{
		TicketID:          ticketID,
		Content:           req.Content,
		VisibleToCustomer: req.VisibleToCustomer,
	}, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordEnterpriseTicketAudit(ctx, operator, tenantID, ticketID, "ticket.progress.created", models.RiskLevelLow, map[string]any{
		"visibleToCustomer": req.VisibleToCustomer,
	})
	writeEnterpriseTicketAggregate(ctx, tenantID, ticketID)
}

func TicketRepairCreate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ticketID, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	var req struct {
		FaultCode         string                      `json:"fault_code"`
		Conclusion        string                      `json:"conclusion"`
		Solution          string                      `json:"solution"`
		RootCause         string                      `json:"root_cause"`
		RepairMethod      string                      `json:"repair_method"`
		ServiceMethod     string                      `json:"service_method"`
		TestResult        string                      `json:"test_result"`
		WarrantyCovered   bool                        `json:"warranty_covered"`
		RemoteResolved    bool                        `json:"remote_resolved"`
		VisibleToCustomer bool                        `json:"visible_to_customer"`
		Parts             []request.RepairPartRequest `json:"parts"`
		CostHours         float64                     `json:"cost_hours"`
		MarkResolved      bool                        `json:"mark_resolved"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if _, err := services.TicketService.CreateRepairRecord(request.CreateTicketRepairRecordRequest{
		TicketID:          ticketID,
		FaultCode:         req.FaultCode,
		Conclusion:        req.Conclusion,
		Solution:          req.Solution,
		RootCause:         req.RootCause,
		RepairMethod:      req.RepairMethod,
		ServiceMethod:     req.ServiceMethod,
		TestResult:        req.TestResult,
		WarrantyCovered:   req.WarrantyCovered,
		RemoteResolved:    req.RemoteResolved,
		VisibleToCustomer: req.VisibleToCustomer,
		Parts:             req.Parts,
		CostHours:         req.CostHours,
		MarkResolved:      req.MarkResolved,
	}, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordEnterpriseTicketAudit(ctx, operator, tenantID, ticketID, "ticket.repair_record.created", models.RiskLevelMedium, map[string]any{
		"faultCode":         req.FaultCode,
		"remoteResolved":    req.RemoteResolved,
		"markResolved":      req.MarkResolved,
		"visibleToCustomer": req.VisibleToCustomer,
	})
	writeEnterpriseTicketAggregate(ctx, tenantID, ticketID)
}

func TicketKnowledgeCandidateList(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ticketID, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	items, err := services.TicketKnowledgeCandidateService.List(tenantID, ticketID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildTicketKnowledgeCandidates(items))
}

func TicketKnowledgeCandidateCreate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ticketID, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	var req request.CreateTicketKnowledgeCandidateRequest
	if err := readOptionalJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.TicketKnowledgeCandidateService.Create(
		tenantID,
		ticketID,
		req,
		enterpriseActionOperator(ctx, tenantID),
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordEnterpriseTicketAudit(ctx, operator, tenantID, ticketID, "ticket.knowledge_candidate.created", models.RiskLevelMedium, map[string]any{
		"candidateId": result.Candidate.ID,
		"created":     result.Created,
	})
	httpx.WriteJSON(ctx, builders.BuildTicketKnowledgeCandidate(result.Candidate, result.Created))
}

func writeEnterpriseTicketAggregate(ctx *gin.Context, tenantID, ticketID int64) {
	result, err := services.EnterpriseTicketService.GetAggregateForOperator(tenantID, ticketID, services.AuthService.GetAuthPrincipal(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func writeEnterpriseTicketListItem(ctx *gin.Context, ticketID int64) {
	ticket := repositories.TicketRepository.Get(sqls.DB(), ticketID)
	if ticket == nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("ticket not found"))
		return
	}
	result, err := services.EnterpriseTicketService.ListForOperator(ticket.TenantID, services.EnterpriseTicketQuery{Page: 1, PageSize: 1, Search: ticket.TicketNo}, services.AuthService.GetAuthPrincipal(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if result == nil || len(result.Items) == 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("ticket not found"))
		return
	}
	httpx.WriteJSON(ctx, result.Items[0])
}

func recordEnterpriseTicketAudit(ctx *gin.Context, operator *dto.AuthPrincipal, tenantID, ticketID int64, action, riskLevel string, afterState map[string]any) {
	if ctx == nil || operator == nil || ticketID <= 0 || action == "" {
		return
	}
	if tenantID <= 0 {
		tenantID = operator.EffectiveTenantID()
	}
	if tenantID <= 0 {
		return
	}
	if afterState == nil {
		afterState = make(map[string]any)
	}
	afterState["ticketId"] = ticketID
	_ = services.AuditService.RecordAudit(ctx.Request.Context(), services.RecordAuditInput{
		TenantID:       tenantID,
		ActorID:        strconv.FormatInt(operator.UserID, 10),
		ActorType:      "user",
		Domain:         "ticket",
		ResourceType:   "ticket",
		ResourceID:     strconv.FormatInt(ticketID, 10),
		Action:         action,
		AfterState:     afterState,
		IPAddress:      ctx.ClientIP(),
		UserAgent:      ctx.Request.UserAgent(),
		RequestID:      httpx.GetRequestID(ctx),
		SupportGrantID: operator.SupportGrantID,
		RiskLevel:      riskLevel,
	})
}

func resolveEnterpriseTicketRoute(ctx *gin.Context) (int64, int64, bool) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return 0, 0, false
	}
	ticketID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || ticketID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid ticket id"))
		return 0, 0, false
	}
	return tenantID, ticketID, true
}

func resolveEnterpriseTenantIDInt(ctx *gin.Context) (int64, bool) {
	tenantID := resolveTenantID(ctx)
	if tenantID == "" {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("tenant is required"))
		return 0, false
	}
	tenantIDInt, err := strconv.ParseInt(tenantID, 10, 64)
	if err != nil || tenantIDInt <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid tenant id"))
		return 0, false
	}
	return tenantIDInt, true
}

func enterpriseActionOperator(ctx *gin.Context, tenantID int64) *dto.AuthPrincipal {
	if operator := services.AuthService.GetAuthPrincipal(ctx); operator != nil {
		copy := *operator
		copy.TenantID = tenantID
		if copy.DomainType == "" {
			copy.DomainType = models.DomainTypeEnterprise
		}
		if copy.Domain == "" {
			copy.Domain = copy.DomainType
		}
		return &copy
	}
	return &dto.AuthPrincipal{
		TenantID:   tenantID,
		UserID:     0,
		Username:   "enterprise-api",
		DomainType: models.DomainTypeEnterprise,
		Domain:     models.DomainTypeEnterprise,
		Status:     enums.StatusOk,
	}
}

func readOptionalJSON(ctx *gin.Context, target any) error {
	if ctx.Request == nil || ctx.Request.Body == nil {
		return nil
	}
	if err := ctx.ShouldBindJSON(target); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return nil
}

func parseEnterpriseFilter(filter string) map[string]string {
	result := make(map[string]string)
	for _, part := range strings.Split(filter, ",") {
		key, value, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			result[key] = value
		}
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func queryInt64(ctx *gin.Context, key string) int64 {
	value := strings.TrimSpace(ctx.Query(key))
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}

func queryBoolPtr(ctx *gin.Context, key string) *bool {
	value := strings.TrimSpace(ctx.Query(key))
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil
	}
	return &parsed
}

func queryBool(ctx *gin.Context, key string) bool {
	value := queryBoolPtr(ctx, key)
	return value != nil && *value
}

func firstBoolPtr(values ...*bool) *bool {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func firstNonZero(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
