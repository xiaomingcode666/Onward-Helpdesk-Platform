package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/eventbus"
	"remotehelpdesk/internal/pkg/providers"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/google/uuid"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MeetingRoomStatus 会议室状态
type MeetingRoomStatus struct {
	MeetingID   string     `json:"meetingId"`
	RoomName    string     `json:"roomName"`
	Status      string     `json:"status"`
	ScheduledAt *time.Time `json:"scheduledAt"`
	StartedAt   *time.Time `json:"startedAt"`
	EndedAt     *time.Time `json:"endedAt"`
	CreatedBy   string     `json:"createdBy"`
	CreatedAt   time.Time  `json:"createdAt"`
	PartCount   int64      `json:"participantCount"`
}

// JoinConfig 入会配置（前端嵌入 Jitsi iframe 需要）
type JoinConfig struct {
	Domain                 string `json:"domain"`
	RoomName               string `json:"roomName"`
	JWT                    string `json:"jwt"`
	JitsiURL               string `json:"jitsiUrl"`
	MeetingID              string `json:"meetingId"`
	TicketID               int64  `json:"ticketId,omitempty"`
	TicketNo               string `json:"ticketNo,omitempty"`
	Subject                string `json:"subject,omitempty"`
	Role                   string `json:"role"`
	CanEnd                 bool   `json:"canEnd"`
	TranscriptionEnabled   bool   `json:"transcriptionEnabled"`
	TranscriptionProvider  string `json:"transcriptionProvider"`
	TranscriptionReady     bool   `json:"transcriptionReady"`
	TranscriptionErrorCode string `json:"transcriptionErrorCode,omitempty"`
	TranscriptionError     string `json:"transcriptionError,omitempty"`
	ARDetectionEnabled     bool   `json:"arDetectionEnabled"`
	ARDetectionProvider    string `json:"arDetectionProvider"`
}

// MeetingService 管理 Jitsi 视频协作的创建、加入和结束
var MeetingService = newMeetingService()

func newMeetingService() *meetingService {
	return &meetingService{}
}

type meetingService struct{}

const (
	meetingStaleAfter = 12 * time.Hour
)

const meetingEndedBusinessCode = 101

const (
	meetingParticipantTypeEnterprise        = "enterprise"
	meetingParticipantTypePlatformAdmin     = "platform_admin"
	meetingParticipantTypeTenantAdmin       = "tenant_admin"
	meetingParticipantTypeAuthorizedSupport = "authorized_support"
)

func meetingEndedError() error {
	return errorsx.BusinessError(meetingEndedBusinessCode, "会议已结束，请重新发起视频协作")
}

func isMeetingEndedStatus(status string) bool {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "ended", "finished":
		return true
	default:
		return false
	}
}

func meetingParticipantRole(meeting *models.MeetingRoomJitsi, userID, userType string) string {
	switch normalizeMeetingParticipantType(userType) {
	case meetingParticipantTypeEnterprise, meetingParticipantTypePlatformAdmin, meetingParticipantTypeTenantAdmin, meetingParticipantTypeAuthorizedSupport:
		if meeting != nil && strings.TrimSpace(meeting.CreatedBy) != "" && strings.TrimSpace(meeting.CreatedBy) == strings.TrimSpace(userID) {
			return "moderator"
		}
	}
	return "participant"
}

type CreateMeetingRoomOptions struct {
	ScheduledAt                  *time.Time
	AllowManagedUnacceptedTicket bool
	ParticipantType              string
}

func generateMeetingTokenWithRetry(ctx context.Context, provider providers.JitsiProvider, roomName string, userInfo providers.JitsiUserInfo) (string, error) {
	if provider == nil {
		return "", errors.New("jitsi provider is not configured")
	}
	maxRetries := config.CurrentOrDefault().Jitsi.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 2
	}
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		token, err := provider.GenerateToken(roomName, userInfo)
		if err == nil {
			return token, nil
		}
		lastErr = err
		if attempt == maxRetries {
			break
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Duration(25*(attempt+1)) * time.Millisecond):
		}
	}
	return "", lastErr
}

func (s *meetingService) CreateMeetingRoomForOperator(ctx context.Context, ticketID int64, operator *dto.AuthPrincipal) (*JoinConfig, error) {
	return s.CreateMeetingRoomForOperatorWithOptions(ctx, ticketID, operator, CreateMeetingRoomOptions{})
}

func (s *meetingService) CreateMeetingRoomForOperatorWithOptions(ctx context.Context, ticketID int64, operator *dto.AuthPrincipal, options CreateMeetingRoomOptions) (*JoinConfig, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	ticket := repositories.TicketRepository.Get(sqls.DB(), ticketID)
	if ticket == nil {
		return nil, errorsx.InvalidParam("ticket not found")
	}
	if err := requireTicketTenantAccess(ticket, operator); err != nil {
		return nil, err
	}
	if canManageTicketDispatch(operator) {
		if err := validateMeetingStartManagedTicketStatus(ticket.Status); err != nil {
			return nil, err
		}
		options.AllowManagedUnacceptedTicket = true
	} else {
		if err := validateMeetingStartTicketStatus(ticket.Status); err != nil {
			return nil, err
		}
		if ticket.CurrentAssigneeID <= 0 || ticket.CurrentAssigneeID != operator.UserID {
			return nil, errorsx.Forbidden("只有当前工单负责人可以发起视频协作")
		}
	}
	options.ParticipantType = enterpriseMeetingParticipantType(ticket, operator)
	return s.CreateMeetingRoomWithOptions(
		ctx,
		strconv.FormatInt(ticket.ID, 10),
		strconv.FormatInt(operator.UserID, 10),
		firstNonEmptyString(strings.TrimSpace(operator.Nickname), strings.TrimSpace(operator.Username)),
		ticket.TenantID,
		options,
	)
}

func (s *meetingService) CreateMeetingRoomForWorkflow(ctx context.Context, tenantID, ticketID int64) (*JoinConfig, error) {
	if tenantID <= 0 || ticketID <= 0 {
		return nil, errorsx.InvalidParam("tenant and ticket are required")
	}
	ticket := repositories.TicketRepository.Get(sqls.DB(), ticketID)
	if ticket == nil || ticket.TenantID != tenantID {
		return nil, errorsx.InvalidParam("ticket not found for this tenant")
	}
	if ticket.CurrentAssigneeID <= 0 {
		return nil, errorsx.InvalidParam("ticket must be assigned before creating a meeting")
	}
	if err := validateMeetingStartTicketStatus(ticket.Status); err != nil {
		return nil, err
	}
	hostName := fmt.Sprintf("工程师 %d", ticket.CurrentAssigneeID)
	if user := repositories.UserRepository.Get(sqls.DB(), ticket.CurrentAssigneeID); user != nil {
		hostName = firstNonEmptyString(strings.TrimSpace(user.Nickname), strings.TrimSpace(user.Username), hostName)
	}
	return s.CreateMeetingRoom(
		ctx,
		strconv.FormatInt(ticket.ID, 10),
		strconv.FormatInt(ticket.CurrentAssigneeID, 10),
		hostName,
		ticket.TenantID,
	)
}

// CreateMeetingRoom 创建会议室
func (s *meetingService) CreateMeetingRoom(ctx context.Context, ticketID, hostUserID, hostName string, tenantID int64) (*JoinConfig, error) {
	return s.CreateMeetingRoomWithOptions(ctx, ticketID, hostUserID, hostName, tenantID, CreateMeetingRoomOptions{})
}

