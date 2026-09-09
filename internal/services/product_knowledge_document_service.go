package services

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"remotehelpdesk/internal/ai/rag"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

type publishedProductKnowledgeDocumentInput struct {
	TenantID          int64
	KnowledgeBaseID   int64
	Title             string
	Content           string
	Language          string
	SourceAssetID     int64
	SourceType        string
	SourceReferenceID int64
}

func (s *knowledgeDocumentService) ListEnterpriseProductDocuments(tenantID, productID int64, page, pageSize int) (*dto.EnterpriseListResponse[dto.EnterpriseProductKnowledgeDocumentDTO], error) {
	product, knowledgeBaseID, err := s.resolveProductKnowledgeBase(tenantID, productID)
	if err != nil {
		return nil, err
	}
	page, pageSize = normalizeEnterprisePage(page, pageSize)
	documentCnd := sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("knowledge_base_id", knowledgeBaseID).
		Where("(source_asset_id > 0 OR review_status = ?)", "published").
		Where("status <> ?", enums.StatusDeleted).
		Desc("updated_at").
		Desc("id").
		Page(page, pageSize)
	documents, paging := repositories.KnowledgeDocumentRepository.FindPageByCnd(sqls.DB(), documentCnd)
	if len(documents) == 0 {
		return enterprisePage([]dto.EnterpriseProductKnowledgeDocumentDTO{}, paging, page, pageSize), nil
	}
	documentIDs := make([]int64, 0, len(documents))
	for _, document := range documents {
		documentIDs = append(documentIDs, document.ID)
	}
	links := repositories.ProductKnowledgeLinkRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		In("knowledge_entry_id", documentIDs).
		Where("status <> ?", enums.StatusDeleted))
	linkByDocumentID := make(map[int64]models.ProductKnowledgeLink, len(links))
	for _, link := range links {
		linkByDocumentID[link.KnowledgeEntryID] = link
	}
	manuals := repositories.ProductManualFileRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		In("knowledge_document_id", documentIDs).
		Where("knowledge_document_id > 0").
		Where("status <> ?", enums.StatusDeleted))
	manualByDocumentID := make(map[int64]int64, len(manuals))
	for _, manual := range manuals {
		manualByDocumentID[manual.KnowledgeDocumentID] = manual.ID
	}
	result := make([]dto.EnterpriseProductKnowledgeDocumentDTO, 0, len(documents))
	for _, document := range documents {
		link := linkByDocumentID[document.ID]
		sourceType, sourceReferenceID := resolveProductKnowledgeDocumentSource(document, link, manualByDocumentID[document.ID])
		result = append(result, s.buildEnterpriseProductDocumentDTO(product.ID, document, link.ID, sourceType, sourceReferenceID))
	}
	return enterprisePage(result, paging, page, pageSize), nil
}

func (s *knowledgeDocumentService) UploadEnterpriseProductDocument(
	tenantID, productID int64,
	file *multipart.FileHeader,
	operator *dto.AuthPrincipal,
) (*dto.EnterpriseProductKnowledgeDocumentDTO, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	product, knowledgeBaseID, err := s.resolveProductKnowledgeBase(tenantID, productID)
	if err != nil {
		return nil, err
	}
	if err := TenantCommercialService.RequireKnowledgeDocumentUpload(tenantID, product.ID, operator, file.Size); err != nil {
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
		Language:          firstNonBlank(strings.TrimSpace(product.DefaultLocale), "default"),
		SourceAssetID:     asset.ID,
		SourceType:        "uploaded_document",
		SourceReferenceID: asset.ID,
	}, operator)
	if err != nil {
		_ = AssetService.DeleteAsset(asset.ID, operator)
		return nil, err
	}

	link, err := ProductKnowledgeLinkService.CreateLink(CreateProductKnowledgeLinkRequest{
		TenantID:         tenantID,
		ProductID:        product.ID,
		KnowledgeBaseID:  knowledgeBaseID,
		KnowledgeEntryID: document.ID,
		LinkType:         "uploaded_document",
		Language:         firstNonBlank(strings.TrimSpace(product.DefaultLocale), "default"),
		Visibility:       "public",
	}, operator)
	if err != nil {
		_ = s.DeleteKnowledgeDocument(document.ID)
		_ = AssetService.DeleteAsset(asset.ID, operator)
		return nil, err
	}
	if err := ProductKnowledgeLinkService.PublishLinkScoped(tenantID, product.ID, link.ID, operator); err != nil {
		_ = ProductKnowledgeLinkService.DeleteLinkScoped(tenantID, product.ID, link.ID, operator)
		_ = s.DeleteKnowledgeDocument(document.ID)
		_ = AssetService.DeleteAsset(asset.ID, operator)
		return nil, err
	}
	if _, err := ProductKnowledgeLinkService.ReindexLink(tenantID, product.ID, link.ID, operator); err != nil {
		slog.Warn("enqueue uploaded product knowledge document failed", "document_id", document.ID, "error", err)
		_ = repositories.KnowledgeDocumentRepository.Updates(sqls.DB(), document.ID, map[string]any{
			"index_status": enums.KnowledgeDocumentIndexStatusFailed,
			"index_error":  err.Error(),
			"indexed_at":   nil,
			"updated_at":   time.Now(),
		})
	}
	document = repositories.KnowledgeDocumentRepository.Get(sqls.DB(), document.ID)
	result := s.buildEnterpriseProductDocumentDTO(product.ID, *document, link.ID, "uploaded_document", asset.ID)
	return &result, nil
}

