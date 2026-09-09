package services

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var MobileConversationPushService = newMobileConversationPushService(NotificationDeliveryService)

type mobileConversationNotificationScheduler interface {
	Schedule(item *models.Notification) error
}

type mobileConversationPushService struct {
	delivery mobileConversationNotificationScheduler
	now      func() time.Time
}

func newMobileConversationPushService(delivery mobileConversationNotificationScheduler) *mobileConversationPushService {
	return &mobileConversationPushService{delivery: delivery, now: time.Now}
}

func (s *mobileConversationPushService) ScheduleReply(conversation *models.Conversation, message *models.Message) error {
	if conversation == nil || message == nil || conversation.ID <= 0 || conversation.TenantID <= 0 || message.ID <= 0 {
		return nil
	}
	switch message.SenderType {
	case enums.IMSenderTypeAgent, enums.IMSenderTypeAI, enums.IMSenderTypePartner:
	default:
		return nil
	}
	userID := s.customerUserID(conversation)
	if userID <= 0 {
		return nil
	}
	customerUser := repositories.CustomerPortalRepository.FindCustomerUserByTenantAndUserID(sqls.DB(), conversation.TenantID, userID)
	if customerUser == nil || customerUser.Status != enums.StatusOk {
		return nil
	}

	idempotencyKey := fmt.Sprintf("conversation:%d:message:%d:customer-push:%d", conversation.ID, message.ID, userID)
	item := repositories.NotificationRepository.FindByIdempotencyKey(sqls.DB(), conversation.TenantID, idempotencyKey)
	if item == nil {
		now := s.now()
		title := "RemoteHelpDesk 新回复"
		if message.SenderType == enums.IMSenderTypeAI {
			title = "智能助手新回复"
		} else if message.SenderType == enums.IMSenderTypePartner {
			title = "协作方新回复"
		}
		content := limitText(buildMessageSummary(message.MessageType, message.Content), 180)
		if content == "" {
			content = "会话中有一条新消息"
		}
		item = &models.Notification{
			IdempotencyKey:   &idempotencyKey,
			TenantID:         conversation.TenantID,
			EventKey:         fmt.Sprintf("conversation:%d:message:%d", conversation.ID, message.ID),
			RecipientUserID:  userID,
			RecipientName:    strings.TrimSpace(customerUser.DisplayName),
			Title:            title,
			Content:          content,
			NotificationType: "conversation_message",
			BizType:          "conversation",
			BizID:            conversation.ID,
			ActionURL:        "/mobile?state=chat&conversationId=" + strconv.FormatInt(conversation.ID, 10),
			DeliveryStatus:   "sent",
			Level:            "info",
			Category:         "conversation",
			Channels:         "push",
			Status:           int(enums.StatusOk),
			CreatedAt:        now,
		}
		if err := repositories.NotificationRepository.Create(sqls.DB(), item); err != nil {
			item = repositories.NotificationRepository.FindByIdempotencyKey(sqls.DB(), conversation.TenantID, idempotencyKey)
			if item == nil {
				return err
			}
		}
	}
	if s.delivery == nil {
		return nil
	}
	return s.delivery.Schedule(item)
}

func (s *mobileConversationPushService) customerUserID(conversation *models.Conversation) int64 {
	participant := repositories.ConversationParticipantRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("conversation_id", conversation.ID).
		Eq("participant_type", enums.IMParticipantTypeCustomer).
		Eq("status", enums.StatusOk))
	if participant == nil {
		return 0
	}
	externalID := strings.TrimSpace(participant.ExternalParticipantID)
	if !strings.HasPrefix(externalID, "user:") {
		return 0
	}
	userID, err := strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(externalID, "user:")), 10, 64)
	if err != nil || userID <= 0 {
		return 0
	}
	return userID
}
