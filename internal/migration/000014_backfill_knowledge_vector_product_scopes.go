package migration

import (
	"strings"

	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(14, "backfill product scope payloads for active knowledge vectors", func() error {
		generation := services.KnowledgeIndexGenerationService.GetActiveGeneration()
		if generation == nil {
			return nil
		}
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			documents := repositories.KnowledgeDocumentRepository.Find(ctx.Tx, sqls.NewCnd().
				Eq("review_status", "published").
				NotEq("status", enums.StatusDeleted).
				Asc("id"))
			for i := range documents {
				revisionID := publishedKnowledgeRevisionID(documents[i].PublishedRevisionID, documents[i].CurrentRevisionID)
				if revisionID <= 0 {
					continue
				}
				if _, err := services.KnowledgeIndexSyncService.EnqueueEntrySyncToGenerationTx(
					ctx.Tx,
					documents[i].TenantID,
					documents[i].KnowledgeBaseID,
					"knowledge_document",
					documents[i].ID,
					revisionID,
					strings.TrimSpace(documents[i].ContentHash),
					"upsert",
					generation,
					nil,
				); err != nil {
					return err
				}
			}

			faqs := repositories.KnowledgeFAQRepository.Find(ctx.Tx, sqls.NewCnd().
				Eq("review_status", "published").
				NotEq("status", enums.StatusDeleted).
				Asc("id"))
			for i := range faqs {
				revisionID := publishedKnowledgeRevisionID(faqs[i].PublishedRevisionID, faqs[i].CurrentRevisionID)
				if revisionID <= 0 {
					continue
				}
				if _, err := services.KnowledgeIndexSyncService.EnqueueEntrySyncToGenerationTx(
					ctx.Tx,
					faqs[i].TenantID,
					faqs[i].KnowledgeBaseID,
					"knowledge_faq",
					faqs[i].ID,
					revisionID,
					"",
					"upsert",
					generation,
					nil,
				); err != nil {
					return err
				}
			}
			return nil
		})
	})
}

func publishedKnowledgeRevisionID(publishedRevisionID, currentRevisionID int64) int64 {
	if publishedRevisionID > 0 {
		return publishedRevisionID
	}
	return currentRevisionID
}
