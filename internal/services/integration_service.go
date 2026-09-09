package services

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/mlogclub/simple/sqls"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"
)

var TenantIntegrationConfigService = &tenantIntegrationConfigService{}
var ProductAIUsageCredentialService = &productAIUsageCredentialService{}

type tenantIntegrationConfigService struct{}

func (s *tenantIntegrationConfigService) Get(id int64) *models.TenantIntegrationConfig {
	if id <= 0 {
		return nil
	}
	return repositories.TenantIntegrationConfigRepository.Get(sqls.DB(), id)
}
func (s *tenantIntegrationConfigService) FindPage(cnd *sqls.Cnd) ([]models.TenantIntegrationConfig, *sqls.Paging) {
	return repositories.TenantIntegrationConfigRepository.FindPage(sqls.DB(), cnd)
}
func (s *tenantIntegrationConfigService) Create(req request.CreateTenantIntegrationConfigRequest, op *dto.AuthPrincipal) (*models.TenantIntegrationConfig, error) {
	if op == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	provider := strings.TrimSpace(req.Provider)
	if _, err := requireActiveTenant(req.TenantID); err != nil {
		return nil, err
	}
	if provider == "" {
		return nil, errorsx.InvalidParam("provider is required")
	}
	if repositories.TenantIntegrationConfigRepository.GetByProvider(sqls.DB(), req.TenantID, provider) != nil {
		return nil, errorsx.InvalidParam("integration config already exists")
	}
	meta, err := normalizeJSON(req.MetadataJSON, "{}")
	if err != nil {
		return nil, err
	}
	secretRef, secretFingerprint := buildSecretReference("tenant-integration", req.TenantID, provider, req.AppSecret)
	item := &models.TenantIntegrationConfig{TenantID: req.TenantID, Provider: provider, BaseURL: strings.TrimRight(strings.TrimSpace(req.BaseURL), "/"), AppID: strings.TrimSpace(req.AppID), AppKey: strings.TrimSpace(req.AppKey), AppSecretRef: secretRef, AppSecretFingerprint: secretFingerprint, Enabled: req.Enabled, Status: enums.StatusOk, MetadataJSON: meta, AuditFields: utils.BuildAuditFields(op)}
	if err := repositories.TenantIntegrationConfigRepository.Create(sqls.DB(), item); err != nil {
		return nil, err
	}
	return item, nil
}
func (s *tenantIntegrationConfigService) Update(req request.UpdateTenantIntegrationConfigRequest, op *dto.AuthPrincipal) error {
	if op == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	old := s.Get(req.ID)
	if old == nil || old.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("integration config does not exist")
	}
	if _, err := requireActiveTenant(req.TenantID); err != nil {
		return err
	}
	meta, err := normalizeJSON(req.MetadataJSON, old.MetadataJSON)
	if err != nil {
		return err
	}
	provider := strings.TrimSpace(req.Provider)
	m := map[string]any{"tenant_id": req.TenantID, "provider": provider, "base_url": strings.TrimRight(strings.TrimSpace(req.BaseURL), "/"), "app_id": strings.TrimSpace(req.AppID), "app_key": strings.TrimSpace(req.AppKey), "enabled": req.Enabled, "metadata_json": meta, "updated_at": time.Now(), "update_user_id": op.UserID, "update_user_name": op.Username}
	if req.AppSecret != "" {
		secretRef, secretFingerprint := buildSecretReference("tenant-integration", req.TenantID, provider, req.AppSecret)
		m["app_secret_ref"] = secretRef
		m["app_secret_fingerprint"] = secretFingerprint
	}
	return repositories.TenantIntegrationConfigRepository.Updates(sqls.DB(), req.ID, m)
}
func (s *tenantIntegrationConfigService) Delete(id int64, op *dto.AuthPrincipal) error {
	return s.status(id, int(enums.StatusDeleted), op)
}
func (s *tenantIntegrationConfigService) UpdateStatus(id int64, status int, op *dto.AuthPrincipal) error {
	return s.status(id, status, op)
}
func (s *tenantIntegrationConfigService) status(id int64, status int, op *dto.AuthPrincipal) error {
	if op == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if !isMutableStatus(status) && status != int(enums.StatusDeleted) {
		return errorsx.InvalidParam("invalid status")
	}
	if s.Get(id) == nil {
		return errorsx.InvalidParam("integration config does not exist")
	}
	return repositories.TenantIntegrationConfigRepository.Updates(sqls.DB(), id, map[string]any{"status": status, "updated_at": time.Now(), "update_user_id": op.UserID, "update_user_name": op.Username})
}

type productAIUsageCredentialService struct{}

