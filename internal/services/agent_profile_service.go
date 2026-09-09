package services

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/mlogclub/simple/sqls"
)

var AgentProfileService = newAgentProfileService()

func newAgentProfileService() *agentProfileService {
	return &agentProfileService{}
}

type agentProfileService struct {
}

type DispatchCandidateQuery struct {
	TenantID       int64
	ProductID      int64
	TeamID         int64
	TicketID       int64
	ConversationID int64
	ManualTransfer bool
}

type DispatchCandidateProfile struct {
	Profile                 models.AgentProfile
	TeamID                  int64
	AssignmentMode          string
	DispatchWeight          int
	TeamDispatchEnabled     bool
	TeamEligible            bool
	WorkStatusConfirmed     bool
	Reachable               bool
	CapacityAvailable       bool
	ActiveConversationCount int
	OpenTicketCount         int
	Workload                int
	LoadRate                float64
}

func (s *agentProfileService) Get(id int64) *models.AgentProfile {
	return repositories.AgentProfileRepository.Get(sqls.DB(), id)
}

func (s *agentProfileService) GetForTenant(id, tenantID int64) *models.AgentProfile {
	item := s.Get(id)
	if item == nil || (tenantID > 0 && item.TenantID != tenantID) {
		return nil
	}
	return item
}

func (s *agentProfileService) Take(where ...interface{}) *models.AgentProfile {
	return repositories.AgentProfileRepository.Take(sqls.DB(), where...)
}

func (s *agentProfileService) Find(cnd *sqls.Cnd) []models.AgentProfile {
	return repositories.AgentProfileRepository.Find(sqls.DB(), cnd)
}

func (s *agentProfileService) FindOne(cnd *sqls.Cnd) *models.AgentProfile {
	return repositories.AgentProfileRepository.FindOne(sqls.DB(), cnd)
}

func (s *agentProfileService) FindPageByParams(params *params.QueryParams) (list []models.AgentProfile, paging *sqls.Paging) {
	return repositories.AgentProfileRepository.FindPageByParams(sqls.DB(), params)
}

func (s *agentProfileService) FindPageByCnd(cnd *sqls.Cnd) (list []models.AgentProfile, paging *sqls.Paging) {
	return repositories.AgentProfileRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *agentProfileService) Count(cnd *sqls.Cnd) int64 {
	return repositories.AgentProfileRepository.Count(sqls.DB(), cnd)
}

func (s *agentProfileService) GetByUserID(userID int64) *models.AgentProfile {
	if userID <= 0 {
		return nil
	}
	return repositories.AgentProfileRepository.FindOne(sqls.DB(), sqls.NewCnd().Eq("user_id", userID))
}

func (s *agentProfileService) CurrentDisplayIdentity(tenantID, userID int64) (string, string) {
	if userID <= 0 {
		return "", ""
	}
	db := sqls.DB()
	displayName := ""
	avatar := ""
	if tenantID > 0 && db.Migrator().HasTable(&models.TenantMember{}) {
		if member := repositories.EnterpriseIAMRepository.FindTenantMemberByUserID(db, tenantID, userID); member != nil {
			displayName = strings.TrimSpace(member.DisplayName)
		}
	}
	if db.Migrator().HasTable(&models.User{}) {
		if user := UserService.Get(userID); user != nil {
			displayName = firstNonEmptyString(displayName, user.Nickname, user.Username)
			avatar = strings.TrimSpace(user.Avatar)
		}
	}
	if db.Migrator().HasTable(&models.AgentProfile{}) {
		if profile := s.GetByUserID(userID); profile != nil && (tenantID == 0 || profile.TenantID == tenantID) {
			if avatar == "" {
				avatar = strings.TrimSpace(profile.Avatar)
			}
			displayName = firstNonEmptyString(displayName, strings.TrimSpace(profile.AgentCode))
		}
	}
	if displayName == "" {
		displayName = fmt.Sprintf("成员 #%d", userID)
	}
	return displayName, avatar
}

