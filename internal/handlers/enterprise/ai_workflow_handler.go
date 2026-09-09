package enterprise

import (
	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/httpx/params"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/sqls"
	"github.com/mlogclub/simple/web"
)

func AIWorkflowSummary(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIWorkflowView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := enterpriseAIWorkflowEnsureVisibleDefaults(operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	workflows := services.AIWorkflowService.Find(enterpriseAIWorkflowBaseCnd(operator))
	stableVersionByWorkflow := enterpriseAIWorkflowStableVersionMap(workflows)
	adoption, stableAdoption := enterpriseAIWorkflowAdoption(operator.TenantID, stableVersionByWorkflow, nil)

	var adopted int64
	var attention int64
	platformTemplates := make([]models.AIWorkflow, 0)
	for i := range workflows {
		item := workflows[i]
		adoptedCount := adoption[item.ID]
		stableAdoptedCount := stableAdoption[item.ID]
		adopted += adoptedCount
		if enterpriseAIWorkflowNeedsAttention(
			item,
			adoptedCount,
			stableAdoptedCount,
			stableVersionByWorkflow[item.SourceWorkflowID],
		) {
			attention++
		}
		if item.Scope == models.AIWorkflowScopePlatform && item.CurrentStableVersionID > 0 {
			platformTemplates = append(platformTemplates, item)
		}
	}

	httpx.WriteJSON(ctx, response.AIWorkflowSummaryResponse{
		Total:                   int64(len(workflows)),
		Adopted:                 adopted,
		Attention:               attention,
		PlatformTemplates:       builders.BuildAIWorkflowList(platformTemplates),
		StableVersionByWorkflow: stableVersionByWorkflow,
	})
}

func AIWorkflowAdoption(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIWorkflowView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	workflowIDs := utils.SplitInt64s(ctx.Query("workflow_ids"))
	workflowCnd := enterpriseAIWorkflowBaseCnd(operator)
	if len(workflowIDs) > 0 {
		workflowCnd.In("id", workflowIDs)
	}
	workflows := services.AIWorkflowService.Find(workflowCnd)
	stableVersionByWorkflow := enterpriseAIWorkflowStableVersionMap(workflows)

	sourceWorkflowIDs := make([]int64, 0)
	seenSourceIDs := make(map[int64]struct{})
	for i := range workflows {
		sourceID := workflows[i].SourceWorkflowID
		if sourceID <= 0 {
			continue
		}
		if _, ok := seenSourceIDs[sourceID]; ok {
			continue
		}
		seenSourceIDs[sourceID] = struct{}{}
		sourceWorkflowIDs = append(sourceWorkflowIDs, sourceID)
	}
	if len(sourceWorkflowIDs) > 0 {
		sourceWorkflows := services.AIWorkflowService.Find(enterpriseAIWorkflowBaseCnd(operator).In("id", sourceWorkflowIDs))
		for i := range sourceWorkflows {
			stableVersionByWorkflow[sourceWorkflows[i].ID] = sourceWorkflows[i].CurrentStableVersionID
		}
	}

	adoption, stableAdoption := enterpriseAIWorkflowAdoption(operator.TenantID, stableVersionByWorkflow, workflowIDs)
	httpx.WriteJSON(ctx, response.AIWorkflowAdoptionResponse{
		Adoption:                adoption,
		StableAdoption:          stableAdoption,
		StableVersionByWorkflow: stableVersionByWorkflow,
	})
}

func AIWorkflowList(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIWorkflowView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := enterpriseAIWorkflowEnsureVisibleDefaults(operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	cnd := params.NewPagedSqlCnd(ctx,
		params.QueryFilter{ParamName: "status"},
		params.QueryFilter{ParamName: "scope"},
		params.QueryFilter{ParamName: "name", Op: params.Like},
	).Where("status <> ?", enums.StatusDeleted).
		Eq("agent_id", 0).
		Desc("scope").Desc("updated_at").Desc("id")
	enterpriseAIWorkflowScopeCnd(cnd, operator)
	enterpriseAIWorkflowCapabilityCnd(cnd, operator)
	list, paging := services.AIWorkflowService.FindPageByCnd(cnd)
	httpx.WriteJSON(ctx, &web.PageResult{Results: builders.BuildAIWorkflowList(list), Page: paging})
}

func enterpriseAIWorkflowBaseCnd(operator *dto.AuthPrincipal) *sqls.Cnd {
	cnd := sqls.NewCnd().
		Where("status <> ?", enums.StatusDeleted).
		Eq("agent_id", 0).
		Desc("scope").
		Desc("updated_at").
		Desc("id")
	enterpriseAIWorkflowScopeCnd(cnd, operator)
	return enterpriseAIWorkflowCapabilityCnd(cnd, operator)
}

func enterpriseAIWorkflowScopeCnd(cnd *sqls.Cnd, operator *dto.AuthPrincipal) *sqls.Cnd {
	if operator != nil && operator.TenantID > 0 {
		return cnd.Where("((tenant_id = 0 AND scope = ? AND locked = ?) OR (tenant_id = ? AND scope = ?))",
			models.AIWorkflowScopePlatform, true, operator.TenantID, models.AIWorkflowScopeTenant)
	}
	return cnd.Eq("tenant_id", 0).Eq("scope", models.AIWorkflowScopePlatform).Eq("locked", true)
}

func enterpriseAIWorkflowCapabilityCnd(cnd *sqls.Cnd, operator *dto.AuthPrincipal) *sqls.Cnd {
	if operator == nil || operator.TenantID <= 0 {
		return cnd
	}
	if services.TenantCapabilityService.AIEnabled(operator.TenantID) {
		if services.TenantCapabilityService.KnowledgeSupport(operator.TenantID) {
			return cnd.Where(
				"(scope <> ? OR code = ?)",
				models.AIWorkflowScopePlatform,
				services.PlatformKnowledgeSupportWorkflowCode,
			)
		}
		return cnd.Where(
			"(scope <> ? OR code NOT IN ?)",
			models.AIWorkflowScopePlatform,
			[]string{services.PlatformDispatchOnlyWorkflowCode, services.PlatformKnowledgeSupportWorkflowCode},
		)
	}
	return cnd.Where(
		"(tenant_id = 0 AND scope = ? AND code = ?)",
		models.AIWorkflowScopePlatform,
		services.PlatformDispatchOnlyWorkflowCode,
	)
}

func enterpriseAIWorkflowEnsureVisibleDefaults(operator *dto.AuthPrincipal) error {
	if operator == nil || operator.TenantID <= 0 || services.TenantCapabilityService.AIEnabled(operator.TenantID) {
		return nil
	}
	_, _, err := services.AIWorkflowService.EnsurePlatformDispatchOnlyWorkflowDB(sqls.DB())
	return err
}

func enterpriseAIWorkflowStableVersionMap(workflows []models.AIWorkflow) map[int64]int64 {
	ret := make(map[int64]int64, len(workflows))
	for i := range workflows {
		ret[workflows[i].ID] = workflows[i].CurrentStableVersionID
	}
	return ret
}

func enterpriseAIWorkflowAdoption(tenantID int64, stableVersionByWorkflow map[int64]int64, workflowIDs []int64) (map[int64]int64, map[int64]int64) {
	adoption := make(map[int64]int64)
	stableAdoption := make(map[int64]int64)
	if tenantID <= 0 || len(stableVersionByWorkflow) == 0 {
		return adoption, stableAdoption
	}
	cnd := sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Gt("product_id", 0).
		NotEq("status", enums.StatusDeleted)
	if len(workflowIDs) > 0 {
		cnd.In("workflow_id", workflowIDs)
	}
	agents := services.AIAgentService.Find(cnd)
	for i := range agents {
		workflowID := agents[i].WorkflowID
		stableVersionID, ok := stableVersionByWorkflow[workflowID]
		if workflowID <= 0 || !ok {
			continue
		}
		adoption[workflowID]++
		if agents[i].WorkflowVersionID == stableVersionID {
			stableAdoption[workflowID]++
		}
	}
	return adoption, stableAdoption
}

func enterpriseAIWorkflowNeedsAttention(item models.AIWorkflow, adopted, stableAdopted, sourceStableVersionID int64) bool {
	if item.CurrentStableVersionID <= 0 {
		return true
	}
	if item.Scope == models.AIWorkflowScopeTenant && item.SourceVersionID > 0 && sourceStableVersionID > 0 && item.SourceVersionID != sourceStableVersionID {
		return true
	}
	if adopted > stableAdopted {
		return true
	}
	return adopted == 0
}

func AIWorkflowGet(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIWorkflowView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	item := services.AIWorkflowService.GetForOperator(id, operator)
	if !services.AIWorkflowService.IsReusableTemplateForOperator(item, operator) {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0002"))
		return
	}
	if !enterpriseAIWorkflowCanUseTemplate(ctx, operator, item) {
		return
	}
	httpx.WriteJSON(ctx, builders.BuildAIWorkflow(item))
}

func AIWorkflowVersionList(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIWorkflowView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	workflowID, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	workflow := services.AIWorkflowService.GetForOperator(workflowID, operator)
	if !services.AIWorkflowService.IsReusableTemplateForOperator(workflow, operator) {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0002"))
		return
	}
	if !enterpriseAIWorkflowCanUseTemplate(ctx, operator, workflow) {
		return
	}
	cnd := params.NewPagedSqlCnd(ctx,
		params.QueryFilter{ParamName: "id"},
		params.QueryFilter{ParamName: "status"},
		params.QueryFilter{ParamName: "releaseChannel", ColumnName: "release_channel"},
	).Eq("workflow_id", workflowID).NotEq("status", enums.StatusDeleted).Desc("version").Desc("id")
	list, paging := services.AIWorkflowService.FindVersionPageByCnd(cnd)
	httpx.WriteJSON(ctx, &web.PageResult{Results: builders.BuildAIWorkflowVersionList(list), Page: paging})
}

func AIWorkflowTemplateCreate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIWorkflowUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if !services.TenantCapabilityService.AIEnabled(operator.TenantID) {
		httpx.WriteJSON(ctx, errorsx.Forbidden("tenant AI capability is disabled"))
		return
	}
	req := request.CreateAIWorkflowTemplateRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.AIWorkflowService.CreateReusableTemplateFromVersion(req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildAIWorkflow(item))
}

func AIWorkflowDraftUpdate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIWorkflowUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	current := services.AIWorkflowService.GetForOperator(id, operator)
	if !services.AIWorkflowService.IsReusableTemplateForOperator(current, operator) {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0002"))
		return
	}
	if !enterpriseAIWorkflowCanUseTemplate(ctx, operator, current) {
		return
	}
	req := request.UpdateAIWorkflowTemplateDraftRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.AIWorkflowService.UpdateReusableTemplateDraft(id, req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildAIWorkflow(item))
}

