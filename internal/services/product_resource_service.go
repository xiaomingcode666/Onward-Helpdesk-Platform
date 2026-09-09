package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/providers"
	"remotehelpdesk/internal/pkg/secretstore"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var ProductResourceService = &productResourceService{}

const (
	productResourceStatusMissing     = "missing"
	productResourceStatusPending     = "pending"
	productResourceStatusReady       = "ready"
	productResourceStatusFailed      = "failed"
	productResourceStatusPartialFail = "partial_failed"
	productDefaultAIQuotaConfigKey   = "default_product_ai_quota_usd"
	productDefaultAIQuotaUSD         = 2.0
	productAICurrencyUSD             = "USD"
	productResourceJobMaxRetries     = 6
	productResourceJobBatchSize      = 20
	productResourceJobLeaseTTL       = 5 * time.Minute
	productResourceJobTaskTimeout    = 2 * time.Minute
)

type productResourceService struct{}

func (s *productResourceService) GetProductResources(ctx context.Context, tenantID, productID int64, operator *dto.AuthPrincipal) (*dto.EnterpriseProductResourcesDTO, error) {
	if _, err := s.requireTenantProduct(tenantID, productID, operator); err != nil {
		return nil, err
	}
	return s.buildProductResourcesDTO(tenantID, productID), nil
}

func (s *productResourceService) EnqueueProductResourceProvisioning(ctx context.Context, tenantID, productID int64, quotaLimit float64, operator *dto.AuthPrincipal) (*dto.EnterpriseProductResourcesDTO, error) {
	product, err := s.requireTenantProduct(tenantID, productID, operator)
	if err != nil {
		return nil, err
	}
	if !TenantCapabilityService.AIEnabled(product.TenantID) {
		return s.buildProductResourcesDTO(tenantID, productID), nil
	}
	if err := s.createOrResetProvisioningJob(product, normalizeProductAIQuotaLimit(quotaLimit), operator, false); err != nil {
		return nil, err
	}
	return s.buildProductResourcesDTO(tenantID, productID), nil
}

func (s *productResourceService) RetryProductResourceProvisioning(ctx context.Context, tenantID, productID int64, operator *dto.AuthPrincipal) (*dto.EnterpriseProductResourcesDTO, error) {
	product, err := s.requireTenantProduct(tenantID, productID, operator)
	if err != nil {
		return nil, err
	}
	if !TenantCapabilityService.AIEnabled(product.TenantID) {
		return s.buildProductResourcesDTO(tenantID, productID), nil
	}
	quotaLimit := defaultProductAIQuotaLimit()
	if credential := repositories.ProductAIUsageCredentialRepository.GetByProduct(sqls.DB(), tenantID, productID); credential != nil && credential.QuotaLimit > 0 {
		quotaLimit = credential.QuotaLimit
	}
	if err := s.createOrResetProvisioningJob(product, quotaLimit, operator, true); err != nil {
		return nil, err
	}
	return s.buildProductResourcesDTO(tenantID, productID), nil
}

func (s *productResourceService) ProvisionProductResources(ctx context.Context, tenantID, productID int64, quotaLimit float64, operator *dto.AuthPrincipal) (*dto.EnterpriseProductResourcesDTO, error) {
	product, err := s.requireTenantProduct(tenantID, productID, operator)
	if err != nil {
		return nil, err
	}
	if !TenantCapabilityService.AIEnabled(product.TenantID) {
		return s.buildProductResourcesDTO(tenantID, productID), nil
	}

	_, _ = s.ensureProductKnowledgeDataset(ctx, product, operator)
	_, _ = s.ensureProductSub2APIKey(ctx, product, normalizeProductAIQuotaLimit(quotaLimit), operator)
	if _, err := ProductAIAgentService.EnsureProductCustomerAgent(tenantID, productID, operator); err != nil {
		// 客服机器人可在企业 AI 服务中心幂等补齐；这里不让产品资源整体失败。
		_ = err
	}
	return s.buildProductResourcesDTO(tenantID, productID), nil
}

func (s *productResourceService) ProcessDueProvisioningJobs(ctx context.Context, limit int) int {
	if limit <= 0 {
		limit = productResourceJobBatchSize
	}
	now := time.Now()
	lockOwner := fmt.Sprintf("product-resource-worker-%d", now.UnixNano())
	jobs, err := repositories.ProductResourceProvisioningJobRepository.ClaimDueJobs(sqls.DB(), now, limit, lockOwner)
	if err != nil {
		slog.Warn("claim product resource provisioning jobs failed", "error", err)
		return 0
	}
	processed := 0
	for i := range jobs {
		item := jobs[i]
		if err := s.ProcessProvisioningJob(ctx, &item); err != nil {
			slog.Warn("process product resource provisioning job failed", "job_id", item.ID, "product_id", item.ProductID, "error", err)
		}
		processed++
	}
	return processed
}

func (s *productResourceService) RecoverTimedOutProvisioningJobs() int64 {
	now := time.Now()
	recovered, err := repositories.ProductResourceProvisioningJobRepository.RecoverExpiredRunningJobs(sqls.DB(), now.Add(-productResourceJobLeaseTTL), now)
	if err != nil {
		slog.Warn("recover product resource provisioning jobs failed", "error", err)
		return 0
	}
	return recovered
}

