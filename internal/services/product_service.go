package services

import (
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/eventbus"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var ProductService = newProductService()

func newProductService() *productService {
	return &productService{}
}

type productService struct {
}

func (s *productService) Get(id int64) *models.Product {
	if id <= 0 {
		return nil
	}
	return repositories.ProductRepository.Get(sqls.DB(), id)
}

func (s *productService) Find(cnd *sqls.Cnd) []models.Product {
	return repositories.ProductRepository.Find(sqls.DB(), cnd)
}

func (s *productService) FindPageByCnd(cnd *sqls.Cnd) (list []models.Product, paging *sqls.Paging) {
	return repositories.ProductRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *productService) Count(cnd *sqls.Cnd) int64 {
	return repositories.ProductRepository.Count(sqls.DB(), cnd)
}

// resolveProductLineID 解析产品线主数据：
//   - productLineID > 0：校验其属于当前租户且未删除，直接采用；
//   - 否则按名称在当前租户内查找，不存在则创建产品线主数据；
//   - 名称为空时返回 0（不关联产品线）。
func (s *productService) resolveProductLineID(tenantID, productLineID int64, productLine string, operator *dto.AuthPrincipal) (int64, error) {
	if productLineID > 0 {
		line := repositories.ProductLineRepository.GetByTenant(sqls.DB(), productLineID, tenantID)
		if line == nil || line.Status == enums.StatusDeleted {
			return 0, errorsx.InvalidParam("product line not found")
		}
		return line.ID, nil
	}
	name := strings.TrimSpace(productLine)
	if name == "" {
		return 0, nil
	}
	if line := repositories.ProductLineRepository.GetByTenantName(sqls.DB(), tenantID, name); line != nil {
		if line.Status == enums.StatusDeleted {
			return 0, errorsx.InvalidParam("product line not found")
		}
		return line.ID, nil
	}
	code := normalizeCode(name)
	if len(code) > 64 {
		code = code[:64]
	}
	if code == "" {
		code = "PL"
	}
	if existing := repositories.ProductLineRepository.GetByTenantCode(sqls.DB(), tenantID, code); existing != nil {
		return existing.ID, nil
	}
	line := &models.ProductLine{
		TenantID: tenantID,
		Code:     code,
		Name:     name,
		Status:   enums.StatusOk,
	}
	if operator != nil {
		line.AuditFields = utils.BuildAuditFields(operator)
	}
	if err := repositories.ProductLineRepository.Create(sqls.DB(), line); err != nil {
		return 0, err
	}
	return line.ID, nil
}

func (s *productService) generateProductCode(tenantID int64) (string, error) {
	for attempt := int64(0); attempt < 10; attempt++ {
		code := "PROD-" + strconv.FormatInt(time.Now().UTC().UnixNano()+attempt, 10)
		if repositories.ProductRepository.GetByTenantCode(sqls.DB(), tenantID, code) == nil {
			return code, nil
		}
	}
	return "", errorsx.InvalidParam("failed to generate product code")
}

// mapProductStatusText 将前端状态文本映射为枚举状态。
func mapProductStatusText(statusText string) (enums.Status, error) {
	switch strings.ToLower(strings.TrimSpace(statusText)) {
	case "active":
		return enums.StatusOk, nil
	case "inactive", "disabled", "discontinued":
		return enums.StatusDisabled, nil
	default:
		return 0, errorsx.InvalidParam("invalid product status")
	}
}

func (s *productService) CreateProduct(req request.CreateProductRequest, operator *dto.AuthPrincipal) (*models.Product, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	// tenant_id 优先从认证主体派生；operator 未携带租户的内部/测试场景回退到请求体
	tenantID := operator.TenantID
	if tenantID <= 0 {
		tenantID = req.TenantID
	}
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenantId is required")
	}
	if _, err := requireActiveTenant(tenantID); err != nil {
		return nil, err
	}
	if err := TenantCommercialService.RequireTenantAIWorkspaceForProductCreate(tenantID); err != nil {
		return nil, err
	}

	code := normalizeCode(req.Code)
	name := strings.TrimSpace(req.Name)
	if code == "" {
		generatedCode, err := s.generateProductCode(tenantID)
		if err != nil {
			return nil, err
		}
		code = generatedCode
	}
	if name == "" {
		return nil, errorsx.InvalidParam("product name is required")
	}
	existing := repositories.ProductRepository.GetByTenantCode(sqls.DB(), tenantID, code)
	if existing != nil {
		return nil, errorsx.InvalidParam("product code already exists")
	}

	productLineID, err := s.resolveProductLineID(tenantID, req.ProductLineID, req.ProductLine, operator)
	if err != nil {
		return nil, err
	}
	if req.OwnerMemberID > 0 {
		if _, err := s.requireActiveProductOwnerMember(tenantID, req.OwnerMemberID); err != nil {
			return nil, err
		}
	}

	item := &models.Product{
		TenantID:      tenantID,
		ProductLineID: productLineID,
		Code:          code,
		Name:          name,
		Description:   strings.TrimSpace(req.Description),
		Category:      strings.TrimSpace(req.Category),
		OwnerMemberID: req.OwnerMemberID,
		DefaultLocale: strings.TrimSpace(req.DefaultLocale),
		Status:        enums.StatusOk,
		AuditFields:   utils.BuildAuditFields(operator),
	}
	if item.DefaultLocale == "" {
		item.DefaultLocale = "en"
	}
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := repositories.ProductRepository.Create(ctx.Tx, item); err != nil {
			return err
		}
		if _, err := ProductSupportOrganizationService.EnsureProductRepairTeamDB(ctx.Tx, item, operator); err != nil {
			return err
		}
		eventID := "tenant:" + strconv.FormatInt(item.TenantID, 10) + ":product.created:" + strconv.FormatInt(item.ID, 10)
		_, err := eventbus.EnqueueTx(ctx.Tx, eventbus.DurableEvent{
			TenantID:       item.TenantID,
			IdempotencyKey: eventID,
			EventType:      events.EventProductCreated,
			Payload: events.ProductCreatedEvent{
				EventID:    eventID,
				ProductID:  item.ID,
				TenantID:   item.TenantID,
				OperatorID: operator.UserID,
			},
			Source:      "product_service",
			AggregateID: strconv.FormatInt(item.ID, 10),
			ActorID:     strconv.FormatInt(operator.UserID, 10),
			ActorType:   "user",
		})
		return err
	}); err != nil {
		return nil, err
	}
	ProductCenterService.InvalidateProductCountCache(tenantID)
	if err := s.ensureDefaultProductCustomerAgent(item, operator); err != nil {
		return nil, err
	}
	eventbus.WakeDefaultOutboxPublisher()
	return item, nil
}

