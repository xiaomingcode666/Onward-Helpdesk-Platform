package models

import "time"

// TicketServiceMetric stores the current and historical state of one service
// metric. Update cadence uses one row per customer-visible update interval.
type TicketServiceMetric struct {
	ID              int64      `gorm:"primaryKey;autoIncrement"`
	TenantID        int64      `gorm:"type:bigint;not null;index"`
	TicketID        int64      `gorm:"type:bigint;not null;index"`
	MetricType      string     `gorm:"type:varchar(32);not null;index"`
	Status          string     `gorm:"type:varchar(20);not null;default:'running';index"`
	StartedAt       time.Time  `gorm:"type:timestamp;not null;index"`
	TargetAt        *time.Time `gorm:"type:timestamp;index"`
	ActualAt        *time.Time `gorm:"type:timestamp;index"`
	BreachedAt      *time.Time `gorm:"type:timestamp;index"`
	Escalated       bool       `gorm:"not null;default:false;index"`
	ConfigVersionID int64      `gorm:"not null;default:0"`
	CreatedAt       time.Time  `gorm:"type:timestamp;not null;index"`
	UpdatedAt       time.Time  `gorm:"type:timestamp;not null;index"`
}

func (TicketServiceMetric) TableName() string {
	return "t_ticket_service_metric"
}
