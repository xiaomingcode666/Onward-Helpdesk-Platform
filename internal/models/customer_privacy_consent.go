package models

import "time"

// CustomerPrivacyConsent is an append-only receipt for a visitor's privacy choices.
type CustomerPrivacyConsent struct {
	ID                int64     `gorm:"primaryKey;autoIncrement"`
	TenantID          int64     `gorm:"type:bigint;not null;index"`
	ProductID         int64     `gorm:"type:bigint;not null;default:0;index"`
	EntrySessionID    int64     `gorm:"type:bigint;not null;index"`
	VisitorID         string    `gorm:"type:varchar(128);not null;default:'';index"`
	PolicyVersion     string    `gorm:"type:varchar(32);not null;index"`
	RequiredAccepted  bool      `gorm:"not null;default:false;index"`
	AnalyticsAccepted bool      `gorm:"not null;default:false"`
	MarketingAccepted bool      `gorm:"not null;default:false"`
	Locale            string    `gorm:"type:varchar(16);not null;default:''"`
	IPAddress         string    `gorm:"type:varchar(64);not null;default:''"`
	UserAgent         string    `gorm:"type:varchar(255);not null;default:''"`
	RequestID         string    `gorm:"type:varchar(128);not null;default:'';index"`
	ReceiptHash       string    `gorm:"type:varchar(64);not null;uniqueIndex"`
	ConsentedAt       time.Time `gorm:"type:timestamp;not null;index"`
	CreatedAt         time.Time `gorm:"type:timestamp;not null;index"`
}

func (CustomerPrivacyConsent) TableName() string { return "customer_privacy_consents" }
