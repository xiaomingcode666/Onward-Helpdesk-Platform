package builders

import (
	"fmt"
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func BuildAgentProfileList(items []models.AgentProfile) []response.AgentProfileResponse {
	if len(items) == 0 {
		return []response.AgentProfileResponse{}
	}

	userIDs := make([]int64, 0, len(items))
	teamIDs := make([]int64, 0, len(items))
	tenantUserIDs := make(map[int64][]int64)
	for _, item := range items {
		if item.UserID > 0 {
			userIDs = append(userIDs, item.UserID)
			tenantUserIDs[item.TenantID] = append(tenantUserIDs[item.TenantID], item.UserID)
		}
		if item.TeamID > 0 {
			teamIDs = append(teamIDs, item.TeamID)
		}
	}

	users := services.UserService.FindByIds(userIDs)
	teams := services.AgentTeamService.FindByIds(teamIDs)

	userMap := make(map[int64]*models.User, len(users))
	for i := range users {
		userMap[users[i].ID] = &users[i]
	}

	teamMap := make(map[int64]*models.AgentTeam, len(teams))
	for i := range teams {
		teamMap[teams[i].ID] = &teams[i]
	}

	tenantMemberMap := make(map[int64]map[int64]*models.TenantMember, len(tenantUserIDs))
	for tenantID, ids := range tenantUserIDs {
		members, err := repositories.EnterpriseIAMRepository.FindTenantMembersByUserIDs(sqls.DB(), tenantID, ids)
		if err != nil {
			continue
		}
		tenantMemberMap[tenantID] = members
	}

	results := make([]response.AgentProfileResponse, 0, len(items))
	for _, item := range items {
		var member *models.TenantMember
		if membersByUserID := tenantMemberMap[item.TenantID]; membersByUserID != nil {
			member = membersByUserID[item.UserID]
		}
		if result := doBuildAgentProfileResponse(&item, userMap[item.UserID], teamMap[item.TeamID], member); result != nil {
			results = append(results, *result)
		}
	}
	return results
}

func BuildAgentProfileResponse(item *models.AgentProfile) *response.AgentProfileResponse {
	user := services.UserService.Get(item.UserID)
	team := services.AgentTeamService.Get(item.TeamID)
	member := repositories.EnterpriseIAMRepository.FindTenantMemberByUserID(sqls.DB(), item.TenantID, item.UserID)
	return doBuildAgentProfileResponse(item, user, team, member)
}

func BuildAgentDispatchCandidateList(items []services.DispatchCandidateProfile) []response.AgentDispatchCandidateResponse {
	if len(items) == 0 {
		return []response.AgentDispatchCandidateResponse{}
	}
	ret := make([]response.AgentDispatchCandidateResponse, 0, len(items))
	for _, item := range items {
		base := BuildAgentProfileResponse(&item.Profile)
		if base == nil {
			continue
		}
		base.TeamID = item.TeamID
		base.TeamDispatchEnabled = item.TeamDispatchEnabled
		base.DispatchWeight = item.DispatchWeight
		if team := services.AgentTeamService.Get(item.TeamID); team != nil {
			base.TeamName = team.Name
		}
		capacityRemaining := 0
		if base.MaxConcurrentCount > 0 && item.Workload < base.MaxConcurrentCount {
			capacityRemaining = base.MaxConcurrentCount - item.Workload
		}
		ret = append(ret, response.AgentDispatchCandidateResponse{
			AgentProfileResponse:    *base,
			AssignmentMode:          item.AssignmentMode,
			TeamEligible:            item.TeamEligible,
			WorkStatusConfirmed:     item.WorkStatusConfirmed,
			Reachable:               item.Reachable,
			CapacityAvailable:       item.CapacityAvailable,
			ActiveConversationCount: item.ActiveConversationCount,
			OpenTicketCount:         item.OpenTicketCount,
			Workload:                item.Workload,
			LoadRate:                item.LoadRate,
			CapacityRemaining:       capacityRemaining,
		})
	}
	return ret
}

func doBuildAgentProfileResponse(item *models.AgentProfile, user *models.User, team *models.AgentTeam, member *models.TenantMember) *response.AgentProfileResponse {
	if item == nil {
		return nil
	}
	displayName := strings.TrimSpace(item.DisplayName)
	avatar := strings.TrimSpace(item.Avatar)
	if member != nil {
		if name := strings.TrimSpace(member.DisplayName); name != "" {
			displayName = name
		}
	}
	if user != nil {
		if displayName == "" {
			displayName = strings.TrimSpace(user.Nickname)
		}
		if displayName == "" {
			displayName = strings.TrimSpace(user.Username)
		}
		if userAvatar := strings.TrimSpace(user.Avatar); userAvatar != "" {
			avatar = userAvatar
		}
	}
	if displayName == "" {
		displayName = strings.TrimSpace(item.AgentCode)
	}
	if displayName == "" {
		displayName = fmt.Sprintf("成员 #%d", item.UserID)
	}
	ret := &response.AgentProfileResponse{
		ID:                    item.ID,
		TenantID:              item.TenantID,
		UserID:                item.UserID,
		TeamID:                item.TeamID,
		AgentCode:             item.AgentCode,
		DisplayName:           displayName,
		Avatar:                avatar,
		ServiceStatus:         item.ServiceStatus,
		MaxConcurrentCount:    item.MaxConcurrentCount,
		PriorityLevel:         item.PriorityLevel,
		AutoAssignEnabled:     item.AutoAssignEnabled,
		TeamDispatchEnabled:   item.AutoAssignEnabled,
		ReceiveOfflineMessage: item.ReceiveOfflineMessage,
		LastOnlineAt:          utils.FormatTimePtr(item.LastOnlineAt),
		LastStatusAt:          utils.FormatTimePtr(item.LastStatusAt),
		Remark:                item.Remark,
	}
	if user != nil {
		ret.Username = user.Username
		ret.Nickname = user.Nickname
	}
	if team != nil {
		ret.TeamName = team.Name
	}
	return ret
}
