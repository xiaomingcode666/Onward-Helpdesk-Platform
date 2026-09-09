package services

import (
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
)

func TestEnterpriseWorkbenchQueuePrioritizesRecentlyUpdatedActiveTickets(t *testing.T) {
	db := setupSLATenantTestDB(t)
	operator := &dto.AuthPrincipal{
		TenantID: 1, UserID: 11, Username: "manager",
		DomainType: models.DomainTypeEnterprise,
		Roles:      []string{EnterpriseRoleServiceManager},
		Status:     enums.StatusOk,
	}
	now := time.Now()
	closedAt := now
	deferredUntil := now.Add(10 * time.Minute)
	tickets := []models.Ticket{
		{
			TicketNo: "WB-CLOSED-P0", TenantID: 1, Status: enums.TicketStatusClosed, PriorityCode: "p0", ResolvedAt: &closedAt,
			AuditFields: models.AuditFields{CreatedAt: now.Add(-7 * time.Hour), UpdatedAt: closedAt},
		},
		{
			TicketNo: "WB-ACTIVE-OLD", TenantID: 1, Status: enums.TicketStatusPendingDispatch,
			DispatchAttempts: 2, DispatchDeferredUntil: &deferredUntil, LastDispatchFailureReason: "no_active_schedule_team",
			AuditFields: models.AuditFields{CreatedAt: now.Add(-5 * time.Hour), UpdatedAt: now.Add(-10 * time.Minute)},
		},
		{
			TicketNo: "WB-ACTIVE-NEW", TenantID: 1, Status: enums.TicketStatusProcessing,
			AuditFields: models.AuditFields{CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now},
		},
		{
			TicketNo: "WB-CANCELLED", TenantID: 1, Status: enums.TicketStatusCancelled,
			AuditFields: models.AuditFields{CreatedAt: now.Add(-6 * time.Hour), UpdatedAt: now},
		},
	}
	if err := db.Create(&tickets).Error; err != nil {
		t.Fatalf("create tickets: %v", err)
	}

	core, err := EnterpriseWorkbenchService.Core(1, operator.UserID, operator)
	if err != nil {
		t.Fatalf("workbench core: %v", err)
	}
	if len(core.Queue) != 2 {
		t.Fatalf("queue length = %d, want two active tickets: %+v", len(core.Queue), core.Queue)
	}
	if core.Queue[0].TicketNo != "WB-ACTIVE-NEW" || core.Queue[1].TicketNo != "WB-ACTIVE-OLD" {
		t.Fatalf("queue should be recently updated active tickets first, got %+v", core.Queue)
	}
	if core.Queue[1].DispatchAttempts != 2 || core.Queue[1].DispatchDeferredUntil == "" || core.Queue[1].LastDispatchFailureReason != "no_active_schedule_team" {
		t.Fatalf("queue should expose dispatch aging signals, got %+v", core.Queue[1])
	}
	if core.Summary.OpenTickets != 2 || core.Summary.UrgentTickets != 0 || core.Summary.ClosedTodayTickets != 2 {
		t.Fatalf("unexpected workbench summary: %+v", core.Summary)
	}
	for _, card := range core.Queues {
		for _, item := range card.Items {
			if item.TicketNo == "WB-CLOSED-P0" || item.TicketNo == "WB-CANCELLED" {
				t.Fatalf("terminal ticket leaked into queue card %s: %+v", card.Key, card.Items)
			}
		}
	}

	queue, err := EnterpriseWorkbenchService.Queue(1, operator.UserID, EnterpriseWorkbenchQueueQuery{Page: 1, PageSize: 10}, operator)
	if err != nil {
		t.Fatalf("workbench queue: %v", err)
	}
	if len(queue.Tickets) != 2 || queue.Tickets[0].TicketNo != "WB-ACTIVE-NEW" || queue.Tickets[1].TicketNo != "WB-ACTIVE-OLD" {
		t.Fatalf("queue endpoint should return recently updated active tickets, got %+v", queue.Tickets)
	}
	if queue.Tickets[1].DispatchAttempts != 2 || queue.Tickets[1].DispatchDeferredUntil == "" || queue.Tickets[1].LastDispatchFailureReason != "no_active_schedule_team" {
		t.Fatalf("queue endpoint should preserve dispatch aging signals, got %+v", queue.Tickets[1])
	}
	firstPage, err := EnterpriseWorkbenchService.Queue(1, operator.UserID, EnterpriseWorkbenchQueueQuery{Page: 1, PageSize: 1}, operator)
	if err != nil {
		t.Fatalf("workbench queue first page: %v", err)
	}
	if len(firstPage.Tickets) != 1 || firstPage.Tickets[0].TicketNo != "WB-ACTIVE-NEW" {
		t.Fatalf("first queue page mismatch: %+v", firstPage.Tickets)
	}
	if firstPage.TicketPagination.Total != 2 || firstPage.TicketPagination.Page != 1 || firstPage.TicketPagination.PageSize != 1 || !firstPage.TicketPagination.HasMore {
		t.Fatalf("first queue page metadata mismatch: %+v", firstPage.TicketPagination)
	}
	secondPage, err := EnterpriseWorkbenchService.Queue(1, operator.UserID, EnterpriseWorkbenchQueueQuery{Page: 2, PageSize: 1}, operator)
	if err != nil {
		t.Fatalf("workbench queue second page: %v", err)
	}
	if len(secondPage.Tickets) != 1 || secondPage.Tickets[0].TicketNo != "WB-ACTIVE-OLD" {
		t.Fatalf("second queue page mismatch: %+v", secondPage.Tickets)
	}
	if secondPage.TicketPagination.Total != 2 || secondPage.TicketPagination.Page != 2 || secondPage.TicketPagination.PageSize != 1 || secondPage.TicketPagination.HasMore {
		t.Fatalf("second queue page metadata mismatch: %+v", secondPage.TicketPagination)
	}
	processingQueue, err := EnterpriseWorkbenchService.Queue(1, operator.UserID, EnterpriseWorkbenchQueueQuery{Page: 1, PageSize: 10, QueueKey: "processing"}, operator)
	if err != nil {
		t.Fatalf("workbench processing queue: %v", err)
	}
	if len(processingQueue.Tickets) != 1 || processingQueue.Tickets[0].TicketNo != "WB-ACTIVE-NEW" || processingQueue.TicketPagination.Total != 1 {
		t.Fatalf("processing queue should be independently paged and filtered, got tickets=%+v pagination=%+v", processingQueue.Tickets, processingQueue.TicketPagination)
	}
}