func (s *meetingService) CreateMeetingRoomWithOptions(ctx context.Context, ticketID, hostUserID, hostName string, tenantID int64, options CreateMeetingRoomOptions) (*JoinConfig, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	participantType := normalizeMeetingParticipantType(options.ParticipantType)
	jp := providers.DefaultJitsiProvider
	ticketIDValue, err := strconv.ParseInt(ticketID, 10, 64)
	if err != nil || ticketIDValue <= 0 {
		return nil, errorsx.InvalidParam("invalid ticket id")
	}
	now := time.Now()
	scheduledAt := normalizeMeetingScheduledAt(options.ScheduledAt, now)
	meetingStatus := "waiting"
	if scheduledAt != nil {
		meetingStatus = "scheduled"
	}
	var meeting models.MeetingRoomJitsi
	var token string
	userInfo := providers.JitsiUserInfo{
		UserID: hostUserID,
		Name:   hostName,
	}
	err = sqls.DB().Transaction(func(tx *gorm.DB) error {
		var ticket models.Ticket
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&ticket, ticketIDValue).Error; err != nil {
			return errorsx.InvalidParam("ticket not found for this tenant")
		}
		if ticket.TenantID != tenantID {
			return errorsx.InvalidParam("ticket not found for this tenant")
		}
		if options.AllowManagedUnacceptedTicket {
			if err := validateMeetingStartManagedTicketStatus(ticket.Status); err != nil {
				return err
			}
		} else if err := validateMeetingStartTicketStatus(ticket.Status); err != nil {
			return err
		}

		// An ended_at timestamp is authoritative even when a provider/webhook left
		// the legacy status as active. Never reopen or reuse that room.
		findResult := tx.Where("tenant_id = ? AND ticket_id = ? AND ended_at IS NULL AND status IN ?", tenantID, ticketID, []string{"waiting", "scheduled", "active"}).
			Order("created_at DESC").First(&meeting)
		if findResult.Error == nil {
			userInfo.IsModerator = meetingParticipantRole(&meeting, hostUserID, participantType) == "moderator"
			if meetingCreateResponseNeedsToken(meeting.Status) {
				token, err = generateMeetingTokenWithRetry(ctx, jp, meeting.RoomName, userInfo)
				if err != nil {
					return errorsx.BusinessError(100, "Jitsi 会议认证配置不完整")
				}
			}
			return nil
		}
		if findResult.Error != gorm.ErrRecordNotFound {
			return findResult.Error
		}
		meeting = models.MeetingRoomJitsi{
			ID:          utils.UUID(),
			TenantID:    tenantID,
			TicketID:    ticketID,
			RoomName:    fmt.Sprintf("rhd-%d-%s-%s", tenantID, ticketID, utils.RandomSuffix(8)),
			Status:      meetingStatus,
			CreatedBy:   hostUserID,
			ScheduledAt: scheduledAt,
			BaseModel: models.BaseModel{
				CreatedAt: now,
				UpdatedAt: now,
			},
		}
		userInfo.IsModerator = true
		if meetingCreateResponseNeedsToken(meeting.Status) {
			token, err = generateMeetingTokenWithRetry(ctx, jp, meeting.RoomName, userInfo)
			if err != nil {
				return errorsx.BusinessError(100, "Jitsi 会议认证配置不完整")
			}
		}
		if err := tx.Create(&meeting).Error; err != nil {
			return err
		}
		authorID, _ := strconv.ParseInt(hostUserID, 10, 64)
		metadata := map[string]string{"meetingId": meeting.ID, "roomName": meeting.RoomName}
		content := "发起视频协作，会议室：" + meeting.RoomName
		if scheduledAt != nil {
			metadata["scheduledAt"] = scheduledAt.Format(time.RFC3339)
			content = "预定视频协作，计划时间：" + formatEnterpriseTimePtr(scheduledAt) + "，会议室：" + meeting.RoomName
		}
		metadataJSON, _ := json.Marshal(metadata)
		return tx.Create(&models.TicketProgress{
			TenantID:          tenantID,
			TicketID:          ticketIDValue,
			EventType:         enums.TicketProgressEventProgress,
			Content:           content,
			MetadataJSON:      string(metadataJSON),
			AuthorID:          authorID,
			VisibleToCustomer: true,
			CreatedAt:         now,
		}).Error
	})
	if err != nil {
		slog.Error("create or reuse jitsi meeting error", "error", err)
		s.recordMeetingCreationFailure(ticketIDValue, tenantID, hostUserID, err)
		return nil, err
	}

	// Register the host identity. The provider webhook is authoritative for the
	// actual join time; issuing a token must not make the user appear online.
	role := meetingParticipantRole(&meeting, hostUserID, participantType)
	if err := s.recordParticipant(meeting.ID, hostUserID, participantType, hostName, role, nil); err != nil {
		slog.Warn("create participant record error", "error", err)
	}
	ticket := repositories.TicketRepository.Get(sqls.DB(), ticketIDValue)
	if ticket != nil {
		scheduled := meeting.Status == "scheduled"
		notificationTitle := "视频协作已发起"
		notificationContent := fmt.Sprintf("工单 %s 已发起视频协作。", firstNonEmptyString(strings.TrimSpace(ticket.TicketNo), fmt.Sprintf("#%d", ticket.ID)))
		notificationType := "meeting_started"
		idempotencyKey := "meeting.started:" + meeting.ID
		if scheduled {
			notificationTitle = "视频协作已预定"
			notificationContent = fmt.Sprintf("工单 %s 已预定视频协作，计划时间：%s。", firstNonEmptyString(strings.TrimSpace(ticket.TicketNo), fmt.Sprintf("#%d", ticket.ID)), formatEnterpriseTimePtr(meeting.ScheduledAt))
			notificationType = "meeting_scheduled"
			idempotencyKey = "meeting.scheduled:" + meeting.ID
		}
		if recipients, recipientErr := NotificationAudienceService.ResolveTicketRecipients(ticket); recipientErr != nil {
			slog.Warn("resolve meeting notification recipients failed", "ticket_id", ticket.ID, "meeting_id", meeting.ID, "error", recipientErr)
		} else if notifyErr := NotificationAudienceService.Deliver(recipients, request.CreateNotificationRequest{
			TenantID:         ticket.TenantID,
			Title:            notificationTitle,
			Content:          notificationContent,
			NotificationType: notificationType,
			BizType:          "ticket",
			BizID:            ticket.ID,
			ActionURL:        enterpriseMeetingRoomPath(meeting.ID, ticket.ID),
			Category:         "video",
			Level:            "info",
			Channels:         "in_app",
			IdempotencyKey:   idempotencyKey,
		}); notifyErr != nil {
			slog.Warn("queue meeting started notification failed", "ticket_id", ticket.ID, "meeting_id", meeting.ID, "error", notifyErr)
		}
	}
	if ticket != nil && ticket.ConversationID > 0 {
		eventType := "meeting_started"
		requestID := "meeting_started_" + meeting.ID
		content := "工程师已发起视频协作，客户、维修工程师和受邀供应商可进入会议。"
		if meeting.Status == "scheduled" {
			eventType = "meeting_scheduled"
			requestID = "meeting_scheduled_" + meeting.ID
			content = "工程师已预定视频协作，计划时间：" + formatEnterpriseTimePtr(meeting.ScheduledAt) + "，客户、维修工程师和受邀供应商可按时进入会议。"
		}
		payloadJSON, _ := json.Marshal(map[string]any{
			"eventType":   eventType,
			"source":      "jitsi_meeting",
			"ticketId":    ticket.ID,
			"meetingId":   meeting.ID,
			"scheduledAt": formatEnterpriseTimePtr(meeting.ScheduledAt),
		})
		if _, eventErr := MessageService.SendSystemMessageWithRequestID(
			ticket.ConversationID,
			requestID,
			content,
			string(payloadJSON),
			"",
		); eventErr != nil {
			slog.Warn("publish meeting conversation event failed", "ticket_id", ticket.ID, "meeting_id", meeting.ID, "error", eventErr)
		}
	}

	speechConfig := providers.CurrentSpeechConfig()
	readiness := SpeechRuntimeService.Readiness(ctx)
	transcriptionReady, transcriptionErrorCode, transcriptionError := meetingTranscriptionReadiness(speechConfig, readiness)
	arProvider := providers.CurrentMeetingARDetectionProvider()
	config := &JoinConfig{
		Domain:                 jp.GetJitsiDomain(),
		RoomName:               meeting.RoomName,
		JWT:                    token,
		JitsiURL:               jp.GetJitsiPublicURL(),
		MeetingID:              meeting.ID,
		Role:                   role,
		CanEnd:                 role == "moderator",
		TranscriptionEnabled:   speechConfig.TranscriptionEnabled(),
		TranscriptionProvider:  speechConfig.ProviderName(),
		TranscriptionReady:     transcriptionReady,
		TranscriptionErrorCode: transcriptionErrorCode,
		TranscriptionError:     transcriptionError,
		ARDetectionEnabled:     arProvider != nil && arProvider.Configured(),
		ARDetectionProvider:    arProvider.Name(),
	}
	applyMeetingTicketContext(config, ticket)
	return config, nil
}

