package third

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func postTelnyxVoiceWebhook(target, body string) int {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	TelnyxVoicePostWebhook(ctx)
	return recorder.Code
}

func TestTelnyxVoiceWebhookFailsClosedWithoutCredentials(t *testing.T) {
	t.Setenv("TELNYX_PUBLIC_KEY", "")
	t.Setenv("TELNYX_WEBHOOK_TOKEN", "")
	if code := postTelnyxVoiceWebhook("/api/third/telnyx/voice/webhook?token=anything", `{"data":{}}`); code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", code)
	}
}

func TestTelnyxVoiceWebhookChecksURLToken(t *testing.T) {
	token := strings.Repeat("a", 64)
	t.Setenv("TELNYX_PUBLIC_KEY", "")
	t.Setenv("TELNYX_WEBHOOK_TOKEN", token)

	if code := postTelnyxVoiceWebhook("/api/third/telnyx/voice/webhook?token="+strings.Repeat("b", 64), `{"data":{}}`); code != http.StatusForbidden {
		t.Fatalf("wrong token status = %d, want 403", code)
	}
	if code := postTelnyxVoiceWebhook("/api/third/telnyx/voice/webhook?token="+token, `not json`); code != http.StatusBadRequest {
		t.Fatalf("malformed body status = %d, want 400", code)
	}
	// An event without a call id is acknowledged and ignored.
	if code := postTelnyxVoiceWebhook("/api/third/telnyx/voice/webhook?token="+token, `{"data":{"id":"e1","event_type":"call.hangup","payload":{}}}`); code != http.StatusOK {
		t.Fatalf("valid token status = %d, want 200", code)
	}
}
