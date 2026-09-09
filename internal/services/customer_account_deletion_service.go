package services

import (
	"context"
	"strconv"
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/errorsx"
)

const (
	customerAccountDeletionSubjectType        = "customer_user"
	pendingCustomerAccountDeletionSubjectType = "customer_account"
)

var CustomerAccountDeletionService = &customerAccountDeletionService{}

type customerAccountDeletionService struct{}

func (s *customerAccountDeletionService) GetStatus(ctx context.Context, principal *dto.AuthPrincipal) (*models.DSARRequest, error) {
	if err := validateCustomerAccountDeletionPrincipal(principal); err != nil {
		return nil, err
	}
	tenantID, subjectType, subjectID := customerAccountDeletionScope(principal)
	return DSARService.GetLatestSubjectRequest(
		ctx,
		tenantID,
		subjectType,
		subjectID,
		"delete",
	)
}

func (s *customerAccountDeletionService) Delete(
	ctx context.Context,
	principal *dto.AuthPrincipal,
	req request.DeleteCustomerPortalAccountRequest,
) (*models.DSARRequest, error) {
	if err := validateCustomerAccountDeletionPrincipal(principal); err != nil {
		return nil, err
	}
	if principal.SupportGrantID > 0 || strings.TrimSpace(principal.SupportMode) != "" || strings.TrimSpace(principal.ImpersonatedBy) != "" {
		return nil, errorsx.Forbidden("account deletion is unavailable during delegated access")
	}
	if req.Confirmation != "DELETE" {
		return nil, errorsx.InvalidParam("type DELETE to confirm account deletion")
	}
	if err := AuthService.VerifyPassword(principal.UserID, req.CurrentPassword); err != nil {
		return nil, err
	}

	tenantID, subjectType, subjectID := customerAccountDeletionScope(principal)
	item, err := DSARService.GetLatestSubjectRequest(ctx, tenantID, subjectType, subjectID, "delete")
	if err != nil {
		return nil, err
	}
	if item == nil || (item.Status != "pending" && item.Status != "awaiting_verification" && item.Status != "processing") {
		item, err = DSARService.SubmitDSAR(ctx, tenantID, subjectType, subjectID, "", "delete")
		if err != nil {
			return nil, err
		}
	}
	if item.Status == "pending" || item.Status == "awaiting_verification" {
		if err := DSARService.VerifyIdentity(ctx, tenantID, item.ID, "current_password"); err != nil {
			return nil, err
		}
	}
	if err := DSARService.ExecuteDataDeletion(ctx, tenantID, item.ID); err != nil {
		return nil, err
	}
	return DSARService.GetDSARStatus(ctx, tenantID, item.ID)
}

func validateCustomerAccountDeletionPrincipal(principal *dto.AuthPrincipal) error {
	if principal == nil || !principal.IsCustomer() || principal.UserID <= 0 {
		return errorsx.Forbidden("a signed-in customer account is required")
	}
	if principal.SubjectType == models.SubjectTypePendingCustomer {
		return nil
	}
	if principal.TenantID <= 0 || principal.SubjectType != models.SubjectTypeCustomerUser || principal.SubjectID <= 0 {
		return errorsx.Forbidden("a signed-in customer account is required")
	}
	return nil
}

func customerAccountDeletionScope(principal *dto.AuthPrincipal) (tenantID, subjectType, subjectID string) {
	if principal.SubjectType == models.SubjectTypePendingCustomer {
		return "0", pendingCustomerAccountDeletionSubjectType, strconv.FormatInt(principal.UserID, 10)
	}
	return strconv.FormatInt(principal.TenantID, 10), customerAccountDeletionSubjectType, strconv.FormatInt(principal.SubjectID, 10)
}
