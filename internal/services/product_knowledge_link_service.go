package services

import (
	"fmt"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ProductKnowledgeLinkService = newProductKnowledgeLinkService()

func newProductKnowledgeLinkService() *productKnowledgeLinkService {
	return &productKnowledgeLinkService{}
}

type productKnowledgeLinkService struct{}

// CreateProductKnowledgeLinkRequest 创建知识挂载请求。
type CreateProductKnowledgeLinkRequest struct {
	TenantID         int64  `json:"tenantId"`
	ProductID        int64  `json:"productId"`
	ProductModelID   int64  `json:"productModelId"`
	KnowledgeBaseID  int64  `json:"knowledgeBaseId"`
	KnowledgeEntryID int64  `json:"knowledgeEntryId"`
	LinkType         string `json:"linkType"`
	Language         string `json:"language"`
	Version          string `json:"version"`
	Visibility       string `json:"visibility"`
	SortNo           int    `json:"sortNo"`
}

// UpdateProductKnowledgeLinkRequest 更新知识挂载请求。
type UpdateProductKnowledgeLinkRequest struct {
	ID               int64  `json:"id"`
	KnowledgeBaseID  int64  `json:"knowledgeBaseId"`
	KnowledgeEntryID int64  `json:"knowledgeEntryId"`
	LinkType         string `json:"linkType"`
	Language         string `json:"language"`
	Version          string `json:"version"`
	Visibility       string `json:"visibility"`
	SortNo           int    `json:"sortNo"`
}

func (s *productKnowledgeLinkService) Get(id int64) *models.ProductKnowledgeLink {
	if id <= 0 {
		return nil
	}
	return repositories.ProductKnowledgeLinkRepository.Get(sqls.DB(), id)
}

func (s *productKnowledgeLinkService) FindByProductID(productID int64) []models.ProductKnowledgeLink {
	return repositories.ProductKnowledgeLinkRepository.FindByProductID(sqls.DB(), productID)
}

func (s *productKnowledgeLinkService) FindByProductAndType(productID int64, linkType string) []models.ProductKnowledgeLink {
	return repositories.ProductKnowledgeLinkRepository.FindByProductAndType(sqls.DB(), productID, linkType)
}

func (s *productKnowledgeLinkService) FindPageByCnd(cnd *sqls.Cnd) ([]models.ProductKnowledgeLink, *sqls.Paging) {
	return repositories.ProductKnowledgeLinkRepository.FindPageByCnd(sqls.DB(), cnd)
}

// CreateLink 创建产品知识链接，Service 内执行完整绑定校验（§7.3）：
// 产品可用、型号归属、知识库归属、知识条目归属与类型兼容、语言版本、有效链接唯一。
func (s *productKnowledgeLinkService) CreateLink(req CreateProductKnowledgeLinkRequest, operator *dto.AuthPrincipal) (*models.ProductKnowledgeLink, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if operator.TenantID > 0 && operator.TenantID != req.TenantID {
		return nil, errorsx.InvalidParam("tenant does not match current operator")
	}
	product, model, err := s.validateReferences(req.TenantID, req.ProductID, req.ProductModelID)
	if err != nil {
		return nil, err
	}
	linkType := strings.TrimSpace(req.LinkType)
	if linkType == "" {
		linkType = "manual"
	}
	entryReviewStatus, err := s.validateKnowledgeEntry(req.TenantID, req.KnowledgeBaseID, req.KnowledgeEntryID, linkType)
	if err != nil {
		return nil, err
	}
	language := strings.TrimSpace(req.Language)
	version := strings.TrimSpace(req.Version)
	if dup := s.findActiveDuplicate(req.TenantID, product.ID, req.ProductModelID, req.KnowledgeEntryID, linkType, language, version, 0); dup != nil {
		return nil, errorsx.InvalidParam("an active knowledge link with the same entry, type, language and version already exists")
	}

	item := &models.ProductKnowledgeLink{
		TenantID:         product.TenantID,
		ProductID:        product.ID,
		ProductModelID:   0,
		KnowledgeBaseID:  req.KnowledgeBaseID,
		KnowledgeEntryID: req.KnowledgeEntryID,
		LinkType:         linkType,
		Language:         language,
		Version:          version,
		Visibility:       normalizeVisibility(req.Visibility),
		SortNo:           req.SortNo,
		PublishStatus:    "draft",
		Status:           enums.StatusOk,
		AuditFields:      utils.BuildAuditFields(operator),
	}
	if model != nil {
		item.ProductModelID = model.ID
	}
	_ = entryReviewStatus
	if err := repositories.ProductKnowledgeLinkRepository.Create(sqls.DB(), item); err != nil {
		return nil, err
	}
	return item, nil
}

// validateKnowledgeEntry 校验知识库与知识条目归属、类型兼容，返回条目审核状态。
func (s *productKnowledgeLinkService) validateKnowledgeEntry(tenantID, knowledgeBaseID, knowledgeEntryID int64, linkType string) (string, error) {
	if knowledgeBaseID <= 0 {
		return "", errorsx.InvalidParam("knowledgeBaseId is required")
	}
	kb := repositories.KnowledgeBaseRepository.Get(sqls.DB(), knowledgeBaseID)
	if kb == nil || kb.TenantID != tenantID || kb.Status == enums.StatusDeleted {
		return "", errorsx.InvalidParam("knowledge base not found for this tenant")
	}
	if knowledgeEntryID <= 0 {
		return "", errorsx.InvalidParam("knowledgeEntryId is required; use the knowledge entry selector instead of free input")
	}
	switch kb.KnowledgeType {
	case "faq":
		faq := repositories.KnowledgeFAQRepository.Get(sqls.DB(), knowledgeEntryID)
		if faq == nil || faq.TenantID != tenantID || faq.KnowledgeBaseID != knowledgeBaseID || faq.Status == enums.StatusDeleted {
			return "", errorsx.InvalidParam("knowledge entry not found in this knowledge base")
		}
		if linkType != "" && linkType != "knowledge_article" && linkType != "faq" {
			return "", errorsx.InvalidParam("link type is not compatible with a FAQ knowledge base")
		}
		return faq.ReviewStatus, nil
	default:
		doc := repositories.KnowledgeDocumentRepository.Get(sqls.DB(), knowledgeEntryID)
		if doc == nil || doc.TenantID != tenantID || doc.KnowledgeBaseID != knowledgeBaseID || doc.Status == enums.StatusDeleted {
			return "", errorsx.InvalidParam("knowledge entry not found in this knowledge base")
		}
		if linkType == "faq" {
			return "", errorsx.InvalidParam("link type faq is not compatible with a document knowledge base")
		}
		return doc.ReviewStatus, nil
	}
}

// findActiveDuplicate 查找相同维度下的有效链接（软删除记录不参与唯一约束，§5.1）。
func (s *productKnowledgeLinkService) findActiveDuplicate(tenantID, productID, modelID, entryID int64, linkType, language, version string, excludeID int64) *models.ProductKnowledgeLink {
	cnd := sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		Eq("product_model_id", modelID).
		Eq("knowledge_entry_id", entryID).
		Eq("link_type", linkType).
		Eq("language", language).
		Eq("version", version).
		Where("status <> ?", enums.StatusDeleted)
	if excludeID > 0 {
		cnd.NotEq("id", excludeID)
	}
	return repositories.ProductKnowledgeLinkRepository.FindOne(sqls.DB(), cnd)
}

func normalizeVisibility(value string) string {
	switch strings.TrimSpace(value) {
	case "public", "internal", "team":
		return strings.TrimSpace(value)
	default:
		return "internal"
	}
}

// requireTenantLink 校验链接属于指定租户和产品且未删除。
func (s *productKnowledgeLinkService) requireTenantLink(tenantID, productID, linkID int64) (*models.ProductKnowledgeLink, error) {
	item := s.Get(linkID)
	if item == nil || item.Status == enums.StatusDeleted || item.TenantID != tenantID || item.ProductID != productID {
		return nil, errorsx.InvalidParam("product knowledge link not found")
	}
	return item, nil
}

func (s *productKnowledgeLinkService) UpdateLink(req UpdateProductKnowledgeLinkRequest, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item := s.Get(req.ID)
	if item == nil || item.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("product knowledge link not found")
	}
	if operator.TenantID > 0 && item.TenantID != operator.TenantID {
		return errorsx.InvalidParam("product knowledge link not found")
	}
	linkType := strings.TrimSpace(req.LinkType)
	if linkType == "" {
		linkType = "manual"
	}
	if _, err := s.validateKnowledgeEntry(item.TenantID, req.KnowledgeBaseID, req.KnowledgeEntryID, linkType); err != nil {
		return err
	}
	language := strings.TrimSpace(req.Language)
	version := strings.TrimSpace(req.Version)
	if duplicate := s.findActiveDuplicate(item.TenantID, item.ProductID, item.ProductModelID, req.KnowledgeEntryID, linkType, language, version, item.ID); duplicate != nil {
		return errorsx.InvalidParam("an active knowledge link with the same entry, type, language and version already exists")
	}
	if item.PublishStatus == "published" {
		if err := s.markLinkEntryIndexPendingDB(sqls.DB(), item); err != nil {
			return err
		}
	}
	if err := repositories.ProductKnowledgeLinkRepository.Updates(sqls.DB(), req.ID, map[string]any{
		"knowledge_base_id":  req.KnowledgeBaseID,
		"knowledge_entry_id": req.KnowledgeEntryID,
		"link_type":          linkType,
		"language":           language,
		"version":            version,
		"visibility":         normalizeVisibility(req.Visibility),
		"sort_no":            req.SortNo,
		"publish_status":     "draft",
		"update_user_id":     operator.UserID,
		"update_user_name":   operator.Username,
		"updated_at":         time.Now(),
	}); err != nil {
		return err
	}
	if item.PublishStatus == "published" {
		s.enqueueReindex(item, "scope_change_previous", operator)
	}
	return nil
}

