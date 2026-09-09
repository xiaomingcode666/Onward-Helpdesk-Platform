package services

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	AgentTeamAssignmentModeBalanced = "balanced"
	AgentTeamAssignmentModeWeighted = "weighted"

	defaultAgentTeamMemberWeight = 1
	maxAgentTeamMemberWeight     = 100
)

var AgentTeamMemberService = newAgentTeamMemberService()

func newAgentTeamMemberService() *agentTeamMemberService {
	return &agentTeamMemberService{}
}

type agentTeamMemberService struct{}

type AgentTeamDispatchProfile struct {
	Profile         models.AgentProfile
	TeamID          int64
	AssignmentMode  string
	DispatchWeight  int
	DispatchEnabled bool
}

type AgentTeamMemberRemovalResult struct {
	TeamID                        int64
	UserID                        int64
	PendingTicketsRecovered       int
	PendingConversationsRecovered int
	AlreadyRemoved                bool
}

func NormalizeAgentTeamAssignmentMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case AgentTeamAssignmentModeWeighted:
		return AgentTeamAssignmentModeWeighted
	default:
		return AgentTeamAssignmentModeBalanced
	}
}

func normalizeAgentTeamMemberWeight(value int) int {
	if value <= 0 {
		return defaultAgentTeamMemberWeight
	}
	if value > maxAgentTeamMemberWeight {
		return maxAgentTeamMemberWeight
	}
	return value
}

func (s *agentTeamMemberService) tableReady(db *gorm.DB) bool {
	return db != nil && db.Migrator().HasTable(&models.AgentTeamMember{})
}

func (s *agentTeamMemberService) EnsureMemberDB(
	db *gorm.DB,
	tenantID int64,
	teamID int64,
	userID int64,
	memberID int64,
	dispatchWeight int,
	dispatchEnabled bool,
	operator *dto.AuthPrincipal,
) (*models.AgentTeamMember, error) {
	if tenantID <= 0 || teamID <= 0 || userID <= 0 {
		return nil, errorsx.InvalidParam("tenant, team and user are required")
	}
	if !s.tableReady(db) {
		return nil, nil
	}
	team := repositories.AgentTeamRepository.Get(db, teamID)
	if team == nil || team.TenantID != tenantID || team.Status != enums.StatusOk {
		return nil, errorsx.InvalidParam("agent team not found")
	}
	user := repositories.UserRepository.Get(db, userID)
	if user == nil || user.Status != enums.StatusOk {
		return nil, errorsx.InvalidParamI18n("error.e0334")
	}
	if memberID <= 0 {
		if member := repositories.EnterpriseIAMRepository.FindTenantMemberByUserID(db, tenantID, userID); member != nil {
			if member.Status != enums.StatusOk {
				return nil, errorsx.InvalidParam("member is not active")
			}
			memberID = member.ID
		}
	}
	if err := s.ensureSupportProfileForMemberDB(db, tenantID, teamID, userID, memberID, dispatchEnabled, operator); err != nil {
		return nil, err
	}
	now := time.Now()
	existing := repositories.AgentTeamMemberRepository.FindOne(db, sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("team_id", teamID).
		Eq("user_id", userID))
	if existing == nil {
		item := &models.AgentTeamMember{
			TenantID:        tenantID,
			TeamID:          teamID,
			UserID:          userID,
			MemberID:        memberID,
			DispatchEnabled: dispatchEnabled,
			DispatchWeight:  normalizeAgentTeamMemberWeight(dispatchWeight),
			Status:          enums.StatusOk,
			AuditFields:     utils.BuildAuditFields(operator),
		}
		if err := repositories.AgentTeamMemberRepository.Create(db, item); err != nil {
			return nil, err
		}
		return item, nil
	}
	if err := repositories.AgentTeamMemberRepository.Updates(db, existing.ID, map[string]any{
		"member_id":        memberID,
		"dispatch_enabled": dispatchEnabled,
		"dispatch_weight":  normalizeAgentTeamMemberWeight(dispatchWeight),
		"status":           enums.StatusOk,
		"update_user_id":   auditOperatorID(operator),
		"update_user_name": auditOperatorName(operator),
		"updated_at":       now,
	}); err != nil {
		return nil, err
	}
	return repositories.AgentTeamMemberRepository.Get(db, existing.ID), nil
}

