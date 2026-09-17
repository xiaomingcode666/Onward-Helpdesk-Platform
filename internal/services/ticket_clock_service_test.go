package services

import (
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
)

func TestTicketClockComputesE2EAndAccountableAcrossSupplierPause(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	if err := db.AutoMigrate(&models.TicketClockPause{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	dueAt := now.Add(-time.Hour)
	ticket := models.Ticket{
		TicketNo: "CLOCK-1", TenantID: 1, Status: enums.TicketStatusSupplierSupport,
		SLADueAt: &dueAt,
		AuditFields: models.AuditFields{
			CreatedAt: now.Add(-4 * time.Hour),
			UpdatedAt: now.Add(-4 * time.Hour),
		},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "manager"}
	if err := TicketClockService.OpenSupplierWaitTx(db, ticket.ID, 71, now.Add(-2*time.Hour), operator); err != nil {
		t.Fatal(err)
	}
	active := TicketClockService.Compute(*repositoriesTicket(t, ticket.ID), now)
	if active.E2ESeconds != int64(4*time.Hour/time.Second) {
		t.Fatalf("e2e seconds = %d", active.E2ESeconds)
	}
	if active.PausedSeconds != int64(2*time.Hour/time.Second) {
		t.Fatalf("paused seconds = %d", active.PausedSeconds)
	}
	if active.AccountableSeconds != int64(2*time.Hour/time.Second) || !active.PauseActive {
		t.Fatalf("unexpected active clock: %+v", active)
	}

	if err := TicketClockService.CloseSupplierWaitTx(db, ticket.ID, 71, now, operator); err != nil {
		t.Fatal(err)
	}
	after := repositoriesTicket(t, ticket.ID)
	if after.DaypopClockPausedAt != nil || after.DaypopClockPausedSeconds != int64(2*time.Hour/time.Second) {
		t.Fatalf("pause projection not closed: %+v", after)
	}
	if after.SLADueAt == nil || !after.SLADueAt.Equal(dueAt.Add(2*time.Hour)) {
		t.Fatalf("accountable deadline not extended: %+v", after.SLADueAt)
	}
	closedClock := TicketClockService.Compute(*after, now.Add(time.Hour))
	if closedClock.E2ESeconds != int64(5*time.Hour/time.Second) || closedClock.AccountableSeconds != int64(3*time.Hour/time.Second) {
		t.Fatalf("unexpected resumed clock: %+v", closedClock)
	}
}

func TestTicketClockKeepsPauseUntilLastSupplierWaitEnds(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	if err := db.AutoMigrate(&models.TicketClockPause{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	ticket := models.Ticket{
		TicketNo: "CLOCK-2", TenantID: 1, Status: enums.TicketStatusSupplierSupport,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-3 * time.Hour), UpdatedAt: now.Add(-3 * time.Hour)},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "manager"}
	if err := TicketClockService.OpenSupplierWaitTx(db, ticket.ID, 81, now.Add(-2*time.Hour), operator); err != nil {
		t.Fatal(err)
	}
	if err := TicketClockService.OpenSupplierWaitTx(db, ticket.ID, 82, now.Add(-90*time.Minute), operator); err != nil {
		t.Fatal(err)
	}
	if err := TicketClockService.CloseSupplierWaitTx(db, ticket.ID, 81, now.Add(-time.Hour), operator); err != nil {
		t.Fatal(err)
	}
	mid := repositoriesTicket(t, ticket.ID)
	if mid.DaypopClockPausedAt == nil {
		t.Fatal("clock resumed while another supplier wait was still active")
	}
	if err := TicketClockService.CloseSupplierWaitTx(db, ticket.ID, 82, now, operator); err != nil {
		t.Fatal(err)
	}
	final := repositoriesTicket(t, ticket.ID)
	if final.DaypopClockPausedAt != nil || final.DaypopClockPausedSeconds != int64(2*time.Hour/time.Second) {
		t.Fatalf("overlapping waits were not collapsed into one pause: %+v", final)
	}
}
