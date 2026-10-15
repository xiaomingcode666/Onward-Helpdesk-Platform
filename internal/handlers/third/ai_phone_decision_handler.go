package third

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

const aiPhoneDecisionBodyBytes = 512 * 1024

// AIPhonePostDecision evaluates a text turn with the Helpdesk tenant LLM.
// The endpoint is for local/third-party voice media adapters; it never writes
// a ticket. Ticket creation remains behind /api/third/ai-phone/tickets.
func AIPhonePostDecision(ctx *gin.Context) {
	if !validAIPhoneServiceToken(ctx.GetHeader("Authorization")) {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	lineID := strings.TrimSpace(ctx.GetHeader("X-AI-Phone-Line-ID"))
	if _, ok := aiPhoneTenantForLine(lineID); !ok {
		ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "phone line is not bound to a tenant"})
		return
	}

	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, aiPhoneDecisionBodyBytes)
	var input services.AIPhoneDecisionRequest
	if err := ctx.ShouldBindJSON(&input); err != nil {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
	input.LineID = lineID
	if err := input.ValidateForHTTP(); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	decision, err := services.AIPhoneCallService.DecideText(ctx.Request.Context(), input)
	if err != nil {
		if errors.Is(err, services.ErrAIPhoneDecisionLineNotConfigured) {
			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "phone line is not bound to a tenant"})
			return
		}
		slog.Warn("ai phone decision failed", "call_id", input.CallID, "line_id", lineID, "error", err)
		ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": "ai phone decision failed"})
		return
	}
	ctx.JSON(http.StatusOK, decision)
}
