package models

import "time"

// MobilePushToken stores an authenticated user's APNs/FCM registration.
// The provider token is encrypted at rest and is never returned by APIs.
type MobilePushToken struct {
	ID               int64      `gorm:"primaryKey;autoIncrement"`
	TenantID         int64      `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_mobile_push_token,priority:1"`
	UserID           int64      `gorm:"type:bigint;not null;default:0;index;uniqueIndex:uk_mobile_push_token,priority:2"`
	Platform         string     `gorm:"type:varchar(16);not null;default:'';uniqueIndex:uk_mobile_push_token,priority:3"`
	TokenFingerprint string     `gorm:"type:char(64);not null;default:'';uniqueIndex:uk_mobile_push_token,priority:4"`
	TokenCiphertext  string     `gorm:"type:text;not null;default:''"`
	DeviceID         string     `gorm:"type:varchar(128);not null;default:'';index"`
	AppVersion       string     `gorm:"type:varchar(32);not null;default:''"`
	LastSeenAt       time.Time  `gorm:"type:timestamp;not null;index"`
	RevokedAt        *time.Time `gorm:"type:timestamp;index"`
	CreatedAt        time.Time  `gorm:"type:timestamp;not null;index"`
	UpdatedAt        time.Time  `gorm:"type:timestamp;not null;index"`
}

func (MobilePushToken) TableName() string {
	return "mobile_push_tokens"
}
