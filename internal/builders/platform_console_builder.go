package builders

import (
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/services"
)

func BuildPlatformOverview(item *services.PlatformOverviewAggregate) *response.PlatformOverviewResponse {
	ret := &response.PlatformOverviewResponse{
		GeneratedAt:  formatTime(item.GeneratedAt),
		TopTenants:   make([]response.PlatformTopTenantResponse, 0, len(item.TopTenants)),
		DailyUsage:   make([]response.PlatformDailyUsageResponse, 0, len(item.DailyUsage)),
		RecentEvents: make([]response.PlatformRecentEventResponse, 0, len(item.RecentEvents)),
	}
	if item.Counts != nil {
		ret.Tenants = response.PlatformTenantSummaryResponse{
			Total: item.Counts.TenantTotal, Active: item.Counts.TenantActive,
			Trial: item.Counts.TenantTrial, Frozen: item.Counts.TenantFrozen,
			ExpiringSoon: item.Counts.TenantExpiring,
		}
		ret.Products = item.Counts.Products
		ret.Devices = item.Counts.Devices
		ret.Tickets = response.PlatformTicketSummaryResponse{
			Total: item.Counts.Tickets, Open: item.Counts.OpenTickets,
			CreatedToday: item.Counts.TicketsToday, SLARisk: item.Counts.SLARiskTickets,
		}
		ret.Knowledge = response.PlatformKnowledgeResponse{
			Documents: item.Counts.KnowledgeDocs,
			Indexed:   item.Counts.IndexedDocs,
			Pending:   item.Counts.PendingDocs,
		}
		ret.Meetings = response.PlatformMeetingResponse{
			Monthly: item.Counts.MonthlyMeetings,
			Active:  item.Counts.ActiveMeetings,
		}
	}
	if item.AIUsage != nil {
		ret.AIUsage = response.PlatformAIUsageResponse{
			Requests:   item.AIUsage.Requests,
			Tokens:     item.AIUsage.Tokens,
			CostAmount: item.AIUsage.CostAmount,
		}
	}
	for _, tenant := range item.TopTenants {
		usage := item.TopUsage[tenant.TenantID]
		ret.TopTenants = append(ret.TopTenants, response.PlatformTopTenantResponse{
			TenantID: tenant.TenantID, TenantName: tenant.TenantName,
			PlanName: item.TopPlans[tenant.TenantID], Devices: tenant.Devices,
			AIRequests: usage.Requests, AITokens: usage.Tokens,
		})
	}
	for _, row := range item.DailyUsage {
		ret.DailyUsage = append(ret.DailyUsage, response.PlatformDailyUsageResponse{
			Date: row.Date, AIRequests: row.Requests, AITokens: row.Tokens, CostAmount: row.CostAmount,
		})
	}
	for _, event := range item.RecentEvents {
		ret.RecentEvents = append(ret.RecentEvents, response.PlatformRecentEventResponse{
			ID: event.ID, TenantID: event.TenantID, TenantName: event.TenantName,
			Action: event.Action, TargetType: event.TargetType,
			RiskLevel: event.RiskLevel, Status: event.Status, OccurredAt: formatTime(event.OccurredAt),
		})
	}
	return ret
}