func (s *knowledgeDocumentService) ReprocessEnterpriseProductDocument(
	tenantID, productID, documentID int64,
	operator *dto.AuthPrincipal,
) (*dto.EnterpriseProductKnowledgeDocumentDTO, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	product, knowledgeBaseID, err := s.resolveProductKnowledgeBase(tenantID, productID)
	if err != nil {
		return nil, err
	}
	document := repositories.KnowledgeDocumentRepository.Get(sqls.DB(), documentID)
	if document == nil || document.TenantID != tenantID || document.KnowledgeBaseID != knowledgeBaseID || document.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParam("product knowledge document not found")
	}
	link := repositories.ProductKnowledgeLinkRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		Eq("knowledge_base_id", knowledgeBaseID).
		Eq("knowledge_entry_id", documentID).
		Where("status <> ?", enums.StatusDeleted))
	if link == nil {
		return nil, errorsx.InvalidParam("product knowledge link not found")
	}

	now := time.Now()
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		snapshot := *document
		snapshot.ReviewStatus = "published"
		revision, revisionErr := EnterpriseKnowledgeService.publishDocumentRevision(ctx.Tx, snapshot, now, operator)
		if revisionErr != nil {
			return revisionErr
		}
		if err := repositories.KnowledgeDocumentRepository.Updates(ctx.Tx, document.ID, map[string]any{
			"review_status":         "published",
			"status":                enums.StatusOk,
			"current_revision_id":   revision.ID,
			"published_revision_id": revision.ID,
			"index_status":          enums.KnowledgeDocumentIndexStatusPending,
			"index_error":           "",
			"indexed_at":            nil,
			"update_user_id":        operator.UserID,
			"update_user_name":      operator.Username,
			"updated_at":            now,
		}); err != nil {
			return err
		}
		return repositories.ProductKnowledgeLinkRepository.Updates(ctx.Tx, link.ID, map[string]any{
			"publish_status":   "published",
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
			"updated_at":       now,
		})
	}); err != nil {
		return nil, err
	}
	if _, err := ProductKnowledgeLinkService.ReindexLink(tenantID, productID, link.ID, operator); err != nil {
		_ = repositories.KnowledgeDocumentRepository.Updates(sqls.DB(), document.ID, map[string]any{
			"index_status": enums.KnowledgeDocumentIndexStatusFailed,
			"index_error":  err.Error(),
			"updated_at":   time.Now(),
		})
		_ = sqls.DB().Model(&models.ProductManualFile{}).
			Where("tenant_id = ? AND product_id = ? AND knowledge_document_id = ? AND status <> ?", tenantID, productID, document.ID, enums.StatusDeleted).
			Updates(map[string]any{
				"sync_status": enums.KnowledgeDocumentIndexStatusFailed,
				"sync_error":  err.Error(),
				"synced_at":   nil,
				"updated_at":  time.Now(),
			}).Error
		return nil, err
	}
	_ = sqls.DB().Model(&models.ProductManualFile{}).
		Where("tenant_id = ? AND product_id = ? AND knowledge_document_id = ? AND status <> ?", tenantID, productID, document.ID, enums.StatusDeleted).
		Updates(map[string]any{
			"sync_status": enums.KnowledgeDocumentIndexStatusPending,
			"sync_error":  "",
			"synced_at":   nil,
			"updated_at":  time.Now(),
		}).Error
	document = repositories.KnowledgeDocumentRepository.Get(sqls.DB(), document.ID)
	sourceType, sourceReferenceID := resolveProductKnowledgeDocumentSource(*document, *link, 0)
	result := s.buildEnterpriseProductDocumentDTO(product.ID, *document, link.ID, sourceType, sourceReferenceID)
	return &result, nil
}

