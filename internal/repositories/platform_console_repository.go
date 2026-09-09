package repositories

import (
	"database/sql"
	"fmt"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"gorm.io/gorm"
)

var PlatformConsoleRepository = &platformConsoleRepository{}

type platformConsoleRepository struct{}

type PlatformOverviewCounts struct {
	TenantTotal     int64
	TenantActive    int64
	TenantTrial     int64
	TenantFrozen    int64
	TenantExpiring  int64
	Products        int64
	Devices         int64
	Tickets         int64
	OpenTickets     int64
	TicketsToday    int64
	SLARiskTickets  int64
	KnowledgeDocs   int64
	IndexedDocs     int64
	PendingDocs     int64
	MonthlyMeetings int64
	ActiveMeetings  int64
}

type PlatformTenantSummaryRow struct {
	Total        int64
	Active       int64
	Trial        int64
	Frozen       int64
	ExpiringSoon int64
}

type PlatformGlobalizationRegionRow struct {
	DataRegion             string
	CountryRegion          string
	DefaultLocale          string
	SupportedLocalesJSON   string
	Timezone               string
	SupportedTimezonesJSON string
	TenantCount            int64
}

type PlatformAIUsageRow struct {
	TenantID      int64
	Requests      int64
	Tokens        int64
	CostAmount    float64
	LastMeteredAt *time.Time
}

type PlatformDailyUsageRow struct {
	Date       string
	Requests   int64
	Tokens     int64
	CostAmount float64
}

type PlatformTopTenantRow struct {
	TenantID   int64
	TenantName string
	Devices    int64
}

type PlatformRecentEventRow struct {
	ID         int64
	TenantID   int64
	TenantName string
	Action     string
	TargetType string
	RiskLevel  string
	Status     string
	OccurredAt time.Time
}

type PlatformUsageTenantBaseRow struct {
	TenantID   int64
	TenantName string
	PlanID     int64
	PlanName   string
}

type PlatformMeetingUsageRow struct {
	TenantID             int64
	TotalDurationSeconds int64
}

type platformAIUsageScanRow struct {
	TenantID      int64
	Requests      int64
	Tokens        int64
	CostAmount    float64
	LastMeteredAt sql.NullString
}

type platformTimeScanRow struct {
	Value sql.NullString
}

type PlatformWorkloadSnapshot struct {
	ActiveMeetings      int64
	OpenTickets         int64
	PendingKnowledge    int64
	UnhealthyConnectors int64
	LastUsageEventAt    *time.Time
	LastAggregateAt     *time.Time
	RunningSyncs        int64
	FailedSyncs24h      int64
}

type PlatformPipelineSnapshot struct {
	LastUsageEventAt *time.Time
	LastAggregateAt  *time.Time
	RunningSyncs     int64
	FailedSyncs24h   int64
}

type PlatformSyncRunRow struct {
	ID               int64
	TenantID         int64
	TenantName       string
	SyncType         string
	Status           string
	RecordsProcessed int64
	ErrorMessage     string
	StartedAt        time.Time
	EndedAt          *time.Time
}

type PlatformDatabaseSnapshot struct {
	LatencyMs int64
	Stats     sql.DBStats
}

type PlatformDatabaseStorageSnapshot struct {
	Engine     string
	SizeBytes  int64
	TableCount int64
	Tables     []PlatformDatabaseTableStorageRow
}

type PlatformDatabaseTableStorageRow struct {
	Name          string
	SizeBytes     int64
	EstimatedRows int64
}

type PlatformAssetStorageRow struct {
	Provider    string
	ObjectCount int64
	SizeBytes   int64
}

type PlatformKnowledgeIndexSnapshot struct {
	PendingTasks   int64
	RunningTasks   int64
	FailedTasks24h int64
	LastFinishedAt *time.Time
}

type PlatformKnowledgeIndexTaskRow struct {
	ID           int64
	TenantID     int64
	TenantName   string
	ProductID    int64
	ProductName  string
	SubjectType  string
	SubjectID    int64
	ProviderType string
	Action       string
	Status       string
	RetryCount   int
	ErrorSummary string
	UpdatedAt    time.Time
}

type PlatformSub2APISnapshot struct {
	TenantAccounts       int64
	HealthyAccounts      int64
	AttentionAccounts    int64
	ProductCredentials   int64
	StaleCredentials24h  int64
	LastUsageEventAt     *time.Time
	LastCredentialSyncAt *time.Time
}

