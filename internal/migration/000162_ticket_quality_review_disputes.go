package migration

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(162, "add ticket quality review snapshots, final outcomes and disputes", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			if err := backfillTicketQualityReviewSnapshots(ctx.Tx); err != nil {
				return err
			}
			permissions, err := ensurePermissions(ctx.Tx)
			if err != nil {
				return err
			}
			roles, err := ensureRoles(ctx.Tx)
			if err != nil {
				return err
			}
			if err := ensureRolePermissions(ctx.Tx, roles, permissions); err != nil {
				return err
			}
			var tenants []models.Tenant
			if err := ctx.Tx.Find(&tenants).Error; err != nil {
				return err
			}
			for _, tenant := range tenants {
				if err := services.EnsureTenantDefaultIAMRolesDB(ctx.Tx, tenant.ID, nil); err != nil {
					return err
				}
			}
			return nil
		})
	})
}

// backfillTicketQualityReviewSnapshots attributes older reviews to the ticket's
// current owner (best effort) and starts their final outcome at the original one.
func backfillTicketQualityReviewSnapshots(tx *gorm.DB) error {
	if err := tx.AutoMigrate(&models.TicketQualityReview{}, &models.TicketQualityDispute{}); err != nil {
		return err
	}
	if err := tx.Exec(`UPDATE t_ticket_quality_review SET
		reviewed_agent_id = COALESCE((SELECT t.current_assignee_id FROM t_ticket t WHERE t.id = t_ticket_quality_review.ticket_id AND t.tenant_id = t_ticket_quality_review.tenant_id), 0),
		reviewed_team_id = COALESCE((SELECT t.current_team_id FROM t_ticket t WHERE t.id = t_ticket_quality_review.ticket_id AND t.tenant_id = t_ticket_quality_review.tenant_id), 0)
		WHERE reviewed_agent_id = 0 AND reviewed_team_id = 0`).Error; err != nil {
		return err
	}
	if err := tx.Exec(`UPDATE t_ticket_quality_review SET final_outcome = CASE WHEN outcome <> '' THEN outcome ELSE result END WHERE final_outcome = ''`).Error; err != nil {
		return err
	}
	return nil
}
