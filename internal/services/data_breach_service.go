package services

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/errorsx"

	"github.com/google/uuid"
	"github.com/mlogclub/simple/sqls"
)

// ============================================================
// Data Breach Service — 数据泄露通知管理（GDPR Art 33/34）
// ============================================================

var DataBreachService = newDataBreachService()

func newDataBreachService() *dataBreachService {
	return &dataBreachService{}
}

type dataBreachService struct{}

// RecordDataBreach 记录数据泄露事件，自动计算 72h 通知截止时间
func (s *dataBreachService) RecordDataBreach(ctx context.Context, record *models.DataBreachRecord) error {
	if record.TenantID == "" {
		return errorsx.InvalidParam("tenant_id is required")
	}
	if record.Description == "" {
		return errorsx.InvalidParam("description is required")
	}
	if !isValidBreachType(record.BreachType) {
		return errorsx.InvalidParam("invalid breach_type: " + record.BreachType)
	}

	now := time.Now()
	if record.DetectedAt.IsZero() {
		record.DetectedAt = now
	}

	record.ID = uuid.NewString()
	record.NotificationDeadline = record.DetectedAt.Add(72 * time.Hour) // GDPR: 72h 通知窗口
	if record.Status == "" {
		record.Status = "investigating"
	}
	record.BaseModel = models.BaseModel{
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := sqls.DB().Create(record).Error; err != nil {
		slog.Error("data_breach: failed to record breach", "tenant_id", record.TenantID, "error", err)
		return err
	}

	slog.Warn("data_breach: breach recorded",
		"breach_id", record.ID,
		"breach_type", record.BreachType,
		"deadline", record.NotificationDeadline.Format(time.RFC3339),
	)

	return nil
}

// NotifySupervisoryAuthority 通知监管机构
func (s *dataBreachService) NotifySupervisoryAuthority(ctx context.Context, tenantID, breachID string) error {
	if tenantID == "" {
		return errorsx.InvalidParam("tenant_id is required")
	}
	if breachID == "" {
		return errorsx.InvalidParam("breach_id is required")
	}

	var breach models.DataBreachRecord
	if err := sqls.DB().Where("tenant_id = ? AND id = ?", tenantID, breachID).First(&breach).Error; err != nil {
		return errorsx.BusinessError(1, "data breach record not found")
	}

	now := time.Now()
	updates := map[string]any{
		"supervisory_authority_notified_at": now,
		"status":                            "contained",
		"updated_at":                        now,
	}

	if err := sqls.DB().Model(&models.DataBreachRecord{}).Where("tenant_id = ? AND id = ?", tenantID, breachID).Updates(updates).Error; err != nil {
		slog.Error("data_breach: failed to notify supervisory authority", "breach_id", breachID, "error", err)
		return err
	}

	slog.Info("data_breach: supervisory authority notified", "breach_id", breachID)
	return nil
}

// NotifyAffectedUsers 通知受影响用户
func (s *dataBreachService) NotifyAffectedUsers(ctx context.Context, tenantID, breachID string) error {
	if tenantID == "" {
		return errorsx.InvalidParam("tenant_id is required")
	}
	if breachID == "" {
		return errorsx.InvalidParam("breach_id is required")
	}

	var breach models.DataBreachRecord
	if err := sqls.DB().Where("tenant_id = ? AND id = ?", tenantID, breachID).First(&breach).Error; err != nil {
		return errorsx.BusinessError(1, "data breach record not found")
	}

	now := time.Now()
	updates := map[string]any{
		"data_subjects_notified_at": now,
		"status":                    "notified",
		"updated_at":                now,
	}

	if err := sqls.DB().Model(&models.DataBreachRecord{}).Where("tenant_id = ? AND id = ?", tenantID, breachID).Updates(updates).Error; err != nil {
		slog.Error("data_breach: failed to notify affected users", "breach_id", breachID, "error", err)
		return err
	}

	slog.Info("data_breach: affected users notified", "breach_id", breachID,
		"affected_users_count", breach.AffectedUsersCount)

	return nil
}

// GetBreach 获取数据泄露记录详情
func (s *dataBreachService) GetBreach(ctx context.Context, tenantID, breachID string) (*models.DataBreachRecord, error) {
	var breach models.DataBreachRecord
	if err := sqls.DB().Where("tenant_id = ? AND id = ?", tenantID, breachID).First(&breach).Error; err != nil {
		return nil, errorsx.BusinessError(1, "data breach record not found")
	}
	return &breach, nil
}

// ListBreaches 查询数据泄露记录列表
func (s *dataBreachService) ListBreaches(ctx context.Context, tenantID string, page, pageSize int) ([]models.DataBreachRecord, int64, error) {
	if tenantID == "" {
		return nil, 0, errorsx.InvalidParam("tenant_id is required")
	}
	db := sqls.DB().Model(&models.DataBreachRecord{})
	db = db.Where("tenant_id = ?", tenantID)

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	var total int64
	db.Count(&total)

	var breaches []models.DataBreachRecord
	db.Order("detected_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&breaches)

	return breaches, total, nil
}

// ResolveBreach 标记数据泄露为已解决
func (s *dataBreachService) ResolveBreach(ctx context.Context, tenantID, breachID, remediationSteps string) error {
	if breachID == "" {
		return errorsx.InvalidParam("breach_id is required")
	}

	now := time.Now()
	updates := map[string]any{
		"status":            "resolved",
		"remediation_steps": remediationSteps,
		"updated_at":        now,
	}

	return sqls.DB().Model(&models.DataBreachRecord{}).
		Where("tenant_id = ? AND id = ?", tenantID, breachID).
		Updates(updates).Error
}

// CheckBreachDeadlines 检查临近 72h 截止的泄露事件（定时任务）
func (s *dataBreachService) CheckBreachDeadlines() error {
	now := time.Now()

	// 检查 12 小时内即将到达通知截止时间的泄露事件
	var urgentBreaches []models.DataBreachRecord
	sqls.DB().
		Where("notification_deadline <= ? AND notification_deadline > ? AND supervisory_authority_notified_at IS NULL",
			now.Add(12*time.Hour), now).
		Find(&urgentBreaches)

	for _, breach := range urgentBreaches {
		remainingHours := int(breach.NotificationDeadline.Sub(now).Hours())
		slog.Error("data_breach: notification deadline approaching",
			"breach_id", breach.ID,
			"tenant_id", breach.TenantID,
			"remaining_hours", remainingHours,
			"deadline", breach.NotificationDeadline.Format(time.RFC3339),
		)

		// 发送紧急通知给租户管理员（安全团队）
		tenantIDStr := strings.TrimSpace(breach.TenantID)
		tenantID, _ := strconv.ParseInt(tenantIDStr, 10, 64)
		adminUserIDs := findTenantAdminUserIDs(tenantIDStr)
		for _, adminID := range adminUserIDs {
			_, _ = NotificationQueueService.EnqueueCreate(context.Background(), request.CreateNotificationRequest{
				TenantID:         tenantID,
				RecipientUserID:  adminID,
				Title:            "数据泄露通知截止时间临近 — 紧急",
				Content:          fmt.Sprintf("数据泄露事件 %s（类型：%s，严重级别：%s）的通知截止时间仅剩 %d 小时（截止时间：%s），请立即通知监管机构。", breach.ID, breach.BreachType, breach.Severity, remainingHours, breach.NotificationDeadline.Format("2006-01-02 15:04")),
				NotificationType: "data_breach_urgent",
				BizType:          "data_breach",
				BizID:            0,
				ActionURL:        "/enterprise/audit",
				Category:         "system",
				Level:            "urgent",
				Channels:         "in_app",
			})
		}
		if len(adminUserIDs) == 0 {
			slog.Error("data_breach: no tenant admin found for urgent notification", "tenant_id", tenantIDStr, "breach_id", breach.ID)
		}
	}

	// 检查已过期的通知截止
	var overdueBreaches []models.DataBreachRecord
	sqls.DB().
		Where("notification_deadline < ? AND supervisory_authority_notified_at IS NULL", now).
		Find(&overdueBreaches)

	for _, breach := range overdueBreaches {
		slog.Error("data_breach: notification deadline exceeded",
			"breach_id", breach.ID,
			"tenant_id", breach.TenantID,
			"deadline", breach.NotificationDeadline.Format(time.RFC3339),
		)

		// 记录合规违规：更新泄露记录状态为 breached
		if err := sqls.DB().Model(&breach).Update("status", "breached").Error; err != nil {
			slog.Error("data_breach: failed to mark breach as breached", "breach_id", breach.ID, "error", err)
		}

		// 通知租户管理员合规违规
		tenantIDStr := strings.TrimSpace(breach.TenantID)
		tenantID, _ := strconv.ParseInt(tenantIDStr, 10, 64)
		adminUserIDs := findTenantAdminUserIDs(tenantIDStr)
		for _, adminID := range adminUserIDs {
			_, _ = NotificationQueueService.EnqueueCreate(context.Background(), request.CreateNotificationRequest{
				TenantID:         tenantID,
				RecipientUserID:  adminID,
				Title:            "数据泄露通知已逾期 — 合规违规",
				Content:          fmt.Sprintf("数据泄露事件 %s（类型：%s）的通知截止时间已过（截止时间：%s），未按时通知监管机构构成合规违规，请立即处理并上报。", breach.ID, breach.BreachType, breach.NotificationDeadline.Format("2006-01-02 15:04")),
				NotificationType: "data_breach_overdue",
				BizType:          "data_breach",
				BizID:            0,
				ActionURL:        "/enterprise/audit",
				Category:         "system",
				Level:            "urgent",
				Channels:         "in_app",
			})
		}
	}

	return nil
}

// --- 辅助函数 ---

func isValidBreachType(t string) bool {
	switch t {
	case "unauthorized_access", "data_leak", "accidental_disclosure":
		return true
	}
	return false
}

// GetBreachNotificationStatus 获取泄露通知状态摘要
func (s *dataBreachService) GetBreachNotificationStatus(ctx context.Context, tenantID, breachID string) (map[string]any, error) {
	breach, err := s.GetBreach(ctx, tenantID, breachID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	status := fmt.Sprintf("Notification deadline: %s", breach.NotificationDeadline.Format(time.RFC3339))
	if breach.SupervisoryAuthorityNotifiedAt != nil {
		status += fmt.Sprintf(", Authority notified at: %s", breach.SupervisoryAuthorityNotifiedAt.Format(time.RFC3339))
	}
	if breach.DataSubjectsNotifiedAt != nil {
		status += fmt.Sprintf(", Subjects notified at: %s", breach.DataSubjectsNotifiedAt.Format(time.RFC3339))
	}
	if now.After(breach.NotificationDeadline) {
		status += ", DEADLINE EXCEEDED"
	}

	return map[string]any{
		"breach_id":                         breach.ID,
		"status":                            breach.Status,
		"notification_deadline":             breach.NotificationDeadline,
		"supervisory_authority_notified_at": breach.SupervisoryAuthorityNotifiedAt,
		"data_subjects_notified_at":         breach.DataSubjectsNotifiedAt,
		"summary":                           status,
	}, nil
}