func (s *productService) UpdateProduct(req request.UpdateProductRequest, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	tenantID := operator.TenantID
	if tenantID <= 0 {
		return errorsx.InvalidParam("tenantId is required")
	}
	// 使用 tenant-scoped 查询，确保只能操作本租户产品
	item := repositories.ProductRepository.GetByTenant(sqls.DB(), req.ID, tenantID)
	if item == nil || item.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("product not found")
	}
	if _, err := requireActiveTenant(tenantID); err != nil {
		return err
	}

	// PATCH 合并语义：空值保持原字段，避免未携带的字段被静默清空。
	code := normalizeCode(req.Code)
	if code == "" {
		code = item.Code
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = item.Name
	}
	if existingCode := repositories.ProductRepository.GetByTenantCode(sqls.DB(), tenantID, code); existingCode != nil && existingCode.ID != req.ID {
		return errorsx.InvalidParam("product code already exists")
	}

	description := item.Description
	if req.DescriptionProvided {
		description = strings.TrimSpace(req.Description)
	}
	category := strings.TrimSpace(req.Category)
	if category == "" {
		category = item.Category
	}
	defaultLocale := strings.TrimSpace(req.DefaultLocale)
	if defaultLocale == "" {
		defaultLocale = item.DefaultLocale
	}
	ownerMemberID := req.OwnerMemberID
	if ownerMemberID == 0 {
		ownerMemberID = item.OwnerMemberID
	}
	if ownerMemberID > 0 {
		if _, err := s.requireActiveProductOwnerMember(tenantID, ownerMemberID); err != nil {
			return err
		}
	}

	productLineID := item.ProductLineID
	if req.ProductLineID > 0 || strings.TrimSpace(req.ProductLine) != "" {
		resolved, err := s.resolveProductLineID(tenantID, req.ProductLineID, req.ProductLine, operator)
		if err != nil {
			return err
		}
		productLineID = resolved
	}

	updates := map[string]any{
		"product_line_id":  productLineID,
		"code":             code,
		"name":             name,
		"description":      description,
		"category":         category,
		"owner_member_id":  ownerMemberID,
		"default_locale":   defaultLocale,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	}
	if strings.TrimSpace(req.StatusText) != "" {
		status, err := mapProductStatusText(req.StatusText)
		if err != nil {
			return err
		}
		updates["status"] = status
	}
	// 禁止修改 tenant_id，始终使用 principal 的 tenant_id。
	if err := repositories.ProductRepository.UpdatesByTenant(sqls.DB(), req.ID, tenantID, updates); err != nil {
		return err
	}
	updated := repositories.ProductRepository.GetByTenant(sqls.DB(), req.ID, tenantID)
	if updated == nil {
		return errorsx.InvalidParam("product not found")
	}
	team, syncErr := ProductSupportOrganizationService.EnsureProductRepairTeamDB(sqls.DB(), updated, operator)
	if syncErr != nil {
		return syncErr
	}
	s.recoverProductRepairTeamIfUnavailable(team, time.Now(), "product_updated")
	ProductCenterService.InvalidateProductCountCache(tenantID)
	return s.ensureDefaultProductCustomerAgent(updated, operator)
}

