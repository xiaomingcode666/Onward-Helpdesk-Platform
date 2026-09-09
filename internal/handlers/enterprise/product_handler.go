package enterprise

import (
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"remotehelpdesk/internal/middleware"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

// 企业端产品中心 Handler。
// 分层约束（§11.4）：Handler 只负责参数解析、鉴权、调用 Service 和 httpx.WriteJSON，
// 不直接访问 Repository / GORM / sqls；响应 DTO 定义在 internal/pkg/dto，
// 模型到 DTO 的映射在 internal/builders。

// ProductList 获取企业产品列表
// GET /api/enterprise/v1/products
func ProductList(ctx *gin.Context) {
	tenantIDInt, ok := requireTenantInt(ctx)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))
	if pageSizeStr := ctx.Query("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			limit = ps
		}
	}
	items, err := services.ProductCenterService.ListEnterpriseProductsForOperator(
		tenantIDInt, page, limit, enterpriseActionOperator(ctx, tenantIDInt),
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, items)
}

// ProductCount 获取当前账号可见的企业产品数量。
// GET /api/enterprise/v1/product-count
func ProductCount(ctx *gin.Context) {
	tenantIDInt, ok := requireTenantInt(ctx)
	if !ok {
		return
	}
	result, err := services.ProductCenterService.CountEnterpriseProductsForOperator(
		tenantIDInt, enterpriseActionOperator(ctx, tenantIDInt),
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

// ProductGet 获取单个产品详情
// GET /api/enterprise/v1/products/:id
func ProductGet(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	item, err := services.ProductCenterService.GetEnterpriseProductDetail(tenantIDInt, productID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, item)
}

// ProductCreate 创建产品
// POST /api/enterprise/v1/products
func ProductCreate(ctx *gin.Context) {
	tenantIDInt, ok := requireTenantInt(ctx)
	if !ok {
		return
	}

	var req dto.EnterpriseProductCreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if req.OwnerMemberID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("product owner is required"))
		return
	}

	operator := enterpriseActionOperator(ctx, tenantIDInt)
	item, err := services.ProductService.CreateProduct(request.CreateProductRequest{
		TenantID:      tenantIDInt,
		ProductLineID: req.ProductLineID,
		ProductLine:   req.ProductLine,
		Code:          req.Code,
		Name:          req.Name,
		Description:   req.Description,
		Category:      req.Category,
		OwnerMemberID: req.OwnerMemberID,
		DefaultLocale: req.DefaultLocale,
	}, operator)
	if err != nil {
		slog.Error("failed to create product", "error", err)
		httpx.WriteJSON(ctx, err)
		return
	}
	if services.TenantCapabilityService.AIEnabled(tenantIDInt) {
		if _, err := services.ProductResourceService.EnqueueProductResourceProvisioning(ctx.Request.Context(), tenantIDInt, item.ID, req.AIQuotaLimit, operator); err != nil {
			// 产品主数据已经创建成功；资源开通任务可在产品编辑弹窗或 AI 服务中心幂等重试。
			slog.Error("failed to enqueue product resource provisioning", "product_id", item.ID, "error", err)
		}
	}

	result, err := services.ProductCenterService.BuildProductListItem(tenantIDInt, item.ID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

// ProductUpdate 更新产品（PATCH 语义：仅更新请求中提供的字段）
// PATCH /api/enterprise/v1/products/:id
func ProductUpdate(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}

	var req dto.EnterpriseProductUpdateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	operator := enterpriseActionOperator(ctx, tenantIDInt)
	if err := services.RequireEnterpriseProductEdit(tenantIDInt, productID, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	updateReq := request.UpdateProductRequest{ID: productID}
	updateReq.TenantID = tenantIDInt
	if req.Code != nil {
		updateReq.Code = *req.Code
	}
	if req.Name != nil {
		updateReq.Name = *req.Name
	}
	if req.Description != nil {
		updateReq.Description = *req.Description
		updateReq.DescriptionProvided = true
	}
	if req.Category != nil {
		updateReq.Category = *req.Category
	}
	if req.ProductLine != nil {
		updateReq.ProductLine = *req.ProductLine
	}
	if req.ProductLineID != nil {
		updateReq.ProductLineID = *req.ProductLineID
	}
	if req.OwnerMemberID != nil {
		if *req.OwnerMemberID <= 0 {
			httpx.WriteJSON(ctx, errorsx.InvalidParam("product owner is required"))
			return
		}
		updateReq.OwnerMemberID = *req.OwnerMemberID
	}
	if req.DefaultLocale != nil {
		updateReq.DefaultLocale = *req.DefaultLocale
	}
	if req.Status != nil {
		updateReq.StatusText = *req.Status
	}

	if err := services.ProductService.UpdateProduct(updateReq, operator); err != nil {
		slog.Error("failed to update product", "id", productID, "error", err)
		httpx.WriteJSON(ctx, err)
		return
	}

	result, err := services.ProductCenterService.BuildProductListItem(tenantIDInt, productID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

// ProductProfile 获取产品聚合档案
// GET /api/enterprise/v1/products/:id/profile
func ProductProfile(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	result, err := services.ProductCenterService.GetEnterpriseProductProfile(tenantIDInt, productID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

// ProductResources 获取产品 AI Key、知识 Dataset 与客服机器人配置状态。
// GET /api/enterprise/v1/products/:id/resources
func ProductResources(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	result, err := services.ProductResourceService.GetProductResources(ctx.Request.Context(), tenantIDInt, productID, enterpriseActionOperator(ctx, tenantIDInt))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

// ProductResourcesRetry 幂等补齐产品资源。
// POST /api/enterprise/v1/products/:id/resources/_retry
func ProductResourcesRetry(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	result, err := services.ProductResourceService.RetryProductResourceProvisioning(ctx.Request.Context(), tenantIDInt, productID, enterpriseActionOperator(ctx, tenantIDInt))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

// ProductAIQuotaUpdate 更新产品作用域 Sub2API Key 的额度。
// PATCH /api/enterprise/v1/products/:id/ai-quota
func ProductAIQuotaUpdate(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	var req dto.EnterpriseProductAIQuotaUpdateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if req.QuotaLimit == nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("quota_limit is required"))
		return
	}
	result, err := services.ProductResourceService.UpdateProductAIQuota(ctx.Request.Context(), tenantIDInt, productID, *req.QuotaLimit, enterpriseActionOperator(ctx, tenantIDInt))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

// ProductAIQuotaReset 重置产品作用域 Sub2API Key 的已用额度。
// POST /api/enterprise/v1/products/:id/ai-quota/_reset
func ProductAIQuotaReset(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	result, err := services.ProductResourceService.ResetProductAIQuota(ctx.Request.Context(), tenantIDInt, productID, enterpriseActionOperator(ctx, tenantIDInt))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

// ProductKnowledgeBaseCreate 为产品创建并绑定默认知识库
// POST /api/enterprise/v1/products/:id/knowledge-base
func ProductKnowledgeBaseCreate(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}

	var req dto.EnterpriseProductKnowledgeBaseCreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	result, err := services.ProductCenterService.CreateProductKnowledgeBase(
		tenantIDInt,
		productID,
		req,
		enterpriseActionOperator(ctx, tenantIDInt),
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	httpx.WriteJSON(ctx, result)
}

// ---- 型号主数据 ----

// ProductModels 获取产品型号列表
// GET /api/enterprise/v1/products/:id/models
func ProductModels(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	items, err := services.ProductCenterService.ListEnterpriseProductModels(tenantIDInt, productID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, items)
}

// ProductModelCreate 创建产品型号
// POST /api/enterprise/v1/products/:id/models
func ProductModelCreate(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	var req dto.EnterpriseProductModelRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.ProductModelService.CreateProductModel(request.CreateProductModelRequest{
		TenantID:        tenantIDInt,
		ProductID:       productID,
		ModelCode:       req.ModelCode,
		Name:            req.Name,
		VersionPolicy:   req.VersionPolicy,
		RegionScopeJSON: req.RegionScopeJSON,
	}, enterpriseActionOperator(ctx, tenantIDInt))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	models, err := services.ProductCenterService.ListEnterpriseProductModels(tenantIDInt, productID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, findByID(models, item.ID))
}

// ProductModelUpdate 更新产品型号
// PATCH /api/enterprise/v1/products/:id/models/:modelId
func ProductModelUpdate(ctx *gin.Context) {
	tenantIDInt, productID, modelID, ok := resolveTenantProductChildRoute(ctx, "modelId")
	if !ok {
		return
	}
	var req dto.EnterpriseProductModelUpdateRequest
	if err := readOptionalJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	existing := services.ProductModelService.Get(modelID)
	if existing == nil || existing.TenantID != tenantIDInt || existing.ProductID != productID {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("product model not found"))
		return
	}
	if err := services.ProductModelService.UpdateProductModel(request.UpdateProductModelRequest{
		ID: modelID,
		CreateProductModelRequest: request.CreateProductModelRequest{
			TenantID:        tenantIDInt,
			ProductID:       productID,
			ModelCode:       chooseOptionalString(req.ModelCode, existing.ModelCode),
			Name:            chooseOptionalString(req.Name, existing.Name),
			VersionPolicy:   chooseOptionalString(req.VersionPolicy, existing.VersionPolicy),
			RegionScopeJSON: chooseOptionalString(req.RegionScopeJSON, existing.RegionScopeJSON),
		},
	}, enterpriseActionOperator(ctx, tenantIDInt)); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	models, err := services.ProductCenterService.ListEnterpriseProductModels(tenantIDInt, productID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, findByID(models, modelID))
}

// ProductModelEnable 启用型号
// POST /api/enterprise/v1/products/:id/models/:modelId/_enable
func ProductModelEnable(ctx *gin.Context) {
	tenantIDInt, productID, modelID, ok := resolveTenantProductChildRoute(ctx, "modelId")
	if !ok {
		return
	}
	if err := services.ProductModelService.EnableModel(tenantIDInt, productID, modelID, enterpriseActionOperator(ctx, tenantIDInt)); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, gin.H{"success": true})
}

// ProductModelDisable 停用型号（校验设备/工单/知识引用）
// POST /api/enterprise/v1/products/:id/models/:modelId/_disable
func ProductModelDisable(ctx *gin.Context) {
	tenantIDInt, productID, modelID, ok := resolveTenantProductChildRoute(ctx, "modelId")
	if !ok {
		return
	}
	if err := services.ProductModelService.DisableModel(tenantIDInt, productID, modelID, enterpriseActionOperator(ctx, tenantIDInt)); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, gin.H{"success": true})
}

// ---- 模块与适用型号 ----

// ProductModules 获取产品模块列表
// GET /api/enterprise/v1/products/:id/modules
func ProductModules(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	items, err := services.ProductCenterService.ListEnterpriseProductModules(tenantIDInt, productID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, items)
}

// ProductModuleCreate 创建产品模块
// POST /api/enterprise/v1/products/:id/modules
func ProductModuleCreate(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	var req dto.EnterpriseProductModuleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.ProductModuleService.CreateModule(services.CreateProductModuleRequest{
		TenantID:          tenantIDInt,
		ProductID:         productID,
		ModuleCode:        req.ModuleCode,
		Name:              req.Name,
		DefaultSupplierID: req.DefaultSupplierID,
		IsSafetyCritical:  req.IsSafetyCritical,
	}, enterpriseActionOperator(ctx, tenantIDInt))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	modules, err := services.ProductCenterService.ListEnterpriseProductModules(tenantIDInt, productID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, findModuleByID(modules, item.ID))
}

// ProductModuleUpdate 更新产品模块
// PATCH /api/enterprise/v1/products/:id/modules/:moduleId
func ProductModuleUpdate(ctx *gin.Context) {
	tenantIDInt, productID, moduleID, ok := resolveTenantProductChildRoute(ctx, "moduleId")
	if !ok {
		return
	}
	var req dto.EnterpriseProductModuleUpdateRequest
	if err := readOptionalJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	existing := services.ProductModuleService.Get(moduleID)
	if existing == nil || existing.TenantID != tenantIDInt || existing.ProductID != productID {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("product module not found"))
		return
	}
	if err := services.ProductModuleService.UpdateModule(services.UpdateProductModuleRequest{
		ID:                moduleID,
		ModuleCode:        chooseOptionalString(req.ModuleCode, existing.ModuleCode),
		Name:              chooseOptionalString(req.Name, existing.Name),
		DefaultSupplierID: chooseOptionalInt64(req.DefaultSupplierID, existing.DefaultSupplierID),
		IsSafetyCritical:  chooseOptionalBool(req.IsSafetyCritical, existing.IsSafetyCritical),
	}, enterpriseActionOperator(ctx, tenantIDInt)); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	modules, err := services.ProductCenterService.ListEnterpriseProductModules(tenantIDInt, productID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, findModuleByID(modules, moduleID))
}

// ProductModuleEnable 启用模块
// POST /api/enterprise/v1/products/:id/modules/:moduleId/_enable
func ProductModuleEnable(ctx *gin.Context) {
	tenantIDInt, productID, moduleID, ok := resolveTenantProductChildRoute(ctx, "moduleId")
	if !ok {
		return
	}
	if err := services.ProductModuleService.EnableModule(tenantIDInt, productID, moduleID, enterpriseActionOperator(ctx, tenantIDInt)); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, gin.H{"success": true})
}

// ProductModuleDisable 停用模块
// POST /api/enterprise/v1/products/:id/modules/:moduleId/_disable
func ProductModuleDisable(ctx *gin.Context) {
	tenantIDInt, productID, moduleID, ok := resolveTenantProductChildRoute(ctx, "moduleId")
	if !ok {
		return
	}
	if err := services.ProductModuleService.DisableModule(tenantIDInt, productID, moduleID, enterpriseActionOperator(ctx, tenantIDInt)); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, gin.H{"success": true})
}

// ProductModuleModels 获取模块适用型号
// GET /api/enterprise/v1/products/:id/modules/:moduleId/models
func ProductModuleModels(ctx *gin.Context) {
	tenantIDInt, productID, moduleID, ok := resolveTenantProductChildRoute(ctx, "moduleId")
	if !ok {
		return
	}
	if _, err := services.ProductCenterService.RequireTenantProduct(tenantIDInt, productID); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	modelIDs := services.ProductModuleService.ListModuleModelIDs(moduleID)
	httpx.WriteJSON(ctx, gin.H{"module_id": moduleID, "model_ids": modelIDs})
}

// ProductModuleModelsReplace 整体替换模块适用型号（事务）
// PUT /api/enterprise/v1/products/:id/modules/:moduleId/models
func ProductModuleModelsReplace(ctx *gin.Context) {
	tenantIDInt, productID, moduleID, ok := resolveTenantProductChildRoute(ctx, "moduleId")
	if !ok {
		return
	}
	var req dto.EnterpriseModuleModelsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	modelIDs, err := services.ProductModuleService.ReplaceModelLinks(tenantIDInt, productID, moduleID, req.ModelIDs, enterpriseActionOperator(ctx, tenantIDInt))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, gin.H{"module_id": moduleID, "model_ids": modelIDs})
}

// ---- 产品手册与知识链接 ----

// ProductManualFiles 获取产品手册文件列表
// GET /api/enterprise/v1/products/:id/manual-files
func ProductManualFiles(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	items, err := services.ProductManualFileService.ListEnterpriseManualFiles(
		tenantIDInt,
		productID,
		enterpriseActionOperator(ctx, tenantIDInt),
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, items)
}

// ProductManualFileContent 返回站内预览或下载使用的原始文件内容。
// GET /api/enterprise/v1/products/:id/manual-files/:manualFileId/content
func ProductManualFileContent(ctx *gin.Context) {
	tenantIDInt, productID, manualFileID, ok := resolveTenantProductChildRoute(ctx, "manualFileId")
	if !ok {
		return
	}
	content, err := services.ProductManualFileService.OpenEnterpriseManualFileContent(
		tenantIDInt,
		productID,
		manualFileID,
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	defer func() { _ = content.Reader.Close() }()

	contentType := strings.TrimSpace(content.MimeType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	contentDisposition := mime.FormatMediaType("inline", map[string]string{"filename": content.Filename})
	ctx.DataFromReader(http.StatusOK, content.FileSize, contentType, content.Reader, map[string]string{
		"Content-Disposition":    contentDisposition,
		"X-Content-Type-Options": "nosniff",
	})
}

// ProductManualFileUpload 上传产品手册文件
// POST /api/enterprise/v1/products/:id/manual-files/_upload
func ProductManualFileUpload(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	header, err := ctx.FormFile("file")
	if err != nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0323"))
		return
	}
	item, err := services.ProductManualFileService.UploadEnterpriseManualFile(
		tenantIDInt,
		productID,
		header,
		dto.EnterpriseProductManualFileMutationRequest{
			Title:      ctx.PostForm("title"),
			Language:   ctx.PostForm("language"),
			Version:    ctx.PostForm("version"),
			Visibility: ctx.PostForm("visibility"),
		},
		enterpriseActionOperator(ctx, tenantIDInt),
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, item)
}

// ProductManualFileUpdate 更新产品手册业务属性。
// PATCH /api/enterprise/v1/products/:id/manual-files/:manualFileId
func ProductManualFileUpdate(ctx *gin.Context) {
	tenantIDInt, productID, manualFileID, ok := resolveTenantProductChildRoute(ctx, "manualFileId")
	if !ok {
		return
	}
	var req dto.EnterpriseProductManualFileMutationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.ProductManualFileService.UpdateEnterpriseManualFile(
		tenantIDInt,
		productID,
		manualFileID,
		req,
		enterpriseActionOperator(ctx, tenantIDInt),
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, item)
}

// ProductManualFileDelete 删除产品手册文件
// DELETE /api/enterprise/v1/products/:id/manual-files/:manualFileId
func ProductManualFileDelete(ctx *gin.Context) {
	tenantIDInt, productID, manualFileID, ok := resolveTenantProductChildRoute(ctx, "manualFileId")
	if !ok {
		return
	}
	if err := services.ProductManualFileService.DeleteEnterpriseManualFile(
		tenantIDInt,
		productID,
		manualFileID,
		enterpriseActionOperator(ctx, tenantIDInt),
	); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, gin.H{"success": true})
}

// ProductKnowledgeDocuments 获取产品知识库上传文档列表。
// GET /api/enterprise/v1/products/:id/knowledge-documents
func ProductKnowledgeDocuments(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	page, pageSize := knowledgeDocumentPageQuery(ctx)
	items, err := services.KnowledgeDocumentService.ListEnterpriseProductDocuments(tenantIDInt, productID, page, pageSize)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, items)
}

