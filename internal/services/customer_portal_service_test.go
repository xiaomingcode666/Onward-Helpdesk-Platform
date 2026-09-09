package services

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/ai"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/openidentity"
	"remotehelpdesk/internal/repositories"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestFirstUpcomingCustomerMeetingSkipsFinishedMeetings(t *testing.T) {
	items := []dto.CustomerPortalMeetingDTO{
		{ID: "finished-latest", Status: "finished"},
		{ID: "active-older", Status: "active"},
		{ID: "waiting-oldest", Status: "waiting"},
	}

	meeting := firstUpcomingCustomerMeeting(items)
	if meeting == nil || meeting.ID != "active-older" {
		t.Fatalf("upcoming meeting = %#v, want active-older", meeting)
	}
}

func TestMapCustomerTicketStatusDoesNotClaimAcceptanceBeforeEngineerAccepts(t *testing.T) {
	tests := []struct {
		status enums.TicketStatus
		want   string
	}{
		{status: enums.TicketStatusPendingDispatch, want: "pending_dispatch"},
		{status: enums.TicketStatusPendingAssigneeAccept, want: "pending_assignee_accept"},
		{status: enums.TicketStatusAccepted, want: "accepted"},
	}
	for _, test := range tests {
		if got := mapCustomerTicketStatus(test.status); got != test.want {
			t.Fatalf("mapCustomerTicketStatus(%q) = %q, want %q", test.status, got, test.want)
		}
	}
}

