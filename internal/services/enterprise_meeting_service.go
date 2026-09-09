package services

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/providers"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func (s *meetingService) ListPersonalMeetings(ctx context.Context, tenantID, userID int64, status string) (*dto.EnterpriseMeetingListDTO, error) {
	return s.listPersonalMeetings(ctx, tenantID, userID, status, "", 1, 100, nil)
}

func (s *meetingService) ListPersonalMeetingsForOperator(ctx context.Context, tenantID int64, status string, operator *dto.AuthPrincipal) (*dto.EnterpriseMeetingListDTO, error) {
	return s.ListPersonalMeetingsPageForOperator(ctx, tenantID, status, 1, 100, operator)
}

func (s *meetingService) ListPersonalMeetingsPageForOperator(ctx context.Context, tenantID int64, status string, page, pageSize int, operator *dto.AuthPrincipal) (*dto.EnterpriseMeetingListDTO, error) {
	return s.ListPersonalMeetingsSearchPageForOperator(ctx, tenantID, status, "", page, pageSize, operator)
}

func (s *meetingService) ListPersonalMeetingsSearchPageForOperator(ctx context.Context, tenantID int64, status, keyword string, page, pageSize int, operator *dto.AuthPrincipal, deviceIDs ...int64) (*dto.EnterpriseMeetingListDTO, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	visibleUserID := operator.UserID
	if canViewAllTenantMeetings(operator) {
		visibleUserID = 0
	}
	return s.listPersonalMeetings(ctx, tenantID, visibleUserID, status, keyword, page, pageSize, operator, firstPositiveMeetingFilterID(deviceIDs...))
}

func canViewAllTenantMeetings(operator *dto.AuthPrincipal) bool {
	if operator == nil {
		return false
	}
	platformTenantSupport := operator.IsEnterprise() && operator.SupportGrantID > 0 &&
		operator.TargetTenantID > 0 && strings.TrimSpace(operator.ImpersonatedBy) != ""
	return platformTenantSupport || operator.IsPlatform() ||
		operator.HasRole(EnterpriseRoleOwner) ||
		operator.HasRole(EnterpriseRoleAdmin) ||
		operator.HasRole(EnterpriseRoleServiceManager)
}

func (s *meetingService) listPersonalMeetings(ctx context.Context, tenantID, userID int64, status, keyword string, page, pageSize int, operator *dto.AuthPrincipal, deviceIDs ...int64) (*dto.EnterpriseMeetingListDTO, error) {
	if tenantID <= 0 {
		return nil, nil
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 100
	}
	db := sqls.DB()
	s.ReconcileStaleMeetingPresenceForTenant(ctx, tenantID, 100)
	participantMeetingIDs := s.personalMeetingIDSet(userID)
	participantIDs := make([]string, 0, len(participantMeetingIDs))
	for meetingID := range participantMeetingIDs {
		participantIDs = append(participantIDs, meetingID)
	}
	deviceID := firstPositiveMeetingFilterID(deviceIDs...)
	visibleQuery := func() *gorm.DB {
		query := db.Model(&models.MeetingRoomJitsi{}).Where("tenant_id = ?", tenantID)
		if userID > 0 {
			if len(participantIDs) > 0 {
				query = query.Where("created_by = ? OR id IN ?", fmt.Sprint(userID), participantIDs)
			} else {
				query = query.Where("created_by = ?", fmt.Sprint(userID))
			}
		}
		return applyEnterpriseMeetingDeviceFilter(query, db, tenantID, deviceID)
	}

	summary := dto.EnterpriseMeetingSummaryDTO{}
	visibleQuery().Where("status = ?", "active").Count(&summary.Active)
	visibleQuery().Where("status IN ?", []string{"waiting", "scheduled"}).Count(&summary.Waiting)
	visibleQuery().Where("status = ?", "ended").Count(&summary.Ended)
	visibleQuery().Count(&summary.Mine)
	activeMeetingIDs := make([]string, 0)
	visibleQuery().Where("status = ?", "active").Pluck("id", &activeMeetingIDs)
	summary.ParticipantsOnline = repositories.MeetingRoomRepository.CountOnlineParticipants(db, activeMeetingIDs)

	var meetings []models.MeetingRoomJitsi
	normalizedStatus := normalizeEnterpriseMeetingStatus(status)
	listQuery := func() *gorm.DB {
		query := visibleQuery()
		if normalizedStatus == "waiting" {
			query = query.Where("status IN ?", []string{"waiting", "scheduled"})
		} else if normalizedStatus != "" {
			query = query.Where("status = ?", normalizedStatus)
		}
		return applyEnterpriseMeetingSearch(query, db, tenantID, keyword)
	}
	var total int64
	listQuery().Count(&total)
	if err := listQuery().Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&meetings).Error; err != nil {
		return nil, err
	}

	items := make([]dto.EnterpriseMeetingListItemDTO, 0, len(meetings))
	meetingIDs := make([]string, 0, len(meetings))
	for i := range meetings {
		meetingIDs = append(meetingIDs, meetings[i].ID)
	}
	attendanceByMeeting := repositories.MeetingRoomRepository.AttendanceStatsByMeetingIDs(db, meetingIDs)
	cache := newEnterpriseMeetingBuildCache()
	cache.participants, cache.archiveStats = loadMeetingArchiveData(db, meetingIDs)
	for _, meeting := range meetings {
		attendance := attendanceByMeeting[meeting.ID]
		item := s.buildEnterpriseMeetingListItem(meeting, operator, &attendance, cache)
		items = append(items, item)
	}
	return &dto.EnterpriseMeetingListDTO{
		Summary:  summary,
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  int64(page*pageSize) < total,
	}, nil
}

