package models

import (
	"time"

	"remotehelpdesk/internal/pkg/enums"
)

const TenantIntegrationProviderOnePanel = "1panel"

type TenantExternalPortalMetadata struct {
	DisplayName string `json:"displayName"`
	EmbedMode   string `json:"embedMode"`
}

type TenantIntegrationConfig struct {
	ID                   int64        `gorm:"primaryKey;autoIncrement"`
	TenantID             int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_tenant_integrations_provider"`
	Provider             string       `gorm:"type:varchar(32);not null;uniqueIndex:uk_tenant_integrations_provider"`
	BaseURL              string       `gorm:"type:varchar(512);not null;default:''"`
	AppID                string       `gorm:"type:varchar(128);not null;default:''"`
	AppKey               string       `gorm:"type:varchar(256);not null;default:''"`
	AppSecretRef         string       `gorm:"column:app_secret_ref;type:varchar(255);not null;default:''"`
	AppSecretFingerprint string       `gorm:"column:app_secret_fingerprint;type:varchar(64);not null;default:''"`
	Enabled              bool         `gorm:"not null;default:false;index"`
	Status               enums.Status `gorm:"type:int;not null;default:0;index"`
	MetadataJSON         string       `gorm:"column:metadata_json;type:text;not null;default:'{}'"`
	AuditFields
}

type ProductAIUsageCredential struct {
	ID                int64        `gorm:"primaryKey;autoIncrement"`
	TenantID          int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_product_ai_usage_credentials_product"`
	ProductID         int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_product_ai_usage_credentials_product"`
	Sub2APIAccount    string       `gorm:"column:sub2api_account;type:varchar(128);not null;default:''"`
	Sub2APIKeyID      string       `gorm:"column:sub2api_key_id;type:varchar(128);not null;default:'';index"`
	KeyName           string       `gorm:"column:key_name;type:varchar(160);not null;default:''"`
	APIKeyRef         string       `gorm:"column:api_key_ref;type:text;not null;default:''"`
	APIKeyFingerprint string       `gorm:"column:api_key_fingerprint;type:varchar(64);not null;default:''"`
	QuotaLimit        float64      `gorm:"column:quota_limit;type:decimal(18,6);not null;default:0"`
	QuotaUsed         float64      `gorm:"column:quota_used;type:decimal(18,6);not null;default:0"`
	Currency          string       `gorm:"type:varchar(12);not null;default:'USD'"`
	QuotaPolicyJSON   string       `gorm:"column:quota_policy_json;type:text;not null;default:'{}'"`
	ProvisionStatus   string       `gorm:"column:provision_status;type:varchar(32);not null;default:'pending';index"`
	ProvisionError    string       `gorm:"column:provision_error;type:text"`
	Status            enums.Status `gorm:"type:int;not null;default:0;index"`
	LastSyncedAt      *time.Time   `gorm:"column:last_synced_at;type:timestamp"`
	AuditFields
}

// ProductResourceProvisioningJob 记录产品创建后的外部资源开通任务。
//
// 产品主数据、Sub2API Key、知识库 Dataset、客服机器人跨越本地 DB 和外部服务，
// 不能放进一个数据库事务；该任务用于生产环境的幂等重试、故障恢复和审计追踪。
type ProductResourceProvisioningJob struct {
	ID                  int64      `gorm:"primaryKey;autoIncrement"`
	TenantID            int64      `gorm:"type:bigint;not null;index;uniqueIndex:uk_product_resource_provisioning_job"`
	ProductID           int64      `gorm:"type:bigint;not null;index;uniqueIndex:uk_product_resource_provisioning_job"`
	IdempotencyKey      string     `gorm:"type:varchar(180);not null;uniqueIndex"`
	Status              string     `gorm:"type:varchar(32);not null;default:'pending';index"` // pending/running/waiting_retry/succeeded/failed/cancelled
	RequestedQuotaLimit float64    `gorm:"type:decimal(18,6);not null;default:0"`
	KnowledgeStatus     string     `gorm:"type:varchar(32);not null;default:'pending'"`
	AIKeyStatus         string     `gorm:"column:ai_key_status;type:varchar(32);not null;default:'pending'"`
	AgentStatus         string     `gorm:"type:varchar(32);not null;default:'pending'"`
	ErrorSummary        string     `gorm:"type:text"`
	RetryCount          int        `gorm:"type:int;not null;default:0"`
	MaxRetries          int        `gorm:"type:int;not null;default:6"`
	NextAttemptAt       *time.Time `gorm:"type:timestamp;index"`
	LastAttemptAt       *time.Time `gorm:"type:timestamp"`
	LockedAt            *time.Time `gorm:"type:timestamp;index"`
	LockOwner           string     `gorm:"type:varchar(128);not null;default:'';index"`
	StartedAt           *time.Time `gorm:"type:timestamp"`
	FinishedAt          *time.Time `gorm:"type:timestamp"`
	AuditFields
}