func TestBuildCustomerTicketProgressFallbackExplainsDispatchStates(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 13, 0, 0, time.Local)
	pendingDispatch := models.Ticket{
		Status:      enums.TicketStatusPendingDispatch,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	progress := buildCustomerTicketProgressFallback(pendingDispatch, "", nil)
	if len(progress) != 1 || progress[0].EventType != string(enums.TicketProgressEventCreated) ||
		!strings.Contains(progress[0].Content, "正在安排工程师") {
		t.Fatalf("pending dispatch fallback progress = %+v", progress)
	}

	pendingAccept := models.Ticket{
		Status:      enums.TicketStatusPendingAssigneeAccept,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	progress = buildCustomerTicketProgressFallback(pendingAccept, "产品测试1值班工程师", nil)
	if len(progress) != 1 || progress[0].EventType != string(enums.TicketProgressEventAssigned) ||
		!strings.Contains(progress[0].Content, "产品测试1值班工程师") ||
		!strings.Contains(progress[0].Content, "等待工程师接单") {
		t.Fatalf("pending accept fallback progress = %+v", progress)
	}
}

func TestFirstUpcomingCustomerMeetingReturnsNilWhenAllMeetingsFinished(t *testing.T) {
	items := []dto.CustomerPortalMeetingDTO{
		{ID: "finished-1", Status: "finished"},
		{ID: "finished-2", Status: "finished"},
	}

	if meeting := firstUpcomingCustomerMeeting(items); meeting != nil {
		t.Fatalf("upcoming meeting = %#v, want nil", meeting)
	}
}

func TestFirstActiveCustomerConversationSkipsClosedConversations(t *testing.T) {
	items := []dto.CustomerPortalConversationDTO{
		{ID: 11, Status: "closed"},
		{ID: 10, Status: "human_serving"},
	}

	conversation := firstActiveCustomerConversation(items)
	if conversation == nil || conversation.ID != 10 {
		t.Fatalf("active conversation = %#v, want 10", conversation)
	}
}

func TestFirstPendingPortalTicketsExcludesTerminalTickets(t *testing.T) {
	items := []dto.CustomerPortalTicketDTO{
		{ID: 15, Status: "closed"},
		{ID: 14, Status: "cancelled"},
		{ID: 13, Status: "action_required"},
		{ID: 12, Status: "processing"},
		{ID: 11, Status: "accepted"},
	}

	tickets := firstPendingPortalTickets(items, 2)
	if len(tickets) != 2 || tickets[0].ID != 13 || tickets[1].ID != 12 {
		t.Fatalf("pending tickets = %#v, want 13 and 12", tickets)
	}
}

func TestCustomerPortalTranslateConversationMessageUsesVisibleCustomerScope(t *testing.T) {
	db := setupCustomerPortalProfileTestDB(t)
	now := time.Now()
	user := models.User{
		Username: "customer.translation",
		Nickname: "Translation Customer",
		Status:   enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	customer := models.Customer{
		Name: "Translation Customer", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	if err := db.Create(&models.CustomerIdentity{
		CustomerID: customer.ID, ExternalSource: enums.ExternalSourceUser, ExternalID: strconv.FormatInt(user.ID, 10),
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create customer identity: %v", err)
	}
	customerUser := models.CustomerUser{
		TenantID: 710, CustomerOrgID: 810, UserID: user.ID, DisplayName: "Translation Customer",
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&customerUser).Error; err != nil {
		t.Fatalf("create customer user: %v", err)
	}
	conversation := models.Conversation{
		TenantID: 710, CustomerID: customer.ID, CustomerName: "Translation Customer",
		Status: enums.IMConversationStatusActive, LastActiveAt: now,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	message := models.Message{
		ConversationID: conversation.ID,
		ClientMsgID:    "customer-portal-translation-message",
		SenderType:     enums.IMSenderTypeAgent,
		MessageType:    enums.IMMessageTypeText,
		Content:        "The pump alarm is active.",
		SendStatus:     enums.IMMessageStatusSent,
		SentAt:         &now,
		AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&message).Error; err != nil {
		t.Fatalf("create message: %v", err)
	}

	originalChat := conversationTranslationChat
	conversationTranslationChat = func(_ context.Context, systemPrompt, userPrompt string) (*ai.ChatCompletionResult, error) {
		if !strings.Contains(systemPrompt, "Simplified Chinese (zh-CN)") {
			t.Fatalf("translation prompt target = %q", systemPrompt)
		}
		if userPrompt != `"The pump alarm is active."` {
			t.Fatalf("translation source = %q", userPrompt)
		}
		return &ai.ChatCompletionResult{
			Content:          "泵报警已触发。",
			ModelName:        "customer-portal-translation-test",
			PromptTokens:     9,
			CompletionTokens: 4,
		}, nil
	}
	t.Cleanup(func() {
		conversationTranslationChat = originalChat
	})

	result, err := CustomerPortalService.TranslateConversationMessage(context.Background(), openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceUser,
		ExternalID:     strconv.FormatInt(user.ID, 10),
	}, conversation.ID, request.TranslateConversationMessageRequest{
		MessageID:      message.ID,
		TargetLanguage: "zh-CN",
	})
	if err != nil {
		t.Fatalf("TranslateConversationMessage() error = %v", err)
	}
	if result == nil || result.Translation == nil {
		t.Fatal("TranslateConversationMessage() returned nil translation")
	}
	if result.Translation.TenantID != 710 || result.Translation.ConversationID != conversation.ID || result.Translation.MessageID != message.ID {
		t.Fatalf("translation scope = %+v", result.Translation)
	}
	if result.Translation.TranslatedText != "泵报警已触发。" || result.Translation.CreateUserID != user.ID || result.Translation.CreateUserName != customer.Name {
		t.Fatalf("translation result = %+v", result.Translation)
	}

	otherUser := models.User{
		Username: "customer.translation.other",
		Nickname: "Other Customer",
		Status:   enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Create(&otherUser).Error; err != nil {
		t.Fatalf("create other user: %v", err)
	}
	otherCustomer := models.Customer{
		Name: "Other Customer", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&otherCustomer).Error; err != nil {
		t.Fatalf("create other customer: %v", err)
	}
	if err := db.Create(&models.CustomerIdentity{
		CustomerID: otherCustomer.ID, ExternalSource: enums.ExternalSourceUser, ExternalID: strconv.FormatInt(otherUser.ID, 10),
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create other customer identity: %v", err)
	}
	if err := db.Create(&models.CustomerUser{
		TenantID: 710, CustomerOrgID: 811, UserID: otherUser.ID, DisplayName: "Other Customer",
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create other customer user: %v", err)
	}
	if _, err := CustomerPortalService.TranslateConversationMessage(context.Background(), openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceUser,
		ExternalID:     strconv.FormatInt(otherUser.ID, 10),
	}, conversation.ID, request.TranslateConversationMessageRequest{
		MessageID:      message.ID,
		TargetLanguage: "zh-CN",
	}); err == nil {
		t.Fatal("other customer unexpectedly translated an invisible conversation")
	}
}

func TestCustomerPortalUpdateProfileSyncsAccountAndConversation(t *testing.T) {
	db := setupCustomerPortalProfileTestDB(t)
	now := time.Now()
	user := models.User{
		Username: "customer.profile",
		Nickname: "Old Name",
		Status:   enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	customer := models.Customer{
		Name: "Old Name", PrimaryEmail: "old@example.com", PrimaryMobile: "+1 111",
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	identity := models.CustomerIdentity{
		CustomerID: customer.ID, ExternalSource: enums.ExternalSourceUser, ExternalID: strconv.FormatInt(user.ID, 10),
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&identity).Error; err != nil {
		t.Fatalf("create identity: %v", err)
	}
	customerUser := models.CustomerUser{
		TenantID: 7, CustomerOrgID: 8, UserID: user.ID, DisplayName: "Old Name", Email: "old@example.com", Phone: "+1 111",
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&customerUser).Error; err != nil {
		t.Fatalf("create customer user: %v", err)
	}
	conversation := models.Conversation{
		TenantID: 7, CustomerID: customer.ID, CustomerName: "Old Name",
		Status: enums.IMConversationStatusActive, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	updated, err := CustomerPortalService.UpdateProfile(openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceUser,
		ExternalID:     strconv.FormatInt(user.ID, 10),
	}, request.UpdateCustomerPortalProfileRequest{
		Name:          "New Customer",
		PrimaryEmail:  "new@example.com",
		PrimaryMobile: "+1 222",
	})
	if err != nil {
		t.Fatalf("UpdateProfile() error = %v", err)
	}
	if updated.Name != "New Customer" || updated.PrimaryEmail != "new@example.com" || updated.PrimaryMobile != "+1 222" {
		t.Fatalf("updated profile = %#v", updated)
	}

	storedCustomer := repositories.CustomerRepository.Get(db, customer.ID)
	if storedCustomer == nil || storedCustomer.Name != "New Customer" || storedCustomer.PrimaryEmail != "new@example.com" || storedCustomer.PrimaryMobile != "+1 222" {
		t.Fatalf("stored customer = %#v", storedCustomer)
	}
	storedUser := repositories.UserRepository.Get(db, user.ID)
	if storedUser == nil || storedUser.Nickname != "New Customer" || storedUser.Email == nil || *storedUser.Email != "new@example.com" || storedUser.Mobile == nil || *storedUser.Mobile != "+1 222" {
		t.Fatalf("stored user = %#v", storedUser)
	}
	var storedCustomerUser models.CustomerUser
	if err := db.First(&storedCustomerUser, customerUser.ID).Error; err != nil {
		t.Fatalf("load customer user: %v", err)
	}
	if storedCustomerUser.DisplayName != "New Customer" || storedCustomerUser.Email != "new@example.com" || storedCustomerUser.Phone != "+1 222" {
		t.Fatalf("stored customer user = %#v", storedCustomerUser)
	}
	var storedConversation models.Conversation
	if err := db.First(&storedConversation, conversation.ID).Error; err != nil {
		t.Fatalf("load conversation: %v", err)
	}
	if storedConversation.CustomerName != "New Customer" {
		t.Fatalf("conversation customer name = %q", storedConversation.CustomerName)
	}
}

func setupCustomerPortalProfileTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(
		&models.User{},
		&models.Customer{},
		&models.CustomerIdentity{},
		&models.CustomerUser{},
		&models.CustomerDeviceBinding{},
		&models.Device{},
		&models.Conversation{},
		&models.ConversationParticipant{},
		&models.Message{},
		&models.ConversationMessageTranslation{},
		&models.Ticket{},
		&models.MeetingRoomJitsi{},
	); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	return db
}