type PlatformJitsiSnapshot struct {
	ActiveMeetings        int64
	Meetings24h           int64
	ParticipantMinutes24h int64
	OnlineParticipants    int64
	StaleParticipants     int64
	StaleMeetings         int64
	LastMeetingAt         *time.Time
	LastHeartbeatAt       *time.Time
}

type PlatformAccessSnapshot struct {
	TotalConnectors     int64
	ActiveConnectors    int64
	HealthyConnectors   int64
	DegradedConnectors  int64
	UnhealthyConnectors int64
	FailedCalls24h      int64
	LastTestedAt        *time.Time
}

type PlatformNotificationSnapshot struct {
	PendingNotifications int64
	UnreadNotifications  int64
	FailedDeliveries24h  int64
	SentDeliveries24h    int64
	LastDeliveredAt      *time.Time
}

func (r *platformConsoleRepository) GetOverviewCounts(db *gorm.DB, now time.Time) (*PlatformOverviewCounts, error) {
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	expiringAt := now.AddDate(0, 0, 30)
	ret := &PlatformOverviewCounts{}

	queries := []struct {
		query *gorm.DB
		out   *int64
	}{
		{db.Model(&models.Tenant{}).Where("status <> ?", enums.StatusDeleted), &ret.TenantTotal},
		{db.Model(&models.Tenant{}).Where("status = ?", enums.StatusOk), &ret.TenantActive},
		{db.Model(&models.Tenant{}).Where("status = ? AND trial_ends_at IS NOT NULL AND trial_ends_at >= ?", enums.StatusOk, now), &ret.TenantTrial},
		{db.Model(&models.Tenant{}).Where("status = ? AND decommissioned_at IS NULL", enums.StatusDisabled), &ret.TenantFrozen},
		{db.Model(&models.Tenant{}).Where("status = ? AND trial_ends_at BETWEEN ? AND ?", enums.StatusOk, now, expiringAt), &ret.TenantExpiring},
		{db.Model(&models.Product{}).Where("status <> ?", enums.StatusDeleted), &ret.Products},
		{db.Model(&models.Device{}).Where("status <> ?", enums.StatusDeleted), &ret.Devices},
		{db.Model(&models.Ticket{}), &ret.Tickets},
		{db.Model(&models.Ticket{}).Where("status NOT IN ?", terminalTicketStatuses()), &ret.OpenTickets},
		{db.Model(&models.Ticket{}).Where("created_at >= ?", startOfToday), &ret.TicketsToday},
		{db.Model(&models.Ticket{}).Where("sla_due_at IS NOT NULL AND sla_due_at < ? AND status NOT IN ?", now, terminalTicketStatuses()), &ret.SLARiskTickets},
		{db.Model(&models.KnowledgeDocument{}).Where("status <> ?", enums.StatusDeleted), &ret.KnowledgeDocs},
		{db.Model(&models.KnowledgeDocument{}).Where("status <> ? AND index_status = ?", enums.StatusDeleted, enums.KnowledgeDocumentIndexStatusIndexed), &ret.IndexedDocs},
		{db.Model(&models.KnowledgeDocument{}).Where("status <> ? AND (review_status IN ? OR index_status = ?)", enums.StatusDeleted, []string{"draft", "review"}, enums.KnowledgeDocumentIndexStatusPending), &ret.PendingDocs},
		{db.Model(&models.MeetingRoomJitsi{}).Where("created_at >= ?", startOfMonth), &ret.MonthlyMeetings},
		{db.Model(&models.MeetingRoomJitsi{}).Where("status = ?", "active"), &ret.ActiveMeetings},
	}
	for _, item := range queries {
		if err := item.query.Count(item.out).Error; err != nil {
			return nil, err
		}
	}
	return ret, nil
}

func (r *platformConsoleRepository) GetTenantSummary(db *gorm.DB, now time.Time) (*PlatformTenantSummaryRow, error) {
	ret := &PlatformTenantSummaryRow{}
	expiringAt := now.AddDate(0, 0, 30)
	queries := []struct {
		query *gorm.DB
		out   *int64
	}{
		{db.Model(&models.Tenant{}).Where("status <> ?", enums.StatusDeleted), &ret.Total},
		{db.Model(&models.Tenant{}).Where("status = ?", enums.StatusOk), &ret.Active},
		{db.Model(&models.Tenant{}).Where("status = ? AND trial_ends_at IS NOT NULL AND trial_ends_at >= ?", enums.StatusOk, now), &ret.Trial},
		{db.Model(&models.Tenant{}).Where("status = ? AND decommissioned_at IS NULL", enums.StatusDisabled), &ret.Frozen},
		{db.Model(&models.Tenant{}).Where("status = ? AND trial_ends_at BETWEEN ? AND ?", enums.StatusOk, now, expiringAt), &ret.ExpiringSoon},
	}
	for _, item := range queries {
		if err := item.query.Count(item.out).Error; err != nil {
			return nil, err
		}
	}
	return ret, nil
}

