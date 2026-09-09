package services

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/servicecode"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var ProductCenterService = newProductCenterService()

func newProductCenterService() *productCenterService {
	return &productCenterService{}
}

type productCenterService struct{}

const productKnowledgeCoveragePreviewLimit = 10

type CreateProductQualitySignalRequest struct {
	TenantID         int64
	ProductID        int64
	ProductModelID   int64
	SignalType       string
	Severity         string
	Title            string
	Description      string
	TriggerCondition string
	MetricValue      float64
	SampleCount      int64
	Source           string
	SourceType       string
	SourceID         string
	OwnerUserID      int64
	DetectedAt       time.Time
}

type UpdateProductQualitySignalRequest struct {
	TenantID         int64
	ProductID        int64
	SignalID         int64
	ProductModelID   *int64
	SignalType       *string
	Severity         *string
	Title            *string
	Description      *string
	TriggerCondition *string
	MetricValue      *float64
	SampleCount      *int64
	Source           *string
	SourceType       *string
	SourceID         *string
	OwnerUserID      *int64
}

func enterpriseStatusText(status enums.Status) string {
	switch status {
	case enums.StatusOk:
		return "active"
	case enums.StatusDeleted:
		return "deleted"
	default:
		return "inactive"
	}
}

func formatEnterpriseTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}

func formatEnterpriseTimePtr(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}

func MapTicketStatusForEnterprise(status enums.TicketStatus) string {
	return string(enums.NormalizeTicketStatus(string(status)))
}

func DeriveTicketPriority(item models.Ticket) string {
	if ticketSLACompleted(item.Status) {
		return "low"
	}
	switch strings.ToLower(strings.TrimSpace(item.PriorityCode)) {
	case "p0":
		return "critical"
	case "p1":
		return "high"
	case "p2":
		return "medium"
	case "p3", "p4":
		return "low"
	}
	if item.SLADueAt != nil && time.Now().After(*item.SLADueAt) {
		return "critical"
	}
	if item.Status == enums.TicketStatusEscalated {
		return "high"
	}
	if item.SLADueAt != nil && time.Until(*item.SLADueAt) <= 24*time.Hour {
		return "medium"
	}
	return "low"
}

func ticketSLACompleted(status enums.TicketStatus) bool {
	switch enums.NormalizeTicketStatus(string(status)) {
	case enums.TicketStatusResolved,
		enums.TicketStatusPendingCustomerConfirm,
		enums.TicketStatusClosed,
		enums.TicketStatusCancelled:
		return true
	default:
		return false
	}
}

func buildKnowledgeLinkTitle(link models.ProductKnowledgeLink) string {
	if title := resolveKnowledgeEntryTitle(link); title != "" {
		return title
	}
	switch {
	case link.LinkType != "" && link.Language != "":
		return link.LinkType + " - " + link.Language
	case link.LinkType != "":
		return link.LinkType
	case link.Language != "":
		return link.Language
	default:
		return "Knowledge Link"
	}
}

func resolveKnowledgeEntryTitle(link models.ProductKnowledgeLink) string {
	if link.KnowledgeEntryID <= 0 {
		return ""
	}
	if doc := repositories.KnowledgeDocumentRepository.Get(sqls.DB(), link.KnowledgeEntryID); doc != nil &&
		doc.TenantID == link.TenantID && doc.KnowledgeBaseID == link.KnowledgeBaseID && doc.Status != enums.StatusDeleted {
		return strings.TrimSpace(doc.Title)
	}
	if faq := repositories.KnowledgeFAQRepository.Get(sqls.DB(), link.KnowledgeEntryID); faq != nil &&
		faq.TenantID == link.TenantID && faq.KnowledgeBaseID == link.KnowledgeBaseID && faq.Status != enums.StatusDeleted {
		return strings.TrimSpace(faq.Question)
	}
	return ""
}

func (s *productCenterService) requireTenantProduct(tenantID, productID int64) (*models.Product, error) {
	if tenantID <= 0 || productID <= 0 {
		return nil, errorsx.InvalidParam("tenantId and productId are required")
	}
	product := repositories.ProductRepository.GetByTenant(sqls.DB(), productID, tenantID)
	if product == nil || product.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParam("product not found")
	}
	return product, nil
}

func (s *productCenterService) RequireTenantProduct(tenantID, productID int64) (*models.Product, error) {
	return s.requireTenantProduct(tenantID, productID)
}

func (s *productCenterService) GetEnterpriseProductProfile(tenantID, productID int64) (*dto.EnterpriseProductProfileDTO, error) {
	product, err := s.requireTenantProduct(tenantID, productID)
	if err != nil {
		return nil, err
	}
	productCenter, _ := s.GetProductProfile(tenantID, productID)
	deviceCount := repositories.DeviceRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID).NotEq("status", enums.StatusDeleted))
	ticketCount := repositories.TicketRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID))
	conversationCount := repositories.ConversationRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID))
	knowledgeCount := repositories.ProductKnowledgeLinkRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID).NotEq("status", enums.StatusDeleted))
	owner := s.productOwnerSummary(tenantID, product.OwnerMemberID)

	result := &dto.EnterpriseProductProfileDTO{
		Product: dto.EnterpriseProductListItemDTO{
			ID:                 product.ID,
			Code:               product.Code,
			Name:               product.Name,
			ProductLine:        "",
			Status:             enterpriseStatusText(product.Status),
			Category:           product.Category,
			OwnerMemberID:      owner.MemberID,
			OwnerUserID:        owner.UserID,
			OwnerName:          owner.Name,
			DeviceCount:        deviceCount,
			TicketCount:        ticketCount,
			ActiveSessionCount: conversationCount,
			AIResolveRate:      0,
			DefaultLocale:      product.DefaultLocale,
			Description:        product.Description,
			CreatedAt:          formatEnterpriseTime(product.CreatedAt),
			UpdatedAt:          formatEnterpriseTime(product.UpdatedAt),
		},
		ServiceProfile: dto.EnterpriseProductServiceProfileDTO{
			ProductID:      product.ID,
			SupportLocales: []string{},
			SupportRegions: []string{},
			WarrantyPolicy: map[string]any{},
			Status:         "active",
		},
		TotalTicketCount:         ticketCount,
		TotalKnowledgeEntryCount: knowledgeCount,
		ProductCenter:            productCenter,
	}
	if product.ProductLineID > 0 {
		if line := repositories.ProductLineRepository.Get(sqls.DB(), product.ProductLineID); line != nil {
			result.Product.ProductLine = line.Name
		}
	}
	if profile := repositories.ProductServiceProfileRepository.GetByProductID(sqls.DB(), productID); profile != nil && profile.Status != enums.StatusDeleted {
		result.ServiceProfile.ProductID = productID
		result.ServiceProfile.DefaultAIAgentID = profile.DefaultAIAgentID
		result.ServiceProfile.SafetyLevel = profile.SafetyLevel
		result.ServiceProfile.MeetingEnabled = profile.MeetingEnabled
		result.ServiceProfile.Status = enterpriseStatusText(profile.Status)
		if profile.DefaultKnowledgeBaseID > 0 {
			kbID := profile.DefaultKnowledgeBaseID
			result.ServiceProfile.KnowledgeBaseID = &kbID
			result.ServiceProfile.RagflowDatasetID = s.ragflowDatasetForKnowledgeBase(tenantID, kbID, nil)
			if kb := repositories.KnowledgeBaseRepository.Get(sqls.DB(), kbID); kb != nil && kb.TenantID == tenantID && kb.Status != enums.StatusDeleted {
				result.ServiceProfile.KnowledgeBaseName = strings.TrimSpace(kb.Name)
				result.ServiceProfile.KnowledgeBaseDescription = strings.TrimSpace(kb.Description)
			}
		}
		_ = json.Unmarshal([]byte(profile.SupportLocalesJSON), &result.ServiceProfile.SupportLocales)
		_ = json.Unmarshal([]byte(profile.SupportRegionsJSON), &result.ServiceProfile.SupportRegions)
		_ = json.Unmarshal([]byte(profile.WarrantyPolicyJSON), &result.ServiceProfile.WarrantyPolicy)
	}
	return result, nil
}

