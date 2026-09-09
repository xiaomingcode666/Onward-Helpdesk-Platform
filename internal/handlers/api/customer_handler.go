package api

import (
	"net/http"
	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/httpx/params"
	"remotehelpdesk/internal/pkg/openidentity"
	"remotehelpdesk/internal/services"
	"strconv"

	"github.com/gin-gonic/gin"
)

func CustomerPostSession_exchange(ctx *gin.Context) {
	channel := services.ChannelService.GetEnabledChannel(ctx)
	if channel == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0209"))
		return
	}
	externalUser, err := openidentity.GetExternalUser(ctx, services.ChannelService.GetUserTokenSecret(channel))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	resp, err := services.CustomerSessionService.Exchange(channel, *externalUser)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, resp)
}

func CustomerPostEntry_session_exchange(ctx *gin.Context) {
	req := request.ExchangeCustomerEntrySessionRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	resp, err := services.CustomerSessionService.ExchangeEntrySession(req)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, resp)
}

func CustomerGetService_code_resolve(ctx *gin.Context) {
	result, err := services.ServiceCodeResolveService.Resolve(ctx.Query("serviceCode"))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	resp := &response.CustomerServiceCodeResolveResponse{
		Valid:        result.Valid,
		EntryState:   string(result.EntryState),
		Reason:       result.Reason,
		ServiceCode:  builders.BuildResolvedServiceCode(result.ServiceCode),
		Tenant:       builders.BuildResolvedTenant(result.Tenant),
		Product:      builders.BuildResolvedProduct(result.Product),
		ProductModel: builders.BuildResolvedProductModel(result.ProductModel),
		Device:       builders.BuildResolvedDevice(result.Device),
	}
	httpx.WriteJSON(ctx, resp)
}

func CustomerGetService_code_qr_image(ctx *gin.Context) {
	size, _ := strconv.Atoi(ctx.DefaultQuery("size", "320"))
	png, err := services.ServiceCodeManagementService.GenerateQRCodePNG(ctx.Query("serviceCode"), size)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "public, max-age=3600")
	ctx.Data(http.StatusOK, "image/png", png)
}

func CustomerPostEntry_session_create(ctx *gin.Context) {
	req := request.CreateCustomerEntrySessionRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	aggregate, err := services.CustomerEntryService.CreateEntrySession(req)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildCustomerEntrySession(
		aggregate.Session,
		aggregate.Tenant,
		aggregate.Product,
		aggregate.ProductModel,
		aggregate.Device,
		aggregate.PrivacyConsent,
		aggregate.ServiceCodeValue,
		aggregate.VisitorToken,
	))
}

func CustomerGetEntry_session_privacy_consent(ctx *gin.Context) {
	entrySessionID, ok := params.GetInt64(ctx, "entrySessionId")
	if !ok {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("entrySessionId is required"))
		return
	}
	item, err := services.CustomerEntryService.GetPrivacyConsent(
		entrySessionID,
		ctx.GetHeader("X-Customer-Entry-Visitor-Id"),
		ctx.GetHeader("X-Customer-Entry-Visitor-Token"),
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildCustomerPrivacyConsent(item))
}

func CustomerPostEntry_session_privacy_consent(ctx *gin.Context) {
	var req request.ConfirmCustomerPrivacyConsentRequest
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.CustomerEntryService.ConfirmPrivacyConsent(
		req,
		ctx.GetHeader("X-Customer-Entry-Visitor-Id"),
		ctx.GetHeader("X-Customer-Entry-Visitor-Token"),
		services.CustomerPrivacyConsentMetadata{
			IPAddress: ctx.ClientIP(),
			UserAgent: ctx.GetHeader("User-Agent"),
			RequestID: httpx.GetRequestID(ctx),
		},
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildCustomerPrivacyConsent(item))
}

func CustomerGetEntry_session_context(ctx *gin.Context) {
	entrySessionID, ok := params.GetInt64(ctx, "entrySessionId")
	if !ok {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("entrySessionId is required"))
		return
	}
	resp, err := services.CustomerEntryService.GetEntrySessionContext(
		entrySessionID,
		ctx.GetHeader("X-Customer-Entry-Visitor-Id"),
		ctx.GetHeader("X-Customer-Entry-Visitor-Token"),
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, resp)
}

func CustomerPostEntry_session_ticket_feedback(ctx *gin.Context) {
	entrySessionID, ok := params.GetInt64(ctx, "entrySessionId")
	if !ok {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("entrySessionId is required"))
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
	result, err := services.CustomerEntryService.SubmitTicketFeedback(
		entrySessionID, ticketID,
		ctx.GetHeader("X-Customer-Entry-Visitor-Id"), ctx.GetHeader("X-Customer-Entry-Visitor-Token"), req,
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func CustomerPostEntry_session_ticket_confirm(ctx *gin.Context) {
	entrySessionID, ok := params.GetInt64(ctx, "entrySessionId")
	if !ok {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("entrySessionId is required"))
		return
	}
	ticketID, ok := customerPathInt64(ctx, "id")
	if !ok {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("ticket id is required"))
		return
	}
	result, err := services.CustomerEntryService.ConfirmTicketResolved(
		entrySessionID, ticketID,
		ctx.GetHeader("X-Customer-Entry-Visitor-Id"), ctx.GetHeader("X-Customer-Entry-Visitor-Token"),
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func CustomerPostEntry_session_ticket_reopen(ctx *gin.Context) {
	entrySessionID, ok := params.GetInt64(ctx, "entrySessionId")
	if !ok {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("entrySessionId is required"))
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
	result, err := services.CustomerEntryService.ReopenTicket(
		entrySessionID, ticketID,
		ctx.GetHeader("X-Customer-Entry-Visitor-Id"), ctx.GetHeader("X-Customer-Entry-Visitor-Token"), req.Reason,
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func CustomerGetEntry_session_meeting_join(ctx *gin.Context) {
	entrySessionID, ok := params.GetInt64(ctx, "entrySessionId")
	if !ok {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("entrySessionId is required"))
		return
	}
	meetingID := ctx.Param("id")
	if meetingID == "" {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("meeting id is required"))
		return
	}
	result, err := services.CustomerEntryService.JoinMeeting(
		ctx.Request.Context(), entrySessionID, meetingID,
		ctx.GetHeader("X-Customer-Entry-Visitor-Id"), ctx.GetHeader("X-Customer-Entry-Visitor-Token"),
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func customerPathInt64(ctx *gin.Context, name string) (int64, bool) {
	value, err := strconv.ParseInt(ctx.Param(name), 10, 64)
	return value, err == nil && value > 0
}

func CustomerPostDevice_binding_confirm(ctx *gin.Context) {
	req := request.ConfirmCustomerDeviceBindingRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	binding, err := services.CustomerEntryService.ConfirmDeviceBinding(req)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildCustomerDeviceBinding(binding, req.EntrySessionID))
}
