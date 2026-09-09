package migration

import (
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(124, "configure ODT customer portal theme and default locale", func() error {
		return configureODTCustomerPortal(sqls.DB())
	})
}

func configureODTCustomerPortal(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.Tenant{}) || !db.Migrator().HasTable(&models.TenantBranding{}) {
		return nil
	}
	var brandings []models.TenantBranding
	if err := db.Where("status <> ?", enums.StatusDeleted).Order("tenant_id ASC").Find(&brandings).Error; err != nil {
		return err
	}
	for i := range brandings {
		branding := &brandings[i]
		if !isODTDomain(branding.CustomDomain) {
			continue
		}
		operator := &dto.AuthPrincipal{
			TenantID:   branding.TenantID,
			Username:   "migration-124",
			DomainType: models.DomainTypePlatform,
			Domain:     models.DomainTypePlatform,
		}
		if _, err := services.TenantPortalSettingsService.SaveBrandingDB(
			db,
			branding.TenantID,
			branding.BrandName,
			0,
			"/images/tenants/odt/onward-logo-web.png",
			branding.CustomDomain,
			models.TenantCustomerThemeODTIntelligence,
			"en-US",
			operator,
		); err != nil {
			return err
		}
		tenant := repositories.PlatformIAMRepository.GetTenant(db, branding.TenantID)
		if tenant == nil {
			continue
		}
		supportedLocales := []string{"en-US"}
		for _, locale := range utils.ParseStringListJSON(tenant.SupportedLocalesJSON) {
			if locale != "en-US" {
				supportedLocales = append(supportedLocales, locale)
			}
		}
		if err := db.Model(&models.Tenant{}).Where("id = ?", tenant.ID).Updates(map[string]any{
			"default_locale":         "en-US",
			"supported_locales_json": utils.MarshalStringListJSON(supportedLocales),
		}).Error; err != nil {
			return err
		}
		if err := db.Model(&models.TenantBranding{}).Where("tenant_id = ?", tenant.ID).Update("default_locale", "en-US").Error; err != nil {
			return err
		}
	}
	return nil
}

func isODTDomain(value string) bool {
	domain := strings.ToLower(strings.TrimSpace(value))
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	if slash := strings.IndexByte(domain, '/'); slash >= 0 {
		domain = domain[:slash]
	}
	return domain == "odt.ae" || domain == "www.odt.ae"
}
