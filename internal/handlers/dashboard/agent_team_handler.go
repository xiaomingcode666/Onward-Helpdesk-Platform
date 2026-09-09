package dashboard

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/sqls"
)

func AgentTeamAnyList(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAgentTeamView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	cnd := params.NewSqlCnd(ctx,
		params.QueryFilter{ParamName: "status"},
		params.QueryFilter{ParamName: "leaderUserId"},
		params.QueryFilter{ParamName: "name", Op: params.Like},
	).Desc("id")
	if _, ok := params.Get(ctx, "status"); !ok {
		cnd.Where("status <> ?", enums.StatusDeleted)
	}
	tenantID := operator.EffectiveTenantID()
	if tenantID > 0 {
		cnd.Eq("tenant_id", tenantID)
	}
	list := services.AgentTeamService.Find(cnd)
	results := make([]response.AgentTeamResponse, 0, len(list))
	for _, item := range list {
		results = append(results, buildAgentTeamResponse(&item))
	}
	httpx.WriteJSON(ctx, results)
}

func AgentTeamGetList_all(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAgentTeamView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	cnd := sqls.NewCnd().Eq("status", enums.StatusOk).Asc("parent_id").Asc("product_id").Asc("id")
	tenantID := operator.EffectiveTenantID()
	if tenantID > 0 {
		cnd.Eq("tenant_id", tenantID)
	}
	list := services.AgentTeamService.Find(cnd)
	results := make([]response.AgentTeamResponse, 0, len(list))
	for _, item := range list {
		results = append(results, buildAgentTeamResponse(&item))
	}
	httpx.WriteJSON(ctx, results)
}

func AgentTeamGetBy(ctx *gin.Context) {
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAgentTeamView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item := services.AgentTeamService.GetForTenant(id, operator.EffectiveTenantID())
	if item == nil || item.Status == enums.StatusDeleted {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0169"))
		return
	}
	httpx.WriteJSON(ctx, buildAgentTeamResponse(item))
}

func AgentTeamPostCreate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAgentTeamCreate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.CreateAgentTeamRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.AgentTeamService.CreateAgentTeam(req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, buildAgentTeamResponse(item))
}

func AgentTeamPostUpdate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAgentTeamUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.UpdateAgentTeamRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.AgentTeamService.UpdateAgentTeam(req, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func AgentTeamMemberPostUpsert(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAgentTeamUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.UpsertAgentTeamMemberRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.AgentTeamMemberService.EnsureMember(operator.EffectiveTenantID(), req.TeamID, req.UserID, 0, req.DispatchWeight, req.DispatchEnabled, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, buildAgentTeamMemberResponse(item))
}

func AgentTeamMemberPostDelete(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAgentTeamUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.DeleteAgentTeamMemberRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.AgentTeamMemberService.RemoveMember(operator.EffectiveTenantID(), req.TeamID, req.UserID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, response.AgentTeamMemberRemovalResponse{
		TeamID:                        result.TeamID,
		UserID:                        result.UserID,
		PendingTicketsRecovered:       result.PendingTicketsRecovered,
		PendingConversationsRecovered: result.PendingConversationsRecovered,
		AlreadyRemoved:                result.AlreadyRemoved,
	})
}

func AgentTeamPostDelete(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAgentTeamDelete)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.DeleteAgentTeamRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.AgentTeamService.DeleteAgentTeam(req.ID, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func buildAgentTeamMemberResponse(item *models.AgentTeamMember) response.AgentTeamMemberResponse {
	if item == nil {
		return response.AgentTeamMemberResponse{}
	}
	return response.AgentTeamMemberResponse{
		ID:              item.ID,
		TenantID:        item.TenantID,
		TeamID:          item.TeamID,
		UserID:          item.UserID,
		MemberID:        item.MemberID,
		DispatchEnabled: item.DispatchEnabled,
		DispatchWeight:  item.DispatchWeight,
		Status:          item.Status,
	}
}

func buildAgentTeamResponse(item *models.AgentTeam) response.AgentTeamResponse {
	ret := response.AgentTeamResponse{
		ID:               item.ID,
		TenantID:         item.TenantID,
		ParentID:         item.ParentID,
		ProductID:        item.ProductID,
		DepartmentID:     item.DepartmentID,
		TeamType:         item.TeamType,
		SystemManaged:    item.SystemManaged,
		Name:             item.Name,
		LeaderUserID:     item.LeaderUserID,
		AssignmentMode:   services.NormalizeAgentTeamAssignmentMode(item.AssignmentMode),
		ScheduleEnforced: item.ScheduleEnforced,
		ScheduleVersion:  item.ScheduleVersion,
		Status:           item.Status,
		Description:      item.Description,
		Remark:           item.Remark,
	}
	if product := services.ProductService.Get(item.ProductID); product != nil && (item.TenantID <= 0 || product.TenantID == item.TenantID) {
		ret.ProductName = product.Name
	}
	if user := services.UserService.Get(item.LeaderUserID); user != nil {
		ret.LeaderUsername = user.Username
		ret.LeaderNickname = user.Nickname
	}
	return ret
}
