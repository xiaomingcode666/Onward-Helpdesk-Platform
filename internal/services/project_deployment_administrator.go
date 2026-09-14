package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
)

// These options belong to the explicit local installer, never to a versioned
// document or an HTTP request. Password is read from an operator-owned file.
type ProjectDeploymentAdministrator struct {
	UserID   int64
	Username string
	Password string
}

type ProjectDeploymentInitialization struct {
	TenantName        string
	Administrator     ProjectDeploymentAdministrator
	ExistingVersionID int64
}

func (a ProjectDeploymentAdministrator) Validate() error {
	if a.UserID < 0 || (a.UserID > 0 && (a.Username != "" || a.Password != "")) {
		return fmt.Errorf("管理员只能指定已有账号编号，或新账号名称及密码文件，不能同时指定")
	}
	if a.Username == "" && a.Password == "" {
		return nil
	}
	if a.Username != strings.TrimSpace(a.Username) || a.Username == "" || len(a.Username) > 100 || strings.ContainsAny(a.Username, "\r\n\t") || len(a.Password) < 12 || len(a.Password) > 72 || a.Password != strings.TrimSpace(a.Password) {
		return fmt.Errorf("新管理员需要有效账号名称和 12–72 字节的密码文件内容")
	}
	return nil
}

// The caller holds the company lock. Reuse the normal company-admin provisioner,
// while refusing account reuse or replacement that a retried install might hide.
func ensureDeploymentAdministratorDB(db *gorm.DB, tenantID int64, admin ProjectDeploymentAdministrator, op *dto.AuthPrincipal) error {
	if err := admin.Validate(); err != nil {
		return err
	}
	now := time.Now()
	var administrators []models.TenantMember
	err := db.Table("tenant_members AS tm").Distinct("tm.*").
		Joins("JOIN auth_role_bindings AS arb ON arb.tenant_id = tm.tenant_id AND arb.domain_type = ? AND arb.subject_type = ? AND arb.subject_id = tm.id AND arb.status = ?", models.DomainTypeEnterprise, models.SubjectTypeTenantMember, enums.StatusOk).
		Joins("JOIN auth_roles AS ar ON ar.id = arb.role_id AND ar.tenant_id = tm.tenant_id AND ar.status = ?", enums.StatusOk).
		Where("tm.tenant_id = ? AND tm.status = ? AND ar.code IN ?", tenantID, enums.StatusOk, []string{EnterpriseRoleOwner, EnterpriseRoleAdmin}).
		Where("(arb.effective_at IS NULL OR arb.effective_at <= ?) AND (arb.expired_at IS NULL OR arb.expired_at > ?)", now, now).
		Find(&administrators).Error
	if err != nil {
		return err
	}
	if len(administrators) > 0 {
		for _, member := range administrators {
			var user models.User
			if err := db.First(&user, member.UserID).Error; err != nil {
				return err
			}
			if (admin.UserID == 0 && admin.Username == "") || admin.UserID == user.ID || admin.Username == user.Username {
				if user.Status != enums.StatusOk || user.Password == "" {
					return fmt.Errorf("已有企业管理员账号不可登录，请使用正常账号恢复流程")
				}
				return nil // Never reset credentials or change a previous administrator.
			}
		}
		return fmt.Errorf("公司已有其他企业管理员，初始化不会替换管理员")
	}
	if admin.UserID == 0 && admin.Username == "" {
		return fmt.Errorf("公司缺少企业管理员；请指定 -project-admin-user-id，或 -project-admin-username 和 -project-admin-password-file")
	}
	var reserved int64
	if err := db.Model(&models.TenantMember{}).Where("tenant_id = ? AND member_type IN ?", tenantID, []string{"owner", "admin"}).Count(&reserved).Error; err != nil {
		return err
	}
	if reserved > 0 {
		return fmt.Errorf("公司已有停用或缺少授权的管理员，请使用正常账号恢复流程")
	}
	var user models.User
	if admin.UserID > 0 {
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, admin.UserID).Error; err != nil {
			return fmt.Errorf("指定管理员账号不存在")
		}
		if user.Status != enums.StatusOk || user.Password == "" {
			return fmt.Errorf("指定管理员账号已停用或没有登录密码")
		}
		// Include disabled memberships: installing an environment is not authority
		// to transfer an existing company's staff or customer account.
		for _, model := range []any{&models.TenantMember{}, &models.CustomerUser{}, &models.PartnerAccount{}} {
			var count int64
			if err := db.Model(model).Where("user_id = ?", user.ID).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return fmt.Errorf("指定账号已有企业、客户或外部协作身份，初始化不会改变其归属")
			}
		}
	} else {
		var existing int64
		if err := db.Unscoped().Model(&models.User{}).Where("username = ?", admin.Username).Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return fmt.Errorf("管理员账号名称已存在；绑定已有独立账号请显式使用 -project-admin-user-id")
		}
		created, _, err := UserService.CreateUserDB(db, request.CreateUserRequest{Username: admin.Username, Nickname: admin.Username, Password: admin.Password, Remark: "Deployment company administrator"}, op)
		if err != nil {
			return err
		}
		user = *created
	}
	if err := EnsureTenantDefaultIAMRolesDB(db, tenantID, op); err != nil {
		return err
	}
	ctx := &sqls.TxContext{Tx: db}
	if err := PlatformIAMService.initTenantAdminTx(ctx, tenantID, user.ID, op); err != nil {
		return err
	}
	return PlatformIAMService.recordAuthAuditTx(ctx, op, tenantID, models.DomainTypeEnterprise, "tenant_administrator", fmt.Sprint(user.ID), "tenant.administrator_initialized", nil, map[string]any{"user_id": user.ID}, models.RiskLevelCritical, "")
}
