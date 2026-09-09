package services

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"gorm.io/gorm"
)

const (
	TenantExternalPortalModeExternal = "external"
	TenantExternalPortalModeIframe   = "iframe"
	TenantBrandLogoStoragePrefix     = "tenant-branding/logo"
)

var TenantPortalSettingsService = &tenantPortalSettingsService{}

type tenantPortalSettingsService struct{}

func (s *tenantPortalSettingsService) GetDB(db *gorm.DB, tenantID int64) (*models.TenantBranding, *models.TenantIntegrationConfig) {
	if db == nil || tenantID <= 0 {
		return nil, nil
	}
	return repositories.PlatformIAMRepository.GetTenantBranding(db, tenantID),
		repositories.TenantIntegrationConfigRepository.GetByProvider(db, tenantID, models.TenantIntegrationProviderOnePanel)
}

func (s *tenantPortalSettingsService) CustomerDefaultLocaleDB(db *gorm.DB, tenant *models.Tenant) string {
	if tenant == nil {
		return "en-US"
	}
	fallback := defaultString(tenant.DefaultLocale, "en-US")
	branding := repositories.PlatformIAMRepository.GetTenantBranding(db, tenant.ID)
	if branding == nil || branding.Status != enums.StatusOk {
		return fallback
	}
	return defaultString(branding.DefaultLocale, fallback)
}

func (s *tenantPortalSettingsService) FindByTenantIDsDB(db *gorm.DB, tenantIDs []int64) (map[int64]*models.TenantBranding, map[int64]*models.TenantIntegrationConfig, error) {
	brandings, err := repositories.PlatformIAMRepository.FindTenantBrandingsByTenantIDs(db, tenantIDs)
	if err != nil {
		return nil, nil, err
	}
	consoles, err := repositories.TenantIntegrationConfigRepository.FindByProviderTenantIDs(db, models.TenantIntegrationProviderOnePanel, tenantIDs)
	if err != nil {
		return nil, nil, err
	}
	return brandings, consoles, nil
}

func (s *tenantPortalSettingsService) SaveBrandingDB(db *gorm.DB, tenantID int64, brandName string, logoAssetID int64, logoURL, customDomain, customerTheme, defaultLocale string, operator *dto.AuthPrincipal) (*models.TenantBranding, error) {
	if db == nil || tenantID <= 0 {
		return nil, errors.New("tenant is required")
	}
	brandName = strings.TrimSpace(brandName)
	customDomain = strings.TrimSpace(customDomain)
	customerTheme = models.NormalizeTenantCustomerTheme(customerTheme)
	defaultLocale = defaultString(defaultLocale, "en-US")
	if logoAssetID > 0 {
		asset := repositories.AssetRepository.Get(db, logoAssetID)
		if asset == nil || asset.Status != enums.AssetStatusSuccess || !strings.HasPrefix(strings.ToLower(asset.MimeType), "image/") {
			return nil, errorsx.InvalidParam("tenant logo asset is invalid")
		}
		if asset.TenantID != 0 && asset.TenantID != tenantID {
			return nil, errorsx.Forbidden("tenant logo asset does not belong to this tenant")
		}
		logoURL = TenantBrandLogoURL(asset.AssetID)
	}
	logoURL, err := normalizeTenantLogoURL(logoURL)
	if err != nil {
		return nil, err
	}
	existing := repositories.PlatformIAMRepository.GetTenantBranding(db, tenantID)
	if existing == nil {
		brandConfigJSON, err := tenantBrandConfigJSON("{}", customerTheme)
		if err != nil {
			return nil, err
		}
		item := &models.TenantBranding{
			TenantID:        tenantID,
			BrandName:       brandName,
			LogoAssetID:     logoAssetID,
			LogoURL:         logoURL,
			CustomDomain:    customDomain,
			DefaultLocale:   defaultLocale,
			BrandConfigJSON: brandConfigJSON,
			Status:          enums.StatusOk,
			AuditFields:     utils.BuildAuditFields(operator),
		}
		if err := repositories.PlatformIAMRepository.CreateTenantBranding(db, item); err != nil {
			return nil, err
		}
		return item, nil
	}
	brandConfigJSON, err := tenantBrandConfigJSON(existing.BrandConfigJSON, customerTheme)
	if err != nil {
		return nil, err
	}
	if err := repositories.PlatformIAMRepository.UpdateTenantBranding(db, tenantID, map[string]any{
		"brand_name":        brandName,
		"logo_asset_id":     logoAssetID,
		"logo_url":          logoURL,
		"custom_domain":     customDomain,
		"default_locale":    defaultLocale,
		"brand_config_json": brandConfigJSON,
		"status":            enums.StatusOk,
		"updated_at":        time.Now(),
		"update_user_id":    auditUserID(operator),
		"update_user_name":  auditUserName(operator),
	}); err != nil {
		return nil, err
	}
	return repositories.PlatformIAMRepository.GetTenantBranding(db, tenantID), nil
}

func TenantBrandLogoURL(assetID string) string {
	assetID = strings.TrimSpace(assetID)
	if assetID == "" {
		return ""
	}
	return "/api/tenant-branding/logo/" + url.PathEscape(assetID)
}

