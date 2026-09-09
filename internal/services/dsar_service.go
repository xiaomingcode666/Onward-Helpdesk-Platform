package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"

	"github.com/google/uuid"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ============================================================
// DSAR Service — 数据主体访问请求管理（GDPR Art 15/17/20）
// ============================================================

var DSARService = newDSARService()

func newDSARService() *dsarService {
	return &dsarService{}
}

type dsarService struct{}

// SubmitDSAR 提交 DSAR 请求，自动设置 deadline = now + 30 天
func (s *dsarService) SubmitDSAR(ctx context.Context, tenantID, subjectType, subjectID, subjectEmail, requestType string) (*models.DSARRequest, error) {
	if tenantID == "" {
		return nil, errorsx.InvalidParam("tenant_id is required")
	}
	if subjectID == "" {
		return nil, errorsx.InvalidParam("subject_id is required")
	}
	if !isValidDSARSubjectType(subjectType) {
		return nil, errorsx.InvalidParam("invalid subject_type: " + subjectType)
	}
	if !isValidDSARRequestType(requestType) {
		return nil, errorsx.InvalidParam("invalid request_type: " + requestType)
	}
	if err := s.validateSubjectOwnership(tenantID, subjectType, subjectID); err != nil {
		return nil, err
	}

	now := time.Now()
	req := &models.DSARRequest{
		ID:           uuid.NewString(),
		TenantID:     tenantID,
		SubjectType:  subjectType,
		SubjectID:    subjectID,
		SubjectEmail: subjectEmail,
		RequestType:  requestType,
		Status:       "pending",
		DeadlineAt:   now.AddDate(0, 0, 30), // GDPR 要求 30 天内响应
		RequestedAt:  now,
		BaseModel: models.BaseModel{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	if err := sqls.DB().Create(req).Error; err != nil {
		slog.Error("dsar: failed to submit dsar request", "tenant_id", tenantID, "error", err)
		return nil, err
	}

	slog.Info("dsar: request submitted", "dsar_id", req.ID, "request_type", requestType)
	return req, nil
}

// VerifyIdentity 验证数据主体身份
func (s *dsarService) VerifyIdentity(ctx context.Context, tenantID, dsarID, verificationMethod string) error {
	if tenantID == "" {
		return errorsx.InvalidParam("tenant_id is required")
	}
	if dsarID == "" {
		return errorsx.InvalidParam("dsar_id is required")
	}

	var req models.DSARRequest
	if err := sqls.DB().Where("tenant_id = ? AND id = ?", tenantID, dsarID).First(&req).Error; err != nil {
		return errorsx.BusinessError(1, "dsar request not found")
	}

	if req.Status != "pending" && req.Status != "awaiting_verification" {
		return errorsx.BusinessError(2, "dsar request is not in a verifiable state")
	}

	now := time.Now()
	updates := map[string]any{
		"status":           "processing",
		"verified_at":      now,
		"updated_at":       now,
		"processing_notes": fmt.Sprintf("Verified via %s at %s", verificationMethod, now.Format(time.RFC3339)),
	}

	if err := sqls.DB().Model(&models.DSARRequest{}).Where("tenant_id = ? AND id = ?", tenantID, dsarID).Updates(updates).Error; err != nil {
		slog.Error("dsar: failed to verify identity", "dsar_id", dsarID, "error", err)
		return err
	}

	slog.Info("dsar: identity verified", "dsar_id", dsarID, "method", verificationMethod)
	return nil
}

// ExecuteDataExport 导出个人数据为 JSON 格式（GDPR Art 20 数据可携带性）
func (s *dsarService) ExecuteDataExport(ctx context.Context, tenantID, dsarID string) ([]byte, string, error) {
	var req models.DSARRequest
	if err := sqls.DB().Where("tenant_id = ? AND id = ?", tenantID, dsarID).First(&req).Error; err != nil {
		return nil, "", errorsx.BusinessError(1, "dsar request not found")
	}

	if req.Status != "processing" && req.Status != "pending" {
		return nil, "", errorsx.BusinessError(2, "dsar request is not in exportable state")
	}

	// 收集该数据主体的个人信息
	exportData := s.collectPersonalData(req.TenantID, req.SubjectType, req.SubjectID)

	exportJSON, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return nil, "", errorsx.BusinessError(3, "failed to marshal export data")
	}

	now := time.Now()
	filename := fmt.Sprintf("dsar_export_%s_%s.json", req.SubjectID, now.Format("20060102150405"))

	// 更新请求状态
	sqls.DB().Model(&models.DSARRequest{}).Where("tenant_id = ? AND id = ?", tenantID, dsarID).Updates(map[string]any{
		"status":       "completed",
		"completed_at": now,
		"updated_at":   now,
	})

	// 记录执行日志
	s.recordExecutionLog(dsarID, "export", "personal_data", req.SubjectID,
		fmt.Sprintf(`{"subject_type":"%s","exported_fields_count":%d}`, req.SubjectType, len(exportData)))

	slog.Info("dsar: data export completed", "dsar_id", dsarID, "subject_id", req.SubjectID)
	return exportJSON, filename, nil
}

// collectPersonalData 收集指定数据主体的所有个人数据
func (s *dsarService) collectPersonalData(tenantID, subjectType, subjectID string) map[string]any {
	data := make(map[string]any)

	switch subjectType {
	case "customer":
		var customer models.Customer
		if err := sqls.DB().Where("id = ?", subjectID).First(&customer).Error; err == nil {
			if s.customerReferencedByOtherTenant(tenantID, subjectID) {
				data["customer_reference"] = map[string]any{"id": customer.ID, "status": customer.Status}
			} else {
				data["customer"] = customer
			}
		}
		// Legacy customer contact rows are not tenant-owned. Only expose them when
		// the customer has no references from another tenant.
		if !s.customerReferencedByOtherTenant(tenantID, subjectID) {
			var contacts []models.CustomerContact
			sqls.DB().Where("customer_id = ?", subjectID).Find(&contacts)
			if len(contacts) > 0 {
				data["contacts"] = contacts
			}
			var identities []models.CustomerIdentity
			sqls.DB().Where("customer_id = ?", subjectID).Find(&identities)
			if len(identities) > 0 {
				data["identities"] = identities
			}
		}
		// 收集会话历史
		var conversations []models.Conversation
		sqls.DB().Where("tenant_id = ? AND customer_id = ?", tenantID, subjectID).Limit(1000).Find(&conversations)
		if len(conversations) > 0 {
			data["conversations"] = conversations
		}
		// 收集工单
		var tickets []models.Ticket
		sqls.DB().Where("tenant_id = ? AND customer_id = ?", tenantID, subjectID).Limit(1000).Find(&tickets)
		if len(tickets) > 0 {
			data["tickets"] = tickets
		}

	case "enterprise_user":
		var user models.User
		if err := sqls.DB().Where("id = ?", subjectID).First(&user).Error; err == nil {
			data["user"] = map[string]any{
				"id": user.ID, "username": user.Username, "nickname": user.Nickname,
				"mobile": user.Mobile, "email": user.Email, "status": user.Status,
			}
		}
		var member models.TenantMember
		if tenantIDInt, err := strconv.ParseInt(tenantID, 10, 64); err == nil {
			if err := sqls.DB().Where("tenant_id = ? AND user_id = ?", tenantIDInt, subjectID).First(&member).Error; err == nil {
				data["tenant_member"] = member
			}
		}

	case "customer_user":
		var customerUser models.CustomerUser
		if err := sqls.DB().Where("tenant_id = ? AND id = ?", tenantID, subjectID).First(&customerUser).Error; err == nil {
			data["customer_user"] = map[string]any{
				"id": customerUser.ID, "display_name": customerUser.DisplayName,
				"email": customerUser.Email, "phone": customerUser.Phone,
				"locale": customerUser.Locale, "timezone": customerUser.Timezone,
				"status": customerUser.Status,
			}
			var bindings []models.CustomerDeviceBinding
			sqls.DB().Where("tenant_id = ? AND customer_user_id = ?", tenantID, subjectID).Limit(1000).Find(&bindings)
			if len(bindings) > 0 {
				data["device_bindings"] = bindings
			}
		}

	case "visitor":
		// 访客数据主要来自会话和扫码日志
		var scanLogs []models.QrScanLog
		sqls.DB().Where("tenant_id = ? AND visitor_id = ?", tenantID, subjectID).Limit(500).Find(&scanLogs)
		if len(scanLogs) > 0 {
			data["scan_logs"] = scanLogs
		}
		// 访客入口会话
		var entrySessions []models.CustomerEntrySession
		sqls.DB().Where("tenant_id = ? AND visitor_id = ?", tenantID, subjectID).Limit(500).Find(&entrySessions)
		if len(entrySessions) > 0 {
			data["entry_sessions"] = entrySessions
		}
		var privacyConsents []models.CustomerPrivacyConsent
		sqls.DB().Where("tenant_id = ? AND visitor_id = ?", tenantID, subjectID).Limit(500).Find(&privacyConsents)
		if len(privacyConsents) > 0 {
			data["privacy_consents"] = privacyConsents
		}
	}

	// 收集审计日志
	var auditLogs []models.AuditLog
	sqls.DB().Where("tenant_id = ? AND resource_id = ?", tenantID, subjectID).Limit(500).Find(&auditLogs)
	if len(auditLogs) > 0 {
		data["audit_logs"] = auditLogs
	}

	return data
}

// ExecuteDataDeletion 执行被遗忘权删除（GDPR Art 17）
// 物理删除 PII 字段，匿名化保留业务记录
func (s *dsarService) ExecuteDataDeletion(ctx context.Context, tenantID, dsarID string) error {
	var subjectID string
	err := sqls.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var req models.DSARRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenantID, dsarID).First(&req).Error; err != nil {
			return errorsx.BusinessError(1, "dsar request not found")
		}
		if req.Status != "processing" && req.Status != "pending" {
			return errorsx.BusinessError(2, "dsar request is not in deletable state")
		}
		deletedFields, err := s.deletePersonalDataDB(tx, req.TenantID, req.SubjectType, req.SubjectID)
		if err != nil {
			return err
		}
		now := time.Now()
		if err := tx.Model(&models.DSARRequest{}).Where("tenant_id = ? AND id = ?", tenantID, dsarID).Updates(map[string]any{
			"status": "completed", "subject_email": "", "completed_at": now, "updated_at": now,
		}).Error; err != nil {
			return err
		}
		if err := s.recordExecutionLogDB(tx, dsarID, "deleted", "personal_data", req.SubjectID,
			fmt.Sprintf(`{"subject_type":"%s","anonymized_fields":%d}`, req.SubjectType, deletedFields)); err != nil {
			return err
		}
		subjectID = req.SubjectID
		return nil
	})
	if err != nil {
		return err
	}
	slog.Info("dsar: data deletion completed", "dsar_id", dsarID, "subject_id", subjectID)
	return nil
}

