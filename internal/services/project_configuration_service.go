package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/eventbus"
	"remotehelpdesk/internal/pkg/projectconfig"
	"remotehelpdesk/internal/pkg/secretstore"
	"remotehelpdesk/internal/repositories"
)

var ErrProjectConfigConflict = errors.New("配置已被其他操作修改，请重新加载；相同提交编号不能用于不同内容")

type projectConfigurationAppliedEvent struct {
	TenantID          int64  `json:"tenant_id"`
	Environment       string `json:"environment"`
	VersionID         int64  `json:"version_id"`
	PreviousVersionID int64  `json:"previous_version_id"`
	Digest            string `json:"digest"`
	ActorID           int64  `json:"actor_id"`
}

func init() {
	eventbus.RegisterDurable[projectConfigurationAppliedEvent]("project.configuration.applied")
}

// Origins of system-written versions. Their stored notes are a fixed audit
// record; clients render these versions from the origin in the reader's locale.
const (
	ProjectConfigOriginMigrationBaseline = "migration_baseline"
	ProjectConfigOriginInitialDeployment = "initial_deployment"
)

type ProjectConfigVersion struct {
	models.ProjectConfigurationVersion
	Origin     string                                 `json:"origin,omitempty"`
	Document   projectconfig.Document                 `json:"document"`
	Activation *models.ProjectConfigurationActivation `json:"activation,omitempty"`
}
type ProjectConfigView struct {
	DeploymentManaged bool                   `json:"deployment_managed"`
	ActiveVersionID   int64                  `json:"active_version_id"`
	Document          projectconfig.Document `json:"document"`
	Versions          []ProjectConfigVersion `json:"versions"`
	NextBeforeID      int64                  `json:"next_before_id"`
}
type ProjectConfigDraft struct {
	BaseVersionID int64                  `json:"base_version_id"`
	RequestKey    string                 `json:"request_key"`
	Note          string                 `json:"note"`
	Document      projectconfig.Document `json:"document"`
	SMTPPassword  string                 `json:"smtp_password,omitempty"`
	IMAPPassword  string                 `json:"imap_password,omitempty"`
}
type ProjectConfigValidationError struct{ Report projectconfig.Report }

func (e *ProjectConfigValidationError) Error() string { return "配置校验未通过" }

func requireProjectConfigOperator(tenantID int64, op *dto.AuthPrincipal) error {
	if op == nil || tenantID <= 0 || op.TenantID != tenantID || !op.HasPermission(constants.PermissionTicketUpdate.Code) {
		return errorsx.ForbiddenI18n("error.e0225")
	}
	return nil
}

func legacyProjectDocument(tenant *models.Tenant) (projectconfig.Document, error) {
	return projectconfig.Document{
		SchemaVersion: 1,
		TenantID:      tenant.ID,
		Environment:   projectconfig.Environment(),
		Projects:      []projectconfig.Project{},
		SecretRefs:    []string{},
	}, nil
}

func decodeProjectVersion(v models.ProjectConfigurationVersion) (ProjectConfigVersion, error) {
	doc, err := projectconfig.Decode([]byte(v.DocumentJSON))
	if err != nil {
		return ProjectConfigVersion{}, err
	}
	if projectconfig.Digest(doc) != v.Digest {
		return ProjectConfigVersion{}, errors.New("配置历史内容校验失败")
	}
	return ProjectConfigVersion{ProjectConfigurationVersion: v, Origin: projectConfigVersionOrigin(v), Document: doc}, nil
}

// Online drafts cannot use the system: prefix and always have a signed-in
// creator, so neither origin can be claimed through the draft API.
func projectConfigVersionOrigin(v models.ProjectConfigurationVersion) string {
	switch {
	case v.RequestKey == "system:baseline":
		return ProjectConfigOriginMigrationBaseline
	case v.CreatedBy == 0 && strings.HasPrefix(v.RequestKey, "initial:"):
		return ProjectConfigOriginInitialDeployment
	}
	return ""
}

