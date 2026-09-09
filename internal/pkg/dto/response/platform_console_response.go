package response

type PlatformTenantSummaryResponse struct {
	Total        int64 `json:"total"`
	Active       int64 `json:"active"`
	Trial        int64 `json:"trial"`
	Frozen       int64 `json:"frozen"`
	ExpiringSoon int64 `json:"expiringSoon"`
}

type PlatformOverviewResponse struct {
	GeneratedAt  string                        `json:"generatedAt"`
	Tenants      PlatformTenantSummaryResponse `json:"tenants"`
	Products     int64                         `json:"products"`
	Devices      int64                         `json:"devices"`
	Tickets      PlatformTicketSummaryResponse `json:"tickets"`
	Knowledge    PlatformKnowledgeResponse     `json:"knowledge"`
	AIUsage      PlatformAIUsageResponse       `json:"aiUsage"`
	Meetings     PlatformMeetingResponse       `json:"meetings"`
	TopTenants   []PlatformTopTenantResponse   `json:"topTenants"`
	DailyUsage   []PlatformDailyUsageResponse  `json:"dailyUsage"`
	RecentEvents []PlatformRecentEventResponse `json:"recentEvents"`
}

type PlatformOverviewMetricsResponse struct {
	GeneratedAt string                        `json:"generatedAt"`
	Tenants     PlatformTenantSummaryResponse `json:"tenants"`
	Products    int64                         `json:"products"`
	Devices     int64                         `json:"devices"`
	Tickets     PlatformTicketSummaryResponse `json:"tickets"`
	Knowledge   PlatformKnowledgeResponse     `json:"knowledge"`
	AIUsage     PlatformAIUsageResponse       `json:"aiUsage"`
	Meetings    PlatformMeetingResponse       `json:"meetings"`
}

type PlatformOverviewTopTenantsResponse struct {
	GeneratedAt string                      `json:"generatedAt"`
	TopTenants  []PlatformTopTenantResponse `json:"topTenants"`
}

type PlatformOverviewUsageTrendResponse struct {
	GeneratedAt string                       `json:"generatedAt"`
	AIUsage     PlatformAIUsageResponse      `json:"aiUsage"`
	DailyUsage  []PlatformDailyUsageResponse `json:"dailyUsage"`
}

type PlatformOverviewEventsResponse struct {
	GeneratedAt  string                        `json:"generatedAt"`
	RecentEvents []PlatformRecentEventResponse `json:"recentEvents"`
}

type PlatformGlobalizationRegionResponse struct {
	DataRegion     string   `json:"dataRegion"`
	TenantCount    int64    `json:"tenantCount"`
	CountryRegions []string `json:"countryRegions"`
	Locales        []string `json:"locales"`
	Timezones      []string `json:"timezones"`
}

type PlatformGlobalizationResponse struct {
	GeneratedAt           string                                `json:"generatedAt"`
	TotalCount            int64                                 `json:"totalCount"`
	Summary               PlatformTenantSummaryResponse         `json:"summary"`
	ConfiguredRegionCount int                                   `json:"configuredRegionCount"`
	LocaleCount           int                                   `json:"localeCount"`
	TimezoneCount         int                                   `json:"timezoneCount"`
	Regions               []PlatformGlobalizationRegionResponse `json:"regions"`
	Tenants               []*PlatformTenantResponse             `json:"tenants"`
	Page                  int                                   `json:"page"`
	PageSize              int                                   `json:"pageSize"`
	TotalPages            int                                   `json:"totalPages"`
	HasMore               bool                                  `json:"hasMore"`
}

type PlatformTicketSummaryResponse struct {
	Total        int64 `json:"total"`
	Open         int64 `json:"open"`
	CreatedToday int64 `json:"createdToday"`
	SLARisk      int64 `json:"slaRisk"`
}

type PlatformKnowledgeResponse struct {
	Documents int64 `json:"documents"`
	Indexed   int64 `json:"indexed"`
	Pending   int64 `json:"pending"`
}

