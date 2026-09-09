package dto

import "time"

// ProductProfileDTO 产品聚合档案
type ProductProfileDTO struct {
	Product           *ProductOverviewDTO      `json:"product"`
	Models            []ProductModelBriefDTO   `json:"models"`
	DeviceOverview    *DeviceOverviewDTO       `json:"deviceOverview"`
	ServiceCodeStats  *ServiceCodeStatsDTO     `json:"serviceCodeStats"`
	TicketStats       *TicketStatsDTO          `json:"ticketStats"`
	RepairHistory     *RepairHistorySummaryDTO `json:"repairHistory"`
	KnowledgeCoverage *KnowledgeCoverageDTO    `json:"knowledgeCoverage"`
	QualitySignals    *QualitySignalSummaryDTO `json:"qualitySignals"`
	UsageSummary      *UsageSummaryDTO         `json:"usageSummary"`
}

// ProductOverviewDTO 产品概览
type ProductOverviewDTO struct {
	ID            int64  `json:"id"`
	TenantID      int64  `json:"tenantId"`
	Code          string `json:"code"`
	Name          string `json:"name"`
	Category      string `json:"category"`
	ProductLineID int64  `json:"productLineId"`
	Status        int    `json:"status"`
	ModelCount    int    `json:"modelCount"`
	ActiveDevices int64  `json:"activeDevices"`
	TotalTickets  int64  `json:"totalTickets"`
}

// ProductModelBriefDTO 型号简要
type ProductModelBriefDTO struct {
	ID          int64  `json:"id"`
	TenantID    int64  `json:"tenantId"`
	ModelCode   string `json:"modelCode"`
	Name        string `json:"name"`
	Status      int    `json:"status"`
	DeviceCount int64  `json:"deviceCount"`
}

// DeviceOverviewDTO 设备概览
type DeviceOverviewDTO struct {
	TotalDevices   int64 `json:"totalDevices"`
	ActiveDevices  int64 `json:"activeDevices"`
	InMaintenance  int64 `json:"inMaintenance"`
	Decommissioned int64 `json:"decommissioned"`
	UnboundDevices int64 `json:"unboundDevices"`
}

// ServiceCodeStatsDTO 服务码状态统计
type ServiceCodeStatsDTO struct {
	Total   int64 `json:"total"`
	Active  int64 `json:"active"`
	Bound   int64 `json:"bound"`
	Expired int64 `json:"expired"`
	Revoked int64 `json:"revoked"`
}

// TicketStatsDTO 工单统计
type TicketStatsDTO struct {
	Total           int64   `json:"total"`
	Pending         int64   `json:"pending"`
	InProgress      int64   `json:"inProgress"`
	Closed          int64   `json:"closed"`
	Escalated       int64   `json:"escalated"`
	Reopened        int64   `json:"reopened"`
	AvgResolveHours float64 `json:"avgResolveHours"`
}

// RepairHistorySummaryDTO 维修历史摘要
type RepairHistorySummaryDTO struct {
	TotalRepairs   int64            `json:"totalRepairs"`
	RemoteResolved int64            `json:"remoteResolved"`
	OnsiteRepairs  int64            `json:"onsiteRepairs"`
	TopFaultCodes  []FaultCodeEntry `json:"topFaultCodes"`
	LastRepairAt   *time.Time       `json:"lastRepairAt"`
}

// FaultCodeEntry 故障码统计项
type FaultCodeEntry struct {
	FaultCode string `json:"faultCode"`
	Count     int64  `json:"count"`
}

// KnowledgeCoverageDTO 知识覆盖
type KnowledgeCoverageDTO struct {
	TotalEntries    int64   `json:"totalEntries"`
	ProductSpecific int64   `json:"productSpecific"`
	ModelSpecific   int64   `json:"modelSpecific"`
	CoverageScore   float64 `json:"coverageScore"`
}

// QualitySignalSummaryDTO 质量信号摘要
type QualitySignalSummaryDTO struct {
	OpenSignals   int64 `json:"openSignals"`
	WarningCount  int64 `json:"warningCount"`
	CriticalCount int64 `json:"criticalCount"`
	ResolvedCount int64 `json:"resolvedCount"`
}

