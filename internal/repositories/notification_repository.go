package repositories

import (
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var NotificationRepository = newNotificationRepository()

func newNotificationRepository() *notificationRepository {
	return &notificationRepository{}
}

type notificationRepository struct {
}

type TenantNotificationEventFilter struct {
	TenantID   int64
	Category   string
	ReadStatus string
	Search     string
	Page       int
	PageSize   int
}

type TenantNotificationEventAggregate struct {
	Item              models.Notification
	EventKey          string
	RecipientCount    int64
	UnreadCount       int64
	EmailPendingCount int64
	EmailFailedCount  int64
}

type TenantNotificationEventSummary struct {
	Total  int64
	Unread int64
	Urgent int64
	Today  int64
}

type tenantNotificationEventRow struct {
	EventKey          string
	RepresentativeID  int64
	RecipientCount    int64
	UnreadCount       int64
	EmailPendingCount int64
	EmailFailedCount  int64
}

const notificationEventGroupExpression = "CASE WHEN event_key = '' THEN 'legacy:' || CAST(id AS TEXT) ELSE event_key END"

func (r *notificationRepository) Get(db *gorm.DB, id int64) *models.Notification {
	ret := &models.Notification{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *notificationRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.Notification) {
	cnd.Find(db, &list)
	return
}

func (r *notificationRepository) FindPageByParams(db *gorm.DB, params *params.QueryParams) (list []models.Notification, paging *sqls.Paging) {
	return r.FindPageByCnd(db, &params.Cnd)
}

func (r *notificationRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.Notification, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.Notification{})

	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *notificationRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.Notification{})
}

func (r *notificationRepository) Create(db *gorm.DB, item *models.Notification) error {
	return db.Create(item).Error
}

func (r *notificationRepository) FindByIdempotencyKey(db *gorm.DB, tenantID int64, key string) *models.Notification {
	key = strings.TrimSpace(key)
	if tenantID <= 0 || key == "" {
		return nil
	}
	item := &models.Notification{}
	if err := db.Where("tenant_id = ? AND idempotency_key = ?", tenantID, key).First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *notificationRepository) FindPushWithoutDelivery(db *gorm.DB, limit int) ([]models.Notification, error) {
	if db == nil {
		return []models.Notification{}, nil
	}
	if limit <= 0 {
		limit = 100
	}
	deliveryIDs := db.Model(&models.DeliveryLog{}).
		Select("notification_id").
		Where("channel = ?", "push")
	var items []models.Notification
	err := db.Where("status = ? AND channels LIKE ? AND external_channel_status <> ? AND id NOT IN (?)", enums.StatusOk, "%push%", "push_unavailable", deliveryIDs).
		Order("id ASC").Limit(limit).Find(&items).Error
	return items, err
}

func (r *notificationRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.Notification{}).Where("id = ?", id).Updates(columns).Error
}

func (r *notificationRepository) MarkAllRead(db *gorm.DB, userID int64, readAt time.Time) error {
	return db.Model(&models.Notification{}).
		Where("recipient_user_id = ? AND status = ? AND read_at IS NULL", userID, enums.StatusOk).
		Updates(map[string]any{"read_at": readAt}).Error
}

// MarkAllReadByTenant 将租户内当前用户可见的未读通知(个人 + 租户广播)全部标记已读。
func (r *notificationRepository) MarkAllReadByTenant(db *gorm.DB, tenantID, userID int64, readAt time.Time) error {
	query := db.Model(&models.Notification{}).
		Where("tenant_id = ? AND status = ? AND read_at IS NULL", tenantID, enums.StatusOk)
	if userID > 0 {
		query = query.Where("recipient_user_id = ? OR recipient_user_id = 0", userID)
	}
	return query.Update("read_at", readAt).Error
}

