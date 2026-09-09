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
	register(92, "repair candidate document references and product link uniqueness", func() error {
		db := sqls.DB()
		if db != nil && db.Migrator().HasTable(&models.ProductKnowledgeLink{}) {
			if err := db.Exec("DROP INDEX IF EXISTS ux_product_knowledge_link_active_entry").Error; err != nil {
				return err
			}
		}
		if err := repairLegacyKnowledgeCandidateDocumentReferences(db); err != nil {
			return err
		}
		return fixProductKnowledgeLinkUniqueScope(db)
	})
}

func repairLegacyKnowledgeCandidateDocumentReferences(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.KnowledgeCandidate{}) || !db.Migrator().HasTable(&models.KnowledgeDocument{}) {
		return nil
	}
	var candidates []models.KnowledgeCandidate
	if err := db.Where("status <> ?", enums.StatusDeleted).Order("id ASC").Find(&candidates).Error; err != nil {
		return err
	}
	now := time.Now()
	for i := range candidates {
		var documents []models.KnowledgeDocument
		if err := db.Where("tenant_id = ? AND source_reference_id = ? AND source_type IN ?",
			candidates[i].TenantID, candidates[i].ID, []string{"knowledge_candidate", "knowledge_entry", "knowledge_candidate_duplicate"}).
			Order("CASE WHEN source_type = 'knowledge_candidate' AND status <> 2 THEN 0 ELSE 1 END ASC, CASE WHEN review_status = 'published' THEN 0 ELSE 1 END ASC, id ASC").
			Find(&documents).Error; err != nil {
			return err
		}
		if len(documents) == 0 {
			continue
		}
		canonical := documents[0]
		canonicalStatus := enums.StatusDisabled
		if canonical.ReviewStatus == "published" {
			canonicalStatus = enums.StatusOk
		}
		if err := db.Model(&models.KnowledgeDocument{}).Where("id = ?", canonical.ID).Updates(map[string]any{
			"source_type": "knowledge_candidate", "status": canonicalStatus, "updated_at": now, "update_user_name": "migration-89",
		}).Error; err != nil {
			return err
		}
		if err := db.Model(&models.KnowledgeCandidate{}).Where("id = ? AND tenant_id = ?", candidates[i].ID, candidates[i].TenantID).Updates(map[string]any{
			"knowledge_base_id": canonical.KnowledgeBaseID, "knowledge_entry_id": canonical.ID*10 + 1, "updated_at": now,
		}).Error; err != nil {
			return err
		}
		for j := 1; j < len(documents); j++ {
			if db.Migrator().HasTable(&models.ProductKnowledgeLink{}) {
				if err := db.Model(&models.ProductKnowledgeLink{}).
					Where("tenant_id = ? AND knowledge_entry_id = ? AND status <> ?", candidates[i].TenantID, documents[j].ID, enums.StatusDeleted).
					Updates(map[string]any{"knowledge_entry_id": canonical.ID, "updated_at": now, "update_user_name": "migration-89"}).Error; err != nil {
					return err
				}
			}
			if err := db.Model(&models.KnowledgeDocument{}).Where("id = ?", documents[j].ID).Updates(map[string]any{
				"source_type": "knowledge_candidate_duplicate", "review_status": "deprecated", "status": enums.StatusDisabled,
				"index_status": enums.KnowledgeDocumentIndexStatusPending, "indexed_at": nil,
				"index_error": "duplicate candidate document requires index cleanup", "updated_at": now, "update_user_name": "migration-89",
			}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func fixProductKnowledgeLinkUniqueScope(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.ProductKnowledgeLink{}) {
		return nil
	}
	type linkIdentity struct {
		tenantID, productID, productModelID, knowledgeBaseID, knowledgeEntryID int64
		linkType, language, version                                            string
	}
	var links []models.ProductKnowledgeLink
	if err := db.Where("status <> ?", enums.StatusDeleted).
		Order("tenant_id ASC, product_id ASC, product_model_id ASC, knowledge_base_id ASC, knowledge_entry_id ASC, link_type ASC, language ASC, version ASC, CASE WHEN publish_status = 'published' THEN 0 ELSE 1 END ASC, id ASC").
		Find(&links).Error; err != nil {
		return err
	}
	seen := make(map[linkIdentity]struct{}, len(links))
	now := time.Now()
	for i := range links {
		key := linkIdentity{
			tenantID: links[i].TenantID, productID: links[i].ProductID, productModelID: links[i].ProductModelID,
			knowledgeBaseID: links[i].KnowledgeBaseID, knowledgeEntryID: links[i].KnowledgeEntryID,
			linkType: links[i].LinkType, language: links[i].Language, version: links[i].Version,
		}
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			continue
		}
		if err := db.Model(&models.ProductKnowledgeLink{}).Where("id = ?", links[i].ID).Updates(map[string]any{
			"status": enums.StatusDeleted, "updated_at": now, "update_user_name": "migration-89",
		}).Error; err != nil {
			return err
		}
	}
	table, err := migrationModelTableName(db, &models.ProductKnowledgeLink{})
	if err != nil {
		return err
	}
	if err := db.Exec("DROP INDEX IF EXISTS ux_product_knowledge_link_active_entry").Error; err != nil {
		return err
	}
	return db.Exec(fmt.Sprintf(
		"CREATE UNIQUE INDEX IF NOT EXISTS ux_product_knowledge_link_active_entry ON %s (tenant_id, product_id, product_model_id, knowledge_base_id, knowledge_entry_id, link_type, language, version) WHERE status <> %d",
		quoteMigrationIdentifier(table), enums.StatusDeleted,
	)).Error
}
