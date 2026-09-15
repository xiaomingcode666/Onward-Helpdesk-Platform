package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var TicketCaseOwnerService = &ticketCaseOwnerService{}

type ticketCaseOwnerService struct{}

type TicketCaseOwnerCandidate struct {
	UserID      int64  `json:"userId"`
	MemberID    int64  `json:"memberId"`
	DisplayName string `json:"displayName"`
	Username    string `json:"username"`
}

func canManageTicketCaseOwner(ticket *models.Ticket, operator *dto.AuthPrincipal) bool {
	if operator == nil || ticket == nil || ticket.MergedIntoID > 0 || ticket.TenantID <= 0 || operator.EffectiveTenantID() != ticket.TenantID {
		return false
	}
	if operator.IsCustomer() || operator.IsPartner() || operator.IsServiceAccount() {
		return false
	}
	if requireTicketTenantAccess(ticket, operator) != nil {
		return false
	}
	return ticket.CaseOwnerID > 0 && ticket.CaseOwnerID == operator.UserID ||
		canManageTicketDispatch(operator) || operator.HasPermission(constants.PermissionTicketUpdate.Code) ||
		operator.HasPermission(constants.PermissionTicketAssign.Code)
}

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
		return errorsx.InvalidParam("请选择负责跟进这张工单的客服")
	}
	var user models.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", ownerID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorsx.InvalidParam("管理负责人账号不存在或已删除")
		}
		return err
	}
	if user.Status != enums.StatusOk {
		return errorsx.InvalidParam("管理负责人账号已停用，请选择其他客服")
	}
	var member models.TenantMember
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, ownerID, enums.StatusOk).First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorsx.InvalidParam("管理负责人必须是本公司启用中的成员，不能选择客户或外部工程师账号")
		}
		return err
	}
	permissions, err := caseOwnerPermissionsDB(tx, tenantID, member.ID, user.ID)
	if err != nil {
		return err
	}
	if !hasCaseOwnerPermissions(permissions) {
		return errorsx.InvalidParam("管理负责人需要有工单处理权限，不能选择只读成员")
	}
	return nil
}

// RequireTicketCaseOwnerDB validates recorded responsibility without inventing a
// historical owner. Read-only access never calls this guard.
func RequireTicketCaseOwnerDB(tx *gorm.DB, ticket *models.Ticket) error {
	if ticket == nil || ticket.CaseOwnerID <= 0 {
		return errorsx.InvalidParam("这张工单尚未登记管理负责人，请先由客服确认受理或认领跟进")
	}
	return ValidateCaseOwnerDB(tx, ticket.TenantID, ticket.CaseOwnerID)
}

func (s *ticketCaseOwnerService) ListCandidates(tenantID int64, operator *dto.AuthPrincipal) ([]TicketCaseOwnerCandidate, error) {
	if operator == nil || tenantID <= 0 || operator.EffectiveTenantID() != tenantID ||
		operator.IsCustomer() || operator.IsPartner() || operator.IsServiceAccount() ||
		(!operator.HasPermission(constants.PermissionTicketView.Code) && !canManageTicketDispatch(operator)) {
		return nil, errorsx.Forbidden("没有查看本公司工单管理负责人的权限")
	}
	db := sqls.DB()
	var members []models.TenantMember
	if err := db.Where("tenant_id = ? AND status = ?", tenantID, enums.StatusOk).Order("id ASC").Find(&members).Error; err != nil {
		return nil, err
	}
	result := make([]TicketCaseOwnerCandidate, 0, len(members))
	for _, member := range members {
		var user models.User
		if err := db.Where("id = ? AND status = ?", member.UserID, enums.StatusOk).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, err
		}
		permissions, err := caseOwnerPermissionsDB(db, tenantID, member.ID, member.UserID)
		if err != nil {
			return nil, err
		}
		if !hasCaseOwnerPermissions(permissions) {
			continue
		}
		name := strings.TrimSpace(member.DisplayName)
		if name == "" {
			name = strings.TrimSpace(user.Nickname)
		}
		if name == "" {
			name = user.Username
		}
		result = append(result, TicketCaseOwnerCandidate{UserID: user.ID, MemberID: member.ID, DisplayName: name, Username: user.Username})
	}
	return result, nil
}

func (s *ticketCaseOwnerService) Transfer(ticketID, expectedOwnerID, nextOwnerID int64, reason string, operator *dto.AuthPrincipal) error {
	return s.transfer(ticketID, expectedOwnerID, nextOwnerID, reason, "", operator)
}

func (s *ticketCaseOwnerService) TransferWithKey(ticketID, expectedOwnerID, nextOwnerID int64, reason, idempotencyKey string, operator *dto.AuthPrincipal) error {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" || utf8.RuneCountInString(idempotencyKey) > 100 {
		return errorsx.InvalidParam("请提供管理负责人交接操作编号，最多 100 个字")
	}
	return s.transfer(ticketID, expectedOwnerID, nextOwnerID, reason, idempotencyKey, operator)
}

