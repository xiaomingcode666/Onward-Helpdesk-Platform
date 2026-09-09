package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/providers"
	"remotehelpdesk/internal/pkg/secretstore"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

const (
	notificationDeliveryStatusPending      = "pending"
	notificationDeliveryStatusWaitingRetry = "waiting_retry"
	notificationDeliveryStatusSent         = "sent"
	notificationDeliveryStatusFailed       = "failed"
	notificationDeliveryStatusSkipped      = "skipped"
	notificationEmailStatusPending         = "email_pending"
	notificationEmailStatusRetrying        = "email_retrying"
	notificationEmailStatusSent            = "email_sent"
	notificationEmailStatusFailed          = "email_failed"
	notificationEmailStatusUnavailable     = "email_unavailable"
	notificationPushStatusUnavailable      = "push_unavailable"
)

type notificationEmailTransport interface {
	SendNotificationEmail(tenantID int64, to, subject, templateName string, data map[string]interface{}) error
}

type notificationPushTransport interface {
	SendNotificationPush(ctx context.Context, platform, token string, message providers.MobilePushMessage) (string, error)
}

type notificationDeliveryService struct {
	emailTransport notificationEmailTransport
	pushTransport  notificationPushTransport
	now            func() time.Time
	leaseTTL       time.Duration
}

var NotificationDeliveryService = newNotificationDeliveryService(EmailNotificationService, providers.DefaultMobilePushProvider)

func newNotificationDeliveryService(emailTransport notificationEmailTransport, pushTransports ...notificationPushTransport) *notificationDeliveryService {
	service := &notificationDeliveryService{
		emailTransport: emailTransport,
		now:            time.Now,
		leaseTTL:       15 * time.Minute,
	}
	if len(pushTransports) > 0 {
		service.pushTransport = pushTransports[0]
	}
	return service
}

func (s *notificationDeliveryService) Schedule(item *models.Notification) error {
	if item == nil || item.ID <= 0 || item.TenantID <= 0 {
		return nil
	}
	if notificationHasChannel(item.Channels, "email") {
		if err := s.scheduleEmail(item); err != nil {
			return err
		}
	}
	if notificationHasChannel(item.Channels, "push") {
		return s.schedulePush(item)
	}
	return nil
}