func (r *platformConsoleRepository) GetGlobalizationRegionRows(db *gorm.DB) ([]PlatformGlobalizationRegionRow, error) {
	rows := make([]PlatformGlobalizationRegionRow, 0)
	err := db.Model(&models.Tenant{}).
		Select(
			"COALESCE(data_region, '') AS data_region, COALESCE(country_region, '') AS country_region, "+
				"COALESCE(default_locale, '') AS default_locale, COALESCE(supported_locales_json, '[]') AS supported_locales_json, "+
				"COALESCE(timezone, '') AS timezone, COALESCE(supported_timezones_json, '[]') AS supported_timezones_json, COUNT(*) AS tenant_count",
		).
		Where("status <> ?", enums.StatusDeleted).
		Group("data_region, country_region, default_locale, supported_locales_json, timezone, supported_timezones_json").
		Scan(&rows).Error
	return rows, err
}

func (r *platformConsoleRepository) GetAIUsage(db *gorm.DB, start, end time.Time) (*PlatformAIUsageRow, error) {
	row := &platformAIUsageScanRow{}
	err := db.Model(&models.ProductAIUsageEvent{}).
		Select("COUNT(*) AS requests, COALESCE(SUM(input_tokens + output_tokens), 0) AS tokens, COALESCE(SUM(cost_amount), 0) AS cost_amount, CAST(MAX(occurred_at) AS TEXT) AS last_metered_at").
		Where("occurred_at >= ? AND occurred_at < ?", start, end).
		Scan(row).Error
	if err != nil {
		return nil, err
	}
	return &PlatformAIUsageRow{
		Requests: row.Requests, Tokens: row.Tokens, CostAmount: row.CostAmount,
		LastMeteredAt: parseDatabaseTime(row.LastMeteredAt),
	}, nil
}

func (r *platformConsoleRepository) GetAIUsageByTenantIDs(db *gorm.DB, tenantIDs []int64, start, end time.Time) (map[int64]PlatformAIUsageRow, error) {
	ret := make(map[int64]PlatformAIUsageRow, len(tenantIDs))
	if len(tenantIDs) == 0 {
		return ret, nil
	}
	rows := make([]platformAIUsageScanRow, 0)
	err := db.Model(&models.ProductAIUsageEvent{}).
		Select("tenant_id, COUNT(*) AS requests, COALESCE(SUM(input_tokens + output_tokens), 0) AS tokens, COALESCE(SUM(cost_amount), 0) AS cost_amount, CAST(MAX(occurred_at) AS TEXT) AS last_metered_at").
		Where("tenant_id IN ? AND occurred_at >= ? AND occurred_at < ?", tenantIDs, start, end).
		Group("tenant_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		ret[row.TenantID] = PlatformAIUsageRow{
			TenantID: row.TenantID, Requests: row.Requests, Tokens: row.Tokens,
			CostAmount: row.CostAmount, LastMeteredAt: parseDatabaseTime(row.LastMeteredAt),
		}
	}
	return ret, nil
}

func (r *platformConsoleRepository) GetAIUsageByAPIKeyID(db *gorm.DB, apiKeyID string, start, end time.Time) (*PlatformAIUsageRow, error) {
	row := &platformAIUsageScanRow{}
	err := db.Model(&models.ProductAIUsageEvent{}).
		Select("COUNT(*) AS requests, COALESCE(SUM(input_tokens + output_tokens), 0) AS tokens, COALESCE(SUM(cost_amount), 0) AS cost_amount, CAST(MAX(occurred_at) AS TEXT) AS last_metered_at").
		Where("api_key_id = ? AND occurred_at >= ? AND occurred_at < ?", apiKeyID, start, end).
		Scan(row).Error
	if err != nil {
		return nil, err
	}
	return &PlatformAIUsageRow{
		Requests: row.Requests, Tokens: row.Tokens, CostAmount: row.CostAmount,
		LastMeteredAt: parseDatabaseTime(row.LastMeteredAt),
	}, nil
}