func BuildPlatformOverviewMetrics(item *services.PlatformOverviewAggregate) *response.PlatformOverviewMetricsResponse {
	ret := &response.PlatformOverviewMetricsResponse{GeneratedAt: formatTime(item.GeneratedAt)}
	if item.Counts != nil {
		ret.Tenants = response.PlatformTenantSummaryResponse{
			Total: item.Counts.TenantTotal, Active: item.Counts.TenantActive,
			Trial: item.Counts.TenantTrial, Frozen: item.Counts.TenantFrozen,
			ExpiringSoon: item.Counts.TenantExpiring,
		}
		ret.Products = item.Counts.Products
		ret.Devices = item.Counts.Devices
		ret.Tickets = response.PlatformTicketSummaryResponse{
			Total: item.Counts.Tickets, Open: item.Counts.OpenTickets,
			CreatedToday: item.Counts.TicketsToday, SLARisk: item.Counts.SLARiskTickets,
		}
		ret.Knowledge = response.PlatformKnowledgeResponse{
			Documents: item.Counts.KnowledgeDocs,
			Indexed:   item.Counts.IndexedDocs,
			Pending:   item.Counts.PendingDocs,
		}
		ret.Meetings = response.PlatformMeetingResponse{
			Monthly: item.Counts.MonthlyMeetings,
			Active:  item.Counts.ActiveMeetings,
		}
	}
	if item.AIUsage != nil {
		ret.AIUsage = response.PlatformAIUsageResponse{
			Requests:   item.AIUsage.Requests,
			Tokens:     item.AIUsage.Tokens,
			CostAmount: item.AIUsage.CostAmount,
		}
	}
	return ret
}

func BuildPlatformOverviewTopTenants(item *services.PlatformOverviewAggregate) *response.PlatformOverviewTopTenantsResponse {
	ret := &response.PlatformOverviewTopTenantsResponse{
		GeneratedAt: formatTime(item.GeneratedAt),
		TopTenants:  make([]response.PlatformTopTenantResponse, 0, len(item.TopTenants)),
	}
	for _, tenant := range item.TopTenants {
		usage := item.TopUsage[tenant.TenantID]
		ret.TopTenants = append(ret.TopTenants, response.PlatformTopTenantResponse{
			TenantID: tenant.TenantID, TenantName: tenant.TenantName,
			PlanName: item.TopPlans[tenant.TenantID], Devices: tenant.Devices,
			AIRequests: usage.Requests, AITokens: usage.Tokens,
		})
	}
	return ret
}

func BuildPlatformOverviewUsageTrend(item *services.PlatformOverviewAggregate) *response.PlatformOverviewUsageTrendResponse {
	ret := &response.PlatformOverviewUsageTrendResponse{
		GeneratedAt: formatTime(item.GeneratedAt),
		DailyUsage:  make([]response.PlatformDailyUsageResponse, 0, len(item.DailyUsage)),
	}
	if item.AIUsage != nil {
		ret.AIUsage = response.PlatformAIUsageResponse{
			Requests:   item.AIUsage.Requests,
			Tokens:     item.AIUsage.Tokens,
			CostAmount: item.AIUsage.CostAmount,
		}
	}
	for _, row := range item.DailyUsage {
		ret.DailyUsage = append(ret.DailyUsage, response.PlatformDailyUsageResponse{
			Date: row.Date, AIRequests: row.Requests, AITokens: row.Tokens, CostAmount: row.CostAmount,
		})
	}
	return ret
}

func BuildPlatformOverviewEvents(item *services.PlatformOverviewAggregate) *response.PlatformOverviewEventsResponse {
	ret := &response.PlatformOverviewEventsResponse{
		GeneratedAt:  formatTime(item.GeneratedAt),
		RecentEvents: make([]response.PlatformRecentEventResponse, 0, len(item.RecentEvents)),
	}
	for _, event := range item.RecentEvents {
		ret.RecentEvents = append(ret.RecentEvents, response.PlatformRecentEventResponse{
			ID: event.ID, TenantID: event.TenantID, TenantName: event.TenantName,
			Action: event.Action, TargetType: event.TargetType,
			RiskLevel: event.RiskLevel, Status: event.Status, OccurredAt: formatTime(event.OccurredAt),
		})
	}
	return ret
}