func (r *notificationRepository) FindTenantEventPage(db *gorm.DB, filter TenantNotificationEventFilter) ([]TenantNotificationEventAggregate, int64, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 50
	}

	countQuery := r.tenantEventGroupQuery(db, filter).
		Select(notificationEventGroupExpression + " AS event_key").
		Group(notificationEventGroupExpression)
	countQuery = applyTenantNotificationEventReadStatus(countQuery, filter.ReadStatus)
	var total int64
	if err := db.Table("(?) AS notification_event_groups", countQuery).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	pageQuery := r.tenantEventGroupQuery(db, filter).
		Select(notificationEventGroupExpression + ` AS event_key,
			MAX(id) AS representative_id,
			COUNT(*) AS recipient_count,
			SUM(CASE WHEN read_at IS NULL THEN 1 ELSE 0 END) AS unread_count,
			SUM(CASE WHEN channels LIKE '%email%' AND external_channel_status NOT IN ('email_sent', 'email_failed', 'email_unavailable', 'email_skipped') THEN 1 ELSE 0 END) AS email_pending_count,
			SUM(CASE WHEN channels LIKE '%email%' AND external_channel_status IN ('email_failed', 'email_unavailable') THEN 1 ELSE 0 END) AS email_failed_count`).
		Group(notificationEventGroupExpression)
	pageQuery = applyTenantNotificationEventReadStatus(pageQuery, filter.ReadStatus).
		Order("MAX(created_at) DESC").
		Order("MAX(id) DESC").
		Offset((filter.Page - 1) * filter.PageSize).
		Limit(filter.PageSize)

	var rows []tenantNotificationEventRow
	if err := pageQuery.Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	if len(rows) == 0 {
		return []TenantNotificationEventAggregate{}, total, nil
	}

	ids := make([]int64, 0, len(rows))
	for i := range rows {
		ids = append(ids, rows[i].RepresentativeID)
	}
	var items []models.Notification
	if err := db.Where("id IN ?", ids).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	itemsByID := make(map[int64]models.Notification, len(items))
	for i := range items {
		itemsByID[items[i].ID] = items[i]
	}
	ret := make([]TenantNotificationEventAggregate, 0, len(rows))
	for i := range rows {
		item, ok := itemsByID[rows[i].RepresentativeID]
		if !ok {
			continue
		}
		ret = append(ret, TenantNotificationEventAggregate{
			Item:              item,
			EventKey:          rows[i].EventKey,
			RecipientCount:    rows[i].RecipientCount,
			UnreadCount:       rows[i].UnreadCount,
			EmailPendingCount: rows[i].EmailPendingCount,
			EmailFailedCount:  rows[i].EmailFailedCount,
		})
	}
	return ret, total, nil
}

func (r *notificationRepository) SummarizeTenantEvents(db *gorm.DB, tenantID int64, todayStart time.Time) (TenantNotificationEventSummary, error) {
	grouped := db.Model(&models.Notification{}).
		Where("tenant_id = ? AND status = ?", tenantID, enums.StatusOk).
		Select(notificationEventGroupExpression + ` AS event_key,
			SUM(CASE WHEN read_at IS NULL THEN 1 ELSE 0 END) AS unread_count,
			MAX(CASE WHEN level = 'urgent' THEN 1 ELSE 0 END) AS urgent_flag,
			MAX(created_at) AS latest_created_at`).
		Group(notificationEventGroupExpression)
	var summary TenantNotificationEventSummary
	err := db.Table("(?) AS notification_events", grouped).
		Select(`COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN unread_count > 0 THEN 1 ELSE 0 END), 0) AS unread,
			COALESCE(SUM(CASE WHEN unread_count > 0 AND urgent_flag > 0 THEN 1 ELSE 0 END), 0) AS urgent,
			COALESCE(SUM(CASE WHEN latest_created_at >= ? THEN 1 ELSE 0 END), 0) AS today`, todayStart).
		Scan(&summary).Error
	return summary, err
}

func (r *notificationRepository) tenantEventGroupQuery(db *gorm.DB, filter TenantNotificationEventFilter) *gorm.DB {
	query := db.Model(&models.Notification{}).
		Where("tenant_id = ? AND status = ?", filter.TenantID, enums.StatusOk)
	if category := strings.TrimSpace(filter.Category); category != "" && category != "all" {
		query = query.Where("category = ?", category)
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		keyword := "%" + search + "%"
		query = query.Where("title LIKE ? OR content LIKE ? OR recipient_name LIKE ? OR biz_type LIKE ?", keyword, keyword, keyword, keyword)
	}
	return query
}

func applyTenantNotificationEventReadStatus(query *gorm.DB, readStatus string) *gorm.DB {
	switch strings.TrimSpace(readStatus) {
	case "unread":
		return query.Having("SUM(CASE WHEN read_at IS NULL THEN 1 ELSE 0 END) > 0")
	case "read":
		return query.Having("SUM(CASE WHEN read_at IS NULL THEN 1 ELSE 0 END) = 0")
	default:
		return query
	}
}