func (s *notificationDeliveryService) scheduleEmail(item *models.Notification) error {
	if item.ExternalChannelStatus == notificationEmailStatusSent ||
		item.ExternalChannelStatus == notificationEmailStatusFailed ||
		item.ExternalChannelStatus == notificationEmailStatusUnavailable ||
		item.ExternalChannelStatus == "email_skipped" {
		return nil
	}

	key := fmt.Sprintf("notification:%d:email", item.ID)
	if existing := repositories.NotificationDeliveryRepository.FindByIdempotencyKey(sqls.DB(), item.TenantID, key); existing != nil {
		return nil
	}
	recipient := NotificationRecipientSettingService.ResolveEmail(item.TenantID, item.RecipientUserID)
	now := s.now()
	maxRetries, _ := notificationMailRetryPolicy(item.TenantID)
	delivery := &models.DeliveryLog{
		TenantID:       item.TenantID,
		IdempotencyKey: &key,
		NotificationID: item.ID,
		Channel:        "email",
		RecipientID:    recipient,
		Status:         notificationDeliveryStatusPending,
		MaxRetries:     maxRetries,
		NextAttemptAt:  &now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := repositories.NotificationDeliveryRepository.Create(sqls.DB(), delivery); err != nil {
		if existing := repositories.NotificationDeliveryRepository.FindByIdempotencyKey(sqls.DB(), item.TenantID, key); existing != nil {
			return nil
		}
		return err
	}
	return repositories.NotificationRepository.Updates(sqls.DB(), item.ID, map[string]any{
		"external_channel_status": notificationEmailStatusPending,
	})
}

func (s *notificationDeliveryService) schedulePush(item *models.Notification) error {
	if item.RecipientUserID <= 0 {
		return nil
	}
	if item.ExternalChannelStatus == notificationPushStatusUnavailable {
		return nil
	}
	key := fmt.Sprintf("notification:%d:push", item.ID)
	if existing := repositories.NotificationDeliveryRepository.FindByIdempotencyKey(sqls.DB(), item.TenantID, key); existing != nil {
		return nil
	}
	tokens, err := repositories.MobilePushTokenRepository.FindActiveByUser(sqls.DB(), item.TenantID, item.RecipientUserID)
	if err != nil {
		return err
	}
	if len(tokens) == 0 {
		return s.markPushUnavailable(item)
	}
	now := s.now()
	delivery := &models.DeliveryLog{
		TenantID:       item.TenantID,
		IdempotencyKey: &key,
		NotificationID: item.ID,
		Channel:        "push",
		RecipientID:    strconv.FormatInt(item.RecipientUserID, 10),
		Status:         notificationDeliveryStatusPending,
		MaxRetries:     3,
		NextAttemptAt:  &now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := repositories.NotificationDeliveryRepository.Create(sqls.DB(), delivery); err != nil {
		if existing := repositories.NotificationDeliveryRepository.FindByIdempotencyKey(sqls.DB(), item.TenantID, key); existing != nil {
			return nil
		}
		return err
	}
	return nil
}

func (s *notificationDeliveryService) ProcessDue(ctx context.Context, limit int) int {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.recoverPushSchedules(max(limit, 100)); err != nil {
		slog.Warn("recover mobile push deliveries failed", "error", err)
	}
	now := s.now()
	items, err := repositories.NotificationDeliveryRepository.ClaimDue(sqls.DB(), now, limit, now.Add(-s.leaseTTL))
	if err != nil {
		slog.Warn("claim notification deliveries failed", "error", err)
		return 0
	}
	processed := 0
	for i := range items {
		if ctx.Err() != nil {
			break
		}
		var processErr error
		switch strings.ToLower(strings.TrimSpace(items[i].Channel)) {
		case "email":
			processErr = s.processEmailDelivery(&items[i])
		case "push":
			processErr = s.processPushDelivery(ctx, &items[i])
		default:
			processErr = s.finishPushDelivery(&items[i], notificationDeliveryStatusFailed, "unsupported delivery channel", nil, "")
		}
		if processErr != nil {
			slog.Warn("notification delivery failed", "channel", items[i].Channel, "deliveryId", items[i].ID, "notificationId", items[i].NotificationID, "error", processErr)
		}
		processed++
	}
	return processed
}

func (s *notificationDeliveryService) recoverPushSchedules(limit int) error {
	items, err := repositories.NotificationRepository.FindPushWithoutDelivery(sqls.DB(), limit)
	if err != nil {
		return err
	}
	for i := range items {
		if err := s.schedulePush(&items[i]); err != nil {
			return err
		}
	}
	return nil
}

func (s *notificationDeliveryService) processPushDelivery(ctx context.Context, delivery *models.DeliveryLog) error {
	if delivery == nil || delivery.ID <= 0 {
		return nil
	}
	item := repositories.NotificationRepository.Get(sqls.DB(), delivery.NotificationID)
	if item == nil || item.TenantID != delivery.TenantID {
		return s.finishPushDelivery(delivery, notificationDeliveryStatusFailed, "notification no longer exists", nil, "")
	}
	if item.Status != int(enums.StatusOk) || !notificationHasChannel(item.Channels, "push") {
		return s.finishPushDelivery(delivery, notificationDeliveryStatusFailed, "notification is inactive or push channel is disabled", nil, "")
	}
	userID, err := strconv.ParseInt(strings.TrimSpace(delivery.RecipientID), 10, 64)
	if err != nil || userID <= 0 {
		return s.finishPushDelivery(delivery, notificationDeliveryStatusFailed, "push recipient is invalid", nil, "")
	}
	tokens, err := repositories.MobilePushTokenRepository.FindActiveByUser(sqls.DB(), item.TenantID, userID)
	if err != nil {
		return s.retryOrFailPush(delivery, err)
	}
	if len(tokens) == 0 {
		return s.skipPushDelivery(delivery, item, "no active mobile push token")
	}
	if s.pushTransport == nil {
		return s.retryOrFailPush(delivery, errors.New("push transport is unavailable"))
	}

	data := map[string]string{"actionUrl": item.ActionURL}
	if item.BizType == "conversation" && item.BizID > 0 {
		data["state"] = "chat"
		data["conversationId"] = strconv.FormatInt(item.BizID, 10)
	}
	message := providers.MobilePushMessage{
		Title: item.Title,
		Body:  limitText(item.Content, 180),
		Data:  data,
	}
	var (
		providerIDs []string
		lastErr     error
		transient   bool
		delivered   bool
	)
	for i := range tokens {
		plainToken, decryptErr := secretstore.Decrypt(tokens[i].TokenCiphertext)
		if decryptErr != nil || strings.TrimSpace(plainToken) == "" {
			if decryptErr == nil {
				decryptErr = errors.New("decrypted push token is empty")
			}
			lastErr = decryptErr
			transient = true
			continue
		}
		providerID, sendErr := s.pushTransport.SendNotificationPush(ctx, tokens[i].Platform, plainToken, message)
		if sendErr == nil {
			delivered = true
			if providerID = strings.TrimSpace(providerID); providerID != "" {
				providerIDs = append(providerIDs, providerID)
			}
			continue
		}
		lastErr = sendErr
		if providers.IsInvalidMobilePushToken(sendErr) {
			if revokeErr := repositories.MobilePushTokenRepository.Revoke(sqls.DB(), item.TenantID, userID, tokens[i].ID, s.now()); revokeErr != nil {
				slog.Warn("revoke invalid mobile push token failed", "tokenId", tokens[i].ID, "error", revokeErr)
			}
			continue
		}
		if !providers.IsPermanentMobilePushError(sendErr) {
			transient = true
		}
	}
	if delivered {
		now := s.now()
		return s.finishPushDelivery(delivery, notificationDeliveryStatusSent, "", &now, strings.Join(providerIDs, ","))
	}
	if transient {
		return s.retryOrFailPush(delivery, lastErr)
	}
	return s.finishPushDelivery(delivery, notificationDeliveryStatusFailed, truncateNotificationDeliveryError(lastErr), nil, "")
}

func (s *notificationDeliveryService) retryOrFailPush(delivery *models.DeliveryLog, cause error) error {
	now := s.now()
	retryCount := delivery.RetryCount + 1
	errorMessage := truncateNotificationDeliveryError(cause)
	if retryCount <= delivery.MaxRetries {
		nextAttemptAt := now.Add(time.Duration(retryCount) * time.Minute)
		if err := repositories.NotificationDeliveryRepository.Updates(sqls.DB(), delivery.ID, map[string]any{
			"status":          notificationDeliveryStatusWaitingRetry,
			"retry_count":     retryCount,
			"next_attempt_at": nextAttemptAt,
			"error_msg":       errorMessage,
			"updated_at":      now,
		}); err != nil {
			return err
		}
		return cause
	}
	delivery.RetryCount = retryCount
	if err := s.finishPushDelivery(delivery, notificationDeliveryStatusFailed, errorMessage, nil, ""); err != nil {
		return err
	}
	return cause
}

func (s *notificationDeliveryService) finishPushDelivery(delivery *models.DeliveryLog, status, errorMessage string, sentAt *time.Time, providerMessageID string) error {
	now := s.now()
	columns := map[string]any{
		"status":          status,
		"retry_count":     delivery.RetryCount,
		"next_attempt_at": nil,
		"error_msg":       errorMessage,
		"provider_msg_id": limitText(providerMessageID, 255),
		"updated_at":      now,
	}
	if sentAt != nil {
		columns["sent_at"] = *sentAt
	}
	return repositories.NotificationDeliveryRepository.Updates(sqls.DB(), delivery.ID, columns)
}

func (s *notificationDeliveryService) markPushUnavailable(item *models.Notification) error {
	if item == nil || item.ID <= 0 {
		return nil
	}
	item.ExternalChannelStatus = notificationPushStatusUnavailable
	return repositories.NotificationRepository.Updates(sqls.DB(), item.ID, map[string]any{
		"external_channel_status": notificationPushStatusUnavailable,
	})
}

func (s *notificationDeliveryService) skipPushDelivery(delivery *models.DeliveryLog, item *models.Notification, reason string) error {
	now := s.now()
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if delivery != nil && delivery.ID > 0 {
			if err := repositories.NotificationDeliveryRepository.Updates(ctx.Tx, delivery.ID, map[string]any{
				"status":          notificationDeliveryStatusSkipped,
				"retry_count":     delivery.RetryCount,
				"next_attempt_at": nil,
				"error_msg":       limitText(reason, 500),
				"updated_at":      now,
			}); err != nil {
				return err
			}
		}
		if item != nil && item.ID > 0 {
			item.ExternalChannelStatus = notificationPushStatusUnavailable
			return repositories.NotificationRepository.Updates(ctx.Tx, item.ID, map[string]any{
				"external_channel_status": notificationPushStatusUnavailable,
			})
		}
		return nil
	})
}