func BuildPlatformGlobalization(item *services.PlatformGlobalizationAggregate) *response.PlatformGlobalizationResponse {
	ret := &response.PlatformGlobalizationResponse{
		GeneratedAt:           formatTime(item.GeneratedAt),
		ConfiguredRegionCount: item.ConfiguredRegionCount,
		LocaleCount:           item.LocaleCount,
		TimezoneCount:         item.TimezoneCount,
		Regions:               make([]response.PlatformGlobalizationRegionResponse, 0, len(item.Regions)),
		Tenants:               make([]*response.PlatformTenantResponse, 0),
	}
	for _, region := range item.Regions {
		ret.Regions = append(ret.Regions, response.PlatformGlobalizationRegionResponse{
			DataRegion: region.DataRegion, TenantCount: region.TenantCount,
			CountryRegions: region.CountryRegions, Locales: region.Locales, Timezones: region.Timezones,
		})
	}
	if item.TenantPage == nil {
		return ret
	}
	ret.Tenants = BuildPlatformTenantList(
		item.TenantPage.Items, item.TenantPage.Subscriptions, item.TenantPage.Plans,
		item.TenantPage.DeviceCounts, item.TenantPage.MemberCounts, item.TenantPage.MonthlyUsage,
	)
	if item.TenantPage.Paging != nil {
		ret.TotalCount = item.TenantPage.Paging.Total
		ret.Page = item.TenantPage.Paging.Page
		ret.PageSize = item.TenantPage.Paging.Limit
		ret.TotalPages = item.TenantPage.Paging.TotalPage()
		ret.HasMore = ret.Page < ret.TotalPages
	}
	if item.TenantPage.Summary != nil {
		ret.Summary = response.PlatformTenantSummaryResponse{
			Total: item.TenantPage.Summary.Total, Active: item.TenantPage.Summary.Active,
			Trial: item.TenantPage.Summary.Trial, Frozen: item.TenantPage.Summary.Frozen,
			ExpiringSoon: item.TenantPage.Summary.ExpiringSoon,
		}
	}
	return ret
}

func BuildPlatformUsageOverview(item *services.PlatformUsageAggregate) *response.PlatformUsageOverviewResponse {
	ret := &response.PlatformUsageOverviewResponse{
		GeneratedAt: formatTime(item.GeneratedAt),
		PeriodStart: formatTime(item.PeriodStart),
		PeriodEnd:   formatTime(item.PeriodEnd),
		Plans:       make([]response.PlatformUsagePlanResponse, 0, len(item.Plans)),
		Tenants:     make([]response.PlatformTenantUsageResponse, 0, len(item.Tenants)),
		DailyUsage:  make([]response.PlatformDailyUsageResponse, 0, len(item.DailyUsage)),
	}
	ret.Summary.PlanCount = int64(len(item.Plans))
	ret.Summary.TenantCount = int64(len(item.Tenants))
	for _, plan := range item.Plans {
		quotas := make([]response.PlatformQuotaResponse, 0, len(plan.Quotas))
		for _, quota := range plan.Quotas {
			quotas = append(quotas, response.PlatformQuotaResponse{
				Key: quota.Key, Value: quota.Value, Unit: quota.Unit, Period: quota.Period,
			})
		}
		ret.Plans = append(ret.Plans, response.PlatformUsagePlanResponse{
			ID: plan.Plan.ID, Code: plan.Plan.Code, Name: plan.Plan.Name,
			PlanType: plan.Plan.PlanType, OverageStrategy: plan.Plan.OverageStrategy,
			Status: int(plan.Plan.Status), FeatureJSON: plan.Plan.FeatureJSON,
			QuotaTemplateJSON: plan.Plan.QuotaTemplateJSON, Quotas: quotas,
			TenantCount: plan.TenantCount, AtRiskTenants: plan.AtRiskTenants,
			OverLimitTenants: plan.OverLimitTenants,
			AIRequests:       plan.AIRequests, AITokens: plan.AITokens,
			Devices: plan.Devices, MeetingMinutes: plan.MeetingMinutes,
		})
	}
	for _, tenant := range item.Tenants {
		ret.Tenants = append(ret.Tenants, response.PlatformTenantUsageResponse{
			TenantID: tenant.TenantID, TenantName: tenant.TenantName,
			PlanID: tenant.PlanID, PlanName: tenant.PlanName,
			AIRequests: tenant.AIRequests, AIRequestLimit: tenant.AIRequestLimit,
			AITokens: tenant.AITokens, AITokenLimit: tenant.AITokenLimit,
			Devices: tenant.Devices, DeviceLimit: tenant.DeviceLimit,
			MeetingMinutes: tenant.MeetingMinutes, MeetingLimit: tenant.MeetingLimit,
			CostAmount: tenant.CostAmount, UsageRatio: tenant.UsageRatio,
			RiskLevel: tenant.RiskLevel, LastMeteredAt: formatTimePtr(tenant.LastMeteredAt),
		})
		ret.Summary.AIRequests += tenant.AIRequests
		ret.Summary.AITokens += tenant.AITokens
		ret.Summary.CostAmount += tenant.CostAmount
		if tenant.UsageRatio >= 0.8 {
			ret.Summary.AtRiskTenantCount++
		}
		if tenant.UsageRatio >= 1 {
			ret.Summary.OverLimitCount++
		}
	}
	ret.SharedTranslationUsage = response.PlatformSharedTranslationUsageResponse{
		Configured:    item.SharedTranslationUsage.Configured,
		APIKeyMasked:  item.SharedTranslationUsage.APIKeyMasked,
		Fingerprint:   item.SharedTranslationUsage.Fingerprint,
		APIKeyID:      item.SharedTranslationUsage.APIKeyID,
		Requests:      item.SharedTranslationUsage.Requests,
		Tokens:        item.SharedTranslationUsage.Tokens,
		CostAmount:    item.SharedTranslationUsage.CostAmount,
		LastMeteredAt: formatTimePtr(item.SharedTranslationUsage.LastMeteredAt),
	}
	for _, row := range item.DailyUsage {
		ret.DailyUsage = append(ret.DailyUsage, response.PlatformDailyUsageResponse{
			Date: row.Date, AIRequests: row.Requests, AITokens: row.Tokens, CostAmount: row.CostAmount,
		})
	}
	return ret
}

