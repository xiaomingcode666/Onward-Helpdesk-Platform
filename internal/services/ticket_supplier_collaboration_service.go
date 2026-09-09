package services

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	SupplierCollaborationInvited    = "invited"
	SupplierCollaborationAccepted   = "accepted"
	SupplierCollaborationProcessing = "processing"
	SupplierCollaborationResolved   = "resolved"
	SupplierCollaborationTimeout    = "timeout"
	SupplierCollaborationExpired    = "expired"
	PartnerRoleAdmin                = "partner_admin"
	PartnerRoleEngineer             = "partner_engineer"
	SupplierParticipantRoleOwner    = "owner"
	SupplierParticipantRoleMember   = "member"

	defaultSupplierCollaborationResponseTimeout = 24 * time.Hour
	supplierCollaborationTimeoutBatchLimit      = 100
)

var TicketSupplierCollaborationService = newTicketSupplierCollaborationService()

type TicketSupplierCollaborationAggregate struct {
	Collaboration *models.TicketSupplierCollaboration
	Ticket        *models.Ticket
	Module        *models.ProductModule
	Company       *models.PartnerCompany
	Account       *models.PartnerAccount
	Participants  []TicketSupplierParticipantAggregate
	Roles         []string
}

type TicketSupplierParticipantAggregate struct {
	Participant models.TicketSupplierCollaborationParticipant
	Account     *models.PartnerAccount
}

type PartnerPortalAccountAggregate struct {
	Account models.PartnerAccount
	User    *models.User
	Roles   []string
}

type PartnerAccountCreateResult struct {
	Account         PartnerPortalAccountAggregate
	InitialPassword string
}

type PartnerPortalMeetingAggregate struct {
	CollaborationID int64
	Meeting         dto.EnterpriseMeetingListItemDTO
}

type PartnerTicketProgressAggregate struct {
	Progress   models.TicketProgress
	AuthorName string
}

type PartnerConversationMessageAggregate struct {
	Message    models.Message
	SenderName string
}

type PartnerTicketDetailAggregate struct {
	Collaboration TicketSupplierCollaborationAggregate
	Progresses    []PartnerTicketProgressAggregate
	Messages      []PartnerConversationMessageAggregate
}

type ticketSupplierCollaborationService struct{}

func newTicketSupplierCollaborationService() *ticketSupplierCollaborationService {
	return &ticketSupplierCollaborationService{}
}

func (s *ticketSupplierCollaborationService) List(tenantID, ticketID int64, operator *dto.AuthPrincipal) ([]TicketSupplierCollaborationAggregate, error) {
	ticket := repositories.TicketRepository.Get(sqls.DB(), ticketID)
	if ticket == nil || ticket.TenantID != tenantID {
		return nil, errorsx.InvalidParam("ticket not found")
	}
	if err := requireTicketTenantAccess(ticket, operator); err != nil {
		return nil, err
	}
	return s.buildAggregates(repositories.TicketSupplierCollaborationRepository.FindByTicket(sqls.DB(), tenantID, ticketID)), nil
}

func (s *ticketSupplierCollaborationService) ListAvailablePartners(ticketID int64, operator *dto.AuthPrincipal) ([]models.PartnerCompany, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	ticket := repositories.TicketRepository.Get(sqls.DB(), ticketID)
	if ticket == nil {
		return nil, errorsx.InvalidParam("ticket not found")
	}
	if err := requireTicketMutationAccess(ticket, operator); err != nil {
		return nil, err
	}
	tenant := repositories.PlatformIAMRepository.GetTenant(sqls.DB(), ticket.TenantID)
	if ticket.ProductID > 0 || tenant == nil || !tenant.IsKnowledgeSupportScene() {
		return nil, errorsx.InvalidParam("direct supplier selection is only available for productless knowledge-support tickets")
	}
	return repositories.TicketSupplierCollaborationRepository.FindAvailablePartnerCompanies(sqls.DB(), ticket.TenantID)
}

func (s *ticketSupplierCollaborationService) ListForPartner(operator *dto.AuthPrincipal) ([]TicketSupplierCollaborationAggregate, error) {
	account, _, err := s.requirePartnerAccount(operator)
	if err != nil {
		return nil, err
	}
	var items []models.TicketSupplierCollaboration
	if s.partnerHasRole(sqls.DB(), account, PartnerRoleAdmin) {
		items = s.findPartnerCompanyCollaborations(sqls.DB(), operator.TenantID, account.PartnerCompanyID)
	} else {
		items = s.findPartnerAccountCollaborations(sqls.DB(), operator.TenantID, account.ID)
	}
	return s.buildAggregates(filterReadablePartnerCollaborations(items, time.Now())), nil
}

func (s *ticketSupplierCollaborationService) ListConversationsForPartner(operator *dto.AuthPrincipal) ([]TicketSupplierCollaborationAggregate, error) {
	items, err := s.ListForPartner(operator)
	if err != nil {
		return nil, err
	}
	result := make([]TicketSupplierCollaborationAggregate, 0, len(items))
	for i := range items {
		collaboration := items[i].Collaboration
		if collaboration == nil || !supplierVisibilityAllows(collaboration, "repair_progress") {
			continue
		}
		if items[i].Ticket == nil || items[i].Ticket.ConversationID <= 0 {
			continue
		}
		result = append(result, items[i])
	}
	return result, nil
}

func (s *ticketSupplierCollaborationService) findPartnerCompanyCollaborations(db *gorm.DB, tenantID, partnerCompanyID int64) []models.TicketSupplierCollaboration {
	return repositories.TicketSupplierCollaborationRepository.FindByPartnerCompany(db, tenantID, partnerCompanyID)
}

func (s *ticketSupplierCollaborationService) findPartnerAccountCollaborations(db *gorm.DB, tenantID, partnerAccountID int64) []models.TicketSupplierCollaboration {
	itemsByID := make(map[int64]models.TicketSupplierCollaboration)
	for _, item := range repositories.TicketSupplierCollaborationRepository.FindByParticipantAccount(db, tenantID, partnerAccountID) {
		itemsByID[item.ID] = item
	}
	for _, item := range repositories.TicketSupplierCollaborationRepository.FindByParticipantAccountAnyStatus(db, tenantID, partnerAccountID) {
		if !isSupplierCollaborationTerminal(item.Status) {
			continue
		}
		itemsByID[item.ID] = item
	}
	for _, item := range repositories.TicketSupplierCollaborationRepository.FindByPartnerAccount(db, tenantID, partnerAccountID) {
		itemsByID[item.ID] = item
	}
	items := make([]models.TicketSupplierCollaboration, 0, len(itemsByID))
	for _, item := range itemsByID {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID > items[j].ID })
	return items
}

func filterReadablePartnerCollaborations(items []models.TicketSupplierCollaboration, now time.Time) []models.TicketSupplierCollaboration {
	result := make([]models.TicketSupplierCollaboration, 0, len(items))
	for i := range items {
		if isSupplierCollaborationTerminal(items[i].Status) {
			result = append(result, items[i])
			continue
		}
		if items[i].AuthorizationEnds != nil && !items[i].AuthorizationEnds.After(now) {
			continue
		}
		result = append(result, items[i])
	}
	return result
}

func isSupplierCollaborationTerminal(status string) bool {
	switch strings.TrimSpace(status) {
	case SupplierCollaborationResolved, SupplierCollaborationTimeout, SupplierCollaborationExpired:
		return true
	default:
		return false
	}
}

func IsSupplierCollaborationTerminalStatus(status string) bool {
	return isSupplierCollaborationTerminal(status)
}

func (s *ticketSupplierCollaborationService) PartnerProfile(operator *dto.AuthPrincipal) (*TicketSupplierCollaborationAggregate, error) {
	account, company, err := s.requirePartnerAccount(operator)
	if err != nil {
		return nil, err
	}
	roles, err := s.partnerRoleCodes(sqls.DB(), account)
	if err != nil {
		return nil, err
	}
	return &TicketSupplierCollaborationAggregate{Account: account, Company: company, Roles: roles}, nil
}

func (s *ticketSupplierCollaborationService) ListPartnerCompanyAccounts(operator *dto.AuthPrincipal) ([]PartnerPortalAccountAggregate, *models.PartnerCompany, error) {
	_, company, err := s.requirePartnerAccount(operator)
	if err != nil {
		return nil, nil, err
	}
	accounts := repositories.TicketSupplierCollaborationRepository.FindPartnerAccountsByCompany(sqls.DB(), operator.TenantID, company.ID)
	result := make([]PartnerPortalAccountAggregate, 0, len(accounts))
	for i := range accounts {
		roles, err := s.partnerRoleCodes(sqls.DB(), &accounts[i])
		if err != nil {
			return nil, nil, err
		}
		result = append(result, PartnerPortalAccountAggregate{Account: accounts[i], User: repositories.UserRepository.Get(sqls.DB(), accounts[i].UserID), Roles: roles})
	}
	return result, company, nil
}

func (s *ticketSupplierCollaborationService) CreatePartnerAccount(req dto.PartnerAccountCreateRequest, operator *dto.AuthPrincipal) (*PartnerAccountCreateResult, error) {
	current, company, err := s.requirePartnerAccount(operator)
	if err != nil {
		return nil, err
	}
	roleCode, err := normalizePartnerRoleCode(req.RoleCode)
	if err != nil {
		return nil, err
	}
	if !operator.HasPermission(constants.PermissionPartnerMemberInvite.Code) {
		return nil, errorsx.ForbiddenI18n("error.e0225")
	}
	isAdmin := operator.HasPermission(constants.PermissionPartnerMemberUpdate.Code) && s.partnerHasRole(sqls.DB(), current, PartnerRoleAdmin)
	if !isAdmin && roleCode != PartnerRoleEngineer {
		return nil, errorsx.ForbiddenI18n("error.e0225")
	}
	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		return nil, errorsx.InvalidParam("display name is required")
	}
	languagesJSON, _ := json.Marshal(normalizePartnerLanguages(req.Languages))
	var account *models.PartnerAccount
	var initialPassword string
	err = sqls.DB().Transaction(func(tx *gorm.DB) error {
		email := strings.TrimSpace(req.Email)
		phone := strings.TrimSpace(req.Phone)
		user, password, err := UserService.CreateUserDB(tx, request.CreateUserRequest{
			Username: strings.TrimSpace(req.Username),
			Nickname: displayName,
			Email:    partnerNullableString(email),
			Mobile:   partnerNullableString(phone),
			Remark:   "Partner account for " + company.Name,
		}, operator)
		if err != nil {
			return err
		}
		initialPassword = password
		now := time.Now()
		account = &models.PartnerAccount{
			TenantID:         operator.TenantID,
			PartnerCompanyID: company.ID,
			UserID:           user.ID,
			DisplayName:      displayName,
			Email:            email,
			Phone:            phone,
			LanguagesJSON:    string(languagesJSON),
			Status:           enums.StatusOk,
			AuditFields:      utils.BuildAuditFields(operator),
		}
		if err := repositories.TicketSupplierCollaborationRepository.CreatePartnerAccount(tx, account); err != nil {
			return err
		}
		return s.replacePartnerRole(tx, account, roleCode, operator, now)
	})
	if err != nil {
		return nil, err
	}
	return &PartnerAccountCreateResult{
		Account:         PartnerPortalAccountAggregate{Account: *account, User: repositories.UserRepository.Get(sqls.DB(), account.UserID), Roles: []string{roleCode}},
		InitialPassword: initialPassword,
	}, nil
}