func (s *productCenterService) CreateProductKnowledgeBase(
	tenantID, productID int64,
	req dto.EnterpriseProductKnowledgeBaseCreateRequest,
	operator *dto.AuthPrincipal,
) (*dto.EnterpriseProductProfileDTO, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	product, err := s.requireTenantProduct(tenantID, productID)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errorsx.InvalidParam("knowledge base name is required")
	}
	description := strings.TrimSpace(req.Description)
	ragflowDatasetID := strings.TrimSpace(req.RagflowDatasetID)
	supportLocalesJSON, err := json.Marshal(normalizeEnterpriseStringList(req.SupportLocales))
	if err != nil {
		return nil, err
	}
	supportRegionsJSON, err := json.Marshal(normalizeEnterpriseStringList(req.SupportRegions))
	if err != nil {
		return nil, err
	}

	existingKnowledgeBase := repositories.KnowledgeBaseRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("name", name).
		Eq("knowledge_type", string(enums.KnowledgeBaseTypeDocument)).
		Eq("access_scope", string(enums.KnowledgeBaseAccessScopeProduct)).
		NotEq("status", enums.StatusDeleted))

	now := time.Now()
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		profile := repositories.ProductServiceProfileRepository.GetByProductID(ctx.Tx, productID)
		if profile != nil && profile.Status != enums.StatusDeleted && profile.DefaultKnowledgeBaseID > 0 {
			return errorsx.InvalidParam("product knowledge base already exists")
		}

		kb := existingKnowledgeBase
		if kb == nil {
			kb = &models.KnowledgeBase{
				TenantID:              tenantID,
				Name:                  name,
				Description:           description,
				KnowledgeType:         string(enums.KnowledgeBaseTypeDocument),
				AccessScope:           string(enums.KnowledgeBaseAccessScopeProduct),
				RagflowDatasetID:      ragflowDatasetID,
				Status:                enums.StatusOk,
				DefaultTopK:           10,
				DefaultScoreThreshold: 0.5,
				DefaultRerankLimit:    5,
				ChunkProvider:         string(enums.KnowledgeChunkProviderStructured),
				ChunkTargetTokens:     300,
				ChunkMaxTokens:        400,
				ChunkOverlapTokens:    40,
				AnswerMode:            1,
				AuditFields:           utils.BuildAuditFields(operator),
			}
			if err := repositories.KnowledgeBaseRepository.Create(ctx.Tx, kb); err != nil {
				return err
			}
		}

		if profile == nil {
			profile = &models.ProductServiceProfile{
				TenantID:               tenantID,
				ProductID:              product.ID,
				SupportLocalesJSON:     string(supportLocalesJSON),
				SupportRegionsJSON:     string(supportRegionsJSON),
				WarrantyPolicyJSON:     "{}",
				DefaultKnowledgeBaseID: kb.ID,
				MeetingEnabled:         true,
				ServicePolicyJSON:      "{}",
				Status:                 enums.StatusOk,
				AuditFields:            utils.BuildAuditFields(operator),
			}
			if err := repositories.ProductServiceProfileRepository.Create(ctx.Tx, profile); err != nil {
				return err
			}
		} else {
			nextSupportLocalesJSON := profile.SupportLocalesJSON
			if len(req.SupportLocales) > 0 {
				nextSupportLocalesJSON = string(supportLocalesJSON)
			}
			nextSupportRegionsJSON := profile.SupportRegionsJSON
			if len(req.SupportRegions) > 0 {
				nextSupportRegionsJSON = string(supportRegionsJSON)
			}
			if nextSupportLocalesJSON == "" {
				nextSupportLocalesJSON = "[]"
			}
			if nextSupportRegionsJSON == "" {
				nextSupportRegionsJSON = "[]"
			}

			updates := map[string]any{
				"tenant_id":                 tenantID,
				"product_id":                product.ID,
				"support_locales_json":      nextSupportLocalesJSON,
				"support_regions_json":      nextSupportRegionsJSON,
				"warranty_policy_json":      firstNonBlank(profile.WarrantyPolicyJSON, "{}"),
				"safety_level":              profile.SafetyLevel,
				"default_flow_template_id":  profile.DefaultFlowTemplateID,
				"default_knowledge_base_id": kb.ID,
				"meeting_enabled":           profile.MeetingEnabled,
				"service_policy_json":       firstNonBlank(profile.ServicePolicyJSON, "{}"),
				"status":                    enums.StatusOk,
				"update_user_id":            operator.UserID,
				"update_user_name":          operator.Username,
				"updated_at":                now,
			}
			if profile.Status == enums.StatusDeleted {
				updates["create_user_id"] = operator.UserID
				updates["create_user_name"] = operator.Username
				updates["created_at"] = now
			}
			if err := repositories.ProductServiceProfileRepository.Updates(ctx.Tx, profile.ID, updates); err != nil {
				return err
			}
		}

		_, err := ProductKnowledgeBindingService.EnsureDefaultBindingTx(ctx.Tx, tenantID, product.ID, kb.ID, operator)
		return err
	}); err != nil {
		return nil, err
	}

	ProductManualFileService.SyncPendingManuals(tenantID, productID, operator)
	return s.GetEnterpriseProductProfile(tenantID, productID)
}