// UsageSummaryDTO 用量摘要
type UsageSummaryDTO struct {
	TotalRequests int64 `json:"totalRequests"`
	TodayRequests int64 `json:"todayRequests"`
	TotalCost     int64 `json:"totalCost"`
}

// EnterpriseProductProfileDTO is the enterprise product dossier shape consumed by
// the product center frontend. It keeps the existing serviceProfile contract while
// exposing the richer product center aggregate for newer tabs.
type EnterpriseProductProfileDTO struct {
	Product                  EnterpriseProductListItemDTO       `json:"product"`
	ServiceProfile           EnterpriseProductServiceProfileDTO `json:"serviceProfile"`
	TotalTicketCount         int64                              `json:"totalTicketCount"`
	TotalKnowledgeEntryCount int64                              `json:"totalKnowledgeEntryCount"`
	ProductCenter            *ProductProfileDTO                 `json:"productCenter,omitempty"`
}

type EnterpriseProductCountDTO struct {
	Total int64 `json:"total"`
}

type EnterpriseProductListItemDTO struct {
	ID                 int64   `json:"id"`
	Code               string  `json:"code"`
	Name               string  `json:"name"`
	ProductLine        string  `json:"product_line"`
	Status             string  `json:"status"`
	Category           string  `json:"category"`
	OwnerMemberID      int64   `json:"owner_member_id"`
	OwnerUserID        int64   `json:"owner_user_id"`
	OwnerName          string  `json:"owner_name"`
	DeviceCount        int64   `json:"device_count"`
	TicketCount        int64   `json:"ticket_count"`
	ActiveSessionCount int64   `json:"active_session_count"`
	AIResolveRate      float64 `json:"ai_resolve_rate"`
	DefaultLocale      string  `json:"default_locale"`
	Description        string  `json:"description"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
}

type EnterpriseProductServiceProfileDTO struct {
	ProductID                int64          `json:"product_id"`
	DefaultAIAgentID         int64          `json:"default_ai_agent_id"`
	SupportLocales           []string       `json:"support_locales"`
	SupportRegions           []string       `json:"support_regions"`
	WarrantyPolicy           map[string]any `json:"warranty_policy"`
	SafetyLevel              string         `json:"safety_level"`
	MeetingEnabled           bool           `json:"meeting_enabled"`
	KnowledgeBaseID          *int64         `json:"knowledge_base_id"`
	KnowledgeBaseName        string         `json:"knowledge_base_name,omitempty"`
	KnowledgeBaseDescription string         `json:"knowledge_base_description,omitempty"`
	RagflowDatasetID         string         `json:"ragflow_dataset_id,omitempty"`
	Status                   string         `json:"status"`
}

type ProductDeviceDTO struct {
	ID           int64  `json:"id"`
	DeviceNo     string `json:"device_no"`
	SerialNo     string `json:"serial_no"`
	ModelName    string `json:"model_name"`
	RegionCode   string `json:"region_code"`
	Status       string `json:"status"`
	LastActiveAt string `json:"last_active_at"`
}

type ProductServiceCodeDTO struct {
	ID          int64   `json:"id"`
	ServiceCode string  `json:"service_code"`
	EntryURL    string  `json:"entry_url"`
	QRURL       string  `json:"qr_url"`
	QRImageURL  string  `json:"qr_image_url"`
	BatchNo     string  `json:"batch_no"`
	Mode        string  `json:"mode"`
	Status      string  `json:"status"`
	BoundDevice *string `json:"bound_device"`
	CreatedAt   string  `json:"created_at"`
}

type ProductTicketBriefDTO struct {
	ID        int64  `json:"id"`
	TicketNo  string `json:"ticket_no"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Priority  string `json:"priority"`
	CreatedAt string `json:"created_at"`
}

type ProductConversationBriefDTO struct {
	ID            int64  `json:"id"`
	Channel       string `json:"channel"`
	CustomerName  string `json:"customer_name"`
	Status        string `json:"status"`
	ServiceMode   string `json:"service_mode"`
	Summary       string `json:"summary"`
	LastMessageAt string `json:"last_message_at"`
	HandoffReason string `json:"handoff_reason"`
}