func (r *platformConsoleRepository) GetDailyAIUsage(db *gorm.DB, start, end time.Time) ([]PlatformDailyUsageRow, error) {
	rows := make([]PlatformDailyUsageRow, 0)
	err := db.Model(&models.ProductAIUsageEvent{}).
		Select("CAST(DATE(occurred_at) AS TEXT) AS date, COUNT(*) AS requests, COALESCE(SUM(input_tokens + output_tokens), 0) AS tokens, COALESCE(SUM(cost_amount), 0) AS cost_amount").
		Where("occurred_at >= ? AND occurred_at < ?", start, end).
		Group("DATE(occurred_at)").
		Order("DATE(occurred_at) ASC").
		Scan(&rows).Error
	return rows, err
}

func (r *platformConsoleRepository) GetTopTenantsByDevices(db *gorm.DB, limit int) ([]PlatformTopTenantRow, error) {
	if limit <= 0 {
		limit = 5
	}
	tenantTable, err := modelTableName(db, &models.Tenant{})
	if err != nil {
		return nil, err
	}
	deviceTable, err := modelTableName(db, &models.Device{})
	if err != nil {
		return nil, err
	}
	rows := make([]PlatformTopTenantRow, 0)
	err = db.Table(tenantTable+" AS tenants").
		Select("tenants.id AS tenant_id, tenants.name AS tenant_name, COUNT(devices.id) AS devices").
		Joins("LEFT JOIN "+deviceTable+" AS devices ON devices.tenant_id = tenants.id AND devices.status <> ?", enums.StatusDeleted).
		Where("tenants.status <> ?", enums.StatusDeleted).
		Group("tenants.id, tenants.name").
		Order("devices DESC, tenants.id ASC").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
}

func (r *platformConsoleRepository) GetRecentEvents(db *gorm.DB, limit int) ([]PlatformRecentEventRow, error) {
	if limit <= 0 {
		limit = 8
	}
	auditTable, err := modelTableName(db, &models.AuthAuditLog{})
	if err != nil {
		return nil, err
	}
	tenantTable, err := modelTableName(db, &models.Tenant{})
	if err != nil {
		return nil, err
	}
	rows := make([]PlatformRecentEventRow, 0)
	err = db.Table(auditTable + " AS logs").
		Select("logs.id, logs.tenant_id, COALESCE(tenants.name, '') AS tenant_name, logs.action, logs.target_type, logs.risk_level, logs.status, logs.occurred_at").
		Joins("LEFT JOIN " + tenantTable + " AS tenants ON tenants.id = logs.tenant_id").
		Order("logs.occurred_at DESC, logs.id DESC").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
}

func (r *platformConsoleRepository) FindUsageTenantBases(db *gorm.DB, limit int) ([]PlatformUsageTenantBaseRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	tenantTable, err := modelTableName(db, &models.Tenant{})
	if err != nil {
		return nil, err
	}
	subscriptionTable, err := modelTableName(db, &models.TenantSubscription{})
	if err != nil {
		return nil, err
	}
	planTable, err := modelTableName(db, &models.TenantPlan{})
	if err != nil {
		return nil, err
	}
	rows := make([]PlatformUsageTenantBaseRow, 0)
	err = db.Table(tenantTable+" AS tenants").
		Select("tenants.id AS tenant_id, tenants.name AS tenant_name, COALESCE(subscriptions.plan_id, 0) AS plan_id, COALESCE(plans.name, '') AS plan_name").
		Joins("LEFT JOIN "+subscriptionTable+" AS subscriptions ON subscriptions.tenant_id = tenants.id AND subscriptions.status = ?", enums.StatusOk).
		Joins("LEFT JOIN "+planTable+" AS plans ON plans.id = subscriptions.plan_id").
		Where("tenants.status <> ?", enums.StatusDeleted).
		Order("tenants.updated_at DESC, tenants.id DESC").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
}

func (r *platformConsoleRepository) FindPlanQuotas(db *gorm.DB, planIDs []int64) (map[int64][]models.TenantPlanQuota, error) {
	ret := make(map[int64][]models.TenantPlanQuota, len(planIDs))
	if len(planIDs) == 0 {
		return ret, nil
	}
	items := make([]models.TenantPlanQuota, 0)
	err := db.Where("plan_id IN ? AND status = ?", planIDs, enums.StatusOk).
		Order("plan_id ASC, id ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		ret[item.PlanID] = append(ret[item.PlanID], item)
	}
	return ret, nil
}