type MeetingRoom struct {
	ID           int64        `gorm:"primaryKey;autoIncrement"`
	TenantID     int64        `gorm:"type:bigint;not null;index"`
	ProductID    int64        `gorm:"type:bigint;not null;index"`
	Provider     string       `gorm:"type:varchar(32);not null;default:'jitsi';index"`
	RoomName     string       `gorm:"type:varchar(160);not null;uniqueIndex"`
	JoinURL      string       `gorm:"type:varchar(1024);not null;default:''"`
	BusinessType string       `gorm:"type:varchar(32);not null;default:'';index"`
	BusinessID   int64        `gorm:"type:bigint;not null;default:0;index"`
	ExpiresAt    time.Time    `gorm:"type:timestamp;not null;index"`
	Status       enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

type ProductAIUsageDaily struct {
	ID             int64     `gorm:"primaryKey;autoIncrement"`
	TenantID       int64     `gorm:"type:bigint;not null;index;uniqueIndex:uk_product_ai_usage_daily"`
	ProductID      int64     `gorm:"type:bigint;not null;index;uniqueIndex:uk_product_ai_usage_daily"`
	UsageDate      string    `gorm:"type:varchar(10);not null;index;uniqueIndex:uk_product_ai_usage_daily"`
	RequestCount   int64     `gorm:"type:bigint;not null;default:0"`
	InputTokens    int64     `gorm:"type:bigint;not null;default:0"`
	OutputTokens   int64     `gorm:"type:bigint;not null;default:0"`
	CostMicros     int64     `gorm:"type:bigint;not null;default:0"`
	SourceProvider string    `gorm:"type:varchar(32);not null;default:'sub2api';index"`
	CreatedAt      time.Time `gorm:"type:timestamp;not null;index"`
	UpdatedAt      time.Time `gorm:"type:timestamp;not null;index"`
}

// ProductAIUsageEvent Sub2API 用量事件明细表
//
// 记录每次 AI 调用的用量明细，支持按 request_id 幂等去重。
type ProductAIUsageEvent struct {
	ID           int64     `gorm:"primaryKey;autoIncrement"`                          // ID 主键
	TenantID     int64     `gorm:"type:bigint;not null;index"`                        // TenantID 租户 ID
	ProductID    int64     `gorm:"type:bigint;not null;index"`                        // ProductID 产品 ID
	APIKeyID     string    `gorm:"type:varchar(128);not null;default:'';index"`       // APIKeyID 使用的 API Key 标识
	UsageType    string    `gorm:"type:varchar(32);not null;default:'';index"`        // UsageType 用量类型：chat_completion / embedding
	InputTokens  int64     `gorm:"type:bigint;not null;default:0"`                    // InputTokens 输入 token 数
	OutputTokens int64     `gorm:"type:bigint;not null;default:0"`                    // OutputTokens 输出 token 数
	CostAmount   float64   `gorm:"type:decimal(12,4);not null;default:0"`             // CostAmount 本次调用费用
	RequestID    string    `gorm:"type:varchar(128);not null;default:'';uniqueIndex"` // RequestID 请求 ID，用于幂等去重
	OccurredAt   time.Time `gorm:"type:timestamp;not null;index"`                     // OccurredAt 用量发生时间
	CreatedAt    time.Time `gorm:"type:timestamp;not null;index"`                     // CreatedAt 记录创建时间
}
