package services

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/deploymentidentity"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/projectconfig"
)

// InitializeProjectConfigurationVersion is only used by the local provisioning
// command. The initial-state check is made under the same lock as activation.
func InitializeProjectConfigurationVersion(tenantID, versionID int64) (*ProjectConfigVersion, error) {
	identity, err := deploymentIdentityForScope(tenantID, projectconfig.Environment())
	if err != nil {
		return nil, err
	}
	op := &dto.AuthPrincipal{TenantID: tenantID, Username: "deployment-initializer", Permissions: []string{constants.PermissionTicketUpdate.Code, constants.PermissionTenantUpdate.Code}}
	return applyProjectConfiguration(tenantID, versionID, op, func(s models.ProjectConfigurationState, _ models.ProjectConfigurationVersion) error {
		if s.ActiveVersionID != 0 {
			return ErrProjectConfigConflict
		}
		return nil
	}, func(db *gorm.DB) error { return bindProjectDeploymentIdentityDB(db, identity) })
}

// ApplyProjectConfigurationDeployment is an explicit local deployment write,
// never called by HTTP or ordinary startup. Validate the mounted bundle before
// activating its exact stored draft; stale deployments cannot roll back state.
func ApplyProjectConfigurationDeployment() error {
	bundle, err := CheckProjectConfigurationDeploymentFile()
	if err != nil {
		return err
	}
	if bundle == nil {
		return fmt.Errorf("部署应用必须指定配置文件、公司、环境及摘要")
	}
	identity, err := deploymentIdentityForScope(bundle.Document.TenantID, bundle.Document.Environment)
	if err != nil {
		return err
	}
	op := &dto.AuthPrincipal{TenantID: bundle.Document.TenantID, Username: "deployment-operator", Permissions: []string{constants.PermissionTicketUpdate.Code, constants.PermissionTenantUpdate.Code}}
	_, err = applyProjectConfiguration(op.TenantID, bundle.VersionID, op, func(s models.ProjectConfigurationState, v models.ProjectConfigurationVersion) error {
		if v.Digest != bundle.Digest {
			return fmt.Errorf("部署配置内容与数据库版本不一致")
		}
		if s.ActiveVersionID != v.ID && s.ActiveVersionID != v.BaseVersionID {
			return ErrProjectConfigConflict
		}
		return nil
	}, func(db *gorm.DB) error { return bindProjectDeploymentIdentityDB(db, identity) })
	return err
}

// CheckProjectConfigurationDeploymentFile validates the bundle before migration.
// Existing local development can start without a pinned deployment bundle.
func CheckProjectConfigurationDeploymentFile() (*projectconfig.Deployment, error) {
	identity, err := deploymentidentity.FromEnvironment()
	if err != nil {
		return nil, err
	}
	path := os.Getenv("RHD_PROJECT_CONFIG_FILE")
	if path == "" {
		if identity != nil || os.Getenv("RHD_PROJECT_CONFIG_REQUIRED") == "1" || projectconfig.Environment() == "staging" || projectconfig.Environment() == "production" {
			return nil, fmt.Errorf("部署必须指定 RHD_PROJECT_CONFIG_FILE")
		}
		return nil, nil
	}
	tenantID, err := strconv.ParseInt(os.Getenv("RHD_PROJECT_CONFIG_TENANT_ID"), 10, 64)
	if err != nil || tenantID <= 0 {
		return nil, fmt.Errorf("部署必须指定有效的公司编号")
	}
	return projectconfig.ReadDeployment(path, tenantID, projectconfig.Environment(), os.Getenv("RHD_PROJECT_CONFIG_DIGEST"), os.Getenv("RHD_PROJECT_SECRET_DIR"))
}

