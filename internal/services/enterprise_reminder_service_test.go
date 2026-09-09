package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestEnterpriseReminderPollReturnsUpcomingMeetingsAndScopedNewTickets(t *testing.T) {
	db := setupEnterpriseReminderTestDB(t)
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	tenantID := int64(7)
	userID := int64(101)
	otherUserID := int64(202)
	operator := &dto.AuthPrincipal{TenantID: tenantID, UserID: userID, DomainType: models.DomainTypeEnterprise, Roles: []string{EnterpriseRoleEngineer}}

	products := []models.Product{
		{TenantID: tenantID, Code: "PUMP", Name: "Pump", Status: enums.StatusOk, AuditFields: auditAt(now)},
		{TenantID: tenantID, Code: "DRIVE", Name: "Drive", Status: enums.StatusOk, AuditFields: auditAt(now)},
	}
	if err := db.Create(&products).Error; err != nil {
		t.Fatalf("create products: %v", err)
	}
	product := products[0]
	otherProduct := products[1]
	teams := []models.AgentTeam{
		{TenantID: tenantID, ProductID: product.ID, Name: "Pump Repair", Status: enums.StatusOk, AuditFields: auditAt(now)},
		{TenantID: tenantID, ProductID: otherProduct.ID, Name: "Drive Repair", Status: enums.StatusOk, AuditFields: auditAt(now)},
	}
	if err := db.Create(&teams).Error; err != nil {
		t.Fatalf("create teams: %v", err)
	}
	team := teams[0]
	otherTeam := teams[1]
	profile := models.AgentProfile{
		TenantID: tenantID, UserID: userID, TeamID: team.ID, AgentCode: "E101", DisplayName: "Engineer", Status: enums.StatusOk,
		AuditFields: auditAt(now),
	}
	member := models.AgentTeamMember{TenantID: tenantID, TeamID: team.ID, UserID: userID, DispatchEnabled: true, Status: enums.StatusOk, AuditFields: auditAt(now)}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("create profile: %v", err)
	}
	if err := db.Create(&member).Error; err != nil {
		t.Fatalf("create member: %v", err)
	}

	tickets := []models.Ticket{
		ticketFixture(tenantID, "TK-MEETING", "Scheduled video", product.ID, team.ID, userID, now.Add(-2*time.Hour), enums.TicketStatusProcessing),
		ticketFixture(tenantID, "TK-NEW", "New pump ticket", product.ID, team.ID, 0, now.Add(-30*time.Second), enums.TicketStatusPendingDispatch),
		ticketFixture(tenantID, "TK-OLD", "Old pump ticket", product.ID, team.ID, 0, now.Add(-10*time.Minute), enums.TicketStatusPendingDispatch),
		ticketFixture(tenantID, "TK-OTHER", "Other product ticket", otherProduct.ID, otherTeam.ID, 0, now.Add(-20*time.Second), enums.TicketStatusPendingDispatch),
	}
	if err := db.Create(&tickets).Error; err != nil {
		t.Fatalf("create tickets: %v", err)
	}
	meetingTicket := tickets[0]

	startsSoon := now.Add(45 * time.Minute)
	startsLater := now.Add(2 * time.Hour)
	meetings := []models.MeetingRoomJitsi{
		{
			ID: "meeting-soon", TenantID: tenantID, TicketID: fmt.Sprint(meetingTicket.ID), RoomName: "meeting-soon-room",
			Status: "scheduled", CreatedBy: fmt.Sprint(userID), ScheduledAt: &startsSoon,
			BaseModel: models.BaseModel{CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)},
		},
		{
			ID: "meeting-later", TenantID: tenantID, TicketID: fmt.Sprint(meetingTicket.ID), RoomName: "meeting-later-room",
			Status: "scheduled", CreatedBy: fmt.Sprint(userID), ScheduledAt: &startsLater,
			BaseModel: models.BaseModel{CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)},
		},
		{
			ID: "meeting-other-user", TenantID: tenantID, TicketID: fmt.Sprint(meetingTicket.ID), RoomName: "meeting-other-user-room",
			Status: "scheduled", CreatedBy: fmt.Sprint(otherUserID), ScheduledAt: &startsSoon,
			BaseModel: models.BaseModel{CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)},
		},
	}
	if err := db.Create(&meetings).Error; err != nil {
		t.Fatalf("create meetings: %v", err)
	}

	result, err := EnterpriseReminderService.Poll(context.Background(), tenantID, operator, now.Add(-2*time.Minute), now)
	if err != nil {
		t.Fatalf("poll reminders: %v", err)
	}
	if len(result.MeetingReminders) != 1 {
		t.Fatalf("meeting reminders len=%d items=%+v", len(result.MeetingReminders), result.MeetingReminders)
	}
	if got := result.MeetingReminders[0]; got.MeetingID != "meeting-soon" || got.StartsInMinutes != 45 || got.TicketNo != "TK-MEETING" || got.ActionURL != fmt.Sprintf("/enterprise/ticket-workbench?ticket_id=%d", meetingTicket.ID) {
		t.Fatalf("meeting reminder = %+v", got)
	}
	if len(result.TicketReminders) != 1 {
		t.Fatalf("ticket reminders len=%d items=%+v", len(result.TicketReminders), result.TicketReminders)
	}
	if got := result.TicketReminders[0]; got.TicketNo != "TK-NEW" || got.ProductName != "Pump" || got.TeamName != "Pump Repair" {
		t.Fatalf("ticket reminder = %+v", got)
	}
}

func setupEnterpriseReminderTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Product{},
		&models.AgentProfile{},
		&models.AgentTeam{},
		&models.AgentTeamMember{},
		&models.Ticket{},
		&models.MeetingRoomJitsi{},
		&models.MeetingParticipant{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func ticketFixture(tenantID int64, ticketNo, title string, productID, teamID, assigneeID int64, createdAt time.Time, status enums.TicketStatus) models.Ticket {
	return models.Ticket{
		TenantID:          tenantID,
		TicketNo:          ticketNo,
		Title:             title,
		Status:            status,
		PriorityCode:      "p2",
		ProductID:         productID,
		CurrentTeamID:     teamID,
		CurrentAssigneeID: assigneeID,
		AuditFields:       auditAt(createdAt),
	}
}

func auditAt(value time.Time) models.AuditFields {
	return models.AuditFields{CreatedAt: value, UpdatedAt: value, CreateUserName: "test", UpdateUserName: "test"}
}
