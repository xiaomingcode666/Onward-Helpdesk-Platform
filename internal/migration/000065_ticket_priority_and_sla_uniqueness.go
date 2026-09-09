package migration

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(65, "add ticket priority and active sla uniqueness", func() error {
		if err := sqls.DB().AutoMigrate(&models.Ticket{}, &services.SLAViolation{}, &services.TicketEscalation{}); err != nil {
			return err
		}
		if err := sqls.DB().Exec(`
UPDATE sla_violations
SET resolved_at = COALESCE(resolved_at, updated_at), updated_at = CURRENT_TIMESTAMP
WHERE id IN (
  SELECT id FROM (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY ticket_id, violation_type ORDER BY created_at DESC, id DESC) AS row_no
    FROM sla_violations
    WHERE resolved_at IS NULL
  ) duplicates
  WHERE row_no > 1
)`).Error; err != nil {
			return err
		}
		if err := sqls.DB().Exec(`
UPDATE ticket_escalations
SET status = 'completed', resolved_at = COALESCE(resolved_at, updated_at), updated_at = CURRENT_TIMESTAMP
WHERE id IN (
  SELECT id FROM (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY ticket_id, to_level ORDER BY escalated_at DESC, id DESC) AS row_no
    FROM ticket_escalations
    WHERE status IN ('pending', 'accepted')
  ) duplicates
  WHERE row_no > 1
)`).Error; err != nil {
			return err
		}
		if err := sqls.DB().Exec("CREATE UNIQUE INDEX IF NOT EXISTS ux_sla_violation_active ON sla_violations(ticket_id, violation_type) WHERE resolved_at IS NULL").Error; err != nil {
			return err
		}
		return sqls.DB().Exec("CREATE UNIQUE INDEX IF NOT EXISTS ux_ticket_escalation_active ON ticket_escalations(ticket_id, to_level) WHERE status IN ('pending', 'accepted')").Error
	})
}