func applyEnterpriseMeetingSearch(query, db *gorm.DB, tenantID int64, keyword string) *gorm.DB {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return query
	}
	pattern := "%" + strings.ToLower(keyword) + "%"
	productIDs := db.Model(&models.Product{}).
		Select("id").
		Where("tenant_id = ? AND LOWER(name) LIKE ?", tenantID, pattern)
	deviceIDs := db.Model(&models.Device{}).
		Select("id").
		Where("tenant_id = ? AND LOWER(device_no) LIKE ?", tenantID, pattern)
	customerIDs := db.Model(&models.Customer{}).
		Select("id").
		Where("LOWER(name) LIKE ?", pattern)
	ticketIDs := db.Model(&models.Ticket{}).
		Select("CAST(id AS TEXT)").
		Where("tenant_id = ?", tenantID).
		Where(
			"LOWER(ticket_no) LIKE ? OR LOWER(title) LIKE ? OR product_id IN (?) OR device_id IN (?) OR customer_id IN (?)",
			pattern, pattern, productIDs, deviceIDs, customerIDs,
		)
	return query.Where("LOWER(room_name) LIKE ? OR ticket_id IN (?)", pattern, ticketIDs)
}

func applyEnterpriseMeetingDeviceFilter(query, db *gorm.DB, tenantID, deviceID int64) *gorm.DB {
	if query == nil || db == nil || tenantID <= 0 || deviceID <= 0 {
		return query
	}
	ticketIDs := db.Model(&models.Ticket{}).
		Select("CAST(id AS TEXT)").
		Where("tenant_id = ? AND device_id = ?", tenantID, deviceID)
	return query.Where("ticket_id IN (?)", ticketIDs)
}

