package services

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"remotehelpdesk/internal/ai"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/toolx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/mlogclub/simple/sqls"
)

var AIAgentService = newAIAgentService()

func newAIAgentService() *aIAgentService {
	return &aIAgentService{}
}

type aIAgentService struct {
}

func (s *aIAgentService) Get(id int64) *models.AIAgent {
	if id <= 0 {
		return nil
	}
	return repositories.AIAgentRepository.Get(sqls.DB(), id)
}

func (s *aIAgentService) GetForOperator(id int64, operator *dto.AuthPrincipal) *models.AIAgent {
	item := s.Get(id)
	if item == nil || !canManageAIAgent(item, operator) {
		return nil
	}
	return item
}

func (s *aIAgentService) Take(where ...interface{}) *models.AIAgent {
	return repositories.AIAgentRepository.Take(sqls.DB(), where...)
}

func (s *aIAgentService) Find(cnd *sqls.Cnd) []models.AIAgent {
	return repositories.AIAgentRepository.Find(sqls.DB(), cnd)
}

func (s *aIAgentService) FindOne(cnd *sqls.Cnd) *models.AIAgent {
	return repositories.AIAgentRepository.FindOne(sqls.DB(), cnd)
}

func (s *aIAgentService) FindPageByParams(params *params.QueryParams) (list []models.AIAgent, paging *sqls.Paging) {
	return repositories.AIAgentRepository.FindPageByParams(sqls.DB(), params)
}

func (s *aIAgentService) FindPageByCnd(cnd *sqls.Cnd) (list []models.AIAgent, paging *sqls.Paging) {
	return repositories.AIAgentRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *aIAgentService) Count(cnd *sqls.Cnd) int64 {
	return repositories.AIAgentRepository.Count(sqls.DB(), cnd)
}

func (s *aIAgentService) FindByIds(ids []int64) []models.AIAgent {
	return repositories.AIAgentRepository.FindByIds(sqls.DB(), ids)
}

