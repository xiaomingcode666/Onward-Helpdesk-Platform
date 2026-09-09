package models

import (
	"time"
)

// DomainEvent 领域事件记录，用于 Outbox 模式的基础模型。
type DomainEvent struct {
	ID             int64      `gorm:"primaryKey;autoIncrement"`
	TenantID       int64      `gorm:"type:bigint;not null;default:0;index"`
	TraceID        string     `gorm:"type:varchar(128);not null;default:'';index"`
	IdempotencyKey string     `gorm:"type:varchar(128);not null;default:'';uniqueIndex"`
	SchemaVersion  int        `gorm:"type:int;not null;default:1"`
	EventType      string     `gorm:"type:varchar(64);not null;index"`
	Payload        string     `gorm:"type:text;not null"`
	Source         string     `gorm:"type:varchar(64);not null;default:''"`
	AggregateID    string     `gorm:"type:varchar(128);not null;default:'';index"`
	ActorID        string     `gorm:"type:varchar(64);not null;default:''"`
	ActorType      string     `gorm:"type:varchar(32);not null;default:''"`
	CreatedAt      time.Time  `gorm:"type:timestamp;not null;index"`
	PublishedAt    *time.Time `gorm:"type:timestamp;index"`
}

// TableName 设置 DomainEvent 表名
func (DomainEvent) TableName() string {
	return "domain_events"
}

// OutboxRecord Outbox 投递记录表。
// 事件先写入 domain_events 表，OutboxRecord 跟踪投递状态。
type OutboxRecord struct {
	ID          int64      `gorm:"primaryKey;autoIncrement"`
	EventID     int64      `gorm:"type:bigint;not null;index;uniqueIndex"`
	EventType   string     `gorm:"type:varchar(64);not null;index"`
	Status      string     `gorm:"type:varchar(20);not null;default:'pending';index"` // pending / publishing / published / failed / dead
	RetryCount  int        `gorm:"type:int;not null;default:0"`
	MaxRetries  int        `gorm:"type:int;not null;default:3"`
	LastError   string     `gorm:"type:text"`
	NextRetryAt *time.Time `gorm:"type:timestamp;index"`
	LockedUntil *time.Time `gorm:"type:timestamp;index"`
	CreatedAt   time.Time  `gorm:"type:timestamp;not null;index"`
	PublishedAt *time.Time `gorm:"type:timestamp;index"`
}

// TableName 设置 OutboxRecord 表名
func (OutboxRecord) TableName() string {
	return "outbox_records"
}

// OutboxStatus 常量定义
const (
	OutboxStatusPending    = "pending"
	OutboxStatusPublishing = "publishing"
	OutboxStatusPublished  = "published"
	OutboxStatusFailed     = "failed"
	OutboxStatusDead       = "dead"
)