type ProductRepairRecordDTO struct {
	ID                int64  `json:"id"`
	TicketID          int64  `json:"ticket_id"`
	TicketNo          string `json:"ticket_no"`
	DeviceID          int64  `json:"device_id"`
	DeviceNo          string `json:"device_no"`
	ServiceType       string `json:"service_type"`
	FaultType         string `json:"fault_type"`
	Summary           string `json:"summary"`
	RootCause         string `json:"root_cause"`
	Solution          string `json:"solution"`
	Resolution        string `json:"resolution"`
	Technician        string `json:"technician"`
	VisibleToCustomer bool   `json:"visible_to_customer"`
	OccurredAt        string `json:"occurred_at"`
	CompletedAt       string `json:"completed_at"`
}

type ProductKnowledgeCoverageDetailDTO struct {
	TotalEntries      int64              `json:"total_entries"`
	ProductSpecific   int64              `json:"product_specific"`
	ModelSpecific     int64              `json:"model_specific"`
	KnowledgeBaseID   *int64             `json:"knowledge_base_id,omitempty"`
	RagflowDatasetID  string             `json:"ragflow_dataset_id,omitempty"`
	CoverageScore     float64            `json:"coverage_score"`
	LinkedEntries     int64              `json:"linked_entries"`
	PendingCandidates int64              `json:"pending_candidates"`
	Gaps              []string           `json:"gaps"`
	Links             []KnowledgeLinkDTO `json:"links"`
}

type KnowledgeLinkDTO struct {
	ID               int64  `json:"id"`
	KnowledgeBaseID  int64  `json:"knowledge_base_id,omitempty"`
	RagflowDatasetID string `json:"ragflow_dataset_id,omitempty"`
	Title            string `json:"title"`
	Type             string `json:"type"`
	URL              string `json:"url"`
	Language         string `json:"language"`
	Version          string `json:"version"`
	Visibility       string `json:"visibility"`
	UpdatedAt        string `json:"updated_at"`
}

type ProductFaultStatsDTO struct {
	Range       string `json:"range"`
	GeneratedAt string `json:"generated_at"`
	// DataStatus 区分“无故障”和“尚未统计”（§6.3）：ready / empty / building / failed。
	DataStatus      string                    `json:"data_status"`
	TotalFaults     int64                     `json:"total_faults"`
	AffectedDevices int64                     `json:"affected_devices"`
	Stats           []ProductFaultStatItemDTO `json:"stats"`
}

// ProductFaultStatsRebuildJobDTO 故障统计重建任务。
type ProductFaultStatsRebuildJobDTO struct {
	ID             int64  `json:"id"`
	ProductID      int64  `json:"product_id"`
	Status         string `json:"status"`
	RangeDays      int    `json:"range_days"`
	ProcessedCount int64  `json:"processed_count"`
	ErrorSummary   string `json:"error_summary"`
	StartedAt      string `json:"started_at"`
	FinishedAt     string `json:"finished_at"`
	CreatedAt      string `json:"created_at"`
}

type ProductFaultStatItemDTO struct {
	Part       string  `json:"part"`
	FaultType  string  `json:"fault_type"`
	ModelName  string  `json:"model_name"`
	Count      int64   `json:"count"`
	Percentage float64 `json:"percentage"`
	Trend      string  `json:"trend"`
	Severity   string  `json:"severity"`
}

type ProductQualitySignalDTO struct {
	ID             int64   `json:"id"`
	SignalType     string  `json:"signal_type"`
	Severity       string  `json:"severity"`
	Title          string  `json:"title"`
	Description    string  `json:"description"`
	Source         string  `json:"source"`
	SourceType     string  `json:"source_type"`
	SourceID       string  `json:"source_id"`
	MetricValue    float64 `json:"metric_value"`
	SampleCount    int64   `json:"sample_count"`
	OwnerUserID    int64   `json:"owner_user_id"`
	DetectedAt     string  `json:"detected_at"`
	AcknowledgedAt string  `json:"acknowledged_at"`
	ResolvedAt     string  `json:"resolved_at"`
	Status         string  `json:"status"`
}

type ProductUsageMetricDTO struct {
	Metric       string `json:"metric"`
	CurrentMonth int64  `json:"current_month"`
	LastMonth    int64  `json:"last_month"`
	Unit         string `json:"unit"`
}
