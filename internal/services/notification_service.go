package services

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/routes"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var NotificationService = newNotificationService()

func newNotificationService() *notificationService {
	return &notificationService{}
}

type notificationService struct {
}

var notificationRecipientIdempotencySuffix = regexp.MustCompile(`:recipient:[0-9]+$`)

func (s *notificationService) Create(req request.CreateNotificationRequest) (*models.Notification, error) {
	item, _, err := s.create(req)
	return item, err
}

func (s *notificationService) create(req request.CreateNotificationRequest) (*models.Notification, bool, error) {
	if req.TenantID <= 0 {
		return nil, false, errorsx.InvalidParam("tenantId is required")
	}
	if req.RecipientUserID <= 0 {
		return nil, false, errorsx.InvalidParamI18n("error.e0212")
	}
	member := repositories.EnterpriseIAMRepository.FindTenantMemberByUserID(sqls.DB(), req.TenantID, req.RecipientUserID)
	if member == nil || member.Status != enums.StatusOk {
		return nil, false, errorsx.Forbidden("notification recipient is outside the tenant or inactive")
	}
	if err := validateNotificationResourceTenant(req); err != nil {
		return nil, false, err
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, false, errorsx.InvalidParam("notification title is required")
	}
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if existing := repositories.NotificationRepository.FindByIdempotencyKey(sqls.DB(), req.TenantID, idempotencyKey); existing != nil {
		return existing, false, nil
	}
	recipientName := strings.TrimSpace(req.RecipientName)
	if recipientName == "" {
		recipientName = strings.TrimSpace(member.DisplayName)
	}
	if recipientName == "" {
		if user := repositories.UserRepository.Get(sqls.DB(), req.RecipientUserID); user != nil {
			recipientName = strings.TrimSpace(user.Nickname)
			if recipientName == "" {
				recipientName = strings.TrimSpace(user.Username)
			}
		}
	}
	category, level := normalizeNotificationCategoryLevel(req.Category, req.Level, req.NotificationType, req.BizType)
	now := time.Now()
	item := &models.Notification{
		TenantID:         req.TenantID,
		EventKey:         notificationEventKey(idempotencyKey),
		RecipientUserID:  req.RecipientUserID,
		RecipientName:    recipientName,
		Title:            title,
		Content:          strings.TrimSpace(req.Content),
		NotificationType: strings.TrimSpace(req.NotificationType),
		BizType:          strings.TrimSpace(req.BizType),
		BizID:            req.BizID,
		ActionURL:        routes.NormalizeEnterpriseActionURL(req.ActionURL, req.BizType, req.BizID),
		Category:         category,
		Level:            level,
		Channels:         normalizeNotificationChannels(req.Channels),
		DeliveryStatus:   "sent",
		Status:           int(enums.StatusOk),
		CreatedAt:        now,
	}
	if idempotencyKey != "" {
		item.IdempotencyKey = &idempotencyKey
	}
	if err := repositories.NotificationRepository.Create(sqls.DB(), item); err != nil {
		if existing := repositories.NotificationRepository.FindByIdempotencyKey(sqls.DB(), req.TenantID, idempotencyKey); existing != nil {
			return existing, false, nil
		}
		return nil, false, err
	}
	return item, true, nil
}

func notificationEventKey(idempotencyKey string) string {
	if value := strings.TrimSpace(idempotencyKey); value != "" {
		return notificationRecipientIdempotencySuffix.ReplaceAllString(value, "")
	}
	return "notification:" + utils.UUID()
}

func (s *notificationService) CreateAndPush(req request.CreateNotificationRequest) (*models.Notification, error) {
	item, created, err := s.create(req)
	if err != nil {
		return nil, err
	}
	if !created {
		return item, nil
	}
	WsService.PublishNotificationCreated(item.RecipientUserID, response.NotificationResponse{
		ID:               item.ID,
		RecipientUserID:  item.RecipientUserID,
		Title:            item.Title,
		Content:          item.Content,
		NotificationType: item.NotificationType,
		BizType:          item.BizType,
		BizID:            item.BizID,
		ActionURL:        routes.NormalizeEnterpriseActionURL(item.ActionURL, item.BizType, item.BizID),
		ReadAt:           utils.FormatTimePtr(item.ReadAt),
		CreatedAt:        utils.FormatTime(item.CreatedAt),
	})
	return item, nil
}