func (s *agentProfileService) GetUserIDsByTeamID(teamID int64) []int64 {
	if teamID <= 0 {
		return nil
	}
	tenantID := int64(0)
	if team := AgentTeamService.Get(teamID); team != nil {
		tenantID = team.TenantID
	}
	list := AgentTeamMemberService.FindProfilesByTeamID(sqls.DB(), tenantID, teamID)
	if len(list) == 0 {
		return nil
	}
	result := make([]int64, 0, len(list))
	for _, item := range list {
		if item.UserID > 0 {
			result = append(result, item.UserID)
		}
	}
	return result
}

func (s *agentProfileService) CreateAgentProfile(req request.CreateAgentProfileRequest, operator *dto.AuthPrincipal) (*models.AgentProfile, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item, err := s.buildProfileModel(0, operator.EffectiveTenantID(), req)
	if err != nil {
		return nil, err
	}
	item.AuditFields = utils.BuildAuditFields(operator)
	if err := repositories.AgentProfileRepository.Create(sqls.DB(), item); err != nil {
		return nil, err
	}
	if err := s.ensurePrimaryTeamMembership(item, operator); err != nil {
		return nil, err
	}
	s.syncTenantMemberDepartment(item)
	s.dispatchPendingWorkIfEligible(item)
	return item, nil
}

func (s *agentProfileService) FindDispatchCandidates(query DispatchCandidateQuery, now time.Time) ([]DispatchCandidateProfile, error) {
	tenantID, productID, teamIDs, err := s.resolveDispatchCandidateScope(query)
	if err != nil {
		return nil, err
	}
	if len(teamIDs) == 0 {
		return []DispatchCandidateProfile{}, nil
	}
	if query.ManualTransfer && query.ConversationID > 0 {
		return s.findManualConversationTransferCandidates(query.ConversationID, tenantID, teamIDs, now)
	}
	candidates, _, err := ConversationDispatchService.pickDispatchCandidates(tenantID, productID, teamIDs, now)
	if err != nil {
		return nil, err
	}
	if query.TicketID > 0 && len(candidates) > 0 {
		ticket := repositories.TicketRepository.Get(sqls.DB(), query.TicketID)
		candidates, _, err = filterDispatchCandidatesByTicketCapabilityDB(sqls.DB(), ticket, candidates)
		if err != nil {
			return nil, err
		}
	}
	ret := make([]DispatchCandidateProfile, 0, len(candidates))
	for _, candidate := range candidates {
		ret = append(ret, DispatchCandidateProfile{
			Profile:                 candidate.profile,
			TeamID:                  candidate.teamID,
			AssignmentMode:          candidate.assignmentMode,
			DispatchWeight:          candidate.dispatchWeight,
			TeamDispatchEnabled:     candidate.teamDispatchEnabled,
			TeamEligible:            true,
			WorkStatusConfirmed:     true,
			Reachable:               isDispatchReachable(&candidate.profile, now),
			CapacityAvailable:       candidate.profile.MaxConcurrentCount > 0 && candidate.workload < candidate.profile.MaxConcurrentCount,
			ActiveConversationCount: candidate.activeCount,
			OpenTicketCount:         candidate.openTicketCount,
			Workload:                candidate.workload,
			LoadRate:                candidate.loadRate,
		})
	}
	return ret, nil
}

