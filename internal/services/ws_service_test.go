package services

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
)

func TestWsNotificationTopic(t *testing.T) {
	svc := newWsService()
	if got := svc.notificationTopic(123); got != "notification:123" {
		t.Fatalf("expected notification:123, got %q", got)
	}
}

func TestWsNotificationCreatedEventType(t *testing.T) {
	event := RealtimeNotificationCreatedEvent{
		Payload: RealtimeNotificationCreatedPayload{
			Notification: response.NotificationResponse{ID: 1},
		},
	}
	if got := event.EventType(); got != "notification.created" {
		t.Fatalf("expected notification.created, got %q", got)
	}
	if payload := event.EventPayload(); payload == nil {
		t.Fatalf("expected payload")
	}
}

func TestWsRouteConversationTopicsUsesTenantAndTeamIsolation(t *testing.T) {
	svc := newWsService()
	topics := svc.routeConversationTopics(&models.Conversation{
		ID:                9,
		TenantID:          3,
		CurrentTeamID:     7,
		CurrentAssigneeID: 11,
	})
	want := map[string]bool{
		"conversation:9": true,
		"admin:tenant:3": true,
		"admin:team:7":   true,
		"admin:11":       true,
	}
	for _, topic := range topics {
		delete(want, topic)
		if topic == realtimeTopicAdminAll {
			t.Fatalf("tenant conversation must not publish to global admin topic: %+v", topics)
		}
	}
	if len(want) > 0 {
		t.Fatalf("missing isolated realtime topics: %+v", want)
	}
}