// ReconcileTicketCreatedAfterAssignment removes stale "unassigned" wording
// when ticket creation and automatic assignment notifications race each other.
func (s *notificationService) ReconcileTicketCreatedAfterAssignment(ticket *models.Ticket) error {
	if ticket == nil || ticket.ID <= 0 || ticket.TenantID <= 0 || ticket.CurrentAssigneeID <= 0 {
		return nil
	}
	items := repositories.NotificationRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", ticket.TenantID).
		Eq("biz_type", "ticket").
		Eq("biz_id", ticket.ID).
		Eq("notification_type", "ticket_created").
		Eq("status", enums.StatusOk))
	if len(items) == 0 {
		return nil
	}

	ticketNo := strings.TrimSpace(ticket.TicketNo)
	if ticketNo == "" {
		ticketNo = fmt.Sprintf("#%d", ticket.ID)
	}
	assigneeName := notificationUserDisplayName(ticket.CurrentAssigneeID, "当前负责人")
	content := strings.TrimSpace(ticket.Title)
	if content != "" {
		content += "\n"
	}
	content += fmt.Sprintf("已分配给 %s，请按当前负责人继续处理。", assigneeName)
	for i := range items {
		if err := repositories.NotificationRepository.Updates(sqls.DB(), items[i].ID, map[string]any{
			"title":   fmt.Sprintf("工单 %s 已分配", ticketNo),
			"content": content,
			"level":   "info",
		}); err != nil {
			return err
		}
	}
	publishNotificationResyncForNotifications(items)
	return nil
}

// ReconcileTicketAssignedAfterReassignment closes the previous assignee's
// actionable assignment notification after a ticket is reassigned.
func (s *notificationService) ReconcileTicketAssignedAfterReassignment(ticket *models.Ticket, fromUserID, toUserID int64) error {
	if ticket == nil || ticket.ID <= 0 || ticket.TenantID <= 0 || fromUserID <= 0 || toUserID <= 0 || fromUserID == toUserID {
		return nil
	}
	items := repositories.NotificationRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", ticket.TenantID).
		Eq("biz_type", "ticket").
		Eq("biz_id", ticket.ID).
		Eq("notification_type", "ticket_assigned").
		Eq("recipient_user_id", fromUserID).
		Eq("status", enums.StatusOk))
	if len(items) == 0 {
		return nil
	}

	ticketNo := strings.TrimSpace(ticket.TicketNo)
	if ticketNo == "" {
		ticketNo = fmt.Sprintf("#%d", ticket.ID)
	}
	assigneeName := notificationUserDisplayName(toUserID, "新负责人")
	content := strings.TrimSpace(ticket.Title)
	if content != "" {
		content += "\n"
	}
	content += fmt.Sprintf("已转派给 %s，原负责人无需继续处理。", assigneeName)
	now := time.Now()
	for i := range items {
		updates := map[string]any{
			"title":   fmt.Sprintf("工单 %s 已转派", ticketNo),
			"content": content,
			"level":   "info",
		}
		if items[i].ReadAt == nil {
			updates["read_at"] = now
		}
		if err := repositories.NotificationRepository.Updates(sqls.DB(), items[i].ID, updates); err != nil {
			return err
		}
	}
	publishNotificationResyncForNotifications(items)
	return nil
}

// ReconcileTicketAssignedAfterAcceptance closes the accepting engineer's
// assignment notification after they confirm the ticket.
func (s *notificationService) ReconcileTicketAssignedAfterAcceptance(ticket *models.Ticket, assigneeID int64) error {
	if ticket == nil || ticket.ID <= 0 || ticket.TenantID <= 0 || assigneeID <= 0 {
		return nil
	}
	items := repositories.NotificationRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", ticket.TenantID).
		Eq("biz_type", "ticket").
		Eq("biz_id", ticket.ID).
		Eq("notification_type", "ticket_assigned").
		Eq("recipient_user_id", assigneeID).
		Eq("status", enums.StatusOk))
	if len(items) == 0 {
		return nil
	}

	ticketNo := strings.TrimSpace(ticket.TicketNo)
	if ticketNo == "" {
		ticketNo = fmt.Sprintf("#%d", ticket.ID)
	}
	content := strings.TrimSpace(ticket.Title)
	if content != "" {
		content += "\n"
	}
	content += "已确认接单，请在处理中工单继续跟进。"
	now := time.Now()
	for i := range items {
		updates := map[string]any{
			"title":   fmt.Sprintf("工单 %s 已接单", ticketNo),
			"content": content,
			"level":   "info",
		}
		if items[i].ReadAt == nil {
			updates["read_at"] = now
		}
		if err := repositories.NotificationRepository.Updates(sqls.DB(), items[i].ID, updates); err != nil {
			return err
		}
	}
	publishNotificationResyncForNotifications(items)
	return nil
}