func (s *productCenterService) ListProductModels(tenantID, productID int64) ([]dto.ProductModelBriefDTO, error) {
	if _, err := s.requireTenantProduct(tenantID, productID); err != nil {
		return nil, err
	}
	return s.buildModelBriefList(tenantID, productID), nil
}

type ProductResourceQuery struct {
	Page     int
	PageSize int
	Search   string
}

func normalizeProductResourceQuery(query ProductResourceQuery) ProductResourceQuery {
	query.Page, query.PageSize = normalizeEnterprisePage(query.Page, query.PageSize)
	query.Search = strings.TrimSpace(query.Search)
	return query
}

func productResourcePage[T any](items []T, paging *sqls.Paging) *dto.EnterpriseListResponse[T] {
	if paging == nil {
		paging = &sqls.Paging{Page: 1, Limit: len(items), Total: int64(len(items))}
	}
	totalPages := 0
	totalPages = paging.TotalPage()
	return &dto.EnterpriseListResponse[T]{
		Items: items, Total: paging.Total, Page: paging.Page, PageSize: paging.Limit,
		TotalPages: totalPages, HasMore: paging.Page < totalPages,
	}
}

func (s *productCenterService) ListProductDevices(tenantID, productID int64, query ProductResourceQuery) (*dto.EnterpriseListResponse[dto.ProductDeviceDTO], error) {
	if _, err := s.requireTenantProduct(tenantID, productID); err != nil {
		return nil, err
	}
	query = normalizeProductResourceQuery(query)
	cnd := sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		NotEq("status", enums.StatusDeleted).
		Desc("updated_at").
		Desc("id").
		Page(query.Page, query.PageSize)
	if query.Search != "" {
		pattern := "%" + strings.ToLower(query.Search) + "%"
		cnd.Where("(LOWER(device_no) LIKE ? OR LOWER(serial_no) LIKE ? OR LOWER(region_code) LIKE ?)", pattern, pattern, pattern)
	}
	devices, paging := repositories.DeviceRepository.FindPageByCnd(sqls.DB(), cnd)
	modelNames := make(map[int64]string)
	for i := range devices {
		modelID := devices[i].ProductModelID
		if modelID > 0 {
			if _, ok := modelNames[modelID]; !ok {
				if model := repositories.ProductModelRepository.Get(sqls.DB(), modelID); model != nil {
					modelNames[modelID] = model.Name
				}
			}
		}
	}
	result := make([]dto.ProductDeviceDTO, 0, len(devices))
	for _, item := range devices {
		lastActive := item.UpdatedAt
		if item.LastServiceAt != nil {
			lastActive = *item.LastServiceAt
		}
		status := item.DeviceStatus
		if status == "" {
			status = enterpriseStatusText(item.Status)
		}
		result = append(result, dto.ProductDeviceDTO{
			ID:           item.ID,
			DeviceNo:     item.DeviceNo,
			SerialNo:     item.SerialNo,
			ModelName:    modelNames[item.ProductModelID],
			RegionCode:   item.RegionCode,
			Status:       status,
			LastActiveAt: formatEnterpriseTime(lastActive),
		})
	}
	return productResourcePage(result, paging), nil
}

func (s *productCenterService) ListProductServiceCodes(tenantID, productID int64, query ProductResourceQuery) (*dto.EnterpriseListResponse[dto.ProductServiceCodeDTO], error) {
	if _, err := s.requireTenantProduct(tenantID, productID); err != nil {
		return nil, err
	}
	query = normalizeProductResourceQuery(query)
	cnd := sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		Desc("id").
		Page(query.Page, query.PageSize)
	if query.Search != "" {
		pattern := "%" + strings.ToLower(query.Search) + "%"
		cnd.Where("LOWER(service_code) LIKE ?", pattern)
	}
	codes, paging := repositories.ServiceCodeRepository.FindPageByCnd(sqls.DB(), cnd)
	result := make([]dto.ProductServiceCodeDTO, 0, len(codes))
	for _, item := range codes {
		batchNo := ""
		if item.BatchID > 0 {
			if batch := repositories.ServiceCodeBatchRepository.Get(sqls.DB(), item.BatchID); batch != nil {
				batchNo = batch.BatchNo
			}
		}
		var boundDevice *string
		if item.DeviceID > 0 {
			if device := repositories.DeviceRepository.Get(sqls.DB(), item.DeviceID); device != nil {
				value := device.DeviceNo
				boundDevice = &value
			}
		}
		result = append(result, dto.ProductServiceCodeDTO{
			ID:          item.ID,
			ServiceCode: servicecode.Normalize(item.ServiceCode),
			EntryURL:    servicecode.BuildEntryURL(item.ServiceCode),
			QRURL:       servicecode.BuildQRURL(item.ServiceCode),
			QRImageURL:  servicecode.BuildQRImageURL(item.ServiceCode),
			BatchNo:     batchNo,
			Mode:        string(item.Mode),
			Status:      string(item.Status),
			BoundDevice: boundDevice,
			CreatedAt:   formatEnterpriseTime(item.CreatedAt),
		})
	}
	return productResourcePage(result, paging), nil
}

func (s *productCenterService) ListProductTickets(tenantID, productID int64, query ProductResourceQuery) (*dto.EnterpriseListResponse[dto.ProductTicketBriefDTO], error) {
	if _, err := s.requireTenantProduct(tenantID, productID); err != nil {
		return nil, err
	}
	query = normalizeProductResourceQuery(query)
	cnd := sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		Desc("created_at").
		Desc("id").
		Page(query.Page, query.PageSize)
	if query.Search != "" {
		pattern := "%" + strings.ToLower(query.Search) + "%"
		cnd.Where("(LOWER(ticket_no) LIKE ? OR LOWER(title) LIKE ?)", pattern, pattern)
	}
	tickets, paging := repositories.TicketRepository.FindPageByCnd(sqls.DB(), cnd)
	result := make([]dto.ProductTicketBriefDTO, 0, len(tickets))
	for _, item := range tickets {
		result = append(result, dto.ProductTicketBriefDTO{
			ID:        item.ID,
			TicketNo:  item.TicketNo,
			Title:     item.Title,
			Status:    MapTicketStatusForEnterprise(item.Status),
			Priority:  DeriveTicketPriority(item),
			CreatedAt: formatEnterpriseTime(item.CreatedAt),
		})
	}
	return productResourcePage(result, paging), nil
}