func (s *agentProfileService) findManualConversationTransferCandidates(conversationID, tenantID int64, teamIDs []int64, now time.Time) ([]DispatchCandidateProfile, error) {
	conversation := ConversationService.Get(conversationID)
	if conversation == nil || conversation.Status == enums.IMConversationStatusClosed {
		return nil, errorsx.InvalidParam("conversation not found")
	}
	if tenantID > 0 && conversation.TenantID != tenantID {
		return nil, errorsx.Forbidden("conversation is outside the current tenant")
	}

	db := sqls.DB()
	teamProfiles := AgentTeamMemberService.FindDispatchProfilesByTeamIDs(db, tenantID, teamIDs)
	if len(teamProfiles) == 0 {
		return []DispatchCandidateProfile{}, nil
	}

	userIDs := make([]int64, 0, len(teamProfiles))
	for _, teamProfile := range teamProfiles {
		if teamProfile.Profile.UserID > 0 && teamProfile.Profile.UserID != conversation.CurrentAssigneeID {
			userIDs = append(userIDs, teamProfile.Profile.UserID)
		}
	}
	if len(userIDs) == 0 {
		return []DispatchCandidateProfile{}, nil
	}

	enabledUsers := UserService.Find(sqls.NewCnd().
		In("id", userIDs).
		Eq("status", enums.StatusOk))
	enabledUserSet := make(map[int64]struct{}, len(enabledUsers))
	for _, user := range enabledUsers {
		enabledUserSet[user.ID] = struct{}{}
	}
	activeCounts, err := ConversationDispatchService.findActiveConversationCountMap(userIDs)
	if err != nil {
		return nil, err
	}
	openTicketCounts, err := ConversationDispatchService.findOpenTicketCountMap(userIDs)
	if err != nil {
		return nil, err
	}

	candidates := make([]dispatchCandidate, 0, len(teamProfiles))
	for _, teamProfile := range teamProfiles {
		profile := teamProfile.Profile
		if profile.UserID == conversation.CurrentAssigneeID {
			continue
		}
		if _, ok := enabledUserSet[profile.UserID]; !ok {
			continue
		}
		activeCount := activeCounts[profile.UserID]
		openTicketCount := openTicketCounts[profile.UserID]
		workload := activeCount + openTicketCount
		loadRate := float64(0)
		if profile.MaxConcurrentCount > 0 {
			loadRate = float64(workload) / float64(profile.MaxConcurrentCount)
		}
		candidates = append(candidates, dispatchCandidate{
			profile:             profile,
			teamID:              teamProfile.TeamID,
			assignmentMode:      teamProfile.AssignmentMode,
			dispatchWeight:      teamProfile.DispatchWeight,
			teamDispatchEnabled: teamProfile.DispatchEnabled,
			activeCount:         activeCount,
			openTicketCount:     openTicketCount,
			workload:            workload,
			loadRate:            loadRate,
			score:               float64(workload),
		})
	}

	ret := make([]DispatchCandidateProfile, 0, len(candidates))
	for _, candidate := range candidates {
		capacityAvailable := candidate.profile.MaxConcurrentCount <= 0 || candidate.workload < candidate.profile.MaxConcurrentCount
		ret = append(ret, DispatchCandidateProfile{
			Profile:                 candidate.profile,
			TeamID:                  candidate.teamID,
			AssignmentMode:          candidate.assignmentMode,
			DispatchWeight:          candidate.dispatchWeight,
			TeamDispatchEnabled:     candidate.teamDispatchEnabled,
			TeamEligible:            true,
			WorkStatusConfirmed:     true,
			Reachable:               isDispatchReachable(&candidate.profile, now),
			CapacityAvailable:       capacityAvailable,
			ActiveConversationCount: candidate.activeCount,
			OpenTicketCount:         candidate.openTicketCount,
			Workload:                candidate.workload,
			LoadRate:                candidate.loadRate,
		})
	}
	return ret, nil
}

func (s *agentProfileService) resolveDispatchCandidateScope(query DispatchCandidateQuery) (int64, int64, []int64, error) {
	switch {
	case query.TicketID > 0:
		return s.resolveDispatchCandidateScopeForTicket(query)
	case query.ConversationID > 0:
		return s.resolveDispatchCandidateScopeForConversation(query)
	case query.TeamID > 0:
		return s.resolveDispatchCandidateScopeForTeam(query)
	default:
		return 0, 0, nil, errorsx.InvalidParam("dispatch candidate scope is missing")
	}
}

