package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"remotehelpdesk/internal/ai/workflow/dsl"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var ProductAIAgentService = &productAIAgentService{}

type productAIAgentService struct{}

var defaultProductAgentSkillRemarks = []string{
	"builtin-aftersales-template:fault-diagnosis",
	"builtin-aftersales-template:safe-stop-escalation",
	"builtin-aftersales-template:operation-guidance",
}

func (s *productAIAgentService) EnsureProductCustomerAgent(
	tenantID, productID int64,
	operator *dto.AuthPrincipal,
) (*models.AIAgent, error) {
	agent, _, err := s.ensureProductCustomerAgent(tenantID, productID, operator)
	return agent, err
}

func (s *productAIAgentService) ProvisionTenantProductAgents(
	tenantID int64,
	operator *dto.AuthPrincipal,
) (*dto.EnterpriseProductAgentProvisionResultDTO, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if tenantID <= 0 || operator.TenantID != tenantID {
		return nil, errorsx.InvalidParam("tenant does not match current operator")
	}
	if !TenantCapabilityService.AIEnabled(tenantID) {
		return nil, errorsx.Forbidden("tenant AI capability is disabled")
	}
	products := repositories.ProductRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("status", enums.StatusOk).
		Asc("id"))
	result := &dto.EnterpriseProductAgentProvisionResultDTO{
		Total:    len(products),
		Failures: make([]dto.EnterpriseProductAgentProvisionFailureDTO, 0),
	}
	for i := range products {
		_, created, err := s.ensureProductCustomerAgent(tenantID, products[i].ID, operator)
		if err != nil {
			result.Failed++
			result.Failures = append(result.Failures, dto.EnterpriseProductAgentProvisionFailureDTO{
				ProductID:   products[i].ID,
				ProductName: products[i].Name,
				Reason:      err.Error(),
			})
			continue
		}
		if created {
			result.Created++
		} else {
			result.Existing++
		}
	}
	return result, nil
}

