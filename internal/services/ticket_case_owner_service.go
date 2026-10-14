package services

import (
	"errors"
	"slices"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func hasCaseOwnerPermissions(permissions []string) bool {
	return slices.Contains(permissions, constants.PermissionTicketUpdate.Code) ||
		slices.Contains(permissions, constants.PermissionTicketAssign.Code) ||
		slices.Contains(permissions, constants.PermissionTicketChangeStatus.Code)
}

func caseOwnerPermissionsDB(db *gorm.DB, tenantID, memberID, userID int64) ([]string, error) {
	if !db.Migrator().HasTable(&models.AuthRoleBinding{}) {
		return AuthService.GetTenantMemberPermissions(db, tenantID, memberID, userID)
	}
	bindings, err := repositories.PlatformIAMRepository.FindRoleBindings(db, tenantID, models.DomainTypeEnterprise, models.SubjectTypeTenantMember, memberID)
	if err != nil || len(bindings) == 0 {
		if err != nil {
			return nil, err
		}
		return AuthService.GetTenantMemberPermissions(db, tenantID, memberID, userID)
	}
	roleIDs := make([]int64, 0, len(bindings))
	for _, binding := range bindings {
		roleIDs = append(roleIDs, binding.RoleID)
	}
	var activeRoleIDs []int64
	if err := db.Model(&models.AuthRole{}).Where("id IN ? AND tenant_id = ? AND domain_type = ? AND status = ?", roleIDs, tenantID, models.DomainTypeEnterprise, enums.StatusOk).
		Pluck("id", &activeRoleIDs).Error; err != nil {
		return nil, err
	}
	rows, err := repositories.PlatformIAMRepository.FindAuthRolePermissionRows(db, tenantID, activeRoleIDs)
	if err != nil {
		return nil, err
	}
	allowed, denied := map[string]bool{}, map[string]bool{}
	for _, row := range rows {
		if row.Effect == "deny" {
			denied[row.PermissionCode] = true
			delete(allowed, row.PermissionCode)
		} else if !denied[row.PermissionCode] {
			allowed[row.PermissionCode] = true
		}
	}
	overrides, err := repositories.PlatformIAMRepository.FindSubjectPermissionOverrides(db, tenantID, models.DomainTypeEnterprise, models.SubjectTypeTenantMember, memberID)
	if err != nil {
		return nil, err
	}
	for _, override := range overrides {
		allowed[override.PermissionCode] = override.Effect != "deny"
	}
	permissions := make([]string, 0, len(allowed))
	for code, allow := range allowed {
		if allow {
			permissions = append(permissions, code)
		}
	}
	return permissions, nil
}

// ValidateCaseOwnerDB must run in the same transaction that accepts or hands over
// the ticket. The user lock serializes ownership changes with account disabling.
func ValidateCaseOwnerDB(tx *gorm.DB, tenantID, ownerID int64) error {
	if tx == nil || tenantID <= 0 || ownerID <= 0 {
		return errorsx.InvalidParamI18n("error.z0124")
	}
	var user models.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", ownerID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorsx.InvalidParamI18n("error.z0125")
		}
		return err
	}
	if user.Status != enums.StatusOk {
		return errorsx.InvalidParamI18n("error.z0126")
	}
	var member models.TenantMember
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, ownerID, enums.StatusOk).First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorsx.InvalidParamI18n("error.z0127")
		}
		return err
	}
	permissions, err := caseOwnerPermissionsDB(tx, tenantID, member.ID, user.ID)
	if err != nil {
		return err
	}
	if !hasCaseOwnerPermissions(permissions) {
		return errorsx.InvalidParamI18n("error.z0128")
	}
	return nil
}

// RequireTicketCaseOwnerDB validates recorded responsibility without inventing a
// historical owner. Read-only access never calls this guard.
func RequireTicketCaseOwnerDB(tx *gorm.DB, ticket *models.Ticket) error {
	if ticket == nil || ticket.CaseOwnerID <= 0 {
		return errorsx.InvalidParamI18n("error.z0129")
	}
	return ValidateCaseOwnerDB(tx, ticket.TenantID, ticket.CaseOwnerID)
}

// requireNoActiveCaseOwnershipDB locks only the user, never the ticket rows.
// Ownership writers take ticket -> user locks; using a plain count here avoids
// an inverse lock order while still preventing disable/accept races.
func requireNoActiveCaseOwnershipDB(tx *gorm.DB, userID int64) error {
	if tx == nil || userID <= 0 {
		return errorsx.InvalidParamI18n("error.z0130")
	}
	var user models.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", userID).First(&user).Error; err != nil {
		return err
	}
	if !tx.Migrator().HasTable(&models.Ticket{}) {
		return nil
	}
	var count int64
	if err := tx.Model(&models.Ticket{}).
		Where("case_owner_id = ? AND case_status NOT IN ?", userID, []string{"closed", "cancelled"}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errorsx.InvalidParamI18n("error.z0131")
	}
	return nil
}

