package services

import (
	"encoding/json"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var PlatformConsoleService = &platformConsoleService{startedAt: time.Now()}

type platformConsoleService struct {
	startedAt time.Time
}

type PlatformOverviewAggregate struct {
	GeneratedAt  time.Time
	Counts       *repositories.PlatformOverviewCounts
	AIUsage      *repositories.PlatformAIUsageRow
	TopTenants   []repositories.PlatformTopTenantRow
	TopUsage     map[int64]repositories.PlatformAIUsageRow
	TopPlans     map[int64]string
	DailyUsage   []repositories.PlatformDailyUsageRow
	RecentEvents []repositories.PlatformRecentEventRow
}

type PlatformResolvedQuota struct {
	Key    string
	Value  int64
	Unit   string
	Period string
}

type PlatformTenantUsageAggregate struct {
	TenantID       int64
	TenantName     string
	PlanID         int64
	PlanName       string
	AIRequests     int64
	AIRequestLimit int64
	AITokens       int64
	AITokenLimit   int64
	Devices        int64
	DeviceLimit    int64
	MeetingMinutes int64
	MeetingLimit   int64
	CostAmount     float64
	UsageRatio     float64
	RiskLevel      string
	LastMeteredAt  *time.Time
}

type PlatformUsagePlanAggregate struct {
	Plan             models.TenantPlan
	Quotas           []PlatformResolvedQuota
	TenantCount      int64
	AtRiskTenants    int64
	OverLimitTenants int64
	AIRequests       int64
	AITokens         int64
	Devices          int64
	MeetingMinutes   int64
}

type PlatformSharedTranslationUsageAggregate struct {
	Configured    bool
	APIKeyMasked  string
	Fingerprint   string
	APIKeyID      string
	Requests      int64
	Tokens        int64
	CostAmount    float64
	LastMeteredAt *time.Time
}

type PlatformUsageAggregate struct {
	GeneratedAt            time.Time
	PeriodStart            time.Time
	PeriodEnd              time.Time
	SharedTranslationUsage PlatformSharedTranslationUsageAggregate
	Plans                  []PlatformUsagePlanAggregate
	Tenants                []PlatformTenantUsageAggregate
	DailyUsage             []repositories.PlatformDailyUsageRow
}

type PlatformOpsAggregate struct {
	GeneratedAt    time.Time
	UptimeSeconds  int64
	GoVersion      string
	Goroutines     int
	HeapAlloc      uint64
	HeapSys        uint64
	GCCount        uint32
	Database       *repositories.PlatformDatabaseSnapshot
	DatabaseError  string
	Workloads      *repositories.PlatformWorkloadSnapshot
	SyncRuns       []repositories.PlatformSyncRunRow
	KnowledgeIndex *repositories.PlatformKnowledgeIndexSnapshot
	KnowledgeTasks []repositories.PlatformKnowledgeIndexTaskRow
	Sub2API        *repositories.PlatformSub2APISnapshot
	Jitsi          *PlatformJitsiAggregate
	Integrations   *PlatformIntegrationAggregate
	Access         *repositories.PlatformAccessSnapshot
	Notifications  *repositories.PlatformNotificationSnapshot
}

type PlatformOpsRuntimeAggregate struct {
	GeneratedAt   time.Time
	UptimeSeconds int64
	GoVersion     string
	Goroutines    int
	HeapAlloc     uint64
	HeapSys       uint64
	GCCount       uint32
}

type PlatformOpsDatabaseAggregate struct {
	GeneratedAt   time.Time
	Database      *repositories.PlatformDatabaseSnapshot
	DatabaseError string
}

type PlatformOpsJitsiAggregate struct {
	GeneratedAt time.Time
	Jitsi       *PlatformJitsiAggregate
}

type PlatformOpsSub2APIAggregate struct {
	GeneratedAt time.Time
	Sub2API     *repositories.PlatformSub2APISnapshot
}

type PlatformOpsAccessAggregate struct {
	GeneratedAt time.Time
	Access      *repositories.PlatformAccessSnapshot
}

type PlatformOpsPipelineAggregate struct {
	GeneratedAt time.Time
	Pipeline    *repositories.PlatformPipelineSnapshot
}

type PlatformOpsKnowledgeQueueAggregate struct {
	GeneratedAt    time.Time
	KnowledgeIndex *repositories.PlatformKnowledgeIndexSnapshot
}

type PlatformOpsNotificationQueueAggregate struct {
	GeneratedAt   time.Time
	Notifications *repositories.PlatformNotificationSnapshot
}

type PlatformGlobalizationRegionAggregate struct {
	DataRegion     string
	TenantCount    int64
	CountryRegions []string
	Locales        []string
	Timezones      []string
}

type PlatformGlobalizationAggregate struct {
	GeneratedAt           time.Time
	TenantPage            *PlatformTenantListAggregate
	Regions               []PlatformGlobalizationRegionAggregate
	ConfiguredRegionCount int
	LocaleCount           int
	TimezoneCount         int
}

func (s *platformConsoleService) GetGlobalization(now time.Time, page, pageSize int) (*PlatformGlobalizationAggregate, error) {
	tenantPage, err := PlatformIAMService.ListTenants("", "", "all", "", page, pageSize)
	if err != nil {
		return nil, err
	}
	rows, err := repositories.PlatformConsoleRepository.GetGlobalizationRegionRows(sqls.DB())
	if err != nil {
		return nil, err
	}
	type regionSets struct {
		count     int64
		countries map[string]struct{}
		locales   map[string]struct{}
		timezones map[string]struct{}
	}
	grouped := make(map[string]*regionSets)
	allLocales := make(map[string]struct{})
	allTimezones := make(map[string]struct{})
	for _, row := range rows {
		key := strings.TrimSpace(row.DataRegion)
		region := grouped[key]
		if region == nil {
			region = &regionSets{countries: map[string]struct{}{}, locales: map[string]struct{}{}, timezones: map[string]struct{}{}}
			grouped[key] = region
		}
		region.count += row.TenantCount
		if value := strings.TrimSpace(row.CountryRegion); value != "" {
			region.countries[value] = struct{}{}
		}
		locales := normalizeGlobalizationPreferences(row.SupportedLocalesJSON, row.DefaultLocale)
		addStringSetValues(region.locales, locales)
		addStringSetValues(allLocales, locales)
		timezones := normalizeGlobalizationPreferences(row.SupportedTimezonesJSON, row.Timezone)
		addStringSetValues(region.timezones, timezones)
		addStringSetValues(allTimezones, timezones)
	}
	regions := make([]PlatformGlobalizationRegionAggregate, 0, len(grouped))
	configured := 0
	for key, values := range grouped {
		if key != "" {
			configured++
		}
		regions = append(regions, PlatformGlobalizationRegionAggregate{
			DataRegion: key, TenantCount: values.count,
			CountryRegions: sortedStringSet(values.countries),
			Locales:        sortedStringSet(values.locales),
			Timezones:      sortedStringSet(values.timezones),
		})
	}
	sort.Slice(regions, func(i, j int) bool {
		if regions[i].DataRegion == "" {
			return false
		}
		if regions[j].DataRegion == "" {
			return true
		}
		if regions[i].TenantCount == regions[j].TenantCount {
			return regions[i].DataRegion < regions[j].DataRegion
		}
		return regions[i].TenantCount > regions[j].TenantCount
	})
	return &PlatformGlobalizationAggregate{
		GeneratedAt: now, TenantPage: tenantPage, Regions: regions,
		ConfiguredRegionCount: configured, LocaleCount: len(allLocales), TimezoneCount: len(allTimezones),
	}, nil
}

func sortedStringSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func normalizeGlobalizationPreferences(raw string, fallback string) []string {
	return normalizeTenantPreferences(utils.ParseStringListJSON(raw), fallback)
}

func addStringSetValues(target map[string]struct{}, values []string) {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			target[value] = struct{}{}
		}
	}
}