func (s *knowledgeDocumentService) createPublishedProductKnowledgeDocument(
	input publishedProductKnowledgeDocumentInput,
	operator *dto.AuthPrincipal,
) (*models.KnowledgeDocument, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	knowledgeBase := repositories.KnowledgeBaseRepository.Get(sqls.DB(), input.KnowledgeBaseID)
	if knowledgeBase == nil || knowledgeBase.TenantID != input.TenantID || knowledgeBase.Status != enums.StatusOk {
		return nil, errorsx.InvalidParam("knowledge base not found for this tenant")
	}
	now := time.Now()
	document := &models.KnowledgeDocument{
		TenantID:          input.TenantID,
		KnowledgeBaseID:   input.KnowledgeBaseID,
		Title:             strings.TrimSpace(input.Title),
		ContentType:       enums.KnowledgeDocumentContentTypeMarkdown,
		Content:           strings.TrimSpace(input.Content),
		SourceAssetID:     input.SourceAssetID,
		SourceType:        strings.TrimSpace(input.SourceType),
		SourceReferenceID: input.SourceReferenceID,
		ReviewStatus:      "published",
		Language:          normalizeKnowledgeLanguage(input.Language),
		TagsJSON:          "[]",
		FaultCodesJSON:    "[]",
		Status:            enums.StatusOk,
		IndexStatus:       enums.KnowledgeDocumentIndexStatusPending,
		ContentHash:       knowledgeContentHash(input.Content),
		AuditFields:       utils.BuildAuditFields(operator),
	}
	document.CreatedAt = now
	document.UpdatedAt = now
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := repositories.KnowledgeDocumentRepository.Create(ctx.Tx, document); err != nil {
			return err
		}
		revision, err := EnterpriseKnowledgeService.publishDocumentRevision(ctx.Tx, *document, now, operator)
		if err != nil {
			return err
		}
		document.CurrentRevisionID = revision.ID
		document.PublishedRevisionID = revision.ID
		return repositories.KnowledgeDocumentRepository.Updates(ctx.Tx, document.ID, map[string]any{
			"current_revision_id":   revision.ID,
			"published_revision_id": revision.ID,
		})
	}); err != nil {
		return nil, err
	}
	return repositories.KnowledgeDocumentRepository.Get(sqls.DB(), document.ID), nil
}

func (s *knowledgeDocumentService) DeleteEnterpriseProductDocument(
	tenantID, productID, documentID int64,
	operator *dto.AuthPrincipal,
) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	_, knowledgeBaseID, err := s.resolveProductKnowledgeBase(tenantID, productID)
	if err != nil {
		return err
	}
	document := repositories.KnowledgeDocumentRepository.Get(sqls.DB(), documentID)
	if document == nil || document.TenantID != tenantID || document.KnowledgeBaseID != knowledgeBaseID || document.Status == enums.StatusDeleted || document.SourceAssetID <= 0 {
		return errorsx.InvalidParam("uploaded knowledge document not found")
	}
	links := repositories.ProductKnowledgeLinkRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		Eq("knowledge_entry_id", documentID).
		Where("status <> ?", enums.StatusDeleted))
	link := models.ProductKnowledgeLink{}
	if len(links) > 0 {
		link = links[0]
	}
	manual := repositories.ProductManualFileRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		Eq("knowledge_document_id", documentID).
		Where("status <> ?", enums.StatusDeleted))
	manualID := int64(0)
	if manual != nil {
		manualID = manual.ID
	}
	sourceType, _ := resolveProductKnowledgeDocumentSource(*document, link, manualID)
	if sourceType != "uploaded_document" {
		return errorsx.InvalidParam("only directly uploaded knowledge documents can be deleted here")
	}
	for _, link := range links {
		if err := ProductKnowledgeLinkService.DeleteLinkScoped(tenantID, productID, link.ID, operator); err != nil {
			return err
		}
	}
	if err := s.DeleteKnowledgeDocument(document.ID); err != nil {
		return err
	}
	return AssetService.DeleteAsset(document.SourceAssetID, operator)
}

