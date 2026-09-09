package enterprise

import (
	"encoding/json"
	"errors"
	"io"
	"strconv"

	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/middleware"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/sqls"
)

func IndustrySolutionPackList(ctx *gin.Context) {
	principal, tenantID, ok := requireIndustrySolutionPackPrincipal(ctx)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "20"))
	result, err := services.DefaultIndustrySolutionPackService(sqls.DB()).ListPacks(
		ctx.Request.Context(), tenantID, ctx.Query("status"), ctx.Query("industry_code"), page, pageSize, principal,
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildIndustrySolutionPackList(result.Items, result.Total, result.Page, result.PageSize))
}

func IndustrySolutionPackImport(ctx *gin.Context) {
	principal, tenantID, ok := requireIndustrySolutionPackPrincipal(ctx)
	if !ok {
		return
	}
	var req dto.IndustrySolutionPackManifestRequest
	if err := decodeIndustrySolutionPackRequest(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.DefaultIndustrySolutionPackService(sqls.DB()).ImportDraftManifest(ctx.Request.Context(), tenantID, req, principal)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildIndustrySolutionPackManifest(&result.Pack, result.Resources, result.Reused))
}

func IndustrySolutionPackDraftUpdate(ctx *gin.Context) {
	principal, tenantID, ok := requireIndustrySolutionPackPrincipal(ctx)
	if !ok {
		return
	}
	packID, ok := parseIndustrySolutionPackID(ctx, "packId")
	if !ok {
		return
	}
	var req dto.IndustrySolutionPackManifestRequest
	if err := decodeIndustrySolutionPackRequest(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.DefaultIndustrySolutionPackService(sqls.DB()).UpdateDraftManifest(ctx.Request.Context(), tenantID, packID, req, principal)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildIndustrySolutionPackManifest(&result.Pack, result.Resources, result.Reused))
}

func IndustrySolutionPackValidate(ctx *gin.Context) {
	principal, tenantID, ok := requireIndustrySolutionPackPrincipal(ctx)
	if !ok {
		return
	}
	packID, ok := parseIndustrySolutionPackID(ctx, "packId")
	if !ok {
		return
	}
	result, err := services.DefaultIndustrySolutionPackService(sqls.DB()).Validate(ctx.Request.Context(), tenantID, packID, principal)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildIndustrySolutionPackValidation(*result))
}

func IndustrySolutionPackPublish(ctx *gin.Context) {
	principal, tenantID, ok := requireIndustrySolutionPackPrincipal(ctx)
	if !ok {
		return
	}
	packID, ok := parseIndustrySolutionPackID(ctx, "packId")
	if !ok {
		return
	}
	result, err := services.DefaultIndustrySolutionPackService(sqls.DB()).Publish(ctx.Request.Context(), tenantID, packID, principal)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildIndustrySolutionPackManifest(&result.Pack, result.Resources, result.Reused))
}

func IndustrySolutionPackDryRun(ctx *gin.Context) {
	executeIndustrySolutionPack(ctx, true)
}

func IndustrySolutionPackApply(ctx *gin.Context) {
	executeIndustrySolutionPack(ctx, false)
}

func IndustrySolutionPackApplicationGet(ctx *gin.Context) {
	principal, tenantID, ok := requireIndustrySolutionPackPrincipal(ctx)
	if !ok {
		return
	}
	applicationID, ok := parseIndustrySolutionPackID(ctx, "applicationId")
	if !ok {
		return
	}
	result, err := services.DefaultIndustrySolutionPackService(sqls.DB()).GetApplicationDetail(
		ctx.Request.Context(), tenantID, applicationID, principal,
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildIndustrySolutionPackExecution(result))
}

func executeIndustrySolutionPack(ctx *gin.Context, dryRun bool) {
	principal, tenantID, ok := requireIndustrySolutionPackPrincipal(ctx)
	if !ok {
		return
	}
	packID, ok := parseIndustrySolutionPackID(ctx, "packId")
	if !ok {
		return
	}
	var req dto.IndustrySolutionPackApplyDTORequest
	if err := decodeIndustrySolutionPackRequest(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	serviceRequest := services.IndustrySolutionPackApplyRequest{
		TenantID: tenantID, PackID: packID, TargetProductID: req.TargetProductID,
		TargetProductModelID: req.TargetProductModelID, TargetContextJSON: string(req.TargetContext),
		ConflictStrategy: req.ConflictStrategy, IdempotencyKey: req.IdempotencyKey,
	}
	service := services.DefaultIndustrySolutionPackService(sqls.DB())
	var result *services.IndustrySolutionPackExecutionResult
	var err error
	if dryRun {
		result, err = service.DryRun(ctx.Request.Context(), serviceRequest, principal)
	} else {
		result, err = service.Apply(ctx.Request.Context(), serviceRequest, principal)
	}
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildIndustrySolutionPackExecution(result))
}

func requireIndustrySolutionPackPrincipal(ctx *gin.Context) (*dto.AuthPrincipal, int64, bool) {
	principal := middleware.GetAuthPrincipal(ctx)
	if principal == nil {
		httpx.WriteJSON(ctx, errorsx.UnauthorizedI18n("error.auth.expired"))
		return nil, 0, false
	}
	if !principal.IsEnterprise() {
		httpx.WriteJSON(ctx, errorsx.Forbidden("industry solution packs require an enterprise principal"))
		return nil, 0, false
	}
	tenantID := principal.EffectiveTenantID()
	if tenantID <= 0 {
		httpx.WriteJSON(ctx, errorsx.Forbidden("tenant scope is required"))
		return nil, 0, false
	}
	return principal, tenantID, true
}

func parseIndustrySolutionPackID(ctx *gin.Context, name string) (int64, bool) {
	value, err := strconv.ParseInt(ctx.Param(name), 10, 64)
	if err != nil || value <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam(name+" is invalid"))
		return 0, false
	}
	return value, true
}

func decodeIndustrySolutionPackRequest(ctx *gin.Context, target any) error {
	decoder := json.NewDecoder(ctx.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errorsx.InvalidParam("request body must contain one JSON object")
	}
	return nil
}
