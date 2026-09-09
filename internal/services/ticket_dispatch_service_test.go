package services

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestTicketDispatchAssignsIndependentTicketAndTracksDeadline(t *testing.T) {
	previousWS := WsService
	WsService = newWsService()
	t.Cleanup(func() {
		WsService = previousWS
	})
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	ticket := models.Ticket{
		TicketNo: "AUTO-1", Title: "独立工单", TenantID: 1, CurrentTeamID: 1,
		Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: time.Now().Add(-time.Hour), UpdatedAt: time.Now()},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}

	dispatched, err := TicketDispatchService.DispatchPendingTickets(1)
	if err != nil || dispatched != 1 {
		t.Fatalf("DispatchPendingTickets() = (%d, %v), want (1, nil)", dispatched, err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.CurrentAssigneeID != 101 || current.Status != enums.TicketStatusPendingAssigneeAccept || current.AssignedAt == nil || current.AcceptDeadlineAt == nil {
		t.Fatalf("unexpected dispatched ticket: %+v", current)
	}
	var attempts []models.TicketDispatchAttempt
	if err := db.Where("ticket_id = ?", ticket.ID).Find(&attempts).Error; err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 1 || attempts[0].Outcome != ticketDispatchOutcomePending || attempts[0].AssigneeID != 101 {
		t.Fatalf("unexpected dispatch attempts: %+v", attempts)
	}
	assignedAt := *current.AssignedAt
	session := &ClientSession{
		ID:     "ticket-accept-notification-refresh",
		Role:   realtimeRoleNotification,
		Topics: map[string]struct{}{},
		Send:   make(chan []byte, 8),
	}
	WsService.manager.Register(session, []string{WsService.notificationTopic(101)})
	assignedNotification := &models.Notification{
		TenantID:         1,
		RecipientUserID:  101,
		Title:            "工单 AUTO-1 已分派",
		Content:          "请尽快接单处理",
		NotificationType: "ticket_assigned",
		BizType:          "ticket",
		BizID:            ticket.ID,
		Level:            "info",
		Category:         "ticket",
		Channels:         "in_app",
		DeliveryStatus:   "sent",
		Status:           int(enums.StatusOk),
		CreatedAt:        time.Now(),
	}
	if err := db.Create(assignedNotification).Error; err != nil {
		t.Fatalf("create assigned notification: %v", err)
	}

	operator := &dto.AuthPrincipal{UserID: 101, Username: "agent-101", TenantID: 1, Roles: []string{EnterpriseRoleEngineer}, Status: enums.StatusOk}
	if err := TicketLifecycleService.Accept(ticket.ID, 101, operator); err != nil {
		t.Fatalf("Accept() error = %v", err)
	}
	accepted := repositoriesTicket(t, ticket.ID)
	if accepted.AcceptedAt == nil || accepted.AcceptDeadlineAt != nil || accepted.AssignedAt == nil || !accepted.AssignedAt.Equal(assignedAt) {
		t.Fatalf("acceptance timestamps were not retained correctly: %+v", accepted)
	}
	if err := db.Where("ticket_id = ?", ticket.ID).Find(&attempts).Error; err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 1 || attempts[0].Outcome != ticketDispatchOutcomeAccepted || attempts[0].AcceptedAt == nil {
		t.Fatalf("dispatch attempt was not accepted: %+v", attempts)
	}
	reconciled := &models.Notification{}
	if err := db.First(reconciled, assignedNotification.ID).Error; err != nil {
		t.Fatalf("reload assigned notification: %v", err)
	}
	if reconciled.ReadAt == nil || !strings.Contains(reconciled.Title, "已接单") || !strings.Contains(reconciled.Content, "继续跟进") {
		t.Fatalf("assigned notification was not reconciled after acceptance: %+v", reconciled)
	}
	if unread := NotificationService.CountUnreadForTenant(1, 101); unread != 0 {
		t.Fatalf("accepted assignee unread notification count = %d, want 0", unread)
	}
	select {
	case raw := <-session.Send:
		var event struct {
			Type string `json:"type"`
			Data struct {
				Reason string `json:"reason"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &event); err != nil {
			t.Fatalf("decode notification resync event: %v", err)
		}
		if event.Type != enums.IMRealtimeEventResyncRequired || event.Data.Reason != enums.IMRealtimeResyncReasonNotificationUpdated {
			t.Fatalf("unexpected notification resync event: %s", raw)
		}
	case <-time.After(time.Second):
		t.Fatal("accepted assignee did not receive notification resync")
	}
}

func TestRecoverUnclaimedLinkedConversationTicketIsIdempotent(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	now := time.Now().Add(-time.Hour)
	conversation := models.Conversation{
		TenantID:      1,
		CustomerName:  "orphaned-linked-ticket",
		Status:        enums.IMConversationStatusAIServing,
		ServiceMode:   enums.IMConversationServiceModeAIFirst,
		LastMessageAt: now,
		LastActiveAt:  now,
		AuditFields:   models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	ticket := models.Ticket{
		TicketNo: "ORPHAN-LINKED-1", Title: "遗留关联工单", TenantID: 1, ConversationID: conversation.ID,
		CurrentTeamID: 5, Status: enums.TicketStatusPendingDispatch,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}

	recovered, err := TicketDispatchService.recoverUnclaimedLinkedConversationTickets(10)
	if err != nil || recovered != 1 {
		t.Fatalf("recoverUnclaimedLinkedConversationTickets() = (%d, %v), want (1, nil)", recovered, err)
	}
	current := repositories.ConversationRepository.Get(db, conversation.ID)
	if current == nil || current.Status != enums.IMConversationStatusPending || current.HandoffAt == nil || current.CurrentTeamID != 5 {
		t.Fatalf("conversation was not recovered: %+v", current)
	}
	recovered, err = TicketDispatchService.recoverUnclaimedLinkedConversationTickets(10)
	if err != nil || recovered != 0 {
		t.Fatalf("second recovery = (%d, %v), want (0, nil)", recovered, err)
	}
	var eventCount int64
	if err := db.Model(&models.ConversationEventLog{}).Where("conversation_id = ? AND event_type = ?", conversation.ID, enums.IMEventTypeTransfer).Count(&eventCount).Error; err != nil {
		t.Fatal(err)
	}
	if eventCount != 1 {
		t.Fatalf("recovery event count = %d, want 1", eventCount)
	}
}

func TestPendingConversationDispatchRespectsLinkedTicketBackoff(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	aiAgent := createHumanDispatchRealtimeAIAgent(t, db, "1")
	conversation := createHumanDispatchRealtimeConversation(t, db, aiAgent.ID)
	if err := db.Model(&models.Conversation{}).Where("id = ?", conversation.ID).Updates(map[string]any{
		"status":          enums.IMConversationStatusPending,
		"current_team_id": int64(1),
	}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	deferredUntil := now.Add(5 * time.Minute)
	ticket := models.Ticket{
		TicketNo: "LINKED-BACKOFF-1", Title: "关联工单退避", TenantID: 1, ConversationID: conversation.ID,
		CurrentTeamID: 1, Status: enums.TicketStatusPendingDispatch,
		DispatchDeferredUntil: &deferredUntil, LastDispatchFailureReason: acceptTimeoutRedispatchingCode,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}

	dispatched, err := ConversationDispatchService.DispatchPendingConversations(1)
	if err != nil || dispatched != 0 {
		t.Fatalf("DispatchPendingConversations() = (%d, %v), want (0, nil)", dispatched, err)
	}
	currentConversation := repositories.ConversationRepository.Get(db, conversation.ID)
	if currentConversation == nil || currentConversation.CurrentAssigneeID != 0 || currentConversation.Status != enums.IMConversationStatusPending {
		t.Fatalf("conversation bypassed linked ticket backoff: %+v", currentConversation)
	}
	currentTicket := repositoriesTicket(t, ticket.ID)
	if currentTicket.CurrentAssigneeID != 0 || currentTicket.DispatchDeferredUntil == nil || !currentTicket.DispatchDeferredUntil.Equal(deferredUntil) {
		t.Fatalf("ticket backoff was mutated by generic dispatch scan: %+v", currentTicket)
	}
}

func TestPendingConversationDispatchDoesNotReturnToLastFailedAssignee(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	aiAgent := createHumanDispatchRealtimeAIAgent(t, db, "1")
	conversation := createHumanDispatchRealtimeConversation(t, db, aiAgent.ID)
	if err := db.Model(&models.Conversation{}).Where("id = ?", conversation.ID).Updates(map[string]any{
		"status":          enums.IMConversationStatusPending,
		"current_team_id": int64(1),
	}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	deferredUntil := now.Add(-time.Minute)
	endedAt := now.Add(-2 * time.Minute)
	ticket := models.Ticket{
		TicketNo: "LINKED-FAILED-ASSIGNEE-1", Title: "关联工单失败处理人排除", TenantID: 1, ConversationID: conversation.ID,
		CurrentTeamID: 1, Status: enums.TicketStatusPendingDispatch, DispatchAttempts: 1,
		DispatchDeferredUntil: &deferredUntil, LastDispatchFailureReason: assigneeUnavailableRedispatchingCode,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TicketDispatchAttempt{
		TenantID: 1, TicketID: ticket.ID, TeamID: 1, AssigneeID: 101, AttemptNo: 1,
		Outcome: ticketDispatchOutcomeSuperseded, Reason: dispatchReasonAutomatic,
		AssignedAt: now.Add(-12 * time.Minute), EndedAt: &endedAt,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-12 * time.Minute), UpdatedAt: endedAt},
	}).Error; err != nil {
		t.Fatal(err)
	}

	dispatched, err := ConversationDispatchService.DispatchPendingConversations(1)
	if err != nil || dispatched != 0 {
		t.Fatalf("DispatchPendingConversations() = (%d, %v), want (0, nil)", dispatched, err)
	}
	currentConversation := repositories.ConversationRepository.Get(db, conversation.ID)
	if currentConversation == nil || currentConversation.CurrentAssigneeID != 0 || currentConversation.Status != enums.IMConversationStatusPending {
		t.Fatalf("conversation returned to last failed assignee: %+v", currentConversation)
	}
	currentTicket := repositoriesTicket(t, ticket.ID)
	if currentTicket.CurrentAssigneeID != 0 || currentTicket.LastDispatchFailureReason != "no_alternative_after_assignee_unavailable" ||
		currentTicket.DispatchDeferredUntil == nil || !currentTicket.DispatchDeferredUntil.After(now) {
		t.Fatalf("ticket did not enter explicit no-alternative backoff: %+v", currentTicket)
	}
}

func TestTicketDispatchFiltersCandidatesByEngineerCapability(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	if err := db.AutoMigrate(&models.TenantMember{}, &models.EngineerProfile{}); err != nil {
		t.Fatalf("migrate engineer capability tables: %v", err)
	}
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 102, 1)
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Update("product_id", int64(10)).Error; err != nil {
		t.Fatalf("bind team to product: %v", err)
	}
	now := time.Now()
	if err := db.Create(&models.Product{
		ID: 10, TenantID: 1, Code: "PWR", Name: "Power System", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	module := models.ProductModule{
		ID: 20, TenantID: 1, ProductID: 10, ModuleCode: "PWR-001", Name: "Power Module", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&module).Error; err != nil {
		t.Fatalf("create product module: %v", err)
	}
	createTicketDispatchEngineerCapability(t, db, 101, `["hydraulic"]`, `["EU"]`, true)
	createTicketDispatchEngineerCapability(t, db, 102, `["PWR-001","power module"]`, `["NA"]`, true)
	ticket := models.Ticket{
		TicketNo: "AUTO-CAPABILITY", Title: "按技能区域派单", TenantID: 1,
		ProductID: 10, ProductModuleID: module.ID, CurrentTeamID: 1,
		FaultCode: "PWR-001", ServiceRegion: "NA",
		Status:      enums.TicketStatusPending,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-time.Hour), UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	candidates, err := AgentProfileService.FindDispatchCandidates(DispatchCandidateQuery{TenantID: 1, TicketID: ticket.ID}, now)
	if err != nil {
		t.Fatalf("FindDispatchCandidates() error = %v", err)
	}
	if len(candidates) != 1 || candidates[0].Profile.UserID != 102 {
		t.Fatalf("candidate capability filter = %+v, want only engineer 102", candidates)
	}
	dispatched, err := TicketDispatchService.DispatchPendingTickets(1)
	if err != nil || dispatched != 1 {
		t.Fatalf("DispatchPendingTickets() = (%d, %v), want (1, nil)", dispatched, err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.CurrentAssigneeID != 102 || current.Status != enums.TicketStatusPendingAssigneeAccept {
		t.Fatalf("ticket should be dispatched to matching engineer: %+v", current)
	}
}

func TestTicketCreateWithInitialAssigneeIgnoresDisplayWorkStatusAndTracksPendingAccept(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "manager", Roles: []string{EnterpriseRoleServiceManager}}
	now := time.Now()
	if err := db.Model(&models.AgentWorkStatus{}).
		Where("tenant_id = ? AND user_id = ?", 1, 101).
		Updates(map[string]any{"status": AgentWorkStatusLeave, "status_changed_at": now}).Error; err != nil {
		t.Fatal(err)
	}

	created, err := TicketService.CreateTicket(request.CreateTicketRequest{
		Title:             "创建时指定请假工程师",
		Description:       "创建时指定请假工程师",
		TenantID:          1,
		CurrentTeamID:     1,
		CurrentAssigneeID: 101,
	}, operator)
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if created.Status != enums.TicketStatusPendingAssigneeAccept || created.CurrentAssigneeID != 101 || created.CurrentTeamID != 1 || created.AcceptDeadlineAt == nil {
		t.Fatalf("initial assignee did not enter pending acceptance cleanly: %+v", created)
	}
	var attempt models.TicketDispatchAttempt
	if err := db.First(&attempt, "ticket_id = ?", created.ID).Error; err != nil {
		t.Fatal(err)
	}
	if attempt.Outcome != ticketDispatchOutcomePending || attempt.AssigneeID != 101 || attempt.AcceptDeadlineAt == nil {
		t.Fatalf("initial assignee dispatch attempt not tracked: %+v", attempt)
	}
}

func TestAgentProfileAvailabilityDispatchesPendingTickets(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	now := time.Now()
	ticket := models.Ticket{
		TicketNo: "PROFILE-WAKEUP", Title: "工程师启用后即时派单", TenantID: 1, CurrentTeamID: 1,
		Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: now.Add(-time.Hour), UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	var profile models.AgentProfile
	if err := db.Where("user_id = ?", 101).First(&profile).Error; err != nil {
		t.Fatal(err)
	}

	AgentProfileService.dispatchPendingWorkIfEligible(&profile)

	current := repositoriesTicket(t, ticket.ID)
	if current.CurrentAssigneeID != 101 || current.Status != enums.TicketStatusPendingAssigneeAccept {
		t.Fatalf("pending ticket was not dispatched when profile became eligible: %+v", current)
	}
}

func TestTicketDispatchSkipsUndispatchableOldTicket(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	now := time.Now()
	tickets := []models.Ticket{
		{TicketNo: "BLOCKED-OLD", Title: "无维修组成员", TenantID: 1, CurrentTeamID: 999, Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now}},
		{TicketNo: "READY-NEW", Title: "可派工单", TenantID: 1, CurrentTeamID: 1, Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: now.Add(-time.Hour), UpdatedAt: now}},
	}
	if err := db.Create(&tickets).Error; err != nil {
		t.Fatal(err)
	}

	dispatched, err := TicketDispatchService.DispatchPendingTickets(1)
	if err != nil || dispatched != 1 {
		t.Fatalf("DispatchPendingTickets() = (%d, %v), want later dispatchable ticket", dispatched, err)
	}
	if current := repositoriesTicket(t, tickets[0].ID); current.CurrentAssigneeID != 0 {
		t.Fatalf("blocked ticket was unexpectedly assigned: %+v", current)
	}
	if current := repositoriesTicket(t, tickets[1].ID); current.CurrentAssigneeID != 101 {
		t.Fatalf("dispatchable ticket was starved: %+v", current)
	}
}

func TestTicketDispatchIgnoresHistoricalScheduleRowsForPendingAcceptance(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	previous := config.CurrentOrDefault()
	cfg := previous
	cfg.TicketDispatch.UnassignedEscalationMinutes = 30
	config.SetCurrent(&cfg)
	t.Cleanup(func() { config.SetCurrent(&previous) })

	productTeam := models.AgentTeam{
		ID: 61, TenantID: 1, ProductID: 906, TeamType: AgentTeamTypeProductRepair,
		Name: "历史时间表兼容维修组", ScheduleEnforced: true, Status: enums.StatusOk,
	}
	if err := db.Create(&productTeam).Error; err != nil {
		t.Fatal(err)
	}
	createHumanDispatchRealtimeAgentProfile(t, db, 101, productTeam.ID)
	now := time.Now()
	if err := db.Create(&models.AgentTeamSchedule{
		TenantID: 1, TeamID: productTeam.ID, UserID: 101, RepeatType: AgentTeamScheduleRepeatOnce,
		StartAt: now.Add(-2 * time.Hour), EndAt: now.Add(-time.Hour),
		PublishStatus: AgentTeamSchedulePublishPublished, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatal(err)
	}
	assignedAt := now.Add(-10 * time.Minute)
	deadlineAt := now.Add(10 * time.Minute)
	ticket := models.Ticket{
		TicketNo: "LEGACY-SCHEDULE-ASSIGN", Title: "历史时间表不再拦截派单", TenantID: 1,
		ProductID: productTeam.ProductID, CurrentTeamID: productTeam.ID,
		Status: enums.TicketStatusPendingAcceptance, AssignedAt: &assignedAt, AcceptDeadlineAt: &deadlineAt,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-5 * time.Minute), UpdatedAt: now.Add(-5 * time.Minute)},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}

	dispatched, err := TicketDispatchService.DispatchPendingTickets(1)
	if err != nil || dispatched != 1 {
		t.Fatalf("DispatchPendingTickets() = (%d, %v), want assignment from team members", dispatched, err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.Status != enums.TicketStatusPendingAssigneeAccept || current.CurrentAssigneeID != 101 || current.CurrentTeamID != productTeam.ID {
		t.Fatalf("historical schedule row should not block member assignment: %+v", current)
	}
	if current.AssignedAt == nil || current.AcceptDeadlineAt == nil || current.AcceptedAt != nil {
		t.Fatalf("assigned ticket did not refresh assignment tracking: %+v", current)
	}
	if current.DispatchDeferredUntil != nil || current.LastDispatchFailureReason != "" {
		t.Fatalf("assigned ticket kept stale dispatch failure evidence: %+v", current)
	}
	var attempt models.TicketDispatchAttempt
	if err := db.Where("ticket_id = ?", ticket.ID).First(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	if attempt.Outcome != ticketDispatchOutcomePending || attempt.AssigneeID != 101 || attempt.Reason != dispatchReasonAutomatic || attempt.EndedAt != nil {
		t.Fatalf("assignment dispatch attempt was not audited: %+v", attempt)
	}
}

func TestDispatchPendingConversationsUsesOldestCreatedAt(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	aiAgent := createHumanDispatchRealtimeAIAgent(t, db, "1")
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	now := time.Now()
	newer := createHumanDispatchRealtimeConversation(t, db, aiAgent.ID)
	older := createHumanDispatchRealtimeConversation(t, db, aiAgent.ID)
	if err := db.Model(&models.Conversation{}).Where("id = ?", newer.ID).Updates(map[string]any{
		"status":              enums.IMConversationStatusPending,
		"current_assignee_id": int64(0),
		"created_at":          now,
		"updated_at":          now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.Conversation{}).Where("id = ?", older.ID).Updates(map[string]any{
		"status":              enums.IMConversationStatusPending,
		"current_assignee_id": int64(0),
		"created_at":          now.Add(-time.Hour),
		"updated_at":          now,
	}).Error; err != nil {
		t.Fatal(err)
	}

	dispatched, err := ConversationDispatchService.DispatchPendingConversations(1)
	if err != nil || dispatched != 1 {
		t.Fatalf("DispatchPendingConversations() = (%d, %v), want oldest conversation dispatched", dispatched, err)
	}
	currentOlder := ConversationService.Get(older.ID)
	currentNewer := ConversationService.Get(newer.ID)
	if currentOlder == nil || currentOlder.CurrentAssigneeID != 101 {
		t.Fatalf("oldest conversation was not dispatched first: %+v", currentOlder)
	}
	if currentNewer == nil || currentNewer.CurrentAssigneeID != 0 {
		t.Fatalf("newer conversation was dispatched before older one: %+v", currentNewer)
	}
}

func TestTicketDispatchDefersBlockedWindowAndDispatchesLaterTicketNextScan(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	now := time.Now()
	blocked := make([]models.Ticket, 0, dispatchScanWindow(1))
	for i := 0; i < dispatchScanWindow(1); i++ {
		blocked = append(blocked, models.Ticket{
			TicketNo: "BLOCKED-WINDOW-" + string(rune('A'+i)), Title: "窗口内不可派", TenantID: 1, CurrentTeamID: 999,
			Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: now.Add(-time.Duration(10+i) * time.Minute), UpdatedAt: now},
		})
	}
	if err := db.Create(&blocked).Error; err != nil {
		t.Fatal(err)
	}
	ready := models.Ticket{
		TicketNo: "READY-AFTER-WINDOW", Title: "窗口后可派", TenantID: 1, CurrentTeamID: 1,
		Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&ready).Error; err != nil {
		t.Fatal(err)
	}

	first, err := TicketDispatchService.DispatchPendingTickets(1)
	if err != nil || first != 0 {
		t.Fatalf("first DispatchPendingTickets() = (%d, %v), want blocked window only", first, err)
	}
	for _, ticket := range blocked {
		current := repositoriesTicket(t, ticket.ID)
		if current.DispatchDeferredUntil == nil || current.LastDispatchFailureReason == "" {
			t.Fatalf("blocked ticket was not deferred: %+v", current)
		}
		var attempt models.TicketDispatchAttempt
		if err := db.Where("ticket_id = ?", ticket.ID).First(&attempt).Error; err != nil {
			t.Fatalf("find failed dispatch attempt: %v", err)
		}
		if attempt.Outcome != ticketDispatchOutcomeFailed || attempt.AssigneeID != 0 || attempt.Reason != current.LastDispatchFailureReason || attempt.EndedAt == nil {
			t.Fatalf("blocked ticket dispatch failure was not audited: ticket=%+v attempt=%+v", current, attempt)
		}
	}
	second, err := TicketDispatchService.DispatchPendingTickets(1)
	if err != nil || second != 1 {
		t.Fatalf("second DispatchPendingTickets() = (%d, %v), want ready ticket dispatched", second, err)
	}
	if current := repositoriesTicket(t, ready.ID); current.CurrentAssigneeID != 101 {
		t.Fatalf("ready ticket was still starved: %+v", current)
	}
}

func TestConversationDispatchHonorsLinkedTicketDispatchDefer(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	aiAgent := createHumanDispatchRealtimeAIAgent(t, db, "1")
	createHumanDispatchRealtimeTeam(t, db, 1)
	previous := config.CurrentOrDefault()
	cfg := previous
	cfg.TicketDispatch.UnassignedEscalationMinutes = 30
	config.SetCurrent(&cfg)
	t.Cleanup(func() { config.SetCurrent(&previous) })

	now := time.Now()
	deferUntil := now.Add(15 * time.Minute)
	conversation := createHumanDispatchRealtimeConversation(t, db, aiAgent.ID)
	if err := db.Model(&models.Conversation{}).Where("id = ?", conversation.ID).Updates(map[string]any{
		"status":              enums.IMConversationStatusPending,
		"current_team_id":     int64(1),
		"current_assignee_id": int64(0),
	}).Error; err != nil {
		t.Fatal(err)
	}
	ticket := models.Ticket{
		TicketNo: "CONV-DEFER", Title: "会话关联工单延迟窗口", TenantID: 1, ConversationID: conversation.ID, CurrentTeamID: 1,
		Status: enums.TicketStatusPending, DispatchDeferredUntil: &deferUntil, LastDispatchFailureReason: "initial_defer",
		AuditFields: models.AuditFields{CreatedAt: now.Add(-5 * time.Minute), UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}

	dispatched, err := ConversationDispatchService.DispatchConversation(conversation.ID)
	if err != nil || dispatched != nil {
		t.Fatalf("DispatchConversation() = (%+v, %v), want deferred linked ticket skipped", dispatched, err)
	}
	currentTicket := repositoriesTicket(t, ticket.ID)
	if currentTicket.CurrentAssigneeID != 0 || currentTicket.Status != enums.TicketStatusPending {
		t.Fatalf("deferred linked ticket was unexpectedly dispatched: %+v", currentTicket)
	}
	if currentTicket.LastDispatchFailureReason != "initial_defer" {
		t.Fatalf("deferred linked ticket failure reason was rewritten: %+v", currentTicket)
	}
	if currentTicket.DispatchDeferredUntil == nil || currentTicket.DispatchDeferredUntil.Before(deferUntil.Add(-time.Second)) || currentTicket.DispatchDeferredUntil.After(deferUntil.Add(time.Second)) {
		t.Fatalf("deferred linked ticket retry window changed: %+v", currentTicket.DispatchDeferredUntil)
	}
}

func TestTicketDispatchRecoversExpiredAcceptanceAndReassigns(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 102, 1)
	now := time.Now()
	assignedAt := now.Add(-time.Hour)
	deadline := now.Add(-time.Minute)
	ticket := models.Ticket{
		TicketNo: "TIMEOUT-1", Title: "接单超时", TenantID: 1, CurrentTeamID: 1, CurrentAssigneeID: 101,
		Status: enums.TicketStatusPendingAssigneeAccept, AssignedAt: &assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TicketDispatchAttempt{
		TenantID: 1, TicketID: ticket.ID, TeamID: 1, AssigneeID: 101, AttemptNo: 1,
		Outcome: ticketDispatchOutcomePending, AssignedAt: assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}).Error; err != nil {
		t.Fatal(err)
	}

	recovered, err := TicketDispatchService.RecoverExpiredTicketAcceptances(1)
	if err != nil || recovered != 1 {
		t.Fatalf("RecoverExpiredTicketAcceptances() = (%d, %v)", recovered, err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.CurrentAssigneeID != 102 || current.DispatchAttempts != 1 || current.AcceptDeadlineAt == nil || !current.AcceptDeadlineAt.After(now) {
		t.Fatalf("ticket was not immediately reassigned: %+v", current)
	}
	var attempts []models.TicketDispatchAttempt
	if err := db.Where("ticket_id = ?", ticket.ID).Order("id ASC").Find(&attempts).Error; err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 2 || attempts[0].Outcome != ticketDispatchOutcomeTimedOut || attempts[1].Outcome != ticketDispatchOutcomePending || attempts[1].AssigneeID != 102 {
		t.Fatalf("unexpected timeout/reassignment attempts: %+v", attempts)
	}
}

func TestAgentWorkStatusLeaveDoesNotOverrideCalendarDispatch(t *testing.T) {
	previousWS := WsService
	WsService = newWsService()
	t.Cleanup(func() {
		WsService = previousWS
	})
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 102, 1)
	now := time.Now()
	assignedAt := now.Add(-5 * time.Minute)
	deadline := now.Add(25 * time.Minute)
	ticket := models.Ticket{
		TicketNo: "LEAVE-RECOVER", Title: "工程师离岗回收", TenantID: 1, CurrentTeamID: 1, CurrentAssigneeID: 101,
		Status: enums.TicketStatusPendingAssigneeAccept, AssignedAt: &assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TicketDispatchAttempt{
		TenantID: 1, TicketID: ticket.ID, TeamID: 1, AssigneeID: 101, AttemptNo: 1,
		Outcome: ticketDispatchOutcomePending, AssignedAt: assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}).Error; err != nil {
		t.Fatal(err)
	}
	assignedNotification := &models.Notification{
		TenantID:         1,
		RecipientUserID:  101,
		Title:            "工单 LEAVE-RECOVER 已分派",
		Content:          "请尽快接单处理",
		NotificationType: "ticket_assigned",
		BizType:          "ticket",
		BizID:            ticket.ID,
		Level:            "info",
		Category:         "ticket",
		Channels:         "in_app",
		DeliveryStatus:   "sent",
		Status:           int(enums.StatusOk),
		CreatedAt:        now,
	}
	if err := db.Create(assignedNotification).Error; err != nil {
		t.Fatalf("create assigned notification: %v", err)
	}
	session := &ClientSession{
		ID:     "ticket-leave-notification-refresh",
		Role:   realtimeRoleNotification,
		Topics: map[string]struct{}{},
		Send:   make(chan []byte, 8),
	}
	WsService.manager.Register(session, []string{WsService.notificationTopic(101)})
	engineer := &dto.AuthPrincipal{TenantID: 1, UserID: 101, Username: "agent-101", Roles: []string{EnterpriseRoleEngineer}, Status: enums.StatusOk}

	if _, err := AgentWorkStatusService.UpdateMyStatus(request.UpdateAgentWorkStatusRequest{
		Status:      AgentWorkStatusLeave,
		Note:        "现场外出",
		AvailableAt: now.Add(2 * time.Hour).Format(time.RFC3339),
	}, engineer, now); err != nil {
		t.Fatalf("UpdateMyStatus() error = %v", err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.CurrentAssigneeID != 101 || current.Status != enums.TicketStatusPendingAssigneeAccept || current.AcceptDeadlineAt == nil || current.DispatchDeferredUntil != nil || current.LastDispatchFailureReason != "" {
		t.Fatalf("work status change should not mutate the pending assignment: %+v", current)
	}
	if current.DispatchAttempts != 0 {
		t.Fatalf("work status change unexpectedly changed dispatch attempts: %+v", current)
	}
	var attempts []models.TicketDispatchAttempt
	if err := db.Where("ticket_id = ?", ticket.ID).Order("id ASC").Find(&attempts).Error; err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 1 || attempts[0].Outcome != ticketDispatchOutcomePending || attempts[0].AssigneeID != 101 {
		t.Fatalf("unexpected attempts after work status update: %+v", attempts)
	}
	reconciled := &models.Notification{}
	if err := db.First(reconciled, assignedNotification.ID).Error; err != nil {
		t.Fatalf("reload assigned notification: %v", err)
	}
	if reconciled.ReadAt != nil || reconciled.Title != assignedNotification.Title || reconciled.Content != assignedNotification.Content {
		t.Fatalf("assigned notification should remain unchanged after work status update: %+v", reconciled)
	}
	if unread := NotificationService.CountUnreadForTenant(1, 101); unread != 1 {
		t.Fatalf("assigned engineer unread notification count = %d, want 1", unread)
	}
	select {
	case raw := <-session.Send:
		t.Fatalf("work status update unexpectedly published notification resync: %s", raw)
	default:
	}
}

func TestAgentWorkStatusLeaveDoesNotOverrideLinkedCalendarDispatch(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	aiAgent := createHumanDispatchRealtimeAIAgent(t, db, "1")
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 102, 1)
	conversation := createHumanDispatchRealtimeConversation(t, db, aiAgent.ID)
	now := time.Now()
	if err := db.Model(&models.Conversation{}).Where("id = ?", conversation.ID).Updates(map[string]any{
		"status":              enums.IMConversationStatusPending,
		"current_team_id":     int64(1),
		"current_assignee_id": int64(101),
		"updated_at":          now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	assignedAt := now.Add(-5 * time.Minute)
	deadline := now.Add(25 * time.Minute)
	ticket := models.Ticket{
		TicketNo: "LEAVE-CONV", Title: "会话来源工单离岗回收", TenantID: 1, ConversationID: conversation.ID,
		CurrentTeamID: 1, CurrentAssigneeID: 101, Status: enums.TicketStatusPendingAssigneeAccept,
		AssignedAt: &assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TicketDispatchAttempt{
		TenantID: 1, TicketID: ticket.ID, TeamID: 1, AssigneeID: 101, AttemptNo: 1,
		Outcome: ticketDispatchOutcomePending, AssignedAt: assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}).Error; err != nil {
		t.Fatal(err)
	}
	engineer := &dto.AuthPrincipal{TenantID: 1, UserID: 101, Username: "agent-101", Roles: []string{EnterpriseRoleEngineer}, Status: enums.StatusOk}

	if _, err := AgentWorkStatusService.UpdateMyStatus(request.UpdateAgentWorkStatusRequest{
		Status:      AgentWorkStatusLeave,
		AvailableAt: now.Add(2 * time.Hour).Format(time.RFC3339),
	}, engineer, now); err != nil {
		t.Fatalf("UpdateMyStatus() error = %v", err)
	}
	currentConversation := ConversationService.Get(conversation.ID)
	if currentConversation == nil || currentConversation.CurrentAssigneeID != 101 || currentConversation.Status != enums.IMConversationStatusPending {
		t.Fatalf("work status change should not mutate linked conversation assignment: %+v", currentConversation)
	}
	currentTicket := repositoriesTicket(t, ticket.ID)
	if currentTicket.CurrentAssigneeID != 101 || currentTicket.CurrentTeamID != 1 || currentTicket.Status != enums.TicketStatusPendingAssigneeAccept || currentTicket.AcceptDeadlineAt == nil {
		t.Fatalf("linked ticket should remain assigned after work status update: %+v", currentTicket)
	}
	var attempts []models.TicketDispatchAttempt
	if err := db.Where("ticket_id = ?", ticket.ID).Order("id ASC").Find(&attempts).Error; err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 1 || attempts[0].Outcome != ticketDispatchOutcomePending || attempts[0].AssigneeID != 101 {
		t.Fatalf("unexpected linked attempts after work status update: %+v", attempts)
	}
}

func TestAgentProfileDispatchDisableRecoversPendingTicket(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 102, 1)
	now := time.Now()
	assignedAt := now.Add(-5 * time.Minute)
	deadline := now.Add(25 * time.Minute)
	ticket := models.Ticket{
		TicketNo: "PROFILE-DISABLE", Title: "关闭自动派单回收", TenantID: 1, CurrentTeamID: 1, CurrentAssigneeID: 101,
		Status: enums.TicketStatusPendingAssigneeAccept, AssignedAt: &assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TicketDispatchAttempt{
		TenantID: 1, TicketID: ticket.ID, TeamID: 1, AssigneeID: 101, AttemptNo: 1,
		Outcome: ticketDispatchOutcomePending, AssignedAt: assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}).Error; err != nil {
		t.Fatal(err)
	}
	var profile models.AgentProfile
	if err := db.Where("tenant_id = ? AND user_id = ?", 1, 101).First(&profile).Error; err != nil {
		t.Fatal(err)
	}
	manager := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "manager", Roles: []string{EnterpriseRoleServiceManager}}

	if err := AgentProfileService.UpdateAgentProfile(request.UpdateAgentProfileRequest{
		ID: profile.ID,
		CreateAgentProfileRequest: request.CreateAgentProfileRequest{
			UserID: profile.UserID, TeamID: profile.TeamID, AgentCode: profile.AgentCode, DisplayName: profile.DisplayName,
			ServiceStatus: enums.ServiceStatusIdle, MaxConcurrentCount: profile.MaxConcurrentCount, PriorityLevel: profile.PriorityLevel,
			AutoAssignEnabled: false, ReceiveOfflineMessage: true,
		},
	}, manager); err != nil {
		t.Fatalf("UpdateAgentProfile() error = %v", err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.CurrentAssigneeID != 102 || current.Status != enums.TicketStatusPendingAssigneeAccept || current.AcceptDeadlineAt == nil {
		t.Fatalf("profile dispatch disable did not reassign pending ticket: %+v", current)
	}
	var attempts []models.TicketDispatchAttempt
	if err := db.Where("ticket_id = ?", ticket.ID).Order("id ASC").Find(&attempts).Error; err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 2 || attempts[0].Outcome != ticketDispatchOutcomeSuperseded || attempts[1].Outcome != ticketDispatchOutcomePending || attempts[1].AssigneeID != 102 {
		t.Fatalf("unexpected profile disable attempts: %+v", attempts)
	}
}

func TestAgentProfileDeleteRecoversPendingTicket(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 102, 1)
	now := time.Now()
	assignedAt := now.Add(-5 * time.Minute)
	deadline := now.Add(25 * time.Minute)
	ticket := models.Ticket{
		TicketNo: "PROFILE-DELETE", Title: "删除工程师档案回收", TenantID: 1, CurrentTeamID: 1, CurrentAssigneeID: 101,
		Status: enums.TicketStatusPendingAssigneeAccept, AssignedAt: &assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TicketDispatchAttempt{
		TenantID: 1, TicketID: ticket.ID, TeamID: 1, AssigneeID: 101, AttemptNo: 1,
		Outcome: ticketDispatchOutcomePending, AssignedAt: assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}).Error; err != nil {
		t.Fatal(err)
	}
	var profile models.AgentProfile
	if err := db.Where("tenant_id = ? AND user_id = ?", 1, 101).First(&profile).Error; err != nil {
		t.Fatal(err)
	}
	manager := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "manager", Roles: []string{EnterpriseRoleServiceManager}}

	if err := AgentProfileService.DeleteAgentProfile(profile.ID, manager); err != nil {
		t.Fatalf("DeleteAgentProfile() error = %v", err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.CurrentAssigneeID != 102 || current.Status != enums.TicketStatusPendingAssigneeAccept || current.AcceptDeadlineAt == nil {
		t.Fatalf("profile delete did not reassign pending ticket: %+v", current)
	}
	var attempts []models.TicketDispatchAttempt
	if err := db.Where("ticket_id = ?", ticket.ID).Order("id ASC").Find(&attempts).Error; err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 2 || attempts[0].Outcome != ticketDispatchOutcomeSuperseded || attempts[1].Outcome != ticketDispatchOutcomePending || attempts[1].AssigneeID != 102 {
		t.Fatalf("unexpected profile delete attempts: %+v", attempts)
	}
}

func TestAgentWorkStatusMarkOfflineRecoversPendingTicket(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 102, 1)
	now := time.Now()
	assignedAt := now.Add(-5 * time.Minute)
	deadline := now.Add(25 * time.Minute)
	ticket := models.Ticket{
		TicketNo: "LOGOUT-RECOVER", Title: "登出回收待接单", TenantID: 1, CurrentTeamID: 1, CurrentAssigneeID: 101,
		Status: enums.TicketStatusPendingAssigneeAccept, AssignedAt: &assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TicketDispatchAttempt{
		TenantID: 1, TicketID: ticket.ID, TeamID: 1, AssigneeID: 101, AttemptNo: 1,
		Outcome: ticketDispatchOutcomePending, AssignedAt: assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := AgentWorkStatusService.MarkOffline(101, now); err != nil {
		t.Fatalf("MarkOffline() error = %v", err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.CurrentAssigneeID != 102 || current.Status != enums.TicketStatusPendingAssigneeAccept || current.AcceptDeadlineAt == nil {
		t.Fatalf("logout did not reassign pending ticket: %+v", current)
	}
	var attempts []models.TicketDispatchAttempt
	if err := db.Where("ticket_id = ?", ticket.ID).Order("id ASC").Find(&attempts).Error; err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 2 || attempts[0].Outcome != ticketDispatchOutcomeSuperseded || attempts[1].Outcome != ticketDispatchOutcomePending || attempts[1].AssigneeID != 102 {
		t.Fatalf("unexpected logout recovery attempts: %+v", attempts)
	}
}

func TestTicketDispatchRecoversPendingAssigneeWhenLeaveExcludesEngineer(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 102, 1)
	now := time.Now()
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Update("schedule_enforced", true).Error; err != nil {
		t.Fatal(err)
	}
	ensureEnterpriseTemplateCoversTime(t, db, 1, now)
	if err := db.Create(&models.AgentScheduleException{
		TenantID: 1, UserID: 101, RequestKey: "ticket-recover-leave-101",
		ExceptionType: AgentScheduleExceptionTypeLeave, StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour),
		ApprovalStatus: AgentScheduleApprovalApproved, RequestedAt: now.Add(-2 * time.Hour), Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour)},
	}).Error; err != nil {
		t.Fatal(err)
	}
	assignedAt := now.Add(-5 * time.Minute)
	deadline := now.Add(25 * time.Minute)
	ticket := models.Ticket{
		TicketNo: "LEAVE-RECOVER", Title: "请假工程师待接单回收", TenantID: 1, CurrentTeamID: 1, CurrentAssigneeID: 101,
		Status: enums.TicketStatusPendingAssigneeAccept, AssignedAt: &assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TicketDispatchAttempt{
		TenantID: 1, TicketID: ticket.ID, TeamID: 1, AssigneeID: 101, AttemptNo: 1,
		Outcome: ticketDispatchOutcomePending, AssignedAt: assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}).Error; err != nil {
		t.Fatal(err)
	}

	recovered, err := TicketDispatchService.RecoverIneligibleTicketAcceptances(1)
	if err != nil || recovered != 1 {
		t.Fatalf("RecoverIneligibleTicketAcceptances() = (%d, %v)", recovered, err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.CurrentAssigneeID != 102 || current.Status != enums.TicketStatusPendingAssigneeAccept || current.AcceptDeadlineAt == nil {
		t.Fatalf("approved leave ineligibility did not reassign pending ticket: %+v", current)
	}
	var attempts []models.TicketDispatchAttempt
	if err := db.Where("ticket_id = ?", ticket.ID).Order("id ASC").Find(&attempts).Error; err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 2 || attempts[0].Outcome != ticketDispatchOutcomeSuperseded || attempts[1].Outcome != ticketDispatchOutcomePending || attempts[1].AssigneeID != 102 {
		t.Fatalf("unexpected leave recovery attempts: %+v", attempts)
	}
}

func TestTicketDispatchRecoversLegacyAssignedTicketWithoutTrackingFields(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 102, 1)
	now := time.Now()
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Update("schedule_enforced", true).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.AgentTeamSchedule{
		TenantID: 1, TeamID: 1, UserID: 102, RepeatType: AgentTeamScheduleRepeatOnce,
		StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour),
		PublishStatus: AgentTeamSchedulePublishPublished, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatal(err)
	}
	ticket := models.Ticket{
		TicketNo: "LEGACY-ASSIGNED-RECOVER", Title: "缺少派单跟踪字段的旧工单", TenantID: 1,
		CurrentTeamID: 1, CurrentAssigneeID: 101, Status: enums.TicketStatusAssigned,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}

	recovered, err := TicketDispatchService.RecoverIneligibleTicketAcceptances(1)
	if err != nil || recovered != 1 {
		t.Fatalf("RecoverIneligibleTicketAcceptances() = (%d, %v)", recovered, err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.CurrentAssigneeID != 102 || current.Status != enums.TicketStatusPendingAssigneeAccept ||
		current.AssignedAt == nil || current.AcceptDeadlineAt == nil || current.AcceptedAt != nil {
		t.Fatalf("legacy assigned ticket was not reconstructed through normal redispatch: %+v", current)
	}
	var attempts []models.TicketDispatchAttempt
	if err := db.Where("ticket_id = ?", ticket.ID).Order("id ASC").Find(&attempts).Error; err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 1 || attempts[0].Outcome != ticketDispatchOutcomePending || attempts[0].AssigneeID != 102 {
		t.Fatalf("legacy assignment recovery did not create a tracked dispatch attempt: %+v", attempts)
	}
}

func TestLegacySchedulePublishDoesNotDriveTicketDispatch(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	now := time.Now()
	draft := models.AgentTeamSchedule{
		TenantID: 1, TeamID: 1, UserID: 101, RepeatType: AgentTeamScheduleRepeatOnce,
		StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour),
		PublishStatus: AgentTeamSchedulePublishDraft, Version: 1, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&draft).Error; err != nil {
		t.Fatal(err)
	}

	result, err := AgentTeamScheduleService.PublishDraft(1, true, &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "manager"})
	if err == nil || !strings.Contains(err.Error(), "旧排班写入已下线") {
		t.Fatalf("PublishDraft() = (%+v, %v), want legacy write disabled", result, err)
	}
	var refreshed models.AgentTeamSchedule
	if err := db.First(&refreshed, draft.ID).Error; err != nil {
		t.Fatal(err)
	}
	if refreshed.PublishStatus != AgentTeamSchedulePublishDraft {
		t.Fatalf("legacy schedule row should remain untouched: %+v", refreshed)
	}
}

func TestAgentTeamMemberDispatchDisableRecoversPendingTicketAndBlocksFallback(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 102, 1)
	now := time.Now()
	assignedAt := now.Add(-5 * time.Minute)
	deadline := now.Add(25 * time.Minute)
	ticket := models.Ticket{
		TicketNo: "MEMBER-DISABLE", Title: "成员停派回收", TenantID: 1, CurrentTeamID: 1, CurrentAssigneeID: 101,
		Status: enums.TicketStatusPendingAssigneeAccept, AssignedAt: &assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TicketDispatchAttempt{
		TenantID: 1, TicketID: ticket.ID, TeamID: 1, AssigneeID: 101, AttemptNo: 1,
		Outcome: ticketDispatchOutcomePending, AssignedAt: assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}).Error; err != nil {
		t.Fatal(err)
	}
	manager := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "manager", Roles: []string{EnterpriseRoleServiceManager}}

	if _, err := AgentTeamMemberService.EnsureMember(1, 1, 101, 0, 1, false, manager); err != nil {
		t.Fatalf("EnsureMember() error = %v", err)
	}
	var member models.AgentTeamMember
	if err := db.Where("tenant_id = ? AND team_id = ? AND user_id = ?", 1, 1, 101).First(&member).Error; err != nil {
		t.Fatalf("load dispatch-disabled membership: %v", err)
	}
	if member.DispatchEnabled {
		t.Fatalf("dispatch-disabled membership was not saved: %+v", member)
	}
	if AgentTeamMemberService.IsUserDispatchEnabledMemberOfTeamDB(db, 1, 1, 101) {
		t.Fatal("dispatch-disabled member is still considered dispatch eligible")
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.CurrentAssigneeID != 102 || current.Status != enums.TicketStatusPendingAssigneeAccept || current.AcceptDeadlineAt == nil {
		t.Fatalf("member dispatch disable did not reassign pending ticket: %+v", current)
	}
	candidates, _, err := ConversationDispatchService.pickDispatchCandidates(1, 0, []int64{1}, now)
	if err != nil {
		t.Fatalf("pick candidates: %v", err)
	}
	for _, candidate := range candidates {
		if candidate.profile.UserID == 101 {
			t.Fatalf("dispatch-disabled member leaked through profile fallback: %+v", candidates)
		}
	}
	var attempts []models.TicketDispatchAttempt
	if err := db.Where("ticket_id = ?", ticket.ID).Order("id ASC").Find(&attempts).Error; err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 2 || attempts[0].Outcome != ticketDispatchOutcomeSuperseded || attempts[1].Outcome != ticketDispatchOutcomePending || attempts[1].AssigneeID != 102 {
		t.Fatalf("unexpected member disable attempts: %+v", attempts)
	}
}

func TestManualTicketAssignmentAllowsDispatchDisabledTeamMember(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	manager := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "manager", Roles: []string{EnterpriseRoleServiceManager}}
	if _, err := AgentTeamMemberService.EnsureMember(1, 1, 101, 0, 1, false, manager); err != nil {
		t.Fatalf("EnsureMember() error = %v", err)
	}
	now := time.Now()
	ticket := models.Ticket{
		TicketNo: "MANUAL-MEMBER-DISABLED", Title: "不能派给本组停派成员", TenantID: 1, CurrentTeamID: 1,
		Status: enums.TicketStatusPendingDispatch, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}

	if err := TicketLifecycleService.Assign(ticket.ID, 101, "经理人工指定本组停派成员", manager); err != nil {
		t.Fatalf("manual assignment should override the automatic-dispatch toggle: %v", err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.CurrentAssigneeID != 101 || current.Status != enums.TicketStatusPendingAssigneeAccept || current.AcceptDeadlineAt == nil {
		t.Fatalf("manual assignment did not enter pending acceptance: %+v", current)
	}
}

func TestAgentTeamDisableRecoversPendingTicket(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	now := time.Now()
	assignedAt := now.Add(-5 * time.Minute)
	deadline := now.Add(25 * time.Minute)
	ticket := models.Ticket{
		TicketNo: "TEAM-DISABLE", Title: "团队禁用回收", TenantID: 1, CurrentTeamID: 1, CurrentAssigneeID: 101,
		Status: enums.TicketStatusPendingAssigneeAccept, AssignedAt: &assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TicketDispatchAttempt{
		TenantID: 1, TicketID: ticket.ID, TeamID: 1, AssigneeID: 101, AttemptNo: 1,
		Outcome: ticketDispatchOutcomePending, AssignedAt: assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}).Error; err != nil {
		t.Fatal(err)
	}
	poolTicket := models.Ticket{
		TicketNo: "TEAM-DISABLE-POOL", Title: "团队禁用池内工单回收", TenantID: 1, CurrentTeamID: 1,
		Status:      enums.TicketStatusPendingDispatch,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-10 * time.Minute), UpdatedAt: now.Add(-10 * time.Minute)},
	}
	if err := db.Create(&poolTicket).Error; err != nil {
		t.Fatal(err)
	}
	manager := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "manager", Roles: []string{EnterpriseRoleServiceManager}}

	if err := AgentTeamService.UpdateAgentTeam(request.UpdateAgentTeamRequest{
		ID:     1,
		Name:   "售后支持组",
		Status: int(enums.StatusDisabled),
	}, manager); err != nil {
		t.Fatalf("UpdateAgentTeam() error = %v", err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.CurrentAssigneeID != 0 || current.CurrentTeamID != 0 || current.Status != enums.TicketStatusPendingDispatch || current.AcceptDeadlineAt != nil || current.LastDispatchFailureReason == "" {
		t.Fatalf("team disable did not return pending assignment to the dispatch pool: %+v", current)
	}
	currentPool := repositoriesTicket(t, poolTicket.ID)
	if currentPool.CurrentAssigneeID != 0 || currentPool.CurrentTeamID != 0 || currentPool.Status != enums.TicketStatusPendingDispatch || currentPool.LastDispatchFailureReason != "team_disabled_pool_released" {
		t.Fatalf("team disable did not release existing team-pool ticket: %+v", currentPool)
	}
	var attempts []models.TicketDispatchAttempt
	if err := db.Where("ticket_id = ?", ticket.ID).Order("id ASC").Find(&attempts).Error; err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 1 || attempts[0].Outcome != ticketDispatchOutcomeSuperseded || attempts[0].EndedAt == nil {
		t.Fatalf("unexpected team disable attempts: %+v", attempts)
	}
}

func TestTicketDispatchRecycleDefersGenericScanUntilExcludedRedispatch(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 102, 1)
	now := time.Now()
	assignedAt := now.Add(-time.Hour)
	deadline := now.Add(-time.Minute)
	ticket := models.Ticket{
		TicketNo: "TIMEOUT-RACE", Title: "接单超时抢跑保护", TenantID: 1, CurrentTeamID: 1, CurrentAssigneeID: 101,
		Status: enums.TicketStatusPendingAssigneeAccept, AssignedAt: &assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TicketDispatchAttempt{
		TenantID: 1, TicketID: ticket.ID, TeamID: 1, AssigneeID: 101, AttemptNo: 1,
		Outcome: ticketDispatchOutcomePending, AssignedAt: assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}).Error; err != nil {
		t.Fatal(err)
	}

	recycled, err := TicketDispatchService.recycleTicketAfterAcceptTimeout(&ticket, 101, 1, now, false)
	if err != nil || !recycled {
		t.Fatalf("recycleTicketAfterAcceptTimeout() = (%v, %v)", recycled, err)
	}
	recycledTicket := repositoriesTicket(t, ticket.ID)
	if recycledTicket.CurrentAssigneeID != 0 || recycledTicket.DispatchDeferredUntil == nil || !recycledTicket.DispatchDeferredUntil.After(now) || recycledTicket.LastDispatchFailureReason != acceptTimeoutRedispatchingCode {
		t.Fatalf("recycled ticket is not protected from generic dispatch scan: %+v", recycledTicket)
	}

	genericDispatched, err := TicketDispatchService.DispatchPendingTickets(1)
	if err != nil || genericDispatched != 0 {
		t.Fatalf("generic DispatchPendingTickets() = (%d, %v), want deferred timeout ticket skipped", genericDispatched, err)
	}
	stillDeferred := repositoriesTicket(t, ticket.ID)
	if stillDeferred.CurrentAssigneeID != 0 {
		t.Fatalf("generic scan reassigned timeout ticket before excluded redispatch: %+v", stillDeferred)
	}

	dispatched, err := TicketDispatchService.dispatchTicket(stillDeferred, 101, now)
	if err != nil {
		t.Fatalf("excluded redispatch error = %v", err)
	}
	if dispatched == nil || dispatched.CurrentAssigneeID != 102 || dispatched.DispatchDeferredUntil != nil || dispatched.LastDispatchFailureReason != "" {
		t.Fatalf("excluded redispatch did not assign the alternative engineer and clear defer: %+v", dispatched)
	}
}

func TestTicketDispatchEscalatesToTeamLeaderAfterAttemptLimit(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	createHumanDispatchRealtimeSupervisor(t, db, 202, 1)
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Update("leader_user_id", 202).Error; err != nil {
		t.Fatal(err)
	}
	previous := config.CurrentOrDefault()
	cfg := previous
	cfg.TicketDispatch.MaxDispatchAttempts = 1
	config.SetCurrent(&cfg)
	t.Cleanup(func() { config.SetCurrent(&previous) })

	now := time.Now()
	assignedAt := now.Add(-time.Hour)
	deadline := now.Add(-time.Minute)
	ticket := models.Ticket{
		TicketNo: "ESCALATE-1", Title: "多次超时", TenantID: 1, CurrentTeamID: 1, CurrentAssigneeID: 101,
		Status: enums.TicketStatusPendingAssigneeAccept, AssignedAt: &assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TicketDispatchAttempt{
		TenantID: 1, TicketID: ticket.ID, TeamID: 1, AssigneeID: 101, AttemptNo: 1,
		Outcome: ticketDispatchOutcomePending, AssignedAt: assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}).Error; err != nil {
		t.Fatal(err)
	}

	recovered, err := TicketDispatchService.RecoverExpiredTicketAcceptances(1)
	if err != nil || recovered != 1 {
		t.Fatalf("RecoverExpiredTicketAcceptances() = (%d, %v)", recovered, err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.Status != enums.TicketStatusProcessing || current.CurrentAssigneeID != 202 || current.AcceptDeadlineAt != nil || current.AcceptedAt == nil {
		t.Fatalf("ticket was not escalated to leader: %+v", current)
	}
	var attempts []models.TicketDispatchAttempt
	if err := db.Where("ticket_id = ?", ticket.ID).Order("id ASC").Find(&attempts).Error; err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 2 || attempts[0].Outcome != ticketDispatchOutcomeEscalated || attempts[1].Outcome != ticketDispatchOutcomeEscalated || attempts[1].AssigneeID != 202 {
		t.Fatalf("unexpected supervisor takeover audit: %+v", attempts)
	}
}

func TestTicketDispatchEscalatesOldUnassignedTicketToTeamLeader(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeSupervisor(t, db, 202, 1)
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Update("leader_user_id", 202).Error; err != nil {
		t.Fatal(err)
	}
	previous := config.CurrentOrDefault()
	cfg := previous
	cfg.TicketDispatch.UnassignedEscalationMinutes = 30
	config.SetCurrent(&cfg)
	t.Cleanup(func() { config.SetCurrent(&previous) })

	now := time.Now()
	ticket := models.Ticket{
		TicketNo: "UNASSIGNED-OLD", Title: "长时间无人可派", TenantID: 1, CurrentTeamID: 1,
		Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: now.Add(-31 * time.Minute), UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}

	dispatched, err := TicketDispatchService.DispatchPendingTickets(1)
	if err != nil || dispatched != 1 {
		t.Fatalf("DispatchPendingTickets() = (%d, %v), want supervisor takeover", dispatched, err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.Status != enums.TicketStatusProcessing || current.CurrentAssigneeID != 202 || current.AcceptDeadlineAt != nil || current.AcceptedAt == nil {
		t.Fatalf("old unassigned ticket was not escalated: %+v", current)
	}
	var attempts []models.TicketDispatchAttempt
	if err := db.Where("ticket_id = ?", ticket.ID).Find(&attempts).Error; err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 1 || attempts[0].Outcome != ticketDispatchOutcomeEscalated || attempts[0].AssigneeID != 202 {
		t.Fatalf("unexpected supervisor dispatch audit: %+v", attempts)
	}
}

func TestTicketDispatchReopenStartsFreshUnassignedEscalationWindow(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	migrateTicketLifecycleSideEffectTables(t, db)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeSupervisor(t, db, 202, 1)
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Update("leader_user_id", 202).Error; err != nil {
		t.Fatal(err)
	}
	previous := config.CurrentOrDefault()
	cfg := previous
	cfg.TicketDispatch.UnassignedEscalationMinutes = 30
	config.SetCurrent(&cfg)
	t.Cleanup(func() { config.SetCurrent(&previous) })

	now := time.Now()
	ticket := models.Ticket{
		TicketNo: "REOPEN-FRESH-ESCALATION", Title: "旧工单重开重新计时", TenantID: 1, CurrentTeamID: 1,
		Status:      enums.TicketStatusClosed,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now.Add(-time.Hour)},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	manager := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "manager", Roles: []string{EnterpriseRoleServiceManager}}
	if err := TicketLifecycleService.Reopen(ticket.ID, "客户反馈问题复现", manager); err != nil {
		t.Fatalf("Reopen() error = %v", err)
	}

	dispatched, err := TicketDispatchService.DispatchPendingTickets(1)
	if err != nil || dispatched != 0 {
		t.Fatalf("DispatchPendingTickets() = (%d, %v), want fresh wait window", dispatched, err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.Status != enums.TicketStatusPendingDispatch || current.CurrentAssigneeID != 0 || current.AcceptedAt != nil || current.DispatchDeferredUntil == nil {
		t.Fatalf("reopened ticket was escalated before its fresh wait window elapsed: %+v", current)
	}
}

func TestTicketDispatchDoesNotEscalateToUnavailableSupervisor(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeSupervisor(t, db, 202, 1)
	if err := db.Model(&models.AgentWorkStatus{}).
		Where("tenant_id = ? AND user_id = ?", int64(1), int64(202)).
		Updates(map[string]any{"status": AgentWorkStatusLeave, "updated_at": time.Now()}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	ticket := models.Ticket{
		TicketNo: "ESCALATE-LEAVE", Title: "主管请假不兜底", TenantID: 1, CurrentTeamID: 1,
		Status:      enums.TicketStatusPendingDispatch,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour)},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}

	dispatched, err := TicketDispatchService.DispatchPendingTickets(1)
	if err != nil || dispatched != 0 {
		t.Fatalf("DispatchPendingTickets() = (%d, %v), want no unavailable supervisor takeover", dispatched, err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.CurrentAssigneeID != 0 || current.Status != enums.TicketStatusPendingDispatch || current.LastDispatchFailureReason != "missing_supervisor" {
		t.Fatalf("unavailable supervisor should not take over ticket: %+v", current)
	}
}

func TestTicketDispatchDoesNotEscalateToFutureAvailableSupervisor(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeSupervisor(t, db, 202, 1)
	now := time.Now()
	future := now.Add(2 * time.Hour)
	if err := db.Model(&models.AgentWorkStatus{}).
		Where("tenant_id = ? AND user_id = ?", int64(1), int64(202)).
		Updates(map[string]any{"status": AgentWorkStatusAvailable, "confirmed_at": now, "available_at": future, "updated_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	ticket := models.Ticket{
		TicketNo: "ESCALATE-FUTURE-AVAILABLE", Title: "主管未来恢复不兜底", TenantID: 1, CurrentTeamID: 1,
		Status:      enums.TicketStatusPendingDispatch,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour)},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}

	dispatched, err := TicketDispatchService.DispatchPendingTickets(1)
	if err != nil || dispatched != 0 {
		t.Fatalf("DispatchPendingTickets() = (%d, %v), want no future-available supervisor takeover", dispatched, err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.CurrentAssigneeID != 0 || current.Status != enums.TicketStatusPendingDispatch || current.LastDispatchFailureReason != "missing_supervisor" {
		t.Fatalf("future-available supervisor should not take over ticket: %+v", current)
	}
}

func TestTicketDispatchDoesNotEscalateToDisabledTeamLeader(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeSupervisor(t, db, 202, 1)
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Updates(map[string]any{
		"leader_user_id": int64(202),
		"status":         enums.StatusDisabled,
	}).Error; err != nil {
		t.Fatal(err)
	}
	previous := config.CurrentOrDefault()
	cfg := previous
	cfg.TicketDispatch.UnassignedEscalationMinutes = 30
	config.SetCurrent(&cfg)
	t.Cleanup(func() { config.SetCurrent(&previous) })
	now := time.Now()
	ticket := models.Ticket{
		TicketNo: "ESCALATE-DISABLED-TEAM", Title: "停用团队主管不兜底", TenantID: 1, CurrentTeamID: 1,
		Status:      enums.TicketStatusPendingDispatch,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour)},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}

	dispatched, err := TicketDispatchService.DispatchPendingTickets(1)
	if err != nil || dispatched != 0 {
		t.Fatalf("DispatchPendingTickets() = (%d, %v), want no disabled-team supervisor takeover", dispatched, err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.CurrentAssigneeID != 0 || current.Status != enums.TicketStatusPendingDispatch || current.LastDispatchFailureReason != "missing_supervisor" {
		t.Fatalf("disabled team supervisor should not take over ticket: %+v", current)
	}
	if AgentTeamMemberService.IsUserActiveMemberOfTeamDB(sqls.DB(), 1, 1, 202) {
		t.Fatal("disabled team membership should not be treated as active")
	}
}

func TestTicketDispatchFallsBackFromDisabledCurrentTeamToProductRepairTeam(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Update("status", enums.StatusDisabled).Error; err != nil {
		t.Fatal(err)
	}
	productTeam := models.AgentTeam{
		ID: 42, TenantID: 1, ProductID: 701, TeamType: AgentTeamTypeProductRepair,
		Name: "当前产品维修组", Status: enums.StatusOk,
	}
	if err := db.Create(&productTeam).Error; err != nil {
		t.Fatal(err)
	}
	createHumanDispatchRealtimeAgentProfile(t, db, 101, productTeam.ID)
	now := time.Now()
	ticket := models.Ticket{
		TicketNo: "DISABLED-TEAM-FALLBACK", Title: "残留停用团队自愈派单", TenantID: 1,
		ProductID: 701, CurrentTeamID: 1, Status: enums.TicketStatusPending,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}

	candidates, err := AgentProfileService.FindDispatchCandidates(DispatchCandidateQuery{TenantID: 1, TicketID: ticket.ID}, now)
	if err != nil {
		t.Fatalf("FindDispatchCandidates() error = %v", err)
	}
	if len(candidates) != 1 || candidates[0].TeamID != productTeam.ID || candidates[0].Profile.UserID != 101 {
		t.Fatalf("dispatch candidates should use active product repair team, got %+v", candidates)
	}
	dispatched, err := TicketDispatchService.DispatchPendingTickets(1)
	if err != nil || dispatched != 1 {
		t.Fatalf("DispatchPendingTickets() = (%d, %v), want fallback dispatch", dispatched, err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.CurrentTeamID != productTeam.ID || current.CurrentAssigneeID != 101 || current.Status != enums.TicketStatusPendingAssigneeAccept {
		t.Fatalf("ticket was not dispatched through active product repair team: %+v", current)
	}
}

func TestTicketDispatchIgnoresLegacyScheduleRowsInAudit(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)

	team := models.AgentTeam{
		ID: 43, TenantID: 1, ProductID: 702, TeamType: AgentTeamTypeProductRepair,
		Name: "成员可用性维修组", ScheduleEnforced: true, Status: enums.StatusOk,
	}
	if err := db.Create(&team).Error; err != nil {
		t.Fatal(err)
	}
	createHumanDispatchRealtimeAgentProfile(t, db, 101, team.ID)
	now := time.Now()
	ensureEnterpriseTemplateCoversTime(t, db, 1, now)
	if err := db.Create(&models.AgentTeamSchedule{
		TenantID: 1, TeamID: team.ID, UserID: 101, RepeatType: AgentTeamScheduleRepeatOnce,
		StartAt: now.Add(-2 * time.Hour), EndAt: now.Add(-time.Hour),
		PublishStatus: AgentTeamSchedulePublishPublished, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatal(err)
	}
	ticket := models.Ticket{
		TicketNo: "MEMBER-AVAILABILITY-AUDIT", Title: "成员可用性审计", TenantID: 1,
		ProductID: team.ProductID, CurrentTeamID: team.ID, Status: enums.TicketStatusPending,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-time.Minute), UpdatedAt: now.Add(-time.Minute)},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}

	dispatched, err := TicketDispatchService.DispatchPendingTickets(1)
	if err != nil || dispatched != 1 {
		t.Fatalf("DispatchPendingTickets() = (%d, %v), want member availability dispatch", dispatched, err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.Status != enums.TicketStatusPendingAssigneeAccept || current.CurrentAssigneeID != 101 {
		t.Fatalf("member availability dispatch did not assign the available engineer: %+v", current)
	}
	var attempt models.TicketDispatchAttempt
	if err := db.Where("ticket_id = ?", ticket.ID).First(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	if attempt.Reason != dispatchReasonAutomatic || attempt.Outcome != ticketDispatchOutcomePending {
		t.Fatalf("legacy schedule rows should not produce schedule fallback audit reason: %+v", attempt)
	}
}

func TestTicketDispatchSupervisorTakeoverUsesProductRepairTeamWhenCurrentTeamBelongsToAnotherProduct(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 301, 1)
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Updates(map[string]any{
		"product_id":     int64(999),
		"leader_user_id": int64(301),
	}).Error; err != nil {
		t.Fatal(err)
	}
	productTeam := models.AgentTeam{
		ID: 42, TenantID: 1, ProductID: 701, TeamType: AgentTeamTypeProductRepair,
		Name: "正确产品维修组", LeaderUserID: 202, Status: enums.StatusOk,
	}
	if err := db.Create(&productTeam).Error; err != nil {
		t.Fatal(err)
	}
	createHumanDispatchRealtimeSupervisor(t, db, 202, productTeam.ID)
	now := time.Now()
	ticket := models.Ticket{
		TicketNo: "STALE-CURRENT-TEAM-TAKEOVER", Title: "旧团队指向其他产品", TenantID: 1,
		ProductID: 701, CurrentTeamID: 1, Status: enums.TicketStatusPendingDispatch,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}

	escalated, err := TicketDispatchService.EscalateToSupervisor(ticket.ID, 0, "产品维修组兜底", now)
	if err != nil || escalated == nil {
		t.Fatalf("EscalateToSupervisor() = (%+v, %v), want product repair leader takeover", escalated, err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.Status != enums.TicketStatusProcessing || current.CurrentAssigneeID != 202 || current.CurrentTeamID != productTeam.ID || current.AcceptedAt == nil {
		t.Fatalf("supervisor takeover should use the product repair team, got %+v", current)
	}
	var attempts []models.TicketDispatchAttempt
	if err := db.Where("ticket_id = ?", ticket.ID).Order("id ASC").Find(&attempts).Error; err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 1 || attempts[0].Outcome != ticketDispatchOutcomeEscalated || attempts[0].TeamID != productTeam.ID || attempts[0].AssigneeID != 202 {
		t.Fatalf("takeover dispatch attempt should be audited against product team, got %+v", attempts)
	}
}

func TestTicketDispatchEscalatesProductTicketWithoutCurrentTeamToProductLeader(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	productTeam := models.AgentTeam{
		ID: 31, TenantID: 1, ProductID: 701, TeamType: AgentTeamTypeProductRepair,
		Name: "产品维修组", LeaderUserID: 202, Status: enums.StatusOk,
	}
	if err := db.Create(&productTeam).Error; err != nil {
		t.Fatal(err)
	}
	createHumanDispatchRealtimeSupervisor(t, db, 202, productTeam.ID)
	previous := config.CurrentOrDefault()
	cfg := previous
	cfg.TicketDispatch.UnassignedEscalationMinutes = 30
	config.SetCurrent(&cfg)
	t.Cleanup(func() { config.SetCurrent(&previous) })

	now := time.Now()
	ticket := models.Ticket{
		TicketNo: "PRODUCT-UNASSIGNED-OLD", Title: "只有产品未写组", TenantID: 1, ProductID: productTeam.ProductID,
		Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: now.Add(-31 * time.Minute), UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}

	dispatched, err := TicketDispatchService.DispatchPendingTickets(1)
	if err != nil || dispatched != 1 {
		t.Fatalf("DispatchPendingTickets() = (%d, %v), want product leader takeover", dispatched, err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.Status != enums.TicketStatusProcessing || current.CurrentAssigneeID != 202 || current.CurrentTeamID != productTeam.ID || current.AcceptedAt == nil {
		t.Fatalf("product ticket was not escalated to product repair leader: %+v", current)
	}
}

func TestTicketDispatchEscalatesWhenTimedOutEngineerHasNoAlternative(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	createHumanDispatchRealtimeSupervisor(t, db, 202, 1)
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Update("leader_user_id", 202).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	assignedAt := now.Add(-time.Hour)
	deadline := now.Add(-time.Minute)
	ticket := models.Ticket{
		TicketNo: "TIMEOUT-NO-ALTERNATIVE", Title: "无替代工程师", TenantID: 1, CurrentTeamID: 1, CurrentAssigneeID: 101,
		Status: enums.TicketStatusPendingAssigneeAccept, AssignedAt: &assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TicketDispatchAttempt{
		TenantID: 1, TicketID: ticket.ID, TeamID: 1, AssigneeID: 101, AttemptNo: 1,
		Outcome: ticketDispatchOutcomePending, AssignedAt: assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}).Error; err != nil {
		t.Fatal(err)
	}

	recovered, err := TicketDispatchService.RecoverExpiredTicketAcceptances(1)
	if err != nil || recovered != 1 {
		t.Fatalf("RecoverExpiredTicketAcceptances() = (%d, %v)", recovered, err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.Status != enums.TicketStatusProcessing || current.CurrentAssigneeID != 202 || current.AcceptedAt == nil {
		t.Fatalf("ticket without an alternative was not escalated: %+v", current)
	}
}

func TestTicketDispatchSupervisorTakeoverDoesNotReturnToLatestTimedOutAssignee(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Update("leader_user_id", 101).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	assignedAt := now.Add(-20 * time.Minute)
	ticket := models.Ticket{
		TicketNo: "TIMEOUT-SAME-SUPERVISOR", Title: "超时负责人也是组长", TenantID: 1,
		CurrentTeamID: 1, Status: enums.TicketStatusPendingDispatch, DispatchAttempts: 1,
		LastDispatchFailureReason: acceptTimeoutRedispatchingCode,
		AuditFields:               models.AuditFields{CreatedAt: assignedAt, UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	endedAt := now.Add(-time.Minute)
	if err := db.Create(&models.TicketDispatchAttempt{
		TenantID: 1, TicketID: ticket.ID, TeamID: 1, AssigneeID: 101, AttemptNo: 1,
		Outcome: ticketDispatchOutcomeTimedOut, AssignedAt: assignedAt, EndedAt: &endedAt,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: endedAt},
	}).Error; err != nil {
		t.Fatal(err)
	}

	escalated, err := TicketDispatchService.EscalateToSupervisor(ticket.ID, 0, "主管兜底", now)
	if err != nil || escalated != nil {
		t.Fatalf("EscalateToSupervisor() = (%+v, %v), want team queue fallback", escalated, err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.Status != enums.TicketStatusPendingDispatch || current.CurrentAssigneeID != 0 || current.AcceptedAt != nil ||
		current.DispatchDeferredUntil == nil || current.LastDispatchFailureReason != "no_alternative_after_accept_timeout" {
		t.Fatalf("timed-out supervisor must stay excluded from automatic takeover: %+v", current)
	}
	var returnedAttempts int64
	if err := db.Model(&models.TicketDispatchAttempt{}).
		Where("ticket_id = ? AND assignee_id = ? AND outcome = ?", ticket.ID, 101, ticketDispatchOutcomeEscalated).
		Count(&returnedAttempts).Error; err != nil || returnedAttempts != 0 {
		t.Fatalf("timed-out supervisor was assigned again: count=%d err=%v", returnedAttempts, err)
	}
}

func TestTicketDispatchTimeoutEscalationRewritesDisabledCurrentTeamToProductRepairTeam(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Update("status", enums.StatusDisabled).Error; err != nil {
		t.Fatal(err)
	}
	productTeam := models.AgentTeam{
		ID: 42, TenantID: 1, ProductID: 701, TeamType: AgentTeamTypeProductRepair,
		Name: "正确产品维修组", LeaderUserID: 202, Status: enums.StatusOk,
	}
	if err := db.Create(&productTeam).Error; err != nil {
		t.Fatal(err)
	}
	createHumanDispatchRealtimeSupervisor(t, db, 202, productTeam.ID)
	previous := config.CurrentOrDefault()
	cfg := previous
	cfg.TicketDispatch.MaxDispatchAttempts = 1
	config.SetCurrent(&cfg)
	t.Cleanup(func() { config.SetCurrent(&previous) })

	now := time.Now()
	assignedAt := now.Add(-time.Hour)
	deadline := now.Add(-time.Minute)
	ticket := models.Ticket{
		TicketNo: "TIMEOUT-DISABLED-TEAM-FALLBACK", Title: "超时升级旧团队自愈", TenantID: 1,
		ProductID: 701, CurrentTeamID: 1, CurrentAssigneeID: 101, Status: enums.TicketStatusPendingAssigneeAccept,
		AssignedAt: &assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TicketDispatchAttempt{
		TenantID: 1, TicketID: ticket.ID, TeamID: 1, AssigneeID: 101, AttemptNo: 1,
		Outcome: ticketDispatchOutcomePending, AssignedAt: assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}).Error; err != nil {
		t.Fatal(err)
	}

	recovered, err := TicketDispatchService.RecoverExpiredTicketAcceptances(1)
	if err != nil || recovered != 1 {
		t.Fatalf("RecoverExpiredTicketAcceptances() = (%d, %v), want product repair takeover", recovered, err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.Status != enums.TicketStatusProcessing || current.CurrentAssigneeID != 202 || current.CurrentTeamID != productTeam.ID || current.AcceptedAt == nil || current.AcceptDeadlineAt != nil {
		t.Fatalf("timeout escalation should rewrite disabled current team to product repair team, got %+v", current)
	}
	var attempts []models.TicketDispatchAttempt
	if err := db.Where("ticket_id = ?", ticket.ID).Order("id ASC").Find(&attempts).Error; err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 2 || attempts[0].Outcome != ticketDispatchOutcomeEscalated ||
		attempts[1].Outcome != ticketDispatchOutcomeEscalated || attempts[1].TeamID != productTeam.ID || attempts[1].AssigneeID != 202 {
		t.Fatalf("timeout escalation attempts should settle old attempt and audit product-team takeover, got %+v", attempts)
	}
}

func TestSLAAssignmentBreachEscalatesAtAttemptLimitBeforeAcceptDeadline(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	createHumanDispatchRealtimeSupervisor(t, db, 202, 1)
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Update("leader_user_id", 202).Error; err != nil {
		t.Fatal(err)
	}
	previous := config.CurrentOrDefault()
	cfg := previous
	cfg.TicketDispatch.MaxDispatchAttempts = 1
	config.SetCurrent(&cfg)
	t.Cleanup(func() { config.SetCurrent(&previous) })

	now := time.Now()
	assignedAt := now.Add(-2 * time.Minute)
	deadline := now.Add(20 * time.Minute)
	ticket := models.Ticket{
		TicketNo: "SLA-BREACH-BEFORE-DEADLINE", Title: "SLA 已违规但接单截止未到", TenantID: 1,
		CurrentTeamID: 1, CurrentAssigneeID: 101, Status: enums.TicketStatusPendingAssigneeAccept,
		AssignedAt: &assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TicketDispatchAttempt{
		TenantID: 1, TicketID: ticket.ID, TeamID: 1, AssigneeID: 101, AttemptNo: 1,
		Outcome: ticketDispatchOutcomePending, AssignedAt: assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}).Error; err != nil {
		t.Fatal(err)
	}

	handled, err := TicketDispatchService.RecoverTicketAssignmentSLA(ticket.ID, now)
	if err != nil || !handled {
		t.Fatalf("RecoverTicketAssignmentSLA() = (%v, %v), want supervisor takeover", handled, err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.Status != enums.TicketStatusProcessing || current.CurrentAssigneeID != 202 || current.AcceptedAt == nil || current.AcceptDeadlineAt != nil {
		t.Fatalf("assignment SLA breach did not force supervisor takeover: %+v", current)
	}
}

func TestSLAAssignmentRecoveryDoesNotOverrideAcceptedRace(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	createHumanDispatchRealtimeSupervisor(t, db, 202, 1)
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Update("leader_user_id", 202).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	assignedAt := now.Add(-10 * time.Minute)
	acceptedAt := now.Add(-time.Minute)
	ticket := models.Ticket{
		TicketNo: "SLA-RACE-ACCEPTED", Title: "接单与 SLA 回收竞态", TenantID: 1,
		CurrentTeamID: 1, CurrentAssigneeID: 101, Status: enums.TicketStatusProcessing,
		AssignedAt: &assignedAt, AcceptedAt: &acceptedAt,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: acceptedAt},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TicketDispatchAttempt{
		TenantID: 1, TicketID: ticket.ID, TeamID: 1, AssigneeID: 101, AttemptNo: 1,
		Outcome: ticketDispatchOutcomeAccepted, AssignedAt: assignedAt, AcceptedAt: &acceptedAt,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: acceptedAt},
	}).Error; err != nil {
		t.Fatal(err)
	}
	staleSnapshot := ticket
	staleSnapshot.AcceptedAt = nil

	recycled, err := TicketDispatchService.recycleTicketAfterAcceptTimeout(&staleSnapshot, 101, 1, now, true)
	if err != nil || recycled {
		t.Fatalf("recycleTicketAfterAcceptTimeout() = (%v, %v), want accepted race ignored", recycled, err)
	}
	escalated, err := TicketDispatchService.escalateTicketAfterRepeatedTimeout(&staleSnapshot, 101, 1, now, true)
	if err != nil || escalated {
		t.Fatalf("escalateTicketAfterRepeatedTimeout() = (%v, %v), want accepted race ignored", escalated, err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.Status != enums.TicketStatusProcessing || current.CurrentAssigneeID != 101 || current.AcceptedAt == nil || !current.AcceptedAt.Equal(acceptedAt) {
		t.Fatalf("accepted ticket was overwritten by SLA recovery race: %+v", current)
	}
}

func TestTicketDispatchDoesNotEscalateToLeaderOutsideRepairTeam(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeTeam(t, db, 2)
	createHumanDispatchRealtimeSupervisor(t, db, 202, 2)
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Update("leader_user_id", 202).Error; err != nil {
		t.Fatal(err)
	}
	previous := config.CurrentOrDefault()
	cfg := previous
	cfg.TicketDispatch.UnassignedEscalationMinutes = 30
	config.SetCurrent(&cfg)
	t.Cleanup(func() { config.SetCurrent(&previous) })

	now := time.Now()
	ticket := models.Ticket{
		TicketNo: "UNASSIGNED-NON-MEMBER-LEADER", Title: "组长不在维修组", TenantID: 1, CurrentTeamID: 1,
		Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: now.Add(-31 * time.Minute), UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}

	dispatched, err := TicketDispatchService.DispatchPendingTickets(1)
	if err != nil || dispatched != 0 {
		t.Fatalf("DispatchPendingTickets() = (%d, %v), want no unusable supervisor takeover", dispatched, err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.Status == enums.TicketStatusProcessing || current.Status == enums.TicketStatusEscalated || current.CurrentAssigneeID != 0 {
		t.Fatalf("ticket was escalated to a leader outside the repair team: %+v", current)
	}
}

func TestConversationDispatchEscalatesLinkedTicketWhenTeamPoolStalls(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	aiAgent := createHumanDispatchRealtimeAIAgent(t, db, "1")
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeSupervisor(t, db, 202, 1)
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Update("leader_user_id", 202).Error; err != nil {
		t.Fatal(err)
	}
	previous := config.CurrentOrDefault()
	cfg := previous
	cfg.TicketDispatch.UnassignedEscalationMinutes = 30
	config.SetCurrent(&cfg)
	t.Cleanup(func() { config.SetCurrent(&previous) })

	now := time.Now()
	conversation := createHumanDispatchRealtimeConversation(t, db, aiAgent.ID)
	if err := db.Model(&models.Conversation{}).Where("id = ?", conversation.ID).Updates(map[string]any{
		"status":              enums.IMConversationStatusPending,
		"current_team_id":     int64(1),
		"current_assignee_id": int64(0),
	}).Error; err != nil {
		t.Fatal(err)
	}
	ticket := models.Ticket{
		TicketNo: "CONV-POOL-STALE", Title: "会话团队池超时", TenantID: 1, ConversationID: conversation.ID, CurrentTeamID: 1,
		Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: now.Add(-31 * time.Minute), UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}

	dispatched, err := ConversationDispatchService.DispatchConversation(conversation.ID)
	if err != nil || dispatched == nil {
		t.Fatalf("DispatchConversation() = (%+v, %v), want supervisor takeover", dispatched, err)
	}
	currentTicket := repositoriesTicket(t, ticket.ID)
	if currentTicket.Status != enums.TicketStatusProcessing || currentTicket.CurrentAssigneeID != 202 || currentTicket.AcceptedAt == nil {
		t.Fatalf("linked ticket was not taken over: %+v", currentTicket)
	}
	currentConversation := ConversationService.Get(conversation.ID)
	if currentConversation == nil || currentConversation.Status != enums.IMConversationStatusActive || currentConversation.CurrentAssigneeID != 202 {
		t.Fatalf("conversation was not synchronized to supervisor takeover: %+v", currentConversation)
	}
}

func TestDispatchCandidateUsesCalendarAndReachability(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	var profile models.AgentProfile
	if err := db.Where("user_id = ?", 101).First(&profile).Error; err != nil {
		t.Fatal(err)
	}
	dispatchProfiles := []AgentTeamDispatchProfile{{Profile: profile, TeamID: 1, DispatchEnabled: true}}
	if enabled, _, reason := ConversationDispatchService.filterEnabledDispatchProfiles(1, dispatchProfiles, time.Now()); len(enabled) != 1 || reason != "" {
		t.Fatalf("recent available engineer rejected: enabled=%+v reason=%s", enabled, reason)
	}
	if err := db.Where("tenant_id = ? AND user_id = ?", 1, 101).Delete(&models.AgentWorkStatus{}).Error; err != nil {
		t.Fatal(err)
	}
	if enabled, _, reason := ConversationDispatchService.filterEnabledDispatchProfiles(1, dispatchProfiles, time.Now()); len(enabled) != 1 || reason != "" {
		t.Fatalf("work-status row must not override the configured work calendar: enabled=%+v reason=%s", enabled, reason)
	}
	stale := time.Now().Add(-10 * time.Minute)
	profile.LastOnlineAt = &stale
	now := time.Now()
	if err := db.Create(&models.AgentWorkStatus{TenantID: 1, UserID: 101, Status: AgentWorkStatusAvailable, ConfirmedAt: now, StatusChangedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if enabled, _, reason := ConversationDispatchService.filterEnabledDispatchProfiles(1, []AgentTeamDispatchProfile{{Profile: profile, TeamID: 1, DispatchEnabled: true}}, now); len(enabled) != 0 || reason != "no_reachable_user" {
		t.Fatalf("stale engineer remained dispatchable: enabled=%+v reason=%s", enabled, reason)
	}
	profile.ReceiveOfflineMessage = true
	if enabled, _, reason := ConversationDispatchService.filterEnabledDispatchProfiles(1, []AgentTeamDispatchProfile{{Profile: profile, TeamID: 1, DispatchEnabled: true}}, now); len(enabled) != 1 || reason != "" {
		t.Fatalf("offline-message engineer was not dispatchable: enabled=%+v reason=%s", enabled, reason)
	}
}

func TestTicketDispatchCapsAcceptDeadlineByAssignmentSLA(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	if err := db.AutoMigrate(&SLAPolicy{}, &SLAPauseRecord{}); err != nil {
		t.Fatal(err)
	}
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	now := time.Now()
	policy := SLAPolicy{
		ID: "dispatch-sla", TenantID: "1", Name: "dispatch sla", Priority: "p2",
		AssignmentMinutes: 10, Status: "active", CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&policy).Error; err != nil {
		t.Fatal(err)
	}
	createdAt := now.Add(-8 * time.Minute)
	ticket := models.Ticket{
		TicketNo: "SLA-CAPPED", Title: "SLA 压缩接单时限", TenantID: 1, CurrentTeamID: 1,
		Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: createdAt, UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}

	dispatched, err := TicketDispatchService.DispatchPendingTickets(1)
	if err != nil || dispatched != 1 {
		t.Fatalf("DispatchPendingTickets() = (%d, %v)", dispatched, err)
	}
	current := repositoriesTicket(t, ticket.ID)
	expectedDeadline := createdAt.Add(10 * time.Minute)
	if current.AcceptDeadlineAt == nil || !current.AcceptDeadlineAt.Equal(expectedDeadline) {
		t.Fatalf("accept deadline was not capped by assignment SLA: got=%v want=%v ticket=%+v", current.AcceptDeadlineAt, expectedDeadline, current)
	}
}

func TestSLAAssignmentViolationTriggersSupervisorTakeover(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	if err := db.AutoMigrate(&SLAPolicy{}, &SLAViolation{}, &SLAPauseRecord{}); err != nil {
		t.Fatal(err)
	}
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeSupervisor(t, db, 202, 1)
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Update("leader_user_id", 202).Error; err != nil {
		t.Fatal(err)
	}
	previous := config.CurrentOrDefault()
	cfg := previous
	cfg.TicketDispatch.UnassignedEscalationMinutes = 1
	config.SetCurrent(&cfg)
	t.Cleanup(func() { config.SetCurrent(&previous) })

	now := time.Now()
	policy := SLAPolicy{
		ID: "assignment-breach", TenantID: "1", Name: "assignment breach", Priority: "p2",
		AssignmentMinutes: 1, Status: "active", CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&policy).Error; err != nil {
		t.Fatal(err)
	}
	ticket := models.Ticket{
		TicketNo: "SLA-TAKEOVER", Title: "SLA 违规触发主管兜底", TenantID: 1, CurrentTeamID: 1,
		Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: now.Add(-2 * time.Minute), UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}

	violations, err := SLAService.CheckSLAViolations()
	if err != nil {
		t.Fatalf("CheckSLAViolations() error = %v", err)
	}
	if len(violations) != 1 || violations[0].ViolationType != "assignment" {
		t.Fatalf("unexpected violations: %+v", violations)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.Status != enums.TicketStatusProcessing || current.CurrentAssigneeID != 202 || current.AcceptedAt == nil || current.AcceptDeadlineAt != nil {
		t.Fatalf("assignment SLA did not trigger supervisor takeover: %+v", current)
	}
}

func TestManualTicketAssignmentRequiresAuthorityButIgnoresDisplayWorkStatus(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	now := time.Now()
	ticket := models.Ticket{
		TicketNo: "MANUAL-AUTH-AVAILABLE", Title: "人工派单校验", TenantID: 1, CurrentTeamID: 1,
		Status: enums.TicketStatusPendingDispatch, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}

	engineer := &dto.AuthPrincipal{TenantID: 1, UserID: 101, Username: "agent-101", Roles: []string{EnterpriseRoleEngineer}}
	if err := TicketLifecycleService.Assign(ticket.ID, 101, "普通工程师不能代派池子工单", engineer); err == nil {
		t.Fatal("unassigned pool ticket assignment by a plain engineer should be rejected")
	}

	manager := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "manager", Roles: []string{EnterpriseRoleServiceManager}}
	if err := db.Model(&models.AgentWorkStatus{}).
		Where("tenant_id = ? AND user_id = ?", 1, 101).
		Updates(map[string]any{"status": AgentWorkStatusLeave}).Error; err != nil {
		t.Fatal(err)
	}
	if err := TicketLifecycleService.Assign(ticket.ID, 101, "经理人工指定当前显示请假状态的工程师", manager); err != nil {
		t.Fatalf("manual assignment should ignore display-only work status: %v", err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.CurrentAssigneeID != 101 || current.Status != enums.TicketStatusPendingAssigneeAccept || current.AcceptDeadlineAt == nil || current.LastDispatchFailureReason != "" {
		t.Fatalf("manual assignment did not enter pending acceptance cleanly: %+v", current)
	}
}

func TestManualTicketAssignmentOverridesAutomaticEligibilityButRequiresCapability(t *testing.T) {
	manager := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "manager", Roles: []string{EnterpriseRoleServiceManager}}
	cases := []struct {
		name    string
		product bool
		reject  bool
		mutate  func(t *testing.T, db *gorm.DB, ticket *models.Ticket)
	}{
		{
			name: "global dispatch disabled",
			mutate: func(t *testing.T, db *gorm.DB, ticket *models.Ticket) {
				t.Helper()
				if err := db.Model(&models.AgentProfile{}).Where("tenant_id = ? AND user_id = ?", 1, 101).
					Update("auto_assign_enabled", false).Error; err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "busy service status",
			mutate: func(t *testing.T, db *gorm.DB, ticket *models.Ticket) {
				t.Helper()
				if err := db.Model(&models.AgentProfile{}).Where("tenant_id = ? AND user_id = ?", 1, 101).
					Update("service_status", enums.ServiceStatusBusy).Error; err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "full capacity",
			mutate: func(t *testing.T, db *gorm.DB, ticket *models.Ticket) {
				t.Helper()
				if err := db.Model(&models.AgentProfile{}).Where("tenant_id = ? AND user_id = ?", 1, 101).
					Update("max_concurrent_count", 1).Error; err != nil {
					t.Fatal(err)
				}
				now := time.Now()
				if err := db.Create(&models.Ticket{
					TicketNo: "MANUAL-CAPACITY-EXISTING", Title: "已有在办工单", TenantID: 1,
					CurrentTeamID: 1, CurrentAssigneeID: 101, Status: enums.TicketStatusProcessing,
					AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
				}).Error; err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "stale offline without offline dispatch",
			mutate: func(t *testing.T, db *gorm.DB, ticket *models.Ticket) {
				t.Helper()
				stale := time.Now().Add(-48 * time.Hour)
				if err := db.Model(&models.AgentProfile{}).Where("tenant_id = ? AND user_id = ?", 1, 101).
					Updates(map[string]any{"last_online_at": stale, "receive_offline_message": false}).Error; err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:    "capability mismatch",
			product: true,
			reject:  true,
			mutate: func(t *testing.T, db *gorm.DB, ticket *models.Ticket) {
				t.Helper()
				if err := db.AutoMigrate(&models.TenantMember{}, &models.EngineerProfile{}); err != nil {
					t.Fatal(err)
				}
				createTicketDispatchEngineerCapability(t, db, 101, `["hydraulic"]`, `["emea"]`, true)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := setupHumanDispatchRealtimeTestDB(t)
			createHumanDispatchRealtimeTeam(t, db, 1)
			createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
			productID := int64(0)
			faultCode := ""
			serviceRegion := ""
			if tc.product {
				productID = 10
				faultCode = "power"
				serviceRegion = "apac"
				now := time.Now()
				if err := db.Create(&models.Product{
					ID: productID, TenantID: 1, Code: "POWER", Name: "电源系统", Status: enums.StatusOk,
					AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
				}).Error; err != nil {
					t.Fatal(err)
				}
				if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Updates(map[string]any{
					"product_id": productID,
					"team_type":  AgentTeamTypeProductRepair,
				}).Error; err != nil {
					t.Fatal(err)
				}
			}
			now := time.Now()
			ticket := models.Ticket{
				TicketNo: "MANUAL-STRICT-" + strings.ToUpper(strings.ReplaceAll(tc.name, " ", "-")),
				Title:    "人工派单不能绕过候选资格", TenantID: 1, ProductID: productID, CurrentTeamID: 1,
				FaultCode: faultCode, ServiceRegion: serviceRegion,
				Status: enums.TicketStatusPendingDispatch, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
			}
			if err := db.Create(&ticket).Error; err != nil {
				t.Fatal(err)
			}
			tc.mutate(t, db, &ticket)

			err := TicketLifecycleService.Assign(ticket.ID, 101, tc.name, manager)
			current := repositoriesTicket(t, ticket.ID)
			if tc.reject {
				if err == nil {
					t.Fatalf("manual assignment should reject %s", tc.name)
				}
				if current.CurrentAssigneeID != 0 || current.Status != enums.TicketStatusPendingDispatch || current.AcceptDeadlineAt != nil {
					t.Fatalf("rejected manual assignment mutated ticket: %+v", current)
				}
				return
			}
			if err != nil {
				t.Fatalf("manual assignment should override %s: %v", tc.name, err)
			}
			if current.CurrentAssigneeID != 101 || current.Status != enums.TicketStatusPendingAssigneeAccept || current.AcceptDeadlineAt == nil {
				t.Fatalf("manual assignment did not enter pending acceptance: %+v", current)
			}
		})
	}
}

func TestTicketAcceptAllowsDispatchManagerToOverrideAssignedTicket(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 202, 1)
	now := time.Now()
	assignedAt := now.Add(-time.Minute)
	deadline := now.Add(9 * time.Minute)
	ticket := models.Ticket{
		TicketNo: "ACCEPT-NO-STEAL", Title: "经理不能用接单接管别人派单", TenantID: 1,
		CurrentTeamID: 1, CurrentAssigneeID: 101, Status: enums.TicketStatusPendingAssigneeAccept,
		AssignedAt: &assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-time.Minute), UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	manager := &dto.AuthPrincipal{TenantID: 1, UserID: 202, Username: "agent-202", Roles: []string{EnterpriseRoleServiceManager}}

	if err := TicketLifecycleService.Accept(ticket.ID, 0, manager); err != nil {
		t.Fatalf("dispatch manager should be able to take over an assigned ticket: %v", err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.CurrentAssigneeID != 202 || current.Status != enums.TicketStatusProcessing || current.AcceptedAt == nil || current.AcceptDeadlineAt != nil {
		t.Fatalf("manager takeover did not settle the assigned ticket: %+v", current)
	}
}

func TestTicketAcceptAllowsExpiredProductTeamPeerTakeover(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	if err := db.AutoMigrate(&models.AuditLog{}); err != nil {
		t.Fatalf("migrate takeover audit table: %v", err)
	}
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 202, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 303, 1)
	now := time.Now()
	assignedAt := now.Add(-10 * time.Minute)
	deadline := now.Add(-time.Minute)
	conversation := models.Conversation{
		TenantID: 1, CustomerID: 1, CustomerName: "真实客户",
		Status: enums.IMConversationStatusPending, ServiceMode: enums.IMConversationServiceModeAIFirst,
		CurrentTeamID: 1, CurrentAssigneeID: 101, LastMessageAt: now, LastActiveAt: now,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	ticket := models.Ticket{
		TicketNo: "ACCEPT-EXPIRED-TAKEOVER", Title: "超时派单允许产品组接管", TenantID: 1,
		ConversationID: conversation.ID, CurrentTeamID: 1, CurrentAssigneeID: 101,
		Status: enums.TicketStatusPendingAssigneeAccept, AssignedAt: &assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TicketDispatchAttempt{
		TenantID: 1, TicketID: ticket.ID, TeamID: 1, AssigneeID: 101, AttemptNo: 1,
		Outcome: ticketDispatchOutcomePending, AssignedAt: assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}).Error; err != nil {
		t.Fatal(err)
	}
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 202, Username: "agent-202", Roles: []string{EnterpriseRoleEngineer}}
	originalAssignee := &dto.AuthPrincipal{TenantID: 1, UserID: 101, Username: "agent-101", Roles: []string{EnterpriseRoleEngineer}}

	if err := TicketLifecycleService.Accept(ticket.ID, 0, originalAssignee); err == nil {
		t.Fatal("original assignee must not accept after the deadline")
	}
	if err := TicketLifecycleService.AcceptWithAudit(ticket.ID, 0, operator, RecordAuditInput{RequestID: "takeover-request"}); err != nil {
		t.Fatalf("expired product-team assignment should allow peer takeover: %v", err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.Status != enums.TicketStatusProcessing || current.CurrentAssigneeID != 202 || current.AcceptedAt == nil || current.AcceptDeadlineAt != nil || current.AssignedAt == nil || !current.AssignedAt.After(assignedAt) || current.DispatchAttempts != 1 {
		t.Fatalf("takeover did not settle ticket state atomically: %+v", current)
	}
	var attempts []models.TicketDispatchAttempt
	if err := db.Where("ticket_id = ?", ticket.ID).Order("id ASC").Find(&attempts).Error; err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 2 || attempts[0].AssigneeID != 101 || attempts[0].Outcome != ticketDispatchOutcomeTimedOut || attempts[0].EndedAt == nil || attempts[1].AssigneeID != 202 || attempts[1].Outcome != ticketDispatchOutcomeAccepted || attempts[1].AcceptedAt == nil || attempts[1].EndedAt == nil {
		t.Fatalf("takeover dispatch audit is incomplete: %+v", attempts)
	}
	var activeConversation models.Conversation
	if err := db.First(&activeConversation, conversation.ID).Error; err != nil {
		t.Fatal(err)
	}
	if activeConversation.Status != enums.IMConversationStatusActive || activeConversation.CurrentTeamID != 1 || activeConversation.CurrentAssigneeID != 202 {
		t.Fatalf("takeover did not move the linked conversation: %+v", activeConversation)
	}
	var progress models.TicketProgress
	if err := db.Where("ticket_id = ? AND event_type = ?", ticket.ID, enums.TicketProgressEventAccepted).First(&progress).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(progress.Content, "接管") {
		t.Fatalf("takeover progress is not auditable: %+v", progress)
	}
	var audit models.AuditLog
	if err := db.Where("tenant_id = ? AND resource_type = ? AND resource_id = ?", 1, "ticket", strconv.FormatInt(ticket.ID, 10)).First(&audit).Error; err != nil {
		t.Fatal(err)
	}
	if audit.Action != "ticket.taken_over" || !strings.Contains(audit.BeforeState, `"assigneeId":101`) || !strings.Contains(audit.AfterState, `"assigneeId":202`) {
		t.Fatalf("transactional takeover audit is incomplete: %+v", audit)
	}
	secondPeer := &dto.AuthPrincipal{TenantID: 1, UserID: 303, Username: "agent-303", Roles: []string{EnterpriseRoleEngineer}}
	if err := TicketLifecycleService.Accept(ticket.ID, 0, secondPeer); err != nil {
		t.Fatalf("a product-team peer should be able to perform an explicit reassignment: %v", err)
	}
	if current = repositoriesTicket(t, ticket.ID); current.CurrentAssigneeID != 303 || current.Status != enums.TicketStatusProcessing {
		t.Fatalf("explicit team reassignment did not change the owner: %+v", current)
	}
}

func TestTicketAcceptAllowsLegacyAssignedPeerTakeoverAfterDerivedDeadline(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	if err := db.AutoMigrate(&models.AuditLog{}); err != nil {
		t.Fatalf("migrate takeover audit table: %v", err)
	}
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 202, 1)
	stale := time.Now().Add(-2 * time.Hour)
	if err := db.Model(&models.AgentProfile{}).Where("tenant_id = ? AND user_id = ?", 1, 202).
		Update("last_online_at", stale.Add(-time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Product{
		ID: 802, TenantID: 1, Code: "LEGACY-ASSIGNED", Name: "遗留派单产品", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: stale, UpdatedAt: stale},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Updates(map[string]any{
		"product_id":        802,
		"team_type":         AgentTeamTypeProductRepair,
		"schedule_enforced": true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	ticket := models.Ticket{
		TicketNo: "TAKEOVER-LEGACY-ASSIGNED", Title: "遗留派单超时允许产品组接管", TenantID: 1,
		ProductID: 802, CurrentTeamID: 1, CurrentAssigneeID: 101, Status: enums.TicketStatusAssigned,
		AuditFields: models.AuditFields{CreatedAt: stale, UpdatedAt: stale},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	originalAssignee := &dto.AuthPrincipal{TenantID: 1, UserID: 101, Username: "agent-101", Roles: []string{EnterpriseRoleEngineer}}
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 202, Username: "agent-202", Roles: []string{EnterpriseRoleEngineer}}

	if err := TicketLifecycleService.Accept(ticket.ID, 0, originalAssignee); err == nil {
		t.Fatal("legacy assignee must not accept after the derived deadline")
	}
	if err := TicketLifecycleService.AcceptWithAudit(ticket.ID, 0, operator, RecordAuditInput{RequestID: "legacy-assigned-takeover"}); err != nil {
		t.Fatalf("stale legacy assignment should allow product-team peer takeover: %v", err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.Status != enums.TicketStatusProcessing || current.CurrentAssigneeID != 202 || current.AcceptedAt == nil || current.AcceptDeadlineAt != nil || current.DispatchAttempts != 1 {
		t.Fatalf("legacy assignment takeover did not settle ticket state atomically: %+v", current)
	}
	var attempt models.TicketDispatchAttempt
	if err := db.Where("ticket_id = ?", ticket.ID).First(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	if attempt.AssigneeID != 202 || attempt.Outcome != ticketDispatchOutcomeAccepted || !strings.Contains(attempt.Reason, "接单超时") {
		t.Fatalf("legacy assignment takeover dispatch audit is incomplete: %+v", attempt)
	}
}

func TestTicketAcceptAllowsOverdueCustomerReplyProductTeamPeerTakeover(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	if err := db.AutoMigrate(&models.AuditLog{}); err != nil {
		t.Fatalf("migrate takeover audit table: %v", err)
	}
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 202, 1)
	now := time.Now()
	acceptedAt := now.Add(-45 * time.Minute)
	if err := db.Create(&models.Product{
		ID: 801, TenantID: 1, Code: "OVERDUE-REPLY", Name: "超时回复产品", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: acceptedAt, UpdatedAt: acceptedAt},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Updates(map[string]any{
		"product_id": 801,
		"team_type":  AgentTeamTypeProductRepair,
	}).Error; err != nil {
		t.Fatal(err)
	}
	conversation := models.Conversation{
		TenantID: 1, ProductID: 801, CustomerID: 1, CustomerName: "真实客户",
		Status: enums.IMConversationStatusActive, ServiceMode: enums.IMConversationServiceModeAIFirst,
		CurrentTeamID: 1, CurrentAssigneeID: 101, LastMessageAt: acceptedAt, LastActiveAt: acceptedAt,
		AuditFields: models.AuditFields{CreatedAt: acceptedAt, UpdatedAt: acceptedAt},
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Message{
		ConversationID: conversation.ID, ClientMsgID: "customer-waiting-for-owner", SenderType: enums.IMSenderTypeCustomer,
		MessageType: enums.IMMessageTypeText, Content: "设备仍然停机，请尽快回复", SendStatus: enums.IMMessageStatusSent,
		SentAt: &acceptedAt, AuditFields: models.AuditFields{CreatedAt: acceptedAt, UpdatedAt: acceptedAt},
	}).Error; err != nil {
		t.Fatal(err)
	}
	ticket := models.Ticket{
		TicketNo: "TAKEOVER-OVERDUE-CUSTOMER-REPLY", Title: "长期未回复允许产品组接管", TenantID: 1,
		ProductID: 801, ConversationID: conversation.ID, CurrentTeamID: 1, CurrentAssigneeID: 101,
		Status: enums.TicketStatusProcessing, AssignedAt: &acceptedAt, AcceptedAt: &acceptedAt, DispatchAttempts: 1,
		AuditFields: models.AuditFields{CreatedAt: acceptedAt, UpdatedAt: acceptedAt},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	endedAt := acceptedAt
	if err := db.Create(&models.TicketDispatchAttempt{
		TenantID: 1, TicketID: ticket.ID, TeamID: 1, AssigneeID: 101, AttemptNo: 1,
		Outcome: ticketDispatchOutcomeAccepted, AssignedAt: acceptedAt, AcceptedAt: &acceptedAt, EndedAt: &endedAt,
		AuditFields: models.AuditFields{CreatedAt: acceptedAt, UpdatedAt: acceptedAt},
	}).Error; err != nil {
		t.Fatal(err)
	}
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 202, Username: "agent-202", Roles: []string{EnterpriseRoleEngineer}}

	if err := TicketLifecycleService.AcceptWithAudit(ticket.ID, 0, operator, RecordAuditInput{RequestID: "overdue-reply-takeover"}); err != nil {
		t.Fatalf("overdue unanswered customer message should allow product-team peer takeover: %v", err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.Status != enums.TicketStatusProcessing || current.CurrentAssigneeID != 202 || current.AcceptedAt == nil || !current.AcceptedAt.After(acceptedAt) || current.DispatchAttempts != 1 {
		t.Fatalf("overdue reply takeover did not settle ticket state atomically: %+v", current)
	}
	var attempts []models.TicketDispatchAttempt
	if err := db.Where("ticket_id = ?", ticket.ID).Order("id ASC").Find(&attempts).Error; err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 2 || attempts[0].Outcome != ticketDispatchOutcomeAccepted || attempts[1].AssigneeID != 202 || attempts[1].Outcome != ticketDispatchOutcomeAccepted || !strings.Contains(attempts[1].Reason, "长期未回复") {
		t.Fatalf("overdue reply takeover dispatch audit is incomplete: %+v", attempts)
	}
	var assignment models.ConversationAssignment
	if err := db.Where("conversation_id = ? AND to_user_id = ?", conversation.ID, 202).First(&assignment).Error; err != nil {
		t.Fatal(err)
	}
	if assignment.FromUserID != 101 || assignment.AssignType != string(enums.IMAssignmentTypeTransfer) {
		t.Fatalf("overdue reply takeover did not record a conversation transfer: %+v", assignment)
	}
	var progress models.TicketProgress
	if err := db.Where("ticket_id = ? AND event_type = ?", ticket.ID, enums.TicketProgressEventAccepted).First(&progress).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(progress.Content, "长期未回复") {
		t.Fatalf("overdue reply takeover progress is not auditable: %+v", progress)
	}
	var audit models.AuditLog
	if err := db.Where("tenant_id = ? AND resource_type = ? AND resource_id = ?", 1, "ticket", strconv.FormatInt(ticket.ID, 10)).First(&audit).Error; err != nil {
		t.Fatal(err)
	}
	if audit.Action != "ticket.taken_over" || !strings.Contains(audit.AfterState, `"takeoverKind":"unanswered_customer"`) {
		t.Fatalf("overdue reply takeover audit is incomplete: %+v", audit)
	}
	secondPeer := &dto.AuthPrincipal{TenantID: 1, UserID: 101, Username: "agent-101", Roles: []string{EnterpriseRoleEngineer}}
	if err := TicketLifecycleService.Accept(ticket.ID, 0, secondPeer); err != nil {
		t.Fatalf("an explicit same-team reassignment should remain available after the owner changes: %v", err)
	}
	if current = repositoriesTicket(t, ticket.ID); current.CurrentAssigneeID != 101 || current.Status != enums.TicketStatusProcessing {
		t.Fatalf("explicit same-team reassignment did not restore the owner: %+v", current)
	}
}

func TestTicketTakeoverRollsBackWhenTransactionalAuditFails(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 202, 1)
	now := time.Now()
	assignedAt := now.Add(-10 * time.Minute)
	deadline := now.Add(-time.Minute)
	ticket := models.Ticket{
		TicketNo: "TAKEOVER-AUDIT-ROLLBACK", Title: "审计失败必须回滚接管", TenantID: 1,
		CurrentTeamID: 1, CurrentAssigneeID: 101, Status: enums.TicketStatusPendingAssigneeAccept,
		AssignedAt: &assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TicketDispatchAttempt{
		TenantID: 1, TicketID: ticket.ID, TeamID: 1, AssigneeID: 101, AttemptNo: 1,
		Outcome: ticketDispatchOutcomePending, AssignedAt: assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}).Error; err != nil {
		t.Fatal(err)
	}
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 202, Username: "agent-202", Roles: []string{EnterpriseRoleEngineer}}

	if err := TicketLifecycleService.AcceptWithAudit(ticket.ID, 0, operator, RecordAuditInput{RequestID: "missing-audit-table"}); err == nil {
		t.Fatal("takeover should fail when its transactional audit cannot be persisted")
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.CurrentAssigneeID != 101 || current.Status != enums.TicketStatusPendingAssigneeAccept || current.AcceptedAt != nil || current.AcceptDeadlineAt == nil {
		t.Fatalf("audit failure did not roll back ticket takeover: %+v", current)
	}
	var attempt models.TicketDispatchAttempt
	if err := db.Where("ticket_id = ?", ticket.ID).First(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	if attempt.Outcome != ticketDispatchOutcomePending || attempt.EndedAt != nil {
		t.Fatalf("audit failure did not roll back dispatch attempt: %+v", attempt)
	}
}

func TestManualTicketAssignmentRejectsEngineerOutsideProductRepairTeam(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	productTeam := models.AgentTeam{
		ID: 31, TenantID: 1, ProductID: 701, TeamType: AgentTeamTypeProductRepair,
		Name: "产品 A 维修组", Status: enums.StatusOk,
	}
	otherTeam := models.AgentTeam{
		ID: 32, TenantID: 1, ProductID: 702, TeamType: AgentTeamTypeProductRepair,
		Name: "产品 B 维修组", Status: enums.StatusOk,
	}
	if err := db.Create(&productTeam).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&otherTeam).Error; err != nil {
		t.Fatal(err)
	}
	createHumanDispatchRealtimeAgentProfile(t, db, 101, otherTeam.ID)
	now := time.Now()
	ticket := models.Ticket{
		TicketNo: "MANUAL-CROSS-PRODUCT", Title: "不能跨产品组派单", TenantID: 1, ProductID: productTeam.ProductID,
		Status: enums.TicketStatusPendingDispatch, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	manager := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "manager", Roles: []string{EnterpriseRoleServiceManager}}

	if err := TicketLifecycleService.Assign(ticket.ID, 101, "跨产品组派单", manager); err == nil {
		t.Fatal("manual assignment to an engineer outside the ticket product repair team should be rejected")
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.CurrentAssigneeID != 0 || current.CurrentTeamID != 0 || current.AcceptDeadlineAt != nil {
		t.Fatalf("rejected cross-product assignment changed the ticket: %+v", current)
	}
}

func TestManualTicketAssignmentAllowsLeaveEngineer(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	productTeam := models.AgentTeam{
		ID: 41, TenantID: 1, ProductID: 801, TeamType: AgentTeamTypeProductRepair,
		Name: "个人可用性维修组", ScheduleEnforced: true, Status: enums.StatusOk,
	}
	if err := db.Create(&productTeam).Error; err != nil {
		t.Fatal(err)
	}
	createHumanDispatchRealtimeAgentProfile(t, db, 101, productTeam.ID)
	createHumanDispatchRealtimeAgentProfile(t, db, 102, productTeam.ID)
	now := time.Now()
	ensureEnterpriseTemplateCoversTime(t, db, 1, now)
	if err := db.Create(&models.AgentScheduleException{
		TenantID:       1,
		UserID:         101,
		RequestKey:     "manual-assignment-leave-101",
		ExceptionType:  AgentScheduleExceptionTypeLeave,
		StartAt:        now.Add(-time.Hour),
		EndAt:          now.Add(time.Hour),
		ApprovalStatus: AgentScheduleApprovalApproved,
		RequestedAt:    now.Add(-2 * time.Hour),
		Status:         enums.StatusOk,
		AuditFields:    models.AuditFields{CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour)},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.AgentTeamSchedule{
		TenantID: 1, TeamID: productTeam.ID, UserID: 102, RepeatType: AgentTeamScheduleRepeatOnce,
		StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour),
		PublishStatus: AgentTeamSchedulePublishPublished, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatal(err)
	}
	ticket := models.Ticket{
		TicketNo: "MANUAL-LEAVE", Title: "手动派给请假成员", TenantID: 1, ProductID: productTeam.ProductID,
		Status: enums.TicketStatusPendingDispatch, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	manager := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "manager", Roles: []string{EnterpriseRoleServiceManager}}

	if err := TicketLifecycleService.Assign(ticket.ID, 101, "人工指定请假成员", manager); err != nil {
		t.Fatalf("manual assignment to a leave engineer should be allowed: %v", err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.CurrentAssigneeID != 101 || current.CurrentTeamID != productTeam.ID || current.Status != enums.TicketStatusPendingAssigneeAccept {
		t.Fatalf("manual assignment to leave engineer did not succeed cleanly: %+v", current)
	}
}

func TestTicketAcceptAllowsLeaveWorkStatus(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	now := time.Now()
	assignedAt := now.Add(-5 * time.Minute)
	deadline := now.Add(10 * time.Minute)
	ticket := models.Ticket{
		TicketNo: "ACCEPT-LEAVE", Title: "离岗人工接单", TenantID: 1, CurrentTeamID: 1, CurrentAssigneeID: 101,
		Status: enums.TicketStatusPendingAssigneeAccept, AssignedAt: &assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-time.Hour), UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.AgentWorkStatus{}).
		Where("tenant_id = ? AND user_id = ?", 1, 101).
		Updates(map[string]any{"status": AgentWorkStatusLeave, "status_changed_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	engineer := &dto.AuthPrincipal{TenantID: 1, UserID: 101, Username: "agent-101", Roles: []string{EnterpriseRoleEngineer}}

	if err := TicketLifecycleService.Accept(ticket.ID, 101, engineer); err != nil {
		t.Fatalf("leave engineer should be able to accept a manually assigned ticket: %v", err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.Status != enums.TicketStatusProcessing || current.AcceptedAt == nil || current.AcceptDeadlineAt != nil {
		t.Fatalf("accepted ticket was not settled cleanly: %+v", current)
	}
}

func TestTicketAcceptUnassignedProductTicketAllowsExplicitOffShiftClaim(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	productTeam := models.AgentTeam{
		ID: 51, TenantID: 1, ProductID: 901, TeamType: AgentTeamTypeProductRepair,
		Name: "自领产品维修组", ScheduleEnforced: true, Status: enums.StatusOk,
	}
	if err := db.Create(&productTeam).Error; err != nil {
		t.Fatal(err)
	}
	createHumanDispatchRealtimeAgentProfile(t, db, 101, productTeam.ID)
	createHumanDispatchRealtimeAgentProfile(t, db, 102, productTeam.ID)
	now := time.Now()
	minute := scheduleMinute(now)
	if err := db.Create(&models.AgentTeamSchedule{
		TenantID: 1, TeamID: productTeam.ID, UserID: 102, RepeatType: AgentTeamScheduleRepeatOnce,
		DayType: AgentTeamScheduleDayTypeWork, Weekday: scheduleWeekday(now),
		StartMinute: max(0, minute-1), EndMinute: min(24*60, minute+30),
		Timezone: EngineerScheduleTimezone,
		StartAt:  now.Add(-time.Hour), EndAt: now.Add(time.Hour),
		PublishStatus: AgentTeamSchedulePublishPublished, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatal(err)
	}
	ticket := models.Ticket{
		TicketNo: "ACCEPT-OFF-SHIFT", Title: "未分配产品票不能被非值班人抢单", TenantID: 1, ProductID: productTeam.ProductID,
		Status: enums.TicketStatusPendingAcceptance, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}

	offShift := &dto.AuthPrincipal{TenantID: 1, UserID: 101, Username: "agent-101", Roles: []string{EnterpriseRoleEngineer}}
	if err := TicketLifecycleService.Accept(ticket.ID, 101, offShift); err != nil {
		t.Fatalf("explicit self-claim should remain available outside the automatic roster: %v", err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.Status != enums.TicketStatusAccepted || current.CurrentAssigneeID != 101 || current.CurrentTeamID != productTeam.ID || current.AcceptedAt == nil {
		t.Fatalf("explicit off-shift claim did not preserve product queue: %+v", current)
	}
}

func TestTicketAcceptPendingDispatchSelfClaimActivatesConversation(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	productTeam := models.AgentTeam{
		ID: 52, TenantID: 1, ProductID: 902, TeamType: AgentTeamTypeProductRepair,
		Name: "待派单池产品维修组", Status: enums.StatusOk,
	}
	if err := db.Create(&productTeam).Error; err != nil {
		t.Fatal(err)
	}
	createHumanDispatchRealtimeAgentProfile(t, db, 101, productTeam.ID)
	now := time.Now()
	conversation := models.Conversation{
		TenantID: 1, ProductID: productTeam.ProductID, CustomerID: 1, CustomerName: "真实客户",
		Status: enums.IMConversationStatusPending, ServiceMode: enums.IMConversationServiceModeAIFirst,
		LastMessageAt: now, LastActiveAt: now,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	ticket := models.Ticket{
		TicketNo: "ACCEPT-POOL", Title: "待派单池接单", TenantID: 1, ProductID: productTeam.ProductID,
		ConversationID: conversation.ID, Status: enums.TicketStatusPendingDispatch,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	engineer := &dto.AuthPrincipal{TenantID: 1, UserID: 101, Username: "agent-101", Roles: []string{EnterpriseRoleEngineer}}

	if err := TicketLifecycleService.Accept(ticket.ID, 0, engineer); err != nil {
		t.Fatalf("pending dispatch self-claim should accept: %v", err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.Status != enums.TicketStatusProcessing || current.CurrentAssigneeID != 101 || current.CurrentTeamID != productTeam.ID || current.AcceptedAt == nil || current.AssignedAt == nil {
		t.Fatalf("pending dispatch self-claim did not settle ticket cleanly: %+v", current)
	}
	var activeConversation models.Conversation
	if err := db.First(&activeConversation, conversation.ID).Error; err != nil {
		t.Fatal(err)
	}
	if activeConversation.Status != enums.IMConversationStatusActive || activeConversation.CurrentAssigneeID != 101 || activeConversation.CurrentTeamID != productTeam.ID {
		t.Fatalf("self-claim did not activate the linked conversation: %+v", activeConversation)
	}
}

func TestTicketAcceptPendingDispatchManualClaimAfterScheduleFailure(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	if err := db.AutoMigrate(&models.TenantMember{}, &models.EngineerProfile{}); err != nil {
		t.Fatalf("migrate engineer capability tables: %v", err)
	}
	productTeam := models.AgentTeam{
		ID: 53, TenantID: 1, ProductID: 904, TeamType: AgentTeamTypeProductRepair,
		Name: "人工兜底产品维修组", ScheduleEnforced: true, Status: enums.StatusOk,
	}
	if err := db.Create(&productTeam).Error; err != nil {
		t.Fatal(err)
	}
	createHumanDispatchRealtimeAgentProfile(t, db, 101, productTeam.ID)
	createTicketDispatchEngineerCapability(t, db, 101, `[]`, `[]`, true)
	now := time.Now()
	ticket := models.Ticket{
		TicketNo: "ACCEPT-NO-SCHEDULE-POOL", Title: "排班失败待派池人工接管", TenantID: 1,
		ProductID: productTeam.ProductID, CurrentTeamID: productTeam.ID,
		FaultCode: "RHD-FLOW-ALPHA-7742", ServiceRegion: "CN-SH",
		Status: enums.TicketStatusPendingDispatch, LastDispatchFailureReason: "no_active_schedule_team",
		AuditFields: models.AuditFields{CreatedAt: now.Add(-time.Hour), UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	engineer := &dto.AuthPrincipal{TenantID: 1, UserID: 101, Username: "agent-101", Roles: []string{EnterpriseRoleEngineer}}

	if err := TicketLifecycleService.Accept(ticket.ID, 0, engineer); err != nil {
		t.Fatalf("schedule-failed pool ticket should allow authorized manual claim: %v", err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.Status != enums.TicketStatusProcessing || current.CurrentAssigneeID != 101 || current.CurrentTeamID != productTeam.ID || current.AcceptedAt == nil || current.AssignedAt == nil {
		t.Fatalf("manual schedule-fallback claim did not settle ticket cleanly: %+v", current)
	}
	if current.LastDispatchFailureReason != "" || current.DispatchDeferredUntil != nil || current.AcceptDeadlineAt != nil {
		t.Fatalf("manual schedule-fallback claim kept stale dispatch failure fields: %+v", current)
	}
}

func TestTicketAcceptAllowsExplicitHolidayIntervention(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	if err := db.AutoMigrate(&models.TenantMember{}, &models.EngineerProfile{}); err != nil {
		t.Fatalf("migrate engineer capability tables: %v", err)
	}
	productTeam := models.AgentTeam{
		ID: 56, TenantID: 1, ProductID: 907, TeamType: AgentTeamTypeProductRepair,
		Name: "节假日产品维修组", ScheduleEnforced: true, Status: enums.StatusOk,
	}
	if err := db.Create(&productTeam).Error; err != nil {
		t.Fatal(err)
	}
	createHumanDispatchRealtimeAgentProfile(t, db, 101, productTeam.ID)
	createTicketDispatchEngineerCapability(t, db, 101, `[]`, `[]`, true)
	now := time.Now().UTC()
	if err := db.Create(&models.AgentTeamHoliday{
		TenantID: 1, TeamID: productTeam.ID,
		HolidayDate: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC),
		Timezone:    "UTC", Name: "service holiday", Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatal(err)
	}
	ticket := models.Ticket{
		TicketNo: "ACCEPT-HOLIDAY-BLOCKED", Title: "节假日禁止普通人工接管", TenantID: 1,
		ProductID: productTeam.ProductID, CurrentTeamID: productTeam.ID,
		Status: enums.TicketStatusPendingDispatch, LastDispatchFailureReason: "holiday_schedule_restricted",
		AuditFields: models.AuditFields{CreatedAt: now.Add(-time.Hour), UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	engineer := &dto.AuthPrincipal{TenantID: 1, UserID: 101, Username: "agent-101", Roles: []string{EnterpriseRoleEngineer}}

	if err := TicketLifecycleService.Accept(ticket.ID, 0, engineer); err != nil {
		t.Fatalf("explicit engineer claim should be available during a holiday: %v", err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.Status != enums.TicketStatusProcessing || current.CurrentAssigneeID != 101 || current.AcceptedAt == nil || current.LastDispatchFailureReason != "" {
		t.Fatalf("holiday intervention did not settle the ticket: %+v", current)
	}
}

func TestTicketAcceptPendingAcceptanceManualClaimAfterScheduleFailure(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	productTeam := models.AgentTeam{
		ID: 54, TenantID: 1, ProductID: 905, TeamType: AgentTeamTypeProductRepair,
		Name: "待接单人工兜底产品维修组", ScheduleEnforced: true, Status: enums.StatusOk,
	}
	if err := db.Create(&productTeam).Error; err != nil {
		t.Fatal(err)
	}
	createHumanDispatchRealtimeAgentProfile(t, db, 101, productTeam.ID)
	now := time.Now()
	ticket := models.Ticket{
		TicketNo: "ACCEPT-NO-SCHEDULE-PENDING", Title: "排班失败待接单人工接管", TenantID: 1,
		ProductID: productTeam.ProductID, CurrentTeamID: productTeam.ID,
		Status: enums.TicketStatusPendingAcceptance, LastDispatchFailureReason: "no_active_schedule_team",
		AuditFields: models.AuditFields{CreatedAt: now.Add(-time.Hour), UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	engineer := &dto.AuthPrincipal{TenantID: 1, UserID: 101, Username: "agent-101", Roles: []string{EnterpriseRoleEngineer}}

	if err := TicketLifecycleService.Accept(ticket.ID, 0, engineer); err != nil {
		t.Fatalf("authorized engineer should be able to claim pending acceptance after schedule failure: %v", err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.Status != enums.TicketStatusAccepted || current.CurrentAssigneeID != 101 || current.CurrentTeamID != productTeam.ID || current.AcceptedAt == nil || current.AssignedAt == nil {
		t.Fatalf("pending acceptance manual claim did not settle ticket cleanly: %+v", current)
	}
	if current.LastDispatchFailureReason != "" || current.DispatchDeferredUntil != nil || current.AcceptDeadlineAt != nil {
		t.Fatalf("pending acceptance manual claim kept stale dispatch failure fields: %+v", current)
	}
}

func TestTicketAcceptAllowsExplicitClaimAfterRosterRecovery(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	productTeam := models.AgentTeam{
		ID: 55, TenantID: 1, ProductID: 906, TeamType: AgentTeamTypeProductRepair,
		Name: "已恢复班次产品维修组", ScheduleEnforced: true, Status: enums.StatusOk,
	}
	if err := db.Create(&productTeam).Error; err != nil {
		t.Fatal(err)
	}
	createHumanDispatchRealtimeAgentProfile(t, db, 101, productTeam.ID)
	createHumanDispatchRealtimeAgentProfile(t, db, 102, productTeam.ID)
	now := time.Now()
	minute := scheduleMinute(now)
	if err := db.Create(&models.AgentTeamSchedule{
		TenantID: 1, TeamID: productTeam.ID, UserID: 102, RepeatType: AgentTeamScheduleRepeatOnce,
		DayType: AgentTeamScheduleDayTypeWork, Weekday: scheduleWeekday(now),
		StartMinute: max(0, minute-1), EndMinute: min(24*60, minute+30),
		Timezone: EngineerScheduleTimezone,
		StartAt:  now.Add(-time.Hour), EndAt: now.Add(time.Hour),
		PublishStatus: AgentTeamSchedulePublishPublished, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatal(err)
	}
	ticket := models.Ticket{
		TicketNo: "ACCEPT-STALE-NO-SCHEDULE", Title: "班次恢复后不能绕过值班名单", TenantID: 1,
		ProductID: productTeam.ProductID, CurrentTeamID: productTeam.ID,
		Status: enums.TicketStatusPendingAcceptance, LastDispatchFailureReason: "no_active_schedule_team",
		AuditFields: models.AuditFields{CreatedAt: now.Add(-time.Hour), UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	offShift := &dto.AuthPrincipal{TenantID: 1, UserID: 101, Username: "agent-101", Roles: []string{EnterpriseRoleEngineer}}

	if err := TicketLifecycleService.Accept(ticket.ID, 0, offShift); err != nil {
		t.Fatalf("explicit self-claim should remain available after roster recovery: %v", err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.Status != enums.TicketStatusAccepted || current.CurrentAssigneeID != 101 || current.AcceptedAt == nil || current.LastDispatchFailureReason != "" {
		t.Fatalf("explicit claim after roster recovery did not settle the ticket: %+v", current)
	}
}

func TestTicketLifecycleCancelClearsDispatchTrackingAndClosesAttempt(t *testing.T) {
	previousWS := WsService
	WsService = newWsService()
	t.Cleanup(func() {
		WsService = previousWS
	})
	db := setupHumanDispatchRealtimeTestDB(t)
	migrateTicketLifecycleSideEffectTables(t, db)
	createHumanDispatchRealtimeTeam(t, db, 1)
	now := time.Now()
	assignedAt := now.Add(-20 * time.Minute)
	deadline := now.Add(-time.Minute)
	deferredUntil := now.Add(10 * time.Minute)
	ticket := models.Ticket{
		TicketNo: "CANCEL-DISPATCH-CLEAN", Title: "取消派单中工单", TenantID: 1, CurrentTeamID: 1, CurrentAssigneeID: 101,
		Status: enums.TicketStatusPendingAssigneeAccept, AssignedAt: &assignedAt, AcceptDeadlineAt: &deadline,
		DispatchDeferredUntil: &deferredUntil, LastDispatchFailureReason: "no_reachable_user",
		AuditFields: models.AuditFields{CreatedAt: now.Add(-time.Hour), UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TicketDispatchAttempt{
		TenantID: 1, TicketID: ticket.ID, TeamID: 1, AssigneeID: 101,
		Outcome: ticketDispatchOutcomePending, AssignedAt: assignedAt, AcceptDeadlineAt: &deadline,
	}).Error; err != nil {
		t.Fatal(err)
	}
	session := &ClientSession{
		ID:     "ticket-cancel-notification-refresh",
		Role:   realtimeRoleNotification,
		Topics: map[string]struct{}{},
		Send:   make(chan []byte, 8),
	}
	WsService.manager.Register(session, []string{WsService.notificationTopic(101)})
	assignedNotification := &models.Notification{
		TenantID:         1,
		RecipientUserID:  101,
		Title:            "工单 CANCEL-DISPATCH-CLEAN 已分派",
		Content:          "请尽快接单处理",
		NotificationType: "ticket_assigned",
		BizType:          "ticket",
		BizID:            ticket.ID,
		Level:            "info",
		Category:         "ticket",
		Channels:         "in_app",
		DeliveryStatus:   "sent",
		Status:           int(enums.StatusOk),
		CreatedAt:        time.Now(),
	}
	if err := db.Create(assignedNotification).Error; err != nil {
		t.Fatalf("create assigned notification: %v", err)
	}
	manager := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "manager", Roles: []string{EnterpriseRoleServiceManager}}

	if err := TicketLifecycleService.Cancel(ticket.ID, "客户撤销请求", manager); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.Status != enums.TicketStatusCancelled || current.AcceptDeadlineAt != nil || current.DispatchDeferredUntil != nil || current.LastDispatchFailureReason != "" {
		t.Fatalf("cancel did not clear dispatch tracking: %+v", current)
	}
	var attempt models.TicketDispatchAttempt
	if err := db.Where("ticket_id = ?", ticket.ID).First(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	if attempt.Outcome != ticketDispatchOutcomeCancelled || attempt.EndedAt == nil {
		t.Fatalf("pending dispatch attempt was not closed on cancel: %+v", attempt)
	}
	reconciled := &models.Notification{}
	if err := db.First(reconciled, assignedNotification.ID).Error; err != nil {
		t.Fatalf("reload assigned notification: %v", err)
	}
	if reconciled.ReadAt == nil || !strings.Contains(reconciled.Title, "已取消") || !strings.Contains(reconciled.Content, "无需继续处理") {
		t.Fatalf("assigned notification was not reconciled after cancellation: %+v", reconciled)
	}
	if unread := NotificationService.CountUnreadForTenant(1, 101); unread != 0 {
		t.Fatalf("cancelled assignee unread notification count = %d, want 0", unread)
	}
	select {
	case raw := <-session.Send:
		var event struct {
			Type string `json:"type"`
			Data struct {
				Reason string `json:"reason"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &event); err != nil {
			t.Fatalf("decode notification resync event: %v", err)
		}
		if event.Type != enums.IMRealtimeEventResyncRequired || event.Data.Reason != enums.IMRealtimeResyncReasonNotificationUpdated {
			t.Fatalf("unexpected notification resync event: %s", raw)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled assignee did not receive notification resync")
	}
}

func TestTicketLifecycleReopenClearsStaleDispatchFailure(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	migrateTicketLifecycleSideEffectTables(t, db)
	createHumanDispatchRealtimeTeam(t, db, 1)
	now := time.Now()
	deadline := now.Add(-time.Hour)
	deferredUntil := now.Add(10 * time.Minute)
	ticket := models.Ticket{
		TicketNo: "REOPEN-DISPATCH-CLEAN", Title: "重开清理派单状态", TenantID: 1, CurrentTeamID: 1, CurrentAssigneeID: 101,
		Status: enums.TicketStatusClosed, AcceptDeadlineAt: &deadline,
		DispatchDeferredUntil: &deferredUntil, LastDispatchFailureReason: "no_profile_for_enabled_user",
		AuditFields: models.AuditFields{CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TicketDispatchAttempt{
		TenantID: 1, TicketID: ticket.ID, TeamID: 1, AssigneeID: 101,
		Outcome: ticketDispatchOutcomePending, AssignedAt: now.Add(-time.Hour), AcceptDeadlineAt: &deadline,
	}).Error; err != nil {
		t.Fatal(err)
	}
	manager := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "manager", Roles: []string{EnterpriseRoleServiceManager}}

	if err := TicketLifecycleService.Reopen(ticket.ID, "客户反馈问题复现", manager); err != nil {
		t.Fatalf("Reopen() error = %v", err)
	}
	current := repositoriesTicket(t, ticket.ID)
	if current.Status != enums.TicketStatusReopened || current.AcceptDeadlineAt != nil || current.DispatchDeferredUntil != nil || current.LastDispatchFailureReason != "" {
		t.Fatalf("reopen did not clear stale dispatch failure: %+v", current)
	}
	var attempt models.TicketDispatchAttempt
	if err := db.Where("ticket_id = ?", ticket.ID).First(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	if attempt.Outcome != ticketDispatchOutcomeSuperseded || attempt.EndedAt == nil {
		t.Fatalf("stale pending attempt was not superseded on reopen: %+v", attempt)
	}
}

func TestTicketLifecycleReopenUsesPersonalAvailabilityDispatch(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	migrateTicketLifecycleSideEffectTables(t, db)
	aiAgent := createHumanDispatchRealtimeAIAgent(t, db, "1")
	createHumanDispatchRealtimeTeam(t, db, 1)
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Updates(map[string]any{
		"tenant_id": 1, "product_id": 906, "team_type": AgentTeamTypeProductRepair,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.AIAgent{}).Where("id = ?", aiAgent.ID).Update("product_id", 906).Error; err != nil {
		t.Fatal(err)
	}
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	now := time.Now()
	conversation := models.Conversation{
		TenantID: 1, ProductID: 906, AIAgentID: aiAgent.ID, Status: enums.IMConversationStatusClosed,
		CurrentTeamID: 1, CurrentAssigneeID: 101, ServiceMode: enums.IMConversationServiceModeAIFirst,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-5 * time.Minute), UpdatedAt: now},
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	ticket := models.Ticket{
		TicketNo: "REOPEN-NO-SCHEDULE-POOL", Title: "无班次重开继续处理", TenantID: 1,
		ProductID: 906, ConversationID: conversation.ID, CurrentTeamID: 1, CurrentAssigneeID: 101,
		Status:      enums.TicketStatusClosed,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-5 * time.Minute), UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	manager := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "manager", Roles: []string{EnterpriseRoleServiceManager}}
	if err := TicketLifecycleService.Reopen(ticket.ID, "客户反馈问题复现", manager); err != nil {
		t.Fatalf("Reopen() error = %v", err)
	}

	reopened := repositoriesTicket(t, ticket.ID)
	if reopened.Status != enums.TicketStatusPendingAssigneeAccept || reopened.CurrentAssigneeID != 101 || reopened.LastDispatchFailureReason != "" || reopened.AcceptDeadlineAt == nil {
		t.Fatalf("reopened ticket did not dispatch through personal availability: %+v", reopened)
	}
	reopenedConversation := repositories.ConversationRepository.Get(db, conversation.ID)
	if reopenedConversation == nil || reopenedConversation.Status != enums.IMConversationStatusPending || reopenedConversation.CurrentAssigneeID != 101 {
		t.Fatalf("reopened conversation is inconsistent with its ticket: %+v", reopenedConversation)
	}
	engineer := &dto.AuthPrincipal{TenantID: 1, UserID: 101, Username: "agent-101", Roles: []string{EnterpriseRoleEngineer}}
	if err := TicketLifecycleService.Accept(ticket.ID, 0, engineer); err != nil {
		t.Fatalf("engineer should be able to accept the reopened ticket: %v", err)
	}
	accepted := repositoriesTicket(t, ticket.ID)
	if accepted.AcceptedAt == nil || accepted.CurrentAssigneeID != engineer.UserID || accepted.LastDispatchFailureReason != "" {
		t.Fatalf("reopened ticket accept did not settle cleanly: %+v", accepted)
	}
	activeConversation := repositories.ConversationRepository.Get(db, conversation.ID)
	if activeConversation == nil || activeConversation.Status != enums.IMConversationStatusActive || activeConversation.CurrentAssigneeID != engineer.UserID {
		t.Fatalf("accepted reopened ticket did not reactivate the conversation: %+v", activeConversation)
	}
}

func TestTicketTransitionToProcessingFinishesPendingDispatchAttempt(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	now := time.Now()
	assignedAt := now.Add(-5 * time.Minute)
	deadline := now.Add(time.Minute)
	ticket := models.Ticket{
		TicketNo: "TRANSITION-ACCEPT-CLEAN", Title: "旧状态接口接单", TenantID: 1, CurrentTeamID: 1, CurrentAssigneeID: 101,
		Status: enums.TicketStatusPendingAssigneeAccept, AssignedAt: &assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-time.Hour), UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TicketDispatchAttempt{
		TenantID: 1, TicketID: ticket.ID, TeamID: 1, AssigneeID: 101,
		Outcome: ticketDispatchOutcomePending, AssignedAt: assignedAt, AcceptDeadlineAt: &deadline,
	}).Error; err != nil {
		t.Fatal(err)
	}
	manager := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "manager", Roles: []string{EnterpriseRoleServiceManager}}

	updated, err := TicketService.Transition(request.TransitionTicketRequest{
		TicketID: ticket.ID,
		Status:   string(enums.TicketStatusProcessing),
		Remark:   "旧状态接口推进",
	}, manager)
	if err != nil {
		t.Fatalf("Transition() error = %v", err)
	}
	if updated.Status != enums.TicketStatusProcessing || updated.AcceptedAt == nil || updated.AcceptDeadlineAt != nil {
		t.Fatalf("transition did not settle ticket dispatch state: %+v", updated)
	}
	var attempt models.TicketDispatchAttempt
	if err := db.Where("ticket_id = ?", ticket.ID).First(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	if attempt.Outcome != ticketDispatchOutcomeAccepted || attempt.AcceptedAt == nil || attempt.EndedAt == nil {
		t.Fatalf("pending dispatch attempt was not accepted by transition: %+v", attempt)
	}
}

func TestTicketAcceptIsIdempotentForAlreadyAcceptedAssignee(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	now := time.Now()
	assignedAt := now.Add(-5 * time.Minute)
	deadline := now.Add(time.Minute)
	ticket := models.Ticket{
		TicketNo: "ACCEPT-IDEMPOTENT", Title: "重复接单幂等", TenantID: 1, CurrentTeamID: 1, CurrentAssigneeID: 101,
		Status: enums.TicketStatusPendingAssigneeAccept, AssignedAt: &assignedAt, AcceptDeadlineAt: &deadline,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-time.Hour), UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TicketDispatchAttempt{
		TenantID: 1, TicketID: ticket.ID, TeamID: 1, AssigneeID: 101,
		Outcome: ticketDispatchOutcomePending, AssignedAt: assignedAt, AcceptDeadlineAt: &deadline,
	}).Error; err != nil {
		t.Fatal(err)
	}
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 101, Username: "agent-101", Roles: []string{EnterpriseRoleEngineer}, Status: enums.StatusOk}

	if err := TicketLifecycleService.Accept(ticket.ID, 101, operator); err != nil {
		t.Fatalf("first Accept() error = %v", err)
	}
	if err := TicketLifecycleService.Accept(ticket.ID, 101, operator); err != nil {
		t.Fatalf("second Accept() should be idempotent, error = %v", err)
	}

	current := repositoriesTicket(t, ticket.ID)
	if current.Status != enums.TicketStatusProcessing || current.AcceptedAt == nil || current.AcceptDeadlineAt != nil {
		t.Fatalf("ticket did not remain accepted cleanly: %+v", current)
	}
	var attempts []models.TicketDispatchAttempt
	if err := db.Where("ticket_id = ?", ticket.ID).Order("id ASC").Find(&attempts).Error; err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 1 || attempts[0].Outcome != ticketDispatchOutcomeAccepted || attempts[0].AcceptedAt == nil || attempts[0].EndedAt == nil {
		t.Fatalf("repeated accept should leave one accepted attempt: %+v", attempts)
	}
	var progressCount int64
	if err := db.Model(&models.TicketProgress{}).
		Where("ticket_id = ? AND event_type = ?", ticket.ID, enums.TicketProgressEventAccepted).
		Count(&progressCount).Error; err != nil {
		t.Fatal(err)
	}
	if progressCount != 1 {
		t.Fatalf("accepted progress count = %d, want 1", progressCount)
	}
}