type PlatformAIUsageResponse struct {
	Requests   int64   `json:"requests"`
	Tokens     int64   `json:"tokens"`
	CostAmount float64 `json:"costAmount"`
}

type PlatformMeetingResponse struct {
	Monthly int64 `json:"monthly"`
	Active  int64 `json:"active"`
}

type PlatformTopTenantResponse struct {
	TenantID   int64  `json:"tenantId"`
	TenantName string `json:"tenantName"`
	PlanName   string `json:"planName"`
	Devices    int64  `json:"devices"`
	AIRequests int64  `json:"aiRequests"`
	AITokens   int64  `json:"aiTokens"`
}

type PlatformDailyUsageResponse struct {
	Date       string  `json:"date"`
	AIRequests int64   `json:"aiRequests"`
	AITokens   int64   `json:"aiTokens"`
	CostAmount float64 `json:"costAmount"`
}

type PlatformRecentEventResponse struct {
	ID         int64  `json:"id"`
	TenantID   int64  `json:"tenantId"`
	TenantName string `json:"tenantName"`
	Action     string `json:"action"`
	TargetType string `json:"targetType"`
	RiskLevel  string `json:"riskLevel"`
	Status     string `json:"status"`
	OccurredAt string `json:"occurredAt"`
}

type PlatformUsageOverviewResponse struct {
	GeneratedAt            string                                 `json:"generatedAt"`
	PeriodStart            string                                 `json:"periodStart"`
	PeriodEnd              string                                 `json:"periodEnd"`
	Summary                PlatformUsageSummaryResponse           `json:"summary"`
	SharedTranslationUsage PlatformSharedTranslationUsageResponse `json:"sharedTranslationUsage"`
	Plans                  []PlatformUsagePlanResponse            `json:"plans"`
	Tenants                []PlatformTenantUsageResponse          `json:"tenants"`
	DailyUsage             []PlatformDailyUsageResponse           `json:"dailyUsage"`
}

type PlatformUsageSummaryResponse struct {
	TenantCount       int64   `json:"tenantCount"`
	PlanCount         int64   `json:"planCount"`
	AIRequests        int64   `json:"aiRequests"`
	AITokens          int64   `json:"aiTokens"`
	CostAmount        float64 `json:"costAmount"`
	AtRiskTenantCount int64   `json:"atRiskTenantCount"`
	OverLimitCount    int64   `json:"overLimitCount"`
}

type PlatformSharedTranslationUsageResponse struct {
	Configured    bool    `json:"configured"`
	APIKeyMasked  string  `json:"apiKeyMasked"`
	Fingerprint   string  `json:"fingerprint"`
	APIKeyID      string  `json:"apiKeyId"`
	Requests      int64   `json:"requests"`
	Tokens        int64   `json:"tokens"`
	CostAmount    float64 `json:"costAmount"`
	LastMeteredAt string  `json:"lastMeteredAt"`
}

type PlatformQuotaResponse struct {
	Key    string `json:"key"`
	Value  int64  `json:"value"`
	Unit   string `json:"unit"`
	Period string `json:"period"`
}

type PlatformUsagePlanResponse struct {
	ID                int64                   `json:"id"`
	Code              string                  `json:"code"`
	Name              string                  `json:"name"`
	PlanType          string                  `json:"planType"`
	OverageStrategy   string                  `json:"overageStrategy"`
	Status            int                     `json:"status"`
	FeatureJSON       string                  `json:"featureJson"`
	QuotaTemplateJSON string                  `json:"quotaTemplateJson"`
	Quotas            []PlatformQuotaResponse `json:"quotas"`
	TenantCount       int64                   `json:"tenantCount"`
	AtRiskTenants     int64                   `json:"atRiskTenants"`
	OverLimitTenants  int64                   `json:"overLimitTenants"`
	AIRequests        int64                   `json:"aiRequests"`
	AITokens          int64                   `json:"aiTokens"`
	Devices           int64                   `json:"devices"`
	MeetingMinutes    int64                   `json:"meetingMinutes"`
}

