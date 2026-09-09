package services

import (
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"strings"
	"time"

	"remotehelpdesk/internal/ai/rag/vectordb"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var ProductManualFileService = newProductManualFileService()

func newProductManualFileService() *productManualFileService {
	return &productManualFileService{}
}

type productManualFileService struct{}

type ProductManualFileContent struct {
	Filename string
	FileSize int64
	MimeType string
	Reader   io.ReadCloser
}

func (s *productManualFileService) ListEnterpriseManualFiles(tenantID, productID int64, operator *dto.AuthPrincipal) ([]dto.EnterpriseProductManualFileDTO, error) {
	if _, err := ProductCenterService.requireTenantProduct(tenantID, productID); err != nil {
		return nil, err
	}
	items := repositories.ProductManualFileRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		Where("status <> ?", enums.StatusDeleted).
		Desc("updated_at").
		Desc("id"))
	profile := repositories.ProductServiceProfileRepository.GetByProductID(sqls.DB(), productID)
	if operator != nil && profile != nil && profile.TenantID == tenantID && profile.DefaultKnowledgeBaseID > 0 {
		for i := range items {
			if items[i].KnowledgeDocumentID > 0 && items[i].AssetID > 0 {
				document := repositories.KnowledgeDocumentRepository.Get(sqls.DB(), items[i].KnowledgeDocumentID)
				if document != nil && document.SourceAssetID <= 0 {
					if err := repositories.KnowledgeDocumentRepository.Updates(sqls.DB(), document.ID, map[string]any{
						"source_asset_id":     items[i].AssetID,
						"source_type":         "product_manual",
						"source_reference_id": items[i].ID,
						"updated_at":          time.Now(),
					}); err != nil {
						slog.Warn("backfill product manual knowledge asset reference failed", "manual_file_id", items[i].ID, "error", err)
					}
				}
			}
			needsKnowledgeCopy := items[i].KnowledgeDocumentID <= 0 && items[i].SyncStatus != enums.KnowledgeDocumentIndexStatusFailed
			needsProviderRecovery := items[i].KnowledgeDocumentID > 0 &&
				items[i].SyncStatus == enums.KnowledgeDocumentIndexStatusFailed &&
				strings.Contains(items[i].SyncError, "vectordb provider not initialized") &&
				vectordb.GetProvider() != nil
			if !needsKnowledgeCopy && !needsProviderRecovery {
				continue
			}
			if err := s.syncManualFileKnowledge(&items[i], operator); err != nil {
				slog.Warn("reconcile legacy product manual knowledge copy failed", "manual_file_id", items[i].ID, "error", err)
			}
			if stored := repositories.ProductManualFileRepository.Get(sqls.DB(), items[i].ID); stored != nil {
				items[i] = *stored
			}
		}
	}
	result := make([]dto.EnterpriseProductManualFileDTO, 0, len(items))
	for i := range items {
		item := items[i]
		result = append(result, s.buildManualFileDTO(item))
	}
	return result, nil
}

func (s *productManualFileService) ListCustomerManualFiles(tenantID, productID int64) ([]dto.EnterpriseProductManualFileDTO, error) {
	items, err := s.ListEnterpriseManualFiles(tenantID, productID, nil)
	if err != nil {
		return nil, err
	}
	visible := make([]dto.EnterpriseProductManualFileDTO, 0, len(items))
	for _, item := range items {
		if item.Visibility == "public" {
			visible = append(visible, item)
		}
	}
	return visible, nil
}

