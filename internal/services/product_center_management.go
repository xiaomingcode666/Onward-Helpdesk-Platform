package services

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

type productOwnerSummary struct {
	MemberID int64
	UserID   int64
	Name     string
}

// 产品中心企业端聚合查询（列表、详情、型号、模块、手册）。
// 查询与计数全部在 Service 层完成，Handler 不直接访问 Repository / sqls（§11）。

// ListEnterpriseProducts 企业端产品列表（含设备/工单计数与产品线名称）。
func (s *productCenterService) ListEnterpriseProducts(tenantID int64, page, limit int) ([]dto.EnterpriseProductListItemDTO, error) {
	return s.listEnterpriseProducts(tenantID, page, limit, enterpriseProductAccessScope{})
}

func (s *productCenterService) ListEnterpriseProductsForOperator(tenantID int64, page, limit int, operator *dto.AuthPrincipal) ([]dto.EnterpriseProductListItemDTO, error) {
	return s.listEnterpriseProducts(tenantID, page, limit, resolveEnterpriseProductAccessScope(tenantID, operator))
}

func (s *productCenterService) listEnterpriseProducts(tenantID int64, page, limit int, scope enterpriseProductAccessScope) ([]dto.EnterpriseProductListItemDTO, error) {
	if _, err := requireActiveTenant(tenantID); err != nil {
		return nil, err
	}
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	cnd := sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Where("status <> ?", enums.StatusDeleted)
	if scope.Restricted {
		if len(scope.ProductIDs) == 0 {
			return []dto.EnterpriseProductListItemDTO{}, nil
		}
		cnd.In("id", scope.ProductIDs)
	}
	cnd.Page(page, limit).Desc("id")
	list, _ := repositories.ProductRepository.FindPageByCnd(sqls.DB(), cnd)

	lineNames := s.productLineNames(tenantID, list)
	results := make([]dto.EnterpriseProductListItemDTO, 0, len(list))
	for i := range list {
		item := &list[i]
		owner := s.productOwnerSummary(tenantID, item.OwnerMemberID)
		results = append(results, dto.EnterpriseProductListItemDTO{
			ID:                 item.ID,
			Code:               item.Code,
			Name:               item.Name,
			ProductLine:        lineNames[item.ProductLineID],
			Status:             enterpriseStatusText(item.Status),
			Category:           item.Category,
			OwnerMemberID:      owner.MemberID,
			OwnerUserID:        owner.UserID,
			OwnerName:          owner.Name,
			DeviceCount:        s.productDeviceCount(tenantID, item.ID),
			TicketCount:        s.productTicketCount(tenantID, item.ID),
			ActiveSessionCount: s.productConversationCount(tenantID, item.ID),
			AIResolveRate:      0,
			DefaultLocale:      item.DefaultLocale,
			Description:        item.Description,
			CreatedAt:          formatEnterpriseTime(item.CreatedAt),
			UpdatedAt:          formatEnterpriseTime(item.UpdatedAt),
		})
	}
	return results, nil
}

// GetEnterpriseProductDetail 企业端产品详情。
func (s *productCenterService) GetEnterpriseProductDetail(tenantID, productID int64) (*dto.EnterpriseProductDetailDTO, error) {
	product, err := s.requireTenantProduct(tenantID, productID)
	if err != nil {
		return nil, err
	}
	lineName := ""
	if product.ProductLineID > 0 {
		if line := repositories.ProductLineRepository.Get(sqls.DB(), product.ProductLineID); line != nil && line.TenantID == tenantID {
			lineName = line.Name
		}
	}
	owner := s.productOwnerSummary(tenantID, product.OwnerMemberID)
	var profile *models.ProductServiceProfile
	if p := repositories.ProductServiceProfileRepository.GetByProductID(sqls.DB(), productID); p != nil {
		profile = p
	}
	detail := dto.EnterpriseProductDetailDTO{
		ID:            product.ID,
		Code:          product.Code,
		Name:          product.Name,
		ProductLine:   lineName,
		ProductLineID: product.ProductLineID,
		Status:        enterpriseStatusText(product.Status),
		Category:      product.Category,
		OwnerMemberID: owner.MemberID,
		OwnerUserID:   owner.UserID,
		OwnerName:     owner.Name,
		DefaultLocale: product.DefaultLocale,
		Description:   product.Description,
		DeviceCount:   s.productDeviceCount(tenantID, product.ID),
		TicketCount:   s.productTicketCount(tenantID, product.ID),
		CreatedAt:     formatEnterpriseTime(product.CreatedAt),
		UpdatedAt:     formatEnterpriseTime(product.UpdatedAt),
	}
	if profile != nil && profile.Status != enums.StatusDeleted {
		detail.MeetingEnabled = profile.MeetingEnabled
		detail.SafetyLevel = profile.SafetyLevel
		detail.KnowledgeBaseID = profile.DefaultKnowledgeBaseID
	}
	return &detail, nil
}

