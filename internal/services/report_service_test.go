package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func setupReportServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(
		&models.User{},
		&models.Product{},
		&models.ProductServiceProfile{},
		&models.ProductKnowledgeBinding{},
		&models.ProductKnowledgeLink{},
		&models.Device{},
		&models.Ticket{},
		&models.DiagnosisSession{},
		&models.MeetingRoomJitsi{},
		&models.MeetingParticipant{},
		&models.PartnerCompany{},
		&models.TicketSupplierCollaboration{},
		&models.TicketFeedback{},
		&models.KnowledgeDocument{},
		&models.KnowledgeFAQ{},
		&models.KnowledgeRetrieveLog{},
		&models.UsageEvent{},
		&models.TicketServiceOutcomeFact{},
	); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	return db
}

func TestReportServiceOverviewCountsCurrentAndLegacyTicketStates(t *testing.T) {
	db := setupReportServiceTestDB(t)
	now := time.Now()
	overdue := now.Add(-time.Hour)
	statuses := []enums.TicketStatus{
		enums.TicketStatusPendingDispatch, enums.TicketStatusPendingAssigneeAccept,
		enums.TicketStatusPending, enums.TicketStatusAssigned, enums.TicketStatusReopened,
		enums.TicketStatusProcessing, enums.TicketStatusVideoSupport,
		enums.TicketStatusSupplierSupport, enums.TicketStatusInProgress,
		enums.TicketStatusPendingCustomerConfirm,
		enums.TicketStatusClosed, enums.TicketStatusCancelled, enums.TicketStatusDone,
	}
	for i, status := range statuses {
		ticket := models.Ticket{
			TenantID: 91, TicketNo: fmt.Sprintf("STATUS-%d", i), Title: string(status),
			Status: status, SLADueAt: &overdue,
			AuditFields: models.AuditFields{CreatedAt: now.Add(time.Duration(i) * time.Second), UpdatedAt: now},
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create %s ticket: %v", status, err)
		}
	}
	foreign := models.Ticket{TenantID: 92, TicketNo: "OTHER-TENANT", Status: enums.TicketStatusProcessing, SLADueAt: &overdue}
	if err := db.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	overview, err := ReportService.GetDashboardOverview(nil, "91")
	if err != nil {
		t.Fatal(err)
	}
	if overview.TotalTickets != 13 || overview.PendingTickets != 5 || overview.InProgressTickets != 5 {
		t.Fatalf("ticket counts: total=%d pending=%d processing=%d; want 13, 5, 5", overview.TotalTickets, overview.PendingTickets, overview.InProgressTickets)
	}
	if overview.SLAAtRisk != 9 {
		t.Fatalf("SLA risk = %d, want 9; confirmation and terminal states must be excluded", overview.SLAAtRisk)
	}
	if len(overview.QueueTickets) != 6 {
		t.Fatalf("queue size = %d, want 6", len(overview.QueueTickets))
	}
	for _, ticket := range overview.QueueTickets {
		switch ticket.Summary {
		case "pending_customer_confirm", "closed", "cancelled", "done":
			t.Fatalf("completed ticket leaked into action queue: %+v", ticket)
		}
	}
}

