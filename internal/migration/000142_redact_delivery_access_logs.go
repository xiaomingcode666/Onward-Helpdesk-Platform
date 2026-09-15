package migration

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/logprivacy"
	"remotehelpdesk/internal/pkg/secretstore"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() { register(142, "redact email and connector logs", redactDeliveryAccessLogs) }

func redactDeliveryAccessLogs() error {
	db := sqls.DB()
	if db.Migrator().HasTable(&models.DeliveryLog{}) {
		if !db.Migrator().HasColumn(&models.DeliveryLog{}, "RecipientCiphertext") {
			if err := db.Migrator().AddColumn(&models.DeliveryLog{}, "RecipientCiphertext"); err != nil {
				return err
			}
		}
		var rows []models.DeliveryLog
		if err := db.Where("channel = ?", "email").FindInBatches(&rows, 200, func(tx *gorm.DB, _ int) error {
			return tx.Transaction(func(batch *gorm.DB) error {
				for _, row := range rows {
					ciphertext := ""
					if row.NotificationID > 0 && (row.Status == "pending" || row.Status == "processing" || row.Status == "waiting_retry") {
						ciphertext = row.RecipientCiphertext
						if ciphertext == "" && row.RecipientID != logprivacy.Redacted {
							var err error
							ciphertext, err = secretstore.Encrypt(row.RecipientID)
							if err != nil {
								return err
							}
						}
					}
					if err := batch.Model(&models.DeliveryLog{}).Where("id = ?", row.ID).UpdateColumns(map[string]any{
						"recipient_id": logprivacy.Value(row.RecipientID), "recipient_ciphertext": ciphertext, "error_msg": logprivacy.Value(row.ErrorMsg),
					}).Error; err != nil {
						return err
					}
				}
				return nil
			})
		}).Error; err != nil {
			return err
		}
	}
	if db.Migrator().HasTable(&models.AccessCallLog{}) {
		var rows []models.AccessCallLog
		return db.FindInBatches(&rows, 200, func(tx *gorm.DB, _ int) error {
			return tx.Transaction(func(batch *gorm.DB) error {
				for _, row := range rows {
					if err := batch.Model(&models.AccessCallLog{}).Where("id = ?", row.ID).UpdateColumns(map[string]any{
						"request_url": logprivacy.Value(row.RequestURL), "request_body": logprivacy.Value(row.RequestBody),
						"response_body": logprivacy.Value(row.ResponseBody), "error_message": logprivacy.Value(row.ErrorMessage),
					}).Error; err != nil {
						return err
					}
				}
				return nil
			})
		}).Error
	}
	return nil
}
