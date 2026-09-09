package models

import (
	"errors"
	"time"

	"remotehelpdesk/internal/pkg/enums"

	"gorm.io/gorm"
)

// Models 注册所有需要迁移和代码生成的模型。
var Models = []any{
	&Migration{},
	&User{},
	&UserIdentity{},
	&Company{},
	&Tenant{},
	&TenantBranding{},
	&TenantPlan{},
	&TenantPlanQuota{},
	&TenantSubscription{},
	&TenantQuotaOverride{},
	&AuthVerificationCode{},
	&Sub2APITenantAccount{},
	&Sub2APIRechargeRecord{},
	&MeteringUsageEvent{},
	&MeteringUsageDaily{},
	&AuthSession{},
	&PlatformStaffProfile{},
	&PlatformTenantGrant{},
	&TenantMember{},
	&Department{},
	&EngineerProfile{},
	&CustomerOrg{},
	&CustomerUser{},
	&CustomerRegistrationGrant{},
	&CustomerUserIdentity{},
	&CustomerUserRole{},
	&PartnerCompany{},
	&PartnerContract{},
	&PartnerAccount{},
	&PartnerAuthorizationScope{},
	&AuthRole{},
	&AuthRoleBinding{},
	&AuthPermissionCatalog{},
	&AuthRolePermission{},
	&AuthSubjectPermissionOverride{},
	&DataScopePolicy{},
	&FieldMaskingPolicy{},
	&TemporaryAccessGrant{},
	&AuthAuditLog{},
	&UserMFASetting{},
	&UserMFABackupCode{},
	&UserPasswordHistory{},
	&ServiceAccount{},
	&RateLimitProfile{},
	&SSOConfig{},
	&Customer{},
	&CustomerIdentity{},
	&CustomerContact{},
	&ProductLine{},
	&Product{},
	&ProductServiceProfile{},
	&ProductKnowledgeBinding{},
	&TenantIntegrationConfig{},
	&ProductAIUsageCredential{},
	&ProductResourceProvisioningJob{},
	&MeetingRoom{},
	&ProductAIUsageDaily{},
	&ProductModel{},
	&ProductModule{},
	&ProductModuleModelLink{},
	&TicketSupplierCollaboration{},
	&TicketSupplierCollaborationParticipant{},
	&ProductFaultStatsDaily{},
	&ProductManualFile{},
	&SystemIntroDoc{},
	&ProductKnowledgeLink{},
	&Device{},
	&ServiceCodeBatch{},
	&ServiceCode{},
	&CustomerDeviceBinding{},
	&CustomerEntrySession{},
	&CustomerPrivacyConsent{},
	&Role{},
	&Permission{},
	&UserRole{},
	&RolePermission{},
	&UserPermission{},
	&LoginSession{},
	&LoginCredentialLog{},
	&Asset{},
	&Tag{},
	&Conversation{},
	&ConversationParticipant{},
	&ConversationReadState{},
	&Message{},
	&AIReplyJob{},
	&ConversationMessageTranslation{},
	&WxWorkKFSyncState{},
	&WxWorkKFConversation{},
	&WxWorkKFMessageRef{},
	&ChannelMessageOutbox{},
	&ConversationAssignment{},
	&ConversationTag{},
	&QuickReply{},
	&ConversationEventLog{},
	&Ticket{},
	&TicketDispatchAttempt{},
	&TicketTag{},
	&TicketProgress{},
	&TicketRepairRecord{},
	&TicketFeedback{},
	&TicketServiceOutcomeFact{},
	&TicketView{},
	&TicketNoSequence{},
	&DeviceServiceRecord{},
	&Notification{},
	&AIAgent{},
	&AIAgentRelease{},
	&Channel{},
	&AgentProfile{},
	&AgentTeam{},
	&AgentTeamMember{},
	&AgentTeamSchedule{},
	&AgentTeamScheduleTemplate{},
	&AgentTeamHoliday{},
	&AgentScheduleException{},
	&AgentWorkStatus{},
	&AIConfig{},
	&KnowledgeBase{},
	&KnowledgeDirectory{},
	&KnowledgeDocument{},
	&KnowledgeFAQ{},
	&KnowledgeChunk{},
	&KnowledgeRevision{},
	&KnowledgeIndexGeneration{},
	&KnowledgeRetrieveLog{},
	&KnowledgeRetrieveHit{},
	&KnowledgeFeedback{},
	&SkillDefinition{},
	&SkillRunLog{},
	&AIWorkflow{},
	&AIWorkflowVersion{},
	&AIWorkflowRun{},
	&AIWorkflowNodeRun{},
	&AIWorkflowEffect{},
	&ConversationInterrupt{},
	&SystemConfig{},
	&DeviceSoftwareVersion{},
	&MeetingRoomJitsi{},
	&MeetingParticipant{},
	&MeetingTranscriptSegment{},
	&MeetingARAnnotation{},
	&DiagnosisSession{},
	&DiagnosisStep{},
	&FaultTreeNode{},
	&IndustrySolutionPack{},
	&IndustrySolutionPackResource{},
	&IndustrySolutionPackApplication{},
	&IndustrySolutionPackApplicationItem{},
	&DiagnosisEvalCase{},
	&DiagnosisEvalRun{},
	&ARWorkInstruction{},
	&ARWorkStep{},
	&ProductAIUsageEvent{},
	&ProductModelVersion{},
	&DeviceWarrantyRecord{},
	&QrLabelTemplate{},
	&QrLabelExportJob{},
	&QrScanLog{},
	&DeviceRegistrationTask{},
	&ProductQualitySignal{},
	&TicketContextSnapshot{},
	&KnowledgeCandidate{},
	&FaultStatsEventInbox{},
	&ProductFaultStatsRebuildJob{},
	&KnowledgeIndexSyncTask{},
	&TicketQualityClue{},
	&DomainEvent{},
	&OutboxRecord{},
	&AuditLog{},
	&AccessConnector{},
	&AccessQueryLog{},
	&AccessCallLog{},
	&ConnectorSyncCursor{},
	&ConnectorWorkerLease{},
	&ConnectorEventInbox{},
	&DeviceTelemetryEvent{},
	&DeviceTelemetrySnapshot{},
	&DeviceAlarmEvent{},
	&ConnectorDeadLetter{},
	&UsageEvent{},
	&QuotaLimit{},
	&BudgetAlert{},
	&SyncRun{},
	&NotificationTemplate{},
	&NotificationPreference{},
	&NotificationRecipientSetting{},
	&MobilePushToken{},
	&TenantMailSetting{},
	&DeliveryLog{},
	&WebhookEndpoint{},
	&WebhookDeliveryLog{},
	&WebhookEventInbox{},
	&DSARRequest{},
	&DSARExecutionLog{},
	&DataBreachRecord{},
	&DataRegionPolicy{},
}

type Migration struct {
	ID         int64     `gorm:"primaryKey;autoIncrement"`
	Version    int64     `gorm:"type:bigint;not null;uniqueIndex"`
	Remark     string    `gorm:"type:text"`
	Success    bool      `gorm:"not null;default:false"`
	ErrorInfo  string    `gorm:"type:text"`
	RetryCount int       `gorm:"type:int;not null;default:0"`
	CreatedAt  time.Time `gorm:"type:timestamp"`
	UpdatedAt  time.Time `gorm:"type:timestamp"`
}

// SystemConfig 运营侧系统配置项；具体有哪些 config_key 由业务代码约定，表内一行一项。
type SystemConfig struct {
	ID          int64        `gorm:"primaryKey;autoIncrement"`
	ConfigKey   string       `gorm:"column:config_key;type:varchar(128);not null;uniqueIndex"`
	ConfigValue string       `gorm:"column:config_value;type:text;not null"`
	GroupCode   string       `gorm:"column:group_code;type:varchar(64);not null;default:'';index"`
	Title       string       `gorm:"type:varchar(200);not null;default:''"`
	Description string       `gorm:"type:text"`
	Status      enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

// TicketNoSequence 工单号日序列表。
//
// 每天一条记录，NextSeq 表示当日下一次可分配的序号。
type TicketNoSequence struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`
	DateKey   string    `gorm:"column:date_key;type:varchar(8);not null;uniqueIndex"`
	NextSeq   int64     `gorm:"column:next_seq;type:bigint;not null;default:1"`
	CreatedAt time.Time `gorm:"type:timestamp;not null;index"`
	UpdatedAt time.Time `gorm:"type:timestamp;not null;index"`
}

// TicketView 工单工作台个人保存视图。
type TicketView struct {
	ID          int64  `gorm:"primaryKey;autoIncrement"`
	TenantID    int64  `gorm:"type:bigint;not null;default:0;index"`
	UserID      int64  `gorm:"column:user_id;type:bigint;not null;index"`
	Name        string `gorm:"column:name;type:varchar(100);not null;default:'';index"`
	FiltersJSON string `gorm:"column:filters_json;type:text;not null"`
	SortNo      int    `gorm:"column:sort_no;type:int;not null;default:0;index"`
	AuditFields
}

// Notification 站内通知。
type Notification struct {
	ID                    int64      `gorm:"primaryKey;autoIncrement"`
	IdempotencyKey        *string    `gorm:"type:varchar(180);uniqueIndex:uk_notification_tenant_idempotency,priority:2"`
	TenantID              int64      `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_notification_tenant_idempotency,priority:1"`
	EventKey              string     `gorm:"type:varchar(180);not null;default:'';index"`
	RecipientUserID       int64      `gorm:"type:bigint;not null;default:0;index"`
	Title                 string     `gorm:"type:varchar(255);not null;default:''"`
	Content               string     `gorm:"type:text"`
	NotificationType      string     `gorm:"type:varchar(50);not null;default:'';index"`
	BizType               string     `gorm:"type:varchar(50);not null;default:'';index"`
	BizID                 int64      `gorm:"type:bigint;not null;default:0;index"`
	ActionURL             string     `gorm:"type:varchar(255);not null;default:''"`
	TemplateID            int64      `gorm:"type:bigint;not null;default:0;index"`
	DeliveryStatus        string     `gorm:"type:varchar(20);not null;default:'pending';index"` // pending / sent / failed / read
	ExternalChannelStatus string     `gorm:"type:varchar(20);not null;default:'';index"`        // wxwork_sent / email_sent / sms_sent
	Level                 string     `gorm:"type:varchar(20);not null;default:'info';index"`    // urgent / warning / info
	Category              string     `gorm:"type:varchar(30);not null;default:'system';index"`  // ticket / sla / approval / quota / knowledge / video / system
	RecipientName         string     `gorm:"type:varchar(120);not null;default:''"`             // 接收人展示名,如 "服务运营部 / 李主管"
	Channels              string     `gorm:"type:varchar(60);not null;default:'in_app'"`        // 逗号分隔: in_app,email
	ReadAt                *time.Time `gorm:"type:timestamp;index"`
	Status                int        `gorm:"type:int;not null;default:0;index"`
	CreatedAt             time.Time  `gorm:"type:timestamp;not null;index"`
}

// NotificationTemplate 通知模板定义。
type NotificationTemplate struct {
	ID              int64     `gorm:"primaryKey;autoIncrement"`
	TenantID        int64     `gorm:"type:bigint;not null;default:0;index"`
	Code            string    `gorm:"type:varchar(64);not null;uniqueIndex;default:''"`
	Name            string    `gorm:"type:varchar(128);not null;default:'';index"`
	Channel         string    `gorm:"type:varchar(32);not null;default:'';index"` // in_app / wxwork / email / sms
	TitleTemplate   string    `gorm:"type:varchar(255);not null;default:''"`
	ContentTemplate string    `gorm:"type:text"`
	VariablesJSON   string    `gorm:"column:variables_json;type:text;not null;default:'[]'"` // JSON array of variable names
	Status          int       `gorm:"type:int;not null;default:0;index"`
	CreatedAt       time.Time `gorm:"type:timestamp;not null;index"`
	UpdatedAt       time.Time `gorm:"type:timestamp;not null;index"`
}

// TableName 设置 NotificationTemplate 表名
func (NotificationTemplate) TableName() string {
	return "notification_templates"
}

// NotificationPreference 用户通知偏好设置。
type NotificationPreference struct {
	ID              int64     `gorm:"primaryKey;autoIncrement"`
	TenantID        int64     `gorm:"type:bigint;not null;default:0;index"`
	UserID          int64     `gorm:"type:bigint;not null;index;uniqueIndex:uk_user_notification_pref"`
	NotifyType      string    `gorm:"type:varchar(50);not null;default:'';index;uniqueIndex:uk_user_notification_pref"` // ticket.created / meeting.invite / sla.warning
	Channel         string    `gorm:"type:varchar(32);not null;default:'in_app';index"`                                 // in_app / wxwork / email / sms
	Enabled         bool      `gorm:"not null;default:true;index"`
	QuietHoursStart string    `gorm:"type:varchar(5);not null;default:''"` // "22:00"
	QuietHoursEnd   string    `gorm:"type:varchar(5);not null;default:''"` // "08:00"
	CreatedAt       time.Time `gorm:"type:timestamp;not null;index"`
	UpdatedAt       time.Time `gorm:"type:timestamp;not null;index"`
}

// NotificationRecipientSetting stores a user's tenant-scoped notification
// delivery address. An empty EmailOverride means the account profile email is
// used, so updates made from the enterprise people directory remain effective.
type NotificationRecipientSetting struct {
	ID            int64     `gorm:"primaryKey;autoIncrement"`
	TenantID      int64     `gorm:"type:bigint;not null;default:0;uniqueIndex:uk_notification_recipient"`
	UserID        int64     `gorm:"type:bigint;not null;default:0;uniqueIndex:uk_notification_recipient"`
	EmailOverride string    `gorm:"type:varchar(128);not null;default:''"`
	EmailEnabled  bool      `gorm:"not null"`
	CreatedAt     time.Time `gorm:"type:timestamp;not null;index"`
	UpdatedAt     time.Time `gorm:"type:timestamp;not null;index"`
}

func (NotificationRecipientSetting) TableName() string {
	return "notification_recipient_settings"
}

// TableName 设置 NotificationPreference 表名
func (NotificationPreference) TableName() string {
	return "notification_preferences"
}

// TenantMailSetting 租户邮箱发送配置。铃铛消息需要发邮件时使用这里的发信账号,
// 未配置的字段回退全局 config.yaml 的 Email 配置。
type TenantMailSetting struct {
	ID          int64     `gorm:"primaryKey;autoIncrement"`
	TenantID    int64     `gorm:"type:bigint;not null;default:0;uniqueIndex"`
	FromAddress string    `gorm:"type:varchar(128);not null;default:''"`           // 发信邮箱
	FromName    string    `gorm:"type:varchar(128);not null;default:''"`           // 发信名称
	SMTPHost    string    `gorm:"type:varchar(128);not null;default:''"`           // SMTP 主机
	SMTPPort    int       `gorm:"type:int;not null;default:465"`                   // SMTP 端口
	Username    string    `gorm:"type:varchar(128);not null;default:''"`           // SMTP 账号
	Password    string    `gorm:"type:varchar(512);not null;default:''"`           // SMTP 密码密文
	ReplyTo     string    `gorm:"type:varchar(128);not null;default:''"`           // 回复地址
	RetryPolicy string    `gorm:"type:varchar(30);not null;default:'retry_3_10m'"` // retry_3_10m / retry_1_5m / no_retry
	UseTLS      bool      `gorm:"not null;default:true"`
	Status      int       `gorm:"type:int;not null;default:0;index"`
	CreatedAt   time.Time `gorm:"type:timestamp;not null;index"`
	UpdatedAt   time.Time `gorm:"type:timestamp;not null;index"`
}