func (s *tenantPortalSettingsService) ResolvePublicLogoAssetDB(db *gorm.DB, assetID string) *models.Asset {
	if db == nil || strings.TrimSpace(assetID) == "" {
		return nil
	}
	asset := repositories.AssetRepository.GetByAssetID(db, strings.TrimSpace(assetID))
	if asset == nil || asset.Status != enums.AssetStatusSuccess || !strings.HasPrefix(strings.ToLower(asset.MimeType), "image/") {
		return nil
	}
	branding := repositories.PlatformIAMRepository.FindActiveTenantBrandingByLogoAssetID(db, asset.ID)
	if branding == nil {
		return nil
	}
	tenant := repositories.PlatformIAMRepository.GetTenant(db, branding.TenantID)
	if tenant == nil || tenant.Status == enums.StatusDeleted {
		return nil
	}
	return asset
}

func tenantBrandConfigJSON(existingJSON, customerTheme string) (string, error) {
	config := map[string]any{}
	if value := strings.TrimSpace(existingJSON); value != "" {
		if err := json.Unmarshal([]byte(value), &config); err != nil {
			return "", errors.New("tenant brand configuration is invalid")
		}
	}
	if config == nil {
		config = map[string]any{}
	}
	config["customerTheme"] = models.NormalizeTenantCustomerTheme(customerTheme)
	encoded, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func (s *tenantPortalSettingsService) SaveOnePanelDB(db *gorm.DB, tenantID int64, displayName, baseURL, embedMode string, enabled bool, operator *dto.AuthPrincipal) (*models.TenantIntegrationConfig, error) {
	if db == nil || tenantID <= 0 {
		return nil, errors.New("tenant is required")
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = "1Panel"
	}
	baseURL, scheme, err := normalizeTenantExternalPortalURL(baseURL)
	if err != nil {
		return nil, err
	}
	if enabled && baseURL == "" {
		return nil, errors.New("server console URL is required when enabled")
	}
	embedMode = normalizeTenantExternalPortalMode(embedMode, scheme)
	metadata, err := json.Marshal(models.TenantExternalPortalMetadata{
		DisplayName: displayName,
		EmbedMode:   embedMode,
	})
	if err != nil {
		return nil, err
	}
	existing := repositories.TenantIntegrationConfigRepository.GetByProvider(db, tenantID, models.TenantIntegrationProviderOnePanel)
	if existing == nil {
		if baseURL == "" && !enabled {
			return nil, nil
		}
		item := &models.TenantIntegrationConfig{
			TenantID:     tenantID,
			Provider:     models.TenantIntegrationProviderOnePanel,
			BaseURL:      baseURL,
			Enabled:      enabled,
			Status:       enums.StatusOk,
			MetadataJSON: string(metadata),
			AuditFields:  utils.BuildAuditFields(operator),
		}
		if err := repositories.TenantIntegrationConfigRepository.Create(db, item); err != nil {
			return nil, err
		}
		return item, nil
	}
	if err := repositories.TenantIntegrationConfigRepository.Updates(db, existing.ID, map[string]any{
		"base_url":         baseURL,
		"enabled":          enabled,
		"metadata_json":    string(metadata),
		"status":           enums.StatusOk,
		"updated_at":       time.Now(),
		"update_user_id":   auditUserID(operator),
		"update_user_name": auditUserName(operator),
	}); err != nil {
		return nil, err
	}
	return repositories.TenantIntegrationConfigRepository.Get(db, existing.ID), nil
}

func TenantExternalPortalMetadata(item *models.TenantIntegrationConfig) models.TenantExternalPortalMetadata {
	ret := models.TenantExternalPortalMetadata{DisplayName: "1Panel", EmbedMode: TenantExternalPortalModeExternal}
	if item == nil {
		return ret
	}
	_ = json.Unmarshal([]byte(item.MetadataJSON), &ret)
	ret.DisplayName = strings.TrimSpace(ret.DisplayName)
	if ret.DisplayName == "" {
		ret.DisplayName = "1Panel"
	}
	parsed, _ := url.Parse(strings.TrimSpace(item.BaseURL))
	ret.EmbedMode = normalizeTenantExternalPortalMode(ret.EmbedMode, strings.ToLower(parsed.Scheme))
	return ret
}

func TenantExternalPortalURL(item *models.TenantIntegrationConfig) string {
	if item == nil {
		return ""
	}
	normalized, _, err := normalizeTenantExternalPortalURL(item.BaseURL)
	if err != nil {
		return ""
	}
	return normalized
}

func normalizeTenantLogoURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
		return value, nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("tenant logo URL must be an application path or an HTTP(S) URL")
	}
	return parsed.String(), nil
}

func normalizeTenantExternalPortalURL(value string) (string, string, error) {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return "", "", nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", "", errors.New("server console URL must be an HTTP(S) URL")
	}
	return parsed.String(), strings.ToLower(parsed.Scheme), nil
}

func normalizeTenantExternalPortalMode(value, scheme string) string {
	if strings.TrimSpace(value) == TenantExternalPortalModeIframe && scheme == "https" {
		return TenantExternalPortalModeIframe
	}
	return TenantExternalPortalModeExternal
}
