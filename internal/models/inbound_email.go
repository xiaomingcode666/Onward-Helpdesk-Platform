package models

import "time"

// InboundEmail is the immutable, tenant-scoped receipt record for an email.
// It is deliberately separate from Ticket so one ticket can contain many emails.
type InboundEmail struct {
	ID              int64      `gorm:"primaryKey;autoIncrement"`
	TenantID        int64      `gorm:"type:bigint;not null;index;uniqueIndex:uk_inbound_email_message,priority:1;uniqueIndex:uk_inbound_email_source,priority:1"`
	Mailbox         string     `gorm:"type:varchar(255);not null;index;uniqueIndex:uk_inbound_email_message,priority:2"`
	MessageID       string     `gorm:"type:varchar(512);not null;uniqueIndex:uk_inbound_email_message,priority:3"`
	InReplyTo       string     `gorm:"type:varchar(512);not null;default:'';index"`
	ReferencesJSON  string     `gorm:"type:text;not null;default:'[]'"`
	FromAddress     string     `gorm:"type:varchar(255);not null;index"`
	Subject         string     `gorm:"type:varchar(512);not null;default:''"`
	TextBody        string     `gorm:"type:text"`
	PayloadHash     string     `gorm:"type:varchar(64);not null;default:''"`
	AttachmentCount int        `gorm:"not null;default:0"`
	SourceRecordID  string     `gorm:"type:varchar(600);not null;uniqueIndex:uk_inbound_email_source,priority:2"`
	TicketID        int64      `gorm:"type:bigint;not null;default:0;index"`
	Status          string     `gorm:"type:varchar(32);not null;default:'received';index"`
	MatchedBy       string     `gorm:"type:varchar(32);not null;default:''"`
	FailureReason   string     `gorm:"type:text"`
	ReceivedAt      time.Time  `gorm:"type:timestamp;not null;index"`
	ProcessedAt     *time.Time `gorm:"type:timestamp"`
	CreatedAt       time.Time  `gorm:"type:timestamp;not null;index"`
	UpdatedAt       time.Time  `gorm:"type:timestamp;not null;index"`
}

func (InboundEmail) TableName() string { return "inbound_emails" }

// MailboxSyncState tracks UIDs independently of the user's read/unread flags.
type MailboxSyncState struct {
	ID            int64  `gorm:"primaryKey"`
	TenantID      int64  `gorm:"not null;uniqueIndex:uk_mail_sync,priority:1"`
	MailboxKey    string `gorm:"size:64;not null;uniqueIndex:uk_mail_sync,priority:2"`
	UIDValidity   uint32 `gorm:"not null;default:0"`
	LastUID       uint32 `gorm:"not null;default:0"`
	Initialized   bool   `gorm:"not null;default:false"`
	LeaseToken    string `gorm:"size:64;not null;default:''"`
	LeaseUntil    *time.Time
	LastSuccessAt *time.Time
	LastError     string `gorm:"type:text"`
}

// TicketEmailReply is a durable outbox. Uncertain SMTP outcomes never auto-resend.
type TicketEmailReply struct {
	ID          int64     `gorm:"primaryKey" json:"id"`
	TenantID    int64     `gorm:"not null;uniqueIndex:uk_ticket_email_request,priority:1" json:"-"`
	TicketID    int64     `gorm:"not null;index;uniqueIndex:uk_ticket_email_request,priority:2" json:"ticket_id"`
	RequestKey  string    `gorm:"size:80;not null;uniqueIndex:uk_ticket_email_request,priority:3" json:"-"`
	PayloadHash string    `gorm:"size:64;not null" json:"-"`
	MessageID   string    `gorm:"size:255;not null;uniqueIndex" json:"message_id"`
	InReplyTo   string    `gorm:"size:512;not null" json:"-"`
	Recipient   string    `gorm:"size:255;not null" json:"recipient"`
	Mailbox     string    `gorm:"size:255;not null" json:"-"`
	Subject     string    `gorm:"size:512;not null" json:"subject"`
	Body        string    `gorm:"type:text;not null" json:"body"`
	Status      string    `gorm:"size:24;not null;index" json:"status"`
	ErrorCode   string    `gorm:"size:80;not null;default:''" json:"error_code"`
	AuthorID    int64     `gorm:"not null" json:"author_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