func (s *agentTeamMemberService) EnsureMemberPreservingDispatchDB(
	db *gorm.DB,
	tenantID int64,
	teamID int64,
	userID int64,
	memberID int64,
	defaultDispatchWeight int,
	defaultDispatchEnabled bool,
	operator *dto.AuthPrincipal,
) (*models.AgentTeamMember, error) {
	dispatchWeight := defaultDispatchWeight
	dispatchEnabled := defaultDispatchEnabled
	if s.tableReady(db) {
		if existing := repositories.AgentTeamMemberRepository.FindOne(db, sqls.NewCnd().
			Eq("tenant_id", tenantID).
			Eq("team_id", teamID).
			Eq("user_id", userID)); existing != nil {
			dispatchWeight = existing.DispatchWeight
			dispatchEnabled = existing.DispatchEnabled
		}
	}
	return s.EnsureMemberDB(db, tenantID, teamID, userID, memberID, dispatchWeight, dispatchEnabled, operator)
}

func (s *agentTeamMemberService) EnsureMember(
	tenantID int64,
	teamID int64,
	userID int64,
	memberID int64,
	dispatchWeight int,
	dispatchEnabled bool,
	operator *dto.AuthPrincipal,
) (*models.AgentTeamMember, error) {
	now := time.Now()
	var item *models.AgentTeamMember
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		var ensureErr error
		item, ensureErr = s.EnsureMemberDB(ctx.Tx, tenantID, teamID, userID, memberID, dispatchWeight, dispatchEnabled, operator)
		if ensureErr != nil || item == nil {
			return ensureErr
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if dispatchEnabled {
		_, _ = ConversationDispatchService.DispatchPendingConversations(0)
		_, _ = TicketDispatchService.DispatchPendingTickets(0)
		return item, nil
	}
	if recovered, recoverErr := TicketDispatchService.RecoverTeamMemberIneligibleAssignments(tenantID, teamID, userID, now); recoverErr != nil {
		slog.Warn("recover pending ticket assignments after team member dispatch disabled failed",
			"tenant_id", tenantID,
			"team_id", teamID,
			"user_id", userID,
			"recovered", recovered,
			"error", recoverErr,
		)
	}
	if recovered, recoverErr := ConversationDispatchService.RecoverTeamMemberIneligibleAssignments(tenantID, teamID, userID, now); recoverErr != nil {
		slog.Warn("recover pending conversation assignments after team member dispatch disabled failed",
			"tenant_id", tenantID,
			"team_id", teamID,
			"user_id", userID,
			"recovered", recovered,
			"error", recoverErr,
		)
	}
	return item, nil
}

func (s *agentTeamMemberService) RemoveMember(
	tenantID int64,
	teamID int64,
	userID int64,
	operator *dto.AuthPrincipal,
) (*AgentTeamMemberRemovalResult, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if tenantID <= 0 || teamID <= 0 || userID <= 0 {
		return nil, errorsx.InvalidParam("tenant, team and user are required")
	}
	result := &AgentTeamMemberRemovalResult{TeamID: teamID, UserID: userID}
	now := time.Now()
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if !s.tableReady(ctx.Tx) {
			return errorsx.InvalidParam("agent team membership is unavailable")
		}
		team := repositories.AgentTeamRepository.Get(ctx.Tx, teamID)
		if team == nil || team.TenantID != tenantID {
			return errorsx.InvalidParam("agent team not found")
		}
		membership := repositories.AgentTeamMemberRepository.FindOne(ctx.Tx, sqls.NewCnd().
			Eq("tenant_id", tenantID).
			Eq("team_id", teamID).
			Eq("user_id", userID))
		if membership == nil || membership.Status != enums.StatusOk {
			result.AlreadyRemoved = true
			return nil
		}
		if team.LeaderUserID == userID {
			return errorsx.InvalidParam("该成员是当前产品组主管，请先更换主管再移出")
		}
		var profile models.AgentProfile
		profileQuery := ctx.Tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, enums.StatusOk).
			Find(&profile)
		if profileQuery.Error != nil {
			return profileQuery.Error
		}
		activeTickets, activeConversations, countErr := s.countActiveAssignedWorkDB(ctx.Tx, tenantID, teamID, userID)
		if countErr != nil {
			return countErr
		}
		if activeTickets > 0 || activeConversations > 0 {
			return errorsx.InvalidParam(fmt.Sprintf("该成员仍有 %d 张已受理工单和 %d 个处理中会话，请先转派或结案", activeTickets, activeConversations))
		}
		if err := repositories.AgentTeamMemberRepository.Updates(ctx.Tx, membership.ID, map[string]any{
			"dispatch_enabled": false,
			"status":           enums.StatusDeleted,
			"update_user_id":   auditOperatorID(operator),
			"update_user_name": auditOperatorName(operator),
			"updated_at":       now,
		}); err != nil {
			return err
		}
		if profile.ID > 0 && profile.TeamID == teamID {
			nextTeamID := int64(0)
			if memberships := repositories.AgentTeamMemberRepository.Find(ctx.Tx, sqls.NewCnd().
				Eq("tenant_id", tenantID).
				Eq("user_id", userID).
				Eq("status", enums.StatusOk).
				Asc("team_id")); len(memberships) > 0 {
				nextTeamID = memberships[0].TeamID
			}
			if err := repositories.AgentProfileRepository.Updates(ctx.Tx, profile.ID, map[string]any{
				"team_id":          nextTeamID,
				"update_user_id":   auditOperatorID(operator),
				"update_user_name": auditOperatorName(operator),
				"updated_at":       now,
			}); err != nil {
				return err
			}
		}
		if err := AuditService.RecordAuditTx(ctx, RecordAuditInput{
			TenantID:     tenantID,
			ActorID:      fmt.Sprint(operator.UserID),
			ActorType:    "user",
			Domain:       "settings",
			ResourceType: "agent_team_member",
			ResourceID:   fmt.Sprint(membership.ID),
			Action:       "agent_team_member.removed",
			BeforeState:  membership,
			AfterState: map[string]any{
				"teamId": teamID,
				"userId": userID,
				"status": enums.StatusDeleted,
			},
			SupportGrantID: operator.SupportGrantID,
			RiskLevel:      models.RiskLevelMedium,
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	var recoveryErrors []error
	result.PendingTicketsRecovered, err = TicketDispatchService.RecoverTeamMemberIneligibleAssignments(tenantID, teamID, userID, now)
	if err != nil {
		recoveryErrors = append(recoveryErrors, err)
		slog.Warn("recover pending tickets after team member removal failed", "tenant_id", tenantID, "team_id", teamID, "user_id", userID, "error", err)
	}
	result.PendingConversationsRecovered, err = ConversationDispatchService.RecoverTeamMemberIneligibleAssignments(tenantID, teamID, userID, now)
	if err != nil {
		recoveryErrors = append(recoveryErrors, err)
		slog.Warn("recover pending conversations after team member removal failed", "tenant_id", tenantID, "team_id", teamID, "user_id", userID, "error", err)
	}
	return result, errors.Join(recoveryErrors...)
}

func (s *agentTeamMemberService) countActiveAssignedWorkDB(db *gorm.DB, tenantID, teamID, userID int64) (int64, int64, error) {
	activeTickets := int64(0)
	if db.Migrator().HasTable(&models.Ticket{}) {
		var tickets []models.Ticket
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "status", "accepted_at").
			Where("tenant_id = ? AND current_team_id = ? AND current_assignee_id = ?", tenantID, teamID, userID).
			Where("status NOT IN ?", []enums.TicketStatus{enums.TicketStatusClosed, enums.TicketStatusDone, enums.TicketStatusCancelled}).
			Find(&tickets).Error; err != nil {
			return 0, 0, err
		}
		for i := range tickets {
			if enums.NormalizeTicketStatus(string(tickets[i].Status)) == enums.TicketStatusPendingAssigneeAccept && tickets[i].AcceptedAt == nil {
				continue
			}
			activeTickets++
		}
	}
	activeConversations := int64(0)
	if db.Migrator().HasTable(&models.Conversation{}) {
		var conversations []models.Conversation
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "status").
			Where("tenant_id = ? AND current_team_id = ? AND current_assignee_id = ?", tenantID, teamID, userID).
			Where("status IN ?", []enums.IMConversationStatus{enums.IMConversationStatusPending, enums.IMConversationStatusActive}).
			Find(&conversations).Error; err != nil {
			return 0, 0, err
		}
		for i := range conversations {
			if conversations[i].Status == enums.IMConversationStatusActive {
				activeConversations++
			}
		}
	}
	return activeTickets, activeConversations, nil
}

