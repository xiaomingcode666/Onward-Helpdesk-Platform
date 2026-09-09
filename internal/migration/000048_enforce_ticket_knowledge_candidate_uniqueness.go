package migration

import (
	"fmt"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(48, "deduplicate ticket knowledge candidates and enforce uniqueness", func() error {
		return enforceTicketKnowledgeCandidateUniqueness(sqls.DB())
	})
}

func enforceTicketKnowledgeCandidateUniqueness(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.KnowledgeCandidate{}) {
		return nil
	}
	var candidates []models.KnowledgeCandidate
	if err := db.Where("ticket_id > 0 AND status <> ?", enums.StatusDeleted).
		Order("tenant_id ASC, ticket_id ASC, id ASC").
		Find(&candidates).Error; err != nil {
		return err
	}
	winners := make(map[[2]int64]models.KnowledgeCandidate)
	for i := range candidates {
		candidate := candidates[i]
		key := [2]int64{candidate.TenantID, candidate.TicketID}
		winner, ok := winners[key]
		if !ok || preferKnowledgeCandidate(candidate, winner) {
			winners[key] = candidate
		}
	}
	now := time.Now()
	for i := range candidates {
		candidate := candidates[i]
		winner := winners[[2]int64{candidate.TenantID, candidate.TicketID}]
		if candidate.ID == winner.ID {
			continue
		}
		if err := db.Model(&models.KnowledgeCandidate{}).Where("id = ?", candidate.ID).Updates(map[string]any{
			"status":           enums.StatusDeleted,
			"review_status":    "rejected",
			"review_remark":    fmt.Sprintf("同工单重复候选已合并到候选 #%d", winner.ID),
			"reviewed_at":      now,
			"updated_at":       now,
			"update_user_name": "migration-48",
		}).Error; err != nil {
			return err
		}
	}
	table, err := migrationModelTableName(db, &models.KnowledgeCandidate{})
	if err != nil {
		return err
	}
	return db.Exec(fmt.Sprintf(
		"CREATE UNIQUE INDEX IF NOT EXISTS ux_knowledge_candidate_active_ticket ON %s (tenant_id, ticket_id) WHERE ticket_id > 0 AND status <> %d",
		quoteMigrationIdentifier(table),
		enums.StatusDeleted,
	)).Error
}

func preferKnowledgeCandidate(candidate, current models.KnowledgeCandidate) bool {
	candidateScore := knowledgeCandidatePriority(candidate)
	currentScore := knowledgeCandidatePriority(current)
	if candidateScore != currentScore {
		return candidateScore > currentScore
	}
	return candidate.ID > current.ID
}

func knowledgeCandidatePriority(candidate models.KnowledgeCandidate) int {
	if candidate.KnowledgeEntryID > 0 || candidate.ReviewStatus == "approved" || candidate.ReviewStatus == "merged" {
		return 3
	}
	if candidate.SourceType == "ticket_repair" {
		return 2
	}
	return 1
}
