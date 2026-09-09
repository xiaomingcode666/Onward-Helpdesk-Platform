package migration

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(15, "backfill revisions and product scopes for legacy published knowledge", func() error {
		generation := services.KnowledgeIndexGenerationService.GetActiveGeneration()
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			if err := backfillPublishedDocumentRevisions(ctx.Tx, generation); err != nil {
				return err
			}
			return backfillPublishedFAQRevisions(ctx.Tx, generation)
		})
	})
}

func backfillPublishedDocumentRevisions(db *gorm.DB, generation *models.KnowledgeIndexGeneration) error {
	documents := repositories.KnowledgeDocumentRepository.Find(db, sqls.NewCnd().
		Eq("review_status", "published").
		NotEq("status", enums.StatusDeleted).
		Asc("id"))
	for i := range documents {
		document := &documents[i]
		revision, err := ensurePublishedKnowledgeRevision(db, models.KnowledgeRevision{
			TenantID:          document.TenantID,
			KnowledgeBaseID:   document.KnowledgeBaseID,
			EntryType:         "document",
			EntryID:           document.ID,
			Title:             document.Title,
			Content:           document.Content,
			Language:          migrationDefault(document.Language, "default"),
			Visibility:        "internal",
			TagsJSON:          migrationDefault(document.TagsJSON, "[]"),
			FaultCodesJSON:    migrationDefault(document.FaultCodesJSON, "[]"),
			ContentHash:       migrationContentHash(document.ContentHash, document.Content),
			PublishedByID:     document.UpdateUserID,
			PublishedByName:   document.UpdateUserName,
			IndexGenerationID: 0,
		})
		if err != nil {
			return err
		}
		if err := repositories.KnowledgeDocumentRepository.Updates(db, document.ID, map[string]any{
			"current_revision_id":   revision.ID,
			"published_revision_id": revision.ID,
			"updated_at":            time.Now(),
		}); err != nil {
			return err
		}
		if generation == nil {
			continue
		}
		if _, err := services.KnowledgeIndexSyncService.EnqueueEntrySyncToGenerationTx(
			db, document.TenantID, document.KnowledgeBaseID, "knowledge_document", document.ID,
			revision.ID, revision.ContentHash, "upsert", generation, nil,
		); err != nil {
			return err
		}
	}
	return nil
}

func backfillPublishedFAQRevisions(db *gorm.DB, generation *models.KnowledgeIndexGeneration) error {
	faqs := repositories.KnowledgeFAQRepository.Find(db, sqls.NewCnd().
		Eq("review_status", "published").
		NotEq("status", enums.StatusDeleted).
		Asc("id"))
	for i := range faqs {
		faq := &faqs[i]
		content := strings.TrimSpace(faq.Question) + "\n" + strings.TrimSpace(faq.Answer)
		revision, err := ensurePublishedKnowledgeRevision(db, models.KnowledgeRevision{
			TenantID:          faq.TenantID,
			KnowledgeBaseID:   faq.KnowledgeBaseID,
			EntryType:         "faq",
			EntryID:           faq.ID,
			Title:             faq.Question,
			Content:           faq.Answer,
			Language:          migrationDefault(faq.Language, "default"),
			Visibility:        "internal",
			TagsJSON:          migrationDefault(faq.TagsJSON, "[]"),
			FaultCodesJSON:    migrationDefault(faq.FaultCodesJSON, "[]"),
			ContentHash:       migrationContentHash("", content),
			PublishedByID:     faq.UpdateUserID,
			PublishedByName:   faq.UpdateUserName,
			IndexGenerationID: 0,
		})
		if err != nil {
			return err
		}
		if err := repositories.KnowledgeFAQRepository.Updates(db, faq.ID, map[string]any{
			"current_revision_id":   revision.ID,
			"published_revision_id": revision.ID,
			"updated_at":            time.Now(),
		}); err != nil {
			return err
		}
		if generation == nil {
			continue
		}
		if _, err := services.KnowledgeIndexSyncService.EnqueueEntrySyncToGenerationTx(
			db, faq.TenantID, faq.KnowledgeBaseID, "knowledge_faq", faq.ID,
			revision.ID, revision.ContentHash, "upsert", generation, nil,
		); err != nil {
			return err
		}
	}
	return nil
}

func ensurePublishedKnowledgeRevision(db *gorm.DB, snapshot models.KnowledgeRevision) (*models.KnowledgeRevision, error) {
	latest := repositories.KnowledgeRevisionRepository.FindLatestByEntry(db, snapshot.TenantID, snapshot.EntryType, snapshot.EntryID)
	if latest != nil && latest.ReviewStatus == "published" {
		return latest, nil
	}
	now := time.Now()
	versionNo := 1
	if latest != nil && latest.VersionNo >= versionNo {
		versionNo = latest.VersionNo + 1
	}
	snapshot.VersionNo = versionNo
	snapshot.VersionLabel = "v" + strconv.Itoa(versionNo)
	snapshot.ReviewStatus = "published"
	snapshot.PublishedAt = &now
	snapshot.AuditFields = models.AuditFields{CreatedAt: now, UpdatedAt: now}
	if err := repositories.KnowledgeRevisionRepository.Create(db, &snapshot); err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func migrationDefault(value, fallback string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return fallback
}

func migrationContentHash(existing, content string) string {
	if existing = strings.TrimSpace(existing); existing != "" {
		return existing
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(content)))
	return hex.EncodeToString(sum[:])
}