func TestReportServicePurchasingMetricsAreScopedAndExposeCoverage(t *testing.T) {
	db := setupReportServiceTestDB(t)
	periodStart := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, 8, 31, 23, 59, 59, 0, time.UTC)
	createdAt := periodStart.Add(24 * time.Hour)

	products := []models.Product{
		{ID: 901, TenantID: 91, Code: "P-901", Name: "主产品", Status: enums.StatusOk},
		{ID: 902, TenantID: 91, Code: "P-902", Name: "同租户其他产品", Status: enums.StatusOk},
		{ID: 903, TenantID: 92, Code: "P-903", Name: "其他租户产品", Status: enums.StatusOk},
	}
	if err := db.Create(&products).Error; err != nil {
		t.Fatalf("create products: %v", err)
	}
	tickets := []models.Ticket{
		{TenantID: 91, ProductID: 901, TicketNo: "METRIC-1", Title: "有完整事实", Status: enums.TicketStatusClosed, AuditFields: models.AuditFields{CreatedAt: createdAt, UpdatedAt: createdAt}},
		{TenantID: 91, ProductID: 901, TicketNo: "METRIC-2", Title: "部分事实未知", Status: enums.TicketStatusClosed, AuditFields: models.AuditFields{CreatedAt: createdAt.Add(time.Hour), UpdatedAt: createdAt.Add(time.Hour)}},
		{TenantID: 91, ProductID: 901, TicketNo: "METRIC-3", Title: "尚未回填事实", Status: enums.TicketStatusClosed, AuditFields: models.AuditFields{CreatedAt: createdAt.Add(2 * time.Hour), UpdatedAt: createdAt.Add(2 * time.Hour)}},
		{TenantID: 91, ProductID: 901, TicketNo: "METRIC-OPEN", Title: "处理中不进入结果覆盖率分母", Status: enums.TicketStatusInProgress, AuditFields: models.AuditFields{CreatedAt: createdAt.Add(150 * time.Minute), UpdatedAt: createdAt.Add(150 * time.Minute)}},
		{TenantID: 91, ProductID: 902, TicketNo: "METRIC-4", Title: "其他产品事实", Status: enums.TicketStatusClosed, AuditFields: models.AuditFields{CreatedAt: createdAt.Add(3 * time.Hour), UpdatedAt: createdAt.Add(3 * time.Hour)}},
		{TenantID: 91, ProductID: 901, TicketNo: "METRIC-CANCELLED", Title: "取消工单不进覆盖率", Status: enums.TicketStatusCancelled, AuditFields: models.AuditFields{CreatedAt: createdAt.Add(4 * time.Hour), UpdatedAt: createdAt.Add(4 * time.Hour)}},
		{TenantID: 92, ProductID: 903, TicketNo: "METRIC-OTHER-TENANT", Title: "其他租户事实", Status: enums.TicketStatusClosed, AuditFields: models.AuditFields{CreatedAt: createdAt, UpdatedAt: createdAt}},
	}
	if err := db.Create(&tickets).Error; err != nil {
		t.Fatalf("create tickets: %v", err)
	}
	facts := []models.TicketServiceOutcomeFact{
		{
			TenantID: 91, TicketID: tickets[0].ID, ProductID: 901, MetricVersion: models.ServiceOutcomeMetricVersionV1,
			TicketStatus: string(enums.TicketStatusClosed), TicketCreatedAt: tickets[0].CreatedAt,
			ExpertInterventionKnown: true, ExpertIntervened: true,
			AvoidedOnsiteVisitKnown: true, AvoidedOnsiteVisit: true,
			FirstTimeFixKnown: true, FirstTimeFixEligible: true, FirstTimeFix: true,
			DowntimeKnown: true, DowntimeMinutes: 120,
			KnowledgeReuseKnown: true, KnowledgeReused: true,
			ResolutionKnown: true, RemoteResolved: true,
			SourceMaxUpdatedAt: createdAt, BuiltAt: createdAt, EvidenceJSON: "{}",
			AuditFields: models.AuditFields{CreatedAt: createdAt, UpdatedAt: createdAt},
		},
		{
			TenantID: 91, TicketID: tickets[1].ID, ProductID: 901, MetricVersion: models.ServiceOutcomeMetricVersionV1,
			TicketStatus: string(enums.TicketStatusClosed), TicketCreatedAt: tickets[1].CreatedAt,
			ExpertInterventionKnown: true, ExpertIntervened: false,
			FirstTimeFixKnown: true, FirstTimeFixEligible: true, FirstTimeFix: false,
			KnowledgeReuseKnown: false,
			ResolutionKnown:     true, RemoteResolved: false,
			SourceMaxUpdatedAt: createdAt, BuiltAt: createdAt, EvidenceJSON: "{}",
			AuditFields: models.AuditFields{CreatedAt: createdAt, UpdatedAt: createdAt},
		},
		{
			TenantID: 91, TicketID: tickets[4].ID, ProductID: 902, MetricVersion: models.ServiceOutcomeMetricVersionV1,
			TicketStatus: string(enums.TicketStatusClosed), TicketCreatedAt: tickets[4].CreatedAt,
			ExpertInterventionKnown: false,
			AvoidedOnsiteVisitKnown: true, AvoidedOnsiteVisit: true,
			FirstTimeFixKnown: false,
			DowntimeKnown:     true, DowntimeMinutes: 60,
			KnowledgeReuseKnown: true, KnowledgeReused: false,
			ResolutionKnown: true, RemoteResolved: true,
			SourceMaxUpdatedAt: createdAt, BuiltAt: createdAt, EvidenceJSON: "{}",
			AuditFields: models.AuditFields{CreatedAt: createdAt, UpdatedAt: createdAt},
		},
		{
			TenantID: 92, TicketID: tickets[6].ID, ProductID: 903, MetricVersion: models.ServiceOutcomeMetricVersionV1,
			TicketStatus: string(enums.TicketStatusClosed), TicketCreatedAt: tickets[6].CreatedAt,
			ExpertInterventionKnown: true, ExpertIntervened: true,
			AvoidedOnsiteVisitKnown: true, AvoidedOnsiteVisit: true,
			FirstTimeFixKnown: true, FirstTimeFixEligible: true, FirstTimeFix: true,
			DowntimeKnown: true, DowntimeMinutes: 999,
			KnowledgeReuseKnown: true, KnowledgeReused: true,
			ResolutionKnown: true, RemoteResolved: true,
			SourceMaxUpdatedAt: createdAt, BuiltAt: createdAt, EvidenceJSON: "{}",
			AuditFields: models.AuditFields{CreatedAt: createdAt, UpdatedAt: createdAt},
		},
	}
	if err := db.Create(&facts).Error; err != nil {
		t.Fatalf("create outcome facts: %v", err)
	}

	productReport, err := ReportService.GetProductReport(nil, "91", "901", periodStart, periodEnd)
	if err != nil {
		t.Fatalf("GetProductReport() error = %v", err)
	}
	if !reportFloatMetricEquals(productReport.ExpertInterventionRate, 50) || !reportIntMetricEquals(productReport.AvoidedTrips, 1) ||
		!reportFloatMetricEquals(productReport.FirstTimeFixRate, 50) || !reportFloatMetricEquals(productReport.AvgDowntimeMinutes, 120) ||
		!reportFloatMetricEquals(productReport.KnowledgeReuseRate, 100) || !reportFloatMetricEquals(productReport.RemoteResolutionRate, 50) ||
		productReport.OutcomeMetricSampleSize != 2 || !reportFloatMetricEquals(productReport.OutcomeMetricCoverageRate, 66.67) {
		t.Fatalf("unexpected product purchasing metrics: %+v", productReport)
	}

	overview, err := ReportService.GetDashboardOverview(nil, "91")
	if err != nil {
		t.Fatalf("GetDashboardOverview() error = %v", err)
	}
	if !reportFloatMetricEquals(overview.ExpertInterventionRate, 50) || !reportIntMetricEquals(overview.AvoidedTrips, 2) ||
		!reportFloatMetricEquals(overview.FirstTimeFixRate, 50) || !reportFloatMetricEquals(overview.AvgDowntimeMinutes, 90) ||
		!reportFloatMetricEquals(overview.KnowledgeReuseRate, 50) || !reportFloatMetricEquals(overview.RemoteResolutionRate, 66.67) ||
		overview.OutcomeMetricSampleSize != 3 || !reportFloatMetricEquals(overview.OutcomeMetricCoverageRate, 75) {
		t.Fatalf("unexpected tenant purchasing metrics: %+v", overview)
	}
}