func (s *aIAgentService) CreateAIAgent(req request.CreateAIAgentRequest, operator *dto.AuthPrincipal) (*models.AIAgent, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item, err := s.buildAIAgentModel(0, operator.TenantID, req)
	if err != nil {
		return nil, err
	}
	item.TenantID = operator.TenantID
	item.Source = "manual"
	item.Status = enums.StatusOk
	item.ReviewStatus = enums.AIAgentReviewStatusApproved
	if item.TenantID > 0 {
		item.Status = enums.StatusDisabled
		item.ReviewStatus = enums.AIAgentReviewStatusUnreviewed
	}
	item.SortNo = 0
	item.AuditFields = utils.BuildAuditFields(operator)
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := repositories.AIAgentRepository.Create(ctx.Tx, item); err != nil {
			return err
		}
		workflow, stableVersion, err := AIWorkflowService.EnsurePlatformDefaultWorkflowDB(ctx.Tx)
		if err != nil {
			return err
		}
		item.WorkflowID = workflow.ID
		item.WorkflowVersionID = stableVersion.ID
		return repositories.AIAgentRepository.Updates(ctx.Tx, item.ID, map[string]any{
			"workflow_id":         workflow.ID,
			"workflow_version_id": stableVersion.ID,
		})
	}); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *aIAgentService) CreateProductAIAgent(
	tenantID, productID int64,
	req request.CreateAIAgentRequest,
	operator *dto.AuthPrincipal,
) (*models.AIAgent, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if tenantID <= 0 || operator.TenantID != tenantID {
		return nil, errorsx.InvalidParam("tenant does not match current operator")
	}
	product := repositories.ProductRepository.GetByTenant(sqls.DB(), productID, tenantID)
	if product == nil || product.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParam("product not found")
	}
	if product.Status != enums.StatusOk {
		return nil, errorsx.InvalidParam("product is not active")
	}
	if existing := repositories.AIAgentRepository.Take(sqls.DB(),
		"tenant_id = ? AND product_id = ? AND status <> ?", tenantID, productID, enums.StatusDeleted); existing != nil {
		return existing, nil
	}

	item, err := s.buildAIAgentModel(0, tenantID, req)
	if err != nil {
		return nil, err
	}
	item.TenantID = tenantID
	item.ProductID = productID
	item.Source = "product_auto"
	item.Status = enums.StatusDisabled
	item.ReviewStatus = enums.AIAgentReviewStatusUnreviewed
	item.DraftRevision = 1
	item.SortNo = 0
	item.AuditFields = utils.BuildAuditFields(operator)

	err = sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := repositories.AIAgentRepository.Create(ctx.Tx, item); err != nil {
			return err
		}
		profile := repositories.ProductServiceProfileRepository.GetByProductID(ctx.Tx, productID)
		if profile == nil || profile.TenantID != tenantID || profile.Status == enums.StatusDeleted {
			return errorsx.InvalidParam("product service profile not found")
		}
		if err := AIWorkflowService.BindProductAgentWorkflowDB(ctx.Tx, item, profile); err != nil {
			return err
		}
		return repositories.ProductServiceProfileRepository.Updates(ctx.Tx, profile.ID, map[string]any{
			"default_ai_agent_id": item.ID,
			"update_user_id":      operator.UserID,
			"update_user_name":    operator.Username,
			"updated_at":          time.Now(),
		})
	})
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (s *aIAgentService) UpdateAIAgent(req request.UpdateAIAgentRequest, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	current := s.GetForOperator(req.ID, operator)
	if current == nil {
		return errorsx.InvalidParamI18n("error.e0002")
	}
	if current.Source == TenantDefaultAIAgentSource {
		return s.updateTenantDefaultAIAgentModelDraft(current, req, operator)
	}
	if current.ProductID > 0 {
		product := repositories.ProductRepository.GetByTenant(sqls.DB(), current.ProductID, current.TenantID)
		if product == nil || product.Status == enums.StatusDeleted {
			return errorsx.InvalidParam("product not found")
		}
		if product.Status != enums.StatusOk {
			return errorsx.InvalidParam("product is not active")
		}
		team, err := ProductSupportOrganizationService.EnsureProductRepairTeamDB(sqls.DB(), product, operator)
		if err != nil {
			return err
		}
		if !slices.Contains(req.TeamIDs, team.ID) {
			req.TeamIDs = append([]int64{team.ID}, req.TeamIDs...)
		}
	}
	item, err := s.buildAIAgentModel(req.ID, current.TenantID, req.CreateAIAgentRequest)
	if err != nil {
		return err
	}
	workflowRestrictions := make(map[string]any)
	if current.WorkflowVersionID > 0 {
		version := repositories.AIWorkflowVersionRepository.Get(sqls.DB(), current.WorkflowVersionID)
		workflowRestrictions, err = workflowAgentRestrictionUpdates(item, version)
		if err != nil {
			return err
		}
		applyWorkflowAgentRestrictions(item, workflowRestrictions)
	}
	updates := map[string]any{
		"name":                  item.Name,
		"description":           item.Description,
		"ai_config_id":          item.AIConfigID,
		"llm_model_name":        item.LLMModelName,
		"service_mode":          item.ServiceMode,
		"system_prompt":         item.SystemPrompt,
		"welcome_message":       item.WelcomeMessage,
		"reply_timeout_seconds": item.ReplyTimeoutSeconds,
		"team_ids":              item.TeamIDs,
		"handoff_mode":          item.HandoffMode,
		"fallback_mode":         item.FallbackMode,
		"fallback_message":      item.FallbackMessage,
		"knowledge_ids":         item.KnowledgeIDs,
		"skill_ids":             item.SkillIDs,
		"allowed_mcp_tools":     item.AllowedMCPTools,
		"allowed_graph_tools":   item.AllowedGraphTools,
		"draft_revision":        current.DraftRevision + 1,
		"update_user_id":        operator.UserID,
		"update_user_name":      operator.Username,
		"updated_at":            time.Now(),
	}
	for key, value := range workflowRestrictions {
		updates[key] = value
	}
	if current.ProductID > 0 && current.ActiveReleaseID <= 0 {
		updates["status"] = enums.StatusDisabled
		updates["review_status"] = enums.AIAgentReviewStatusUnreviewed
		updates["review_comment"] = ""
		updates["reviewed_at"] = nil
		updates["reviewed_by_id"] = 0
		updates["reviewed_by_name"] = ""
	}
	return repositories.AIAgentRepository.Updates(sqls.DB(), req.ID, updates)
}