// TableName 设置 TenantMailSetting 表名
func (TenantMailSetting) TableName() string {
	return "tenant_mail_settings"
}

// DeliveryLog 投递日志。
type DeliveryLog struct {
	ID             int64      `gorm:"primaryKey;autoIncrement"`
	TenantID       int64      `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_delivery_tenant_idempotency,priority:1"`
	IdempotencyKey *string    `gorm:"type:varchar(180);uniqueIndex:uk_delivery_tenant_idempotency,priority:2"`
	NotificationID int64      `gorm:"type:bigint;not null;index"`
	Channel        string     `gorm:"type:varchar(32);not null;default:'';index"` // in_app / wxwork / email / sms
	RecipientID    string     `gorm:"type:varchar(128);not null;default:'';index"`
	Status         string     `gorm:"type:varchar(20);not null;default:'pending';index"` // pending / processing / waiting_retry / sent / delivered / failed / bounced
	ProviderMsgID  string     `gorm:"type:varchar(255);not null;default:''"`
	ErrorMsg       string     `gorm:"type:text"`
	RetryCount     int        `gorm:"type:int;not null;default:0"`
	MaxRetries     int        `gorm:"type:int;not null;default:0"`
	NextAttemptAt  *time.Time `gorm:"type:timestamp;index"`
	LastAttemptAt  *time.Time `gorm:"type:timestamp;index"`
	SentAt         *time.Time `gorm:"type:timestamp;index"`
	DeliveredAt    *time.Time `gorm:"type:timestamp;index"`
	CreatedAt      time.Time  `gorm:"type:timestamp;not null;index"`
	UpdatedAt      time.Time  `gorm:"type:timestamp;not null;index"`
}

// TableName 设置 DeliveryLog 表名
func (DeliveryLog) TableName() string {
	return "delivery_logs"
}