func TestReportPurchasingMetricJSONDistinguishesUnknownFromZero(t *testing.T) {
	product := &ProductReport{ProductID: "1", ProductName: "测试产品"}
	overview := &DashboardOverview{}
	applyPurchasingMetricsToProductReport(product, purchasingMetricAggregate{})
	applyPurchasingMetricsToDashboardOverview(overview, purchasingMetricAggregate{})

	assertReportMetricJSON(t, product, true)
	assertReportMetricJSON(t, overview, true)

	knownZero := purchasingMetricAggregate{
		EligibleTickets:      1,
		ExpertKnown:          1,
		AvoidedTripsKnown:    1,
		FirstTimeFixEligible: 1,
		DowntimeKnown:        1,
		KnowledgeKnown:       1,
		ResolutionKnown:      1,
	}
	applyPurchasingMetricsToProductReport(product, knownZero)
	applyPurchasingMetricsToDashboardOverview(overview, knownZero)
	assertReportMetricJSON(t, product, false)
	assertReportMetricJSON(t, overview, false)
}

func assertReportMetricJSON(t *testing.T, value any, wantNull bool) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	for _, key := range []string{
		"expert_intervention_rate", "avoided_trips", "first_time_fix_rate", "avg_downtime_minutes",
		"knowledge_reuse_rate", "remote_resolution_rate", "outcome_metric_coverage_rate",
	} {
		if wantNull && payload[key] != nil {
			t.Fatalf("%s = %#v, want null in %s", key, payload[key], string(raw))
		}
		if !wantNull && payload[key] != float64(0) {
			t.Fatalf("%s = %#v, want numeric zero in %s", key, payload[key], string(raw))
		}
	}
	if _, exists := payload["ProductID"]; exists {
		t.Fatalf("ProductReport leaked PascalCase key: %s", string(raw))
	}
	if _, isProduct := value.(*ProductReport); isProduct {
		for _, key := range []string{"product_id", "product_name", "total_tickets", "ai_resolve_rate", "knowledge_hit_rate", "estimated_cost"} {
			if _, exists := payload[key]; !exists {
				t.Fatalf("ProductReport missing snake_case key %q: %s", key, string(raw))
			}
		}
	}
}

