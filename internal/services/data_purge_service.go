package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/errorsx"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

// ============================================================
// Data Purge Service — 个人数据删除与匿名化（GDPR Art 17 / CCPA）
// ============================================================

var DataPurgeService = newDataPurgeService()

func newDataPurgeService() *dataPurgeService {
	return &dataPurgeService{}
}

type dataPurgeService struct{}

// DataDeletionPolicy 数据删除策略定义
type DataDeletionPolicy struct {
	TableName       string
	DeletionType    string // physical_delete / anonymize / retain_with_purpose
	RetentionDays   int
	CascadeTables   []string
	AnonymizeFields []string // 匿名化处理的字段
}

// DeletionImpact 删除影响范围
type DeletionImpact struct {
	SubjectType    string               `json:"subjectType"`
	SubjectID      string               `json:"subjectId"`
	TablesAffected []string             `json:"tablesAffected"`
	RecordsCount   int                  `json:"recordsCount"`
	PolicyDetails  []DataDeletionPolicy `json:"policyDetails"`
}

// 系统级删除策略矩阵
var deletionPolicyMatrix = []DataDeletionPolicy{
	// Customer 相关
	{TableName: "customers", DeletionType: "anonymize", AnonymizeFields: []string{"name", "primary_mobile", "primary_email"}},
	{TableName: "customer_contacts", DeletionType: "physical_delete"},
	{TableName: "customer_identities", DeletionType: "physical_delete"},
	{TableName: "customer_device_bindings", DeletionType: "anonymize", AnonymizeFields: []string{"customer_org_id"}},

	// User 相关
	{TableName: "users", DeletionType: "anonymize", AnonymizeFields: []string{"username", "nickname", "mobile", "email"}},
	{TableName: "user_identities", DeletionType: "physical_delete"},
	{TableName: "user_roles", DeletionType: "retain_with_purpose", RetentionDays: 365},
	{TableName: "login_sessions", DeletionType: "physical_delete"},

	// 会话与消息（保留业务记录但匿名化）
	{TableName: "conversations", DeletionType: "anonymize", AnonymizeFields: []string{"customer_name"}},
	{TableName: "messages", DeletionType: "retain_with_purpose", RetentionDays: 730},

	// 工单（保留业务记录）
	{TableName: "tickets", DeletionType: "retain_with_purpose", RetentionDays: 1825}, // 5 年
	{TableName: "ticket_progresses", DeletionType: "retain_with_purpose", RetentionDays: 1825},
	{TableName: "ticket_repair_records", DeletionType: "retain_with_purpose", RetentionDays: 1825},

	// 审计日志
	{TableName: "audit_logs", DeletionType: "retain_with_purpose", RetentionDays: 1095}, // 3 年
}

// PurgePersonalData 按策略矩阵删除/匿名化个人数据
func (s *dataPurgeService) PurgePersonalData(ctx context.Context, subjectType, subjectID string) error {
	if subjectID == "" {
		return errorsx.InvalidParam("subject_id is required")
	}

	impact, err := s.GetDeletionImpact(ctx, subjectType, subjectID)
	if err != nil {
		return err
	}

	slog.Info("data_purge: starting purge",
		"subject_type", subjectType,
		"subject_id", subjectID,
		"tables_affected", len(impact.TablesAffected),
	)

	for _, policy := range deletionPolicyMatrix {
		if !s.isTableRelevant(policy, subjectType) {
			continue
		}

		switch policy.DeletionType {
		case "physical_delete":
			s.physicalDeleteRecords(policy, subjectType, subjectID)
		case "anonymize":
			s.anonymizeFields(policy, subjectType, subjectID)
		case "retain_with_purpose":
			// 记录保留目的，不做物理删除
			slog.Info("data_purge: retain with purpose",
				"table", policy.TableName,
				"retention_days", policy.RetentionDays,
			)
		}
	}

	slog.Info("data_purge: purge completed",
		"subject_type", subjectType,
		"subject_id", subjectID,
	)
	return nil
}

// GetDeletionImpact 计算删除影响范围
func (s *dataPurgeService) GetDeletionImpact(ctx context.Context, subjectType, subjectID string) (*DeletionImpact, error) {
	impact := &DeletionImpact{
		SubjectType:    subjectType,
		SubjectID:      subjectID,
		TablesAffected: make([]string, 0),
		PolicyDetails:  make([]DataDeletionPolicy, 0),
	}

	for _, policy := range deletionPolicyMatrix {
		if !s.isTableRelevant(policy, subjectType) {
			continue
		}
		impact.TablesAffected = append(impact.TablesAffected, policy.TableName)
		impact.PolicyDetails = append(impact.PolicyDetails, policy)
		impact.RecordsCount++
	}

	return impact, nil
}