func (s *notificationDeliveryService) processEmailDelivery(delivery *models.DeliveryLog) error {
	if delivery == nil || delivery.ID <= 0 {
		return nil
	}
	item := repositories.NotificationRepository.Get(sqls.DB(), delivery.NotificationID)
	if item == nil || item.TenantID != delivery.TenantID {
		return s.finishDelivery(delivery, nil, notificationDeliveryStatusFailed, notificationEmailStatusFailed, "notification no longer exists", nil)
	}
	if item.ExternalChannelStatus == notificationEmailStatusSent {
		now := s.now()
		return s.finishDelivery(delivery, item, notificationDeliveryStatusSent, notificationEmailStatusSent, "", &now)
	}
	if item.Status != int(enums.StatusOk) || !notificationHasChannel(item.Channels, "email") {
		return s.finishDelivery(delivery, item, notificationDeliveryStatusFailed, notificationEmailStatusUnavailable, "notification is inactive or email channel is disabled", nil)
	}

	recipient := strings.TrimSpace(delivery.RecipientID)
	if recipient == "" {
		return s.finishDelivery(delivery, item, notificationDeliveryStatusFailed, notificationEmailStatusUnavailable, "recipient email is unavailable", nil)
	}
	if s.emailTransport == nil {
		return s.retryOrFail(delivery, item, fmt.Errorf("email transport is unavailable"))
	}
	err := s.emailTransport.SendNotificationEmail(item.TenantID, recipient, "", "notification_generic", map[string]interface{}{
		"Title":     item.Title,
		"Content":   item.Content,
		"ActionURL": item.ActionURL,
	})
	if err != nil {
		return s.retryOrFail(delivery, item, err)
	}
	now := s.now()
	return s.finishDelivery(delivery, item, notificationDeliveryStatusSent, notificationEmailStatusSent, "", &now)
}