func (s *platformConsoleService) GetOverview(now time.Time) (*PlatformOverviewAggregate, error) {
	db := sqls.DB()
	counts, err := repositories.PlatformConsoleRepository.GetOverviewCounts(db, now)
	if err != nil {
		return nil, err
	}
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	end := now.Add(time.Second)
	aiUsage, err := repositories.PlatformConsoleRepository.GetAIUsage(db, startOfMonth, end)
	if err != nil {
		return nil, err
	}
	topTenants, err := repositories.PlatformConsoleRepository.GetTopTenantsByDevices(db, 5)
	if err != nil {
		return nil, err
	}
	topTenantIDs := make([]int64, 0, len(topTenants))
	for _, item := range topTenants {
		topTenantIDs = append(topTenantIDs, item.TenantID)
	}
	topUsage, err := repositories.PlatformConsoleRepository.GetAIUsageByTenantIDs(db, topTenantIDs, startOfMonth, end)
	if err != nil {
		return nil, err
	}
	topPlans, err := s.resolveTenantPlanNames(topTenantIDs)
	if err != nil {
		return nil, err
	}
	startOfTrend := startOfDay(now).AddDate(0, 0, -6)
	dailyUsage, err := repositories.PlatformConsoleRepository.GetDailyAIUsage(db, startOfTrend, end)
	if err != nil {
		return nil, err
	}
	recentEvents, err := repositories.PlatformConsoleRepository.GetRecentEvents(db, 8)
	if err != nil {
		return nil, err
	}
	return &PlatformOverviewAggregate{
		GeneratedAt: now, Counts: counts, AIUsage: aiUsage,
		TopTenants: topTenants, TopUsage: topUsage, TopPlans: topPlans,
		DailyUsage: fillDailyUsage(startOfTrend, 7, dailyUsage), RecentEvents: recentEvents,
	}, nil
}