// WebhookEndpoint Webhook 端点配置。
type WebhookEndpoint struct {
	ID       int64        `gorm:"primaryKey;autoIncrement"`
	TenantID int64        `gorm:"type:bigint;not null;index"`
	Name     string       `gorm:"type:varchar(128);not null;default:'';index"`
	URL      string       `gorm:"type:varchar(512);not null"`
	Secret   string       `gorm:"type:varchar(256);not null;default:''"`
	Events   string       `gorm:"type:text;not null"` // JSON array of event types
	Active   bool         `gorm:"not null;default:true;index"`
	Status   enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

// TableName 设置 WebhookEndpoint 表名
func (WebhookEndpoint) TableName() string {
	return "webhook_endpoints"
}

// WebhookDeliveryLog Webhook 投递日志。
type WebhookDeliveryLog struct {
	ID           int64     `gorm:"primaryKey;autoIncrement"`
	TenantID     int64     `gorm:"type:bigint;not null;index"`
	WebhookID    int64     `gorm:"type:bigint;not null;index"`
	EventType    string    `gorm:"type:varchar(64);not null;index"`
	RequestURL   string    `gorm:"type:varchar(512);not null;default:''"`
	RequestBody  string    `gorm:"type:text"`
	ResponseBody string    `gorm:"type:text"`
	StatusCode   int       `gorm:"type:int;not null;default:0"`
	DurationMs   int64     `gorm:"type:bigint;not null;default:0"`
	Status       string    `gorm:"type:varchar(20);not null;default:'pending';index"` // pending/success/failed
	RetryCount   int       `gorm:"type:int;not null;default:0"`
	ErrorMessage string    `gorm:"type:text"`
	CreatedAt    time.Time `gorm:"type:timestamp;not null;index"`
}

// TableName 设置 WebhookDeliveryLog 表名
func (WebhookDeliveryLog) TableName() string {
	return "webhook_delivery_logs"
}

// AuditFields 定义涉及用户操作数据的统一审计字段。
// 该结构记录数据创建与更新的时间、操作者ID和操作者名称。
type AuditFields struct {
	CreatedAt      time.Time `gorm:"type:timestamp;not null;index"`         // CreatedAt 记录数据创建时间。
	CreateUserID   int64     `gorm:"type:bigint;not null;default:0;index"`  // CreateUserID 记录创建人用户ID；系统任务写0。
	CreateUserName string    `gorm:"type:varchar(100);not null;default:''"` // CreateUserName 记录创建人名称；系统任务写system。
	UpdatedAt      time.Time `gorm:"type:timestamp;not null;index"`         // UpdatedAt 记录数据最近更新时间。
	UpdateUserID   int64     `gorm:"type:bigint;not null;default:0;index"`  // UpdateUserID 记录最后更新人用户ID；系统任务写0。
	UpdateUserName string    `gorm:"type:varchar(100);not null;default:''"` // UpdateUserName 记录最后更新人名称；系统任务写system。
}

// User 后台用户账号。
type User struct {
	ID           int64          `gorm:"primaryKey;autoIncrement"`
	Username     string         `gorm:"type:varchar(100);not null;uniqueIndex"`
	Nickname     string         `gorm:"type:varchar(100);not null;default:'';index"`
	Avatar       string         `gorm:"type:varchar(255);not null;default:''"`
	Mobile       *string        `gorm:"type:varchar(32);uniqueIndex"`
	Email        *string        `gorm:"type:varchar(100);uniqueIndex"`
	Locale       string         `gorm:"type:varchar(16);not null;default:'zh-CN'"`
	Timezone     string         `gorm:"type:varchar(64);not null;default:'Asia/Shanghai'"`
	Password     string         `gorm:"type:varchar(255);not null;default:''"`
	PasswordSalt string         `gorm:"type:varchar(64);not null;default:''"`
	Status       enums.Status   `gorm:"type:int;not null;default:0;index"`
	LastLoginAt  *time.Time     `gorm:"type:timestamp"`
	LastLoginIP  string         `gorm:"type:varchar(64);not null;default:''"`
	Remark       string         `gorm:"type:text"`
	DeletedAt    gorm.DeletedAt `gorm:"type:timestamp;index"`
	AuditFields
}

// UserIdentity 第三方身份绑定信息。
type UserIdentity struct {
	ID              int64               `gorm:"primaryKey;autoIncrement"`
	UserID          int64               `gorm:"type:bigint;not null;index;uniqueIndex:uk_provider_user"`
	Provider        enums.ThirdProvider `gorm:"type:varchar(50);not null;default:'';index;uniqueIndex:uk_provider_user;uniqueIndex:uk_provider_union"`
	ProviderUserID  string              `gorm:"type:varchar(128);not null;default:'';uniqueIndex:uk_provider_user"`
	ProviderUnionID *string             `gorm:"type:varchar(128);uniqueIndex:uk_provider_union"`
	ProviderCorpID  string              `gorm:"type:varchar(128);not null;default:'';index"`
	ProviderName    string              `gorm:"type:varchar(100);not null;default:''"`
	RawProfile      string              `gorm:"type:text"`
	Status          enums.Status        `gorm:"type:int;not null;default:0;index"`
	LastAuthAt      *time.Time          `gorm:"type:timestamp"`
	AuditFields
}

// Company 客户公司（组织）表。
//
//	用于存储公司主体信息；Customer（人）可通过 CompanyID 关联到所属公司。
type Company struct {
	ID     int64        `gorm:"primaryKey;autoIncrement"`                               // ID 为公司主键。
	Name   string       `gorm:"type:varchar(200);not null;uniqueIndex:uk_company_name"` // Name 为公司名称（唯一）。
	Code   string       `gorm:"type:varchar(64);not null;index"`                        // Code 为公司编码/统一社会信用代码（可空语义用空串表示）。
	Status enums.Status `gorm:"type:int;not null;default:0"`                            // Status 为公司状态。
	Remark string       `gorm:"type:text"`                                              // Remark 为备注。
	AuditFields
}

// Customer 客户主表。
//
//	用于存储客户稳定画像信息，不包含平台身份映射和多联系方式明细。
type Customer struct {
	ID            int64        `gorm:"primaryKey;autoIncrement"`                    // ID 为客户主键。
	Name          string       `gorm:"type:varchar(100);not null;default:'';index"` // Name 为客户姓名或展示名称。
	Gender        enums.Gender `gorm:"type:int;not null;default:0;"`                // Gender 为性别：0未知 1男 2女。
	CompanyID     int64        `gorm:"type:bigint;not null;default:0;index"`        // CompanyID 为所属公司ID；0表示无所属公司（个人客户）。
	LastActiveAt  *time.Time   `gorm:"type:timestamp;"`                             // LastActiveAt 为最近活跃时间。
	PrimaryMobile string       `gorm:"type:varchar(32);not null;default:'';index"`  // PrimaryMobile 为主手机号（冗余展示字段）。
	PrimaryEmail  string       `gorm:"type:varchar(100);not null;default:'';index"` // PrimaryEmail 为主邮箱（冗余展示字段）。
	Status        enums.Status `gorm:"type:int;not null;default:0;"`                // Status 为客户状态。
	Remark        string       `gorm:"type:text"`                                   // Remark 为备注。
	AuditFields
}

// CustomerIdentity 客户第三方身份映射表。
type CustomerIdentity struct {
	ID             int64                `gorm:"primaryKey;autoIncrement"`
	CustomerID     int64                `gorm:"type:bigint;not null;index"`                                               // 为所属客户ID。
	ExternalSource enums.ExternalSource `gorm:"type:varchar(30);uniqueIndex:uk_customer_external"`                        // 为外部身份来源
	ExternalID     string               `gorm:"type:varchar(128);index:idx_external_id;uniqueIndex:uk_customer_external"` // 为平台侧用户唯一ID，与访客 ExternalID 对齐。
	RawProfile     string               `gorm:"type:text"`                                                                // 为第三方原始资料JSON。
	Status         enums.Status         `gorm:"type:int;not null;default:0;index"`                                        // 为映射状态。
	AuditFields
}

// CustomerContact 客户联系方式表。
//
//	用于维护客户的一对多联系方式，支持主联系方式、验证状态与失效标记。
type CustomerContact struct {
	ID           int64             `gorm:"primaryKey;autoIncrement"`
	CustomerID   int64             `gorm:"type:bigint;not null;index;uniqueIndex:uk_customer_contact"`                  // CustomerID 为所属客户ID。
	ContactType  enums.ContactType `gorm:"type:varchar(30);not null;default:'';index;uniqueIndex:uk_customer_contact"`  // ContactType 为联系方式类型：mobile/email/wechat/other。
	ContactValue string            `gorm:"type:varchar(200);not null;default:'';index;uniqueIndex:uk_customer_contact"` // ContactValue 为联系方式值。
	IsPrimary    bool              `gorm:"not null;default:false;index"`                                                // IsPrimary 表示是否主联系方式。
	IsVerified   bool              `gorm:"not null;default:false;index"`                                                // IsVerified 表示是否已验证。
	VerifiedAt   *time.Time        `gorm:"type:timestamp"`                                                              // VerifiedAt 为验证时间。
	Source       string            `gorm:"type:varchar(30);not null;default:'';index"`                                  // Source 为来源：manual/import/system。
	Status       enums.Status      `gorm:"type:int;not null;default:0;index"`                                           // Status 为联系方式状态。
	Remark       string            `gorm:"type:varchar(255);not null;default:''"`                                       // Remark 为备注。
	AuditFields
}

// Role 角色定义。
type Role struct {
	ID         int64        `gorm:"primaryKey;autoIncrement"`
	TenantID   int64        `gorm:"type:bigint;not null;default:0;index"`       // TenantID 为租户 ID；0 表示平台级角色。
	DomainType string       `gorm:"type:varchar(30);not null;default:'';index"` // DomainType 角色所属域：platform / enterprise / customer。
	Name       string       `gorm:"type:varchar(100);not null;default:'';index"`
	Code       string       `gorm:"type:varchar(100);not null;uniqueIndex"`
	Status     enums.Status `gorm:"type:int;not null;default:0;index"`
	IsSystem   bool         `gorm:"not null;default:false;index"`
	Scope      string       `gorm:"type:varchar(50);not null;default:'';index"` // Scope 角色作用范围：all_tenant / department / self / custom。
	SortNo     int          `gorm:"type:int;not null;default:0;index"`
	Remark     string       `gorm:"type:text"`
	AuditFields
}

// Permission 权限点定义。
type Permission struct {
	ID        int64        `gorm:"primaryKey;autoIncrement"`
	Name      string       `gorm:"type:varchar(100);not null;default:''"`
	Code      string       `gorm:"type:varchar(150);not null;uniqueIndex"`
	Type      string       `gorm:"type:varchar(20);not null;default:'';index"`
	GroupName string       `gorm:"type:varchar(100);not null;default:'';index"`
	ParentID  int64        `gorm:"type:bigint;not null;default:0;index"`
	Path      string       `gorm:"type:varchar(255);not null;default:''"`
	Method    string       `gorm:"type:varchar(20);not null;default:''"`
	APIPath   string       `gorm:"type:varchar(255);not null;default:''"`
	SortNo    int          `gorm:"type:int;not null;default:0;index"`
	Status    enums.Status `gorm:"type:int;not null;default:0;index"`
	IsBuiltin bool         `gorm:"not null;default:true;index"`
	Remark    string       `gorm:"type:text"`
	AuditFields
}

// UserRole 用户和角色关联（角色绑定）。
type UserRole struct {
	ID          int64  `gorm:"primaryKey;autoIncrement"`
	TenantID    int64  `gorm:"type:bigint;not null;default:0;index"`       // TenantID 为租户 ID。
	DomainType  string `gorm:"type:varchar(30);not null;default:'';index"` // DomainType 绑定所属域：platform / enterprise / customer。
	SubjectType string `gorm:"type:varchar(30);not null;default:'';index"` // SubjectType 主体类型：platform_staff / enterprise_member / customer / partner。
	UserID      int64  `gorm:"type:bigint;not null;index;uniqueIndex:uk_user_role"`
	RoleID      int64  `gorm:"type:bigint;not null;index;uniqueIndex:uk_user_role"`
	AuditFields
}

// RolePermission 角色和权限关联。
type RolePermission struct {
	ID           int64  `gorm:"primaryKey;autoIncrement"`
	TenantID     int64  `gorm:"type:bigint;not null;default:0;index"`       // TenantID 为租户 ID。
	DomainType   string `gorm:"type:varchar(30);not null;default:'';index"` // DomainType 关联所属域。
	RoleID       int64  `gorm:"type:bigint;not null;index;uniqueIndex:uk_role_permission"`
	PermissionID int64  `gorm:"type:bigint;not null;index;uniqueIndex:uk_role_permission"`
	AuditFields
}

// UserPermission 用户级例外权限。
//
//	用于处理少量临时授权或拒绝授权场景。
type UserPermission struct {
	ID           int64      `gorm:"primaryKey;autoIncrement"`
	TenantID     int64      `gorm:"type:bigint;not null;default:0;index"`       // TenantID 为租户 ID。
	DomainType   string     `gorm:"type:varchar(30);not null;default:'';index"` // DomainType 所属域。
	SubjectType  string     `gorm:"type:varchar(30);not null;default:'';index"` // SubjectType 主体类型。
	UserID       int64      `gorm:"type:bigint;not null;index;uniqueIndex:uk_user_permission"`
	PermissionID int64      `gorm:"type:bigint;not null;index;uniqueIndex:uk_user_permission"`
	Effect       int        `gorm:"type:int;not null;default:1;index"` // Effect 表示权限生效方式：1允许 -1拒绝。
	ExpiredAt    *time.Time `gorm:"type:timestamp"`
	Remark       string     `gorm:"type:text"`
	AuditFields
}

// LoginSession 表示一次后台登录会话。
type LoginSession struct {
	ID             int64      `gorm:"primaryKey;autoIncrement"`                   // ID 为登录会话主键。
	UserID         int64      `gorm:"type:bigint;not null;index"`                 // UserID 为登录用户 ID。
	Token          string     `gorm:"type:varchar(128);not null;uniqueIndex"`     // Token 为随机不透明登录凭证，使用 ak_ 前缀。
	ClientType     string     `gorm:"type:varchar(50);not null;default:'';index"` // ClientType 为客户端类型，后台 Web 端固定为 admin_web。
	DomainType     string     `gorm:"type:varchar(32);not null;default:'';index"` // DomainType 固定本次登录使用的平台、企业、供应商或客户身份域。
	TenantID       int64      `gorm:"type:bigint;not null;default:0;index"`       // TenantID 为代管会话的目标租户，普通会话为 0。
	SubjectType    string     `gorm:"type:varchar(32);not null;default:'';index"` // SubjectType 为代管会话在目标域使用的主体类型。
	SubjectID      int64      `gorm:"type:bigint;not null;default:0;index"`       // SubjectID 为代管会话在目标域使用的主体 ID。
	SupportGrantID int64      `gorm:"type:bigint;not null;default:0;index"`       // SupportGrantID 关联平台协助或客户代访的临时授权。
	SupportMode    string     `gorm:"type:varchar(32);not null;default:'';index"` // SupportMode 区分平台协助、客户代访和员工代登录。
	ImpersonatedBy string     `gorm:"type:varchar(100);not null;default:''"`      // ImpersonatedBy 记录发起授权协助的原账号。
	ClientIP       string     `gorm:"type:varchar(64);not null;default:''"`       // ClientIP 为登录请求来源 IP。
	UserAgent      string     `gorm:"type:varchar(255);not null;default:''"`      // UserAgent 为登录请求浏览器或客户端 UA。
	ExpiredAt      time.Time  `gorm:"type:timestamp;not null;index"`              // ExpiredAt 为 token 过期时间。
	RevokedAt      *time.Time `gorm:"type:timestamp;index"`                       // RevokedAt 为主动注销或踢下线时间，非空表示已失效。
	LastSeenAt     *time.Time `gorm:"type:timestamp"`                             // LastSeenAt 为最近一次成功鉴权时间。
	AuditFields
}

// LoginCredentialLog 记录一次后台登录凭证校验结果。
type LoginCredentialLog struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`                    // ID 为登录凭证日志主键。
	Principal string    `gorm:"type:varchar(100);not null;default:'';index"` // Principal 为用户输入的登录名。
	UserID    int64     `gorm:"type:bigint;not null;default:0;index"`        // UserID 为匹配到的用户 ID，未匹配时为 0。
	Success   bool      `gorm:"not null;default:false;index"`                // Success 表示本次凭证校验是否成功。
	ClientIP  string    `gorm:"type:varchar(64);not null;default:''"`        // ClientIP 为登录请求来源 IP。
	UserAgent string    `gorm:"type:varchar(255);not null;default:''"`       // UserAgent 为登录请求浏览器或客户端 UA。
	Reason    string    `gorm:"type:varchar(255);not null;default:''"`       // Reason 为校验结果原因。
	CreatedAt time.Time `gorm:"type:timestamp;not null;index"`               // CreatedAt 为日志创建时间。
}

// Asset 存储的文件资源，如上传的附件等。
type Asset struct {
	ID             int64               `gorm:"primaryKey;autoIncrement"`
	TenantID       int64               `gorm:"type:bigint;not null;default:0;index"`
	ConversationID int64               `gorm:"type:bigint;not null;default:0;index"`
	AssetID        string              `gorm:"type:varchar(64);not null;uniqueIndex"`
	Provider       enums.AssetProvider `gorm:"type:varchar(50);not null;default:'';index"`
	StorageKey     string              `gorm:"type:varchar(255);not null;default:'';uniqueIndex:uk_storage_key"`
	Filename       string              `gorm:"type:varchar(255);not null;default:''"`
	FileSize       int64               `gorm:"type:bigint;not null;default:0;check:ck_asset_file_size_nonnegative,file_size >= 0"`
	MimeType       string              `gorm:"type:varchar(100);not null;default:''"`
	Status         enums.AssetStatus   `gorm:"type:int;not null;default:1;index"`
	AuditFields
}

var ErrAssetFileSizeNegative = errors.New("asset file size must not be negative")

// BeforeSave prevents invalid metadata from reaching any supported database,
// including development databases created before the CHECK constraint existed.
func (a *Asset) BeforeSave(_ *gorm.DB) error {
	if a != nil && a.FileSize < 0 {
		return ErrAssetFileSizeNegative
	}
	return nil
}

type Tag struct {
	ID       int64        `gorm:"primaryKey;autoIncrement"`
	ParentID int64        `gorm:"type:bigint;not null;index"`
	Name     string       `gorm:"type:varchar(50);not null;"`
	Remark   string       `gorm:"type:text;"`
	SortNo   int          `gorm:"type:int;not null;default:0"`
	Status   enums.Status `gorm:"type:int;not null;default:0"`
	AuditFields
}

// Conversation 客服会话。
type Conversation struct {
	ID                     int64                           `gorm:"primaryKey;autoIncrement"`                    // ID 为会话主键。
	AIAgentID              int64                           `gorm:"type:bigint;not null;default:0;index"`        // AIAgentID 为当前会话绑定的 AI Agent ID。
	ChannelID              int64                           `gorm:"type:bigint;not null;default:0;index"`        // ChannelID 为该会话来源接入渠道ID。
	CustomerID             int64                           `gorm:"type:bigint;not null;default:0;index"`        // CustomerID 为会话所属客户 ID。
	CustomerName           string                          `gorm:"type:varchar(100);not null;default:'';index"` // CustomerName 为客户名称冗余字段，用于列表展示和搜索。
	TenantID               int64                           `gorm:"type:bigint;not null;default:0;index"`        // TenantID 为设备售后租户上下文。
	ProductID              int64                           `gorm:"type:bigint;not null;default:0;index"`        // ProductID 为关联产品上下文。
	ProductModelID         int64                           `gorm:"type:bigint;not null;default:0;index"`        // ProductModelID 为关联产品型号上下文。
	DeviceID               int64                           `gorm:"type:bigint;not null;default:0;index"`        // DeviceID 为关联设备上下文。
	ServiceCodeID          int64                           `gorm:"type:bigint;not null;default:0;index"`        // ServiceCodeID 为关联服务码上下文。
	CustomerEntrySessionID int64                           `gorm:"type:bigint;not null;default:0;index"`        // CustomerEntrySessionID 为客户扫码入口会话，用于校验入口作用域并关联售后上下文。
	Status                 enums.IMConversationStatus      `gorm:"type:int;not null;default:1;index"`           // Status 为会话状态，如待接入、处理中、已关闭。
	ServiceMode            enums.IMConversationServiceMode `gorm:"type:int;not null;default:3;index"`           // ServiceMode 为服务模式，如仅AI、仅人工、AI优先人工接管。
	Priority               int                             `gorm:"type:int;not null;default:0;index"`           // Priority 为会话优先级。
	CurrentAssigneeID      int64                           `gorm:"type:bigint;not null;default:0;index"`        // CurrentAssigneeID 为当前接待客服ID。
	CurrentTeamID          int64                           `gorm:"type:bigint;not null;default:0;index"`        // CurrentTeamID 为当前处理客服组ID。
	LastMessageID          int64                           `gorm:"type:bigint;not null;default:0;index"`        // LastMessageID 为最后一条消息ID。
	LastMessageAt          time.Time                       `gorm:"type:timestamp;index"`                        // LastMessageAt 为最后消息时间。
	LastActiveAt           time.Time                       `gorm:"type:timestamp;index"`                        // LastActiveAt 为会话最近活跃时间。
	LastMessageSummary     string                          `gorm:"type:varchar(255);not null;default:''"`       // LastMessageSummary 为最后一条消息摘要。
	CustomerUnreadCount    int                             `gorm:"type:int;not null;default:0"`                 // CustomerUnreadCount 为用户侧未读数。
	AgentUnreadCount       int                             `gorm:"type:int;not null;default:0"`                 // AgentUnreadCount 为客服侧未读数。
	HandoffAt              *time.Time                      `gorm:"type:timestamp;index"`                        // HandoffAt 为最近一次转人工时间。
	HandoffReason          string                          `gorm:"type:varchar(255);not null;default:''"`       // HandoffReason 为最近一次转人工原因。
	AIReplyRounds          int                             `gorm:"type:int;not null;default:0"`                 // AIReplyRounds 为当前会话内 AI 已成功回复次数。
	ClosedAt               *time.Time                      `gorm:"type:timestamp;index"`                        // ClosedAt 为会话关闭时间。
	ClosedBy               int64                           `gorm:"type:bigint;not null;default:0;index"`        // ClosedBy 为关闭人用户ID，访客关闭时写0。
	CloseReason            string                          `gorm:"type:varchar(255);not null;default:''"`       // CloseReason 为关闭原因。
	AuditFields
}

// ConversationParticipant 会话参与方。
type ConversationParticipant struct {
	ID                    int64        `gorm:"primaryKey;autoIncrement"`
	ConversationID        int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_conversation_participant"`
	ParticipantType       string       `gorm:"type:varchar(30);not null;default:'';index;uniqueIndex:uk_conversation_participant"`
	ParticipantID         int64        `gorm:"type:bigint;not null;default:0;uniqueIndex:uk_conversation_participant"`
	ExternalParticipantID string       `gorm:"type:varchar(128);not null;default:''"`
	JoinedAt              *time.Time   `gorm:"type:timestamp"`
	LeftAt                *time.Time   `gorm:"type:timestamp"`
	Status                enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

// ConversationReadState 会话读游标。
type ConversationReadState struct {
	ID                int64              `gorm:"primaryKey;autoIncrement"`
	TenantID          int64              `gorm:"type:bigint;not null;default:0;index"`
	ConversationID    int64              `gorm:"type:bigint;not null;index;uniqueIndex:uk_conversation_reader"`
	ReaderType        enums.IMSenderType `gorm:"type:varchar(30);not null;default:'';index;uniqueIndex:uk_conversation_reader"`
	ReaderID          int64              `gorm:"type:bigint;not null;default:0;uniqueIndex:uk_conversation_reader"`
	ExternalReaderID  string             `gorm:"type:varchar(128);not null;default:'';uniqueIndex:uk_conversation_reader"`
	LastReadMessageID int64              `gorm:"type:bigint;not null;default:0;index"`
	LastReadAt        *time.Time         `gorm:"type:timestamp"`
	AuditFields
}

// Message 会话消息。
type Message struct {
	ID              int64                 `gorm:"primaryKey;autoIncrement"`
	ConversationID  int64                 `gorm:"type:bigint;not null;index;uniqueIndex:uk_conversation_client_msg"`
	RequestID       string                `gorm:"type:varchar(128);not null;default:'';index"`
	WorkflowRunID   int64                 `gorm:"type:bigint;not null;default:0;index"`
	ClientMsgID     string                `gorm:"type:varchar(128);not null;default:'';uniqueIndex:uk_conversation_client_msg"`
	SenderType      enums.IMSenderType    `gorm:"type:varchar(30);not null;default:'';index"`
	SenderID        int64                 `gorm:"type:bigint;not null;default:0;index"`
	ReceiverType    string                `gorm:"type:varchar(30);not null;default:'';index"`
	MessageType     enums.IMMessageType   `gorm:"type:varchar(30);not null;default:'';index"`
	Content         string                `gorm:"type:text"`
	Payload         string                `gorm:"type:text"`
	SendStatus      enums.IMMessageStatus `gorm:"type:int;not null;default:2;index"`
	SentAt          *time.Time            `gorm:"type:timestamp;index"`
	DeliveredAt     *time.Time            `gorm:"type:timestamp"`
	ReadAt          *time.Time            `gorm:"type:timestamp"`
	RecalledAt      *time.Time            `gorm:"type:timestamp"`
	QuotedMessageID int64                 `gorm:"type:bigint;not null;default:0;index"`
	AuditFields
}

// WxWorkKFSyncState 企业微信客服消息同步状态表。
//
//	按 open_kfid 记录企业微信客服消息同步游标，用于 SyncMsg 增量拉取。
type WxWorkKFSyncState struct {
	ID         int64        `gorm:"primaryKey;autoIncrement"`                         // ID 为同步状态主键。
	OpenKfID   string       `gorm:"type:varchar(64);not null;default:'';uniqueIndex"` // OpenKfID 为企业微信客服账号ID。
	NextCursor string       `gorm:"type:varchar(128);not null;default:''"`            // NextCursor 为下一次增量同步使用的游标。
	LastSyncAt *time.Time   `gorm:"type:timestamp;index"`                             // LastSyncAt 为最近一次成功同步时间。
	Status     enums.Status `gorm:"type:int;not null;default:0;index"`                // Status 为同步状态记录状态。
	Remark     string       `gorm:"type:text"`                                        // Remark 为同步异常、人工备注等补充信息。
	AuditFields
}

// WxWorkKFConversation 企业微信客服渠道会话映射表。
//
//	维护平台会话与企业微信客服会话上下文的对应关系，供入站同步和下行发送复用。
type WxWorkKFConversation struct {
	ID             int64        `gorm:"primaryKey;autoIncrement"`                                   // ID 为渠道会话映射主键。
	ConversationID int64        `gorm:"type:bigint;not null;uniqueIndex"`                           // ConversationID 为平台会话ID，一条平台会话仅对应一条当前有效渠道映射。
	ChannelID      int64        `gorm:"type:bigint;not null;default:0;index"`                       // ChannelID 为所属接入渠道ID，用于标识该会话来自哪个企业微信渠道配置。
	OpenKfID       string       `gorm:"type:varchar(64);not null;default:'';index:idx_openkf_ext"`  // OpenKfID 为企业微信客服账号ID。
	ExternalUserID string       `gorm:"type:varchar(128);not null;default:'';index:idx_openkf_ext"` // ExternalUserID 为企业微信客户ID。
	ServicerUserID string       `gorm:"type:varchar(128);not null;default:'';index"`                // ServicerUserID 为企业微信当前接待客服成员UserID。
	SessionStatus  string       `gorm:"type:varchar(30);not null;default:'';index"`                 // SessionStatus 为微信侧会话状态快照，如接入中、转接中、已结束。
	LastWxMsgID    string       `gorm:"type:varchar(64);not null;default:'';index"`                 // LastWxMsgID 为最近一次同步到的微信消息ID。
	LastWxMsgTime  *time.Time   `gorm:"type:timestamp;index"`                                       // LastWxMsgTime 为最近一次微信消息时间。
	RawProfile     string       `gorm:"type:text"`                                                  // RawProfile 为微信侧原始会话补充信息JSON。
	Status         enums.Status `gorm:"type:int;not null;default:0;index"`                          // Status 为渠道会话映射状态。
	AuditFields
}

// WxWorkKFMessageRef 企业微信客服消息映射表。
//
//	用于实现微信消息幂等消费，并保存平台消息与微信消息的双向映射关系。
type WxWorkKFMessageRef struct {
	ID             int64        `gorm:"primaryKey;autoIncrement"`                         // ID 为消息映射主键。
	ConversationID int64        `gorm:"type:bigint;not null;default:0;index"`             // ConversationID 为所属平台会话ID。
	MessageID      int64        `gorm:"type:bigint;not null;default:0;index"`             // MessageID 为所属平台消息ID；仅渠道消息尚未生成平台消息时可暂为0。
	WxMsgID        string       `gorm:"type:varchar(64);not null;default:'';uniqueIndex"` // WxMsgID 为企业微信消息ID，用于幂等去重。
	Direction      string       `gorm:"type:varchar(20);not null;default:'';index"`       // Direction 为消息方向，如 in/out。
	Origin         int          `gorm:"type:int;not null;default:0;index"`                // Origin 为企业微信消息来源值，如客户发送、系统事件、企微客户端发送。
	OpenKfID       string       `gorm:"type:varchar(64);not null;default:'';index"`       // OpenKfID 为发送或接收该消息的客服账号ID。
	ExternalUserID string       `gorm:"type:varchar(128);not null;default:'';index"`      // ExternalUserID 为消息对应的企业微信客户ID。
	SendStatus     string       `gorm:"type:varchar(30);not null;default:'';index"`       // SendStatus 为渠道发送状态快照，如 sent、failed。
	FailReason     string       `gorm:"type:text"`                                        // FailReason 为渠道发送失败原因或补偿说明。
	RawPayload     string       `gorm:"type:text"`                                        // RawPayload 为企业微信原始消息JSON。
	Status         enums.Status `gorm:"type:int;not null;default:0;index"`                // Status 为消息映射状态。
	AuditFields
}

// ChannelMessageOutbox 外部渠道消息投递任务表。
//
//	用于记录平台消息提交后的渠道发送任务，保证第三方发送动作与主事务解耦。
type ChannelMessageOutbox struct {
	ID             int64      `gorm:"primaryKey;autoIncrement"`                                                  // ID 为投递任务主键。
	ChannelType    string     `gorm:"type:varchar(30);not null;default:'';index;uniqueIndex:uk_channel_message"` // ChannelType 为目标渠道类型，如 wxwork_kf。
	ConversationID int64      `gorm:"type:bigint;not null;default:0;index"`                                      // ConversationID 为所属平台会话ID。
	MessageID      int64      `gorm:"type:bigint;not null;default:0;uniqueIndex:uk_channel_message"`             // MessageID 为待投递的平台消息ID。
	Payload        string     `gorm:"type:text"`                                                                 // Payload 为渠道发送所需的标准化请求数据JSON。
	SendStatus     string     `gorm:"type:varchar(30);not null;default:'';index"`                                // SendStatus 为当前投递状态，如 pending、sending、sent、failed。
	RetryCount     int        `gorm:"type:int;not null;default:0"`                                               // RetryCount 为已重试次数。
	NextRetryAt    *time.Time `gorm:"type:timestamp;index"`                                                      // NextRetryAt 为下一次允许重试时间。
	LastError      string     `gorm:"type:text"`                                                                 // LastError 为最近一次发送失败信息。
	SentAt         *time.Time `gorm:"type:timestamp;index"`                                                      // SentAt 为最终发送成功时间。
	AuditFields
}

// ConversationAssignment 会话接待关系。
type ConversationAssignment struct {
	ID             int64                    `gorm:"primaryKey;autoIncrement"`
	ConversationID int64                    `gorm:"type:bigint;not null;index"`
	FromUserID     int64                    `gorm:"type:bigint;not null;default:0;index"`
	ToUserID       int64                    `gorm:"type:bigint;not null;default:0;index"`
	AssignType     string                   `gorm:"type:varchar(30);not null;default:'';index"`
	Reason         string                   `gorm:"type:varchar(255);not null;default:''"`
	Status         enums.IMAssignmentStatus `gorm:"type:int;not null;index"`
	CreatedAt      time.Time                `gorm:"type:timestamp;not null;index"`
	FinishedAt     *time.Time               `gorm:"type:timestamp"`
	OperatorID     int64                    `gorm:"type:bigint;not null;default:0;index"`
}

// ConversationTag 会话标签关联
type ConversationTag struct {
	ID             int64 `gorm:"primaryKey;autoIncrement"`
	ConversationID int64 `gorm:"type:bigint;not null;index;uniqueIndex:uk_conversation_tag"`
	TagID          int64 `gorm:"type:bigint;not null;index;uniqueIndex:uk_conversation_tag"`
	AuditFields
}

// QuickReply 快捷回复。
type QuickReply struct {
	ID        int64        `gorm:"primaryKey;autoIncrement"`
	TenantID  int64        `gorm:"type:bigint;not null;default:0;index"`
	GroupName string       `gorm:"type:varchar(50);not null;default:'';index"`
	Title     string       `gorm:"type:varchar(100);not null;default:'';index"`
	Content   string       `gorm:"type:text"`
	Status    enums.Status `gorm:"type:int;not null;index"`
	SortNo    int          `gorm:"type:int;not null;index"`
	AuditFields
}

// AIAgent AI 接待实例。
type AIAgent struct {
	ID                  int64                           `gorm:"primaryKey;autoIncrement"`             // ID 为 AI Agent 主键。
	TenantID            int64                           `gorm:"type:bigint;not null;default:0;index"` // TenantID 为企业租户；0 表示平台级 Agent。
	ProductID           int64                           `gorm:"type:bigint;not null;default:0;index"` // ProductID 为产品客服机器人归属产品；0 表示通用 Agent。
	Source              string                          `gorm:"type:varchar(32);not null;default:'manual';index"`
	Name                string                          `gorm:"type:varchar(100);not null;default:'';index"` // Name 为 AI Agent 名称。
	Description         string                          `gorm:"type:varchar(255);not null;default:''"`       // Description 为 AI Agent 描述。
	Status              enums.Status                    `gorm:"type:int;not null;index"`                     // Status 为 AI Agent
	AIConfigID          int64                           `gorm:"type:bigint;not null;default:0;index"`        // AIConfigID 为关联的 AI 配置ID。
	LLMModelName        string                          `gorm:"type:varchar(100);not null;default:''"`       // LLMModelName 为统一 AI 能力路由下该 Agent 选择的模型；空值跟随平台默认模型。
	ServiceMode         enums.IMConversationServiceMode `gorm:"type:int;not null;default:3;index"`           // ServiceMode 为服务模式，如仅AI、仅人工、AI优先人工接管。
	SystemPrompt        string                          `gorm:"type:text"`                                   // SystemPrompt 为该 Agent 的系统提示词。
	WelcomeMessage      string                          `gorm:"type:text"`                                   // WelcomeMessage 为该 Agent 的欢迎语或首响模板。
	ReplyTimeoutSeconds int                             `gorm:"type:int;not null;default:180"`               // ReplyTimeoutSeconds 为异步自动回复超时秒数。
	TeamIDs             string                          `gorm:"type:varchar(500);not null;default:''"`       // TeamIDs 为转人工时可路由的客服组ID列表，多个之间使用逗号分隔。
	HandoffMode         enums.AIAgentHandoffMode        `gorm:"type:int;not null;default:1"`                 // HandoffMode 为转人工模式，如进入待接入池、进入默认客服组待接入池。
	FallbackMode        enums.AIAgentFallbackMode       `gorm:"type:int;not null;default:1"`                 // FallbackMode 为知识库未命中时的兜底策略。
	FallbackMessage     string                          `gorm:"type:text"`                                   // FallbackMessage 为兜底回复文案。
	KnowledgeIDs        string                          `gorm:"type:varchar(500);not null;default:''"`       // KnowledgeIDs 为绑定的知识库ID列表，按顺序表示优先级。
	SkillIDs            string                          `gorm:"type:varchar(500);not null;default:''"`       // SkillIDs 为绑定的技能ID列表，按顺序表示允许路由的范围。
	AllowedMCPTools     string                          `gorm:"type:text"`                                   // AllowedMCPTools 为允许 direct tool 路由的 MCP 工具白名单配置JSON。
	AllowedGraphTools   string                          `gorm:"type:text"`                                   // AllowedGraphTools 为允许 Graph Tool 的白名单配置JSON。
	WorkflowID          int64                           `gorm:"type:bigint;not null;default:0;index"`        // WorkflowID 为草稿当前选择的可复用 Workflow 模板。
	WorkflowVersionID   int64                           `gorm:"type:bigint;not null;default:0;index"`        // WorkflowVersionID 为绑定的已发布会话流程版本ID。
	ActiveReleaseID     int64                           `gorm:"type:bigint;not null;default:0;index"`        // ActiveReleaseID 为用户端实际运行的不可变 Release。
	DraftRevision       int64                           `gorm:"type:bigint;not null;default:1"`              // DraftRevision 为 Agent 草稿配置修订号。
	ReviewStatus        enums.AIAgentReviewStatus       `gorm:"type:varchar(20);not null;default:'approved';index"`
	ReviewComment       string                          `gorm:"type:text"`
	ReviewedAt          *time.Time                      `gorm:"type:timestamp;index"`
	ReviewedByID        int64                           `gorm:"type:bigint;not null;default:0;index"`
	ReviewedByName      string                          `gorm:"type:varchar(100);not null;default:''"`
	SortNo              int                             `gorm:"type:int;not null;default:0;index"` // SortNo 为后台展示排序号。
	AuditFields
}

// AIWorkflow 表示客服 AI Agent 可编辑会话流程主表。
type AIWorkflow struct {
	ID                     int64        `gorm:"primaryKey;autoIncrement"`
	TenantID               int64        `gorm:"type:bigint;not null;default:0;index"`
	Code                   string       `gorm:"type:varchar(100);not null;default:'';index"`
	Scope                  string       `gorm:"type:varchar(20);not null;default:'tenant';index"`
	Name                   string       `gorm:"type:varchar(100);not null;default:'';index"`
	Description            string       `gorm:"type:text"`
	AgentID                int64        `gorm:"type:bigint;not null;default:0;index"`
	SourceWorkflowID       int64        `gorm:"type:bigint;not null;default:0;index"`
	SourceVersionID        int64        `gorm:"type:bigint;not null;default:0;index"`
	Status                 enums.Status `gorm:"type:int;not null;default:0;index"`
	DraftDefinition        string       `gorm:"type:text"`
	PublishedVersionID     int64        `gorm:"type:bigint;not null;default:0;index"`
	CurrentStableVersionID int64        `gorm:"type:bigint;not null;default:0;index"`
	Locked                 bool         `gorm:"not null;default:false;index"`
	SortNo                 int          `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

// AIWorkflowVersion 表示 AI 会话流程的不可变发布版本。
type AIWorkflowVersion struct {
	ID                  int64        `gorm:"primaryKey;autoIncrement"`
	WorkflowID          int64        `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_ai_workflow_version_no"`
	Version             int          `gorm:"type:int;not null;default:0;index;uniqueIndex:uk_ai_workflow_version_no"`
	Status              enums.Status `gorm:"type:int;not null;default:0;index"`
	Definition          string       `gorm:"type:text"`
	DefinitionHash      string       `gorm:"type:varchar(64);not null;default:'';index"`
	ModelPolicySnapshot string       `gorm:"type:text;not null;default:'{}'"`
	ReleaseChannel      string       `gorm:"type:varchar(20);not null;default:'stable';index"`
	SchemaVersion       int          `gorm:"type:int;not null;default:1"`
	ChangeSummary       string       `gorm:"type:text"`
	SourceVersionID     int64        `gorm:"type:bigint;not null;default:0;index"`
	PublishedAt         *time.Time   `gorm:"type:timestamp;index"`
	PublishedByID       int64        `gorm:"type:bigint;not null;default:0;index"`
	PublishedByName     string       `gorm:"type:varchar(100);not null;default:''"`
	AuditFields
}

// AIAgentRelease freezes the complete reviewed configuration used in production.
type AIAgentRelease struct {
	ID                     int64                     `gorm:"primaryKey;autoIncrement"`
	TenantID               int64                     `gorm:"type:bigint;not null;default:0;index"`
	ProductID              int64                     `gorm:"type:bigint;not null;default:0;index"`
	AgentID                int64                     `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_ai_agent_release_no;uniqueIndex:ux_ai_agent_release_active,where:deployment_status = 'active' AND status <> 2"`
	ReleaseNo              int                       `gorm:"type:int;not null;default:0;uniqueIndex:uk_ai_agent_release_no"`
	WorkflowID             int64                     `gorm:"type:bigint;not null;default:0;index"`
	WorkflowVersionID      int64                     `gorm:"type:bigint;not null;default:0;index"`
	WorkflowDefinitionHash string                    `gorm:"type:varchar(64);not null;default:'';index"`
	AgentConfigSnapshot    string                    `gorm:"type:text;not null;default:'{}'"`
	AgentConfigHash        string                    `gorm:"type:varchar(64);not null;default:'';index"`
	KnowledgeScopeSnapshot string                    `gorm:"type:text;not null;default:'{}'"`
	KnowledgeScopeHash     string                    `gorm:"type:varchar(64);not null;default:'';index"`
	ReviewStatus           enums.AIAgentReviewStatus `gorm:"type:varchar(20);not null;default:'unreviewed';index"`
	ReviewComment          string                    `gorm:"type:text"`
	ReviewedAt             *time.Time                `gorm:"type:timestamp;index"`
	ReviewedByID           int64                     `gorm:"type:bigint;not null;default:0;index"`
	ReviewedByName         string                    `gorm:"type:varchar(100);not null;default:''"`
	DeploymentStatus       string                    `gorm:"type:varchar(20);not null;default:'inactive';index"`
	DeployedAt             *time.Time                `gorm:"type:timestamp;index"`
	DeployedByID           int64                     `gorm:"type:bigint;not null;default:0;index"`
	DeployedByName         string                    `gorm:"type:varchar(100);not null;default:''"`
	RollbackFromReleaseID  int64                     `gorm:"type:bigint;not null;default:0;index"`
	Status                 enums.Status              `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

const (
	AIWorkflowScopePlatform = "platform"
	AIWorkflowScopeTenant   = "tenant"

	AIWorkflowReleaseChannelStable     = "stable"
	AIWorkflowReleaseChannelPreview    = "preview"
	AIWorkflowReleaseChannelDeprecated = "deprecated"
	AIWorkflowReleaseChannelRevoked    = "revoked"

	AIAgentReleaseDeploymentInactive   = "inactive"
	AIAgentReleaseDeploymentActive     = "active"
	AIAgentReleaseDeploymentRetired    = "retired"
	AIAgentReleaseDeploymentRolledBack = "rolled_back"
)

// AIWorkflowRun 表示一次会话 workflow 执行记录。
type AIWorkflowRun struct {
	ID                int64      `gorm:"primaryKey;autoIncrement"`
	TenantID          int64      `gorm:"type:bigint;not null;default:0;index"`
	ProductID         int64      `gorm:"type:bigint;not null;default:0;index"`
	WorkflowID        int64      `gorm:"type:bigint;not null;default:0;index"`
	WorkflowVersionID int64      `gorm:"type:bigint;not null;default:0;index"`
	DefinitionHash    string     `gorm:"type:varchar(64);not null;default:'';index"`
	RuntimeEngine     string     `gorm:"type:varchar(32);not null;default:'';index"`
	ConversationID    int64      `gorm:"type:bigint;not null;default:0;index"`
	AIAgentID         int64      `gorm:"type:bigint;not null;default:0;index"`
	AgentReleaseID    int64      `gorm:"type:bigint;not null;default:0;index"`
	MessageID         int64      `gorm:"type:bigint;not null;default:0;index"`
	Status            int        `gorm:"type:int;not null;default:0;index"`
	StartedAt         time.Time  `gorm:"type:timestamp;not null;index"`
	EndedAt           *time.Time `gorm:"type:timestamp;index"`
	InterruptType     string     `gorm:"type:varchar(50);not null;default:'';index"`
	InterruptNodeID   string     `gorm:"type:varchar(100);not null;default:'';index"`
	ErrorMessage      string     `gorm:"type:text"`
	ConfigSnapshot    string     `gorm:"type:text"`
	TraceData         string     `gorm:"type:text"`
	AuditFields
}

// AIWorkflowNodeRun 表示 workflow 执行中的单节点审计记录。
type AIWorkflowNodeRun struct {
	ID             int64      `gorm:"primaryKey;autoIncrement"`
	WorkflowRunID  int64      `gorm:"type:bigint;not null;default:0;index"`
	NodeID         string     `gorm:"type:varchar(100);not null;default:'';index"`
	NodeType       string     `gorm:"type:varchar(50);not null;default:'';index"`
	Attempt        int        `gorm:"type:int;not null;default:1"`
	IdempotencyKey string     `gorm:"type:varchar(160);not null;default:'';index"`
	Status         int        `gorm:"type:int;not null;default:0;index"`
	InputPreview   string     `gorm:"type:text"`
	OutputPreview  string     `gorm:"type:text"`
	ErrorMessage   string     `gorm:"type:text"`
	StartedAt      time.Time  `gorm:"type:timestamp;not null;index"`
	EndedAt        *time.Time `gorm:"type:timestamp;index"`
	DurationMS     int        `gorm:"type:int;not null;default:0"`
}

// AIWorkflowEffect records idempotent high-risk workflow side effects. A lease
// prevents concurrent execution; the business operation remains retryable
// after a crash through its own idempotency key.
type AIWorkflowEffect struct {
	ID                int64      `gorm:"primaryKey;autoIncrement"`
	TenantID          int64      `gorm:"type:bigint;not null;default:0;index"`
	ProductID         int64      `gorm:"type:bigint;not null;default:0;index"`
	WorkflowVersionID int64      `gorm:"type:bigint;not null;default:0;index"`
	AgentReleaseID    int64      `gorm:"type:bigint;not null;default:0;index"`
	ConversationID    int64      `gorm:"type:bigint;not null;default:0;index"`
	MessageID         int64      `gorm:"type:bigint;not null;default:0;index"`
	NodeID            string     `gorm:"type:varchar(100);not null;default:'';index"`
	EffectType        string     `gorm:"type:varchar(50);not null;default:'';index"`
	IdempotencyKey    string     `gorm:"type:varchar(180);not null;uniqueIndex"`
	Status            string     `gorm:"type:varchar(20);not null;default:'pending';index"`
	Attempt           int        `gorm:"type:int;not null;default:0"`
	RequestData       string     `gorm:"type:text"`
	ResultData        string     `gorm:"type:text"`
	LastError         string     `gorm:"type:text"`
	LockedUntil       *time.Time `gorm:"type:timestamp;index"`
	CompletedAt       *time.Time `gorm:"type:timestamp;index"`
	CreatedAt         time.Time  `gorm:"type:timestamp;not null;index"`
	UpdatedAt         time.Time  `gorm:"type:timestamp;not null;index"`
}

const (
	AIWorkflowEffectStatusPending   = "pending"
	AIWorkflowEffectStatusRunning   = "running"
	AIWorkflowEffectStatusSucceeded = "succeeded"
	AIWorkflowEffectStatusFailed    = "failed"
)

// Channel 接入渠道配置。
//
//	用于统一描述系统的外部接入入口。不同渠道类型共享统一的接入配置骨架，
//	例如网页客服渠道（web）和企业微信客服渠道（wxwork_kf）。
//	渠道本身负责定义“入口如何识别、默认接入哪个 AI Agent、渠道专属配置是什么”，
//	而具体消息收发、会话映射等运行时数据由各自的渠道业务表承载。
type Channel struct {
	ID          int64  `gorm:"primaryKey;autoIncrement"`                         // ID 为渠道主键。
	Name        string `gorm:"type:varchar(100);not null;default:'';index"`      // Name 为渠道名称，用于后台展示和业务识别，例如“官网客服”“企业微信主客服”。
	ChannelType string `gorm:"type:varchar(30);not null;default:'';index"`       // ChannelType 为渠道类型，决定该渠道的接入方式和配置解释规则。当前规划的典型取值包括：web、wxwork_kf。
	ChannelID   string `gorm:"type:varchar(64);not null;default:'';uniqueIndex"` // ChannelID 为渠道入口标识，由系统自动生成。对 web 渠道，该字段用于前端通过 X-Channel-Id 标识接入来源；对其他渠道，作为统一的系统内稳定渠道标识保留。
	AIAgentID   int64  `gorm:"type:bigint;not null;default:0;index"`             // AIAgentID 为该渠道默认接入的 AI Agent。 当外部客户通过该渠道首次进入系统且尚未命中现有未结束会话时，系统会使用该 AI Agent 作为会话默认接待实例。
	// ConfigJSON 为渠道专属扩展配置，使用 JSON 存储。
	// 例如：
	// 1. web 渠道可记录允许域名、品牌配置等；
	// 2. wxwork_kf 渠道可记录 openKfId、欢迎语策略等。
	// 该字段只存储渠道类型私有配置，不承载通用主字段。
	ConfigJSON string       `gorm:"type:text"`
	Status     enums.Status `gorm:"type:int;not null;default:0;index"` // Status 为渠道状态。禁用后，该渠道不再允许新会话接入；删除时采用软删除状态保留历史关联数据。
	Remark     string       `gorm:"type:text"`                         // Remark 为渠道备注，用于记录接入说明、维护说明和内部运维信息。
	AuditFields
}

// ConversationEventLog 会话事件日志。
type ConversationEventLog struct {
	ID             int64              `gorm:"primaryKey;autoIncrement"`
	ConversationID int64              `gorm:"type:bigint;not null;index"`
	RequestID      string             `gorm:"type:varchar(128);not null;default:'';index"`
	EventType      enums.IMEventType  `gorm:"type:varchar(50);not null;default:'';index"`
	OperatorType   enums.IMSenderType `gorm:"type:varchar(30);not null;default:'';index"`
	OperatorID     int64              `gorm:"type:bigint;not null;default:0;index"`
	Content        string             `gorm:"type:text"`
	Payload        string             `gorm:"type:text"`
	CreatedAt      time.Time          `gorm:"type:timestamp;not null;index"`
}

// Ticket 客服问题记录。
type Ticket struct {
	ID                          int64              `gorm:"primaryKey;autoIncrement"`
	TicketNo                    string             `gorm:"type:varchar(64);not null;default:'';uniqueIndex"`
	IdempotencyKey              *string            `gorm:"type:varchar(160);uniqueIndex:uk_ticket_tenant_idempotency,priority:2"`
	Title                       string             `gorm:"type:varchar(255);not null;default:'';index"`
	Description                 string             `gorm:"type:text"`
	Source                      enums.TicketSource `gorm:"type:varchar(50);not null;default:'';index"`
	Channel                     string             `gorm:"type:varchar(50);not null;default:'';index"`
	SourceRecordID              string             `gorm:"type:varchar(160);not null;default:'';index"`
	SourceRecordKey             *string            `gorm:"type:varchar(64);uniqueIndex:uk_ticket_intake_source,priority:2"`
	ProjectKey                  string             `gorm:"type:varchar(64);not null;default:''"`
	TicketType                  string             `gorm:"type:varchar(64);not null;default:''"`
	CallerName                  string             `gorm:"type:varchar(120);not null;default:''"`
	CallerPhone                 string             `gorm:"type:varchar(64);not null;default:''"`
	ReceivedAt                  *time.Time         `gorm:"type:timestamp"`
	ContextStatus               string             `gorm:"type:varchar(32);not null;default:'not_evaluated'"`
	MissingContextJSON          string             `gorm:"type:text;not null;default:'[]'"`
	CustomerID                  int64              `gorm:"type:bigint;not null;default:0;index"`
	CustomerRegistrationGrantID int64              `gorm:"type:bigint;not null;default:0;index"`
	ConversationID              int64              `gorm:"type:bigint;not null;default:0;index"`
	Status                      enums.TicketStatus `gorm:"type:varchar(50);not null;default:'pending';index"`
	PriorityCode                string             `gorm:"type:varchar(8);not null;default:'p2';index"`
	CurrentTeamID               int64              `gorm:"type:bigint;not null;default:0;index"`
	CurrentAssigneeID           int64              `gorm:"type:bigint;not null;default:0;index"`
	HandledAt                   *time.Time         `gorm:"type:timestamp;index"`
	TenantID                    int64              `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_ticket_tenant_idempotency,priority:1;uniqueIndex:uk_ticket_intake_source,priority:1"`
	ProductID                   int64              `gorm:"type:bigint;not null;default:0;index"`
	ProductModelID              int64              `gorm:"type:bigint;not null;default:0;index"`
	ProductModuleID             int64              `gorm:"type:bigint;not null;default:0;index"`
	DeviceID                    int64              `gorm:"type:bigint;not null;default:0;index"`
	ServiceCodeID               int64              `gorm:"type:bigint;not null;default:0;index"`
	CustomerEntrySessionID      int64              `gorm:"type:bigint;not null;default:0;index"`
	ServiceRegion               string             `gorm:"type:varchar(64);not null;default:'';index"`
	FaultCode                   string             `gorm:"type:varchar(128);not null;default:'';index"`
	SymptomSummary              string             `gorm:"type:text"`
	DiagnosisSummary            string             `gorm:"type:text"`
	SLADueAt                    *time.Time         `gorm:"type:timestamp;index"`
	ResolvedAt                  *time.Time         `gorm:"type:timestamp;index"`
	// AssignedAt 自动派单/人工指派写入负责人的时间。
	AssignedAt *time.Time `gorm:"type:timestamp;index"`
	// AcceptedAt 工程师明确确认接单的时间，用于接单 SLA 计时。
	AcceptedAt *time.Time `gorm:"type:timestamp;index"`
	// AcceptDeadlineAt 接单截止时间：负责人需在此之前确认接单，逾期回收重派。
	AcceptDeadlineAt *time.Time `gorm:"type:timestamp;index"`
	// DispatchAttempts 接单超时回收重派的累计次数，达到上限后升级主管。
	DispatchAttempts int `gorm:"type:int;not null;default:0;index"`
	// DispatchDeferredUntil 自动派单失败后的下一次重试时间，避免永久不可派工单阻塞扫描窗口。
	DispatchDeferredUntil *time.Time `gorm:"type:timestamp;index"`
	// LastDispatchFailureReason 记录最近一次自动派单失败原因，用于工作台展示和排障。
	LastDispatchFailureReason string `gorm:"type:varchar(64);not null;default:'';index"`
	AuditFields
}

// TicketDispatchAttempt 记录每次派单及其最终结果，便于审计接单耗时和重派链路。
type TicketDispatchAttempt struct {
	ID               int64      `gorm:"primaryKey;autoIncrement"`
	TenantID         int64      `gorm:"type:bigint;not null;default:0;index"`
	TicketID         int64      `gorm:"type:bigint;not null;index"`
	TeamID           int64      `gorm:"type:bigint;not null;default:0;index"`
	AssigneeID       int64      `gorm:"type:bigint;not null;default:0;index"`
	AttemptNo        int        `gorm:"type:int;not null;default:1"`
	Outcome          string     `gorm:"type:varchar(32);not null;default:'pending';index"`
	Reason           string     `gorm:"type:varchar(255);not null;default:''"`
	AssignedAt       time.Time  `gorm:"type:timestamp;not null;index"`
	AcceptDeadlineAt *time.Time `gorm:"type:timestamp;index"`
	AcceptedAt       *time.Time `gorm:"type:timestamp;index"`
	EndedAt          *time.Time `gorm:"type:timestamp;index"`
	AuditFields
}

// TicketTag 工单标签关联。
type TicketTag struct {
	ID       int64 `gorm:"primaryKey;autoIncrement"`
	TenantID int64 `gorm:"type:bigint;not null;default:0;index"`
	TicketID int64 `gorm:"type:bigint;not null;index;uniqueIndex:uk_ticket_tag"`
	TagID    int64 `gorm:"type:bigint;not null;index;uniqueIndex:uk_ticket_tag"`
	AuditFields
}

// TicketProgress 工单处理进展。
type TicketProgress struct {
	ID                int64                         `gorm:"primaryKey;autoIncrement"`
	TenantID          int64                         `gorm:"type:bigint;not null;default:0;index"`
	TicketID          int64                         `gorm:"type:bigint;not null;index"`
	EventType         enums.TicketProgressEventType `gorm:"type:varchar(50);not null;default:'progress';index"`
	Content           string                        `gorm:"type:text"`
	VisibleToCustomer bool                          `gorm:"not null;default:false;index"`
	MetadataJSON      string                        `gorm:"column:metadata_json;type:text;not null;default:'{}'"`
	AuthorID          int64                         `gorm:"type:bigint;not null;default:0;index"`
	CreatedAt         time.Time                     `gorm:"type:timestamp;not null;index"`
}

// TicketRepairRecord 工单维修记录。工单关闭时填写本次维修结论、根因、处理方法和测试结果。
type TicketRepairRecord struct {
	ID                int64      `gorm:"primaryKey;autoIncrement"`
	TenantID          int64      `gorm:"type:bigint;not null;index"`
	TicketID          int64      `gorm:"type:bigint;not null;index"`
	DeviceID          int64      `gorm:"type:bigint;not null;default:0;index"`
	ProductID         int64      `gorm:"type:bigint;not null;default:0;index"`
	ProductModelID    int64      `gorm:"type:bigint;not null;default:0;index"`
	ServiceCodeID     int64      `gorm:"type:bigint;not null;default:0;index"`
	ServiceMethod     string     `gorm:"type:varchar(64);not null;default:'';index"` // remote/onsite/depot/replacement
	Conclusion        string     `gorm:"type:text"`
	RootCause         string     `gorm:"type:text"`
	RepairMethod      string     `gorm:"type:text"`
	Solution          string     `gorm:"type:text"`
	TestResult        string     `gorm:"type:varchar(64);not null;default:'';index"` // passed/failed/partial
	WarrantyCovered   bool       `gorm:"not null;default:false;index"`
	RemoteResolved    bool       `gorm:"not null;default:false;index"`
	VisibleToCustomer bool       `gorm:"not null;default:false;index"`
	PartsJSON         string     `gorm:"column:parts_json;type:text;not null;default:'[]'"`
	CostHours         float64    `gorm:"type:decimal(10,2);not null;default:0"`
	StartedAt         *time.Time `gorm:"type:timestamp"`
	FinishedAt        *time.Time `gorm:"type:timestamp"`
	MetadataJSON      string     `gorm:"column:metadata_json;type:text;not null;default:'{}'"`
	AuditFields
}

// TicketFeedback 工单客户评价。低分评价会触发产品质量线索（设计 §6/§7）。
type TicketFeedback struct {
	ID             int64     `gorm:"primaryKey;autoIncrement"`
	TenantID       int64     `gorm:"type:bigint;not null;index"`
	TicketID       int64     `gorm:"type:bigint;not null;index"`
	CustomerUserID int64     `gorm:"type:bigint;not null;default:0;index"`
	Rating         int       `gorm:"type:int;not null;default:0;index"` // 1-5
	TagsJSON       string    `gorm:"column:tags_json;type:text;not null;default:'[]'"`
	Comment        string    `gorm:"type:text"`
	Status         string    `gorm:"type:varchar(32);not null;default:'submitted';index"`
	SubmittedAt    time.Time `gorm:"type:timestamp;not null;index"`
	AuditFields
}

// DeviceServiceRecord 设备正式维修/服务历史记录，工单关闭后生成。
type DeviceServiceRecord struct {
	ID                int64     `gorm:"primaryKey;autoIncrement"`
	TenantID          int64     `gorm:"type:bigint;not null;index"`
	DeviceID          int64     `gorm:"type:bigint;not null;index"`
	ProductID         int64     `gorm:"type:bigint;not null;default:0;index"`
	ProductModelID    int64     `gorm:"type:bigint;not null;default:0;index"`
	SourceType        string    `gorm:"type:varchar(32);not null;default:'';index"` // ticket/meeting/manual
	SourceID          string    `gorm:"type:varchar(64);not null;default:'';index"`
	TicketID          int64     `gorm:"type:bigint;not null;default:0;index"`
	MeetingID         string    `gorm:"type:varchar(36);not null;default:'';index"`
	ServiceType       string    `gorm:"type:varchar(32);not null;default:'';index"` // repair/inspection/maintenance/replacement
	Summary           string    `gorm:"type:text"`
	RootCause         string    `gorm:"type:text"`
	Solution          string    `gorm:"type:text"`
	VisibleToCustomer bool      `gorm:"not null;default:true;index"`
	OccurredAt        time.Time `gorm:"type:timestamp;not null;index"`
	MetadataJSON      string    `gorm:"column:metadata_json;type:text;not null;default:'{}'"`
	AuditFields
}

// AgentProfile 客服档案。
type AgentProfile struct {
	ID                    int64               `gorm:"primaryKey;autoIncrement"`                         // ID 为客服档案主键。
	TenantID              int64               `gorm:"type:bigint;not null;default:0;index"`             // TenantID 为客服档案所属租户。
	UserID                int64               `gorm:"type:bigint;not null;uniqueIndex"`                 // UserID 关联后台用户，一名用户只允许一份客服档案。
	TeamID                int64               `gorm:"type:bigint;not null;default:0;index"`             // TeamID 为客服所属客服组。
	AgentCode             string              `gorm:"type:varchar(64);not null;default:'';uniqueIndex"` // AgentCode 为客服工号，用于业务侧识别客服。
	DisplayName           string              `gorm:"type:varchar(100);not null;default:'';index"`      // DisplayName 为客服展示名，可区别于后台昵称。
	Avatar                string              `gorm:"type:varchar(1024);not null;default:''"`           // Avatar 为客服头像 URL。
	ServiceStatus         enums.ServiceStatus `gorm:"type:int;not null;default:0;index"`                // ServiceStatus 表示客服服务状态：0空闲 1忙碌。
	MaxConcurrentCount    int                 `gorm:"type:int;not null;default:0"`                      // MaxConcurrentCount 表示客服最大并发接待数。
	PriorityLevel         int                 `gorm:"type:int;not null;default:0;index"`                // PriorityLevel 表示自动分配优先级，值越大越优先。
	AutoAssignEnabled     bool                `gorm:"not null;default:true;index"`                      // AutoAssignEnabled 表示是否参与自动分配。
	ReceiveOfflineMessage bool                `gorm:"not null;default:false"`                           // ReceiveOfflineMessage 表示离线时是否仍接收离线消息或转接消息。
	LastOnlineAt          *time.Time          `gorm:"type:timestamp;index"`                             // LastOnlineAt 记录最近一次在线时间。
	LastStatusAt          *time.Time          `gorm:"type:timestamp;index"`                             // LastStatusAt 记录最近一次状态变更时间。
	Status                enums.Status        `gorm:"type:int;not null;default:0;index"`                // Status 表示客服档案状态
	Remark                string              `gorm:"type:text"`                                        // Remark 记录客服备注信息。
	AuditFields
}

// AgentTeam 客服组。
type AgentTeam struct {
	ID               int64        `gorm:"primaryKey;autoIncrement"`                                                     // ID 为客服组主键。
	TenantID         int64        `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_agent_team_tenant_system"` // TenantID 为客服组所属租户。
	ParentID         int64        `gorm:"type:bigint;not null;default:0;index"`                                         // ParentID 为上级服务组织节点。
	ProductID        int64        `gorm:"type:bigint;not null;default:0;index"`                                         // ProductID 为产品维修组关联产品；0 表示非产品组。
	DepartmentID     int64        `gorm:"type:bigint;not null;default:0;index"`                                         // DepartmentID 关联企业组织架构节点。
	TeamType         string       `gorm:"type:varchar(32);not null;default:'custom';index"`                             // TeamType 为 technical_repair / product_repair / custom。
	SystemKey        *string      `gorm:"type:varchar(96);uniqueIndex:uk_agent_team_tenant_system"`                     // SystemKey 为系统组幂等键；自定义组保持 NULL。
	SystemManaged    bool         `gorm:"not null;default:false;index"`                                                 // SystemManaged 表示由租户或产品生命周期维护。
	Name             string       `gorm:"type:varchar(100);not null;default:'';index"`                                  // Name 为客服组名称。
	LeaderUserID     int64        `gorm:"type:bigint;not null;default:0;index"`                                         // LeaderUserID 为组长用户ID，0 表示暂未设置。
	AssignmentMode   string       `gorm:"type:varchar(32);not null;default:'balanced';index"`                           // AssignmentMode 为自动派单策略：balanced / weighted。
	ScheduleEnforced bool         `gorm:"not null;default:false;index"`                                                 // ScheduleEnforced 为历史排班兼容字段；派单以成员个人可接单规则为准。
	ScheduleVersion  int          `gorm:"type:int;not null;default:0"`                                                  // ScheduleVersion 为历史排班版本字段，仅供只读展示兼容。
	Status           enums.Status `gorm:"type:int;not null;default:0;index"`                                            // Status 表示客服组状态
	Description      string       `gorm:"type:varchar(255);not null;default:''"`                                        // Description 为客服组简介，用于说明职责边界。
	Remark           string       `gorm:"type:text"`                                                                    // Remark 记录客服组内部备注。
	AuditFields
}

// AgentTeamMember 产品维修组成员。
type AgentTeamMember struct {
	ID              int64        `gorm:"primaryKey;autoIncrement"`
	TenantID        int64        `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_agent_team_member"`
	TeamID          int64        `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_agent_team_member"`
	UserID          int64        `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_agent_team_member"`
	MemberID        int64        `gorm:"type:bigint;not null;default:0;index"` // MemberID 关联 tenant_members，便于回到企业组织身份。
	DispatchEnabled bool         `gorm:"not null;default:false;index"`         // DispatchEnabled 表示该成员是否参与自动派单。
	DispatchWeight  int          `gorm:"type:int;not null;default:1"`          // DispatchWeight 为加权派单权重，最小为 1。
	Status          enums.Status `gorm:"type:int;not null;default:0;index"`
	Remark          string       `gorm:"type:text"`
	AuditFields
}

// AgentTeamSchedule 客服组排班。
type AgentTeamSchedule struct {
	ID             int64        `gorm:"primaryKey;autoIncrement"`                            // ID 为组排班主键。
	TenantID       int64        `gorm:"type:bigint;not null;default:0;index"`                // TenantID 为排班所属租户。
	TeamID         int64        `gorm:"type:bigint;not null;index"`                          // TeamID 为被排班的客服组ID。
	UserID         int64        `gorm:"type:bigint;not null;default:0;index"`                // UserID 为值班成员；0 表示兼容旧的整组排班。
	RepeatType     string       `gorm:"type:varchar(16);not null;default:'once';index"`      // RepeatType 为 once / weekly。
	DayType        string       `gorm:"type:varchar(16);not null;default:'work';index"`      // DayType 为 work / rest；历史空值按 work 兼容。
	Weekday        int          `gorm:"type:int;not null;default:0;index"`                   // Weekday 为每周模板的星期，1-7 表示周一至周日。
	StartMinute    int          `gorm:"type:int;not null;default:0"`                         // StartMinute 为每周模板当天的开始分钟数。
	EndMinute      int          `gorm:"type:int;not null;default:0"`                         // EndMinute 为每周模板当天的结束分钟数。
	Timezone       string       `gorm:"type:varchar(64);not null;default:'';index"`          // Timezone 为 IANA 时区；空值仅兼容历史数据并回退到服务器时区。
	EffectiveFrom  *time.Time   `gorm:"type:date;index"`                                     // EffectiveFrom 为周模板生效日期（含）。
	EffectiveUntil *time.Time   `gorm:"type:date;index"`                                     // EffectiveUntil 为周模板失效日期（含）。
	PublishStatus  string       `gorm:"type:varchar(16);not null;default:'published';index"` // PublishStatus 为 draft / published / archived。
	Version        int          `gorm:"type:int;not null;default:1;index"`                   // Version 为所属产品组的排班版本。
	StartAt        time.Time    `gorm:"type:timestamp;not null;index"`                       // StartAt 为单次班次时间；周模板保存稳定锚点以兼容旧查询。
	EndAt          time.Time    `gorm:"type:timestamp;not null;index"`                       // EndAt 为单次班次时间；周模板保存稳定锚点以兼容旧查询。
	Remark         string       `gorm:"type:varchar(255);not null;default:''"`               // Remark 记录排班备注。
	Status         enums.Status `gorm:"type:int;not null;default:0;index"`                   // Status 表示组排班记录状态。
	AuditFields
}

// AgentTeamScheduleTemplate 企业级默认维修排班模板。
type AgentTeamScheduleTemplate struct {
	ID          int64        `gorm:"primaryKey;autoIncrement"`
	TenantID    int64        `gorm:"type:bigint;not null;default:0;uniqueIndex"`
	Workdays    string       `gorm:"type:text;not null;default:'[1,2,3,4,5]'"` // Workdays 为 JSON 星期数组，1-7 表示周一至周日。
	StartMinute int          `gorm:"type:int;not null;default:0"`
	EndMinute   int          `gorm:"type:int;not null;default:1440"`
	Timezone    string       `gorm:"type:varchar(64);not null;default:'Asia/Shanghai'"`
	Status      enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

// AgentScheduleException records engineer-level exceptions to the enterprise
// work calendar. An approved leave applies to every product team the engineer
// belongs to; product membership is deliberately not duplicated here.
type AgentScheduleException struct {
	ID               int64        `gorm:"primaryKey;autoIncrement"`
	TenantID         int64        `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_agent_schedule_exception_request"`
	UserID           int64        `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_agent_schedule_exception_request"`
	RequestKey       string       `gorm:"type:varchar(80);not null;default:'';uniqueIndex:uk_agent_schedule_exception_request"`
	ExceptionType    string       `gorm:"type:varchar(24);not null;default:'leave';index"`
	StartAt          time.Time    `gorm:"type:timestamp;not null;index"`
	EndAt            time.Time    `gorm:"type:timestamp;not null;index"`
	ApprovalStatus   string       `gorm:"type:varchar(24);not null;default:'pending';index"`
	Reason           string       `gorm:"type:varchar(255);not null;default:''"`
	ReviewNote       string       `gorm:"type:varchar(255);not null;default:''"`
	RequestedAt      time.Time    `gorm:"type:timestamp;not null;index"`
	ReviewedAt       *time.Time   `gorm:"type:timestamp;index"`
	ReviewerUserID   int64        `gorm:"type:bigint;not null;default:0;index"`
	ReviewerUserName string       `gorm:"type:varchar(100);not null;default:''"`
	Status           enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

// AgentTeamHoliday stores team-level holiday or no-dispatch dates. It is a
// dispatch exception calendar layered on top of published weekly schedules.
type AgentTeamHoliday struct {
	ID          int64        `gorm:"primaryKey;autoIncrement"`
	TenantID    int64        `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_agent_team_holiday"`
	TeamID      int64        `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_agent_team_holiday"`
	HolidayDate time.Time    `gorm:"type:date;not null;index;uniqueIndex:uk_agent_team_holiday"`
	Timezone    string       `gorm:"type:varchar(64);not null;default:'';index"`
	Name        string       `gorm:"type:varchar(100);not null;default:''"`
	Remark      string       `gorm:"type:varchar(255);not null;default:''"`
	Status      enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

// AgentWorkStatus 是工程师本人维护的展示和 presence 状态，不直接决定自动派单资格。
// 自动派单资格由工作日、已批准请假、档案/成员配置、容量和可达性共同计算。
type AgentWorkStatus struct {
	ID              int64      `gorm:"primaryKey;autoIncrement"`
	TenantID        int64      `gorm:"type:bigint;not null;index;uniqueIndex:uk_agent_work_status_tenant_user"`
	UserID          int64      `gorm:"type:bigint;not null;index;uniqueIndex:uk_agent_work_status_tenant_user"`
	Status          string     `gorm:"type:varchar(24);not null;default:'available';index"`
	Note            string     `gorm:"type:varchar(255);not null;default:''"`
	AvailableAt     *time.Time `gorm:"type:timestamp;index"`
	ConfirmedAt     time.Time  `gorm:"type:timestamp;not null;index"`
	StatusChangedAt time.Time  `gorm:"type:timestamp;not null;index"`
	AuditFields
}

// AIConfig AI 统一配置。
// 一条记录表示一个可直接调用的 AI 配置实例，
// 同时包含厂商接入信息、模型信息和调用参数，不再拆分 endpoint/model 两层概念。
type AIConfig struct {
	ID                        int64             `gorm:"primaryKey;autoIncrement"`                    // ID 为配置主键。
	Name                      string            `gorm:"type:varchar(100);not null;default:'';index"` // Name 为配置名称，用于后台识别和展示。
	Provider                  enums.AIProvider  `gorm:"type:varchar(50);not null;default:'';index"`  // Provider 为供应商标识，例如 openai、azure_openai、dashscope。
	BaseURL                   string            `gorm:"type:varchar(255);not null;default:''"`       // BaseURL 为模型服务基础地址，例如 https://api.openai.com/v1。
	APIKey                    string            `gorm:"type:varchar(255);not null;default:''"`       // APIKey 为服务端请求模型接口所需密钥。
	ModelType                 enums.AIModelType `gorm:"type:varchar(30);not null;default:'';index"`  // ModelType 为模型类型，例如 llm、embedding、rerank。
	ModelName                 string            `gorm:"type:varchar(100);not null;default:'';index"` // ModelName 为实际请求时传给上游的模型名。
	Dimension                 int               `gorm:"type:int;not null;default:0"`                 // Dimension 为向量维度，仅 embedding 模型通常需要填写。
	MaxContextTokens          int               `gorm:"type:int;not null;default:0"`                 // MaxContextTokens 为模型支持的最大上下文 token 数。
	MaxOutputTokens           int               `gorm:"type:int;not null;default:0"`                 // MaxOutputTokens 为模型建议的最大输出 token 数。
	TimeoutMS                 int               `gorm:"type:int;not null;default:30000"`             // TimeoutMS 为调用该配置的默认超时时间，单位毫秒。
	MaxRetryCount             int               `gorm:"type:int;not null;default:0"`                 // MaxRetryCount 为默认最大重试次数。
	RPMLimit                  int               `gorm:"type:int;not null;default:0"`                 // RPMLimit 为每分钟请求数限制，0 表示未显式配置。
	TPMLimit                  int               `gorm:"type:int;not null;default:0"`                 // TPMLimit 为每分钟 token 数限制，0 表示未显式配置。
	Status                    enums.Status      `gorm:"type:int;not null;index"`                     // Status 状态；同一 modelType 仅允许一条启用记录。
	SortNo                    int               `gorm:"type:int;not null;index"`                     // SortNo 为排序号，用于后台展示和人工调整顺序。
	Remark                    string            `gorm:"type:text"`                                   // Remark 为备注，用于记录用途、成本、限制和切换说明等补充信息。
	RuntimeCredentialScope    string            `gorm:"-" json:"-"`                                  // RuntimeCredentialScope 为本次调用实际解析到的账号作用域。
	RuntimeAPIKeyID           string            `gorm:"-" json:"-"`                                  // RuntimeAPIKeyID 为本次调用实际使用的 Key 标识，不包含密钥明文。
	RuntimeCredentialFallback bool              `gorm:"-" json:"-"`                                  // RuntimeCredentialFallback 表示产品 Key 不可用后回退到了租户默认 Key。
	AuditFields
}

// KnowledgeBase 知识库主表。
type KnowledgeBase struct {
	ID                    int64        `gorm:"primaryKey;autoIncrement"`                           // ID 为知识库主键。
	TenantID              int64        `gorm:"type:bigint;not null;default:0;index"`               // TenantID 为租户 ID。
	Name                  string       `gorm:"type:varchar(100);not null;default:'';index"`        // Name 为知识库名称。
	Description           string       `gorm:"type:text"`                                          // Description 为知识库描述。
	KnowledgeType         string       `gorm:"type:varchar(20);not null;default:'document';index"` // KnowledgeType 为知识库类型：document/faq。
	AccessScope           string       `gorm:"type:varchar(20);not null;default:'tenant';index"`   // AccessScope 为访问范围：tenant/product。
	RagflowDatasetID      string       `gorm:"column:ragflow_dataset_id;type:varchar(128);not null;default:'';index"`
	Status                enums.Status `gorm:"type:int;not null;index"`                        // Status 为状态
	DefaultTopK           int          `gorm:"type:int;not null;default:10"`                   // DefaultTopK 为默认召回数量。
	DefaultScoreThreshold float64      `gorm:"type:decimal(5,4);not null;default:0.5"`         // DefaultScoreThreshold 为默认相似度阈值。
	DefaultRerankLimit    int          `gorm:"type:int;not null;default:5"`                    // DefaultRerankLimit 为默认重排后保留数量。
	ChunkProvider         string       `gorm:"type:varchar(30);not null;default:'structured'"` // ChunkProvider 为知识库分块策略 provider。
	ChunkTargetTokens     int          `gorm:"type:int;not null;default:300"`                  // ChunkTargetTokens 为目标 chunk token 数。
	ChunkMaxTokens        int          `gorm:"type:int;not null;default:400"`                  // ChunkMaxTokens 为单 chunk 最大 token 数。
	ChunkOverlapTokens    int          `gorm:"type:int;not null;default:40"`                   // ChunkOverlapTokens 为相邻 chunk 重叠 token 数。
	AnswerMode            int          `gorm:"type:int;not null;default:1"`                    // AnswerMode 为回答模式：1严格知识库模式 2辅助解释模式。
	SortNo                int          `gorm:"type:int;not null;default:0;index"`              // SortNo 为排序号，用于后台展示和知识库的人工排序管理。
	Remark                string       `gorm:"type:text"`                                      // Remark 为备注。
	AuditFields
}

// KnowledgeDirectory 知识库内部目录表。
type KnowledgeDirectory struct {
	ID              int64        `gorm:"primaryKey;autoIncrement"`                                // ID 为目录主键。
	TenantID        int64        `gorm:"type:bigint;not null;default:0;index"`                    // TenantID 为租户 ID。
	KnowledgeBaseID int64        `gorm:"type:bigint;not null;index:idx_kb_parent_sort"`           // KnowledgeBaseID 为所属知识库 ID。
	ParentID        int64        `gorm:"type:bigint;not null;default:0;index:idx_kb_parent_sort"` // ParentID 为父目录 ID，0 表示一级目录。
	Name            string       `gorm:"type:varchar(100);not null;default:'';index"`             // Name 为目录名称。
	SortNo          int          `gorm:"type:int;not null;default:0;index:idx_kb_parent_sort"`    // SortNo 为同级排序号。
	Status          enums.Status `gorm:"type:int;not null;default:0;index"`                       // Status 为状态。
	Remark          string       `gorm:"type:text"`                                               // Remark 为备注。
	AuditFields
}

// KnowledgeDocument 知识文档主表。
type KnowledgeDocument struct {
	ID                  int64                              `gorm:"primaryKey;autoIncrement"`                                  // ID 为文档主键。
	TenantID            int64                              `gorm:"type:bigint;not null;default:0;index"`                      // TenantID 为租户 ID。
	KnowledgeBaseID     int64                              `gorm:"type:bigint;not null;index"`                                // KnowledgeBaseID 为所属知识库ID。
	DirectoryID         int64                              `gorm:"type:bigint;not null;default:0;index"`                      // DirectoryID 为所属知识库内部目录 ID，0 表示根目录。
	Title               string                             `gorm:"type:varchar(255);not null;default:'';index"`               // Title 为文档标题。
	ContentType         enums.KnowledgeDocumentContentType `gorm:"type:varchar(20);not null;default:'html'"`                  // ContentType 为内容类型：html/markdown。
	Content             string                             `gorm:"type:text"`                                                 // Content 为文档内容。
	SourceAssetID       int64                              `gorm:"type:bigint;not null;default:0;index"`                      // SourceAssetID 为上传知识文档的原始文件 Asset；编辑器文档为 0。
	SourceType          string                             `gorm:"type:varchar(32);not null;default:'knowledge_entry';index"` // SourceType 区分 uploaded_document/product_manual/knowledge_entry。
	SourceReferenceID   int64                              `gorm:"type:bigint;not null;default:0;index"`                      // SourceReferenceID 为手册等来源业务记录 ID。
	ReviewStatus        string                             `gorm:"type:varchar(20);not null;default:'draft';index"`           // ReviewStatus 为企业知识中心审核状态：draft/review/published/deprecated。
	Language            string                             `gorm:"type:varchar(16);not null;default:'default';index"`         // Language 为主语言。
	TagsJSON            string                             `gorm:"column:tags_json;type:text;not null;default:'[]'"`          // TagsJSON 为标签 JSON 数组。
	FaultCodesJSON      string                             `gorm:"column:fault_codes_json;type:text;not null;default:'[]'"`
	Status              enums.Status                       `gorm:"type:int;not null;default:0;index"`                 // Status 为状态
	IndexStatus         enums.KnowledgeDocumentIndexStatus `gorm:"type:varchar(20);not null;default:'pending';index"` // IndexStatus 为索引状态：pending/indexed/failed。
	IndexedAt           *time.Time                         `gorm:"type:timestamp;index"`                              // IndexedAt 为最近一次索引成功时间。
	IndexError          string                             `gorm:"type:text"`                                         // IndexError 为最近一次索引失败信息。
	ContentHash         string                             `gorm:"type:varchar(64);not null;default:'';index"`        // ContentHash 为内容哈希，用于变更检测。
	CurrentRevisionID   int64                              `gorm:"type:bigint;not null;default:0;index"`
	PublishedRevisionID int64                              `gorm:"type:bigint;not null;default:0;index"`
	DraftVersionNo      int                                `gorm:"type:int;not null;default:1"`
	AuditFields
}

// KnowledgeFAQ FAQ 条目主表。
type KnowledgeFAQ struct {
	ID                  int64                              `gorm:"primaryKey;autoIncrement"`                          // ID 为 FAQ 主键。
	TenantID            int64                              `gorm:"type:bigint;not null;default:0;index"`              // TenantID 为租户 ID。
	KnowledgeBaseID     int64                              `gorm:"type:bigint;not null;index"`                        // KnowledgeBaseID 为所属 FAQ 知识库 ID。
	DirectoryID         int64                              `gorm:"type:bigint;not null;default:0;index"`              // DirectoryID 为所属知识库内部目录 ID，0 表示根目录。
	Question            string                             `gorm:"type:varchar(500);not null;default:'';index"`       // Question 为标准问题。
	Answer              string                             `gorm:"type:text"`                                         // Answer 为标准答案。
	SimilarQuestions    string                             `gorm:"type:text"`                                         // SimilarQuestions 为相似问 JSON 数组。
	ReviewStatus        string                             `gorm:"type:varchar(20);not null;default:'draft';index"`   // ReviewStatus 为企业知识中心审核状态：draft/review/published/deprecated。
	Language            string                             `gorm:"type:varchar(16);not null;default:'default';index"` // Language 为主语言。
	TagsJSON            string                             `gorm:"column:tags_json;type:text;not null;default:'[]'"`  // TagsJSON 为标签 JSON 数组。
	FaultCodesJSON      string                             `gorm:"column:fault_codes_json;type:text;not null;default:'[]'"`
	Status              enums.Status                       `gorm:"type:int;not null;default:0;index"`                 // Status 为状态。
	IndexStatus         enums.KnowledgeDocumentIndexStatus `gorm:"type:varchar(20);not null;default:'pending';index"` // IndexStatus 为索引状态：pending/indexed/failed。
	IndexedAt           *time.Time                         `gorm:"type:timestamp;index"`                              // IndexedAt 为最近一次索引成功时间。
	IndexError          string                             `gorm:"type:text"`                                         // IndexError 为最近一次索引失败信息。
	Remark              string                             `gorm:"type:text"`                                         // Remark 为备注。
	CurrentRevisionID   int64                              `gorm:"type:bigint;not null;default:0;index"`
	PublishedRevisionID int64                              `gorm:"type:bigint;not null;default:0;index"`
	DraftVersionNo      int                                `gorm:"type:int;not null;default:1"`
	AuditFields
}

// KnowledgeRevision 知识条目的不可变发布修订快照。
type KnowledgeRevision struct {
	ID                int64      `gorm:"primaryKey;autoIncrement"`
	TenantID          int64      `gorm:"type:bigint;not null;index;uniqueIndex:uk_knowledge_revision_entry_version"`
	KnowledgeBaseID   int64      `gorm:"type:bigint;not null;index"`
	EntryType         string     `gorm:"type:varchar(20);not null;default:'';index;uniqueIndex:uk_knowledge_revision_entry_version"`
	EntryID           int64      `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_knowledge_revision_entry_version"`
	VersionNo         int        `gorm:"type:int;not null;default:1;uniqueIndex:uk_knowledge_revision_entry_version"`
	VersionLabel      string     `gorm:"type:varchar(64);not null;default:''"`
	Title             string     `gorm:"type:varchar(255);not null;default:''"`
	Content           string     `gorm:"type:text"`
	Language          string     `gorm:"type:varchar(16);not null;default:'default';index"`
	Visibility        string     `gorm:"type:varchar(32);not null;default:'internal';index"`
	TagsJSON          string     `gorm:"column:tags_json;type:text;not null;default:'[]'"`
	FaultCodesJSON    string     `gorm:"column:fault_codes_json;type:text;not null;default:'[]'"`
	ContentHash       string     `gorm:"type:varchar(64);not null;default:'';index"`
	ReviewStatus      string     `gorm:"type:varchar(20);not null;default:'published';index"`
	PublishedAt       *time.Time `gorm:"type:timestamp;index"`
	PublishedByID     int64      `gorm:"type:bigint;not null;default:0;index"`
	PublishedByName   string     `gorm:"type:varchar(100);not null;default:''"`
	SupersededAt      *time.Time `gorm:"type:timestamp;index"`
	ParserVersion     string     `gorm:"type:varchar(64);not null;default:''"`
	EmbeddingModel    string     `gorm:"type:varchar(100);not null;default:''"`
	EmbeddingVersion  string     `gorm:"type:varchar(64);not null;default:''"`
	IndexGenerationID int64      `gorm:"type:bigint;not null;default:0;index"`
	AuditFields
}

// KnowledgeChunk 切片元数据表。
type KnowledgeChunk struct {
	ID                int64        `gorm:"primaryKey;autoIncrement"`                   // ID 为切片主键。
	TenantID          int64        `gorm:"type:bigint;not null;default:0;index"`       // TenantID 为租户 ID。
	KnowledgeBaseID   int64        `gorm:"type:bigint;not null;index"`                 // KnowledgeBaseID 为知识库ID。
	DocumentID        int64        `gorm:"type:bigint;not null;default:0;index"`       // DocumentID 为文档ID。
	FaqID             int64        `gorm:"type:bigint;not null;default:0;index"`       // FaqID 为 FAQ ID。
	ChunkNo           int          `gorm:"type:int;not null;default:0;index"`          // ChunkNo 为切片序号。
	Title             string       `gorm:"type:varchar(255);not null;default:''"`      // Title 为切片标题。
	Content           string       `gorm:"type:text"`                                  // Content 为切片内容。
	ContentHash       string       `gorm:"type:varchar(64);not null;default:'';index"` // ContentHash 为内容哈希。
	CharCount         int          `gorm:"type:int;not null;default:0"`                // CharCount 为字符数。
	TokenCount        int          `gorm:"type:int;not null;default:0"`                // TokenCount 为token数。
	ChunkType         string       `gorm:"type:varchar(30);not null;default:''"`       // ChunkType 为切片类型。
	SectionPath       string       `gorm:"type:text"`                                  // SectionPath 为章节路径。
	Provider          string       `gorm:"type:varchar(30);not null;default:''"`       // Provider 为分块 provider。
	RevisionID        int64        `gorm:"type:bigint;not null;default:0;index"`
	Language          string       `gorm:"type:varchar(16);not null;default:'default';index"`
	Visibility        string       `gorm:"type:varchar(32);not null;default:'internal';index"`
	ReviewStatus      string       `gorm:"type:varchar(20);not null;default:'draft';index"`
	ParserVersion     string       `gorm:"type:varchar(64);not null;default:''"`
	EmbeddingModel    string       `gorm:"type:varchar(100);not null;default:''"`
	EmbeddingVersion  string       `gorm:"type:varchar(64);not null;default:''"`
	IndexGenerationID int64        `gorm:"type:bigint;not null;default:0;index"`
	Status            enums.Status `gorm:"type:int;not null;default:0;index"`           // Status 为状态：1有效 2已删除。
	VectorID          string       `gorm:"type:varchar(100);not null;default:'';index"` // VectorID 为向量库中的point ID。
	CreatedAt         time.Time    `gorm:"type:timestamp;not null;index"`
	UpdatedAt         time.Time    `gorm:"type:timestamp;not null;index"`
}

// KnowledgeIndexGeneration 记录当前知识向量索引 generation 和别名切换状态。
type KnowledgeIndexGeneration struct {
	ID               int64      `gorm:"primaryKey;autoIncrement"`
	CollectionName   string     `gorm:"type:varchar(128);not null;default:'';uniqueIndex"`
	CollectionAlias  string     `gorm:"type:varchar(128);not null;default:'';index"`
	SchemaVersion    int        `gorm:"type:int;not null;default:0"`
	EmbeddingModel   string     `gorm:"type:varchar(100);not null;default:''"`
	EmbeddingVersion string     `gorm:"type:varchar(64);not null;default:''"`
	Dimension        int        `gorm:"type:int;not null;default:0"`
	Status           string     `gorm:"type:varchar(20);not null;default:'building';index"`
	PointCount       int64      `gorm:"type:bigint;not null;default:0"`
	ActivatedAt      *time.Time `gorm:"type:timestamp"`
	RetiredAt        *time.Time `gorm:"type:timestamp"`
	AuditFields
}

// KnowledgeRetrieveLog 检索日志表。
type KnowledgeRetrieveLog struct {
	ID                 int64     `gorm:"primaryKey;autoIncrement"`             // ID 为日志主键。
	TenantID           int64     `gorm:"type:bigint;not null;default:0;index"` // TenantID 为租户 ID。
	KnowledgeBaseID    int64     `gorm:"type:bigint;not null;index"`           // KnowledgeBaseID 为知识库ID。
	CollectionName     string    `gorm:"type:varchar(128);not null;default:'';index"`
	IndexGenerationID  int64     `gorm:"type:bigint;not null;default:0;index"`
	Channel            string    `gorm:"type:varchar(30);not null;default:'';index"` // Channel 为渠道：im会话, agent_assist坐席辅助, api开放接口, debug调试。
	Scene              string    `gorm:"type:varchar(50);not null;default:'';index"` // Scene 为场景：first_response首响, assist辅助, qa问答。
	SessionID          string    `gorm:"type:varchar(64);not null;default:'';index"` // SessionID 为会话ID。
	ConversationID     int64     `gorm:"type:bigint;not null;default:0;index"`       // ConversationID 为会话ID。
	RequestID          string    `gorm:"type:varchar(64);not null;default:'';index"` // RequestID 为请求ID。
	Question           string    `gorm:"type:text"`                                  // Question 为原始问题。
	RewriteQuestion    string    `gorm:"type:text"`                                  // RewriteQuestion 为改写后问题。
	Answer             string    `gorm:"type:text"`                                  // Answer 为生成的答案。
	AnswerStatus       int       `gorm:"type:int;not null;default:1;index"`          // AnswerStatus 为答案状态：1正常 2无答案 3兜底 4风控拦截。
	HitCount           int       `gorm:"type:int;not null;default:0"`                // HitCount 为命中数量。
	TopScore           float64   `gorm:"type:decimal(5,4);not null;default:0"`       // TopScore 为最高相似度分数。
	ChunkProvider      string    `gorm:"type:varchar(30);not null;default:'';index"` // ChunkProvider 为分块 provider。
	ChunkTargetTokens  int       `gorm:"type:int;not null;default:0"`                // ChunkTargetTokens 为目标 token 数。
	ChunkMaxTokens     int       `gorm:"type:int;not null;default:0"`                // ChunkMaxTokens 为最大 token 数。
	ChunkOverlapTokens int       `gorm:"type:int;not null;default:0"`                // ChunkOverlapTokens 为重叠 token 数。
	RerankEnabled      bool      `gorm:"not null;default:false;index"`               // RerankEnabled 是否启用 rerank。
	RerankLimit        int       `gorm:"type:int;not null;default:0"`                // RerankLimit 为 rerank 条数。
	CitationCount      int       `gorm:"type:int;not null;default:0"`                // CitationCount 为最终引用条数。
	UsedChunkCount     int       `gorm:"type:int;not null;default:0"`                // UsedChunkCount 为进入上下文的 chunk 数。
	LatencyMs          int64     `gorm:"type:bigint;not null;default:0"`             // LatencyMs 为总耗时毫秒。
	RetrieveMs         int64     `gorm:"type:bigint;not null;default:0"`             // RetrieveMs 为检索耗时毫秒。
	GenerateMs         int64     `gorm:"type:bigint;not null;default:0"`             // GenerateMs 为生成耗时毫秒。
	PromptTokens       int       `gorm:"type:int;not null;default:0"`                // PromptTokens 为prompt token数。
	CompletionTokens   int       `gorm:"type:int;not null;default:0"`                // CompletionTokens 为completion token数。
	EmbeddingModel     string    `gorm:"type:varchar(100);not null;default:'';index"`
	ModelName          string    `gorm:"type:varchar(100);not null;default:''"` // ModelName 为使用的模型名称。
	NoAnswerReason     string    `gorm:"type:varchar(64);not null;default:'';index"`
	TraceData          string    `gorm:"type:text"` // TraceData 为链路追踪数据JSON。
	CreatedAt          time.Time `gorm:"type:timestamp;not null;index"`
}

// KnowledgeRetrieveHit 检索命中详情表。
type KnowledgeRetrieveHit struct {
	ID              int64     `gorm:"primaryKey;autoIncrement"`              // ID 为命中记录主键。
	TenantID        int64     `gorm:"type:bigint;not null;default:0;index"`  // TenantID 为租户 ID。
	RetrieveLogID   int64     `gorm:"type:bigint;not null;index"`            // RetrieveLogID 为检索日志ID。
	KnowledgeBaseID int64     `gorm:"type:bigint;not null;default:0;index"`  // KnowledgeBaseID 为命中来源知识库ID。
	ChunkID         int64     `gorm:"type:bigint;not null;index"`            // ChunkID 为切片ID。
	DocumentID      int64     `gorm:"type:bigint;not null;index"`            // DocumentID 为文档ID。
	DocumentTitle   string    `gorm:"type:varchar(255);not null;default:''"` // DocumentTitle 为文档标题。
	FaqID           int64     `gorm:"type:bigint;not null;default:0;index"`  // FaqID 为 FAQ ID。
	FaqQuestion     string    `gorm:"type:varchar(500);not null;default:''"` // FaqQuestion 为 FAQ 问题。
	ChunkNo         int       `gorm:"type:int;not null;default:0"`           // ChunkNo 为切片序号。
	Title           string    `gorm:"type:varchar(255);not null;default:''"` // Title 为切片标题。
	SectionPath     string    `gorm:"type:text"`                             // SectionPath 为章节路径。
	ChunkType       string    `gorm:"type:varchar(30);not null;default:''"`  // ChunkType 为切片类型。
	Provider        string    `gorm:"type:varchar(30);not null;default:''"`  // Provider 为分块 provider。
	RankNo          int       `gorm:"type:int;not null;default:0"`           // RankNo 为排名。
	Score           float64   `gorm:"type:decimal(5,4);not null;default:0"`  // Score 为相似度分数。
	RerankScore     float64   `gorm:"type:decimal(5,4);not null;default:0"`  // RerankScore 为重排分数。
	UsedInAnswer    bool      `gorm:"not null;default:false"`                // UsedInAnswer 是否用于生成答案。
	IsCitation      bool      `gorm:"not null;default:false"`                // IsCitation 是否作为引用返回。
	Snippet         string    `gorm:"type:text"`                             // Snippet 为内容片段。
	CreatedAt       time.Time `gorm:"type:timestamp;not null;index"`
}

// KnowledgeFeedback 问答反馈表。
type KnowledgeFeedback struct {
	ID             int64     `gorm:"primaryKey;autoIncrement"`              // ID 为反馈主键。
	TenantID       int64     `gorm:"type:bigint;not null;default:0;index"`  // TenantID 为租户 ID。
	RetrieveLogID  int64     `gorm:"type:bigint;not null;index"`            // RetrieveLogID 为检索日志ID。
	FeedbackType   int       `gorm:"type:int;not null;default:1;index"`     // FeedbackType 为反馈类型：1点赞 2点踩 3无帮助 4引用错误 5其他。
	FeedbackReason string    `gorm:"type:varchar(500);not null;default:''"` // FeedbackReason 为反馈原因。
	UserID         int64     `gorm:"type:bigint;not null;default:0;index"`  // UserID 为用户ID。
	AgentID        int64     `gorm:"type:bigint;not null;default:0;index"`  // AgentID 为坐席ID。
	Remark         string    `gorm:"type:text"`                             // Remark 为备注。
	CreatedAt      time.Time `gorm:"type:timestamp;not null;index"`
}

// SkillDefinition 表示可由后台配置并参与运行时路由的 Skill 定义。
type SkillDefinition struct {
	ID            int64        `gorm:"primaryKey;autoIncrement"`                    // ID 为 Skill 主键。
	TenantID      int64        `gorm:"type:bigint;not null;default:0;index"`        // TenantID 为企业租户；0 表示平台只读模板。
	Name          string       `gorm:"type:varchar(100);not null;default:'';index"` // Name 为 Skill 的展示名称，用于后台列表、配置页和人工选择场景。
	Description   string       `gorm:"type:varchar(255);not null;default:''"`       // Description 为 Skill 的简要说明，用于描述该 Skill 的适用场景和职责边界。
	Instruction   string       `gorm:"type:text"`                                   // Instruction 为 Skill 的主体说明文档存储字段，使用 Markdown 编写，供 Agent 理解任务目标、步骤和工具使用要求。
	Examples      string       `gorm:"type:text"`                                   // Examples 为示例问法 JSON 数组字符串。
	ToolWhitelist string       `gorm:"type:text"`                                   // ToolWhitelist 为允许使用的工具编码 JSON 数组字符串。
	Status        enums.Status `gorm:"type:int;not null;default:0;index"`           // Status 为 Skill 当前状态，使用全局通用状态：0启用 1禁用 2删除。
	Remark        string       `gorm:"type:text"`                                   // Remark 为后台备注，用于记录配置说明、维护信息或内部协作信息。
	AuditFields
}

// SkillRunLog 表示一次 Skill 运行过程的审计日志。
type SkillRunLog struct {
	ID                int64            `gorm:"primaryKey;autoIncrement"`              // ID 为 Skill 运行日志主键。
	TenantID          int64            `gorm:"type:bigint;not null;default:0;index"`  // TenantID 为运行所属租户。
	ConversationID    int64            `gorm:"type:bigint;not null;default:0;index"`  // ConversationID 为关联会话ID，无会话上下文时为0。
	WorkflowRunID     int64            `gorm:"type:bigint;not null;default:0;index"`  // WorkflowRunID 为触发本次 Skill 的会话流程运行ID。
	SourceMessageID   int64            `gorm:"type:bigint;not null;default:0;index"`  // SourceMessageID 为触发本次 Skill 的客户消息ID。
	AIAgentID         int64            `gorm:"type:bigint;not null;default:0;index"`  // AIAgentID 为本次运行所属的 AI Agent ID。
	AIConfigID        int64            `gorm:"type:bigint;not null;default:0;index"`  // AIConfigID 为本次运行实际使用的 AI 配置ID。
	SkillDefinitionID int64            `gorm:"type:bigint;not null;default:0;index"`  // SkillDefinitionID 为最终命中的 Skill 定义ID，未命中时为0。
	ManualSkillID     int64            `gorm:"type:bigint;not null;default:0;index"`  // ManualSkillID 为本次请求显式指定的 Skill 定义ID。
	UserMessage       string           `gorm:"type:text"`                             // UserMessage 为本次请求的用户输入内容。
	Matched           bool             `gorm:"not null;default:false;index"`          // Matched 表示本次请求是否命中了 Skill。
	MatchReason       string           `gorm:"type:varchar(500);not null;default:''"` // MatchReason 为命中或未命中的原因说明。
	FinalSelected     bool             `gorm:"not null;default:false;index"`          // FinalSelected 表示该日志记录的 Skill 是否为最终选中的执行 Skill。
	UsedModel         string           `gorm:"type:varchar(100);not null;default:''"` // UsedModel 为本次实际调用的模型名称。
	UsedProvider      enums.AIProvider `gorm:"type:varchar(50);not null;default:''"`  // UsedProvider 为本次实际调用的模型供应商。
	ErrorMessage      string           `gorm:"type:text"`                             // ErrorMessage 为运行过程中的错误信息。
	TraceData         string           `gorm:"type:text"`                             // TraceData 为 Skill 执行链路追踪数据JSON。
	CreatedAt         time.Time        `gorm:"type:timestamp;not null;index"`         // CreatedAt 为运行日志创建时间。
}

// ConversationInterrupt 表示会话级待恢复中断记录。
type ConversationInterrupt struct {
	ID                  int64      `gorm:"primaryKey;autoIncrement"`
	ConversationID      int64      `gorm:"type:bigint;not null;default:0;index"`
	AIAgentID           int64      `gorm:"type:bigint;not null;default:0;index"`
	SourceMessageID     int64      `gorm:"type:bigint;not null;default:0;index"`
	LastResumeMessageID int64      `gorm:"type:bigint;not null;default:0;index"`
	WorkflowRunID       int64      `gorm:"type:bigint;not null;default:0;index"`
	WorkflowNodeID      string     `gorm:"type:varchar(100);not null;default:'';index"`
	CheckPointID        string     `gorm:"type:varchar(128);not null;default:'';uniqueIndex"`
	InterruptID         string     `gorm:"type:varchar(255);not null;default:'';index"`
	InterruptType       string     `gorm:"type:varchar(50);not null;default:'';index"`
	Status              string     `gorm:"type:varchar(30);not null;default:'';index"`
	PromptText          string     `gorm:"type:text"`
	RequestData         string     `gorm:"type:text"`
	CheckPointData      string     `gorm:"type:text"`
	ResumeCount         int        `gorm:"type:int;not null;default:0"`
	ExpiresAt           *time.Time `gorm:"type:timestamp;index"`
	CreatedAt           time.Time  `gorm:"type:timestamp;not null;index"`
	UpdatedAt           time.Time  `gorm:"type:timestamp;not null;index"`
}

// DSARRequest 数据主体访问请求（GDPR Art 15/17/20）
type DSARRequest struct {
	ID              string     `gorm:"primaryKey;type:varchar(36)"`
	TenantID        string     `gorm:"type:varchar(36);not null;index"`
	SubjectType     string     `gorm:"type:varchar(32);not null;default:''"`
	SubjectID       string     `gorm:"type:varchar(64);not null;default:'';index"`
	SubjectEmail    string     `gorm:"type:varchar(255);not null;default:''"`
	RequestType     string     `gorm:"type:varchar(32);not null;default:''"`
	Status          string     `gorm:"type:varchar(20);not null;default:'pending';index"`
	DeadlineAt      time.Time  `gorm:"type:timestamp;not null;index"`
	RequestedAt     time.Time  `gorm:"type:timestamp;not null"`
	VerifiedAt      *time.Time `gorm:"type:timestamp"`
	ProcessingNotes string     `gorm:"type:text"`
	CompletedAt     *time.Time `gorm:"type:timestamp"`
	BaseModel
}

// DSARExecutionLog DSAR 执行日志
type DSARExecutionLog struct {
	ID             string    `gorm:"primaryKey;type:varchar(36)"`
	IdempotencyKey *string   `gorm:"type:varchar(180);uniqueIndex"`
	DSARID         string    `gorm:"type:varchar(36);not null;index"`
	Action         string    `gorm:"type:varchar(32);not null;default:''"`
	TargetTable    string    `gorm:"type:varchar(128);not null;default:''"`
	TargetID       string    `gorm:"type:varchar(64);not null;default:''"`
	Details        string    `gorm:"type:text"`
	ExecutedAt     time.Time `gorm:"type:timestamp;not null"`
	BaseModel
}

// DataBreachRecord 定义在 remote_helpdesk.go

// DataRegionPolicy 数据区域保留策略。
type DataRegionPolicy struct {
	ID                string    `gorm:"primaryKey;type:varchar(36)"`
	TenantID          string    `gorm:"type:varchar(36);not null;index"`
	DataRegion        string    `gorm:"type:varchar(64);not null;default:'';index"`
	RetentionDays     int       `gorm:"type:int;not null;default:365"`
	ArchiveAfterDays  int       `gorm:"type:int;not null;default:90"`
	AutoDeleteEnabled bool      `gorm:"not null;default:false"`
	GDPRRegion        bool      `gorm:"not null;default:false"`
	CCPARegion        bool      `gorm:"not null;default:false"`
	Status            string    `gorm:"type:varchar(20);not null;default:'active'"`
	CreatedAt         time.Time `gorm:"type:timestamp;not null"`
	UpdatedAt         time.Time `gorm:"type:timestamp;not null"`
}
