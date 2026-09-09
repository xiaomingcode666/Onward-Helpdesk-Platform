package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var TenantMailSettingRepository = newTenantMailSettingRepository()

func newTenantMailSettingRepository() *tenantMailSettingRepository {
	return &tenantMailSettingRepository{}
}

type tenantMailSettingRepository struct {
}

func (r *tenantMailSettingRepository) GetByTenantID(db *gorm.DB, tenantID int64) *models.TenantMailSetting {
	ret := &models.TenantMailSetting{}
	if err := db.Where("tenant_id = ?", tenantID).First(ret).Error; err != nil {
		return nil
	}
	return ret
}

func (r *tenantMailSettingRepository) Upsert(db *gorm.DB, item *models.TenantMailSetting) error {
	existing := r.GetByTenantID(db, item.TenantID)
	if existing == nil {
		return db.Create(item).Error
	}
	return db.Model(&models.TenantMailSetting{}).
		Where("id = ?", existing.ID).
		Updates(map[string]any{
			"from_address": item.FromAddress,
			"from_name":    item.FromName,
			"smtp_host":    item.SMTPHost,
			"smtp_port":    item.SMTPPort,
			"username":     item.Username,
			"password":     item.Password,
			"reply_to":     item.ReplyTo,
			"retry_policy": item.RetryPolicy,
			"use_tls":      item.UseTLS,
			"status":       item.Status,
			"updated_at":   item.UpdatedAt,
		}).Error
}

func (r *tenantMailSettingRepository) FindAll(db *gorm.DB) []models.TenantMailSetting {
	var list []models.TenantMailSetting
	sqls.NewCnd().Find(db, &list)
	return list
}