func (r *platformConsoleRepository) GetMeetingUsageByTenantIDs(db *gorm.DB, tenantIDs []int64, start, end time.Time) (map[int64]int64, error) {
	ret := make(map[int64]int64, len(tenantIDs))
	if len(tenantIDs) == 0 {
		return ret, nil
	}
	rows := make([]PlatformMeetingUsageRow, 0)
	err := db.Table("meeting_rooms_jitsi AS meetings").
		Select("meetings.tenant_id, COALESCE(SUM(participants.duration), 0) AS total_duration_seconds").
		Joins("LEFT JOIN meeting_participants AS participants ON participants.meeting_id = meetings.id").
		Where("meetings.tenant_id IN ? AND meetings.created_at >= ? AND meetings.created_at < ?", tenantIDs, start, end).
		Group("meetings.tenant_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		ret[row.TenantID] = row.TotalDurationSeconds / 60
	}
	return ret, nil
}

func (r *platformConsoleRepository) GetWorkloadSnapshot(db *gorm.DB, now time.Time) (*PlatformWorkloadSnapshot, error) {
	ret := &PlatformWorkloadSnapshot{}
	if err := db.Model(&models.MeetingRoomJitsi{}).Where("status = ?", "active").Count(&ret.ActiveMeetings).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&models.Ticket{}).Where("status NOT IN ?", terminalTicketStatuses()).Count(&ret.OpenTickets).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&models.KnowledgeDocument{}).Where("status <> ? AND (review_status IN ? OR index_status = ?)", enums.StatusDeleted, []string{"draft", "review"}, enums.KnowledgeDocumentIndexStatusPending).Count(&ret.PendingKnowledge).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&models.AccessConnector{}).Where("status = ? AND health_status <> ?", "active", "healthy").Count(&ret.UnhealthyConnectors).Error; err != nil {
		return nil, err
	}
	pipeline, err := r.GetPipelineSnapshot(db, now)
	if err != nil {
		return nil, err
	}
	ret.LastUsageEventAt = pipeline.LastUsageEventAt
	ret.LastAggregateAt = pipeline.LastAggregateAt
	ret.RunningSyncs = pipeline.RunningSyncs
	ret.FailedSyncs24h = pipeline.FailedSyncs24h
	return ret, nil
}

func (r *platformConsoleRepository) GetPipelineSnapshot(db *gorm.DB, now time.Time) (*PlatformPipelineSnapshot, error) {
	ret := &PlatformPipelineSnapshot{}
	lastUsageEventAt, err := scanMaxTime(db.Model(&models.ProductAIUsageEvent{}), "occurred_at")
	if err != nil {
		return nil, err
	}
	ret.LastUsageEventAt = lastUsageEventAt
	lastAggregateAt, err := scanMaxTime(db.Model(&models.ProductAIUsageDaily{}), "updated_at")
	if err != nil {
		return nil, err
	}
	ret.LastAggregateAt = lastAggregateAt
	if err := db.Model(&models.SyncRun{}).Where("status = ?", models.SyncStatusRunning).Count(&ret.RunningSyncs).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&models.SyncRun{}).Where("status = ? AND started_at >= ?", models.SyncStatusFailed, now.Add(-24*time.Hour)).Count(&ret.FailedSyncs24h).Error; err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *platformConsoleRepository) GetRecentSyncRuns(db *gorm.DB, limit int) ([]PlatformSyncRunRow, error) {
	if limit <= 0 {
		limit = 10
	}
	syncRunTable, err := modelTableName(db, &models.SyncRun{})
	if err != nil {
		return nil, err
	}
	tenantTable, err := modelTableName(db, &models.Tenant{})
	if err != nil {
		return nil, err
	}
	rows := make([]PlatformSyncRunRow, 0)
	err = db.Table(syncRunTable + " AS runs").
		Select("runs.id, runs.tenant_id, COALESCE(tenants.name, '') AS tenant_name, runs.sync_type, runs.status, runs.records_processed, runs.error_message, runs.started_at, runs.ended_at").
		Joins("LEFT JOIN " + tenantTable + " AS tenants ON tenants.id = runs.tenant_id").
		Order("runs.started_at DESC, runs.id DESC").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
}

func (r *platformConsoleRepository) PingDatabase(db *gorm.DB) (*PlatformDatabaseSnapshot, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	startedAt := time.Now()
	if err = sqlDB.Ping(); err != nil {
		return nil, err
	}
	return &PlatformDatabaseSnapshot{LatencyMs: time.Since(startedAt).Milliseconds(), Stats: sqlDB.Stats()}, nil
}

