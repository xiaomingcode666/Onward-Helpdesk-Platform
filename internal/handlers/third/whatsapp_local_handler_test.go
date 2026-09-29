package third

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestWhatsAppLocalGetWebhookReturnsChallengeForLocalToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/third/whatsapp/local/webhook?hub.mode=subscribe&hub.verify_token=local-whatsapp-test-token&hub.challenge=challenge-1", nil)

	WhatsAppLocalGetWebhook(ctx)

	if recorder.Code != http.StatusOK || recorder.Body.String() != "challenge-1" {
		t.Fatalf("verification response = %d %q, want 200 challenge-1", recorder.Code, recorder.Body.String())
	}
}

func TestWhatsAppLocalPostWebhookRejectsInvalidPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/third/whatsapp/local/webhook", nil)

	WhatsAppLocalPostWebhook(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid payload status = %d, want 400", recorder.Code)
	}
}
