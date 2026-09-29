package models

import "time"

// ProjectRetentionApproval records approval for one project's immutable
// production retention policy snapshot.
type ProjectRetentionApproval struct {
	ID              int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID        int64      `gorm:"not null;index;uniqueIndex:uk_project_retention_approval,priority:1" json:"tenant_id"`
	Environment     string     `gorm:"type:varchar(24);not null;index;uniqueIndex:uk_project_retention_approval,priority:2" json:"environment"`
	ProjectKey      string     `gorm:"type:varchar(64);not null;index;uniqueIndex:uk_project_retention_approval,priority:3" json:"project_key"`
	VersionID       int64      `gorm:"not null;index;uniqueIndex:uk_project_retention_approval,priority:4" json:"version_id"`
	PolicyDigest    string     `gorm:"type:varchar(64);not null" json:"policy_digest"`
	Status          string     `gorm:"type:varchar(20);not null;index" json:"status"`
	SubmittedBy     int64      `gorm:"not null" json:"submitted_by"`
	SubmittedByName string     `gorm:"type:varchar(120);not null;default:''" json:"submitted_by_name"`
	SubmittedAt     time.Time  `gorm:"not null" json:"submitted_at"`
	ReviewedBy      int64      `gorm:"not null;default:0" json:"reviewed_by"`
	ReviewedByName  string     `gorm:"type:varchar(120);not null;default:''" json:"reviewed_by_name"`
	ReviewedAt      *time.Time `gorm:"type:timestamp" json:"reviewed_at,omitempty"`
	ReviewComment   string     `gorm:"type:varchar(1000);not null;default:''" json:"review_comment"`
	CreatedAt       time.Time  `gorm:"not null" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"not null" json:"updated_at"`
}

func (ProjectRetentionApproval) TableName() string { return "project_retention_approvals" }