func (r *platformConsoleRepository) GetDatabaseStorageSnapshot(db *gorm.DB, limit int) (*PlatformDatabaseStorageSnapshot, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if limit <= 0 {
		limit = 8
	}
	ret := &PlatformDatabaseStorageSnapshot{Engine: db.Dialector.Name(), Tables: make([]PlatformDatabaseTableStorageRow, 0)}
	switch ret.Engine {
	case "postgres":
		sizeRow := struct{ SizeBytes int64 }{}
		if err := db.Raw("SELECT pg_database_size(current_database()) AS size_bytes").Scan(&sizeRow).Error; err != nil {
			return ret, err
		}
		ret.SizeBytes = sizeRow.SizeBytes
		countRow := struct{ TableCount int64 }{}
		if err := db.Raw("SELECT COUNT(*) AS table_count FROM pg_catalog.pg_stat_user_tables").Scan(&countRow).Error; err != nil {
			return ret, err
		}
		ret.TableCount = countRow.TableCount
		if err := db.Raw(`
			SELECT relname AS name,
			       pg_total_relation_size(relid) AS size_bytes,
			       GREATEST(n_live_tup, 0)::bigint AS estimated_rows
			FROM pg_catalog.pg_stat_user_tables
			ORDER BY size_bytes DESC, relname ASC
			LIMIT ?`, limit).Scan(&ret.Tables).Error; err != nil {
			return ret, err
		}
	case "sqlite":
		var pageCount int64
		var pageSize int64
		if err := db.Raw("PRAGMA page_count").Scan(&pageCount).Error; err != nil {
			return ret, err
		}
		if err := db.Raw("PRAGMA page_size").Scan(&pageSize).Error; err != nil {
			return ret, err
		}
		ret.SizeBytes = pageCount * pageSize
		countRow := struct{ TableCount int64 }{}
		if err := db.Raw("SELECT COUNT(*) AS table_count FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'").Scan(&countRow).Error; err != nil {
			return ret, err
		}
		ret.TableCount = countRow.TableCount
	}
	return ret, nil
}

func (r *platformConsoleRepository) GetAssetStorageRows(db *gorm.DB) ([]PlatformAssetStorageRow, error) {
	ret := make([]PlatformAssetStorageRow, 0)
	if db == nil {
		return ret, fmt.Errorf("database not initialized")
	}
	if !db.Migrator().HasTable(&models.Asset{}) {
		return ret, nil
	}
	err := db.Model(&models.Asset{}).
		Select("CAST(provider AS TEXT) AS provider, COUNT(*) AS object_count, COALESCE(SUM(file_size), 0) AS size_bytes").
		Where("status <> ?", enums.AssetStatusDeleted).
		Group("provider").
		Scan(&ret).Error
	return ret, err
}

func (r *platformConsoleRepository) GetKnowledgeIndexSnapshot(db *gorm.DB, now time.Time) (*PlatformKnowledgeIndexSnapshot, error) {
	ret := &PlatformKnowledgeIndexSnapshot{}
	if err := db.Model(&models.KnowledgeIndexSyncTask{}).Where("status = ?", "pending").Count(&ret.PendingTasks).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&models.KnowledgeIndexSyncTask{}).Where("status = ?", "running").Count(&ret.RunningTasks).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&models.KnowledgeIndexSyncTask{}).Where("status = ? AND updated_at >= ?", "failed", now.Add(-24*time.Hour)).Count(&ret.FailedTasks24h).Error; err != nil {
		return nil, err
	}
	lastFinishedAt, err := scanMaxTime(db.Model(&models.KnowledgeIndexSyncTask{}), "finished_at")
	if err != nil {
		return nil, err
	}
	ret.LastFinishedAt = lastFinishedAt
	return ret, nil
}

