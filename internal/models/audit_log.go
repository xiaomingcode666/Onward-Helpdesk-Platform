package models

import (
	"time"
)

// AuditLog 审计日志记录。
type AuditLog struct {
	ID             int64     `gorm:"primaryKey;autoIncrement"`
	TenantID       int64     `gorm:"type:bigint;not null;default:0;index"`
	ActorID        string    `gorm:"type:varchar(64);not null;default:'';index"`
	ActorType      string    `gorm:"type:varchar(32);not null;default:'';index"` // user / system / api_key
	Domain         string    `gorm:"type:varchar(64);not null;default:'';index"` // ticket / meeting / diagnosis / knowledge / settings
	ResourceType   string    `gorm:"type:varchar(64);not null;default:'';index"` // ticket / meeting / diagnosis_session / product / ...
	ResourceID     string    `gorm:"type:varchar(128);not null;default:'';index"`
	Action         string    `gorm:"type:varchar(64);not null;default:'';index"` // created / updated / deleted / closed / assigned / ...
	BeforeState    string    `gorm:"type:text"`                                  // JSON - 操作前快照
	AfterState     string    `gorm:"type:text"`                                  // JSON - 操作后快照
	IPAddress      string    `gorm:"type:varchar(64);not null;default:''"`
	UserAgent      string    `gorm:"type:varchar(255);not null;default:''"`
	RequestID      string    `gorm:"type:varchar(128);not null;default:'';index"`
	SupportGrantID int64     `gorm:"type:bigint;not null;default:0;index"`
	RiskLevel      string    `gorm:"type:varchar(16);not null;default:'low';index"`     // low / medium / high / critical
	Status         string    `gorm:"type:varchar(20);not null;default:'success';index"` // success / failure / blocked
	CreatedAt      time.Time `gorm:"type:timestamp;not null;index"`
}

// TableName 设置 AuditLog 表名
func (AuditLog) TableName() string {
	return "audit_logs"
}

// 审计事件类型常量
const (
	AuditActionTenantCreated       = "tenant.created"
	AuditActionPermissionChanged   = "permission.changed"
	AuditActionSecretRotated       = "secret.rotated"
	AuditActionDataExported        = "data.exported"
	AuditActionFileDownloaded      = "file.downloaded"
	AuditActionKnowledgePublished  = "knowledge.published"
	AuditActionTicketTransitioned  = "ticket.transitioned"
	AuditActionMeetingJoined       = "meeting.joined"
	AuditActionAccessQueryExecuted = "access.query.executed"
	AuditActionTicketClosed        = "ticket.closed"
	AuditActionTicketAssigned      = "ticket.assigned"
)

// 风险等级常量
const (
	RiskLevelLow      = "low"
	RiskLevelMedium   = "medium"
	RiskLevelHigh     = "high"
	RiskLevelCritical = "critical"
)

// 审计状态常量
const (
	AuditStatusSuccess = "success"
	AuditStatusFailure = "failure"
	AuditStatusBlocked = "blocked"
)