func TestWsTypingIsScopedAndDeliveredToConversationSubscribers(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	conversation := models.Conversation{
		TenantID:     1,
		CustomerName: "客户",
		Status:       enums.IMConversationStatusActive,
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	svc := newWsService()
	sender := &ClientSession{
		ID:   "sender",
		Role: realtimeRoleAdmin,
		Principal: &dto.AuthPrincipal{
			UserID: 25, TenantID: 1, DomainType: models.DomainTypeEnterprise,
			Roles: []string{EnterpriseRoleAdmin}, Nickname: "工程师",
		},
		Topics: map[string]struct{}{}, Send: make(chan []byte, 8),
	}
	receiver := &ClientSession{
		ID: "receiver", Role: realtimeRoleUser,
		Topics: map[string]struct{}{}, Send: make(chan []byte, 8),
	}
	topic := svc.conversationTopic(conversation.ID)
	svc.manager.Register(sender, []string{topic})
	svc.manager.Register(receiver, []string{topic})

	svc.publishTyping(sender, conversation.ID, true)
	select {
	case raw := <-receiver.Send:
		var event struct {
			Type string                       `json:"type"`
			Data RealtimeTypingChangedPayload `json:"data"`
		}
		if err := json.Unmarshal(raw, &event); err != nil {
			t.Fatalf("decode typing event: %v", err)
		}
		if event.Type != enums.IMRealtimeEventTypingChanged || !event.Data.Typing || event.Data.ParticipantType != string(enums.IMParticipantTypeAgent) {
			t.Fatalf("unexpected typing event: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("typing event was not delivered")
	}

	crossTenant := models.Conversation{TenantID: 2, Status: enums.IMConversationStatusActive}
	if err := db.Create(&crossTenant).Error; err != nil {
		t.Fatalf("create cross-tenant conversation: %v", err)
	}
	if svc.canSubscribeConversation(sender, crossTenant.ID) {
		t.Fatal("enterprise session must not subscribe to another tenant conversation")
	}
}

func TestWsCustomerFramesOmitInternalRoutingIDs(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	conversation := models.Conversation{
		TenantID: 1, CustomerName: "客户", Status: enums.IMConversationStatusActive,
		CurrentAssigneeID: 101, CurrentTeamID: 1,
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	svc := newWsService()
	topic := svc.conversationTopic(conversation.ID)
	admin := &ClientSession{
		ID: "admin", Role: realtimeRoleAdmin,
		Principal: &dto.AuthPrincipal{UserID: 101, TenantID: 1, DomainType: models.DomainTypeEnterprise},
		Topics:    map[string]struct{}{}, Send: make(chan []byte, 8),
	}
	customer := &ClientSession{
		ID: "customer", Role: realtimeRoleUser,
		Principal: &dto.AuthPrincipal{UserID: 31, TenantID: 1, DomainType: models.DomainTypeCustomer},
		Topics:    map[string]struct{}{}, Send: make(chan []byte, 8),
	}
	svc.manager.Register(admin, []string{topic})
	svc.manager.Register(customer, []string{topic})

	sentAt := time.Now()
	svc.PublishMessageCreated(&conversation, &models.Message{
		ID: 99, ConversationID: conversation.ID, RequestID: "request-internal",
		SenderType: enums.IMSenderTypeAgent, SenderID: 101,
		MessageType: enums.IMMessageTypeText, Content: "排查中",
		SendStatus: enums.IMMessageStatusSent, SentAt: &sentAt,
	})

	adminMessage := readWsMapEvent(t, admin, enums.IMRealtimeEventMessageCreated)
	if adminMessage["requestId"] != "request-internal" || adminMessage["senderId"] != float64(101) || adminMessage["currentAssigneeId"] != float64(101) {
		t.Fatalf("admin message event lost routing fields: %+v", adminMessage)
	}
	customerMessage := readWsMapEvent(t, customer, enums.IMRealtimeEventMessageCreated)
	assertMapOmitsKeys(t, customerMessage, "requestId", "senderId", "currentAssigneeId")
	if nested, ok := customerMessage["message"].(map[string]any); ok {
		assertMapOmitsKeys(t, nested, "requestId", "workflowRunId", "senderId", "agentRead", "agentReadAt")
	} else {
		t.Fatalf("customer message event missing nested message: %+v", customerMessage)
	}

	svc.PublishConversationChanged(&conversation, enums.IMRealtimeEventConversationUpdated)
	adminConversation := readWsMapEvent(t, admin, enums.IMRealtimeEventConversationUpdated)
	if adminConversation["currentAssigneeId"] != float64(101) || adminConversation["currentTeamId"] != float64(1) {
		t.Fatalf("admin conversation event lost assignment fields: %+v", adminConversation)
	}
	customerConversation := readWsMapEvent(t, customer, enums.IMRealtimeEventConversationUpdated)
	assertMapOmitsKeys(t, customerConversation, "currentAssigneeId", "currentTeamId", "agentUnreadCount", "agentLastReadMessageId", "agentLastReadAt")
}

func TestWsFormalCustomerSubscriptionUsesTenantAndParticipantOwnership(t *testing.T) {
	db := setupCustomerWorkbenchSecurityTestDB(t,
		&models.CustomerIdentity{},
		&models.Conversation{},
		&models.ConversationParticipant{},
	)
	for _, identity := range []models.CustomerIdentity{
		{CustomerID: 9, ExternalSource: enums.ExternalSourceUser, ExternalID: "31", Status: enums.StatusOk},
		{CustomerID: 10, ExternalSource: enums.ExternalSourceUser, ExternalID: "32", Status: enums.StatusOk},
	} {
		if err := db.Create(&identity).Error; err != nil {
			t.Fatalf("create customer identity: %v", err)
		}
	}
	conversation := models.Conversation{
		TenantID: 1, CustomerID: 9, CustomerName: "正式客户", Status: enums.IMConversationStatusActive,
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	if err := db.Create(&models.ConversationParticipant{
		ConversationID:        conversation.ID,
		ParticipantType:       string(enums.IMParticipantTypeCustomer),
		ExternalParticipantID: "user:31",
		Status:                enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create customer participant: %v", err)
	}

	newCustomerSession := func(userID, tenantID int64) *ClientSession {
		return &ClientSession{
			Role: realtimeRoleUser,
			Principal: &dto.AuthPrincipal{
				UserID: userID, TenantID: tenantID, DomainType: models.DomainTypeCustomer,
				Nickname: "正式客户 " + strconv.FormatInt(userID, 10),
			},
		}
	}
	svc := newWsService()
	if !svc.canSubscribeConversation(newCustomerSession(31, 1), conversation.ID) {
		t.Fatal("formal customer owner must subscribe to the conversation")
	}
	if svc.canSubscribeConversation(newCustomerSession(32, 1), conversation.ID) {
		t.Fatal("another customer account must not subscribe to the conversation")
	}
	if svc.canSubscribeConversation(newCustomerSession(31, 2), conversation.ID) {
		t.Fatal("formal customer must not subscribe across tenants")
	}
}

func readWsMapEvent(t *testing.T, session *ClientSession, eventType string) map[string]any {
	t.Helper()
	select {
	case raw := <-session.Send:
		var event struct {
			Type string         `json:"type"`
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal(raw, &event); err != nil {
			t.Fatalf("decode realtime event: %v", err)
		}
		if event.Type != eventType {
			t.Fatalf("event type = %q, want %q; raw=%s", event.Type, eventType, raw)
		}
		return event.Data
	case <-time.After(time.Second):
		t.Fatalf("expected realtime event %q", eventType)
	}
	return nil
}

func assertMapOmitsKeys(t *testing.T, data map[string]any, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if _, exists := data[key]; exists {
			t.Fatalf("customer realtime payload exposed %s: %+v", key, data)
		}
	}
}

func TestWsPresenceEventType(t *testing.T) {
	event := RealtimePresenceChangedEvent{Payload: RealtimePresenceChangedPayload{
		ConversationID: 1,
		ActorID:        "user:2",
		Online:         true,
	}}
	if event.EventType() != enums.IMRealtimeEventPresenceChanged {
		t.Fatalf("unexpected event type %q", event.EventType())
	}
}

func TestWsPresenceSnapshotReplacesIncrementalBootstrap(t *testing.T) {
	svc := newWsService()
	conversationID := int64(91)
	topic := svc.conversationTopic(conversationID)
	receiver := &ClientSession{
		ID: "receiver", Role: realtimeRoleAdmin,
		Principal: &dto.AuthPrincipal{UserID: 25, TenantID: 1, DomainType: models.DomainTypeEnterprise, Nickname: "工程师"},
		Topics:    map[string]struct{}{}, Send: make(chan []byte, 8),
	}
	partner := &ClientSession{
		ID: "partner", Role: realtimeRoleAdmin,
		Principal: &dto.AuthPrincipal{UserID: 31, TenantID: 1, DomainType: models.DomainTypePartner, Nickname: "供应商"},
		Topics:    map[string]struct{}{}, Send: make(chan []byte, 8),
	}
	svc.manager.Register(receiver, []string{topic})
	svc.manager.Register(partner, []string{topic})

	svc.enqueuePresenceSnapshot(receiver, conversationID)
	select {
	case raw := <-receiver.Send:
		var event struct {
			Type string                          `json:"type"`
			Data RealtimePresenceSnapshotPayload `json:"data"`
		}
		if err := json.Unmarshal(raw, &event); err != nil {
			t.Fatalf("decode presence snapshot: %v", err)
		}
		if event.Type != enums.IMRealtimeEventPresenceSnapshot || event.Data.ConversationID != conversationID {
			t.Fatalf("unexpected presence snapshot: %+v", event)
		}
		if len(event.Data.Participants) != 2 {
			t.Fatalf("presence participants = %+v, want 2", event.Data.Participants)
		}
	case <-time.After(time.Second):
		t.Fatal("presence snapshot was not delivered")
	}
}

func TestWsRepeatedConversationSubscribeRefreshesPresenceSnapshot(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	conversation := models.Conversation{
		TenantID:     1,
		CustomerName: "客户",
		Status:       enums.IMConversationStatusActive,
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	svc := newWsService()
	topic := svc.conversationTopic(conversation.ID)
	receiver := &ClientSession{
		ID: "receiver", Role: realtimeRoleAdmin,
		Principal: &dto.AuthPrincipal{
			UserID: 25, TenantID: 1, DomainType: models.DomainTypeEnterprise,
			Roles: []string{EnterpriseRoleAdmin}, Nickname: "工程师",
		},
		Topics: map[string]struct{}{}, Send: make(chan []byte, 8),
	}
	partner := &ClientSession{
		ID: "partner", Role: realtimeRoleAdmin,
		Principal: &dto.AuthPrincipal{
			UserID: 31, TenantID: 1, DomainType: models.DomainTypePartner, Nickname: "供应商",
		},
		Topics: map[string]struct{}{}, Send: make(chan []byte, 8),
	}
	svc.manager.Register(receiver, nil)
	svc.manager.Register(partner, []string{topic})

	if subscribed := svc.subscribeTopics(receiver, []string{topic}); len(subscribed) != 1 {
		t.Fatalf("initial subscribe = %v, want one topic", subscribed)
	}
	drainRealtimeEvents(receiver.Send)

	if subscribed := svc.subscribeTopics(receiver, []string{topic}); len(subscribed) != 0 {
		t.Fatalf("repeated subscribe = %v, want no newly added topic", subscribed)
	}
	select {
	case raw := <-receiver.Send:
		var event struct {
			Type string                          `json:"type"`
			Data RealtimePresenceSnapshotPayload `json:"data"`
		}
		if err := json.Unmarshal(raw, &event); err != nil {
			t.Fatalf("decode repeated presence snapshot: %v", err)
		}
		if event.Type != enums.IMRealtimeEventPresenceSnapshot || len(event.Data.Participants) != 2 {
			t.Fatalf("unexpected repeated presence snapshot: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("repeated conversation subscribe did not refresh presence snapshot")
	}
}

func TestWsPresenceStaysOnlineUntilActorLastSessionCloses(t *testing.T) {
	svc := newWsService()
	conversationID := int64(92)
	topic := svc.conversationTopic(conversationID)
	observer := &ClientSession{
		ID: "observer", Role: realtimeRoleAdmin,
		Principal: &dto.AuthPrincipal{UserID: 99, TenantID: 1, DomainType: models.DomainTypeEnterprise, Nickname: "观察者"},
		Topics:    map[string]struct{}{}, Send: make(chan []byte, 8),
	}
	newActorSession := func(id string) *ClientSession {
		return &ClientSession{
			ID: id, Role: realtimeRoleAdmin,
			Principal: &dto.AuthPrincipal{UserID: 25, TenantID: 1, DomainType: models.DomainTypeEnterprise, Nickname: "同一工程师"},
			Topics:    map[string]struct{}{}, Send: make(chan []byte, 8),
		}
	}
	tabOne := newActorSession("actor-tab-1")
	tabTwo := newActorSession("actor-tab-2")
	svc.manager.Register(observer, []string{topic})
	svc.manager.Register(tabOne, []string{topic})
	svc.manager.Register(tabTwo, []string{topic})

	svc.closeSession(tabOne)
	select {
	case raw := <-observer.Send:
		t.Fatalf("closing one of two actor sessions published an offline event: %s", raw)
	default:
	}

	svc.closeSession(tabTwo)
	select {
	case raw := <-observer.Send:
		var event struct {
			Type string                         `json:"type"`
			Data RealtimePresenceChangedPayload `json:"data"`
		}
		if err := json.Unmarshal(raw, &event); err != nil {
			t.Fatalf("decode final offline event: %v", err)
		}
		if event.Type != enums.IMRealtimeEventPresenceChanged || event.Data.ActorID != "user:25" || event.Data.Online {
			t.Fatalf("unexpected final offline event: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("closing the actor's final session did not publish offline presence")
	}
}

func drainRealtimeEvents(events <-chan []byte) {
	for {
		select {
		case <-events:
		default:
			return
		}
	}
}