func (s *aIAgentService) updateTenantDefaultAIAgentModelDraft(
	current *models.AIAgent,
	req request.UpdateAIAgentRequest,
	operator *dto.AuthPrincipal,
) error {
	if current == nil {
		return errorsx.InvalidParamI18n("error.e0002")
	}
	aiConfigID := req.AIConfigID
	if aiConfigID > 0 {
		aiConfig := AIConfigService.Get(aiConfigID)
		if aiConfig == nil {
			return errorsx.InvalidParamI18n("error.e0009")
		}
		if aiConfig.Status != enums.StatusOk {
			return errorsx.InvalidParamI18n("error.e0011")
		}
	}
	llmModelName := strings.TrimSpace(req.LLMModelName)
	if aiConfigID > 0 {
		llmModelName = ""
	}
	if len(llmModelName) > 100 {
		return errorsx.InvalidParam("llmModelName must not exceed 100 characters")
	}
	if err := s.validateTenantLLMModelName(current.TenantID, llmModelName); err != nil {
		return err
	}
	if current.AIConfigID == aiConfigID && current.LLMModelName == llmModelName {
		return nil
	}
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := repositories.AIAgentRepository.Updates(ctx.Tx, current.ID, map[string]any{
			"ai_config_id":     aiConfigID,
			"llm_model_name":   llmModelName,
			"draft_revision":   current.DraftRevision + 1,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
			"updated_at":       time.Now(),
		}); err != nil {
			return err
		}
		updated := repositories.AIAgentRepository.Get(ctx.Tx, current.ID)
		if updated == nil {
			return errorsx.InvalidParam("tenant default AI agent does not exist")
		}
		workflow := repositories.AIWorkflowRepository.Get(ctx.Tx, updated.WorkflowID)
		version := repositories.AIWorkflowVersionRepository.Get(ctx.Tx, updated.WorkflowVersionID)
		if workflow == nil || version == nil {
			return errorsx.InvalidParam("tenant default AI agent workflow is unavailable")
		}
		return TenantDefaultAIAgentService.ensureActiveRelease(ctx.Tx, updated, workflow, version, operator)
	})
}

func (s *aIAgentService) DeleteAIAgent(id int64, operator *dto.AuthPrincipal) error {
	current := s.GetForOperator(id, operator)
	if current == nil {
		return errorsx.InvalidParamI18n("error.e0002")
	}
	if current.Source == TenantDefaultAIAgentSource {
		return errorsx.Forbidden("tenant default AI agent cannot be deleted")
	}
	if ChannelService.Take("ai_agent_id = ?", id) != nil {
		return errorsx.ForbiddenI18n("error.e0185")
	}
	return repositories.AIAgentRepository.Updates(sqls.DB(), id, map[string]any{
		"status":           enums.StatusDeleted,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	})
}