func validateMeetingStartTicketStatus(status enums.TicketStatus) error {
	normalized := enums.NormalizeTicketStatus(string(status))
	switch normalized {
	case enums.TicketStatusClosed, enums.TicketStatusDone, enums.TicketStatusCancelled:
		return errorsx.InvalidParam("工单已结束，不能发起视频协作")
	}
	if !canStartMeetingFromStatus(normalized) {
		return errorsx.InvalidParam("工单受理后才能发起视频协作")
	}
	return nil
}

func meetingTranscriptionReadiness(speechConfig config.SpeechConfig, readiness speechProviderReadiness) (bool, string, string) {
	if !speechConfig.TranscriptionEnabled() {
		return false, readiness.Code, readiness.Message
	}
	if !speechConfig.Jigasi.Enabled {
		return false, "jigasi_disabled", "视频会议转录桥接未启用，请先启用 Jigasi 转录服务"
	}
	return readiness.Ready, readiness.Code, readiness.Message
}

func validateMeetingStartManagedTicketStatus(status enums.TicketStatus) error {
	normalized := enums.NormalizeTicketStatus(string(status))
	switch normalized {
	case enums.TicketStatusClosed, enums.TicketStatusDone, enums.TicketStatusCancelled:
		return errorsx.InvalidParam("工单已结束，不能发起视频协作")
	default:
		return nil
	}
}

func meetingCreateResponseNeedsToken(status string) bool {
	return strings.TrimSpace(status) != "scheduled"
}

func normalizeMeetingScheduledAt(value *time.Time, now time.Time) *time.Time {
	if value == nil || !value.After(now) {
		return nil
	}
	normalized := value.UTC()
	return &normalized
}

func enterpriseMeetingRoomPath(meetingID string, ticketID int64) string {
	values := url.Values{}
	values.Set("meeting_id", meetingID)
	if ticketID > 0 {
		values.Set("ticket_id", strconv.FormatInt(ticketID, 10))
	}
	return "/enterprise/meeting-room?" + values.Encode()
}

func (s *meetingService) recordMeetingCreationFailure(ticketID, tenantID int64, hostUserID string, cause error) {
	if ticketID <= 0 || tenantID <= 0 {
		return
	}
	ticket := repositories.TicketRepository.Get(sqls.DB(), ticketID)
	if ticket == nil || ticket.TenantID != tenantID || !canStartMeetingFromStatus(ticket.Status) {
		return
	}
	now := time.Now()
	content := "视频协作暂时创建失败，已自动降级为文字/图片协同；工程师可以稍后重试发起视频。"
	errorMessage := ""
	if cause != nil {
		errorMessage = strings.TrimSpace(cause.Error())
		if len(errorMessage) > 1000 {
			errorMessage = errorMessage[:1000]
		}
	}
	metadataJSON, _ := json.Marshal(map[string]any{
		"eventType": "meeting_creation_failed",
		"source":    "jitsi_meeting",
		"ticketId":  ticket.ID,
		"error":     errorMessage,
	})
	var existing int64
	if err := sqls.DB().Model(&models.TicketProgress{}).
		Where("ticket_id = ? AND metadata_json LIKE ?", ticket.ID, "%meeting_creation_failed%").
		Count(&existing).Error; err == nil && existing == 0 {
		authorID, _ := strconv.ParseInt(hostUserID, 10, 64)
		if err := repositories.TicketProgressRepository.Create(sqls.DB(), &models.TicketProgress{
			TenantID:          tenantID,
			TicketID:          ticket.ID,
			EventType:         enums.TicketProgressEventProgress,
			Content:           content,
			VisibleToCustomer: true,
			MetadataJSON:      string(metadataJSON),
			AuthorID:          authorID,
			CreatedAt:         now,
		}); err != nil {
			slog.Warn("record meeting creation failure progress failed", "ticket_id", ticket.ID, "error", err)
		}
	}
	if ticket.ConversationID <= 0 {
		return
	}
	if _, err := MessageService.SendSystemMessageWithRequestID(
		ticket.ConversationID,
		"meeting_creation_failed_"+strconv.FormatInt(ticket.ID, 10),
		content,
		string(metadataJSON),
		"",
	); err != nil {
		slog.Warn("publish meeting creation failure conversation event failed", "ticket_id", ticket.ID, "error", err)
	}
}

// requireMeetingTenantAccess 校验操作者租户对会议的访问权限
func (s *meetingService) requireMeetingTenantAccess(meetingID string, operatorTenantID int64) (*models.MeetingRoomJitsi, error) {
	var meeting models.MeetingRoomJitsi
	if err := sqls.DB().Where("id = ?", meetingID).First(&meeting).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errorsx.InvalidParam("meeting not found")
		}
		return nil, err
	}

	// 如果操作者有租户上下文，校验会议属于该租户
	if operatorTenantID > 0 {
		if meeting.TenantID != operatorTenantID {
			return nil, errorsx.InvalidParam("meeting not found for this tenant")
		}

		// 如果会议关联了工单，进一步校验工单也属于该租户
		if meeting.TicketID != "" {
			ticketID, parseErr := strconv.ParseInt(meeting.TicketID, 10, 64)
			if parseErr == nil && ticketID > 0 {
				ticket := repositories.TicketRepository.Get(sqls.DB(), ticketID)
				if ticket == nil || ticket.TenantID != operatorTenantID {
					return nil, errorsx.InvalidParam("meeting not found for this tenant")
				}
			}
		}
	}

	return &meeting, nil
}

// JoinMeeting 加入会议室（获取入会配置）
func (s *meetingService) JoinMeeting(ctx context.Context, meetingID, userID, userName, userType string, tenantID int64) (*JoinConfig, error) {
	jp := providers.DefaultJitsiProvider
	recordedUserType := normalizeMeetingParticipantType(userType)
	if userType == "member" || userType == "agent" {
		recordedUserType = meetingParticipantTypeEnterprise
	} else if userType == "enterprise_participant" {
		recordedUserType = meetingParticipantTypeEnterprise
	}

	meeting, err := s.requireMeetingTenantAccess(meetingID, tenantID)
	if err != nil {
		return nil, err
	}

	if isMeetingEndedStatus(meeting.Status) {
		return nil, meetingEndedError()
	}
	role := meetingParticipantRole(meeting, userID, recordedUserType)

	userInfo := providers.JitsiUserInfo{
		UserID:      userID,
		Name:        userName,
		IsModerator: role == "moderator",
	}
	token, err := jp.GenerateToken(meeting.RoomName, userInfo)
	if err != nil {
		return nil, errorsx.BusinessError(100, "Jitsi 入会认证配置不完整")
	}

	if err := s.recordParticipant(meetingID, userID, recordedUserType, userName, role, nil); err != nil {
		slog.Warn("create participant record error", "error", err)
	}

	speechConfig := providers.CurrentSpeechConfig()
	readiness := SpeechRuntimeService.Readiness(ctx)
	transcriptionReady, transcriptionErrorCode, transcriptionError := meetingTranscriptionReadiness(speechConfig, readiness)
	arProvider := providers.CurrentMeetingARDetectionProvider()
	config := &JoinConfig{
		Domain:                 jp.GetJitsiDomain(),
		RoomName:               meeting.RoomName,
		JWT:                    token,
		JitsiURL:               jp.GetJitsiPublicURL(),
		MeetingID:              meeting.ID,
		Role:                   role,
		CanEnd:                 role == "moderator",
		TranscriptionEnabled:   speechConfig.TranscriptionEnabled(),
		TranscriptionProvider:  speechConfig.ProviderName(),
		TranscriptionReady:     transcriptionReady,
		TranscriptionErrorCode: transcriptionErrorCode,
		TranscriptionError:     transcriptionError,
		ARDetectionEnabled:     arProvider != nil && arProvider.Configured(),
		ARDetectionProvider:    arProvider.Name(),
	}
	if ticketID, parseErr := strconv.ParseInt(meeting.TicketID, 10, 64); parseErr == nil && ticketID > 0 {
		ticket := repositories.TicketRepository.Get(sqls.DB(), ticketID)
		if ticket != nil && ticket.TenantID == meeting.TenantID {
			applyMeetingTicketContext(config, ticket)
		}
	}
	return config, nil
}