func (s *productCenterService) ListProductConversations(tenantID, productID int64, query ProductResourceQuery) (*dto.EnterpriseListResponse[dto.ProductConversationBriefDTO], error) {
	if _, err := s.requireTenantProduct(tenantID, productID); err != nil {
		return nil, err
	}
	query = normalizeProductResourceQuery(query)
	cnd := sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		Desc("last_message_at").
		Desc("id").
		Page(query.Page, query.PageSize)
	if query.Search != "" {
		pattern := "%" + strings.ToLower(query.Search) + "%"
		cnd.Where("(LOWER(customer_name) LIKE ? OR LOWER(last_message_summary) LIKE ?)", pattern, pattern)
	}
	conversations, paging := repositories.ConversationRepository.FindPageByCnd(sqls.DB(), cnd)
	result := make([]dto.ProductConversationBriefDTO, 0, len(conversations))
	for _, item := range conversations {
		channelName := ""
		if item.ChannelID > 0 {
			if channel := repositories.ChannelRepository.Get(sqls.DB(), item.ChannelID); channel != nil {
				channelName = channel.Name
				if channelName == "" {
					channelName = channel.ChannelType
				}
			}
		}
		result = append(result, dto.ProductConversationBriefDTO{
			ID:            item.ID,
			Channel:       channelName,
			CustomerName:  item.CustomerName,
			Status:        enums.GetIMConversationStatusLabel(item.Status),
			ServiceMode:   enums.GetIMConversationServiceModeLabel(item.ServiceMode),
			Summary:       item.LastMessageSummary,
			LastMessageAt: formatEnterpriseTime(item.LastMessageAt),
			HandoffReason: item.HandoffReason,
		})
	}
	return productResourcePage(result, paging), nil
}

func (s *productCenterService) ListProductRepairHistory(tenantID, productID int64, query ProductResourceQuery) (*dto.EnterpriseListResponse[dto.ProductRepairRecordDTO], error) {
	if _, err := s.requireTenantProduct(tenantID, productID); err != nil {
		return nil, err
	}
	query = normalizeProductResourceQuery(query)
	cnd := sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		Desc("occurred_at").
		Desc("id").
		Page(query.Page, query.PageSize)
	if query.Search != "" {
		pattern := "%" + strings.ToLower(query.Search) + "%"
		cnd.Where("(LOWER(summary) LIKE ? OR LOWER(root_cause) LIKE ? OR LOWER(solution) LIKE ?)", pattern, pattern, pattern)
	}
	records, paging := repositories.DeviceServiceRecordRepository.FindPageByCnd(sqls.DB(), cnd)
	result := make([]dto.ProductRepairRecordDTO, 0, len(records))
	for _, item := range records {
		ticketNo := ""
		faultType := item.ServiceType
		technician := ""
		if item.TicketID > 0 {
			if ticket := repositories.TicketRepository.Get(sqls.DB(), item.TicketID); ticket != nil {
				ticketNo = ticket.TicketNo
				if ticket.FaultCode != "" {
					faultType = ticket.FaultCode
				}
				technician = ticket.CreateUserName
			}
		}
		deviceNo := ""
		if item.DeviceID > 0 {
			if device := repositories.DeviceRepository.Get(sqls.DB(), item.DeviceID); device != nil {
				deviceNo = device.DeviceNo
			}
		}
		result = append(result, dto.ProductRepairRecordDTO{
			ID:                item.ID,
			TicketID:          item.TicketID,
			TicketNo:          ticketNo,
			DeviceID:          item.DeviceID,
			DeviceNo:          deviceNo,
			ServiceType:       item.ServiceType,
			FaultType:         faultType,
			Summary:           item.Summary,
			RootCause:         item.RootCause,
			Solution:          item.Solution,
			Resolution:        item.Solution,
			Technician:        technician,
			VisibleToCustomer: item.VisibleToCustomer,
			OccurredAt:        formatEnterpriseTime(item.OccurredAt),
			CompletedAt:       formatEnterpriseTime(item.OccurredAt),
		})
	}
	return productResourcePage(result, paging), nil
}

func (s *productCenterService) GetProductKnowledgeCoverageDetail(tenantID, productID int64) (*dto.ProductKnowledgeCoverageDetailDTO, error) {
	if _, err := s.requireTenantProduct(tenantID, productID); err != nil {
		return nil, err
	}
	documentCount := repositories.KnowledgeDocumentRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).NotEq("status", enums.StatusDeleted))
	faqCount := repositories.KnowledgeFAQRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).NotEq("status", enums.StatusDeleted))
	productSpecific := repositories.ProductKnowledgeLinkRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID).NotEq("status", enums.StatusDeleted))
	modelSpecific := repositories.ProductKnowledgeLinkRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID).NotEq("product_model_id", 0).NotEq("status", enums.StatusDeleted))
	pendingCandidates := repositories.KnowledgeCandidateRepository.Count(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		Eq("review_status", "pending").
		Eq("status", enums.StatusOk))
	total := documentCount + faqCount
	coverageScore := 0.0
	if total > 0 {
		coverageScore = float64(productSpecific) / float64(total) * 100
	}
	links, _ := s.listProductKnowledgeLinks(tenantID, productID, productKnowledgeCoveragePreviewLimit)
	var knowledgeBaseID *int64
	var ragflowDatasetID string
	if profile := repositories.ProductServiceProfileRepository.GetByProductID(sqls.DB(), productID); profile != nil && profile.Status != enums.StatusDeleted && profile.DefaultKnowledgeBaseID > 0 {
		kbID := profile.DefaultKnowledgeBaseID
		knowledgeBaseID = &kbID
		ragflowDatasetID = s.ragflowDatasetForKnowledgeBase(tenantID, kbID, nil)
	}
	gaps := []string{}
	if productSpecific == 0 {
		gaps = append(gaps, "No product-specific knowledge links")
	}
	if modelSpecific == 0 {
		gaps = append(gaps, "No model-specific knowledge links")
	}
	if pendingCandidates > 0 {
		gaps = append(gaps, "Pending knowledge candidates need review")
	}
	return &dto.ProductKnowledgeCoverageDetailDTO{
		TotalEntries:      total,
		ProductSpecific:   productSpecific,
		ModelSpecific:     modelSpecific,
		KnowledgeBaseID:   knowledgeBaseID,
		RagflowDatasetID:  ragflowDatasetID,
		CoverageScore:     coverageScore,
		LinkedEntries:     productSpecific,
		PendingCandidates: pendingCandidates,
		Gaps:              gaps,
		Links:             links,
	}, nil
}

func (s *productCenterService) ListProductKnowledgeLinks(tenantID, productID int64) ([]dto.KnowledgeLinkDTO, error) {
	return s.listProductKnowledgeLinks(tenantID, productID, 0)
}

