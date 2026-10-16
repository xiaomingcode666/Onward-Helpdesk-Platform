package dashboard

import (
	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/i18nx"
	"remotehelpdesk/internal/services"
	"strconv"
	"strings"

	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/common/strs"
	"github.com/mlogclub/simple/sqls"
	"github.com/mlogclub/simple/web"
	"github.com/spf13/cast"
)

func ConversationAnyList(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	cnd := params.NewPagedSqlCnd(ctx,
		params.QueryFilter{ParamName: "status"},
		params.QueryFilter{ParamName: "serviceMode"},
		params.QueryFilter{ParamName: "currentAssigneeId"},
		params.QueryFilter{ParamName: "productId"},
		params.QueryFilter{ParamName: "productModelId"},
		params.QueryFilter{ParamName: "deviceId"},
		params.QueryFilter{ParamName: "serviceCodeId"},
	).Desc("last_message_at").Desc("id")
	// 强制 tenant_id 过滤：从 AuthPrincipal 派生，不接受客户端传入
	if operator.TenantID > 0 {
		cnd.Eq("tenant_id", operator.TenantID)
	}

	paging := params.GetPaging(ctx)

	if keyword, _ := params.Get(ctx, "keyword"); strs.IsNotBlank(keyword) {
		keywordLike := "%" + strings.TrimSpace(keyword) + "%"
		cnd.Where("customer_name LIKE ? OR last_message_summary LIKE ?", keywordLike, keywordLike)
	}

	// 标签搜索
	if tagID, _ := params.GetInt64(ctx, "tagId"); tagID > 0 {
		tagIDs := services.TagService.GetSelfAndDescendantIDs(tagID)
		if len(tagIDs) == 0 {
			httpx.WriteJSON(ctx, &web.PageResult{
				Results: []response.ConversationResponse{},
				Page:    paging,
			})
			return
		}
		cnd.Where("id IN (SELECT conversation_id FROM conversation_tag_rels WHERE tag_id IN (?))", tagIDs)
	}
	if agentTeamID, _ := params.GetInt64(ctx, "agentTeamId"); agentTeamID > 0 {
		applyConversationAgentTeamFilter(cnd, agentTeamID)
	}

	list, paging := services.ConversationService.FindPageByCnd(cnd)
	results := make([]response.ConversationResponse, 0, len(list))
	for _, item := range list {
		results = append(results, builders.BuildConversationWithLocale(&item, i18nx.Locale(ctx)))
	}
	httpx.WriteJSON(ctx, &web.PageResult{Results: results, Page: paging})
}

func applyConversationAgentTeamFilter(cnd *sqls.Cnd, agentTeamID int64) {
	if cnd == nil || agentTeamID <= 0 {
		return
	}
	userIDs := services.AgentProfileService.GetUserIDsByTeamID(agentTeamID)
	if len(userIDs) == 0 {
		cnd.Eq("current_team_id", agentTeamID)
		return
	}
	cnd.Where("current_team_id = ? OR current_assignee_id IN ?", agentTeamID, userIDs)
}

func ConversationAnyConversations(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	filterValue, _ := params.Get(ctx, "filter")
	keyword, _ := params.Get(ctx, "keyword")
	paging := params.GetPaging(ctx)

	list, paging, err := services.ConversationService.ListConversations(
		operator.TenantID,
		operator.UserID,
		request.AgentConversationFilter(strings.TrimSpace(filterValue)),
		keyword,
		paging,
		operator,
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	results := make([]response.ConversationResponse, 0, len(list))
	for _, item := range list {
		results = append(results, builders.BuildConversationWithLocale(&item, i18nx.Locale(ctx)))
	}
	httpx.WriteJSON(ctx, &web.PageResult{Results: results, Page: paging})
}

func ConversationGetBy(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}

	item := services.ConversationService.Get(id)
	if item == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0116"))
		return
	}
	if !services.ConversationService.CanAccessConversation(item, operator) {
		httpx.WriteJSON(ctx, errorsx.ForbiddenI18n("error.e0225"))
		return
	}

	detail := response.ConversationDetailResponse{
		ConversationResponse: builders.BuildConversationWithLocale(item, i18nx.Locale(ctx)),
		Participants:         builders.BuildParticipantResponses(id),
	}
	httpx.WriteJSON(ctx, detail)
}

