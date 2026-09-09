package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/errorsx"

	"github.com/google/uuid"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

// ============================================================
// Data Retention Service — 数据保留与归档管理
// ============================================================

var DataRetentionService = newDataRetentionService()

func newDataRetentionService() *dataRetentionService {
	return &dataRetentionService{}
}

type dataRetentionService struct{}

// RetentionPolicy 数据保留策略
type RetentionPolicy struct {
	TenantID          string `json:"tenantId"`
	DataRegion        string `json:"dataRegion"`
	RetentionDays     int    `json:"retentionDays"`
	ArchiveAfterDays  int    `json:"archiveAfterDays"`
	AutoDeleteEnabled bool   `json:"autoDeleteEnabled"`
	GDPRRegion        bool   `json:"gdprRegion"`
	CCPARegion        bool   `json:"ccpaRegion"`
}

var retentionArchivePredicates = map[string]string{
	"audit_logs":              "tenant_id = ? AND created_at < ?",
	"conversation_event_logs": "conversation_id IN (SELECT id FROM conversations WHERE tenant_id = ?) AND created_at < ?",
}

// ArchiveOldData moves one tenant's old rows into an archive table in a single
// transaction. Table names are deliberately restricted because SQL parameters
// cannot safely bind identifiers.
func (s *dataRetentionService) ArchiveOldData(ctx context.Context, tableName, tenantID string, beforeDate time.Time) error {
	tableName = strings.TrimSpace(tableName)
	predicate, allowed := retentionArchivePredicates[tableName]
	if !allowed {
		return errorsx.InvalidParam("unsupported retention archive table")
	}
	parsedTenantID, err := strconv.ParseInt(strings.TrimSpace(tenantID), 10, 64)
	if err != nil || parsedTenantID <= 0 {
		return errorsx.InvalidParam("tenant_id is invalid")
	}
	if beforeDate.IsZero() {
		return errorsx.InvalidParam("before_date is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	archiveTableName := tableName + "_archive"
	db := sqls.DB().WithContext(ctx)
	var archivedCount int64
	err = db.Transaction(func(tx *gorm.DB) error {
		createSQL := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s AS SELECT * FROM %s WHERE 1 = 0", archiveTableName, tableName)
		if err := tx.Exec(createSQL).Error; err != nil {
			return fmt.Errorf("create archive table: %w", err)
		}
		insertSQL := fmt.Sprintf("INSERT INTO %s SELECT * FROM %s WHERE %s", archiveTableName, tableName, predicate)
		result := tx.Exec(insertSQL, parsedTenantID, beforeDate)
		if result.Error != nil {
			return fmt.Errorf("archive rows: %w", result.Error)
		}
		archivedCount = result.RowsAffected
		deleteSQL := fmt.Sprintf("DELETE FROM %s WHERE %s", tableName, predicate)
		if err := tx.Exec(deleteSQL, parsedTenantID, beforeDate).Error; err != nil {
			return fmt.Errorf("delete archived rows: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	slog.Info("data_retention: archive completed", "tenant_id", parsedTenantID, "table", tableName,
		"archive_table", archiveTableName, "archived_count", archivedCount, "before_date", beforeDate.Format(time.RFC3339))
	return nil
}

// CleanupExpiredData 基于 data_region_policies.retention_days 清理过期数据
func (s *dataRetentionService) CleanupExpiredData(ctx context.Context) error {
	slog.Info("data_retention: cleanup expired data started")

	var policies []models.DataRegionPolicy
	if err := sqls.DB().WithContext(ctx).Where("status = 'active' AND auto_delete_enabled = ?", true).Find(&policies).Error; err != nil {
		return fmt.Errorf("query retention policies: %w", err)
	}
	var failures []error

	for _, policy := range policies {
		if policy.RetentionDays <= 0 {
			failures = append(failures, fmt.Errorf("tenant %s retention_days must be positive", policy.TenantID))
			continue
		}
		cutoffDate := time.Now().AddDate(0, 0, -policy.RetentionDays)

		slog.Info("data_retention: cleaning by policy",
			"tenant_id", policy.TenantID,
			"data_region", policy.DataRegion,
			"retention_days", policy.RetentionDays,
			"cutoff_date", cutoffDate.Format(time.RFC3339),
		)

		if err := s.deleteExpiredAuditLogs(ctx, policy.TenantID, cutoffDate); err != nil {
			failures = append(failures, err)
		}
		if err := s.deleteExpiredConversationLogs(ctx, policy.TenantID, cutoffDate); err != nil {
			failures = append(failures, err)
		}
		if err := s.deleteExpiredNotifications(ctx, policy.TenantID, cutoffDate); err != nil {
			failures = append(failures, err)
		}
		if err := s.deleteExpiredPrivacyConsents(ctx, policy.TenantID, cutoffDate); err != nil {
			failures = append(failures, err)
		}

		// 归档旧数据（如 retention 较长可再做归档）
		if policy.ArchiveAfterDays > 0 && policy.ArchiveAfterDays < policy.RetentionDays {
			archiveDate := time.Now().AddDate(0, 0, -policy.ArchiveAfterDays)
			if err := s.ArchiveOldData(ctx, "audit_logs", policy.TenantID, archiveDate); err != nil {
				failures = append(failures, err)
			}

			if policy.RetentionDays > policy.ArchiveAfterDays*2 {
				if err := s.ArchiveOldData(ctx, "conversation_event_logs", policy.TenantID, archiveDate); err != nil {
					failures = append(failures, err)
				}
			}
		}
	}

	slog.Info("data_retention: cleanup completed")
	return errors.Join(failures...)
}

// GetRetentionPolicy 获取租户的数据保留策略
func (s *dataRetentionService) GetRetentionPolicy(ctx context.Context, tenantID string) (*RetentionPolicy, error) {
	if tenantID == "" {
		return nil, errorsx.InvalidParam("tenant_id is required")
	}

	var policy models.DataRegionPolicy
	if err := sqls.DB().WithContext(ctx).
		Where("tenant_id = ? AND status = 'active'", tenantID).
		First(&policy).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return &RetentionPolicy{
			TenantID:          tenantID,
			DataRegion:        "global",
			RetentionDays:     365,
			ArchiveAfterDays:  180,
			AutoDeleteEnabled: true,
			GDPRRegion:        false,
			CCPARegion:        false,
		}, nil
	}

	return &RetentionPolicy{
		TenantID:          tenantID,
		DataRegion:        policy.DataRegion,
		RetentionDays:     policy.RetentionDays,
		ArchiveAfterDays:  policy.ArchiveAfterDays,
		AutoDeleteEnabled: policy.AutoDeleteEnabled,
		GDPRRegion:        policy.GDPRRegion,
		CCPARegion:        policy.CCPARegion,
	}, nil
}

// SetRetentionPolicy 设置租户的数据保留策略
func (s *dataRetentionService) SetRetentionPolicy(ctx context.Context, tenantID, dataRegion string, retentionDays, archiveAfterDays int, autoDelete, gdpr, ccpa bool) error {
	if strings.TrimSpace(tenantID) == "" {
		return errorsx.InvalidParam("tenant_id is required")
	}
	if strings.TrimSpace(dataRegion) == "" {
		return errorsx.InvalidParam("data_region is required")
	}
	if retentionDays <= 0 || archiveAfterDays < 0 || (archiveAfterDays > 0 && archiveAfterDays >= retentionDays) {
		return errorsx.InvalidParam("retention policy duration is invalid")
	}

	var existing models.DataRegionPolicy
	result := sqls.DB().WithContext(ctx).Where("tenant_id = ? AND data_region = ?", tenantID, dataRegion).First(&existing)

	now := time.Now()
	updates := map[string]any{
		"retention_days":      retentionDays,
		"archive_after_days":  archiveAfterDays,
		"auto_delete_enabled": autoDelete,
		"gdpr_region":         gdpr,
		"ccpa_region":         ccpa,
		"updated_at":          now,
	}

	if result.Error != nil {
		// 创建新策略
		policy := &models.DataRegionPolicy{
			ID:                uuid.NewString(),
			TenantID:          tenantID,
			DataRegion:        dataRegion,
			RetentionDays:     retentionDays,
			ArchiveAfterDays:  archiveAfterDays,
			AutoDeleteEnabled: autoDelete,
			GDPRRegion:        gdpr,
			CCPARegion:        ccpa,
			Status:            "active",
			CreatedAt:         now,
			UpdatedAt:         now,
		}
		return sqls.DB().Create(policy).Error
	}

	return sqls.DB().Model(&existing).Updates(updates).Error
}

// ScheduledRetentionCleanup 定时清理任务（每日执行）
func (s *dataRetentionService) ScheduledRetentionCleanup() error {
	slog.Info("data_retention: scheduled retention cleanup started")
	return s.CleanupExpiredData(context.Background())
}

// deleteExpiredAuditLogs 清理过期的审计日志
func (s *dataRetentionService) deleteExpiredAuditLogs(ctx context.Context, tenantID string, cutoffDate time.Time) error {
	result := sqls.DB().WithContext(ctx).Where("tenant_id = ? AND created_at < ?", tenantID, cutoffDate).Delete(&models.AuditLog{})
	if result.Error == nil && result.RowsAffected > 0 {
		slog.Info("data_retention: deleted expired audit logs", "tenant_id", tenantID, "count", result.RowsAffected)
	}
	return result.Error
}

// deleteExpiredConversationLogs 清理过期的会话日志
func (s *dataRetentionService) deleteExpiredConversationLogs(ctx context.Context, tenantID string, cutoffDate time.Time) error {
	conversationIDs := sqls.DB().WithContext(ctx).Model(&models.Conversation{}).Select("id").Where("tenant_id = ?", tenantID)
	result := sqls.DB().WithContext(ctx).
		Where("created_at < ? AND conversation_id IN (?)", cutoffDate, conversationIDs).
		Delete(&models.ConversationEventLog{})
	if result.Error == nil && result.RowsAffected > 0 {
		slog.Info("data_retention: deleted expired conversation logs", "tenant_id", tenantID, "count", result.RowsAffected)
	}
	return result.Error
}

// deleteExpiredNotifications 清理过期的通知
func (s *dataRetentionService) deleteExpiredNotifications(ctx context.Context, tenantID string, cutoffDate time.Time) error {
	result := sqls.DB().WithContext(ctx).Where("tenant_id = ? AND created_at < ?", tenantID, cutoffDate).Delete(&models.Notification{})
	if result.Error == nil && result.RowsAffected > 0 {
		slog.Info("data_retention: deleted expired notifications", "tenant_id", tenantID, "count", result.RowsAffected)
	}
	return result.Error
}

func (s *dataRetentionService) deleteExpiredPrivacyConsents(ctx context.Context, tenantID string, cutoffDate time.Time) error {
	result := sqls.DB().WithContext(ctx).
		Where("tenant_id = ? AND consented_at < ?", tenantID, cutoffDate).
		Delete(&models.CustomerPrivacyConsent{})
	if result.Error == nil && result.RowsAffected > 0 {
		slog.Info("data_retention: deleted expired privacy consents", "tenant_id", tenantID, "count", result.RowsAffected)
	}
	return result.Error
}