func projectState(db *gorm.DB, tenantID int64) (models.ProjectConfigurationState, error) {
	var s models.ProjectConfigurationState
	err := db.Where("tenant_id = ? AND environment = ?", tenantID, projectconfig.Environment()).Limit(1).Find(&s).Error
	if err == nil && s.ID == 0 {
		return models.ProjectConfigurationState{TenantID: tenantID, Environment: projectconfig.Environment()}, nil
	}
	return s, err
}

func GetProjectConfiguration(tenantID int64) (*ProjectConfigView, error) {
	return GetProjectConfigurationHistory(tenantID, 0)
}

func GetProjectConfigurationHistory(tenantID, beforeID int64) (*ProjectConfigView, error) {
	result := ProjectConfigView{DeploymentManaged: projectconfig.DeploymentManaged()}
	err := sqls.DB().Transaction(func(db *gorm.DB) error {
		var tenant models.Tenant
		if err := db.First(&tenant, tenantID).Error; err != nil {
			return err
		}
		s, err := projectState(db, tenantID)
		if err != nil {
			return err
		}
		result.ActiveVersionID = s.ActiveVersionID
		if s.ActiveVersionID == 0 {
			result.Document, err = legacyProjectDocument(&tenant)
		} else {
			var v models.ProjectConfigurationVersion
			if err = db.Where("id = ? AND tenant_id = ? AND environment = ?", s.ActiveVersionID, tenantID, s.Environment).First(&v).Error; err == nil {
				var version ProjectConfigVersion
				version, err = decodeProjectVersion(v)
				result.Document = version.Document
			}
		}
		if err != nil {
			return err
		}
		var versions []models.ProjectConfigurationVersion
		query := db.Where("tenant_id = ? AND environment = ?", tenantID, s.Environment)
		if beforeID > 0 {
			query = query.Where("id < ?", beforeID)
		}
		if err := query.Order("id DESC").Limit(10).Find(&versions).Error; err != nil {
			return err
		}
		result.Versions = []ProjectConfigVersion{}
		if len(versions) == 100 {
			result.NextBeforeID = versions[len(versions)-1].ID
		}
		for _, v := range versions {
			version, err := decodeProjectVersion(v)
			if err != nil {
				return err
			}
			var a models.ProjectConfigurationActivation
			err = db.Where("version_id = ? AND tenant_id = ?", v.ID, tenantID).Limit(1).Find(&a).Error
			if err == nil && a.ID > 0 {
				version.Activation = &a
			} else if err != nil {
				return err
			}
			result.Versions = append(result.Versions, version)
		}
		return nil
	})
	return &result, err
}

// Locking the tenant serializes initial state creation as well as later writes.
func lockProjectScope(db *gorm.DB, tenantID int64) (models.Tenant, models.ProjectConfigurationState, error) {
	var tenant models.Tenant
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&tenant, tenantID).Error; err != nil {
		return tenant, models.ProjectConfigurationState{}, err
	}
	s, err := projectState(db, tenantID)
	if err == nil && s.ID == 0 {
		err = db.Create(&s).Error
	}
	return tenant, s, err
}

func SaveProjectConfigurationDraft(tenantID int64, input ProjectConfigDraft, op *dto.AuthPrincipal) (*ProjectConfigVersion, error) {
	return saveProjectConfigurationDraftDB(sqls.DB(), tenantID, input, op)
}