func (s *knowledgeDocumentService) resolveProductKnowledgeBase(tenantID, productID int64) (*models.Product, int64, error) {
	product, err := ProductCenterService.requireTenantProduct(tenantID, productID)
	if err != nil {
		return nil, 0, err
	}
	profile := repositories.ProductServiceProfileRepository.GetByProductID(sqls.DB(), product.ID)
	if profile == nil || profile.TenantID != tenantID || profile.Status != enums.StatusOk || profile.DefaultKnowledgeBaseID <= 0 {
		return nil, 0, errorsx.InvalidParam("product knowledge base not found; create the knowledge base first")
	}
	knowledgeBase := repositories.KnowledgeBaseRepository.Get(sqls.DB(), profile.DefaultKnowledgeBaseID)
	if knowledgeBase == nil || knowledgeBase.TenantID != tenantID || knowledgeBase.Status != enums.StatusOk {
		return nil, 0, errorsx.InvalidParam("product knowledge base not found; create the knowledge base first")
	}
	return product, knowledgeBase.ID, nil
}

func (s *knowledgeDocumentService) buildEnterpriseProductDocumentDTO(
	productID int64,
	document models.KnowledgeDocument,
	knowledgeLinkID int64,
	sourceType string,
	sourceReferenceID int64,
) dto.EnterpriseProductKnowledgeDocumentDTO {
	result := dto.EnterpriseProductKnowledgeDocumentDTO{
		ID:                document.ID,
		KnowledgeEntryID:  encodeEnterpriseKnowledgeID("document", document.ID),
		ProductID:         productID,
		KnowledgeBaseID:   document.KnowledgeBaseID,
		KnowledgeLinkID:   knowledgeLinkID,
		SourceAssetID:     document.SourceAssetID,
		SourceType:        sourceType,
		SourceReferenceID: sourceReferenceID,
		Title:             document.Title,
		FileSize:          int64(len([]byte(document.Content))),
		MimeType:          knowledgeDocumentContentMimeType(document.ContentType),
		IndexStatus:       string(document.IndexStatus),
		ReviewStatus:      firstNonBlank(document.ReviewStatus, "draft"),
		Deletable:         sourceType == "uploaded_document",
		IndexError:        strings.TrimSpace(document.IndexError),
		IndexedAt:         formatEnterpriseTimePtr(document.IndexedAt),
		UploadedAt:        formatEnterpriseTime(document.CreatedAt),
		UploadedBy:        firstNonBlank(document.CreateUserName, "enterprise-api"),
		CreatedAt:         formatEnterpriseTime(document.CreatedAt),
		UpdatedAt:         formatEnterpriseTime(document.UpdatedAt),
	}
	if document.SourceAssetID <= 0 {
		return result
	}
	asset := AssetService.Get(document.SourceAssetID)
	if asset == nil {
		return result
	}
	result.Filename = asset.Filename
	result.FileSize = asset.FileSize
	result.MimeType = asset.MimeType
	result.Provider = string(asset.Provider)
	result.UploadedAt = formatEnterpriseTime(asset.CreatedAt)
	result.UploadedBy = firstNonBlank(asset.CreateUserName, result.UploadedBy)
	if url, err := AssetService.GetSignedURL(asset.ID); err == nil {
		result.URL = url
	}
	return result
}