func VerifyProjectConfigurationDeployment() error {
	if err := CheckDeploymentIdentityBeforeMigrations(sqls.DB(), false); err != nil {
		return err
	}
	bundle, err := CheckProjectConfigurationDeploymentFile()
	if err != nil || bundle == nil {
		return err
	}
	tenantID := bundle.Document.TenantID
	state, err := projectState(sqls.DB(), tenantID)
	if err != nil {
		return err
	}
	if state.ActiveVersionID != bundle.VersionID {
		return fmt.Errorf("部署配置版本与数据库当前生效版本不一致，请核对后重新部署")
	}
	var version models.ProjectConfigurationVersion
	if err := sqls.DB().Where("id = ? AND tenant_id = ? AND environment = ?", bundle.VersionID, tenantID, bundle.Document.Environment).First(&version).Error; err != nil {
		return err
	}
	parsed, err := decodeProjectVersion(version)
	if err != nil {
		return err
	}
	if parsed.Digest != bundle.Digest {
		return fmt.Errorf("部署配置内容与数据库版本不一致")
	}
	return nil
}

// CheckDeploymentIdentityBeforeMigrations must run immediately after opening the
// database, before migrations or background workers can write to a wrong target.
func CheckDeploymentIdentityBeforeMigrations(db *gorm.DB, allowInitialize bool) error {
	id, err := deploymentidentity.FromEnvironment()
	if err != nil {
		return err
	}
	return checkProjectDeploymentIdentityDB(db, id, allowInitialize)
}

func checkProjectDeploymentIdentityDB(db *gorm.DB, id *deploymentidentity.Identity, allowInitialize bool) error {
	if err := deploymentidentity.Check(db, id, allowInitialize); err != nil {
		return err
	}
	if id == nil {
		return nil
	}
	var boundCount int64
	if db.Migrator().HasTable(&deploymentidentity.Record{}) {
		if err := db.Model(&deploymentidentity.Record{}).Count(&boundCount).Error; err != nil {
			return err
		}
	}
	if boundCount > 0 || !db.Migrator().HasTable(&models.ProjectConfigurationState{}) {
		return nil
	}
	// Old FND-002 databases have configuration scopes but no instance record.
	// They must not be adopted by another environment before the first binding.
	var conflicting int64
	if err := db.Model(&models.ProjectConfigurationState{}).
		Where("active_version_id > 0 AND (tenant_id <> ? OR environment <> ?)", id.TenantID, id.Environment).
		Count(&conflicting).Error; err != nil {
		return err
	}
	if conflicting > 0 {
		return fmt.Errorf("未绑定实例的数据库已有其他公司或环境的生效配置，不能直接接管；请核对项目并先迁移到独立数据库")
	}
	return nil
}

func bindProjectDeploymentIdentityDB(db *gorm.DB, id *deploymentidentity.Identity) error {
	if err := checkProjectDeploymentIdentityDB(db, id, true); err != nil {
		return err
	}
	return deploymentidentity.Bind(db, id)
}

func deploymentIdentityForScope(tenantID int64, environment string) (*deploymentidentity.Identity, error) {
	id, err := deploymentidentity.FromEnvironment()
	if err != nil {
		return nil, err
	}
	if id != nil && (id.TenantID != tenantID || id.Environment != environment) {
		return nil, fmt.Errorf("部署实例与配置包的公司或环境不一致")
	}
	return id, nil
}

// InitializeProjectConfigurationDocument is only called by the explicit local
// initializer. A missing company requires an explicit display name and a usable
// administrator. Startup and upgrade never create companies. All rows commit together.
func InitializeProjectConfigurationDocument(doc projectconfig.Document, tenantName string) (*ProjectConfigVersion, error) {
	return InitializeProjectConfigurationDocumentWithOptions(doc, ProjectDeploymentInitialization{TenantName: tenantName})
}