func (s *platformConsoleService) GetOverviewMetrics(now time.Time) (*PlatformOverviewAggregate, error) {
	db := sqls.DB()
	counts, err := repositories.PlatformConsoleRepository.GetOverviewCounts(db, now)
	if err != nil {
		return nil, err
	}
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	end := now.Add(time.Second)
	aiUsage, err := repositories.PlatformConsoleRepository.GetAIUsage(db, startOfMonth, end)
	if err != nil {
		return nil, err
	}
	return &PlatformOverviewAggregate{
		GeneratedAt: now,
		Counts:      counts,
		AIUsage:     aiUsage,
	}, nil
}

func (s *platformConsoleService) GetOverviewTopTenants(now time.Time, limit int) (*PlatformOverviewAggregate, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}
	db := sqls.DB()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	end := now.Add(time.Second)
	topTenants, err := repositories.PlatformConsoleRepository.GetTopTenantsByDevices(db, limit)
	if err != nil {
		return nil, err
	}
	topTenantIDs := make([]int64, 0, len(topTenants))
	for _, item := range topTenants {
		topTenantIDs = append(topTenantIDs, item.TenantID)
	}
	topUsage, err := repositories.PlatformConsoleRepository.GetAIUsageByTenantIDs(db, topTenantIDs, startOfMonth, end)
	if err != nil {
		return nil, err
	}
	topPlans, err := s.resolveTenantPlanNames(topTenantIDs)
	if err != nil {
		return nil, err
	}
	return &PlatformOverviewAggregate{
		GeneratedAt: now,
		TopTenants:  topTenants,
		TopUsage:    topUsage,
		TopPlans:    topPlans,
	}, nil
}

func (s *platformConsoleService) GetOverviewUsageTrend(now time.Time, days int) (*PlatformOverviewAggregate, error) {
	if days <= 0 {
		days = 7
	}
	if days > 31 {
		days = 31
	}
	db := sqls.DB()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	end := now.Add(time.Second)
	aiUsage, err := repositories.PlatformConsoleRepository.GetAIUsage(db, startOfMonth, end)
	if err != nil {
		return nil, err
	}
	startOfTrend := startOfDay(now).AddDate(0, 0, -(days - 1))
	dailyUsage, err := repositories.PlatformConsoleRepository.GetDailyAIUsage(db, startOfTrend, end)
	if err != nil {
		return nil, err
	}
	return &PlatformOverviewAggregate{
		GeneratedAt: now,
		AIUsage:     aiUsage,
		DailyUsage:  fillDailyUsage(startOfTrend, days, dailyUsage),
	}, nil
}

func (s *platformConsoleService) GetOverviewRecentEvents(now time.Time, limit int) (*PlatformOverviewAggregate, error) {
	if limit <= 0 {
		limit = 8
	}
	if limit > 50 {
		limit = 50
	}
	recentEvents, err := repositories.PlatformConsoleRepository.GetRecentEvents(sqls.DB(), limit)
	if err != nil {
		return nil, err
	}
	return &PlatformOverviewAggregate{
		GeneratedAt:  now,
		RecentEvents: recentEvents,
	}, nil
}