// UpdateLinkPartial PATCH 语义更新：指针为 nil 的字段保持原值。
func (s *productKnowledgeLinkService) UpdateLinkPartial(tenantID, productID, linkID int64, productModelID *int64, language, version, visibility *string, sortNo *int, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item, err := s.requireTenantLink(tenantID, productID, linkID)
	if err != nil {
		return err
	}
	updates := map[string]any{
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	}
	if productModelID != nil {
		if *productModelID > 0 {
			model := repositories.ProductModelRepository.Get(sqls.DB(), *productModelID)
			if model == nil || model.TenantID != tenantID || model.ProductID != productID || model.Status == enums.StatusDeleted {
				return errorsx.InvalidParam("product model not found or does not belong to this product")
			}
		}
		updates["product_model_id"] = *productModelID
	}
	if language != nil {
		updates["language"] = strings.TrimSpace(*language)
	}
	if version != nil {
		updates["version"] = strings.TrimSpace(*version)
	}
	if visibility != nil {
		updates["visibility"] = normalizeVisibility(*visibility)
	}
	if sortNo != nil {
		updates["sort_no"] = *sortNo
	}
	if item.PublishStatus == "published" {
		if err := s.markLinkEntryIndexPendingDB(sqls.DB(), item); err != nil {
			return err
		}
	}
	if err := repositories.ProductKnowledgeLinkRepository.Updates(sqls.DB(), linkID, updates); err != nil {
		return err
	}
	if item.PublishStatus == "published" {
		s.enqueueReindex(item, "scope_change", operator)
	}
	return nil
}