// ListEnterpriseProductModels 型号主数据列表（含设备/工单计数）。
func (s *productCenterService) ListEnterpriseProductModels(tenantID, productID int64) ([]dto.EnterpriseProductModelDTO, error) {
	if _, err := s.requireTenantProduct(tenantID, productID); err != nil {
		return nil, err
	}
	list := repositories.ProductModelRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		Where("status <> ?", enums.StatusDeleted).
		Asc("id"))
	results := make([]dto.EnterpriseProductModelDTO, 0, len(list))
	for i := range list {
		item := &list[i]
		deviceCount := repositories.DeviceRepository.Count(sqls.DB(), sqls.NewCnd().
			Eq("tenant_id", tenantID).Eq("product_model_id", item.ID).Where("status <> ?", enums.StatusDeleted))
		ticketCount := repositories.TicketRepository.Count(sqls.DB(), sqls.NewCnd().
			Eq("tenant_id", tenantID).Eq("product_model_id", item.ID))
		results = append(results, dto.EnterpriseProductModelDTO{
			ID:              item.ID,
			TenantID:        item.TenantID,
			ProductID:       item.ProductID,
			ModelCode:       item.ModelCode,
			Name:            item.Name,
			VersionPolicy:   item.VersionPolicy,
			RegionScopeJSON: item.RegionScopeJSON,
			Status:          enterpriseStatusText(item.Status),
			DeviceCount:     deviceCount,
			TicketCount:     ticketCount,
			CreatedAt:       formatEnterpriseTime(item.CreatedAt),
			UpdatedAt:       formatEnterpriseTime(item.UpdatedAt),
		})
	}
	return results, nil
}

// ListEnterpriseProductModules 模块列表（含适用型号）。
func (s *productCenterService) ListEnterpriseProductModules(tenantID, productID int64) ([]dto.EnterpriseProductModuleDTO, error) {
	if _, err := s.requireTenantProduct(tenantID, productID); err != nil {
		return nil, err
	}
	list := repositories.ProductModuleRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		Where("status <> ?", enums.StatusDeleted).
		Asc("id"))
	results := make([]dto.EnterpriseProductModuleDTO, 0, len(list))
	for i := range list {
		item := &list[i]
		modelIDs := ProductModuleService.ListModuleModelIDs(item.ID)
		modelNames := make([]string, 0, len(modelIDs))
		for _, modelID := range modelIDs {
			if model := repositories.ProductModelRepository.Get(sqls.DB(), modelID); model != nil {
				modelNames = append(modelNames, firstNonBlank(model.Name, model.ModelCode))
			}
		}
		moduleType := "component"
		if item.IsSafetyCritical {
			moduleType = "safety_critical"
		}
		results = append(results, dto.EnterpriseProductModuleDTO{
			ID:                item.ID,
			TenantID:          item.TenantID,
			ProductID:         item.ProductID,
			ModuleCode:        item.ModuleCode,
			Name:              item.Name,
			Type:              moduleType,
			DefaultSupplierID: item.DefaultSupplierID,
			IsSafetyCritical:  item.IsSafetyCritical,
			Status:            enterpriseStatusText(item.Status),
			ModelIDs:          modelIDs,
			ModelNames:        modelNames,
			CreatedAt:         formatEnterpriseTime(item.CreatedAt),
			UpdatedAt:         formatEnterpriseTime(item.UpdatedAt),
		})
	}
	return results, nil
}

