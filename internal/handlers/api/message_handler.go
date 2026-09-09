package api

import (
	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/i18nx"
	"remotehelpdesk/internal/services"
	"strconv"
	"strings"

	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

func MessageAnyList(ctx *gin.Context) {
	external := httpx.GetExternalUser(ctx)
	if external == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0150"))
		return
	}

	conversationID, _ := params.GetInt64(ctx, "conversationId")
	if conversationID <= 0 {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0064"))
		return
	}
	conversation := services.ConversationService.Get(conversationID)
	if conversation == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0116"))
		return
	}
	if !services.ConversationService.IsCustomerConversationOwner(conversation, *external) {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0222"))
		return
	}

	var (
		senderType, _  = params.Get(ctx, "senderType")
		messageType, _ = params.Get(ctx, "messageType")
		cursor, _      = params.GetInt64(ctx, "cursor")
		limit, _       = params.GetInt(ctx, "limit")
	)
	list, nextCursor, hasMore := services.MessageService.FindCustomerVisibleByConversationIDCursor(
		conversationID, cursor, limit, senderType, messageType,
	)
	results := builders.BuildCustomerMessagesWithLocale(list, i18nx.Locale(ctx))
	httpx.WriteJSON(ctx, httpx.CursorData(results, cast.ToString(nextCursor), hasMore))
}

func MessagePostSend(ctx *gin.Context) {
	external := httpx.GetExternalUser(ctx)
	if external == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0150"))
		return
	}

	req := request.SendConversationMessageRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.CustomerPortalService.RequireServiceInteractionAccess(*external, httpx.GetCustomerSessionAccess(ctx).TenantID); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	item, err := services.MessageService.SendCustomerMessageWithRequestID(req.ConversationID, req.ClientMsgID, req.MessageType, req.Content, req.Payload, *external, httpx.GetRequestID(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildCustomerMessageWithLocale(item, i18nx.Locale(ctx)))
}

func MessagePostRead(ctx *gin.Context) {
	external := httpx.GetExternalUser(ctx)
	if external == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0150"))
		return
	}

	req := request.ReadConversationRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.ConversationService.MarkCustomerConversationReadToMessage(req.ConversationID, req.MessageID, external); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func MessagePostUpload_image(ctx *gin.Context) {
	external := httpx.GetExternalUser(ctx)
	if external == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0150"))
		return
	}

	rawConv := strings.TrimSpace(params.FormValue(ctx, "conversationId"))
	if rawConv == "" {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0064"))
		return
	}
	conversationID, err := strconv.ParseInt(rawConv, 10, 64)
	if err != nil || conversationID <= 0 {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0064"))
		return
	}
	if err := services.CustomerPortalService.RequireServiceInteractionAccess(*external, httpx.GetCustomerSessionAccess(ctx).TenantID); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	conversation := services.ConversationService.Get(conversationID)
	if conversation == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0116"))
		return
	}
	if !services.ConversationService.IsCustomerConversationOwner(conversation, *external) {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0222"))
		return
	}
	if _, err := services.MessageService.ValidateConversationSender(conversationID, enums.IMSenderTypeCustomer, nil, external); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	header, err := ctx.FormFile("file")
	if err != nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0322"))
		return
	}
	if !strings.HasPrefix(strings.ToLower(header.Header.Get("Content-Type")), "image/") {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0090"))
		return
	}

	item, err := services.AssetService.UploadConversationFile(header, "images", conversationID, customerMessageAssetPrincipal(conversation, external.ExternalName))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildMessageAsset(item))
}

func MessagePostUpload_attachment(ctx *gin.Context) {
	external := httpx.GetExternalUser(ctx)
	if external == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0150"))
		return
	}

	rawConv := strings.TrimSpace(params.FormValue(ctx, "conversationId"))
	if rawConv == "" {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0064"))
		return
	}
	conversationID, err := strconv.ParseInt(rawConv, 10, 64)
	if err != nil || conversationID <= 0 {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0064"))
		return
	}
	if err := services.CustomerPortalService.RequireServiceInteractionAccess(*external, httpx.GetCustomerSessionAccess(ctx).TenantID); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if _, err := services.MessageService.ValidateConversationSender(conversationID, enums.IMSenderTypeCustomer, nil, external); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	conversation := services.ConversationService.Get(conversationID)
	if conversation == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0116"))
		return
	}

	header, err := ctx.FormFile("file")
	if err != nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0324"))
		return
	}
	item, err := services.AssetService.UploadConversationFile(header, "attachments", conversationID, customerMessageAssetPrincipal(conversation, external.ExternalName))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildMessageAsset(item))
}

func MessagePostUpload_audio(ctx *gin.Context) {
	external := httpx.GetExternalUser(ctx)
	if external == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0150"))
		return
	}
	conversationID, err := strconv.ParseInt(strings.TrimSpace(params.FormValue(ctx, "conversationId")), 10, 64)
	if err != nil || conversationID <= 0 {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0064"))
		return
	}
	if err := services.CustomerPortalService.RequireServiceInteractionAccess(*external, httpx.GetCustomerSessionAccess(ctx).TenantID); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	conversation, err := services.MessageService.ValidateConversationSender(conversationID, enums.IMSenderTypeCustomer, nil, external)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	header, err := ctx.FormFile("file")
	if err != nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0324"))
		return
	}
	if !strings.HasPrefix(strings.ToLower(header.Header.Get("Content-Type")), "audio/") {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("audio upload requires an audio file"))
		return
	}
	item, err := services.AssetService.UploadConversationFile(header, "audio", conversationID, customerMessageAssetPrincipal(conversation, external.ExternalName))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildMessageAsset(item))
}

func customerMessageAssetPrincipal(conversation *models.Conversation, displayName string) *dto.AuthPrincipal {
	if conversation == nil {
		return nil
	}
	displayName = strings.TrimSpace(displayName)
	return &dto.AuthPrincipal{
		TenantID:    conversation.TenantID,
		Username:    displayName,
		Nickname:    displayName,
		DomainType:  "customer",
		SubjectType: "customer_user",
	}
}
