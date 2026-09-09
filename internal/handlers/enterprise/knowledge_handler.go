package enterprise

import (
	"strconv"
	"strings"

	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

func TenantKnowledgeDocuments(ctx *gin.Context) {
	tenantID, knowledgeBaseID, ok := resolveEnterpriseKnowledgeBaseRoute(ctx)
	if !ok {
		return
	}
	page, pageSize := knowledgeDocumentPageQuery(ctx)
	items, err := services.KnowledgeDocumentService.ListEnterpriseTenantDocuments(
		tenantID,
		knowledgeBaseID,
		page,
		pageSize,
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, items)
}

func TenantKnowledgeDocumentUpload(ctx *gin.Context) {
	tenantID, knowledgeBaseID, ok := resolveEnterpriseKnowledgeBaseRoute(ctx)
	if !ok {
		return
	}
	header, err := ctx.FormFile("file")
	if err != nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0323"))
		return
	}
	item, err := services.KnowledgeDocumentService.UploadEnterpriseTenantDocument(
		tenantID,
		knowledgeBaseID,
		header,
		enterpriseActionOperator(ctx, tenantID),
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, item)
}

func TenantKnowledgeDocumentReprocess(ctx *gin.Context) {
	tenantID, knowledgeBaseID, documentID, ok := resolveEnterpriseKnowledgeBaseDocumentRoute(ctx)
	if !ok {
		return
	}
	item, err := services.KnowledgeDocumentService.ReprocessEnterpriseTenantDocument(
		tenantID,
		knowledgeBaseID,
		documentID,
		enterpriseActionOperator(ctx, tenantID),
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, item)
}

func TenantKnowledgeDocumentDelete(ctx *gin.Context) {
	tenantID, knowledgeBaseID, documentID, ok := resolveEnterpriseKnowledgeBaseDocumentRoute(ctx)
	if !ok {
		return
	}
	if err := services.KnowledgeDocumentService.DeleteEnterpriseTenantDocument(
		tenantID,
		knowledgeBaseID,
		documentID,
		enterpriseActionOperator(ctx, tenantID),
	); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, gin.H{"success": true})
}

func KnowledgeUploadQuota(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	operator := enterpriseActionOperator(ctx, tenantID)
	productIDRaw := strings.TrimSpace(ctx.Query("product_id"))
	if productIDRaw == "" {
		productIDRaw = strings.TrimSpace(ctx.Query("productId"))
	}
	productID, _ := strconv.ParseInt(productIDRaw, 10, 64)
	quota, err := services.TenantCommercialService.DescribeKnowledgeDocumentUploadQuota(tenantID, productID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, quota)
}

func KnowledgeEntryList(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("page_size", ctx.DefaultQuery("limit", "20")))
	productID, _ := strconv.ParseInt(firstNonEmpty(ctx.Query("product_id"), ctx.Query("productId")), 10, 64)
	filter := parseEnterpriseFilter(ctx.Query("filter"))
	query := services.EnterpriseKnowledgeQuery{
		Page:      page,
		PageSize:  pageSize,
		Search:    firstNonEmpty(ctx.Query("search"), strings.TrimPrefix(filter["title"], "~")),
		Category:  firstNonEmpty(ctx.Query("category"), filter["category"]),
		Status:    firstNonEmpty(ctx.Query("status"), filter["status"]),
		Product:   firstNonEmpty(ctx.Query("product"), filter["product"]),
		ProductID: productID,
	}
	result, err := services.EnterpriseKnowledgeService.ListEntries(tenantID, query)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func KnowledgeStats(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	result, err := services.EnterpriseKnowledgeService.Stats(tenantID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func KnowledgeEntryGet(ctx *gin.Context) {
	tenantID, entryID, ok := resolveEnterpriseKnowledgeEntryRoute(ctx)
	if !ok {
		return
	}
	result, err := services.EnterpriseKnowledgeService.GetEntry(tenantID, entryID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func KnowledgeEntryCreate(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	var req dto.EnterpriseKnowledgeMutationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid request body"))
		return
	}
	result, err := services.EnterpriseKnowledgeService.CreateEntry(tenantID, req, enterpriseActionOperator(ctx, tenantID))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func KnowledgeEntryUpdate(ctx *gin.Context) {
	tenantID, entryID, ok := resolveEnterpriseKnowledgeEntryRoute(ctx)
	if !ok {
		return
	}
	var req dto.EnterpriseKnowledgeMutationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid request body"))
		return
	}
	result, err := services.EnterpriseKnowledgeService.UpdateEntry(tenantID, entryID, req, enterpriseActionOperator(ctx, tenantID))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func KnowledgeEntrySubmit(ctx *gin.Context) {
	knowledgeEntryStatusAction(ctx, "review")
}

func KnowledgeEntryPublish(ctx *gin.Context) {
	knowledgeEntryStatusAction(ctx, "published")
}

func KnowledgeEntryDeprecate(ctx *gin.Context) {
	knowledgeEntryStatusAction(ctx, "deprecated")
}

func KnowledgeEntryVersions(ctx *gin.Context) {
	tenantID, entryID, ok := resolveEnterpriseKnowledgeEntryRoute(ctx)
	if !ok {
		return
	}
	result, err := services.EnterpriseKnowledgeService.Versions(tenantID, entryID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func KnowledgeEntryQuality(ctx *gin.Context) {
	tenantID, entryID, ok := resolveEnterpriseKnowledgeEntryRoute(ctx)
	if !ok {
		return
	}
	result, err := services.EnterpriseKnowledgeService.Quality(tenantID, entryID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func KnowledgeEntryTranslations(ctx *gin.Context) {
	tenantID, entryID, ok := resolveEnterpriseKnowledgeEntryRoute(ctx)
	if !ok {
		return
	}
	result, err := services.EnterpriseKnowledgeService.Translations(tenantID, entryID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func knowledgeEntryStatusAction(ctx *gin.Context, nextStatus string) {
	tenantID, entryID, ok := resolveEnterpriseKnowledgeEntryRoute(ctx)
	if !ok {
		return
	}
	result, err := services.EnterpriseKnowledgeService.UpdateEntryStatus(tenantID, entryID, nextStatus, enterpriseActionOperator(ctx, tenantID))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func resolveEnterpriseKnowledgeEntryRoute(ctx *gin.Context) (int64, int64, bool) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return 0, 0, false
	}
	entryID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || entryID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid knowledge entry id"))
		return 0, 0, false
	}
	return tenantID, entryID, true
}

func resolveEnterpriseKnowledgeBaseRoute(ctx *gin.Context) (int64, int64, bool) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return 0, 0, false
	}
	knowledgeBaseID, err := strconv.ParseInt(ctx.Param("knowledgeBaseId"), 10, 64)
	if err != nil || knowledgeBaseID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid knowledge base id"))
		return 0, 0, false
	}
	return tenantID, knowledgeBaseID, true
}

func resolveEnterpriseKnowledgeBaseDocumentRoute(ctx *gin.Context) (int64, int64, int64, bool) {
	tenantID, knowledgeBaseID, ok := resolveEnterpriseKnowledgeBaseRoute(ctx)
	if !ok {
		return 0, 0, 0, false
	}
	documentID, err := strconv.ParseInt(ctx.Param("documentId"), 10, 64)
	if err != nil || documentID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid knowledge document id"))
		return 0, 0, 0, false
	}
	return tenantID, knowledgeBaseID, documentID, true
}
