package services_test

import (
	"fmt"
	"testing"
	"time"

	"gorm.io/gorm"
	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"
)

func ensureAgentTeamMemberEnterpriseWorkTime(t *testing.T, db *gorm.DB, tenantID int64, at time.Time) {
	t.Helper()
	location, err := time.LoadLocation(services.EngineerScheduleTimezone)
	if err != nil {
		location = time.FixedZone("CST", 8*60*60)
	}
	local := at.In(location)
	weekday := int(local.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	minute := local.Hour()*60 + local.Minute()
	startMinute := max(0, minute-1)
	endMinute := min(24*60, minute+30)
	if endMinute <= startMinute {
		startMinute = 0
		endMinute = 24 * 60
	}
	if err := db.Create(&models.AgentTeamScheduleTemplate{
		TenantID:    tenantID,
		Workdays:    fmt.Sprintf("[%d]", weekday),
		StartMinute: startMinute,
		EndMinute:   endMinute,
		Timezone:    services.EngineerScheduleTimezone,
		Status:      enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create enterprise work-time template: %v", err)
	}
}

func TestAgentWorkStatusServiceUsesCalendarAndApprovedLeave(t *testing.T) {
	db := setupAgentTeamScheduleTestDB(t)
	team := models.AgentTeam{ID: 31, TenantID: 1, Name: "产品维修组", Status: enums.StatusOk}
	profile := models.AgentProfile{TenantID: 1, UserID: 301, TeamID: team.ID, AgentCode: "engineer-301", DisplayName: "Engineer 301", ServiceStatus: enums.ServiceStatusIdle, AutoAssignEnabled: true, Status: enums.StatusOk}
	if err := db.Create(&team).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatal(err)
	}
	operator := &dto.AuthPrincipal{UserID: profile.UserID, Username: "engineer-301", TenantID: 1, Status: enums.StatusOk}
	now := time.Now()
	ensureAgentTeamMemberEnterpriseWorkTime(t, db, 1, now)
	item, err := services.AgentWorkStatusService.UpdateMyStatus(request.UpdateAgentWorkStatusRequest{
		Status: "leave", Note: "annual leave", AvailableAt: now.Add(8 * time.Hour).Format(time.RFC3339),
	}, operator, now)
	if err != nil {
		t.Fatalf("set leave: %v", err)
	}
	if item.Status != services.AgentWorkStatusLeave {
		t.Fatalf("status = %q", item.Status)
	}
	if !services.AgentWorkStatusService.IsDispatchAvailable(1, profile.UserID) {
		t.Fatal("work status leave must not override the configured work calendar")
	}
	if available := services.AgentWorkStatusService.FindAvailableUserSet(1, []int64{profile.UserID}); !available[profile.UserID] {
		t.Fatal("work status leave must not remove engineer from the calendar availability set")
	}
	if err := db.Create(&models.AgentScheduleException{
		TenantID: 1, UserID: profile.UserID, RequestKey: "status-leave-approved",
		ExceptionType: services.AgentScheduleExceptionTypeLeave,
		StartAt:       now.Add(-time.Hour), EndAt: now.Add(time.Hour),
		ApprovalStatus: services.AgentScheduleApprovalApproved, RequestedAt: now.Add(-2 * time.Hour), Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create approved leave: %v", err)
	}
	if services.AgentWorkStatusService.IsDispatchAvailable(1, profile.UserID) {
		t.Fatal("approved leave must block dispatch availability")
	}
	if available := services.AgentWorkStatusService.FindAvailableUserSet(1, []int64{profile.UserID}); available[profile.UserID] {
		t.Fatal("approved leave remained in the available set")
	}
	item, err = services.AgentWorkStatusService.UpdateMyStatus(request.UpdateAgentWorkStatusRequest{Status: "available"}, operator, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("restore available: %v", err)
	}
	if item.AvailableAt != nil || services.AgentWorkStatusService.IsDispatchAvailable(1, profile.UserID) {
		t.Fatalf("approved leave should continue to block dispatch after work-status change: %+v", item)
	}
	if err := db.Where("tenant_id = ? AND user_id = ?", 1, profile.UserID).Delete(&models.AgentScheduleException{}).Error; err != nil {
		t.Fatalf("remove approved leave: %v", err)
	}
	if !services.AgentWorkStatusService.IsDispatchAvailable(1, profile.UserID) {
		t.Fatal("engineer should return to dispatch availability after approved leave ends")
	}
}

func TestAgentWorkStatusAvailableSetHonorsFutureAvailableAt(t *testing.T) {
	db := setupAgentTeamScheduleTestDB(t)
	now := time.Now()
	ensureAgentTeamMemberEnterpriseWorkTime(t, db, 1, now)
	future := now.Add(30 * time.Minute)
	if err := db.Create(&models.AgentWorkStatus{
		TenantID:        1,
		UserID:          303,
		Status:          services.AgentWorkStatusAvailable,
		AvailableAt:     &future,
		ConfirmedAt:     now,
		StatusChangedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if available := services.AgentWorkStatusService.FindAvailableUserSet(1, []int64{303}); !available[303] {
		t.Fatal("future available_at must not override the configured work calendar")
	}
	if !services.AgentWorkStatusService.IsDispatchAvailable(1, 303) {
		t.Fatal("future available_at must not override single-user calendar availability checks")
	}
	if err := db.Model(&models.AgentWorkStatus{}).Where("tenant_id = ? AND user_id = ?", 1, 303).Update("available_at", nil).Error; err != nil {
		t.Fatal(err)
	}
	if available := services.AgentWorkStatusService.FindAvailableUserSet(1, []int64{303}); !available[303] {
		t.Fatal("engineer on a configured workday should be dispatchable")
	}
	if !services.AgentWorkStatusService.IsDispatchAvailable(1, 303) {
		t.Fatal("confirmed available engineer without future available_at should pass single-user dispatch availability checks")
	}
}

func TestAgentWorkStatusServiceBriefingCreatesUnconfirmedDefaultStatus(t *testing.T) {
	db := setupAgentTeamScheduleTestDB(t)
	team := models.AgentTeam{ID: 32, TenantID: 1, Name: "默认确认组", Status: enums.StatusOk}
	profile := models.AgentProfile{TenantID: 1, UserID: 302, TeamID: team.ID, AgentCode: "engineer-302", DisplayName: "Engineer 302", ServiceStatus: enums.ServiceStatusIdle, AutoAssignEnabled: true, Status: enums.StatusOk}
	if err := db.Create(&team).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatal(err)
	}
	ensureAgentTeamMemberEnterpriseWorkTime(t, db, 1, time.Now())
	operator := &dto.AuthPrincipal{UserID: profile.UserID, Username: "engineer-302", TenantID: 1, Status: enums.StatusOk}
	result, err := services.AgentWorkStatusService.GetBriefing(operator, time.Now())
	if err != nil {
		t.Fatalf("briefing: %v", err)
	}
	if !result.NeedsConfirmation || result.WorkStatus.Status != services.AgentWorkStatusAvailable || !result.WorkStatus.ConfirmedAt.IsZero() {
		t.Fatalf("default status should require explicit confirmation: %+v", result)
	}
	if !services.AgentWorkStatusService.IsDispatchAvailable(1, profile.UserID) {
		t.Fatal("unconfirmed default status must not override calendar dispatch availability")
	}
}

func TestAgentWorkStatusServiceBriefingSkipsTemporarySupportSessions(t *testing.T) {
	db := setupAgentTeamScheduleTestDB(t)
	profile := models.AgentProfile{
		TenantID:          1,
		UserID:            304,
		AgentCode:         "support-operator-304",
		DisplayName:       "Support Operator 304",
		ServiceStatus:     enums.ServiceStatusIdle,
		AutoAssignEnabled: true,
		Status:            enums.StatusOk,
	}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatal(err)
	}
	operator := &dto.AuthPrincipal{
		UserID:         profile.UserID,
		Username:       "support-operator-304",
		TenantID:       1,
		Status:         enums.StatusOk,
		SupportGrantID: 99,
		SupportMode:    models.SupportModePlatformTenant,
	}
	result, err := services.AgentWorkStatusService.GetBriefing(operator, time.Now())
	if err != nil {
		t.Fatalf("briefing: %v", err)
	}
	if result.IsEngineer {
		t.Fatalf("temporary support session must not receive engineer briefing: %+v", result)
	}
	if _, err := services.AgentWorkStatusService.UpdateMyStatus(request.UpdateAgentWorkStatusRequest{Status: "available"}, operator, time.Now()); err == nil {
		t.Fatal("temporary support session must not change engineer work status")
	}
}

func TestAgentWorkStatusServiceBriefingAllowsEmployeePortalSession(t *testing.T) {
	db := setupAgentTeamScheduleTestDB(t)
	profile := models.AgentProfile{
		TenantID:          1,
		UserID:            305,
		AgentCode:         "engineer-305",
		DisplayName:       "Engineer 305",
		ServiceStatus:     enums.ServiceStatusIdle,
		AutoAssignEnabled: true,
		Status:            enums.StatusOk,
	}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatal(err)
	}
	operator := &dto.AuthPrincipal{
		UserID:         profile.UserID,
		Username:       "engineer-305",
		TenantID:       1,
		Status:         enums.StatusOk,
		DomainType:     models.DomainTypeEnterprise,
		SubjectType:    models.SubjectTypeTenantMember,
		SubjectID:      7305,
		MemberID:       7305,
		SupportGrantID: 100,
		SupportMode:    models.SupportModeEmployeePortal,
		ImpersonatedBy: "enterprise-admin",
	}
	now := time.Now()
	result, err := services.AgentWorkStatusService.GetBriefing(operator, now)
	if err != nil {
		t.Fatalf("briefing: %v", err)
	}
	if !result.IsEngineer || !result.NeedsConfirmation {
		t.Fatalf("employee portal session should receive engineer briefing: %+v", result)
	}
	item, err := services.AgentWorkStatusService.UpdateMyStatus(request.UpdateAgentWorkStatusRequest{Status: "available"}, operator, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("employee portal session should confirm engineer work status: %v", err)
	}
	if item.Status != services.AgentWorkStatusAvailable || item.ConfirmedAt.IsZero() {
		t.Fatalf("unexpected confirmed status: %+v", item)
	}
}

func TestAgentWorkStatusServiceBriefingScopesTeamAndPersonalTickets(t *testing.T) {
	db := setupAgentTeamScheduleTestDB(t)
	team := models.AgentTeam{ID: 41, TenantID: 1, ProductID: 501, Name: "产品 A 维修组", Status: enums.StatusOk}
	otherTeam := models.AgentTeam{ID: 42, TenantID: 1, ProductID: 502, Name: "产品 B 维修组", Status: enums.StatusOk}
	profile := models.AgentProfile{TenantID: 1, UserID: 401, TeamID: team.ID, AgentCode: "engineer-401", DisplayName: "Engineer 401", ServiceStatus: enums.ServiceStatusIdle, AutoAssignEnabled: true, Status: enums.StatusOk}
	member := models.AgentTeamMember{TenantID: 1, TeamID: team.ID, UserID: profile.UserID, Status: enums.StatusOk, DispatchEnabled: true}
	for _, item := range []any{&team, &otherTeam, &profile, &member} {
		if err := db.Create(item).Error; err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	deferredUntil := now.Add(15 * time.Minute)
	tickets := []models.Ticket{
		{TicketNo: "T-A-PRODUCT", TenantID: 1, ProductID: team.ProductID, Status: enums.TicketStatusPendingDispatch, AuditFields: models.AuditFields{CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now}},
		{
			TicketNo: "T-A-TEAM", TenantID: 1, CurrentTeamID: team.ID, Status: enums.TicketStatusPendingDispatch,
			DispatchAttempts: 1, DispatchDeferredUntil: &deferredUntil, LastDispatchFailureReason: "all_candidates_at_capacity",
			AuditFields: models.AuditFields{CreatedAt: now.Add(-3 * time.Hour), UpdatedAt: now},
		},
		{TicketNo: "T-B-OTHER", TenantID: 1, CurrentTeamID: otherTeam.ID, Status: enums.TicketStatusPendingDispatch, AuditFields: models.AuditFields{CreatedAt: now.Add(-4 * time.Hour), UpdatedAt: now}},
		{TicketNo: "T-MINE-OLD", TenantID: 1, CurrentTeamID: team.ID, CurrentAssigneeID: profile.UserID, Status: enums.TicketStatusProcessing, AuditFields: models.AuditFields{CreatedAt: now.Add(-5 * time.Hour), UpdatedAt: now}},
		{TicketNo: "T-MINE-NEW", TenantID: 1, CurrentTeamID: team.ID, CurrentAssigneeID: profile.UserID, Status: enums.TicketStatusPendingAssigneeAccept, AuditFields: models.AuditFields{CreatedAt: now.Add(-time.Hour), UpdatedAt: now}},
		{TicketNo: "T-MINE-CLOSED", TenantID: 1, CurrentTeamID: team.ID, CurrentAssigneeID: profile.UserID, Status: enums.TicketStatusClosed, AuditFields: models.AuditFields{CreatedAt: now.Add(-6 * time.Hour), UpdatedAt: now}},
	}
	if err := db.Create(&tickets).Error; err != nil {
		t.Fatal(err)
	}
	operator := &dto.AuthPrincipal{UserID: profile.UserID, Username: "engineer-401", TenantID: 1, Status: enums.StatusOk}
	result, err := services.AgentWorkStatusService.GetBriefing(operator, time.Now())
	if err != nil {
		t.Fatalf("briefing: %v", err)
	}
	if !result.IsEngineer || len(result.Teams) != 1 {
		t.Fatalf("unexpected engineer scope: %+v", result)
	}
	if len(result.UnassignedTickets) != 2 {
		t.Fatalf("unassigned tickets = %+v, want product and team scoped tickets", result.UnassignedTickets)
	}
	if result.UnassignedTickets[0].TicketNo != "T-A-TEAM" {
		t.Fatalf("unassigned tickets are not oldest first: %+v", result.UnassignedTickets)
	}
	if len(result.MyOpenTickets) != 2 || result.MyOpenTickets[0].TicketNo != "T-MINE-OLD" {
		t.Fatalf("my open tickets = %+v", result.MyOpenTickets)
	}
	if result.UnassignedTicketCount != 2 || result.MyOpenTicketCount != 2 {
		t.Fatalf("unexpected briefing counts: unassigned=%d mine=%d", result.UnassignedTicketCount, result.MyOpenTicketCount)
	}
	briefingDTO := builders.BuildEngineerBriefingResponse(result)
	if briefingDTO == nil || len(briefingDTO.UnassignedTickets) == 0 {
		t.Fatalf("briefing dto did not include unassigned tickets: %+v", briefingDTO)
	}
	oldest := briefingDTO.UnassignedTickets[0]
	if oldest.DispatchAttempts != 1 || oldest.DispatchDeferredUntil == "" || oldest.LastDispatchFailureReason != "all_candidates_at_capacity" {
		t.Fatalf("briefing dto should expose dispatch aging signals, got %+v", oldest)
	}
}

func TestAgentWorkStatusServiceBriefingCountsFullQueuesWhilePreviewingOldest(t *testing.T) {
	db := setupAgentTeamScheduleTestDB(t)
	team := models.AgentTeam{ID: 43, TenantID: 1, ProductID: 503, Name: "产品 C 维修组", Status: enums.StatusOk}
	profile := models.AgentProfile{TenantID: 1, UserID: 402, TeamID: team.ID, AgentCode: "engineer-402", DisplayName: "Engineer 402", ServiceStatus: enums.ServiceStatusIdle, AutoAssignEnabled: true, Status: enums.StatusOk}
	member := models.AgentTeamMember{TenantID: 1, TeamID: team.ID, UserID: profile.UserID, Status: enums.StatusOk, DispatchEnabled: true}
	for _, item := range []any{&team, &profile, &member} {
		if err := db.Create(item).Error; err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	tickets := make([]models.Ticket, 0, 23)
	for i := 0; i < 12; i++ {
		tickets = append(tickets, models.Ticket{
			TicketNo:  fmt.Sprintf("T-UNASSIGNED-%02d", i),
			TenantID:  1,
			ProductID: team.ProductID,
			Status:    enums.TicketStatusPendingDispatch,
			AuditFields: models.AuditFields{
				CreatedAt: now.Add(time.Duration(i-120) * time.Minute),
				UpdatedAt: now,
			},
		})
	}
	for i := 0; i < 11; i++ {
		tickets = append(tickets, models.Ticket{
			TicketNo:          fmt.Sprintf("T-MINE-%02d", i),
			TenantID:          1,
			CurrentTeamID:     team.ID,
			CurrentAssigneeID: profile.UserID,
			Status:            enums.TicketStatusProcessing,
			AuditFields: models.AuditFields{
				CreatedAt: now.Add(time.Duration(i-90) * time.Minute),
				UpdatedAt: now,
			},
		})
	}
	if err := db.Create(&tickets).Error; err != nil {
		t.Fatal(err)
	}

	operator := &dto.AuthPrincipal{UserID: profile.UserID, Username: "engineer-402", TenantID: 1, Status: enums.StatusOk}
	result, err := services.AgentWorkStatusService.GetBriefing(operator, time.Now())
	if err != nil {
		t.Fatalf("briefing: %v", err)
	}

	if result.UnassignedTicketCount != 12 || len(result.UnassignedTickets) != 10 {
		t.Fatalf("unassigned preview/count mismatch: count=%d len=%d", result.UnassignedTicketCount, len(result.UnassignedTickets))
	}
	if result.MyOpenTicketCount != 11 || len(result.MyOpenTickets) != 10 {
		t.Fatalf("my open preview/count mismatch: count=%d len=%d", result.MyOpenTicketCount, len(result.MyOpenTickets))
	}
	if result.UnassignedTickets[0].TicketNo != "T-UNASSIGNED-00" || result.UnassignedTickets[9].TicketNo != "T-UNASSIGNED-09" {
		t.Fatalf("unassigned preview is not oldest first: %+v", result.UnassignedTickets)
	}
	if result.MyOpenTickets[0].TicketNo != "T-MINE-00" || result.MyOpenTickets[9].TicketNo != "T-MINE-09" {
		t.Fatalf("my open preview is not oldest first: %+v", result.MyOpenTickets)
	}
}
