package services

import (
	"context"
	"fmt"
	"log/slog"
	"mime/multipart"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

func (s *knowledgeDocumentService) ListEnterpriseTenantDocuments(
	tenantID, knowledgeBaseID int64,
	page, pageSize int,
) (*dto.EnterpriseListResponse[dto.EnterpriseProductKnowledgeDocumentDTO], error) {
	if _, err := s.requireEnterpriseTenantKnowledgeBase(tenantID, knowledgeBaseID); err != nil {
		return nil, err
	}
	page, pageSize = normalizeEnterprisePage(page, pageSize)
	cnd := sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("knowledge_base_id", knowledgeBaseID).
		Where("(source_asset_id > 0 OR review_status = ?)", "published").
		Where("status <> ?", enums.StatusDeleted).
		Desc("updated_at").
		Desc("id").
		Page(page, pageSize)
	documents, paging := repositories.KnowledgeDocumentRepository.FindPageByCnd(sqls.DB(), cnd)
	result := make([]dto.EnterpriseProductKnowledgeDocumentDTO, 0, len(documents))
	for _, document := range documents {
		sourceType := strings.TrimSpace(document.SourceType)
		if sourceType == "" {
			sourceType = "knowledge_entry"
		}
		result = append(result, s.buildEnterpriseProductDocumentDTO(0, document, 0, sourceType, document.SourceReferenceID))
	}
	return enterprisePage(result, paging, page, pageSize), nil
}

func (s *knowledgeDocumentService) UploadEnterpriseTenantDocument(
	tenantID, knowledgeBaseID int64,
	file *multipart.FileHeader,
	operator *dto.AuthPrincipal,
) (*dto.EnterpriseProductKnowledgeDocumentDTO, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if _, err := s.requireEnterpriseTenantKnowledgeBase(tenantID, knowledgeBaseID); err != nil {
		return nil, err
	}
	if err := requireKnowledgeTenantOwnership(tenantID, operator); err != nil {
		return nil, err
	}
	if err := TenantCommercialService.RequireKnowledgeDocumentUpload(tenantID, 0, operator, file.Size); err != nil {
		return nil, err
	}
	asset, err := AssetService.UploadFile(file, fmt.Sprintf("knowledge-documents/%d", knowledgeBaseID), operator)
	if err != nil {
		return nil, err
	}
	content, err := extractKnowledgeDocumentContentFromAsset(asset)
	if err != nil {
		_ = AssetService.DeleteAsset(asset.ID, operator)
		return nil, err
	}
	document, err := s.createPublishedProductKnowledgeDocument(publishedProductKnowledgeDocumentInput{
		TenantID:          tenantID,
		KnowledgeBaseID:   knowledgeBaseID,
		Title:             firstNonBlank(strings.TrimSpace(file.Filename), fmt.Sprintf("Knowledge document %d", asset.ID)),
		Content:           content,
		Language:          "default",
		SourceAssetID:     asset.ID,
		SourceType:        "uploaded_document",
		SourceReferenceID: asset.ID,
	}, operator)
	if err != nil {
		_ = AssetService.DeleteAsset(asset.ID, operator)
		return nil, err
	}
	if _, err := KnowledgeIndexSyncService.RequestEntryReindex(context.Background(), tenantID, "document", document.ID, operator); err != nil {
		slog.Warn("enqueue uploaded tenant knowledge document failed", "document_id", document.ID, "error", err)
		_ = repositories.KnowledgeDocumentRepository.Updates(sqls.DB(), document.ID, map[string]any{
			"index_status": enums.KnowledgeDocumentIndexStatusFailed,
			"index_error":  err.Error(),
			"indexed_at":   nil,
			"updated_at":   time.Now(),
		})
	}
	document = repositories.KnowledgeDocumentRepository.Get(sqls.DB(), document.ID)
	result := s.buildEnterpriseProductDocumentDTO(0, *document, 0, "uploaded_document", asset.ID)
	return &result, nil
}

func (s *knowledgeDocumentService) ReprocessEnterpriseTenantDocument(
	tenantID, knowledgeBaseID, documentID int64,
	operator *dto.AuthPrincipal,
) (*dto.EnterpriseProductKnowledgeDocumentDTO, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if _, err := s.requireEnterpriseTenantKnowledgeBase(tenantID, knowledgeBaseID); err != nil {
		return nil, err
	}
	if err := requireKnowledgeTenantOwnership(tenantID, operator); err != nil {
		return nil, err
	}
	document := repositories.KnowledgeDocumentRepository.Get(sqls.DB(), documentID)
	if document == nil || document.TenantID != tenantID || document.KnowledgeBaseID != knowledgeBaseID || document.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParam("tenant knowledge document not found")
	}
	if _, err := KnowledgeIndexSyncService.RequestEntryReindex(context.Background(), tenantID, "document", document.ID, operator); err != nil {
		_ = repositories.KnowledgeDocumentRepository.Updates(sqls.DB(), document.ID, map[string]any{
			"index_status": enums.KnowledgeDocumentIndexStatusFailed,
			"index_error":  err.Error(),
			"updated_at":   time.Now(),
		})
		return nil, err
	}
	document = repositories.KnowledgeDocumentRepository.Get(sqls.DB(), document.ID)
	result := s.buildEnterpriseProductDocumentDTO(0, *document, 0, firstNonBlank(document.SourceType, "knowledge_entry"), document.SourceReferenceID)
	return &result, nil
}