func (s *productAIAgentService) ensureProductCustomerAgent(
	tenantID, productID int64,
	operator *dto.AuthPrincipal,
) (*models.AIAgent, bool, error) {
	if operator == nil {
		return nil, false, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if tenantID <= 0 || operator.TenantID != tenantID {
		return nil, false, errorsx.InvalidParam("tenant does not match current operator")
	}
	if !TenantCapabilityService.AIEnabled(tenantID) {
		return nil, false, errorsx.Forbidden("tenant AI capability is disabled")
	}
	product := repositories.ProductRepository.GetByTenant(sqls.DB(), productID, tenantID)
	if product == nil || product.Status == enums.StatusDeleted {
		return nil, false, errorsx.InvalidParam("product not found")
	}
	if product.Status != enums.StatusOk {
		return nil, false, errorsx.InvalidParam("product is not active")
	}

	profile := repositories.ProductServiceProfileRepository.GetByProductID(sqls.DB(), productID)
	if profile == nil || profile.Status == enums.StatusDeleted || profile.DefaultKnowledgeBaseID <= 0 {
		locales := []string{strings.TrimSpace(product.DefaultLocale)}
		if locales[0] == "" {
			locales[0] = "en"
		}
		_, err := ProductCenterService.CreateProductKnowledgeBase(tenantID, productID, dto.EnterpriseProductKnowledgeBaseCreateRequest{
			Name:           product.Name + " 知识库",
			Description:    fmt.Sprintf("保存 %s 的手册、FAQ、维修案例和诊断知识，服务团队和产品机器人都从这里查。", product.Name),
			SupportLocales: locales,
		}, operator)
		if err != nil {
			return nil, false, err
		}
		profile = repositories.ProductServiceProfileRepository.GetByProductID(sqls.DB(), productID)
	}
	if profile == nil || profile.TenantID != tenantID || profile.DefaultKnowledgeBaseID <= 0 {
		return nil, false, errorsx.InvalidParam("product knowledge base provisioning failed")
	}
	repairTeam, err := ProductSupportOrganizationService.EnsureProductRepairTeamDB(sqls.DB(), product, operator)
	if err != nil {
		return nil, false, err
	}
	if existing := repositories.AIAgentRepository.Take(sqls.DB(),
		"tenant_id = ? AND product_id = ? AND status <> ?", tenantID, productID, enums.StatusDeleted); existing != nil {
		if err := AIWorkflowService.BindProductAgentWorkflowDB(sqls.DB(), existing, profile); err != nil {
			return nil, false, err
		}
		if err := ProductSupportOrganizationService.EnsureProductAgentTeamBindingDB(sqls.DB(), existing, repairTeam.ID); err != nil {
			return nil, false, err
		}
		if err := s.ensureProfileAgentReference(productID, existing.ID, operator); err != nil {
			return nil, false, err
		}
		if err := s.ensureProductAgentManagedSafetyReleaseDB(existing.ID, operator); err != nil {
			return nil, false, err
		}
		if err := s.ensureProductAgentInitialReleaseDB(existing.ID, operator); err != nil {
			return nil, false, err
		}
		if refreshed := repositories.AIAgentRepository.Get(sqls.DB(), existing.ID); refreshed != nil {
			existing = refreshed
		}
		return existing, false, nil
	}

	agentName := fmt.Sprintf("%s 客服机器人", product.Name)
	if duplicate := repositories.AIAgentRepository.Take(sqls.DB(),
		"tenant_id = ? AND name = ? AND status <> ?", tenantID, agentName, enums.StatusDeleted); duplicate != nil {
		agentName = fmt.Sprintf("%s 客服机器人 (%s)", product.Name, product.Code)
	}
	agent, err := AIAgentService.CreateProductAIAgent(tenantID, productID, request.CreateAIAgentRequest{
		Name:                agentName,
		Description:         fmt.Sprintf("%s 的售后客服机器人，创建后自动部署；后续配置变更需提交审核。", product.Name),
		ServiceMode:         enums.IMConversationServiceModeAIFirst,
		SystemPrompt:        defaultProductAgentSystemPrompt(product),
		WelcomeMessage:      defaultProductAgentWelcomeMessage(product),
		ReplyTimeoutSeconds: 180,
		TeamIDs:             []int64{repairTeam.ID},
		HandoffMode:         enums.AIAgentHandoffModeDefaultTeamPool,
		FallbackMode:        enums.AIAgentFallbackModeSuggestRetry,
		FallbackMessage:     "当前知识不足以给出可靠结论，我会记录现象并协助转接人工工程师。",
		KnowledgeIDs:        []int64{profile.DefaultKnowledgeBaseID},
		SkillIDs:            s.defaultProductAgentSkillIDs(sqls.DB()),
	}, operator)
	if err != nil {
		return nil, false, err
	}
	if err := s.ensureProductAgentInitialReleaseDB(agent.ID, operator); err != nil {
		return nil, false, err
	}
	if refreshed := repositories.AIAgentRepository.Get(sqls.DB(), agent.ID); refreshed != nil {
		agent = refreshed
	}
	return agent, true, nil
}

func (s *productAIAgentService) defaultProductAgentSkillIDs(db *gorm.DB) []int64 {
	if db == nil {
		return nil
	}
	items := repositories.SkillDefinitionRepository.Find(db, sqls.NewCnd().
		Eq("tenant_id", 0).
		Eq("status", enums.StatusOk).
		In("remark", defaultProductAgentSkillRemarks).
		Asc("id"))
	ret := make([]int64, 0, len(items))
	for i := range items {
		ret = append(ret, items[i].ID)
	}
	return ret
}

func defaultProductAgentWelcomeMessage(product *models.Product) string {
	productName := "this product"
	if product != nil && strings.TrimSpace(product.Name) != "" {
		productName = strings.TrimSpace(product.Name)
	}
	return fmt.Sprintf("Hello, I'm the after-sales support assistant for %s. Please describe the device issue, model, and troubleshooting steps you've already tried.", productName)
}

func (s *productAIAgentService) BindDefaultProductAgentSkills(db *gorm.DB) error {
	return repositories.AIAgentRepository.BindDefaultSkillIDsToEmptyProductAgents(
		db,
		utils.JoinInt64s(s.defaultProductAgentSkillIDs(db)),
		time.Now(),
	)
}

func (s *productAIAgentService) ensureProfileAgentReference(productID, agentID int64, operator *dto.AuthPrincipal) error {
	profile := repositories.ProductServiceProfileRepository.GetByProductID(sqls.DB(), productID)
	if profile == nil || profile.DefaultAIAgentID == agentID {
		return nil
	}
	return repositories.ProductServiceProfileRepository.Updates(sqls.DB(), profile.ID, map[string]any{
		"default_ai_agent_id": agentID,
		"update_user_id":      operator.UserID,
		"update_user_name":    operator.Username,
		"updated_at":          time.Now(),
	})
}

// ensureProductAgentInitialReleaseDB makes the system-created product agent
// runnable immediately. Manual agents continue to use the explicit review and
// deployment lifecycle in AIAgentReleaseService.
func (s *productAIAgentService) ensureProductAgentInitialReleaseDB(agentID int64, operator *dto.AuthPrincipal) error {
	if agentID <= 0 || operator == nil {
		return nil
	}
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		agent := repositories.AIAgentRepository.GetForUpdate(ctx.Tx, agentID)
		if agent == nil || agent.Status == enums.StatusDeleted || agent.Source != "product_auto" || agent.ProductID <= 0 {
			return nil
		}
		if agent.ActiveReleaseID > 0 {
			active := repositories.AIAgentReleaseRepository.Get(ctx.Tx, agent.ActiveReleaseID)
			if active != nil &&
				active.AgentID == agent.ID &&
				active.TenantID == agent.TenantID &&
				active.ProductID == agent.ProductID &&
				active.Status == enums.StatusOk &&
				active.ReviewStatus == enums.AIAgentReviewStatusApproved &&
				active.DeploymentStatus == models.AIAgentReleaseDeploymentActive {
				return nil
			}
		}

		workflow, version, definition, err := AIAgentReleaseService.resolveReleaseWorkflow(ctx.Tx, agent)
		if err != nil {
			return err
		}
		agentSnapshot, agentHash, err := buildAgentReleaseConfigSnapshotForDefinition(agent, &definition)
		if err != nil {
			return err
		}
		knowledgeSnapshot, knowledgeHash, err := AIAgentReleaseService.buildKnowledgeScopeSnapshot(ctx.Tx, agent)
		if err != nil {
			return err
		}
		existing := repositories.AIAgentReleaseRepository.Find(ctx.Tx, sqls.NewCnd().
			Eq("tenant_id", agent.TenantID).
			Eq("agent_id", agent.ID).
			Eq("workflow_version_id", version.ID).
			Eq("workflow_definition_hash", version.DefinitionHash).
			Eq("agent_config_hash", agentHash).
			Eq("knowledge_scope_hash", knowledgeHash).
			NotEq("status", enums.StatusDeleted).
			Desc("release_no"))
		now := time.Now()
		reviewComment := "产品创建时自动部署默认售后接待机器人"
		if len(existing) > 0 {
			return s.activateProductManagedReleaseDB(ctx.Tx, agent, &existing[0], now, reviewComment, operator)
		}

		release := &models.AIAgentRelease{
			TenantID:               agent.TenantID,
			ProductID:              agent.ProductID,
			AgentID:                agent.ID,
			ReleaseNo:              repositories.AIAgentReleaseRepository.MaxReleaseNo(ctx.Tx, agent.ID) + 1,
			WorkflowID:             workflow.ID,
			WorkflowVersionID:      version.ID,
			WorkflowDefinitionHash: version.DefinitionHash,
			AgentConfigSnapshot:    agentSnapshot,
			AgentConfigHash:        agentHash,
			KnowledgeScopeSnapshot: knowledgeSnapshot,
			KnowledgeScopeHash:     knowledgeHash,
			ReviewStatus:           enums.AIAgentReviewStatusApproved,
			ReviewComment:          reviewComment,
			ReviewedAt:             &now,
			ReviewedByID:           auditUserID(operator),
			ReviewedByName:         auditUserName(operator),
			DeploymentStatus:       models.AIAgentReleaseDeploymentInactive,
			Status:                 enums.StatusOk,
			AuditFields:            utils.BuildAuditFields(operator),
		}
		if err := repositories.AIAgentReleaseRepository.Create(ctx.Tx, release); err != nil {
			return err
		}
		return s.activateProductManagedReleaseDB(ctx.Tx, agent, release, now, reviewComment, operator)
	})
}

