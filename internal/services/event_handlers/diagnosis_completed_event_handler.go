package event_handlers

import (
	"context"
	"log/slog"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/eventbus"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	eventbus.Register[events.DiagnosisCompletedEvent]().Subscribe(handleDiagnosisCompleted)
}

// handleDiagnosisCompleted 诊断完成事件处理
// - 如果是 escalated 状态，判断是否需要转人工
// - 更新会话状态
func handleDiagnosisCompleted(ctx context.Context, event events.DiagnosisCompletedEvent) error {
	slog.Info("handling diagnosis completed event",
		"sessionId", event.SessionID,
		"status", event.Status,
		"confidence", event.ConfidenceScore,
		"tenantId", event.TenantID)

	if event.SessionID == "" {
		slog.Warn("diagnosis completed event with empty session id")
		return nil
	}

	if event.Status == "escalated" {
		// 更新会话为已转人工
		sqls.DB().Model(&models.DiagnosisSession{}).Where("id = ?", event.SessionID).
			UpdateColumn("status", "escalated")

		slog.Info("diagnosis session escalated to human",
			"sessionId", event.SessionID,
			"confidence", event.ConfidenceScore,
			"reason", event.HandoffReason)

		// 如果转人工原因为空，尝试自动判断
		if event.HandoffReason == "" {
			shouldEscalate, reason := services.DiagnosisService.ShouldEscalateToHuman(event.SessionID)
			if shouldEscalate && reason != "" {
				slog.Info("automatic escalation trigger detected",
					"sessionId", event.SessionID,
					"reason", reason)
			}
		}
	} else if event.Status == "resolved" {
		// 问题已解决，更新状态
		sqls.DB().Model(&models.DiagnosisSession{}).Where("id = ?", event.SessionID).
			UpdateColumn("status", "resolved")

		slog.Info("diagnosis session resolved",
			"sessionId", event.SessionID,
			"confidence", event.ConfidenceScore)
	}

	return nil
}