func TestEnterpriseWorkbenchQueueBreaksUpdatedAtTiesByNewestID(t *testing.T) {
	db := setupSLATenantTestDB(t)
	operator := &dto.AuthPrincipal{
		TenantID: 1, UserID: 12, Username: "manager",
		DomainType: models.DomainTypeEnterprise,
		Roles:      []string{EnterpriseRoleServiceManager},
		Status:     enums.StatusOk,
	}
	createdAt := time.Now().Add(-3 * time.Hour).Truncate(time.Second)
	first := models.Ticket{
		TicketNo: "WB-SAME-CREATED-FIRST", TenantID: 1, Status: enums.TicketStatusPendingDispatch,
		AuditFields: models.AuditFields{CreatedAt: createdAt, UpdatedAt: createdAt},
	}
	second := models.Ticket{
		TicketNo: "WB-SAME-CREATED-SECOND", TenantID: 1, Status: enums.TicketStatusPendingDispatch,
		AuditFields: models.AuditFields{CreatedAt: createdAt, UpdatedAt: createdAt},
	}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create first ticket: %v", err)
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("create second ticket: %v", err)
	}
	if first.ID >= second.ID {
		t.Fatalf("test fixture expected first id before second id: first=%d second=%d", first.ID, second.ID)
	}

	queue, err := EnterpriseWorkbenchService.Queue(1, operator.UserID, EnterpriseWorkbenchQueueQuery{Page: 1, PageSize: 10}, operator)
	if err != nil {
		t.Fatalf("workbench queue: %v", err)
	}
	if len(queue.Tickets) < 2 || queue.Tickets[0].TicketNo != second.TicketNo || queue.Tickets[1].TicketNo != first.TicketNo {
		t.Fatalf("queue endpoint should break updated_at ties by newest id, got %+v", queue.Tickets)
	}
}

