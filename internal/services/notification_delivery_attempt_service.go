package services

import (
	"log/slog"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var NotificationDeliveryAttemptService = newNotificationDeliveryAttemptService()

func newNotificationDeliveryAttemptService() *notificationDeliveryAttemptService {
	return &notificationDeliveryAttemptService{}
}

type notificationDeliveryAttemptService struct{}

// Record 记录一次真实的发送尝试，失败原因和最终结果都会各自留痕。
func (s *notificationDeliveryAttemptService) Record(delivery *models.DeliveryLog, status, reason, detail string) {
	if delivery == nil || delivery.ID <= 0 || delivery.TenantID <= 0 {
		return
	}
	s.create(&models.NotificationDeliveryAttempt{
		TenantID:   delivery.TenantID,
		DeliveryID: delivery.ID,
		AttemptNo:  delivery.RetryCount + 1,
		Channel:    strings.ToLower(strings.TrimSpace(delivery.Channel)),
		Status:     strings.ToLower(strings.TrimSpace(status)),
		Reason:     limitText(reason, 64),
		Detail:     limitText(detail, 2000),
		CreatedAt:  time.Now(),
	})
}

// RecordBlocked 记录被敏感信息规则拦截、没有真正发出的通知。
func (s *notificationDeliveryAttemptService) RecordBlocked(item *models.Notification, channel string, finding *NotificationSensitiveFinding, templateCode, language string) {
	if item == nil || item.TenantID <= 0 {
		return
	}
	reason := "sensitive_rule"
	if finding != nil && finding.Code != "" {
		reason = "sensitive_rule:" + finding.Code
	}
	detail := DescribeNotificationSensitiveFinding(finding)
	if code := strings.TrimSpace(templateCode); code != "" {
		detail = "模板 " + code + "（" + strings.TrimSpace(language) + "）：" + detail
	} else if content := strings.TrimSpace(item.Content); content != "" {
		detail = detail + "，内容片段：" + limitText(content, 80)
	}
	s.RecordBlockedWithReason(item, channel, reason, detail)
}

// RecordBlockedWithReason 记录因为规则或治理原因被拦下、没有真正发出的通知。
func (s *notificationDeliveryAttemptService) RecordBlockedWithReason(item *models.Notification, channel, reason, detail string) {
	if item == nil || item.TenantID <= 0 {
		return
	}
	s.create(&models.NotificationDeliveryAttempt{
		TenantID:   item.TenantID,
		DeliveryID: 0,
		AttemptNo:  1,
		Channel:    strings.ToLower(strings.TrimSpace(channel)),
		Status:     NotificationDeliveryAttemptStatusBlocked,
		Reason:     limitText(reason, 64),
		Detail:     limitText(detail, 2000),
		CreatedAt:  time.Now(),
	})
}

func (s *notificationDeliveryAttemptService) create(item *models.NotificationDeliveryAttempt) {
	if item == nil {
		return
	}
	if err := repositories.NotificationDeliveryAttemptRepository.Create(sqls.DB(), item); err != nil {
		slog.Warn("record notification delivery attempt failed", "tenantId", item.TenantID, "deliveryId", item.DeliveryID, "error", err)
	}
}