// ProductKnowledgeDocumentUpload 上传并索引产品知识库文档。
// POST /api/enterprise/v1/products/:id/knowledge-documents/_upload
func ProductKnowledgeDocumentUpload(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	header, err := ctx.FormFile("file")
	if err != nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0323"))
		return
	}
	item, err := services.KnowledgeDocumentService.UploadEnterpriseProductDocument(
		tenantIDInt,
		productID,
		header,
		enterpriseActionOperator(ctx, tenantIDInt),
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, item)
}

// ProductKnowledgeDocumentReprocess 重新提交产品知识文档的异步索引任务。
// POST /api/enterprise/v1/products/:id/knowledge-documents/:documentId/_reprocess
func ProductKnowledgeDocumentReprocess(ctx *gin.Context) {
	tenantIDInt, productID, documentID, ok := resolveTenantProductChildRoute(ctx, "documentId")
	if !ok {
		return
	}
	item, err := services.KnowledgeDocumentService.ReprocessEnterpriseProductDocument(
		tenantIDInt,
		productID,
		documentID,
		enterpriseActionOperator(ctx, tenantIDInt),
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, item)
}

// ProductKnowledgeDocumentDelete 删除产品知识库上传文档及其向量索引。
// DELETE /api/enterprise/v1/products/:id/knowledge-documents/:documentId
func ProductKnowledgeDocumentDelete(ctx *gin.Context) {
	tenantIDInt, productID, documentID, ok := resolveTenantProductChildRoute(ctx, "documentId")
	if !ok {
		return
	}
	if err := services.KnowledgeDocumentService.DeleteEnterpriseProductDocument(
		tenantIDInt,
		productID,
		documentID,
		enterpriseActionOperator(ctx, tenantIDInt),
	); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, gin.H{"success": true})
}

