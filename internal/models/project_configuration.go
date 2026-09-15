package models

import "time"

// ProjectConfigurationVersion is an immutable draft or historical snapshot.
// Applying a version only changes the separate state/activation records.
type ProjectConfigurationVersion struct {
	ID            int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID      int64     `gorm:"not null;uniqueIndex:uk_project_config_request,priority:1;index" json:"tenant_id"`
	Environment   string    `gorm:"type:varchar(24);not null;uniqueIndex:uk_project_config_request,priority:2" json:"environment"`
	RequestKey    string    `gorm:"type:varchar(100);not null;uniqueIndex:uk_project_config_request,priority:3" json:"-"`
	BaseVersionID int64     `gorm:"not null;default:0" json:"base_version_id"`
	Digest        string    `gorm:"type:varchar(64);not null" json:"digest"`
	DocumentJSON  string    `gorm:"type:text;not null" json:"-"`
	Note          string    `gorm:"type:varchar(500);not null" json:"note"`
	CreatedBy     int64     `gorm:"not null" json:"created_by"`
	CreatedByName string    `gorm:"type:varchar(120);not null;default:''" json:"created_by_name"`
	CreatedAt     time.Time `gorm:"not null" json:"created_at"`
}

type ProjectConfigurationState struct {
	ID              int64  `gorm:"primaryKey;autoIncrement"`
	TenantID        int64  `gorm:"not null;uniqueIndex:uk_project_config_scope,priority:1"`
	Environment     string `gorm:"type:varchar(24);not null;uniqueIndex:uk_project_config_scope,priority:2"`
	ActiveVersionID int64  `gorm:"not null;default:0"`
}

type ProjectConfigurationActivation struct {
	ID                int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID          int64     `gorm:"not null;index" json:"tenant_id"`
	VersionID         int64     `gorm:"not null;uniqueIndex" json:"version_id"`
	PreviousVersionID int64     `gorm:"not null" json:"previous_version_id"`
	AppliedBy         int64     `gorm:"not null" json:"applied_by"`
	AppliedByName     string    `gorm:"type:varchar(120);not null;default:''" json:"applied_by_name"`
	AppliedAt         time.Time `gorm:"not null" json:"applied_at"`
}