// deletePersonalData 删除指定数据主体的个人数据（匿名化处理）
func (s *dsarService) deletePersonalDataDB(db *gorm.DB, tenantID, subjectType, subjectID string) (int, error) {
	if subjectType == "customer_account" {
		return s.deleteCustomerAccountDataDB(db, subjectID)
	}
	deletedCount := 0
	tenantIDInt, err := strconv.ParseInt(tenantID, 10, 64)
	if err != nil || tenantIDInt <= 0 {
		return 0, errorsx.InvalidParam("invalid tenant_id")
	}
	if err := s.validateSubjectOwnershipDB(db, tenantID, subjectType, subjectID); err != nil {
		return 0, err
	}

	switch subjectType {
	case "customer":
		if s.customerReferencedByOtherTenantDB(db, tenantID, subjectID) {
			return 0, errorsx.BusinessError(3, "customer is shared by multiple tenants; manual privacy review is required")
		}
		// 匿名化 Customer PII 字段
		if err := db.Model(&models.Customer{}).Where("id = ?", subjectID).Updates(map[string]any{
			"name": "ANONYMIZED_" + subjectID, "primary_mobile": "", "primary_email": "",
		}).Error; err != nil {
			return deletedCount, err
		}
		deletedCount++

		// 删除联系方式
		if err := db.Model(&models.CustomerContact{}).
			Where("customer_id = ?", subjectID).
			Delete(&models.CustomerContact{}).Error; err != nil {
			return deletedCount, err
		}
		deletedCount++

		// 删除身份映射
		if err := db.Model(&models.CustomerIdentity{}).
			Where("customer_id = ?", subjectID).
			Delete(&models.CustomerIdentity{}).Error; err != nil {
			return deletedCount, err
		}
		deletedCount++

	case "enterprise_user":
		if err := db.Model(&models.TenantMember{}).
			Where("tenant_id = ? AND user_id = ?", tenantIDInt, subjectID).
			Updates(map[string]any{"display_name": "ANONYMIZED_" + subjectID, "member_no": "", "job_title": ""}).Error; err != nil {
			return deletedCount, err
		}
		deletedCount++
		var otherMemberships int64
		db.Model(&models.TenantMember{}).Where("tenant_id <> ? AND user_id = ?", tenantIDInt, subjectID).Count(&otherMemberships)
		if otherMemberships == 0 {
			if err := db.Model(&models.User{}).Where("id = ?", subjectID).Updates(map[string]any{
				"nickname": "ANONYMIZED_" + subjectID, "mobile": nil, "email": nil,
			}).Error; err != nil {
				return deletedCount, err
			}
			if err := db.Where("user_id = ?", subjectID).Delete(&models.UserIdentity{}).Error; err != nil {
				return deletedCount, err
			}
			deletedCount++
		}

	case "customer_user":
		customerUserID, parseErr := strconv.ParseInt(subjectID, 10, 64)
		if parseErr != nil || customerUserID <= 0 {
			return deletedCount, errorsx.InvalidParam("invalid customer user subject_id")
		}
		var customerUser models.CustomerUser
		if err := db.Where("tenant_id = ? AND id = ?", tenantIDInt, customerUserID).First(&customerUser).Error; err != nil {
			return deletedCount, errorsx.InvalidParam("customer user subject not found for this tenant")
		}
		now := time.Now()
		if err := db.Model(&models.CustomerUser{}).Where("tenant_id = ? AND id = ?", tenantIDInt, customerUserID).Updates(map[string]any{
			"display_name": "Deleted account", "email": "", "phone": "", "status": enums.StatusDeleted,
			"last_seen_at": nil, "update_user_id": customerUser.UserID, "update_user_name": "account_deletion", "updated_at": now,
		}).Error; err != nil {
			return deletedCount, err
		}
		deletedCount++

		if err := s.deleteCustomerAccountTenantDataDB(db, tenantIDInt, customerUser, now); err != nil {
			return deletedCount, err
		}
		deletedCount++

		otherCustomerAccounts := s.countOtherActiveCustomerAccountsDB(db, customerUser.UserID, customerUserID)
		if otherCustomerAccounts == 0 {
			if err := s.deleteLegacyCustomerIdentityDB(db, tenantID, customerUser.UserID); err != nil {
				return deletedCount, err
			}
			deletedCount++
		}

		if !s.hasOtherActiveUserContextDB(db, customerUser.UserID, customerUserID) {
			anonymizedUsername := fmt.Sprintf("deleted_%d_%s", customerUser.UserID, uuid.NewString()[:8])
			if err := db.Model(&models.User{}).Where("id = ?", customerUser.UserID).Updates(map[string]any{
				"username": anonymizedUsername, "nickname": "Deleted account", "avatar": "", "mobile": nil, "email": nil,
				"password": "", "password_salt": "", "status": enums.StatusDeleted, "remark": "",
				"update_user_id": customerUser.UserID, "update_user_name": "account_deletion", "updated_at": now,
			}).Error; err != nil {
				return deletedCount, err
			}
			if db.Migrator().HasTable(&models.UserIdentity{}) {
				if err := db.Where("user_id = ?", customerUser.UserID).Delete(&models.UserIdentity{}).Error; err != nil {
					return deletedCount, err
				}
			}
			if err := s.revokeAccountSessionsDB(db, customerUser.UserID, 0, 0, now); err != nil {
				return deletedCount, err
			}
			deletedCount++
		} else if err := s.revokeAccountSessionsDB(db, customerUser.UserID, tenantIDInt, customerUserID, now); err != nil {
			return deletedCount, err
		}

	case "visitor":
		// 访客记录主要保留但标记匿名化
		anonymizedVisitorID := "ANONYMIZED_" + subjectID
		if err := db.Model(&models.CustomerEntrySession{}).
			Where("tenant_id = ? AND visitor_id = ?", tenantIDInt, subjectID).
			Update("visitor_id", anonymizedVisitorID).Error; err != nil {
			return deletedCount, err
		}
		deletedCount++

		if err := db.Model(&models.QrScanLog{}).
			Where("tenant_id = ? AND visitor_id = ?", tenantIDInt, subjectID).
			Update("visitor_id", anonymizedVisitorID).Error; err != nil {
			return deletedCount, err
		}
		deletedCount++

		// Consent receipts are retained as compliance evidence, but direct and
		// indirect visitor identifiers are removed on a deletion request.
		if err := db.Model(&models.CustomerPrivacyConsent{}).
			Where("tenant_id = ? AND visitor_id = ?", tenantIDInt, subjectID).
			Updates(map[string]any{
				"visitor_id": anonymizedVisitorID,
				"ip_address": "",
				"user_agent": "",
				"request_id": "",
			}).Error; err != nil {
			return deletedCount, err
		}
		deletedCount++
	}

	return deletedCount, nil
}