func (s *platformConsoleService) GetUsageOverview(now time.Time) (*PlatformUsageAggregate, error) {
	db := sqls.DB()
	plans, err := repositories.PlatformIAMRepository.FindPlans(db, "all")
	if err != nil {
		return nil, err
	}
	planIDs := make([]int64, 0, len(plans))
	for _, plan := range plans {
		planIDs = append(planIDs, plan.ID)
	}
	storedQuotas, err := repositories.PlatformConsoleRepository.FindPlanQuotas(db, planIDs)
	if err != nil {
		return nil, err
	}
	resolvedQuotas := make(map[int64][]PlatformResolvedQuota, len(plans))
	for _, plan := range plans {
		resolvedQuotas[plan.ID] = resolvePlanQuotas(plan, storedQuotas[plan.ID])
	}

	bases, err := repositories.PlatformConsoleRepository.FindUsageTenantBases(db, 200)
	if err != nil {
		return nil, err
	}
	tenantIDs := make([]int64, 0, len(bases))
	for _, item := range bases {
		tenantIDs = append(tenantIDs, item.TenantID)
	}
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	periodEnd := now.Add(time.Second)
	sharedTranslationUsage, err := s.getSharedTranslationUsage(db, periodStart, periodEnd)
	if err != nil {
		return nil, err
	}
	usage, err := repositories.PlatformConsoleRepository.GetAIUsageByTenantIDs(db, tenantIDs, periodStart, periodEnd)
	if err != nil {
		return nil, err
	}
	deviceCounts, err := repositories.PlatformIAMRepository.CountDevicesByTenantIDs(db, tenantIDs)
	if err != nil {
		return nil, err
	}
	meetingMinutes, err := repositories.PlatformConsoleRepository.GetMeetingUsageByTenantIDs(db, tenantIDs, periodStart, periodEnd)
	if err != nil {
		return nil, err
	}

	tenantUsage := make([]PlatformTenantUsageAggregate, 0, len(bases))
	planAggregates := make(map[int64]*PlatformUsagePlanAggregate, len(plans))
	for _, plan := range plans {
		planCopy := plan
		planAggregates[plan.ID] = &PlatformUsagePlanAggregate{Plan: planCopy, Quotas: resolvedQuotas[plan.ID]}
	}
	for _, base := range bases {
		metered := usage[base.TenantID]
		quotas := resolvedQuotas[base.PlanID]
		item := PlatformTenantUsageAggregate{
			TenantID: base.TenantID, TenantName: base.TenantName,
			PlanID: base.PlanID, PlanName: base.PlanName,
			AIRequests: metered.Requests, AIRequestLimit: quotaValue(quotas, "ai_requests"),
			AITokens: metered.Tokens, AITokenLimit: quotaValue(quotas, "ai_tokens"),
			Devices: deviceCounts[base.TenantID], DeviceLimit: quotaValue(quotas, "devices"),
			MeetingMinutes: meetingMinutes[base.TenantID], MeetingLimit: quotaValue(quotas, "meeting_minutes"),
			CostAmount: metered.CostAmount, LastMeteredAt: metered.LastMeteredAt,
		}
		item.UsageRatio = maxUsageRatio(item)
		item.RiskLevel = usageRiskLevel(item.UsageRatio)
		tenantUsage = append(tenantUsage, item)
		if aggregate := planAggregates[base.PlanID]; aggregate != nil {
			aggregate.TenantCount++
			aggregate.AIRequests += item.AIRequests
			aggregate.AITokens += item.AITokens
			aggregate.Devices += item.Devices
			aggregate.MeetingMinutes += item.MeetingMinutes
			if item.UsageRatio >= 0.8 {
				aggregate.AtRiskTenants++
			}
			if item.UsageRatio >= 1 {
				aggregate.OverLimitTenants++
			}
		}
	}
	planUsage := make([]PlatformUsagePlanAggregate, 0, len(plans))
	for _, plan := range plans {
		planUsage = append(planUsage, *planAggregates[plan.ID])
	}
	trendStart := startOfDay(now).AddDate(0, 0, -6)
	dailyUsage, err := repositories.PlatformConsoleRepository.GetDailyAIUsage(db, trendStart, periodEnd)
	if err != nil {
		return nil, err
	}
	return &PlatformUsageAggregate{
		GeneratedAt: now, PeriodStart: periodStart, PeriodEnd: now,
		SharedTranslationUsage: sharedTranslationUsage,
		Plans:                  planUsage, Tenants: tenantUsage,
		DailyUsage: fillDailyUsage(trendStart, 7, dailyUsage),
	}, nil
}

