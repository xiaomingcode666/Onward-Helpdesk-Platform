package dashboard

import (
	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/httpx/params"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/web"
)

func AIWorkflowAnyList(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	cnd := params.NewPagedSqlCnd(ctx,
		params.QueryFilter{ParamName: "status"},
		params.QueryFilter{ParamName: "name", Op: params.Like},
		params.QueryFilter{ParamName: "agentId"},
	).Desc("id")
	if operator.TenantID > 0 {
		cnd.Where("(tenant_id = ? OR (tenant_id = 0 AND scope = ?))", operator.TenantID, models.AIWorkflowScopePlatform)
	}
	list, paging := services.AIWorkflowService.FindPageByCnd(cnd)
	httpx.WriteJSON(ctx, &web.PageResult{Results: builders.BuildAIWorkflowList(list), Page: paging})
}

func AIWorkflowGetBy(ctx *gin.Context) {
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item := services.AIWorkflowService.GetForOperator(id, operator)
	if item == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0002"))
		return
	}
	httpx.WriteJSON(ctx, builders.BuildAIWorkflow(item))
}

func AIWorkflowPostCreate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentCreate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.CreateAIWorkflowRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.AIWorkflowService.CreateWorkflow(req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildAIWorkflow(item))
}

func AIWorkflowPostUpdate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.UpdateAIWorkflowRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.AIWorkflowService.UpdateWorkflow(req, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func AIWorkflowPostDelete(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentDelete)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.DeleteAIWorkflowRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.AIWorkflowService.DeleteWorkflow(req.ID, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func AIWorkflowGetNodeSpecList(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildAIWorkflowNodeSpecs(services.AIWorkflowService.ListNodeSpecs()))
}

func AIWorkflowGetDefaultDefinition(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, services.AIWorkflowService.DefaultAgentWorkflowDefinition())
}

func AIWorkflowPostValidate(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.ValidateAIWorkflowRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result := services.AIWorkflowService.ValidateDefinition(req.Definition)
	httpx.WriteJSON(ctx, response.AIWorkflowValidationResponse{
		Valid:  result.Valid,
		Errors: result.Errors,
	})
}

func AIWorkflowPostPublish(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.PublishAIWorkflowRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.AIWorkflowService.PublishWorkflow(req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildAIWorkflowVersion(item))
}

func AIWorkflowGetByAgent(ctx *gin.Context) {
	agentID, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.AIWorkflowService.GetOrCreateAgentWorkflow(agentID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildAIWorkflow(item))
}

func AIWorkflowPostSaveAgent(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.SaveAIWorkflowRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.AIWorkflowService.SaveAgentWorkflow(req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildAIWorkflow(item))
}

func AIWorkflowPostPublishAgent(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.PublishAIWorkflowRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.AIWorkflowService.PublishAgentWorkflow(req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildAIWorkflowVersion(item))
}

func AIWorkflowAnyVersionList(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	queryParams := params.NewQueryParams(ctx)
	cnd := params.NewPagedSqlCnd(ctx, params.QueryFilter{ParamName: "workflowId"}).Desc("version").Desc("id")
	workflowID, _ := params.GetInt64(ctx, "workflowId")
	if workflowID <= 0 || services.AIWorkflowService.GetForOperator(workflowID, operator) == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0002"))
		return
	}
	queryParams.Cnd = *cnd
	list, paging := services.AIWorkflowService.FindVersionPageByParams(queryParams)
	httpx.WriteJSON(ctx, &web.PageResult{Results: builders.BuildAIWorkflowVersionList(list), Page: paging})
}

func AIWorkflowGetVersionBy(ctx *gin.Context) {
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item := services.AIWorkflowService.GetVersionForOperator(id, operator)
	if item == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0002"))
		return
	}
	httpx.WriteJSON(ctx, builders.BuildAIWorkflowVersion(item))
}

func AIWorkflowAnyRunList(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	cnd := params.NewPagedSqlCnd(ctx,
		params.QueryFilter{ParamName: "workflowId"},
		params.QueryFilter{ParamName: "workflowVersionId"},
		params.QueryFilter{ParamName: "conversationId"},
		params.QueryFilter{ParamName: "aiAgentId"},
		params.QueryFilter{ParamName: "messageId"},
		params.QueryFilter{ParamName: "status"},
	).Desc("id")
	if operator.TenantID > 0 {
		cnd.Eq("tenant_id", operator.TenantID)
	}
	list, paging := services.AIWorkflowService.FindRunPageByCnd(cnd)
	auditItems := services.AIWorkflowService.BuildRunAuditItems(list)
	results := make([]response.AIWorkflowRunResponse, 0, len(auditItems))
	for i := range auditItems {
		item := auditItems[i]
		results = append(results, builders.BuildAIWorkflowRunWithContext(&item.Run, item.Workflow, item.Version, item.Agent, &item.Skill))
	}
	httpx.WriteJSON(ctx, &web.PageResult{Results: results, Page: paging})
}

func AIWorkflowGetRunBy(ctx *gin.Context) {
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, nodes := services.AIWorkflowService.GetRunDetail(id)
	if item == nil || (operator.TenantID > 0 && item.TenantID != operator.TenantID) {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0002"))
		return
	}
	humanHandling := services.AIWorkflowService.BuildRunHumanHandlingAudit(item)
	auditItems := services.AIWorkflowService.BuildRunAuditItems([]models.AIWorkflowRun{*item})
	if len(auditItems) == 0 {
		ret := builders.BuildAIWorkflowRunDetail(item, nodes)
		ret.HumanHandling = builders.BuildAIWorkflowHumanHandlingAudit(humanHandling)
		httpx.WriteJSON(ctx, ret)
		return
	}
	auditItem := auditItems[0]
	ret := builders.BuildAIWorkflowRunDetailWithContext(&auditItem.Run, nodes, auditItem.Workflow, auditItem.Version, auditItem.Agent, &auditItem.Skill)
	ret.HumanHandling = builders.BuildAIWorkflowHumanHandlingAudit(humanHandling)
	httpx.WriteJSON(ctx, ret)
}