func (s *productResourceService) ProcessProvisioningJob(ctx context.Context, job *models.ProductResourceProvisioningJob) error {
	if job == nil {
		return errorsx.InvalidParam("product resource provisioning job is required")
	}
	product := repositories.ProductRepository.GetByTenant(sqls.DB(), job.ProductID, job.TenantID)
	if product == nil || product.Status == enums.StatusDeleted {
		return s.failProvisioningJob(job, errorsx.InvalidParam("product not found"))
	}
	if !TenantCapabilityService.AIEnabled(job.TenantID) {
		return s.finishSkippedProvisioningJob(job)
	}
	taskCtx := ctx
	if taskCtx == nil {
		taskCtx = context.Background()
	}
	var cancel context.CancelFunc
	taskCtx, cancel = context.WithTimeout(taskCtx, productResourceJobTaskTimeout)
	defer cancel()

	now := time.Now()
	if job.StartedAt == nil {
		job.StartedAt = &now
	}
	_ = repositories.ProductResourceProvisioningJobRepository.Updates(sqls.DB(), job.ID, map[string]any{
		"status":          "running",
		"started_at":      job.StartedAt,
		"last_attempt_at": now,
		"locked_at":       now,
		"updated_at":      now,
	})

	operator := productProvisioningOperator(job.TenantID)
	stepErrors := make([]string, 0, 3)

	if err := s.runProvisioningKnowledgeStep(taskCtx, job, product, operator); err != nil {
		stepErrors = append(stepErrors, "knowledge: "+err.Error())
	}
	if err := s.runProvisioningAIKeyStep(taskCtx, job, product, operator); err != nil {
		stepErrors = append(stepErrors, "ai_key: "+err.Error())
	}
	if err := s.runProvisioningAgentStep(taskCtx, job, product, operator); err != nil {
		stepErrors = append(stepErrors, "agent: "+err.Error())
	}
	if len(stepErrors) > 0 {
		return s.failProvisioningJob(job, errors.New(strings.Join(stepErrors, "; ")))
	}
	return s.finishProvisioningJob(job)
}

func (s *productResourceService) runProvisioningKnowledgeStep(ctx context.Context, job *models.ProductResourceProvisioningJob, product *models.Product, operator *dto.AuthPrincipal) error {
	now := time.Now()
	_ = repositories.ProductResourceProvisioningJobRepository.Updates(sqls.DB(), job.ID, map[string]any{
		"knowledge_status": productResourceStatusPending,
		"updated_at":       now,
	})
	if _, err := s.ensureProductKnowledgeDataset(ctx, product, operator); err != nil {
		_ = repositories.ProductResourceProvisioningJobRepository.Updates(sqls.DB(), job.ID, map[string]any{
			"knowledge_status": productResourceStatusFailed,
			"updated_at":       time.Now(),
		})
		return err
	}
	_ = repositories.ProductResourceProvisioningJobRepository.Updates(sqls.DB(), job.ID, map[string]any{
		"knowledge_status": productResourceStatusReady,
		"updated_at":       time.Now(),
	})
	return nil
}

func (s *productResourceService) runProvisioningAIKeyStep(ctx context.Context, job *models.ProductResourceProvisioningJob, product *models.Product, operator *dto.AuthPrincipal) error {
	now := time.Now()
	_ = repositories.ProductResourceProvisioningJobRepository.Updates(sqls.DB(), job.ID, map[string]any{
		"ai_key_status": productResourceStatusPending,
		"updated_at":    now,
	})
	credential, err := s.ensureProductSub2APIKey(ctx, product, normalizeProductAIQuotaLimit(job.RequestedQuotaLimit), operator)
	if err != nil {
		_ = repositories.ProductResourceProvisioningJobRepository.Updates(sqls.DB(), job.ID, map[string]any{
			"ai_key_status": productResourceStatusFailed,
			"updated_at":    time.Now(),
		})
		return err
	}
	if credential == nil {
		credential = repositories.ProductAIUsageCredentialRepository.GetByProduct(sqls.DB(), product.TenantID, product.ID)
	}
	if credential == nil {
		err := errors.New("product ai credential was not created")
		_ = repositories.ProductResourceProvisioningJobRepository.Updates(sqls.DB(), job.ID, map[string]any{
			"ai_key_status": productResourceStatusFailed,
			"updated_at":    time.Now(),
		})
		return err
	}
	if credential.ProvisionStatus == productResourceStatusFailed {
		err := errors.New(firstNonBlank(credential.ProvisionError, "product ai key provisioning failed"))
		_ = repositories.ProductResourceProvisioningJobRepository.Updates(sqls.DB(), job.ID, map[string]any{
			"ai_key_status": productResourceStatusFailed,
			"updated_at":    time.Now(),
		})
		return err
	}
	_ = repositories.ProductResourceProvisioningJobRepository.Updates(sqls.DB(), job.ID, map[string]any{
		"ai_key_status": productResourceStatusReady,
		"updated_at":    time.Now(),
	})
	return nil
}

func (s *productResourceService) runProvisioningAgentStep(ctx context.Context, job *models.ProductResourceProvisioningJob, product *models.Product, operator *dto.AuthPrincipal) error {
	_ = ctx
	now := time.Now()
	_ = repositories.ProductResourceProvisioningJobRepository.Updates(sqls.DB(), job.ID, map[string]any{
		"agent_status": productResourceStatusPending,
		"updated_at":   now,
	})
	if _, err := ProductAIAgentService.EnsureProductCustomerAgent(product.TenantID, product.ID, operator); err != nil {
		_ = repositories.ProductResourceProvisioningJobRepository.Updates(sqls.DB(), job.ID, map[string]any{
			"agent_status": productResourceStatusFailed,
			"updated_at":   time.Now(),
		})
		return err
	}
	_ = repositories.ProductResourceProvisioningJobRepository.Updates(sqls.DB(), job.ID, map[string]any{
		"agent_status": productResourceStatusReady,
		"updated_at":   time.Now(),
	})
	return nil
}