func BuildPlatformOpsOverview(item *services.PlatformOpsAggregate) *response.PlatformOpsOverviewResponse {
	ret := &response.PlatformOpsOverviewResponse{
		GeneratedAt:   item.GeneratedAt.Format("2006-01-02T15:04:05Z07:00"),
		UptimeSeconds: item.UptimeSeconds,
		Runtime: response.PlatformRuntimeResponse{
			GoVersion: item.GoVersion, Goroutines: item.Goroutines,
			HeapAllocBytes: item.HeapAlloc, HeapSysBytes: item.HeapSys, GCCount: item.GCCount,
		},
		SyncRuns:       make([]response.PlatformSyncRunResponse, 0, len(item.SyncRuns)),
		KnowledgeTasks: make([]response.PlatformKnowledgeTaskResponse, 0, len(item.KnowledgeTasks)),
	}
	if item.Database != nil {
		ret.Database = response.PlatformDatabaseHealthResponse{
			Status: "healthy", LatencyMs: item.Database.LatencyMs,
			OpenConnections: item.Database.Stats.OpenConnections,
			InUse:           item.Database.Stats.InUse, Idle: item.Database.Stats.Idle,
			MaxOpen: item.Database.Stats.MaxOpenConnections,
		}
	} else {
		ret.Database = response.PlatformDatabaseHealthResponse{Status: "unhealthy", Error: item.DatabaseError}
	}
	if item.Workloads != nil {
		ret.Workloads = response.PlatformWorkloadHealthResponse{
			ActiveMeetings:      item.Workloads.ActiveMeetings,
			OpenTickets:         item.Workloads.OpenTickets,
			PendingKnowledge:    item.Workloads.PendingKnowledge,
			UnhealthyConnectors: item.Workloads.UnhealthyConnectors,
		}
		ret.Pipeline = response.PlatformPipelineHealthResponse{
			LastUsageEventAt: formatTimePtr(item.Workloads.LastUsageEventAt),
			LastAggregateAt:  formatTimePtr(item.Workloads.LastAggregateAt),
			RunningSyncs:     item.Workloads.RunningSyncs,
			FailedSyncs24h:   item.Workloads.FailedSyncs24h,
		}
	}
	if item.KnowledgeIndex != nil {
		ret.KnowledgeIndex = response.PlatformKnowledgeIndexResponse{
			PendingTasks:   item.KnowledgeIndex.PendingTasks,
			RunningTasks:   item.KnowledgeIndex.RunningTasks,
			FailedTasks24h: item.KnowledgeIndex.FailedTasks24h,
			LastFinishedAt: formatTimePtr(item.KnowledgeIndex.LastFinishedAt),
		}
	}
	if item.Sub2API != nil {
		ret.Sub2API = response.PlatformSub2APIHealthResponse{
			TenantAccounts:       item.Sub2API.TenantAccounts,
			HealthyAccounts:      item.Sub2API.HealthyAccounts,
			AttentionAccounts:    item.Sub2API.AttentionAccounts,
			ProductCredentials:   item.Sub2API.ProductCredentials,
			StaleCredentials24h:  item.Sub2API.StaleCredentials24h,
			LastUsageEventAt:     formatTimePtr(item.Sub2API.LastUsageEventAt),
			LastCredentialSyncAt: formatTimePtr(item.Sub2API.LastCredentialSyncAt),
		}
	}
	if item.Jitsi != nil {
		ret.Jitsi = response.PlatformJitsiHealthResponse{
			ActiveMeetings: item.Jitsi.ActiveMeetings, Meetings24h: item.Jitsi.Meetings24h,
			ParticipantMinutes24h: item.Jitsi.ParticipantMinutes24h,
			OnlineParticipants:    item.Jitsi.OnlineParticipants, StaleParticipants: item.Jitsi.StaleParticipants,
			StaleMeetings: item.Jitsi.StaleMeetings, HeartbeatStatus: item.Jitsi.HeartbeatStatus,
			HeartbeatTimeoutSeconds: item.Jitsi.HeartbeatTimeoutSeconds,
			LastMeetingAt:           formatTimePtr(item.Jitsi.LastMeetingAt), LastHeartbeatAt: formatTimePtr(item.Jitsi.LastHeartbeatAt),
			ServiceStatus:   item.Jitsi.ServiceStatus,
			ServiceURL:      item.Jitsi.ServiceURL,
			ProbeURL:        item.Jitsi.ProbeURL,
			ProbeHTTPStatus: item.Jitsi.ProbeHTTPStatus,
			ProbeLatencyMs:  item.Jitsi.ProbeLatencyMs,
			ProbeCheckedAt:  formatTimePtr(item.Jitsi.ProbeCheckedAt),
			ProbeError:      item.Jitsi.ProbeError,
		}
	}
	if item.Integrations != nil {
		ret.Integrations = response.PlatformIntegrationHealthResponse{
			SpeechProvider:        item.Integrations.SpeechProvider,
			SpeechConfigured:      item.Integrations.SpeechConfigured,
			JigasiEnabled:         item.Integrations.JigasiEnabled,
			TranslationProvider:   item.Integrations.TranslationProvider,
			TranslationConfigured: item.Integrations.TranslationConfigured,
			ARProvider:            item.Integrations.ARProvider,
			ARConfigured:          item.Integrations.ARConfigured,
			SMTPConfigured:        item.Integrations.SMTPConfigured,
		}
	}
	if item.Access != nil {
		ret.Access = response.PlatformAccessHealthResponse{
			TotalConnectors:     item.Access.TotalConnectors,
			ActiveConnectors:    item.Access.ActiveConnectors,
			HealthyConnectors:   item.Access.HealthyConnectors,
			DegradedConnectors:  item.Access.DegradedConnectors,
			UnhealthyConnectors: item.Access.UnhealthyConnectors,
			FailedCalls24h:      item.Access.FailedCalls24h,
			LastTestedAt:        formatTimePtr(item.Access.LastTestedAt),
		}
	}
	if item.Notifications != nil {
		ret.Notifications = response.PlatformNotificationHealthResponse{
			PendingNotifications: item.Notifications.PendingNotifications,
			UnreadNotifications:  item.Notifications.UnreadNotifications,
			FailedDeliveries24h:  item.Notifications.FailedDeliveries24h,
			SentDeliveries24h:    item.Notifications.SentDeliveries24h,
			LastDeliveredAt:      formatTimePtr(item.Notifications.LastDeliveredAt),
		}
	}
	for _, run := range item.SyncRuns {
		ret.SyncRuns = append(ret.SyncRuns, response.PlatformSyncRunResponse{
			ID: run.ID, TenantID: run.TenantID, TenantName: run.TenantName,
			SyncType: run.SyncType, Status: run.Status,
			RecordsProcessed: run.RecordsProcessed, ErrorMessage: run.ErrorMessage,
			StartedAt: formatTime(run.StartedAt), EndedAt: formatTimePtr(run.EndedAt),
		})
	}
	for _, task := range item.KnowledgeTasks {
		ret.KnowledgeTasks = append(ret.KnowledgeTasks, response.PlatformKnowledgeTaskResponse{
			ID:           task.ID,
			TenantID:     task.TenantID,
			TenantName:   task.TenantName,
			ProductID:    task.ProductID,
			ProductName:  task.ProductName,
			SubjectType:  task.SubjectType,
			SubjectID:    task.SubjectID,
			ProviderType: task.ProviderType,
			Action:       task.Action,
			Status:       task.Status,
			RetryCount:   task.RetryCount,
			ErrorSummary: task.ErrorSummary,
			UpdatedAt:    formatTime(task.UpdatedAt),
		})
	}
	return ret
}