func (r *platformConsoleRepository) GetRecentKnowledgeIndexTasks(db *gorm.DB, limit int) ([]PlatformKnowledgeIndexTaskRow, error) {
	if limit <= 0 {
		limit = 10
	}
	taskTable, err := modelTableName(db, &models.KnowledgeIndexSyncTask{})
	if err != nil {
		return nil, err
	}
	tenantTable, err := modelTableName(db, &models.Tenant{})
	if err != nil {
		return nil, err
	}
	productTable, err := modelTableName(db, &models.Product{})
	if err != nil {
		return nil, err
	}
	rows := make([]PlatformKnowledgeIndexTaskRow, 0)
	err = db.Table(taskTable + " AS tasks").
		Select("tasks.id, tasks.tenant_id, COALESCE(tenants.name, '') AS tenant_name, tasks.product_id, COALESCE(products.name, '') AS product_name, tasks.subject_type, tasks.subject_id, tasks.provider_type, tasks.action, tasks.status, tasks.retry_count, tasks.error_summary, tasks.updated_at").
		Joins("LEFT JOIN " + tenantTable + " AS tenants ON tenants.id = tasks.tenant_id").
		Joins("LEFT JOIN " + productTable + " AS products ON products.id = tasks.product_id").
		Order("tasks.updated_at DESC, tasks.id DESC").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
}

func (r *platformConsoleRepository) GetSub2APISnapshot(db *gorm.DB, now time.Time) (*PlatformSub2APISnapshot, error) {
	ret := &PlatformSub2APISnapshot{}
	if err := db.Model(&models.Sub2APITenantAccount{}).Where("status <> ?", enums.StatusDeleted).Count(&ret.TenantAccounts).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&models.Sub2APITenantAccount{}).Where("status = ? AND lower(account_status) IN ?", enums.StatusOk, []string{"active", "healthy", "ok", "normal"}).Count(&ret.HealthyAccounts).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&models.Sub2APITenantAccount{}).Where("status = ? AND lower(account_status) NOT IN ?", enums.StatusOk, []string{"active", "healthy", "ok", "normal"}).Count(&ret.AttentionAccounts).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&models.ProductAIUsageCredential{}).Where("status = ?", enums.StatusOk).Count(&ret.ProductCredentials).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&models.ProductAIUsageCredential{}).Where("status = ? AND (last_synced_at IS NULL OR last_synced_at < ?)", enums.StatusOk, now.Add(-24*time.Hour)).Count(&ret.StaleCredentials24h).Error; err != nil {
		return nil, err
	}
	lastUsageEventAt, err := scanMaxTime(db.Model(&models.ProductAIUsageEvent{}), "occurred_at")
	if err != nil {
		return nil, err
	}
	lastCredentialSyncAt, err := scanMaxTime(db.Model(&models.ProductAIUsageCredential{}), "last_synced_at")
	if err != nil {
		return nil, err
	}
	ret.LastUsageEventAt = lastUsageEventAt
	ret.LastCredentialSyncAt = lastCredentialSyncAt
	return ret, nil
}

func (r *platformConsoleRepository) GetJitsiSnapshot(db *gorm.DB, now, heartbeatCutoff time.Time) (*PlatformJitsiSnapshot, error) {
	ret := &PlatformJitsiSnapshot{}
	if err := db.Model(&models.MeetingRoomJitsi{}).Where("status = ?", "active").Count(&ret.ActiveMeetings).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&models.MeetingRoomJitsi{}).Where("created_at >= ?", now.Add(-24*time.Hour)).Count(&ret.Meetings24h).Error; err != nil {
		return nil, err
	}
	type meetingMinutesRow struct {
		TotalDurationSeconds int64
	}
	minutes := &meetingMinutesRow{}
	if err := db.Table("meeting_rooms_jitsi AS meetings").
		Select("COALESCE(SUM(participants.duration), 0) AS total_duration_seconds").
		Joins("LEFT JOIN meeting_participants AS participants ON participants.meeting_id = meetings.id").
		Where("meetings.created_at >= ?", now.Add(-24*time.Hour)).
		Scan(minutes).Error; err != nil {
		return nil, err
	}
	ret.ParticipantMinutes24h = minutes.TotalDurationSeconds / 60
	activeParticipants := func() *gorm.DB {
		return db.Table("meeting_participants AS participants").
			Joins("JOIN meeting_rooms_jitsi AS meetings ON meetings.id = participants.meeting_id").
			Where("meetings.status = ? AND participants.joined_at IS NOT NULL AND participants.left_at IS NULL", "active")
	}
	if err := activeParticipants().Where("participants.updated_at >= ?", heartbeatCutoff).Count(&ret.OnlineParticipants).Error; err != nil {
		return nil, err
	}
	if err := activeParticipants().Where("participants.updated_at < ?", heartbeatCutoff).Count(&ret.StaleParticipants).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&models.MeetingRoomJitsi{}).
		Where("status = ? AND updated_at < ?", "active", heartbeatCutoff).
		Where("EXISTS (SELECT 1 FROM meeting_participants AS participants WHERE participants.meeting_id = meeting_rooms_jitsi.id AND participants.joined_at IS NOT NULL)").
		Where("NOT EXISTS (SELECT 1 FROM meeting_participants AS participants WHERE participants.meeting_id = meeting_rooms_jitsi.id AND participants.joined_at IS NOT NULL AND participants.left_at IS NULL AND participants.updated_at >= ?)", heartbeatCutoff).
		Count(&ret.StaleMeetings).Error; err != nil {
		return nil, err
	}
	lastMeetingAt, err := scanMaxTime(db.Model(&models.MeetingRoomJitsi{}), "updated_at")
	if err != nil {
		return nil, err
	}
	ret.LastMeetingAt = lastMeetingAt
	lastHeartbeatAt, err := scanMaxTime(activeParticipants(), "participants.updated_at")
	if err != nil {
		return nil, err
	}
	ret.LastHeartbeatAt = lastHeartbeatAt
	return ret, nil
}