// ProductManuals 获取产品手册链接列表
// GET /api/enterprise/v1/products/:id/manuals
func ProductManuals(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	items, err := services.ProductCenterService.ListEnterpriseManuals(tenantIDInt, productID, productKnowledgePreviewLimit(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, items)
}

// ProductManualLinkCreate 挂载产品手册知识（条目归属校验在 Service 内完成，§7.3）
// POST /api/enterprise/v1/products/:id/manuals/_link
func ProductManualLinkCreate(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	var req dto.EnterpriseManualLinkRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	link, err := services.ProductKnowledgeLinkService.CreateLink(services.CreateProductKnowledgeLinkRequest{
		TenantID:         tenantIDInt,
		ProductID:        productID,
		ProductModelID:   req.ProductModelID,
		KnowledgeBaseID:  req.KnowledgeBaseID,
		KnowledgeEntryID: req.KnowledgeEntryID,
		LinkType:         req.LinkType,
		Language:         req.Language,
		Version:          req.Version,
		Visibility:       req.Visibility,
		SortNo:           req.SortNo,
	}, enterpriseActionOperator(ctx, tenantIDInt))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, services.ProductCenterService.BuildManualDTO(tenantIDInt, link))
}

// ProductManualUpdate 更新手册链接（PATCH 语义）
// PATCH /api/enterprise/v1/products/:id/manuals/:linkId
func ProductManualUpdate(ctx *gin.Context) {
	tenantIDInt, productID, linkID, ok := resolveTenantProductChildRoute(ctx, "linkId")
	if !ok {
		return
	}
	var req dto.EnterpriseManualUpdateRequest
	if err := readOptionalJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.ProductKnowledgeLinkService.UpdateLinkPartial(tenantIDInt, productID, linkID, req.ProductModelID, req.Language, req.Version, req.Visibility, req.SortNo, enterpriseActionOperator(ctx, tenantIDInt)); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, manualDTOOrEmpty(tenantIDInt, linkID))
}