func (s *productManualFileService) OpenEnterpriseManualFileContent(
	tenantID, productID, manualFileID int64,
) (*ProductManualFileContent, error) {
	if _, err := ProductCenterService.requireTenantProduct(tenantID, productID); err != nil {
		return nil, err
	}
	item := repositories.ProductManualFileRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		Eq("id", manualFileID).
		Where("status <> ?", enums.StatusDeleted))
	if item == nil {
		return nil, errorsx.InvalidParam("product manual file not found")
	}
	asset := AssetService.Get(item.AssetID)
	if asset == nil || (asset.TenantID > 0 && asset.TenantID != tenantID) {
		return nil, errorsx.InvalidParam("product manual asset not found")
	}
	if asset.Status != enums.AssetStatusSuccess {
		return nil, errorsx.InvalidParam("product manual asset is unavailable")
	}
	reader, err := AssetService.OpenReader(asset)
	if err != nil {
		return nil, err
	}
	return &ProductManualFileContent{
		Filename: asset.Filename,
		FileSize: asset.FileSize,
		MimeType: asset.MimeType,
		Reader:   reader,
	}, nil
}

func (s *productManualFileService) UploadEnterpriseManualFile(
	tenantID, productID int64,
	file *multipart.FileHeader,
	req dto.EnterpriseProductManualFileMutationRequest,
	operator *dto.AuthPrincipal,
) (*dto.EnterpriseProductManualFileDTO, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	product, err := ProductCenterService.requireTenantProduct(tenantID, productID)
	if err != nil {
		return nil, err
	}
	asset, err := AssetService.UploadFile(file, fmt.Sprintf("product-manuals/%d", product.ID), operator)
	if err != nil {
		return nil, err
	}
	item := &models.ProductManualFile{
		TenantID:    tenantID,
		ProductID:   product.ID,
		AssetID:     asset.ID,
		Title:       firstNonBlank(strings.TrimSpace(req.Title), strings.TrimSpace(file.Filename)),
		Language:    firstNonBlank(strings.TrimSpace(req.Language), strings.TrimSpace(product.DefaultLocale), "default"),
		Version:     firstNonBlank(strings.TrimSpace(req.Version), "v1.0"),
		Visibility:  normalizeProductManualVisibility(req.Visibility),
		SyncStatus:  enums.KnowledgeDocumentIndexStatusPending,
		Status:      enums.StatusOk,
		AuditFields: utils.BuildAuditFields(operator),
	}
	if err := repositories.ProductManualFileRepository.Create(sqls.DB(), item); err != nil {
		_ = AssetService.DeleteAsset(asset.ID, operator)
		return nil, err
	}
	if err := s.syncManualFileKnowledge(item, operator); err != nil {
		slog.Warn("sync product manual to knowledge base failed", "manual_file_id", item.ID, "error", err)
	}
	if stored := repositories.ProductManualFileRepository.Get(sqls.DB(), item.ID); stored != nil {
		item = stored
	}
	result := s.buildManualFileDTO(*item)
	return &result, nil
}

func (s *productManualFileService) UpdateEnterpriseManualFile(
	tenantID, productID, manualFileID int64,
	req dto.EnterpriseProductManualFileMutationRequest,
	operator *dto.AuthPrincipal,
) (*dto.EnterpriseProductManualFileDTO, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item := repositories.ProductManualFileRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		Eq("id", manualFileID).
		Where("status <> ?", enums.StatusDeleted))
	if item == nil {
		return nil, errorsx.InvalidParam("product manual file not found")
	}
	updates := map[string]any{
		"title":            firstNonBlank(strings.TrimSpace(req.Title), item.Title),
		"language":         firstNonBlank(strings.TrimSpace(req.Language), item.Language, "default"),
		"version":          firstNonBlank(strings.TrimSpace(req.Version), item.Version, "v1.0"),
		"visibility":       normalizeProductManualVisibility(firstNonBlank(req.Visibility, item.Visibility)),
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	}
	if err := repositories.ProductManualFileRepository.Updates(sqls.DB(), item.ID, updates); err != nil {
		return nil, err
	}
	item = repositories.ProductManualFileRepository.Get(sqls.DB(), item.ID)
	if err := s.syncManualFileKnowledge(item, operator); err != nil {
		slog.Warn("resync updated product manual failed", "manual_file_id", item.ID, "error", err)
	}
	item = repositories.ProductManualFileRepository.Get(sqls.DB(), item.ID)
	result := s.buildManualFileDTO(*item)
	return &result, nil
}

