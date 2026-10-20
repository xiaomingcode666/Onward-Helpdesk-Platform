package services

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"testing"
	"time"
)

func TestWsAIStreamRoutesWithoutPersistingMessages(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	conversation := models.Conversation{TenantID: 7, Status: enums.IMConversationStatusActive}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	service := newWsService()
	customer := &ClientSession{ID: "stream-customer", Role: realtimeRoleUser, Topics: map[string]struct{}{}, Send: make(chan []byte, 8)}
	otherTenant := &ClientSession{ID: "other-tenant", Role: realtimeRoleAdmin, Topics: map[string]struct{}{}, Send: make(chan []byte, 8)}
	service.manager.Register(customer, []string{service.conversationTopic(conversation.ID)})
	service.manager.Register(otherTenant, []string{service.adminTenantTopic(8)})
	update := AIMessageStreamPayload{ConversationID: conversation.ID, SourceMessageID: 10, StreamID: "stream-1", ClientMsgID: "ai_reply_10", Sequence: 1, Content: "Hello", StartedAt: time.Now().UnixMilli(), ExpiresAt: time.Now().Add(time.Minute).UnixMilli()}
	service.PublishAIMessageStream(&conversation, update)
	payload := readWsMapEvent(t, customer, "message.streaming")
	if payload["content"] != "Hello" || payload["clientMsgId"] != "ai_reply_10" {
		t.Fatalf("unexpected stream: %#v", payload)
	}
	select {
	case data := <-otherTenant.Send:
		t.Fatalf("stream leaked across tenants: %s", data)
	default:
	}
	var count int64
	if err := db.Model(&models.Message{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("stream persisted %d partial messages", count)
	}
	update.Done = true
	update.Content = ""
	update.Sequence++
	service.PublishAIMessageStream(&conversation, update)
	if readWsMapEvent(t, customer, "message.streaming")["done"] != true {
		t.Fatal("terminal frame missing")
	}
}