func (s *productCenterService) listProductKnowledgeLinks(tenantID, productID int64, limit int) ([]dto.KnowledgeLinkDTO, error) {
	if _, err := s.requireTenantProduct(tenantID, productID); err != nil {
		return nil, err
	}
	cnd := sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		NotEq("status", enums.StatusDeleted).
		Asc("sort_no").
		Desc("updated_at")
	if limit > 0 {
		cnd.Limit(limit)
	}
	links := repositories.ProductKnowledgeLinkRepository.Find(sqls.DB(), cnd)
	result := make([]dto.KnowledgeLinkDTO, 0, len(links))
	datasetCache := make(map[int64]string)
	for _, item := range links {
		result = append(result, dto.KnowledgeLinkDTO{
			ID:               item.ID,
			KnowledgeBaseID:  item.KnowledgeBaseID,
			RagflowDatasetID: s.ragflowDatasetForKnowledgeBase(tenantID, item.KnowledgeBaseID, datasetCache),
			Title:            buildKnowledgeLinkTitle(item),
			Type:             item.LinkType,
			Language:         item.Language,
			Version:          item.Version,
			Visibility:       item.Visibility,
			UpdatedAt:        formatEnterpriseTime(item.UpdatedAt),
		})
	}
	return result, nil
}

func (s *productCenterService) ListProductManuals(tenantID, productID int64) ([]dto.KnowledgeLinkDTO, error) {
	if _, err := s.requireTenantProduct(tenantID, productID); err != nil {
		return nil, err
	}
	links := repositories.ProductKnowledgeLinkRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		Eq("link_type", "manual").
		NotEq("status", enums.StatusDeleted).
		Asc("sort_no").
		Desc("updated_at"))
	result := make([]dto.KnowledgeLinkDTO, 0, len(links))
	datasetCache := make(map[int64]string)
	for _, item := range links {
		result = append(result, dto.KnowledgeLinkDTO{
			ID:               item.ID,
			KnowledgeBaseID:  item.KnowledgeBaseID,
			RagflowDatasetID: s.ragflowDatasetForKnowledgeBase(tenantID, item.KnowledgeBaseID, datasetCache),
			Title:            buildKnowledgeLinkTitle(item),
			Type:             item.LinkType,
			Language:         item.Language,
			Version:          item.Version,
			Visibility:       item.Visibility,
			UpdatedAt:        formatEnterpriseTime(item.UpdatedAt),
		})
	}
	return result, nil
}

func (s *productCenterService) ragflowDatasetForKnowledgeBase(tenantID, knowledgeBaseID int64, cache map[int64]string) string {
	if knowledgeBaseID <= 0 {
		return ""
	}
	if cache != nil {
		if datasetID, ok := cache[knowledgeBaseID]; ok {
			return datasetID
		}
	}
	datasetID := ""
	if kb := repositories.KnowledgeBaseRepository.Get(sqls.DB(), knowledgeBaseID); kb != nil && kb.TenantID == tenantID && kb.Status != enums.StatusDeleted {
		datasetID = strings.TrimSpace(kb.RagflowDatasetID)
	}
	if cache != nil {
		cache[knowledgeBaseID] = datasetID
	}
	return datasetID
}

func (s *productCenterService) GetProductFaultStats(tenantID, productID int64, rangeValue string) (*dto.ProductFaultStatsDTO, error) {
	if _, err := s.requireTenantProduct(tenantID, productID); err != nil {
		return nil, err
	}

	rangeLabel, days := normalizeProductFaultStatsRange(rangeValue)
	now := time.Now()
	startDate := now.AddDate(0, 0, -days)
	previousStart := startDate.AddDate(0, 0, -days)

	currentStats := repositories.ProductFaultStatsRepository.FindByTenantProductAndDateRange(sqls.DB(), tenantID, productID, startDate, now)
	previousStats := repositories.ProductFaultStatsRepository.FindByTenantProductAndDateRange(sqls.DB(), tenantID, productID, previousStart, startDate)
	previousCounts := aggregateFaultStatsCounts(previousStats)
	modelNames := map[int64]string{}

	type aggregate struct {
		part           string
		faultType      string
		productModelID int64
		count          int64
	}
	aggregates := map[string]*aggregate{}
	var totalFaults int64
	for _, item := range currentStats {
		key := productFaultStatsKey(item)
		if _, ok := aggregates[key]; !ok {
			aggregates[key] = &aggregate{
				part:           firstNonBlank(item.FaultPart, "Unspecified"),
				faultType:      firstNonBlank(item.FaultCode, "unknown"),
				productModelID: item.ProductModelID,
			}
		}
		aggregates[key].count += item.TicketCount
		totalFaults += item.TicketCount
		if item.ProductModelID > 0 {
			modelNames[item.ProductModelID] = s.lookupProductModelName(tenantID, productID, item.ProductModelID)
		}
	}

	items := make([]dto.ProductFaultStatItemDTO, 0, len(aggregates))
	for key, item := range aggregates {
		percentage := 0.0
		if totalFaults > 0 {
			percentage = float64(item.count) / float64(totalFaults) * 100
		}
		modelName := "All models"
		if item.productModelID > 0 {
			modelName = firstNonBlank(modelNames[item.productModelID], "Unknown model")
		}
		items = append(items, dto.ProductFaultStatItemDTO{
			Part:       item.part,
			FaultType:  item.faultType,
			ModelName:  modelName,
			Count:      item.count,
			Percentage: percentage,
			Trend:      productFaultTrend(item.count, previousCounts[key]),
			Severity:   productFaultSeverity(item.count, totalFaults),
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].FaultType < items[j].FaultType
		}
		return items[i].Count > items[j].Count
	})

	return &dto.ProductFaultStatsDTO{
		Range:           rangeLabel,
		GeneratedAt:     now.UTC().Format(time.RFC3339),
		DataStatus:      s.faultStatsDataStatus(tenantID, productID, startDate, now, totalFaults),
		TotalFaults:     totalFaults,
		AffectedDevices: s.countAffectedDevices(tenantID, productID, startDate, now),
		Stats:           items,
	}, nil
}

// faultStatsDataStatus 区分“无故障”与“尚未统计/统计失败”（§6.3）。
func (s *productCenterService) faultStatsDataStatus(tenantID, productID int64, startDate, now time.Time, totalFaults int64) string {
	if totalFaults > 0 {
		return "ready"
	}
	factCount := ProductFaultStatsService.CountTicketsInRange(tenantID, productID, startDate, now)
	if factCount == 0 {
		return "empty"
	}
	if ProductFaultStatsService.LatestRebuildFailed(tenantID, productID) {
		return "failed"
	}
	return "building"
}

