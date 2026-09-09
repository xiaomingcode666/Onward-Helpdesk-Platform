package dashboard

import (
	"time"

	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/sqls"
	"github.com/mlogclub/simple/web"
)

func AgentAnyList(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAgentView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	cnd := params.NewPagedSqlCnd(ctx,
		params.QueryFilter{ParamName: "userId"},
		params.QueryFilter{ParamName: "teamId"},
		params.QueryFilter{ParamName: "serviceStatus"},
		params.QueryFilter{ParamName: "agentCode", Op: params.Like},
		params.QueryFilter{ParamName: "displayName", Op: params.Like},
	).Desc("id")
	tenantID := operator.EffectiveTenantID()
	if tenantID > 0 {
		cnd.Eq("tenant_id", tenantID)
	}
	list, paging := services.AgentProfileService.FindPageByCnd(cnd)
	results := enrichAgentProfileTeamIDs(tenantID, builders.BuildAgentProfileList(list))
	httpx.WriteJSON(ctx, &web.PageResult{Results: results, Page: paging})
}

func AgentGetList_all(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAgentView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	cnd := params.NewPagedSqlCnd(ctx,
		params.QueryFilter{ParamName: "userId"},
		params.QueryFilter{ParamName: "teamId"},
		params.QueryFilter{ParamName: "serviceStatus"},
		params.QueryFilter{ParamName: "agentCode", Op: params.Like},
	).Desc("id")
	tenantID := operator.EffectiveTenantID()
	if tenantID > 0 {
		cnd.Eq("tenant_id", tenantID)
	}
	if teamID, ok := params.GetInt64(ctx, "teamId"); ok && teamID > 0 {
		httpx.WriteJSON(ctx, buildAgentProfileListForTeam(tenantID, teamID))
		return
	}
	list := services.AgentProfileService.Find(cnd)
	httpx.WriteJSON(ctx, enrichAgentProfileTeamIDs(tenantID, builders.BuildAgentProfileList(list)))
}

func AgentGetDispatchCandidates(ctx *gin.Context) {
	operator, err := requireAgentDispatchCandidatePermission(ctx)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	teamID, _ := params.GetInt64(ctx, "teamId")
	productID, _ := params.GetInt64(ctx, "productId")
	ticketID, _ := params.GetInt64(ctx, "ticketId")
	conversationID, _ := params.GetInt64(ctx, "conversationId")
	manualTransfer, _ := params.GetBool(ctx, "manualTransfer")
	if manualTransfer {
		operator, err = services.AuthService.RequirePermission(ctx, constants.PermissionConversationTransfer)
		if err != nil {
			httpx.WriteJSON(ctx, err)
			return
		}
		conversation := services.ConversationService.Get(conversationID)
		if conversation == nil {
			httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0116"))
			return
		}
		if !services.ConversationService.CanTransferConversation(conversation, operator) {
			httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0223"))
			return
		}
	}
	candidates, err := services.AgentProfileService.FindDispatchCandidates(services.DispatchCandidateQuery{
		TenantID:       operator.EffectiveTenantID(),
		ProductID:      productID,
		TeamID:         teamID,
		TicketID:       ticketID,
		ConversationID: conversationID,
		ManualTransfer: manualTransfer,
	}, time.Now())
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildAgentDispatchCandidateList(candidates))
}

func requireAgentDispatchCandidatePermission(ctx *gin.Context) (*dto.AuthPrincipal, error) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAgentView)
	if err == nil || operator == nil {
		return operator, err
	}
	for _, permission := range []constants.Permission{
		constants.PermissionAgentTeamView,
		constants.PermissionTicketAssign,
		constants.PermissionTicketChangeStatus,
		constants.PermissionConversationAssign,
		constants.PermissionConversationTransfer,
	} {
		if operator.HasPermission(permission.Code) {
			return operator, nil
		}
	}
	return operator, err
}

func AgentGetBy(ctx *gin.Context) {
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAgentView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item := services.AgentProfileService.GetForTenant(id, operator.EffectiveTenantID())
	if item == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0164"))
		return
	}
	result := builders.BuildAgentProfileResponse(item)
	if result != nil {
		result.TeamIDs = services.AgentTeamMemberService.FindTeamIDsByUserID(sqls.DB(), operator.EffectiveTenantID(), result.UserID)
	}
	httpx.WriteJSON(ctx, result)
}

func AgentPostCreate(ctx *gin.Context) {
	user, err := services.AuthService.RequirePermission(ctx, constants.PermissionAgentCreate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.CreateAgentProfileRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.AgentProfileService.CreateAgentProfile(req, user)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildAgentProfileResponse(item))
}

func AgentPostUpdate(ctx *gin.Context) {
	user, err := services.AuthService.RequirePermission(ctx, constants.PermissionAgentUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.UpdateAgentProfileRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.AgentProfileService.UpdateAgentProfile(req, user); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func buildAgentProfileListForTeam(tenantID, teamID int64) []response.AgentProfileResponse {
	profiles := services.AgentTeamMemberService.FindProfilesByTeamID(sqls.DB(), tenantID, teamID)
	results := builders.BuildAgentProfileList(profiles)
	members := services.AgentTeamMemberService.FindActiveMembersByTeamID(sqls.DB(), tenantID, teamID)
	memberByUserID := make(map[int64]models.AgentTeamMember, len(members))
	for _, member := range members {
		memberByUserID[member.UserID] = member
	}
	for i := range results {
		results[i].TeamDispatchEnabled = results[i].AutoAssignEnabled
		results[i].DispatchWeight = 1
		if member, ok := memberByUserID[results[i].UserID]; ok {
			results[i].TeamDispatchEnabled = member.DispatchEnabled
			results[i].DispatchWeight = member.DispatchWeight
		}
	}
	return enrichAgentProfileTeamIDs(tenantID, results)
}

func enrichAgentProfileTeamIDs(tenantID int64, results []response.AgentProfileResponse) []response.AgentProfileResponse {
	userIDs := make([]int64, 0, len(results))
	for _, result := range results {
		if result.UserID > 0 {
			userIDs = append(userIDs, result.UserID)
		}
	}
	teamIDsByUser := services.AgentTeamMemberService.FindTeamIDsByUserIDs(sqls.DB(), tenantID, userIDs)
	for i := range results {
		results[i].TeamIDs = teamIDsByUser[results[i].UserID]
	}
	return results
}

func AgentPostDelete(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAgentDelete)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.DeleteAgentProfileRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.AgentProfileService.DeleteAgentProfile(req.ID, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}