type PlatformTenantUsageResponse struct {
	TenantID       int64   `json:"tenantId"`
	TenantName     string  `json:"tenantName"`
	PlanID         int64   `json:"planId"`
	PlanName       string  `json:"planName"`
	AIRequests     int64   `json:"aiRequests"`
	AIRequestLimit int64   `json:"aiRequestLimit"`
	AITokens       int64   `json:"aiTokens"`
	AITokenLimit   int64   `json:"aiTokenLimit"`
	Devices        int64   `json:"devices"`
	DeviceLimit    int64   `json:"deviceLimit"`
	MeetingMinutes int64   `json:"meetingMinutes"`
	MeetingLimit   int64   `json:"meetingLimit"`
	CostAmount     float64 `json:"costAmount"`
	UsageRatio     float64 `json:"usageRatio"`
	RiskLevel      string  `json:"riskLevel"`
	LastMeteredAt  string  `json:"lastMeteredAt"`
}

type PlatformOpsOverviewResponse struct {
	GeneratedAt    string                             `json:"generatedAt"`
	UptimeSeconds  int64                              `json:"uptimeSeconds"`
	Runtime        PlatformRuntimeResponse            `json:"runtime"`
	Database       PlatformDatabaseHealthResponse     `json:"database"`
	Workloads      PlatformWorkloadHealthResponse     `json:"workloads"`
	Pipeline       PlatformPipelineHealthResponse     `json:"pipeline"`
	KnowledgeIndex PlatformKnowledgeIndexResponse     `json:"knowledgeIndex"`
	Sub2API        PlatformSub2APIHealthResponse      `json:"sub2api"`
	Jitsi          PlatformJitsiHealthResponse        `json:"jitsi"`
	Integrations   PlatformIntegrationHealthResponse  `json:"integrations"`
	Access         PlatformAccessHealthResponse       `json:"access"`
	Notifications  PlatformNotificationHealthResponse `json:"notifications"`
	SyncRuns       []PlatformSyncRunResponse          `json:"syncRuns"`
	KnowledgeTasks []PlatformKnowledgeTaskResponse    `json:"knowledgeTasks"`
}

type PlatformOpsRuntimeResponse struct {
	GeneratedAt   string                  `json:"generatedAt"`
	UptimeSeconds int64                   `json:"uptimeSeconds"`
	Runtime       PlatformRuntimeResponse `json:"runtime"`
}

type PlatformOpsDatabaseResponse struct {
	GeneratedAt string                         `json:"generatedAt"`
	Database    PlatformDatabaseHealthResponse `json:"database"`
}

type PlatformOpsInfrastructureResponse struct {
	GeneratedAt        string                                     `json:"generatedAt"`
	TotalMeasuredBytes int64                                      `json:"totalMeasuredBytes"`
	Dependencies       []PlatformInfrastructureDependencyResponse `json:"dependencies"`
	DatabaseTables     []PlatformDatabaseTableStorageResponse     `json:"databaseTables"`
}

type PlatformInfrastructureDependencyResponse struct {
	Key             string  `json:"key"`
	Name            string  `json:"name"`
	Provider        string  `json:"provider"`
	Status          string  `json:"status"`
	Endpoint        string  `json:"endpoint"`
	LatencyMs       int64   `json:"latencyMs"`
	StoredBytes     int64   `json:"storedBytes"`
	AllocatedBytes  int64   `json:"allocatedBytes"`
	CapacityBytes   int64   `json:"capacityBytes"`
	SharePercent    float64 `json:"sharePercent"`
	CapacityPercent float64 `json:"capacityPercent"`
	ItemCount       int64   `json:"itemCount"`
	ItemUnit        string  `json:"itemUnit"`
	SecondaryValue  int64   `json:"secondaryValue"`
	SecondaryLabel  string  `json:"secondaryLabel"`
	Estimated       bool    `json:"estimated"`
	Measurement     string  `json:"measurement"`
	Error           string  `json:"error"`
}

type PlatformDatabaseTableStorageResponse struct {
	Name          string  `json:"name"`
	SizeBytes     int64   `json:"sizeBytes"`
	EstimatedRows int64   `json:"estimatedRows"`
	SharePercent  float64 `json:"sharePercent"`
}