func (s *productCenterService) CreateProductQualitySignal(req CreateProductQualitySignalRequest, operator *dto.AuthPrincipal) (*dto.ProductQualitySignalDTO, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if _, err := s.requireTenantProduct(req.TenantID, req.ProductID); err != nil {
		return nil, err
	}
	if err := s.validateProductQualitySignalModel(req.TenantID, req.ProductID, req.ProductModelID); err != nil {
		return nil, err
	}
	signalType := strings.TrimSpace(req.SignalType)
	if signalType == "" {
		return nil, errorsx.InvalidParam("signalType is required")
	}
	detectedAt := req.DetectedAt
	if detectedAt.IsZero() {
		detectedAt = time.Now()
	}
	source := firstNonBlank(req.Source, "manual")
	sourceType := firstNonBlank(req.SourceType, source)

	item := &models.ProductQualitySignal{
		TenantID:         req.TenantID,
		ProductID:        req.ProductID,
		ProductModelID:   req.ProductModelID,
		SignalType:       signalType,
		Severity:         firstNonBlank(req.Severity, "info"),
		Title:            strings.TrimSpace(req.Title),
		Description:      strings.TrimSpace(req.Description),
		TriggerCondition: strings.TrimSpace(req.TriggerCondition),
		MetricValue:      req.MetricValue,
		SampleCount:      req.SampleCount,
		DetectedAt:       detectedAt,
		Source:           source,
		SourceType:       sourceType,
		SourceID:         strings.TrimSpace(req.SourceID),
		OwnerUserID:      req.OwnerUserID,
		Status:           enums.StatusOk,
		AuditFields:      utils.BuildAuditFields(operator),
	}
	if err := repositories.ProductQualitySignalRepository.Create(sqls.DB(), item); err != nil {
		return nil, err
	}
	result := buildProductQualitySignalDTO(*item)
	return &result, nil
}

func (s *productCenterService) UpdateProductQualitySignal(req UpdateProductQualitySignalRequest, operator *dto.AuthPrincipal) (*dto.ProductQualitySignalDTO, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item, err := s.requireProductQualitySignal(req.TenantID, req.ProductID, req.SignalID)
	if err != nil {
		return nil, err
	}

	updates := map[string]any{}
	if req.ProductModelID != nil {
		if err := s.validateProductQualitySignalModel(req.TenantID, req.ProductID, *req.ProductModelID); err != nil {
			return nil, err
		}
		updates["product_model_id"] = *req.ProductModelID
	}
	if req.SignalType != nil {
		value := strings.TrimSpace(*req.SignalType)
		if value == "" {
			return nil, errorsx.InvalidParam("signalType is required")
		}
		updates["signal_type"] = value
	}
	if req.Severity != nil {
		updates["severity"] = firstNonBlank(*req.Severity, "info")
	}
	if req.Title != nil {
		updates["title"] = strings.TrimSpace(*req.Title)
	}
	if req.Description != nil {
		updates["description"] = strings.TrimSpace(*req.Description)
	}
	if req.TriggerCondition != nil {
		updates["trigger_condition"] = strings.TrimSpace(*req.TriggerCondition)
	}
	if req.MetricValue != nil {
		updates["metric_value"] = *req.MetricValue
	}
	if req.SampleCount != nil {
		updates["sample_count"] = *req.SampleCount
	}
	if req.Source != nil {
		updates["source"] = firstNonBlank(*req.Source, "manual")
	}
	if req.SourceType != nil {
		updates["source_type"] = strings.TrimSpace(*req.SourceType)
	}
	if req.SourceID != nil {
		updates["source_id"] = strings.TrimSpace(*req.SourceID)
	}
	if req.OwnerUserID != nil {
		updates["owner_user_id"] = *req.OwnerUserID
	}
	if len(updates) == 0 {
		result := buildProductQualitySignalDTO(*item)
		return &result, nil
	}
	updates["update_user_id"] = operator.UserID
	updates["update_user_name"] = operator.Username
	updates["updated_at"] = time.Now()
	if err := repositories.ProductQualitySignalRepository.Updates(sqls.DB(), item.ID, updates); err != nil {
		return nil, err
	}
	item = repositories.ProductQualitySignalRepository.Get(sqls.DB(), item.ID)
	result := buildProductQualitySignalDTO(*item)
	return &result, nil
}

func (s *productCenterService) ResolveProductQualitySignal(tenantID, productID, signalID int64, resolution string, operator *dto.AuthPrincipal) (*dto.ProductQualitySignalDTO, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item, err := s.requireProductQualitySignal(tenantID, productID, signalID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if err := repositories.ProductQualitySignalRepository.Updates(sqls.DB(), item.ID, map[string]any{
		"resolved_at":      now,
		"resolution":       strings.TrimSpace(resolution),
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       now,
	}); err != nil {
		return nil, err
	}
	item = repositories.ProductQualitySignalRepository.Get(sqls.DB(), item.ID)
	result := buildProductQualitySignalDTO(*item)
	return &result, nil
}

func (s *productCenterService) ListProductQualitySignals(tenantID, productID int64) ([]dto.ProductQualitySignalDTO, error) {
	if _, err := s.requireTenantProduct(tenantID, productID); err != nil {
		return nil, err
	}
	signals := repositories.ProductQualitySignalRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		Desc("detected_at").
		Desc("id"))
	result := make([]dto.ProductQualitySignalDTO, 0, len(signals))
	for _, item := range signals {
		result = append(result, buildProductQualitySignalDTO(item))
	}
	return result, nil
}

func (s *productCenterService) ListProductUsageMetrics(tenantID, productID int64) ([]dto.ProductUsageMetricDTO, error) {
	if _, err := s.requireTenantProduct(tenantID, productID); err != nil {
		return nil, err
	}
	now := time.Now()
	currentStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	lastStart := currentStart.AddDate(0, -1, 0)
	currentRequests := repositories.ProductAIUsageEventRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID).Where("occurred_at >= ?", currentStart))
	lastRequests := repositories.ProductAIUsageEventRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID).Where("occurred_at >= ? AND occurred_at < ?", lastStart, currentStart))
	return []dto.ProductUsageMetricDTO{
		{Metric: "AI Requests", CurrentMonth: currentRequests, LastMonth: lastRequests, Unit: "calls"},
		{Metric: "Knowledge Retrieval", CurrentMonth: 0, LastMonth: 0, Unit: "calls"},
		{Metric: "Meeting Minutes", CurrentMonth: 0, LastMonth: 0, Unit: "minutes"},
	}, nil
}