func (s *productResourceService) finishProvisioningJob(job *models.ProductResourceProvisioningJob) error {
	now := time.Now()
	return repositories.ProductResourceProvisioningJobRepository.Updates(sqls.DB(), job.ID, map[string]any{
		"status":           "succeeded",
		"knowledge_status": productResourceStatusReady,
		"ai_key_status":    productResourceStatusReady,
		"agent_status":     productResourceStatusReady,
		"error_summary":    "",
		"next_attempt_at":  nil,
		"locked_at":        nil,
		"lock_owner":       "",
		"finished_at":      now,
		"updated_at":       now,
	})
}

func (s *productResourceService) finishSkippedProvisioningJob(job *models.ProductResourceProvisioningJob) error {
	now := time.Now()
	return repositories.ProductResourceProvisioningJobRepository.Updates(sqls.DB(), job.ID, map[string]any{
		"status":           "succeeded",
		"knowledge_status": productResourceStatusMissing,
		"ai_key_status":    productResourceStatusMissing,
		"agent_status":     productResourceStatusMissing,
		"error_summary":    "",
		"next_attempt_at":  nil,
		"locked_at":        nil,
		"lock_owner":       "",
		"finished_at":      now,
		"updated_at":       now,
	})
}

func (s *productResourceService) failProvisioningJob(job *models.ProductResourceProvisioningJob, cause error) error {
	if cause == nil {
		cause = errors.New("product resource provisioning failed")
	}
	now := time.Now()
	retryCount := job.RetryCount + 1
	maxRetries := job.MaxRetries
	if maxRetries <= 0 {
		maxRetries = productResourceJobMaxRetries
	}
	status := "waiting_retry"
	nextAttemptAt := any(now.Add(productResourceProvisioningBackoff(retryCount)))
	finishedAt := any(nil)
	if retryCount >= maxRetries {
		status = productResourceStatusFailed
		nextAttemptAt = nil
		finishedAt = now
	}
	_ = repositories.ProductResourceProvisioningJobRepository.Updates(sqls.DB(), job.ID, map[string]any{
		"status":          status,
		"retry_count":     retryCount,
		"next_attempt_at": nextAttemptAt,
		"locked_at":       nil,
		"lock_owner":      "",
		"error_summary":   truncateErrorSummary(cause.Error()),
		"finished_at":     finishedAt,
		"updated_at":      now,
	})
	return cause
}

func (s *productResourceService) UpdateProductAIQuota(ctx context.Context, tenantID, productID int64, quotaLimit float64, operator *dto.AuthPrincipal) (*dto.EnterpriseProductResourcesDTO, error) {
	product, err := s.requireTenantProduct(tenantID, productID, operator)
	if err != nil {
		return nil, err
	}
	if !TenantCapabilityService.AIEnabled(product.TenantID) {
		return s.buildProductResourcesDTO(tenantID, productID), nil
	}
	if quotaLimit < 0 {
		return nil, errorsx.InvalidParam("quota_limit must be greater than or equal to 0")
	}
	credential := repositories.ProductAIUsageCredentialRepository.GetByProduct(sqls.DB(), tenantID, productID)
	if credential == nil || credential.Status == enums.StatusDeleted || strings.TrimSpace(credential.Sub2APIKeyID) == "" {
		if err := s.createOrResetProvisioningJob(product, quotaLimit, operator, true); err != nil {
			return nil, err
		}
		return s.buildProductResourcesDTO(tenantID, productID), nil
	}

	provider, token, err := s.resolveProductSub2APIProvider(ctx, tenantID, operator)
	if err != nil {
		_ = s.markProductCredentialFailed(credential, err, operator)
		return s.buildProductResourcesDTO(tenantID, productID), nil
	}
	keyID := parseInt64(credential.Sub2APIKeyID)
	key, err := s.findRemoteSub2APIKey(ctx, provider, token, keyID, defaultProductSub2APIKeyName(product))
	if err != nil {
		_ = s.markProductCredentialFailed(credential, err, operator)
		return s.buildProductResourcesDTO(tenantID, productID), nil
	}
	if key == nil {
		err := fmt.Errorf("sub2api key %s not found", credential.Sub2APIKeyID)
		_ = s.markProductCredentialFailed(credential, err, operator)
		return s.buildProductResourcesDTO(tenantID, productID), nil
	}
	updated, err := s.updateRemoteKeyQuota(ctx, provider, token, key, quotaLimit)
	if err != nil {
		_ = s.markProductCredentialFailed(credential, err, operator)
		return s.buildProductResourcesDTO(tenantID, productID), nil
	}
	if updated != nil {
		key = updated
		keyID = key.ID
	}
	if err := s.saveProductCredentialFromKey(credential, product, key, quotaLimit, operator); err != nil {
		return nil, err
	}
	if keyID > 0 && key.ID == 0 {
		_ = repositories.ProductAIUsageCredentialRepository.Updates(sqls.DB(), credential.ID, map[string]any{"sub2api_key_id": fmt.Sprintf("%d", keyID)})
	}
	return s.buildProductResourcesDTO(tenantID, productID), nil
}