func (s *aIAgentService) buildAIAgentModel(id, tenantID int64, req request.CreateAIAgentRequest) (*models.AIAgent, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errorsx.InvalidParamI18n("error.e0005")
	}
	if exists := s.Take("tenant_id = ? AND name = ? AND id <> ? AND status <> ?", tenantID, name, id, enums.StatusDeleted); exists != nil {
		return nil, errorsx.InvalidParamI18n("error.e0006")
	}
	if req.AIConfigID > 0 {
		aiConfig := AIConfigService.Get(req.AIConfigID)
		if aiConfig == nil {
			return nil, errorsx.InvalidParamI18n("error.e0009")
		}
		if aiConfig.Status != enums.StatusOk {
			return nil, errorsx.InvalidParamI18n("error.e0011")
		}
	}
	llmModelName := strings.TrimSpace(req.LLMModelName)
	if req.AIConfigID > 0 {
		llmModelName = ""
	}
	if len(llmModelName) > 100 {
		return nil, errorsx.InvalidParam("llmModelName must not exceed 100 characters")
	}
	if err := s.validateTenantLLMModelName(tenantID, llmModelName); err != nil {
		return nil, err
	}
	if !slices.Contains(enums.IMConversationServiceModeValues, req.ServiceMode) {
		return nil, errorsx.InvalidParamI18n("error.e0230")
	}
	teamIDs, err := s.normalizeTeamIDs(req.TeamIDs, tenantID)
	if err != nil {
		return nil, err
	}

	if !slices.Contains(enums.AIAgentHandoffModeValues, enums.AIAgentHandoffMode(req.HandoffMode)) {
		return nil, errorsx.InvalidParamI18n("error.e0336")
	}
	if req.FallbackMode == 0 {
		req.FallbackMode = enums.AIAgentFallbackModeNoAnswer
	}
	if !slices.Contains(enums.AIAgentFallbackModeValues, enums.AIAgentFallbackMode(req.FallbackMode)) {
		return nil, errorsx.InvalidParamI18n("error.e0123")
	}
	if enums.AIAgentHandoffMode(req.HandoffMode) == enums.AIAgentHandoffModeDefaultTeamPool && len(teamIDs) == 0 {
		return nil, errorsx.InvalidParamI18n("error.e0347")
	}
	if req.ReplyTimeoutSeconds < 0 {
		return nil, errorsx.InvalidParamI18n("error.e0144")
	}

	knowledgeIDs, err := s.normalizeKnowledgeIDs(req.KnowledgeIDs, tenantID)
	if err != nil {
		return nil, err
	}
	if len(knowledgeIDs) == 0 {
		return nil, errorsx.InvalidParamI18n("error.e0320")
	}
	skillIDs, err := s.normalizeSkillIDs(req.SkillIDs)
	if err != nil {
		return nil, err
	}
	directTools, err := s.normalizeDirectTools(req.DirectTools)
	if err != nil {
		return nil, err
	}
	graphTools, err := s.normalizeGraphTools(req.GraphTools)
	if err != nil {
		return nil, err
	}
	directToolsJSON := ""
	if len(directTools) > 0 {
		buf, marshalErr := json.Marshal(directTools)
		if marshalErr != nil {
			return nil, errorsx.InvalidParamI18n("error.e0021")
		}
		directToolsJSON = string(buf)
	}
	graphToolsJSON := ""
	if len(graphTools) > 0 {
		buf, marshalErr := json.Marshal(graphTools)
		if marshalErr != nil {
			return nil, errorsx.InvalidParamI18n("error.e0028")
		}
		graphToolsJSON = string(buf)
	}
	return &models.AIAgent{
		Name:                name,
		Description:         strings.TrimSpace(req.Description),
		AIConfigID:          req.AIConfigID,
		LLMModelName:        llmModelName,
		ServiceMode:         req.ServiceMode,
		SystemPrompt:        strings.TrimSpace(req.SystemPrompt),
		WelcomeMessage:      strings.TrimSpace(req.WelcomeMessage),
		ReplyTimeoutSeconds: req.ReplyTimeoutSeconds,
		TeamIDs:             utils.JoinInt64s(teamIDs),
		HandoffMode:         req.HandoffMode,
		FallbackMode:        req.FallbackMode,
		FallbackMessage:     strings.TrimSpace(req.FallbackMessage),
		KnowledgeIDs:        utils.JoinInt64s(knowledgeIDs),
		SkillIDs:            utils.JoinInt64s(skillIDs),
		AllowedMCPTools:     directToolsJSON,
		AllowedGraphTools:   graphToolsJSON,
		WorkflowVersionID:   0,
	}, nil
}

func (s *aIAgentService) validateTenantLLMModelName(tenantID int64, modelName string) error {
	modelName = strings.TrimSpace(modelName)
	if tenantID <= 0 || modelName == "" {
		return nil
	}
	route, err := ai.ResolveCapabilityRouteForScope(context.Background(), enums.AIModelTypeLLM, ai.CapabilityScope{TenantID: tenantID})
	if err != nil {
		return err
	}
	if route == nil || route.Source != ai.CapabilitySourceSub2API {
		return nil
	}
	catalogModels, err := ai.ListCapabilityModels(context.Background(), route)
	if err != nil {
		return errorsx.InvalidParam("tenant LLM model catalog is unavailable")
	}
	if len(catalogModels) > 0 && !containsStringFold(catalogModels, modelName) {
		return errorsx.InvalidParam("llmModelName is not in the tenant model catalog")
	}
	return nil
}

