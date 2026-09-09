package migration

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(20, "backfill ticket customer entry sessions from conversations", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			return backfillTicketEntrySessions(ctx.Tx)
		})
	})
}

func backfillTicketEntrySessions(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.Ticket{}) || !db.Migrator().HasTable(&models.Conversation{}) {
		return nil
	}
	var tickets []models.Ticket
	if err := db.Where("customer_entry_session_id = 0 AND conversation_id > 0").Find(&tickets).Error; err != nil {
		return err
	}
	for i := range tickets {
		ticket := &tickets[i]
		conversation := repositories.ConversationRepository.Get(db, ticket.ConversationID)
		if conversation == nil || conversation.TenantID != ticket.TenantID || conversation.CustomerEntrySessionID <= 0 {
			continue
		}
		if err := db.Model(&models.Ticket{}).Where("id = ?", ticket.ID).
			Update("customer_entry_session_id", conversation.CustomerEntrySessionID).Error; err != nil {
			return err
		}
	}
	return nil
}