// ReconcileTicketAssignedAfterCancellation closes assignment notifications
// when a dispatch is superseded by ticket cancellation.
func (s *notificationService) ReconcileTicketAssignedAfterCancellation(ticket *models.Ticket, assigneeID int64) error {
	if ticket == nil || ticket.ID <= 0 || ticket.TenantID <= 0 || assigneeID <= 0 {
		return nil
	}
	items := repositories.NotificationRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", ticket.TenantID).
		Eq("biz_type", "ticket").
		Eq("biz_id", ticket.ID).
		Eq("notification_type", "ticket_assigned").
		Eq("recipient_user_id", assigneeID).
		Eq("status", enums.StatusOk))
	if len(items) == 0 {
		return nil
	}

	ticketNo := strings.TrimSpace(ticket.TicketNo)
	if ticketNo == "" {
		ticketNo = fmt.Sprintf("#%d", ticket.ID)
	}
	content := strings.TrimSpace(ticket.Title)
	if content != "" {
		content += "\n"
	}
	content += "工单已取消，原派单无需继续处理。"
	now := time.Now()
	for i := range items {
		updates := map[string]any{
			"title":   fmt.Sprintf("工单 %s 已取消", ticketNo),
			"content": content,
			"level":   "info",
		}
		if items[i].ReadAt == nil {
			updates["read_at"] = now
		}
		if err := repositories.NotificationRepository.Updates(sqls.DB(), items[i].ID, updates); err != nil {
			return err
		}
	}
	publishNotificationResyncForNotifications(items)
	return nil
}

// ReconcileTicketAssignedAfterRecovery closes assignment notifications when the
// original assignee is no longer eligible and the ticket returns to the dispatch
// pool or moves to another engineer.
func (s *notificationService) ReconcileTicketAssignedAfterRecovery(ticket *models.Ticket, assigneeID int64, reason string) error {
	if ticket == nil || ticket.ID <= 0 || ticket.TenantID <= 0 || assigneeID <= 0 {
		return nil
	}
	items := repositories.NotificationRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", ticket.TenantID).
		Eq("biz_type", "ticket").
		Eq("biz_id", ticket.ID).
		Eq("notification_type", "ticket_assigned").
		Eq("recipient_user_id", assigneeID).
		Eq("status", enums.StatusOk))
	if len(items) == 0 {
		return nil
	}

	ticketNo := strings.TrimSpace(ticket.TicketNo)
	if ticketNo == "" {
		ticketNo = fmt.Sprintf("#%d", ticket.ID)
	}
	content := strings.TrimSpace(ticket.Title)
	if content != "" {
		content += "\n"
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "原派单已回到待派池，无需继续处理。"
	}
	content += reason
	now := time.Now()
	for i := range items {
		updates := map[string]any{
			"title":   fmt.Sprintf("工单 %s 已回收", ticketNo),
			"content": content,
			"level":   "info",
		}
		if items[i].ReadAt == nil {
			updates["read_at"] = now
		}
		if err := repositories.NotificationRepository.Updates(sqls.DB(), items[i].ID, updates); err != nil {
			return err
		}
	}
	publishNotificationResyncForNotifications(items)
	return nil
}

