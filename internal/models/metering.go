package models

import (
	"time"
)

// UsageEvent Sub2API 原始用量事件。
type UsageEvent struct {
	ID        int64   `gorm:"primaryKey;autoIncrement"`
	TenantID  int64   `gorm:"type:bigint;not null;index"`
	ProductID int64   `gorm:"type:bigint;not null;index"`
	APIKeyID  string  `gorm:"type:varchar(128);not null;default:'';index"`
	Model     string  `gorm:"type:varchar(100);not null;default:'';index"`
	Endpoint  string  `gorm:"type:varchar(64);not null;default:'';index"` // chat_completion / embedding
	TokensIn  int64   `gorm:"type:bigint;not null;default:0"`
	TokensOut int64   `gorm:"type:bigint;not null;default:0"`
	Cost      float64 `gorm:"type:decimal(12,4);not null;default:0"`
	AuditFields
}

// TableName 设置 UsageEvent 表名
func (UsageEvent) TableName() string {
	return "usage_events"
}

// QuotaLimit 配额限制定义。
type QuotaLimit struct {
	ID              int64   `gorm:"primaryKey;autoIncrement"`
	TenantID        int64   `gorm:"type:bigint;not null;index;uniqueIndex:uk_quota_limit"`
	ProductID       int64   `gorm:"type:bigint;not null;index;uniqueIndex:uk_quota_limit"`
	ResourceType    string  `gorm:"type:varchar(32);not null;default:'';index;uniqueIndex:uk_quota_limit"` // tokens / requests / cost
	LimitValue      int64   `gorm:"type:bigint;not null;default:0"`
	Period          string  `gorm:"type:varchar(16);not null;default:'daily';index"` // daily / weekly / monthly
	NotifyThreshold float64 `gorm:"type:decimal(5,2);not null;default:0.8"`          // 达到此百分比时触发告警
	Status          int     `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

// TableName 设置 QuotaLimit 表名
func (QuotaLimit) TableName() string {
	return "quota_limits"
}

// BudgetAlert 预算告警配置。
type BudgetAlert struct {
	ID              int64      `gorm:"primaryKey;autoIncrement"`
	TenantID        int64      `gorm:"type:bigint;not null;index"`
	ProductID       int64      `gorm:"type:bigint;not null;index"`
	Threshold       float64    `gorm:"type:decimal(12,2);not null;default:0"`
	NotifyEmail     string     `gorm:"type:varchar(200);not null;default:''"`
	NotifyWebhook   string     `gorm:"type:varchar(512);not null;default:''"`
	LastTriggeredAt *time.Time `gorm:"type:timestamp;index"`
	Status          int        `gorm:"type:int;not null;default:0;index"`
	AuditFields
}

// TableName 设置 BudgetAlert 表名
func (BudgetAlert) TableName() string {
	return "budget_alerts"
}

// SyncRun 同步运行记录。
type SyncRun struct {
	ID               int64      `gorm:"primaryKey;autoIncrement"`
	TenantID         int64      `gorm:"type:bigint;not null;index"`
	SyncType         string     `gorm:"type:varchar(32);not null;default:'';index"`        // usage_aggregation / quota_sync / product_sync
	Status           string     `gorm:"type:varchar(20);not null;default:'running';index"` // running / completed / failed
	StartedAt        time.Time  `gorm:"type:timestamp;not null;index"`
	EndedAt          *time.Time `gorm:"type:timestamp;index"`
	RecordsProcessed int64      `gorm:"type:bigint;not null;default:0"`
	ErrorMessage     string     `gorm:"type:text"`
	AuditFields
}

// TableName 设置 SyncRun 表名
func (SyncRun) TableName() string {
	return "sync_runs"
}

// 配额资源类型常量
const (
	QuotaResourceTokens   = "tokens"
	QuotaResourceRequests = "requests"
	QuotaResourceCost     = "cost"
)

// 配额周期常量
const (
	QuotaPeriodDaily   = "daily"
	QuotaPeriodWeekly  = "weekly"
	QuotaPeriodMonthly = "monthly"
)

// 同步状态常量
const (
	SyncStatusRunning   = "running"
	SyncStatusCompleted = "completed"
	SyncStatusFailed    = "failed"
)
