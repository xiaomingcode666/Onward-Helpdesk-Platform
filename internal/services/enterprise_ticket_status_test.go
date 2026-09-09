package services

import (
	"fmt"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
)

func TestEnterpriseStatusFilterCoversAfterSalesLifecycleGroups(t *testing.T) {
	tests := []struct {
		filter string
		want   []enums.TicketStatus
	}{
		{"pending_acceptance", []enums.TicketStatus{enums.TicketStatusPendingAcceptance, enums.TicketStatusPending, enums.TicketStatusReopened}},
		{"pending_dispatch", []enums.TicketStatus{enums.TicketStatusAccepted, enums.TicketStatusPendingDispatch, enums.TicketStatusAssigned, enums.TicketStatusPendingAssigneeAccept}},
		{"in_progress", []enums.TicketStatus{enums.TicketStatusInProgress, enums.TicketStatusProcessing, enums.TicketStatusVideoSupport, enums.TicketStatusSupplierSupport, enums.TicketStatusEscalated, enums.TicketStatusWaitingCustomer, enums.TicketStatusResolved, enums.TicketStatusPendingCustomerConfirm}},
		{"done", []enums.TicketStatus{enums.TicketStatusClosed, enums.TicketStatusDone, enums.TicketStatusCancelled}},
	}

	for _, tt := range tests {
		t.Run(tt.filter, func(t *testing.T) {
			got := enterpriseStatusFilterToDB(tt.filter)
			if len(got) != len(tt.want) {
				t.Fatalf("statuses = %v, want %v", got, tt.want)
			}
			for index := range tt.want {
				if got[index] != tt.want[index] {
					t.Fatalf("statuses = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestEnterpriseTicketActionsAllowAssignedEngineerToAccept(t *testing.T) {
	ticket := &models.Ticket{Status: enums.TicketStatusAssigned, CurrentAssigneeID: 25}
	actions := EnterpriseTicketService.BuildActions(ticket)
	if !actions.CanAccept {
		t.Fatal("assigned ticket must expose the engineer accept action")
	}
}

func TestEnterpriseTicketActionsAllowPendingDispatchPoolEngineerToAccept(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	if err := db.AutoMigrate(&models.Product{}, &models.AgentTeamMember{}); err != nil {
		t.Fatalf("auto migrate enterprise ticket action fixture: %v", err)
	}
	teamID := createEnterpriseTicketActionRepairTeamFixture(t, 1, 704, 1804, false)
	if err := db.Model(&models.AgentProfile{}).Where("tenant_id = ? AND user_id = ?", 1, 1804).
		Update("last_online_at", time.Now().Add(-24*time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	ticket := &models.Ticket{
		TenantID: 1, ProductID: 704, CurrentTeamID: teamID,
		Status: enums.TicketStatusPendingDispatch,
	}
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 1804, Roles: []string{EnterpriseRoleEngineer}}

	if actions := EnterpriseTicketService.BuildActionsForOperator(ticket, operator); !actions.CanAccept {
		t.Fatalf("product repair engineer should be able to accept a pending dispatch pool ticket: %+v", actions)
	}
}

func TestEnterpriseTicketActionsAllowManualClaimAfterScheduleFailure(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	if err := db.AutoMigrate(&models.Product{}, &models.AgentTeamMember{}); err != nil {
		t.Fatalf("auto migrate enterprise ticket action fixture: %v", err)
	}
	teamID := createEnterpriseTicketActionRepairTeamFixture(t, 1, 705, 1805, true)
	ticket := &models.Ticket{
		TenantID: 1, ProductID: 705, CurrentTeamID: teamID,
		Status: enums.TicketStatusPendingDispatch, LastDispatchFailureReason: "no_active_schedule_team",
	}
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 1805, Roles: []string{EnterpriseRoleEngineer}}

	if actions := EnterpriseTicketService.BuildActionsForOperator(ticket, operator); !actions.CanAccept {
		t.Fatalf("authorized engineer should be able to manually claim a schedule-failed pool ticket: %+v", actions)
	}
}

func TestEnterpriseTicketActionsAlignSupplierEscalationStatuses(t *testing.T) {
	allowed := []enums.TicketStatus{
		enums.TicketStatusAccepted,
		enums.TicketStatusProcessing,
		enums.TicketStatusVideoSupport,
		enums.TicketStatusSupplierSupport,
	}
	for _, status := range allowed {
		if actions := EnterpriseTicketService.BuildActions(&models.Ticket{Status: status}); !actions.CanEscalateSupplier {
			t.Fatalf("status %s must allow supplier escalation", status)
		}
	}

	blocked := []enums.TicketStatus{
		enums.TicketStatusPendingAcceptance,
		enums.TicketStatusPendingAssigneeAccept,
		enums.TicketStatusResolved,
		enums.TicketStatusClosed,
		enums.TicketStatusCancelled,
	}
	for _, status := range blocked {
		if actions := EnterpriseTicketService.BuildActions(&models.Ticket{Status: status}); actions.CanEscalateSupplier {
			t.Fatalf("status %s must not offer a new supplier escalation", status)
		}
	}
}

func TestEnterpriseTicketSLARequiresExplicitDeadline(t *testing.T) {
	now := time.Now().Add(-96 * time.Hour)
	legacyTicket := models.Ticket{
		TicketNo: "NO-SLA-DEADLINE", Status: enums.TicketStatusProcessing,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if enterpriseTicketSLAAtRisk(legacyTicket) || enterpriseTicketSLABreached(legacyTicket) {
		t.Fatal("ticket without sla_due_at must not be reported as SLA risk or breach")
	}
	listItem := EnterpriseTicketService.BuildListItem(legacyTicket)
	if listItem.SLADeadline != "" || listItem.SLABreached {
		t.Fatalf("list item exposed synthetic SLA deadline: %+v", listItem)
	}
	header := EnterpriseTicketService.buildHeader(&legacyTicket)
	if header.SLADeadline != "" {
		t.Fatalf("ticket header exposed synthetic SLA deadline: %+v", header)
	}

	dueAt := time.Now().Add(-time.Minute)
	slaTicket := legacyTicket
	slaTicket.SLADueAt = &dueAt
	if !enterpriseTicketSLAAtRisk(slaTicket) || !enterpriseTicketSLABreached(slaTicket) {
		t.Fatal("ticket with expired sla_due_at must be reported as SLA risk and breach")
	}
}

func TestEnterpriseTicketActionsOnlyAllowRepairAfterAcceptanceAndSupplierResolution(t *testing.T) {
	allowed := []enums.TicketStatus{
		enums.TicketStatusAccepted,
		enums.TicketStatusProcessing,
		enums.TicketStatusVideoSupport,
	}
	for _, status := range allowed {
		if actions := EnterpriseTicketService.BuildActions(&models.Ticket{Status: status}); !actions.CanSaveRepair {
			t.Fatalf("status %s must allow a repair conclusion", status)
		}
	}

	blocked := []enums.TicketStatus{
		enums.TicketStatusPendingAcceptance,
		enums.TicketStatusPendingAssigneeAccept,
		enums.TicketStatusReopened,
		enums.TicketStatusResolved,
		enums.TicketStatusClosed,
	}
	for _, status := range blocked {
		if actions := EnterpriseTicketService.BuildActions(&models.Ticket{Status: status}); actions.CanSaveRepair {
			t.Fatalf("status %s must not allow a repair conclusion", status)
		}
	}
}

func TestEnterpriseTicketActionsAllowCancellationOnlyBeforeCustomerConfirmation(t *testing.T) {
	for _, status := range []enums.TicketStatus{
		enums.TicketStatusPendingAcceptance,
		enums.TicketStatusAssigned,
		enums.TicketStatusProcessing,
		enums.TicketStatusSupplierSupport,
	} {
		if actions := EnterpriseTicketService.BuildActions(&models.Ticket{Status: status}); !actions.CanCancel {
			t.Fatalf("status %s must allow cancellation", status)
		}
	}

	for _, status := range []enums.TicketStatus{
		enums.TicketStatusResolved,
		enums.TicketStatusPendingCustomerConfirm,
		enums.TicketStatusClosed,
		enums.TicketStatusCancelled,
	} {
		if actions := EnterpriseTicketService.BuildActions(&models.Ticket{Status: status}); actions.CanCancel {
			t.Fatalf("status %s must not allow cancellation", status)
		}
	}
}

func TestEnterpriseTicketActionsRespectEngineerOwnership(t *testing.T) {
	ticket := &models.Ticket{
		TenantID:          1,
		Status:            enums.TicketStatusProcessing,
		CurrentAssigneeID: 25,
	}
	owner := &dto.AuthPrincipal{TenantID: 1, UserID: 25, Roles: []string{EnterpriseRoleEngineer}}
	peer := &dto.AuthPrincipal{TenantID: 1, UserID: 26, Roles: []string{EnterpriseRoleEngineer}}
	manager := &dto.AuthPrincipal{TenantID: 1, UserID: 27, Roles: []string{EnterpriseRoleServiceManager}}

	if actions := EnterpriseTicketService.BuildActionsForOperator(ticket, owner); !actions.CanCancel || !actions.CanSaveRepair || actions.CanAssign {
		t.Fatalf("owner actions = %+v", actions)
	}
	if actions := EnterpriseTicketService.BuildActionsForOperator(ticket, peer); actions.CanCancel || actions.CanSaveRepair || actions.CanStartMeeting {
		t.Fatalf("peer actions = %+v", actions)
	}
	if actions := EnterpriseTicketService.BuildActionsForOperator(ticket, manager); !actions.CanCancel || !actions.CanAssign || !actions.CanSaveRepair || !actions.CanStartMeeting {
		t.Fatalf("manager actions = %+v", actions)
	}
}

func TestEnterpriseTicketActionsIgnoreAssignedEngineerDisplayWorkStatus(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	if err := db.AutoMigrate(&models.Product{}, &models.AgentTeamMember{}); err != nil {
		t.Fatalf("auto migrate enterprise ticket action fixture: %v", err)
	}
	now := time.Now()
	teamID := createEnterpriseTicketActionRepairTeamFixture(t, 1, 701, 1701, false)
	ticket := &models.Ticket{
		TenantID: 1, ProductID: 701, CurrentTeamID: teamID, CurrentAssigneeID: 1701,
		Status: enums.TicketStatusPendingAssigneeAccept,
	}
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 1701, Roles: []string{EnterpriseRoleEngineer}}

	if actions := EnterpriseTicketService.BuildActionsForOperator(ticket, operator); !actions.CanAccept {
		t.Fatalf("available assigned engineer should be able to accept: %+v", actions)
	}
	if err := db.Model(&models.AgentWorkStatus{}).
		Where("tenant_id = ? AND user_id = ?", 1, 1701).
		Updates(map[string]any{"status": AgentWorkStatusLeave, "status_changed_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	if actions := EnterpriseTicketService.BuildActionsForOperator(ticket, operator); !actions.CanAccept {
		t.Fatalf("display work status must not hide an explicitly assigned ticket: %+v", actions)
	}
}

func TestEnterpriseTicketActionsHideAcceptForDispatchManagerOnAnotherAssignee(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	if err := db.AutoMigrate(&models.Product{}, &models.AgentTeamMember{}); err != nil {
		t.Fatalf("auto migrate enterprise ticket action fixture: %v", err)
	}
	teamID := createEnterpriseTicketActionRepairTeamFixture(t, 1, 703, 1801, false)
	createEnterpriseTicketActionEngineerFixture(t, 1, teamID, 1802)
	deadline := time.Now().Add(time.Minute)
	ticket := &models.Ticket{
		TenantID: 1, ProductID: 703, CurrentTeamID: teamID, CurrentAssigneeID: 1801,
		Status: enums.TicketStatusPendingAssigneeAccept, AcceptDeadlineAt: &deadline,
	}
	manager := &dto.AuthPrincipal{TenantID: 1, UserID: 1802, Roles: []string{EnterpriseRoleServiceManager}}

	if actions := EnterpriseTicketService.BuildActionsForOperator(ticket, manager); !actions.CanAccept || !actions.CanTakeover {
		t.Fatalf("dispatch manager should be able to override another assignee availability: %+v", actions)
	}
}

func TestEnterpriseTicketActionsAllowExpiredProductTeamPeerTakeover(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	if err := db.AutoMigrate(&models.Product{}, &models.AgentTeamMember{}); err != nil {
		t.Fatalf("auto migrate enterprise ticket action fixture: %v", err)
	}
	teamID := createEnterpriseTicketActionRepairTeamFixture(t, 1, 706, 1806, false)
	createEnterpriseTicketActionEngineerFixture(t, 1, teamID, 1807)
	deadline := time.Now().Add(-time.Minute)
	ticket := &models.Ticket{
		TenantID: 1, ProductID: 706, CurrentTeamID: teamID, CurrentAssigneeID: 1806,
		Status: enums.TicketStatusPendingAssigneeAccept, AcceptDeadlineAt: &deadline,
	}
	peer := &dto.AuthPrincipal{TenantID: 1, UserID: 1807, Roles: []string{EnterpriseRoleEngineer}}
	originalAssignee := &dto.AuthPrincipal{TenantID: 1, UserID: 1806, Roles: []string{EnterpriseRoleEngineer}}

	actions := EnterpriseTicketService.BuildActionsForOperator(ticket, peer)
	if actions.CanAccept || !actions.CanTakeover {
		t.Fatalf("eligible product-team peer should see takeover, not ordinary accept: %+v", actions)
	}
	if actions := EnterpriseTicketService.BuildActionsForOperator(ticket, originalAssignee); actions.CanAccept {
		t.Fatalf("original assignee must not accept after the deadline: %+v", actions)
	}
	if err := db.Model(&models.AgentWorkStatus{}).
		Where("tenant_id = ? AND user_id = ?", 1, 1807).
		Updates(map[string]any{"status": AgentWorkStatusLeave, "status_changed_at": time.Now()}).Error; err != nil {
		t.Fatal(err)
	}
	if actions := EnterpriseTicketService.BuildActionsForOperator(ticket, peer); !actions.CanTakeover {
		t.Fatalf("display work status must not hide manual product-team takeover: %+v", actions)
	}
}

func TestEnterpriseTicketActionsAllowLegacyAssignedPeerTakeoverAfterDerivedDeadline(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	if err := db.AutoMigrate(&models.Product{}, &models.AgentTeamMember{}); err != nil {
		t.Fatalf("auto migrate enterprise ticket action fixture: %v", err)
	}
	teamID := createEnterpriseTicketActionRepairTeamFixture(t, 1, 708, 1810, true)
	createEnterpriseTicketActionEngineerFixture(t, 1, teamID, 1811)
	stale := time.Now().Add(-2 * time.Hour)
	if err := db.Model(&models.AgentProfile{}).Where("tenant_id = ? AND user_id = ?", 1, 1811).
		Update("last_online_at", stale.Add(-time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	ticket := &models.Ticket{
		TenantID: 1, ProductID: 708, CurrentTeamID: teamID, CurrentAssigneeID: 1810,
		Status:      enums.TicketStatusAssigned,
		AuditFields: models.AuditFields{CreatedAt: stale, UpdatedAt: stale},
	}
	peer := &dto.AuthPrincipal{TenantID: 1, UserID: 1811, Roles: []string{EnterpriseRoleEngineer}}
	originalAssignee := &dto.AuthPrincipal{TenantID: 1, UserID: 1810, Roles: []string{EnterpriseRoleEngineer}}

	if actions := EnterpriseTicketService.BuildActionsForOperator(ticket, peer); !actions.CanTakeover || actions.CanAccept {
		t.Fatalf("eligible peer should be able to take over a stale legacy assignment: %+v", actions)
	}
	if actions := EnterpriseTicketService.BuildActionsForOperator(ticket, originalAssignee); actions.CanAccept {
		t.Fatalf("legacy assignee must not accept after the derived deadline: %+v", actions)
	}
}

func TestEnterpriseTicketActionsAllowProductTeamTakeoverWhenAnotherEngineerIsOnShift(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	if err := db.AutoMigrate(&models.Product{}, &models.AgentTeamMember{}); err != nil {
		t.Fatalf("auto migrate enterprise ticket action fixture: %v", err)
	}
	teamID := createEnterpriseTicketActionRepairTeamFixture(t, 1, 709, 1812, true)
	createEnterpriseTicketActionEngineerFixture(t, 1, teamID, 1813)
	now := time.Now()
	minute := scheduleMinute(now)
	if err := db.Create(&models.AgentTeamSchedule{
		TenantID:      1,
		TeamID:        teamID,
		UserID:        1812,
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
	stale := now.Add(-2 * time.Hour)
	ticket := &models.Ticket{
		TenantID: 1, ProductID: 709, CurrentTeamID: teamID, CurrentAssigneeID: 1812,
		Status:      enums.TicketStatusAssigned,
		AuditFields: models.AuditFields{CreatedAt: stale, UpdatedAt: stale},
	}
	offShiftPeer := &dto.AuthPrincipal{TenantID: 1, UserID: 1813, Roles: []string{EnterpriseRoleEngineer}}

	if actions := EnterpriseTicketService.BuildActionsForOperator(ticket, offShiftPeer); !actions.CanTakeover {
		t.Fatalf("product-team peer should retain the explicit takeover action: %+v", actions)
	}
}

func TestEnterpriseTicketActionsAllowOverdueCustomerReplyProductTeamPeerTakeover(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	if err := db.AutoMigrate(&models.Product{}, &models.AgentTeamMember{}); err != nil {
		t.Fatalf("auto migrate enterprise ticket action fixture: %v", err)
	}
	teamID := createEnterpriseTicketActionRepairTeamFixture(t, 1, 707, 1808, false)
	createEnterpriseTicketActionEngineerFixture(t, 1, teamID, 1809)
	now := time.Now()
	acceptedAt := now.Add(-45 * time.Minute)
	conversation := models.Conversation{
		TenantID: 1, ProductID: 707, CurrentTeamID: teamID, CurrentAssigneeID: 1808,
		Status: enums.IMConversationStatusActive, LastMessageAt: acceptedAt, LastActiveAt: acceptedAt,
		AuditFields: models.AuditFields{CreatedAt: acceptedAt, UpdatedAt: acceptedAt},
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Message{
		ConversationID: conversation.ID, ClientMsgID: "overdue-customer-reply", SenderType: enums.IMSenderTypeCustomer,
		MessageType: enums.IMMessageTypeText, Content: "设备仍然无法启动，请回复", SendStatus: enums.IMMessageStatusSent,
		SentAt: &acceptedAt, AuditFields: models.AuditFields{CreatedAt: acceptedAt, UpdatedAt: acceptedAt},
	}).Error; err != nil {
		t.Fatal(err)
	}
	ticket := &models.Ticket{
		TenantID: 1, ProductID: 707, ConversationID: conversation.ID, CurrentTeamID: teamID, CurrentAssigneeID: 1808,
		Status: enums.TicketStatusProcessing, AcceptedAt: &acceptedAt,
		AuditFields: models.AuditFields{CreatedAt: acceptedAt, UpdatedAt: acceptedAt},
	}
	peer := &dto.AuthPrincipal{TenantID: 1, UserID: 1809, Roles: []string{EnterpriseRoleEngineer}}

	if actions := EnterpriseTicketService.BuildActionsForOperator(ticket, peer); !actions.CanTakeover {
		t.Fatalf("eligible product-team peer should be able to take over an overdue unanswered customer message: %+v", actions)
	}
	repliedAt := now.Add(-time.Minute)
	if err := db.Create(&models.Message{
		ConversationID: conversation.ID, ClientMsgID: "recent-engineer-reply", SenderType: enums.IMSenderTypeAgent,
		SenderID: 1808, MessageType: enums.IMMessageTypeText, Content: "正在处理", SendStatus: enums.IMMessageStatusSent,
		SentAt: &repliedAt, AuditFields: models.AuditFields{CreatedAt: repliedAt, UpdatedAt: repliedAt},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if actions := EnterpriseTicketService.BuildActionsForOperator(ticket, peer); !actions.CanTakeover {
		t.Fatalf("product-team peer should retain explicit takeover after a recent reply: %+v", actions)
	}
}

func TestEnterpriseTicketActionsAllowManualAcceptOutsideProductRepairSchedule(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	if err := db.AutoMigrate(&models.Product{}, &models.AgentTeamMember{}); err != nil {
		t.Fatalf("auto migrate enterprise ticket action fixture: %v", err)
	}
	now := time.Now()
	minute := scheduleMinute(now)
	teamID := createEnterpriseTicketActionRepairTeamFixture(t, 1, 702, 1702, true)
	createEnterpriseTicketActionEngineerFixture(t, 1, teamID, 1703)
	if err := db.Create(&models.AgentTeamSchedule{
		TenantID:      1,
		TeamID:        teamID,
		UserID:        1703,
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
	ticket := &models.Ticket{
		TenantID: 1, ProductID: 702,
		Status: enums.TicketStatusPendingAcceptance,
	}
	offShift := &dto.AuthPrincipal{TenantID: 1, UserID: 1702, Roles: []string{EnterpriseRoleEngineer}}
	onShift := &dto.AuthPrincipal{TenantID: 1, UserID: 1703, Roles: []string{EnterpriseRoleEngineer}}

	if actions := EnterpriseTicketService.BuildActionsForOperator(ticket, offShift); !actions.CanAccept {
		t.Fatalf("off-shift product repair engineer should be able to manually accept: %+v", actions)
	}
	if actions := EnterpriseTicketService.BuildActionsForOperator(ticket, onShift); !actions.CanAccept {
		t.Fatalf("on-shift product repair engineer should be able to accept: %+v", actions)
	}
}

func TestEnterpriseTicketActionsAllowManualAcceptWhenEngineerNotInAutoDispatch(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	if err := db.AutoMigrate(&models.Product{}, &models.ProductModule{}, &models.AgentTeamMember{}, &models.TenantMember{}, &models.EngineerProfile{}); err != nil {
		t.Fatalf("auto migrate enterprise ticket action fixture: %v", err)
	}
	now := time.Now()
	teamID := createEnterpriseTicketActionRepairTeamFixture(t, 1, 706, 1706, false)
	module := models.ProductModule{
		TenantID: 1, ProductID: 706, ModuleCode: "PUMP-CTRL", Name: "Pump Controller", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&module).Error; err != nil {
		t.Fatalf("create product module: %v", err)
	}
	member := models.TenantMember{
		TenantID: 1, UserID: 1706, MemberNo: "M-1706", DisplayName: "Manual Engineer", MemberType: "employee", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&member).Error; err != nil {
		t.Fatalf("create tenant member: %v", err)
	}
	engineer := models.EngineerProfile{
		TenantID: 1, MemberID: member.ID, SkillTagsJSON: `["PUMP-CTRL"]`, ServiceRegionsJSON: `["CN"]`,
		LanguagesJSON: "[]", Timezone: "UTC", MaxTicketLoad: 3, DispatchEnabled: false, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&engineer).Error; err != nil {
		t.Fatalf("create engineer profile: %v", err)
	}
	ticket := &models.Ticket{
		TenantID: 1, ProductID: 706, ProductModuleID: module.ID, CurrentTeamID: teamID,
		ServiceRegion: "CN", Status: enums.TicketStatusPendingDispatch,
	}
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 1706, Roles: []string{EnterpriseRoleEngineer}}

	if actions := EnterpriseTicketService.BuildActionsForOperator(ticket, operator); !actions.CanAccept {
		t.Fatalf("manual product-team claim should not require auto-dispatch enablement: %+v", actions)
	}
}

func createEnterpriseTicketActionRepairTeamFixture(t *testing.T, tenantID, productID, userID int64, scheduleEnforced bool) int64 {
	t.Helper()
	db := sqls.DB()
	product := models.Product{
		ID: productID, TenantID: tenantID, Code: fmt.Sprintf("P-%d", productID), Name: fmt.Sprintf("Product %d", productID), Status: enums.StatusOk,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	team := models.AgentTeam{
		TenantID: tenantID, ProductID: productID, TeamType: AgentTeamTypeProductRepair,
		Name: fmt.Sprintf("product-%d-repair", productID), ScheduleEnforced: scheduleEnforced, Status: enums.StatusOk,
	}
	if err := db.Create(&team).Error; err != nil {
		t.Fatalf("create repair team: %v", err)
	}
	createEnterpriseTicketActionEngineerFixture(t, tenantID, team.ID, userID)
	return team.ID
}

func createEnterpriseTicketActionEngineerFixture(t *testing.T, tenantID, teamID, userID int64) {
	t.Helper()
	db := sqls.DB()
	now := time.Now()
	if err := db.Create(&models.User{
		ID: userID, Username: fmt.Sprintf("engineer-%d", userID), Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&models.AgentProfile{
		TenantID: tenantID, UserID: userID, TeamID: teamID, AgentCode: fmt.Sprintf("E-%d", userID),
		DisplayName: fmt.Sprintf("Engineer %d", userID), ServiceStatus: enums.ServiceStatusIdle,
		MaxConcurrentCount: 3, AutoAssignEnabled: true, LastOnlineAt: &now, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create profile: %v", err)
	}
	if err := db.Create(&models.AgentTeamMember{
		TenantID: tenantID, TeamID: teamID, UserID: userID, DispatchEnabled: true, DispatchWeight: 1, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create team member: %v", err)
	}
	if err := db.Create(&models.AgentWorkStatus{
		TenantID: tenantID, UserID: userID, Status: AgentWorkStatusAvailable, ConfirmedAt: now, StatusChangedAt: now,
	}).Error; err != nil {
		t.Fatalf("create work status: %v", err)
	}
}
