package services

import (
	"testing"
	"time"

	"gorm.io/gorm"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
)

func duplicateFixture(t *testing.T) (*gorm.DB, *dto.AuthPrincipal, models.Ticket, models.Ticket) {
	t.Helper()
	db, op, source := setupGovernance(t)
	source.CustomerID = 42
	source.Description = "第二次报障：显示 E42，附有复现步骤"
	source.PriorityLevel = "p1"
	if err := db.Save(&source).Error; err != nil {
		t.Fatal(err)
	}
	main := source
	main.ID = 0
	main.CreatedAt = source.CreatedAt.Add(-time.Hour)
	main.TicketNo = "MAIN-" + t.Name()
	main.PriorityLevel = "p2"
	main.Description = "第一次报障原文"
	main.CurrentAssigneeID = 0
	main.CaseOwnerID = 0
	main.AcknowledgedAt = nil
	main.AcceptedAt = nil
	deadline := time.Now().Add(time.Hour)
	main.SLADueAt = &deadline
	if err := db.Create(&main).Error; err != nil {
		t.Fatal(err)
	}
	return db, op, source, main
}

func duplicateCommand(t *testing.T, db *gorm.DB, source, main models.Ticket) TicketGovernanceCommand {
	cmd := governanceCommand(t, db, source.ID, "duplicate_child")
	cmd.TargetID = main.ID
	cmd.TargetRevision = &main.GovernanceRevision
	cmd.Reason = ""
	return cmd
}