// ListEnterpriseManuals 产品手册/知识挂载列表（含条目审核状态、索引状态与知识库名称）。
func (s *productCenterService) ListEnterpriseManuals(tenantID, productID int64, limit int) ([]dto.EnterpriseProductManualDTO, error) {
	if _, err := s.requireTenantProduct(tenantID, productID); err != nil {
		return nil, err
	}
	cnd := sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		Where("status <> ?", enums.StatusDeleted).
		Asc("sort_no").
		Desc("updated_at")
	if limit = normalizeProductKnowledgePreviewLimit(limit); limit > 0 {
		cnd.Limit(limit)
	}
	links := repositories.ProductKnowledgeLinkRepository.Find(sqls.DB(), cnd)
	results := make([]dto.EnterpriseProductManualDTO, 0, len(links))
	for _, link := range links {
		results = append(results, s.buildManualDTO(tenantID, link))
	}
	return results, nil
}

func normalizeProductKnowledgePreviewLimit(limit int) int {
	if limit <= 0 {
		return 0
	}
	if limit > 100 {
		return 100
	}
	return limit
}

// BuildManualDTO 对外暴露的手册 DTO 构建（供 Handler 在创建/更新后返回一致结构）。
func (s *productCenterService) BuildManualDTO(tenantID int64, link *models.ProductKnowledgeLink) dto.EnterpriseProductManualDTO {
	return s.buildManualDTO(tenantID, *link)
}

func (s *productCenterService) buildManualDTO(tenantID int64, link models.ProductKnowledgeLink) dto.EnterpriseProductManualDTO {
	item := dto.EnterpriseProductManualDTO{
		ID:               link.ID,
		ProductID:        link.ProductID,
		ProductModelID:   link.ProductModelID,
		KnowledgeBaseID:  link.KnowledgeBaseID,
		KnowledgeEntryID: link.KnowledgeEntryID,
		LinkType:         link.LinkType,
		Language:         link.Language,
		Version:          link.Version,
		Visibility:       link.Visibility,
		PublishStatus:    link.PublishStatus,
		SortNo:           link.SortNo,
		Status:           enterpriseStatusText(link.Status),
		CreatedAt:        formatEnterpriseTime(link.CreatedAt),
		UpdatedAt:        formatEnterpriseTime(link.UpdatedAt),
	}
	if link.ProductModelID > 0 {
		if model := repositories.ProductModelRepository.Get(sqls.DB(), link.ProductModelID); model != nil && model.TenantID == tenantID {
			item.ProductModelName = firstNonBlank(model.Name, model.ModelCode)
		}
	}
	if link.KnowledgeBaseID > 0 {
		if kb := repositories.KnowledgeBaseRepository.Get(sqls.DB(), link.KnowledgeBaseID); kb != nil && kb.TenantID == tenantID {
			item.KnowledgeBaseName = kb.Name
		}
	}
	item.EntryReviewStatus = s.entryReviewStatus(tenantID, link)
	item.IndexStatus = s.entryIndexStatus(tenantID, link)
	item.Title = buildKnowledgeLinkTitle(link)
	return item
}

func (s *productCenterService) entryReviewStatus(tenantID int64, link models.ProductKnowledgeLink) string {
	if link.KnowledgeEntryID <= 0 {
		return ""
	}
	if doc := repositories.KnowledgeDocumentRepository.Get(sqls.DB(), link.KnowledgeEntryID); doc != nil && doc.TenantID == tenantID && doc.KnowledgeBaseID == link.KnowledgeBaseID {
		return doc.ReviewStatus
	}
	if faq := repositories.KnowledgeFAQRepository.Get(sqls.DB(), link.KnowledgeEntryID); faq != nil && faq.TenantID == tenantID && faq.KnowledgeBaseID == link.KnowledgeBaseID {
		return faq.ReviewStatus
	}
	return ""
}

