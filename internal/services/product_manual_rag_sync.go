package services

import (
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

func (s *productManualFileService) SyncPendingManuals(tenantID, productID int64, operator *dto.AuthPrincipal) {
	items := repositories.ProductManualFileRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		Where("(knowledge_document_id = 0 OR sync_status IN ?)", []string{
			string(enums.KnowledgeDocumentIndexStatusPending),
			string(enums.KnowledgeDocumentIndexStatusFailed),
		}).
		Where("status <> ?", enums.StatusDeleted).
		Asc("id"))
	for i := range items {
		_ = s.syncManualFileKnowledge(&items[i], operator)
	}
}

func (s *productManualFileService) syncManualFileKnowledge(item *models.ProductManualFile, operator *dto.AuthPrincipal) error {
	if item == nil || item.ID <= 0 {
		return errorsx.InvalidParam("product manual file is required")
	}
	product, knowledgeBaseID, err := KnowledgeDocumentService.resolveProductKnowledgeBase(item.TenantID, item.ProductID)
	if err != nil {
		return repositories.ProductManualFileRepository.Updates(sqls.DB(), item.ID, map[string]any{
			"knowledge_base_id": 0,
			"sync_status":       enums.KnowledgeDocumentIndexStatusPending,
			"sync_error":        "",
			"updated_at":        time.Now(),
		})
	}

	if item.KnowledgeDocumentID > 0 {
		document := repositories.KnowledgeDocumentRepository.Get(sqls.DB(), item.KnowledgeDocumentID)
		link := repositories.ProductKnowledgeLinkRepository.Get(sqls.DB(), item.KnowledgeLinkID)
		if document != nil && link != nil && document.Status != enums.StatusDeleted && link.Status != enums.StatusDeleted && document.KnowledgeBaseID == knowledgeBaseID {
			return s.updateManualKnowledgeMetadata(item, document, link, operator)
		}
	}

	asset := AssetService.Get(item.AssetID)
	if asset == nil {
		return s.markManualKnowledgeSyncFailed(item.ID, "product manual asset not found")
	}
	content, err := extractKnowledgeDocumentContentFromAsset(asset)
	if err != nil {
		_ = s.markManualKnowledgeSyncFailed(item.ID, err.Error())
		return err
	}
	document, err := KnowledgeDocumentService.createPublishedProductKnowledgeDocument(publishedProductKnowledgeDocumentInput{
		TenantID:          item.TenantID,
		KnowledgeBaseID:   knowledgeBaseID,
		Title:             firstNonBlank(strings.TrimSpace(item.Title), asset.Filename),
		Content:           content,
		Language:          firstNonBlank(item.Language, product.DefaultLocale, "default"),
		SourceAssetID:     asset.ID,
		SourceType:        "product_manual",
		SourceReferenceID: item.ID,
	}, operator)
	if err != nil {
		_ = s.markManualKnowledgeSyncFailed(item.ID, err.Error())
		return err
	}
	link, err := ProductKnowledgeLinkService.CreateLink(CreateProductKnowledgeLinkRequest{
		TenantID:         item.TenantID,
		ProductID:        item.ProductID,
		KnowledgeBaseID:  knowledgeBaseID,
		KnowledgeEntryID: document.ID,
		LinkType:         "manual",
		Language:         firstNonBlank(item.Language, product.DefaultLocale, "default"),
		Version:          firstNonBlank(item.Version, "v1.0"),
		Visibility:       normalizeProductManualVisibility(item.Visibility),
	}, operator)
	if err != nil {
		_ = KnowledgeDocumentService.DeleteKnowledgeDocument(document.ID)
		_ = s.markManualKnowledgeSyncFailed(item.ID, err.Error())
		return err
	}
	if err := ProductKnowledgeLinkService.PublishLinkScoped(item.TenantID, item.ProductID, link.ID, operator); err != nil {
		_ = ProductKnowledgeLinkService.DeleteLinkScoped(item.TenantID, item.ProductID, link.ID, operator)
		_ = KnowledgeDocumentService.DeleteKnowledgeDocument(document.ID)
		_ = s.markManualKnowledgeSyncFailed(item.ID, err.Error())
		return err
	}
	if err := repositories.ProductManualFileRepository.Updates(sqls.DB(), item.ID, map[string]any{
		"knowledge_base_id":     knowledgeBaseID,
		"knowledge_document_id": document.ID,
		"knowledge_link_id":     link.ID,
		"sync_status":           enums.KnowledgeDocumentIndexStatusPending,
		"sync_error":            "",
		"synced_at":             nil,
		"updated_at":            time.Now(),
	}); err != nil {
		return err
	}
	if _, err := ProductKnowledgeLinkService.ReindexLink(item.TenantID, item.ProductID, link.ID, operator); err != nil {
		_ = s.markManualKnowledgeSyncFailed(item.ID, err.Error())
		return err
	}
	return nil
}

