package services

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/openidentity"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func setupCustomerWorkbenchSecurityTestDB(t *testing.T, modelsToMigrate ...any) *gorm.DB {
	t.Helper()
	dbName := "customer_workbench_security_" + strings.NewReplacer("/", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(modelsToMigrate...); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	sqls.SetDB(db)
	return db
}

func TestCustomerConversationOwnerUsesActiveParticipant(t *testing.T) {
	db := setupCustomerWorkbenchSecurityTestDB(t,
		&models.CustomerIdentity{},
		&models.Conversation{},
		&models.ConversationParticipant{},
	)
	for _, identity := range []models.CustomerIdentity{
		{CustomerID: 9, ExternalSource: enums.ExternalSourceUser, ExternalID: "user-a", Status: enums.StatusOk},
		{CustomerID: 9, ExternalSource: enums.ExternalSourceUser, ExternalID: "user-b", Status: enums.StatusOk},
	} {
		if err := db.Create(&identity).Error; err != nil {
			t.Fatalf("create identity: %v", err)
		}
	}
	conversation := models.Conversation{CustomerID: 9, TenantID: 3, Status: enums.IMConversationStatusActive}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	if err := db.Create(&models.ConversationParticipant{
		ConversationID:        conversation.ID,
		ParticipantType:       string(enums.IMParticipantTypeCustomer),
		ExternalParticipantID: "user:user-a",
		Status:                enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create participant: %v", err)
	}

	userA := openidentity.ExternalUser{ExternalSource: enums.ExternalSourceUser, ExternalID: "user-a"}
	userB := openidentity.ExternalUser{ExternalSource: enums.ExternalSourceUser, ExternalID: "user-b"}
	if !ConversationService.IsCustomerConversationOwner(&conversation, userA) {
		t.Fatal("expected active participant to access conversation")
	}
	if ConversationService.IsCustomerConversationOwner(&conversation, userB) {
		t.Fatal("expected another identity under the same customer to be denied")
	}
}

func TestCustomerConversationMatchDoesNotCrossParticipants(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	now := time.Now()
	customer := models.Customer{Name: "Shared customer", Status: enums.StatusOk}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	for _, externalID := range []string{"user-a", "user-b"} {
		if err := db.Create(&models.CustomerIdentity{
			CustomerID: customer.ID, ExternalSource: enums.ExternalSourceUser, ExternalID: externalID,
			Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
		}).Error; err != nil {
			t.Fatalf("create customer identity: %v", err)
		}
	}
	aiAgent := createWelcomeTestAIAgent(t, db, "Welcome")
	userA := openidentity.ExternalUser{ExternalSource: enums.ExternalSourceUser, ExternalID: "user-a"}
	userB := openidentity.ExternalUser{ExternalSource: enums.ExternalSourceUser, ExternalID: "user-b"}
	conversationA, err := ConversationService.Create(userA, 11, aiAgent.ID)
	if err != nil {
		t.Fatalf("create conversation A: %v", err)
	}
	conversationB, err := ConversationService.Create(userB, 11, aiAgent.ID)
	if err != nil {
		t.Fatalf("create conversation B: %v", err)
	}
	if conversationA.ID == conversationB.ID {
		t.Fatalf("different customer participants matched the same conversation %d", conversationA.ID)
	}
}

func TestCustomerVisibleMessagesExcludeAgentOnlyMessages(t *testing.T) {
	db := setupCustomerWorkbenchSecurityTestDB(t, &models.Message{})
	now := time.Now()
	for _, receiverType := range []string{"", "customer", "agent"} {
		message := models.Message{
			ConversationID: 5,
			ClientMsgID:    "message-" + receiverType,
			SenderType:     enums.IMSenderTypeAgent,
			ReceiverType:   receiverType,
			MessageType:    enums.IMMessageTypeText,
			Content:        receiverType,
			SendStatus:     enums.IMMessageStatusSent,
			SentAt:         &now,
		}
		if err := db.Create(&message).Error; err != nil {
			t.Fatalf("create message: %v", err)
		}
	}

	items, _, _ := MessageService.FindCustomerVisibleByConversationIDCursor(5, 0, 20, "", "")
	if len(items) != 2 {
		t.Fatalf("expected 2 customer-visible messages, got %d", len(items))
	}
	for _, item := range items {
		if item.ReceiverType == "agent" {
			t.Fatal("agent-only message leaked into customer result")
		}
	}
}

func TestCustomerPortalDeviceAccessDistinguishesUnregisteredAndBoundCustomers(t *testing.T) {
	db := setupCustomerWorkbenchSecurityTestDB(t,
		&models.Customer{},
		&models.CustomerIdentity{},
		&models.CustomerDeviceBinding{},
		&models.Device{},
	)
	device := models.Device{TenantID: 21, DeviceNo: "DEV-ACCESS", Status: enums.StatusOk}
	if err := db.Create(&device).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}

	unregistered := openidentity.ExternalUser{ExternalSource: enums.ExternalSourceUser, ExternalID: "701"}
	accessible, err := CustomerPortalService.HasDeviceAccess(unregistered, device.ID)
	if err != nil || accessible {
		t.Fatalf("unregistered customer access = %v, err = %v; want false, nil", accessible, err)
	}

	customer := models.Customer{Name: "Bound customer", Status: enums.StatusOk}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	if err := db.Create(&models.CustomerIdentity{
		CustomerID: customer.ID, ExternalSource: enums.ExternalSourceUser, ExternalID: "702", Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create identity: %v", err)
	}
	if err := db.Create(&models.CustomerDeviceBinding{
		TenantID: 21, DeviceID: device.ID, CustomerUserID: 702, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create binding: %v", err)
	}

	bound := openidentity.ExternalUser{ExternalSource: enums.ExternalSourceUser, ExternalID: "702"}
	accessible, err = CustomerPortalService.HasDeviceAccess(bound, device.ID)
	if err != nil || !accessible {
		t.Fatalf("bound customer access = %v, err = %v; want true, nil", accessible, err)
	}
}

func TestCustomerPortalConversationExposesAssignedEngineer(t *testing.T) {
	db := setupCustomerWorkbenchSecurityTestDB(t,
		&models.Tenant{},
		&models.Customer{},
		&models.CustomerIdentity{},
		&models.CustomerDeviceBinding{},
		&models.Device{},
		&models.User{},
		&models.Conversation{},
		&models.ConversationParticipant{},
		&models.Ticket{},
		&models.MeetingRoomJitsi{},
	)
	customer := models.Customer{Name: "Assigned customer", Status: enums.StatusOk}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	external := openidentity.ExternalUser{ExternalSource: enums.ExternalSourceUser, ExternalID: "301"}
	if err := db.Create(&models.CustomerIdentity{
		CustomerID: customer.ID, ExternalSource: external.ExternalSource, ExternalID: external.ExternalID, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create customer identity: %v", err)
	}
	device := models.Device{TenantID: 8, DeviceNo: "DEV-ASSIGNED", Status: enums.StatusOk}
	if err := db.Create(&device).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	if err := db.Create(&models.CustomerDeviceBinding{
		TenantID: 8, DeviceID: device.ID, CustomerUserID: 301, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create binding: %v", err)
	}
	engineer := models.User{Username: "assigned.engineer", Nickname: "已分配工程师", Status: enums.StatusOk}
	if err := db.Create(&engineer).Error; err != nil {
		t.Fatalf("create engineer: %v", err)
	}
	conversation := models.Conversation{
		TenantID: 8, CustomerID: customer.ID, DeviceID: device.ID,
		Status: enums.IMConversationStatusPending, CurrentAssigneeID: engineer.ID,
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	if err := db.Create(&models.ConversationParticipant{
		ConversationID: conversation.ID, ParticipantType: string(enums.IMParticipantTypeCustomer),
		ExternalParticipantID: "user:301", Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create participant: %v", err)
	}
	if err := db.Create(&models.Ticket{
		TenantID: 8, CustomerID: customer.ID, ConversationID: conversation.ID,
		DeviceID: device.ID, TicketNo: "T-ASSIGNED", Title: "Assigned issue",
		Status: enums.TicketStatusAssigned, CurrentAssigneeID: engineer.ID,
	}).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	items, err := CustomerPortalService.ListConversations(external)
	if err != nil {
		t.Fatalf("ListConversations() error = %v", err)
	}
	if len(items) != 1 || items[0].CurrentAssigneeID != engineer.ID || items[0].CurrentAssigneeName != engineer.Nickname {
		t.Fatalf("assigned engineer missing from customer conversation: %+v", items)
	}
	raw, err := json.Marshal(items[0])
	if err != nil {
		t.Fatalf("marshal customer portal conversation: %v", err)
	}
	for _, forbidden := range []string{"current_assignee_id"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("customer portal conversation exposed %s: %s", forbidden, raw)
		}
	}
	if items[0].CurrentTicketID == 0 || !strings.Contains(string(raw), "current_ticket_id") {
		t.Fatalf("customer portal conversation omitted navigable ticket id: %s", raw)
	}
}

func TestCustomerPortalConversationVisibleWithoutDeviceBinding(t *testing.T) {
	db := setupCustomerWorkbenchSecurityTestDB(t,
		&models.Tenant{},
		&models.Customer{},
		&models.CustomerIdentity{},
		&models.CustomerDeviceBinding{},
		&models.Device{},
		&models.Product{},
		&models.User{},
		&models.AIAgent{},
		&models.Conversation{},
		&models.ConversationParticipant{},
		&models.Ticket{},
		&models.MeetingRoomJitsi{},
	)
	customer := models.Customer{Name: "General consultation customer", Status: enums.StatusOk}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	external := openidentity.ExternalUser{ExternalSource: enums.ExternalSourceUser, ExternalID: "general-user"}
	if err := db.Create(&models.CustomerIdentity{
		CustomerID: customer.ID, ExternalSource: external.ExternalSource, ExternalID: external.ExternalID, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create customer identity: %v", err)
	}
	product := models.Product{TenantID: 12, Name: "General product", Status: enums.StatusOk}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	agent := models.AIAgent{
		TenantID: 12, ProductID: product.ID, Name: "Product support", ServiceMode: enums.IMConversationServiceModeAIFirst,
		Status: enums.StatusOk,
	}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatalf("create product agent: %v", err)
	}
	conversation := models.Conversation{
		TenantID: 12, ProductID: product.ID, AIAgentID: agent.ID, CustomerID: customer.ID,
		Status: enums.IMConversationStatusAIServing, LastActiveAt: time.Now(),
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	if err := db.Create(&models.ConversationParticipant{
		ConversationID: conversation.ID, ParticipantType: string(enums.IMParticipantTypeCustomer),
		ExternalParticipantID: "user:general-user", Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create participant: %v", err)
	}

	items, err := CustomerPortalService.ListConversations(external)
	if err != nil {
		t.Fatalf("ListConversations() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != conversation.ID {
		t.Fatalf("no-device conversation missing from customer portal: %+v", items)
	}
	if items[0].DeviceID != 0 || items[0].ProductName != product.Name {
		t.Fatalf("no-device conversation context mismatch: %+v", items[0])
	}
	if !items[0].HumanHandoffEnabled {
		t.Fatalf("product conversation should expose human handoff capability: %+v", items[0])
	}
}

func TestCustomerPortalGeneralConversationVisibleWithoutProductOrDevice(t *testing.T) {
	db := setupCustomerWorkbenchSecurityTestDB(t,
		&models.Tenant{},
		&models.Customer{},
		&models.CustomerIdentity{},
		&models.CustomerDeviceBinding{},
		&models.Device{},
		&models.Product{},
		&models.User{},
		&models.AIAgent{},
		&models.Conversation{},
		&models.ConversationParticipant{},
		&models.Ticket{},
		&models.MeetingRoomJitsi{},
	)
	customer := models.Customer{Name: "General-only customer", Status: enums.StatusOk}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	external := openidentity.ExternalUser{ExternalSource: enums.ExternalSourceUser, ExternalID: "general-only-user"}
	if err := db.Create(&models.CustomerIdentity{
		CustomerID: customer.ID, ExternalSource: external.ExternalSource, ExternalID: external.ExternalID, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create customer identity: %v", err)
	}
	conversation := models.Conversation{
		TenantID: 12, CustomerID: customer.ID, ProductID: 0, DeviceID: 0,
		Status: enums.IMConversationStatusAIServing, LastMessageSummary: "通用咨询已创建", LastActiveAt: time.Now(),
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create general conversation: %v", err)
	}
	if err := db.Create(&models.ConversationParticipant{
		ConversationID: conversation.ID, ParticipantType: string(enums.IMParticipantTypeCustomer),
		ExternalParticipantID: "user:general-only-user", Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create participant: %v", err)
	}

	items, err := CustomerPortalService.ListConversations(external)
	if err != nil {
		t.Fatalf("ListConversations() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != conversation.ID {
		t.Fatalf("general conversation missing from customer portal: %+v", items)
	}
	if items[0].DeviceID != 0 || items[0].DeviceNo != "" || items[0].ProductName != "" {
		t.Fatalf("general conversation should stay unbound to product/device: %+v", items[0])
	}
}

func TestCustomerPortalGeneralConversationDisablesHumanHandoffEvenWhenAgentMisconfigured(t *testing.T) {
	db := setupCustomerWorkbenchSecurityTestDB(t,
		&models.Tenant{},
		&models.Customer{},
		&models.CustomerIdentity{},
		&models.AIAgent{},
		&models.Conversation{},
		&models.ConversationParticipant{},
		&models.Ticket{},
		&models.MeetingRoomJitsi{},
	)
	customer := models.Customer{Name: "General AI customer", Status: enums.StatusOk}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	external := openidentity.ExternalUser{ExternalSource: enums.ExternalSourceUser, ExternalID: "general-ai-user"}
	if err := db.Create(&models.CustomerIdentity{
		CustomerID: customer.ID, ExternalSource: external.ExternalSource, ExternalID: external.ExternalID, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create customer identity: %v", err)
	}
	agent := models.AIAgent{
		TenantID: 12, ProductID: 0, Source: TenantDefaultAIAgentSource, Name: "通用咨询",
		ServiceMode: enums.IMConversationServiceModeAIFirst, Status: enums.StatusOk,
	}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatalf("create tenant default agent: %v", err)
	}
	if !AIWorkflowService.AgentAllowsHumanHandoff(&agent) {
		t.Fatal("test setup expects the misconfigured default agent to appear handoff-capable")
	}
	conversation := models.Conversation{
		TenantID: 12, AIAgentID: agent.ID, CustomerID: customer.ID,
		Status: enums.IMConversationStatusAIServing, LastActiveAt: time.Now(),
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	if err := db.Create(&models.ConversationParticipant{
		ConversationID: conversation.ID, ParticipantType: string(enums.IMParticipantTypeCustomer),
		ExternalParticipantID: "user:general-ai-user", Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create participant: %v", err)
	}

	items, err := CustomerPortalService.ListConversations(external)
	if err != nil {
		t.Fatalf("ListConversations() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != conversation.ID {
		t.Fatalf("general conversation missing from customer portal: %+v", items)
	}
	if items[0].HumanHandoffEnabled || items[0].TicketCreationEnabled {
		t.Fatalf("general conversation must not expose handoff or ticket creation: %+v", items[0])
	}
}

func TestCustomerMeetingVisibilityRequiresBoundTicketContext(t *testing.T) {
	db := setupCustomerWorkbenchSecurityTestDB(t,
		&models.Customer{},
		&models.CustomerIdentity{},
		&models.CustomerDeviceBinding{},
		&models.Device{},
		&models.Ticket{},
		&models.MeetingRoomJitsi{},
	)
	customer := models.Customer{Name: "Meeting customer", Status: enums.StatusOk}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	external := openidentity.ExternalUser{ExternalSource: enums.ExternalSourceUser, ExternalID: "501"}
	if err := db.Create(&models.CustomerIdentity{
		CustomerID: customer.ID, ExternalSource: external.ExternalSource, ExternalID: external.ExternalID, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create customer identity: %v", err)
	}
	boundDevice := models.Device{TenantID: 11, DeviceNo: "DEV-BOUND", Status: enums.StatusOk}
	unboundDevice := models.Device{TenantID: 11, DeviceNo: "DEV-UNBOUND", Status: enums.StatusOk}
	if err := db.Create(&boundDevice).Error; err != nil {
		t.Fatalf("create bound device: %v", err)
	}
	if err := db.Create(&unboundDevice).Error; err != nil {
		t.Fatalf("create unbound device: %v", err)
	}
	if err := db.Create(&models.CustomerDeviceBinding{
		TenantID: 11, DeviceID: boundDevice.ID, CustomerUserID: 501, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create customer device binding: %v", err)
	}

	createMeeting := func(ticketNo, meetingID string, deviceID int64) models.MeetingRoomJitsi {
		ticket := models.Ticket{
			TenantID: 11, CustomerID: customer.ID, DeviceID: deviceID,
			TicketNo: ticketNo, Title: ticketNo, Status: enums.TicketStatusProcessing,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create ticket %s: %v", ticketNo, err)
		}
		meeting := models.MeetingRoomJitsi{
			ID: meetingID, TenantID: 11, TicketID: strconv.FormatInt(ticket.ID, 10),
			RoomName: meetingID, Status: "active", CreatedBy: "1",
			BaseModel: models.BaseModel{CreatedAt: time.Now(), UpdatedAt: time.Now()},
		}
		if err := db.Create(&meeting).Error; err != nil {
			t.Fatalf("create meeting %s: %v", meetingID, err)
		}
		return meeting
	}
	visibleMeeting := createMeeting("T-MEETING-VISIBLE", "meeting-visible", boundDevice.ID)
	hiddenMeeting := createMeeting("T-MEETING-HIDDEN", "meeting-hidden", unboundDevice.ID)

	if _, meeting, err := CustomerPortalService.resolveVisibleMeeting(external, visibleMeeting.ID); err != nil || meeting.ID != visibleMeeting.ID {
		t.Fatalf("visible meeting lookup = %#v, %v", meeting, err)
	}
	if _, _, err := CustomerPortalService.resolveVisibleMeeting(external, hiddenMeeting.ID); err == nil {
		t.Fatal("meeting for an unbound device was visible")
	}
}

func TestCustomerPortalTicketsExposeOnlyPublicProgressAndRepair(t *testing.T) {
	db := setupCustomerWorkbenchSecurityTestDB(t,
		&models.Customer{},
		&models.CustomerIdentity{},
		&models.CustomerDeviceBinding{},
		&models.Device{},
		&models.Conversation{},
		&models.ConversationParticipant{},
		&models.Ticket{},
		&models.TicketProgress{},
		&models.TicketRepairRecord{},
		&models.MeetingRoomJitsi{},
	)
	now := time.Now()
	customer := models.Customer{Name: "Customer A", Status: enums.StatusOk}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	external := openidentity.ExternalUser{ExternalSource: enums.ExternalSourceUser, ExternalID: "101"}
	if err := db.Create(&models.CustomerIdentity{
		CustomerID: customer.ID, ExternalSource: external.ExternalSource, ExternalID: external.ExternalID, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create customer identity: %v", err)
	}
	device := models.Device{TenantID: 7, DeviceNo: "DEV-7", Status: enums.StatusOk}
	if err := db.Create(&device).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	if err := db.Create(&models.CustomerDeviceBinding{
		TenantID: 7, DeviceID: device.ID, CustomerUserID: 101, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create binding: %v", err)
	}
	conversation := models.Conversation{TenantID: 7, CustomerID: customer.ID, DeviceID: device.ID, Status: enums.IMConversationStatusActive}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	if err := db.Create(&models.ConversationParticipant{
		ConversationID: conversation.ID, ParticipantType: string(enums.IMParticipantTypeCustomer), ExternalParticipantID: "user:101", Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create participant: %v", err)
	}
	ticket := models.Ticket{
		TenantID: 7, CustomerID: customer.ID, ConversationID: conversation.ID, DeviceID: device.ID,
		TicketNo: "T-7", Title: "Device issue", Status: enums.TicketStatusProcessing,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	for _, progress := range []models.TicketProgress{
		{TenantID: 7, TicketID: ticket.ID, Content: "public progress", VisibleToCustomer: true, CreatedAt: now},
		{TenantID: 7, TicketID: ticket.ID, Content: "internal note", VisibleToCustomer: false, CreatedAt: now.Add(time.Minute)},
	} {
		if err := db.Create(&progress).Error; err != nil {
			t.Fatalf("create progress: %v", err)
		}
	}
	for _, repair := range []models.TicketRepairRecord{
		{TenantID: 7, TicketID: ticket.ID, Solution: "public solution", VisibleToCustomer: true, FinishedAt: &now},
		{TenantID: 7, TicketID: ticket.ID, Solution: "internal solution", VisibleToCustomer: false, FinishedAt: ptrTime(now.Add(time.Minute))},
	} {
		if err := db.Create(&repair).Error; err != nil {
			t.Fatalf("create repair: %v", err)
		}
	}
	if err := db.Model(&models.TicketRepairRecord{}).
		Where("ticket_id = ? AND solution = ?", ticket.ID, "internal solution").
		Update("visible_to_customer", false).Error; err != nil {
		t.Fatalf("mark repair internal: %v", err)
	}

	detail, err := CustomerPortalService.GetTicketDetail(external, ticket.ID)
	if err != nil {
		t.Fatalf("GetTicketDetail() error = %v", err)
	}
	if detail == nil {
		t.Fatal("expected visible ticket detail")
	}
	if len(detail.Progress) != 1 || detail.Progress[0].Content != "public progress" {
		t.Fatalf("unexpected customer progress: %+v", detail.Progress)
	}
	if detail.RepairSummary != "public solution" {
		t.Fatalf("unexpected repair summary: %q", detail.RepairSummary)
	}
}