// ProductManualDelete 解除手册关联
// DELETE /api/enterprise/v1/products/:id/manuals/:linkId
func ProductManualDelete(ctx *gin.Context) {
	tenantIDInt, productID, linkID, ok := resolveTenantProductChildRoute(ctx, "linkId")
	if !ok {
		return
	}
	if err := services.ProductKnowledgeLinkService.DeleteLinkScoped(tenantIDInt, productID, linkID, enterpriseActionOperator(ctx, tenantIDInt)); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, gin.H{"success": true})
}

// ProductManualPublish 发布手册
// POST /api/enterprise/v1/products/:id/manuals/:linkId/_publish
func ProductManualPublish(ctx *gin.Context) {
	tenantIDInt, productID, linkID, ok := resolveTenantProductChildRoute(ctx, "linkId")
	if !ok {
		return
	}
	if err := services.ProductKnowledgeLinkService.PublishLinkScoped(tenantIDInt, productID, linkID, enterpriseActionOperator(ctx, tenantIDInt)); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, manualDTOOrEmpty(tenantIDInt, linkID))
}

// ProductManualDeprecate 下架手册
// POST /api/enterprise/v1/products/:id/manuals/:linkId/_deprecate
func ProductManualDeprecate(ctx *gin.Context) {
	tenantIDInt, productID, linkID, ok := resolveTenantProductChildRoute(ctx, "linkId")
	if !ok {
		return
	}
	if err := services.ProductKnowledgeLinkService.DeprecateLink(tenantIDInt, productID, linkID, enterpriseActionOperator(ctx, tenantIDInt)); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, manualDTOOrEmpty(tenantIDInt, linkID))
}