func knowledgeDocumentContentMimeType(contentType enums.KnowledgeDocumentContentType) string {
	switch contentType {
	case enums.KnowledgeDocumentContentTypeMarkdown:
		return "text/markdown"
	case enums.KnowledgeDocumentContentTypeHTML:
		return "text/html"
	default:
		return "text/plain"
	}
}

func resolveProductKnowledgeDocumentSource(
	document models.KnowledgeDocument,
	link models.ProductKnowledgeLink,
	manualFileID int64,
) (string, int64) {
	if manualFileID > 0 || document.SourceType == "product_manual" || link.LinkType == "manual" {
		return "product_manual", firstPositiveInt64(manualFileID, document.SourceReferenceID)
	}
	if document.SourceAssetID > 0 || document.SourceType == "uploaded_document" || link.LinkType == "uploaded_document" {
		return "uploaded_document", firstPositiveInt64(document.SourceReferenceID, document.SourceAssetID)
	}
	return "knowledge_entry", firstPositiveInt64(document.SourceReferenceID, document.ID)
}

func firstPositiveInt64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func extractKnowledgeDocumentContentFromAsset(asset *models.Asset) (string, error) {
	reader, err := AssetService.OpenReader(asset)
	if err != nil {
		return "", err
	}
	defer func() { _ = reader.Close() }()
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", errorsx.InvalidParam("uploaded knowledge document is empty")
	}
	return extractKnowledgeDocumentContent(data, asset.Filename, asset.MimeType)
}

func extractKnowledgeDocumentContent(data []byte, filename, mimeType string) (string, error) {
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(filename)))
	mime := strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0]))
	var content string
	switch {
	case ext == ".docx" || mime == "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		content = extractPlainTextFromDOCX(data)
	case ext == ".md" || ext == ".markdown" || ext == ".txt":
		content = normalizeKnowledgeDocumentMarkdown(string(data))
	case ext == ".html" || ext == ".htm" || mime == "text/html":
		content = normalizeKnowledgeDocumentMarkdown(rag.ExtractPlainTextFromHTML(string(data)))
	default:
		return "", errorsx.InvalidParam("unsupported knowledge document type; currently supports .docx, .md, .txt and .html")
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return "", errorsx.InvalidParam("failed to extract readable content from uploaded knowledge document")
	}
	return content, nil
}

func extractPlainTextFromDOCX(data []byte) string {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return ""
	}
	var documentXML []byte
	for _, file := range reader.File {
		if file.Name != "word/document.xml" {
			continue
		}
		src, openErr := file.Open()
		if openErr != nil {
			return ""
		}
		documentXML, err = io.ReadAll(src)
		_ = src.Close()
		if err != nil {
			return ""
		}
		break
	}
	if len(documentXML) == 0 {
		return ""
	}
	decoder := xml.NewDecoder(bytes.NewReader(documentXML))
	paragraphs := make([]string, 0, 32)
	var current strings.Builder
	flush := func() {
		text := normalizeKnowledgeDocumentParagraph(current.String())
		current.Reset()
		if text != "" {
			paragraphs = append(paragraphs, text)
		}
	}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ""
		}
		switch typed := token.(type) {
		case xml.StartElement:
			switch typed.Name.Local {
			case "tab":
				current.WriteByte(' ')
			case "br", "cr":
				current.WriteByte('\n')
			}
		case xml.EndElement:
			if typed.Name.Local == "p" {
				flush()
			}
		case xml.CharData:
			current.WriteString(string(typed))
		}
	}
	flush()
	return strings.Join(paragraphs, "\n\n")
}

func normalizeKnowledgeDocumentParagraph(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(strings.TrimSpace(line)), " ")
		if line != "" {
			normalized = append(normalized, line)
		}
	}
	return strings.TrimSpace(strings.Join(normalized, "\n"))
}

func normalizeKnowledgeDocumentMarkdown(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	lines := strings.Split(value, "\n")
	normalized := make([]string, 0, len(lines))
	blankCount := 0
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			blankCount++
			if blankCount > 1 {
				continue
			}
			normalized = append(normalized, "")
			continue
		}
		blankCount = 0
		normalized = append(normalized, line)
	}
	return strings.TrimSpace(strings.Join(normalized, "\n"))
}