func validateOwnedCasesAfterPermissionChangeDB(tx *gorm.DB, tenantID, userID int64) error {
	if !tx.Migrator().HasTable(&models.Ticket{}) {
		return nil
	}
	var count int64
	if err := tx.Model(&models.Ticket{}).Where("tenant_id = ? AND case_owner_id = ? AND case_status NOT IN ?", tenantID, userID, []string{"closed", "cancelled"}).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	if err := ValidateCaseOwnerDB(tx, tenantID, userID); err != nil {
		return errorsx.InvalidParamI18n("error.z0132")
	}
	return nil
}

func withCaseOwnerRoleGuardDB(tx *gorm.DB, role *models.AuthRole, mutate func() error) error {
	if role.DomainType != models.DomainTypeEnterprise || !tx.Migrator().HasTable(&models.Ticket{}) {
		return mutate()
	}
	return withCaseOwnerRolesGuardDB(tx, role.TenantID, []int64{role.ID}, mutate)
}

// IAM writers must serialize before discovering affected users. Otherwise a
// concurrent role assignment can add an owner after a role edit has collected
// its bindings, leaving that owner's last processing permission unguarded.
// Acquire before user locks where possible. Shared provisioning helpers may be
// entered with existing row locks, so contention must fail instead of waiting
// and introducing a user -> IAM -> user deadlock. Callers roll back the entire
// change and can retry against the newly committed permission state.
func lockCaseOwnerIAMChangesDB(tx *gorm.DB) error {
	if tx.Dialector.Name() != "postgres" {
		// SQLite serializes writers and rejects stale read-to-write upgrades.
		return nil
	}
	var acquired bool
	if err := tx.Raw("SELECT pg_try_advisory_xact_lock(hashtextextended(?, 0))", "ticket-case-owner:iam-changes").Scan(&acquired).Error; err != nil {
		return err
	}
	if !acquired {
		return errorsx.InvalidParamI18n("error.z0133")
	}
	return nil
}

// A refresh can modify several roles shared by the same members. Acquire all
// affected user locks in one order, then validate the final effective policies.
func withCaseOwnerRolesGuardDB(tx *gorm.DB, tenantID int64, roleIDs []int64, mutate func() error) error {
	if len(roleIDs) == 0 || !tx.Migrator().HasTable(&models.Ticket{}) {
		return mutate()
	}
	if err := lockCaseOwnerIAMChangesDB(tx); err != nil {
		return err
	}
	var bindings []models.AuthRoleBinding
	if err := tx.Where("tenant_id = ? AND domain_type = ? AND role_id IN ? AND subject_type = ?", tenantID, models.DomainTypeEnterprise, roleIDs, models.SubjectTypeTenantMember).
		Find(&bindings).Error; err != nil {
		return err
	}
	memberIDs := make([]int64, 0, len(bindings))
	for _, binding := range bindings {
		memberIDs = append(memberIDs, binding.SubjectID)
	}
	var members []models.TenantMember
	if len(memberIDs) > 0 {
		if err := tx.Where("tenant_id = ? AND id IN ?", tenantID, memberIDs).Order("user_id ASC").Find(&members).Error; err != nil {
			return err
		}
	}
	userIDs := make([]int64, 0, len(members))
	for _, member := range members {
		userIDs = append(userIDs, member.UserID)
	}
	return withCaseOwnerUsersGuardDB(tx, tenantID, userIDs, mutate)
}

func withCaseOwnerLegacyRoleGuardDB(tx *gorm.DB, roleID int64, mutate func() error) error {
	if !tx.Migrator().HasTable(&models.Ticket{}) {
		return mutate()
	}
	if err := lockCaseOwnerIAMChangesDB(tx); err != nil {
		return err
	}
	var userIDs []int64
	if err := tx.Model(&models.UserRole{}).Where("role_id = ?", roleID).Distinct().Pluck("user_id", &userIDs).Error; err != nil {
		return err
	}
	return withCaseOwnerUsersGuardDB(tx, 0, userIDs, mutate)
}

func withCaseOwnerUsersGuardDB(tx *gorm.DB, tenantID int64, userIDs []int64, mutate func() error) error {
	if !tx.Migrator().HasTable(&models.Ticket{}) {
		return mutate()
	}
	if err := lockCaseOwnerIAMChangesDB(tx); err != nil {
		return err
	}
	userIDs = slices.Clone(userIDs)
	slices.Sort(userIDs)
	userIDs = slices.Compact(userIDs)
	for _, userID := range userIDs {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", userID).First(&user).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}
	if err := mutate(); err != nil {
		return err
	}
	for _, userID := range userIDs {
		tenantIDs := []int64{tenantID}
		if tenantID <= 0 {
			if err := tx.Model(&models.Ticket{}).Where("case_owner_id = ? AND case_status NOT IN ?", userID, []string{"closed", "cancelled"}).
				Distinct().Pluck("tenant_id", &tenantIDs).Error; err != nil {
				return err
			}
		}
		for _, ownedTenantID := range tenantIDs {
			if err := validateOwnedCasesAfterPermissionChangeDB(tx, ownedTenantID, userID); err != nil {
				return err
			}
		}
	}
	return nil
}