func (s *dsarService) deleteCustomerAccountTenantDataDB(db *gorm.DB, tenantID int64, customerUser models.CustomerUser, now time.Time) error {
	customerUserID := customerUser.ID
	userID := customerUser.UserID
	if db.Migrator().HasTable(&models.CustomerUserIdentity{}) {
		if err := db.Where("tenant_id = ? AND customer_user_id = ?", tenantID, customerUserID).Delete(&models.CustomerUserIdentity{}).Error; err != nil {
			return err
		}
	}
	if db.Migrator().HasTable(&models.CustomerUserRole{}) {
		if err := db.Where("tenant_id = ? AND customer_user_id = ?", tenantID, customerUserID).Delete(&models.CustomerUserRole{}).Error; err != nil {
			return err
		}
	}
	if db.Migrator().HasTable(&models.AuthRoleBinding{}) {
		if err := db.Model(&models.AuthRoleBinding{}).
			Where("tenant_id = ? AND domain_type = ? AND subject_type = ? AND subject_id = ?", tenantID, models.DomainTypeCustomer, models.SubjectTypeCustomerUser, customerUserID).
			Updates(map[string]any{"status": enums.StatusDeleted, "updated_at": now}).Error; err != nil {
			return err
		}
	}
	if db.Migrator().HasTable(&models.CustomerDeviceBinding{}) {
		if err := db.Model(&models.CustomerDeviceBinding{}).
			Where("tenant_id = ? AND customer_user_id = ?", tenantID, customerUserID).
			Updates(map[string]any{"status": enums.StatusDeleted, "update_user_id": userID, "update_user_name": "account_deletion", "updated_at": now}).Error; err != nil {
			return err
		}
	}
	if db.Migrator().HasTable(&models.CustomerEntrySession{}) {
		if err := db.Model(&models.CustomerEntrySession{}).
			Where("tenant_id = ? AND customer_user_id = ?", tenantID, customerUserID).
			Updates(map[string]any{"customer_user_id": 0, "updated_at": now}).Error; err != nil {
			return err
		}
	}
	if db.Migrator().HasTable(&models.DeviceRegistrationTask{}) {
		if err := db.Model(&models.DeviceRegistrationTask{}).
			Where("tenant_id = ? AND customer_user_id = ?", tenantID, customerUserID).
			Updates(map[string]any{"customer_user_id": 0, "updated_at": now}).Error; err != nil {
			return err
		}
	}
	if db.Migrator().HasTable(&models.TicketFeedback{}) {
		if err := db.Model(&models.TicketFeedback{}).
			Where("tenant_id = ? AND customer_user_id = ?", tenantID, customerUserID).
			Updates(map[string]any{"customer_user_id": 0, "updated_at": now}).Error; err != nil {
			return err
		}
	}
	if db.Migrator().HasTable(&models.MobilePushToken{}) {
		if err := db.Model(&models.MobilePushToken{}).
			Where("tenant_id = ? AND user_id = ? AND revoked_at IS NULL", tenantID, userID).
			Updates(map[string]any{"revoked_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
	}
	if db.Migrator().HasTable(&models.Notification{}) {
		if err := db.Where("tenant_id = ? AND recipient_user_id = ?", tenantID, userID).Delete(&models.Notification{}).Error; err != nil {
			return err
		}
	}
	if db.Migrator().HasTable(&models.NotificationPreference{}) {
		if err := db.Where("tenant_id = ? AND user_id = ?", tenantID, userID).Delete(&models.NotificationPreference{}).Error; err != nil {
			return err
		}
	}
	if db.Migrator().HasTable(&models.NotificationRecipientSetting{}) {
		if err := db.Where("tenant_id = ? AND user_id = ?", tenantID, userID).Delete(&models.NotificationRecipientSetting{}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *dsarService) deleteCustomerAccountDataDB(db *gorm.DB, subjectID string) (int, error) {
	userID, err := strconv.ParseInt(subjectID, 10, 64)
	if err != nil || userID <= 0 {
		return 0, errorsx.InvalidParam("invalid customer account subject_id")
	}

	var user models.User
	if err := db.Where("id = ? AND status = ?", userID, enums.StatusOk).First(&user).Error; err != nil {
		return 0, errorsx.InvalidParam("customer account subject not found")
	}

	now := time.Now()
	deletedCount := 0
	if db.Migrator().HasTable(&models.CustomerIdentity{}) {
		var identity models.CustomerIdentity
		identityResult := db.Where("external_source = ? AND external_id = ?", enums.ExternalSourceUser, subjectID).First(&identity)
		if identityResult.Error == nil {
			var otherIdentities int64
			db.Model(&models.CustomerIdentity{}).
				Where("customer_id = ? AND id <> ? AND status = ?", identity.CustomerID, identity.ID, enums.StatusOk).
				Count(&otherIdentities)
			if otherIdentities == 0 && db.Migrator().HasTable(&models.Customer{}) {
				if err := db.Model(&models.Customer{}).Where("id = ?", identity.CustomerID).Updates(map[string]any{
					"name": "Deleted account", "primary_mobile": "", "primary_email": "", "status": enums.StatusDeleted,
				}).Error; err != nil {
					return deletedCount, err
				}
				deletedCount++
				if db.Migrator().HasTable(&models.CustomerContact{}) {
					if err := db.Where("customer_id = ?", identity.CustomerID).Delete(&models.CustomerContact{}).Error; err != nil {
						return deletedCount, err
					}
					deletedCount++
				}
			}
			if err := db.Where("id = ?", identity.ID).Delete(&models.CustomerIdentity{}).Error; err != nil {
				return deletedCount, err
			}
			deletedCount++
		} else if !errors.Is(identityResult.Error, gorm.ErrRecordNotFound) {
			return deletedCount, identityResult.Error
		}
	}

	if err := db.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]any{
		"username":         "deleted_" + subjectID,
		"nickname":         "Deleted account",
		"avatar":           "",
		"mobile":           nil,
		"email":            nil,
		"password":         "",
		"password_salt":    "",
		"status":           enums.StatusDeleted,
		"remark":           "",
		"update_user_id":   userID,
		"update_user_name": "account_deletion",
		"updated_at":       now,
	}).Error; err != nil {
		return deletedCount, err
	}
	deletedCount++
	if db.Migrator().HasTable(&models.UserIdentity{}) {
		if err := db.Where("user_id = ?", userID).Delete(&models.UserIdentity{}).Error; err != nil {
			return deletedCount, err
		}
		deletedCount++
	}
	if err := s.revokeAccountSessionsDB(db, userID, 0, 0, now); err != nil {
		return deletedCount, err
	}
	return deletedCount, nil
}

func (s *dsarService) countOtherActiveCustomerAccountsDB(db *gorm.DB, userID, excludedCustomerUserID int64) int64 {
	if !db.Migrator().HasTable(&models.CustomerUser{}) {
		return 0
	}
	var count int64
	db.Model(&models.CustomerUser{}).
		Where("user_id = ? AND id <> ? AND status = ?", userID, excludedCustomerUserID, enums.StatusOk).
		Count(&count)
	return count
}

func (s *dsarService) hasOtherActiveUserContextDB(db *gorm.DB, userID, excludedCustomerUserID int64) bool {
	if s.countOtherActiveCustomerAccountsDB(db, userID, excludedCustomerUserID) > 0 {
		return true
	}
	for _, scope := range []struct {
		model any
	}{
		{model: &models.TenantMember{}},
		{model: &models.PartnerAccount{}},
		{model: &models.PlatformStaffProfile{}},
	} {
		if !db.Migrator().HasTable(scope.model) {
			continue
		}
		var count int64
		db.Model(scope.model).Where("user_id = ? AND status = ?", userID, enums.StatusOk).Count(&count)
		if count > 0 {
			return true
		}
	}
	return false
}

func (s *dsarService) deleteLegacyCustomerIdentityDB(db *gorm.DB, tenantID string, userID int64) error {
	if !db.Migrator().HasTable(&models.CustomerIdentity{}) {
		return nil
	}
	var identity models.CustomerIdentity
	if err := db.Where("external_source = ? AND external_id = ?", enums.ExternalSourceUser, strconv.FormatInt(userID, 10)).First(&identity).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	} else if err != nil {
		return err
	}
	if !s.customerReferencedByOtherTenantDB(db, tenantID, strconv.FormatInt(identity.CustomerID, 10)) {
		if db.Migrator().HasTable(&models.Customer{}) {
			if err := db.Model(&models.Customer{}).Where("id = ?", identity.CustomerID).Updates(map[string]any{
				"name": "Deleted account", "primary_mobile": "", "primary_email": "", "status": enums.StatusDeleted,
			}).Error; err != nil {
				return err
			}
		}
		if db.Migrator().HasTable(&models.CustomerContact{}) {
			if err := db.Where("customer_id = ?", identity.CustomerID).Delete(&models.CustomerContact{}).Error; err != nil {
				return err
			}
		}
	}
	return db.Where("id = ?", identity.ID).Delete(&models.CustomerIdentity{}).Error
}

func (s *dsarService) revokeAccountSessionsDB(db *gorm.DB, userID, tenantID, customerUserID int64, now time.Time) error {
	if db.Migrator().HasTable(&models.LoginSession{}) {
		query := db.Model(&models.LoginSession{}).Where("user_id = ? AND revoked_at IS NULL", userID)
		if tenantID > 0 && customerUserID > 0 {
			query = query.Where("domain_type = ? AND (tenant_id = ? OR subject_id = ?)", models.DomainTypeCustomer, tenantID, customerUserID)
		}
		if err := query.Updates(map[string]any{
			"revoked_at": now, "update_user_id": userID, "update_user_name": "account_deletion", "updated_at": now,
		}).Error; err != nil {
			return err
		}
	}
	if db.Migrator().HasTable(&models.AuthSession{}) {
		query := db.Model(&models.AuthSession{}).Where("user_id = ? AND revoked_at IS NULL", userID)
		if tenantID > 0 && customerUserID > 0 {
			query = query.Where("domain_type = ? AND (tenant_id = ? OR subject_id = ?)", models.DomainTypeCustomer, tenantID, customerUserID)
		}
		if err := query.Updates(map[string]any{
			"revoked_at": now, "update_user_id": userID, "update_user_name": "account_deletion", "updated_at": now,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

// ExecuteDataRestriction 限制数据处理（GDPR Art 18）
func (s *dsarService) ExecuteDataRestriction(ctx context.Context, tenantID, dsarID string) error {
	var req models.DSARRequest
	if err := sqls.DB().Where("tenant_id = ? AND id = ?", tenantID, dsarID).First(&req).Error; err != nil {
		return errorsx.BusinessError(1, "dsar request not found")
	}

	if req.RequestType != "restrict" {
		return errorsx.BusinessError(2, "dsar request type is not restriction")
	}

	now := time.Now()
	sqls.DB().Model(&models.DSARRequest{}).Where("tenant_id = ? AND id = ?", tenantID, dsarID).Updates(map[string]any{
		"status":       "completed",
		"completed_at": now,
		"updated_at":   now,
	})

	s.recordExecutionLog(dsarID, "restricted", "processing_restriction", req.SubjectID,
		fmt.Sprintf(`{"subject_type":"%s"}`, req.SubjectType))

	slog.Info("dsar: data restriction completed", "dsar_id", dsarID, "subject_id", req.SubjectID)
	return nil
}

// GetDSARStatus 查询 DSAR 请求状态
func (s *dsarService) GetDSARStatus(ctx context.Context, tenantID, dsarID string) (*models.DSARRequest, error) {
	var req models.DSARRequest
	if err := sqls.DB().Where("tenant_id = ? AND id = ?", tenantID, dsarID).First(&req).Error; err != nil {
		return nil, errorsx.BusinessError(1, "dsar request not found")
	}
	return &req, nil
}

func (s *dsarService) GetLatestSubjectRequest(ctx context.Context, tenantID, subjectType, subjectID, requestType string) (*models.DSARRequest, error) {
	var req models.DSARRequest
	err := sqls.DB().WithContext(ctx).
		Where("tenant_id = ? AND subject_type = ? AND subject_id = ? AND request_type = ?", tenantID, subjectType, subjectID, requestType).
		Order("requested_at DESC").First(&req).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &req, nil
}

// ListDSARRequests 查询 DSAR 请求列表（管理员）
func (s *dsarService) ListDSARRequests(ctx context.Context, tenantID string, page, pageSize int) ([]models.DSARRequest, int64, error) {
	if tenantID == "" {
		return nil, 0, errorsx.InvalidParam("tenant_id is required")
	}
	db := sqls.DB().Model(&models.DSARRequest{})
	db = db.Where("tenant_id = ?", tenantID)

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	var total int64
	db.Count(&total)

	var requests []models.DSARRequest
	db.Order("requested_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&requests)

	return requests, total, nil
}

// CheckDSARDeadlines 检查即将过期的 DSAR 请求，发送提醒（定时任务）
func (s *dsarService) CheckDSARDeadlines() error {
	now := time.Now()
	var failures []error
	// 查找 3 天内即将到期且尚未完成的 DSAR 请求
	var expiringRequests []models.DSARRequest
	if err := sqls.DB().
		Where("deadline_at <= ? AND deadline_at > ? AND status NOT IN (?, ?)",
			now.AddDate(0, 0, 3), now, "completed", "rejected").
		Find(&expiringRequests).Error; err != nil {
		return fmt.Errorf("query expiring DSAR requests: %w", err)
	}

	for _, req := range expiringRequests {
		remainingDays := int(req.DeadlineAt.Sub(now).Hours() / 24)
		slog.Warn("dsar: deadline approaching",
			"dsar_id", req.ID,
			"tenant_id", req.TenantID,
			"remaining_days", remainingDays,
			"deadline", req.DeadlineAt.Format(time.RFC3339),
		)

		// 查找租户管理员并发送通知
		adminUserIDs := findTenantAdminUserIDs(req.TenantID)
		tenantID, parseErr := strconv.ParseInt(req.TenantID, 10, 64)
		if parseErr != nil || tenantID <= 0 {
			failures = append(failures, fmt.Errorf("DSAR %s has invalid tenant_id %q", req.ID, req.TenantID))
			continue
		}
		for _, adminID := range adminUserIDs {
			_, err := NotificationQueueService.EnqueueCreate(context.Background(), request.CreateNotificationRequest{
				TenantID:         tenantID,
				RecipientUserID:  adminID,
				Title:            "DSAR 请求即将到期",
				Content:          fmt.Sprintf("DSAR 请求 %s（类型：%s）将在 %d 天后到期（截止日期：%s），请及时处理。", req.ID, req.RequestType, remainingDays, req.DeadlineAt.Format("2006-01-02")),
				NotificationType: "dsar_expiring",
				BizType:          "dsar",
				BizID:            0,
				ActionURL:        "/enterprise/audit",
				Category:         "system",
				Level:            "warning",
				Channels:         "in_app",
				IdempotencyKey:   fmt.Sprintf("dsar:expiring:%s:%d:%s", req.ID, adminID, now.Format("2006-01-02")),
			})
			if err != nil {
				failures = append(failures, fmt.Errorf("enqueue DSAR deadline notification: %w", err))
			}
		}
		if len(adminUserIDs) == 0 {
			slog.Warn("dsar: no tenant admin found for notification", "tenant_id", req.TenantID, "dsar_id", req.ID)
		}
	}

	// 检查已过期的 DSAR 请求
	var overdueRequests []models.DSARRequest
	if err := sqls.DB().
		Where("deadline_at < ? AND status NOT IN (?, ?)", now, "completed", "rejected").
		Find(&overdueRequests).Error; err != nil {
		return fmt.Errorf("query overdue DSAR requests: %w", err)
	}

	for _, req := range overdueRequests {
		slog.Error("dsar: deadline exceeded",
			"dsar_id", req.ID,
			"tenant_id", req.TenantID,
			"deadline", req.DeadlineAt.Format(time.RFC3339),
		)

		// 记录合规违规日志
		idempotencyKey := "dsar:compliance-breach:" + req.ID
		execLog := &models.DSARExecutionLog{
			ID:             uuid.NewString(),
			IdempotencyKey: &idempotencyKey,
			DSARID:         req.ID,
			Action:         "compliance_breach",
			TargetTable:    "dsar_requests",
			TargetID:       req.ID,
			Details:        fmt.Sprintf("DSAR 请求已超过法定处理期限，截止日期：%s", req.DeadlineAt.Format(time.RFC3339)),
			ExecutedAt:     now,
		}
		if err := sqls.DB().Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "idempotency_key"}},
			DoNothing: true,
		}).Create(execLog).Error; err != nil {
			failures = append(failures, fmt.Errorf("record DSAR compliance breach: %w", err))
		}

		// 通知租户管理员监管违规
		adminUserIDs := findTenantAdminUserIDs(req.TenantID)
		tenantID, parseErr := strconv.ParseInt(req.TenantID, 10, 64)
		if parseErr != nil || tenantID <= 0 {
			failures = append(failures, fmt.Errorf("DSAR %s has invalid tenant_id %q", req.ID, req.TenantID))
			continue
		}
		for _, adminID := range adminUserIDs {
			_, err := NotificationQueueService.EnqueueCreate(context.Background(), request.CreateNotificationRequest{
				TenantID:         tenantID,
				RecipientUserID:  adminID,
				Title:            "DSAR 请求已逾期 — 监管违规",
				Content:          fmt.Sprintf("DSAR 请求 %s（类型：%s）已超过法定处理期限（截止日期：%s），存在监管处罚风险，请立即处理。", req.ID, req.RequestType, req.DeadlineAt.Format("2006-01-02")),
				NotificationType: "dsar_overdue",
				BizType:          "dsar",
				BizID:            0,
				ActionURL:        "/enterprise/audit",
				Category:         "system",
				Level:            "urgent",
				Channels:         "in_app",
				IdempotencyKey:   fmt.Sprintf("dsar:overdue:%s:%d:%s", req.ID, adminID, now.Format("2006-01-02")),
			})
			if err != nil {
				failures = append(failures, fmt.Errorf("enqueue overdue DSAR notification: %w", err))
			}
		}
	}

	return errors.Join(failures...)
}

// recordExecutionLog 记录 DSAR 执行日志
func (s *dsarService) recordExecutionLog(dsarID, action, targetTable, targetID, details string) {
	if err := s.recordExecutionLogDB(sqls.DB(), dsarID, action, targetTable, targetID, details); err != nil {
		slog.Error("dsar: failed to record execution log", "dsar_id", dsarID, "error", err)
	}
}

func (s *dsarService) recordExecutionLogDB(db *gorm.DB, dsarID, action, targetTable, targetID, details string) error {
	idempotencyKey := "dsar:execution:" + dsarID + ":" + action
	now := time.Now()
	log := &models.DSARExecutionLog{
		ID:             uuid.NewString(),
		IdempotencyKey: &idempotencyKey,
		DSARID:         dsarID,
		Action:         action,
		TargetTable:    targetTable,
		TargetID:       targetID,
		Details:        details,
		ExecutedAt:     now,
		BaseModel: models.BaseModel{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "idempotency_key"}},
		DoNothing: true,
	}).Create(log).Error
}

