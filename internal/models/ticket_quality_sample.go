package models

import "time"

// TicketQualitySample is a deterministic QA sampling decision for one ticket
// and one sampling period. It is separate from ticket lifecycle status.
type TicketQualitySample struct {
	ID          int64      `gorm:"primaryKey;autoIncrement"`
	TenantID    int64      `gorm:"not null;index;uniqueIndex:uk_ticket_quality_sample_period,priority:1"`
	TicketID    int64      `gorm:"not null;index;uniqueIndex:uk_ticket_quality_sample_period,priority:2"`
	PeriodKey   string     `gorm:"type:varchar(32);not null;uniqueIndex:uk_ticket_quality_sample_period,priority:3"`
	ReasonsJSON string     `gorm:"column:reasons_json;type:text;not null;default:'[]'"`
	Status      string     `gorm:"type:varchar(20);not null;default:'pending';index"`
	ReviewerID  int64      `gorm:"not null;default:0;index"`
	ReviewNote  string     `gorm:"type:text"`
	SampledAt   time.Time  `gorm:"not null;index"`
	StartedAt   *time.Time `gorm:"index"`
	CompletedAt *time.Time `gorm:"index"`
	AuditFields
}