func (s *notificationDeliveryService) retryOrFail(delivery *models.DeliveryLog, item *models.Notification, cause error) error {
	now := s.now()
	retryCount := delivery.RetryCount + 1
	errorMessage := truncateNotificationDeliveryError(cause)
	if retryCount <= delivery.MaxRetries {
		_, retryDelay := notificationMailRetryPolicy(delivery.TenantID)
		nextAttemptAt := now.Add(retryDelay)
		if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			if err := repositories.NotificationDeliveryRepository.Updates(ctx.Tx, delivery.ID, map[string]any{
				"status":          notificationDeliveryStatusWaitingRetry,
				"retry_count":     retryCount,
				"next_attempt_at": nextAttemptAt,
				"error_msg":       errorMessage,
				"updated_at":      now,
			}); err != nil {
				return err
			}
			return repositories.NotificationRepository.Updates(ctx.Tx, item.ID, map[string]any{
				"external_channel_status": notificationEmailStatusRetrying,
			})
		}); err != nil {
			return err
		}
		return cause
	}
	delivery.RetryCount = retryCount
	if err := s.finishDelivery(delivery, item, notificationDeliveryStatusFailed, notificationEmailStatusFailed, errorMessage, nil); err != nil {
		return err
	}
	return cause
}

func (s *notificationDeliveryService) finishDelivery(delivery *models.DeliveryLog, item *models.Notification, deliveryStatus, emailStatus, errorMessage string, sentAt *time.Time) error {
	now := s.now()
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		columns := map[string]any{
			"status":          deliveryStatus,
			"retry_count":     delivery.RetryCount,
			"next_attempt_at": nil,
			"error_msg":       errorMessage,
			"updated_at":      now,
		}
		if sentAt != nil {
			columns["sent_at"] = *sentAt
		}
		if err := repositories.NotificationDeliveryRepository.Updates(ctx.Tx, delivery.ID, columns); err != nil {
			return err
		}
		if item == nil {
			return nil
		}
		return repositories.NotificationRepository.Updates(ctx.Tx, item.ID, map[string]any{
			"external_channel_status": emailStatus,
		})
	})
}

func notificationHasChannel(channels, target string) bool {
	for _, channel := range strings.Split(channels, ",") {
		if strings.EqualFold(strings.TrimSpace(channel), target) {
			return true
		}
	}
	return false
}

func notificationMailRetryPolicy(tenantID int64) (int, time.Duration) {
	policy := "retry_3_10m"
	if setting := repositories.TenantMailSettingRepository.GetByTenantID(sqls.DB(), tenantID); setting != nil {
		if value := strings.TrimSpace(setting.RetryPolicy); value != "" {
			policy = value
		}
	}
	switch policy {
	case "no_retry":
		return 0, 0
	case "retry_1_5m":
		return 1, 5 * time.Minute
	default:
		return 3, 10 * time.Minute
	}
}

func truncateNotificationDeliveryError(err error) string {
	if err == nil {
		return ""
	}
	value := strings.TrimSpace(err.Error())
	if len(value) > 2000 {
		return value[:2000]
	}
	return value
}
