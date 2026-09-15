// Package deploymentidentity prevents an isolated database from being reused
// by a different deployment project or environment.
package deploymentidentity

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Identity struct {
	InstanceID  string
	ProjectID   string
	Environment string
	TenantID    int64
}

// Record is deliberately separate from tenant settings and has no HTTP writer.
// A restored database keeps this identity until a deliberate restore procedure
// is reviewed; changing process environment variables cannot repurpose it.
type Record struct {
	ID          int64     `gorm:"primaryKey;autoIncrement:false;check:deployment_identity_singleton,id = 1"`
	InstanceID  string    `gorm:"type:varchar(100);not null"`
	ProjectID   string    `gorm:"type:varchar(63);not null"`
	Environment string    `gorm:"type:varchar(24);not null"`
	TenantID    int64     `gorm:"not null"`
	BoundAt     time.Time `gorm:"not null"`
}

func (Record) TableName() string { return "t_deployment_identity" }

var projectName = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)

func FromEnvironment() (*Identity, error) {
	instance := strings.TrimSpace(os.Getenv("RHD_INSTANCE_ID"))
	project := strings.TrimSpace(os.Getenv("RHD_DEPLOYMENT_PROJECT_ID"))
	if instance == "" && project == "" {
		return nil, nil // Existing local and FND-002 deployments remain compatible.
	}
	tenant, err := strconv.ParseInt(os.Getenv("RHD_PROJECT_CONFIG_TENANT_ID"), 10, 64)
	if err != nil || tenant <= 0 {
		return nil, fmt.Errorf("独立部署必须指定有效的 RHD_PROJECT_CONFIG_TENANT_ID")
	}
	id := &Identity{InstanceID: instance, ProjectID: project, Environment: strings.TrimSpace(os.Getenv("RHD_PROJECT_ENVIRONMENT")), TenantID: tenant}
	if err := id.Validate(); err != nil {
		return nil, err
	}
	return id, nil
}

func (id Identity) Validate() error {
	if len(id.ProjectID) > 63 || !projectName.MatchString(id.ProjectID) || id.TenantID <= 0 {
		return fmt.Errorf("部署项目标识必须是 1–63 位小写字母、数字和连字符，并指定有效公司编号")
	}
	expected := "onward-" + id.ProjectID + "-" + id.Environment
	switch id.Environment {
	case "integration":
		if id.ProjectID != "daypop-shared" {
			return fmt.Errorf("integration 仅用于 daypop-shared 共享联调项目")
		}
		expected = "onward-shared-integration"
	case "staging", "production":
		if id.ProjectID == "daypop-shared" || id.ProjectID == "shared" {
			return fmt.Errorf("共享联调项目不能作为客户的预发布或正式环境")
		}
	default:
		return fmt.Errorf("独立部署环境只能是 integration、staging 或 production")
	}
	if id.InstanceID != expected {
		return fmt.Errorf("部署实例标识与项目及环境不一致，应为 %s", expected)
	}
	return nil
}

// Check never creates tables or rows. allowUnbound is only for explicit local
// provisioning, which will bind identity in the configuration transaction.
func Check(db *gorm.DB, expected *Identity, allowUnbound bool) error {
	if db == nil {
		return fmt.Errorf("无法检查部署身份：数据库未初始化")
	}
	if expected != nil {
		if err := expected.Validate(); err != nil {
			return err
		}
	}
	var stored []Record
	if db.Migrator().HasTable(&Record{}) {
		if err := db.Limit(2).Find(&stored).Error; err != nil {
			return fmt.Errorf("无法读取数据库部署身份")
		}
	}
	if len(stored) == 0 {
		if expected != nil && !allowUnbound {
			return fmt.Errorf("数据库尚未绑定部署实例，请先执行显式配置初始化或 -migrate -apply-project-config")
		}
		return nil
	}
	if len(stored) != 1 || stored[0].ID != 1 {
		return fmt.Errorf("数据库部署身份记录异常")
	}
	if expected == nil {
		return fmt.Errorf("数据库已绑定独立部署实例，不能移除 RHD_INSTANCE_ID 或 RHD_DEPLOYMENT_PROJECT_ID 后启动")
	}
	s := stored[0]
	if s.InstanceID != expected.InstanceID || s.ProjectID != expected.ProjectID || s.Environment != expected.Environment || s.TenantID != expected.TenantID {
		return fmt.Errorf("数据库已绑定其他部署实例、项目、环境或公司，拒绝启动或迁移")
	}
	return nil
}

// Bind must run inside the transaction that applies the validated configuration.
// The singleton key makes concurrent first bindings and retries non-overwriting.
func Bind(db *gorm.DB, expected *Identity) error {
	if err := Check(db, expected, true); err != nil {
		return err
	}
	if expected == nil {
		return nil
	}
	if !db.Migrator().HasTable(&Record{}) {
		return fmt.Errorf("部署身份表不存在，请先执行已审核的数据库迁移")
	}
	r := Record{ID: 1, InstanceID: expected.InstanceID, ProjectID: expected.ProjectID, Environment: expected.Environment, TenantID: expected.TenantID, BoundAt: time.Now().UTC()}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&r).Error; err != nil {
		return fmt.Errorf("无法绑定数据库部署身份")
	}
	return Check(db, expected, false)
}