// GetProductProfile 获取产品聚合档案
// 返回产品档案、型号列表、设备概览、服务码状态统计、工单统计、维修历史摘要、知识覆盖、质量信号
func (s *productCenterService) GetProductProfile(tenantID, productID int64) (*dto.ProductProfileDTO, error) {
	if tenantID <= 0 || productID <= 0 {
		return nil, errorsx.InvalidParam("tenantId and productId are required")
	}

	product := repositories.ProductRepository.Get(sqls.DB(), productID)
	if product == nil || product.TenantID != tenantID {
		return nil, errorsx.InvalidParam("product not found")
	}

	profile := &dto.ProductProfileDTO{
		Product: s.buildProductOverview(product),
		Models:  s.buildModelBriefList(tenantID, productID),
	}

	profile.DeviceOverview = s.buildDeviceOverview(tenantID, productID)
	profile.ServiceCodeStats = s.buildServiceCodeStats(tenantID, productID)
	profile.TicketStats = s.buildTicketStats(tenantID, productID)
	profile.RepairHistory = s.buildRepairHistorySummary(tenantID, productID)
	profile.KnowledgeCoverage = s.buildKnowledgeCoverage(tenantID, productID)
	profile.QualitySignals = s.buildQualitySignalSummary(tenantID, productID)
	profile.UsageSummary = s.buildUsageSummary(tenantID, productID)

	return profile, nil
}

func (s *productCenterService) buildProductOverview(product *models.Product) *dto.ProductOverviewDTO {
	if product == nil {
		return nil
	}
	modelCount := repositories.ProductModelRepository.Count(sqls.DB(), sqls.NewCnd().Eq("product_id", product.ID).Eq("status", enums.StatusOk))
	deviceCount := repositories.DeviceRepository.Count(sqls.DB(), sqls.NewCnd().Eq("product_id", product.ID).Eq("status", enums.StatusOk))
	ticketCount := repositories.TicketRepository.Count(sqls.DB(), sqls.NewCnd().Eq("product_id", product.ID))
	return &dto.ProductOverviewDTO{
		ID:            product.ID,
		TenantID:      product.TenantID,
		Code:          product.Code,
		Name:          product.Name,
		Category:      product.Category,
		ProductLineID: product.ProductLineID,
		Status:        int(product.Status),
		ModelCount:    int(modelCount),
		ActiveDevices: deviceCount,
		TotalTickets:  ticketCount,
	}
}

func (s *productCenterService) buildModelBriefList(tenantID, productID int64) []dto.ProductModelBriefDTO {
	models := repositories.ProductModelRepository.Find(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID).Asc("id"))
	result := make([]dto.ProductModelBriefDTO, 0, len(models))
	for _, m := range models {
		deviceCount := repositories.DeviceRepository.Count(sqls.DB(), sqls.NewCnd().Eq("product_model_id", m.ID))
		result = append(result, dto.ProductModelBriefDTO{
			ID:          m.ID,
			TenantID:    m.TenantID,
			ModelCode:   m.ModelCode,
			Name:        m.Name,
			Status:      int(m.Status),
			DeviceCount: deviceCount,
		})
	}
	return result
}

func (s *productCenterService) buildDeviceOverview(tenantID, productID int64) *dto.DeviceOverviewDTO {
	allDevices := repositories.DeviceRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID))
	activeDevices := repositories.DeviceRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID).Eq("device_status", string(enums.DeviceStatusOperational)))
	inMaintenance := repositories.DeviceRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID).Eq("device_status", string(enums.DeviceStatusUnderMaintenance)))
	decommissioned := repositories.DeviceRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID).Eq("device_status", string(enums.DeviceStatusDecommissioned)))
	// 未绑定设备的判断：设备状态正常但没有顾客绑定记录
	unbound := activeDevices // 简化，近似统计
	return &dto.DeviceOverviewDTO{
		TotalDevices:   allDevices,
		ActiveDevices:  activeDevices,
		InMaintenance:  inMaintenance,
		Decommissioned: decommissioned,
		UnboundDevices: unbound,
	}
}

func (s *productCenterService) buildServiceCodeStats(tenantID, productID int64) *dto.ServiceCodeStatsDTO {
	stats := &dto.ServiceCodeStatsDTO{}
	stats.Total = repositories.ServiceCodeRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID))
	stats.Active = repositories.ServiceCodeRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID).Eq("status", string(enums.ServiceCodeStatusActive)))
	stats.Bound = repositories.ServiceCodeRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID).Eq("status", string(enums.ServiceCodeStatusBound)))
	stats.Expired = repositories.ServiceCodeRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID).Eq("status", string(enums.ServiceCodeStatusExpired)))
	stats.Revoked = repositories.ServiceCodeRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID).Eq("status", string(enums.ServiceCodeStatusRevoked)))
	return stats
}

func (s *productCenterService) buildTicketStats(tenantID, productID int64) *dto.TicketStatsDTO {
	stats := &dto.TicketStatsDTO{}
	stats.Total = repositories.TicketRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID))
	stats.Pending = repositories.TicketRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID).Eq("status", string(enums.TicketStatusPending)))
	stats.InProgress = repositories.TicketRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID).Eq("status", string(enums.TicketStatusInProgress)))
	stats.Closed = repositories.TicketRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID).Eq("status", string(enums.TicketStatusClosed)))
	stats.Escalated = repositories.TicketRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID).Eq("status", string(enums.TicketStatusEscalated)))
	stats.Reopened = repositories.TicketRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID).Eq("status", string(enums.TicketStatusReopened)))
	return stats
}

func (s *productCenterService) buildRepairHistorySummary(tenantID, productID int64) *dto.RepairHistorySummaryDTO {
	summary := &dto.RepairHistorySummaryDTO{}
	repairs := repositories.DeviceServiceRecordRepository.FindByProductID(sqls.DB(), productID)
	summary.TotalRepairs = int64(len(repairs))
	remoteCount := int64(0)
	onsiteCount := int64(0)
	faultCodeMap := make(map[string]int64)
	for _, r := range repairs {
		if r.ServiceType == "remote" || r.ServiceType == "repair" {
			remoteCount++
		} else if r.ServiceType == "onsite" {
			onsiteCount++
		}
		if summary.LastRepairAt == nil || r.OccurredAt.After(*summary.LastRepairAt) {
			summary.LastRepairAt = &r.OccurredAt
		}
	}
	summary.RemoteResolved = remoteCount
	summary.OnsiteRepairs = onsiteCount
	// 统计故障码
	tickets := repositories.TicketRepository.Find(sqls.DB(), sqls.NewCnd().Eq("product_id", productID).NotEq("fault_code", ""))
	for _, t := range tickets {
		if t.FaultCode != "" {
			faultCodeMap[t.FaultCode]++
		}
	}
	topFaults := make([]dto.FaultCodeEntry, 0, 5)
	for code, count := range faultCodeMap {
		topFaults = append(topFaults, dto.FaultCodeEntry{FaultCode: code, Count: count})
		if len(topFaults) >= 5 {
			break
		}
	}
	summary.TopFaultCodes = topFaults
	return summary
}