func (s *agentTeamMemberService) ensureSupportProfileForMemberDB(
	db *gorm.DB,
	tenantID int64,
	teamID int64,
	userID int64,
	memberID int64,
	dispatchEnabled bool,
	operator *dto.AuthPrincipal,
) error {
	if db == nil || tenantID <= 0 || teamID <= 0 || userID <= 0 || !db.Migrator().HasTable(&models.AgentProfile{}) {
		return nil
	}
	profile := repositories.AgentProfileRepository.FindOne(db, sqls.NewCnd().Eq("user_id", userID))
	if profile != nil {
		if profile.TenantID != tenantID {
			return errorsx.InvalidParam("member already has a dispatch profile in another tenant")
		}
		if profile.Status == enums.StatusDeleted {
			return repositories.AgentProfileRepository.Updates(db, profile.ID, map[string]any{
				"team_id":                 teamID,
				"service_status":          enums.ServiceStatusIdle,
				"auto_assign_enabled":     dispatchEnabled,
				"receive_offline_message": true,
				"status":                  enums.StatusOk,
				"update_user_id":          auditOperatorID(operator),
				"update_user_name":        auditOperatorName(operator),
				"updated_at":              time.Now(),
			})
		}
		if profile.TeamID == 0 {
			return repositories.AgentProfileRepository.Updates(db, profile.ID, map[string]any{
				"team_id":          teamID,
				"update_user_id":   auditOperatorID(operator),
				"update_user_name": auditOperatorName(operator),
				"updated_at":       time.Now(),
			})
		}
		return nil
	}
	user := repositories.UserRepository.Get(db, userID)
	if user == nil || user.Status != enums.StatusOk {
		return errorsx.InvalidParamI18n("error.e0334")
	}
	displayName := strings.TrimSpace(user.Nickname)
	if memberID > 0 && db.Migrator().HasTable(&models.TenantMember{}) {
		if member := repositories.EnterpriseIAMRepository.GetTenantMember(db, tenantID, memberID); member != nil && strings.TrimSpace(member.DisplayName) != "" {
			displayName = strings.TrimSpace(member.DisplayName)
		}
	}
	if displayName == "" {
		displayName = strings.TrimSpace(user.Username)
	}
	if displayName == "" {
		displayName = fmt.Sprintf("工程师 %d", userID)
	}
	profile = &models.AgentProfile{
		TenantID:              tenantID,
		UserID:                userID,
		TeamID:                teamID,
		AgentCode:             s.defaultAgentCodeDB(db, tenantID, userID),
		DisplayName:           displayName,
		ServiceStatus:         enums.ServiceStatusIdle,
		MaxConcurrentCount:    5,
		PriorityLevel:         50,
		AutoAssignEnabled:     dispatchEnabled,
		ReceiveOfflineMessage: true,
		Status:                enums.StatusOk,
		Remark:                "产品维修组成员默认派单档案",
		AuditFields:           utils.BuildAuditFields(operator),
	}
	return repositories.AgentProfileRepository.Create(db, profile)
}

