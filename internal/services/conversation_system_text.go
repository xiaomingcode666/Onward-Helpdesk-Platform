package services

import (
	"log/slog"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/i18nx"

	"gorm.io/gorm"
)

func prepareConversationSystemMessage(sender enums.IMSenderType, messageType enums.IMMessageType, content, payload, summary string) (string, string, string) {
	if sender != enums.IMSenderTypeSystem || messageType != enums.IMMessageTypeText {
		return content, payload, summary
	}
	enriched, text := i18nx.WithConversationSystemText(payload)
	if text == nil {
		return content, payload, summary
	}
	return text.Content.Render(i18nx.DefaultLocale), enriched, text.Summary.Render(i18nx.DefaultLocale)
}

// ConversationSummaryTexts resolves the language-neutral last event in one
// query. Matching both IDs prevents stale/cross-conversation references from
// exposing another conversation's event. Caller supplies authorized conversations.
func ConversationSummaryTexts(db *gorm.DB, conversations []models.Conversation) map[int64]*i18nx.Text {
	result := make(map[int64]*i18nx.Text)
	ids := make([]int64, 0, len(conversations))
	lastIDs := make(map[int64]int64, len(conversations))
	for _, conversation := range conversations {
		if conversation.LastMessageID > 0 {
			ids = append(ids, conversation.LastMessageID)
			lastIDs[conversation.ID] = conversation.LastMessageID
		}
	}
	if len(ids) == 0 || db == nil {
		return result
	}
	var messages []models.Message
	if err := db.Select("id", "conversation_id", "sender_type", "message_type", "payload", "recalled_at", "send_status").Where("id IN ?", ids).Find(&messages).Error; err != nil {
		slog.Warn("resolve conversation system summaries failed", "error", err)
		return result
	}
	for _, message := range messages {
		if lastIDs[message.ConversationID] != message.ID || message.SenderType != enums.IMSenderTypeSystem || message.MessageType != enums.IMMessageTypeText || message.RecalledAt != nil || message.SendStatus == enums.IMMessageStatusRecalled {
			continue
		}
		if text := i18nx.ConversationSystemTextFromPayload(message.Payload); text != nil {
			summary := text.Summary
			result[message.ConversationID] = &summary
		}
	}
	return result
}