func (s *productManualFileService) updateManualKnowledgeMetadata(
	item *models.ProductManualFile,
	document *models.KnowledgeDocument,
	link *models.ProductKnowledgeLink,
	operator *dto.AuthPrincipal,
) error {
	now := time.Now()
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		snapshot := *document
		snapshot.Title = item.Title
		snapshot.Language = firstNonBlank(item.Language, "default")
		snapshot.ReviewStatus = "published"
		revision, err := EnterpriseKnowledgeService.publishDocumentRevision(ctx.Tx, snapshot, now, operator)
		if err != nil {
			return err
		}
		if err := repositories.KnowledgeDocumentRepository.Updates(ctx.Tx, document.ID, map[string]any{
			"title":                 snapshot.Title,
			"language":              snapshot.Language,
			"source_asset_id":       item.AssetID,
			"source_type":           "product_manual",
			"source_reference_id":   item.ID,
			"review_status":         "published",
			"current_revision_id":   revision.ID,
			"published_revision_id": revision.ID,
			"index_status":          enums.KnowledgeDocumentIndexStatusPending,
			"indexed_at":            nil,
			"index_error":           "",
			"updated_at":            now,
		}); err != nil {
			return err
		}
		return repositories.ProductKnowledgeLinkRepository.Updates(ctx.Tx, link.ID, map[string]any{
			"language":         firstNonBlank(item.Language, "default"),
			"version":          firstNonBlank(item.Version, "v1.0"),
			"visibility":       normalizeProductManualVisibility(item.Visibility),
			"publish_status":   "published",
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
			"updated_at":       now,
		})
	}); err != nil {
		_ = s.markManualKnowledgeSyncFailed(item.ID, err.Error())
		return err
	}
	if _, err := ProductKnowledgeLinkService.ReindexLink(item.TenantID, item.ProductID, link.ID, operator); err != nil {
		_ = s.markManualKnowledgeSyncFailed(item.ID, err.Error())
		return err
	}
	return repositories.ProductManualFileRepository.Updates(sqls.DB(), item.ID, map[string]any{
		"knowledge_base_id": document.KnowledgeBaseID,
		"sync_status":       enums.KnowledgeDocumentIndexStatusPending,
		"sync_error":        "",
		"synced_at":         nil,
		"updated_at":        time.Now(),
	})
}

func (s *productManualFileService) markManualKnowledgeSyncFailed(manualFileID int64, message string) error {
	now := time.Now()
	item := repositories.ProductManualFileRepository.Get(sqls.DB(), manualFileID)
	if item != nil && item.KnowledgeDocumentID > 0 {
		_ = repositories.KnowledgeDocumentRepository.Updates(sqls.DB(), item.KnowledgeDocumentID, map[string]any{
			"index_status": enums.KnowledgeDocumentIndexStatusFailed,
			"index_error":  strings.TrimSpace(message),
			"indexed_at":   nil,
			"updated_at":   now,
		})
	}
	return repositories.ProductManualFileRepository.Updates(sqls.DB(), manualFileID, map[string]any{
		"sync_status": enums.KnowledgeDocumentIndexStatusFailed,
		"sync_error":  strings.TrimSpace(message),
		"synced_at":   nil,
		"updated_at":  now,
	})
}