func applyMeetingTicketContext(config *JoinConfig, ticket *models.Ticket) {
	if config == nil || ticket == nil {
		return
	}
	config.TicketID = ticket.ID
	config.TicketNo = strings.TrimSpace(ticket.TicketNo)
	title := strings.TrimSpace(ticket.Title)
	switch {
	case config.TicketNo != "" && title != "":
		config.Subject = config.TicketNo + " · " + title
	case title != "":
		config.Subject = title
	case config.TicketNo != "":
		config.Subject = config.TicketNo + " · 远程视频协作"
	default:
		config.Subject = "远程视频协作"
	}
}

// JoinMeetingForOperator applies the same product-team data scope used by the
// ticket workbench before issuing an enterprise meeting token.
func (s *meetingService) JoinMeetingForOperator(ctx context.Context, meetingID string, operator *dto.AuthPrincipal) (*JoinConfig, error) {
	meeting, ticket, err := s.requireMeetingTicketAccess(meetingID, operator, false)
	if err != nil {
		return nil, err
	}
	userType := enterpriseMeetingParticipantType(ticket, operator)
	if userType == meetingParticipantTypeEnterprise &&
		(ticket == nil || ticket.CurrentAssigneeID != operator.UserID) && !canManageTicketDispatch(operator) {
		userType = "enterprise_participant"
	}
	name := firstNonEmptyString(strings.TrimSpace(operator.Nickname), strings.TrimSpace(operator.Username), fmt.Sprintf("工程师 %d", operator.UserID))
	return s.JoinMeeting(ctx, meeting.ID, strconv.FormatInt(operator.UserID, 10), name, userType, ticket.TenantID)
}

// ConfirmJoinForOperator records the authenticated participant only after the
// embedded Jitsi client confirms that it joined the conference.
func (s *meetingService) ConfirmJoinForOperator(ctx context.Context, meetingID string, operator *dto.AuthPrincipal) error {
	meeting, ticket, err := s.requireMeetingTicketAccess(meetingID, operator, false)
	if err != nil {
		return err
	}
	if isMeetingEndedStatus(meeting.Status) {
		return meetingEndedError()
	}
	name := firstNonEmptyString(strings.TrimSpace(operator.Nickname), strings.TrimSpace(operator.Username), fmt.Sprintf("工程师 %d", operator.UserID))
	return s.confirmParticipantJoin(
		meeting.ID,
		strconv.FormatInt(operator.UserID, 10),
		enterpriseMeetingParticipantType(ticket, operator),
		name,
		time.Now(),
	)
}

// ConfirmLeaveForOperator closes the authenticated participant's current
// attendance window after the embedded Jitsi client leaves the conference.
func (s *meetingService) ConfirmLeaveForOperator(ctx context.Context, meetingID string, operator *dto.AuthPrincipal) error {
	meeting, ticket, err := s.requireMeetingTicketAccess(meetingID, operator, false)
	if err != nil {
		return err
	}
	return s.confirmParticipantLeave(
		meeting,
		strconv.FormatInt(operator.UserID, 10),
		enterpriseMeetingParticipantType(ticket, operator),
		time.Now(),
	)
}

func enterpriseMeetingParticipantType(ticket *models.Ticket, operator *dto.AuthPrincipal) string {
	if operator == nil {
		return meetingParticipantTypeEnterprise
	}
	if operator.SupportGrantID > 0 || strings.TrimSpace(operator.SupportMode) != "" {
		return meetingParticipantTypeAuthorizedSupport
	}
	if isPlatformMeetingOperator(operator) {
		return meetingParticipantTypePlatformAdmin
	}
	if operator.HasRole(EnterpriseRoleOwner) || operator.HasRole(EnterpriseRoleAdmin) || operator.HasRole("tenant_admin_seed") || operator.HasRole(EnterpriseRoleServiceManager) {
		return meetingParticipantTypeTenantAdmin
	}
	if canManageTicketDispatch(operator) || ticket != nil && ticket.CurrentAssigneeID == operator.UserID {
		return meetingParticipantTypeEnterprise
	}
	return meetingParticipantTypeEnterprise
}

func isPlatformMeetingOperator(operator *dto.AuthPrincipal) bool {
	if operator == nil {
		return false
	}
	if operator.IsPlatform() || operator.SubjectType == models.SubjectTypePlatformStaff || operator.PlatformStaffID > 0 {
		return true
	}
	for _, role := range operator.Roles {
		switch strings.TrimSpace(role) {
		case constants.RoleCodeSuperAdmin, constants.RoleCodeAdmin, PlatformRoleAdmin, "platform_staff":
			return true
		}
	}
	return false
}

func normalizeMeetingParticipantType(userType string) string {
	switch strings.TrimSpace(userType) {
	case "", "member", "agent", meetingParticipantTypeEnterprise, "enterprise_participant":
		return meetingParticipantTypeEnterprise
	case meetingParticipantTypePlatformAdmin, meetingParticipantTypeTenantAdmin, meetingParticipantTypeAuthorizedSupport:
		return strings.TrimSpace(userType)
	default:
		return strings.TrimSpace(userType)
	}
}

func (s *meetingService) HeartbeatForOperator(ctx context.Context, meetingID string, operator *dto.AuthPrincipal) error {
	meeting, ticket, err := s.requireMeetingTicketAccess(meetingID, operator, false)
	if err != nil {
		return err
	}
	return s.heartbeatParticipant(meeting, strconv.FormatInt(operator.UserID, 10), enterpriseMeetingParticipantType(ticket, operator), time.Now())
}

func (s *meetingService) heartbeatParticipant(meeting *models.MeetingRoomJitsi, userID, userType string, heartbeatAt time.Time) error {
	if meeting == nil || isMeetingEndedStatus(meeting.Status) {
		return meetingEndedError()
	}
	result := sqls.DB().Model(&models.MeetingParticipant{}).
		Where("meeting_id = ? AND user_id = ? AND user_type = ? AND joined_at IS NOT NULL AND left_at IS NULL", meeting.ID, userID, userType).
		Update("updated_at", heartbeatAt)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errorsx.BusinessError(100, "参会状态已失效，请重新进入会议")
	}
	return sqls.DB().Model(&models.MeetingRoomJitsi{}).
		Where("id = ? AND status = ?", meeting.ID, "active").
		Update("updated_at", heartbeatAt).Error
}