func (s *productAIAgentService) ensureProductAgentManagedSafetyReleaseDB(agentID int64, operator *dto.AuthPrincipal) error {
	if agentID <= 0 {
		return nil
	}
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		agent := repositories.AIAgentRepository.GetForUpdate(ctx.Tx, agentID)
		if agent == nil || agent.Status == enums.StatusDeleted {
			return nil
		}
		if agent.Source != "product_auto" || agent.ProductID <= 0 {
			return nil
		}
		workflow := repositories.AIWorkflowRepository.Get(ctx.Tx, agent.WorkflowID)
		version := repositories.AIWorkflowVersionRepository.Get(ctx.Tx, agent.WorkflowVersionID)
		if workflow == nil || version == nil ||
			workflow.Status == enums.StatusDeleted ||
			version.Status != enums.StatusOk ||
			version.WorkflowID != workflow.ID ||
			workflow.Code != PlatformDefaultAfterSalesWorkflowCode ||
			!IsPlatformBuiltInWorkflow(workflow) {
			return nil
		}
		if !s.productActiveReleaseNeedsSafetyReplacement(ctx.Tx, agent) {
			return nil
		}
		if defaultAfterSalesWorkflowAutoHandoffOnKnowledgeFailure(version) {
			return errorsx.InvalidParam("platform default aftersales workflow still allows automatic handoff on diagnosis failure")
		}
		definition := dsl.Definition{}
		if err := json.Unmarshal([]byte(version.Definition), &definition); err != nil || !AIWorkflowService.ValidateDefinition(definition).Valid {
			return errorsx.InvalidParam("platform default aftersales workflow definition is invalid")
		}
		agentSnapshot, agentHash, err := buildAgentReleaseConfigSnapshotForDefinition(agent, &definition)
		if err != nil {
			return err
		}
		knowledgeSnapshot, knowledgeHash, err := AIAgentReleaseService.buildKnowledgeScopeSnapshot(ctx.Tx, agent)
		if err != nil {
			return err
		}
		existing := repositories.AIAgentReleaseRepository.Find(ctx.Tx, sqls.NewCnd().
			Eq("tenant_id", agent.TenantID).
			Eq("agent_id", agent.ID).
			Eq("workflow_version_id", version.ID).
			Eq("workflow_definition_hash", version.DefinitionHash).
			Eq("agent_config_hash", agentHash).
			Eq("knowledge_scope_hash", knowledgeHash).
			NotEq("status", enums.StatusDeleted).
			Desc("release_no"))
		now := time.Now()
		reviewComment := "平台内置产品售后流程已同步为知识失败安全收口"
		if len(existing) > 0 {
			return s.activateProductManagedReleaseDB(ctx.Tx, agent, &existing[0], now, reviewComment, operator)
		}
		release := &models.AIAgentRelease{
			TenantID:               agent.TenantID,
			ProductID:              agent.ProductID,
			AgentID:                agent.ID,
			ReleaseNo:              repositories.AIAgentReleaseRepository.MaxReleaseNo(ctx.Tx, agent.ID) + 1,
			WorkflowID:             workflow.ID,
			WorkflowVersionID:      version.ID,
			WorkflowDefinitionHash: version.DefinitionHash,
			AgentConfigSnapshot:    agentSnapshot,
			AgentConfigHash:        agentHash,
			KnowledgeScopeSnapshot: knowledgeSnapshot,
			KnowledgeScopeHash:     knowledgeHash,
			ReviewStatus:           enums.AIAgentReviewStatusApproved,
			ReviewComment:          reviewComment,
			ReviewedAt:             &now,
			ReviewedByID:           auditUserID(operator),
			ReviewedByName:         auditUserName(operator),
			DeploymentStatus:       models.AIAgentReleaseDeploymentInactive,
			Status:                 enums.StatusOk,
			AuditFields:            utils.BuildAuditFields(operator),
		}
		if err := repositories.AIAgentReleaseRepository.Create(ctx.Tx, release); err != nil {
			return err
		}
		return s.activateProductManagedReleaseDB(ctx.Tx, agent, release, now, reviewComment, operator)
	})
}