func (s *productService) requireActiveProductOwnerMember(tenantID, ownerMemberID int64) (*models.TenantMember, error) {
	if tenantID <= 0 || ownerMemberID <= 0 {
		return nil, errorsx.InvalidParam("product owner is required")
	}
	member := repositories.EnterpriseIAMRepository.GetTenantMember(sqls.DB(), tenantID, ownerMemberID)
	if member == nil {
		return nil, errorsx.InvalidParam("product owner must be an active enterprise member")
	}
	user := repositories.UserRepository.Get(sqls.DB(), member.UserID)
	if user == nil || user.Status != enums.StatusOk {
		return nil, errorsx.InvalidParam("product owner user is not active")
	}
	return member, nil
}

func (s *productService) ensureDefaultProductCustomerAgent(product *models.Product, operator *dto.AuthPrincipal) error {
	if product == nil || product.ID <= 0 || product.TenantID <= 0 || product.Status != enums.StatusOk {
		return nil
	}
	if !TenantCapabilityService.AIEnabled(product.TenantID) {
		return nil
	}
	db := sqls.DB()
	if db == nil ||
		!db.Migrator().HasTable(&models.ProductServiceProfile{}) ||
		!db.Migrator().HasTable(&models.ProductKnowledgeBinding{}) ||
		!db.Migrator().HasTable(&models.KnowledgeBase{}) ||
		!db.Migrator().HasTable(&models.SkillDefinition{}) ||
		!db.Migrator().HasTable(&models.AIAgent{}) ||
		!db.Migrator().HasTable(&models.AIWorkflow{}) ||
		!db.Migrator().HasTable(&models.AIWorkflowVersion{}) {
		return nil
	}
	agentOperator := operator
	if operator != nil && operator.TenantID != product.TenantID {
		copied := *operator
		if copied.TenantID <= 0 {
			copied.TenantID = product.TenantID
		}
		agentOperator = &copied
	}
	_, err := ProductAIAgentService.EnsureProductCustomerAgent(product.TenantID, product.ID, agentOperator)
	return err
}

func (s *productService) DeleteProduct(id int64, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	tenantID := operator.TenantID
	if tenantID <= 0 {
		return errorsx.InvalidParam("tenantId is required")
	}
	item := repositories.ProductRepository.GetByTenant(sqls.DB(), id, tenantID)
	if item == nil || item.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("product not found")
	}
	modelCount := repositories.ProductModelRepository.Count(sqls.DB(), sqls.NewCnd().
		Eq("product_id", id).
		Eq("tenant_id", tenantID).
		Where("status <> ?", enums.StatusDeleted))
	deviceCount := repositories.DeviceRepository.Count(sqls.DB(), sqls.NewCnd().
		Eq("product_id", id).
		Eq("tenant_id", tenantID).
		Where("status <> ?", enums.StatusDeleted))
	if modelCount > 0 || deviceCount > 0 {
		return errorsx.InvalidParam("product has related models or devices")
	}
	if err := repositories.ProductRepository.UpdatesByTenant(sqls.DB(), id, tenantID, map[string]any{
		"status":           enums.StatusDeleted,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	}); err != nil {
		return err
	}
	item.Status = enums.StatusDeleted
	team, err := ProductSupportOrganizationService.EnsureProductRepairTeamDB(sqls.DB(), item, operator)
	if err == nil {
		s.recoverProductRepairTeamIfUnavailable(team, time.Now(), "product_deleted")
	}
	ProductCenterService.InvalidateProductCountCache(tenantID)
	return err
}

func (s *productService) UpdateStatus(id int64, status int, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if !isMutableStatus(status) {
		return errorsx.InvalidParam("invalid status")
	}
	tenantID := operator.TenantID
	if tenantID <= 0 {
		return errorsx.InvalidParam("tenantId is required")
	}
	item := repositories.ProductRepository.GetByTenant(sqls.DB(), id, tenantID)
	if item == nil || item.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("product not found")
	}
	var team *models.AgentTeam
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := repositories.ProductRepository.UpdatesByTenant(ctx.Tx, id, tenantID, map[string]any{
			"status":           status,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
			"updated_at":       time.Now(),
		}); err != nil {
			return err
		}
		item.Status = enums.Status(status)
		var err error
		team, err = ProductSupportOrganizationService.EnsureProductRepairTeamDB(ctx.Tx, item, operator)
		return err
	}); err != nil {
		return err
	}
	s.recoverProductRepairTeamIfUnavailable(team, time.Now(), "product_status_changed")
	ProductCenterService.InvalidateProductCountCache(tenantID)
	if enums.Status(status) == enums.StatusOk {
		item.Status = enums.StatusOk
		return s.ensureDefaultProductCustomerAgent(item, operator)
	}
	return nil
}

func (s *productService) recoverProductRepairTeamIfUnavailable(team *models.AgentTeam, now time.Time, action string) {
	if team == nil || team.ID <= 0 || team.TenantID <= 0 || team.Status == enums.StatusOk {
		return
	}
	AgentTeamService.recoverWorkAfterTeamDisabled(team.TenantID, team.ID, now, action)
}