func (s *meetingService) ReconcileStaleMeetingPresenceForTenant(ctx context.Context, tenantID int64, limit int) int {
	if tenantID <= 0 {
		return 0
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	var meetings []models.MeetingRoomJitsi
	if err := sqls.DB().Where("tenant_id = ? AND status = ?", tenantID, "active").Order("updated_at ASC").Limit(limit).Find(&meetings).Error; err != nil {
		slog.Warn("list active meetings for presence reconciliation failed", "tenant_id", tenantID, "error", err)
		return 0
	}
	ended := 0
	for i := range meetings {
		changed, err := s.reconcileStaleMeetingPresence(ctx, &meetings[i])
		if err != nil {
			slog.Warn("reconcile meeting presence failed", "meeting_id", meetings[i].ID, "error", err)
			continue
		}
		if changed {
			ended++
		}
	}
	return ended
}

func (s *meetingService) reconcileStaleMeetingPresence(ctx context.Context, meeting *models.MeetingRoomJitsi) (bool, error) {
	if meeting == nil || meeting.Status != "active" {
		return false, nil
	}
	now := time.Now()
	cutoff := now.Add(-meetingParticipantStaleAfter)
	var latestActivity models.MeetingParticipant
	latestResult := sqls.DB().Where("meeting_id = ? AND joined_at IS NOT NULL", meeting.ID).Order("updated_at DESC").First(&latestActivity)
	if latestResult.Error != nil {
		if errors.Is(latestResult.Error, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, latestResult.Error
	}
	var stale []models.MeetingParticipant
	if err := sqls.DB().Where(
		"meeting_id = ? AND joined_at IS NOT NULL AND left_at IS NULL AND updated_at < ?",
		meeting.ID, cutoff,
	).Find(&stale).Error; err != nil {
		return false, err
	}
	for i := range stale {
		leftAt := stale[i].UpdatedAt.Add(meetingParticipantStaleAfter)
		if leftAt.After(now) {
			leftAt = now
		}
		if stale[i].JoinedAt != nil && leftAt.Before(*stale[i].JoinedAt) {
			leftAt = *stale[i].JoinedAt
		}
		if err := sqls.DB().Model(&models.MeetingParticipant{}).Where("id = ? AND left_at IS NULL", stale[i].ID).Updates(map[string]any{
			"left_at": leftAt, "duration": participantDurationSeconds(*stale[i].JoinedAt, leftAt), "updated_at": now,
		}).Error; err != nil {
			return false, err
		}
	}
	var online int64
	if err := sqls.DB().Model(&models.MeetingParticipant{}).
		Where("meeting_id = ? AND joined_at IS NOT NULL AND left_at IS NULL", meeting.ID).
		Count(&online).Error; err != nil || online > 0 {
		return false, err
	}
	if latestActivity.UpdatedAt.After(cutoff) {
		return false, nil
	}
	endedAt := latestActivity.UpdatedAt.Add(meetingParticipantStaleAfter)
	if endedAt.After(now) {
		endedAt = now
	}
	return s.endMeetingAt(ctx, meeting.ID, meeting.TenantID, endedAt)
}

func (s *meetingService) requireMeetingTicketAccess(meetingID string, operator *dto.AuthPrincipal, mutation bool) (*models.MeetingRoomJitsi, *models.Ticket, error) {
	if operator == nil {
		return nil, nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	meeting, err := s.requireMeetingTenantAccess(meetingID, operator.EffectiveTenantID())
	if err != nil {
		return nil, nil, err
	}
	ticketID, err := strconv.ParseInt(meeting.TicketID, 10, 64)
	if err != nil || ticketID <= 0 {
		return nil, nil, errorsx.Forbidden("meeting is not bound to an accessible ticket")
	}
	ticket := repositories.TicketRepository.Get(sqls.DB(), ticketID)
	if ticket == nil || ticket.TenantID != meeting.TenantID {
		return nil, nil, errorsx.Forbidden("meeting is not bound to an accessible ticket")
	}
	if mutation {
		err = requireTicketMutationAccess(ticket, operator)
	} else {
		err = requireTicketTenantAccess(ticket, operator)
	}
	if err != nil {
		return nil, nil, err
	}
	return meeting, ticket, nil
}

func (s *meetingService) recordParticipant(meetingID, userID, userType, userName, role string, joinedAt *time.Time) error {
	now := time.Now()
	updatedAt := now
	if joinedAt != nil {
		updatedAt = *joinedAt
	}
	participant := &models.MeetingParticipant{
		ID:              utils.UUID(),
		MeetingID:       meetingID,
		UserID:          userID,
		UserType:        userType,
		ParticipantName: userName,
		Role:            role,
		JoinedAt:        joinedAt,
		BaseModel: models.BaseModel{
			CreatedAt: now,
			UpdatedAt: updatedAt,
		},
	}
	updates := map[string]any{
		"participant_name": userName,
		"role":             role,
		"updated_at":       updatedAt,
	}
	if joinedAt != nil {
		updates["joined_at"] = *joinedAt
		updates["left_at"] = nil
		updates["duration"] = 0
	}
	return sqls.DB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "meeting_id"}, {Name: "user_id"}, {Name: "user_type"}},
		DoUpdates: clause.Assignments(updates),
	}).Create(participant).Error
}

// EndMeeting 结束会议
func (s *meetingService) EndMeeting(ctx context.Context, meetingID string, tenantID int64) error {
	_, err := s.endMeetingAt(ctx, meetingID, tenantID, time.Now())
	return err
}

func (s *meetingService) EndMeetingFromWebhook(ctx context.Context, meetingID, roomName string) error {
	return s.EndMeetingFromWebhookAt(ctx, meetingID, roomName, time.Now())
}

func (s *meetingService) EndMeetingFromWebhookAt(ctx context.Context, meetingID, roomName string, endedAt time.Time) error {
	meeting, err := s.resolveWebhookMeeting(meetingID, roomName)
	if err != nil {
		return err
	}
	if endedAt.IsZero() {
		endedAt = time.Now()
	}
	_, err = s.endMeetingAt(ctx, meeting.ID, meeting.TenantID, endedAt)
	return err
}

func (s *meetingService) EndMeetingForOperator(ctx context.Context, meetingID string, operator *dto.AuthPrincipal) error {
	_, err := s.EndMeetingForOperatorWithResult(ctx, meetingID, operator)
	return err
}

func (s *meetingService) EndMeetingForOperatorWithResult(ctx context.Context, meetingID string, operator *dto.AuthPrincipal) (bool, error) {
	meeting, ticket, err := s.requireMeetingTicketAccess(meetingID, operator, false)
	if err != nil {
		return false, err
	}
	if err := requireTicketMeetingEndAccess(ticket, operator); err != nil {
		return false, err
	}
	return s.endMeetingAt(ctx, meeting.ID, ticket.TenantID, time.Now())
}

func requireTicketMeetingEndAccess(ticket *models.Ticket, operator *dto.AuthPrincipal) error {
	if err := requireTicketTenantAccess(ticket, operator); err != nil {
		return err
	}
	if operator == nil {
		return errorsx.ForbiddenI18n("error.e0225")
	}
	if canManageTicketDispatch(operator) {
		return nil
	}
	if ticket != nil && ticket.CurrentAssigneeID > 0 && ticket.CurrentAssigneeID == operator.UserID {
		return nil
	}
	if ticket == nil || !operator.HasRole(EnterpriseRoleEngineer) {
		return errorsx.Forbidden("only the current assignee, product repair engineer, or administrator can end the meeting")
	}
	teamID := ticket.CurrentTeamID
	if teamID <= 0 && ticket.ProductID > 0 {
		if team := ProductSupportOrganizationService.FindProductRepairTeam(sqls.DB(), ticket.TenantID, ticket.ProductID); team != nil {
			teamID = team.ID
		}
	}
	if teamID > 0 && AgentTeamMemberService.IsUserActiveMemberOfTeamDB(sqls.DB(), ticket.TenantID, teamID, operator.UserID) {
		return nil
	}
	return errorsx.Forbidden("only the current assignee, product repair engineer, or administrator can end the meeting")
}

func (s *meetingService) endMeetingAt(ctx context.Context, meetingID string, tenantID int64, endedAt time.Time) (bool, error) {
	changed := false
	err := sqls.DB().Transaction(func(tx *gorm.DB) error {
		var err error
		changed, _, err = s.endMeetingAtDB(tx, meetingID, tenantID, endedAt, "meeting_service", "", "")
		return err
	})
	if err != nil {
		return false, err
	}
	if changed {
		eventbus.WakeDefaultOutboxPublisher()
	}
	return changed, nil
}