// --- 辅助验证函数 ---

func isValidDSARSubjectType(t string) bool {
	switch t {
	case "customer", "customer_user", "customer_account", "enterprise_user", "visitor":
		return true
	}
	return false
}

func isValidDSARRequestType(t string) bool {
	switch t {
	case "access", "delete", "portability", "restrict":
		return true
	}
	return false
}

func (s *dsarService) validateSubjectOwnership(tenantID, subjectType, subjectID string) error {
	return s.validateSubjectOwnershipDB(sqls.DB(), tenantID, subjectType, subjectID)
}

func (s *dsarService) validateSubjectOwnershipDB(db *gorm.DB, tenantID, subjectType, subjectID string) error {
	if subjectType == "customer_account" {
		userID, parseErr := strconv.ParseInt(subjectID, 10, 64)
		if parseErr != nil || userID <= 0 {
			return errorsx.InvalidParam("invalid customer account subject_id")
		}
		var user models.User
		if err := db.Where("id = ? AND status = ?", userID, enums.StatusOk).First(&user).Error; err != nil {
			return errorsx.InvalidParam("customer account subject not found")
		}
		return nil
	}
	tenantIDInt, err := strconv.ParseInt(tenantID, 10, 64)
	if err != nil || tenantIDInt <= 0 {
		return errorsx.InvalidParam("invalid tenant_id")
	}
	if subjectID == "" {
		return errorsx.InvalidParam("subject_id is required")
	}
	switch subjectType {
	case "customer":
		subjectIDInt, parseErr := strconv.ParseInt(subjectID, 10, 64)
		if parseErr != nil || subjectIDInt <= 0 {
			return errorsx.InvalidParam("invalid customer subject_id")
		}
		var customer models.Customer
		if err := db.Where("id = ?", subjectIDInt).First(&customer).Error; err != nil {
			return errorsx.InvalidParam("customer subject not found")
		}
		var references int64
		db.Model(&models.Ticket{}).Where("tenant_id = ? AND customer_id = ?", tenantIDInt, subjectIDInt).Count(&references)
		if references == 0 {
			db.Model(&models.Conversation{}).Where("tenant_id = ? AND customer_id = ?", tenantIDInt, subjectIDInt).Count(&references)
		}
		if references == 0 {
			return errorsx.InvalidParam("customer subject not found for this tenant")
		}
	case "enterprise_user":
		subjectIDInt, parseErr := strconv.ParseInt(subjectID, 10, 64)
		if parseErr != nil || subjectIDInt <= 0 {
			return errorsx.InvalidParam("invalid enterprise user subject_id")
		}
		var member models.TenantMember
		if err := db.Where("tenant_id = ? AND user_id = ?", tenantIDInt, subjectIDInt).First(&member).Error; err != nil {
			return errorsx.InvalidParam("enterprise user subject not found for this tenant")
		}
	case "customer_user":
		subjectIDInt, parseErr := strconv.ParseInt(subjectID, 10, 64)
		if parseErr != nil || subjectIDInt <= 0 {
			return errorsx.InvalidParam("invalid customer user subject_id")
		}
		var customerUser models.CustomerUser
		if err := db.Where("tenant_id = ? AND id = ? AND status = ?", tenantIDInt, subjectIDInt, enums.StatusOk).First(&customerUser).Error; err != nil {
			return errorsx.InvalidParam("customer user subject not found for this tenant")
		}
	case "visitor":
		var references int64
		db.Model(&models.CustomerEntrySession{}).Where("tenant_id = ? AND visitor_id = ?", tenantIDInt, subjectID).Count(&references)
		if references == 0 {
			db.Model(&models.QrScanLog{}).Where("tenant_id = ? AND visitor_id = ?", tenantIDInt, subjectID).Count(&references)
		}
		if references == 0 {
			return errorsx.InvalidParam("visitor subject not found for this tenant")
		}
	default:
		return errorsx.InvalidParam("invalid subject_type: " + subjectType)
	}
	return nil
}

