package dashboard

import (
	"context"

	"remotehelpdesk/internal/ai"
	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/httpx/params"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

func KnowledgeIndexGenerationAnyList(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeBaseView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	limit := 20
	if value, ok := params.GetInt64(ctx, "limit"); ok && value > 0 && value <= 100 {
		limit = int(value)
	}
	items := services.KnowledgeIndexGenerationService.ListGenerations(limit)
	results := make([]response.KnowledgeIndexGenerationResponse, 0, len(items))
	for i := range items {
		results = append(results, builders.BuildKnowledgeIndexGeneration(&items[i]))
	}
	httpx.WriteJSON(ctx, results)
}

func KnowledgeIndexGenerationGetActive(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeBaseView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	state, err := services.KnowledgeIndexGenerationService.ResolveAliasState(context.Background())
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	resp := response.KnowledgeIndexGenerationActiveResponse{
		Alias:                 state.Alias,
		AliasTargetCollection: state.AliasTargetCollection,
		AliasDrift:            state.AliasDrift,
		CollectionPointCount:  state.CollectionPointCount,
		DatabaseChunkCount:    state.DatabaseChunkCount,
		VectorPointDrift:      state.VectorPointDrift,
		VectorPointDriftCause: state.VectorPointDriftCause,
	}
	if state.ActiveGeneration != nil {
		item := builders.BuildKnowledgeIndexGeneration(state.ActiveGeneration)
		resp.ActiveGeneration = &item
	}
	httpx.WriteJSON(ctx, resp)
}

func KnowledgeIndexGenerationPostEnsureActive(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeBaseUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	item, err := services.KnowledgeIndexGenerationService.EnsureActiveGeneration(context.Background(), operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildKnowledgeIndexGeneration(item))
}

func KnowledgeIndexGenerationPostPrepare(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeBaseUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := struct {
		TenantID int64 `json:"tenantId"`
	}{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	runCtx := ai.WithCapabilityScope(ctx.Request.Context(), ai.CapabilityScope{TenantID: req.TenantID})
	item, err := services.KnowledgeIndexGenerationService.PrepareCompatibleGeneration(runCtx, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildKnowledgeIndexGeneration(item))
}

func KnowledgeIndexGenerationPostActivate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeBaseUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	req := struct {
		GenerationID int64 `json:"generationId"`
	}{}
	if err := params.ReadJSON(ctx, &req); err != nil || req.GenerationID <= 0 {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0066"))
		return
	}
	if err := services.KnowledgeIndexGenerationService.ActivateGeneration(context.Background(), req.GenerationID, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item := services.KnowledgeIndexGenerationService.GetGeneration(req.GenerationID)
	httpx.WriteJSON(ctx, builders.BuildKnowledgeIndexGeneration(item))
}

func KnowledgeIndexGenerationPostBackfill(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeBaseUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	req := struct {
		GenerationID int64 `json:"generationId"`
		Limit        int   `json:"limit"`
	}{}
	if err := params.ReadJSON(ctx, &req); err != nil || req.GenerationID <= 0 {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0066"))
		return
	}
	count, err := services.KnowledgeIndexGenerationService.EnqueueBackfillForGeneration(context.Background(), req.GenerationID, operator, req.Limit)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, gin.H{
		"generationId":  req.GenerationID,
		"enqueuedCount": count,
	})
}
