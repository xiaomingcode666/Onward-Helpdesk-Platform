package services

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

const (
	auditRetentionConfigKey   = "platform.audit_retention.v1"
	auditRetentionDefaultDays = 90
	auditRetentionMaxDays     = 180
	auditRetentionMinDays     = 1
	auditRetentionBatchSize   = 1000
)

var AuditRetentionService = newAuditRetentionService()

func newAuditRetentionService() *auditRetentionService {
	return &auditRetentionService{}
}

type auditRetentionService struct{}

type auditRetentionStoredConfig struct {
	RetentionDays int `json:"retentionDays"`
}

type AuditRetentionCleanupResult struct {
	RetentionDays        int
	CutoffAt             time.Time
	AuthAuditDeleted     int64
	BusinessAuditDeleted int64
	ArchiveAuditDeleted  int64
}

func (s *auditRetentionService) GetSettings() *response.PlatformAuditRetentionResponse {
	retentionDays, configured := s.currentRetentionDays()
	return buildAuditRetentionResponse(retentionDays, configured)
}

func (s *auditRetentionService) UpdateSettings(req request.PlatformAuditRetentionUpdateRequest, operator *dto.AuthPrincipal) (*response.PlatformAuditRetentionResponse, error) {
	if err := validateAuditRetentionDays(req.RetentionDays); err != nil {
		return nil, err
	}
	before := s.GetSettings()
	payload, err := json.Marshal(auditRetentionStoredConfig{RetentionDays: req.RetentionDays})
	if err != nil {
		return nil, err
	}
	now := time.Now()
	item := &models.SystemConfig{
		ConfigKey:   auditRetentionConfigKey,
		ConfigValue: string(payload),
		GroupCode:   "platform_audit",
		Title:       "审计日志留存策略",
		Description: "平台审计和业务审计记录留存天数，默认 90 天，最大 180 天。",
		Status:      enums.StatusOk,
		AuditFields: utils.BuildAuditFields(operator),
	}
	item.UpdatedAt = now
	if err := repositories.SystemConfigRepository.SaveByKey(sqls.DB(), item); err != nil {
		return nil, err
	}
	after := s.GetSettings()
	_ = PlatformIAMService.RecordAuthAudit(operator, 0, models.DomainTypePlatform, "audit_retention", "platform", "audit_retention.updated",
		map[string]any{"retentionDays": before.RetentionDays, "cutoffAt": before.CutoffAt},
		map[string]any{"retentionDays": after.RetentionDays, "cutoffAt": after.CutoffAt, "summary": "留存 " + strconv.Itoa(after.RetentionDays) + " 天"},
		models.RiskLevelHigh, "")
	return after, nil
}

func (s *auditRetentionService) CleanupExpiredLogs(ctx context.Context) (AuditRetentionCleanupResult, error) {
	retentionDays, _ := s.currentRetentionDays()
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	if ctx == nil {
		ctx = context.Background()
	}
	db := sqls.DB().WithContext(ctx)
	var failures []error
	authDeleted, err := deleteAuditRetentionBatches(func() (int64, error) {
		return repositories.PlatformIAMRepository.DeleteAuthAuditLogsBefore(db, cutoff, auditRetentionBatchSize)
	})
	if err != nil {
		failures = append(failures, err)
	}
	businessDeleted, err := deleteAuditRetentionBatches(func() (int64, error) {
		return repositories.PlatformIAMRepository.DeleteBusinessAuditLogsBefore(db, cutoff, auditRetentionBatchSize)
	})
	if err != nil {
		failures = append(failures, err)
	}
	archiveDeleted, err := deleteAuditRetentionBatches(func() (int64, error) {
		return repositories.PlatformIAMRepository.DeleteBusinessAuditArchiveLogsBefore(db, cutoff, auditRetentionBatchSize)
	})
	if err != nil {
		failures = append(failures, err)
	}
	return AuditRetentionCleanupResult{
		RetentionDays:        retentionDays,
		CutoffAt:             cutoff,
		AuthAuditDeleted:     authDeleted,
		BusinessAuditDeleted: businessDeleted,
		ArchiveAuditDeleted:  archiveDeleted,
	}, errors.Join(failures...)
}

func (s *auditRetentionService) ScheduledCleanup() error {
	result, err := s.CleanupExpiredLogs(context.Background())
	if err != nil {
		return err
	}
	if result.AuthAuditDeleted > 0 || result.BusinessAuditDeleted > 0 || result.ArchiveAuditDeleted > 0 {
		slog.Info("audit_retention: expired audit logs deleted",
			"retention_days", result.RetentionDays,
			"cutoff_at", result.CutoffAt.Format(time.RFC3339),
			"auth_audit_deleted", result.AuthAuditDeleted,
			"business_audit_deleted", result.BusinessAuditDeleted,
			"archive_audit_deleted", result.ArchiveAuditDeleted,
		)
	}
	return nil
}

func (s *auditRetentionService) currentRetentionDays() (int, bool) {
	item := repositories.SystemConfigRepository.FindByKey(sqls.DB(), auditRetentionConfigKey)
	if item == nil {
		return auditRetentionDefaultDays, false
	}
	days, ok := parseAuditRetentionDays(item.ConfigValue)
	if !ok {
		return auditRetentionDefaultDays, true
	}
	return normalizeAuditRetentionDays(days), true
}

func buildAuditRetentionResponse(retentionDays int, configured bool) *response.PlatformAuditRetentionResponse {
	retentionDays = normalizeAuditRetentionDays(retentionDays)
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	return &response.PlatformAuditRetentionResponse{
		RetentionDays:        retentionDays,
		DefaultRetentionDays: auditRetentionDefaultDays,
		MaxRetentionDays:     auditRetentionMaxDays,
		MinRetentionDays:     auditRetentionMinDays,
		CutoffAt:             cutoff.Format(time.DateTime),
		Configured:           configured,
	}
}

func parseAuditRetentionDays(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	if strings.HasPrefix(raw, "{") {
		var cfg auditRetentionStoredConfig
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			return 0, false
		}
		return cfg.RetentionDays, cfg.RetentionDays > 0
	}
	days, err := strconv.Atoi(raw)
	return days, err == nil && days > 0
}

func normalizeAuditRetentionDays(days int) int {
	if days <= 0 {
		return auditRetentionDefaultDays
	}
	if days > auditRetentionMaxDays {
		return auditRetentionMaxDays
	}
	return days
}

func validateAuditRetentionDays(days int) error {
	if days < auditRetentionMinDays {
		return errorsx.InvalidParam("审计日志留存天数至少为 1 天")
	}
	if days > auditRetentionMaxDays {
		return errorsx.InvalidParam("审计日志最多只能保留 180 天")
	}
	return nil
}

func deleteAuditRetentionBatches(deleteBatch func() (int64, error)) (int64, error) {
	var total int64
	for {
		deleted, err := deleteBatch()
		if err != nil {
			return total, err
		}
		total += deleted
		if deleted < auditRetentionBatchSize {
			return total, nil
		}
	}
}