func (s *productAIAgentService) productActiveReleaseNeedsSafetyReplacement(db *gorm.DB, agent *models.AIAgent) bool {
	if db == nil || agent == nil || agent.ActiveReleaseID <= 0 {
		return false
	}
	active := repositories.AIAgentReleaseRepository.Get(db, agent.ActiveReleaseID)
	if active == nil ||
		active.AgentID != agent.ID ||
		active.TenantID != agent.TenantID ||
		active.ProductID != agent.ProductID ||
		active.Status != enums.StatusOk ||
		active.DeploymentStatus != models.AIAgentReleaseDeploymentActive ||
		active.ReviewStatus != enums.AIAgentReviewStatusApproved {
		return false
	}
	activeWorkflow := repositories.AIWorkflowRepository.Get(db, active.WorkflowID)
	activeVersion := repositories.AIWorkflowVersionRepository.Get(db, active.WorkflowVersionID)
	if activeWorkflow == nil || activeVersion == nil ||
		activeWorkflow.Code != PlatformDefaultAfterSalesWorkflowCode ||
		!IsPlatformBuiltInWorkflow(activeWorkflow) ||
		activeVersion.WorkflowID != activeWorkflow.ID {
		return false
	}
	return defaultAfterSalesWorkflowAutoHandoffOnKnowledgeFailure(activeVersion)
}