// AnonymizeRecord 将指定表的记录 PII 字段替换为 "ANONYMIZED_xxx"
func (s *dataPurgeService) AnonymizeRecord(ctx context.Context, tableName, recordID string, fields []string) error {
	if tableName == "" || recordID == "" {
		return errorsx.InvalidParam("table_name and record_id are required")
	}

	updateMap := make(map[string]any)
	for _, field := range fields {
		updateMap[field] = fmt.Sprintf("ANONYMIZED_%s", recordID)
	}

	return sqls.DB().Table(tableName).Where("id = ?", recordID).Updates(updateMap).Error
}

// ScheduledDataPurge 定时清理超过保留期的数据（定时任务）
func (s *dataPurgeService) ScheduledDataPurge() error {
	slog.Info("data_purge: scheduled purge started")

	cutoffDate := time.Now().AddDate(0, 0, -365) // 默认保留 1 年

	for _, policy := range deletionPolicyMatrix {
		if policy.DeletionType != "physical_delete" {
			continue
		}
		if policy.RetentionDays > 0 {
			cutoffDate = time.Now().AddDate(0, 0, -policy.RetentionDays)
		}

		var count int64
		sqls.DB().Table(policy.TableName).
			Where("created_at < ?", cutoffDate).
			Count(&count)

		if count > 0 {
			slog.Info("data_purge: deleting expired records",
				"table", policy.TableName,
				"count", count,
				"cutoff_date", cutoffDate.Format(time.RFC3339),
			)

			sqls.DB().Table(policy.TableName).
				Where("created_at < ?", cutoffDate).
				Delete(&gorm.Model{})
		}
	}

	slog.Info("data_purge: scheduled purge completed")
	return nil
}

// isTableRelevant 判断策略是否适用于该主体类型
func (s *dataPurgeService) isTableRelevant(policy DataDeletionPolicy, subjectType string) bool {
	switch subjectType {
	case "customer":
		switch policy.TableName {
		case "customers", "customer_contacts", "customer_identities", "customer_device_bindings",
			"conversations", "messages", "tickets", "ticket_progresses", "ticket_repair_records",
			"audit_logs":
			return true
		}
	case "enterprise_user":
		switch policy.TableName {
		case "users", "user_identities", "user_roles", "login_sessions",
			"audit_logs":
			return true
		}
	case "visitor":
		switch policy.TableName {
		case "conversations", "messages":
			return true
		}
	}
	return false
}

// physicalDeleteRecords 物理删除记录
func (s *dataPurgeService) physicalDeleteRecords(policy DataDeletionPolicy, subjectType, subjectID string) {
	var db *gorm.DB
	switch policy.TableName {
	case "customer_contacts":
		db = sqls.DB().Where("customer_id = ?", subjectID).Delete(&models.CustomerContact{})
	case "customer_identities":
		db = sqls.DB().Where("customer_id = ?", subjectID).Delete(&models.CustomerIdentity{})
	case "user_identities":
		db = sqls.DB().Where("user_id = ?", subjectID).Delete(&models.UserIdentity{})
	case "user_roles":
		db = sqls.DB().Where("user_id = ?", subjectID).Delete(&models.UserRole{})
	case "login_sessions":
		db = sqls.DB().Where("user_id = ?", subjectID).Delete(&models.LoginSession{})
	default:
		slog.Warn("data_purge: no delete mapping for table", "table", policy.TableName)
		return
	}

	if db.Error != nil {
		slog.Error("data_purge: failed to delete records",
			"table", policy.TableName,
			"error", db.Error,
		)
	}
}

// anonymizeFields 匿名化记录中的指定字段
func (s *dataPurgeService) anonymizeFields(policy DataDeletionPolicy, subjectType, subjectID string) {
	updateMap := make(map[string]any)
	for _, field := range policy.AnonymizeFields {
		updateMap[field] = fmt.Sprintf("ANONYMIZED_%s", subjectID)
	}

	if len(updateMap) == 0 {
		return
	}

	var db *gorm.DB
	switch policy.TableName {
	case "customers":
		db = sqls.DB().Model(&models.Customer{}).Where("id = ?", subjectID).Updates(updateMap)
	case "customer_device_bindings":
		db = sqls.DB().Table("customer_device_bindings").Where("customer_user_id = ?", subjectID).Updates(updateMap)
	case "users":
		db = sqls.DB().Model(&models.User{}).Where("id = ?", subjectID).Updates(updateMap)
	case "conversations":
		db = sqls.DB().Model(&models.Conversation{}).Where("customer_id = ?", subjectID).Updates(updateMap)
	default:
		slog.Warn("data_purge: no anonymize mapping for table", "table", policy.TableName)
		return
	}

	if db.Error != nil {
		slog.Error("data_purge: failed to anonymize records",
			"table", policy.TableName,
			"error", db.Error,
		)
	}
}