// ProductManualReindex 重索引手册
// POST /api/enterprise/v1/products/:id/manuals/:linkId/_reindex
func ProductManualReindex(ctx *gin.Context) {
	tenantIDInt, productID, linkID, ok := resolveTenantProductChildRoute(ctx, "linkId")
	if !ok {
		return
	}
	task, err := services.ProductKnowledgeLinkService.ReindexLink(tenantIDInt, productID, linkID, enterpriseActionOperator(ctx, tenantIDInt))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result := manualDTOOrEmpty(tenantIDInt, linkID)
	if task != nil {
		result.IndexStatus = "pending"
	}
	httpx.WriteJSON(ctx, result)
}

// ProductKnowledgeLinks 获取产品知识链接列表（与 manuals 同构，返回全部类型）
// GET /api/enterprise/v1/products/:id/knowledge-links
func ProductKnowledgeLinks(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	items, err := services.ProductCenterService.ListEnterpriseManuals(tenantIDInt, productID, productKnowledgePreviewLimit(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, items)
}

// ProductKnowledgeLinkCreate 创建知识链接（与手册挂载共用统一契约）
// POST /api/enterprise/v1/products/:id/knowledge-links
func ProductKnowledgeLinkCreate(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	var req dto.EnterpriseManualLinkRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	link, err := services.ProductKnowledgeLinkService.CreateLink(services.CreateProductKnowledgeLinkRequest{
		TenantID:         tenantIDInt,
		ProductID:        productID,
		ProductModelID:   req.ProductModelID,
		KnowledgeBaseID:  req.KnowledgeBaseID,
		KnowledgeEntryID: req.KnowledgeEntryID,
		LinkType:         firstNonEmpty(req.LinkType, "knowledge_article"),
		Language:         req.Language,
		Version:          req.Version,
		Visibility:       req.Visibility,
		SortNo:           req.SortNo,
	}, enterpriseActionOperator(ctx, tenantIDInt))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, services.ProductCenterService.BuildManualDTO(tenantIDInt, link))
}