func (s *platformConsoleService) getSharedTranslationUsage(db *gorm.DB, start, end time.Time) (PlatformSharedTranslationUsageAggregate, error) {
	apiKey, fingerprint, _, err := resolvePlatformSub2APITranslationKeyConfig()
	if err != nil {
		return PlatformSharedTranslationUsageAggregate{}, err
	}
	ret := PlatformSharedTranslationUsageAggregate{
		Configured:   strings.TrimSpace(apiKey) != "",
		APIKeyMasked: maskedIntegrationSecret(apiKey),
		Fingerprint:  fingerprint,
	}
	if !ret.Configured {
		return ret, nil
	}
	ret.APIKeyID = platformTranslationUsageAPIKeyID(fingerprint)
	row, err := repositories.PlatformConsoleRepository.GetAIUsageByAPIKeyID(db, ret.APIKeyID, start, end)
	if err != nil {
		return PlatformSharedTranslationUsageAggregate{}, err
	}
	ret.Requests = row.Requests
	ret.Tokens = row.Tokens
	ret.CostAmount = row.CostAmount
	ret.LastMeteredAt = row.LastMeteredAt
	return ret, nil
}

func platformTranslationUsageAPIKeyID(fingerprint string) string {
	fingerprint = strings.TrimSpace(fingerprint)
	if fingerprint == "" {
		return "platform_translation"
	}
	return "platform_translation:" + fingerprint
}

func (s *platformConsoleService) GetOpsRuntime(now time.Time) *PlatformOpsRuntimeAggregate {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	return &PlatformOpsRuntimeAggregate{
		GeneratedAt: now, UptimeSeconds: int64(now.Sub(s.startedAt).Seconds()),
		GoVersion: runtime.Version(), Goroutines: runtime.NumGoroutine(),
		HeapAlloc: memory.HeapAlloc, HeapSys: memory.HeapSys, GCCount: memory.NumGC,
	}
}

func (s *platformConsoleService) GetOpsDatabase(now time.Time) *PlatformOpsDatabaseAggregate {
	database, err := repositories.PlatformConsoleRepository.PingDatabase(sqls.DB())
	ret := &PlatformOpsDatabaseAggregate{GeneratedAt: now, Database: database}
	if err != nil {
		ret.DatabaseError = err.Error()
	}
	return ret
}

func (s *platformConsoleService) GetOpsJitsi(now time.Time) (*PlatformOpsJitsiAggregate, error) {
	snapshot, err := repositories.PlatformConsoleRepository.GetJitsiSnapshot(
		sqls.DB(), now, now.Add(-meetingParticipantStaleAfter),
	)
	if err != nil {
		return nil, err
	}
	jitsi := buildPlatformJitsiAggregate(snapshot)
	applyPlatformJitsiServiceHealth(jitsi, probePlatformJitsiService())
	return &PlatformOpsJitsiAggregate{GeneratedAt: now, Jitsi: jitsi}, nil
}

func (s *platformConsoleService) GetOpsSub2API(now time.Time) (*PlatformOpsSub2APIAggregate, error) {
	snapshot, err := repositories.PlatformConsoleRepository.GetSub2APISnapshot(sqls.DB(), now)
	if err != nil {
		return nil, err
	}
	return &PlatformOpsSub2APIAggregate{GeneratedAt: now, Sub2API: snapshot}, nil
}

func (s *platformConsoleService) GetOpsAccess(now time.Time) (*PlatformOpsAccessAggregate, error) {
	snapshot, err := repositories.PlatformConsoleRepository.GetAccessSnapshot(sqls.DB(), now)
	if err != nil {
		return nil, err
	}
	return &PlatformOpsAccessAggregate{GeneratedAt: now, Access: snapshot}, nil
}

func (s *platformConsoleService) GetOpsPipeline(now time.Time) (*PlatformOpsPipelineAggregate, error) {
	snapshot, err := repositories.PlatformConsoleRepository.GetPipelineSnapshot(sqls.DB(), now)
	if err != nil {
		return nil, err
	}
	return &PlatformOpsPipelineAggregate{GeneratedAt: now, Pipeline: snapshot}, nil
}

func (s *platformConsoleService) GetOpsKnowledgeQueue(now time.Time) (*PlatformOpsKnowledgeQueueAggregate, error) {
	snapshot, err := repositories.PlatformConsoleRepository.GetKnowledgeIndexSnapshot(sqls.DB(), now)
	if err != nil {
		return nil, err
	}
	return &PlatformOpsKnowledgeQueueAggregate{GeneratedAt: now, KnowledgeIndex: snapshot}, nil
}

