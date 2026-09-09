package migration

import (
	"fmt"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(86, "add knowledge candidate reassessment and deduplication controls", func() error {
		return ensureKnowledgeCandidateStateRecoveryColumns(sqls.DB())
	})
}

func ensureKnowledgeCandidateStateRecoveryColumns(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.KnowledgeCandidate{}) {
		return nil
	}
	for _, field := range []string{"RequiresReassessment", "DeduplicationOverride"} {
		if !db.Migrator().HasColumn(&models.KnowledgeCandidate{}, field) {
			if err := db.Migrator().AddColumn(&models.KnowledgeCandidate{}, field); err != nil {
				return err
			}
		}
	}
	if db.Migrator().HasTable(&models.KnowledgeDocument{}) {
		var documents []models.KnowledgeDocument
		if err := db.Where("source_reference_id > 0 AND source_type IN ? AND status <> ?", []string{"knowledge_candidate", "knowledge_entry"}, enums.StatusDeleted).
			Order("tenant_id ASC, source_reference_id ASC, CASE WHEN review_status = 'published' THEN 0 ELSE 1 END ASC, id ASC").Find(&documents).Error; err != nil {
			return err
		}
		seen := make(map[[2]int64]int64)
		now := time.Now()
		for i := range documents {
			if documents[i].SourceType == "knowledge_entry" {
				if !db.Migrator().HasTable(&models.KnowledgeCandidate{}) {
					continue
				}
				var candidateCount int64
				if err := db.Model(&models.KnowledgeCandidate{}).
					Where("tenant_id = ? AND id = ? AND status <> ?", documents[i].TenantID, documents[i].SourceReferenceID, enums.StatusDeleted).
					Count(&candidateCount).Error; err != nil {
					return err
				}
				if candidateCount == 0 {
					continue
				}
			}
			key := [2]int64{documents[i].TenantID, documents[i].SourceReferenceID}
			if _, ok := seen[key]; !ok {
				seen[key] = documents[i].ID
				if err := db.Model(&models.KnowledgeDocument{}).Where("id = ?", documents[i].ID).Update("source_type", "knowledge_candidate").Error; err != nil {
					return err
				}
				if err := db.Model(&models.KnowledgeCandidate{}).
					Where("tenant_id = ? AND id = ? AND status <> ?", documents[i].TenantID, documents[i].SourceReferenceID, enums.StatusDeleted).
					Updates(map[string]any{
						"knowledge_base_id":  documents[i].KnowledgeBaseID,
						"knowledge_entry_id": documents[i].ID*10 + 1,
						"updated_at":         now,
					}).Error; err != nil {
					return err
				}
				continue
			}
			canonicalDocumentID := seen[key]
			if db.Migrator().HasTable(&models.ProductKnowledgeLink{}) {
				if err := db.Model(&models.ProductKnowledgeLink{}).
					Where("tenant_id = ? AND knowledge_entry_id = ? AND status <> ?", documents[i].TenantID, documents[i].ID, enums.StatusDeleted).
					Updates(map[string]any{"knowledge_entry_id": canonicalDocumentID, "updated_at": now, "update_user_name": "migration-86"}).Error; err != nil {
					return err
				}
			}
			if err := db.Model(&models.KnowledgeDocument{}).Where("id = ?", documents[i].ID).Updates(map[string]any{
				"source_type": "knowledge_candidate_duplicate", "review_status": "deprecated", "status": enums.StatusDisabled,
				"index_status": enums.KnowledgeDocumentIndexStatusPending, "indexed_at": nil,
				"index_error": "duplicate candidate document requires index cleanup", "updated_at": now, "update_user_name": "migration-86",
			}).Error; err != nil {
				return err
			}
		}
		table, err := migrationModelTableName(db, &models.KnowledgeDocument{})
		if err != nil {
			return err
		}
		if err := db.Exec(fmt.Sprintf(
			"CREATE UNIQUE INDEX IF NOT EXISTS ux_knowledge_document_candidate_source ON %s (tenant_id, source_reference_id) WHERE source_type = 'knowledge_candidate' AND source_reference_id > 0 AND status <> %d",
			quoteMigrationIdentifier(table), enums.StatusDeleted,
		)).Error; err != nil {
			return err
		}
	}
	if db.Migrator().HasTable(&models.ProductKnowledgeLink{}) {
		var links []models.ProductKnowledgeLink
		if err := db.Where("status <> ?", enums.StatusDeleted).
			Order("tenant_id ASC, product_id ASC, product_model_id ASC, knowledge_base_id ASC, knowledge_entry_id ASC, CASE WHEN publish_status = 'published' THEN 0 ELSE 1 END ASC, id ASC").
			Find(&links).Error; err != nil {
			return err
		}
		type linkIdentity struct {
			tenantID, productID, productModelID, knowledgeBaseID, knowledgeEntryID int64
			linkType, language, version                                            string
		}
		seen := make(map[linkIdentity]int64)
		now := time.Now()
		for i := range links {
			key := linkIdentity{
				tenantID: links[i].TenantID, productID: links[i].ProductID, productModelID: links[i].ProductModelID,
				knowledgeBaseID: links[i].KnowledgeBaseID, knowledgeEntryID: links[i].KnowledgeEntryID,
				linkType: links[i].LinkType, language: links[i].Language, version: links[i].Version,
			}
			if _, ok := seen[key]; !ok {
				seen[key] = links[i].ID
				continue
			}
			if err := db.Model(&models.ProductKnowledgeLink{}).Where("id = ?", links[i].ID).Updates(map[string]any{
				"status": enums.StatusDeleted, "updated_at": now, "update_user_name": "migration-86",
			}).Error; err != nil {
				return err
			}
		}
		table, err := migrationModelTableName(db, &models.ProductKnowledgeLink{})
		if err != nil {
			return err
		}
		if err := db.Exec(fmt.Sprintf(
			"CREATE UNIQUE INDEX IF NOT EXISTS ux_product_knowledge_link_active_entry ON %s (tenant_id, product_id, product_model_id, knowledge_base_id, knowledge_entry_id, link_type, language, version) WHERE status <> %d",
			quoteMigrationIdentifier(table), enums.StatusDeleted,
		)).Error; err != nil {
			return err
		}
	}
	return services.KnowledgeCandidateScoringService.BackfillDB(db)
}
