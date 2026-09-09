package enterprise

import (
	"strings"

	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/httpx/params"
	"remotehelpdesk/internal/pkg/i18nx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/sqls"
	"github.com/mlogclub/simple/web"
)

func EnterpriseAIAgentSummary(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := requireTenantInt(ctx)
	if !ok {
		return
	}

	productAgents := services.AIAgentService.Find(enterpriseProductAIAgentCnd(tenantID).Asc("product_id").Asc("id"))
	productIDs := make([]int64, 0, len(productAgents))
	seenProducts := make(map[int64]struct{}, len(productAgents))
	for i := range productAgents {
		productID := productAgents[i].ProductID
		if productID <= 0 {
			continue
		}
		if _, ok := seenProducts[productID]; ok {
			continue
		}
		seenProducts[productID] = struct{}{}
		productIDs = append(productIDs, productID)
	}
	productCount, err := services.ProductCenterService.CountEnterpriseProductsForOperator(tenantID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	productTotal := productCount.Total
	missingProducts := productTotal - int64(len(productIDs))
	if missingProducts < 0 {
		missingProducts = 0
	}

	ret := response.EnterpriseAIAgentSummaryResponse{
		Total:                  services.AIAgentService.Count(enterpriseAIAgentBaseCnd(tenantID)),
		Active:                 services.AIAgentService.Count(enterpriseAIAgentBaseCnd(tenantID).Eq("status", enums.StatusOk).Where("active_release_id > ?", 0)),
		NotDeployed:            services.AIAgentService.Count(enterpriseAIAgentBaseCnd(tenantID).Where("active_release_id <= ?", 0)),
		ProductTotal:           productTotal,
		ProductAgentTotal:      int64(len(productIDs)),
		MissingProductAgents:   missingProducts,
		TenantDefaultReady:     services.AIAgentService.Count(enterpriseTenantDefaultAIAgentCnd(tenantID)) > 0,
		ProductAgentProductIDs: productIDs,
	}
	httpx.WriteJSON(ctx, ret)
}

func EnterpriseAIAgentList(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := requireTenantInt(ctx)
	if !ok {
		return
	}
	cnd := params.NewPagedSqlCnd(ctx,
		params.QueryFilter{ParamName: "search", ColumnName: "name", Op: params.Like},
		params.QueryFilter{ParamName: "productId"},
		params.QueryFilter{ParamName: "productScoped", ColumnName: "product_id", Op: params.Gt},
		params.QueryFilter{ParamName: "workflowId"},
		params.QueryFilter{ParamName: "reviewStatus", ColumnName: "review_status"},
		params.QueryFilter{ParamName: "source"},
	).Eq("tenant_id", tenantID).
		NotEq("status", enums.StatusDeleted).
		Where("(product_id > ? OR source = ?)", 0, services.TenantDefaultAIAgentSource).
		Asc("sort_no").
		Desc("id")

	switch strings.TrimSpace(ctx.Query("runtimeStatus")) {
	case "running":
		cnd.Eq("status", enums.StatusOk).Where("active_release_id > ?", 0)
	case "not_deployed":
		cnd.Where("active_release_id <= ?", 0)
	case "disabled":
		cnd.NotEq("status", enums.StatusOk).Where("active_release_id > ?", 0)
	}

	list, paging := services.AIAgentService.FindPageByCnd(cnd)
	httpx.WriteJSON(ctx, &web.PageResult{
		Results: builders.BuildAIAgentListWithLocale(list, i18nx.Locale(ctx)),
		Page:    paging,
	})
}

func enterpriseAIAgentBaseCnd(tenantID int64) *sqls.Cnd {
	return sqls.NewCnd().
		Eq("tenant_id", tenantID).
		NotEq("status", enums.StatusDeleted).
		Where("(product_id > ? OR source = ?)", 0, services.TenantDefaultAIAgentSource)
}

func enterpriseProductAIAgentCnd(tenantID int64) *sqls.Cnd {
	return sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Gt("product_id", 0).
		NotEq("status", enums.StatusDeleted)
}

func enterpriseTenantDefaultAIAgentCnd(tenantID int64) *sqls.Cnd {
	return sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", 0).
		Eq("source", services.TenantDefaultAIAgentSource).
		NotEq("status", enums.StatusDeleted)
}

func TenantDefaultAIAgentEnsure(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentCreate); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := requireTenantInt(ctx)
	if !ok {
		return
	}
	if !services.TenantCapabilityService.AIEnabled(tenantID) {
		httpx.WriteJSON(ctx, errorsx.Forbidden("tenant AI capability is disabled"))
		return
	}
	item, err := services.TenantDefaultAIAgentService.EnsureDB(
		sqls.DB(),
		tenantID,
		enterpriseActionOperator(ctx, tenantID),
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if item == nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("tenant default AI agent is unavailable"))
		return
	}
	if err := services.TenantDefaultAIAgentService.EnsureDefaultCredential(tenantID, enterpriseActionOperator(ctx, tenantID)); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, dto.EnsureProductAIAgentDTO{ID: item.ID, ProductID: 0})
}

func ProductAIAgentEnsure(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentCreate); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := requireTenantInt(ctx)
	if !ok {
		return
	}
	req := request.EnsureProductAIAgentRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.ProductAIAgentService.EnsureProductCustomerAgent(
		tenantID,
		req.ProductID,
		enterpriseActionOperator(ctx, tenantID),
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, dto.EnsureProductAIAgentDTO{ID: item.ID, ProductID: item.ProductID})
}

func ProductAIAgentsProvision(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentUpdate); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, ok := requireTenantInt(ctx)
	if !ok {
		return
	}
	result, err := services.ProductAIAgentService.ProvisionTenantProductAgents(
		tenantID,
		enterpriseActionOperator(ctx, tenantID),
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func ProductAIAgentSubmitReview(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentUpdate); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, id, ok := resolveProductAIAgentRoute(ctx)
	if !ok {
		return
	}
	if err := services.AIAgentService.SubmitReview(id, enterpriseActionOperator(ctx, tenantID)); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func ProductAIAgentApprove(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentUpdate); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, id, ok := resolveProductAIAgentRoute(ctx)
	if !ok {
		return
	}
	req := dto.EnterpriseAIAgentReviewRequest{}
	if err := readOptionalJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.AIAgentService.Review(id, true, req.Comment, enterpriseActionOperator(ctx, tenantID)); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func ProductAIAgentReject(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentUpdate); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	tenantID, id, ok := resolveProductAIAgentRoute(ctx)
	if !ok {
		return
	}
	req := dto.EnterpriseAIAgentReviewRequest{}
	if err := readOptionalJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.AIAgentService.Review(id, false, req.Comment, enterpriseActionOperator(ctx, tenantID)); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func resolveProductAIAgentRoute(ctx *gin.Context) (int64, int64, bool) {
	tenantID, ok := requireTenantInt(ctx)
	if !ok {
		return 0, 0, false
	}
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return 0, 0, false
	}
	return tenantID, id, true
}
