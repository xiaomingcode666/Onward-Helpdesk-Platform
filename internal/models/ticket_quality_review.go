package models

import (
	"time"
)

type TicketQualityScorecardVersion struct {
	ID          int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID    int64      `gorm:"not null;index" json:"tenant_id"`
	Version     string     `gorm:"type:varchar(32);not null;index" json:"version"`
	Name        string     `gorm:"type:varchar(120);not null;default:''" json:"name"`
	ItemsJSON   string     `gorm:"type:text;not null" json:"items_json"`
	Status      string     `gorm:"type:varchar(20);not null;default:'draft';index" json:"status"`
	PublishedAt *time.Time `gorm:"index" json:"published_at,omitempty"`
	CreatedBy   int64      `gorm:"not null;default:0" json:"created_by"`
	CreatedAt   time.Time  `gorm:"not null;index" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"not null" json:"updated_at"`
}

type TicketQualityReview struct {
	ID                 int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID           int64     `gorm:"not null;index" json:"tenant_id"`
	TicketID           int64     `gorm:"not null;index" json:"ticket_id"`
	ScorecardVersionID int64     `gorm:"not null;index" json:"scorecard_version_id"`
	ScoreSnapshotJSON  string    `gorm:"type:text;not null" json:"score_snapshot_json"`
	TotalScore         int       `gorm:"not null;default:0" json:"total_score"`
	MaxScore           int       `gorm:"not null;default:18" json:"max_score"`
	Result             string    `gorm:"type:varchar(24);not null;index" json:"result"`
	Outcome            string    `gorm:"type:varchar(24);not null;default:'';index" json:"outcome"`
	DefectCodesJSON    string    `gorm:"column:defect_codes_json;type:text;not null;default:'[]'" json:"-"`
	DefectCodes        []string  `gorm:"-" json:"defect_codes"`
	Evidence           string    `gorm:"type:text;not null;default:''" json:"evidence"`
	DisputeStatus      string    `gorm:"type:varchar(16);not null;default:'none';index" json:"dispute_status"`
	DisputeNote        string    `gorm:"type:text;not null;default:''" json:"dispute_note"`
	CoachingAction     string    `gorm:"type:text;not null;default:''" json:"coaching_action"`
	Remark             string    `gorm:"type:text" json:"remark"`
	ReviewerID         int64     `gorm:"not null;index" json:"reviewer_id"`
	ReviewedAt         time.Time `gorm:"not null;index" json:"reviewed_at"`
}
