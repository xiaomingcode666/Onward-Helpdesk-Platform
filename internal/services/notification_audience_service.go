package services

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var NotificationAudienceService = newNotificationAudienceService()

func newNotificationAudienceService() *notificationAudienceService {
	return &notificationAudienceService{}
}

type notificationAudienceService struct{}

type NotificationRecipient struct {
	UserID      int64
	MemberID    int64
	DisplayName string
	Permissions []string
}

func (s *notificationAudienceService) ResolveTenantRecipients(tenantID int64, permissions ...string) ([]NotificationRecipient, error) {
	if tenantID <= 0 {
		return []NotificationRecipient{}, nil
	}
	required := append([]string{constants.PermissionNotificationView.Code}, permissions...)
	return s.resolvePermissionRecipients(tenantID, required...)
}

func (s *notificationAudienceService) ResolveProductRecipients(product *models.Product, operatorUserID int64) ([]NotificationRecipient, error) {
	if product == nil || product.TenantID <= 0 {
		return []NotificationRecipient{}, nil
	}
	recipients, err := s.resolvePermissionRecipients(
		product.TenantID,
		constants.PermissionNotificationView.Code,
		constants.PermissionProductView.Code,
	)
	if err != nil {
		return nil, err
	}
	teamUserIDs := make(map[int64]bool)
	if team := ProductSupportOrganizationService.FindProductRepairTeam(sqls.DB(), product.TenantID, product.ID); team != nil {
		for _, membership := range AgentTeamMemberService.FindActiveMembersByTeamID(sqls.DB(), product.TenantID, team.ID) {
			teamUserIDs[membership.UserID] = true
		}
	}
	ret := make([]NotificationRecipient, 0, len(recipients))
	for _, recipient := range recipients {
		if recipient.UserID == operatorUserID ||
			recipient.MemberID == product.OwnerMemberID ||
			teamUserIDs[recipient.UserID] ||
			hasAnyNotificationPermission(recipient.Permissions, constants.PermissionProductCreate.Code, constants.PermissionProductUpdate.Code) {
			ret = append(ret, recipient)
		}
	}
	return ret, nil
}

func (s *notificationAudienceService) ResolveTicketRecipients(ticket *models.Ticket) ([]NotificationRecipient, error) {
	if ticket == nil || ticket.TenantID <= 0 {
		return []NotificationRecipient{}, nil
	}
	recipients, err := s.resolvePermissionRecipients(
		ticket.TenantID,
		constants.PermissionNotificationView.Code,
		constants.PermissionTicketView.Code,
	)
	if err != nil {
		return nil, err
	}
	teamUserIDs := make(map[int64]bool)
	if ticket.CurrentTeamID > 0 {
		for _, membership := range AgentTeamMemberService.FindActiveMembersByTeamID(sqls.DB(), ticket.TenantID, ticket.CurrentTeamID) {
			teamUserIDs[membership.UserID] = true
		}
	}
	ret := make([]NotificationRecipient, 0, len(recipients))
	for _, recipient := range recipients {
		if recipient.UserID == ticket.CurrentAssigneeID ||
			teamUserIDs[recipient.UserID] ||
			hasAnyNotificationPermission(recipient.Permissions, constants.PermissionTicketAssign.Code) {
			ret = append(ret, recipient)
		}
	}
	return ret, nil
}

func (s *notificationAudienceService) ResolveTicketAssignee(ticket *models.Ticket, userID int64) ([]NotificationRecipient, error) {
	if ticket == nil || userID <= 0 || ticket.CurrentAssigneeID != userID {
		return []NotificationRecipient{}, nil
	}
	return s.resolveCandidateRecipients(
		ticket.TenantID,
		map[int64]bool{userID: true},
		constants.PermissionNotificationView.Code,
		constants.PermissionTicketView.Code,
	)
}

func (s *notificationAudienceService) ResolveConversationAssignee(conversation *models.Conversation, userID int64) ([]NotificationRecipient, error) {
	if conversation == nil || userID <= 0 || conversation.CurrentAssigneeID != userID {
		return []NotificationRecipient{}, nil
	}
	return s.resolveCandidateRecipients(
		conversation.TenantID,
		map[int64]bool{userID: true},
		constants.PermissionNotificationView.Code,
		constants.PermissionConversationView.Code,
	)
}

func (s *notificationAudienceService) Deliver(recipients []NotificationRecipient, req request.CreateNotificationRequest) error {
	var errs []error
	seen := make(map[int64]bool, len(recipients))
	for _, recipient := range recipients {
		if recipient.UserID <= 0 || seen[recipient.UserID] {
			continue
		}
		seen[recipient.UserID] = true
		item := req
		item.RecipientUserID = recipient.UserID
		item.RecipientName = recipient.DisplayName
		if idempotencyKey := strings.TrimSpace(req.IdempotencyKey); idempotencyKey != "" {
			item.IdempotencyKey = idempotencyKey + ":recipient:" + strconv.FormatInt(recipient.UserID, 10)
		}
		if _, err := NotificationQueueService.EnqueueCreate(context.Background(), item); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *notificationAudienceService) resolvePermissionRecipients(tenantID int64, permissions ...string) ([]NotificationRecipient, error) {
	return s.resolveCandidateRecipients(tenantID, nil, permissions...)
}

func (s *notificationAudienceService) resolveCandidateRecipients(tenantID int64, candidateUserIDs map[int64]bool, permissions ...string) ([]NotificationRecipient, error) {
	members, err := repositories.EnterpriseIAMRepository.FindActiveTenantMembers(sqls.DB(), tenantID)
	if err != nil {
		return nil, err
	}
	recipients := make([]NotificationRecipient, 0, len(members))
	for _, member := range members {
		if len(candidateUserIDs) > 0 && !candidateUserIDs[member.UserID] {
			continue
		}
		user := repositories.UserRepository.Get(sqls.DB(), member.UserID)
		if user == nil || user.Status != enums.StatusOk {
			continue
		}
		permissionCodes, err := AuthService.GetTenantMemberPermissions(sqls.DB(), tenantID, member.ID, member.UserID)
		if err != nil {
			return nil, err
		}
		if !hasAllNotificationPermissions(permissionCodes, permissions...) {
			continue
		}
		displayName := strings.TrimSpace(member.DisplayName)
		if displayName == "" {
			displayName = strings.TrimSpace(user.Nickname)
			if displayName == "" {
				displayName = strings.TrimSpace(user.Username)
			}
		}
		recipients = append(recipients, NotificationRecipient{
			UserID:      member.UserID,
			MemberID:    member.ID,
			DisplayName: displayName,
			Permissions: permissionCodes,
		})
	}
	return recipients, nil
}

func hasAllNotificationPermissions(granted []string, required ...string) bool {
	for _, permission := range required {
		if permission != "" && !slices.Contains(granted, permission) {
			return false
		}
	}
	return true
}

func hasAnyNotificationPermission(granted []string, permissions ...string) bool {
	for _, permission := range permissions {
		if permission != "" && slices.Contains(granted, permission) {
			return true
		}
	}
	return false
}