func (r *platformConsoleRepository) GetAccessSnapshot(db *gorm.DB, now time.Time) (*PlatformAccessSnapshot, error) {
	ret := &PlatformAccessSnapshot{}
	if err := db.Model(&models.AccessConnector{}).Count(&ret.TotalConnectors).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&models.AccessConnector{}).Where("status = ?", "active").Count(&ret.ActiveConnectors).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&models.AccessConnector{}).Where("health_status = ?", "healthy").Count(&ret.HealthyConnectors).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&models.AccessConnector{}).Where("health_status = ?", "degraded").Count(&ret.DegradedConnectors).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&models.AccessConnector{}).Where("health_status = ?", "unhealthy").Count(&ret.UnhealthyConnectors).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&models.AccessCallLog{}).Where("created_at >= ? AND (error_message <> '' OR response_code >= ?)", now.Add(-24*time.Hour), 400).Count(&ret.FailedCalls24h).Error; err != nil {
		return nil, err
	}
	lastTestedAt, err := scanMaxTime(db.Model(&models.AccessConnector{}), "last_tested_at")
	if err != nil {
		return nil, err
	}
	ret.LastTestedAt = lastTestedAt
	return ret, nil
}

func (r *platformConsoleRepository) GetNotificationSnapshot(db *gorm.DB, now time.Time) (*PlatformNotificationSnapshot, error) {
	ret := &PlatformNotificationSnapshot{}
	if err := db.Model(&models.Notification{}).Where("status <> ? AND delivery_status = ?", enums.StatusDeleted, "pending").Count(&ret.PendingNotifications).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&models.Notification{}).Where("status <> ? AND read_at IS NULL", enums.StatusDeleted).Count(&ret.UnreadNotifications).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&models.DeliveryLog{}).Where("created_at >= ? AND status IN ?", now.Add(-24*time.Hour), []string{"failed", "bounced"}).Count(&ret.FailedDeliveries24h).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&models.DeliveryLog{}).Where("created_at >= ? AND status IN ?", now.Add(-24*time.Hour), []string{"sent", "delivered"}).Count(&ret.SentDeliveries24h).Error; err != nil {
		return nil, err
	}
	lastDeliveredAt, err := scanMaxTime(db.Model(&models.DeliveryLog{}), "delivered_at")
	if err != nil {
		return nil, err
	}
	ret.LastDeliveredAt = lastDeliveredAt
	return ret, nil
}

func terminalTicketStatuses() []string {
	return []string{
		string(enums.TicketStatusResolved),
		string(enums.TicketStatusClosed),
		string(enums.TicketStatusDone),
		string(enums.TicketStatusCancelled),
	}
}

func scanMaxTime(db *gorm.DB, column string) (*time.Time, error) {
	row := &platformTimeScanRow{}
	if err := db.Select("CAST(MAX(" + column + ") AS TEXT) AS value").Scan(row).Error; err != nil {
		return nil, err
	}
	return parseDatabaseTime(row.Value), nil
}

func parseDatabaseTime(value sql.NullString) *time.Time {
	if !value.Valid || value.String == "" {
		return nil
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		time.DateTime,
	} {
		parsed, err := time.Parse(layout, value.String)
		if err == nil {
			return &parsed
		}
	}
	return nil
}

func modelTableName(db *gorm.DB, model any) (string, error) {
	statement := &gorm.Statement{DB: db}
	if err := statement.Parse(model); err != nil {
		return "", err
	}
	return statement.Schema.Table, nil
}
