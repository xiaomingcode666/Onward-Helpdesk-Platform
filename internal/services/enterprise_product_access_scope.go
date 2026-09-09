package services

import (
	"slices"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

// enterpriseProductAccessScope is the canonical data scope for enterprise
// engineers. Product repair team membership grants product access; the tenant
// technical repair root remains a queue for records without a product.
type enterpriseProductAccessScope struct {
	Restricted bool
	UserID     int64
	TeamIDs    []int64
	ProductIDs []int64
}

func resolveEnterpriseProductAccessScope(tenantID int64, operator *dto.AuthPrincipal) enterpriseProductAccessScope {
	if operator == nil || operator.IsPlatform() || !operator.HasRole(EnterpriseRoleEngineer) || canManageTicketDispatch(operator) {
		return enterpriseProductAccessScope{}
	}

	scope := enterpriseProductAccessScope{Restricted: true, UserID: operator.UserID}
	scope.TeamIDs = AgentTeamMemberService.FindTeamIDsByUserID(sqls.DB(), tenantID, operator.UserID)
	for _, team := range AgentTeamService.FindByIds(scope.TeamIDs) {
		if team.TenantID != tenantID || team.TeamType != AgentTeamTypeProductRepair || team.Status != enums.StatusOk || team.ProductID <= 0 {
			continue
		}
		scope.ProductIDs = append(scope.ProductIDs, team.ProductID)
	}
	if memberID := enterpriseOperatorMemberID(tenantID, operator); memberID > 0 {
		for _, product := range repositories.ProductRepository.Find(sqls.DB(), sqls.NewCnd().
			Eq("tenant_id", tenantID).
			Eq("owner_member_id", memberID).
			Where("status <> ?", enums.StatusDeleted)) {
			scope.ProductIDs = append(scope.ProductIDs, product.ID)
		}
	}
	scope.TeamIDs = uniqueServiceInt64s(scope.TeamIDs)
	scope.ProductIDs = uniqueServiceInt64s(scope.ProductIDs)
	return scope
}

func (scope enterpriseProductAccessScope) canAccessProduct(productID int64) bool {
	return !scope.Restricted || (productID > 0 && slices.Contains(scope.ProductIDs, productID))
}

func (scope enterpriseProductAccessScope) canAccessTicket(ticket *models.Ticket) bool {
	if ticket == nil {
		return false
	}
	if !scope.Restricted || ticket.CurrentAssigneeID == scope.UserID {
		return true
	}
	if ticket.ProductID > 0 {
		return slices.Contains(scope.ProductIDs, ticket.ProductID)
	}
	return ticket.CurrentTeamID > 0 && slices.Contains(scope.TeamIDs, ticket.CurrentTeamID)
}

func RequireEnterpriseProductAccess(tenantID, productID int64, operator *dto.AuthPrincipal) error {
	if !resolveEnterpriseProductAccessScope(tenantID, operator).canAccessProduct(productID) {
		return errorsx.Forbidden("product is outside the engineer's authorized product scope")
	}
	return nil
}

func RequireEnterpriseProductEdit(tenantID, productID int64, operator *dto.AuthPrincipal) error {
	if CanEditEnterpriseProduct(tenantID, productID, operator) {
		return nil
	}
	return errorsx.Forbidden("only product owners or administrators can edit product information")
}

func CanEditEnterpriseProduct(tenantID, productID int64, operator *dto.AuthPrincipal) bool {
	if operator == nil || tenantID <= 0 || productID <= 0 {
		return false
	}
	if operator.HasPermission(constants.PermissionProductUpdate.Code) ||
		operator.HasRole(EnterpriseRoleOwner) ||
		operator.HasRole(EnterpriseRoleAdmin) ||
		operator.HasRole(EnterpriseRoleServiceManager) {
		return true
	}
	product := repositories.ProductRepository.GetByTenant(sqls.DB(), productID, tenantID)
	if product == nil || product.Status == enums.StatusDeleted || product.OwnerMemberID <= 0 {
		return false
	}
	return enterpriseOperatorMemberID(tenantID, operator) == product.OwnerMemberID
}

func enterpriseOperatorMemberID(tenantID int64, operator *dto.AuthPrincipal) int64 {
	if tenantID <= 0 || operator == nil {
		return 0
	}
	for _, memberID := range []int64{operator.MemberID, operator.SubjectID} {
		if memberID <= 0 {
			continue
		}
		member := repositories.EnterpriseIAMRepository.GetTenantMember(sqls.DB(), tenantID, memberID)
		if member == nil {
			continue
		}
		if operator.UserID <= 0 || member.UserID == operator.UserID {
			return member.ID
		}
	}
	if operator.UserID <= 0 {
		return 0
	}
	member := repositories.EnterpriseIAMRepository.FindTenantMemberByUserID(sqls.DB(), tenantID, operator.UserID)
	if member == nil || member.Status != enums.StatusOk {
		return 0
	}
	return member.ID
}