func (s *meetingService) endMeetingAtDB(
	tx *gorm.DB,
	meetingID string,
	tenantID int64,
	endedAt time.Time,
	source string,
	actorID string,
	actorType string,
) (bool, models.MeetingRoomJitsi, error) {
	var endedMeeting models.MeetingRoomJitsi
	if tx == nil {
		return false, endedMeeting, errorsx.InvalidParam("database is required")
	}
	if endedAt.IsZero() {
		endedAt = time.Now()
	}
	if source == "" {
		source = "meeting_service"
	}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&endedMeeting, "id = ?", meetingID).Error; err != nil {
		return false, endedMeeting, errorsx.InvalidParam("meeting not found")
	}
	if tenantID > 0 && endedMeeting.TenantID != tenantID {
		return false, endedMeeting, errorsx.InvalidParam("meeting not found for this tenant")
	}
	if isMeetingEndedStatus(endedMeeting.Status) {
		return false, endedMeeting, nil
	}
	if err := tx.Model(&models.MeetingRoomJitsi{}).Where("id = ?", meetingID).Updates(map[string]any{
		"status":     "ended",
		"ended_at":   endedAt,
		"updated_at": endedAt,
	}).Error; err != nil {
		return false, endedMeeting, err
	}
	var participants []models.MeetingParticipant
	if err := tx.Where("meeting_id = ? AND left_at IS NULL", meetingID).Find(&participants).Error; err != nil {
		return false, endedMeeting, err
	}
	attendance := repositories.MeetingRoomRepository.AttendanceStats(tx, meetingID)
	firstJoinedAt := confirmedMeetingStartedAt(&endedMeeting, attendance)
	for i := range participants {
		duration := int64(0)
		if participants[i].JoinedAt != nil && endedAt.After(*participants[i].JoinedAt) {
			duration = int64(endedAt.Sub(*participants[i].JoinedAt).Seconds())
		}
		if participants[i].JoinedAt != nil && (firstJoinedAt == nil || participants[i].JoinedAt.Before(*firstJoinedAt)) {
			joinedAt := *participants[i].JoinedAt
			firstJoinedAt = &joinedAt
		}
		if err := tx.Model(&models.MeetingParticipant{}).Where("id = ?", participants[i].ID).Updates(map[string]any{
			"left_at":    endedAt,
			"updated_at": endedAt,
			"duration":   duration,
		}).Error; err != nil {
			return false, endedMeeting, err
		}
	}
	durationMs := int64(0)
	if firstJoinedAt != nil && endedAt.After(*firstJoinedAt) {
		durationMs = endedAt.Sub(*firstJoinedAt).Milliseconds()
	}
	if ticketID, parseErr := strconv.ParseInt(endedMeeting.TicketID, 10, 64); parseErr == nil && ticketID > 0 {
		var ticket models.Ticket
		if err := tx.First(&ticket, ticketID).Error; err == nil && ticket.TenantID == endedMeeting.TenantID {
			if enums.NormalizeTicketStatus(string(ticket.Status)) == enums.TicketStatusVideoSupport {
				if err := tx.Model(&models.Ticket{}).Where("id = ?", ticket.ID).Updates(map[string]any{
					"status":     enums.TicketStatusProcessing,
					"updated_at": endedAt,
				}).Error; err != nil {
					return false, endedMeeting, err
				}
			}
			if err := ensureMeetingEndedTicketProgress(tx, &endedMeeting, ticket.ID, durationMs, endedAt); err != nil {
				return false, endedMeeting, err
			}
		}
	}
	if err := MeetingIntelligenceService.EnsureClosureTranscriptArchiveDB(tx, &endedMeeting, durationMs, endedAt); err != nil {
		return false, endedMeeting, err
	}
	meetingEndedEvent := events.MeetingEndedEvent{
		MeetingID:  endedMeeting.ID,
		TicketID:   endedMeeting.TicketID,
		TenantID:   strconv.FormatInt(endedMeeting.TenantID, 10),
		DurationMs: durationMs,
	}
	if actorID != "" {
		meetingEndedEvent.EndedBy = actorID
	}
	event := eventbus.DurableEvent{
		TenantID:       endedMeeting.TenantID,
		IdempotencyKey: "tenant:" + strconv.FormatInt(endedMeeting.TenantID, 10) + ":meeting.ended:" + endedMeeting.ID,
		EventType:      events.EventMeetingEnded,
		Payload:        meetingEndedEvent,
		Source:         source,
		AggregateID:    endedMeeting.ID,
		CreatedAt:      endedAt,
	}
	if actorID != "" {
		event.ActorID = actorID
	}
	if actorType != "" {
		event.ActorType = actorType
	}
	if _, err := eventbus.EnqueueTx(tx, event); err != nil {
		return false, endedMeeting, err
	}
	return true, endedMeeting, nil
}

func ensureMeetingEndedTicketProgress(tx *gorm.DB, meeting *models.MeetingRoomJitsi, ticketID, durationMs int64, endedAt time.Time) error {
	if tx == nil || meeting == nil || ticketID <= 0 {
		return nil
	}
	metadata, err := json.Marshal(map[string]any{"meetingId": meeting.ID})
	if err != nil {
		return err
	}
	var existingCount int64
	if err := tx.Model(&models.TicketProgress{}).
		Where("ticket_id = ? AND event_type = ? AND metadata_json = ?", ticketID, enums.TicketProgressEventMeetingEnded, string(metadata)).
		Count(&existingCount).Error; err != nil {
		return err
	}
	if existingCount > 0 {
		return nil
	}
	durationText := formatMeetingDurationText(durationMs)
	content := fmt.Sprintf("视频协作已结束，时长：%s", durationText)
	if strings.TrimSpace(meeting.RoomName) != "" {
		content = fmt.Sprintf("视频协作已结束，会议室：%s，时长：%s", meeting.RoomName, durationText)
	}
	return tx.Create(&models.TicketProgress{
		TenantID:          meeting.TenantID,
		TicketID:          ticketID,
		EventType:         enums.TicketProgressEventMeetingEnded,
		Content:           content,
		VisibleToCustomer: true,
		MetadataJSON:      string(metadata),
		CreatedAt:         endedAt,
	}).Error
}

func formatMeetingDurationText(ms int64) string {
	if ms < 1000 {
		return "不足 1 秒"
	}
	seconds := ms / 1000
	if seconds < 60 {
		return fmt.Sprintf("%d 秒", seconds)
	}
	minutes := seconds / 60
	secs := seconds % 60
	if minutes < 60 {
		if secs > 0 {
			return fmt.Sprintf("%d 分 %d 秒", minutes, secs)
		}
		return fmt.Sprintf("%d 分", minutes)
	}
	hours := minutes / 60
	mins := minutes % 60
	if mins > 0 {
		return fmt.Sprintf("%d 小时 %d 分", hours, mins)
	}
	return fmt.Sprintf("%d 小时", hours)
}

// ExpireStaleMeetings closes sessions left active after an abnormal client or
// provider disconnect. The recorded duration is capped at the stale deadline.
func (s *meetingService) ExpireStaleMeetings(limit int) int {
	if limit <= 0 {
		limit = 100
	}
	now := time.Now()
	cutoff := now.Add(-meetingStaleAfter)
	var meetings []models.MeetingRoomJitsi
	if err := sqls.DB().Where(
		"status IN ? AND ((started_at IS NOT NULL AND started_at <= ?) OR (started_at IS NULL AND created_at <= ?))",
		[]string{"waiting", "scheduled", "active"}, cutoff, cutoff,
	).Order("created_at ASC").Limit(limit).Find(&meetings).Error; err != nil {
		slog.Warn("list stale meetings failed", "error", err)
		return 0
	}
	expired := 0
	for i := range meetings {
		startedAt := meetings[i].CreatedAt
		if meetings[i].StartedAt != nil {
			startedAt = *meetings[i].StartedAt
		}
		endedAt := startedAt.Add(meetingStaleAfter)
		if endedAt.After(now) {
			endedAt = now
		}
		if _, err := s.endMeetingAt(context.Background(), meetings[i].ID, meetings[i].TenantID, endedAt); err != nil {
			slog.Warn("expire stale meeting failed", "meetingId", meetings[i].ID, "error", err)
			continue
		}
		expired++
	}
	return expired
}

// GetMeetingStatus 获取会议室状态
func (s *meetingService) GetMeetingStatus(ctx context.Context, meetingID string, tenantID int64) (*MeetingRoomStatus, error) {
	meeting, err := s.requireMeetingTenantAccess(meetingID, tenantID)
	if err != nil {
		return nil, err
	}
	if _, err := s.reconcileStaleMeetingPresence(ctx, meeting); err != nil {
		slog.Warn("reconcile meeting presence before status failed", "meeting_id", meeting.ID, "error", err)
	}
	meeting, err = s.requireMeetingTenantAccess(meetingID, tenantID)
	if err != nil {
		return nil, err
	}

	attendance := repositories.MeetingRoomRepository.AttendanceStats(sqls.DB(), meetingID)
	startedAt := confirmedMeetingStartedAt(meeting, attendance)

	return &MeetingRoomStatus{
		MeetingID:   meeting.ID,
		RoomName:    meeting.RoomName,
		Status:      meeting.Status,
		ScheduledAt: meeting.ScheduledAt,
		StartedAt:   startedAt,
		EndedAt:     meeting.EndedAt,
		CreatedBy:   meeting.CreatedBy,
		CreatedAt:   meeting.CreatedAt,
		PartCount:   attendance.Count,
	}, nil
}

func (s *meetingService) GetMeetingStatusForOperator(ctx context.Context, meetingID string, operator *dto.AuthPrincipal) (*MeetingRoomStatus, error) {
	meeting, ticket, err := s.requireMeetingTicketAccess(meetingID, operator, false)
	if err != nil {
		return nil, err
	}
	return s.GetMeetingStatus(ctx, meeting.ID, ticket.TenantID)
}

// ListMeetingsByTicket 获取工单关联的会议列表
func (s *meetingService) ListMeetingsByTicket(ctx context.Context, ticketID string) ([]*MeetingRoomStatus, error) {
	return s.listMeetingsByTicket(ctx, 0, ticketID)
}

