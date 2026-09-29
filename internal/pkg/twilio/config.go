package twilio

import (
	"os"
	"strconv"
	"strings"
)

type WhatsAppConfig struct {
	Enabled           bool
	AccountSID        string
	AuthToken         string
	From              string
	ChannelID         string
	TenantID          int64
	WebhookURL        string
	ValidateSignature bool
	APIBaseURL        string
}

func CurrentWhatsAppConfig() WhatsAppConfig {
	return WhatsAppConfig{
		Enabled:           envBool("RHD_TWILIO_WHATSAPP_ENABLED"),
		AccountSID:        strings.TrimSpace(os.Getenv("RHD_TWILIO_ACCOUNT_SID")),
		AuthToken:         strings.TrimSpace(os.Getenv("RHD_TWILIO_AUTH_TOKEN")),
		From:              strings.TrimSpace(os.Getenv("RHD_TWILIO_WHATSAPP_FROM")),
		ChannelID:         strings.TrimSpace(os.Getenv("RHD_TWILIO_WHATSAPP_CHANNEL_ID")),
		TenantID:          envInt64("RHD_TWILIO_WHATSAPP_TENANT_ID"),
		WebhookURL:        strings.TrimSpace(os.Getenv("RHD_TWILIO_WHATSAPP_WEBHOOK_URL")),
		ValidateSignature: envBoolDefault("RHD_TWILIO_VALIDATE_SIGNATURE", true),
		APIBaseURL:        strings.TrimRight(strings.TrimSpace(envOr("RHD_TWILIO_API_BASE_URL", "https://api.twilio.com")), "/"),
	}
}

func envBool(name string) bool {
	return envBoolDefault(name, false)
}

func envBoolDefault(name string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	return err == nil && parsed
}

func envInt64(name string) int64 {
	value, _ := strconv.ParseInt(strings.TrimSpace(os.Getenv(name)), 10, 64)
	return value
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
