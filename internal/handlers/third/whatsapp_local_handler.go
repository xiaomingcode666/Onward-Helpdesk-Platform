package third

import (
	"net/http"
	"os"
	"strings"

	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

const defaultWhatsAppLocalVerifyToken = "local-whatsapp-test-token"

func whatsappLocalVerifyToken() string {
	if value := strings.TrimSpace(os.Getenv("RHD_WHATSAPP_LOCAL_VERIFY_TOKEN")); value != "" {
		return value
	}
	return defaultWhatsAppLocalVerifyToken
}

// WhatsAppLocalGetWebhook implements the Meta verification handshake for the
// local-only simulator. It is registered only when explicitly enabled.
func WhatsAppLocalGetWebhook(ctx *gin.Context) {
	mode := strings.TrimSpace(ctx.Query("hub.mode"))
	verifyToken := strings.TrimSpace(ctx.Query("hub.verify_token"))
	challenge := ctx.Query("hub.challenge")
	if mode != "subscribe" || verifyToken == "" || verifyToken != whatsappLocalVerifyToken() {
		ctx.AbortWithStatus(http.StatusForbidden)
		return
	}
	ctx.String(http.StatusOK, challenge)
}

// WhatsAppLocalPostWebhook accepts a small deterministic payload for local
// development. It intentionally does not claim to be a Meta payload.
func WhatsAppLocalPostWebhook(ctx *gin.Context) {
	var input services.WhatsAppLocalInbound
	if err := ctx.ShouldBindJSON(&input); err != nil {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
	result, err := services.WhatsAppLocalService.ProcessInbound(ctx.Request.Context(), input)
	if err != nil {
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "too long") {
			ctx.AbortWithStatus(http.StatusBadRequest)
			return
		}
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	ctx.JSON(http.StatusOK, result)
}
