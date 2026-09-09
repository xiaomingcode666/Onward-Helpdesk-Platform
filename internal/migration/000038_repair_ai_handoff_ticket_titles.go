package migration

import (
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(38, "repair ai handoff ticket titles", func() error {
		return repairAIHandoffTicketTitles(sqls.DB())
	})
}

func repairAIHandoffTicketTitles(db *gorm.DB) error {
	var tickets []models.Ticket
	if err := db.Where("source = ? AND conversation_id > 0 AND idempotency_key LIKE ?", enums.TicketSourceConversation, "ai-handoff:%").Find(&tickets).Error; err != nil {
		return err
	}

	for i := range tickets {
		var customerMessage models.Message
		if err := db.Where(
			"conversation_id = ? AND sender_type = ? AND created_at <= ? AND recalled_at IS NULL AND send_status NOT IN ?",
			tickets[i].ConversationID,
			enums.IMSenderTypeCustomer,
			tickets[i].CreatedAt,
			[]enums.IMMessageStatus{enums.IMMessageStatusFailed, enums.IMMessageStatusRecalled},
		).Order("id DESC").Take(&customerMessage).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				continue
			}
			return err
		}

		customerTitle := limitMigrationText(customerMessage.Content, 255)
		currentTitle := strings.TrimSpace(tickets[i].Title)
		if customerTitle == "" {
			continue
		}

		updates := map[string]any{}
		if customerTitle != currentTitle {
			var generatedMessageCount int64
			if err := db.Model(&models.Message{}).
				Where("conversation_id = ? AND sender_type <> ? AND content = ?", tickets[i].ConversationID, enums.IMSenderTypeCustomer, currentTitle).
				Count(&generatedMessageCount).Error; err != nil {
				return err
			}
			var postCreationCustomerMessageCount int64
			if err := db.Model(&models.Message{}).
				Where("conversation_id = ? AND sender_type = ? AND created_at > ? AND content = ?", tickets[i].ConversationID, enums.IMSenderTypeCustomer, tickets[i].CreatedAt, currentTitle).
				Count(&postCreationCustomerMessageCount).Error; err != nil {
				return err
			}
			if generatedMessageCount > 0 || postCreationCustomerMessageCount > 0 {
				updates["title"] = customerTitle
			}
		}

		description := strings.TrimSpace(tickets[i].Description)
		if description != "" && !strings.Contains(description, customerTitle) {
			var generatedMessages []models.Message
			if err := db.Where(
				"conversation_id = ? AND sender_type <> ? AND created_at <= ? AND recalled_at IS NULL AND send_status NOT IN ?",
				tickets[i].ConversationID,
				enums.IMSenderTypeCustomer,
				tickets[i].CreatedAt,
				[]enums.IMMessageStatus{enums.IMMessageStatusFailed, enums.IMMessageStatusRecalled},
			).Order("id DESC").Find(&generatedMessages).Error; err != nil {
				return err
			}
			for j := range generatedMessages {
				generatedText := strings.TrimSpace(generatedMessages[j].Content)
				if generatedText == "" || !strings.Contains(description, generatedText) {
					continue
				}
				updates["description"] = strings.Replace(description, generatedText, customerTitle, 1)
				break
			}
		}
		if len(updates) == 0 {
			continue
		}

		updates["updated_at"] = time.Now()
		updates["update_user_name"] = "system"
		if err := db.Model(&models.Ticket{}).Where("id = ?", tickets[i].ID).Updates(updates).Error; err != nil {
			return err
		}
	}
	return nil
}

func limitMigrationText(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if maxLen <= 0 {
		return ""
	}
	if len(runes) <= maxLen {
		return value
	}
	return string(runes[:maxLen])
}
