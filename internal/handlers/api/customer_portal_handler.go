package api

import (
	"strconv"

	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/httpx/params"
	"remotehelpdesk/internal/pkg/openidentity"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

func CustomerPortalGetMe(ctx *gin.Context) {
	external, ok := requireCustomerPortalExternal(ctx)
	if !ok {
		return
	}
	result, err := services.CustomerPortalService.GetProfile(*external)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func CustomerPortalPostMeUpdate(ctx *gin.Context) {
	external, ok := requireCustomerPortalExternal(ctx)
	if !ok {
		return
	}
	var req request.UpdateCustomerPortalProfileRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam(err.Error()))
		return
	}
	result, err := services.CustomerPortalService.UpdateProfile(*external, req)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func CustomerPortalPostPresenceHeartbeat(ctx *gin.Context) {
	if _, ok := requireCustomerPortalExternal(ctx); !ok {
		return
	}
	result, err := services.CustomerPortalService.HeartbeatPresence(services.AuthService.GetAuthPrincipal(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func CustomerPortalGetAccountDeletion(ctx *gin.Context) {
	if _, ok := requireCustomerPortalExternal(ctx); !ok {
		return
	}
	item, err := services.CustomerAccountDeletionService.GetStatus(ctx.Request.Context(), services.AuthService.GetAuthPrincipal(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildCustomerAccountDeletion(item))
}

func CustomerPortalPostAccountDeletion(ctx *gin.Context) {
	if _, ok := requireCustomerPortalExternal(ctx); !ok {
		return
	}
	var req request.DeleteCustomerPortalAccountRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam(err.Error()))
		return
	}
	item, err := services.CustomerAccountDeletionService.Delete(
		ctx.Request.Context(), services.AuthService.GetAuthPrincipal(ctx), req,
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildCustomerAccountDeletion(item))
}

func CustomerPortalGetHome(ctx *gin.Context) {
	external, ok := requireCustomerPortalExternal(ctx)
	if !ok {
		return
	}
	principal := services.AuthService.GetAuthPrincipal(ctx)
	tenantID := int64(0)
	if principal != nil {
		tenantID = principal.TenantID
	}
	result, err := services.CustomerPortalService.GetHome(*external, tenantID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func CustomerPortalGetDevicePage(ctx *gin.Context) {
	external, ok := requireCustomerPortalExternal(ctx)
	if !ok {
		return
	}
	if !requireCustomerPortalDeviceFeature(ctx) {
		return
	}
	result, paging, err := services.CustomerPortalService.ListDevicesPage(*external, readCustomerPortalListRequest(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, httpx.PageData(result, paging))
}

func CustomerPortalGetDevices(ctx *gin.Context) {
	external, ok := requireCustomerPortalExternal(ctx)
	if !ok {
		return
	}
	if !requireCustomerPortalDeviceFeature(ctx) {
		return
	}
	result, err := services.CustomerPortalService.ListDevices(*external)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func CustomerPortalGetDeviceAccess(ctx *gin.Context) {
	external, ok := requireCustomerPortalExternal(ctx)
	if !ok {
		return
	}
	if !requireCustomerPortalDeviceFeature(ctx) {
		return
	}
	deviceID, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	accessible, err := services.CustomerPortalService.HasDeviceAccess(*external, deviceID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildCustomerPortalDeviceAccess(deviceID, accessible))
}

func CustomerPortalPostDeviceBind(ctx *gin.Context) {
	principal, ok := requireCustomerPortalBindPrincipal(ctx)
	if !ok {
		return
	}
	if !requireCustomerPortalDeviceFeature(ctx) {
		return
	}
	var req request.BindCustomerDeviceByServiceCodeRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	binding, device, product, err := services.CustomerPortalService.BindDeviceByServiceCode(req, principal)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildCustomerPortalDeviceBinding(binding, device, product))
}

func CustomerPortalGetDeviceManuals(ctx *gin.Context) {
	external, ok := requireCustomerPortalExternal(ctx)
	if !ok {
		return
	}
	if !requireCustomerPortalDeviceFeature(ctx) {
		return
	}
	deviceID, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	result, err := services.CustomerPortalService.ListDeviceManuals(*external, deviceID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

// CustomerPortalGetSystemIntroDocs 系统介绍文档列表（访客可见，仅已上架）
// GET /api/customer/v1/system-intro-docs
func CustomerPortalGetSystemIntroDocs(ctx *gin.Context) {
	if _, ok := requireCustomerPortalExternal(ctx); !ok {
		return
	}
	result, err := services.SystemIntroService.ListPublishedDocs()
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func CustomerPortalGetConversations(ctx *gin.Context) {
	external, ok := requireCustomerPortalExternal(ctx)
	if !ok {
		return
	}
	result, err := services.CustomerPortalService.ListConversations(*external)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func CustomerPortalGetConversationPage(ctx *gin.Context) {
	external, ok := requireCustomerPortalExternal(ctx)
	if !ok {
		return
	}
	result, paging, err := services.CustomerPortalService.ListConversationsPage(*external, readCustomerPortalListRequest(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, httpx.PageData(result, paging))
}

func CustomerPortalPostConversationTranslate(ctx *gin.Context) {
	external, ok := requireCustomerPortalExternal(ctx)
	if !ok {
		return
	}
	conversationID, ok := customerPathInt64(ctx, "id")
	if !ok {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("conversation id is required"))
		return
	}
	var req request.TranslateConversationMessageRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.CustomerPortalService.TranslateConversationMessage(ctx.Request.Context(), *external, conversationID, req)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildConversationMessageTranslation(result.Translation, result.Cached))
}

func CustomerPortalGetTicketPage(ctx *gin.Context) {
	external, ok := requireCustomerPortalExternal(ctx)
	if !ok {
		return
	}
	result, paging, err := services.CustomerPortalService.ListTicketsPage(*external, readCustomerPortalListRequest(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, httpx.PageData(result, paging))
}

func CustomerPortalGetTickets(ctx *gin.Context) {
	external, ok := requireCustomerPortalExternal(ctx)
	if !ok {
		return
	}
	result, err := services.CustomerPortalService.ListTickets(*external)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func CustomerPortalGetTicketDetail(ctx *gin.Context) {
	external, ok := requireCustomerPortalExternal(ctx)
	if !ok {
		return
	}
	ticketID, ok := customerPathInt64(ctx, "id")
	if !ok {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("ticket id is required"))
		return
	}
	result, err := services.CustomerPortalService.GetTicketDetail(*external, ticketID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func CustomerPortalGetMeetingPage(ctx *gin.Context) {
	external, ok := requireCustomerPortalExternal(ctx)
	if !ok {
		return
	}
	result, paging, err := services.CustomerPortalService.ListMeetingsPage(*external, readCustomerPortalListRequest(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, httpx.PageData(result, paging))
}

func CustomerPortalPostTicketFeedback(ctx *gin.Context) {
	external, ok := requireFormalCustomerPortalExternal(ctx)
	if !ok {
		return
	}
	ticketID, ok := customerPathInt64(ctx, "id")
	if !ok {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("ticket id is required"))
		return
	}
	var req request.SubmitTicketFeedbackRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req.TicketID = ticketID
	result, err := services.CustomerPortalService.SubmitTicketFeedback(*external, ticketID, req)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func readCustomerPortalListRequest(ctx *gin.Context) request.CustomerPortalListRequest {
	paging := params.GetPaging(ctx)
	locale := params.FormValue(ctx, "locale")
	if locale == "" {
		locale = params.FormValue(ctx, "language")
	}
	return request.CustomerPortalListRequest{
		Page:           paging.Page,
		Limit:          paging.Limit,
		Locale:         locale,
		Keyword:        params.FormValue(ctx, "keyword"),
		Filter:         params.FormValue(ctx, "filter"),
		DeviceID:       params.FormValueInt64Default(ctx, "deviceId", 0),
		ConversationID: params.FormValueInt64Default(ctx, "conversationId", 0),
		TicketID:       params.FormValueInt64Default(ctx, "ticketId", 0),
		MeetingID:      params.FormValue(ctx, "meetingId"),
	}
}

func CustomerPortalPostTicketConfirm(ctx *gin.Context) {
	external, ok := requireFormalCustomerPortalExternal(ctx)
	if !ok {
		return
	}
	ticketID, ok := customerPathInt64(ctx, "id")
	if !ok {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("ticket id is required"))
		return
	}
	result, err := services.CustomerPortalService.ConfirmTicketResolved(*external, ticketID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func CustomerPortalPostTicketReopen(ctx *gin.Context) {
	external, ok := requireFormalCustomerPortalExternal(ctx)
	if !ok {
		return
	}
	ticketID, ok := customerPathInt64(ctx, "id")
	if !ok {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("ticket id is required"))
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.CustomerPortalService.ReopenTicket(*external, ticketID, req.Reason)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func CustomerPortalGetMeetings(ctx *gin.Context) {
	external, ok := requireCustomerPortalExternal(ctx)
	if !ok {
		return
	}
	result, err := services.CustomerPortalService.ListMeetings(*external)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func CustomerPortalGetMeetingJoin(ctx *gin.Context) {
	external, ok := requireFormalCustomerPortalExternal(ctx)
	if !ok {
		return
	}
	meetingID := ctx.Param("id")
	if meetingID == "" {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("meeting id is required"))
		return
	}
	result, err := services.CustomerPortalService.JoinMeeting(ctx.Request.Context(), *external, meetingID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func CustomerPortalPostMeetingJoined(ctx *gin.Context) {
	external, ok := requireFormalCustomerPortalExternal(ctx)
	if !ok {
		return
	}
	meetingID := ctx.Param("id")
	if meetingID == "" {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("meeting id is required"))
		return
	}
	if err := services.CustomerPortalService.ConfirmMeetingJoined(ctx.Request.Context(), *external, meetingID); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func CustomerPortalPostMeetingLeft(ctx *gin.Context) {
	external, ok := requireFormalCustomerPortalExternal(ctx)
	if !ok {
		return
	}
	if err := services.CustomerPortalService.ConfirmMeetingLeft(ctx.Request.Context(), *external, ctx.Param("id")); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func CustomerPortalPostMeetingHeartbeat(ctx *gin.Context) {
	external, ok := requireFormalCustomerPortalExternal(ctx)
	if !ok {
		return
	}
	if err := services.CustomerPortalService.HeartbeatMeeting(ctx.Request.Context(), *external, ctx.Param("id")); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func CustomerPortalGetMeetingStatus(ctx *gin.Context) {
	external, ok := requireCustomerPortalExternal(ctx)
	if !ok {
		return
	}
	status, err := services.CustomerPortalService.GetMeetingStatus(ctx.Request.Context(), *external, ctx.Param("id"))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, status)
}

func CustomerPortalGetMeetingTranscripts(ctx *gin.Context) {
	external, ok := requireCustomerPortalExternal(ctx)
	if !ok {
		return
	}
	items, err := services.CustomerPortalService.ListMeetingTranscripts(*external, ctx.Param("id"))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildMeetingTranscriptList(items))
}

func CustomerPortalGetMeetingTranscriptPage(ctx *gin.Context) {
	external, ok := requireCustomerPortalExternal(ctx)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(ctx.Query("limit"))
	items, nextCursor, hasMore, err := services.CustomerPortalService.ListMeetingTranscriptPage(
		*external, ctx.Param("id"), ctx.Query("cursor"), limit,
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, httpx.CursorData(builders.BuildMeetingTranscriptList(items), nextCursor, hasMore))
}

func CustomerPortalPostMeetingTranscript(ctx *gin.Context) {
	external, ok := requireFormalCustomerPortalExternal(ctx)
	if !ok {
		return
	}
	var input request.MeetingTranscriptIngestRequest
	if err := ctx.ShouldBindJSON(&input); err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid transcript request"))
		return
	}
	item, err := services.CustomerPortalService.IngestMeetingTranscript(
		ctx.Request.Context(), *external, ctx.Param("id"), input,
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildMeetingTranscript(item))
}

func CustomerPortalPostMobilePushToken(ctx *gin.Context) {
	if _, ok := requireFormalCustomerPortalExternal(ctx); !ok {
		return
	}
	principal := services.AuthService.GetAuthPrincipal(ctx)
	var req request.RegisterMobilePushTokenRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.MobilePushTokenService.RegisterCustomer(
		principal.TenantID,
		principal.UserID,
		principal.SubjectID,
		req,
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, item)
}

func CustomerPortalPostMobilePushTokenRevoke(ctx *gin.Context) {
	if _, ok := requireFormalCustomerPortalExternal(ctx); !ok {
		return
	}
	principal := services.AuthService.GetAuthPrincipal(ctx)
	tokenID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || tokenID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid mobile push token id"))
		return
	}
	if err := services.MobilePushTokenService.Revoke(principal.TenantID, principal.UserID, tokenID); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func requireCustomerPortalExternal(ctx *gin.Context) (*openidentity.ExternalUser, bool) {
	principal := services.AuthService.GetAuthPrincipal(ctx)
	if principal == nil ||
		!principal.IsCustomer() ||
		(principal.SubjectType != models.SubjectTypeCustomerUser && principal.SubjectType != models.SubjectTypePendingCustomer) {
		httpx.WriteJSON(ctx, errorsx.Unauthorized("a signed-in customer account is required"))
		return nil, false
	}
	external := httpx.GetExternalUser(ctx)
	if external == nil || external.ExternalSource != enums.ExternalSourceUser {
		httpx.WriteJSON(ctx, errorsx.Unauthorized("customer identity is not initialized"))
		return nil, false
	}
	return external, true
}

func requireFormalCustomerPortalExternal(ctx *gin.Context) (*openidentity.ExternalUser, bool) {
	external, ok := requireCustomerPortalExternal(ctx)
	if !ok {
		return nil, false
	}
	principal := services.AuthService.GetAuthPrincipal(ctx)
	if principal == nil || principal.SubjectType != models.SubjectTypeCustomerUser {
		httpx.WriteJSON(ctx, errorsx.Forbidden("bind a device before using this action"))
		return nil, false
	}
	return external, true
}

func requireCustomerPortalBindPrincipal(ctx *gin.Context) (*dto.AuthPrincipal, bool) {
	principal := services.AuthService.GetAuthPrincipal(ctx)
	if principal == nil ||
		!principal.IsCustomer() ||
		(principal.SubjectType != models.SubjectTypeCustomerUser && principal.SubjectType != models.SubjectTypePendingCustomer) {
		httpx.WriteJSON(ctx, errorsx.Unauthorized("a signed-in customer account is required"))
		return nil, false
	}
	external := httpx.GetExternalUser(ctx)
	if external == nil || external.ExternalSource != enums.ExternalSourceUser {
		httpx.WriteJSON(ctx, errorsx.Unauthorized("customer identity is not initialized"))
		return nil, false
	}
	return principal, true
}

func requireCustomerPortalDeviceFeature(ctx *gin.Context) bool {
	principal := services.AuthService.GetAuthPrincipal(ctx)
	if principal != nil && principal.TenantID > 0 && services.TenantCapabilityService.KnowledgeSupport(principal.TenantID) {
		httpx.WriteJSON(ctx, errorsx.Forbidden("device access is unavailable for this tenant"))
		return false
	}
	return true
}