func reportFloatMetricEquals(actual *float64, expected float64) bool {
	return actual != nil && *actual == expected
}

func reportIntMetricEquals(actual *int64, expected int64) bool {
	return actual != nil && *actual == expected
}

func TestReportServiceTeamAndMeetingMetricsUseProductionTables(t *testing.T) {
	db := setupReportServiceTestDB(t)
	periodStart := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.Add(24 * time.Hour)
	createdAt := periodStart.Add(time.Hour)
	handledAt1 := createdAt.Add(10 * time.Minute)
	handledAt2 := createdAt.Add(20 * time.Minute)
	resolvedAt := createdAt.Add(2 * time.Hour)
	user := models.User{Username: "report-engineer", Nickname: "报表工程师", Status: enums.StatusOk}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create engineer: %v", err)
	}
	tickets := []models.Ticket{
		{TenantID: 91, ProductID: 901, TicketNo: "REPORT-1", Title: "已解决", Status: enums.TicketStatusClosed, CurrentAssigneeID: user.ID, HandledAt: &handledAt1, ResolvedAt: &resolvedAt, AuditFields: models.AuditFields{CreatedAt: createdAt, UpdatedAt: resolvedAt}},
		{TenantID: 91, ProductID: 901, TicketNo: "REPORT-2", Title: "处理中", Status: enums.TicketStatusProcessing, CurrentAssigneeID: user.ID, HandledAt: &handledAt2, AuditFields: models.AuditFields{CreatedAt: createdAt, UpdatedAt: handledAt2}},
	}
	if err := db.Create(&tickets).Error; err != nil {
		t.Fatalf("create tickets: %v", err)
	}
	feedbackItems := []models.TicketFeedback{
		{TenantID: 91, TicketID: tickets[0].ID, Rating: 3, Status: "submitted", SubmittedAt: resolvedAt.Add(time.Minute)},
		{TenantID: 91, TicketID: tickets[0].ID, Rating: 5, Status: "submitted", SubmittedAt: resolvedAt.Add(2 * time.Minute)},
		{TenantID: 91, TicketID: tickets[1].ID, Rating: 1, Status: "draft", SubmittedAt: resolvedAt.Add(3 * time.Minute)},
		{TenantID: 92, TicketID: tickets[0].ID, Rating: 1, Status: "submitted", SubmittedAt: resolvedAt.Add(4 * time.Minute)},
	}
	if err := db.Create(&feedbackItems).Error; err != nil {
		t.Fatalf("create feedback: %v", err)
	}
	meeting := models.MeetingRoomJitsi{
		ID: "report-meeting", TenantID: 91, TicketID: int642Str(tickets[0].ID), RoomName: "report-room", Status: "ended",
		CreatedBy: int642Str(user.ID), StartedAt: &handledAt1, EndedAt: &resolvedAt, BaseModel: models.BaseModel{CreatedAt: handledAt1, UpdatedAt: resolvedAt},
	}
	if err := db.Create(&meeting).Error; err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	joinedAt := handledAt1
	if err := db.Create(&models.MeetingParticipant{
		ID: "report-participant", MeetingID: meeting.ID, UserID: int642Str(user.ID), UserType: "enterprise",
		ParticipantName: user.Nickname, Role: "moderator", JoinedAt: &joinedAt, Duration: 1800,
		BaseModel: models.BaseModel{CreatedAt: joinedAt, UpdatedAt: joinedAt},
	}).Error; err != nil {
		t.Fatalf("create participant: %v", err)
	}

	performance, err := ReportService.GetTeamPerformance(nil, "91", periodStart, periodEnd)
	if err != nil {
		t.Fatalf("GetTeamPerformance() error = %v", err)
	}
	if len(performance) != 1 {
		t.Fatalf("performance = %+v", performance)
	}
	got := performance[0]
	if got.AgentName != user.Nickname || got.TicketsAssigned != 2 || got.TicketsResolved != 1 || got.AvgResponseMin != 15 || got.AvgHandleMin != 120 || got.Satisfaction != 5 || got.MeetingsHeld != 1 {
		t.Fatalf("unexpected team performance: %+v", got)
	}
	if gotMeetings := ReportService.countProductMeetings(db, "91", "901", periodStart, periodEnd); gotMeetings != 1 {
		t.Fatalf("product meeting count = %d, want 1", gotMeetings)
	}
	if gotMinutes := ReportService.sumMeetingMinutes(db, "91", "901", periodStart, periodEnd); gotMinutes != 30 {
		t.Fatalf("product meeting minutes = %d, want 30", gotMinutes)
	}
}