func (s *aIAgentService) normalizeTeamIDs(input []int64, tenantID int64) ([]int64, error) {
	ret := make([]int64, 0, len(input))
	seen := make(map[int64]struct{})
	for _, id := range input {
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		team := AgentTeamService.Get(id)
		if team == nil || team.Status == enums.StatusDeleted || (tenantID > 0 && team.TenantID != tenantID) {
			continue
		}
		if team.Status != enums.StatusOk {
			return nil, errorsx.InvalidParam("agent team is not active")
		}
		seen[id] = struct{}{}
		ret = append(ret, id)
	}
	slices.Sort(ret)
	return ret, nil
}

func (s *aIAgentService) normalizeKnowledgeIDs(input []int64, tenantID int64) ([]int64, error) {
	ret := make([]int64, 0, len(input))
	seen := make(map[int64]struct{})
	for _, id := range input {
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		kb := KnowledgeBaseService.Get(id)
		if kb == nil || kb.Status == enums.StatusDeleted || (tenantID > 0 && kb.TenantID != tenantID) {
			continue
		}
		// if kb.Status != enums.StatusOk {
		// 	return nil, errorsx.InvalidParamI18n("error.e0285")
		// }
		seen[id] = struct{}{}
		ret = append(ret, id)
	}
	return ret, nil
}

func (s *aIAgentService) normalizeSkillIDs(input []int64) ([]int64, error) {
	ret := make([]int64, 0, len(input))
	seen := make(map[int64]struct{})
	for _, id := range input {
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		skill := SkillDefinitionService.Get(id)
		if skill == nil || skill.Status == enums.StatusDeleted {
			continue
		}
		// if skill.Status != enums.StatusOk {
		// 	return nil, errorsx.InvalidParamI18n("error.e0056")
		// }
		seen[id] = struct{}{}
		ret = append(ret, id)
	}
	return ret, nil
}

func (s *aIAgentService) normalizeDirectTools(input []request.AIAgentMCPToolRequest) ([]request.AIAgentMCPToolRequest, error) {
	if len(input) == 0 {
		return nil, nil
	}
	ret := make([]request.AIAgentMCPToolRequest, 0, len(input))
	seen := make(map[string]struct{})
	for _, item := range input {
		normalized, err := toolx.NormalizeMCPToolRequest(item)
		if err != nil {
			return nil, err
		}
		if toolx.IsAutoInjectedToolCode(strings.TrimSpace(normalized.ToolCode)) {
			continue
		}
		if toolx.ResolveToolSourceType(normalized.ToolCode) != enums.ToolSourceTypeMCP {
			return nil, errorsx.InvalidParamI18n("error.e0020")
		}
		if err := ToolCatalogService.ValidateToolCode(normalized.ToolCode); err != nil {
			return nil, err
		}
		key := strings.TrimSpace(normalized.ToolCode)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		ret = append(ret, normalized)
	}
	return ret, nil
}

func (s *aIAgentService) normalizeGraphTools(input []string) ([]string, error) {
	if len(input) == 0 {
		return nil, nil
	}
	ret := make([]string, 0, len(input))
	seen := make(map[string]struct{})
	for _, item := range input {
		toolCode := toolx.NormalizeToolCodeAlias(strings.TrimSpace(item))
		if toolCode == "" {
			continue
		}
		if !toolx.IsAgentDirectGraphToolCode(toolCode) {
			return nil, errorsx.InvalidParamI18n("error.e0027")
		}
		if _, exists := seen[toolCode]; exists {
			continue
		}
		seen[toolCode] = struct{}{}
		ret = append(ret, toolCode)
	}
	return ret, nil
}