func (s *meetingService) ListMeetingsByTicketForOperator(ctx context.Context, ticketID string, operator *dto.AuthPrincipal) ([]*MeetingRoomStatus, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	ticketIDValue, err := strconv.ParseInt(ticketID, 10, 64)
	if err != nil || ticketIDValue <= 0 {
		return nil, errorsx.InvalidParam("invalid ticket id")
	}
	ticket := repositories.TicketRepository.Get(sqls.DB(), ticketIDValue)
	if err := requireTicketTenantAccess(ticket, operator); err != nil {
		return nil, err
	}
	return s.listMeetingsByTicket(ctx, ticket.TenantID, ticketID)
}

func (s *meetingService) listMeetingsByTicket(_ context.Context, tenantID int64, ticketID string) ([]*MeetingRoomStatus, error) {
	var meetings []models.MeetingRoomJitsi
	query := sqls.DB().Where("ticket_id = ?", ticketID)
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if err := query.Order("created_at DESC").Find(&meetings).Error; err != nil {
		return nil, err
	}

	result := make([]*MeetingRoomStatus, 0, len(meetings))
	for _, m := range meetings {
		attendance := repositories.MeetingRoomRepository.AttendanceStats(sqls.DB(), m.ID)
		startedAt := confirmedMeetingStartedAt(&m, attendance)
		result = append(result, &MeetingRoomStatus{
			MeetingID:   m.ID,
			RoomName:    m.RoomName,
			Status:      m.Status,
			ScheduledAt: m.ScheduledAt,
			StartedAt:   startedAt,
			EndedAt:     m.EndedAt,
			CreatedBy:   m.CreatedBy,
			CreatedAt:   m.CreatedAt,
			PartCount:   attendance.Count,
		})
	}
	return result, nil
}

// recordParticipantLeave 记录参与者离开
func (s *meetingService) recordParticipantLeave(meetingID, userID string) error {
	now := time.Now()
	return sqls.DB().Transaction(func(tx *gorm.DB) error {
		var participants []models.MeetingParticipant
		if err := tx.Where("meeting_id = ? AND user_id = ? AND left_at IS NULL", meetingID, userID).Find(&participants).Error; err != nil {
			return err
		}
		for i := range participants {
			duration := int64(0)
			if participants[i].JoinedAt != nil && now.After(*participants[i].JoinedAt) {
				duration = int64(now.Sub(*participants[i].JoinedAt).Seconds())
			}
			if err := tx.Model(&models.MeetingParticipant{}).Where("id = ?", participants[i].ID).Updates(map[string]any{
				"left_at":    now,
				"duration":   duration,
				"updated_at": now,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// RecordParticipantJoin 记录参与者加入（从 webhook 调用）
func (s *meetingService) RecordParticipantJoin(ctx context.Context, meetingID, userID, userName, role string) {
	now := time.Now()
	if err := s.recordParticipant(meetingID, userID, "external", userName, role, &now); err != nil {
		slog.Warn("record participant join error", "error", err)
	}
}

// HandleParticipantLeft 处理参与者离开事件
func (s *meetingService) HandleParticipantLeft(ctx context.Context, meetingID, userID string) error {
	return s.recordParticipantLeave(meetingID, userID)
}

func (s *meetingService) RecordParticipantJoinFromWebhook(ctx context.Context, meetingID, roomName, userID, userName string, occurredAt time.Time) error {
	if strings.TrimSpace(userID) == "" {
		return errorsx.InvalidParam("jitsi participant id is required")
	}
	meeting, err := s.resolveWebhookMeeting(meetingID, roomName)
	if err != nil {
		return err
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}
	if meeting.EndedAt != nil && occurredAt.After(*meeting.EndedAt) {
		return nil
	}
	return s.recordWebhookParticipantJoin(
		meeting.ID, strings.TrimSpace(userID), strings.TrimSpace(userName), occurredAt, meeting.EndedAt,
	)
}

func (s *meetingService) recordWebhookParticipantJoin(meetingID, userID, userName string, joinedAt time.Time, meetingEndedAt *time.Time) error {
	return sqls.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.mergeWebhookParticipantJoin(tx, meetingID, userID, userName, joinedAt, meetingEndedAt); err != nil {
			return err
		}
		return s.activateMeetingFromAttendance(tx, meetingID)
	})
}

func (s *meetingService) confirmParticipantJoin(meetingID, userID, userType, userName string, joinedAt time.Time) error {
	return sqls.DB().Transaction(func(tx *gorm.DB) error {
		var meeting models.MeetingRoomJitsi
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&meeting, "id = ?", meetingID).Error; err != nil {
			return errorsx.InvalidParam("meeting not found")
		}
		if isMeetingEndedStatus(meeting.Status) {
			return meetingEndedError()
		}
		var participant models.MeetingParticipant
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("meeting_id = ? AND user_id = ? AND user_type = ?", meetingID, userID, userType).
			First(&participant).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errorsx.InvalidParam("meeting participant is not registered")
			}
			return err
		}
		updates := map[string]any{
			"participant_name": userName,
			"updated_at":       joinedAt,
		}
		if participant.JoinedAt == nil || participant.LeftAt != nil {
			updates["joined_at"] = joinedAt
			updates["left_at"] = nil
			updates["duration"] = 0
		}
		if err := tx.Model(&models.MeetingParticipant{}).Where("id = ?", participant.ID).Updates(updates).Error; err != nil {
			return err
		}
		return s.activateMeetingFromAttendance(tx, meetingID)
	})
}

func (s *meetingService) confirmParticipantLeave(meeting *models.MeetingRoomJitsi, userID, userType string, leftAt time.Time) error {
	if meeting == nil {
		return errorsx.InvalidParam("meeting not found")
	}
	if meeting.EndedAt != nil && leftAt.After(*meeting.EndedAt) {
		leftAt = *meeting.EndedAt
	}
	return sqls.DB().Transaction(func(tx *gorm.DB) error {
		var participant models.MeetingParticipant
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("meeting_id = ? AND user_id = ? AND user_type = ?", meeting.ID, userID, userType).
			First(&participant).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil || participant.JoinedAt == nil || participant.LeftAt != nil {
			return err
		}
		if leftAt.Before(*participant.JoinedAt) {
			leftAt = *participant.JoinedAt
		}
		return tx.Model(&models.MeetingParticipant{}).Where("id = ?", participant.ID).Updates(map[string]any{
			"left_at":    leftAt,
			"duration":   participantDurationSeconds(*participant.JoinedAt, leftAt),
			"updated_at": time.Now(),
		}).Error
	})
}

func (s *meetingService) activateMeetingFromAttendance(tx *gorm.DB, meetingID string) error {
	attendance := repositories.MeetingRoomRepository.AttendanceStats(tx, meetingID)
	if attendance.FirstJoinedAt == nil {
		return nil
	}
	var meeting models.MeetingRoomJitsi
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&meeting, "id = ?", meetingID).Error; err != nil {
		return err
	}
	if isMeetingEndedStatus(meeting.Status) {
		return nil
	}
	startedAt := *attendance.FirstJoinedAt
	if meeting.StartedAt != nil && meeting.StartedAt.Before(startedAt) {
		startedAt = *meeting.StartedAt
	}
	return tx.Model(&models.MeetingRoomJitsi{}).
		Where("id = ? AND status NOT IN ?", meetingID, []string{"ended", "finished"}).
		Updates(map[string]any{
			"status":     "active",
			"started_at": startedAt,
			"updated_at": time.Now(),
		}).Error
}

func confirmedMeetingStartedAt(meeting *models.MeetingRoomJitsi, attendance repositories.MeetingAttendanceStats) *time.Time {
	if attendance.Count <= 0 {
		return nil
	}
	if meeting != nil && meeting.StartedAt != nil &&
		(attendance.FirstJoinedAt == nil || !meeting.StartedAt.After(*attendance.FirstJoinedAt)) {
		startedAt := *meeting.StartedAt
		return &startedAt
	}
	if attendance.FirstJoinedAt == nil {
		return nil
	}
	startedAt := *attendance.FirstJoinedAt
	return &startedAt
}

