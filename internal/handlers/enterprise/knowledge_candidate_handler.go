package enterprise

import (
	"strconv"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

// 企业知识候选处理 Handler（§7.4）。

// KnowledgeCandidateList 知识候选列表
// GET /api/enterprise/v1/knowledge/candidates?product_id=&review_status=
func KnowledgeCandidateList(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	productID, _ := strconv.ParseInt(ctx.Query("product_id"), 10, 64)
	items, err := services.KnowledgeCandidateReviewService.ListCandidates(tenantID, productID, ctx.Query("review_status"))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, items)
}

// KnowledgeCandidatePage 分页列出知识候选。
func KnowledgeCandidatePage(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	productID, _ := strconv.ParseInt(ctx.Query("product_id"), 10, 64)
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "100"))
	result, err := services.KnowledgeCandidateReviewService.ListCandidatePage(tenantID, productID, ctx.Query("review_status"), page, pageSize)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

// KnowledgeCandidateGet 知识候选详情
// GET /api/enterprise/v1/knowledge/candidates/:id
func KnowledgeCandidateGet(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	candidateID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || candidateID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid candidate id"))
		return
	}
	item, err := services.KnowledgeCandidateReviewService.GetCandidate(tenantID, candidateID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, item)
}

// KnowledgeCandidateEnrich 补充候选内容并重新评分。
// POST /api/enterprise/v1/knowledge/candidates/:id/_enrich
func KnowledgeCandidateEnrich(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	candidateID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || candidateID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid candidate id"))
		return
	}
	var req dto.EnterpriseKnowledgeCandidateEnrichRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.KnowledgeCandidateReviewService.EnrichCandidate(
		tenantID,
		candidateID,
		req,
		enterpriseActionOperator(ctx, tenantID),
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, item)
}

// KnowledgeCandidateDetachDuplicate 将误判的重复候选恢复为独立候选。
func KnowledgeCandidateDetachDuplicate(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	candidateID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || candidateID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid candidate id"))
		return
	}
	item, err := services.KnowledgeCandidateReviewService.DetachDuplicateCandidate(tenantID, candidateID, enterpriseActionOperator(ctx, tenantID))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, item)
}

func recordEnterpriseKnowledgeCandidateAudit(ctx *gin.Context, operator *dto.AuthPrincipal, tenantID int64, item *dto.EnterpriseKnowledgeCandidateDTO, action, riskLevel string, afterState map[string]any) {
	if ctx == nil || operator == nil || item == nil || item.ID <= 0 || action == "" {
		return
	}
	if tenantID <= 0 {
		tenantID = operator.EffectiveTenantID()
	}
	if tenantID <= 0 {
		return
	}
	if afterState == nil {
		afterState = make(map[string]any)
	}
	afterState["candidateId"] = item.ID
	afterState["productId"] = item.ProductID
	afterState["ticketId"] = item.TicketID
	afterState["reviewStatus"] = item.ReviewStatus
	afterState["knowledgeBaseId"] = item.KnowledgeBaseID
	afterState["knowledgeEntryId"] = item.KnowledgeEntryID
	_ = services.AuditService.RecordAudit(ctx.Request.Context(), services.RecordAuditInput{
		TenantID:       tenantID,
		ActorID:        strconv.FormatInt(operator.UserID, 10),
		ActorType:      "user",
		Domain:         "knowledge",
		ResourceType:   "knowledge_candidate",
		ResourceID:     strconv.FormatInt(item.ID, 10),
		Action:         action,
		AfterState:     afterState,
		IPAddress:      ctx.ClientIP(),
		UserAgent:      ctx.Request.UserAgent(),
		RequestID:      httpx.GetRequestID(ctx),
		SupportGrantID: operator.SupportGrantID,
		RiskLevel:      riskLevel,
	})
}

// KnowledgeCandidateApprove 批准候选（创建知识条目与产品知识链接）
// POST /api/enterprise/v1/knowledge/candidates/:id/_approve
func KnowledgeCandidateApprove(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	candidateID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || candidateID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid candidate id"))
		return
	}
	var req dto.EnterpriseKnowledgeCandidateApproveRequest
	if err := readOptionalJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	operator := enterpriseActionOperator(ctx, tenantID)
	item, err := services.KnowledgeCandidateReviewService.ApproveCandidate(tenantID, candidateID, req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordEnterpriseKnowledgeCandidateAudit(ctx, operator, tenantID, item, "knowledge_candidate.approved", models.RiskLevelMedium, map[string]any{
		"publish": req.Publish,
	})
	httpx.WriteJSON(ctx, item)
}