func (s *aIAgentService) UpdateSort(ids []int64, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		for i, id := range ids {
			if s.GetForOperator(id, operator) == nil {
				return errorsx.InvalidParam("agent not found")
			}
			if err := repositories.AIAgentRepository.UpdateColumn(ctx.Tx, id, "sort_no", i+1); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *aIAgentService) UpdateStatus(id int64, status int, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	current := s.GetForOperator(id, operator)
	if current == nil {
		return errorsx.InvalidParamI18n("error.e0002")
	}
	if status != int(enums.StatusOk) && status != int(enums.StatusDisabled) {
		return errorsx.InvalidParamI18n("error.e0254")
	}
	if status == int(enums.StatusOk) && current.ProductID > 0 {
		if current.ReviewStatus != enums.AIAgentReviewStatusApproved {
			return errorsx.InvalidParam("agent must be approved before it can be enabled")
		}
		if current.WorkflowVersionID <= 0 {
			return errorsx.InvalidParam("agent workflow must be published before it can be enabled")
		}
	}

	return repositories.AIAgentRepository.Updates(sqls.DB(), id, map[string]any{
		"status":           status,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	})
}

func (s *aIAgentService) SubmitReview(id int64, operator *dto.AuthPrincipal) error {
	current := s.GetForOperator(id, operator)
	if current == nil || current.ProductID <= 0 {
		return errorsx.InvalidParam("product agent not found")
	}
	if current.WorkflowID <= 0 || current.WorkflowVersionID <= 0 {
		return errorsx.InvalidParam("publish the workflow before submitting for review")
	}
	pending := repositories.AIAgentReleaseRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", current.TenantID).
		Eq("agent_id", current.ID).
		Eq("review_status", enums.AIAgentReviewStatusPending).
		NotEq("status", enums.StatusDeleted).
		Desc("release_no"))
	if len(pending) > 0 {
		return errorsx.InvalidParam("an AI Agent release is already pending review")
	}
	release, err := AIAgentReleaseService.CreateCandidate(id, operator)
	if err != nil {
		return err
	}
	if err := AIAgentReleaseService.SubmitReview(release.ID, operator); err != nil {
		return err
	}
	updates := map[string]any{
		"review_status":    enums.AIAgentReviewStatusPending,
		"review_comment":   "",
		"reviewed_at":      nil,
		"reviewed_by_id":   0,
		"reviewed_by_name": "",
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	}
	if current.ActiveReleaseID <= 0 {
		updates["status"] = enums.StatusDisabled
	}
	return repositories.AIAgentRepository.Updates(sqls.DB(), id, updates)
}

func (s *aIAgentService) Review(id int64, approved bool, comment string, operator *dto.AuthPrincipal) error {
	current := s.GetForOperator(id, operator)
	if current == nil || current.ProductID <= 0 {
		return errorsx.InvalidParam("product agent not found")
	}
	comment = strings.TrimSpace(comment)
	if !approved && comment == "" {
		return errorsx.InvalidParam("rejection reason is required")
	}
	pending := repositories.AIAgentReleaseRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", current.TenantID).
		Eq("agent_id", current.ID).
		Eq("review_status", enums.AIAgentReviewStatusPending).
		NotEq("status", enums.StatusDeleted).
		Desc("release_no"))
	if len(pending) == 0 {
		return errorsx.InvalidParam("only pending AI Agent releases can be reviewed")
	}
	release := pending[0]
	if err := AIAgentReleaseService.Review(release.ID, approved, comment, operator); err != nil {
		return err
	}
	if approved {
		return AIAgentReleaseService.Deploy(release.ID, operator)
	}
	now := time.Now()
	updates := map[string]any{
		"review_status":    enums.AIAgentReviewStatusRejected,
		"review_comment":   comment,
		"reviewed_at":      &now,
		"reviewed_by_id":   operator.UserID,
		"reviewed_by_name": operator.Username,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       now,
	}
	if current.ActiveReleaseID <= 0 {
		updates["status"] = enums.StatusDisabled
	}
	return repositories.AIAgentRepository.Updates(sqls.DB(), id, updates)
}

func canManageAIAgent(item *models.AIAgent, operator *dto.AuthPrincipal) bool {
	if item == nil || operator == nil {
		return false
	}
	if operator.IsPlatform() && operator.TenantID <= 0 {
		return true
	}
	return operator.TenantID > 0 && item.TenantID == operator.TenantID
}
