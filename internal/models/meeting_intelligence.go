package models

import "time"

// MeetingTranscriptSegment stores finalized provider-neutral speech-to-text output.
type MeetingTranscriptSegment struct {
	ID                     string     `gorm:"primaryKey;type:varchar(36)"`
	TenantID               int64      `gorm:"type:bigint;not null;index;index:idx_meeting_transcript_timeline,priority:1"`
	MeetingID              string     `gorm:"type:varchar(36);not null;index;index:idx_meeting_transcript_event_lookup;index:idx_meeting_transcript_timeline,priority:2"`
	ParticipantID          string     `gorm:"type:varchar(64);not null;default:'';index"`
	SpeakerName            string     `gorm:"type:varchar(100);not null;default:''"`
	Provider               string     `gorm:"type:varchar(32);not null;index;index:idx_meeting_transcript_event_lookup"`
	ProviderEventID        string     `gorm:"type:varchar(128);not null;index:idx_meeting_transcript_event_lookup"`
	IngestSource           string     `gorm:"type:varchar(24);not null;default:'provider';index"`
	Language               string     `gorm:"type:varchar(16);not null;default:'zh-CN';index"`
	Text                   string     `gorm:"type:text;not null"`
	IsFinal                bool       `gorm:"not null;default:false;index"`
	StartedAtMS            int64      `gorm:"type:bigint;not null;default:0;index:idx_meeting_transcript_timeline,priority:3"`
	EndedAtMS              int64      `gorm:"type:bigint;not null;default:0"`
	Confidence             float64    `gorm:"type:double precision;not null;default:0"`
	RawJSON                string     `gorm:"column:raw_json;type:text;not null;default:'{}'"`
	TranslatedLanguage     string     `gorm:"type:varchar(16);not null;default:'';index"`
	TranslatedText         string     `gorm:"type:text;not null;default:''"`
	TranslationProvider    string     `gorm:"type:varchar(32);not null;default:'';index"`
	TranslationStatus      string     `gorm:"type:varchar(24);not null;default:'not_configured';index"`
	TranslationError       string     `gorm:"type:varchar(500);not null;default:''"`
	TranslationUpdatedAt   *time.Time `gorm:"type:timestamp"`
	TranslationRetryCount  int        `gorm:"not null;default:0"`
	TranslationNextRetryAt *time.Time `gorm:"type:timestamp;index"`
	TranslationLeaseUntil  *time.Time `gorm:"type:timestamp;index"`
	BaseModel
}

func (MeetingTranscriptSegment) TableName() string { return "meeting_transcript_segments" }

// MeetingARAnnotation stores normalized image coordinates so annotations can
// be rendered consistently on freeze frames with different display sizes.
type MeetingARAnnotation struct {
	ID                  string    `gorm:"primaryKey;type:varchar(36)"`
	TenantID            int64     `gorm:"type:bigint;not null;index"`
	MeetingID           string    `gorm:"type:varchar(36);not null;index"`
	TicketID            string    `gorm:"type:varchar(36);not null;index"`
	FrameAssetID        int64     `gorm:"type:bigint;not null;default:0;index"`
	DetectionProvider   string    `gorm:"type:varchar(32);not null;default:'manual';index"`
	ExternalDetectionID string    `gorm:"type:varchar(128);not null;default:'';index"`
	Label               string    `gorm:"type:varchar(128);not null"`
	PartCode            string    `gorm:"type:varchar(128);not null;default:'';index"`
	Confidence          float64   `gorm:"type:double precision;not null;default:0"`
	X                   float64   `gorm:"type:double precision;not null"`
	Y                   float64   `gorm:"type:double precision;not null"`
	Width               float64   `gorm:"type:double precision;not null"`
	Height              float64   `gorm:"type:double precision;not null"`
	Color               string    `gorm:"type:varchar(20);not null;default:'#ef4444'"`
	Note                string    `gorm:"type:text"`
	CreatedBy           int64     `gorm:"type:bigint;not null;default:0;index"`
	MetadataJSON        string    `gorm:"column:metadata_json;type:text;not null;default:'{}'"`
	CreatedAt           time.Time `gorm:"type:timestamp;not null;index"`
	UpdatedAt           time.Time `gorm:"type:timestamp;not null;index"`
}

func (MeetingARAnnotation) TableName() string { return "meeting_ar_annotations" }