func (s *agentTeamMemberService) defaultAgentCodeDB(db *gorm.DB, tenantID, userID int64) string {
	base := fmt.Sprintf("ENG-%d-%d", tenantID, userID)
	code := base
	for i := 2; i <= 100; i++ {
		if existing := repositories.AgentProfileRepository.FindOne(db, sqls.NewCnd().Eq("agent_code", code)); existing == nil {
			return code
		}
		code = fmt.Sprintf("%s-%d", base, i)
	}
	return fmt.Sprintf("%s-%d", base, time.Now().UnixNano())
}

func (s *agentTeamMemberService) FindActiveMembersByTeamID(db *gorm.DB, tenantID, teamID int64) []models.AgentTeamMember {
	if teamID <= 0 || !s.tableReady(db) {
		return []models.AgentTeamMember{}
	}
	cnd := sqls.NewCnd().
		Eq("team_id", teamID).
		Eq("status", enums.StatusOk).
		Asc("id")
	if tenantID > 0 {
		cnd.Eq("tenant_id", tenantID)
	}
	return repositories.AgentTeamMemberRepository.Find(db, cnd)
}

func (s *agentTeamMemberService) FindTeamIDsByUserID(db *gorm.DB, tenantID, userID int64) []int64 {
	if tenantID <= 0 || userID <= 0 {
		return nil
	}
	teamIDs := make([]int64, 0, 4)
	if s.tableReady(db) {
		memberships := repositories.AgentTeamMemberRepository.Find(db, sqls.NewCnd().
			Eq("tenant_id", tenantID).
			Eq("user_id", userID).
			Eq("status", enums.StatusOk).
			Asc("team_id"))
		for _, item := range memberships {
			if item.TeamID > 0 {
				teamIDs = append(teamIDs, item.TeamID)
			}
		}
		return s.filterActiveTeamIDsForTenant(db, tenantID, uniqueServiceInt64s(teamIDs))
	}
	return nil
}