func (s *agentProfileService) resolveDispatchCandidateScopeForTicket(query DispatchCandidateQuery) (int64, int64, []int64, error) {
	ticket := repositories.TicketRepository.Get(sqls.DB(), query.TicketID)
	if ticket == nil {
		return 0, 0, nil, errorsx.InvalidParam("ticket not found")
	}
	if query.TenantID > 0 && ticket.TenantID != query.TenantID {
		return 0, 0, nil, errorsx.Forbidden("ticket is outside the current tenant")
	}
	teamID := resolveTicketDispatchTeamIDDB(sqls.DB(), ticket)
	if teamID <= 0 {
		return ticket.TenantID, ticket.ProductID, []int64{}, nil
	}
	return ticket.TenantID, ticket.ProductID, []int64{teamID}, nil
}

func (s *agentProfileService) resolveDispatchCandidateScopeForConversation(query DispatchCandidateQuery) (int64, int64, []int64, error) {
	conversation := ConversationService.Get(query.ConversationID)
	if conversation == nil || conversation.Status == enums.IMConversationStatusClosed {
		return 0, 0, nil, errorsx.InvalidParam("conversation not found")
	}
	if query.TenantID > 0 && conversation.TenantID != query.TenantID {
		return 0, 0, nil, errorsx.Forbidden("conversation is outside the current tenant")
	}
	if query.ManualTransfer && conversation.ProductID > 0 {
		if team := ProductSupportOrganizationService.FindProductRepairTeam(sqls.DB(), conversation.TenantID, conversation.ProductID); team != nil {
			return conversation.TenantID, conversation.ProductID, []int64{team.ID}, nil
		}
		return conversation.TenantID, conversation.ProductID, []int64{}, nil
	}
	if conversation.CurrentTeamID > 0 {
		team := repositories.AgentTeamRepository.Get(sqls.DB(), conversation.CurrentTeamID)
		if team != nil && team.TenantID == conversation.TenantID && team.Status == enums.StatusOk &&
			(conversation.ProductID <= 0 || team.ProductID <= 0 || team.ProductID == conversation.ProductID) {
			return conversation.TenantID, conversation.ProductID, []int64{conversation.CurrentTeamID}, nil
		}
	}
	if conversation.AIAgentID > 0 {
		if aiAgent := AIAgentService.Get(conversation.AIAgentID); aiAgent != nil && aiAgent.Status == enums.StatusOk {
			tenantID, productID, scopeOK := resolveConversationDispatchScope(conversation, aiAgent)
			teamIDs := orderedPositiveIDs(aiAgent.TeamIDs)
			if scopeOK && len(teamIDs) > 0 {
				return tenantID, productID, teamIDs, nil
			}
		}
	}
	if conversation.ProductID > 0 {
		if team := ProductSupportOrganizationService.FindProductRepairTeam(sqls.DB(), conversation.TenantID, conversation.ProductID); team != nil {
			return conversation.TenantID, conversation.ProductID, []int64{team.ID}, nil
		}
	}
	return conversation.TenantID, conversation.ProductID, []int64{}, nil
}

func (s *agentProfileService) resolveDispatchCandidateScopeForTeam(query DispatchCandidateQuery) (int64, int64, []int64, error) {
	team := repositories.AgentTeamRepository.Get(sqls.DB(), query.TeamID)
	if team == nil || team.Status != enums.StatusOk {
		return 0, 0, nil, errorsx.InvalidParam("agent team is not active")
	}
	if query.TenantID > 0 && team.TenantID != query.TenantID {
		return 0, 0, nil, errorsx.Forbidden("agent team is outside the current tenant")
	}
	productID := query.ProductID
	if productID <= 0 {
		productID = team.ProductID
	}
	return team.TenantID, productID, []int64{team.ID}, nil
}

