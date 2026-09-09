package models

import (
	"time"

	"remotehelpdesk/internal/pkg/enums"
)

type Tenant struct {
	ID                     int64        `gorm:"primaryKey;autoIncrement"`
	Name                   string       `gorm:"type:varchar(200);not null;index"`
	ServiceScene           string       `gorm:"column:service_scene;type:varchar(32);not null;default:'equipment_after_sales';index"`
	Industry               string       `gorm:"type:varchar(100);not null;default:'';index"`
	CountryRegion          string       `gorm:"type:varchar(64);not null;default:'';index"`
	DefaultLocale          string       `gorm:"type:varchar(16);not null;default:'en'"`
	SupportedLocalesJSON   string       `gorm:"column:supported_locales_json;type:text;not null;default:'[]'"`
	Timezone               string       `gorm:"type:varchar(64);not null;default:'UTC'"`
	SupportedTimezonesJSON string       `gorm:"column:supported_timezones_json;type:text;not null;default:'[]'"`
	DataRegion             string       `gorm:"type:varchar(64);not null;default:''"`
	Status                 enums.Status `gorm:"type:int;not null;default:0;index"`
	AIEnabled              *bool        `gorm:"column:ai_enabled;not null;default:true;index"`
	TrialEndsAt            *time.Time   `gorm:"type:timestamp"`
	FrozenReason           string       `gorm:"type:text"`
	DecommissionedAt       *time.Time   `gorm:"type:timestamp;index"`
	DecommissionReason     string       `gorm:"type:varchar(500);not null;default:''"`
	PurgeAfterDays         int          `gorm:"type:int;not null;default:30"`
	TicketAutoCloseEnabled bool         `gorm:"not null;default:false"`
	TicketAutoCloseDays    int          `gorm:"type:int;not null;default:7"`
	TicketIntakePolicyJSON string       `gorm:"type:text;not null;default:'{}'"`
	PurgedAt               *time.Time   `gorm:"type:timestamp;index"`
	DataExportCompletedAt  *time.Time   `gorm:"type:timestamp;index"`
	AuditFields
}

const (
	TenantServiceSceneEquipmentAfterSales = "equipment_after_sales"
	TenantServiceSceneKnowledgeSupport    = "knowledge_support"
)

func NormalizeTenantServiceScene(value string) string {
	switch value {
	case TenantServiceSceneEquipmentAfterSales, TenantServiceSceneKnowledgeSupport:
		return value
	default:
		return ""
	}
}

func (t *Tenant) EffectiveServiceScene() string {
	if t == nil {
		return TenantServiceSceneEquipmentAfterSales
	}
	if normalized := NormalizeTenantServiceScene(t.ServiceScene); normalized != "" {
		return normalized
	}
	return TenantServiceSceneEquipmentAfterSales
}

func (t *Tenant) IsKnowledgeSupportScene() bool {
	return t != nil && t.EffectiveServiceScene() == TenantServiceSceneKnowledgeSupport
}

func (t *Tenant) IsAIEnabled() bool {
	if t == nil || t.AIEnabled == nil {
		return true
	}
	return *t.AIEnabled
}

