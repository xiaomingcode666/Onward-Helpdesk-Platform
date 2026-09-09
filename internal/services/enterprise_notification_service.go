package services

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/routes"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var EnterpriseNotificationService = newEnterpriseNotificationService()

func newEnterpriseNotificationService() *enterpriseNotificationService {
	return &enterpriseNotificationService{}
}

type enterpriseNotificationService struct{}

var legacyNotificationInt64FormatPattern = regexp.MustCompile(`%!s\(int64=([0-9]+)\)`)

type EnterpriseNotificationQuery struct {
	ReadStatus string
	Category   string // ticket / sla / approval / quota / knowledge / video / system;空或 all 不过滤
	Search     string
	Scope      string // personal / tenant;无租户级查看权限时强制降级为 personal
	Page       int
	PageSize   int
	Limit      int // 兼容工作台等旧调用，优先级低于 PageSize
}

type enterpriseNotificationViewer struct {
	UserID     int64
	TenantWide bool
}

// scopeCnd 企业端可见范围:租户内有效通知,且属于当前用户或为租户广播(recipient_user_id=0)。
func (s *enterpriseNotificationService) scopeCnd(tenantID, userID int64) *sqls.Cnd {
	return s.viewerScopeCnd(tenantID, enterpriseNotificationViewer{UserID: userID})
}

func (s *enterpriseNotificationService) viewerScopeCnd(tenantID int64, viewer enterpriseNotificationViewer) *sqls.Cnd {
	cnd := sqls.NewCnd().Eq("tenant_id", tenantID).Eq("status", enums.StatusOk)
	if viewer.TenantWide {
		return cnd
	}
	if viewer.UserID > 0 {
		cnd.Where("recipient_user_id = ? OR recipient_user_id = 0", viewer.UserID)
	} else {
		// Internal/platform support principals without an enterprise user identity
		// may only see tenant broadcasts, never another member's personal messages.
		cnd.Eq("recipient_user_id", 0)
	}
	return cnd
}

func (s *enterpriseNotificationService) List(tenantID, userID int64, query EnterpriseNotificationQuery) (*dto.EnterpriseNotificationListDTO, error) {
	return s.list(tenantID, enterpriseNotificationViewer{UserID: userID}, false, query)
}

func (s *enterpriseNotificationService) ListForOperator(tenantID int64, operator *dto.AuthPrincipal, query EnterpriseNotificationQuery) (*dto.EnterpriseNotificationListDTO, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	canViewTenantScope := canViewAllTenantNotifications(operator)
	viewer := enterpriseNotificationViewer{UserID: operator.UserID}
	viewer.TenantWide = canViewTenantScope && strings.TrimSpace(query.Scope) == "tenant"
	return s.list(tenantID, viewer, canViewTenantScope, query)
}

func canViewAllTenantNotifications(operator *dto.AuthPrincipal) bool {
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

func (s *enterpriseNotificationService) list(tenantID int64, viewer enterpriseNotificationViewer, canViewTenantScope bool, query EnterpriseNotificationQuery) (*dto.EnterpriseNotificationListDTO, error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant is required")
	}
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = query.Limit
	}
	if query.PageSize <= 0 {
		query.PageSize = 50
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}

	var (
		items         []dto.EnterpriseNotificationDTO
		filteredTotal int64
	)
	if viewer.TenantWide {
		events, total, err := repositories.NotificationRepository.FindTenantEventPage(sqls.DB(), repositories.TenantNotificationEventFilter{
			TenantID:   tenantID,
			Category:   query.Category,
			ReadStatus: query.ReadStatus,
			Search:     query.Search,
			Page:       query.Page,
			PageSize:   query.PageSize,
		})
		if err != nil {
			return nil, err
		}
		filteredTotal = total
		items = make([]dto.EnterpriseNotificationDTO, 0, len(events))
		for i := range events {
			items = append(items, buildEnterpriseNotificationEvent(events[i]))
		}
	} else {
		cnd := s.viewerScopeCnd(tenantID, viewer)
		if query.Category != "" && query.Category != "all" {
			cnd.Eq("category", query.Category)
		}
		switch query.ReadStatus {
		case "unread":
			cnd.Where("read_at IS NULL")
		case "read":
			cnd.Where("read_at IS NOT NULL")
		}
		if search := strings.TrimSpace(query.Search); search != "" {
			keyword := "%" + search + "%"
			cnd.Where("title LIKE ? OR content LIKE ? OR recipient_name LIKE ? OR biz_type LIKE ?", keyword, keyword, keyword, keyword)
		}
		filteredTotal = repositories.NotificationRepository.Count(sqls.DB(), cnd)
		cnd.Desc("created_at").Desc("id").Page(query.Page, query.PageSize)
		list := repositories.NotificationRepository.Find(sqls.DB(), cnd)
		items = make([]dto.EnterpriseNotificationDTO, 0, len(list))
		for _, item := range list {
			items = append(items, buildEnterpriseNotification(item, viewer.UserID))
		}
	}
	summary := s.summary(tenantID, viewer)
	return &dto.EnterpriseNotificationListDTO{
		Summary:            summary,
		Items:              items,
		Total:              filteredTotal,
		Page:               query.Page,
		PageSize:           query.PageSize,
		HasMore:            int64(query.Page*query.PageSize) < filteredTotal,
		Scope:              notificationViewerScope(viewer),
		CanViewTenantScope: canViewTenantScope,
	}, nil
}