// KnowledgeCandidateReject 拒绝候选
// POST /api/enterprise/v1/knowledge/candidates/:id/_reject
func KnowledgeCandidateReject(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	candidateID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || candidateID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid candidate id"))
		return
	}
	var req dto.EnterpriseKnowledgeCandidateRejectRequest
	if err := readOptionalJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	operator := enterpriseActionOperator(ctx, tenantID)
	item, err := services.KnowledgeCandidateReviewService.RejectCandidate(tenantID, candidateID, req.Remark, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordEnterpriseKnowledgeCandidateAudit(ctx, operator, tenantID, item, "knowledge_candidate.rejected", models.RiskLevelMedium, map[string]any{
		"remark": req.Remark,
	})
	httpx.WriteJSON(ctx, item)
}

// KnowledgeCandidateMerge 合并候选到已有知识条目
// POST /api/enterprise/v1/knowledge/candidates/:id/_merge
func KnowledgeCandidateMerge(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	candidateID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || candidateID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid candidate id"))
		return
	}
	var req dto.EnterpriseKnowledgeCandidateMergeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	operator := enterpriseActionOperator(ctx, tenantID)
	item, err := services.KnowledgeCandidateReviewService.MergeCandidate(tenantID, candidateID, req.KnowledgeEntryID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	recordEnterpriseKnowledgeCandidateAudit(ctx, operator, tenantID, item, "knowledge_candidate.merged", models.RiskLevelMedium, map[string]any{
		"knowledgeEntryId": req.KnowledgeEntryID,
	})
	httpx.WriteJSON(ctx, item)
}

// KnowledgeIndexTaskList 索引同步任务列表（§12.2 索引任务与失败重试）
// GET /api/enterprise/v1/knowledge/index-tasks?product_id=
func KnowledgeIndexTaskList(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	productID, _ := strconv.ParseInt(ctx.Query("product_id"), 10, 64)
	tasks := services.KnowledgeIndexSyncService.ListTasks(tenantID, productID)
	results := make([]gin.H, 0, len(tasks))
	for _, task := range tasks {
		results = append(results, gin.H{
			"id":                  task.ID,
			"idempotency_key":     task.IdempotencyKey,
			"subject_type":        task.SubjectType,
			"subject_id":          task.SubjectID,
			"knowledge_base_id":   task.KnowledgeBaseID,
			"product_id":          task.ProductID,
			"revision_id":         task.RevisionID,
			"index_generation_id": task.IndexGenerationID,
			"provider_type":       task.ProviderType,
			"action":              task.Action,
			"input_version":       task.InputVersion,
			"collection_name":     task.CollectionName,
			"content_hash":        task.ContentHash,
			"status":              task.Status,
			"external_job_id":     task.ExternalJobID,
			"retry_count":         task.RetryCount,
			"max_retries":         task.MaxRetries,
			"next_attempt_at":     task.NextAttemptAt,
			"locked_at":           task.LockedAt,
			"lock_owner":          task.LockOwner,
			"error_code":          task.ErrorCode,
			"error_summary":       task.ErrorSummary,
			"last_attempt_at":     task.LastAttemptAt,
			"finished_at":         task.FinishedAt,
			"created_at":          task.CreatedAt,
			"updated_at":          task.UpdatedAt,
		})
	}
	httpx.WriteJSON(ctx, results)
}

// KnowledgeIndexTaskRetry 手工重试索引任务
// POST /api/enterprise/v1/knowledge/index-tasks/:id/_retry
func KnowledgeIndexTaskRetry(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	taskID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || taskID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid task id"))
		return
	}
	operator := enterpriseActionOperator(ctx, tenantID)
	task, err := services.KnowledgeIndexSyncService.RetryTask(tenantID, taskID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, gin.H{"id": task.ID, "status": task.Status})
}
