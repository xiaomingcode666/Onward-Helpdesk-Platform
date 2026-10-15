package third

import (
	"io"
	"log/slog"
	"net/http"

	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

const telnyxVoiceWebhookBodyLimit = 1 << 20

// TelnyxVoicePostWebhook receives Telnyx Voice API call events. It
// authenticates the raw body, queues the event and acknowledges at once;
// Call Control commands, LLM decisions and ticket creation run afterwards.
func TelnyxVoicePostWebhook(ctx *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(ctx.Request.Body, telnyxVoiceWebhookBodyLimit+1))
	if err != nil || len(body) == 0 || len(body) > telnyxVoiceWebhookBodyLimit {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if err := services.AIPhoneWebhookAuthenticator().Authenticate(body, ctx.Request.Header, ctx.Query("token")); err != nil {
		slog.Warn("telnyx voice webhook rejected", "error", err)
		ctx.AbortWithStatus(http.StatusForbidden)
		return
	}
	if err := services.AIPhoneCallService.Enqueue(body); err != nil {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
	ctx.Status(http.StatusOK)
}