func notificationViewerScope(viewer enterpriseNotificationViewer) string {
	if viewer.TenantWide {
		return "tenant"
	}
	return "personal"
}

// Summary 消息中心概览:总数、未读、未读中紧急、已启用邮件的消息数、待发邮件数、今日消息数。
func (s *enterpriseNotificationService) Summary(tenantID, userID int64) dto.EnterpriseNotificationSummaryDTO {
	return s.summary(tenantID, enterpriseNotificationViewer{UserID: userID})
}

func (s *enterpriseNotificationService) summary(tenantID int64, viewer enterpriseNotificationViewer) dto.EnterpriseNotificationSummaryDTO {
	deliveryTotal := repositories.NotificationRepository.Count(sqls.DB(), s.viewerScopeCnd(tenantID, viewer))
	deliveryUnread := repositories.NotificationRepository.Count(sqls.DB(), s.viewerScopeCnd(tenantID, viewer).Where("read_at IS NULL"))
	markableUnreadCount := int64(0)
	if viewer.UserID > 0 {
		markableUnread := s.scopeCnd(tenantID, viewer.UserID).Where("read_at IS NULL")
		markableUnreadCount = repositories.NotificationRepository.Count(sqls.DB(), markableUnread)
	}
	emailEnabled := s.viewerScopeCnd(tenantID, viewer).Where("channels LIKE '%email%'")
	pendingEmail := s.viewerScopeCnd(tenantID, viewer).
		Where("channels LIKE '%email%'").
		Where("external_channel_status NOT IN ('email_sent', 'email_failed', 'email_unavailable', 'email_skipped')")
	failedEmail := s.viewerScopeCnd(tenantID, viewer).
		Where("channels LIKE '%email%'").
		Where("external_channel_status IN ('email_failed', 'email_unavailable')")

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	total := deliveryTotal
	unread := deliveryUnread
	urgent := repositories.NotificationRepository.Count(sqls.DB(), s.viewerScopeCnd(tenantID, viewer).Where("read_at IS NULL").Eq("level", "urgent"))
	today := repositories.NotificationRepository.Count(sqls.DB(), s.viewerScopeCnd(tenantID, viewer).Where("created_at >= ?", todayStart))
	if viewer.TenantWide {
		if eventSummary, err := repositories.NotificationRepository.SummarizeTenantEvents(sqls.DB(), tenantID, todayStart); err == nil {
			total = eventSummary.Total
			unread = eventSummary.Unread
			urgent = eventSummary.Urgent
			today = eventSummary.Today
		}
	}

	return dto.EnterpriseNotificationSummaryDTO{
		Total:          total,
		Unread:         unread,
		DeliveryTotal:  deliveryTotal,
		DeliveryUnread: deliveryUnread,
		MarkableUnread: markableUnreadCount,
		Urgent:         urgent,
		EmailEnabled:   repositories.NotificationRepository.Count(sqls.DB(), emailEnabled),
		PendingEmail:   repositories.NotificationRepository.Count(sqls.DB(), pendingEmail),
		FailedEmail:    repositories.NotificationRepository.Count(sqls.DB(), failedEmail),
		Today:          today,
	}
}