func (s *agentTeamMemberService) FindTeamIDsByUserIDs(db *gorm.DB, tenantID int64, userIDs []int64) map[int64][]int64 {
	ret := make(map[int64][]int64)
	userIDs = uniqueServiceInt64s(userIDs)
	if db == nil || tenantID <= 0 || len(userIDs) == 0 {
		return ret
	}

	if !s.tableReady(db) {
		return ret
	}
	memberships := repositories.AgentTeamMemberRepository.Find(db, sqls.NewCnd().
		Eq("tenant_id", tenantID).
		In("user_id", userIDs).
		Eq("status", enums.StatusOk).
		Asc("team_id"))
	for _, membership := range memberships {
		if membership.UserID > 0 && membership.TeamID > 0 {
			ret[membership.UserID] = append(ret[membership.UserID], membership.TeamID)
		}
	}

	allTeamIDs := make([]int64, 0)
	for userID, teamIDs := range ret {
		ret[userID] = uniqueServiceInt64s(teamIDs)
		allTeamIDs = append(allTeamIDs, ret[userID]...)
	}
	activeTeamIDs := s.filterActiveTeamIDsForTenant(db, tenantID, allTeamIDs)
	activeTeamSet := make(map[int64]struct{}, len(activeTeamIDs))
	for _, teamID := range activeTeamIDs {
		activeTeamSet[teamID] = struct{}{}
	}
	for userID, teamIDs := range ret {
		filtered := make([]int64, 0, len(teamIDs))
		for _, teamID := range teamIDs {
			if _, ok := activeTeamSet[teamID]; ok {
				filtered = append(filtered, teamID)
			}
		}
		ret[userID] = filtered
	}
	return ret
}

