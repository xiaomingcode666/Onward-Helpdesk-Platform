package migration

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func init() {
	register(61, "preserve supplier collaboration participants and co-worker access", func() error {
		return migrateSupplierCollaborationParticipants(sqls.DB())
	})
}

func migrateSupplierCollaborationParticipants(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if err := db.AutoMigrate(&models.TicketSupplierCollaborationParticipant{}); err != nil {
		return err
	}
	if !db.Migrator().HasTable(&models.TicketSupplierCollaboration{}) {
		return nil
	}
	var collaborations []models.TicketSupplierCollaboration
	if err := db.Where("record_status <> ?", enums.StatusDeleted).Order("id ASC").Find(&collaborations).Error; err != nil {
		return err
	}
	for i := range collaborations {
		item := collaborations[i]
		if item.PartnerAccountID <= 0 {
			continue
		}
		status := enums.StatusOk
		leftAt := item.ResolvedAt
		if item.Status == "resolved" {
			status = enums.StatusDisabled
		}
		participant := &models.TicketSupplierCollaborationParticipant{
			TenantID:         item.TenantID,
			CollaborationID:  item.ID,
			PartnerCompanyID: item.PartnerCompanyID,
			PartnerAccountID: item.PartnerAccountID,
			Role:             "owner",
			JoinedAt:         item.InvitedAt,
			LeftAt:           leftAt,
			Status:           status,
			AuditFields:      item.AuditFields,
		}
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "collaboration_id"}, {Name: "partner_account_id"}},
			DoNothing: true,
		}).Create(participant).Error; err != nil {
			return err
		}
	}
	return nil
}