func (s *enterpriseNotificationService) MarkRead(tenantID, userID, id int64) error {
	if tenantID <= 0 || id <= 0 {
		return errorsx.InvalidParam("tenant and notification are required")
	}
	item := repositories.NotificationRepository.Get(sqls.DB(), id)
	if userID <= 0 || item == nil || item.TenantID != tenantID || item.Status != int(enums.StatusOk) || (item.RecipientUserID != userID && item.RecipientUserID != 0) {
		return errorsx.InvalidParam("notification not found")
	}
	if item.ReadAt != nil {
		return nil
	}
	return repositories.NotificationRepository.Updates(sqls.DB(), id, map[string]any{
		"read_at": time.Now(),
	})
}

// MarkAllRead 将租户内当前用户可见的全部未读通知标记已读。
func (s *enterpriseNotificationService) MarkAllRead(tenantID, userID int64) error {
	if tenantID <= 0 || userID <= 0 {
		return errorsx.InvalidParam("tenant and user are required")
	}
	return repositories.NotificationRepository.MarkAllReadByTenant(sqls.DB(), tenantID, userID, time.Now())
}

func buildEnterpriseNotification(item models.Notification, viewerUserID int64) dto.EnterpriseNotificationDTO {
	channels := parseNotificationChannels(item.Channels)
	return dto.EnterpriseNotificationDTO{
		ID:                    item.ID,
		TenantID:              item.TenantID,
		RecipientUserID:       item.RecipientUserID,
		RecipientName:         item.RecipientName,
		Title:                 item.Title,
		Content:               normalizeLegacyNotificationContent(item),
		NotificationType:      item.NotificationType,
		BizType:               item.BizType,
		BizID:                 item.BizID,
		Category:              item.Category,
		Level:                 item.Level,
		Channels:              channels,
		EmailStatus:           deriveEmailStatus(item, channels),
		ActionURL:             routes.NormalizeEnterpriseActionURL(item.ActionURL, item.BizType, item.BizID),
		DeliveryStatus:        item.DeliveryStatus,
		ExternalChannelStatus: item.ExternalChannelStatus,
		ReadAt:                formatEnterpriseTimePtr(item.ReadAt),
		CreatedAt:             formatEnterpriseTime(item.CreatedAt),
		CanMarkRead:           viewerUserID > 0 && (item.RecipientUserID == viewerUserID || item.RecipientUserID == 0),
		RecipientCount:        1,
		UnreadCount:           notificationUnreadCount(item),
	}
}

func buildEnterpriseNotificationEvent(event repositories.TenantNotificationEventAggregate) dto.EnterpriseNotificationDTO {
	ret := buildEnterpriseNotification(event.Item, 0)
	ret.IsEventSummary = true
	ret.RecipientCount = event.RecipientCount
	ret.UnreadCount = event.UnreadCount
	ret.EmailPendingCount = event.EmailPendingCount
	ret.EmailFailedCount = event.EmailFailedCount
	ret.RecipientName = fmt.Sprintf("%d 位接收人", event.RecipientCount)
	ret.CanMarkRead = false
	return ret
}

func notificationUnreadCount(item models.Notification) int64 {
	if item.ReadAt == nil {
		return 1
	}
	return 0
}

func normalizeLegacyNotificationContent(item models.Notification) string {
	if strings.TrimSpace(item.NotificationType) != "conversation_assigned" {
		return item.Content
	}
	return legacyNotificationInt64FormatPattern.ReplaceAllString(item.Content, "$1")
}

func parseNotificationChannels(raw string) []string {
	parts := strings.Split(raw, ",")
	channels := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			channels = append(channels, value)
		}
	}
	if len(channels) == 0 {
		return []string{"in_app"}
	}
	return channels
}

// deriveEmailStatus 邮件状态:未启用邮件渠道 → disabled;已发送 → sent;否则 pending(待发送)。
func deriveEmailStatus(item models.Notification, channels []string) string {
	emailEnabled := false
	for _, channel := range channels {
		if channel == "email" {
			emailEnabled = true
			break
		}
	}
	if !emailEnabled {
		return "disabled"
	}
	if item.ExternalChannelStatus == "email_skipped" {
		return "skipped"
	}
	if item.ExternalChannelStatus == "email_unavailable" || strings.Contains(item.ExternalChannelStatus, "failed") {
		return "failed"
	}
	if item.ExternalChannelStatus == "email_sent" {
		return "sent"
	}
	return "pending"
}