func ConversationAnyMessage_list(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	var (
		conversationID, _ = params.GetInt64(ctx, "conversationId")
		senderType, _     = params.Get(ctx, "senderType")
		messageType, _    = params.Get(ctx, "messageType")
		cursor, _         = params.GetInt64(ctx, "cursor")
		limit, _          = params.GetInt(ctx, "limit")
	)
	conversation := services.ConversationService.Get(conversationID)
	if conversation == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0116"))
		return
	}
	if !services.ConversationService.CanAccessConversation(conversation, operator) {
		httpx.WriteJSON(ctx, errorsx.ForbiddenI18n("error.e0225"))
		return
	}

	list, nextCursor, hasMore := services.MessageService.FindByConversationIDCursor(
		conversationID, cursor, limit, senderType, messageType,
	)
	results := builders.BuildMessagesWithLocale(list, i18nx.Locale(ctx))

	httpx.WriteJSON(ctx, httpx.CursorData(results, cast.ToString(nextCursor), hasMore))
}

func ConversationPostAssign(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationAssign)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	req := request.AssignConversationRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.ConversationService.AssignConversation(req, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func ConversationPostDispatch(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationAssign)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	req := request.DispatchConversationRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.ConversationService.AutoAssignConversation(req.ConversationID, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func ConversationPostTransfer(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationTransfer)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	req := request.TransferConversationRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.ConversationService.TransferConversation(req.ConversationID, req.ToUserID, req.Reason, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func ConversationPostClose(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationClose)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	req := request.CloseConversationRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.ConversationService.CloseConversation(req.ConversationID, req.CloseReason, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func ConversationPostLink_customer(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationLinkCustomer)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.LinkConversationCustomerRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.ConversationService.LinkConversationCustomer(req.ConversationID, req.CustomerID, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func ConversationPostSend_message(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationSend)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	req := request.SendConversationMessageRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.MessageService.SendAgentMessageWithRequestID(req.ConversationID, 0, req.ClientMsgID, req.MessageType, req.Content, req.Payload, operator, httpx.GetRequestID(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildMessageWithLocale(item, i18nx.Locale(ctx)))
}

func ConversationPostForward_image(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationSend)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.ForwardConversationImageRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.MessageService.ForwardAgentImage(
		req.SourceMessageID,
		req.TargetConversationID,
		req.ClientMsgID,
		operator,
		httpx.GetRequestID(ctx),
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildMessageWithLocale(item, i18nx.Locale(ctx)))
}

func ConversationPostRecall_message(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationSend)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	req := request.RecallConversationMessageRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.MessageService.RecallAgentMessage(req.MessageID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildMessageWithLocale(item, i18nx.Locale(ctx)))
}

func ConversationPostRead(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	req := request.ReadConversationRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.ConversationService.MarkAgentConversationReadToMessage(req.ConversationID, req.MessageID, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func ConversationPostUpload_image(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationSend)
	if err != nil {
		httpx.WriteJSON(ctx, err)
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
	if _, err := services.MessageService.ValidateConversationSender(conversationID, enums.IMSenderTypeAgent, operator, nil); err != nil {
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

	item, err := services.AssetService.UploadConversationFile(header, "images", conversationID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildMessageAsset(item))
}

func ConversationPostUpload_attachment(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationSend)
	if err != nil {
		httpx.WriteJSON(ctx, err)
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
	if _, err := services.MessageService.ValidateConversationSender(conversationID, enums.IMSenderTypeAgent, operator, nil); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	header, err := ctx.FormFile("file")
	if err != nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0324"))
		return
	}
	item, err := services.AssetService.UploadConversationFile(header, "attachments", conversationID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildMessageAsset(item))
}

func ConversationPostUpload_audio(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationSend)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	conversationID, err := strconv.ParseInt(strings.TrimSpace(params.FormValue(ctx, "conversationId")), 10, 64)
	if err != nil || conversationID <= 0 {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0064"))
		return
	}
	if _, err := services.MessageService.ValidateConversationSender(conversationID, enums.IMSenderTypeAgent, operator, nil); err != nil {
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
	item, err := services.AssetService.UploadConversationFile(header, "audio", conversationID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildMessageAsset(item))
}

func ConversationPostAdd_tag(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationTag)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	req := request.AddConversationTagRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.ConversationTagService.AddTag(req, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func ConversationPostRemove_tag(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationTag); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	req := request.RemoveConversationTagRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.ConversationTagService.RemoveTag(req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}
