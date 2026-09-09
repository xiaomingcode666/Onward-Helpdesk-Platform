package models

import "time"

const (
	AIReplyJobStatusPending      = "pending"
	AIReplyJobStatusRunning      = "running"
	AIReplyJobStatusWaitingRetry = "waiting_retry"
	AIReplyJobStatusSucceeded    = "succeeded"
	AIReplyJobStatusFailed       = "failed"
	AIReplyJobStatusCancelled    = "cancelled"
)

// AIReplyJob durably connects a committed customer message to its AI reply.
// MessageID is unique so HTTP retries and worker recovery cannot execute the
// same customer turn as separate jobs.
type AIReplyJob struct {
	ID             int64      `gorm:"primaryKey;autoIncrement"`
	TenantID       int64      `gorm:"type:bigint;not null;default:0;index"`
	ProductID      int64      `gorm:"type:bigint;not null;default:0;index"`
	ConversationID int64      `gorm:"type:bigint;not null;index"`
	MessageID      int64      `gorm:"type:bigint;not null;uniqueIndex"`
	AIAgentID      int64      `gorm:"type:bigint;not null;default:0;index"`
	RequestID      string     `gorm:"type:varchar(128);not null;default:'';index"`
	Status         string     `gorm:"type:varchar(32);not null;default:'pending';index"`
	RetryCount     int        `gorm:"type:int;not null;default:0"`
	MaxRetries     int        `gorm:"type:int;not null;default:5"`
	NextAttemptAt  *time.Time `gorm:"type:timestamp;index"`
	LastAttemptAt  *time.Time `gorm:"type:timestamp"`
	LockedAt       *time.Time `gorm:"type:timestamp;index"`
	LockOwner      string     `gorm:"type:varchar(128);not null;default:'';index"`
	LastError      string     `gorm:"type:text"`
	StartedAt      *time.Time `gorm:"type:timestamp"`
	FinishedAt     *time.Time `gorm:"type:timestamp"`
	CreatedAt      time.Time  `gorm:"type:timestamp;not null;index"`
	UpdatedAt      time.Time  `gorm:"type:timestamp;not null;index"`
}
