package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/projectconfig"
)

const (
	RetentionApprovalPending  = "pending"
	RetentionApprovalApproved = "approved"
	RetentionApprovalRejected = "rejected"
	RetentionApprovalRevoked  = "revoked"
)

type RetentionApprovalView struct {
	models.ProjectRetentionApproval
	CanApprove bool `json:"can_approve"`
}

type retentionApprovalRequest struct {
	Comment string `json:"comment"`
}

func requireRetentionApprover(op *dto.AuthPrincipal) error {
	if op != nil && (op.HasRole(EnterpriseRoleOwner) || op.HasPermission(constants.PermissionDataRetentionApprove.Code)) {
		return nil
	}
	return errorsx.Forbidden("只有租户所有者可以批准数据保留策略")
}

func explicitRetentionError(doc projectconfig.Document) error {
	if doc.Environment != "production" || doc.Runtime == nil {
		return nil
	}
	r := doc.Runtime.Retention
	if strings.TrimSpace(r.DataRegion) == "" || r.Days <= 0 || r.ArchiveAfterDays < 0 || r.ArchiveAfterDays >= r.Days {
		return fmt.Errorf("生产环境必须填写有效的数据保留总期限和归档期限")
	}
	values := map[string]int{"ticket": r.TicketDays, "attachment": r.AttachmentDays, "event": r.EventDays, "audit": r.AuditDays, "metric": r.MetricDays, "log": r.LogDays, "report": r.ReportDays, "backup": r.BackupDays}
	for name, days := range values {
		if days <= 0 {
			return fmt.Errorf("生产环境必须明确填写 %s retention_days", name)
		}
	}
	return nil
}

func validateRetentionVersion(v models.ProjectConfigurationVersion) (projectconfig.Document, error) {
	doc, err := projectconfig.Decode([]byte(v.DocumentJSON))
	if err != nil {
		return doc, err
	}
	if projectconfig.Digest(doc) != v.Digest {
		return doc, fmt.Errorf("配置版本摘要不匹配")
	}
	if err := explicitRetentionError(doc); err != nil {
		return doc, err
	}
	if doc.Environment != "production" {
		return doc, fmt.Errorf("只有 production 配置需要 Retention 审批")
	}
	if doc.Runtime == nil || len(doc.Projects) == 0 {
		return doc, fmt.Errorf("生产配置必须包含项目和运营配置")
	}
	return doc, nil
}