func firstPositiveMeetingFilterID(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func (s *meetingService) ListTicketMeetings(ctx context.Context, tenantID, ticketID int64) ([]dto.EnterpriseMeetingListItemDTO, error) {
	return s.listTicketMeetings(ctx, tenantID, ticketID, nil)
}

func (s *meetingService) listTicketMeetings(ctx context.Context, tenantID, ticketID int64, operator *dto.AuthPrincipal) ([]dto.EnterpriseMeetingListItemDTO, error) {
	if tenantID <= 0 || ticketID <= 0 {
		return nil, nil
	}
	ticket := repositories.TicketRepository.Get(sqls.DB(), ticketID)
	if ticket == nil || ticket.TenantID != tenantID {
		return nil, nil
	}
	meetings := repositories.MeetingRoomRepository.FindJitsiByTicketIDs(
		sqls.DB(), tenantID, []string{strconv.FormatInt(ticketID, 10)},
	)
	items := make([]dto.EnterpriseMeetingListItemDTO, 0, len(meetings))
	meetingIDs := make([]string, 0, len(meetings))
	for i := range meetings {
		meetingIDs = append(meetingIDs, meetings[i].ID)
	}
	attendanceByMeeting := repositories.MeetingRoomRepository.AttendanceStatsByMeetingIDs(sqls.DB(), meetingIDs)
	cache := newEnterpriseMeetingBuildCache()
	cache.participants, cache.archiveStats = loadMeetingArchiveData(sqls.DB(), meetingIDs)
	for _, meeting := range meetings {
		attendance := attendanceByMeeting[meeting.ID]
		items = append(items, s.buildEnterpriseMeetingListItem(meeting, operator, &attendance, cache))
	}
	return items, nil
}

func (s *meetingService) ListTicketMeetingsForOperator(ctx context.Context, ticketID int64, operator *dto.AuthPrincipal) ([]dto.EnterpriseMeetingListItemDTO, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if ticketID <= 0 {
		return nil, errorsx.InvalidParam("invalid ticket id")
	}
	ticket := repositories.TicketRepository.Get(sqls.DB(), ticketID)
	if err := requireTicketTenantAccess(ticket, operator); err != nil {
		return nil, err
	}
	return s.listTicketMeetings(ctx, ticket.TenantID, ticketID, operator)
}

func (s *meetingService) personalMeetingIDSet(userID int64) map[string]bool {
	ret := make(map[string]bool)
	if userID <= 0 {
		return ret
	}
	var participants []models.MeetingParticipant
	sqls.DB().Where("user_id = ? AND user_type IN ?", fmt.Sprint(userID), meetingEnterpriseParticipantTypes()).Find(&participants)
	for _, item := range participants {
		ret[item.MeetingID] = true
	}
	return ret
}

type enterpriseMeetingBuildCache struct {
	tickets      map[int64]*models.Ticket
	products     map[int64]*models.Product
	devices      map[int64]*models.Device
	customers    map[int64]*models.Customer
	users        map[int64]*models.User
	participants map[string][]dto.MeetingParticipantDTO
	archiveStats map[string]repositories.MeetingArchiveStats
}

func newEnterpriseMeetingBuildCache() *enterpriseMeetingBuildCache {
	return &enterpriseMeetingBuildCache{
		tickets: make(map[int64]*models.Ticket), products: make(map[int64]*models.Product),
		devices: make(map[int64]*models.Device), customers: make(map[int64]*models.Customer), users: make(map[int64]*models.User),
		participants: make(map[string][]dto.MeetingParticipantDTO), archiveStats: make(map[string]repositories.MeetingArchiveStats),
	}
}

func (s *meetingService) buildEnterpriseMeetingListItem(
	meeting models.MeetingRoomJitsi,
	operator *dto.AuthPrincipal,
	attendance *repositories.MeetingAttendanceStats,
	cache *enterpriseMeetingBuildCache,
) dto.EnterpriseMeetingListItemDTO {
	if cache == nil {
		cache = newEnterpriseMeetingBuildCache()
	}
	ticketID, _ := strconv.ParseInt(meeting.TicketID, 10, 64)
	ticketNo := ""
	productName := ""
	deviceNo := ""
	customerName := ""
	canEnd := false
	title := "视频协作"
	if ticketID > 0 {
		ticket, loaded := cache.tickets[ticketID]
		if !loaded {
			ticket = repositories.TicketRepository.Get(sqls.DB(), ticketID)
			cache.tickets[ticketID] = ticket
		}
		if ticket != nil {
			ticketNo = ticket.TicketNo
			if ticket.Title != "" {
				title = ticket.Title
			}
			if ticket.ProductID > 0 {
				product, loaded := cache.products[ticket.ProductID]
				if !loaded {
					product = repositories.ProductRepository.Get(sqls.DB(), ticket.ProductID)
					cache.products[ticket.ProductID] = product
				}
				if product != nil {
					productName = product.Name
				}
			}
			if ticket.DeviceID > 0 {
				device, loaded := cache.devices[ticket.DeviceID]
				if !loaded {
					device = repositories.DeviceRepository.Get(sqls.DB(), ticket.DeviceID)
					cache.devices[ticket.DeviceID] = device
				}
				if device != nil {
					deviceNo = device.DeviceNo
				}
			}
			if ticket.CustomerID > 0 {
				customer, loaded := cache.customers[ticket.CustomerID]
				if !loaded {
					customer = repositories.CustomerRepository.Get(sqls.DB(), ticket.CustomerID)
					cache.customers[ticket.CustomerID] = customer
				}
				if customer != nil {
					customerName = customer.Name
				}
			}
			canEnd = meeting.Status != "ended" && operator != nil &&
				(ticket.CurrentAssigneeID == operator.UserID || canManageTicketDispatch(operator))
		}
	}

	if attendance == nil {
		value := repositories.MeetingRoomRepository.AttendanceStats(sqls.DB(), meeting.ID)
		attendance = &value
	}
	confirmedStartedAt := confirmedMeetingStartedAt(&meeting, *attendance)
	startedAt := formatEnterpriseTimePtr(confirmedStartedAt)
	duration := meetingDurationSeconds(confirmedStartedAt, meeting.EndedAt)
	createdBy := meeting.CreatedBy
	if userID, err := strconv.ParseInt(meeting.CreatedBy, 10, 64); err == nil && userID > 0 {
		user, loaded := cache.users[userID]
		if !loaded {
			user = repositories.UserRepository.Get(sqls.DB(), userID)
			cache.users[userID] = user
		}
		if user != nil {
			createdBy = firstNonEmptyString(user.Nickname, user.Username, meeting.CreatedBy)
		}
	}

	jitsiBase := strings.TrimRight(providers.DefaultJitsiProvider.GetJitsiPublicURL(), "/")
	participants := cache.participants[meeting.ID]
	if participants == nil {
		participants = []dto.MeetingParticipantDTO{}
	}
	archiveStats := cache.archiveStats[meeting.ID]
	return dto.EnterpriseMeetingListItemDTO{
		ID:               meeting.ID,
		TenantID:         meeting.TenantID,
		TicketID:         ticketID,
		TicketNo:         ticketNo,
		Title:            title,
		RoomName:         meeting.RoomName,
		Status:           mapEnterpriseMeetingStatus(meeting.Status),
		ProductName:      productName,
		DeviceNo:         deviceNo,
		CustomerName:     customerName,
		CreatedBy:        createdBy,
		ScheduledAt:      formatEnterpriseTimePtr(meeting.ScheduledAt),
		StartedAt:        startedAt,
		EndedAt:          formatEnterpriseTimePtr(meeting.EndedAt),
		CreatedAt:        formatEnterpriseTime(meeting.CreatedAt),
		DurationSeconds:  duration,
		ParticipantCount: attendance.Count,
		Participants:     participants,
		TranscriptCount:  archiveStats.TranscriptCount,
		AnnotationCount:  archiveStats.AnnotationCount,
		CanEnd:           canEnd,
		JoinPath:         fmt.Sprintf("/api/enterprise/v1/meetings/%s/join", meeting.ID),
		JitsiURL:         jitsiBase + "/" + meeting.RoomName,
	}
}

func meetingDurationSeconds(startedAt, endedAt *time.Time) int64 {
	if startedAt == nil {
		return 0
	}
	end := time.Now()
	if endedAt != nil {
		end = *endedAt
	}
	if end.Before(*startedAt) {
		return 0
	}
	return int64(end.Sub(*startedAt).Seconds())
}

func normalizeEnterpriseMeetingStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "", "all":
		return ""
	case "active":
		return "active"
	case "waiting", "scheduled":
		return "waiting"
	case "finished", "ended":
		return "ended"
	default:
		return strings.TrimSpace(status)
	}
}

func mapEnterpriseMeetingStatus(status string) string {
	switch status {
	case "active":
		return "active"
	case "ended":
		return "finished"
	default:
		return "waiting"
	}
}