func (s *productManualFileService) DeleteEnterpriseManualFile(tenantID, productID, manualFileID int64, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item := repositories.ProductManualFileRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		Eq("id", manualFileID).
		Where("status <> ?", enums.StatusDeleted))
	if item == nil {
		return errorsx.InvalidParam("product manual file not found")
	}
	if err := repositories.ProductManualFileRepository.Updates(sqls.DB(), item.ID, map[string]any{
		"status":           enums.StatusDeleted,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	}); err != nil {
		return err
	}
	if item.KnowledgeLinkID > 0 {
		if err := ProductKnowledgeLinkService.DeleteLinkScoped(tenantID, productID, item.KnowledgeLinkID, operator); err != nil {
			slog.Warn("delete product manual knowledge link failed", "manual_file_id", item.ID, "error", err)
		}
	}
	if item.KnowledgeDocumentID > 0 {
		if err := KnowledgeDocumentService.DeleteKnowledgeDocument(item.KnowledgeDocumentID); err != nil {
			slog.Warn("delete product manual knowledge document failed", "manual_file_id", item.ID, "error", err)
		}
	}
	if item.AssetID > 0 {
		_ = AssetService.DeleteAsset(item.AssetID, operator)
	}
	return nil
}

func (s *productManualFileService) buildManualFileDTO(item models.ProductManualFile) dto.EnterpriseProductManualFileDTO {
	if item.KnowledgeDocumentID > 0 {
		if document := repositories.KnowledgeDocumentRepository.Get(sqls.DB(), item.KnowledgeDocumentID); document != nil && document.TenantID == item.TenantID {
			item.SyncStatus = document.IndexStatus
			item.SyncError = document.IndexError
			item.SyncedAt = document.IndexedAt
		}
	}
	result := dto.EnterpriseProductManualFileDTO{
		ID:                  item.ID,
		ProductID:           item.ProductID,
		AssetID:             item.AssetID,
		Title:               item.Title,
		Language:            firstNonBlank(item.Language, "default"),
		Version:             firstNonBlank(item.Version, "v1.0"),
		Visibility:          normalizeProductManualVisibility(item.Visibility),
		KnowledgeBaseID:     item.KnowledgeBaseID,
		KnowledgeDocumentID: item.KnowledgeDocumentID,
		RAGSyncStatus:       string(item.SyncStatus),
		RAGSyncError:        strings.TrimSpace(item.SyncError),
		SyncedAt:            formatEnterpriseTimePtr(item.SyncedAt),
		Status:              enterpriseStatusText(item.Status),
		CreatedAt:           formatEnterpriseTime(item.CreatedAt),
		UpdatedAt:           formatEnterpriseTime(item.UpdatedAt),
		UploadedAt:          formatEnterpriseTime(item.CreatedAt),
		UploadedBy:          firstNonBlank(item.CreateUserName, "enterprise-api"),
	}
	asset := AssetService.Get(item.AssetID)
	if asset == nil {
		return result
	}
	result.AssetToken = asset.AssetID
	result.Filename = asset.Filename
	result.FileSize = asset.FileSize
	result.MimeType = asset.MimeType
	result.Provider = string(asset.Provider)
	result.UploadedAt = formatEnterpriseTime(asset.CreatedAt)
	result.UploadedBy = firstNonBlank(asset.CreateUserName, result.UploadedBy)
	if result.Title == "" {
		result.Title = asset.Filename
	}
	if url, err := AssetService.GetSignedURL(asset.ID); err == nil {
		result.URL = url
	}
	return result
}

func normalizeProductManualVisibility(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "internal") {
		return "internal"
	}
	return "public"
}
