package enterprise

import (
	"strconv"

	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

func ConversationMessageTranslate(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	conversationID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || conversationID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid conversation id"))
		return
	}
	var req request.TranslateConversationMessageRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.ConversationTranslationService.Translate(
		ctx.Request.Context(),
		tenantID,
		conversationID,
		req,
		enterpriseActionOperator(ctx, tenantID),
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildConversationMessageTranslation(result.Translation, result.Cached))
}