func BuildPlatformOpsRuntime(item *services.PlatformOpsRuntimeAggregate) *response.PlatformOpsRuntimeResponse {
	return &response.PlatformOpsRuntimeResponse{
		GeneratedAt:   formatTime(item.GeneratedAt),
		UptimeSeconds: item.UptimeSeconds,
		Runtime: response.PlatformRuntimeResponse{
			GoVersion: item.GoVersion, Goroutines: item.Goroutines,
			HeapAllocBytes: item.HeapAlloc, HeapSysBytes: item.HeapSys, GCCount: item.GCCount,
		},
	}
}

func BuildPlatformOpsDatabase(item *services.PlatformOpsDatabaseAggregate) *response.PlatformOpsDatabaseResponse {
	ret := &response.PlatformOpsDatabaseResponse{GeneratedAt: formatTime(item.GeneratedAt)}
	if item.Database == nil {
		ret.Database = response.PlatformDatabaseHealthResponse{Status: "unhealthy", Error: item.DatabaseError}
		return ret
	}
	ret.Database = response.PlatformDatabaseHealthResponse{
		Status: "healthy", LatencyMs: item.Database.LatencyMs,
		OpenConnections: item.Database.Stats.OpenConnections,
		InUse:           item.Database.Stats.InUse, Idle: item.Database.Stats.Idle,
		MaxOpen: item.Database.Stats.MaxOpenConnections,
	}
	return ret
}

