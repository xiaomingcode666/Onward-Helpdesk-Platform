package services

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/i18nx"
)

func TestConversationSummaryTextsUsesOnlyMatchingTrustedLastMessage(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	now := time.Now()
	payload := `{"eventType":"ticket_created","ticketNo":"TK42"}`
	messages := []models.Message{
		{ConversationID: 11, SenderType: enums.IMSenderTypeSystem, MessageType: enums.IMMessageTypeText, Payload: payload, Content: "old rendered content"},
		{ConversationID: 12, SenderType: enums.IMSenderTypeCustomer, MessageType: enums.IMMessageTypeText, Payload: payload, Content: "客户原文"},
		{ConversationID: 13, SenderType: enums.IMSenderTypeSystem, MessageType: enums.IMMessageTypeText, Payload: payload, RecalledAt: &now},
		{ConversationID: 15, SenderType: enums.IMSenderTypeSystem, MessageType: enums.IMMessageTypeText, Payload: payload, SendStatus: enums.IMMessageStatusRecalled},
	}
	if err := db.Create(&messages).Error; err != nil {
		t.Fatal(err)
	}
	conversations := []models.Conversation{
		{ID: 11, LastMessageID: messages[0].ID},
		{ID: 12, LastMessageID: messages[1].ID},
		{ID: 13, LastMessageID: messages[2].ID},
		{ID: 14, LastMessageID: messages[0].ID},
		{ID: 15, LastMessageID: messages[3].ID},
	}
	texts := ConversationSummaryTexts(db, conversations)
	if len(texts) != 1 || texts[11].Render(i18nx.LocaleEnUS) != "Service ticket TK42 created" {
		t.Fatalf("unexpected summary descriptors: %#v", texts)
	}
	if texts[11].Render(i18nx.LocaleZhCN) != "已生成服务工单 TK42" {
		t.Fatal("language switch lost")
	}
}

func TestSystemEventWebsocketUsesEachConnectionLocale(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	conversation := models.Conversation{TenantID: 21, Status: enums.IMConversationStatusActive}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	message := models.Message{ConversationID: conversation.ID, SenderType: enums.IMSenderTypeSystem, MessageType: enums.IMMessageTypeText, Content: "historical snapshot", Payload: `{"eventType":"ticket_created","ticketNo":"TK42"}`}
	if err := db.Create(&message).Error; err != nil {
		t.Fatal(err)
	}
	conversation.LastMessageID = message.ID
	conversation.LastMessageSummary = "old summary"
	svc := newWsService()
	sessions := make([]*ClientSession, 0, 2)
	for _, locale := range []string{i18nx.LocaleEnUS, i18nx.LocaleZhCN} {
		session := &ClientSession{ID: locale, Role: realtimeRoleAdmin, Locale: locale, Principal: &dto.AuthPrincipal{TenantID: 21}, Topics: map[string]struct{}{}, Send: make(chan []byte, 8)}
		svc.manager.Register(session, []string{svc.conversationTopic(conversation.ID)})
		sessions = append(sessions, session)
	}
	svc.PublishMessageCreated(&conversation, &message)
	svc.PublishConversationChanged(&conversation, enums.IMRealtimeEventConversationUpdated)
	text := i18nx.ConversationSystemTextFromPayload(message.Payload)
	for _, session := range sessions {
		created := readWsMapEvent(t, session, enums.IMRealtimeEventMessageCreated)
		if created["content"] != text.Content.Render(session.Locale) {
			t.Fatalf("wrong %s content: %+v", session.Locale, created)
		}
		changed := readWsMapEvent(t, session, enums.IMRealtimeEventConversationUpdated)
		if changed["lastMessageSummary"] != text.Summary.Render(session.Locale) || changed["lastMessageSummaryText"] == nil {
			t.Fatalf("wrong %s summary: %+v", session.Locale, changed)
		}
	}
}

func TestPrepareConversationSystemMessageKeepsUserContent(t *testing.T) {
	payload := `{"eventType":"supplier_collaboration_invited","partnerCompanyName":"Onward"}`
	content, enriched, summary := prepareConversationSystemMessage(enums.IMSenderTypeSystem, enums.IMMessageTypeText, "", payload, "")
	if content == "" || summary == "" || !strings.Contains(enriched, `"systemText"`) {
		t.Fatalf("system event did not gain a descriptor: %q %q %q", content, enriched, summary)
	}
	for _, sender := range []enums.IMSenderType{enums.IMSenderTypeCustomer, enums.IMSenderTypeAI, enums.IMSenderTypeAgent} {
		got, gotPayload, gotSummary := prepareConversationSystemMessage(sender, enums.IMMessageTypeText, "客户原文", payload, "客户原文")
		if got != "客户原文" || gotPayload != payload || gotSummary != "客户原文" {
			t.Fatal("user content was localized")
		}
	}
}

func TestSystemEventPersistsTranslationDescriptorAndSummary(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	conversation := models.Conversation{TenantID: 21, Status: enums.IMConversationStatusActive}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	message, err := MessageService.SendSystemMessageWithRequestID(conversation.ID, "typed-ticket-created", "", `{"eventType":"ticket_created","ticketNo":"TK42"}`, "test-request")
	if err != nil {
		t.Fatal(err)
	}
	var saved models.Message
	if err := db.First(&saved, message.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(saved.Payload, `"systemText"`) || saved.Content == "" {
		t.Fatalf("missing persisted descriptor: %+v", saved)
	}
	if err := db.First(&conversation, conversation.ID).Error; err != nil {
		t.Fatal(err)
	}
	text := ConversationSummaryTexts(db, []models.Conversation{conversation})[conversation.ID]
	if text == nil || text.Render(i18nx.LocaleEnUS) != "Service ticket TK42 created" || conversation.LastMessageSummary != text.Render(i18nx.DefaultLocale) {
		t.Fatalf("summary and source event diverged: %+v", conversation)
	}
	retry, err := MessageService.SendSystemMessageWithRequestID(conversation.ID, "typed-ticket-created", "", `{"eventType":"ticket_created","ticketNo":"TK42"}`, "retry-request")
	if err != nil || retry.ID != message.ID {
		t.Fatalf("event idempotency lost: %+v %v", retry, err)
	}
}
