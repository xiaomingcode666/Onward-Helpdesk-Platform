package migration

import (
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(53, "normalize pending knowledge candidate solution prefixes", func() error {
		return normalizePendingKnowledgeCandidateSolutionPrefixes(sqls.DB())
	})
}

func normalizePendingKnowledgeCandidateSolutionPrefixes(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.KnowledgeCandidate{}) {
		return nil
	}
	var candidates []models.KnowledgeCandidate
	if err := db.Where("status <> ? AND knowledge_entry_id = 0 AND review_status = ? AND solution_summary LIKE ?", enums.StatusDeleted, "pending", "处理方案：%").
		Order("id ASC").
		Find(&candidates).Error; err != nil {
		return err
	}
	now := time.Now()
	for i := range candidates {
		next := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(candidates[i].SolutionSummary), "处理方案："))
		if next == "" || next == candidates[i].SolutionSummary {
			continue
		}
		if err := db.Model(&models.KnowledgeCandidate{}).Where("id = ?", candidates[i].ID).Updates(map[string]any{
			"solution_summary": next,
			"updated_at":       now,
			"update_user_name": "migration-53",
		}).Error; err != nil {
			return err
		}
	}
	return nil
}