type ProductLine struct {
	ID            int64        `gorm:"primaryKey;autoIncrement"`
	TenantID      int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_product_lines_tenant_code"`
	Code          string       `gorm:"type:varchar(64);not null;default:'';uniqueIndex:uk_product_lines_tenant_code"`
	Name          string       `gorm:"type:varchar(128);not null;default:'';index"`
	Category      string       `gorm:"type:varchar(64);not null;default:'';index"`
	OwnerMemberID int64        `gorm:"type:bigint;not null;default:0;index"`
	Status        enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

type Product struct {
	ID            int64        `gorm:"primaryKey;autoIncrement"`
	TenantID      int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_products_tenant_code"`
	ProductLineID int64        `gorm:"type:bigint;not null;default:0;index"`
	Code          string       `gorm:"type:varchar(64);not null;default:'';uniqueIndex:uk_products_tenant_code"`
	Name          string       `gorm:"type:varchar(128);not null;default:'';index"`
	Category      string       `gorm:"type:varchar(64);not null;default:'';index"`
	OwnerMemberID int64        `gorm:"type:bigint;not null;default:0;index"`
	DefaultLocale string       `gorm:"type:varchar(16);not null;default:'en'"`
	Description   string       `gorm:"type:text"`
	Status        enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

type ProductServiceProfile struct {
	ID                     int64        `gorm:"primaryKey;autoIncrement"`
	TenantID               int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_product_service_profiles_product"`
	ProductID              int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_product_service_profiles_product"`
	SupportLocalesJSON     string       `gorm:"column:support_locales_json;type:text;not null;default:'[]'"`
	SupportRegionsJSON     string       `gorm:"column:support_regions_json;type:text;not null;default:'[]'"`
	WarrantyPolicyJSON     string       `gorm:"column:warranty_policy_json;type:text;not null;default:'{}'"`
	SafetyLevel            string       `gorm:"type:varchar(32);not null;default:'';index"`
	DefaultFlowTemplateID  int64        `gorm:"type:bigint;not null;default:0"`
	DefaultKnowledgeBaseID int64        `gorm:"type:bigint;not null;default:0"`
	DefaultAIAgentID       int64        `gorm:"type:bigint;not null;default:0;index"`
	MeetingEnabled         bool         `gorm:"not null;default:true;index"`
	ServicePolicyJSON      string       `gorm:"column:service_policy_json;type:text;not null;default:'{}'"`
	Status                 enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

// ProductKnowledgeBinding maps a product or model and optional locale/region to a knowledge base.
type ProductKnowledgeBinding struct {
	ID              int64        `gorm:"primaryKey;autoIncrement"`
	TenantID        int64        `gorm:"type:bigint;not null;index"`
	ProductID       int64        `gorm:"type:bigint;not null;index"`
	ProductModelID  int64        `gorm:"type:bigint;not null;default:0;index"`
	KnowledgeBaseID int64        `gorm:"type:bigint;not null;index"`
	ScopeType       string       `gorm:"type:varchar(20);not null;default:'product';index"`
	Locale          string       `gorm:"type:varchar(16);not null;default:'';index"`
	RegionCode      string       `gorm:"type:varchar(64);not null;default:'';index"`
	SortNo          int          `gorm:"type:int;not null;default:0;index"`
	Status          enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

type ProductModel struct {
	ID              int64        `gorm:"primaryKey;autoIncrement"`
	TenantID        int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_product_models_product_code"`
	ProductID       int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_product_models_product_code"`
	ModelCode       string       `gorm:"type:varchar(64);not null;default:'';uniqueIndex:uk_product_models_product_code"`
	Name            string       `gorm:"type:varchar(128);not null;default:'';index"`
	VersionPolicy   string       `gorm:"type:varchar(64);not null;default:''"`
	RegionScopeJSON string       `gorm:"column:region_scope_json;type:text;not null;default:'[]'"`
	Status          enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

// ProductModule 产品粗粒度模块或 BOM，可作为故障部位统计维度。
type ProductModule struct {
	ID                int64        `gorm:"primaryKey;autoIncrement"`
	TenantID          int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_product_modules_tenant_product_code"`
	ProductID         int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_product_modules_tenant_product_code"`
	ModuleCode        string       `gorm:"type:varchar(64);not null;default:'';uniqueIndex:uk_product_modules_tenant_product_code"`
	Name              string       `gorm:"type:varchar(128);not null;default:'';index"`
	DefaultSupplierID int64        `gorm:"type:bigint;not null;default:0"`
	IsSafetyCritical  bool         `gorm:"not null;default:false"`
	MetadataJSON      string       `gorm:"column:metadata_json;type:text;not null;default:'{}'"`
	Status            enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

// ProductModuleModelLink 模块适用型号关联。
// 联合唯一索引 uk_product_module_model_links_module_model 防止同一模块重复挂载同一型号；
// 关联删除采用物理删除，避免软删除记录与唯一索引冲突。
type ProductModuleModelLink struct {
	ID              int64        `gorm:"primaryKey;autoIncrement"`
	TenantID        int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_product_module_model_links_module_model"`
	ProductModuleID int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_product_module_model_links_module_model"`
	ProductModelID  int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_product_module_model_links_module_model"`
	Status          enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

// TicketSupplierCollaboration records a scoped supplier escalation for one ticket.
type TicketSupplierCollaboration struct {
	ID                int64        `gorm:"primaryKey;autoIncrement"`
	TenantID          int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_ticket_supplier_active,where:record_status <> 2 AND status <> 'resolved' AND status <> 'timeout'"`
	TicketID          int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_ticket_supplier_active,where:record_status <> 2 AND status <> 'resolved' AND status <> 'timeout'"`
	ProductID         int64        `gorm:"type:bigint;not null;index"`
	ProductModuleID   int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_ticket_supplier_active,where:record_status <> 2 AND status <> 'resolved' AND status <> 'timeout'"`
	PartnerCompanyID  int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_ticket_supplier_active,where:record_status <> 2 AND status <> 'resolved' AND status <> 'timeout'"`
	PartnerAccountID  int64        `gorm:"type:bigint;not null;default:0;index"`
	Status            string       `gorm:"type:varchar(32);not null;default:'invited';index"`
	Reason            string       `gorm:"type:text"`
	VisibilityJSON    string       `gorm:"column:visibility_json;type:text;not null;default:'[]'"`
	Resolution        string       `gorm:"type:text"`
	InvitedAt         time.Time    `gorm:"type:timestamp;not null;index"`
	AcceptedAt        *time.Time   `gorm:"type:timestamp;index"`
	ResolvedAt        *time.Time   `gorm:"type:timestamp;index"`
	AuthorizationEnds *time.Time   `gorm:"type:timestamp;index"`
	RecordStatus      enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

func (TicketSupplierCollaboration) TableName() string { return "ticket_supplier_collaborations" }

// TicketSupplierCollaborationParticipant records every supplier account that
// participates in a collaboration. The collaboration's PartnerAccountID stays
// as the accountable owner; this relation preserves co-worker access/history.
type TicketSupplierCollaborationParticipant struct {
	ID               int64        `gorm:"primaryKey;autoIncrement"`
	TenantID         int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_ticket_supplier_participant"`
	CollaborationID  int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_ticket_supplier_participant"`
	PartnerCompanyID int64        `gorm:"type:bigint;not null;index"`
	PartnerAccountID int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_ticket_supplier_participant"`
	Role             string       `gorm:"type:varchar(32);not null;default:'member';index"`
	JoinedAt         time.Time    `gorm:"type:timestamp;not null;index"`
	LeftAt           *time.Time   `gorm:"type:timestamp;index"`
	Status           enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

func (TicketSupplierCollaborationParticipant) TableName() string {
	return "ticket_supplier_collaboration_participants"
}

// ProductFaultStatsDaily 产品故障日统计聚合表（投影，不作为业务事实来源）。
type ProductFaultStatsDaily struct {
	ID                  int64     `gorm:"primaryKey;autoIncrement"`
	TenantID            int64     `gorm:"type:bigint;not null;index;uniqueIndex:uk_product_fault_stats_daily_dimensions"`
	ProductID           int64     `gorm:"type:bigint;not null;index;uniqueIndex:uk_product_fault_stats_daily_dimensions"`
	ProductModelID      int64     `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_product_fault_stats_daily_dimensions"`
	FaultCode           string    `gorm:"type:varchar(128);not null;default:'';index;uniqueIndex:uk_product_fault_stats_daily_dimensions"`
	FaultPart           string    `gorm:"type:varchar(128);not null;default:'';index;uniqueIndex:uk_product_fault_stats_daily_dimensions"`
	ModuleID            int64     `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_product_fault_stats_daily_dimensions"`
	BucketDate          time.Time `gorm:"type:date;not null;index;uniqueIndex:uk_product_fault_stats_daily_dimensions"`
	TicketCount         int64     `gorm:"type:bigint;not null;default:0"`
	RepeatCount         int64     `gorm:"type:bigint;not null;default:0"`
	LowScoreCount       int64     `gorm:"type:bigint;not null;default:0"`
	MeetingCount        int64     `gorm:"type:bigint;not null;default:0"`
	AffectedDeviceCount int64     `gorm:"type:bigint;not null;default:0"`
	CreatedAt           time.Time `gorm:"type:timestamp;not null;index"`
	UpdatedAt           time.Time `gorm:"type:timestamp;not null;index"`
}

// TableName 固定表名为 product_fault_stats_daily，
// 与 UpsertDaily 的原生冲突表达式中引用的表名保持一致。
func (ProductFaultStatsDaily) TableName() string {
	return "product_fault_stats_daily"
}

// ProductKnowledgeLink 产品和知识、手册、FAQ 的挂载关系。
// idx_product_knowledge_links_lookup 覆盖有效记录重复校验与产品/型号维度检索；
// 有效记录的唯一性由 Service 层校验（软删除记录不参与唯一约束）。
type ProductKnowledgeLink struct {
	ID               int64        `gorm:"primaryKey;autoIncrement"`
	TenantID         int64        `gorm:"type:bigint;not null;index:idx_product_knowledge_links_lookup"`
	ProductID        int64        `gorm:"type:bigint;not null;index;index:idx_product_knowledge_links_lookup"`
	ProductModelID   int64        `gorm:"type:bigint;not null;default:0;index;index:idx_product_knowledge_links_lookup"`
	KnowledgeBaseID  int64        `gorm:"type:bigint;not null;default:0;index"`
	KnowledgeEntryID int64        `gorm:"type:bigint;not null;default:0;index;index:idx_product_knowledge_links_lookup"`
	LinkType         string       `gorm:"type:varchar(32);not null;default:'';index;index:idx_product_knowledge_links_lookup"`
	Language         string       `gorm:"type:varchar(16);not null;default:'';index;index:idx_product_knowledge_links_lookup"`
	Version          string       `gorm:"type:varchar(32);not null;default:'';index:idx_product_knowledge_links_lookup"`
	Visibility       string       `gorm:"type:varchar(32);not null;default:'internal';index"`
	PublishStatus    string       `gorm:"type:varchar(32);not null;default:'draft';index"`
	SortNo           int          `gorm:"type:int;not null;default:0;index"`
	Status           enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

// ProductManualFile 产品手册业务记录。原始文件供用户下载；产品知识库存在时，
// 通过 KnowledgeDocument 建立独立的 RAG 副本，二者生命周期仍由该记录关联。
type ProductManualFile struct {
	ID                  int64                              `gorm:"primaryKey;autoIncrement"`
	TenantID            int64                              `gorm:"type:bigint;not null;index:idx_product_manual_files_lookup"`
	ProductID           int64                              `gorm:"type:bigint;not null;index;index:idx_product_manual_files_lookup"`
	AssetID             int64                              `gorm:"type:bigint;not null;index;uniqueIndex:uk_product_manual_files_asset"`
	Title               string                             `gorm:"type:varchar(255);not null;default:''"`
	Language            string                             `gorm:"type:varchar(16);not null;default:'';index"`
	Version             string                             `gorm:"type:varchar(32);not null;default:''"`
	Visibility          string                             `gorm:"type:varchar(32);not null;default:'public';index"`
	KnowledgeBaseID     int64                              `gorm:"type:bigint;not null;default:0;index"`
	KnowledgeDocumentID int64                              `gorm:"type:bigint;not null;default:0;index"`
	KnowledgeLinkID     int64                              `gorm:"type:bigint;not null;default:0;index"`
	SyncStatus          enums.KnowledgeDocumentIndexStatus `gorm:"type:varchar(20);not null;default:'pending';index"`
	SyncError           string                             `gorm:"type:text"`
	SyncedAt            *time.Time                         `gorm:"type:timestamp;index"`
	Status              enums.Status                       `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

// SystemIntroDoc 平台系统介绍文档。平台域记录（无 TenantID），由平台管理员上传，
// 供客户门户访客在「通用咨询」入口预览。Status: draft | published。
type SystemIntroDoc struct {
	ID          int64      `gorm:"primaryKey;autoIncrement"`
	AssetID     int64      `gorm:"type:bigint;not null;index;uniqueIndex:uk_system_intro_docs_asset"`
	Title       string     `gorm:"type:varchar(255);not null;default:''"`
	FileName    string     `gorm:"type:varchar(255);not null;default:''"`
	FileSize    int64      `gorm:"type:bigint;not null;default:0"`
	MimeType    string     `gorm:"type:varchar(128);not null;default:''"`
	SortNo      int        `gorm:"type:int;not null;default:0;index"`
	Status      string     `gorm:"type:varchar(20);not null;default:'draft';index"` // draft | published
	PublishedAt *time.Time `gorm:"type:timestamp;index"`
	AuditFields
}

type Device struct {
	ID                  int64        `gorm:"primaryKey;autoIncrement"`
	TenantID            int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_devices_tenant_device_no"`
	DeviceNo            string       `gorm:"type:varchar(64);not null;default:'';uniqueIndex:uk_devices_tenant_device_no"`
	ProductID           int64        `gorm:"type:bigint;not null;index"`
	ProductModelID      int64        `gorm:"type:bigint;not null;default:0;index"`
	SerialNo            string       `gorm:"type:varchar(100);not null;default:'';index"`
	CustomerOrgID       int64        `gorm:"type:bigint;not null;default:0;index"`
	ExternalDeviceID    string       `gorm:"type:varchar(128);not null;default:'';index"`
	InstallLocationJSON string       `gorm:"column:install_location_json;type:text;not null;default:'{}'"`
	RegionCode          string       `gorm:"type:varchar(64);not null;default:'';index"`
	InstalledAt         *time.Time   `gorm:"type:timestamp"`
	Source              string       `gorm:"type:varchar(32);not null;default:'';index"`
	Status              enums.Status `gorm:"type:int;not null;default:0;index"`
	// DeviceStatus 设备生命周期状态：new/operational/under_maintenance/decommissioned/archived
	DeviceStatus     string     `gorm:"type:varchar(32);not null;default:'new';index"`
	StatusChangedAt  *time.Time `gorm:"type:timestamp"`
	StatusReason     string     `gorm:"type:varchar(500);not null;default:''"`
	LastServiceAt    *time.Time `gorm:"type:timestamp"`
	DecommissionedAt *time.Time `gorm:"type:timestamp"`
	ArchivedAt       *time.Time `gorm:"type:timestamp"`
	TransferCount    int        `gorm:"type:int;not null;default:0"`
	MetadataJSON     string     `gorm:"column:metadata_json;type:text;not null;default:'{}'"`
	AuditFields
}

type ServiceCodeBatch struct {
	ID              int64                 `gorm:"primaryKey;autoIncrement"`
	TenantID        int64                 `gorm:"type:bigint;not null;index;uniqueIndex:uk_service_code_batches_tenant_no"`
	BatchNo         string                `gorm:"type:varchar(64);not null;default:'';uniqueIndex:uk_service_code_batches_tenant_no"`
	Mode            enums.ServiceCodeMode `gorm:"type:varchar(32);not null;default:'traceable';index"`
	ProductID       int64                 `gorm:"type:bigint;not null;default:0;index"`
	ProductModelID  int64                 `gorm:"type:bigint;not null;default:0;index"`
	Quantity        int64                 `gorm:"type:bigint;not null;default:0"`
	LabelTemplateID int64                 `gorm:"type:bigint;not null;default:0"`
	Status          enums.Status          `gorm:"type:int;not null;default:0;index"`
	GeneratedCount  int64                 `gorm:"type:bigint;not null;default:0"`
	ExportedCount   int64                 `gorm:"type:bigint;not null;default:0"`
	AuditFields
}

type ServiceCode struct {
	ID               int64                   `gorm:"primaryKey;autoIncrement"`
	TenantID         int64                   `gorm:"type:bigint;not null;index"`
	BatchID          int64                   `gorm:"type:bigint;not null;default:0;index"`
	ServiceCode      string                  `gorm:"type:varchar(128);not null;uniqueIndex"`
	Mode             enums.ServiceCodeMode   `gorm:"type:varchar(32);not null;default:'traceable';index"`
	DeviceID         int64                   `gorm:"type:bigint;not null;default:0;index"`
	ProductID        int64                   `gorm:"type:bigint;not null;default:0;index"`
	ProductModelID   int64                   `gorm:"type:bigint;not null;default:0;index"`
	Status           enums.ServiceCodeStatus `gorm:"type:varchar(32);not null;default:'active';index"`
	ActivatedAt      *time.Time              `gorm:"type:timestamp"`
	BoundAt          *time.Time              `gorm:"type:timestamp"`
	EffectiveStartAt *time.Time              `gorm:"type:timestamp"`
	EffectiveEndAt   *time.Time              `gorm:"type:timestamp"`
	RevokedAt        *time.Time              `gorm:"type:timestamp"`
	RevokeReason     string                  `gorm:"type:varchar(500);not null;default:''"`
	MetadataJSON     string                  `gorm:"column:metadata_json;type:text;not null;default:'{}'"`
	AuditFields
}

type CustomerDeviceBinding struct {
	ID                  int64        `gorm:"primaryKey;autoIncrement"`
	TenantID            int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_customer_device_binding"`
	CustomerOrgID       int64        `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_customer_device_binding"`
	CustomerUserID      int64        `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_customer_device_binding"`
	DeviceID            int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_customer_device_binding"`
	BindingRole         string       `gorm:"type:varchar(32);not null;default:'';index"`
	PermissionFlagsJSON string       `gorm:"column:permission_flags_json;type:text;not null;default:'{}'"`
	Source              string       `gorm:"type:varchar(32);not null;default:'';index"`
	Status              enums.Status `gorm:"type:int;not null;default:0;index"`
	ConfirmedAt         *time.Time   `gorm:"type:timestamp"`
	PreviousOwnerID     int64        `gorm:"type:bigint;not null;default:0"`
	PreviousOwnerType   string       `gorm:"type:varchar(32);not null;default:''"`
	TransferCount       int          `gorm:"type:int;not null;default:0"`
	AuditFields
}

type CustomerEntrySession struct {
	ID                 int64      `gorm:"primaryKey;autoIncrement"`
	TenantID           int64      `gorm:"type:bigint;not null;index"`
	ProductID          int64      `gorm:"type:bigint;not null;default:0;index"`
	ProductModelID     int64      `gorm:"type:bigint;not null;default:0;index"`
	EntryType          string     `gorm:"type:varchar(32);not null;default:'';index"`
	ServiceCodeID      int64      `gorm:"type:bigint;not null;default:0;index"`
	DeviceID           int64      `gorm:"type:bigint;not null;default:0;index"`
	CustomerUserID     int64      `gorm:"type:bigint;not null;default:0;index"`
	VisitorID          string     `gorm:"type:varchar(128);not null;default:'';index"`
	Locale             string     `gorm:"type:varchar(16);not null;default:'en'"`
	State              string     `gorm:"type:varchar(32);not null;default:'';index"`
	EntryContextJSON   string     `gorm:"column:entry_context_json;type:text;not null;default:'{}'"`
	VisitorTokenHash   string     `gorm:"type:varchar(128);not null;default:'';index;uniqueIndex:ux_customer_entry_session_visitor_token_hash"`
	VisitorDataJSON    string     `gorm:"column:visitor_data;type:text;not null;default:'{}'"`
	BindingCompletedAt *time.Time `gorm:"type:timestamp;index"`
	ExpiresAt          *time.Time `gorm:"type:timestamp;index"`
	CreatedAt          time.Time  `gorm:"type:timestamp;not null;index"`
	UpdatedAt          time.Time  `gorm:"type:timestamp;not null;index"`
}

// DeviceSoftwareVersion 设备固件/软件版本追踪
type DeviceSoftwareVersion struct {
	ID              int64     `gorm:"primaryKey;autoIncrement"`
	TenantID        int64     `gorm:"type:bigint;not null;index"`
	DeviceID        int64     `gorm:"type:bigint;not null;index;uniqueIndex:ux_device_sw_version_unique"`
	ComponentType   string    `gorm:"type:varchar(32);not null;index;uniqueIndex:ux_device_sw_version_unique"` // firmware / bios / driver / os / application
	ComponentName   string    `gorm:"type:varchar(128);not null;default:'';uniqueIndex:ux_device_sw_version_unique"`
	Version         string    `gorm:"type:varchar(64);not null;uniqueIndex:ux_device_sw_version_unique"`
	ReleaseNotes    string    `gorm:"type:text"`
	InstalledAt     time.Time `gorm:"type:timestamp;not null;index"`
	Source          string    `gorm:"type:varchar(32);not null;default:'manual';index"` // manual / ota / factory / repair
	InstalledByType string    `gorm:"type:varchar(32);not null;default:''"`             // member / customer / system
	InstalledByID   int64     `gorm:"type:bigint;not null;default:0"`
	Checksum        string    `gorm:"type:varchar(128);not null;default:''"`
	MetadataJSON    string    `gorm:"column:metadata_json;type:text;not null;default:'{}'"`
	CreatedAt       time.Time `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
}

// BaseModel 包含通用审计字段
type BaseModel struct {
	CreatedAt time.Time `gorm:"type:timestamp;not null;index"`
	UpdatedAt time.Time `gorm:"type:timestamp;not null;index"`
}

// MeetingRoomJitsi Jitsi 会议室
type MeetingRoomJitsi struct {
	ID          string     `gorm:"primaryKey;type:varchar(36)"`
	TenantID    int64      `gorm:"type:bigint;not null;index"`
	TicketID    string     `gorm:"type:varchar(36);index;not null"`
	RoomName    string     `gorm:"type:varchar(160);uniqueIndex;not null"`
	Status      string     `gorm:"type:varchar(20);default:'active';index"`
	CreatedBy   string     `gorm:"type:varchar(36);not null;default:''"`
	ScheduledAt *time.Time `gorm:"type:timestamp"`
	StartedAt   *time.Time `gorm:"type:timestamp"`
	EndedAt     *time.Time `gorm:"type:timestamp"`
	BaseModel
}

// MeetingParticipant 参会者记录
type MeetingParticipant struct {
	ID              string     `gorm:"primaryKey;type:varchar(36)"`
	MeetingID       string     `gorm:"type:varchar(36);index;not null;uniqueIndex:uk_meeting_participant_identity"`
	UserID          string     `gorm:"type:varchar(36);not null;default:'';uniqueIndex:uk_meeting_participant_identity"`
	UserType        string     `gorm:"type:varchar(20);not null;default:'';index;uniqueIndex:uk_meeting_participant_identity"`
	ParticipantName string     `gorm:"type:varchar(100);not null;default:''"`
	Role            string     `gorm:"type:varchar(20);not null;default:'participant'"`
	JoinedAt        *time.Time `gorm:"type:timestamp"`
	LeftAt          *time.Time `gorm:"type:timestamp"`
	Duration        int64      `gorm:"type:bigint;not null;default:0"`
	BaseModel
}

// TableName 设置 MeetingRoomJitsi 表名
func (MeetingRoomJitsi) TableName() string {
	return "meeting_rooms_jitsi"
}

// TableName 设置 MeetingParticipant 表名
func (MeetingParticipant) TableName() string {
	return "meeting_participants"
}

// DiagnosisSession AI 诊断会话
type DiagnosisSession struct {
	ID                     string     `gorm:"primaryKey;type:varchar(36)"`
	TenantID               int64      `gorm:"type:bigint;not null;index"`
	CustomerID             string     `gorm:"type:varchar(36);index;not null;default:''"`
	DeviceID               string     `gorm:"type:varchar(36);index;not null;default:''"`
	ProductID              string     `gorm:"type:varchar(36);index;not null;default:''"`
	ProductModelID         int64      `gorm:"type:bigint;not null;default:0;index"`
	ServiceCodeID          string     `gorm:"type:varchar(36);index;not null;default:''"`
	ConversationID         string     `gorm:"type:varchar(36);index;not null;default:''"`
	DeviceRefID            int64      `gorm:"type:bigint;not null;default:0;index"`
	ProductRefID           int64      `gorm:"type:bigint;not null;default:0;index"`
	ServiceCodeRefID       int64      `gorm:"type:bigint;not null;default:0;index"`
	ConversationRefID      int64      `gorm:"type:bigint;not null;default:0;index"`
	Status                 string     `gorm:"type:varchar(32);not null;default:'active';index"` // active/escalated/resolved/abandoned
	Language               string     `gorm:"type:varchar(16);not null;default:''"`
	RegionCode             string     `gorm:"type:varchar(64);not null;default:'';index"`
	Audience               string     `gorm:"type:varchar(32);not null;default:'agent';index"`
	KnowledgeScopeSnapshot string     `gorm:"type:text"`
	Symptoms               string     `gorm:"type:text"` // JSON array
	FaultCodes             string     `gorm:"type:text"` // JSON array
	RetrieveCount          int        `gorm:"type:int;not null;default:0"`
	RetrieveTopScore       float64    `gorm:"type:decimal(5,4);not null;default:0"`
	RetrieveHitCount       int        `gorm:"type:int;not null;default:0"`
	TotalRounds            int        `gorm:"type:int;not null;default:0"`
	MaxRounds              int        `gorm:"type:int;not null;default:10"`
	Resolution             string     `gorm:"type:text"`
	ConfidenceScore        float64    `gorm:"type:decimal(5,4);not null;default:0"`
	ExperimentGroup        string     `gorm:"type:varchar(64);not null;default:''"`
	ExperimentVersion      string     `gorm:"type:varchar(32);not null;default:''"`
	CreatedAt              time.Time  `gorm:"type:timestamp;not null;index"`
	EndedAt                *time.Time `gorm:"type:timestamp"`
	BaseModel
}

// TableName 设置 DiagnosisSession 表名
func (DiagnosisSession) TableName() string {
	return "diagnosis_sessions"
}

// DiagnosisStep 诊断会话步骤
type DiagnosisStep struct {
	ID                     string    `gorm:"primaryKey;type:varchar(36)"`
	SessionID              string    `gorm:"type:varchar(36);index;not null"`
	TenantID               int64     `gorm:"type:bigint;not null;index"`
	SequenceNo             int       `gorm:"type:int;not null;default:0"`
	StepType               string    `gorm:"type:varchar(32);not null;default:'';index"` // user_input / ai_response / check_result / guide_step / handoff
	InputContent           string    `gorm:"type:text"`
	OutputContent          string    `gorm:"type:text"`
	FaultNodeIDs           string    `gorm:"type:text"` // JSON array of matched fault tree node IDs
	RagCitations           string    `gorm:"type:text"` // JSON array of RAG citation IDs
	ConfidenceScore        float64   `gorm:"type:decimal(5,4);not null;default:0"`
	UserConfirmed          string    `gorm:"type:varchar(20);not null;default:''"` // confirmed / rejected / skipped
	SecondaryCheckRequired bool      `gorm:"not null;default:false"`
	SecondaryCheckResult   string    `gorm:"type:varchar(16);not null;default:''"`
	ReasoningTrace         string    `gorm:"type:text"` // JSON - reasoning trace
	Metadata               string    `gorm:"type:text"` // JSON - extra metadata
	CreatedAt              time.Time `gorm:"type:timestamp;not null;index"`
}

// TableName 设置 DiagnosisStep 表名
func (DiagnosisStep) TableName() string {
	return "diagnosis_steps"
}

// FaultTreeNode 故障树节点
type FaultTreeNode struct {
	ID                string    `gorm:"primaryKey;type:varchar(36)"`
	TenantID          int64     `gorm:"type:bigint;not null;index"`
	ProductID         string    `gorm:"type:varchar(36);index;not null"`
	ParentID          string    `gorm:"type:varchar(36);index;not null;default:''"` // 父节点
	Title             string    `gorm:"type:varchar(200);not null;default:''"`
	Description       string    `gorm:"type:text"`
	NodeType          string    `gorm:"type:varchar(32);not null;default:'';index"`           // symptom/check/action/diagnosis
	FaultPattern      string    `gorm:"type:varchar(32);not null;default:'persistent';index"` // persistent/intermittent/conditional
	TriggerConditions string    `gorm:"type:text"`                                            // JSON
	IsComposite       bool      `gorm:"not null;default:false"`
	RiskLevel         string    `gorm:"type:varchar(16);not null;default:'low';index"` // low/medium/high/critical
	OrderIndex        int       `gorm:"type:int;not null;default:0"`
	Status            string    `gorm:"type:varchar(32);not null;default:'draft';index"` // draft/published/archived
	CreatedAt         time.Time `gorm:"type:timestamp;not null;index"`
	UpdatedAt         time.Time `gorm:"type:timestamp;not null;index"`
}

// TableName 设置 FaultTreeNode 表名
func (FaultTreeNode) TableName() string {
	return "fault_tree_nodes"
}

// ProductModelVersion 产品型号版本记录
type ProductModelVersion struct {
	ID                  int64        `gorm:"primaryKey;autoIncrement"`
	TenantID            int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_product_model_versions"`
	ProductModelID      int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_product_model_versions"`
	VersionLabel        string       `gorm:"type:varchar(64);not null;default:'';uniqueIndex:uk_product_model_versions"`
	ReleaseNotesURL     string       `gorm:"type:varchar(512);not null;default:''"`
	BreakingChanges     string       `gorm:"type:text"`
	CompatibilityMatrix string       `gorm:"type:text;not null;default:'{}'"`
	Status              enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

// DeviceWarrantyRecord 设备质保记录
type DeviceWarrantyRecord struct {
	ID                 int64     `gorm:"primaryKey;autoIncrement"`
	TenantID           int64     `gorm:"type:bigint;not null;index"`
	DeviceID           int64     `gorm:"type:bigint;not null;index"`
	ProductID          int64     `gorm:"type:bigint;not null;default:0;index"`
	ProductModelID     int64     `gorm:"type:bigint;not null;default:0;index"`
	WarrantyType       string    `gorm:"type:varchar(32);not null;default:'standard';index"`
	StartAt            time.Time `gorm:"type:date;not null;index"`
	EndAt              time.Time `gorm:"type:date;not null;index"`
	PolicySummary      string    `gorm:"type:text"`
	WarrantyPolicyJSON string    `gorm:"column:warranty_policy_json;type:text;not null;default:'{}'"`
	Source             string    `gorm:"type:varchar(32);not null;default:'manual';index"`
	SourceID           string    `gorm:"type:varchar(64);not null;default:'';index"`
	MetadataJSON       string    `gorm:"column:metadata_json;type:text;not null;default:'{}'"`
	AuditFields
}

// QrLabelTemplate QR 标签模板
type QrLabelTemplate struct {
	ID             int64        `gorm:"primaryKey;autoIncrement"`
	TenantID       int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_qr_label_templates_tenant_code"`
	ProductID      int64        `gorm:"type:bigint;not null;default:0;index"`
	TemplateCode   string       `gorm:"type:varchar(64);not null;default:'';uniqueIndex:uk_qr_label_templates_tenant_code"`
	Name           string       `gorm:"type:varchar(128);not null;default:'';index"`
	DesignSpecJSON string       `gorm:"column:design_spec_json;type:text;not null;default:'{}'"`
	PageSize       string       `gorm:"type:varchar(32);not null;default:'A4'"`
	LabelWidth     float64      `gorm:"type:decimal(8,2);not null;default:0"`
	LabelHeight    float64      `gorm:"type:decimal(8,2);not null;default:0"`
	QRPrefix       string       `gorm:"type:varchar(32);not null;default:''"`
	IsDefault      bool         `gorm:"not null;default:false;index"`
	Status         enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

// QrLabelExportJob QR 导出任务
type QrLabelExportJob struct {
	ID             int64      `gorm:"primaryKey;autoIncrement"`
	TenantID       int64      `gorm:"type:bigint;not null;index"`
	BatchID        int64      `gorm:"type:bigint;not null;default:0;index"`
	TemplateID     int64      `gorm:"type:bigint;not null;default:0;index"`
	JobType        string     `gorm:"type:varchar(32);not null;default:'batch';index"`
	Status         string     `gorm:"type:varchar(32);not null;default:'pending';index"`
	TotalCount     int64      `gorm:"type:bigint;not null;default:0"`
	CompletedCount int64      `gorm:"type:bigint;not null;default:0"`
	FailedCount    int64      `gorm:"type:bigint;not null;default:0"`
	ResultAssetID  string     `gorm:"type:varchar(64);not null;default:'';index"`
	ErrorMessage   string     `gorm:"type:text"`
	StartedAt      *time.Time `gorm:"type:timestamp"`
	CompletedAt    *time.Time `gorm:"type:timestamp"`
	ExpiresAt      *time.Time `gorm:"type:timestamp;index"`
	AuditFields
}

// QrScanLog 扫码日志记录
type QrScanLog struct {
	ID             int64     `gorm:"primaryKey;autoIncrement"`
	TenantID       int64     `gorm:"type:bigint;not null;index"`
	ServiceCodeID  int64     `gorm:"type:bigint;not null;default:0;index"`
	ServiceCodeVal string    `gorm:"type:varchar(128);not null;default:'';index"`
	DeviceID       int64     `gorm:"type:bigint;not null;default:0;index"`
	ProductID      int64     `gorm:"type:bigint;not null;default:0;index"`
	ProductModelID int64     `gorm:"type:bigint;not null;default:0;index"`
	IPAddress      string    `gorm:"type:varchar(64);not null;default:'';index"`
	UserAgent      string    `gorm:"type:varchar(512);not null;default:''"`
	VisitorID      string    `gorm:"type:varchar(128);not null;default:'';index"`
	Result         string    `gorm:"type:varchar(32);not null;default:'success';index"`
	ErrorMessage   string    `gorm:"type:varchar(500);not null;default:''"`
	LatencyMs      int64     `gorm:"type:bigint;not null;default:0"`
	CreatedAt      time.Time `gorm:"type:timestamp;not null;index"`
}

// DeviceRegistrationTask 通用码设备登记任务
type DeviceRegistrationTask struct {
	ID                   int64      `gorm:"primaryKey;autoIncrement"`
	TenantID             int64      `gorm:"type:bigint;not null;index"`
	ServiceCodeID        int64      `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_device_registration_sc"`
	ServiceCodeVal       string     `gorm:"type:varchar(128);not null;default:'';index"`
	ProductID            int64      `gorm:"type:bigint;not null;default:0;index"`
	ProductModelID       int64      `gorm:"type:bigint;not null;default:0;index"`
	CustomerUserID       int64      `gorm:"type:bigint;not null;default:0;index"`
	CustomerOrgID        int64      `gorm:"type:bigint;not null;default:0;index"`
	DeviceNo             string     `gorm:"type:varchar(64);not null;default:'';index"`
	SerialNo             string     `gorm:"type:varchar(100);not null;default:'';index"`
	RegistrationDataJSON string     `gorm:"column:registration_data_json;type:text;not null;default:'{}'"`
	Status               string     `gorm:"type:varchar(32);not null;default:'pending';index"`
	CompletedAt          *time.Time `gorm:"type:timestamp"`
	ExpiresAt            *time.Time `gorm:"type:timestamp;index"`
	AuditFields
}

// ProductQualitySignal 产品质量信号
type ProductQualitySignal struct {
	ID               int64        `gorm:"primaryKey;autoIncrement"`
	TenantID         int64        `gorm:"type:bigint;not null;index"`
	ProductID        int64        `gorm:"type:bigint;not null;index"`
	ProductModelID   int64        `gorm:"type:bigint;not null;default:0;index"`
	SignalType       string       `gorm:"type:varchar(64);not null;default:'';index"`
	Severity         string       `gorm:"type:varchar(32);not null;default:'info';index"`
	Title            string       `gorm:"type:varchar(255);not null;default:'';index"`
	Description      string       `gorm:"type:text"`
	TriggerCondition string       `gorm:"type:text"`
	SignalDataJSON   string       `gorm:"column:signal_data_json;type:text;not null;default:'{}'"`
	MetricValue      float64      `gorm:"type:decimal(18,4);not null;default:0"`
	SampleCount      int64        `gorm:"type:bigint;not null;default:0"`
	DetectedAt       time.Time    `gorm:"type:timestamp;not null;index"`
	AcknowledgedAt   *time.Time   `gorm:"type:timestamp"`
	AcknowledgedByID int64        `gorm:"type:bigint;not null;default:0;index"`
	Source           string       `gorm:"type:varchar(32);not null;default:'system';index"`
	SourceType       string       `gorm:"type:varchar(32);not null;default:'';index"`
	SourceID         string       `gorm:"type:varchar(64);not null;default:'';index"`
	OwnerUserID      int64        `gorm:"type:bigint;not null;default:0;index"`
	ResolvedAt       *time.Time   `gorm:"type:timestamp;index"`
	Resolution       string       `gorm:"type:text"`
	Status           enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

// TicketContextSnapshot 工单创建时的上下文快照
type TicketContextSnapshot struct {
	ID               int64     `gorm:"primaryKey;autoIncrement"`
	TicketID         int64     `gorm:"type:bigint;not null;index;uniqueIndex"`
	TenantID         int64     `gorm:"type:bigint;not null;index"`
	SnapshotDataJSON string    `gorm:"column:snapshot_data_json;type:text;not null;default:'{}'"`
	ProductJSON      string    `gorm:"column:product_json;type:text;not null;default:'{}'"`
	DeviceJSON       string    `gorm:"column:device_json;type:text;not null;default:'{}'"`
	CustomerJSON     string    `gorm:"column:customer_json;type:text;not null;default:'{}'"`
	ConversationJSON string    `gorm:"column:conversation_json;type:text;not null;default:'{}'"`
	DiagnosisJSON    string    `gorm:"column:diagnosis_json;type:text;not null;default:'{}'"`
	ServiceCodeJSON  string    `gorm:"column:service_code_json;type:text;not null;default:'{}'"`
	CreatedAt        time.Time `gorm:"type:timestamp;not null;index"`
}

// ConversationMessageTranslation stores a cached translation for one conversation message.
type ConversationMessageTranslation struct {
	ID               int64  `gorm:"primaryKey;autoIncrement"`
	TenantID         int64  `gorm:"type:bigint;not null;index;uniqueIndex:uk_conversation_message_translation"`
	ConversationID   int64  `gorm:"type:bigint;not null;index"`
	MessageID        int64  `gorm:"type:bigint;not null;index;uniqueIndex:uk_conversation_message_translation"`
	SourceLanguage   string `gorm:"type:varchar(16);not null;default:'auto'"`
	TargetLanguage   string `gorm:"type:varchar(16);not null;index;uniqueIndex:uk_conversation_message_translation"`
	SourceTextHash   string `gorm:"type:varchar(64);not null;uniqueIndex:uk_conversation_message_translation"`
	TranslatedText   string `gorm:"type:text;not null"`
	ModelName        string `gorm:"type:varchar(100);not null;default:''"`
	PromptTokens     int    `gorm:"type:int;not null;default:0"`
	CompletionTokens int    `gorm:"type:int;not null;default:0"`
	AuditFields
}

// KnowledgeCandidate 知识候选，从工单/会议/低分诊断提炼
type KnowledgeCandidate struct {
	ID                    int64        `gorm:"primaryKey;autoIncrement"`
	TenantID              int64        `gorm:"type:bigint;not null;index"`
	ProductID             int64        `gorm:"type:bigint;not null;default:0;index"`
	ProductModelID        int64        `gorm:"type:bigint;not null;default:0;index"`
	SourceType            string       `gorm:"type:varchar(32);not null;default:'';index"`
	SourceID              string       `gorm:"type:varchar(64);not null;default:'';index"`
	TicketID              int64        `gorm:"type:bigint;not null;default:0;index"`
	Title                 string       `gorm:"type:varchar(255);not null;default:'';index"`
	Suggestion            string       `gorm:"type:text"`
	RootCauseSummary      string       `gorm:"type:text"`
	SolutionSummary       string       `gorm:"type:text"`
	KnowledgeBaseID       int64        `gorm:"type:bigint;not null;default:0;index"`
	KnowledgeEntryID      int64        `gorm:"type:bigint;not null;default:0;index"`
	QualityScore          int          `gorm:"type:int;not null;default:0;index"`
	ValueScore            int          `gorm:"type:int;not null;default:0;index"`
	CandidateScore        int          `gorm:"type:int;not null;default:0;index"`
	ScoreBreakdownJSON    string       `gorm:"column:score_breakdown_json;type:text;not null;default:'{}'"`
	QualityFlagsJSON      string       `gorm:"column:quality_flags_json;type:text;not null;default:'[]'"`
	ScoreVersion          string       `gorm:"type:varchar(32);not null;default:'';index"`
	ScoredAt              *time.Time   `gorm:"type:timestamp;index"`
	SimilarityHash        string       `gorm:"type:varchar(64);not null;default:'';index"`
	DuplicateGroupID      string       `gorm:"type:varchar(64);not null;default:'';index"`
	MergedToCandidateID   int64        `gorm:"type:bigint;not null;default:0;index"`
	DeduplicationOverride bool         `gorm:"not null;default:false"`
	RecurrenceCount       int          `gorm:"type:int;not null;default:1"`
	AffectedDeviceCount   int          `gorm:"type:int;not null;default:0"`
	RequiresReassessment  bool         `gorm:"not null;default:false;index"`
	ReviewStatus          string       `gorm:"type:varchar(32);not null;default:'pending';index"`
	ReviewRemark          string       `gorm:"type:text"`
	ReviewedAt            *time.Time   `gorm:"type:timestamp"`
	ReviewerID            int64        `gorm:"type:bigint;not null;default:0"`
	Status                enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

// FaultStatsEventInbox 故障统计事件消费幂等表。
// 以 event_id 做消费幂等，避免事件重试导致 ProductFaultStatsDaily 重复累计。
type FaultStatsEventInbox struct {
	ID             int64     `gorm:"primaryKey;autoIncrement"`
	EventID        string    `gorm:"type:varchar(64);not null;uniqueIndex:uk_fault_stats_event_inbox_event"`
	TenantID       int64     `gorm:"type:bigint;not null;index"`
	EventType      string    `gorm:"type:varchar(64);not null;default:'';index"`
	ProductID      int64     `gorm:"type:bigint;not null;default:0;index"`
	ProductModelID int64     `gorm:"type:bigint;not null;default:0"`
	DeviceID       int64     `gorm:"type:bigint;not null;default:0"`
	TicketID       int64     `gorm:"type:bigint;not null;default:0;index"`
	PayloadJSON    string    `gorm:"column:payload_json;type:text;not null;default:'{}'"`
	ProcessedAt    time.Time `gorm:"type:timestamp;not null;index"`
	CreatedAt      time.Time `gorm:"type:timestamp;not null;index"`
}

// ProductFaultStatsRebuildJob 故障统计历史重建任务。
type ProductFaultStatsRebuildJob struct {
	ID             int64      `gorm:"primaryKey;autoIncrement"`
	TenantID       int64      `gorm:"type:bigint;not null;index"`
	ProductID      int64      `gorm:"type:bigint;not null;index"`
	Status         string     `gorm:"type:varchar(32);not null;default:'pending';index"` // pending/running/succeeded/failed
	RangeDays      int        `gorm:"type:int;not null;default:180"`
	ProcessedCount int64      `gorm:"type:bigint;not null;default:0"`
	ErrorSummary   string     `gorm:"type:text"`
	RequestedByID  int64      `gorm:"type:bigint;not null;default:0"`
	StartedAt      *time.Time `gorm:"type:timestamp"`
	FinishedAt     *time.Time `gorm:"type:timestamp"`
	AuditFields
}

// KnowledgeIndexSyncTask 知识索引同步任务。
// 知识发布、修订、废弃或产品手册重索引时生成，由 KnowledgeIndexSyncService
// 调用对应 KnowledgeIndexProvider（内部索引或 RAGFlow）异步执行。
type KnowledgeIndexSyncTask struct {
	ID                int64      `gorm:"primaryKey;autoIncrement"`
	TenantID          int64      `gorm:"type:bigint;not null;index"`
	IdempotencyKey    string     `gorm:"type:varchar(128);not null;uniqueIndex:uk_knowledge_index_sync_task_key"`
	SubjectType       string     `gorm:"type:varchar(32);not null;default:'knowledge_link';index"` // knowledge_link/knowledge_document/knowledge_faq
	SubjectID         int64      `gorm:"type:bigint;not null;default:0;index"`
	KnowledgeBaseID   int64      `gorm:"type:bigint;not null;default:0;index"`
	ProductID         int64      `gorm:"type:bigint;not null;default:0;index"`
	RevisionID        int64      `gorm:"type:bigint;not null;default:0;index"`
	IndexGenerationID int64      `gorm:"type:bigint;not null;default:0;index"`
	ProviderType      string     `gorm:"type:varchar(32);not null;default:'internal';index"` // internal/ragflow
	Action            string     `gorm:"type:varchar(32);not null;default:'upsert';index"`   // upsert/delete
	InputVersion      string     `gorm:"type:varchar(64);not null;default:''"`
	CollectionName    string     `gorm:"type:varchar(128);not null;default:''"`
	ContentHash       string     `gorm:"type:varchar(64);not null;default:'';index"`
	Status            string     `gorm:"type:varchar(32);not null;default:'pending';index"` // pending/running/waiting_retry/succeeded/failed/cancelled
	ExternalJobID     string     `gorm:"type:varchar(128);not null;default:''"`
	RetryCount        int        `gorm:"type:int;not null;default:0"`
	MaxRetries        int        `gorm:"type:int;not null;default:3"`
	NextAttemptAt     *time.Time `gorm:"type:timestamp;index"`
	LockedAt          *time.Time `gorm:"type:timestamp;index"`
	LockOwner         string     `gorm:"type:varchar(64);not null;default:'';index"`
	ErrorCode         string     `gorm:"type:varchar(64);not null;default:'';index"`
	ErrorSummary      string     `gorm:"type:text"`
	LastAttemptAt     *time.Time `gorm:"type:timestamp"`
	FinishedAt        *time.Time `gorm:"type:timestamp"`
	AuditFields
}

// TicketQualityClue 工单质量线索
type TicketQualityClue struct {
	ID             int64      `gorm:"primaryKey;autoIncrement"`
	TenantID       int64      `gorm:"type:bigint;not null;index"`
	TicketID       int64      `gorm:"type:bigint;not null;index"`
	ProductID      int64      `gorm:"type:bigint;not null;default:0;index"`
	ProductModelID int64      `gorm:"type:bigint;not null;default:0;index"`
	ClueType       string     `gorm:"type:varchar(64);not null;default:'';index"`
	Severity       string     `gorm:"type:varchar(32);not null;default:'info';index"`
	Description    string     `gorm:"type:text"`
	ClueDataJSON   string     `gorm:"column:clue_data_json;type:text;not null;default:'{}'"`
	DetectedAt     time.Time  `gorm:"type:timestamp;not null;index"`
	Status         string     `gorm:"type:varchar(32);not null;default:'open';index"`
	AssignedToID   int64      `gorm:"type:bigint;not null;default:0;index"`
	ResolvedAt     *time.Time `gorm:"type:timestamp"`
	Resolution     string     `gorm:"type:text"`
	AuditFields
}

// DataBreachRecord 数据泄露事件记录
type DataBreachRecord struct {
	ID                             string     `gorm:"primaryKey;type:varchar(36)"`
	TenantID                       string     `gorm:"type:varchar(36);not null;index"`
	BreachType                     string     `gorm:"type:varchar(64);not null;default:'';index"`
	Description                    string     `gorm:"type:text"`
	AffectedData                   string     `gorm:"type:text"`
	AffectedDataCategories         string     `gorm:"type:varchar(256);not null;default:''"`
	AffectedUsersCount             int        `gorm:"type:int;not null;default:0"`
	Severity                       string     `gorm:"type:varchar(20);not null;default:'low';index"`
	Status                         string     `gorm:"type:varchar(20);not null;default:'open';index"`
	RemediationSteps               string     `gorm:"type:text"`
	ReportedBy                     string     `gorm:"type:varchar(36);not null;default:''"`
	DetectedAt                     time.Time  `gorm:"type:timestamp;not null"`
	NotificationDeadline           time.Time  `gorm:"type:timestamp;not null;index"`
	SupervisoryAuthorityNotifiedAt *time.Time `gorm:"type:timestamp"`
	DataSubjectsNotifiedAt         *time.Time `gorm:"type:timestamp"`
	ResolvedAt                     *time.Time `gorm:"type:timestamp"`
	BaseModel
}

// TableName 设置 DataBreachRecord 表名
func (DataBreachRecord) TableName() string {
	return "data_breach_records"
}