func TestEnterpriseWorkbenchQueueIncludesOperatorTicketActions(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	if err := db.AutoMigrate(&models.Product{}, &models.AgentTeamMember{}); err != nil {
		t.Fatalf("auto migrate workbench action fixture: %v", err)
	}
	now := time.Now()
	minute := scheduleMinute(now)
	teamID := createEnterpriseTicketActionRepairTeamFixture(t, 1, 703, 1710, true)
	createEnterpriseTicketActionEngineerFixture(t, 1, teamID, 1711)
	if err := db.Create(&models.AgentTeamSchedule{
		TenantID:      1,
		TeamID:        teamID,
		UserID:        1711,
		RepeatType:    AgentTeamScheduleRepeatOnce,
		DayType:       AgentTeamScheduleDayTypeWork,
		Weekday:       scheduleWeekday(now),
		StartMinute:   max(0, minute-1),
		EndMinute:     min(24*60, minute+30),
		Timezone:      EngineerScheduleTimezone,
		PublishStatus: AgentTeamSchedulePublishPublished,
		StartAt:       now.Add(-time.Hour),
		EndAt:         now.Add(time.Hour),
		Status:        enums.StatusOk,
	}).Error; err != nil {
		t.Fatal(err)
	}
	ticket := models.Ticket{
		TicketNo: "WB-ACTIONS", TenantID: 1, ProductID: 703, Status: enums.TicketStatusPendingAcceptance,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	offShift := &dto.AuthPrincipal{TenantID: 1, UserID: 1710, Roles: []string{EnterpriseRoleEngineer}, Status: enums.StatusOk}
	onShift := &dto.AuthPrincipal{TenantID: 1, UserID: 1711, Roles: []string{EnterpriseRoleEngineer}, Status: enums.StatusOk}

	offShiftQueue, err := EnterpriseWorkbenchService.Queue(1, offShift.UserID, EnterpriseWorkbenchQueueQuery{Page: 1, PageSize: 10}, offShift)
	if err != nil {
		t.Fatalf("off-shift workbench queue: %v", err)
	}
	offShiftTicket := findWorkbenchQueueTicketByNo(offShiftQueue.Tickets, "WB-ACTIONS")
	if offShiftTicket == nil || offShiftTicket.Actions == nil {
		t.Fatalf("off-shift queue did not include ticket actions: %+v", offShiftQueue.Tickets)
	}
	if !offShiftTicket.Actions.CanAccept {
		t.Fatalf("explicit self-claim should not be hidden by roster display state: %+v", offShiftTicket.Actions)
	}

	onShiftQueue, err := EnterpriseWorkbenchService.Queue(1, onShift.UserID, EnterpriseWorkbenchQueueQuery{Page: 1, PageSize: 10}, onShift)
	if err != nil {
		t.Fatalf("on-shift workbench queue: %v", err)
	}
	onShiftTicket := findWorkbenchQueueTicketByNo(onShiftQueue.Tickets, "WB-ACTIONS")
	if onShiftTicket == nil || onShiftTicket.Actions == nil {
		t.Fatalf("on-shift queue did not include ticket actions: %+v", onShiftQueue.Tickets)
	}
	if !onShiftTicket.Actions.CanAccept {
		t.Fatalf("on-shift engineer should receive can_accept=true: %+v", onShiftTicket.Actions)
	}
}

func TestEnterpriseWorkbenchQueueAllowsManualClaimAfterScheduleFailure(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	if err := db.AutoMigrate(&models.Product{}, &models.AgentTeamMember{}); err != nil {
		t.Fatalf("auto migrate workbench action fixture: %v", err)
	}
	now := time.Now()
	teamID := createEnterpriseTicketActionRepairTeamFixture(t, 1, 706, 1810, true)
	ticket := models.Ticket{
		TicketNo: "WB-NO-SCHEDULE-CLAIM", TenantID: 1, ProductID: 706, CurrentTeamID: teamID,
		Status: enums.TicketStatusPendingDispatch, LastDispatchFailureReason: "no_active_schedule_team",
		AuditFields: models.AuditFields{CreatedAt: now.Add(-time.Hour), UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 1810, Roles: []string{EnterpriseRoleEngineer}, Status: enums.StatusOk}

	queue, err := EnterpriseWorkbenchService.Queue(1, operator.UserID, EnterpriseWorkbenchQueueQuery{Page: 1, PageSize: 10}, operator)
	if err != nil {
		t.Fatalf("workbench queue: %v", err)
	}
	item := findWorkbenchQueueTicketByNo(queue.Tickets, "WB-NO-SCHEDULE-CLAIM")
	if item == nil || item.Actions == nil {
		t.Fatalf("schedule-failed pool ticket should be visible with actions: %+v", queue.Tickets)
	}
	if !item.Actions.CanAccept {
		t.Fatalf("schedule-failed pool ticket should expose manual claim action: %+v", item.Actions)
	}
}

func findWorkbenchQueueTicketByNo(items []dto.EnterpriseTicketListItemDTO, ticketNo string) *dto.EnterpriseTicketListItemDTO {
	for index := range items {
		if items[index].TicketNo == ticketNo {
			return &items[index]
		}
	}
	return nil
}
