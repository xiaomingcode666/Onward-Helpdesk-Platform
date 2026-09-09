package migration

import (
	"fmt"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(85, "score and gate knowledge candidates before review", func() error {
		return ensureKnowledgeCandidateQualityGate(sqls.DB())
	})
}

func ensureKnowledgeCandidateQualityGate(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.KnowledgeCandidate{}) {
		return nil
	}
	for _, field := range []string{
		"QualityScore",
		"ValueScore",
		"CandidateScore",
		"ScoreBreakdownJSON",
		"QualityFlagsJSON",
		"ScoreVersion",
		"ScoredAt",
		"SimilarityHash",
		"DuplicateGroupID",
		"MergedToCandidateID",
		"RecurrenceCount",
		"AffectedDeviceCount",
	} {
		if !db.Migrator().HasColumn(&models.KnowledgeCandidate{}, field) {
			if err := db.Migrator().AddColumn(&models.KnowledgeCandidate{}, field); err != nil {
				return err
			}
		}
	}
	if err := services.KnowledgeCandidateScoringService.BackfillDB(db); err != nil {
		return err
	}
	table, err := migrationModelTableName(db, &models.KnowledgeCandidate{})
	if err != nil {
		return err
	}
	quotedTable := quoteMigrationIdentifier(table)
	for _, statement := range []string{
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_knowledge_candidate_review_score ON %s (tenant_id, product_id, review_status, candidate_score)", quotedTable),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_knowledge_candidate_similarity_group ON %s (tenant_id, product_id, product_model_id, similarity_hash)", quotedTable),
	} {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}
