package third

import (
	"io"
	"net/http"
	"strings"

	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

const metaWhatsAppWebhookBodyLimit = 2 << 20

func MetaWhatsAppGetWebhook(ctx *gin.Context) {
	mode := strings.TrimSpace(ctx.Query("hub.mode"))
	token := strings.TrimSpace(ctx.Query("hub.verify_token"))
	challenge := strings.TrimSpace(ctx.Query("hub.challenge"))
	if mode != "subscribe" || challenge == "" || !services.MetaWhatsAppService.VerifyToken(token) {
		ctx.AbortWithStatus(http.StatusForbidden)
		return
	}
	ctx.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(challenge))
}

func MetaWhatsAppPostWebhook(ctx *gin.Context) {
	signature := strings.TrimSpace(ctx.GetHeader("X-Hub-Signature-256"))
	if signature == "" {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	body, err := io.ReadAll(io.LimitReader(ctx.Request.Body, metaWhatsAppWebhookBodyLimit+1))
	if err != nil || len(body) == 0 || len(body) > metaWhatsAppWebhookBodyLimit {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
	result, err := services.MetaWhatsAppService.ProcessWebhook(ctx.Request.Context(), body, signature)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "signature") {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Meta WhatsApp webhook was rejected"})
		return
	}
	ctx.JSON(http.StatusOK, result)
}
