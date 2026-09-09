package models

import "time"

// WebhookEventInbox records inbound provider callbacks before business
// processing so retries are idempotent across processes and deployments.
type WebhookEventInbox struct {
	ID           int64      `gorm:"primaryKey;autoIncrement"`
	Provider     string     `gorm:"type:varchar(32);not null;uniqueIndex:uk_webhook_event_inbox"`
	EventID      string     `gorm:"type:varchar(160);not null;uniqueIndex:uk_webhook_event_inbox"`
	EventType    string     `gorm:"type:varchar(64);not null;default:'';index"`
	PayloadHash  string     `gorm:"type:varchar(64);not null;default:''"`
	PayloadJSON  string     `gorm:"type:text;not null;default:''"`
	Status       string     `gorm:"type:varchar(20);not null;default:'processing';index"`
	AttemptCount int        `gorm:"type:int;not null;default:1"`
	LockedUntil  *time.Time `gorm:"type:timestamp;index"`
	LastError    string     `gorm:"type:text"`
	OccurredAt   *time.Time `gorm:"type:timestamp;index"`
	ReceivedAt   time.Time  `gorm:"type:timestamp;not null;index"`
	ProcessedAt  *time.Time `gorm:"type:timestamp;index"`
	CreatedAt    time.Time  `gorm:"type:timestamp;not null;index"`
	UpdatedAt    time.Time  `gorm:"type:timestamp;not null;index"`
}