func (s *platformConsoleService) GetOpsNotificationQueue(now time.Time) (*PlatformOpsNotificationQueueAggregate, error) {
	snapshot, err := repositories.PlatformConsoleRepository.GetNotificationSnapshot(sqls.DB(), now)
	if err != nil {
		return nil, err
	}
	return &PlatformOpsNotificationQueueAggregate{GeneratedAt: now, Notifications: snapshot}, nil
}

func (s *platformConsoleService) GetOpsOverview(now time.Time) (*PlatformOpsAggregate, error) {
	workloads, err := repositories.PlatformConsoleRepository.GetWorkloadSnapshot(sqls.DB(), now)
	if err != nil {
		return nil, err
	}
	syncRuns, err := repositories.PlatformConsoleRepository.GetRecentSyncRuns(sqls.DB(), 10)
	if err != nil {
		return nil, err
	}
	databaseStatus := s.GetOpsDatabase(now)
	runtimeStatus := s.GetOpsRuntime(now)
	ret := &PlatformOpsAggregate{
		GeneratedAt: now, UptimeSeconds: runtimeStatus.UptimeSeconds,
		GoVersion: runtimeStatus.GoVersion, Goroutines: runtimeStatus.Goroutines,
		HeapAlloc: runtimeStatus.HeapAlloc, HeapSys: runtimeStatus.HeapSys, GCCount: runtimeStatus.GCCount,
		Database: databaseStatus.Database, Workloads: workloads, SyncRuns: syncRuns,
	}
	ret.DatabaseError = databaseStatus.DatabaseError
	if ret.KnowledgeIndex, err = repositories.PlatformConsoleRepository.GetKnowledgeIndexSnapshot(sqls.DB(), now); err != nil {
		return nil, err
	}
	if ret.KnowledgeTasks, err = repositories.PlatformConsoleRepository.GetRecentKnowledgeIndexTasks(sqls.DB(), 10); err != nil {
		return nil, err
	}
	if ret.Sub2API, err = repositories.PlatformConsoleRepository.GetSub2APISnapshot(sqls.DB(), now); err != nil {
		return nil, err
	}
	jitsi, err := repositories.PlatformConsoleRepository.GetJitsiSnapshot(
		sqls.DB(), now, now.Add(-meetingParticipantStaleAfter),
	)
	if err != nil {
		return nil, err
	}
	ret.Jitsi = buildPlatformJitsiAggregate(jitsi)
	applyPlatformJitsiServiceHealth(ret.Jitsi, probePlatformJitsiService())
	ret.Integrations = buildPlatformIntegrationAggregate()
	if ret.Access, err = repositories.PlatformConsoleRepository.GetAccessSnapshot(sqls.DB(), now); err != nil {
		return nil, err
	}
	if ret.Notifications, err = repositories.PlatformConsoleRepository.GetNotificationSnapshot(sqls.DB(), now); err != nil {
		return nil, err
	}
	return ret, nil
}

func (s *platformConsoleService) resolveTenantPlanNames(tenantIDs []int64) (map[int64]string, error) {
	ret := make(map[int64]string, len(tenantIDs))
	subscriptions, err := repositories.PlatformIAMRepository.FindActiveSubscriptions(sqls.DB(), tenantIDs)
	if err != nil {
		return nil, err
	}
	planIDs := make([]int64, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		planIDs = append(planIDs, subscription.PlanID)
	}
	plans, err := repositories.PlatformIAMRepository.FindPlansByIDs(sqls.DB(), uniqueInt64s(planIDs))
	if err != nil {
		return nil, err
	}
	for tenantID, subscription := range subscriptions {
		if plan := plans[subscription.PlanID]; plan != nil {
			ret[tenantID] = plan.Name
		}
	}
	return ret, nil
}

func resolvePlanQuotas(plan models.TenantPlan, rows []models.TenantPlanQuota) []PlatformResolvedQuota {
	ret := make([]PlatformResolvedQuota, 0, len(rows)+4)
	seen := make(map[string]bool)
	for _, row := range rows {
		key := canonicalQuotaKey(row.QuotaKey)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		ret = append(ret, PlatformResolvedQuota{Key: key, Value: row.QuotaValue, Unit: row.Unit, Period: row.Period})
	}
	var raw map[string]any
	if json.Unmarshal([]byte(plan.QuotaTemplateJSON), &raw) == nil {
		for key, value := range raw {
			canonical := canonicalQuotaKey(key)
			if canonical == "" || seen[canonical] {
				continue
			}
			quotaValue, unit, period := decodeQuotaValue(value)
			seen[canonical] = true
			ret = append(ret, PlatformResolvedQuota{Key: canonical, Value: quotaValue, Unit: unit, Period: period})
		}
	}
	return ret
}

