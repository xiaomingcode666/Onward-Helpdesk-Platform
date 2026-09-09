package services

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
)

func TestPlatformConsoleAggregatesLivePlatformData(t *testing.T) {
	db, operator := setupPlatformIAMTestDB(t)
	if err := db.AutoMigrate(
		&models.TenantPlanQuota{},
		&models.Product{},
		&models.Device{},
		&models.Ticket{},
		&models.KnowledgeDocument{},
		&models.KnowledgeIndexSyncTask{},
		&models.MeetingRoomJitsi{},
		&models.MeetingParticipant{},
		&models.ProductAIUsageEvent{},
		&models.ProductAIUsageDaily{},
		&models.ProductAIUsageCredential{},
		&models.Sub2APITenantAccount{},
		&models.AccessConnector{},
		&models.AccessCallLog{},
		&models.SyncRun{},
		&models.Notification{},
		&models.DeliveryLog{},
	); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	now := time.Now().Truncate(time.Second)
	plan, err := PlatformIAMService.SavePlan(request.PlatformPlanSaveRequest{
		Code:              "pro",
		Name:              "Pro",
		PlanType:          "subscription",
		QuotaTemplateJSON: `{"ai_requests":{"value":2,"unit":"requests","period":"monthly"},"ai_tokens":{"value":100,"unit":"tokens","period":"monthly"},"devices":{"value":3,"unit":"devices"},"meeting_minutes":{"value":10,"unit":"minutes"}}`,
		OverageStrategy:   "soft_alert",
	}, operator)
	if err != nil {
		t.Fatalf("SavePlan() error = %v", err)
	}
	tenant, err := PlatformIAMService.CreateTenant(request.PlatformTenantCreateRequest{
		Name: "Atlas Machines", PlanID: plan.ID,
	}, operator)
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}

	product := &models.Product{
		TenantID: tenant.ID, Code: "atlas", Name: "Atlas Pump", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product error = %v", err)
	}
	devices := []models.Device{
		{TenantID: tenant.ID, ProductID: product.ID, DeviceNo: "D-1", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TenantID: tenant.ID, ProductID: product.ID, DeviceNo: "D-2", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
	}
	if err := db.Create(&devices).Error; err != nil {
		t.Fatalf("create devices error = %v", err)
	}
	slaDueAt := now.Add(-time.Hour)
	if err := db.Create(&models.Ticket{
		TenantID: tenant.ID, ProductID: product.ID, TicketNo: "TK-1", Title: "Pump stopped",
		Status: enums.TicketStatusPending, SLADueAt: &slaDueAt,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create ticket error = %v", err)
	}
	if err := db.Create(&models.KnowledgeDocument{
		TenantID: tenant.ID, KnowledgeBaseID: 1, Title: "Pump guide",
		Status: enums.StatusOk, ReviewStatus: "draft", IndexStatus: enums.KnowledgeDocumentIndexStatusPending,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create knowledge document error = %v", err)
	}

	usageEvents := []models.ProductAIUsageEvent{
		{TenantID: tenant.ID, ProductID: product.ID, RequestID: "req-1", InputTokens: 20, OutputTokens: 30, CostAmount: 0.01, OccurredAt: now.Add(-2 * time.Hour), CreatedAt: now},
		{TenantID: tenant.ID, ProductID: product.ID, RequestID: "req-2", InputTokens: 20, OutputTokens: 30, CostAmount: 0.02, OccurredAt: now.Add(-time.Hour), CreatedAt: now},
	}
	if err := db.Create(&usageEvents).Error; err != nil {
		t.Fatalf("create usage events error = %v", err)
	}
	if err := db.Create(&models.ProductAIUsageDaily{
		TenantID: tenant.ID, ProductID: product.ID, UsageDate: now.Format("2006-01-02"),
		RequestCount: 2, InputTokens: 40, OutputTokens: 60, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create usage daily error = %v", err)
	}

	meeting := &models.MeetingRoomJitsi{
		ID: "meeting-1", TenantID: tenant.ID, TicketID: "1", RoomName: "atlas-room", Status: "active",
		BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(meeting).Error; err != nil {
		t.Fatalf("create meeting error = %v", err)
	}
	if err := db.Create(&models.MeetingParticipant{
		ID: "participant-1", MeetingID: meeting.ID, Duration: 120, JoinedAt: &now,
		BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create meeting participant error = %v", err)
	}
	connector := &models.AccessConnector{
		TenantID: tenant.ID, Name: "ERP", Status: "active", HealthStatus: "degraded", CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(connector).Error; err != nil {
		t.Fatalf("create connector error = %v", err)
	}
	if err := db.Create(&models.AccessCallLog{
		ID: "call-1", TenantID: tenant.ID, ConnectorID: connector.ID, ResponseCode: 502, ErrorMessage: "timeout", CreatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create access call log error = %v", err)
	}
	if err := db.Create(&models.SyncRun{
		TenantID: tenant.ID, SyncType: "usage_aggregation", Status: models.SyncStatusFailed,
		StartedAt: now.Add(-time.Hour), RecordsProcessed: 12, ErrorMessage: "upstream timeout",
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create sync run error = %v", err)
	}
	finishedAt := now.Add(-10 * time.Minute)
	if err := db.Create(&models.KnowledgeIndexSyncTask{
		TenantID: tenant.ID, IdempotencyKey: "idx-1", SubjectType: "knowledge_document", SubjectID: 1,
		KnowledgeBaseID: 1, ProductID: product.ID, ProviderType: "ragflow", Action: "upsert",
		Status: "failed", RetryCount: 1, ErrorSummary: "ragflow timeout", FinishedAt: &finishedAt,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create knowledge index sync task error = %v", err)
	}
	staleSyncAt := now.Add(-25 * time.Hour)
	if err := db.Create(&models.Sub2APITenantAccount{
		TenantID: tenant.ID, Sub2APIAccountID: "sub2api-1", AccountName: "Atlas AI", AccountStatus: "degraded",
		LastSyncedAt: &staleSyncAt, Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create sub2api tenant account error = %v", err)
	}
	if err := db.Create(&models.ProductAIUsageCredential{
		TenantID: tenant.ID, ProductID: product.ID, Sub2APIAccount: "sub2api-1", APIKeyFingerprint: "fp-1",
		LastSyncedAt: &staleSyncAt, Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create product ai usage credential error = %v", err)
	}
	notification := &models.Notification{
		TenantID: tenant.ID, RecipientUserID: 1001, Title: "Sync alert", DeliveryStatus: "pending",
		Status: int(enums.StatusOk), CreatedAt: now,
	}
	if err := db.Create(notification).Error; err != nil {
		t.Fatalf("create notification error = %v", err)
	}
	deliveredAt := now.Add(-5 * time.Minute)
	if err := db.Create(&models.DeliveryLog{
		NotificationID: notification.ID, Channel: "email", Status: "failed", CreatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create failed delivery log error = %v", err)
	}
	if err := db.Create(&models.DeliveryLog{
		NotificationID: notification.ID, Channel: "in_app", Status: "delivered", DeliveredAt: &deliveredAt, CreatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create delivered log error = %v", err)
	}
	if err := db.Create(&models.AuthAuditLog{
		TenantID: tenant.ID, DomainType: "platform", ActorUserID: operator.UserID,
		TargetType: "tenant", TargetID: "tenant-1", Action: "tenant.updated",
		RiskLevel: "low", Status: "success", OccurredAt: now,
	}).Error; err != nil {
		t.Fatalf("create auth audit log error = %v", err)
	}

	overview, err := PlatformConsoleService.GetOverview(now)
	if err != nil {
		t.Fatalf("GetOverview() error = %v", err)
	}
	if overview.Counts.TenantTotal != 1 || overview.Counts.Products != 1 || overview.Counts.Devices != 2 {
		t.Fatalf("overview counts = tenants %d products %d devices %d", overview.Counts.TenantTotal, overview.Counts.Products, overview.Counts.Devices)
	}
	if overview.AIUsage.Requests != 2 || overview.AIUsage.Tokens != 100 {
		t.Fatalf("overview AI usage = requests %d tokens %d", overview.AIUsage.Requests, overview.AIUsage.Tokens)
	}
	if len(overview.TopTenants) != 1 || overview.TopTenants[0].TenantID != tenant.ID {
		t.Fatalf("top tenants = %#v", overview.TopTenants)
	}

	metrics, err := PlatformConsoleService.GetOverviewMetrics(now)
	if err != nil {
		t.Fatalf("GetOverviewMetrics() error = %v", err)
	}
	if metrics.Counts.TenantTotal != 1 || metrics.AIUsage.Requests != 2 {
		t.Fatalf("overview metrics = %#v", metrics)
	}
	topTenants, err := PlatformConsoleService.GetOverviewTopTenants(now, 50)
	if err != nil {
		t.Fatalf("GetOverviewTopTenants() error = %v", err)
	}
	if len(topTenants.TopTenants) != 1 || topTenants.TopUsage[tenant.ID].Requests != 2 {
		t.Fatalf("overview top tenants = %#v", topTenants)
	}
	trend, err := PlatformConsoleService.GetOverviewUsageTrend(now, 40)
	if err != nil {
		t.Fatalf("GetOverviewUsageTrend() error = %v", err)
	}
	if len(trend.DailyUsage) != 31 || trend.AIUsage.Tokens != 100 {
		t.Fatalf("overview usage trend = len %d usage %#v", len(trend.DailyUsage), trend.AIUsage)
	}
	events, err := PlatformConsoleService.GetOverviewRecentEvents(now, 0)
	if err != nil {
		t.Fatalf("GetOverviewRecentEvents() error = %v", err)
	}
	foundTenantEvent := false
	for _, event := range events.RecentEvents {
		if event.TenantName == tenant.Name && event.Action == "tenant.updated" {
			foundTenantEvent = true
			break
		}
	}
	if !foundTenantEvent {
		t.Fatalf("overview recent events = %#v", events.RecentEvents)
	}

	usage, err := PlatformConsoleService.GetUsageOverview(now)
	if err != nil {
		t.Fatalf("GetUsageOverview() error = %v", err)
	}
	if len(usage.Tenants) != 1 {
		t.Fatalf("usage tenants = %d, want 1", len(usage.Tenants))
	}
	tenantUsage := usage.Tenants[0]
	if tenantUsage.AIRequestLimit != 2 || tenantUsage.AITokenLimit != 100 || tenantUsage.DeviceLimit != 3 || tenantUsage.MeetingLimit != 10 {
		t.Fatalf("resolved quotas = %#v", tenantUsage)
	}
	if tenantUsage.MeetingMinutes != 2 || tenantUsage.RiskLevel != "over_limit" {
		t.Fatalf("tenant usage = minutes %d risk %q", tenantUsage.MeetingMinutes, tenantUsage.RiskLevel)
	}

	ops, err := PlatformConsoleService.GetOpsOverview(now)
	if err != nil {
		t.Fatalf("GetOpsOverview() error = %v", err)
	}
	if ops.Database == nil {
		t.Fatalf("database snapshot = nil, error %q", ops.DatabaseError)
	}
	if ops.Workloads.ActiveMeetings != 1 || ops.Workloads.OpenTickets != 1 || ops.Workloads.UnhealthyConnectors != 1 {
		t.Fatalf("workloads = %#v", ops.Workloads)
	}
	if ops.Workloads.LastUsageEventAt == nil || ops.Workloads.LastAggregateAt == nil || ops.Workloads.FailedSyncs24h != 1 {
		t.Fatalf("pipeline workload = %#v", ops.Workloads)
	}
	if len(ops.SyncRuns) != 1 || ops.SyncRuns[0].TenantName != tenant.Name {
		t.Fatalf("sync runs = %#v", ops.SyncRuns)
	}
	if ops.KnowledgeIndex == nil || ops.KnowledgeIndex.FailedTasks24h != 1 {
		t.Fatalf("knowledge index = %#v", ops.KnowledgeIndex)
	}
	if len(ops.KnowledgeTasks) != 1 || ops.KnowledgeTasks[0].TenantName != tenant.Name {
		t.Fatalf("knowledge tasks = %#v", ops.KnowledgeTasks)
	}
	if ops.Sub2API == nil || ops.Sub2API.TenantAccounts != 1 || ops.Sub2API.StaleCredentials24h != 1 {
		t.Fatalf("sub2api = %#v", ops.Sub2API)
	}
	if ops.Jitsi == nil || ops.Jitsi.Meetings24h != 1 || ops.Jitsi.OnlineParticipants != 1 ||
		ops.Jitsi.StaleParticipants != 0 || ops.Jitsi.StaleMeetings != 0 ||
		ops.Jitsi.HeartbeatStatus != "healthy" || ops.Jitsi.HeartbeatTimeoutSeconds != 180 || ops.Jitsi.LastHeartbeatAt == nil {
		t.Fatalf("jitsi = %#v", ops.Jitsi)
	}
	staleHeartbeatAt := now.Add(-meetingParticipantStaleAfter - time.Minute)
	if err := db.Model(&models.MeetingParticipant{}).Where("id = ?", "participant-1").UpdateColumn("updated_at", staleHeartbeatAt).Error; err != nil {
		t.Fatalf("age meeting participant heartbeat: %v", err)
	}
	if err := db.Model(&models.MeetingRoomJitsi{}).Where("id = ?", meeting.ID).UpdateColumn("updated_at", staleHeartbeatAt).Error; err != nil {
		t.Fatalf("age meeting heartbeat: %v", err)
	}
	staleOps, err := PlatformConsoleService.GetOpsOverview(now)
	if err != nil {
		t.Fatalf("GetOpsOverview() with stale meeting error = %v", err)
	}
	if staleOps.Jitsi == nil || staleOps.Jitsi.OnlineParticipants != 0 || staleOps.Jitsi.StaleParticipants != 1 ||
		staleOps.Jitsi.StaleMeetings != 1 || staleOps.Jitsi.HeartbeatStatus != "degraded" {
		t.Fatalf("stale jitsi heartbeat = %#v", staleOps.Jitsi)
	}
	if ops.Access == nil || ops.Access.TotalConnectors != 1 || ops.Access.FailedCalls24h != 1 {
		t.Fatalf("access = %#v", ops.Access)
	}
	if ops.Notifications == nil || ops.Notifications.PendingNotifications != 1 || ops.Notifications.FailedDeliveries24h != 1 || ops.Notifications.SentDeliveries24h != 1 {
		t.Fatalf("notifications = %#v", ops.Notifications)
	}
	if err := db.Model(&models.Tenant{}).Where("id = ?", tenant.ID).Updates(map[string]any{
		"country_region": "US", "default_locale": "en-US", "supported_locales_json": `["en-US","es-ES"]`,
		"timezone": "America/New_York", "supported_timezones_json": `["America/New_York","UTC"]`, "data_region": "us-east",
	}).Error; err != nil {
		t.Fatal(err)
	}
	globalization, err := PlatformConsoleService.GetGlobalization(now, 1, 1)
	if err != nil {
		t.Fatalf("GetGlobalization() error = %v", err)
	}
	if globalization.TenantPage.Paging.Total != 1 || len(globalization.TenantPage.Items) != 1 {
		t.Fatalf("globalization tenant page = %#v", globalization.TenantPage.Paging)
	}
	if globalization.ConfiguredRegionCount != 1 || globalization.LocaleCount != 2 || globalization.TimezoneCount != 2 || len(globalization.Regions) != 1 {
		t.Fatalf("globalization aggregate = %#v", globalization)
	}
	if globalization.Regions[0].DataRegion != "us-east" || globalization.Regions[0].TenantCount != 1 {
		t.Fatalf("globalization region = %#v", globalization.Regions[0])
	}
	if strings.Join(globalization.Regions[0].Locales, ",") != "en-US,es-ES" || strings.Join(globalization.Regions[0].Timezones, ",") != "America/New_York,UTC" {
		t.Fatalf("globalization locale/timezone coverage = %#v", globalization.Regions[0])
	}
}
