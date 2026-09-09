package services

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func scheduleWeekday(at time.Time) int {
	local := at.In(engineerScheduleLocation())
	weekday := int(local.Weekday())
	if weekday == 0 {
		return 7
	}
	return weekday
}

func scheduleMinute(at time.Time) int {
	local := at.In(engineerScheduleLocation())
	return local.Hour()*60 + local.Minute()
}

func TestTicketLifecycleRejectsCrossTenantMutation(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Ticket{}, &models.TicketProgress{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	now := time.Now()
	ticket := models.Ticket{
		TenantID:    7301,
		TicketNo:    "TENANT-ISOLATION-1",
		Title:       "Tenant isolation",
		Status:      enums.TicketStatusPending,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	operator := &dto.AuthPrincipal{TenantID: 7302, UserID: 9, Username: "other-tenant"}
	if err := TicketLifecycleService.Accept(ticket.ID, 0, operator); err == nil {
		t.Fatal("cross-tenant accept should fail")
	}
	var persisted models.Ticket
	if err := db.First(&persisted, ticket.ID).Error; err != nil {
		t.Fatalf("reload ticket: %v", err)
	}
	if persisted.Status != enums.TicketStatusPending {
		t.Fatalf("status = %s, want %s", persisted.Status, enums.TicketStatusPending)
	}
	var progressCount int64
	if err := db.Model(&models.TicketProgress{}).Where("ticket_id = ?", ticket.ID).Count(&progressCount).Error; err != nil {
		t.Fatalf("count progress: %v", err)
	}
	if progressCount != 0 {
		t.Fatalf("progress count = %d, want 0", progressCount)
	}
}

func TestTicketLifecycleAcceptRequiresProductTeamMembership(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.AgentProfile{}, &models.AgentTeam{}, &models.AgentTeamMember{}, &models.AgentTeamScheduleTemplate{}, &models.AgentWorkStatus{}, &models.Ticket{}, &models.TicketProgress{}, &models.TicketDispatchAttempt{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	user := models.User{Username: "product-engineer", Status: enums.StatusOk}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	now := time.Now()
	local := now.In(time.FixedZone("CST", 8*60*60))
	weekday := int(local.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	minute := local.Hour()*60 + local.Minute()
	startMinute := minute - 1
	if startMinute < 0 {
		startMinute = 0
	}
	endMinute := minute + 30
	if endMinute > 24*60 {
		endMinute = 24 * 60
	}
	if endMinute <= startMinute {
		startMinute = 0
		endMinute = 24 * 60
	}
	if err := db.Create(&models.AgentTeamScheduleTemplate{
		TenantID:    1,
		Workdays:    "[" + strconv.Itoa(weekday) + "]",
		StartMinute: startMinute,
		EndMinute:   endMinute,
		Timezone:    EngineerScheduleTimezone,
		Status:      enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create enterprise work-time template: %v", err)
	}
	profile := models.AgentProfile{
		TenantID: 1, UserID: user.ID, TeamID: 11, AgentCode: "PRODUCT-ENGINEER",
		DisplayName: "产品工程师", ServiceStatus: enums.ServiceStatusIdle,
		MaxConcurrentCount: 3, AutoAssignEnabled: true, LastOnlineAt: &now, Status: enums.StatusOk,
	}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("create profile: %v", err)
	}
	for _, team := range []models.AgentTeam{
		{ID: 10, TenantID: 1, Name: "产品 A", Status: enums.StatusOk},
		{ID: 11, TenantID: 1, Name: "产品 B", Status: enums.StatusOk},
	} {
		if err := db.Create(&team).Error; err != nil {
			t.Fatalf("create team: %v", err)
		}
	}
	if err := db.Create(&models.AgentTeamMember{
		TenantID: 1, TeamID: 11, UserID: user.ID, DispatchEnabled: true, DispatchWeight: 1, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create non-product-team membership: %v", err)
	}
	if err := db.Create(&models.AgentWorkStatus{
		TenantID: 1, UserID: user.ID, Status: AgentWorkStatusAvailable, ConfirmedAt: now, StatusChangedAt: now,
	}).Error; err != nil {
		t.Fatalf("create work status: %v", err)
	}
	ticket := models.Ticket{
		TenantID: 1, CurrentTeamID: 10, TicketNo: "PRODUCT-TEAM-1", Title: "产品组工单",
		Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: user.ID, Username: user.Username, Roles: []string{EnterpriseRoleEngineer}}
	if err := TicketLifecycleService.Accept(ticket.ID, user.ID, operator); err == nil {
		t.Fatal("engineer outside the product team should not accept the ticket")
	}
	if err := db.Create(&models.AgentTeamMember{
		TenantID: 1, TeamID: 10, UserID: user.ID, DispatchEnabled: true, DispatchWeight: 1, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create product-team membership: %v", err)
	}
	if err := TicketLifecycleService.Accept(ticket.ID, user.ID, operator); err != nil {
		t.Fatalf("product team engineer accept error = %v", err)
	}
	var persisted models.Ticket
	if err := db.First(&persisted, ticket.ID).Error; err != nil {
		t.Fatalf("reload ticket: %v", err)
	}
	if persisted.Status != enums.TicketStatusAccepted || persisted.CurrentAssigneeID != user.ID {
		t.Fatalf("unexpected accepted ticket: %+v", persisted)
	}
	peer := models.User{Username: "product-engineer-peer", Status: enums.StatusOk}
	if err := db.Create(&peer).Error; err != nil {
		t.Fatalf("create peer user: %v", err)
	}
	if err := db.Create(&models.AgentProfile{
		TenantID: 1, UserID: peer.ID, TeamID: 10, AgentCode: "PRODUCT-ENGINEER-PEER",
		DisplayName: "同组工程师", ServiceStatus: enums.ServiceStatusIdle,
		MaxConcurrentCount: 3, AutoAssignEnabled: true, LastOnlineAt: &now, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create peer profile: %v", err)
	}
	if err := db.Create(&models.AgentTeamMember{
		TenantID: 1, TeamID: 10, UserID: peer.ID, DispatchEnabled: true, DispatchWeight: 1, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create peer product-team membership: %v", err)
	}
	peerOperator := &dto.AuthPrincipal{TenantID: 1, UserID: peer.ID, Username: peer.Username, Roles: []string{EnterpriseRoleEngineer}}
	if err := requireTicketTenantAccess(&persisted, peerOperator); err != nil {
		t.Fatalf("same product team engineer should be able to view ticket: %v", err)
	}
	if err := requireTicketMutationAccess(&persisted, peerOperator); err == nil {
		t.Fatal("same product team engineer must not mutate another assignee's ticket")
	}
	if err := requireTicketMutationAccess(&persisted, operator); err != nil {
		t.Fatalf("current assignee should be able to mutate ticket: %v", err)
	}
}

func TestEnterpriseTicketListRestrictsEngineerToProductTeam(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.AgentProfile{}, &models.AgentTeam{}, &models.AgentTeamMember{}, &models.Ticket{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	user := models.User{Username: "scoped-engineer", Status: enums.StatusOk}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&models.AgentProfile{
		TenantID: 1, UserID: user.ID, TeamID: 10, AgentCode: "SCOPED-ENGINEER",
		DisplayName: "产品组工程师", Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create profile: %v", err)
	}
	if err := db.Create(&models.AgentTeamMember{
		TenantID: 1, TeamID: 10, UserID: user.ID, DispatchEnabled: true, DispatchWeight: 1, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create product-team membership: %v", err)
	}
	for _, team := range []models.AgentTeam{
		{ID: 10, TenantID: 1, Name: "产品 A", Status: enums.StatusOk},
		{ID: 11, TenantID: 1, Name: "产品 B", Status: enums.StatusOk},
	} {
		if err := db.Create(&team).Error; err != nil {
			t.Fatalf("create team: %v", err)
		}
	}
	now := time.Now()
	for _, ticket := range []models.Ticket{
		{TenantID: 1, CurrentTeamID: 10, TicketNo: "VISIBLE-GROUP", Title: "本组待认领", Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TenantID: 1, CurrentTeamID: 11, CurrentAssigneeID: user.ID, TicketNo: "VISIBLE-ASSIGNED", Title: "跨组但已指派本人", Status: enums.TicketStatusAssigned, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TenantID: 1, CurrentTeamID: 11, TicketNo: "HIDDEN-GROUP", Title: "其他产品组", Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
	} {
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
	}
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: user.ID, Username: user.Username, Roles: []string{EnterpriseRoleEngineer}}
	result, err := EnterpriseTicketService.ListForOperator(1, EnterpriseTicketQuery{Page: 1, PageSize: 20}, operator)
	if err != nil {
		t.Fatalf("list tickets: %v", err)
	}
	if result.Total != 2 || len(result.Items) != 2 {
		t.Fatalf("visible tickets = %+v, want product team and directly assigned tickets", result.Items)
	}
	for _, item := range result.Items {
		if item.TicketNo == "HIDDEN-GROUP" {
			t.Fatal("engineer must not see another product team's unassigned ticket")
		}
	}
	mine, err := EnterpriseTicketService.ListForOperator(1, EnterpriseTicketQuery{Page: 1, PageSize: 20, Mine: true}, operator)
	if err != nil {
		t.Fatalf("list personal tickets: %v", err)
	}
	if mine.Total != 1 || len(mine.Items) != 1 || mine.Items[0].TicketNo != "VISIBLE-ASSIGNED" {
		t.Fatalf("personal tickets = %+v, want only directly assigned ticket", mine.Items)
	}
	mineSummary, err := EnterpriseTicketService.SummaryForOperator(1, EnterpriseTicketQuery{Mine: true}, operator)
	if err != nil {
		t.Fatalf("summarize personal tickets: %v", err)
	}
	if mineSummary.Total != 1 {
		t.Fatalf("personal ticket summary total = %d, want 1", mineSummary.Total)
	}
}

func TestEnterpriseTicketListAndAcceptSupportMultiProductTeamMembership(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.AgentProfile{}, &models.AgentTeam{}, &models.AgentTeamMember{}, &models.AgentTeamScheduleTemplate{}, &models.AgentWorkStatus{}, &models.Ticket{}, &models.TicketProgress{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	user := models.User{Username: "multi-product-engineer", Status: enums.StatusOk}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	now := time.Now()
	if err := db.Create(&models.AgentProfile{
		TenantID: 1, UserID: user.ID, TeamID: 10, AgentCode: "MULTI-PRODUCT-ENGINEER",
		DisplayName: "多产品工程师", ServiceStatus: enums.ServiceStatusIdle,
		MaxConcurrentCount: 3, AutoAssignEnabled: true, LastOnlineAt: &now, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create profile: %v", err)
	}
	if err := db.Create(&models.AgentTeamScheduleTemplate{
		TenantID: 1, Workdays: "[" + strconv.Itoa(scheduleWeekday(now)) + "]",
		StartMinute: max(0, scheduleMinute(now)-1), EndMinute: min(24*60, scheduleMinute(now)+30),
		Timezone: EngineerScheduleTimezone, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create enterprise work-time template: %v", err)
	}
	if err := db.Create(&models.AgentWorkStatus{
		TenantID: 1, UserID: user.ID, Status: AgentWorkStatusAvailable, ConfirmedAt: now, StatusChangedAt: now,
	}).Error; err != nil {
		t.Fatalf("create work status: %v", err)
	}
	for _, team := range []models.AgentTeam{
		{ID: 10, TenantID: 1, ProductID: 100, Name: "产品 A", TeamType: AgentTeamTypeProductRepair, Status: enums.StatusOk},
		{ID: 11, TenantID: 1, ProductID: 101, Name: "产品 B", TeamType: AgentTeamTypeProductRepair, Status: enums.StatusOk},
		{ID: 12, TenantID: 1, ProductID: 102, Name: "产品 C", TeamType: AgentTeamTypeProductRepair, Status: enums.StatusOk},
	} {
		if err := db.Create(&team).Error; err != nil {
			t.Fatalf("create team: %v", err)
		}
	}
	if err := db.Create(&models.AgentTeamMember{
		TenantID: 1, TeamID: 10, UserID: user.ID, DispatchEnabled: true, DispatchWeight: 1, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create primary team membership: %v", err)
	}
	if err := db.Create(&models.AgentTeamMember{
		TenantID: 1, TeamID: 11, UserID: user.ID, DispatchEnabled: true, DispatchWeight: 2, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create extra team membership: %v", err)
	}
	tickets := []models.Ticket{
		{TenantID: 1, CurrentTeamID: 10, TicketNo: "VISIBLE-PRIMARY-TEAM", Title: "主产品组待认领", Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TenantID: 1, CurrentTeamID: 11, TicketNo: "VISIBLE-EXTRA-TEAM", Title: "额外产品组待认领", Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TenantID: 1, CurrentTeamID: 12, TicketNo: "HIDDEN-OTHER-TEAM", Title: "其他产品组", Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
	}
	if err := db.Create(&tickets).Error; err != nil {
		t.Fatalf("create tickets: %v", err)
	}

	operator := &dto.AuthPrincipal{TenantID: 1, UserID: user.ID, Username: user.Username, Roles: []string{EnterpriseRoleEngineer}}
	result, err := EnterpriseTicketService.ListForOperator(1, EnterpriseTicketQuery{Page: 1, PageSize: 20}, operator)
	if err != nil {
		t.Fatalf("list tickets: %v", err)
	}
	visible := make(map[string]struct{}, len(result.Items))
	for _, item := range result.Items {
		visible[item.TicketNo] = struct{}{}
	}
	if result.Total != 2 {
		t.Fatalf("visible ticket total = %d, want 2, items=%+v", result.Total, result.Items)
	}
	if _, ok := visible["VISIBLE-PRIMARY-TEAM"]; !ok {
		t.Fatal("primary product team ticket should be visible")
	}
	if _, ok := visible["VISIBLE-EXTRA-TEAM"]; !ok {
		t.Fatal("extra product team ticket should be visible")
	}
	if _, ok := visible["HIDDEN-OTHER-TEAM"]; ok {
		t.Fatal("other product team ticket must stay hidden")
	}
	if err := TicketLifecycleService.Accept(tickets[1].ID, user.ID, operator); err != nil {
		t.Fatalf("engineer should accept ticket from extra product team: %v", err)
	}
	var accepted models.Ticket
	if err := db.First(&accepted, tickets[1].ID).Error; err != nil {
		t.Fatalf("reload accepted ticket: %v", err)
	}
	if accepted.Status != enums.TicketStatusAccepted || accepted.CurrentAssigneeID != user.ID || accepted.CurrentTeamID != 11 {
		t.Fatalf("unexpected accepted extra-team ticket: %+v", accepted)
	}
}

func TestEnterpriseTicketListAndSummaryFilterByRelatedContextInDatabase(t *testing.T) {
	db := setupEnterpriseTicketQueryTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	product := models.Product{
		TenantID: 1, Code: "HYP-9000", Name: "Hydraulic Press Alpha", Category: "presses", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	device := models.Device{
		TenantID: 1, ProductID: product.ID, DeviceNo: "DEV-NEEDLE-3001", SerialNo: "SN-NEEDLE-3001", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&device).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	customer := models.Customer{
		Name: "Acme Needle Works", PrimaryEmail: "ops@acme.example", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	for i := 0; i < 12; i++ {
		createdAt := now.Add(time.Duration(i) * time.Minute)
		ticket := models.Ticket{
			TenantID: 1, TicketNo: "FILLER-" + strconv.Itoa(i), Title: "ordinary queue item", Status: enums.TicketStatusPendingAcceptance, PriorityCode: "p2",
			AuditFields: models.AuditFields{CreatedAt: createdAt, UpdatedAt: createdAt},
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create filler ticket %d: %v", i, err)
		}
	}
	overdueAt := now.Add(-30 * time.Minute)
	matchingTicket := models.Ticket{
		TenantID: 1, TicketNo: "RELATION-MATCH-CRITICAL", Title: "controller vibration", Status: enums.TicketStatusPendingAcceptance, PriorityCode: "p0",
		ProductID: product.ID, DeviceID: device.ID, CustomerID: customer.ID, SLADueAt: &overdueAt,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour)},
	}
	if err := db.Create(&matchingTicket).Error; err != nil {
		t.Fatalf("create matching ticket: %v", err)
	}
	closedTicket := models.Ticket{
		TenantID: 1, TicketNo: "RELATION-MATCH-CLOSED", Title: "closed calibration", Status: enums.TicketStatusClosed, PriorityCode: "p0",
		ProductID: product.ID, CustomerID: customer.ID,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-3 * time.Hour), UpdatedAt: now.Add(-3 * time.Hour)},
	}
	if err := db.Create(&closedTicket).Error; err != nil {
		t.Fatalf("create closed ticket: %v", err)
	}

	result, err := EnterpriseTicketService.List(1, EnterpriseTicketQuery{
		Page: 1, PageSize: 5, Search: "Hydraulic Press Alpha", Priority: "critical",
	})
	if err != nil {
		t.Fatalf("list by product search: %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 || result.Items[0].TicketNo != matchingTicket.TicketNo {
		t.Fatalf("product related critical list = %+v, want only %s", result, matchingTicket.TicketNo)
	}
	deviceResult, err := EnterpriseTicketService.List(1, EnterpriseTicketQuery{
		Page: 1, PageSize: 5, Search: "DEV-NEEDLE-3001", Priority: "critical",
	})
	if err != nil {
		t.Fatalf("list by device search: %v", err)
	}
	if deviceResult.Total != 1 || len(deviceResult.Items) != 1 || deviceResult.Items[0].TicketNo != matchingTicket.TicketNo {
		t.Fatalf("device related critical list = %+v, want only %s", deviceResult, matchingTicket.TicketNo)
	}
	deviceIDResult, err := EnterpriseTicketService.List(1, EnterpriseTicketQuery{
		Page: 1, PageSize: 5, DeviceID: device.ID, Priority: "critical",
	})
	if err != nil {
		t.Fatalf("list by device id: %v", err)
	}
	if deviceIDResult.Total != 1 || len(deviceIDResult.Items) != 1 || deviceIDResult.Items[0].TicketNo != matchingTicket.TicketNo {
		t.Fatalf("device id related critical list = %+v, want only %s", deviceIDResult, matchingTicket.TicketNo)
	}
	lowResult, err := EnterpriseTicketService.List(1, EnterpriseTicketQuery{
		Page: 1, PageSize: 5, Search: "Acme Needle", Priority: "low",
	})
	if err != nil {
		t.Fatalf("list low priority by customer search: %v", err)
	}
	if lowResult.Total != 1 || len(lowResult.Items) != 1 || lowResult.Items[0].TicketNo != closedTicket.TicketNo {
		t.Fatalf("customer related low list = %+v, want only completed %s", lowResult, closedTicket.TicketNo)
	}

	summary, err := EnterpriseTicketService.Summary(1, EnterpriseTicketQuery{Search: "Acme Needle"})
	if err != nil {
		t.Fatalf("summary by customer search: %v", err)
	}
	if summary.Total != 2 || summary.Pending != 1 || summary.Done != 1 || summary.SLARisk != 1 || summary.Urgent != 1 {
		t.Fatalf("summary = %+v, want total=2 pending=1 done=1 slaRisk=1 urgent=1", summary)
	}
	deviceSummary, err := EnterpriseTicketService.Summary(1, EnterpriseTicketQuery{DeviceID: device.ID})
	if err != nil {
		t.Fatalf("summary by device id: %v", err)
	}
	if deviceSummary.Total != 1 || deviceSummary.Pending != 1 || deviceSummary.Done != 0 || deviceSummary.SLARisk != 1 || deviceSummary.Urgent != 1 {
		t.Fatalf("device summary = %+v, want total=1 pending=1 done=0 slaRisk=1 urgent=1", deviceSummary)
	}
}

func setupEnterpriseTicketQueryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Product{}, &models.Device{}, &models.Customer{}, &models.CustomerRegistrationGrant{}, &models.Ticket{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}