func BuildPlatformOpsInfrastructure(item *services.PlatformOpsInfrastructureAggregate) *response.PlatformOpsInfrastructureResponse {
	ret := &response.PlatformOpsInfrastructureResponse{
		GeneratedAt: formatTime(item.GeneratedAt), TotalMeasuredBytes: item.TotalMeasuredBytes,
		Dependencies:   make([]response.PlatformInfrastructureDependencyResponse, 0, len(item.Dependencies)),
		DatabaseTables: make([]response.PlatformDatabaseTableStorageResponse, 0, len(item.DatabaseTables)),
	}
	for _, dependency := range item.Dependencies {
		ret.Dependencies = append(ret.Dependencies, response.PlatformInfrastructureDependencyResponse{
			Key: dependency.Key, Name: dependency.Name, Provider: dependency.Provider,
			Status: dependency.Status, Endpoint: dependency.Endpoint, LatencyMs: dependency.LatencyMs,
			StoredBytes: dependency.StoredBytes, AllocatedBytes: dependency.AllocatedBytes,
			CapacityBytes: dependency.CapacityBytes, SharePercent: dependency.SharePercent,
			CapacityPercent: dependency.CapacityPercent, ItemCount: dependency.ItemCount,
			ItemUnit: dependency.ItemUnit, SecondaryValue: dependency.SecondaryValue,
			SecondaryLabel: dependency.SecondaryLabel, Estimated: dependency.Estimated,
			Measurement: dependency.Measurement, Error: dependency.Error,
		})
	}
	databaseBytes := int64(0)
	for _, dependency := range item.Dependencies {
		if dependency.Key == "database" {
			databaseBytes = dependency.StoredBytes
			break
		}
	}
	for _, table := range item.DatabaseTables {
		share := float64(0)
		if databaseBytes > 0 {
			share = float64(table.SizeBytes) / float64(databaseBytes) * 100
		}
		ret.DatabaseTables = append(ret.DatabaseTables, response.PlatformDatabaseTableStorageResponse{
			Name: table.Name, SizeBytes: table.SizeBytes, EstimatedRows: table.EstimatedRows,
			SharePercent: share,
		})
	}
	return ret
}

