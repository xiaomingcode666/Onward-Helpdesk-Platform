package third

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/providers"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const jigasiTranscriptMaxBodyBytes = 256 * 1024

const (
	jigasiWebSocketCredentialPurpose = "remotehelpdesk-jigasi-transcription-websocket-v1"
	jigasiEventCredentialPurpose     = "remotehelpdesk-jigasi-transcription-event-v1"
)

var jigasiTranscriptionUpgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool { return true },
}

func JigasiTranscriptionWebSocket(ctx *gin.Context) {
	speechConfig, provider := providers.CurrentSpeechRuntime()
	if !validJigasiCredential(speechConfig, ctx.Param("accessToken"), jigasiWebSocketCredentialPurpose) {
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}
	if provider == nil || !provider.Configured() {
		ctx.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	client, err := jigasiTranscriptionUpgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		slog.Warn("upgrade jigasi transcription websocket failed", "error", err)
		return
	}
	defer client.Close()
	client.SetReadLimit(jigasiTranscriptMaxBodyBytes)
	connectionID := ctx.Param("connectionId")
	gateway := providers.NewJigasiWhisperGateway(provider, speechConfig.Jigasi.MaxStreams()).WithConnectionID(connectionID)
	if err := gateway.Serve(ctx.Request.Context(), client); err != nil && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
		services.SpeechRuntimeService.InvalidateReadiness()
		slog.Warn("jigasi transcription gateway stopped", "connection_id", connectionID, "error", err)
	}
}

func JigasiTranscriptionPostEvent(ctx *gin.Context) {
	speechConfig := providers.CurrentSpeechConfig()
	if !validJigasiCredential(speechConfig, ctx.Param("accessToken"), jigasiEventCredentialPurpose) {
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, jigasiTranscriptMaxBodyBytes)
	var input request.JigasiTranscriptEventRequest
	if err := ctx.ShouldBindJSON(&input); err != nil {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
	ingested, err := services.MeetingIntelligenceService.IngestJigasiTranscriptEvent(ctx.Request.Context(), input)
	if err != nil {
		slog.Error("ingest jigasi transcript event failed", "room", input.RoomName, "message_id", input.MessageID, "error", err)
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if ingested {
		slog.Info("ingest jigasi transcript event", "room", input.RoomName, "message_id", input.MessageID, "event", input.Event)
	} else {
		slog.Info("ignore jigasi transcript event", "room", input.RoomName, "message_id", input.MessageID, "event", input.Event, "interim", input.IsInterim)
	}
	ctx.JSON(http.StatusOK, gin.H{"success": true, "ingested": ingested})
}

func validJigasiCredential(speechConfig config.SpeechConfig, provided, purpose string) bool {
	if !speechConfig.Jigasi.Enabled && !strings.EqualFold(strings.TrimSpace(os.Getenv("SPEECH_JIGASI_ENABLED")), "true") {
		return false
	}
	provided = strings.TrimSpace(provided)
	purpose = strings.TrimSpace(purpose)
	if purpose == "" {
		return false
	}
	secrets := []string{speechConfig.Jigasi.SharedSecret, os.Getenv("SPEECH_JIGASI_SHARED_SECRET")}
	for _, secret := range secrets {
		if expected := strings.TrimSpace(secret); len(expected) >= 32 && constantTimeCredentialEqual(provided, jigasiCredentialToken(expected, purpose)) {
			return true
		}
	}
	return false
}

func jigasiCredentialToken(sharedSecret, purpose string) string {
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(sharedSecret)))
	_, _ = mac.Write([]byte(strings.TrimSpace(purpose)))
	return hex.EncodeToString(mac.Sum(nil))
}

func constantTimeCredentialEqual(provided, expected string) bool {
	return len(provided) == len(expected) && subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}