func (s *dsarService) customerReferencedByOtherTenant(tenantID, subjectID string) bool {
	return s.customerReferencedByOtherTenantDB(sqls.DB(), tenantID, subjectID)
}

func (s *dsarService) customerReferencedByOtherTenantDB(db *gorm.DB, tenantID, subjectID string) bool {
	tenantIDInt, err := strconv.ParseInt(tenantID, 10, 64)
	if err != nil || tenantIDInt <= 0 {
		return true
	}
	subjectIDInt, err := strconv.ParseInt(subjectID, 10, 64)
	if err != nil || subjectIDInt <= 0 {
		return true
	}
	var references int64
	db.Model(&models.Ticket{}).Where("tenant_id <> ? AND customer_id = ?", tenantIDInt, subjectIDInt).Count(&references)
	if references > 0 {
		return true
	}
	db.Model(&models.Conversation{}).Where("tenant_id <> ? AND customer_id = ?", tenantIDInt, subjectIDInt).Count(&references)
	return references > 0
}

// findTenantAdminUserIDs finds tenant administrators from the current IAM graph.
func findTenantAdminUserIDs(tenantID string) []int64 {
	parsedTenantID, parseErr := strconv.ParseInt(tenantID, 10, 64)
	if parseErr != nil || parsedTenantID <= 0 {
		slog.Warn("failed to parse tenant id for admin lookup", "tenant_id", tenantID, "error", parseErr)
		return nil
	}

	type tenantAdminRow struct {
		UserID int64 `gorm:"column:user_id"`
	}
	var rows []tenantAdminRow
	now := time.Now()
	adminRoleCodes := []string{"tenant_owner", "tenant_admin", "tenant_admin_seed"}
	err := sqls.DB().
		Table("auth_role_bindings AS arb").
		Select("DISTINCT tm.user_id").
		Joins("JOIN auth_roles AS ar ON ar.id = arb.role_id AND ar.tenant_id = arb.tenant_id AND ar.domain_type = arb.domain_type").
		Joins("JOIN tenant_members AS tm ON tm.id = arb.subject_id AND tm.tenant_id = arb.tenant_id").
		Where("arb.tenant_id = ? AND arb.domain_type = ? AND arb.subject_type = ? AND arb.status = ?", parsedTenantID, models.DomainTypeEnterprise, models.SubjectTypeTenantMember, enums.StatusOk).
		Where("ar.code IN ? AND ar.status = ?", adminRoleCodes, enums.StatusOk).
		Where("(arb.effective_at IS NULL OR arb.effective_at <= ?) AND (arb.expired_at IS NULL OR arb.expired_at > ?)", now, now).
		Scan(&rows).Error
	if err != nil {
		slog.Error("failed to query tenant admin users", "tenant_id", tenantID, "error", err)
		return nil
	}

	userIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row.UserID > 0 {
			userIDs = append(userIDs, row.UserID)
		}
	}
	return userIDs
}
