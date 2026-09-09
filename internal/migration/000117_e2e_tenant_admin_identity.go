package migration

import (
	"errors"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

const e2eTenantAdminUsername = "e2e.tenant1.admin"

func init() {
	register(117, "repair E2E tenant administrator enterprise identity", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			return repairE2ETenantAdminIdentity(ctx.Tx)
		})
	})
}

func repairE2ETenantAdminIdentity(db *gorm.DB) error {
	if db == nil ||
		!db.Migrator().HasTable(&models.User{}) ||
		!db.Migrator().HasTable(&models.Tenant{}) ||
		!db.Migrator().HasTable(&models.TenantMember{}) ||
		!db.Migrator().HasTable(&models.AuthRole{}) ||
		!db.Migrator().HasTable(&models.AuthRoleBinding{}) {
		return nil
	}

	var user models.User
	err := db.Unscoped().Where("username = ?", e2eTenantAdminUsername).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	var tenant models.Tenant
	if err := db.Where("id = ? AND status <> ?", int64(1), enums.StatusDeleted).First(&tenant).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	if err := services.EnsureTenantDefaultIAMRolesDB(db, tenant.ID, nil); err != nil {
		return err
	}
	if _, err := services.ProductSupportOrganizationService.EnsureTenantTechnicalRepairTeamDB(db, tenant.ID, nil); err != nil {
		return err
	}

	now := time.Now()
	if user.DeletedAt.Valid || user.Status != enums.StatusOk {
		if err := db.Unscoped().Model(&models.User{}).Where("id = ?", user.ID).Updates(map[string]any{
			"deleted_at":       nil,
			"status":           enums.StatusOk,
			"updated_at":       now,
			"update_user_id":   constants.SystemAuditUserID,
			"update_user_name": "migration-117",
		}).Error; err != nil {
			return err
		}
	}

	root := repositories.EnterpriseIAMRepository.FindRootDepartment(db, tenant.ID)
	if root == nil {
		return errors.New("tenant root department not found")
	}

	member := &models.TenantMember{}
	err = db.Where("tenant_id = ? AND user_id = ?", tenant.ID, user.ID).First(member).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		member = &models.TenantMember{
			TenantID:     tenant.ID,
			UserID:       user.ID,
			DepartmentID: root.ID,
			DisplayName:  defaultE2ETenantAdminDisplayName(user),
			MemberType:   "owner",
			Status:       enums.StatusOk,
			JoinedAt:     &now,
			AuditFields: models.AuditFields{
				CreatedAt:      now,
				CreateUserID:   constants.SystemAuditUserID,
				CreateUserName: "migration-117",
				UpdatedAt:      now,
				UpdateUserID:   constants.SystemAuditUserID,
				UpdateUserName: "migration-117",
			},
		}
		if err := db.Create(member).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if member.Status != enums.StatusOk || member.DepartmentID <= 0 || member.MemberType == "" {
		updates := map[string]any{
			"status":           enums.StatusOk,
			"updated_at":       now,
			"update_user_id":   constants.SystemAuditUserID,
			"update_user_name": "migration-117",
		}
		if member.DepartmentID <= 0 {
			updates["department_id"] = root.ID
		}
		if member.MemberType == "" {
			updates["member_type"] = "owner"
		}
		if err := db.Model(&models.TenantMember{}).Where("id = ?", member.ID).Updates(updates).Error; err != nil {
			return err
		}
	}

	role := &models.AuthRole{}
	if err := db.Where(
		"tenant_id = ? AND domain_type = ? AND code = ? AND status = ?",
		tenant.ID, models.DomainTypeEnterprise, services.EnterpriseRoleOwner, enums.StatusOk,
	).First(role).Error; err != nil {
		return err
	}
	binding := &models.AuthRoleBinding{}
	err = db.Where(
		"tenant_id = ? AND domain_type = ? AND role_id = ? AND subject_type = ? AND subject_id = ?",
		tenant.ID, models.DomainTypeEnterprise, role.ID, models.SubjectTypeTenantMember, member.ID,
	).First(binding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(&models.AuthRoleBinding{
			TenantID: tenant.ID, DomainType: models.DomainTypeEnterprise, RoleID: role.ID,
			SubjectType: models.SubjectTypeTenantMember, SubjectID: member.ID, Status: enums.StatusOk,
			EffectiveAt: &now,
			AuditFields: models.AuditFields{
				CreatedAt:      now,
				CreateUserID:   constants.SystemAuditUserID,
				CreateUserName: "migration-117",
				UpdatedAt:      now,
				UpdateUserID:   constants.SystemAuditUserID,
				UpdateUserName: "migration-117",
			},
		}).Error
	}
	if err != nil {
		return err
	}
	return db.Model(&models.AuthRoleBinding{}).Where("id = ?", binding.ID).Updates(map[string]any{
		"status":           enums.StatusOk,
		"effective_at":     &now,
		"expired_at":       nil,
		"updated_at":       now,
		"update_user_id":   constants.SystemAuditUserID,
		"update_user_name": "migration-117",
	}).Error
}

func defaultE2ETenantAdminDisplayName(user models.User) string {
	if user.Nickname != "" {
		return user.Nickname
	}
	return user.Username
}