func BuildPlatformOpsJitsi(item *services.PlatformOpsJitsiAggregate) *response.PlatformOpsJitsiResponse {
	ret := &response.PlatformOpsJitsiResponse{GeneratedAt: formatTime(item.GeneratedAt)}
	if item.Jitsi == nil {
		return ret
	}
	ret.Jitsi = response.PlatformJitsiHealthResponse{
		ActiveMeetings: item.Jitsi.ActiveMeetings, Meetings24h: item.Jitsi.Meetings24h,
		ParticipantMinutes24h: item.Jitsi.ParticipantMinutes24h,
		OnlineParticipants:    item.Jitsi.OnlineParticipants, StaleParticipants: item.Jitsi.StaleParticipants,
		StaleMeetings: item.Jitsi.StaleMeetings, HeartbeatStatus: item.Jitsi.HeartbeatStatus,
		HeartbeatTimeoutSeconds: item.Jitsi.HeartbeatTimeoutSeconds,
		LastMeetingAt:           formatTimePtr(item.Jitsi.LastMeetingAt), LastHeartbeatAt: formatTimePtr(item.Jitsi.LastHeartbeatAt),
		ServiceStatus: item.Jitsi.ServiceStatus, ServiceURL: item.Jitsi.ServiceURL,
		ProbeURL: item.Jitsi.ProbeURL, ProbeHTTPStatus: item.Jitsi.ProbeHTTPStatus,
		ProbeLatencyMs: item.Jitsi.ProbeLatencyMs, ProbeCheckedAt: formatTimePtr(item.Jitsi.ProbeCheckedAt),
		ProbeError: item.Jitsi.ProbeError,
	}
	return ret
}

func BuildPlatformOpsSpeech(item *services.PlatformOpsSpeechAggregate) *response.PlatformOpsSpeechResponse {
	return &response.PlatformOpsSpeechResponse{
		GeneratedAt: formatTime(item.GeneratedAt),
		Speech: response.PlatformSpeechHealthResponse{
			Provider: item.Provider, Configured: item.Configured, JigasiEnabled: item.JigasiEnabled,
		},
	}
}

func BuildPlatformOpsTranslation(item *services.PlatformOpsTranslationAggregate) *response.PlatformOpsTranslationResponse {
	return &response.PlatformOpsTranslationResponse{
		GeneratedAt: formatTime(item.GeneratedAt),
		Translation: response.PlatformTranslationHealthResponse{
			Provider: item.Provider, Configured: item.Configured,
		},
	}
}

func BuildPlatformOpsAR(item *services.PlatformOpsARAggregate) *response.PlatformOpsARResponse {
	return &response.PlatformOpsARResponse{
		GeneratedAt: formatTime(item.GeneratedAt),
		AR: response.PlatformARHealthResponse{
			Provider: item.Provider, Configured: item.Configured,
		},
	}
}