func (s *agentTeamMemberService) IsUserActiveMemberOfTeamDB(db *gorm.DB, tenantID, teamID, userID int64) bool {
	if tenantID <= 0 || teamID <= 0 || userID <= 0 {
		return false
	}
	if db == nil {
		db = sqls.DB()
	}
	if !s.tableReady(db) {
		return false
	}
	if db != nil && db.Migrator().HasTable(&models.AgentTeam{}) {
		team := repositories.AgentTeamRepository.Get(db, teamID)
		if team == nil || team.TenantID != tenantID || team.Status != enums.StatusOk {
			return false
		}
	}
	profile := repositories.AgentProfileRepository.FindOne(db, sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("user_id", userID).
		Eq("status", enums.StatusOk))
	if profile == nil {
		return false
	}
	if item := repositories.AgentTeamMemberRepository.FindOne(db, sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("team_id", teamID).
		Eq("user_id", userID).
		Eq("status", enums.StatusOk)); item != nil {
		return true
	}
	return false
}

func (s *agentTeamMemberService) IsUserDispatchEnabledMemberOfTeamDB(db *gorm.DB, tenantID, teamID, userID int64) bool {
	if tenantID <= 0 || teamID <= 0 || userID <= 0 {
		return false
	}
	if db == nil {
		db = sqls.DB()
	}
	if db == nil {
		return false
	}
	if !s.tableReady(db) || !db.Migrator().HasTable(&models.AgentProfile{}) {
		return false
	}
	if db != nil && db.Migrator().HasTable(&models.AgentTeam{}) {
		team := repositories.AgentTeamRepository.Get(db, teamID)
		if team == nil || team.TenantID != tenantID || team.Status != enums.StatusOk {
			return false
		}
	}
	profile := repositories.AgentProfileRepository.FindOne(db, sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("user_id", userID).
		Eq("status", enums.StatusOk))
	if profile == nil || !profile.AutoAssignEnabled {
		return false
	}
	if item := repositories.AgentTeamMemberRepository.FindOne(db, sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("team_id", teamID).
		Eq("user_id", userID).
		Eq("status", enums.StatusOk)); item != nil {
		return item.DispatchEnabled
	}
	return false
}

func (s *agentTeamMemberService) filterActiveTeamIDsForTenant(db *gorm.DB, tenantID int64, teamIDs []int64) []int64 {
	teamIDs = uniqueServiceInt64s(teamIDs)
	if tenantID <= 0 || len(teamIDs) == 0 {
		return teamIDs
	}
	if db == nil || !db.Migrator().HasTable(&models.AgentTeam{}) {
		return teamIDs
	}
	teams := repositories.AgentTeamRepository.FindByIds(db, teamIDs)
	activeSet := make(map[int64]struct{}, len(teams))
	for _, team := range teams {
		if team.TenantID != tenantID || team.Status != enums.StatusOk {
			continue
		}
		activeSet[team.ID] = struct{}{}
	}
	ret := make([]int64, 0, len(teamIDs))
	for _, teamID := range teamIDs {
		if _, ok := activeSet[teamID]; ok {
			ret = append(ret, teamID)
		}
	}
	return ret
}