func ListRetentionApprovals(tenantID, versionID int64, op *dto.AuthPrincipal) ([]RetentionApprovalView, error) {
	if versionID <= 0 {
		return []RetentionApprovalView{}, nil
	}
	var rows []models.ProjectRetentionApproval
	if err := sqls.DB().Where("tenant_id = ? AND version_id = ?", tenantID, versionID).Order("project_key ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	can := op != nil && (op.HasRole(EnterpriseRoleOwner) || op.HasPermission(constants.PermissionDataRetentionApprove.Code))
	result := make([]RetentionApprovalView, 0, len(rows))
	for _, row := range rows {
		result = append(result, RetentionApprovalView{ProjectRetentionApproval: row, CanApprove: can && row.Status == RetentionApprovalPending && row.SubmittedBy != op.UserID})
	}
	return result, nil
}

func SubmitRetentionApproval(tenantID, versionID int64, op *dto.AuthPrincipal) ([]RetentionApprovalView, error) {
	if err := RequireProjectRuntimeOperator(op); err != nil {
		return nil, err
	}
	var result []RetentionApprovalView
	err := sqls.DB().Transaction(func(db *gorm.DB) error {
		var v models.ProjectConfigurationVersion
		if err := db.Where("id = ? AND tenant_id = ?", versionID, tenantID).First(&v).Error; err != nil {
			return err
		}
		state, err := projectState(db, tenantID)
		if err != nil {
			return err
		}
		if state.ActiveVersionID == versionID {
			return errorsx.InvalidParam("已经生效的 Production 配置不能补提交审批")
		}
		var activation models.ProjectConfigurationActivation
		if err := db.Where("tenant_id = ? AND version_id = ?", tenantID, versionID).First(&activation).Error; err == nil {
			return errorsx.InvalidParam("已经应用过的配置版本不能提交 Production 审批")
		} else if err != gorm.ErrRecordNotFound {
			return err
		}
		doc, err := validateRetentionVersion(v)
		if err != nil {
			return errorsx.InvalidParam(err.Error())
		}
		now := time.Now().UTC()
		for _, project := range doc.Projects {
			var row models.ProjectRetentionApproval
			q := db.Where("tenant_id = ? AND environment = ? AND project_key = ? AND version_id = ?", tenantID, v.Environment, project.Key, versionID).First(&row)
			if q.Error != nil && q.Error != gorm.ErrRecordNotFound {
				return q.Error
			}
			if row.ID == 0 {
				row = models.ProjectRetentionApproval{TenantID: tenantID, Environment: v.Environment, ProjectKey: project.Key, VersionID: versionID, PolicyDigest: v.Digest, Status: RetentionApprovalPending, SubmittedBy: op.UserID, SubmittedByName: op.Username, SubmittedAt: now, CreatedAt: now, UpdatedAt: now}
				if err := db.Create(&row).Error; err != nil {
					return err
				}
			} else if row.PolicyDigest != v.Digest || row.Status != RetentionApprovalApproved {
				if err := db.Model(&row).Updates(map[string]any{"policy_digest": v.Digest, "status": RetentionApprovalPending, "submitted_by": op.UserID, "submitted_by_name": op.Username, "submitted_at": now, "reviewed_by": 0, "reviewed_by_name": "", "reviewed_at": nil, "review_comment": "", "updated_at": now}).Error; err != nil {
					return err
				}
				row.PolicyDigest, row.Status, row.SubmittedBy, row.SubmittedByName, row.SubmittedAt, row.ReviewedBy, row.ReviewedByName, row.ReviewedAt, row.ReviewComment = v.Digest, RetentionApprovalPending, op.UserID, op.Username, now, 0, "", nil, ""
			}
			result = append(result, RetentionApprovalView{ProjectRetentionApproval: row, CanApprove: false})
		}
		return nil
	})
	if err == nil {
		if auditErr := AuditService.RecordAudit(context.Background(), RecordAuditInput{TenantID: tenantID, ActorID: strconv.FormatInt(op.UserID, 10), ActorType: "user", Domain: "settings", ResourceType: "project_retention_policy", ResourceID: strconv.FormatInt(versionID, 10), Action: "retention_policy.submitted", AfterState: map[string]any{"version_id": versionID, "status": RetentionApprovalPending}}); auditErr != nil {
			slog.Warn("retention approval audit failed", "version_id", versionID, "error", auditErr)
		}
	}
	return result, err
}

func ReviewRetentionApproval(tenantID, approvalID int64, approved bool, comment string, op *dto.AuthPrincipal) (*RetentionApprovalView, error) {
	if err := requireRetentionApprover(op); err != nil {
		return nil, err
	}
	var result RetentionApprovalView
	err := sqls.DB().Transaction(func(db *gorm.DB) error {
		var row models.ProjectRetentionApproval
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND tenant_id = ?", approvalID, tenantID).First(&row).Error; err != nil {
			return err
		}
		if row.Status != RetentionApprovalPending {
			return errorsx.InvalidParam("该审批不是待审批状态")
		}
		if row.SubmittedBy == op.UserID {
			return errorsx.InvalidParam("配置提交人不能批准自己的 Retention Policy")
		}
		var v models.ProjectConfigurationVersion
		if err := db.Where("id = ? AND tenant_id = ?", row.VersionID, tenantID).First(&v).Error; err != nil {
			return err
		}
		if v.Digest != row.PolicyDigest {
			return errorsx.InvalidParam("配置版本已变化，请重新提交审批")
		}
		if _, err := validateRetentionVersion(v); err != nil {
			return errorsx.InvalidParam(err.Error())
		}
		now := time.Now().UTC()
		status := RetentionApprovalRejected
		if approved {
			status = RetentionApprovalApproved
		}
		if err := db.Model(&row).Updates(map[string]any{"status": status, "reviewed_by": op.UserID, "reviewed_by_name": op.Username, "reviewed_at": now, "review_comment": strings.TrimSpace(comment), "updated_at": now}).Error; err != nil {
			return err
		}
		row.Status, row.ReviewedBy, row.ReviewedByName, row.ReviewedAt, row.ReviewComment, row.UpdatedAt = status, op.UserID, op.Username, &now, strings.TrimSpace(comment), now
		result = RetentionApprovalView{ProjectRetentionApproval: row, CanApprove: false}
		return nil
	})
	if err == nil {
		if auditErr := AuditService.RecordAudit(context.Background(), RecordAuditInput{TenantID: tenantID, ActorID: strconv.FormatInt(op.UserID, 10), ActorType: "user", Domain: "settings", ResourceType: "project_retention_policy", ResourceID: strconv.FormatInt(approvalID, 10), Action: "retention_policy." + statusForAudit(approved), AfterState: map[string]any{"approval_id": approvalID, "status": map[bool]string{true: RetentionApprovalApproved, false: RetentionApprovalRejected}[approved], "comment": strings.TrimSpace(comment)}}); auditErr != nil {
			slog.Warn("retention review audit failed", "approval_id", approvalID, "error", auditErr)
		}
	}
	return &result, err
}

func statusForAudit(approved bool) string {
	if approved {
		return "approved"
	}
	return "rejected"
}

func RequireApprovedRetentionPolicies(db *gorm.DB, v models.ProjectConfigurationVersion) error {
	doc, err := validateRetentionVersion(v)
	if err != nil {
		return err
	}
	for _, project := range doc.Projects {
		var count int64
		if err := db.Model(&models.ProjectRetentionApproval{}).Where("tenant_id = ? AND environment = ? AND project_key = ? AND version_id = ? AND policy_digest = ? AND status = ?", v.TenantID, "production", project.Key, v.ID, v.Digest, RetentionApprovalApproved).Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return fmt.Errorf("项目 %s 的 Retention Policy 尚未批准", project.Key)
		}
	}
	return nil
}

func decodeRetentionApprovalBody(data []byte) (retentionApprovalRequest, error) {
	var req retentionApprovalRequest
	if len(data) > 4096 {
		return req, errorsx.InvalidParam("审批意见过长")
	}
	if err := json.Unmarshal(data, &req); err != nil {
		return req, errorsx.InvalidParam("审批请求格式不正确")
	}
	return req, nil
}