func (s *productAIAgentService) activateProductManagedReleaseDB(db *gorm.DB, agent *models.AIAgent, release *models.AIAgentRelease, now time.Time, reviewComment string, operator *dto.AuthPrincipal) error {
	if db == nil || agent == nil || release == nil {
		return errorsx.InvalidParam("product AI agent release is invalid")
	}
	if release.AgentID != agent.ID || release.TenantID != agent.TenantID || release.ProductID != agent.ProductID {
		return errorsx.InvalidParam("product AI agent release scope is invalid")
	}
	active := repositories.AIAgentReleaseRepository.Find(db, sqls.NewCnd().
		Eq("agent_id", agent.ID).
		Eq("deployment_status", models.AIAgentReleaseDeploymentActive).
		NotEq("id", release.ID))
	for i := range active {
		if err := repositories.AIAgentReleaseRepository.Updates(db, active[i].ID, map[string]any{
			"deployment_status": models.AIAgentReleaseDeploymentRetired,
			"update_user_id":    auditUserID(operator),
			"update_user_name":  auditUserName(operator),
			"updated_at":        now,
		}); err != nil {
			return err
		}
	}
	if strings.TrimSpace(reviewComment) == "" {
		reviewComment = "平台内置产品售后流程已同步为知识失败安全收口"
	}
	if err := repositories.AIAgentReleaseRepository.Updates(db, release.ID, map[string]any{
		"review_status":     enums.AIAgentReviewStatusApproved,
		"review_comment":    reviewComment,
		"reviewed_at":       &now,
		"reviewed_by_id":    auditUserID(operator),
		"reviewed_by_name":  auditUserName(operator),
		"deployment_status": models.AIAgentReleaseDeploymentActive,
		"deployed_at":       &now,
		"deployed_by_id":    auditUserID(operator),
		"deployed_by_name":  auditUserName(operator),
		"update_user_id":    auditUserID(operator),
		"update_user_name":  auditUserName(operator),
		"updated_at":        now,
	}); err != nil {
		return err
	}
	return repositories.AIAgentRepository.Updates(db, agent.ID, map[string]any{
		"active_release_id": release.ID,
		"status":            enums.StatusOk,
		"review_status":     enums.AIAgentReviewStatusApproved,
		"review_comment":    reviewComment,
		"reviewed_at":       &now,
		"reviewed_by_id":    auditUserID(operator),
		"reviewed_by_name":  auditUserName(operator),
		"update_user_id":    auditUserID(operator),
		"update_user_name":  auditUserName(operator),
		"updated_at":        now,
	})
}

func defaultAfterSalesWorkflowAutoHandoffOnKnowledgeFailure(version *models.AIWorkflowVersion) bool {
	if version == nil || strings.TrimSpace(version.Definition) == "" {
		return true
	}
	definition := dsl.Definition{}
	if err := json.Unmarshal([]byte(version.Definition), &definition); err != nil {
		return true
	}
	unsafeSources := map[string]bool{
		"answerability_route_1":     true,
		"diagnosis_failure_route_1": true,
	}
	for _, node := range definition.Nodes {
		if !unsafeSources[node.ID] || node.Type != "condition" {
			continue
		}
		config := dsl.ConditionConfig{}
		if err := json.Unmarshal(node.Config, &config); err != nil {
			return true
		}
		for _, branch := range config.Branches {
			if branch.TargetNodeID == "handoff_1" {
				return true
			}
		}
	}
	for _, edge := range definition.Edges {
		if unsafeSources[edge.Source] && edge.Target == "handoff_1" {
			return true
		}
	}
	return false
}

func defaultProductAgentSystemPrompt(product *models.Product) string {
	return fmt.Sprintf(`你是 %s（%s）的产品售后服务助手。
以绑定的产品知识库作为主要事实来源。
始终使用客户原始最新消息的主要语言回答；客户使用英文时必须完整使用英文，不要因为系统提示或知识片段是中文而切回中文。
当上下文不完整时，先追问设备型号、序列号、故障现象、故障码、所在地区和已尝试的排障步骤。
不要编造规格参数、安全说明、保修条款或维修流程。
回答直接、简洁，不重复问候或用户已经提供的信息；不要使用 Emoji 或装饰性小标题。
涉及电气、液压、压力、运动部件等安全关键操作时，先说明隔离防护和升级处理要求，再继续排障。
证据不足或问题需要授权处理时，转交人工工程师，并附上简洁的问题摘要。`, product.Name, product.Code)
}