func (s *knowledgeDocumentService) DeleteEnterpriseTenantDocument(
	tenantID, knowledgeBaseID, documentID int64,
	operator *dto.AuthPrincipal,
) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if _, err := s.requireEnterpriseTenantKnowledgeBase(tenantID, knowledgeBaseID); err != nil {
		return err
	}
	if err := requireKnowledgeTenantOwnership(tenantID, operator); err != nil {
		return err
	}
	document := repositories.KnowledgeDocumentRepository.Get(sqls.DB(), documentID)
	if document == nil || document.TenantID != tenantID || document.KnowledgeBaseID != knowledgeBaseID || document.Status == enums.StatusDeleted || document.SourceAssetID <= 0 || document.SourceType != "uploaded_document" {
		return errorsx.InvalidParam("uploaded tenant knowledge document not found")
	}
	if document.PublishedRevisionID > 0 {
		if _, err := KnowledgeIndexSyncService.EnqueueEntryRevisionSync(
			tenantID,
			knowledgeBaseID,
			"document",
			document.ID,
			document.PublishedRevisionID,
			document.ContentHash,
			"delete",
			operator,
		); err != nil {
			return err
		}
	}
	if err := s.DeleteKnowledgeDocument(document.ID); err != nil {
		return err
	}
	return AssetService.DeleteAsset(document.SourceAssetID, operator)
}

func (s *knowledgeDocumentService) requireEnterpriseTenantKnowledgeBase(tenantID, knowledgeBaseID int64) (*models.KnowledgeBase, error) {
	if tenantID <= 0 || knowledgeBaseID <= 0 {
		return nil, errorsx.InvalidParam("tenant knowledge base is required")
	}
	knowledgeBase := repositories.KnowledgeBaseRepository.Get(sqls.DB(), knowledgeBaseID)
	if knowledgeBase == nil || knowledgeBase.TenantID != tenantID || knowledgeBase.Status != enums.StatusOk {
		return nil, errorsx.InvalidParam("tenant knowledge base not found")
	}
	if knowledgeBase.AccessScope != string(enums.KnowledgeBaseAccessScopeTenant) {
		return nil, errorsx.InvalidParam("knowledge base is not tenant scoped")
	}
	if knowledgeBase.KnowledgeType != string(enums.KnowledgeBaseTypeDocument) {
		return nil, errorsx.InvalidParam("tenant knowledge base does not accept documents")
	}
	return knowledgeBase, nil
}
