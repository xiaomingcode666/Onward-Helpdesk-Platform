package models

import "time"

// PriorityCode on Ticket remains a legacy SLA key; PriorityLevel is the P1-P4 business value.
type TicketGovernance struct {
	MergedIntoID            int64      `gorm:"not null;default:0;index" json:"merged_into_id"`
	MergedAt                *time.Time `json:"merged_at"`
	CreateParentTicketID    int64      `gorm:"-" json:"-"`
	CreateRelationReason    string     `gorm:"-" json:"-"`
	CaseType                string     `gorm:"type:varchar(32);not null;default:'';index"`
	PriorityLevel           string     `gorm:"type:varchar(8);not null;default:'';index"`
	PrioritySuggested       string     `gorm:"type:varchar(8);not null;default:''"`
	PriorityReviewRequired  bool       `gorm:"not null;default:false"`
	PriorityOverridden      bool       `gorm:"not null;default:false"`
	PriorityFactsJSON       string     `gorm:"type:text;not null;default:'{}'"`
	PriorityPolicyJSON      string     `gorm:"type:text;not null;default:''"`
	PriorityExplanation     string     `gorm:"type:text;not null;default:''"`
	PriorityConfigVersionID int64      `gorm:"not null;default:0"`
	GovernanceRevision      int64      `gorm:"not null;default:0"`
}
type TicketGovernanceOperation struct {
	ID           int64  `gorm:"primaryKey;autoIncrement"`
	TenantID     int64  `gorm:"not null;uniqueIndex:uk_ticket_governance_operation,priority:1"`
	TicketID     int64  `gorm:"not null;uniqueIndex:uk_ticket_governance_operation,priority:2"`
	OperationKey string `gorm:"type:varchar(100);not null;uniqueIndex:uk_ticket_governance_operation,priority:3"`
	PayloadHash  string `gorm:"type:varchar(64);not null"`
	ResultJSON   string `gorm:"type:text;not null"`
	CreatedAt    time.Time
}
type TicketPriorityProposal struct {
	ID           int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID     int64      `gorm:"not null;index" json:"-"`
	TicketID     int64      `gorm:"not null;index" json:"ticket_id"`
	Revision     int64      `gorm:"not null" json:"revision"`
	Priority     string     `gorm:"type:varchar(8);not null" json:"priority"`
	Category     string     `gorm:"type:varchar(32);not null" json:"category"`
	Reason       string     `gorm:"type:text;not null" json:"reason"`
	ProposedBy   int64      `gorm:"not null" json:"proposed_by"`
	Status       string     `gorm:"type:varchar(20);not null" json:"status"`
	ReviewReason string     `gorm:"type:text;not null;default:''" json:"review_reason"`
	ReviewedBy   int64      `json:"reviewed_by"`
	CreatedAt    time.Time  `json:"created_at"`
	ReviewedAt   *time.Time `json:"reviewed_at"`
}
type TicketRelation struct {
	ID            int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID      int64      `gorm:"not null;index;uniqueIndex:uk_ticket_relation_active,priority:1;uniqueIndex:uk_ticket_relation_child,priority:1" json:"-"`
	SourceID      int64      `gorm:"not null;index" json:"source_id"`
	TargetID      int64      `gorm:"not null;index" json:"target_id"`
	Kind          string     `gorm:"type:varchar(20);not null;index" json:"kind"`
	ActiveKey     *string    `gorm:"type:varchar(100);uniqueIndex:uk_ticket_relation_active,priority:2" json:"-"`
	ChildKey      *int64     `gorm:"uniqueIndex:uk_ticket_relation_child,priority:2" json:"-"`
	Reason        string     `gorm:"type:text;not null" json:"reason"`
	CreatedBy     int64      `gorm:"not null" json:"created_by"`
	CreatedAt     time.Time  `json:"created_at"`
	RemovedAt     *time.Time `json:"removed_at"`
	RemovedBy     int64      `json:"removed_by"`
	RemovalReason string     `gorm:"type:text;not null;default:''" json:"removal_reason"`
}