func InitializeProjectConfigurationDocumentWithOptions(doc projectconfig.Document, options ProjectDeploymentInitialization) (*ProjectConfigVersion, error) {
	id, err := deploymentIdentityForScope(doc.TenantID, doc.Environment)
	if err != nil {
		return nil, err
	}
	report := projectconfig.Validate(doc, doc.TenantID, projectconfig.Environment(), projectconfig.RuntimeSecretCheck)
	if !report.Valid {
		return nil, &ProjectConfigValidationError{report}
	}
	op := &dto.AuthPrincipal{TenantID: doc.TenantID, Username: "deployment-initializer", Permissions: []string{constants.PermissionTicketUpdate.Code, constants.PermissionTenantUpdate.Code}}
	var result *ProjectConfigVersion
	err = sqls.DB().Transaction(func(db *gorm.DB) error {
		if err := checkProjectDeploymentIdentityDB(db, id, true); err != nil {
			return err
		}
		if err := ensureDeploymentTenantDB(db, doc.TenantID, options.TenantName); err != nil {
			return err
		}
		_, state, err := lockProjectScope(db, doc.TenantID)
		if err != nil {
			return err
		}
		if options.ExistingVersionID > 0 && options.ExistingVersionID != state.ActiveVersionID {
			return fmt.Errorf("已有输出配置不是此数据库的当前生效版本，不会修改公司或覆盖文件")
		}
		if state.ActiveVersionID > 0 {
			var version models.ProjectConfigurationVersion
			if err := db.Where("id = ? AND tenant_id = ? AND environment = ?", state.ActiveVersionID, doc.TenantID, doc.Environment).First(&version).Error; err != nil {
				return err
			}
			parsed, err := decodeProjectVersion(version)
			if err != nil {
				return err
			}
			if parsed.Digest != projectconfig.Digest(doc) {
				return fmt.Errorf("此公司及环境已有生效配置，首次配置命令不会替换它")
			}
			if err := ensureDeploymentAdministratorDB(db, doc.TenantID, options.Administrator, op); err != nil {
				return err
			}
			if err := bindProjectDeploymentIdentityDB(db, id); err != nil {
				return err
			}
			result = &parsed
			return nil
		}
		if err := ensureDeploymentAdministratorDB(db, doc.TenantID, options.Administrator, op); err != nil {
			return err
		}
		draft, err := saveProjectConfigurationDraftDB(db, doc.TenantID, ProjectConfigDraft{Document: doc, RequestKey: "initial:" + projectconfig.Digest(doc), Note: "本机部署工具首次初始化（系统操作）；未覆盖已有生效配置"}, op)
		if err != nil {
			return err
		}
		result, err = applyProjectConfigurationDB(db, doc.TenantID, draft.ID, op, func(s models.ProjectConfigurationState, _ models.ProjectConfigurationVersion) error {
			if s.ActiveVersionID != 0 {
				return ErrProjectConfigConflict
			}
			return nil
		}, func(tx *gorm.DB) error { return bindProjectDeploymentIdentityDB(tx, id) })
		return err
	})
	return result, err
}

func ensureDeploymentTenantDB(db *gorm.DB, tenantID int64, name string) error {
	var count int64
	if err := db.Model(&models.Tenant{}).Where("id = ?", tenantID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil // An operator's display-name argument must never rename a company.
	}
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 200 {
		return fmt.Errorf("指定公司不存在；首次安装请显式提供 -project-tenant-name（1–200 字），升级不会自动创建公司")
	}
	if db.Dialector.Name() == "postgres" {
		if err := db.Exec("LOCK TABLE ? IN SHARE ROW EXCLUSIVE MODE", clause.Table{Name: db.NamingStrategy.TableName("Tenant")}).Error; err != nil {
			return err
		}
	}
	now := time.Now().UTC()
	disabled := false
	tenant := models.Tenant{ID: tenantID, Name: name, DefaultLocale: "en", Timezone: "UTC", Status: enums.StatusOk, AIEnabled: &disabled,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now, CreateUserName: "deployment-initializer", UpdateUserName: "deployment-initializer"}}
	if err := db.Create(&tenant).Error; err != nil {
		return err
	}
	if db.Dialector.Name() == "postgres" {
		// Explicit IDs must not cause a later ordinary company creation to collide.
		table := db.NamingStrategy.TableName("Tenant")
		if err := db.Exec("SELECT setval(pg_get_serial_sequence(?, 'id'), GREATEST(?, nextval(pg_get_serial_sequence(?, 'id'))), true)", table, tenantID, table).Error; err != nil {
			return err
		}
	}
	return nil
}
