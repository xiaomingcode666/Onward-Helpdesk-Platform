package migration

import (
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(69, "add sla tenant guards and active uniqueness", func() error {
		if err := sqls.DB().AutoMigrate(&services.SLAPolicy{}, &services.SLAPauseRecord{}); err != nil {
			return err
		}
		if err := sqls.DB().Exec(`
UPDATE sla_pause_records AS pause
SET tenant_id = CAST(ticket.tenant_id AS varchar)
FROM t_ticket AS ticket
WHERE pause.ticket_id = CAST(ticket.id AS varchar)
  AND COALESCE(pause.tenant_id, '') = ''`).Error; err != nil {
			return err
		}
		if err := sqls.DB().Exec(`
UPDATE sla_pause_records
SET resumed_at = COALESCE(resumed_at, updated_at), updated_at = CURRENT_TIMESTAMP
WHERE id IN (
  SELECT id FROM (
    SELECT id, ROW_NUMBER() OVER (
      PARTITION BY tenant_id, ticket_id
      ORDER BY paused_at DESC, created_at DESC, id DESC
    ) AS row_no
    FROM sla_pause_records
    WHERE resumed_at IS NULL
  ) duplicates
  WHERE row_no > 1
)`).Error; err != nil {
			return err
		}
		if err := sqls.DB().Exec(`
UPDATE sla_policies
SET status = 'inactive', updated_at = CURRENT_TIMESTAMP
WHERE id IN (
  SELECT id FROM (
    SELECT id, ROW_NUMBER() OVER (
      PARTITION BY tenant_id, priority
      ORDER BY updated_at DESC, created_at DESC, id DESC
    ) AS row_no
    FROM sla_policies
    WHERE status = 'active'
  ) duplicates
  WHERE row_no > 1
)`).Error; err != nil {
			return err
		}
		if err := sqls.DB().Exec("CREATE UNIQUE INDEX IF NOT EXISTS ux_sla_pause_active ON sla_pause_records(tenant_id, ticket_id) WHERE resumed_at IS NULL").Error; err != nil {
			return err
		}
		return sqls.DB().Exec("CREATE UNIQUE INDEX IF NOT EXISTS ux_sla_policy_active_priority ON sla_policies(tenant_id, priority) WHERE status = 'active'").Error
	})
}