// ProductKnowledgeLinkDelete 删除知识链接
// DELETE /api/enterprise/v1/products/:id/knowledge-links/:linkId
func ProductKnowledgeLinkDelete(ctx *gin.Context) {
	tenantIDInt, productID, linkID, ok := resolveTenantProductChildRoute(ctx, "linkId")
	if !ok {
		return
	}
	if err := services.ProductKnowledgeLinkService.DeleteLinkScoped(tenantIDInt, productID, linkID, enterpriseActionOperator(ctx, tenantIDInt)); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, gin.H{"success": true})
}

// ---- 故障统计（P2 增加 data_status / 重建）----

// ProductFaultStats 获取产品故障统计
// GET /api/enterprise/v1/products/:id/fault-stats
func ProductFaultStats(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	result, err := services.ProductCenterService.GetProductFaultStats(tenantIDInt, productID, ctx.DefaultQuery("range", "90d"))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

// ProductFaultStatsRebuild 触发故障统计历史重建
// POST /api/enterprise/v1/products/:id/fault-stats/_rebuild
func ProductFaultStatsRebuild(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	job, err := services.ProductFaultStatsService.RequestRebuild(tenantIDInt, productID, ctx.DefaultQuery("range", "180d"), enterpriseActionOperator(ctx, tenantIDInt))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, services.ProductFaultStatsService.BuildJobDTO(job))
}

// ProductFaultStatsJob 查询重建任务状态
// GET /api/enterprise/v1/products/:id/fault-stats/jobs/:jobId
func ProductFaultStatsJob(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	jobID, err := strconv.ParseInt(ctx.Param("jobId"), 10, 64)
	if err != nil || jobID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid job id"))
		return
	}
	job, err := services.ProductFaultStatsService.GetRebuildJob(tenantIDInt, productID, jobID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, services.ProductFaultStatsService.BuildJobDTO(job))
}

// ---- 其余只读聚合端点 ----

