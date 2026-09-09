package event_handlers

import (
	"context"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestTicketCreatedConversationEventIsCustomerVisibleAndIdempotent(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Ticket{},
		&models.Conversation{},
		&models.ConversationReadState{},
		&models.ConversationEventLog{},
		&models.Message{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqls.SetDB(db)
	now := time.Now()
	conversation := &models.Conversation{
		TenantID: 1,
		Status:   enums.IMConversationStatusPending,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Create(conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	ticket := &models.Ticket{
		TenantID:       1,
		ConversationID: conversation.ID,
		TicketNo:       "TK-CUSTOMER-1001",
		Title:          "液压泵无法启动",
		Status:         enums.TicketStatusPending,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Create(ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	event := events.TicketCreatedEvent{EventID: "ticket.created:test", TicketID: ticket.ID}
	for attempt := 0; attempt < 2; attempt++ {
		if err := handleTicketCreatedConversationEvent(context.Background(), event); err != nil {
			t.Fatalf("handle ticket created event %d: %v", attempt, err)
		}
	}
	var messages []models.Message
	if err := db.Where("conversation_id = ? AND sender_type = ?", conversation.ID, enums.IMSenderTypeSystem).Find(&messages).Error; err != nil {
		t.Fatalf("find system messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("system message count=%d, want 1", len(messages))
	}
	if !strings.Contains(messages[0].Content, ticket.TicketNo) || !strings.Contains(messages[0].Payload, "ticket_created") {
		t.Fatalf("unexpected ticket event message: %+v", messages[0])
	}
}