func (s *productResourceService) ResetProductAIQuota(ctx context.Context, tenantID, productID int64, operator *dto.AuthPrincipal) (*dto.EnterpriseProductResourcesDTO, error) {
	product, err := s.requireTenantProduct(tenantID, productID, operator)
	if err != nil {
		return nil, err
	}
	if !TenantCapabilityService.AIEnabled(product.TenantID) {
		return s.buildProductResourcesDTO(tenantID, productID), nil
	}
	credential := repositories.ProductAIUsageCredentialRepository.GetByProduct(sqls.DB(), tenantID, productID)
	if credential == nil || credential.Status == enums.StatusDeleted || strings.TrimSpace(credential.Sub2APIKeyID) == "" {
		return s.RetryProductResourceProvisioning(ctx, tenantID, productID, operator)
	}
	provider, token, err := s.resolveProductSub2APIProvider(ctx, tenantID, operator)
	if err != nil {
		_ = s.markProductCredentialFailed(credential, err, operator)
		return s.buildProductResourcesDTO(tenantID, productID), nil
	}
	keyID := parseInt64(credential.Sub2APIKeyID)
	if keyID <= 0 {
		return nil, errorsx.InvalidParam("product sub2api key is invalid")
	}
	if _, err := provider.UpdateKey(ctx, token, keyID, providers.Sub2APIUpdateKeyRequest{ResetQuota: true}); err != nil {
		_ = s.markProductCredentialFailed(credential, err, operator)
		return s.buildProductResourcesDTO(tenantID, productID), nil
	}
	key, err := s.findRemoteSub2APIKey(ctx, provider, token, keyID, defaultProductSub2APIKeyName(product))
	if err != nil || key == nil {
		_ = repositories.ProductAIUsageCredentialRepository.Updates(sqls.DB(), credential.ID, map[string]any{
			"quota_used":       0,
			"provision_status": productResourceStatusReady,
			"provision_error":  "",
			"last_synced_at":   time.Now(),
		})
		return s.buildProductResourcesDTO(tenantID, productID), nil
	}
	if err := s.saveProductCredentialFromKey(credential, product, key, credential.QuotaLimit, operator); err != nil {
		return nil, err
	}
	return s.buildProductResourcesDTO(tenantID, productID), nil
}

func (s *productResourceService) createOrResetProvisioningJob(product *models.Product, quotaLimit float64, operator *dto.AuthPrincipal, force bool) error {
	if product == nil {
		return errorsx.InvalidParam("product is required")
	}
	if !TenantCapabilityService.AIEnabled(product.TenantID) {
		return nil
	}
	quotaLimit = normalizeProductAIQuotaLimit(quotaLimit)
	if err := s.ensureQueuedCredentialShell(product, quotaLimit, operator, force); err != nil {
		return err
	}
	now := time.Now()
	idempotencyKey := productResourceProvisioningIdempotencyKey(product.ID)
	job := repositories.ProductResourceProvisioningJobRepository.GetByIdempotencyKey(sqls.DB(), idempotencyKey)
	if job == nil {
		job = &models.ProductResourceProvisioningJob{
			TenantID:            product.TenantID,
			ProductID:           product.ID,
			IdempotencyKey:      idempotencyKey,
			Status:              productResourceStatusPending,
			RequestedQuotaLimit: quotaLimit,
			KnowledgeStatus:     productResourceStatusPending,
			AIKeyStatus:         productResourceStatusPending,
			AgentStatus:         productResourceStatusPending,
			MaxRetries:          productResourceJobMaxRetries,
			NextAttemptAt:       &now,
			AuditFields:         utils.BuildAuditFields(operator),
		}
		job.CreatedAt = now
		job.UpdatedAt = now
		if err := repositories.ProductResourceProvisioningJobRepository.Create(sqls.DB(), job); err != nil {
			if existing := repositories.ProductResourceProvisioningJobRepository.GetByIdempotencyKey(sqls.DB(), idempotencyKey); existing != nil {
				return nil
			}
			return err
		}
		return nil
	}
	if !force && (job.Status == productResourceStatusPending || job.Status == "running" || job.Status == "waiting_retry") {
		return nil
	}
	return repositories.ProductResourceProvisioningJobRepository.Updates(sqls.DB(), job.ID, map[string]any{
		"status":                productResourceStatusPending,
		"requested_quota_limit": quotaLimit,
		"knowledge_status":      productResourceStatusPending,
		"ai_key_status":         productResourceStatusPending,
		"agent_status":          productResourceStatusPending,
		"error_summary":         "",
		"retry_count":           0,
		"max_retries":           productResourceJobMaxRetries,
		"next_attempt_at":       now,
		"last_attempt_at":       nil,
		"locked_at":             nil,
		"lock_owner":            "",
		"started_at":            nil,
		"finished_at":           nil,
		"update_user_id":        operator.UserID,
		"update_user_name":      operatorUsername(operator),
		"updated_at":            now,
	})
}

func (s *productResourceService) ensureQueuedCredentialShell(product *models.Product, quotaLimit float64, operator *dto.AuthPrincipal, force bool) error {
	credential := repositories.ProductAIUsageCredentialRepository.GetByProduct(sqls.DB(), product.TenantID, product.ID)
	if credential == nil {
		if _, err := s.createProductCredentialShell(product, quotaLimit, operator); err != nil {
			credential = repositories.ProductAIUsageCredentialRepository.GetByProduct(sqls.DB(), product.TenantID, product.ID)
			if credential == nil {
				return err
			}
		} else {
			return nil
		}
	}
	updates := map[string]any{
		"key_name":          defaultProductSub2APIKeyName(product),
		"quota_limit":       quotaLimit,
		"currency":          productAICurrencyUSD,
		"quota_policy_json": buildProductQuotaPolicyJSON(credential.Sub2APIKeyID, quotaLimit, credential.QuotaUsed, productAICurrencyUSD),
		"updated_at":        time.Now(),
		"update_user_id":    operator.UserID,
		"update_user_name":  operatorUsername(operator),
	}
	if force || credential.ProvisionStatus == productResourceStatusFailed || strings.TrimSpace(credential.ProvisionStatus) == "" {
		updates["provision_status"] = productResourceStatusPending
		updates["provision_error"] = ""
	}
	return repositories.ProductAIUsageCredentialRepository.Updates(sqls.DB(), credential.ID, updates)
}