func TestReportServiceAggregatesSupplierPerformanceByTicket(t *testing.T) {
	db := setupReportServiceTestDB(t)
	const tenantID int64 = 81
	periodStart := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, 7, 31, 23, 59, 59, 0, time.UTC)
	now := time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC)

	suppliers := []models.PartnerCompany{
		{TenantID: tenantID, PartnerNo: "SUP-A", Name: "电源模块供应商", Status: enums.StatusOk},
		{TenantID: tenantID, PartnerNo: "SUP-B", Name: "传感器供应商", Status: enums.StatusOk},
		{TenantID: 99, PartnerNo: "SUP-OTHER", Name: "其他租户供应商", Status: enums.StatusOk},
	}
	if err := db.Create(&suppliers).Error; err != nil {
		t.Fatalf("create suppliers: %v", err)
	}
	accepted20 := now.Add(20 * time.Minute)
	accepted40 := now.Add(40 * time.Minute)
	accepted100 := now.Add(24*time.Hour + 100*time.Minute)
	accepted15 := now.Add(48*time.Hour + 15*time.Minute)
	resolvedAt := now.Add(72 * time.Hour)
	collaborations := []models.TicketSupplierCollaboration{
		{TenantID: tenantID, TicketID: 101, ProductModuleID: 1, PartnerCompanyID: suppliers[0].ID, Status: SupplierCollaborationResolved, InvitedAt: now, AcceptedAt: &accepted20, ResolvedAt: &resolvedAt, RecordStatus: enums.StatusOk},
		{TenantID: tenantID, TicketID: 101, ProductModuleID: 2, PartnerCompanyID: suppliers[0].ID, Status: SupplierCollaborationProcessing, InvitedAt: now.Add(5 * time.Minute), AcceptedAt: &accepted40, RecordStatus: enums.StatusOk},
		{TenantID: tenantID, TicketID: 102, ProductModuleID: 3, PartnerCompanyID: suppliers[0].ID, Status: SupplierCollaborationResolved, InvitedAt: now.Add(24 * time.Hour), AcceptedAt: &accepted100, ResolvedAt: &resolvedAt, RecordStatus: enums.StatusOk},
		{TenantID: tenantID, TicketID: 103, ProductModuleID: 4, PartnerCompanyID: suppliers[1].ID, Status: SupplierCollaborationResolved, InvitedAt: now.Add(48 * time.Hour), AcceptedAt: &accepted15, ResolvedAt: &resolvedAt, RecordStatus: enums.StatusOk},
		{TenantID: tenantID, TicketID: 104, ProductModuleID: 5, PartnerCompanyID: suppliers[0].ID, Status: SupplierCollaborationProcessing, InvitedAt: periodStart.Add(-time.Hour), RecordStatus: enums.StatusOk},
		{TenantID: tenantID, TicketID: 105, ProductModuleID: 6, PartnerCompanyID: suppliers[0].ID, Status: SupplierCollaborationProcessing, InvitedAt: now, RecordStatus: enums.StatusDeleted},
		{TenantID: 99, TicketID: 106, ProductModuleID: 7, PartnerCompanyID: suppliers[2].ID, Status: SupplierCollaborationProcessing, InvitedAt: now, RecordStatus: enums.StatusOk},
	}
	if err := db.Create(&collaborations).Error; err != nil {
		t.Fatalf("create collaborations: %v", err)
	}
	feedback := []models.TicketFeedback{
		{TenantID: tenantID, TicketID: 101, Rating: 2, Status: "submitted", SubmittedAt: now.Add(4 * time.Hour)},
		{TenantID: tenantID, TicketID: 101, Rating: 4, Status: "submitted", SubmittedAt: now.Add(5 * time.Hour)},
		{TenantID: tenantID, TicketID: 102, Rating: 5, Status: "submitted", SubmittedAt: now.Add(48 * time.Hour)},
		{TenantID: tenantID, TicketID: 103, Rating: 3, Status: "submitted", SubmittedAt: now.Add(72 * time.Hour)},
		{TenantID: 99, TicketID: 101, Rating: 1, Status: "submitted", SubmittedAt: now.Add(96 * time.Hour)},
	}
	if err := db.Create(&feedback).Error; err != nil {
		t.Fatalf("create feedback: %v", err)
	}

	result, err := ReportService.GetSupplierPerformance(tenantID, periodStart, periodEnd)
	if err != nil {
		t.Fatalf("GetSupplierPerformance() error = %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("supplier performance len = %d, want 2: %+v", len(result), result)
	}
	if result[0].SupplierID != suppliers[1].ID || result[0].TicketsAssigned != 1 || result[0].TicketsCompleted != 1 || result[0].CompletionRate != 100 || result[0].AvgResponseMinutes != 15 || result[0].AverageRating != 3 {
		t.Fatalf("supplier B performance = %+v", result[0])
	}
	if result[1].SupplierID != suppliers[0].ID || result[1].TicketsAssigned != 2 || result[1].TicketsProcessing != 1 || result[1].TicketsCompleted != 1 || result[1].TicketsResponded != 2 {
		t.Fatalf("supplier A ticket aggregation = %+v", result[1])
	}
	if result[1].AvgResponseMinutes != 60 || result[1].CompletionRate != 50 || result[1].AverageRating != 4.5 || result[1].RatingCount != 2 {
		t.Fatalf("supplier A performance metrics = %+v", result[1])
	}
}

func TestReportServiceSupplierPerformanceReturnsEmptyArray(t *testing.T) {
	setupReportServiceTestDB(t)
	result, err := ReportService.GetSupplierPerformance(81, time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatalf("GetSupplierPerformance() error = %v", err)
	}
	if result == nil || len(result) != 0 {
		t.Fatalf("empty supplier performance = %#v, want non-nil empty slice", result)
	}
}