func (s *ticketSupplierCollaborationService) UpdatePartnerAccount(accountID int64, req dto.PartnerAccountUpdateRequest, operator *dto.AuthPrincipal) (*PartnerPortalAccountAggregate, error) {
	current, company, err := s.requirePartnerAccount(operator)
	if err != nil {
		return nil, err
	}
	if !operator.HasPermission(constants.PermissionPartnerMemberUpdate.Code) || !s.partnerHasRole(sqls.DB(), current, PartnerRoleAdmin) {
		return nil, errorsx.ForbiddenI18n("error.e0225")
	}
	target := repositories.TicketSupplierCollaborationRepository.GetPartnerAccountAnyStatus(sqls.DB(), operator.TenantID, company.ID, accountID)
	if target == nil {
		return nil, errorsx.InvalidParam("partner account not found")
	}
	roleCode, err := normalizePartnerRoleCode(req.RoleCode)
	if err != nil {
		return nil, err
	}
	status := enums.Status(req.Status)
	if status != enums.StatusOk && status != enums.StatusDisabled {
		return nil, errorsx.InvalidParam("partner account status is invalid")
	}
	if target.ID == current.ID && status != enums.StatusOk {
		return nil, errorsx.InvalidParam("current account cannot disable itself")
	}
	if s.partnerHasRole(sqls.DB(), target, PartnerRoleAdmin) && (roleCode != PartnerRoleAdmin || status != enums.StatusOk) && s.countActivePartnerAdmins(company.ID, operator.TenantID) <= 1 {
		return nil, errorsx.InvalidParam("the last supplier administrator cannot be disabled or demoted")
	}
	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		return nil, errorsx.InvalidParam("display name is required")
	}
	languagesJSON, _ := json.Marshal(normalizePartnerLanguages(req.Languages))
	now := time.Now()
	err = sqls.DB().Transaction(func(tx *gorm.DB) error {
		if err := repositories.TicketSupplierCollaborationRepository.UpdatePartnerAccount(tx, operator.TenantID, company.ID, target.ID, map[string]any{
			"display_name":     displayName,
			"email":            strings.TrimSpace(req.Email),
			"phone":            strings.TrimSpace(req.Phone),
			"languages_json":   string(languagesJSON),
			"status":           status,
			"updated_at":       now,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
		}); err != nil {
			return err
		}
		if err := s.replacePartnerRole(tx, target, roleCode, operator, now); err != nil {
			return err
		}
		if status == enums.StatusDisabled {
			return s.revokeActiveLoginSessionsDB(tx, target.UserID, operator, now)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	target = repositories.TicketSupplierCollaborationRepository.GetPartnerAccountAnyStatus(sqls.DB(), operator.TenantID, company.ID, target.ID)
	return &PartnerPortalAccountAggregate{Account: *target, User: repositories.UserRepository.Get(sqls.DB(), target.UserID), Roles: []string{roleCode}}, nil
}

func (s *ticketSupplierCollaborationService) revokeActiveLoginSessionsDB(db *gorm.DB, userID int64, operator *dto.AuthPrincipal, now time.Time) error {
	sessions := repositories.LoginSessionRepository.Find(db, sqls.NewCnd().
		Eq("user_id", userID).
		Where("revoked_at IS NULL").
		Gt("expired_at", now))
	for i := range sessions {
		if err := repositories.LoginSessionRepository.Updates(db, sessions[i].ID, map[string]any{
			"revoked_at":       now,
			"updated_at":       now,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *ticketSupplierCollaborationService) ListPortalMeetings(ctx context.Context, status string, operator *dto.AuthPrincipal) ([]PartnerPortalMeetingAggregate, error) {
	account, _, err := s.requirePartnerAccount(operator)
	if err != nil {
		return nil, err
	}
	status = strings.TrimSpace(status)
	now := time.Now()
	seenTickets := make(map[int64]bool)
	items := make([]PartnerPortalMeetingAggregate, 0)
	var collaborations []models.TicketSupplierCollaboration
	if s.partnerHasRole(sqls.DB(), account, PartnerRoleAdmin) {
		collaborations = s.findPartnerCompanyCollaborations(sqls.DB(), operator.TenantID, account.PartnerCompanyID)
	} else {
		collaborations = s.findPartnerAccountCollaborations(sqls.DB(), operator.TenantID, account.ID)
	}
	prioritizePartnerMeetingCollaborations(collaborations)
	for _, collaboration := range collaborations {
		if !supplierVisibilityAllows(&collaboration, "meeting") || seenTickets[collaboration.TicketID] {
			continue
		}
		if !isSupplierCollaborationTerminal(collaboration.Status) && collaboration.AuthorizationEnds != nil && !collaboration.AuthorizationEnds.After(now) {
			continue
		}
		seenTickets[collaboration.TicketID] = true
		meetings, err := MeetingService.ListTicketMeetings(ctx, operator.TenantID, collaboration.TicketID)
		if err != nil {
			return nil, err
		}
		for _, meeting := range meetings {
			if !partnerPortalMeetingStatusMatchesFilter(meeting.Status, status) {
				continue
			}
			items = append(items, PartnerPortalMeetingAggregate{CollaborationID: collaboration.ID, Meeting: meeting})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Meeting.CreatedAt > items[j].Meeting.CreatedAt })
	return items, nil
}

func prioritizePartnerMeetingCollaborations(items []models.TicketSupplierCollaboration) {
	sort.SliceStable(items, func(i, j int) bool {
		iTerminal := isSupplierCollaborationTerminal(items[i].Status)
		jTerminal := isSupplierCollaborationTerminal(items[j].Status)
		if iTerminal != jTerminal {
			return !iTerminal
		}
		return items[i].ID > items[j].ID
	})
}

func partnerPortalMeetingStatusMatchesFilter(meetingStatus, filter string) bool {
	filter = strings.TrimSpace(filter)
	if filter == "" || filter == "all" {
		return true
	}
	if filter == "finished" || filter == "ended" {
		return meetingStatus == "finished" || meetingStatus == "ended"
	}
	return meetingStatus == filter
}

func (s *ticketSupplierCollaborationService) Invite(ticketID int64, req dto.TicketSupplierInviteRequest, operator *dto.AuthPrincipal) (*TicketSupplierCollaborationAggregate, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	ticket := repositories.TicketRepository.Get(sqls.DB(), ticketID)
	if ticket == nil {
		return nil, errorsx.InvalidParam("ticket not found")
	}
	if err := requireTicketTenantAccess(ticket, operator); err != nil {
		return nil, err
	}
	if err := requireTicketMutationAccess(ticket, operator); err != nil {
		return nil, err
	}
	db := sqls.DB()
	var module *models.ProductModule
	moduleID := int64(0)
	moduleName := ""
	partnerCompanyID := req.PartnerCompanyID
	if ticket.ProductID <= 0 {
		tenant := repositories.PlatformIAMRepository.GetTenant(db, ticket.TenantID)
		if tenant == nil || !tenant.IsKnowledgeSupportScene() {
			return nil, errorsx.InvalidParam("supplier collaboration without a product is only available for knowledge-support tenants")
		}
		if req.ProductModuleID > 0 {
			return nil, errorsx.InvalidParam("product module is not available for a productless ticket")
		}
		if partnerCompanyID <= 0 {
			return nil, errorsx.InvalidParam("supplier is required for a productless ticket")
		}
	} else {
		module = repositories.ProductModuleRepository.Get(db, req.ProductModuleID)
		if module == nil || module.TenantID != ticket.TenantID || module.ProductID != ticket.ProductID || module.Status != enums.StatusOk {
			return nil, errorsx.InvalidParam("product module is invalid for ticket")
		}
		moduleID = module.ID
		moduleName = strings.TrimSpace(module.Name)
		if partnerCompanyID <= 0 {
			partnerCompanyID = module.DefaultSupplierID
		}
	}
	company := repositories.TicketSupplierCollaborationRepository.GetPartnerCompany(db, ticket.TenantID, partnerCompanyID)
	if company == nil {
		if ticket.ProductID <= 0 {
			return nil, errorsx.InvalidParam("supplier is not available for this tenant")
		}
		return nil, errorsx.InvalidParam("supplier is not configured for this product module")
	}
	account := repositories.TicketSupplierCollaborationRepository.GetPartnerAccount(db, ticket.TenantID, company.ID, req.PartnerAccountID)
	if account == nil {
		return nil, errorsx.InvalidParam("supplier has no active account to participate")
	}
	existingBeforeValidate := repositories.TicketSupplierCollaborationRepository.FindActive(db, ticket.TenantID, ticket.ID, moduleID, company.ID)
	if existingBeforeValidate == nil && strings.TrimSpace(req.Reason) == "" {
		return nil, errorsx.InvalidParam("supplier escalation reason is required")
	}
	status := enums.NormalizeTicketStatus(string(ticket.Status))
	if status != enums.TicketStatusAccepted && status != enums.TicketStatusProcessing && status != enums.TicketStatusVideoSupport && status != enums.TicketStatusSupplierSupport {
		return nil, errorsx.InvalidParam("supplier escalation requires an accepted, processing, video-support, or supplier-support ticket")
	}

	visibility := normalizeSupplierVisibility(req.Visibility)
	visibilityJSON, _ := json.Marshal(visibility)
	now := time.Now()
	endsAt, explicitAuthorizationEnd, err := resolveSupplierAuthorizationEnd(req, now)
	if err != nil {
		return nil, err
	}
	item := &models.TicketSupplierCollaboration{
		TenantID:          ticket.TenantID,
		TicketID:          ticket.ID,
		ProductID:         ticket.ProductID,
		ProductModuleID:   moduleID,
		PartnerCompanyID:  company.ID,
		PartnerAccountID:  account.ID,
		Status:            SupplierCollaborationInvited,
		Reason:            strings.TrimSpace(req.Reason),
		VisibilityJSON:    string(visibilityJSON),
		InvitedAt:         now,
		AuthorizationEnds: &endsAt,
		RecordStatus:      enums.StatusOk,
		AuditFields:       utils.BuildAuditFields(operator),
	}
	created := false
	err = sqls.DB().Transaction(func(tx *gorm.DB) error {
		var lockedTicket models.Ticket
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND tenant_id = ?", ticket.ID, ticket.TenantID).
			First(&lockedTicket).Error; err != nil {
			return errorsx.InvalidParam("ticket not found")
		}
		if existing := repositories.TicketSupplierCollaborationRepository.FindActive(tx, ticket.TenantID, ticket.ID, moduleID, company.ID); existing != nil {
			item = existing
			existingAccount := repositories.TicketSupplierCollaborationRepository.GetPartnerAccount(tx, ticket.TenantID, company.ID, existing.PartnerAccountID)
			if existingAccount == nil {
				return errorsx.InvalidParam("supplier has no active account to participate")
			}
			if err := s.refreshSupplierCollaborationAuthorizationTx(tx, existing, visibility, &endsAt, explicitAuthorizationEnd, operator); err != nil {
				return err
			}
			if err := s.ensureSupplierCollaborationParticipantTx(tx, existing, existingAccount, SupplierParticipantRoleOwner, now, operator); err != nil {
				return err
			}
			return s.ensurePartnerConversationParticipantTx(tx, lockedTicket.ConversationID, existingAccount.UserID, existingAccount.ID, now, operator)
		}
		if err := repositories.TicketSupplierCollaborationRepository.Create(tx, item); err != nil {
			return err
		}
		created = true
		scopeRuleJSON, _ := json.Marshal(map[string]any{"visibility": visibility, "collaboration_id": item.ID})
		if err := repositories.TicketSupplierCollaborationRepository.CreateAuthorizationScope(tx, &models.PartnerAuthorizationScope{
			TenantID:         ticket.TenantID,
			PartnerAccountID: account.ID,
			ResourceType:     "ticket",
			ResourceID:       strconv.FormatInt(ticket.ID, 10),
			ScopeRuleJSON:    string(scopeRuleJSON),
			ExpiredAt:        &endsAt,
			Status:           enums.StatusOk,
			AuditFields:      utils.BuildAuditFields(operator),
		}); err != nil {
			return err
		}
		if err := s.ensureSupplierCollaborationParticipantTx(tx, item, account, SupplierParticipantRoleOwner, now, operator); err != nil {
			return err
		}
		ticketUpdates := map[string]any{
			"status":           enums.TicketStatusSupplierSupport,
			"updated_at":       now,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
		}
		if moduleID > 0 {
			ticketUpdates["product_module_id"] = moduleID
		}
		if err := repositories.TicketRepository.Updates(tx, ticket.ID, ticketUpdates); err != nil {
			return err
		}
		progressContent := "升级供应商协作：" + company.Name
		if moduleName != "" {
			progressContent += " / " + moduleName
		}
		if err := repositories.TicketProgressRepository.Create(tx, &models.TicketProgress{
			TenantID:     ticket.TenantID,
			TicketID:     ticket.ID,
			EventType:    enums.TicketProgressEventEscalated,
			Content:      progressContent,
			MetadataJSON: string(scopeRuleJSON),
			AuthorID:     operator.UserID,
			CreatedAt:    now,
		}); err != nil {
			return err
		}
		return s.ensurePartnerConversationParticipantTx(tx, ticket.ConversationID, account.UserID, account.ID, now, operator)
	})
	if err != nil {
		return nil, err
	}
	aggregate := s.buildAggregate(item)
	if created {
		eventContent := "已邀请供应商协作：" + company.Name
		if moduleName != "" {
			eventContent += " · " + moduleName
		}
		eventMetadata := map[string]any{
			"partnerCompanyId":   company.ID,
			"partnerCompanyName": strings.TrimSpace(company.Name),
		}
		if moduleID > 0 {
			eventMetadata["productModuleId"] = moduleID
			eventMetadata["productModuleName"] = moduleName
		}
		s.publishConversationEvent(
			aggregate.Ticket,
			item.ID,
			"supplier_collaboration_invited",
			eventContent,
			eventMetadata,
		)
	}
	return &aggregate, nil
}

func (s *ticketSupplierCollaborationService) refreshSupplierCollaborationAuthorizationTx(tx *gorm.DB, item *models.TicketSupplierCollaboration, requestedVisibility []string, requestedEndsAt *time.Time, replaceAuthorizationEnd bool, operator *dto.AuthPrincipal) error {
	if tx == nil || item == nil {
		return nil
	}
	visibility := mergeSupplierVisibility(item.VisibilityJSON, requestedVisibility)
	visibilityJSON, _ := json.Marshal(visibility)
	authorizationEnds := item.AuthorizationEnds
	if requestedEndsAt != nil && (replaceAuthorizationEnd || authorizationEnds == nil || requestedEndsAt.After(*authorizationEnds)) {
		authorizationEnds = requestedEndsAt
	}
	updates := map[string]any{
		"visibility_json":    string(visibilityJSON),
		"authorization_ends": authorizationEnds,
		"updated_at":         time.Now(),
	}
	if operator != nil {
		updates["update_user_id"] = operator.UserID
		updates["update_user_name"] = operator.Username
	}
	if err := tx.Model(&models.TicketSupplierCollaboration{}).Where("id = ?", item.ID).Updates(updates).Error; err != nil {
		return err
	}
	item.VisibilityJSON = string(visibilityJSON)
	item.AuthorizationEnds = authorizationEnds
	scopeRuleJSON, _ := json.Marshal(map[string]any{"visibility": visibility, "collaboration_id": item.ID})
	return s.ensureSupplierAuthorizationScopeTx(tx, item, item.PartnerAccountID, string(scopeRuleJSON), authorizationEnds, operator)
}

func resolveSupplierAuthorizationEnd(req dto.TicketSupplierInviteRequest, now time.Time) (time.Time, bool, error) {
	if raw := strings.TrimSpace(req.AuthorizationEndsAt); raw != "" {
		endsAt, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return time.Time{}, true, errorsx.InvalidParam("authorization_ends_at must be an RFC3339 timestamp")
		}
		if !endsAt.After(now) {
			return time.Time{}, true, errorsx.InvalidParam("supplier authorization end must be in the future")
		}
		if endsAt.After(now.AddDate(0, 0, 90)) {
			return time.Time{}, true, errorsx.InvalidParam("supplier authorization cannot exceed 90 days")
		}
		// PostgreSQL uses timestamp without time zone for this model. Preserve the
		// instant while storing its wall clock in the database session location.
		return endsAt.In(now.Location()), true, nil
	}
	accessDays := req.AccessDays
	if accessDays <= 0 {
		accessDays = 30
	}
	if accessDays > 90 {
		accessDays = 90
	}
	return now.AddDate(0, 0, accessDays), false, nil
}

type supplierAuthorizationExpiryResult struct {
	Handled bool
	Ticket  *models.Ticket
	Content string
}

func (s *ticketSupplierCollaborationService) ExpireAuthorizations(limit int) (int, error) {
	if limit <= 0 {
		limit = supplierCollaborationTimeoutBatchLimit
	}
	now := time.Now()
	items, err := repositories.TicketSupplierCollaborationRepository.FindAuthorizationExpiredBefore(sqls.DB(), now, limit)
	if err != nil {
		return 0, err
	}
	handled := 0
	failures := make([]error, 0)
	for i := range items {
		result, expireErr := s.expireSupplierAuthorization(items[i].ID, now)
		if expireErr != nil {
			failures = append(failures, expireErr)
			continue
		}
		if result == nil || !result.Handled {
			continue
		}
		handled++
		s.publishConversationEvent(
			result.Ticket,
			items[i].ID,
			"supplier_collaboration_authorization_expired",
			result.Content,
			map[string]any{"authorizationEndsAt": items[i].AuthorizationEnds},
		)
		if auditErr := AuditService.RecordAudit(context.Background(), RecordAuditInput{
			TenantID:     items[i].TenantID,
			ActorID:      "system",
			ActorType:    "system",
			Domain:       "ticket",
			ResourceType: "supplier_collaboration",
			ResourceID:   strconv.FormatInt(items[i].ID, 10),
			Action:       "ticket.supplier_collaboration.authorization_expired",
			AfterState: map[string]any{
				"ticketId":            items[i].TicketID,
				"authorizationEndsAt": items[i].AuthorizationEnds,
				"status":              SupplierCollaborationExpired,
			},
			RiskLevel: models.RiskLevelMedium,
		}); auditErr != nil {
			failures = append(failures, auditErr)
		}
	}
	return handled, errors.Join(failures...)
}

func (s *ticketSupplierCollaborationService) expireSupplierAuthorization(id int64, now time.Time) (*supplierAuthorizationExpiryResult, error) {
	if id <= 0 {
		return nil, nil
	}
	operator := systemDispatchPrincipal()
	result := &supplierAuthorizationExpiryResult{}
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		tx := ctx.Tx
		locked := repositories.TicketSupplierCollaborationRepository.GetForUpdate(tx, id)
		if locked == nil || locked.RecordStatus == enums.StatusDeleted || isSupplierCollaborationTerminal(locked.Status) || locked.AuthorizationEnds == nil || locked.AuthorizationEnds.After(now) {
			return nil
		}
		ticket := loadTicketForUpdate(tx, locked.TicketID)
		if ticket == nil || ticket.TenantID != locked.TenantID {
			return nil
		}
		content := "供应商协作授权已到期，供应商访问已回收，工单返回企业工程师继续处理。"
		if err := repositories.TicketSupplierCollaborationRepository.Updates(tx, locked.ID, map[string]any{
			"status":           SupplierCollaborationExpired,
			"resolution":       content,
			"resolved_at":      now,
			"updated_at":       now,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
		}); err != nil {
			return err
		}
		participants := repositories.TicketSupplierCollaborationRepository.FindParticipants(tx, locked.TenantID, locked.ID)
		participantAccountIDs := make([]int64, 0, len(participants))
		activeAccountIDs := make([]int64, 0, len(participants))
		for i := range participants {
			participantAccountIDs = append(participantAccountIDs, participants[i].PartnerAccountID)
			if participants[i].Status == enums.StatusOk {
				activeAccountIDs = append(activeAccountIDs, participants[i].PartnerAccountID)
			}
		}
		if err := repositories.TicketSupplierCollaborationRepository.DisableCollaborationAuthorizationScopes(
			tx, locked.TenantID, locked.TicketID, locked.ID, participantAccountIDs, now,
		); err != nil {
			return err
		}
		if err := repositories.TicketSupplierCollaborationRepository.DisableActiveParticipants(
			tx, locked.TenantID, locked.ID, now, operator.UserID, operator.Username,
		); err != nil {
			return err
		}
		for _, accountID := range activeAccountIDs {
			if repositories.TicketSupplierCollaborationRepository.CountActiveParticipantByTicket(tx, locked.TenantID, locked.TicketID, accountID) > 0 {
				continue
			}
			account := repositories.TicketSupplierCollaborationRepository.GetPartnerAccountAnyStatus(tx, locked.TenantID, locked.PartnerCompanyID, accountID)
			if account != nil {
				if err := s.deactivatePartnerConversationParticipantTx(tx, ticket.ConversationID, account.UserID, now, operator); err != nil {
					return err
				}
			}
		}
		if repositories.TicketSupplierCollaborationRepository.CountActiveByTicket(tx, locked.TenantID, locked.TicketID) == 0 &&
			enums.NormalizeTicketStatus(string(ticket.Status)) == enums.TicketStatusSupplierSupport {
			if err := repositories.TicketRepository.Updates(tx, ticket.ID, map[string]any{
				"status":           enums.TicketStatusProcessing,
				"updated_at":       now,
				"update_user_id":   operator.UserID,
				"update_user_name": operator.Username,
			}); err != nil {
				return err
			}
			ticket.Status = enums.TicketStatusProcessing
		}
		metadataJSON, _ := json.Marshal(map[string]any{
			"source":                "supplier_authorization_expired",
			"collaboration_id":      locked.ID,
			"authorization_ends_at": locked.AuthorizationEnds,
		})
		if err := repositories.TicketProgressRepository.Create(tx, &models.TicketProgress{
			TenantID:          locked.TenantID,
			TicketID:          locked.TicketID,
			EventType:         enums.TicketProgressEventProgress,
			Content:           content,
			VisibleToCustomer: true,
			MetadataJSON:      string(metadataJSON),
			AuthorID:          operator.UserID,
			CreatedAt:         now,
		}); err != nil {
			return err
		}
		result = &supplierAuthorizationExpiryResult{Handled: true, Ticket: ticket, Content: content}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *ticketSupplierCollaborationService) Accept(id int64, operator *dto.AuthPrincipal) (*TicketSupplierCollaborationAggregate, error) {
	item, err := s.requirePartnerCollaborationWork(id, operator)
	if err != nil {
		return nil, err
	}
	account, _, err := s.requirePartnerAccount(operator)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	metadataJSON := partnerProgressMetadata(item.ID)
	acceptedNow := false
	if err := sqls.DB().Transaction(func(tx *gorm.DB) error {
		locked := repositories.TicketSupplierCollaborationRepository.GetForUpdate(tx, item.ID)
		if locked == nil || locked.RecordStatus == enums.StatusDeleted {
			return errorsx.InvalidParam("supplier collaboration not found")
		}
		if locked.PartnerCompanyID != account.PartnerCompanyID {
			return errorsx.ForbiddenI18n("error.e0225")
		}
		if isSupplierCollaborationTerminal(locked.Status) {
			return errorsx.InvalidParam("supplier collaboration is already closed")
		}
		if locked.AuthorizationEnds != nil && !locked.AuthorizationEnds.After(now) {
			return errorsx.ForbiddenI18n("error.e0225")
		}
		if locked.Status == SupplierCollaborationAccepted || locked.Status == SupplierCollaborationProcessing {
			item = locked
			return s.ensureAcceptingPartnerParticipationTx(tx, locked, account, now, operator)
		}
		if locked.Status != SupplierCollaborationInvited {
			return errorsx.InvalidParam("supplier collaboration is not awaiting acceptance")
		}
		if err := repositories.TicketSupplierCollaborationRepository.Updates(tx, locked.ID, map[string]any{
			"status":           SupplierCollaborationProcessing,
			"accepted_at":      now,
			"updated_at":       now,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
		}); err != nil {
			return err
		}
		locked.Status = SupplierCollaborationProcessing
		locked.AcceptedAt = &now
		item = locked
		if err := s.ensureAcceptingPartnerParticipationTx(tx, locked, account, now, operator); err != nil {
			return err
		}
		if err := repositories.TicketProgressRepository.Create(tx, &models.TicketProgress{
			TenantID:     locked.TenantID,
			TicketID:     locked.TicketID,
			EventType:    enums.TicketProgressEventAccepted,
			Content:      "供应商已接单",
			MetadataJSON: metadataJSON,
			AuthorID:     operator.UserID,
			CreatedAt:    now,
		}); err != nil {
			return err
		}
		acceptedNow = true
		return nil
	}); err != nil {
		return nil, err
	}
	item = repositories.TicketSupplierCollaborationRepository.Get(sqls.DB(), item.ID)
	aggregate := s.buildAggregate(item)
	if !acceptedNow {
		return &aggregate, nil
	}
	companyName := "供应商"
	if aggregate.Company != nil && strings.TrimSpace(aggregate.Company.Name) != "" {
		companyName = strings.TrimSpace(aggregate.Company.Name)
	}
	moduleName := ""
	if aggregate.Module != nil {
		moduleName = strings.TrimSpace(aggregate.Module.Name)
	}
	metadata := map[string]any{
		"partnerAccountId":   account.ID,
		"partnerCompanyName": companyName,
	}
	if moduleName != "" {
		metadata["productModuleName"] = moduleName
	}
	s.publishConversationEvent(
		aggregate.Ticket,
		item.ID,
		"supplier_collaboration_accepted",
		companyName+"已加入协作会话，客户、维修工程师和供应商可共同沟通。",
		metadata,
	)
	return &aggregate, nil
}

type supplierCollaborationTimeoutResult struct {
	Handled            bool
	TicketID           int64
	ExpectedAssigneeID int64
	ConversationTicket *models.Ticket
	Content            string
}

func (s *ticketSupplierCollaborationService) EscalateUnresponsiveInvitations(timeout time.Duration, limit int) (int, error) {
	if timeout <= 0 {
		timeout = defaultSupplierCollaborationResponseTimeout
	}
	if limit <= 0 {
		limit = supplierCollaborationTimeoutBatchLimit
	}
	now := time.Now()
	items, err := repositories.TicketSupplierCollaborationRepository.FindInvitedBefore(sqls.DB(), now.Add(-timeout), limit)
	if err != nil {
		return 0, err
	}
	handled := 0
	failures := make([]error, 0)
	for i := range items {
		result, err := s.timeoutSupplierInvitation(items[i].ID, timeout, now)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if result == nil || !result.Handled {
			continue
		}
		handled++
		if result.ConversationTicket != nil {
			s.publishConversationEvent(
				result.ConversationTicket,
				items[i].ID,
				"supplier_collaboration_timeout",
				result.Content,
				map[string]any{"timeoutHours": int(timeout.Hours())},
			)
		}
		if _, err := TicketDispatchService.EscalateToSupervisor(
			result.TicketID,
			result.ExpectedAssigneeID,
			result.Content,
			now,
		); err != nil {
			failures = append(failures, err)
		}
	}
	return handled, errors.Join(failures...)
}

func (s *ticketSupplierCollaborationService) timeoutSupplierInvitation(id int64, timeout time.Duration, now time.Time) (*supplierCollaborationTimeoutResult, error) {
	if id <= 0 {
		return nil, nil
	}
	operator := systemDispatchPrincipal()
	dueBefore := now.Add(-timeout)
	result := &supplierCollaborationTimeoutResult{}
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		tx := ctx.Tx
		locked := repositories.TicketSupplierCollaborationRepository.GetForUpdate(tx, id)
		if locked == nil || locked.RecordStatus == enums.StatusDeleted {
			return nil
		}
		if locked.Status != SupplierCollaborationInvited || locked.AcceptedAt != nil || locked.InvitedAt.After(dueBefore) {
			return nil
		}
		ticket := loadTicketForUpdate(tx, locked.TicketID)
		if ticket == nil || ticket.TenantID != locked.TenantID {
			return nil
		}
		companyName := "供应商"
		if company := repositories.TicketSupplierCollaborationRepository.GetPartnerCompany(tx, locked.TenantID, locked.PartnerCompanyID); company != nil && strings.TrimSpace(company.Name) != "" {
			companyName = strings.TrimSpace(company.Name)
		}
		moduleName := ""
		if module := repositories.ProductModuleRepository.Get(tx, locked.ProductModuleID); module != nil && module.TenantID == locked.TenantID {
			moduleName = strings.TrimSpace(module.Name)
		}
		targetName := companyName
		if moduleName != "" {
			targetName += " / " + moduleName
		}
		content := "供应商未响应超时，已升级企业侧继续处理：" + targetName
		resolution := "供应商在响应窗口内未接单，系统已关闭本次供应商协作并升级企业侧兜底处理。"
		if err := repositories.TicketSupplierCollaborationRepository.Updates(tx, locked.ID, map[string]any{
			"status":           SupplierCollaborationTimeout,
			"resolution":       resolution,
			"resolved_at":      now,
			"updated_at":       now,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
		}); err != nil {
			return err
		}
		participants := repositories.TicketSupplierCollaborationRepository.FindParticipants(tx, locked.TenantID, locked.ID)
		activeAccountIDs := make([]int64, 0, len(participants))
		for i := range participants {
			if participants[i].Status == enums.StatusOk {
				activeAccountIDs = append(activeAccountIDs, participants[i].PartnerAccountID)
			}
		}
		if err := repositories.TicketSupplierCollaborationRepository.DisableCollaborationAuthorizationScopes(
			tx, locked.TenantID, locked.TicketID, locked.ID, activeAccountIDs, now,
		); err != nil {
			return err
		}
		if err := repositories.TicketSupplierCollaborationRepository.DisableActiveParticipants(
			tx, locked.TenantID, locked.ID, now, operator.UserID, operator.Username,
		); err != nil {
			return err
		}
		for _, accountID := range activeAccountIDs {
			if repositories.TicketSupplierCollaborationRepository.CountActiveParticipantByTicket(tx, locked.TenantID, locked.TicketID, accountID) > 0 {
				continue
			}
			account := repositories.TicketSupplierCollaborationRepository.GetPartnerAccountAnyStatus(tx, locked.TenantID, locked.PartnerCompanyID, accountID)
			if account != nil {
				if err := s.deactivatePartnerConversationParticipantTx(tx, ticket.ConversationID, account.UserID, now, operator); err != nil {
					return err
				}
			}
		}
		if repositories.TicketSupplierCollaborationRepository.CountActiveByTicket(tx, locked.TenantID, locked.TicketID) == 0 &&
			enums.NormalizeTicketStatus(string(ticket.Status)) == enums.TicketStatusSupplierSupport {
			if err := repositories.TicketRepository.Updates(tx, ticket.ID, map[string]any{
				"status":           enums.TicketStatusProcessing,
				"updated_at":       now,
				"update_user_id":   operator.UserID,
				"update_user_name": operator.Username,
			}); err != nil {
				return err
			}
			ticket.Status = enums.TicketStatusProcessing
		}
		metadataJSON, _ := json.Marshal(map[string]any{
			"source":           "supplier_timeout",
			"collaboration_id": locked.ID,
			"timeout_hours":    int(timeout.Hours()),
		})
		if err := repositories.TicketProgressRepository.Create(tx, &models.TicketProgress{
			TenantID:          locked.TenantID,
			TicketID:          locked.TicketID,
			EventType:         enums.TicketProgressEventEscalated,
			Content:           content,
			VisibleToCustomer: true,
			MetadataJSON:      string(metadataJSON),
			AuthorID:          operator.UserID,
			CreatedAt:         now,
		}); err != nil {
			return err
		}
		result = &supplierCollaborationTimeoutResult{
			Handled:            true,
			TicketID:           ticket.ID,
			ExpectedAssigneeID: ticket.CurrentAssigneeID,
			ConversationTicket: ticket,
			Content:            content,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *ticketSupplierCollaborationService) publishConversationEvent(
	ticket *models.Ticket,
	collaborationID int64,
	eventType, content string,
	metadata map[string]any,
) {
	if ticket == nil || ticket.ConversationID <= 0 || collaborationID <= 0 {
		return
	}
	payload := map[string]any{
		"eventType":       eventType,
		"source":          "supplier_collaboration",
		"ticketId":        ticket.ID,
		"collaborationId": collaborationID,
	}
	for key, value := range metadata {
		payload[key] = value
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		slog.Warn("marshal supplier conversation event failed", "ticket_id", ticket.ID, "event_type", eventType, "error", err)
		return
	}
	clientMsgID := eventType + "_" + strconv.FormatInt(collaborationID, 10)
	if _, err := MessageService.SendSystemMessageWithRequestID(ticket.ConversationID, clientMsgID, content, string(payloadJSON), ""); err != nil {
		slog.Warn("publish supplier conversation event failed", "ticket_id", ticket.ID, "event_type", eventType, "error", err)
	}
}

func (s *ticketSupplierCollaborationService) ensureAcceptingPartnerParticipationTx(tx *gorm.DB, collaboration *models.TicketSupplierCollaboration, account *models.PartnerAccount, acceptedAt time.Time, operator *dto.AuthPrincipal) error {
	if tx == nil || collaboration == nil || account == nil {
		return errorsx.InvalidParam("supplier account is required")
	}
	if err := s.ensureSupplierCollaborationParticipantTx(tx, collaboration, account, participantRoleForAccount(collaboration, account.ID), acceptedAt, operator); err != nil {
		return err
	}
	visibility := normalizeSupplierVisibility(nil)
	if err := json.Unmarshal([]byte(collaboration.VisibilityJSON), &visibility); err != nil {
		visibility = normalizeSupplierVisibility(nil)
	}
	scopeRuleJSON, _ := json.Marshal(map[string]any{"visibility": visibility, "collaboration_id": collaboration.ID})
	if err := s.ensureSupplierAuthorizationScopeTx(tx, collaboration, account.ID, string(scopeRuleJSON), collaboration.AuthorizationEnds, operator); err != nil {
		return err
	}
	ticket := repositories.TicketRepository.Get(tx, collaboration.TicketID)
	if ticket == nil {
		return errorsx.InvalidParam("ticket not found")
	}
	return s.ensurePartnerConversationParticipantTx(tx, ticket.ConversationID, account.UserID, account.ID, acceptedAt, operator)
}

func (s *ticketSupplierCollaborationService) Assign(id, partnerAccountID int64, operator *dto.AuthPrincipal) (*TicketSupplierCollaborationAggregate, error) {
	current, company, err := s.requirePartnerAccount(operator)
	if err != nil {
		return nil, err
	}
	if !s.partnerHasRole(sqls.DB(), current, PartnerRoleAdmin) {
		return nil, errorsx.ForbiddenI18n("error.e0225")
	}
	item, err := s.requirePartnerCollaborationWork(id, operator)
	if err != nil {
		return nil, err
	}
	if item.PartnerCompanyID != company.ID {
		return nil, errorsx.ForbiddenI18n("error.e0225")
	}
	if isSupplierCollaborationTerminal(item.Status) {
		return nil, errorsx.InvalidParam("closed supplier collaboration cannot be reassigned")
	}
	if item.AuthorizationEnds != nil && !item.AuthorizationEnds.After(time.Now()) {
		return nil, errorsx.ForbiddenI18n("error.e0225")
	}
	target := repositories.TicketSupplierCollaborationRepository.GetPartnerAccount(sqls.DB(), operator.TenantID, company.ID, partnerAccountID)
	if target == nil {
		return nil, errorsx.InvalidParam("target supplier account is not active in this company")
	}
	if target.ID == item.PartnerAccountID {
		if ticket := repositories.TicketRepository.Get(sqls.DB(), item.TicketID); ticket != nil && ticket.ConversationID > 0 {
			if err := sqls.DB().Transaction(func(tx *gorm.DB) error {
				now := time.Now()
				if err := s.ensureSupplierCollaborationParticipantTx(tx, item, target, SupplierParticipantRoleOwner, now, operator); err != nil {
					return err
				}
				return s.ensurePartnerConversationParticipantTx(tx, ticket.ConversationID, target.UserID, target.ID, now, operator)
			}); err != nil {
				return nil, err
			}
		}
		aggregate := s.buildAggregate(item)
		return &aggregate, nil
	}
	now := time.Now()
	endsAt := item.AuthorizationEnds
	if endsAt == nil || endsAt.Before(now) {
		defaultEnd := now.AddDate(0, 0, 30)
		endsAt = &defaultEnd
	}
	err = sqls.DB().Transaction(func(tx *gorm.DB) error {
		ticket := repositories.TicketRepository.Get(tx, item.TicketID)
		if ticket == nil {
			return errorsx.InvalidParam("ticket not found")
		}
		visibility := normalizeSupplierVisibility(nil)
		if err := json.Unmarshal([]byte(item.VisibilityJSON), &visibility); err != nil {
			visibility = normalizeSupplierVisibility(nil)
		} else {
			visibility = normalizeSupplierVisibility(visibility)
		}
		scopeRuleJSON, _ := json.Marshal(map[string]any{"visibility": visibility, "collaboration_id": item.ID})
		if err := s.ensureSupplierAuthorizationScopeTx(tx, item, target.ID, string(scopeRuleJSON), endsAt, operator); err != nil {
			return err
		}
		if err := repositories.TicketSupplierCollaborationRepository.UpdateParticipants(tx, item.TenantID, item.ID, map[string]any{
			"role":             SupplierParticipantRoleMember,
			"updated_at":       now,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
		}); err != nil {
			return err
		}
		if err := s.ensureSupplierCollaborationParticipantTx(tx, item, target, SupplierParticipantRoleOwner, now, operator); err != nil {
			return err
		}
		if err := repositories.TicketSupplierCollaborationRepository.Updates(tx, item.ID, map[string]any{
			"partner_account_id": target.ID,
			"authorization_ends": endsAt,
			"updated_at":         now,
			"update_user_id":     operator.UserID,
			"update_user_name":   operator.Username,
		}); err != nil {
			return err
		}
		if err := repositories.TicketProgressRepository.Create(tx, &models.TicketProgress{
			TenantID:     item.TenantID,
			TicketID:     item.TicketID,
			EventType:    enums.TicketProgressEventAssigned,
			Content:      "供应商内部派工：" + target.DisplayName,
			MetadataJSON: partnerProgressMetadata(item.ID),
			AuthorID:     operator.UserID,
			CreatedAt:    now,
		}); err != nil {
			return err
		}
		return s.ensurePartnerConversationParticipantTx(tx, ticket.ConversationID, target.UserID, target.ID, now, operator)
	})
	if err != nil {
		return nil, err
	}
	item = repositories.TicketSupplierCollaborationRepository.Get(sqls.DB(), item.ID)
	aggregate := s.buildAggregate(item)
	return &aggregate, nil
}

func (s *ticketSupplierCollaborationService) AddParticipant(id, partnerAccountID int64, operator *dto.AuthPrincipal) (*PartnerTicketDetailAggregate, error) {
	current, company, err := s.requirePartnerAccount(operator)
	if err != nil {
		return nil, err
	}
	if !s.partnerHasRole(sqls.DB(), current, PartnerRoleAdmin) {
		return nil, errorsx.ForbiddenI18n("error.e0225")
	}
	item, err := s.requirePartnerCollaborationWork(id, operator)
	if err != nil {
		return nil, err
	}
	if item.PartnerCompanyID != company.ID {
		return nil, errorsx.ForbiddenI18n("error.e0225")
	}
	if item.AuthorizationEnds != nil && !item.AuthorizationEnds.After(time.Now()) {
		return nil, errorsx.ForbiddenI18n("error.e0225")
	}
	target := repositories.TicketSupplierCollaborationRepository.GetPartnerAccount(sqls.DB(), item.TenantID, company.ID, partnerAccountID)
	if target == nil {
		return nil, errorsx.InvalidParam("supplier collaboration member is not active")
	}
	now := time.Now()
	err = sqls.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.ensureSupplierCollaborationParticipantTx(tx, item, target, participantRoleForAccount(item, target.ID), now, operator); err != nil {
			return err
		}
		visibility := normalizeSupplierVisibility(nil)
		if err := json.Unmarshal([]byte(item.VisibilityJSON), &visibility); err != nil {
			visibility = normalizeSupplierVisibility(nil)
		}
		scopeRuleJSON, _ := json.Marshal(map[string]any{"visibility": normalizeSupplierVisibility(visibility), "collaboration_id": item.ID})
		if err := s.ensureSupplierAuthorizationScopeTx(tx, item, target.ID, string(scopeRuleJSON), item.AuthorizationEnds, operator); err != nil {
			return err
		}
		ticket := repositories.TicketRepository.Get(tx, item.TicketID)
		if ticket == nil {
			return errorsx.InvalidParam("ticket not found")
		}
		if err := s.ensurePartnerConversationParticipantTx(tx, ticket.ConversationID, target.UserID, target.ID, now, operator); err != nil {
			return err
		}
		return repositories.TicketProgressRepository.Create(tx, &models.TicketProgress{
			TenantID:     item.TenantID,
			TicketID:     item.TicketID,
			EventType:    enums.TicketProgressEventAssigned,
			Content:      "供应商协作成员加入：" + target.DisplayName,
			MetadataJSON: partnerProgressMetadata(item.ID),
			AuthorID:     operator.UserID,
			CreatedAt:    now,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.PartnerTicketDetail(id, operator)
}

func (s *ticketSupplierCollaborationService) RemoveParticipant(id, partnerAccountID int64, operator *dto.AuthPrincipal) (*PartnerTicketDetailAggregate, error) {
	current, company, err := s.requirePartnerAccount(operator)
	if err != nil {
		return nil, err
	}
	if !s.partnerHasRole(sqls.DB(), current, PartnerRoleAdmin) {
		return nil, errorsx.ForbiddenI18n("error.e0225")
	}
	item, err := s.requirePartnerCollaborationWork(id, operator)
	if err != nil {
		return nil, err
	}
	if item.PartnerCompanyID != company.ID {
		return nil, errorsx.ForbiddenI18n("error.e0225")
	}
	if item.PartnerAccountID == partnerAccountID {
		return nil, errorsx.InvalidParam("change the primary supplier owner before removing this member")
	}
	participant := repositories.TicketSupplierCollaborationRepository.FindActiveParticipant(sqls.DB(), item.TenantID, item.ID, partnerAccountID)
	account := repositories.TicketSupplierCollaborationRepository.GetPartnerAccountAnyStatus(sqls.DB(), item.TenantID, company.ID, partnerAccountID)
	if participant == nil || account == nil {
		return nil, errorsx.InvalidParam("supplier collaboration member not found")
	}
	now := time.Now()
	err = sqls.DB().Transaction(func(tx *gorm.DB) error {
		if err := repositories.TicketSupplierCollaborationRepository.UpdateParticipant(tx, participant.ID, map[string]any{
			"left_at":          now,
			"status":           enums.StatusDisabled,
			"updated_at":       now,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
		}); err != nil {
			return err
		}
		if err := repositories.TicketSupplierCollaborationRepository.DisableCollaborationAuthorizationScopes(tx, item.TenantID, item.TicketID, item.ID, []int64{partnerAccountID}, now); err != nil {
			return err
		}
		ticket := repositories.TicketRepository.Get(tx, item.TicketID)
		if ticket != nil && repositories.TicketSupplierCollaborationRepository.CountActiveParticipantByTicket(tx, item.TenantID, item.TicketID, partnerAccountID) == 0 {
			if err := s.deactivatePartnerConversationParticipantTx(tx, ticket.ConversationID, account.UserID, now, operator); err != nil {
				return err
			}
		}
		return repositories.TicketProgressRepository.Create(tx, &models.TicketProgress{
			TenantID:     item.TenantID,
			TicketID:     item.TicketID,
			EventType:    enums.TicketProgressEventAssigned,
			Content:      "供应商协作成员退出：" + account.DisplayName,
			MetadataJSON: partnerProgressMetadata(item.ID),
			AuthorID:     operator.UserID,
			CreatedAt:    now,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.PartnerTicketDetail(id, operator)
}

func (s *ticketSupplierCollaborationService) Resolve(id int64, resolution string, operator *dto.AuthPrincipal) (*TicketSupplierCollaborationAggregate, error) {
	item := repositories.TicketSupplierCollaborationRepository.Get(sqls.DB(), id)
	if item == nil || item.RecordStatus == enums.StatusDeleted {
		return nil, errorsx.InvalidParam("supplier collaboration not found")
	}
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if operator.IsPartner() {
		var err error
		item, err = s.requirePartnerCollaborationRead(id, operator)
		if err != nil {
			return nil, err
		}
	} else {
		ticket := repositories.TicketRepository.Get(sqls.DB(), item.TicketID)
		if err := requireTicketTenantAccess(ticket, operator); err != nil {
			return nil, err
		}
	}
	if isSupplierCollaborationTerminal(item.Status) {
		if operator.IsPartner() {
			return nil, errorsx.ForbiddenI18n("error.e0225")
		}
		aggregate := s.buildAggregate(item)
		return &aggregate, nil
	}
	resolution = strings.TrimSpace(resolution)
	if resolution == "" {
		return nil, errorsx.InvalidParam("supplier resolution is required")
	}
	now := time.Now()
	resolvedNow := false
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		tx := ctx.Tx
		locked := repositories.TicketSupplierCollaborationRepository.GetForUpdate(tx, item.ID)
		if locked == nil || locked.RecordStatus == enums.StatusDeleted {
			return errorsx.InvalidParam("supplier collaboration not found")
		}
		if isSupplierCollaborationTerminal(locked.Status) {
			item = locked
			if operator.IsPartner() {
				return errorsx.ForbiddenI18n("error.e0225")
			}
			return nil
		}
		if err := repositories.TicketSupplierCollaborationRepository.Updates(tx, locked.ID, map[string]any{
			"status":           SupplierCollaborationResolved,
			"resolution":       resolution,
			"resolved_at":      now,
			"updated_at":       now,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
		}); err != nil {
			return err
		}
		locked.Status = SupplierCollaborationResolved
		locked.Resolution = resolution
		locked.ResolvedAt = &now
		item = locked
		progress := &models.TicketProgress{
			TenantID:     locked.TenantID,
			TicketID:     locked.TicketID,
			EventType:    enums.TicketProgressEventProgress,
			Content:      "供应商处理完成：" + resolution,
			MetadataJSON: partnerProgressMetadata(item.ID),
			AuthorID:     operator.UserID,
			CreatedAt:    now,
		}
		if err := repositories.TicketProgressRepository.Create(tx, progress); err != nil {
			return err
		}
		if operator.IsPartner() {
			if err := s.appendPartnerConversationMessageTx(
				ctx,
				repositories.TicketRepository.Get(tx, locked.TicketID),
				locked,
				progress,
				enums.IMMessageTypeText,
				progress.Content,
				"",
				operator,
				false,
			); err != nil {
				return err
			}
		}
		participants := repositories.TicketSupplierCollaborationRepository.FindParticipants(tx, locked.TenantID, locked.ID)
		activeAccountIDs := make([]int64, 0, len(participants))
		for i := range participants {
			if participants[i].Status == enums.StatusOk {
				activeAccountIDs = append(activeAccountIDs, participants[i].PartnerAccountID)
			}
		}
		if err := repositories.TicketSupplierCollaborationRepository.DisableCollaborationAuthorizationScopes(
			tx, locked.TenantID, locked.TicketID, locked.ID, activeAccountIDs, now,
		); err != nil {
			return err
		}
		if err := repositories.TicketSupplierCollaborationRepository.DisableActiveParticipants(
			tx, locked.TenantID, locked.ID, now, operator.UserID, operator.Username,
		); err != nil {
			return err
		}
		ticket := repositories.TicketRepository.Get(tx, locked.TicketID)
		for _, accountID := range activeAccountIDs {
			if repositories.TicketSupplierCollaborationRepository.CountActiveParticipantByTicket(tx, locked.TenantID, locked.TicketID, accountID) > 0 {
				continue
			}
			account := repositories.TicketSupplierCollaborationRepository.GetPartnerAccountAnyStatus(tx, locked.TenantID, locked.PartnerCompanyID, accountID)
			if ticket != nil && account != nil {
				if err := s.deactivatePartnerConversationParticipantTx(tx, ticket.ConversationID, account.UserID, now, operator); err != nil {
					return err
				}
			}
		}
		if repositories.TicketSupplierCollaborationRepository.CountActiveByTicket(tx, locked.TenantID, locked.TicketID) == 0 {
			current := repositories.TicketRepository.Get(tx, locked.TicketID)
			if current != nil && enums.NormalizeTicketStatus(string(current.Status)) == enums.TicketStatusSupplierSupport {
				if err := repositories.TicketRepository.Updates(tx, locked.TicketID, map[string]any{
					"status":           enums.TicketStatusProcessing,
					"updated_at":       now,
					"update_user_id":   operator.UserID,
					"update_user_name": operator.Username,
				}); err != nil {
					return err
				}
			}
		}
		resolvedNow = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	if resolvedNow {
		item = repositories.TicketSupplierCollaborationRepository.Get(sqls.DB(), item.ID)
	}
	aggregate := s.buildAggregate(item)
	return &aggregate, nil
}

func (s *ticketSupplierCollaborationService) ResolveForTicket(ticketID, collaborationID int64, resolution string, operator *dto.AuthPrincipal) (*TicketSupplierCollaborationAggregate, error) {
	item := repositories.TicketSupplierCollaborationRepository.Get(sqls.DB(), collaborationID)
	if item == nil || item.TicketID != ticketID {
		return nil, errorsx.InvalidParam("supplier collaboration does not belong to ticket")
	}
	return s.Resolve(collaborationID, resolution, operator)
}

func (s *ticketSupplierCollaborationService) PartnerTicketDetail(id int64, operator *dto.AuthPrincipal) (*PartnerTicketDetailAggregate, error) {
	item, err := s.requirePartnerCollaborationRead(id, operator)
	if err != nil {
		return nil, err
	}
	return s.buildTicketDetail(item), nil
}

func (s *ticketSupplierCollaborationService) EnterpriseTicketDetail(ticketID, collaborationID int64, operator *dto.AuthPrincipal) (*PartnerTicketDetailAggregate, error) {
	item := repositories.TicketSupplierCollaborationRepository.Get(sqls.DB(), collaborationID)
	if item == nil || item.TicketID != ticketID || item.RecordStatus == enums.StatusDeleted {
		return nil, errorsx.InvalidParam("supplier collaboration does not belong to ticket")
	}
	ticket := repositories.TicketRepository.Get(sqls.DB(), ticketID)
	if err := requireTicketTenantAccess(ticket, operator); err != nil {
		return nil, err
	}
	return s.buildTicketDetail(item), nil
}

func (s *ticketSupplierCollaborationService) buildTicketDetail(item *models.TicketSupplierCollaboration) *PartnerTicketDetailAggregate {
	collaboration := s.buildAggregate(item)
	progresses := repositories.TicketProgressRepository.Find(sqls.DB(), sqls.NewCnd().Eq("ticket_id", item.TicketID).Asc("id"))
	visible := make([]models.TicketProgress, 0, len(progresses))
	userIDs := make([]int64, 0)
	seenUsers := make(map[int64]bool)
	for i := range progresses {
		if partnerProgressCollaborationID(progresses[i].MetadataJSON) != item.ID {
			continue
		}
		visible = append(visible, progresses[i])
		if progresses[i].AuthorID > 0 && !seenUsers[progresses[i].AuthorID] {
			seenUsers[progresses[i].AuthorID] = true
			userIDs = append(userIDs, progresses[i].AuthorID)
		}
	}
	messages := make([]models.Message, 0)
	if collaboration.Ticket != nil && collaboration.Ticket.ConversationID > 0 && supplierVisibilityAllows(item, "repair_progress") {
		messages = repositories.MessageRepository.FindSupplierConversationMessages(sqls.DB(), collaboration.Ticket.ConversationID, item.InvitedAt, item.ResolvedAt)
		for i := range messages {
			if messages[i].SenderType == enums.IMSenderTypePartner && partnerMessageCollaborationID(messages[i].Payload) != item.ID {
				continue
			}
			if messages[i].SenderID > 0 && !seenUsers[messages[i].SenderID] {
				seenUsers[messages[i].SenderID] = true
				userIDs = append(userIDs, messages[i].SenderID)
			}
		}
	}
	users := make(map[int64]*models.User)
	for _, user := range repositories.UserRepository.FindByIds(sqls.DB(), userIDs) {
		current := user
		users[user.ID] = &current
	}
	detail := &PartnerTicketDetailAggregate{
		Collaboration: collaboration,
		Progresses:    make([]PartnerTicketProgressAggregate, 0, len(visible)),
		Messages:      make([]PartnerConversationMessageAggregate, 0, len(messages)),
	}
	for i := range visible {
		authorName := "系统"
		if user := users[visible[i].AuthorID]; user != nil {
			authorName = strings.TrimSpace(user.Nickname)
			if authorName == "" {
				authorName = user.Username
			}
		}
		detail.Progresses = append(detail.Progresses, PartnerTicketProgressAggregate{Progress: visible[i], AuthorName: authorName})
	}
	for i := range messages {
		message := messages[i]
		if message.SenderType == enums.IMSenderTypePartner && partnerMessageCollaborationID(message.Payload) != item.ID {
			continue
		}
		senderName := enums.GetIMSenderTypeLabel(message.SenderType)
		if user := users[message.SenderID]; user != nil {
			senderName = strings.TrimSpace(user.Nickname)
			if senderName == "" {
				senderName = user.Username
			}
		}
		detail.Messages = append(detail.Messages, PartnerConversationMessageAggregate{Message: message, SenderName: senderName})
	}
	return detail
}

func (s *ticketSupplierCollaborationService) ensureSupplierCollaborationParticipantTx(
	tx *gorm.DB,
	collaboration *models.TicketSupplierCollaboration,
	account *models.PartnerAccount,
	role string,
	joinedAt time.Time,
	operator *dto.AuthPrincipal,
) error {
	if tx == nil || collaboration == nil || account == nil {
		return nil
	}
	if role != SupplierParticipantRoleOwner {
		role = SupplierParticipantRoleMember
	}
	existing := repositories.TicketSupplierCollaborationRepository.FindParticipantAnyStatus(tx, collaboration.TenantID, collaboration.ID, account.ID)
	if existing != nil {
		updates := map[string]any{
			"partner_company_id": collaboration.PartnerCompanyID,
			"role":               role,
			"status":             enums.StatusOk,
			"updated_at":         joinedAt,
			"update_user_id":     operator.UserID,
			"update_user_name":   operator.Username,
		}
		if existing.Status != enums.StatusOk {
			updates["joined_at"] = joinedAt
			updates["left_at"] = nil
		}
		return repositories.TicketSupplierCollaborationRepository.UpdateParticipant(tx, existing.ID, updates)
	}
	return repositories.TicketSupplierCollaborationRepository.CreateParticipant(tx, &models.TicketSupplierCollaborationParticipant{
		TenantID:         collaboration.TenantID,
		CollaborationID:  collaboration.ID,
		PartnerCompanyID: collaboration.PartnerCompanyID,
		PartnerAccountID: account.ID,
		Role:             role,
		JoinedAt:         joinedAt,
		Status:           enums.StatusOk,
		AuditFields:      utils.BuildAuditFields(operator),
	})
}

func (s *ticketSupplierCollaborationService) ensureSupplierAuthorizationScopeTx(
	tx *gorm.DB,
	collaboration *models.TicketSupplierCollaboration,
	partnerAccountID int64,
	scopeRuleJSON string,
	expiredAt *time.Time,
	operator *dto.AuthPrincipal,
) error {
	if tx == nil || collaboration == nil || partnerAccountID <= 0 {
		return nil
	}
	now := time.Now()
	existing := repositories.TicketSupplierCollaborationRepository.FindAuthorizationScope(tx, collaboration.TenantID, collaboration.TicketID, collaboration.ID, partnerAccountID)
	if existing != nil {
		return repositories.TicketSupplierCollaborationRepository.UpdateAuthorizationScope(tx, existing.ID, map[string]any{
			"scope_rule_json":  scopeRuleJSON,
			"expired_at":       expiredAt,
			"status":           enums.StatusOk,
			"updated_at":       now,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
		})
	}
	return repositories.TicketSupplierCollaborationRepository.CreateAuthorizationScope(tx, &models.PartnerAuthorizationScope{
		TenantID:         collaboration.TenantID,
		PartnerAccountID: partnerAccountID,
		ResourceType:     "ticket",
		ResourceID:       strconv.FormatInt(collaboration.TicketID, 10),
		ScopeRuleJSON:    scopeRuleJSON,
		ExpiredAt:        expiredAt,
		Status:           enums.StatusOk,
		AuditFields:      utils.BuildAuditFields(operator),
	})
}

func (s *ticketSupplierCollaborationService) ensurePartnerConversationParticipantTx(tx *gorm.DB, conversationID, userID, partnerAccountID int64, joinedAt time.Time, operator *dto.AuthPrincipal) error {
	if tx == nil || conversationID <= 0 || userID <= 0 {
		return nil
	}
	existing := repositories.ConversationParticipantRepository.FindOne(tx, sqls.NewCnd().
		Eq("conversation_id", conversationID).
		Eq("participant_type", enums.IMParticipantTypePartner).
		Eq("participant_id", userID))
	if existing != nil {
		return repositories.ConversationParticipantRepository.Updates(tx, existing.ID, map[string]any{
			"external_participant_id": "partner-account:" + strconv.FormatInt(partnerAccountID, 10),
			"joined_at":               joinedAt,
			"left_at":                 nil,
			"status":                  enums.StatusOk,
			"updated_at":              joinedAt,
			"update_user_id":          operator.UserID,
			"update_user_name":        operator.Username,
		})
	}
	return repositories.ConversationParticipantRepository.Create(tx, &models.ConversationParticipant{
		ConversationID:        conversationID,
		ParticipantType:       string(enums.IMParticipantTypePartner),
		ParticipantID:         userID,
		ExternalParticipantID: "partner-account:" + strconv.FormatInt(partnerAccountID, 10),
		JoinedAt:              &joinedAt,
		Status:                enums.StatusOk,
		AuditFields:           utils.BuildAuditFields(operator),
	})
}

func (s *ticketSupplierCollaborationService) deactivatePartnerConversationParticipantTx(tx *gorm.DB, conversationID, userID int64, leftAt time.Time, operator *dto.AuthPrincipal) error {
	if tx == nil || conversationID <= 0 || userID <= 0 {
		return nil
	}
	return tx.Model(&models.ConversationParticipant{}).
		Where("conversation_id = ? AND participant_type = ? AND participant_id = ? AND status = ?", conversationID, enums.IMParticipantTypePartner, userID, enums.StatusOk).
		Updates(map[string]any{
			"left_at":          leftAt,
			"status":           enums.StatusDisabled,
			"updated_at":       leftAt,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
		}).Error
}

func (s *ticketSupplierCollaborationService) appendPartnerConversationMessageTx(ctx *sqls.TxContext, ticket *models.Ticket, collaboration *models.TicketSupplierCollaboration, progress *models.TicketProgress, messageType enums.IMMessageType, content, messagePayload string, operator *dto.AuthPrincipal, allowClosed bool) error {
	if ctx == nil || ctx.Tx == nil || ticket == nil || collaboration == nil || progress == nil || operator == nil || ticket.ConversationID <= 0 {
		return nil
	}
	if !supplierVisibilityAllows(collaboration, "repair_progress") {
		return nil
	}
	conversation := repositories.ConversationRepository.Get(ctx.Tx, ticket.ConversationID)
	if conversation == nil || conversation.TenantID != ticket.TenantID || (conversation.Status == enums.IMConversationStatusClosed && !allowClosed) {
		return nil
	}
	if !allowClosed {
		account := repositories.TicketSupplierCollaborationRepository.GetPartnerAccountAnyStatus(ctx.Tx, collaboration.TenantID, collaboration.PartnerCompanyID, operator.PartnerAccountID)
		if account != nil {
			if err := s.ensureSupplierCollaborationParticipantTx(ctx.Tx, collaboration, account, participantRoleForAccount(collaboration, account.ID), progress.CreatedAt, operator); err != nil {
				return err
			}
			visibility := normalizeSupplierVisibility(nil)
			if err := json.Unmarshal([]byte(collaboration.VisibilityJSON), &visibility); err != nil {
				visibility = normalizeSupplierVisibility(nil)
			}
			scopeRuleJSON, _ := json.Marshal(map[string]any{"visibility": normalizeSupplierVisibility(visibility), "collaboration_id": collaboration.ID})
			if err := s.ensureSupplierAuthorizationScopeTx(ctx.Tx, collaboration, account.ID, string(scopeRuleJSON), collaboration.AuthorizationEnds, operator); err != nil {
				return err
			}
		}
	}
	if err := s.ensurePartnerConversationParticipantTx(ctx.Tx, conversation.ID, operator.UserID, operator.PartnerAccountID, progress.CreatedAt, operator); err != nil {
		return err
	}
	clientMsgID := "supplier-progress:" + strconv.FormatInt(progress.ID, 10)
	if repositories.MessageRepository.GetByClientMsgID(ctx.Tx, conversation.ID, clientMsgID) != nil {
		return nil
	}
	if messageType == "" {
		messageType = enums.IMMessageTypeText
	}
	payloadFields := map[string]any{
		"source":           "partner",
		"collaborationId":  collaboration.ID,
		"partnerAccountId": operator.PartnerAccountID,
		"ticketProgressId": progress.ID,
	}
	if company := repositories.TicketSupplierCollaborationRepository.GetPartnerCompany(ctx.Tx, collaboration.TenantID, collaboration.PartnerCompanyID); company != nil && strings.TrimSpace(company.Name) != "" {
		payloadFields["partnerCompanyName"] = strings.TrimSpace(company.Name)
	}
	if module := repositories.ProductModuleRepository.Get(ctx.Tx, collaboration.ProductModuleID); module != nil && module.TenantID == collaboration.TenantID && strings.TrimSpace(module.Name) != "" {
		payloadFields["productModuleName"] = strings.TrimSpace(module.Name)
	}
	if strings.TrimSpace(messagePayload) != "" {
		var assetFields map[string]any
		if json.Unmarshal([]byte(messagePayload), &assetFields) == nil {
			for key, value := range assetFields {
				payloadFields[key] = value
			}
		}
	}
	payload, _ := json.Marshal(payloadFields)
	now := progress.CreatedAt
	message := &models.Message{
		ConversationID: conversation.ID,
		ClientMsgID:    clientMsgID,
		SenderType:     enums.IMSenderTypePartner,
		SenderID:       operator.UserID,
		MessageType:    messageType,
		Content:        strings.TrimSpace(content),
		Payload:        string(payload),
		SendStatus:     enums.IMMessageStatusSent,
		SentAt:         &now,
		AuditFields: models.AuditFields{
			CreatedAt:      now,
			CreateUserID:   operator.UserID,
			CreateUserName: operator.Username,
			UpdatedAt:      now,
			UpdateUserID:   operator.UserID,
			UpdateUserName: operator.Username,
		},
	}
	if err := repositories.MessageRepository.Create(ctx.Tx, message); err != nil {
		return err
	}
	agentReadState, customerReadState := ConversationReadStateService.getConversationReadStates(ctx.Tx, conversation.ID)
	agentUnreadCount, err := ConversationReadStateService.CountUnreadMessages(ctx, conversation.ID, MessageService.readMessageID(agentReadState), enums.IMSenderTypeCustomer, enums.IMSenderTypePartner)
	if err != nil {
		return err
	}
	customerUnreadCount, err := ConversationReadStateService.CountUnreadMessages(ctx, conversation.ID, MessageService.readMessageID(customerReadState), enums.IMSenderTypeAgent, enums.IMSenderTypeAI, enums.IMSenderTypePartner)
	if err != nil {
		return err
	}
	conversation.AgentUnreadCount = int(agentUnreadCount)
	conversation.CustomerUnreadCount = int(customerUnreadCount)
	conversationUpdates := map[string]any{
		"agent_unread_count":    conversation.AgentUnreadCount,
		"customer_unread_count": conversation.CustomerUnreadCount,
	}
	if conversation.LastMessageAt.IsZero() || !conversation.LastMessageAt.After(now) {
		conversation.LastMessageID = message.ID
		conversation.LastMessageAt = now
		conversation.LastActiveAt = now
		conversation.LastMessageSummary = limitText(buildMessageSummary(messageType, content), 255)
		conversation.UpdatedAt = now
		conversation.UpdateUserID = operator.UserID
		conversation.UpdateUserName = operator.Username
		conversationUpdates["last_message_id"] = conversation.LastMessageID
		conversationUpdates["last_message_at"] = conversation.LastMessageAt
		conversationUpdates["last_active_at"] = conversation.LastActiveAt
		conversationUpdates["last_message_summary"] = conversation.LastMessageSummary
		conversationUpdates["updated_at"] = conversation.UpdatedAt
		conversationUpdates["update_user_id"] = conversation.UpdateUserID
		conversationUpdates["update_user_name"] = conversation.UpdateUserName
	}
	if err := repositories.ConversationRepository.Updates(ctx.Tx, conversation.ID, conversationUpdates); err != nil {
		return err
	}
	if err := ConversationEventLogService.CreateEvent(ctx, conversation.ID, enums.IMEventTypeMessageSend, enums.IMSenderTypePartner, operator.UserID, "供应商发送消息", string(payload)); err != nil {
		return err
	}
	if err := MessageService.recordConversationMessageAuditTx(ctx, conversation, message, operator, nil, "conversation.message_sent", buildMessageSummary(messageType, content), map[string]any{
		"collaborationId":  collaboration.ID,
		"ticketId":         collaboration.TicketID,
		"ticketProgressId": progress.ID,
	}); err != nil {
		slog.Warn("record partner conversation message audit failed", "conversation_id", conversation.ID, "message_id", message.ID, "error", err)
	}
	if ctx.RegisterCallback != nil {
		conversationCopy := *conversation
		messageCopy := *message
		ctx.RegisterCallback(func() {
			WsService.PublishMessageCreated(&conversationCopy, &messageCopy)
			WsService.PublishConversationChanged(&conversationCopy, enums.IMRealtimeEventConversationUpdated)
			if err := ChannelMessageOutboxService.EnqueueWxWorkKFMessage(&conversationCopy, &messageCopy); err != nil {
				slog.Error("enqueue partner conversation message failed", "conversation_id", conversationCopy.ID, "message_id", messageCopy.ID, "error", err)
			}
		})
	}
	return nil
}

func (s *ticketSupplierCollaborationService) AddPartnerProgress(id int64, content string, operator *dto.AuthPrincipal) (*PartnerTicketDetailAggregate, error) {
	return s.AddPartnerMessage(id, dto.PartnerTicketProgressCreateRequest{
		Content:     content,
		MessageType: string(enums.IMMessageTypeText),
	}, operator)
}

func (s *ticketSupplierCollaborationService) ResolvePartnerMessageUploadConversation(id int64, operator *dto.AuthPrincipal) (*models.Conversation, error) {
	item, err := s.requirePartnerCollaborationWork(id, operator)
	if err != nil {
		return nil, err
	}
	if item.Status == SupplierCollaborationInvited {
		return nil, errorsx.InvalidParam("accept supplier collaboration before uploading a message asset")
	}
	if isSupplierCollaborationTerminal(item.Status) {
		return nil, errorsx.InvalidParam("closed supplier collaboration cannot upload message assets")
	}
	ticket := repositories.TicketRepository.Get(sqls.DB(), item.TicketID)
	if ticket == nil || ticket.TenantID != item.TenantID || ticket.ConversationID <= 0 {
		return nil, errorsx.InvalidParam("supplier collaboration conversation is unavailable")
	}
	conversation := repositories.ConversationRepository.Get(sqls.DB(), ticket.ConversationID)
	if conversation == nil || conversation.TenantID != item.TenantID || conversation.Status == enums.IMConversationStatusClosed {
		return nil, errorsx.InvalidParam("supplier collaboration conversation is closed")
	}
	return conversation, nil
}

func (s *ticketSupplierCollaborationService) AddPartnerMessage(id int64, req dto.PartnerTicketProgressCreateRequest, operator *dto.AuthPrincipal) (*PartnerTicketDetailAggregate, error) {
	item, err := s.requirePartnerCollaborationWork(id, operator)
	if err != nil {
		return nil, err
	}
	if item.Status == SupplierCollaborationInvited {
		return nil, errorsx.InvalidParam("accept supplier collaboration before adding progress")
	}
	if isSupplierCollaborationTerminal(item.Status) {
		return nil, errorsx.InvalidParam("closed supplier collaboration cannot add progress")
	}
	messageType := enums.IMMessageType(strings.TrimSpace(req.MessageType))
	if messageType == "" {
		messageType = enums.IMMessageTypeText
	}
	content := strings.TrimSpace(req.Content)
	messagePayload := ""
	progressContent := content
	switch messageType {
	case enums.IMMessageTypeText:
		if content == "" {
			return nil, errorsx.InvalidParam("progress content is required")
		}
	case enums.IMMessageTypeImage, enums.IMMessageTypeAudio, enums.IMMessageTypeAttachment:
		asset := AssetService.GetByAssetID(strings.TrimSpace(req.AssetID))
		ticket := repositories.TicketRepository.Get(sqls.DB(), item.TicketID)
		if ticket == nil || ticket.TenantID != item.TenantID || ticket.ConversationID <= 0 {
			return nil, errorsx.InvalidParam("supplier collaboration conversation is unavailable")
		}
		asset, err = AssetService.claimLegacyConversationAsset(asset, ticket.ConversationID, messageType, operator)
		if err != nil {
			return nil, err
		}
		messagePayload, err = buildIMMessageAssetPayload(asset)
		if err != nil {
			return nil, err
		}
		if messageType == enums.IMMessageTypeAudio && req.DurationSeconds > 0 {
			var payload imMessageAssetPayload
			if json.Unmarshal([]byte(messagePayload), &payload) == nil {
				payload.DurationSeconds = min(req.DurationSeconds, 120)
				if data, marshalErr := json.Marshal(payload); marshalErr == nil {
					messagePayload = string(data)
				}
			}
		}
		content = strings.TrimSpace(asset.Filename)
		progressContent = buildMessageSummary(messageType, content)
	default:
		return nil, errorsx.InvalidParam("unsupported supplier message type")
	}
	now := time.Now()
	err = sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		progress := &models.TicketProgress{
			TenantID:     item.TenantID,
			TicketID:     item.TicketID,
			EventType:    enums.TicketProgressEventProgress,
			Content:      progressContent,
			MetadataJSON: partnerProgressMetadata(item.ID),
			AuthorID:     operator.UserID,
			CreatedAt:    now,
		}
		if err := repositories.TicketProgressRepository.Create(ctx.Tx, progress); err != nil {
			return err
		}
		if err := repositories.TicketRepository.Updates(ctx.Tx, item.TicketID, map[string]any{
			"updated_at":       now,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
		}); err != nil {
			return err
		}
		ticket := repositories.TicketRepository.Get(ctx.Tx, item.TicketID)
		return s.appendPartnerConversationMessageTx(ctx, ticket, item, progress, messageType, content, messagePayload, operator, false)
	})
	if err != nil {
		return nil, err
	}
	return s.PartnerTicketDetail(id, operator)
}

func (s *ticketSupplierCollaborationService) AddEnterpriseProgress(ticketID, collaborationID int64, content string, operator *dto.AuthPrincipal) (*PartnerTicketDetailAggregate, error) {
	detail, err := s.EnterpriseTicketDetail(ticketID, collaborationID, operator)
	if err != nil {
		return nil, err
	}
	item := detail.Collaboration.Collaboration
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, errorsx.InvalidParam("progress content is required")
	}
	if isSupplierCollaborationTerminal(item.Status) {
		return nil, errorsx.InvalidParam("closed supplier collaboration cannot add progress")
	}
	now := time.Now()
	err = sqls.DB().Transaction(func(tx *gorm.DB) error {
		if err := repositories.TicketProgressRepository.Create(tx, &models.TicketProgress{
			TenantID:     item.TenantID,
			TicketID:     item.TicketID,
			EventType:    enums.TicketProgressEventProgress,
			Content:      content,
			MetadataJSON: partnerProgressMetadata(item.ID),
			AuthorID:     operator.UserID,
			CreatedAt:    now,
		}); err != nil {
			return err
		}
		return repositories.TicketRepository.Updates(tx, item.TicketID, map[string]any{
			"updated_at":       now,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.EnterpriseTicketDetail(ticketID, collaborationID, operator)
}

func (s *ticketSupplierCollaborationService) ListMeetingsForPartner(ctx context.Context, ticketID int64, operator *dto.AuthPrincipal) ([]dto.EnterpriseMeetingListItemDTO, error) {
	item, err := s.requirePartnerTicket(ticketID, operator)
	if err != nil {
		return nil, err
	}
	if !supplierVisibilityAllows(item, "meeting") {
		return nil, errorsx.ForbiddenI18n("error.e0225")
	}
	return MeetingService.ListTicketMeetings(ctx, operator.TenantID, ticketID)
}

func (s *ticketSupplierCollaborationService) JoinMeeting(ctx context.Context, collaborationID int64, meetingID string, operator *dto.AuthPrincipal) (*JoinConfig, error) {
	item, meeting, account, err := s.requirePartnerMeetingAccess(ctx, collaborationID, meetingID, operator, false, false)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(account.DisplayName)
	if name == "" {
		name = "Supplier Engineer"
	}
	return MeetingService.JoinMeeting(ctx, meeting.ID, strconv.FormatInt(account.UserID, 10), name, "partner", item.TenantID)
}

// JoinMeetingForTicket resolves the legacy ticket-scoped route to an
// authorized collaboration before issuing the shared meeting token.
func (s *ticketSupplierCollaborationService) JoinMeetingForTicket(ctx context.Context, ticketID int64, meetingID string, operator *dto.AuthPrincipal) (*JoinConfig, error) {
	account, _, err := s.requirePartnerAccount(operator)
	if err != nil {
		return nil, err
	}

	var collaborations []models.TicketSupplierCollaboration
	if s.partnerHasRole(sqls.DB(), account, PartnerRoleAdmin) {
		collaborations = s.findPartnerCompanyCollaborations(sqls.DB(), operator.TenantID, account.PartnerCompanyID)
	} else {
		collaborations = s.findPartnerAccountCollaborations(sqls.DB(), operator.TenantID, account.ID)
	}
	for i := range collaborations {
		if collaborations[i].TicketID != ticketID {
			continue
		}
		if _, _, _, accessErr := s.requirePartnerMeetingAccess(ctx, collaborations[i].ID, meetingID, operator, false, false); accessErr == nil {
			return s.JoinMeeting(ctx, collaborations[i].ID, meetingID, operator)
		}
	}
	return nil, errorsx.ForbiddenI18n("error.e0225")
}

func (s *ticketSupplierCollaborationService) ConfirmMeetingJoined(ctx context.Context, collaborationID int64, meetingID string, operator *dto.AuthPrincipal) error {
	_, meeting, account, err := s.requirePartnerMeetingAccess(ctx, collaborationID, meetingID, operator, false, false)
	if err != nil {
		return err
	}
	return MeetingService.confirmParticipantJoin(
		meeting.ID,
		strconv.FormatInt(account.UserID, 10),
		"partner",
		firstNonEmptyString(strings.TrimSpace(account.DisplayName), "Supplier Engineer"),
		time.Now(),
	)
}

func (s *ticketSupplierCollaborationService) ConfirmMeetingLeft(ctx context.Context, collaborationID int64, meetingID string, operator *dto.AuthPrincipal) error {
	_, meeting, account, err := s.requirePartnerMeetingAccess(ctx, collaborationID, meetingID, operator, true, true)
	if err != nil {
		return err
	}
	return MeetingService.confirmParticipantLeave(meeting, strconv.FormatInt(account.UserID, 10), "partner", time.Now())
}

func (s *ticketSupplierCollaborationService) HeartbeatMeeting(ctx context.Context, collaborationID int64, meetingID string, operator *dto.AuthPrincipal) error {
	_, meeting, account, err := s.requirePartnerMeetingAccess(ctx, collaborationID, meetingID, operator, false, false)
	if err != nil {
		return err
	}
	return MeetingService.heartbeatParticipant(meeting, strconv.FormatInt(account.UserID, 10), "partner", time.Now())
}

func (s *ticketSupplierCollaborationService) GetMeetingStatus(ctx context.Context, collaborationID int64, meetingID string, operator *dto.AuthPrincipal) (*MeetingRoomStatus, error) {
	item, meeting, _, err := s.requirePartnerMeetingAccess(ctx, collaborationID, meetingID, operator, true, true)
	if err != nil {
		return nil, err
	}
	return MeetingService.GetMeetingStatus(ctx, meeting.ID, item.TenantID)
}

func (s *ticketSupplierCollaborationService) ListMeetingTranscripts(collaborationID int64, meetingID string, operator *dto.AuthPrincipal) ([]models.MeetingTranscriptSegment, error) {
	item, meeting, _, err := s.requirePartnerMeetingAccess(context.Background(), collaborationID, meetingID, operator, true, true)
	if err != nil {
		return nil, err
	}
	return repositories.MeetingIntelligenceRepository.ListTranscripts(sqls.DB(), item.TenantID, meeting.ID, 100)
}

func (s *ticketSupplierCollaborationService) ListMeetingTranscriptPage(collaborationID int64, meetingID, cursor string, limit int, operator *dto.AuthPrincipal) ([]models.MeetingTranscriptSegment, string, bool, error) {
	item, meeting, _, err := s.requirePartnerMeetingAccess(context.Background(), collaborationID, meetingID, operator, true, true)
	if err != nil {
		return nil, "", false, err
	}
	return MeetingIntelligenceService.listTranscriptPage(item.TenantID, meeting.ID, cursor, limit)
}

func (s *ticketSupplierCollaborationService) IngestMeetingTranscript(ctx context.Context, collaborationID int64, meetingID string, input request.MeetingTranscriptIngestRequest, operator *dto.AuthPrincipal) (*models.MeetingTranscriptSegment, error) {
	_, meeting, account, err := s.requirePartnerMeetingAccess(ctx, collaborationID, meetingID, operator, false, false)
	if err != nil {
		return nil, err
	}
	return MeetingIntelligenceService.IngestClientTranscript(ctx, meeting, MeetingTranscriptSpeaker{
		ParticipantID: strconv.FormatInt(account.UserID, 10),
		Name:          firstNonEmptyString(strings.TrimSpace(account.DisplayName), "Supplier Engineer"),
		Language:      input.Language,
	}, input)
}

func (s *ticketSupplierCollaborationService) requirePartnerMeetingAccess(ctx context.Context, collaborationID int64, meetingID string, operator *dto.AuthPrincipal, allowTerminal bool, allowEnded bool) (*models.TicketSupplierCollaboration, *models.MeetingRoomJitsi, *models.PartnerAccount, error) {
	meetingID = strings.TrimSpace(meetingID)
	if meetingID == "" {
		return nil, nil, nil, errorsx.InvalidParam("meeting id is required")
	}
	var item *models.TicketSupplierCollaboration
	var err error
	if allowTerminal {
		item, err = s.requirePartnerCollaborationRead(collaborationID, operator)
	} else {
		item, err = s.requirePartnerCollaborationWork(collaborationID, operator)
	}
	if err != nil {
		return nil, nil, nil, err
	}
	if !supplierVisibilityAllows(item, "meeting") {
		return nil, nil, nil, errorsx.ForbiddenI18n("error.e0225")
	}
	var meeting models.MeetingRoomJitsi
	query := sqls.DB()
	if ctx != nil {
		query = query.WithContext(ctx)
	}
	if err := query.Where(
		"id = ? AND tenant_id = ? AND ticket_id = ?",
		meetingID,
		item.TenantID,
		strconv.FormatInt(item.TicketID, 10),
	).First(&meeting).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, errorsx.InvalidParam("meeting is not visible for supplier ticket")
		}
		return nil, nil, nil, err
	}
	if !allowEnded && isMeetingEndedStatus(meeting.Status) {
		return nil, nil, nil, meetingEndedError()
	}
	account, _, err := s.requirePartnerAccount(operator)
	if err != nil {
		return nil, nil, nil, errorsx.ForbiddenI18n("error.e0225")
	}
	return item, &meeting, account, nil
}

func (s *ticketSupplierCollaborationService) BackfillPartnerConversationMessages(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	var collaborations []models.TicketSupplierCollaboration
	if err := db.Where("record_status <> ?", enums.StatusDeleted).Order("id ASC").Find(&collaborations).Error; err != nil {
		return err
	}
	for i := range collaborations {
		collaboration := collaborations[i]
		if err := db.Transaction(func(tx *gorm.DB) error {
			ticket := repositories.TicketRepository.Get(tx, collaboration.TicketID)
			if ticket == nil || ticket.ConversationID <= 0 || ticket.TenantID != collaboration.TenantID {
				return nil
			}
			ctx := &sqls.TxContext{Tx: tx}
			participantUserIDs := make(map[int64]struct{})
			assignedAccount := repositories.TicketSupplierCollaborationRepository.GetPartnerAccountAnyStatus(tx, collaboration.TenantID, collaboration.PartnerCompanyID, collaboration.PartnerAccountID)
			if assignedAccount != nil {
				participantUserIDs[assignedAccount.UserID] = struct{}{}
				operator := s.partnerBackfillOperator(tx, assignedAccount)
				if err := s.ensurePartnerConversationParticipantTx(tx, ticket.ConversationID, assignedAccount.UserID, assignedAccount.ID, collaboration.InvitedAt, operator); err != nil {
					return err
				}
			}

			progresses := repositories.TicketProgressRepository.Find(tx, sqls.NewCnd().Eq("ticket_id", ticket.ID).Asc("id"))
			for j := range progresses {
				progress := &progresses[j]
				if progress.EventType != enums.TicketProgressEventProgress || partnerProgressCollaborationID(progress.MetadataJSON) != collaboration.ID || progress.AuthorID <= 0 {
					continue
				}
				authorAccount := repositories.TicketSupplierCollaborationRepository.FindPartnerAccountByUserAnyStatus(tx, progress.AuthorID)
				if authorAccount == nil || authorAccount.TenantID != collaboration.TenantID || authorAccount.PartnerCompanyID != collaboration.PartnerCompanyID {
					continue
				}
				participantUserIDs[authorAccount.UserID] = struct{}{}
				operator := s.partnerBackfillOperator(tx, authorAccount)
				if err := s.appendPartnerConversationMessageTx(
					ctx,
					ticket,
					&collaboration,
					progress,
					enums.IMMessageTypeText,
					progress.Content,
					"",
					operator,
					true,
				); err != nil {
					return err
				}
			}

			status := enums.NormalizeTicketStatus(string(ticket.Status))
			inactive := isSupplierCollaborationTerminal(collaboration.Status) || status == enums.TicketStatusClosed || status == enums.TicketStatusDone || status == enums.TicketStatusCancelled
			if !inactive {
				return nil
			}
			leftAt := ticket.UpdatedAt
			if collaboration.ResolvedAt != nil {
				leftAt = *collaboration.ResolvedAt
			}
			for userID := range participantUserIDs {
				operator := &dto.AuthPrincipal{UserID: userID, Username: "system", TenantID: collaboration.TenantID}
				if err := s.deactivatePartnerConversationParticipantTx(tx, ticket.ConversationID, userID, leftAt, operator); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *ticketSupplierCollaborationService) partnerBackfillOperator(db *gorm.DB, account *models.PartnerAccount) *dto.AuthPrincipal {
	operator := &dto.AuthPrincipal{TenantID: account.TenantID, PartnerAccountID: account.ID, UserID: account.UserID, DomainType: models.DomainTypePartner}
	if user := repositories.UserRepository.Get(db, account.UserID); user != nil {
		operator.Username = user.Username
		operator.Nickname = user.Nickname
	}
	if operator.Username == "" {
		operator.Username = account.DisplayName
	}
	return operator
}

func (s *ticketSupplierCollaborationService) requirePartnerTicket(ticketID int64, operator *dto.AuthPrincipal) (*models.TicketSupplierCollaboration, error) {
	account, _, err := s.requirePartnerAccount(operator)
	if err != nil {
		return nil, err
	}
	var items []models.TicketSupplierCollaboration
	if s.partnerHasRole(sqls.DB(), account, PartnerRoleAdmin) {
		items = s.findPartnerCompanyCollaborations(sqls.DB(), operator.TenantID, account.PartnerCompanyID)
	} else {
		items = s.findPartnerAccountCollaborations(sqls.DB(), operator.TenantID, account.ID)
	}
	for i := range items {
		if items[i].TicketID == ticketID {
			item, err := s.requirePartnerCollaborationRead(items[i].ID, operator)
			if err == nil {
				return item, nil
			}
		}
	}
	return nil, errorsx.ForbiddenI18n("error.e0225")
}

func (s *ticketSupplierCollaborationService) requirePartnerCollaborationRead(id int64, operator *dto.AuthPrincipal) (*models.TicketSupplierCollaboration, error) {
	account, company, err := s.requirePartnerAccount(operator)
	if err != nil {
		return nil, err
	}
	item := repositories.TicketSupplierCollaborationRepository.Get(sqls.DB(), id)
	if item == nil || item.TenantID != operator.TenantID || item.RecordStatus == enums.StatusDeleted {
		return nil, errorsx.ForbiddenI18n("error.e0225")
	}
	if item.PartnerCompanyID != company.ID {
		return nil, errorsx.ForbiddenI18n("error.e0225")
	}
	if !isSupplierCollaborationTerminal(item.Status) && item.AuthorizationEnds != nil && !item.AuthorizationEnds.After(time.Now()) {
		return nil, errorsx.ForbiddenI18n("error.e0225")
	}
	if item.PartnerAccountID != account.ID && !s.partnerHasRole(sqls.DB(), account, PartnerRoleAdmin) {
		participant := repositories.TicketSupplierCollaborationRepository.FindActiveParticipant(sqls.DB(), item.TenantID, item.ID, account.ID)
		if participant == nil && isSupplierCollaborationTerminal(item.Status) {
			participant = repositories.TicketSupplierCollaborationRepository.FindParticipantAnyStatus(sqls.DB(), item.TenantID, item.ID, account.ID)
		}
		if participant == nil {
			return nil, errorsx.ForbiddenI18n("error.e0225")
		}
	}
	return item, nil
}

func (s *ticketSupplierCollaborationService) requirePartnerCollaborationWork(id int64, operator *dto.AuthPrincipal) (*models.TicketSupplierCollaboration, error) {
	item, err := s.requirePartnerCollaborationRead(id, operator)
	if err != nil {
		return nil, err
	}
	if isSupplierCollaborationTerminal(item.Status) {
		return nil, errorsx.ForbiddenI18n("error.e0225")
	}
	if item.AuthorizationEnds != nil && !item.AuthorizationEnds.After(time.Now()) {
		return nil, errorsx.ForbiddenI18n("error.e0225")
	}
	account, _, err := s.requirePartnerAccount(operator)
	if err != nil {
		return nil, err
	}
	if item.PartnerAccountID == account.ID || s.partnerHasRole(sqls.DB(), account, PartnerRoleAdmin) {
		return item, nil
	}
	if repositories.TicketSupplierCollaborationRepository.FindActiveParticipant(sqls.DB(), item.TenantID, item.ID, account.ID) == nil {
		return nil, errorsx.ForbiddenI18n("error.e0225")
	}
	return item, nil
}

func (s *ticketSupplierCollaborationService) requirePartnerAccount(operator *dto.AuthPrincipal) (*models.PartnerAccount, *models.PartnerCompany, error) {
	if operator == nil || !operator.IsPartner() || operator.TenantID <= 0 || operator.PartnerAccountID <= 0 {
		return nil, nil, errorsx.ForbiddenI18n("error.e0225")
	}
	account := repositories.TicketSupplierCollaborationRepository.GetPartnerAccount(sqls.DB(), operator.TenantID, 0, operator.PartnerAccountID)
	if account == nil {
		return nil, nil, errorsx.ForbiddenI18n("error.e0225")
	}
	company := repositories.TicketSupplierCollaborationRepository.GetPartnerCompany(sqls.DB(), operator.TenantID, account.PartnerCompanyID)
	if company == nil {
		return nil, nil, errorsx.ForbiddenI18n("error.e0225")
	}
	return account, company, nil
}

func (s *ticketSupplierCollaborationService) buildAggregates(items []models.TicketSupplierCollaboration) []TicketSupplierCollaborationAggregate {
	result := make([]TicketSupplierCollaborationAggregate, 0, len(items))
	for i := range items {
		result = append(result, s.buildAggregate(&items[i]))
	}
	return result
}

func (s *ticketSupplierCollaborationService) buildAggregate(item *models.TicketSupplierCollaboration) TicketSupplierCollaborationAggregate {
	if item == nil {
		return TicketSupplierCollaborationAggregate{}
	}
	aggregate := TicketSupplierCollaborationAggregate{
		Collaboration: item,
		Ticket:        repositories.TicketRepository.Get(sqls.DB(), item.TicketID),
		Company:       repositories.TicketSupplierCollaborationRepository.GetPartnerCompany(sqls.DB(), item.TenantID, item.PartnerCompanyID),
		Account:       repositories.TicketSupplierCollaborationRepository.GetPartnerAccountAnyStatus(sqls.DB(), item.TenantID, item.PartnerCompanyID, item.PartnerAccountID),
		Participants:  make([]TicketSupplierParticipantAggregate, 0),
	}
	if item.ProductModuleID > 0 {
		aggregate.Module = repositories.ProductModuleRepository.Get(sqls.DB(), item.ProductModuleID)
	}
	for _, participant := range repositories.TicketSupplierCollaborationRepository.FindParticipants(sqls.DB(), item.TenantID, item.ID) {
		aggregate.Participants = append(aggregate.Participants, TicketSupplierParticipantAggregate{
			Participant: participant,
			Account: repositories.TicketSupplierCollaborationRepository.GetPartnerAccountAnyStatus(
				sqls.DB(), item.TenantID, item.PartnerCompanyID, participant.PartnerAccountID,
			),
		})
	}
	return aggregate
}

func participantRoleForAccount(item *models.TicketSupplierCollaboration, partnerAccountID int64) string {
	if item != nil && item.PartnerAccountID == partnerAccountID {
		return SupplierParticipantRoleOwner
	}
	return SupplierParticipantRoleMember
}

func normalizeSupplierVisibility(values []string) []string {
	allowed := map[string]bool{
		"ticket_summary":  true,
		"diagnosis":       true,
		"repair_progress": true,
		"meeting":         true,
	}
	if len(values) == 0 {
		return []string{"ticket_summary", "diagnosis", "repair_progress", "meeting"}
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]bool)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if allowed[value] && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	if len(result) == 0 {
		return []string{"ticket_summary"}
	}
	return result
}

func mergeSupplierVisibility(currentJSON string, requested []string) []string {
	current := make([]string, 0)
	if strings.TrimSpace(currentJSON) != "" {
		_ = json.Unmarshal([]byte(currentJSON), &current)
	}
	values := append([]string{}, current...)
	values = append(values, requested...)
	return normalizeSupplierVisibility(values)
}

func supplierVisibilityAllows(item *models.TicketSupplierCollaboration, capability string) bool {
	if item == nil {
		return false
	}
	values := make([]string, 0)
	if strings.TrimSpace(item.VisibilityJSON) == "" {
		// Older collaborations were created before visibility was persisted.
		// Keep their historical full-access behavior instead of rejecting an
		// otherwise valid supplier meeting as unauthorized.
		values = normalizeSupplierVisibility(nil)
	} else if err := json.Unmarshal([]byte(item.VisibilityJSON), &values); err != nil {
		return false
	} else {
		values = normalizeSupplierVisibility(values)
	}
	for _, value := range values {
		if value == capability {
			return true
		}
	}
	return false
}

func partnerProgressMetadata(collaborationID int64) string {
	data, _ := json.Marshal(map[string]any{"source": "partner", "collaboration_id": collaborationID})
	return string(data)
}

func partnerProgressCollaborationID(metadataJSON string) int64 {
	var metadata struct {
		CollaborationID int64 `json:"collaboration_id"`
	}
	if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
		return 0
	}
	return metadata.CollaborationID
}

func partnerMessageCollaborationID(payloadJSON string) int64 {
	var payload struct {
		CollaborationID int64 `json:"collaborationId"`
	}
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		return 0
	}
	return payload.CollaborationID
}

func (s *ticketSupplierCollaborationService) partnerRoleCodes(db *gorm.DB, account *models.PartnerAccount) ([]string, error) {
	if account == nil {
		return []string{}, nil
	}
	bindings, err := repositories.PlatformIAMRepository.FindRoleBindings(db, account.TenantID, models.DomainTypePartner, models.SubjectTypePartnerAccount, account.ID)
	if err != nil {
		return nil, err
	}
	roleIDs := make([]int64, 0, len(bindings))
	for _, binding := range bindings {
		roleIDs = append(roleIDs, binding.RoleID)
	}
	rolesByID, err := repositories.PlatformIAMRepository.FindAuthRolesByIDs(db, roleIDs)
	if err != nil {
		return nil, err
	}
	roles := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		if role := rolesByID[binding.RoleID]; role != nil && role.DomainType == models.DomainTypePartner {
			roles = append(roles, role.Code)
		}
	}
	if len(roles) > 0 {
		return roles, nil
	}
	accounts := repositories.TicketSupplierCollaborationRepository.FindPartnerAccountsByCompany(db, account.TenantID, account.PartnerCompanyID)
	firstActiveID := int64(0)
	for _, item := range accounts {
		if item.Status == enums.StatusOk && (firstActiveID == 0 || item.ID < firstActiveID) {
			firstActiveID = item.ID
		}
	}
	if account.ID == firstActiveID {
		return []string{PartnerRoleAdmin}, nil
	}
	return []string{PartnerRoleEngineer}, nil
}

func (s *ticketSupplierCollaborationService) partnerHasRole(db *gorm.DB, account *models.PartnerAccount, roleCode string) bool {
	roles, err := s.partnerRoleCodes(db, account)
	if err != nil {
		return false
	}
	for _, role := range roles {
		if role == roleCode {
			return true
		}
	}
	return false
}

func (s *ticketSupplierCollaborationService) replacePartnerRole(db *gorm.DB, account *models.PartnerAccount, roleCode string, operator *dto.AuthPrincipal, now time.Time) error {
	role := repositories.PlatformIAMRepository.FindAuthRoleByCode(db, account.TenantID, models.DomainTypePartner, roleCode)
	if role == nil {
		name := "Supplier Engineer"
		description := "Handles supplier collaboration tickets assigned by the supplier administrator"
		sortNo := 20
		if roleCode == PartnerRoleAdmin {
			name = "Supplier Administrator"
			description = "Manages supplier company accounts and internal ticket assignment"
			sortNo = 10
		}
		role = &models.AuthRole{
			TenantID:    account.TenantID,
			DomainType:  models.DomainTypePartner,
			Code:        roleCode,
			Name:        name,
			Description: description,
			IsBuiltin:   true,
			Status:      enums.StatusOk,
			SortNo:      sortNo,
			AuditFields: utils.BuildAuditFields(operator),
		}
		if err := repositories.PlatformIAMRepository.CreateAuthRole(db, role); err != nil {
			return err
		}
	}
	return repositories.PlatformIAMRepository.ReplaceRoleBinding(db, &models.AuthRoleBinding{
		TenantID:    account.TenantID,
		DomainType:  models.DomainTypePartner,
		RoleID:      role.ID,
		SubjectType: models.SubjectTypePartnerAccount,
		SubjectID:   account.ID,
		Status:      enums.StatusOk,
		EffectiveAt: &now,
		AuditFields: utils.BuildAuditFields(operator),
	})
}

func (s *ticketSupplierCollaborationService) countActivePartnerAdmins(companyID, tenantID int64) int {
	count := 0
	accounts := repositories.TicketSupplierCollaborationRepository.FindPartnerAccountsByCompany(sqls.DB(), tenantID, companyID)
	for i := range accounts {
		account := accounts[i]
		if account.Status == enums.StatusOk && s.partnerHasRole(sqls.DB(), &account, PartnerRoleAdmin) {
			count++
		}
	}
	return count
}

func normalizePartnerRoleCode(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return PartnerRoleEngineer, nil
	}
	if value != PartnerRoleAdmin && value != PartnerRoleEngineer {
		return "", errorsx.InvalidParam("partner role is invalid")
	}
	return value, nil
}

func normalizePartnerLanguages(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]bool)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func partnerNullableString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