func (s *productResourceService) ensureProductKnowledgeDataset(ctx context.Context, product *models.Product, operator *dto.AuthPrincipal) (*models.KnowledgeBase, error) {
	tenantID := product.TenantID
	profile := repositories.ProductServiceProfileRepository.GetByProductID(sqls.DB(), product.ID)
	if profile == nil || profile.Status == enums.StatusDeleted || profile.DefaultKnowledgeBaseID <= 0 {
		locales := []string{strings.TrimSpace(product.DefaultLocale)}
		if locales[0] == "" {
			locales[0] = "en"
		}
		if _, err := ProductCenterService.CreateProductKnowledgeBase(tenantID, product.ID, dto.EnterpriseProductKnowledgeBaseCreateRequest{
			Name:           product.Name + " 知识库",
			Description:    fmt.Sprintf("保存 %s 的手册、FAQ、维修案例和诊断知识，服务团队和产品机器人都从这里查。", product.Name),
			SupportLocales: locales,
		}, operator); err != nil && !strings.Contains(err.Error(), "already exists") {
			return nil, err
		}
		profile = repositories.ProductServiceProfileRepository.GetByProductID(sqls.DB(), product.ID)
	}
	if profile == nil || profile.DefaultKnowledgeBaseID <= 0 {
		return nil, errorsx.InvalidParam("product knowledge base provisioning failed")
	}
	kb := repositories.KnowledgeBaseRepository.Get(sqls.DB(), profile.DefaultKnowledgeBaseID)
	if kb == nil || kb.TenantID != tenantID || kb.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParam("product knowledge base not found")
	}
	if strings.TrimSpace(kb.RagflowDatasetID) != "" {
		return kb, nil
	}
	provider := providers.NewInternalKnowledgeIndexProvider(nil)
	ref, err := provider.EnsureDataset(ctx, providers.EnsureDatasetRequest{
		TenantID: tenantID,
		Name:     fmt.Sprintf("product-%d-%s", product.ID, product.Code),
		Locale:   product.DefaultLocale,
	})
	if err != nil {
		return nil, err
	}
	if ref != nil && strings.TrimSpace(ref.ExternalDatasetID) != "" {
		if err := repositories.KnowledgeBaseRepository.Updates(sqls.DB(), kb.ID, map[string]any{
			"ragflow_dataset_id": ref.ExternalDatasetID,
			"update_user_id":     operator.UserID,
			"update_user_name":   operator.Username,
			"updated_at":         time.Now(),
		}); err != nil {
			return nil, err
		}
		kb.RagflowDatasetID = ref.ExternalDatasetID
	}
	return kb, nil
}

func (s *productResourceService) ensureProductSub2APIKey(ctx context.Context, product *models.Product, quotaLimit float64, operator *dto.AuthPrincipal) (*models.ProductAIUsageCredential, error) {
	quotaLimit = normalizeProductAIQuotaLimit(quotaLimit)
	credential := repositories.ProductAIUsageCredentialRepository.GetByProduct(sqls.DB(), product.TenantID, product.ID)
	if credential != nil && credential.Status != enums.StatusDeleted && strings.TrimSpace(credential.Sub2APIKeyID) != "" && strings.TrimSpace(credential.APIKeyRef) != "" && sameFloat(credential.QuotaLimit, quotaLimit) && credential.ProvisionStatus == productResourceStatusReady {
		return credential, nil
	}
	if credential == nil {
		var err error
		credential, err = s.createProductCredentialShell(product, quotaLimit, operator)
		if err != nil {
			return nil, err
		}
	}
	provider, account, token, err := s.resolveProductSub2APIProviderWithAccount(ctx, product.TenantID, operator)
	if err != nil {
		_ = s.markProductCredentialFailed(credential, err, operator)
		return credential, err
	}
	keyName := defaultProductSub2APIKeyName(product)
	keyID := parseInt64(credential.Sub2APIKeyID)
	key, err := s.findRemoteSub2APIKey(ctx, provider, token, keyID, keyName)
	if err != nil {
		_ = s.markProductCredentialFailed(credential, err, operator)
		return credential, err
	}
	if key == nil {
		keyResp, err := provider.CreateKey(ctx, token, providers.Sub2APICreateKeyRequest{
			Name:    keyName,
			GroupID: platformSub2APIDefaultKeyGroupID,
			Quota:   quotaLimit,
		})
		if err != nil {
			_ = s.markProductCredentialFailed(credential, err, operator)
			return credential, err
		}
		key = &keyResp.Data
	} else if !sameFloat(key.Quota, quotaLimit) {
		updated, err := s.updateRemoteKeyQuota(ctx, provider, token, key, quotaLimit)
		if err != nil {
			_ = s.markProductCredentialFailed(credential, err, operator)
			return credential, err
		}
		if updated != nil {
			key = updated
		}
	}
	if strings.TrimSpace(credential.Sub2APIAccount) == "" {
		credential.Sub2APIAccount = firstNonBlank(account.AccountName, account.LoginEmail, fmt.Sprintf("%d", product.TenantID))
	}
	if err := s.saveProductCredentialFromKey(credential, product, key, quotaLimit, operator); err != nil {
		return nil, err
	}
	return repositories.ProductAIUsageCredentialRepository.Get(sqls.DB(), credential.ID), nil
}