// ProductDevices 获取产品设备列表
// GET /api/enterprise/v1/products/:id/devices
func ProductDevices(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	items, err := services.ProductCenterService.ListProductDevices(tenantIDInt, productID, productResourceQuery(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, items)
}

// ProductServiceCodes 获取产品服务码列表
// GET /api/enterprise/v1/products/:id/service-codes
func ProductServiceCodes(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	items, err := services.ProductCenterService.ListProductServiceCodes(tenantIDInt, productID, productResourceQuery(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, items)
}

// ProductTickets 获取产品关联工单列表
// GET /api/enterprise/v1/products/:id/tickets
func ProductTickets(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	items, err := services.ProductCenterService.ListProductTickets(tenantIDInt, productID, productResourceQuery(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, items)
}

// ProductConversations 获取产品关联会话列表
// GET /api/enterprise/v1/products/:id/conversations
func ProductConversations(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	items, err := services.ProductCenterService.ListProductConversations(tenantIDInt, productID, productResourceQuery(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, items)
}

// ProductRepairHistory 获取产品维修历史
// GET /api/enterprise/v1/products/:id/repair-history
func ProductRepairHistory(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	items, err := services.ProductCenterService.ListProductRepairHistory(tenantIDInt, productID, productResourceQuery(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, items)
}

func productResourceQuery(ctx *gin.Context) services.ProductResourceQuery {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("page_size", ctx.DefaultQuery("limit", "20")))
	return services.ProductResourceQuery{Page: page, PageSize: pageSize, Search: ctx.Query("search")}
}

func productKnowledgePreviewLimit(ctx *gin.Context) int {
	limit, _ := strconv.Atoi(ctx.Query("limit"))
	return limit
}

func knowledgeDocumentPageQuery(ctx *gin.Context) (int, int) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("page_size", ctx.DefaultQuery("limit", "10")))
	return page, pageSize
}

// ProductKnowledgeCoverage 获取产品知识覆盖详情
// GET /api/enterprise/v1/products/:id/knowledge-coverage
func ProductKnowledgeCoverage(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	item, err := services.ProductCenterService.GetProductKnowledgeCoverageDetail(tenantIDInt, productID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, item)
}

// ProductQualitySignals 获取产品质量信号
// GET /api/enterprise/v1/products/:id/quality-signals
func ProductQualitySignals(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	items, err := services.ProductCenterService.ListProductQualitySignals(tenantIDInt, productID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, items)
}

// ProductUsage 获取产品用量指标
// GET /api/enterprise/v1/products/:id/usage
func ProductUsage(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	items, err := services.ProductCenterService.ListProductUsageMetrics(tenantIDInt, productID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, items)
}

// ProductQualitySignalCreate 创建产品质量线索
// POST /api/enterprise/v1/products/:id/quality-signals
func ProductQualitySignalCreate(ctx *gin.Context) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return
	}
	var req productQualitySignalPayload
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.ProductCenterService.CreateProductQualitySignal(services.CreateProductQualitySignalRequest{
		TenantID:         tenantIDInt,
		ProductID:        productID,
		ProductModelID:   req.ProductModelID,
		SignalType:       req.SignalType,
		Severity:         req.Severity,
		Title:            req.Title,
		Description:      req.Description,
		TriggerCondition: req.TriggerCondition,
		MetricValue:      req.MetricValue,
		SampleCount:      req.SampleCount,
		Source:           req.Source,
		SourceType:       req.SourceType,
		SourceID:         req.SourceID,
		OwnerUserID:      req.OwnerUserID,
	}, enterpriseActionOperator(ctx, tenantIDInt))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

// ProductQualitySignalUpdate 更新产品质量线索
// PATCH /api/enterprise/v1/products/:id/quality-signals/:signalId
func ProductQualitySignalUpdate(ctx *gin.Context) {
	tenantIDInt, productID, signalID, ok := resolveProductQualitySignalRoute(ctx)
	if !ok {
		return
	}
	var req productQualitySignalPatchPayload
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.ProductCenterService.UpdateProductQualitySignal(services.UpdateProductQualitySignalRequest{
		TenantID:         tenantIDInt,
		ProductID:        productID,
		SignalID:         signalID,
		ProductModelID:   req.ProductModelID,
		SignalType:       req.SignalType,
		Severity:         req.Severity,
		Title:            req.Title,
		Description:      req.Description,
		TriggerCondition: req.TriggerCondition,
		MetricValue:      req.MetricValue,
		SampleCount:      req.SampleCount,
		Source:           req.Source,
		SourceType:       req.SourceType,
		SourceID:         req.SourceID,
		OwnerUserID:      req.OwnerUserID,
	}, enterpriseActionOperator(ctx, tenantIDInt))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

// ProductQualitySignalResolve 标记质量线索已处理
// POST /api/enterprise/v1/products/:id/quality-signals/:signalId/_resolve
func ProductQualitySignalResolve(ctx *gin.Context) {
	tenantIDInt, productID, signalID, ok := resolveProductQualitySignalRoute(ctx)
	if !ok {
		return
	}
	type resolveReq struct {
		Resolution string `json:"resolution"`
	}
	var req resolveReq
	if err := readOptionalJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.ProductCenterService.ResolveProductQualitySignal(tenantIDInt, productID, signalID, req.Resolution, enterpriseActionOperator(ctx, tenantIDInt))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

// ---- 共享载荷与辅助函数 ----

type productQualitySignalPayload struct {
	ProductModelID   int64   `json:"product_model_id"`
	SignalType       string  `json:"signal_type"`
	Severity         string  `json:"severity"`
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	TriggerCondition string  `json:"trigger_condition"`
	MetricValue      float64 `json:"metric_value"`
	SampleCount      int64   `json:"sample_count"`
	Source           string  `json:"source"`
	SourceType       string  `json:"source_type"`
	SourceID         string  `json:"source_id"`
	OwnerUserID      int64   `json:"owner_user_id"`
}

type productQualitySignalPatchPayload struct {
	ProductModelID   *int64   `json:"product_model_id"`
	SignalType       *string  `json:"signal_type"`
	Severity         *string  `json:"severity"`
	Title            *string  `json:"title"`
	Description      *string  `json:"description"`
	TriggerCondition *string  `json:"trigger_condition"`
	MetricValue      *float64 `json:"metric_value"`
	SampleCount      *int64   `json:"sample_count"`
	Source           *string  `json:"source"`
	SourceType       *string  `json:"source_type"`
	SourceID         *string  `json:"source_id"`
	OwnerUserID      *int64   `json:"owner_user_id"`
}

func requireTenantInt(ctx *gin.Context) (int64, bool) {
	tenantID := resolveTenantID(ctx)
	if tenantID == "" {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("tenant is required"))
		return 0, false
	}
	tenantIDInt, err := strconv.ParseInt(tenantID, 10, 64)
	if err != nil || tenantIDInt <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid tenant id"))
		return 0, false
	}
	return tenantIDInt, true
}

func resolveTenantProductRoute(ctx *gin.Context) (int64, int64, bool) {
	tenantIDInt, ok := requireTenantInt(ctx)
	if !ok {
		return 0, 0, false
	}
	productID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || productID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid product id"))
		return 0, 0, false
	}
	if err := services.RequireEnterpriseProductAccess(tenantIDInt, productID, enterpriseActionOperator(ctx, tenantIDInt)); err != nil {
		httpx.WriteJSON(ctx, err)
		return 0, 0, false
	}
	return tenantIDInt, productID, true
}

