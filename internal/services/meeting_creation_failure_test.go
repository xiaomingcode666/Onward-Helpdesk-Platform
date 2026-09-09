package services_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/providers"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

type flakyTokenJitsiProvider struct {
	providers.JitsiProvider
	failures int
	calls    int
}

func (p *flakyTokenJitsiProvider) GenerateToken(roomName string, userInfo providers.JitsiUserInfo) (string, error) {
	p.calls++
	if p.calls <= p.failures {
		return "", errors.New("temporary jitsi token failure")
	}
	return "test-token-" + strconv.Itoa(p.calls), nil
}

func (p *flakyTokenJitsiProvider) GenerateJoinToken(roomName string, userInfo providers.JitsiUserInfo) (string, error) {
	return p.GenerateToken(roomName, userInfo)
}

func TestMeetingCreationRetriesTokenBeforeSucceeding(t *testing.T) {
	setupTicketTestDB(t)
	config.SetCurrent(&config.Config{Jitsi: config.JitsiConfig{MaxRetries: 2}})
	t.Cleanup(func() { config.SetCurrent(&config.Config{}) })

	realProvider := providers.NewJitsiClient(&config.JitsiConfig{AppID: "test-app", AppSecret: "test-secret"})
	provider := &flakyTokenJitsiProvider{JitsiProvider: realProvider, failures: 1}
	previousProvider := providers.DefaultJitsiProvider
	providers.DefaultJitsiProvider = provider
	t.Cleanup(func() { providers.DefaultJitsiProvider = previousProvider })

	operator, ticket := createMeetingFailureTicketFixture(t, "meeting-retry")
	config, err := services.MeetingService.CreateMeetingRoomForOperator(nil, ticket.ID, operator)
	if err != nil {
		t.Fatalf("CreateMeetingRoomForOperator() error = %v", err)
	}
	if config == nil || config.JWT == "" || provider.calls != 2 {
		t.Fatalf("meeting token retry result=%+v calls=%d", config, provider.calls)
	}
	var failureProgressCount int64
	if err := sqls.DB().Model(&models.TicketProgress{}).
		Where("ticket_id = ? AND metadata_json LIKE ?", ticket.ID, "%meeting_creation_failed%").
		Count(&failureProgressCount).Error; err != nil || failureProgressCount != 0 {
		t.Fatalf("successful retry should not write degradation progress count=%d err=%v", failureProgressCount, err)
	}
}

func TestMeetingCreationFailurePublishesCustomerVisibleDegradationOnce(t *testing.T) {
	setupTicketTestDB(t)
	config.SetCurrent(&config.Config{Jitsi: config.JitsiConfig{MaxRetries: 1}})
	t.Cleanup(func() { config.SetCurrent(&config.Config{}) })

	realProvider := providers.NewJitsiClient(&config.JitsiConfig{AppID: "test-app", AppSecret: "test-secret"})
	provider := &flakyTokenJitsiProvider{JitsiProvider: realProvider, failures: 10}
	previousProvider := providers.DefaultJitsiProvider
	providers.DefaultJitsiProvider = provider
	t.Cleanup(func() { providers.DefaultJitsiProvider = previousProvider })

	operator, ticket := createMeetingFailureTicketFixture(t, "meeting-degrade")
	if _, err := services.MeetingService.CreateMeetingRoomForOperator(nil, ticket.ID, operator); err == nil {
		t.Fatal("CreateMeetingRoomForOperator() expected provider failure")
	}
	if _, err := services.MeetingService.CreateMeetingRoomForOperator(nil, ticket.ID, operator); err == nil {
		t.Fatal("second CreateMeetingRoomForOperator() expected provider failure")
	}

	var meetingCount int64
	if err := sqls.DB().Model(&models.MeetingRoomJitsi{}).
		Where("tenant_id = ? AND ticket_id = ?", ticket.TenantID, strconv.FormatInt(ticket.ID, 10)).
		Count(&meetingCount).Error; err != nil || meetingCount != 0 {
		t.Fatalf("failed meeting creation persisted rooms=%d err=%v", meetingCount, err)
	}
	var failureProgress []models.TicketProgress
	if err := sqls.DB().Where("ticket_id = ? AND metadata_json LIKE ?", ticket.ID, "%meeting_creation_failed%").
		Find(&failureProgress).Error; err != nil {
		t.Fatalf("find meeting degradation progress: %v", err)
	}
	if len(failureProgress) != 1 || !failureProgress[0].VisibleToCustomer || !strings.Contains(failureProgress[0].Content, "重试") {
		t.Fatalf("meeting degradation progress = %+v", failureProgress)
	}
	var failureMessages []models.Message
	if err := sqls.DB().Where("conversation_id = ? AND sender_type = ? AND payload LIKE ?", ticket.ConversationID, enums.IMSenderTypeSystem, "%meeting_creation_failed%").
		Find(&failureMessages).Error; err != nil {
		t.Fatalf("find meeting degradation messages: %v", err)
	}
	if len(failureMessages) != 1 || !strings.Contains(failureMessages[0].Content, "文字/图片协同") {
		t.Fatalf("meeting degradation messages = %+v", failureMessages)
	}
}