type PlatformOpsJitsiResponse struct {
	GeneratedAt string                      `json:"generatedAt"`
	Jitsi       PlatformJitsiHealthResponse `json:"jitsi"`
}

type PlatformOpsSpeechResponse struct {
	GeneratedAt string                       `json:"generatedAt"`
	Speech      PlatformSpeechHealthResponse `json:"speech"`
}

type PlatformOpsTranslationResponse struct {
	GeneratedAt string                            `json:"generatedAt"`
	Translation PlatformTranslationHealthResponse `json:"translation"`
}

type PlatformOpsARResponse struct {
	GeneratedAt string                   `json:"generatedAt"`
	AR          PlatformARHealthResponse `json:"ar"`
}

type PlatformOpsSub2APIResponse struct {
	GeneratedAt string                        `json:"generatedAt"`
	Sub2API     PlatformSub2APIHealthResponse `json:"sub2api"`
}

type PlatformOpsAccessResponse struct {
	GeneratedAt string                       `json:"generatedAt"`
	Access      PlatformAccessHealthResponse `json:"access"`
}

type PlatformOpsPipelineResponse struct {
	GeneratedAt string                         `json:"generatedAt"`
	Pipeline    PlatformPipelineHealthResponse `json:"pipeline"`
}

type PlatformOpsKnowledgeQueueResponse struct {
	GeneratedAt    string                         `json:"generatedAt"`
	KnowledgeIndex PlatformKnowledgeIndexResponse `json:"knowledgeIndex"`
}

type PlatformOpsNotificationQueueResponse struct {
	GeneratedAt   string                             `json:"generatedAt"`
	Notifications PlatformNotificationHealthResponse `json:"notifications"`
}

type PlatformRuntimeResponse struct {
	GoVersion      string `json:"goVersion"`
	Goroutines     int    `json:"goroutines"`
	HeapAllocBytes uint64 `json:"heapAllocBytes"`
	HeapSysBytes   uint64 `json:"heapSysBytes"`
	GCCount        uint32 `json:"gcCount"`
}

type PlatformDatabaseHealthResponse struct {
	Status          string `json:"status"`
	LatencyMs       int64  `json:"latencyMs"`
	OpenConnections int    `json:"openConnections"`
	InUse           int    `json:"inUse"`
	Idle            int    `json:"idle"`
	MaxOpen         int    `json:"maxOpen"`
	Error           string `json:"error"`
}

type PlatformWorkloadHealthResponse struct {
	ActiveMeetings      int64 `json:"activeMeetings"`
	OpenTickets         int64 `json:"openTickets"`
	PendingKnowledge    int64 `json:"pendingKnowledge"`
	UnhealthyConnectors int64 `json:"unhealthyConnectors"`
}

type PlatformPipelineHealthResponse struct {
	LastUsageEventAt string `json:"lastUsageEventAt"`
	LastAggregateAt  string `json:"lastAggregateAt"`
	RunningSyncs     int64  `json:"runningSyncs"`
	FailedSyncs24h   int64  `json:"failedSyncs24h"`
}

type PlatformSyncRunResponse struct {
	ID               int64  `json:"id"`
	TenantID         int64  `json:"tenantId"`
	TenantName       string `json:"tenantName"`
	SyncType         string `json:"syncType"`
	Status           string `json:"status"`
	RecordsProcessed int64  `json:"recordsProcessed"`
	ErrorMessage     string `json:"errorMessage"`
	StartedAt        string `json:"startedAt"`
	EndedAt          string `json:"endedAt"`
}

type PlatformKnowledgeIndexResponse struct {
	PendingTasks   int64  `json:"pendingTasks"`
	RunningTasks   int64  `json:"runningTasks"`
	FailedTasks24h int64  `json:"failedTasks24h"`
	LastFinishedAt string `json:"lastFinishedAt"`
}

