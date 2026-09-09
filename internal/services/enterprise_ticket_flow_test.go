package services

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestTicketFlowUsesAcceptanceAsDispatchMilestoneForTeamPoolClaim(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.Ticket{},
		&models.TicketProgress{},
		&models.TicketRepairRecord{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	engineer := models.User{ID: 41, Username: "pool.engineer", Nickname: "产品组工程师", Status: enums.StatusOk}
	if err := db.Create(&engineer).Error; err != nil {
		t.Fatalf("create engineer: %v", err)
	}
	now := time.Now().Truncate(time.Second)
	ticket := models.Ticket{
		TenantID:          1,
		TicketNo:          "FLOW-TEAM-POOL-1",
		Title:             "产品组公共池接单",
		Status:            enums.TicketStatusProcessing,
		CurrentTeamID:     5,
		CurrentAssigneeID: engineer.ID,
		AuditFields:       models.AuditFields{CreatedAt: now.Add(-time.Minute), UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	if err := db.Create(&models.TicketProgress{
		TenantID:     ticket.TenantID,
		TicketID:     ticket.ID,
		EventType:    enums.TicketProgressEventAccepted,
		Content:      "受理工单",
		MetadataJSON: "{}",
		AuthorID:     engineer.ID,
		CreatedAt:    now,
	}).Error; err != nil {
		t.Fatalf("create acceptance progress: %v", err)
	}

	flow := EnterpriseTicketService.buildFlow(&ticket)
	var acceptCompletedAt, dispatchCompletedAt, dispatchCompletedBy string
	for _, step := range flow.Steps {
		switch step.Name {
		case "Accept":
			acceptCompletedAt = step.CompletedAt
		case "Dispatch":
			dispatchCompletedAt = step.CompletedAt
			dispatchCompletedBy = step.CompletedBy
		}
	}
	if acceptCompletedAt == "" || dispatchCompletedAt != acceptCompletedAt {
		t.Fatalf("dispatch milestone = %q, want acceptance milestone %q", dispatchCompletedAt, acceptCompletedAt)
	}
	if dispatchCompletedBy != engineer.Nickname {
		t.Fatalf("dispatch completed by = %q, want %q", dispatchCompletedBy, engineer.Nickname)
	}
}

func TestTicketFlowPendingAssigneeAcceptDoesNotClaimProcessingStarted(t *testing.T) {
	if currentIndex := ticketFlowCurrentIndex(enums.TicketStatusPendingAssigneeAccept); currentIndex != 0 {
		t.Fatalf("pending assignee accept index = %d, want Accept/current index 0", currentIndex)
	}
}

func TestTicketTimelineDoesNotDuplicateCreatedProgress(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.TicketProgress{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	operator := models.User{ID: 73, Username: "timeline.engineer", Nickname: "处理工程师", Status: enums.StatusOk}
	if err := db.Create(&operator).Error; err != nil {
		t.Fatalf("create operator: %v", err)
	}
	now := time.Now().Truncate(time.Second)
	ticket := models.Ticket{
		ID:       91,
		TenantID: 7,
		AuditFields: models.AuditFields{
			CreatedAt:      now,
			CreateUserName: "客服机器人",
		},
	}
	if err := db.Create(&models.TicketProgress{
		TenantID:     ticket.TenantID,
		TicketID:     ticket.ID,
		EventType:    enums.TicketProgressEventCreated,
		Content:      "Created ticket",
		MetadataJSON: "{}",
		AuthorID:     operator.ID,
		CreatedAt:    now,
	}).Error; err != nil {
		t.Fatalf("create ticket progress: %v", err)
	}

	timeline := EnterpriseTicketService.buildTimeline(&ticket)
	if len(timeline) != 1 {
		t.Fatalf("timeline length = %d, want 1: %+v", len(timeline), timeline)
	}
	if timeline[0].Type != string(enums.TicketProgressEventCreated) || timeline[0].Actor != operator.Nickname {
		t.Fatalf("unexpected created timeline item: %+v", timeline[0])
	}
}

func TestTicketTimelineFallsBackForLegacyTicketWithoutCreatedProgress(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.TicketProgress{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	ticket := models.Ticket{
		ID:       92,
		TenantID: 7,
		AuditFields: models.AuditFields{
			CreatedAt:      time.Now().Truncate(time.Second),
			CreateUserName: "历史客服",
		},
	}

	timeline := EnterpriseTicketService.buildTimeline(&ticket)
	if len(timeline) != 1 || timeline[0].Content != "Ticket created" || timeline[0].Actor != "历史客服" {
		t.Fatalf("unexpected legacy timeline fallback: %+v", timeline)
	}
}

func TestTicketAssignmentUsesDispatchAttemptHistoryWhenAvailable(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Ticket{}, &models.TicketDispatchAttempt{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	now := time.Now().Truncate(time.Second)
	ticket := models.Ticket{
		TenantID:         7,
		TicketNo:         "DISPATCH-HISTORY-1",
		Title:            "派单明细展示",
		DispatchAttempts: 0,
		AuditFields:      models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	attempts := []models.TicketDispatchAttempt{
		{TenantID: ticket.TenantID, TicketID: ticket.ID, TeamID: 5, AssigneeID: 41, AttemptNo: 1, Outcome: "accepted", AssignedAt: now.Add(-2 * time.Minute)},
		{TenantID: ticket.TenantID, TicketID: ticket.ID, TeamID: 5, AssigneeID: 42, AttemptNo: 2, Outcome: "superseded", AssignedAt: now.Add(-time.Minute)},
	}
	if err := db.Create(&attempts).Error; err != nil {
		t.Fatalf("create dispatch attempts: %v", err)
	}

	assignment := EnterpriseTicketService.buildAssignment(&ticket)
	if assignment.DispatchAttempts != len(attempts) {
		t.Fatalf("dispatch attempts = %d, want %d", assignment.DispatchAttempts, len(attempts))
	}
}