func BuildPlatformOpsSub2API(item *services.PlatformOpsSub2APIAggregate) *response.PlatformOpsSub2APIResponse {
	ret := &response.PlatformOpsSub2APIResponse{GeneratedAt: formatTime(item.GeneratedAt)}
	if item.Sub2API != nil {
		ret.Sub2API = response.PlatformSub2APIHealthResponse{
			TenantAccounts: item.Sub2API.TenantAccounts, HealthyAccounts: item.Sub2API.HealthyAccounts,
			AttentionAccounts: item.Sub2API.AttentionAccounts, ProductCredentials: item.Sub2API.ProductCredentials,
			StaleCredentials24h:  item.Sub2API.StaleCredentials24h,
			LastUsageEventAt:     formatTimePtr(item.Sub2API.LastUsageEventAt),
			LastCredentialSyncAt: formatTimePtr(item.Sub2API.LastCredentialSyncAt),
		}
	}
	return ret
}

func BuildPlatformOpsAccess(item *services.PlatformOpsAccessAggregate) *response.PlatformOpsAccessResponse {
	ret := &response.PlatformOpsAccessResponse{GeneratedAt: formatTime(item.GeneratedAt)}
	if item.Access != nil {
		ret.Access = response.PlatformAccessHealthResponse{
			TotalConnectors: item.Access.TotalConnectors, ActiveConnectors: item.Access.ActiveConnectors,
			HealthyConnectors: item.Access.HealthyConnectors, DegradedConnectors: item.Access.DegradedConnectors,
			UnhealthyConnectors: item.Access.UnhealthyConnectors,
			FailedCalls24h:      item.Access.FailedCalls24h, LastTestedAt: formatTimePtr(item.Access.LastTestedAt),
		}
	}
	return ret
}

func BuildPlatformOpsPipeline(item *services.PlatformOpsPipelineAggregate) *response.PlatformOpsPipelineResponse {
	ret := &response.PlatformOpsPipelineResponse{GeneratedAt: formatTime(item.GeneratedAt)}
	if item.Pipeline != nil {
		ret.Pipeline = response.PlatformPipelineHealthResponse{
			LastUsageEventAt: formatTimePtr(item.Pipeline.LastUsageEventAt),
			LastAggregateAt:  formatTimePtr(item.Pipeline.LastAggregateAt),
			RunningSyncs:     item.Pipeline.RunningSyncs, FailedSyncs24h: item.Pipeline.FailedSyncs24h,
		}
	}
	return ret
}

func BuildPlatformOpsKnowledgeQueue(item *services.PlatformOpsKnowledgeQueueAggregate) *response.PlatformOpsKnowledgeQueueResponse {
	ret := &response.PlatformOpsKnowledgeQueueResponse{GeneratedAt: formatTime(item.GeneratedAt)}
	if item.KnowledgeIndex != nil {
		ret.KnowledgeIndex = response.PlatformKnowledgeIndexResponse{
			PendingTasks: item.KnowledgeIndex.PendingTasks, RunningTasks: item.KnowledgeIndex.RunningTasks,
			FailedTasks24h: item.KnowledgeIndex.FailedTasks24h,
			LastFinishedAt: formatTimePtr(item.KnowledgeIndex.LastFinishedAt),
		}
	}
	return ret
}

func BuildPlatformOpsNotificationQueue(item *services.PlatformOpsNotificationQueueAggregate) *response.PlatformOpsNotificationQueueResponse {
	ret := &response.PlatformOpsNotificationQueueResponse{GeneratedAt: formatTime(item.GeneratedAt)}
	if item.Notifications != nil {
		ret.Notifications = response.PlatformNotificationHealthResponse{
			PendingNotifications: item.Notifications.PendingNotifications,
			UnreadNotifications:  item.Notifications.UnreadNotifications,
			FailedDeliveries24h:  item.Notifications.FailedDeliveries24h,
			SentDeliveries24h:    item.Notifications.SentDeliveries24h,
			LastDeliveredAt:      formatTimePtr(item.Notifications.LastDeliveredAt),
		}
	}
	return ret
}
