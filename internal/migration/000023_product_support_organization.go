package migration

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(23, "provision tenant product repair organization", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			if err := ensureProductSupportOrganizationSchema(ctx.Tx); err != nil {
				return err
			}
			return services.ProductSupportOrganizationService.BackfillAllDB(ctx.Tx, nil)
		})
	})
}

func ensureProductSupportOrganizationSchema(db *gorm.DB) error {
	migrator := db.Migrator()
	tables := []struct {
		model   any
		columns []string
	}{
		{&models.AgentTeam{}, []string{"TenantID", "ParentID", "ProductID", "DepartmentID", "TeamType", "SystemKey", "SystemManaged"}},
		{&models.AgentProfile{}, []string{"TenantID"}},
		{&models.AgentTeamSchedule{}, []string{"TenantID"}},
		{&models.Ticket{}, []string{"CurrentTeamID"}},
	}
	for _, table := range tables {
		if !migrator.HasTable(table.model) {
			if err := migrator.CreateTable(table.model); err != nil {
				return err
			}
			continue
		}
		for _, column := range table.columns {
			if !migrator.HasColumn(table.model, column) {
				if err := migrator.AddColumn(table.model, column); err != nil {
					return err
				}
			}
		}
	}
	if !migrator.HasIndex(&models.AgentTeam{}, "uk_agent_team_tenant_system") {
		if err := migrator.CreateIndex(&models.AgentTeam{}, "uk_agent_team_tenant_system"); err != nil {
			return err
		}
	}
	return nil
}