func (s *ticketCaseOwnerService) transfer(ticketID, expectedOwnerID, nextOwnerID int64, reason, idempotencyKey string, operator *dto.AuthPrincipal) error {
	reason = strings.TrimSpace(reason)
	if operator == nil || operator.UserID <= 0 || ticketID <= 0 || expectedOwnerID <= 0 || nextOwnerID <= 0 || expectedOwnerID == nextOwnerID {
		return errorsx.InvalidParam("请选择当前管理负责人和接替的客服")
	}
	if operator.EffectiveTenantID() <= 0 || operator.IsCustomer() || operator.IsPartner() || operator.IsServiceAccount() {
		return errorsx.Forbidden("无权交接工单管理负责人")
	}
	if reason == "" || utf8.RuneCountInString(reason) > 1000 {
		return errorsx.InvalidParam("请填写交接原因，最多 1000 个字")
	}
	payload, err := json.Marshal(struct {
		Action          string
		ExpectedOwnerID int64
		NextOwnerID     int64
		Reason          string
		ActorID         int64
	}{"transfer_case_owner", expectedOwnerID, nextOwnerID, reason, operator.UserID})
	if err != nil {
		return err
	}
	hash := sha256.Sum256(payload)
	payloadHash := hex.EncodeToString(hash[:])
	return sqls.DB().Transaction(func(tx *gorm.DB) error {
		var ticket models.Ticket
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND tenant_id = ?", ticketID, operator.EffectiveTenantID()).First(&ticket).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errorsx.InvalidParam("当前公司中没有这张工单")
			}
			return err
		}
		if idempotencyKey != "" {
			var operation models.TicketCaseOperation
			err := tx.Where("tenant_id = ? AND ticket_id = ? AND operation_key = ?", ticket.TenantID, ticket.ID, idempotencyKey).Take(&operation).Error
			if err == nil {
				if operation.PayloadHash != payloadHash {
					return ErrTicketCaseConflict
				}
				// This authenticated actor already completed the handover. They may
				// no longer be the owner; replay returns the receipt without mutation.
				return nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if !canManageTicketCaseOwner(&ticket, operator) {
			return errorsx.Forbidden("只有当前管理负责人或有工单管理权限的人员可以交接")
		}
		if ticket.CaseOwnerID != expectedOwnerID {
			return ErrTicketCaseConflict
		}
		if ticket.CaseStatus == "new" {
			return errorsx.InvalidParam("新工单请先由客服确认受理，再交接管理负责人")
		}
		if err := ValidateCaseOwnerDB(tx, ticket.TenantID, nextOwnerID); err != nil {
			return err
		}
		now := time.Now()
		updated := tx.Model(&models.Ticket{}).Where("id = ? AND tenant_id = ? AND case_owner_id = ?", ticket.ID, ticket.TenantID, expectedOwnerID).
			Updates(map[string]any{"case_owner_id": nextOwnerID, "case_revision": gorm.Expr("case_revision + 1"), "updated_at": now, "update_user_id": operator.UserID, "update_user_name": operator.Username})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrTicketCaseConflict
		}
		metadata, err := json.Marshal(map[string]any{"fromOwnerId": expectedOwnerID, "toOwnerId": nextOwnerID, "caseStatus": ticket.CaseStatus, "reason": reason})
		if err != nil {
			return err
		}
		if err := tx.Create(&models.TicketProgress{
			TenantID: ticket.TenantID, TicketID: ticket.ID, EventType: enums.TicketProgressEventType("case_owner_transferred"),
			Content: "客服管理负责人交接：" + reason, MetadataJSON: string(metadata), AuthorID: operator.UserID, CreatedAt: now,
		}).Error; err != nil {
			return err
		}
		if idempotencyKey != "" {
			return tx.Create(&models.TicketCaseOperation{TenantID: ticket.TenantID, TicketID: ticket.ID, OperationKey: idempotencyKey,
				PayloadHash: payloadHash, ResultStatus: models.EffectiveTicketCaseStatus(ticket), ResultRevision: ticket.CaseRevision + 1, CreatedAt: now}).Error
		}
		return nil
	})
}

// requireNoActiveCaseOwnershipDB locks only the user, never the ticket rows.
// Ownership writers take ticket -> user locks; using a plain count here avoids
// an inverse lock order while still preventing disable/accept races.
func requireNoActiveCaseOwnershipDB(tx *gorm.DB, userID int64) error {
	if tx == nil || userID <= 0 {
		return errorsx.InvalidParam("账号不存在")
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
		return errorsx.InvalidParam("该账号还有工单需要跟进，请先将管理负责人交接给其他客服，再停用或删除账号")
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
		return errorsx.InvalidParam("该成员仍有工单需要跟进，请先交接管理负责人，再撤销工单处理权限")
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
		return errorsx.InvalidParam("公司成员或角色权限正在更新，请稍后重试")
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