func (s *productKnowledgeLinkService) DeleteLink(id int64, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item := s.Get(id)
	if item == nil || item.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("product knowledge link not found")
	}
	if operator.TenantID > 0 && item.TenantID != operator.TenantID {
		return errorsx.InvalidParam("product knowledge link not found")
	}
	if item.PublishStatus == "published" {
		if err := s.markLinkEntryIndexPendingDB(sqls.DB(), item); err != nil {
			return err
		}
	}
	if err := repositories.ProductKnowledgeLinkRepository.Updates(sqls.DB(), id, map[string]any{
		"status":           enums.StatusDeleted,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	}); err != nil {
		return err
	}
	if item.PublishStatus == "published" {
		s.enqueueReindex(item, "scope_change", operator)
	}
	return nil
}

// DeleteLinkScoped 租户+产品作用域内解除手册关联。
func (s *productKnowledgeLinkService) DeleteLinkScoped(tenantID, productID, linkID int64, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item, err := s.requireTenantLink(tenantID, productID, linkID)
	if err != nil {
		return err
	}
	if item.PublishStatus == "published" {
		if err := s.markLinkEntryIndexPendingDB(sqls.DB(), item); err != nil {
			return err
		}
	}
	if err := repositories.ProductKnowledgeLinkRepository.Updates(sqls.DB(), linkID, map[string]any{
		"status":           enums.StatusDeleted,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	}); err != nil {
		return err
	}
	if item.PublishStatus == "published" {
		s.enqueueReindex(item, "scope_change", operator)
	}
	return nil
}