func (s *productResourceService) createProductCredentialShell(product *models.Product, quotaLimit float64, operator *dto.AuthPrincipal) (*models.ProductAIUsageCredential, error) {
	now := time.Now()
	policy := buildProductQuotaPolicyJSON("", quotaLimit, 0, productAICurrencyUSD)
	item := &models.ProductAIUsageCredential{
		TenantID:        product.TenantID,
		ProductID:       product.ID,
		KeyName:         defaultProductSub2APIKeyName(product),
		QuotaLimit:      quotaLimit,
		Currency:        productAICurrencyUSD,
		QuotaPolicyJSON: policy,
		ProvisionStatus: productResourceStatusPending,
		Status:          enums.StatusOk,
		AuditFields:     utils.BuildAuditFields(operator),
	}
	item.CreatedAt = now
	item.UpdatedAt = now
	if err := repositories.ProductAIUsageCredentialRepository.Create(sqls.DB(), item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *productResourceService) saveProductCredentialFromKey(credential *models.ProductAIUsageCredential, product *models.Product, key *providers.Sub2APIKey, quotaLimit float64, operator *dto.AuthPrincipal) error {
	if credential == nil || key == nil {
		return nil
	}
	keyName := firstNonBlank(key.Name, defaultProductSub2APIKeyName(product))
	keyID := key.ID
	if keyID <= 0 {
		keyID = parseInt64(credential.Sub2APIKeyID)
	}
	keyRef := credential.APIKeyRef
	keyFingerprint := credential.APIKeyFingerprint
	if strings.TrimSpace(key.Key) != "" {
		ciphertext, err := secretstore.Encrypt(key.Key)
		if err != nil {
			return err
		}
		keyRef = ciphertext
		keyFingerprint = secretstore.Fingerprint(key.Key)
	}
	if quotaLimit <= 0 {
		quotaLimit = key.Quota
	}
	if quotaLimit <= 0 {
		quotaLimit = defaultProductAIQuotaLimit()
	}
	now := time.Now()
	policy := buildProductQuotaPolicyJSON(fmt.Sprintf("%d", keyID), quotaLimit, key.QuotaUsed, productAICurrencyUSD)
	return repositories.ProductAIUsageCredentialRepository.Updates(sqls.DB(), credential.ID, map[string]any{
		"tenant_id":           product.TenantID,
		"product_id":          product.ID,
		"sub2api_account":     firstNonBlank(credential.Sub2APIAccount, fmt.Sprintf("%d", product.TenantID)),
		"sub2api_key_id":      fmt.Sprintf("%d", keyID),
		"key_name":            keyName,
		"api_key_ref":         keyRef,
		"api_key_fingerprint": keyFingerprint,
		"quota_limit":         quotaLimit,
		"quota_used":          key.QuotaUsed,
		"currency":            productAICurrencyUSD,
		"quota_policy_json":   policy,
		"provision_status":    productResourceStatusReady,
		"provision_error":     "",
		"status":              enums.StatusOk,
		"last_synced_at":      now,
		"updated_at":          now,
		"update_user_id":      operator.UserID,
		"update_user_name":    operator.Username,
	})
}

func (s *productResourceService) markProductCredentialFailed(credential *models.ProductAIUsageCredential, cause error, operator *dto.AuthPrincipal) error {
	if credential == nil {
		return nil
	}
	message := "product ai key provisioning failed"
	if cause != nil {
		message = cause.Error()
	}
	return repositories.ProductAIUsageCredentialRepository.Updates(sqls.DB(), credential.ID, map[string]any{
		"provision_status": productResourceStatusFailed,
		"provision_error":  message,
		"updated_at":       time.Now(),
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
	})
}

func (s *productResourceService) resolveProductSub2APIProvider(ctx context.Context, tenantID int64, operator *dto.AuthPrincipal) (providers.Sub2APIProvider, string, error) {
	provider, _, token, err := EnterpriseAIModelService.resolveTenantProvider(ctx, tenantID, operator)
	return provider, token, err
}

func (s *productResourceService) resolveProductSub2APIProviderWithAccount(ctx context.Context, tenantID int64, operator *dto.AuthPrincipal) (providers.Sub2APIProvider, *models.Sub2APITenantAccount, string, error) {
	return EnterpriseAIModelService.resolveTenantProvider(ctx, tenantID, operator)
}

func (s *productResourceService) findRemoteSub2APIKey(ctx context.Context, provider providers.Sub2APIProvider, token string, keyID int64, keyName string) (*providers.Sub2APIKey, error) {
	keysResp, err := provider.ListKeys(ctx, token, providers.Sub2APIListKeysQuery{
		Page:      1,
		PageSize:  100,
		SortBy:    "created_at",
		SortOrder: "desc",
		Timezone:  defaultTimezone(""),
	})
	if err != nil {
		return nil, err
	}
	if keyID > 0 {
		if key := findSub2APIKey(keysResp.Data.Items, keyID); key != nil {
			return key, nil
		}
	}
	keyName = strings.TrimSpace(keyName)
	if keyName == "" {
		return nil, nil
	}
	for i := range keysResp.Data.Items {
		if strings.EqualFold(strings.TrimSpace(keysResp.Data.Items[i].Name), keyName) {
			return &keysResp.Data.Items[i], nil
		}
	}
	return nil, nil
}

func (s *productResourceService) updateRemoteKeyQuota(ctx context.Context, provider providers.Sub2APIProvider, token string, current *providers.Sub2APIKey, quota float64) (*providers.Sub2APIKey, error) {
	if current == nil {
		return nil, errors.New("sub2api key is required")
	}
	ipWhitelist := stringSliceFromAny(current.IPWhitelist)
	ipBlacklist := stringSliceFromAny(current.IPBlacklist)
	expiresAt := ""
	if current.ExpiresAt != nil {
		expiresAt = *current.ExpiresAt
	}
	resp, err := provider.UpdateKey(ctx, token, current.ID, providers.Sub2APIUpdateKeyRequest{
		Name:        current.Name,
		GroupID:     current.GroupID,
		IPWhitelist: &ipWhitelist,
		IPBlacklist: &ipBlacklist,
		Quota:       &quota,
		ExpiresAt:   expiresAt,
		RateLimit5h: current.RateLimit5h,
		RateLimit1d: current.RateLimit1d,
		RateLimit7d: current.RateLimit7d,
		Status:      current.Status,
	})
	if err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (s *productResourceService) requireTenantProduct(tenantID, productID int64, operator *dto.AuthPrincipal) (*models.Product, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if tenantID <= 0 || operator.TenantID != tenantID {
		return nil, errorsx.InvalidParam("tenant does not match current operator")
	}
	product := repositories.ProductRepository.GetByTenant(sqls.DB(), productID, tenantID)
	if product == nil || product.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParam("product not found")
	}
	return product, nil
}

func (s *productResourceService) buildProductResourcesDTO(tenantID, productID int64) *dto.EnterpriseProductResourcesDTO {
	ret := &dto.EnterpriseProductResourcesDTO{ProductID: productID}
	ret.Knowledge = buildProductKnowledgeResourceDTO(tenantID, productID)
	ret.AIKey = buildProductAIKeyResourceDTO(tenantID, productID)
	ret.Agent = buildProductAgentResourceDTO(tenantID, productID)
	ret.Status = summarizeProductResourceStatus(ret)
	ret.Job = buildProductResourceJobDTO(tenantID, productID)
	ret.Status = mergeProductResourceJobStatus(ret.Status, ret.Job)
	return ret
}

func buildProductResourceJobDTO(tenantID, productID int64) *dto.EnterpriseProductResourceJobDTO {
	job := repositories.ProductResourceProvisioningJobRepository.LatestByProduct(sqls.DB(), tenantID, productID)
	if job == nil {
		return nil
	}
	return &dto.EnterpriseProductResourceJobDTO{
		ID:                  job.ID,
		Status:              job.Status,
		RequestedQuotaLimit: job.RequestedQuotaLimit,
		KnowledgeStatus:     job.KnowledgeStatus,
		AIKeyStatus:         job.AIKeyStatus,
		AgentStatus:         job.AgentStatus,
		ErrorSummary:        job.ErrorSummary,
		RetryCount:          job.RetryCount,
		MaxRetries:          job.MaxRetries,
		NextAttemptAt:       formatEnterpriseTimePtr(job.NextAttemptAt),
		LastAttemptAt:       formatEnterpriseTimePtr(job.LastAttemptAt),
		StartedAt:           formatEnterpriseTimePtr(job.StartedAt),
		FinishedAt:          formatEnterpriseTimePtr(job.FinishedAt),
		UpdatedAt:           formatEnterpriseTime(job.UpdatedAt),
	}
}

func buildProductKnowledgeResourceDTO(tenantID, productID int64) dto.EnterpriseProductKnowledgeDTO {
	ret := dto.EnterpriseProductKnowledgeDTO{Status: productResourceStatusMissing}
	profile := repositories.ProductServiceProfileRepository.GetByProductID(sqls.DB(), productID)
	if profile == nil || profile.TenantID != tenantID || profile.Status == enums.StatusDeleted || profile.DefaultKnowledgeBaseID <= 0 {
		return ret
	}
	kb := repositories.KnowledgeBaseRepository.Get(sqls.DB(), profile.DefaultKnowledgeBaseID)
	if kb == nil || kb.TenantID != tenantID || kb.Status == enums.StatusDeleted {
		return ret
	}
	datasetID := strings.TrimSpace(kb.RagflowDatasetID)
	ret.KnowledgeBaseID = kb.ID
	ret.KnowledgeBaseName = kb.Name
	ret.DatasetID = datasetID
	ret.Provider = datasetProviderFromID(datasetID)
	ret.Status = productResourceStatusPending
	if datasetID != "" {
		ret.Status = productResourceStatusReady
	}
	return ret
}

func buildProductAIKeyResourceDTO(tenantID, productID int64) dto.EnterpriseProductAIKeyDTO {
	ret := dto.EnterpriseProductAIKeyDTO{
		Status:          productResourceStatusMissing,
		ProvisionStatus: productResourceStatusMissing,
		Currency:        productAICurrencyUSD,
		QuotaLimit:      defaultProductAIQuotaLimit(),
	}
	credential := repositories.ProductAIUsageCredentialRepository.GetByProduct(sqls.DB(), tenantID, productID)
	if credential == nil || credential.Status == enums.StatusDeleted {
		return ret
	}
	ret.Status = credential.ProvisionStatus
	ret.ProvisionStatus = credential.ProvisionStatus
	ret.KeyID = credential.Sub2APIKeyID
	ret.KeyName = credential.KeyName
	ret.KeyPreview = productKeyPreview(credential)
	ret.QuotaLimit = credential.QuotaLimit
	ret.QuotaUsed = credential.QuotaUsed
	ret.Currency = firstNonBlank(credential.Currency, productAICurrencyUSD)
	ret.ProvisionError = credential.ProvisionError
	if credential.LastSyncedAt != nil {
		ret.LastSyncedAt = formatEnterpriseTime(*credential.LastSyncedAt)
	}
	if ret.Status == "" {
		ret.Status = productResourceStatusPending
	}
	if ret.QuotaLimit <= 0 {
		ret.QuotaLimit = defaultProductAIQuotaLimit()
	}
	return ret
}

func buildProductAgentResourceDTO(tenantID, productID int64) dto.EnterpriseProductAgentDTO {
	ret := dto.EnterpriseProductAgentDTO{Status: productResourceStatusMissing}
	agent := repositories.AIAgentRepository.Take(sqls.DB(), "tenant_id = ? AND product_id = ? AND status <> ?", tenantID, productID, enums.StatusDeleted)
	if agent == nil {
		return ret
	}
	ret.AgentID = agent.ID
	ret.AgentName = agent.Name
	ret.ReviewStatus = string(agent.ReviewStatus)
	ret.Enabled = agent.Status == enums.StatusOk
	ret.Status = productResourceStatusReady
	if agent.ReviewStatus != enums.AIAgentReviewStatusApproved {
		ret.Status = string(agent.ReviewStatus)
	}
	return ret
}

func summarizeProductResourceStatus(resources *dto.EnterpriseProductResourcesDTO) string {
	if resources == nil {
		return productResourceStatusMissing
	}
	readyCount := 0
	failedCount := 0
	pendingCount := 0
	for _, status := range []string{resources.AIKey.Status, resources.Knowledge.Status, resources.Agent.Status} {
		switch status {
		case productResourceStatusReady, string(enums.AIAgentReviewStatusUnreviewed), string(enums.AIAgentReviewStatusPending), string(enums.AIAgentReviewStatusApproved), string(enums.AIAgentReviewStatusRejected):
			readyCount++
		case productResourceStatusFailed:
			failedCount++
		default:
			pendingCount++
		}
	}
	if failedCount == 0 && pendingCount == 0 {
		return productResourceStatusReady
	}
	if failedCount > 0 && readyCount > 0 {
		return productResourceStatusPartialFail
	}
	if failedCount > 0 {
		return productResourceStatusFailed
	}
	return productResourceStatusPending
}

func mergeProductResourceJobStatus(current string, job *dto.EnterpriseProductResourceJobDTO) string {
	if job == nil || current == productResourceStatusReady {
		return current
	}
	switch job.Status {
	case productResourceStatusFailed:
		if current == productResourceStatusPending || current == productResourceStatusMissing {
			return productResourceStatusFailed
		}
		return productResourceStatusPartialFail
	case productResourceStatusPending, "running", "waiting_retry":
		return productResourceStatusPending
	default:
		return current
	}
}

func defaultProductSub2APIKeyName(product *models.Product) string {
	return fmt.Sprintf("RHD-PRODUCT-%d", product.ID)
}

func productResourceProvisioningIdempotencyKey(productID int64) string {
	return fmt.Sprintf("product:%d:bootstrap:v1", productID)
}

func productProvisioningOperator(tenantID int64) *dto.AuthPrincipal {
	return &dto.AuthPrincipal{
		UserID:      0,
		Username:    "product-resource-worker",
		TenantID:    tenantID,
		DomainType:  "enterprise",
		SubjectType: "service_account",
	}
}

func productResourceProvisioningBackoff(retryCount int) time.Duration {
	if retryCount <= 0 {
		return time.Minute
	}
	delay := time.Minute * time.Duration(1<<(retryCount-1))
	if delay > 30*time.Minute {
		return 30 * time.Minute
	}
	return delay
}

func normalizeProductAIQuotaLimit(value float64) float64 {
	if value > 0 {
		return value
	}
	return defaultProductAIQuotaLimit()
}

func defaultProductAIQuotaLimit() float64 {
	configItem := repositories.SystemConfigRepository.FindByKey(sqls.DB(), productDefaultAIQuotaConfigKey)
	if configItem == nil {
		return productDefaultAIQuotaUSD
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(configItem.ConfigValue), 64)
	if err != nil || value <= 0 {
		return productDefaultAIQuotaUSD
	}
	return value
}

func buildProductQuotaPolicyJSON(keyID string, quotaLimit, quotaUsed float64, currency string) string {
	payload := map[string]any{
		"sub2api_key_id": strings.TrimSpace(keyID),
		"quota_limit":    quotaLimit,
		"quota_used":     quotaUsed,
		"currency":       firstNonBlank(currency, productAICurrencyUSD),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func datasetProviderFromID(datasetID string) string {
	if strings.TrimSpace(datasetID) == "" {
		return ""
	}
	if strings.HasPrefix(datasetID, "internal:") {
		return "internal"
	}
	return "ragflow"
}

func productKeyPreview(credential *models.ProductAIUsageCredential) string {
	if credential == nil {
		return ""
	}
	if plain, err := secretstore.Decrypt(credential.APIKeyRef); err == nil && strings.TrimSpace(plain) != "" && !strings.HasPrefix(plain, "secret://") {
		plain = strings.TrimSpace(plain)
		if len(plain) <= 8 {
			return "****"
		}
		return plain[:3] + "-****" + plain[len(plain)-4:]
	}
	if credential.APIKeyFingerprint != "" {
		if len(credential.APIKeyFingerprint) <= 4 {
			return "sk-****" + credential.APIKeyFingerprint
		}
		return "sk-****" + credential.APIKeyFingerprint[len(credential.APIKeyFingerprint)-4:]
	}
	return ""
}

func sameFloat(left, right float64) bool {
	if left > right {
		return left-right < 0.000001
	}
	return right-left < 0.000001
}