func (s *productAIUsageCredentialService) Get(id int64) *models.ProductAIUsageCredential {
	if id <= 0 {
		return nil
	}
	return repositories.ProductAIUsageCredentialRepository.Get(sqls.DB(), id)
}
func (s *productAIUsageCredentialService) GetByProduct(tenantID, productID int64) *models.ProductAIUsageCredential {
	if tenantID <= 0 || productID <= 0 {
		return nil
	}
	return repositories.ProductAIUsageCredentialRepository.GetByProduct(sqls.DB(), tenantID, productID)
}
func (s *productAIUsageCredentialService) FindPage(cnd *sqls.Cnd) ([]models.ProductAIUsageCredential, *sqls.Paging) {
	return repositories.ProductAIUsageCredentialRepository.FindPage(sqls.DB(), cnd)
}
func (s *productAIUsageCredentialService) Create(req request.CreateProductAIUsageCredentialRequest, op *dto.AuthPrincipal) (*models.ProductAIUsageCredential, error) {
	if op == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if err := s.requireProduct(req.TenantID, req.ProductID); err != nil {
		return nil, err
	}
	if repositories.ProductAIUsageCredentialRepository.GetByProduct(sqls.DB(), req.TenantID, req.ProductID) != nil {
		return nil, errorsx.InvalidParam("product usage credential already exists")
	}
	quota, err := normalizeJSON(req.QuotaPolicyJSON, "{}")
	if err != nil {
		return nil, err
	}
	apiKeyRef, apiKeyFingerprint := buildSecretReference("product-ai-credential", req.TenantID, fmt.Sprintf("%d", req.ProductID), req.APIKey)
	item := &models.ProductAIUsageCredential{TenantID: req.TenantID, ProductID: req.ProductID, Sub2APIAccount: strings.TrimSpace(req.Sub2APIAccount), APIKeyRef: apiKeyRef, APIKeyFingerprint: apiKeyFingerprint, QuotaPolicyJSON: quota, Status: enums.StatusOk, AuditFields: utils.BuildAuditFields(op)}
	if err := repositories.ProductAIUsageCredentialRepository.Create(sqls.DB(), item); err != nil {
		return nil, err
	}
	return item, nil
}
func (s *productAIUsageCredentialService) Update(req request.UpdateProductAIUsageCredentialRequest, op *dto.AuthPrincipal) error {
	if op == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	old := s.Get(req.ID)
	if old == nil || old.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("product usage credential does not exist")
	}
	if err := s.requireProduct(req.TenantID, req.ProductID); err != nil {
		return err
	}
	quota, err := normalizeJSON(req.QuotaPolicyJSON, old.QuotaPolicyJSON)
	if err != nil {
		return err
	}
	m := map[string]any{"tenant_id": req.TenantID, "product_id": req.ProductID, "sub2api_account": strings.TrimSpace(req.Sub2APIAccount), "quota_policy_json": quota, "updated_at": time.Now(), "update_user_id": op.UserID, "update_user_name": op.Username}
	if req.APIKey != "" {
		apiKeyRef, apiKeyFingerprint := buildSecretReference("product-ai-credential", req.TenantID, fmt.Sprintf("%d", req.ProductID), req.APIKey)
		m["api_key_ref"] = apiKeyRef
		m["api_key_fingerprint"] = apiKeyFingerprint
	}
	return repositories.ProductAIUsageCredentialRepository.Updates(sqls.DB(), req.ID, m)
}
func (s *productAIUsageCredentialService) Delete(id int64, op *dto.AuthPrincipal) error {
	return s.status(id, int(enums.StatusDeleted), op)
}
func (s *productAIUsageCredentialService) UpdateStatus(id int64, status int, op *dto.AuthPrincipal) error {
	return s.status(id, status, op)
}
func (s *productAIUsageCredentialService) status(id int64, status int, op *dto.AuthPrincipal) error {
	if op == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if !isMutableStatus(status) && status != int(enums.StatusDeleted) {
		return errorsx.InvalidParam("invalid status")
	}
	if s.Get(id) == nil {
		return errorsx.InvalidParam("product usage credential does not exist")
	}
	return repositories.ProductAIUsageCredentialRepository.Updates(sqls.DB(), id, map[string]any{"status": status, "updated_at": time.Now(), "update_user_id": op.UserID, "update_user_name": op.Username})
}
func (s *productAIUsageCredentialService) requireProduct(tenantID, productID int64) error {
	if _, err := requireActiveTenant(tenantID); err != nil {
		return err
	}
	p := ProductService.Get(productID)
	if p == nil || p.TenantID != tenantID || p.Status != enums.StatusOk {
		return errorsx.InvalidParam("product does not exist")
	}
	return nil
}

func buildSecretReference(scope string, tenantID int64, subject string, secret string) (string, string) {
	trimmed := strings.TrimSpace(secret)
	if trimmed == "" {
		return "", ""
	}
	sum := sha256.Sum256([]byte(trimmed))
	fingerprint := hex.EncodeToString(sum[:])
	return fmt.Sprintf("secret://%s/%d/%s/%s", scope, tenantID, url.PathEscape(strings.TrimSpace(subject)), fingerprint[:16]), fingerprint
}