// PublishLinkScoped 发布手册链接（状态流转 draft/review -> published，§7.1）。
func (s *productKnowledgeLinkService) PublishLinkScoped(tenantID, productID, linkID int64, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	snapshot, err := s.requireTenantLink(tenantID, productID, linkID)
	if err != nil {
		return err
	}
	var item *models.ProductKnowledgeLink
	changed := false
	if err := sqls.DB().Transaction(func(tx *gorm.DB) error {
		// Entry lifecycle transitions lock the entry before synchronizing links;
		// use the same order here to avoid a publish/edit deadlock.
		if err := s.requirePublishedKnowledgeEntryForLinkTx(tx, snapshot); err != nil {
			return err
		}
		locked := &models.ProductKnowledgeLink{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND tenant_id = ? AND product_id = ? AND status <> ?", linkID, tenantID, productID, enums.StatusDeleted).
			First(locked).Error; err != nil {
			return errorsx.InvalidParam("product knowledge link not found")
		}
		if locked.KnowledgeBaseID != snapshot.KnowledgeBaseID || locked.KnowledgeEntryID != snapshot.KnowledgeEntryID || locked.LinkType != snapshot.LinkType {
			return errorsx.InvalidParam("product knowledge link changed while publishing; retry the operation")
		}
		switch locked.PublishStatus {
		case "published":
			item = locked
			return nil
		case "deprecated":
			return errorsx.InvalidParam("deprecated link must be revised to draft before publishing")
		}
		if err := s.markLinkEntryIndexPendingDB(tx, locked); err != nil {
			return err
		}
		if err := repositories.ProductKnowledgeLinkRepository.Updates(tx, linkID, map[string]any{
			"publish_status":   "published",
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
			"updated_at":       time.Now(),
		}); err != nil {
			return err
		}
		locked.PublishStatus = "published"
		item = locked
		changed = true
		return nil
	}); err != nil {
		return err
	}
	if changed {
		// 发布后触发索引同步（P3 Provider 实际消费任务）。
		s.enqueueReindex(item, "publish", operator)
	}
	return nil
}

func (s *productKnowledgeLinkService) requirePublishedKnowledgeEntryForLinkTx(tx *gorm.DB, link *models.ProductKnowledgeLink) error {
	if tx == nil || link == nil {
		return errorsx.InvalidParam("product knowledge link not found")
	}
	knowledgeBase := &models.KnowledgeBase{}
	if err := tx.Where("id = ? AND tenant_id = ?", link.KnowledgeBaseID, link.TenantID).First(knowledgeBase).Error; err != nil || knowledgeBase.Status != enums.StatusOk {
		return errorsx.InvalidParam("knowledge base not found for this tenant")
	}
	if knowledgeBase.KnowledgeType == string(enums.KnowledgeBaseTypeFAQ) {
		if link.LinkType != "" && link.LinkType != "knowledge_article" && link.LinkType != "faq" {
			return errorsx.InvalidParam("link type is not compatible with a FAQ knowledge base")
		}
		faq := &models.KnowledgeFAQ{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND tenant_id = ? AND knowledge_base_id = ?", link.KnowledgeEntryID, link.TenantID, link.KnowledgeBaseID).
			First(faq).Error; err != nil || faq.Status != enums.StatusOk || faq.ReviewStatus != "published" || faq.PublishedRevisionID <= 0 {
			return errorsx.InvalidParam("publish the knowledge entry revision before publishing its product link")
		}
		if repositories.KnowledgeRevisionRepository.FindPublishedForEntry(
			tx, faq.PublishedRevisionID, link.TenantID, link.KnowledgeBaseID, "faq", faq.ID,
		) == nil {
			return errorsx.InvalidParam("published knowledge revision is missing or does not belong to this entry")
		}
		return nil
	}
	if link.LinkType == "faq" {
		return errorsx.InvalidParam("link type faq is not compatible with a document knowledge base")
	}
	document := &models.KnowledgeDocument{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND tenant_id = ? AND knowledge_base_id = ?", link.KnowledgeEntryID, link.TenantID, link.KnowledgeBaseID).
		First(document).Error; err != nil || document.Status != enums.StatusOk || document.ReviewStatus != "published" || document.PublishedRevisionID <= 0 {
		return errorsx.InvalidParam("publish the knowledge entry revision before publishing its product link")
	}
	if repositories.KnowledgeRevisionRepository.FindPublishedForEntry(
		tx, document.PublishedRevisionID, link.TenantID, link.KnowledgeBaseID, "document", document.ID,
	) == nil {
		return errorsx.InvalidParam("published knowledge revision is missing or does not belong to this entry")
	}
	return nil
}

func (s *productKnowledgeLinkService) markLinkEntryIndexPendingDB(db *gorm.DB, link *models.ProductKnowledgeLink) error {
	if db == nil || link == nil {
		return errorsx.InvalidParam("product knowledge link not found")
	}
	updates := map[string]any{
		"index_status": enums.KnowledgeDocumentIndexStatusPending,
		"indexed_at":   nil,
		"index_error":  "",
		"updated_at":   time.Now(),
	}
	knowledgeBase := repositories.KnowledgeBaseRepository.Get(db, link.KnowledgeBaseID)
	if knowledgeBase == nil || knowledgeBase.TenantID != link.TenantID {
		return errorsx.InvalidParam("knowledge base not found for this tenant")
	}
	model := any(&models.KnowledgeDocument{})
	if knowledgeBase.KnowledgeType == string(enums.KnowledgeBaseTypeFAQ) {
		model = &models.KnowledgeFAQ{}
	}
	result := db.Model(model).
		Where("id = ? AND tenant_id = ? AND knowledge_base_id = ? AND status <> ?", link.KnowledgeEntryID, link.TenantID, link.KnowledgeBaseID, enums.StatusDeleted).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errorsx.InvalidParam("knowledge entry not found in this knowledge base")
	}
	return nil
}

// DeprecateLink 下架手册链接（published -> deprecated，§7.1）。
func (s *productKnowledgeLinkService) DeprecateLink(tenantID, productID, linkID int64, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item, err := s.requireTenantLink(tenantID, productID, linkID)
	if err != nil {
		return err
	}
	if item.PublishStatus == "deprecated" {
		return nil
	}
	if item.PublishStatus == "published" {
		if err := s.markLinkEntryIndexPendingDB(sqls.DB(), item); err != nil {
			return err
		}
	}
	if err := repositories.ProductKnowledgeLinkRepository.Updates(sqls.DB(), linkID, map[string]any{
		"publish_status":   "deprecated",
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	}); err != nil {
		return err
	}
	if item.PublishStatus == "published" {
		s.enqueueReindex(item, "deprecate", operator)
	}
	return nil
}

// ReindexLink 手工触发手册重索引（§10.4），生成索引同步任务。
func (s *productKnowledgeLinkService) ReindexLink(tenantID, productID, linkID int64, operator *dto.AuthPrincipal) (*models.KnowledgeIndexSyncTask, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item, err := s.requireTenantLink(tenantID, productID, linkID)
	if err != nil {
		return nil, err
	}
	if item.PublishStatus != "published" {
		return nil, errorsx.InvalidParam("publish the product knowledge link before reindexing it")
	}
	if err := sqls.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.requirePublishedKnowledgeEntryForLinkTx(tx, item); err != nil {
			return err
		}
		return s.markLinkEntryIndexPendingDB(tx, item)
	}); err != nil {
		return nil, err
	}
	task, err := KnowledgeIndexSyncService.EnqueueLinkSync(item, "upsert", "manual", operator)
	if err != nil {
		return nil, err
	}
	if task != nil && (task.Status == "waiting_retry" || task.Status == "failed" || task.Status == "cancelled" || task.Status == "succeeded") {
		return KnowledgeIndexSyncService.RetryTask(tenantID, task.ID, operator)
	}
	return task, nil
}

