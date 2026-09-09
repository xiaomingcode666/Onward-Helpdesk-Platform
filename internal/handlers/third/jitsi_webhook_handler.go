package third

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/providers"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/web"
)

const (
	// jitsiWebhookMaxAge 允许的 webhook 最大时间偏差（秒），用于防重放攻击
	jitsiWebhookMaxAge = 300 // 5分钟
)

// jitsiWebhookPayload Jitsi Webhook 事件负载
type jitsiWebhookPayload struct {
	EventID   string `json:"event_id"`
	Action    string `json:"action"`
	Timestamp int64  `json:"timestamp,omitempty"`
	Data      struct {
		MeetingID   string `json:"meeting_id,omitempty"`
		RoomName    string `json:"room,omitempty"`
		DurationMs  int64  `json:"duration_ms,omitempty"`
		Participant *struct {
			ID   string `json:"id,omitempty"`
			Name string `json:"name,omitempty"`
		} `json:"participant,omitempty"`
	} `json:"data"`
}

// JitsiPostWebhook 接收 Jitsi 回调事件
//
// 签名验证流程：
//  1. 从 X-Jitsi-Webhook-Signature 请求头获取签名
//  2. 使用 HMAC-SHA256 对原始 body 进行签名并与请求头对比
//  3. 验证时间戳窗口防止重放攻击
//  4. 事件写入工单时间线
func JitsiPostWebhook(ctx *gin.Context) {
	jp := providers.DefaultJitsiProvider

	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		slog.Error("read jitsi webhook body error", "error", err)
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// 1. 校验签名
	signature := ctx.GetHeader("X-Jitsi-Webhook-Signature")
	if !jp.ValidateWebhookSignature(body, signature) {
		slog.Warn("jitsi webhook signature validation failed")
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// 2. 业务字段和时间戳必须来自签名覆盖的 body。仅事件 ID 允许
	// 从请求头补充，因为它只参与幂等，不决定业务动作。
	var payload jitsiWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		slog.Warn("jitsi webhook body parse failed", "error", err)
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if payload.EventID == "" {
		payload.EventID = ctx.GetHeader("X-Jitsi-Event-Id")
	}
	if payload.Data.Participant == nil {
		payload.Data.Participant = &struct {
			ID   string `json:"id,omitempty"`
			Name string `json:"name,omitempty"`
		}{}
	}

	// 3. 时间窗是签名校验的一部分。缺失或过期的时间戳不能继续处理，
	// 否则截获的合法请求可以被无限重放。
	now := time.Now()
	delta := now.Unix() - payload.Timestamp
	if payload.Timestamp <= 0 || delta > jitsiWebhookMaxAge || delta < -jitsiWebhookMaxAge {
		slog.Warn("reject jitsi webhook outside timestamp window",
			"eventId", payload.EventID,
			"timestamp", payload.Timestamp,
			"maxAge", jitsiWebhookMaxAge)
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	if payload.Action == "" {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
	bodyHash := fmt.Sprintf("%x", sha256.Sum256(body))
	if payload.EventID == "" {
		payload.EventID = "sha256:" + bodyHash
	}

	slog.Info("jitsi webhook received",
		"eventId", payload.EventID,
		"action", payload.Action,
		"room", payload.Data.RoomName)

	// 4. 通过持久化 Inbox 处理，重复 event_id 不会重复执行。
	duplicate, err := services.JitsiWebhookService.Process(ctx.Request.Context(), services.JitsiWebhookProcessInput{
		EventID:         payload.EventID,
		Action:          payload.Action,
		PayloadHash:     bodyHash,
		MeetingID:       payload.Data.MeetingID,
		RoomName:        payload.Data.RoomName,
		ParticipantID:   payload.Data.Participant.ID,
		ParticipantName: payload.Data.Participant.Name,
		OccurredAt:      time.Unix(payload.Timestamp, 0),
		ReceivedAt:      now,
	})
	if err != nil {
		slog.Error("process jitsi webhook failed", "eventId", payload.EventID, "action", payload.Action, "error", err)
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	slog.Info("jitsi webhook processed", "eventId", payload.EventID, "action", payload.Action, "duplicate", duplicate)

	httpx.WriteJSON(ctx, web.JsonSuccess())
}
