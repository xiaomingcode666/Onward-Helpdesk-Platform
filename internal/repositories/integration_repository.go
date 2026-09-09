package repositories

import (
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"remotehelpdesk/internal/models"
)

type tenantIntegrationConfigRepository struct{}

var TenantIntegrationConfigRepository = &tenantIntegrationConfigRepository{}

func (r *tenantIntegrationConfigRepository) Get(db *gorm.DB, id int64) *models.TenantIntegrationConfig {
	v := &models.TenantIntegrationConfig{}
	if db.First(v, "id = ?", id).Error != nil {
		return nil
	}
	return v
}
func (r *tenantIntegrationConfigRepository) GetByProvider(db *gorm.DB, tenantID int64, provider string) *models.TenantIntegrationConfig {
	v := &models.TenantIntegrationConfig{}
	if db.Where("tenant_id = ? AND provider = ?", tenantID, provider).First(v).Error != nil {
		return nil
	}
	return v
}
func (r *tenantIntegrationConfigRepository) FindByProviderTenantIDs(db *gorm.DB, provider string, tenantIDs []int64) (map[int64]*models.TenantIntegrationConfig, error) {
	ret := make(map[int64]*models.TenantIntegrationConfig, len(tenantIDs))
	if len(tenantIDs) == 0 {
		return ret, nil
	}
	items := make([]models.TenantIntegrationConfig, 0, len(tenantIDs))
	if err := db.Where("provider = ? AND tenant_id IN ? AND status <> ?", provider, tenantIDs, enums.StatusDeleted).Find(&items).Error; err != nil {
		return nil, err
	}
	for i := range items {
		ret[items[i].TenantID] = &items[i]
	}
	return ret, nil
}
func (r *tenantIntegrationConfigRepository) FindPage(db *gorm.DB, cnd *sqls.Cnd) ([]models.TenantIntegrationConfig, *sqls.Paging) {
	var v []models.TenantIntegrationConfig
	cnd.Find(db, &v)
	return v, &sqls.Paging{Page: cnd.Paging.Page, Limit: cnd.Paging.Limit, Total: cnd.Count(db, &models.TenantIntegrationConfig{})}
}
func (r *tenantIntegrationConfigRepository) Create(db *gorm.DB, v *models.TenantIntegrationConfig) error {
	return db.Create(v).Error
}
func (r *tenantIntegrationConfigRepository) Updates(db *gorm.DB, id int64, m map[string]any) error {
	return db.Model(&models.TenantIntegrationConfig{}).Where("id = ?", id).Updates(m).Error
}

type productAIUsageCredentialRepository struct{}

var ProductAIUsageCredentialRepository = &productAIUsageCredentialRepository{}

func (r *productAIUsageCredentialRepository) Get(db *gorm.DB, id int64) *models.ProductAIUsageCredential {
	v := &models.ProductAIUsageCredential{}
	if db.First(v, "id = ?", id).Error != nil {
		return nil
	}
	return v
}
func (r *productAIUsageCredentialRepository) GetByProduct(db *gorm.DB, tenantID, productID int64) *models.ProductAIUsageCredential {
	v := &models.ProductAIUsageCredential{}
	if db.Where("tenant_id = ? AND product_id = ?", tenantID, productID).First(v).Error != nil {
		return nil
	}
	return v
}
func (r *productAIUsageCredentialRepository) Find(db *gorm.DB, cnd *sqls.Cnd) []models.ProductAIUsageCredential {
	var v []models.ProductAIUsageCredential
	cnd.Find(db, &v)
	return v
}
func (r *productAIUsageCredentialRepository) FindPage(db *gorm.DB, cnd *sqls.Cnd) ([]models.ProductAIUsageCredential, *sqls.Paging) {
	var v []models.ProductAIUsageCredential
	cnd.Find(db, &v)
	return v, &sqls.Paging{Page: cnd.Paging.Page, Limit: cnd.Paging.Limit, Total: cnd.Count(db, &models.ProductAIUsageCredential{})}
}
func (r *productAIUsageCredentialRepository) Create(db *gorm.DB, v *models.ProductAIUsageCredential) error {
	return db.Create(v).Error
}
func (r *productAIUsageCredentialRepository) Updates(db *gorm.DB, id int64, m map[string]any) error {
	return db.Model(&models.ProductAIUsageCredential{}).Where("id = ?", id).Updates(m).Error
}