// enqueueReindex 生成索引同步任务；失败不阻塞业务主流程，仅记录日志由调用方感知空任务。
func (s *productKnowledgeLinkService) enqueueReindex(link *models.ProductKnowledgeLink, trigger string, operator *dto.AuthPrincipal) *models.KnowledgeIndexSyncTask {
	task, err := KnowledgeIndexSyncService.EnqueueLinkSync(link, "upsert", trigger, operator)
	if err != nil {
		return nil
	}
	return task
}

// PublishLink 兼容旧调用：发布链接。
func (s *productKnowledgeLinkService) PublishLink(id int64, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item := s.Get(id)
	if item == nil || item.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("product knowledge link not found")
	}
	if operator.TenantID > 0 && item.TenantID != operator.TenantID {
		return errorsx.InvalidParam("product knowledge link not found")
	}
	return s.PublishLinkScoped(item.TenantID, item.ProductID, item.ID, operator)
}

func (s *productKnowledgeLinkService) ArchiveLink(id int64, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item := s.Get(id)
	if item == nil || item.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("product knowledge link not found")
	}
	if operator.TenantID > 0 && item.TenantID != operator.TenantID {
		return errorsx.InvalidParam("product knowledge link not found")
	}
	if item.PublishStatus == "published" {
		if err := s.markLinkEntryIndexPendingDB(sqls.DB(), item); err != nil {
			return err
		}
	}
	if err := repositories.ProductKnowledgeLinkRepository.Updates(sqls.DB(), id, map[string]any{
		"publish_status":   "archived",
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	}); err != nil {
		return err
	}
	if item.PublishStatus == "published" {
		s.enqueueReindex(item, "scope_change", operator)
	}
	return nil
}