func TestScheduledMeetingCreationDefersJitsiJWTUntilJoin(t *testing.T) {
	setupTicketTestDB(t)

	realProvider := providers.NewJitsiClient(&config.JitsiConfig{
		URL: "https://meet.example.com", AppID: "test-app", AppSecret: "test-secret",
	})
	provider := &flakyTokenJitsiProvider{JitsiProvider: realProvider}
	previousProvider := providers.DefaultJitsiProvider
	providers.DefaultJitsiProvider = provider
	t.Cleanup(func() { providers.DefaultJitsiProvider = previousProvider })

	operator, ticket := createMeetingFailureTicketFixture(t, "meeting-scheduled-token")
	scheduledAt := time.Now().Add(2 * time.Hour)
	created, err := services.MeetingService.CreateMeetingRoomWithOptions(
		nil,
		strconv.FormatInt(ticket.ID, 10),
		strconv.FormatInt(operator.UserID, 10),
		operator.Username,
		ticket.TenantID,
		services.CreateMeetingRoomOptions{ScheduledAt: &scheduledAt},
	)
	if err != nil {
		t.Fatalf("CreateMeetingRoomWithOptions() error = %v", err)
	}
	if created == nil || created.MeetingID == "" || created.RoomName == "" {
		t.Fatalf("scheduled meeting create returned incomplete config: %+v", created)
	}
	if created.JWT != "" {
		t.Fatalf("scheduled meeting create returned a stale-prone JWT: %q", created.JWT)
	}
	if provider.calls != 0 {
		t.Fatalf("scheduled meeting creation generated token %d times, want 0", provider.calls)
	}

	var meeting models.MeetingRoomJitsi
	if err := sqls.DB().First(&meeting, "id = ?", created.MeetingID).Error; err != nil {
		t.Fatalf("load scheduled meeting: %v", err)
	}
	if meeting.Status != "scheduled" || meeting.ScheduledAt == nil {
		t.Fatalf("persisted scheduled meeting = status %q scheduledAt %v", meeting.Status, meeting.ScheduledAt)
	}

	join, err := services.MeetingService.JoinMeeting(
		nil,
		created.MeetingID,
		strconv.FormatInt(operator.UserID, 10),
		operator.Username,
		"member",
		ticket.TenantID,
	)
	if err != nil {
		t.Fatalf("JoinMeeting() error = %v", err)
	}
	if join == nil || join.JWT == "" {
		t.Fatalf("JoinMeeting() did not issue a fresh token: %+v", join)
	}
	if provider.calls != 1 {
		t.Fatalf("join generated token %d times, want 1", provider.calls)
	}
}

func createMeetingFailureTicketFixture(t *testing.T, prefix string) (*dto.AuthPrincipal, *models.Ticket) {
	t.Helper()
	operator := createTestOperator(t, prefix+"-engineer")
	tenant, product, productModel, device, serviceCode := createTicketAfterSalesFixture(t, prefix, enums.StatusOk)
	operator.TenantID = tenant.ID
	teamID := ensureTestProductRepairEngineer(t, tenant.ID, product.ID, operator.UserID)
	now := time.Now()
	conversation := &models.Conversation{
		TenantID: tenant.ID, ProductID: product.ID, ProductModelID: productModel.ID, DeviceID: device.ID,
		Status: enums.IMConversationStatusActive, CurrentTeamID: teamID, CurrentAssigneeID: operator.UserID,
		LastMessageAt: now, LastActiveAt: now, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := sqls.DB().Create(conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	acceptedAt := now
	ticket := &models.Ticket{
		TenantID: tenant.ID, TicketNo: prefix + "-ticket", Title: "视频创建失败降级",
		Status: enums.TicketStatusAccepted, ProductID: product.ID, ProductModelID: productModel.ID, DeviceID: device.ID,
		ServiceCodeID: serviceCode.ID, ConversationID: conversation.ID, CurrentTeamID: teamID, CurrentAssigneeID: operator.UserID,
		AcceptedAt: &acceptedAt, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := repositories.TicketRepository.Create(sqls.DB(), ticket); err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	return operator, ticket
}