func (s *agentProfileService) UpdateAgentProfile(req request.UpdateAgentProfileRequest, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	current := s.GetForTenant(req.ID, operator.EffectiveTenantID())
	if current == nil {
		return errorsx.InvalidParamI18n("error.e0164")
	}
	item, err := s.buildProfileModel(req.ID, current.TenantID, req.CreateAgentProfileRequest)
	if err != nil {
		return err
	}
	now := time.Now()
	if err := repositories.AgentProfileRepository.Updates(sqls.DB(), req.ID, map[string]any{
		"tenant_id":               item.TenantID,
		"user_id":                 item.UserID,
		"team_id":                 item.TeamID,
		"agent_code":              item.AgentCode,
		"display_name":            item.DisplayName,
		"avatar":                  item.Avatar,
		"service_status":          item.ServiceStatus,
		"max_concurrent_count":    item.MaxConcurrentCount,
		"priority_level":          item.PriorityLevel,
		"auto_assign_enabled":     item.AutoAssignEnabled,
		"receive_offline_message": item.ReceiveOfflineMessage,
		"remark":                  item.Remark,
		"update_user_id":          operator.UserID,
		"update_user_name":        operator.Username,
		"updated_at":              now,
	}); err != nil {
		return err
	}
	if err := s.ensurePrimaryTeamMembership(item, operator); err != nil {
		return err
	}
	s.syncTenantMemberDepartment(item)
	if current.UserID != item.UserID {
		s.recoverPendingAssignmentsForUser(current.TenantID, current.UserID, "profile_user_changed", now)
	}
	if updated := s.Get(req.ID); updated != nil {
		s.handleDispatchEligibilityChanged(updated, now)
	}
	return nil
}

func (s *agentProfileService) DeleteAgentProfile(id int64, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	current := s.GetForTenant(id, operator.EffectiveTenantID())
	if current == nil {
		return errorsx.InvalidParamI18n("error.e0164")
	}
	repositories.AgentProfileRepository.Delete(sqls.DB(), id)
	s.recoverPendingAssignmentsForUser(current.TenantID, current.UserID, "profile_deleted", time.Now())
	return nil
}

func (s *agentProfileService) buildProfileModel(id, tenantID int64, req request.CreateAgentProfileRequest) (*models.AgentProfile, error) {
	if req.UserID <= 0 {
		return nil, errorsx.InvalidParamI18n("error.e0325")
	}
	if UserService.Get(req.UserID) == nil {
		return nil, errorsx.InvalidParamI18n("error.e0127")
	}
	if tenantID > 0 && sqls.DB().Migrator().HasTable(&models.TenantMember{}) {
		member := repositories.EnterpriseIAMRepository.FindTenantMemberByUserID(sqls.DB(), tenantID, req.UserID)
		if member == nil || member.Status != enums.StatusOk {
			return nil, errorsx.InvalidParam("agent must be an active member of the tenant")
		}
	}
	if req.TeamID <= 0 {
		return nil, errorsx.InvalidParamI18n("error.e0328")
	}
	team := AgentTeamService.GetForTenant(req.TeamID, tenantID)
	if team == nil {
		return nil, errorsx.InvalidParamI18n("error.e0205")
	}
	if team.Status != enums.StatusOk {
		return nil, errorsx.InvalidParam("agent team is not active")
	}
	req.AgentCode = strings.TrimSpace(req.AgentCode)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if req.AgentCode == "" || req.DisplayName == "" {
		return nil, errorsx.InvalidParamI18n("error.e0162")
	}
	if exists := s.Take("user_id = ? AND id <> ?", req.UserID, id); exists != nil {
		return nil, errorsx.InvalidParamI18n("error.e0314")
	}
	if exists := s.Take("agent_code = ? AND id <> ?", req.AgentCode, id); exists != nil {
		return nil, errorsx.InvalidParamI18n("error.e0163")
	}
	if !enums.IsValidServiceStatus(req.ServiceStatus) {
		return nil, errorsx.InvalidParamI18n("error.e0165")
	}
	if req.MaxConcurrentCount < 0 {
		return nil, errorsx.InvalidParamI18n("error.e0229")
	}
	return &models.AgentProfile{
		TenantID:              tenantID,
		UserID:                req.UserID,
		TeamID:                req.TeamID,
		AgentCode:             req.AgentCode,
		DisplayName:           req.DisplayName,
		Avatar:                strings.TrimSpace(req.Avatar),
		ServiceStatus:         req.ServiceStatus,
		MaxConcurrentCount:    req.MaxConcurrentCount,
		PriorityLevel:         req.PriorityLevel,
		AutoAssignEnabled:     req.AutoAssignEnabled,
		ReceiveOfflineMessage: req.ReceiveOfflineMessage,
		Remark:                strings.TrimSpace(req.Remark),
	}, nil
}