func repositoriesTicket(t *testing.T, ticketID int64) *models.Ticket {
	t.Helper()
	var ticket models.Ticket
	if err := sqls.DB().First(&ticket, ticketID).Error; err != nil {
		t.Fatal(err)
	}
	return &ticket
}

func migrateTicketLifecycleSideEffectTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(
		&models.MeetingRoomJitsi{},
		&models.MeetingParticipant{},
		&models.TicketSupplierCollaboration{},
		&models.TicketSupplierCollaborationParticipant{},
		&models.PartnerAuthorizationScope{},
		&models.KnowledgeCandidate{},
		&models.TicketQualityClue{},
	); err != nil {
		t.Fatalf("migrate lifecycle side effect tables: %v", err)
	}
}

func createTicketDispatchEngineerCapability(t *testing.T, db *gorm.DB, userID int64, skillTagsJSON, regionsJSON string, enabled bool) {
	t.Helper()
	now := time.Now()
	member := models.TenantMember{
		TenantID: 1, UserID: userID, MemberNo: "M" + strconv.FormatInt(userID, 10),
		DisplayName: "工程师", MemberType: "employee", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&member).Error; err != nil {
		t.Fatalf("create tenant member: %v", err)
	}
	engineer := models.EngineerProfile{
		TenantID: 1, MemberID: member.ID, SkillTagsJSON: skillTagsJSON, ServiceRegionsJSON: regionsJSON,
		LanguagesJSON: "[]", Timezone: "UTC", MaxTicketLoad: 3, DispatchEnabled: enabled, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&engineer).Error; err != nil {
		t.Fatalf("create engineer profile: %v", err)
	}
}

func createHumanDispatchRealtimeSupervisor(t *testing.T, db *gorm.DB, userID, teamID int64) {
	t.Helper()
	now := time.Now()
	if err := db.Create(&models.User{
		ID:       userID,
		Username: "leader",
		Nickname: "组长",
		Status:   enums.StatusOk,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.AgentProfile{
		TenantID:              1,
		UserID:                userID,
		TeamID:                teamID,
		AgentCode:             "LEADER",
		DisplayName:           "组长",
		ServiceStatus:         enums.ServiceStatusIdle,
		MaxConcurrentCount:    0,
		AutoAssignEnabled:     false,
		ReceiveOfflineMessage: true,
		LastOnlineAt:          &now,
		Status:                enums.StatusOk,
		AuditFields:           models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.AgentTeamMember{
		TenantID:        1,
		TeamID:          teamID,
		UserID:          userID,
		DispatchEnabled: false,
		DispatchWeight:  1,
		Status:          enums.StatusOk,
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.AgentWorkStatus{
		TenantID:        1,
		UserID:          userID,
		Status:          AgentWorkStatusAvailable,
		ConfirmedAt:     now,
		StatusChangedAt: now,
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatal(err)
	}
}