type PlatformSub2APIHealthResponse struct {
	TenantAccounts       int64  `json:"tenantAccounts"`
	HealthyAccounts      int64  `json:"healthyAccounts"`
	AttentionAccounts    int64  `json:"attentionAccounts"`
	ProductCredentials   int64  `json:"productCredentials"`
	StaleCredentials24h  int64  `json:"staleCredentials24h"`
	LastUsageEventAt     string `json:"lastUsageEventAt"`
	LastCredentialSyncAt string `json:"lastCredentialSyncAt"`
}

type PlatformJitsiHealthResponse struct {
	ActiveMeetings          int64  `json:"activeMeetings"`
	Meetings24h             int64  `json:"meetings24h"`
	ParticipantMinutes24h   int64  `json:"participantMinutes24h"`
	OnlineParticipants      int64  `json:"onlineParticipants"`
	StaleParticipants       int64  `json:"staleParticipants"`
	StaleMeetings           int64  `json:"staleMeetings"`
	HeartbeatStatus         string `json:"heartbeatStatus"`
	HeartbeatTimeoutSeconds int64  `json:"heartbeatTimeoutSeconds"`
	LastMeetingAt           string `json:"lastMeetingAt"`
	LastHeartbeatAt         string `json:"lastHeartbeatAt"`
	ServiceStatus           string `json:"serviceStatus"`
	ServiceURL              string `json:"serviceUrl"`
	ProbeURL                string `json:"probeUrl"`
	ProbeHTTPStatus         int    `json:"probeHttpStatus"`
	ProbeLatencyMs          int64  `json:"probeLatencyMs"`
	ProbeCheckedAt          string `json:"probeCheckedAt"`
	ProbeError              string `json:"probeError"`
}

type PlatformIntegrationHealthResponse struct {
	SpeechProvider        string `json:"speechProvider"`
	SpeechConfigured      bool   `json:"speechConfigured"`
	JigasiEnabled         bool   `json:"jigasiEnabled"`
	TranslationProvider   string `json:"translationProvider"`
	TranslationConfigured bool   `json:"translationConfigured"`
	ARProvider            string `json:"arProvider"`
	ARConfigured          bool   `json:"arConfigured"`
	SMTPConfigured        bool   `json:"smtpConfigured"`
}

type PlatformSpeechHealthResponse struct {
	Provider      string `json:"provider"`
	Configured    bool   `json:"configured"`
	JigasiEnabled bool   `json:"jigasiEnabled"`
}

type PlatformTranslationHealthResponse struct {
	Provider   string `json:"provider"`
	Configured bool   `json:"configured"`
}

type PlatformARHealthResponse struct {
	Provider   string `json:"provider"`
	Configured bool   `json:"configured"`
}

type PlatformAccessHealthResponse struct {
	TotalConnectors     int64  `json:"totalConnectors"`
	ActiveConnectors    int64  `json:"activeConnectors"`
	HealthyConnectors   int64  `json:"healthyConnectors"`
	DegradedConnectors  int64  `json:"degradedConnectors"`
	UnhealthyConnectors int64  `json:"unhealthyConnectors"`
	FailedCalls24h      int64  `json:"failedCalls24h"`
	LastTestedAt        string `json:"lastTestedAt"`
}

type PlatformNotificationHealthResponse struct {
	PendingNotifications int64  `json:"pendingNotifications"`
	UnreadNotifications  int64  `json:"unreadNotifications"`
	FailedDeliveries24h  int64  `json:"failedDeliveries24h"`
	SentDeliveries24h    int64  `json:"sentDeliveries24h"`
	LastDeliveredAt      string `json:"lastDeliveredAt"`
}

type PlatformKnowledgeTaskResponse struct {
	ID           int64  `json:"id"`
	TenantID     int64  `json:"tenantId"`
	TenantName   string `json:"tenantName"`
	ProductID    int64  `json:"productId"`
	ProductName  string `json:"productName"`
	SubjectType  string `json:"subjectType"`
	SubjectID    int64  `json:"subjectId"`
	ProviderType string `json:"providerType"`
	Action       string `json:"action"`
	Status       string `json:"status"`
	RetryCount   int    `json:"retryCount"`
	ErrorSummary string `json:"errorSummary"`
	UpdatedAt    string `json:"updatedAt"`
}