func (s *productCenterService) entryIndexStatus(tenantID int64, link models.ProductKnowledgeLink) string {
	if link.KnowledgeEntryID <= 0 {
		return ""
	}
	if doc := repositories.KnowledgeDocumentRepository.Get(sqls.DB(), link.KnowledgeEntryID); doc != nil && doc.TenantID == tenantID && doc.KnowledgeBaseID == link.KnowledgeBaseID {
		return string(doc.IndexStatus)
	}
	if faq := repositories.KnowledgeFAQRepository.Get(sqls.DB(), link.KnowledgeEntryID); faq != nil && faq.TenantID == tenantID && faq.KnowledgeBaseID == link.KnowledgeBaseID {
		return string(faq.IndexStatus)
	}
	return ""
}

// BuildProductListItem 构建单个产品列表项 DTO（创建/更新后返回一致结构）。
func (s *productCenterService) BuildProductListItem(tenantID, productID int64) (dto.EnterpriseProductListItemDTO, error) {
	product, err := s.requireTenantProduct(tenantID, productID)
	if err != nil {
		return dto.EnterpriseProductListItemDTO{}, err
	}
	lineName := ""
	if product.ProductLineID > 0 {
		if line := repositories.ProductLineRepository.GetByTenant(sqls.DB(), product.ProductLineID, tenantID); line != nil {
			lineName = line.Name
		}
	}
	owner := s.productOwnerSummary(tenantID, product.OwnerMemberID)
	return dto.EnterpriseProductListItemDTO{
		ID:                 product.ID,
		Code:               product.Code,
		Name:               product.Name,
		ProductLine:        lineName,
		Status:             enterpriseStatusText(product.Status),
		Category:           product.Category,
		OwnerMemberID:      owner.MemberID,
		OwnerUserID:        owner.UserID,
		OwnerName:          owner.Name,
		DeviceCount:        s.productDeviceCount(tenantID, product.ID),
		TicketCount:        s.productTicketCount(tenantID, product.ID),
		ActiveSessionCount: s.productConversationCount(tenantID, product.ID),
		AIResolveRate:      0,
		DefaultLocale:      product.DefaultLocale,
		Description:        product.Description,
		CreatedAt:          formatEnterpriseTime(product.CreatedAt),
		UpdatedAt:          formatEnterpriseTime(product.UpdatedAt),
	}, nil
}

func (s *productCenterService) productOwnerSummary(tenantID, ownerMemberID int64) productOwnerSummary {
	if tenantID <= 0 || ownerMemberID <= 0 {
		return productOwnerSummary{}
	}
	member := repositories.EnterpriseIAMRepository.GetTenantMember(sqls.DB(), tenantID, ownerMemberID)
	if member == nil {
		return productOwnerSummary{}
	}
	ret := productOwnerSummary{
		MemberID: member.ID,
		UserID:   member.UserID,
		Name:     member.DisplayName,
	}
	if user := repositories.UserRepository.Get(sqls.DB(), member.UserID); user != nil {
		if ret.Name == "" {
			ret.Name = user.Nickname
		}
		if ret.Name == "" {
			ret.Name = user.Username
		}
	}
	return ret
}

func (s *productCenterService) productLineNames(tenantID int64, products []models.Product) map[int64]string {
	names := map[int64]string{}
	for i := range products {
		lineID := products[i].ProductLineID
		if lineID <= 0 {
			continue
		}
		if _, ok := names[lineID]; ok {
			continue
		}
		if line := repositories.ProductLineRepository.GetByTenant(sqls.DB(), lineID, tenantID); line != nil {
			names[lineID] = line.Name
		}
	}
	return names
}

func (s *productCenterService) productDeviceCount(tenantID, productID int64) int64 {
	return repositories.DeviceRepository.Count(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).Eq("product_id", productID).Where("status <> ?", enums.StatusDeleted))
}

func (s *productCenterService) productTicketCount(tenantID, productID int64) int64 {
	return repositories.TicketRepository.Count(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).Eq("product_id", productID))
}

func (s *productCenterService) productConversationCount(tenantID, productID int64) int64 {
	return repositories.ConversationRepository.Count(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).Eq("product_id", productID))
}
