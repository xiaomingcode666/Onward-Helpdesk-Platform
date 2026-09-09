package api

import (
	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/i18nx"
	"remotehelpdesk/internal/services"

	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/gin-gonic/gin"
)

func ConversationGetBy(ctx *gin.Context) {
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	external := httpx.GetExternalUser(ctx)
	if external == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0150"))
		return
	}

	item := services.ConversationService.Get(id)
	if item == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0116"))
		return
	}
	if !services.ConversationService.IsCustomerConversationOwner(item, *external) {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0222"))
		return
	}

	httpx.WriteJSON(ctx, builders.BuildCustomerConversationDetailWithLocale(item, i18nx.Locale(ctx)))
}

func ConversationPostCreate_or_match(ctx *gin.Context) {
	external := httpx.GetExternalUser(ctx)
	if external == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0150"))
		return
	}

	req := request.CreateOrMatchConversationRequest{}
	if err := params.ReadStrictJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.CustomerPortalService.RequireServiceInteractionAccess(*external, httpx.GetCustomerSessionAccess(ctx).TenantID); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req.IdempotencyKey = ctx.GetHeader("Idempotency-Key")
	access := httpx.GetCustomerSessionAccess(ctx)
	req.ContextTenantID = access.TenantID
	req.ContextProductID = access.ProductID
	if access.EntrySessionID > 0 {
		if req.CustomerEntrySessionID > 0 && req.CustomerEntrySessionID != access.EntrySessionID {
			httpx.WriteJSON(ctx, errorsx.Forbidden("customer entry session does not belong to current session"))
			return
		}
		req.CustomerEntrySessionID = access.EntrySessionID
	}
	channelID := int64(0)
	fallbackAgentID := int64(0)
	if access.ChannelID > 0 {
		channel := services.ChannelService.Get(access.ChannelID)
		if channel == nil {
			httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0211"))
			return
		}
		channelID = channel.ID
		fallbackAgentID = channel.AIAgentID
	}
	item, err := services.ConversationService.CreateWithContext(*external, channelID, fallbackAgentID, req)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildCustomerConversationDetailWithLocale(item, i18nx.Locale(ctx)))
}

func ConversationPostClose(ctx *gin.Context) {
	external := httpx.GetExternalUser(ctx)
	if external == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0150"))
		return
	}

	req := request.CloseConversationRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.ConversationService.CloseCustomerConversation(req.ConversationID, *external); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func ConversationPostRequest_human(ctx *gin.Context) {
	external := httpx.GetExternalUser(ctx)
	if external == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0150"))
		return
	}
	req := request.RequestHumanConversationRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.CustomerPortalService.RequireServiceInteractionAccess(*external, httpx.GetCustomerSessionAccess(ctx).TenantID); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.ConversationHumanDispatchService.RequestByCustomerForLocale(
		req.ConversationID,
		*external,
		req.Reason,
		httpx.GetRequestID(ctx),
		req.Locale,
	); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}