func (s *productKnowledgeLinkService) FindPublishedByProduct(productID int64) []models.ProductKnowledgeLink {
	return repositories.ProductKnowledgeLinkRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("product_id", productID).
		Eq("publish_status", "published").
		Eq("status", enums.StatusOk))
}

func (s *productKnowledgeLinkService) validateReferences(tenantID, productID, modelID int64) (*models.Product, *models.ProductModel, error) {
	if _, err := requireActiveTenant(tenantID); err != nil {
		return nil, nil, err
	}
	product := repositories.ProductRepository.Get(sqls.DB(), productID)
	if product == nil || product.Status != enums.StatusOk || product.TenantID != tenantID {
		return nil, nil, errorsx.InvalidParam("product not found or disabled")
	}
	if modelID <= 0 {
		return product, nil, nil
	}
	model := repositories.ProductModelRepository.Get(sqls.DB(), modelID)
	if model == nil || model.Status != enums.StatusOk || model.TenantID != tenantID || model.ProductID != product.ID {
		return nil, nil, errorsx.InvalidParam("product model not found or does not belong to product")
	}
	return product, model, nil
}

// knowledgeLinkIDString 供日志使用。
func knowledgeLinkIDString(id int64) string {
	return fmt.Sprintf("link:%d", id)
}