func AIWorkflowDelete(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIWorkflowUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	workflow := services.AIWorkflowService.GetForOperator(id, operator)
	if !services.AIWorkflowService.IsReusableTemplateForOperator(workflow, operator) || workflow.Scope != models.AIWorkflowScopeTenant {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0002"))
		return
	}
	if err := services.AIWorkflowService.DeleteWorkflow(id, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func AIWorkflowPublish(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIWorkflowPublish)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	current := services.AIWorkflowService.GetForOperator(id, operator)
	if !services.AIWorkflowService.IsReusableTemplateForOperator(current, operator) {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0002"))
		return
	}
	if !enterpriseAIWorkflowCanUseTemplate(ctx, operator, current) {
		return
	}
	req := request.UpdateAIWorkflowTemplateDraftRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if _, err := services.AIWorkflowService.UpdateReusableTemplateDraft(id, req, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.AIWorkflowService.PublishWorkflow(request.PublishAIWorkflowRequest{
		WorkflowID: id,
		Definition: req.Definition,
	}, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildAIWorkflowVersion(item))
}

func AIWorkflowVersionRollback(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIWorkflowPublish)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	workflowID, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	workflow := services.AIWorkflowService.GetForOperator(workflowID, operator)
	if !services.AIWorkflowService.IsReusableTemplateForOperator(workflow, operator) {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0002"))
		return
	}
	if !enterpriseAIWorkflowCanUseTemplate(ctx, operator, workflow) {
		return
	}
	versionID, ok := httpx.GetPathInt64(ctx, "versionId")
	if !ok {
		return
	}
	item, err := services.AIWorkflowService.RollbackWorkflowVersion(workflowID, versionID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildAIWorkflowVersion(item))
}

func AIWorkflowTestRun(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIWorkflowUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	workflow := services.AIWorkflowService.GetForOperator(id, operator)
	if !services.AIWorkflowService.IsReusableTemplateForOperator(workflow, operator) {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0002"))
		return
	}
	if !enterpriseAIWorkflowCanUseTemplate(ctx, operator, workflow) {
		return
	}
	req := request.TestAIWorkflowRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	ret, err := services.AIWorkflowService.TestRun(ctx.Request.Context(), id, req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, ret)
}

func enterpriseAIWorkflowCanUseTemplate(ctx *gin.Context, operator *dto.AuthPrincipal, workflow *models.AIWorkflow) bool {
	if services.TenantCapabilityService.CanUseWorkflowTemplate(operator.TenantID, workflow) {
		return true
	}
	httpx.WriteJSON(ctx, errorsx.Forbidden("tenant AI capability is disabled"))
	return false
}

func AIAgentWorkflowBindingUpdate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	agentID, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	req := request.BindAIAgentWorkflowVersionRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	version := services.AIWorkflowService.GetVersionForOperator(req.WorkflowVersionID, operator)
	if version == nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("工作流版本不存在或无权访问"))
		return
	}
	workflow := services.AIWorkflowService.GetForOperator(version.WorkflowID, operator)
	if !services.AIWorkflowService.IsReusableTemplateForOperator(workflow, operator) {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("当前版本不属于可复用工作流模板"))
		return
	}
	if err := services.AIWorkflowService.BindAgentWorkflowVersion(agentID, req.WorkflowVersionID, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}