func (s *agentTeamMemberService) FindProfilesByTeamID(db *gorm.DB, tenantID, teamID int64) []models.AgentProfile {
	if teamID <= 0 {
		return []models.AgentProfile{}
	}
	if tenantID <= 0 {
		if team := repositories.AgentTeamRepository.Get(db, teamID); team != nil {
			tenantID = team.TenantID
		}
	}
	userIDSet := make(map[int64]struct{})
	userIDs := make([]int64, 0)
	for _, membership := range s.FindActiveMembersByTeamID(db, tenantID, teamID) {
		if membership.UserID <= 0 {
			continue
		}
		if _, exists := userIDSet[membership.UserID]; exists {
			continue
		}
		userIDSet[membership.UserID] = struct{}{}
		userIDs = append(userIDs, membership.UserID)
	}
	profiles := make([]models.AgentProfile, 0)
	if len(userIDs) > 0 {
		cnd := sqls.NewCnd().
			In("user_id", userIDs).
			Eq("status", enums.StatusOk).
			Asc("id")
		if tenantID > 0 {
			cnd.Eq("tenant_id", tenantID)
		}
		profiles = append(profiles, repositories.AgentProfileRepository.Find(db, cnd)...)
	}
	slices.SortFunc(profiles, func(a, b models.AgentProfile) int {
		switch {
		case a.PriorityLevel > b.PriorityLevel:
			return -1
		case a.PriorityLevel < b.PriorityLevel:
			return 1
		case a.UserID < b.UserID:
			return -1
		case a.UserID > b.UserID:
			return 1
		default:
			return 0
		}
	})
	return profiles
}

func (s *agentTeamMemberService) FindDispatchProfilesByTeamIDs(db *gorm.DB, tenantID int64, teamIDs []int64) []AgentTeamDispatchProfile {
	teamIDs = uniqueServiceInt64s(teamIDs)
	if len(teamIDs) == 0 || !s.tableReady(db) {
		return []AgentTeamDispatchProfile{}
	}
	teams := repositories.AgentTeamRepository.FindByIds(db, teamIDs)
	teamByID := make(map[int64]models.AgentTeam, len(teams))
	for _, team := range teams {
		if team.Status != enums.StatusOk || (tenantID > 0 && team.TenantID != tenantID) {
			continue
		}
		team.AssignmentMode = NormalizeAgentTeamAssignmentMode(team.AssignmentMode)
		teamByID[team.ID] = team
	}
	if len(teamByID) == 0 {
		return []AgentTeamDispatchProfile{}
	}

	profileByUserID := make(map[int64]models.AgentProfile)
	addProfile := func(profile models.AgentProfile) {
		if profile.UserID <= 0 || profile.Status != enums.StatusOk {
			return
		}
		if tenantID > 0 && profile.TenantID != tenantID {
			return
		}
		profileByUserID[profile.UserID] = profile
	}
	profiles := repositories.AgentProfileRepository.Find(db, sqls.NewCnd().
		Eq("status", enums.StatusOk))
	for _, profile := range profiles {
		addProfile(profile)
	}

	ret := make([]AgentTeamDispatchProfile, 0)
	seen := make(map[string]struct{})
	memberships := repositories.AgentTeamMemberRepository.Find(db, sqls.NewCnd().
		In("team_id", teamIDs).
		Eq("status", enums.StatusOk).
		Asc("team_id").
		Asc("id"))
	for _, membership := range memberships {
		team, ok := teamByID[membership.TeamID]
		if !ok || (tenantID > 0 && membership.TenantID != tenantID) {
			continue
		}
		profile, ok := profileByUserID[membership.UserID]
		if !ok {
			continue
		}
		key := dispatchProfileKey(membership.TeamID, membership.UserID)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		ret = append(ret, AgentTeamDispatchProfile{
			Profile:         profile,
			TeamID:          membership.TeamID,
			AssignmentMode:  team.AssignmentMode,
			DispatchWeight:  normalizeAgentTeamMemberWeight(membership.DispatchWeight),
			DispatchEnabled: membership.DispatchEnabled,
		})
	}
	return ret
}

func dispatchProfileKey(teamID, userID int64) string {
	return utils.JoinInt64s([]int64{teamID, userID})
}
