package models

import (
	"time"

	"remotehelpdesk/internal/pkg/enums"
)

// KnowledgeAccessGrant records which user or team can access a knowledge base.
type KnowledgeAccessGrant struct {
	ID              int64        `gorm:"primaryKey;autoIncrement"`
	TenantID        int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_knowledge_access_grant,priority:1"`
	KnowledgeBaseID int64        `gorm:"type:bigint;not null;index;uniqueIndex:uk_knowledge_access_grant,priority:2"`
	SubjectType     string       `gorm:"type:varchar(16);not null;uniqueIndex:uk_knowledge_access_grant,priority:3"`
	SubjectID       int64        `gorm:"type:bigint;not null;uniqueIndex:uk_knowledge_access_grant,priority:4"`
	AccessLevel     string       `gorm:"type:varchar(16);not null;default:'operate'"`
	EffectiveFrom   *time.Time   `gorm:"type:timestamp"`
	EffectiveUntil  *time.Time   `gorm:"type:timestamp"`
	Note            string       `gorm:"type:varchar(255);not null;default:''"`
	Status          enums.Status `gorm:"type:int;not null;default:0;index"`
	AuditFields
}