func (s *agentProfileService) syncTenantMemberDepartment(item *models.AgentProfile) {
	if item == nil || item.TenantID <= 0 || item.UserID <= 0 || item.TeamID <= 0 {
		return
	}
	team := AgentTeamService.GetForTenant(item.TeamID, item.TenantID)
	if team == nil || team.DepartmentID <= 0 {
		return
	}
	_ = repositories.EnterpriseIAMRepository.UpdateTenantMemberDepartmentByUserID(sqls.DB(), item.TenantID, item.UserID, team.DepartmentID)
}

func (s *agentProfileService) ensurePrimaryTeamMembership(item *models.AgentProfile, operator *dto.AuthPrincipal) error {
	if item == nil || item.TenantID <= 0 || item.TeamID <= 0 || item.UserID <= 0 {
		return nil
	}
	db := sqls.DB()
	memberID := int64(0)
	if member := repositories.EnterpriseIAMRepository.FindTenantMemberByUserID(db, item.TenantID, item.UserID); member != nil {
		memberID = member.ID
	}
	_, err := AgentTeamMemberService.EnsureMemberPreservingDispatchDB(db, item.TenantID, item.TeamID, item.UserID, memberID, defaultAgentTeamMemberWeight, item.AutoAssignEnabled, operator)
	return err
}

func (s *agentProfileService) dispatchPendingWorkIfEligible(item *models.AgentProfile) {
	if item == nil {
		return
	}
	if item.Status != enums.StatusOk {
		return
	}
	if !item.AutoAssignEnabled || item.MaxConcurrentCount <= 0 {
		return
	}
	if item.ServiceStatus != enums.ServiceStatusIdle {
		return
	}
	_, _ = ConversationDispatchService.DispatchPendingConversations(0)
	_, _ = TicketDispatchService.DispatchPendingTickets(0)
}

func (s *agentProfileService) handleDispatchEligibilityChanged(item *models.AgentProfile, now time.Time) {
	if item == nil || item.TenantID <= 0 || item.UserID <= 0 {
		return
	}
	if s.canProfileKeepPendingAssignments(item, now) {
		s.dispatchPendingWorkIfEligible(item)
		return
	}
	s.recoverPendingAssignmentsForUser(item.TenantID, item.UserID, "profile_dispatch_disabled", now)
}

func (s *agentProfileService) canProfileKeepPendingAssignments(item *models.AgentProfile, now time.Time) bool {
	if item == nil || item.Status != enums.StatusOk || !item.AutoAssignEnabled || item.MaxConcurrentCount <= 0 || item.ServiceStatus != enums.ServiceStatusIdle {
		return false
	}
	db := sqls.DB()
	if db != nil && !AgentScheduleExceptionService.IsUserAvailableDB(db, item.TenantID, item.UserID, now) {
		return false
	}
	if db != nil && db.Migrator().HasTable(&models.AgentProfile{}) && !isDispatchReachable(item, now) {
		return false
	}
	return true
}

func (s *agentProfileService) recoverPendingAssignmentsForUser(tenantID, userID int64, reason string, now time.Time) {
	recovered, err := TicketDispatchService.RecoverUnavailableAssigneeAssignments(tenantID, userID, now)
	if err != nil {
		slog.Warn("recover pending ticket assignments after engineer profile changed failed",
			"tenant_id", tenantID,
			"user_id", userID,
			"reason", reason,
			"recovered", recovered,
			"error", err,
		)
	}
	if recovered, err := ConversationDispatchService.RecoverUnavailableAssigneeAssignments(tenantID, userID, now); err != nil {
		slog.Warn("recover pending conversation assignments after engineer profile changed failed",
			"tenant_id", tenantID,
			"user_id", userID,
			"reason", reason,
			"recovered", recovered,
			"error", err,
		)
	}
}