func (s *productCenterService) buildKnowledgeCoverage(tenantID, productID int64) *dto.KnowledgeCoverageDTO {
	totalEntries := repositories.KnowledgeBaseRepository.Count(sqls.DB(), sqls.NewCnd())
	productSpecific := repositories.ProductKnowledgeLinkRepository.Count(sqls.DB(), sqls.NewCnd().Eq("product_id", productID))
	modelSpecific := repositories.ProductKnowledgeLinkRepository.Count(sqls.DB(), sqls.NewCnd().Eq("product_id", productID).NotEq("product_model_id", 0))
	coverage := 0.0
	if totalEntries > 0 {
		coverage = float64(productSpecific) / float64(totalEntries) * 100
	}
	return &dto.KnowledgeCoverageDTO{
		TotalEntries:    totalEntries,
		ProductSpecific: productSpecific,
		ModelSpecific:   modelSpecific,
		CoverageScore:   coverage,
	}
}

func (s *productCenterService) buildQualitySignalSummary(tenantID, productID int64) *dto.QualitySignalSummaryDTO {
	summary := &dto.QualitySignalSummaryDTO{}
	openCnd := sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID).NotEq("status", enums.StatusDeleted).Where("resolved_at IS NULL")
	summary.OpenSignals = repositories.ProductQualitySignalRepository.Count(sqls.DB(), openCnd)
	summary.WarningCount = repositories.ProductQualitySignalRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID).Eq("severity", "warning").NotEq("status", enums.StatusDeleted).Where("resolved_at IS NULL"))
	summary.CriticalCount = repositories.ProductQualitySignalRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID).Eq("severity", "critical").NotEq("status", enums.StatusDeleted).Where("resolved_at IS NULL"))
	summary.ResolvedCount = repositories.ProductQualitySignalRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID).NotEq("status", enums.StatusDeleted).Where("resolved_at IS NOT NULL"))
	return summary
}

func (s *productCenterService) buildUsageSummary(tenantID, productID int64) *dto.UsageSummaryDTO {
	summary := &dto.UsageSummaryDTO{}
	summary.TotalRequests = repositories.ProductAIUsageEventRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", tenantID).Eq("product_id", productID))
	return summary
}

func normalizeProductFaultStatsRange(value string) (string, int) {
	switch value {
	case "30d":
		return "30d", 30
	case "180d":
		return "180d", 180
	case "90d", "":
		return "90d", 90
	default:
		return "90d", 90
	}
}

func productFaultStatsKey(item models.ProductFaultStatsDaily) string {
	return firstNonBlank(item.FaultPart, "Unspecified") + "|" + firstNonBlank(item.FaultCode, "unknown") + "|" + formatInt64(item.ProductModelID)
}

func aggregateFaultStatsCounts(items []models.ProductFaultStatsDaily) map[string]int64 {
	result := make(map[string]int64, len(items))
	for _, item := range items {
		result[productFaultStatsKey(item)] += item.TicketCount
	}
	return result
}

func productFaultTrend(current, previous int64) string {
	switch {
	case current > previous:
		return "up"
	case current < previous:
		return "down"
	default:
		return "stable"
	}
}

func normalizeEnterpriseStringList(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	sort.Strings(result)
	return result
}

func productFaultSeverity(count, total int64) string {
	if total <= 0 || count <= 0 {
		return "low"
	}
	percentage := float64(count) / float64(total) * 100
	switch {
	case count >= 20 || percentage >= 50:
		return "critical"
	case count >= 5 || percentage >= 25:
		return "high"
	case count >= 2 || percentage >= 10:
		return "medium"
	default:
		return "low"
	}
}

func qualitySignalStatusText(item models.ProductQualitySignal) string {
	if item.ResolvedAt != nil {
		return "resolved"
	}
	if item.AcknowledgedAt != nil {
		return "acknowledged"
	}
	if item.Status == enums.StatusDeleted {
		return "deleted"
	}
	return "open"
}

func buildProductQualitySignalDTO(item models.ProductQualitySignal) dto.ProductQualitySignalDTO {
	source := firstNonBlank(item.SourceType, item.Source)
	return dto.ProductQualitySignalDTO{
		ID:             item.ID,
		SignalType:     item.SignalType,
		Severity:       item.Severity,
		Title:          firstNonBlank(item.Title, item.Description, item.SignalType),
		Description:    item.Description,
		Source:         source,
		SourceType:     item.SourceType,
		SourceID:       item.SourceID,
		MetricValue:    item.MetricValue,
		SampleCount:    item.SampleCount,
		OwnerUserID:    item.OwnerUserID,
		DetectedAt:     formatEnterpriseTime(item.DetectedAt),
		AcknowledgedAt: formatEnterpriseTimePtr(item.AcknowledgedAt),
		ResolvedAt:     formatEnterpriseTimePtr(item.ResolvedAt),
		Status:         qualitySignalStatusText(item),
	}
}

func (s *productCenterService) requireProductQualitySignal(tenantID, productID, signalID int64) (*models.ProductQualitySignal, error) {
	if _, err := s.requireTenantProduct(tenantID, productID); err != nil {
		return nil, err
	}
	item := repositories.ProductQualitySignalRepository.Get(sqls.DB(), signalID)
	if item == nil || item.TenantID != tenantID || item.ProductID != productID || item.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParam("product quality signal not found")
	}
	return item, nil
}

func (s *productCenterService) validateProductQualitySignalModel(tenantID, productID, modelID int64) error {
	if modelID <= 0 {
		return nil
	}
	model := repositories.ProductModelRepository.Get(sqls.DB(), modelID)
	if model == nil || model.TenantID != tenantID || model.ProductID != productID || model.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("product model not found")
	}
	return nil
}

func (s *productCenterService) lookupProductModelName(tenantID, productID, modelID int64) string {
	if modelID <= 0 {
		return ""
	}
	model := repositories.ProductModelRepository.Get(sqls.DB(), modelID)
	if model == nil || model.TenantID != tenantID || model.ProductID != productID || model.Status == enums.StatusDeleted {
		return ""
	}
	return firstNonBlank(model.Name, model.ModelCode)
}

func (s *productCenterService) countAffectedDevices(tenantID, productID int64, startDate, endDate time.Time) int64 {
	var count int64
	sqls.DB().Model(&models.Ticket{}).
		Where("tenant_id = ? AND product_id = ? AND device_id > 0 AND created_at >= ? AND created_at <= ?", tenantID, productID, startDate, endDate).
		Distinct("device_id").
		Count(&count)
	return count
}

func formatInt64(value int64) string {
	if value == 0 {
		return "0"
	}
	return strconv.FormatInt(value, 10)
}