func publishNotificationResyncForNotifications(items []models.Notification) {
	if len(items) == 0 {
		return
	}
	seen := make(map[int64]struct{}, len(items))
	for _, item := range items {
		userID := item.RecipientUserID
		if userID <= 0 {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		WsService.PublishNotificationResyncRequired(userID, enums.IMRealtimeResyncReasonNotificationUpdated)
	}
}

func notificationUserDisplayName(userID int64, fallback string) string {
	if userID > 0 {
		if user := repositories.UserRepository.Get(sqls.DB(), userID); user != nil {
			if nickname := strings.TrimSpace(user.Nickname); nickname != "" {
				return nickname
			}
			if username := strings.TrimSpace(user.Username); username != "" {
				return username
			}
		}
	}
	if fallback = strings.TrimSpace(fallback); fallback != "" {
		return fallback
	}
	return "负责人"
}

func (s *notificationService) FindPageByCnd(cnd *sqls.Cnd) ([]models.Notification, *sqls.Paging) {
	return repositories.NotificationRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *notificationService) CountUnread(userID int64) int64 {
	return s.CountUnreadForTenant(0, userID)
}

func (s *notificationService) CountUnreadForTenant(tenantID, userID int64) int64 {
	if userID <= 0 {
		return 0
	}
	cnd := sqls.NewCnd().
		Eq("recipient_user_id", userID).
		Eq("status", enums.StatusOk).
		Where("read_at IS NULL")
	if tenantID > 0 {
		cnd.Eq("tenant_id", tenantID)
	}
	return repositories.NotificationRepository.Count(sqls.DB(), cnd)
}

func (s *notificationService) MarkRead(id int64, userID int64) error {
	return s.MarkReadForTenant(id, 0, userID)
}

func (s *notificationService) MarkReadForTenant(id, tenantID, userID int64) error {
	if id <= 0 {
		return errorsx.InvalidParamI18n("error.e0337")
	}
	item := repositories.NotificationRepository.Get(sqls.DB(), id)
	if item == nil || item.Status != int(enums.StatusOk) || item.RecipientUserID != userID || (tenantID > 0 && item.TenantID != tenantID) {
		return errorsx.InvalidParamI18n("error.e0337")
	}
	if item.ReadAt != nil {
		return nil
	}
	now := time.Now()
	return repositories.NotificationRepository.Updates(sqls.DB(), id, map[string]any{
		"read_at": now,
	})
}

func (s *notificationService) MarkAllRead(userID int64) error {
	return s.MarkAllReadForTenant(0, userID)
}

func (s *notificationService) MarkAllReadForTenant(tenantID, userID int64) error {
	if userID <= 0 {
		return errorsx.InvalidParamI18n("error.e0212")
	}
	if tenantID > 0 {
		return repositories.NotificationRepository.MarkAllReadByTenant(sqls.DB(), tenantID, userID, time.Now())
	}
	return repositories.NotificationRepository.MarkAllRead(sqls.DB(), userID, time.Now())
}

func validateNotificationResourceTenant(req request.CreateNotificationRequest) error {
	if req.BizID <= 0 {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(req.BizType)) {
	case "product":
		if repositories.ProductRepository.GetByTenant(sqls.DB(), req.BizID, req.TenantID) == nil {
			return errorsx.Forbidden("notification product is outside the tenant")
		}
	case "ticket":
		item := repositories.TicketRepository.Get(sqls.DB(), req.BizID)
		if item == nil || item.TenantID != req.TenantID {
			return errorsx.Forbidden("notification ticket is outside the tenant")
		}
	case "conversation":
		item := repositories.ConversationRepository.Get(sqls.DB(), req.BizID)
		if item == nil || item.TenantID != req.TenantID {
			return errorsx.Forbidden("notification conversation is outside the tenant")
		}
	}
	return nil
}

func normalizeNotificationCategoryLevel(category, level, notificationType, bizType string) (string, string) {
	category = strings.ToLower(strings.TrimSpace(category))
	switch category {
	case "ticket", "sla", "approval", "quota", "knowledge", "video", "system":
	default:
		category, _ = deriveRuntimeNotificationCategoryLevel(notificationType, bizType)
	}
	level = strings.ToLower(strings.TrimSpace(level))
	switch level {
	case "urgent", "warning", "info":
	default:
		_, level = deriveRuntimeNotificationCategoryLevel(notificationType, bizType)
	}
	return category, level
}

func deriveRuntimeNotificationCategoryLevel(notificationType, bizType string) (string, string) {
	notificationType = strings.ToLower(strings.TrimSpace(notificationType))
	bizType = strings.ToLower(strings.TrimSpace(bizType))
	switch {
	case strings.Contains(notificationType, "sla"), strings.Contains(notificationType, "escalat"):
		return "sla", "urgent"
	case strings.HasPrefix(notificationType, "data_breach_"):
		return "system", "urgent"
	case strings.HasPrefix(notificationType, "dsar_"):
		return "system", "warning"
	case strings.HasPrefix(notificationType, "ticket"), bizType == "ticket", bizType == "conversation":
		return "ticket", "info"
	case strings.Contains(notificationType, "meeting"), bizType == "meeting":
		return "video", "info"
	case strings.Contains(notificationType, "knowledge"):
		return "knowledge", "info"
	case strings.Contains(notificationType, "approval"):
		return "approval", "warning"
	case strings.Contains(notificationType, "quota"):
		return "quota", "warning"
	default:
		return "system", "info"
	}
}

func normalizeNotificationChannels(value string) string {
	seen := make(map[string]struct{})
	channels := make([]string, 0, 2)
	for _, channel := range strings.Split(value, ",") {
		channel = strings.ToLower(strings.TrimSpace(channel))
		switch channel {
		case "in_app", "email", "sms", "wxwork", "push":
		default:
			continue
		}
		if _, ok := seen[channel]; ok {
			continue
		}
		seen[channel] = struct{}{}
		channels = append(channels, channel)
	}
	if len(channels) == 0 {
		return "in_app"
	}
	if _, ok := seen["in_app"]; !ok {
		channels = append([]string{"in_app"}, channels...)
	}
	return strings.Join(channels, ",")
}
