package models

// ChannelSecret stores a credential entered for a channel in the dashboard,
// encrypted with secretstore. Only ChannelSecretService reads this table;
// API responses expose configured/fingerprint status, never the value.
type ChannelSecret struct {
	ID          int64  `gorm:"primaryKey;autoIncrement"`
	ChannelID   int64  `gorm:"type:bigint;not null;uniqueIndex:uk_channel_secret_channel_name,priority:1"`
	Name        string `gorm:"type:varchar(64);not null;uniqueIndex:uk_channel_secret_channel_name,priority:2"`
	Ciphertext  string `gorm:"type:text;not null"`
	Fingerprint string `gorm:"type:varchar(64);not null;default:''"`
	AuditFields
}