func canonicalQuotaKey(key string) string {
	normalized := strings.ToLower(strings.TrimSpace(key))
	switch normalized {
	case "ai_requests", "requests", "request_limit", "monthly_requests", "monthlyrequestlimit":
		return "ai_requests"
	case "ai_tokens", "tokens", "token_limit", "monthly_tokens", "monthlytokenlimit":
		return "ai_tokens"
	case "devices", "device_limit", "device_count", "devicelimit":
		return "devices"
	case "meeting_minutes", "meetings", "meeting_limit", "meetingminutes":
		return "meeting_minutes"
	case "knowledge_documents_per_tenant", "tenant_knowledge_documents", "tenant_document_limit":
		return QuotaKnowledgeDocumentsPerTenant
	case "knowledge_documents_per_product", "product_knowledge_documents", "product_document_limit":
		return QuotaKnowledgeDocumentsPerProduct
	case "knowledge_documents_per_user", "user_knowledge_documents", "user_document_limit":
		return QuotaKnowledgeDocumentsPerUser
	case "knowledge_documents_per_non_admin_user", "non_admin_knowledge_documents", "non_admin_document_limit":
		return QuotaKnowledgeDocumentsPerNonAdminUser
	case "knowledge_document_file_size_bytes", "knowledge_file_size_bytes":
		return QuotaKnowledgeDocumentSizeBytes
	case "knowledge_document_file_size_mb", "knowledge_file_size_mb":
		return QuotaKnowledgeDocumentSizeMB
	default:
		return ""
	}
}

func decodeQuotaValue(value any) (int64, string, string) {
	switch typed := value.(type) {
	case float64:
		return int64(typed), "", "monthly"
	case string:
		parsed, _ := strconv.ParseInt(strings.ReplaceAll(typed, ",", ""), 10, 64)
		return parsed, "", "monthly"
	case map[string]any:
		parsed, unit, period := decodeQuotaValue(typed["value"])
		if rawUnit, ok := typed["unit"].(string); ok {
			unit = rawUnit
		}
		if rawPeriod, ok := typed["period"].(string); ok {
			period = rawPeriod
		}
		return parsed, unit, period
	default:
		return 0, "", "monthly"
	}
}

func quotaValue(quotas []PlatformResolvedQuota, key string) int64 {
	for _, quota := range quotas {
		if quota.Key == key {
			return quota.Value
		}
	}
	return 0
}

func maxUsageRatio(item PlatformTenantUsageAggregate) float64 {
	values := [][2]int64{
		{item.AIRequests, item.AIRequestLimit},
		{item.AITokens, item.AITokenLimit},
		{item.Devices, item.DeviceLimit},
		{item.MeetingMinutes, item.MeetingLimit},
	}
	maxRatio := 0.0
	for _, value := range values {
		if value[1] <= 0 {
			continue
		}
		ratio := float64(value[0]) / float64(value[1])
		if ratio > maxRatio {
			maxRatio = ratio
		}
	}
	return maxRatio
}

func usageRiskLevel(ratio float64) string {
	switch {
	case ratio >= 1:
		return "over_limit"
	case ratio >= 0.8:
		return "warning"
	default:
		return "normal"
	}
}

func fillDailyUsage(start time.Time, days int, rows []repositories.PlatformDailyUsageRow) []repositories.PlatformDailyUsageRow {
	byDate := make(map[string]repositories.PlatformDailyUsageRow, len(rows))
	for _, row := range rows {
		byDate[row.Date] = row
	}
	ret := make([]repositories.PlatformDailyUsageRow, 0, days)
	for index := 0; index < days; index++ {
		date := start.AddDate(0, 0, index).Format("2006-01-02")
		row := byDate[date]
		row.Date = date
		ret = append(ret, row)
	}
	return ret
}

func (s *platformConsoleService) SetStartedAtForTest(value time.Time) {
	if value.IsZero() {
		return
	}
	s.startedAt = value
}