func resolveTenantProductChildRoute(ctx *gin.Context, childParam string) (int64, int64, int64, bool) {
	tenantIDInt, productID, ok := resolveTenantProductRoute(ctx)
	if !ok {
		return 0, 0, 0, false
	}
	childID, err := strconv.ParseInt(ctx.Param(childParam), 10, 64)
	if err != nil || childID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid "+childParam))
		return 0, 0, 0, false
	}
	return tenantIDInt, productID, childID, true
}

func resolveProductQualitySignalRoute(ctx *gin.Context) (int64, int64, int64, bool) {
	return resolveTenantProductChildRoute(ctx, "signalId")
}

func findByID(items []dto.EnterpriseProductModelDTO, id int64) dto.EnterpriseProductModelDTO {
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	return dto.EnterpriseProductModelDTO{ID: id}
}

func findModuleByID(items []dto.EnterpriseProductModuleDTO, id int64) dto.EnterpriseProductModuleDTO {
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	return dto.EnterpriseProductModuleDTO{ID: id}
}

func manualDTOOrEmpty(tenantID, linkID int64) dto.EnterpriseProductManualDTO {
	link := services.ProductKnowledgeLinkService.Get(linkID)
	if link == nil {
		return dto.EnterpriseProductManualDTO{ID: linkID}
	}
	return services.ProductCenterService.BuildManualDTO(tenantID, link)
}

func chooseOptionalString(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return *value
}

func chooseOptionalInt64(value *int64, fallback int64) int64 {
	if value == nil {
		return fallback
	}
	return *value
}

func chooseOptionalBool(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

// resolveTenantID 从 AuthPrincipal 或 context 中获取 tenantID。
// 优先使用 middleware.AuthPrincipal，其次为 ctx.GetString("tenantId")。
func resolveTenantID(ctx *gin.Context) string {
	principal := middleware.GetAuthPrincipal(ctx)
	if principal != nil && principal.TenantID > 0 {
		return strconv.FormatInt(principal.TenantID, 10)
	}
	// 平台用户（super_admin 等）可跨租户操作，使用默认系统租户
	if principal != nil && principal.IsPlatform() {
		return "1"
	}
	tenantID := ctx.GetString("tenantId")
	if tenantID != "" {
		return tenantID
	}
	return ""
}
