package third

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"remotehelpdesk/internal/pkg/twilio"

	"github.com/gin-gonic/gin"
)

func TestBuildTwilioWhatsAppInboundMapsSandboxForm(t *testing.T) {
	form := url.Values{
		"MessageSid":  []string{"SM123"},
		"From":        []string{"whatsapp:+8613800000000"},
		"Body":        []string{"来自手机的测试"},
		"ProfileName": []string{"本地访客"},
		"WaId":        []string{"8613800000000"},
	}
	cfg := twilio.WhatsAppConfig{ChannelID: "channel-1", TenantID: 7}
	got := buildTwilioWhatsAppInbound(form, cfg)
	if got.Provider != "twilio_whatsapp" || got.ClientPrefix != "twilio-whatsapp:" {
		t.Fatalf("provider metadata = %q/%q", got.Provider, got.ClientPrefix)
	}
	if got.EventID != "SM123" || got.MessageID != "SM123" || got.From != "8613800000000" || got.Text != "来自手机的测试" || got.ProfileName != "本地访客" {
		t.Fatalf("unexpected inbound mapping: %+v", got)
	}
	if got.ChannelID != cfg.ChannelID || got.TenantID != cfg.TenantID {
		t.Fatalf("routing metadata = %+v, want channel=%q tenant=%d", got, cfg.ChannelID, cfg.TenantID)
	}
}

func TestTwilioWhatsAppPostWebhookRejectsMissingSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("RHD_TWILIO_WHATSAPP_ENABLED", "true")
	t.Setenv("RHD_TWILIO_VALIDATE_SIGNATURE", "true")
	t.Setenv("RHD_TWILIO_AUTH_TOKEN", "auth-token")
	t.Setenv("RHD_TWILIO_WHATSAPP_CHANNEL_ID", "channel-1")
	form := url.Values{
		"MessageSid": []string{"SM123"},
		"From":       []string{"whatsapp:+8613800000000"},
		"Body":       []string{"hello"},
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/third/twilio/whatsapp/webhook", strings.NewReader(form.Encode()))
	ctx.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	TwilioWhatsAppPostWebhook(ctx)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("missing signature status = %d, want 401", recorder.Code)
	}
}