func (s *meetingService) mergeWebhookParticipantJoin(tx *gorm.DB, meetingID, userID, userName string, joinedAt time.Time, meetingEndedAt *time.Time) error {
	var existing models.MeetingParticipant
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("meeting_id = ? AND user_id = ?", meetingID, userID).
		Order("CASE WHEN user_type = 'external' THEN 1 ELSE 0 END ASC").
		Order("created_at ASC").
		First(&existing).Error
	if err == nil {
		updates := map[string]any{"updated_at": time.Now()}
		if userName != "" {
			updates["participant_name"] = userName
		}
		switch {
		case existing.LeftAt != nil && joinedAt.After(*existing.LeftAt):
			updates["joined_at"] = joinedAt
			updates["left_at"] = nil
			updates["duration"] = 0
		case existing.LeftAt != nil:
			// A delayed join belongs to the already closed attendance window.
			// Keep the participant offline and only refine that window.
			if existing.JoinedAt == nil || joinedAt.After(*existing.JoinedAt) {
				updates["joined_at"] = joinedAt
				updates["duration"] = participantDurationSeconds(joinedAt, *existing.LeftAt)
			}
		case existing.JoinedAt == nil || existing.UserType != "external":
			// Authenticated users are pre-registered before the provider sees
			// them; the provider timestamp is the authoritative join time.
			updates["joined_at"] = joinedAt
		}
		if meetingEndedAt != nil {
			updates["left_at"] = *meetingEndedAt
			windowStart := joinedAt
			if value, ok := updates["joined_at"].(time.Time); ok {
				windowStart = value
			} else if existing.JoinedAt != nil {
				windowStart = *existing.JoinedAt
			}
			updates["duration"] = participantDurationSeconds(windowStart, *meetingEndedAt)
		}
		return tx.Model(&models.MeetingParticipant{}).Where("id = ?", existing.ID).Updates(updates).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	participant := &models.MeetingParticipant{
		ID:              utils.UUID(),
		MeetingID:       meetingID,
		UserID:          userID,
		UserType:        "external",
		ParticipantName: userName,
		Role:            "participant",
		JoinedAt:        &joinedAt,
		BaseModel: models.BaseModel{
			CreatedAt: joinedAt,
			UpdatedAt: time.Now(),
		},
	}
	if meetingEndedAt != nil {
		endedAt := *meetingEndedAt
		participant.LeftAt = &endedAt
		participant.Duration = participantDurationSeconds(joinedAt, endedAt)
	}
	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(participant)
	if result.Error != nil || result.RowsAffected == 1 {
		return result.Error
	}
	return s.mergeWebhookParticipantJoin(tx, meetingID, userID, userName, joinedAt, meetingEndedAt)
}

func (s *meetingService) RecordParticipantLeftFromWebhook(ctx context.Context, meetingID, roomName, userID string, occurredAt time.Time) error {
	if strings.TrimSpace(userID) == "" {
		return errorsx.InvalidParam("jitsi participant id is required")
	}
	meeting, err := s.resolveWebhookMeeting(meetingID, roomName)
	if err != nil {
		return err
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}
	if meeting.EndedAt != nil && occurredAt.After(*meeting.EndedAt) {
		occurredAt = *meeting.EndedAt
	}
	return s.recordWebhookParticipantLeave(meeting.ID, strings.TrimSpace(userID), occurredAt)
}

func (s *meetingService) recordWebhookParticipantLeave(meetingID, userID string, leftAt time.Time) error {
	return sqls.DB().Transaction(func(tx *gorm.DB) error {
		var participant models.MeetingParticipant
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("meeting_id = ? AND user_id = ?", meetingID, userID).
			Order("CASE WHEN user_type = 'external' THEN 1 ELSE 0 END ASC").
			Order("created_at ASC").
			First(&participant).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			item := &models.MeetingParticipant{
				ID:        utils.UUID(),
				MeetingID: meetingID,
				UserID:    userID,
				UserType:  "external",
				Role:      "participant",
				LeftAt:    &leftAt,
				BaseModel: models.BaseModel{CreatedAt: leftAt, UpdatedAt: time.Now()},
			}
			return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(item).Error
		}
		if err != nil {
			return err
		}
		if participant.LeftAt != nil && !leftAt.Before(*participant.LeftAt) {
			return nil
		}
		if participant.JoinedAt != nil && leftAt.Before(*participant.JoinedAt) {
			// This is a late leave for an older attendance window; it must not
			// close a newer active join.
			return nil
		}
		return tx.Model(&models.MeetingParticipant{}).Where("id = ?", participant.ID).Updates(map[string]any{
			"left_at":    leftAt,
			"duration":   participantDurationFromPointers(participant.JoinedAt, leftAt),
			"updated_at": time.Now(),
		}).Error
	})
}

func participantDurationFromPointers(joinedAt *time.Time, leftAt time.Time) int64 {
	if joinedAt == nil {
		return 0
	}
	return participantDurationSeconds(*joinedAt, leftAt)
}

func participantDurationSeconds(joinedAt, leftAt time.Time) int64 {
	if !leftAt.After(joinedAt) {
		return 0
	}
	return int64(leftAt.Sub(joinedAt).Seconds())
}

func (s *meetingService) resolveWebhookMeeting(meetingID, roomName string) (*models.MeetingRoomJitsi, error) {
	meetingID = strings.TrimSpace(meetingID)
	roomNames := jitsiRoomLookupNames(roomName)
	var meeting models.MeetingRoomJitsi
	var err error
	if meetingID != "" {
		err = sqls.DB().Where("id = ?", meetingID).First(&meeting).Error
		if err == nil {
			if len(roomNames) > 0 && !jitsiRoomCandidateMatches(roomNames, meeting.RoomName) {
				return nil, errorsx.InvalidParam("jitsi meeting id and room do not match")
			}
			return &meeting, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	for _, candidate := range roomNames {
		err = sqls.DB().Where("room_name = ?", candidate).Order("created_at DESC").First(&meeting).Error
		if err == nil {
			return &meeting, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	return nil, errorsx.InvalidParam("jitsi meeting not found")
}

func jitsiRoomCandidateMatches(candidates []string, roomName string) bool {
	roomName = strings.TrimSpace(roomName)
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == roomName {
			return true
		}
	}
	return false
}

func jitsiRoomLookupNames(value string) []string {
	original := strings.TrimSpace(value)
	if original == "" {
		return nil
	}
	candidates := []string{original}
	normalized := original
	if index := strings.IndexByte(normalized, '/'); index >= 0 {
		normalized = normalized[:index]
	}
	if index := strings.IndexByte(normalized, '@'); index >= 0 {
		normalized = normalized[:index]
	}
	normalized = strings.TrimSpace(normalized)
	if normalized != "" && normalized != original {
		candidates = append(candidates, normalized)
	}
	return candidates
}

// MeetingPreviewResult 会议预览结果
type MeetingPreviewResult struct {
	RoomName  string
	JoinURL   string
	ExpiresAt time.Time
}

// Preview 预览会议（创建临时会议室，用于管理员预览配置效果）
func (s *meetingService) Preview(req request.MeetingPreviewCreateRequest) (*MeetingPreviewResult, error) {
	product := ProductService.Get(req.ProductID)
	if product == nil || product.TenantID != req.TenantID || product.Status != enums.StatusOk {
		return nil, errorsx.InvalidParam("product does not exist")
	}
	profile := repositories.ProductServiceProfileRepository.GetByProductID(sqls.DB(), req.ProductID)
	if profile == nil || profile.TenantID != req.TenantID || profile.Status != enums.StatusOk || !profile.MeetingEnabled {
		return nil, errorsx.InvalidParam("meeting is disabled for product")
	}
	cfg := repositories.TenantIntegrationConfigRepository.GetByProvider(sqls.DB(), req.TenantID, "jitsi")
	if cfg == nil || cfg.Status != enums.StatusOk || !cfg.Enabled || strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, errorsx.InvalidParam("jitsi integration is not enabled")
	}
	base := strings.TrimRight(cfg.BaseURL, "/")
	room := "rhdp" + strings.ReplaceAll(uuid.NewString(), "-", "")
	expires := time.Now().Add(time.Hour)
	result := &MeetingPreviewResult{RoomName: room, JoinURL: base + "/" + url.PathEscape(room), ExpiresAt: expires}
	item := &models.MeetingRoom{
		TenantID:     req.TenantID,
		ProductID:    req.ProductID,
		Provider:     "jitsi",
		RoomName:     room,
		JoinURL:      result.JoinURL,
		BusinessType: strings.TrimSpace(req.BusinessType),
		BusinessID:   req.BusinessID,
		ExpiresAt:    expires,
		Status:       enums.StatusOk,
		AuditFields:  utils.BuildAuditFields(nil),
	}
	if err := repositories.MeetingRoomRepository.Create(sqls.DB(), item); err != nil {
		return nil, err
	}
	return result, nil
}
