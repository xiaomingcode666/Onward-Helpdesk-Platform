package third

import (
	"net/http"
	"net/url"
	"strings"

	"remotehelpdesk/internal/pkg/twilio"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

func buildTwilioWhatsAppInbound(form url.Values, cfg twilio.WhatsAppConfig) services.WhatsAppLocalInbound {
	messageID := strings.TrimSpace(form.Get("MessageSid"))
	from := twilio.NormalizeWhatsAppAddress(form.Get("WaId"))
	if from == "" {
		from = twilio.NormalizeWhatsAppAddress(form.Get("From"))
	}
	return services.WhatsAppLocalInbound{
		EventID:      messageID,
		ChannelID:    cfg.ChannelID,
		TenantID:     cfg.TenantID,
		From:         from,
		ProfileName:  strings.TrimSpace(form.Get("ProfileName")),
		MessageID:    messageID,
		Text:         strings.TrimSpace(form.Get("Body")),
		Provider:     "twilio_whatsapp",
		ClientPrefix: "twilio-whatsapp:",
	}
}

func TwilioWhatsAppPostWebhook(ctx *gin.Context) {
	cfg := twilio.CurrentWhatsAppConfig()
	if !cfg.Enabled {
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}
	if err := ctx.Request.ParseForm(); err != nil {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
	form := ctx.Request.PostForm
	if cfg.ValidateSignature {
		callbackURL := cfg.WebhookURL
		if callbackURL == "" {
			callbackURL = requestCallbackURL(ctx)
		}
		params := make(map[string]string, len(form))
		for key := range form {
			params[key] = form.Get(key)
		}
		if !twilio.ValidateSignature(cfg.AuthToken, callbackURL, params, ctx.GetHeader("X-Twilio-Signature")) {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}
	}
	input := buildTwilioWhatsAppInbound(form, cfg)
	if _, err := services.WhatsAppLocalService.ProcessInbound(ctx.Request.Context(), input); err != nil {
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "too long") {
			ctx.AbortWithStatus(http.StatusBadRequest)
			return
		}
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	ctx.Data(http.StatusOK, "application/xml", []byte("<Response></Response>"))
}

func requestCallbackURL(ctx *gin.Context) string {
	scheme := "http"
	if strings.EqualFold(ctx.GetHeader("X-Forwarded-Proto"), "https") || ctx.Request.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + ctx.Request.Host + ctx.Request.URL.RequestURI()
}
