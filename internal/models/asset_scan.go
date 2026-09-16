package models

import (
	"remotehelpdesk/internal/pkg/enums"
	"time"
)

const (
	AssetScanUnscanned   = "unscanned"
	AssetScanPending     = "pending"
	AssetScanClean       = "clean"
	AssetScanQuarantined = "quarantined"
)

// Usable requires a recorded antivirus verdict; pre-migration assets are not trusted.
func (a *Asset) Usable() bool {
	return a != nil && a.Status == enums.AssetStatusSuccess && a.ScanStatus == AssetScanClean
}

// AssetScanAttempt is append-only. Storage locations and attachment contents are never returned.
type AssetScanAttempt struct {
	ID              int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID        int64     `gorm:"not null;index" json:"-"`
	AssetID         int64     `gorm:"not null;index" json:"assetId"`
	Status          string    `gorm:"type:varchar(24);not null" json:"status"`
	Reason          string    `gorm:"type:varchar(40);not null;default:''" json:"reason"`
	Detail          string    `gorm:"type:varchar(500);not null;default:''" json:"detail"`
	SHA256          string    `gorm:"type:varchar(64);not null;default:''" json:"sha256"`
	EngineVersion   string    `gorm:"type:varchar(100);not null;default:''" json:"engineVersion"`
	DatabaseVersion string    `gorm:"type:varchar(100);not null;default:''" json:"databaseVersion"`
	ScanPerformed   bool      `gorm:"not null;default:false" json:"scanPerformed"`
	OperatorID      int64     `gorm:"not null;default:0" json:"operatorId"`
	CreatedAt       time.Time `gorm:"not null;index" json:"createdAt"`
}
