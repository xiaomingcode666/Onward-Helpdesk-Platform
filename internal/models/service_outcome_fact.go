package models

import "time"

const ServiceOutcomeMetricVersionV1 = "service-outcome-v1"

// TicketServiceOutcomeFact is the versioned, rebuildable purchasing-metric fact
// for a ticket. Nullable/known fields keep missing evidence distinct from a
// negative business outcome.
type TicketServiceOutcomeFact struct {
	ID             int64  `gorm:"primaryKey;autoIncrement"`
	TenantID       int64  `gorm:"type:bigint;not null;index;uniqueIndex:uk_ticket_service_outcome_fact,priority:1"`
	TicketID       int64  `gorm:"type:bigint;not null;index;uniqueIndex:uk_ticket_service_outcome_fact,priority:2"`
	MetricVersion  string `gorm:"type:varchar(32);not null;index;uniqueIndex:uk_ticket_service_outcome_fact,priority:3"`
	ProductID      int64  `gorm:"type:bigint;not null;default:0;index"`
	ProductModelID int64  `gorm:"type:bigint;not null;default:0;index"`
	DeviceID       int64  `gorm:"type:bigint;not null;default:0;index"`
	ConversationID int64  `gorm:"type:bigint;not null;default:0;index"`

	TicketStatus     string     `gorm:"type:varchar(50);not null;default:'';index"`
	TicketCreatedAt  time.Time  `gorm:"type:timestamp;not null;index"`
	TicketResolvedAt *time.Time `gorm:"type:timestamp;index"`

	AIEngaged               bool `gorm:"not null;default:false;index"`
	AISelfServiceResolved   bool `gorm:"not null;default:false;index"`
	DiagnosisSessionCount   int  `gorm:"type:int;not null;default:0"`
	HumanEscalated          bool `gorm:"not null;default:false;index"`
	ExpertInterventionKnown bool `gorm:"not null;default:false;index"`
	ExpertIntervened        bool `gorm:"not null;default:false;index"`

	SupplierInvolved           bool  `gorm:"not null;default:false;index"`
	SupplierCollaborationCount int64 `gorm:"type:bigint;not null;default:0"`
	VideoUsed                  bool  `gorm:"not null;default:false;index"`
	MeetingCount               int64 `gorm:"type:bigint;not null;default:0"`

	ResolutionKnown         bool `gorm:"not null;default:false;index"`
	RemoteResolved          bool `gorm:"not null;default:false;index"`
	OnsiteVisitKnown        bool `gorm:"not null;default:false;index"`
	OnsiteVisitOccurred     bool `gorm:"not null;default:false;index"`
	AvoidedOnsiteVisitKnown bool `gorm:"not null;default:false;index"`
	AvoidedOnsiteVisit      bool `gorm:"not null;default:false;index"`
	FirstTimeFixKnown       bool `gorm:"not null;default:false;index"`
	FirstTimeFixEligible    bool `gorm:"not null;default:false;index"`
	FirstTimeFix            bool `gorm:"not null;default:false;index"`
	Reopened                bool `gorm:"not null;default:false;index"`
	ReopenCount             int  `gorm:"type:int;not null;default:0"`
	RepeatTicketCount30Days int  `gorm:"type:int;not null;default:0"`

	DowntimeKnown   bool  `gorm:"not null;default:false;index"`
	DowntimeMinutes int64 `gorm:"type:bigint;not null;default:0"`

	KnowledgeReuseKnown       bool  `gorm:"not null;default:false;index"`
	KnowledgeReused           bool  `gorm:"not null;default:false;index"`
	KnowledgeRetrieveCount    int64 `gorm:"type:bigint;not null;default:0"`
	KnowledgeUsedCount        int64 `gorm:"type:bigint;not null;default:0"`
	KnowledgeCitationCount    int64 `gorm:"type:bigint;not null;default:0"`
	KnowledgeCandidateID      int64 `gorm:"type:bigint;not null;default:0;index"`
	PublishedKnowledgeEntryID int64 `gorm:"type:bigint;not null;default:0;index"`

	ResponseDurationSeconds   *int64  `gorm:"type:bigint"`
	AcceptanceDurationSeconds *int64  `gorm:"type:bigint"`
	ResolutionDurationSeconds *int64  `gorm:"type:bigint"`
	WorkHours                 float64 `gorm:"type:decimal(12,2);not null;default:0"`
	RatingKnown               bool    `gorm:"not null;default:false;index"`
	CustomerRating            int     `gorm:"type:int;not null;default:0"`

	SourceFingerprint  string    `gorm:"type:varchar(64);not null;default:'';index"`
	SourceMaxUpdatedAt time.Time `gorm:"type:timestamp;not null;index"`
	BuiltAt            time.Time `gorm:"type:timestamp;not null;index"`
	EvidenceJSON       string    `gorm:"column:evidence_json;type:text;not null;default:'{}'"`
	AuditFields
}