func saveProjectConfigurationDraftDB(sourceDB *gorm.DB, tenantID int64, input ProjectConfigDraft, op *dto.AuthPrincipal) (*ProjectConfigVersion, error) {
	if err := requireProjectConfigOperator(tenantID, op); err != nil {
		return nil, err
	}
	if input.Document.TenantID != tenantID || input.Document.Environment != projectconfig.Environment() {
		return nil, errorsx.InvalidParam("配置必须属于当前公司和运行环境")
	}
	// Passwords are write-only request fields. Store only a non-sensitive
	// database handle in the versioned document; the encrypted values are kept
	// in the existing tenant mail credential record.
	if input.Document.Runtime != nil {
		if strings.TrimSpace(input.SMTPPassword) != "" {
			input.Document.SecretRefs = removeLegacyMailSecretRefs(input.Document.SecretRefs)
			input.Document.Runtime.Mail.PasswordRef = "secret://managed-smtp"
			input.Document.SecretRefs = appendUniqueString(input.Document.SecretRefs, "secret://managed-smtp")
		}
		if input.Document.Runtime.Mail.IMAP != nil && strings.TrimSpace(input.IMAPPassword) != "" {
			input.Document.SecretRefs = removeLegacyMailSecretRefs(input.Document.SecretRefs)
			input.Document.Runtime.Mail.IMAP.PasswordRef = "secret://managed-imap"
			input.Document.SecretRefs = appendUniqueString(input.Document.SecretRefs, "secret://managed-imap")
		}
	}
	if err := projectconfig.ValidateDraftShape(input.Document); err != nil {
		return nil, errorsx.InvalidParam(err.Error())
	}
	if input.Document.Runtime != nil || len(input.Document.TicketWorkflows) > 0 {
		if err := RequireProjectRuntimeOperator(op); err != nil {
			return nil, err
		}
	}
	if input.BaseVersionID < 0 || len(input.RequestKey) < 8 || len(input.RequestKey) > 100 || strings.HasPrefix(input.RequestKey, "system:") || strings.TrimSpace(input.Note) == "" || utf8.RuneCountInString(input.Note) > 500 {
		return nil, errorsx.InvalidParam("请填写修改说明及有效的提交编号")
	}
	encoded, err := json.Marshal(input.Document)
	for _, ref := range input.Document.SecretRefs {
		if !projectconfig.ValidSecretReference(ref) {
			return nil, errorsx.InvalidParam("密钥字段只能填写 secret://名称，不能填写密码或旧的不可解析引用")
		}
	}
	if err != nil || len(encoded) > 256*1024 {
		return nil, errorsx.InvalidParam("配置不能超过 256 KiB")
	}
	// Drafts may contain invalid business rules, but unknown fields and plaintext
	// secrets cannot enter this typed document. Validation is mandatory at apply.
	var result ProjectConfigVersion
	err = sourceDB.Transaction(func(db *gorm.DB) error {
		tenant, s, err := lockProjectScope(db, tenantID)
		if err != nil {
			return err
		}
		if err := requireExistingWorkflowAdministratorDB(db, s.ActiveVersionID, op); err != nil {
			return err
		}
		var existing models.ProjectConfigurationVersion
		err = db.Where("tenant_id = ? AND environment = ? AND request_key = ?", tenantID, s.Environment, input.RequestKey).Limit(1).Find(&existing).Error
		if err == nil && existing.ID > 0 {
			if existing.BaseVersionID != input.BaseVersionID || existing.Digest != projectconfig.Digest(input.Document) || existing.Note != input.Note || !managedMailSecretsMatch(db, tenantID, input.SMTPPassword, input.IMAPPassword) {
				return ErrProjectConfigConflict
			}
			result, err = decodeProjectVersion(existing)
			return err
		}
		if err != nil {
			return err
		}
		if s.ActiveVersionID != input.BaseVersionID {
			return ErrProjectConfigConflict
		}
		if strings.TrimSpace(input.SMTPPassword) != "" || strings.TrimSpace(input.IMAPPassword) != "" {
			if err := saveManagedMailSecrets(db, tenantID, input.SMTPPassword, input.IMAPPassword); err != nil {
				return err
			}
		}
		// Preserve the real current legacy settings before the first managed write.
		var count int64
		if err := db.Model(&models.ProjectConfigurationVersion{}).Where("tenant_id = ? AND environment = ? AND request_key = ?", tenantID, s.Environment, "system:baseline").Count(&count).Error; err != nil {
			return err
		}
		if count == 0 && s.ActiveVersionID == 0 {
			baseline, err := legacyProjectDocument(&tenant)
			if err != nil {
				return err
			}
			b, _ := json.Marshal(baseline)
			v := models.ProjectConfigurationVersion{TenantID: tenantID, Environment: s.Environment, RequestKey: "system:baseline", DocumentJSON: string(b), Digest: projectconfig.Digest(baseline), Note: "迁移基线：保存版本管理启用前的现有规则；更早历史未记录", CreatedAt: time.Now().UTC()}
			v.CreatedByName = "系统迁移"
			if err := db.Create(&v).Error; err != nil {
				return err
			}
		}
		v := models.ProjectConfigurationVersion{TenantID: tenantID, Environment: s.Environment, RequestKey: input.RequestKey, BaseVersionID: input.BaseVersionID, DocumentJSON: string(encoded), Digest: projectconfig.Digest(input.Document), Note: input.Note, CreatedBy: op.UserID, CreatedAt: time.Now().UTC()}
		v.CreatedByName = op.Username
		if err := db.Create(&v).Error; err != nil {
			return err
		}
		result, err = decodeProjectVersion(v)
		return err
	})
	return &result, err
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func removeLegacyMailSecretRefs(values []string) []string {
	filtered := values[:0]
	for _, value := range values {
		if value == "secret://smtp-password" || value == "secret://imap-password" {
			continue
		}
		filtered = append(filtered, value)
	}
	return filtered
}

// saveManagedMailSecrets keeps credentials out of immutable configuration
// documents and API responses while retaining the existing encrypted storage.
func saveManagedMailSecrets(db *gorm.DB, tenantID int64, smtpPassword, imapPassword string) error {
	if tenantID <= 0 {
		return errorsx.InvalidParam("tenant is required")
	}
	existing := repositories.TenantMailSettingRepository.GetByTenantID(db, tenantID)
	updates := map[string]any{"updated_at": time.Now().UTC()}
	if strings.TrimSpace(smtpPassword) != "" {
		value, err := secretstore.Encrypt(strings.TrimSpace(smtpPassword))
		if err != nil {
			return err
		}
		updates["password"] = value
	}
	if strings.TrimSpace(imapPassword) != "" {
		value, err := secretstore.Encrypt(strings.TrimSpace(imapPassword))
		if err != nil {
			return err
		}
		updates["imap_password"] = value
	}
	if existing == nil {
		item := &models.TenantMailSetting{TenantID: tenantID, SMTPPort: 465, IMAPPort: 993, UseTLS: true, IMAPUseTLS: true, Status: int(enums.StatusOk), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
		if value, ok := updates["password"].(string); ok {
			item.Password = value
		}
		if value, ok := updates["imap_password"].(string); ok {
			item.IMAPPassword = value
		}
		return db.Create(item).Error
	}
	return db.Model(&models.TenantMailSetting{}).Where("id = ?", existing.ID).Updates(updates).Error
}

func managedMailSecretsMatch(db *gorm.DB, tenantID int64, smtpPassword, imapPassword string) bool {
	if strings.TrimSpace(smtpPassword) == "" && strings.TrimSpace(imapPassword) == "" {
		return true
	}
	setting := repositories.TenantMailSettingRepository.GetByTenantID(db, tenantID)
	if setting == nil {
		return false
	}
	if strings.TrimSpace(smtpPassword) != "" {
		plain, err := secretstore.Decrypt(setting.Password)
		if err != nil || plain != strings.TrimSpace(smtpPassword) {
			return false
		}
	}
	if strings.TrimSpace(imapPassword) != "" {
		plain, err := secretstore.Decrypt(setting.IMAPPassword)
		if err != nil || plain != strings.TrimSpace(imapPassword) {
			return false
		}
	}
	return true
}

func ApplyProjectConfiguration(tenantID, versionID int64, op *dto.AuthPrincipal) (*ProjectConfigVersion, error) {
	return applyProjectConfiguration(tenantID, versionID, op, func(_ models.ProjectConfigurationState, _ models.ProjectConfigurationVersion) error {
		if projectconfig.DeploymentManaged() {
			return errorsx.InvalidParam("当前环境的配置由部署管理，请导出已检查的草稿，由运维部署应用；页面不能直接修改生效版本")
		}
		return nil
	})
}

func applyProjectConfiguration(tenantID, versionID int64, op *dto.AuthPrincipal, guard func(models.ProjectConfigurationState, models.ProjectConfigurationVersion) error, withinTransaction ...func(*gorm.DB) error) (*ProjectConfigVersion, error) {
	return applyProjectConfigurationDB(sqls.DB(), tenantID, versionID, op, guard, withinTransaction...)
}

func applyProjectConfigurationDB(sourceDB *gorm.DB, tenantID, versionID int64, op *dto.AuthPrincipal, guard func(models.ProjectConfigurationState, models.ProjectConfigurationVersion) error, withinTransaction ...func(*gorm.DB) error) (*ProjectConfigVersion, error) {
	if err := requireProjectConfigOperator(tenantID, op); err != nil {
		return nil, err
	}
	var result ProjectConfigVersion
	err := sourceDB.Transaction(func(db *gorm.DB) error {
		_, s, err := lockProjectScope(db, tenantID)
		if err != nil {
			return err
		}
		if err := requireExistingWorkflowAdministratorDB(db, s.ActiveVersionID, op); err != nil {
			return err
		}
		var v models.ProjectConfigurationVersion
		if err := db.Where("id = ? AND tenant_id = ? AND environment = ?", versionID, tenantID, s.Environment).First(&v).Error; err != nil {
			return errorsx.InvalidParam("配置版本不存在或不属于当前公司及环境")
		}
		result, err = decodeProjectVersion(v)
		if err != nil {
			return err
		}
		if result.Document.Runtime != nil || len(result.Document.TicketWorkflows) > 0 {
			if err := RequireProjectRuntimeOperator(op); err != nil {
				return err
			}
		}
		if err := guard(s, v); err != nil {
			return err
		}
		for _, hook := range withinTransaction {
			if err := hook(db); err != nil {
				return err
			}
		}
		var previous models.ProjectConfigurationActivation
		err = db.Where("version_id = ? AND tenant_id = ?", versionID, tenantID).Limit(1).Find(&previous).Error
		if err == nil && previous.ID > 0 {
			result.Activation = &previous
			return nil
		}
		if err != nil {
			return err
		}
		if v.RequestKey == "system:baseline" {
			return errorsx.InvalidParam("恢复历史配置时请先另存为新草稿")
		}
		if v.BaseVersionID != s.ActiveVersionID {
			return ErrProjectConfigConflict
		}
		report := projectconfig.Validate(result.Document, tenantID, s.Environment, projectconfig.RuntimeSecretCheck)
		if !report.Valid {
			return &ProjectConfigValidationError{report}
		}
		if s.Environment == "production" {
			if err := RequireApprovedRetentionPolicies(db, v); err != nil {
				return errorsx.InvalidParam(err.Error())
			}
		}
		if err := applyProjectRuntimeDB(db, result.Document, op); err != nil {
			return err
		}
		updated := db.Model(&models.ProjectConfigurationState{}).Where("id = ? AND active_version_id = ?", s.ID, s.ActiveVersionID).Update("active_version_id", versionID)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrProjectConfigConflict
		}
		a := models.ProjectConfigurationActivation{TenantID: tenantID, VersionID: versionID, PreviousVersionID: s.ActiveVersionID, AppliedBy: op.UserID, AppliedAt: time.Now().UTC()}
		a.AppliedByName = op.Username
		if err := db.Create(&a).Error; err != nil {
			return err
		}
		result.Activation = &a
		_, err = eventbus.EnqueueTx(db, eventbus.DurableEvent{TenantID: tenantID, SchemaVersion: 1, EventType: "project.configuration.applied", Source: "project_configuration", AggregateID: fmt.Sprint(versionID), ActorID: fmt.Sprint(op.UserID), ActorType: "user", IdempotencyKey: fmt.Sprintf("project-config:%d:%s:%d", tenantID, s.Environment, versionID), Payload: projectConfigurationAppliedEvent{tenantID, s.Environment, versionID, s.ActiveVersionID, v.Digest, op.UserID}})
		return err
	})
	return &result, err
}
