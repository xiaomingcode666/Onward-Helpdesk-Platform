package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
	"time"

	workflowcapability "remotehelpdesk/internal/ai/workflow/capability"
	"remotehelpdesk/internal/ai/workflow/dsl"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var AIAgentReleaseService = &aiAgentReleaseService{}

type aiAgentReleaseService struct{}

// AlignAgentWorkflowToCurrentStableDB removes legacy routing from an Agent
// draft while preserving agent-specific custom workflows.
func (s *aiAgentReleaseService) AlignAgentWorkflowToCurrentStableDB(db *gorm.DB, agentID int64, operator *dto.AuthPrincipal) error {
	if db == nil || agentID <= 0 || operator == nil {
		return errorsx.InvalidParam("AI Agent workflow alignment context is invalid")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		agent := repositories.AIAgentRepository.GetForUpdate(tx, agentID)
		if agent == nil || agent.WorkflowID <= 0 || agent.WorkflowVersionID <= 0 {
			return errorsx.InvalidParam("客服机器人绑定的工作流无效")
		}
		workflow := repositories.AIWorkflowRepository.Get(tx, agent.WorkflowID)
		version := repositories.AIWorkflowVersionRepository.Get(tx, agent.WorkflowVersionID)
		validCurrent := workflow != nil && version != nil && workflow.Status != enums.StatusDeleted && version.Status == enums.StatusOk && version.WorkflowID == workflow.ID && workflow.CurrentStableVersionID == version.ID
		if validCurrent {
			definition := dsl.Definition{}
			validCurrent = json.Unmarshal([]byte(version.Definition), &definition) == nil && AIWorkflowService.ValidateDefinition(definition).Valid
		}
		if validCurrent && (IsPlatformBuiltInWorkflow(workflow) || workflow.AgentID == agent.ID || workflow.SourceWorkflowID == 0) {
			return nil
		}

		var targetWorkflow *models.AIWorkflow
		if IsPlatformBuiltInWorkflow(workflow) {
			targetWorkflow = workflow
		} else if workflow != nil && workflow.SourceWorkflowID > 0 {
			source := repositories.AIWorkflowRepository.Get(tx, workflow.SourceWorkflowID)
			if IsPlatformBuiltInWorkflow(source) {
				targetWorkflow = source
			}
		}
		var targetVersion *models.AIWorkflowVersion
		var err error
		if targetWorkflow == nil {
			if agent.Source == TenantDefaultAIAgentSource {
				tenant := repositories.PlatformIAMRepository.GetTenant(tx, agent.TenantID)
				targetWorkflow, targetVersion, err = AIWorkflowService.EnsureTenantDefaultWorkflowDB(tx, tenant)
			} else {
				targetWorkflow, targetVersion, err = AIWorkflowService.EnsurePlatformDefaultWorkflowDB(tx)
			}
			if err != nil {
				return err
			}
		} else {
			targetVersion = repositories.AIWorkflowVersionRepository.Get(tx, targetWorkflow.CurrentStableVersionID)
		}
		if targetWorkflow == nil || targetVersion == nil || targetVersion.Status != enums.StatusOk || targetVersion.ReleaseChannel != models.AIWorkflowReleaseChannelStable {
			return errorsx.InvalidParam("当前平台注册工作流没有可用稳定版本")
		}
		restrictions, err := workflowAgentRestrictionUpdates(agent, targetVersion)
		if err != nil {
			return err
		}
		updates := map[string]any{
			"workflow_id":         targetWorkflow.ID,
			"workflow_version_id": targetVersion.ID,
			"draft_revision":      agent.DraftRevision + 1,
			"update_user_id":      operator.UserID,
			"update_user_name":    operator.Username,
			"updated_at":          time.Now(),
		}
		for key, value := range restrictions {
			updates[key] = value
		}
		return repositories.AIAgentRepository.Updates(tx, agent.ID, updates)
	})
}

func (s *aiAgentReleaseService) MaterializeRuntimeAgent(db *gorm.DB, agent models.AIAgent) (models.AIAgent, *models.AIAgentRelease, error) {
	if agent.ActiveReleaseID <= 0 {
		return agent, nil, nil
	}
	release := repositories.AIAgentReleaseRepository.Get(db, agent.ActiveReleaseID)
	if release == nil || release.Status != enums.StatusOk || release.AgentID != agent.ID || release.TenantID != agent.TenantID || release.ProductID != agent.ProductID {
		return agent, nil, errorsx.Forbidden("当前机器人生产版本与运行范围不匹配")
	}
	if release.ReviewStatus != enums.AIAgentReviewStatusApproved || release.DeploymentStatus != models.AIAgentReleaseDeploymentActive {
		return agent, nil, errorsx.InvalidParam("当前机器人生产版本尚未审核并部署")
	}
	workflow := repositories.AIWorkflowRepository.Get(db, release.WorkflowID)
	version := repositories.AIWorkflowVersionRepository.Get(db, release.WorkflowVersionID)
	if workflow == nil || version == nil || workflow.Status == enums.StatusDeleted || version.Status != enums.StatusOk || version.WorkflowID != workflow.ID {
		return agent, nil, errorsx.InvalidParam("当前机器人生产版本使用的工作流已经退役")
	}
	if IsPlatformBuiltInWorkflow(workflow) && version.ReleaseChannel != models.AIWorkflowReleaseChannelStable {
		return agent, nil, errorsx.InvalidParam("当前机器人生产版本使用的平台流程版本已经退役")
	}
	if hashString(release.AgentConfigSnapshot) != release.AgentConfigHash || hashString(release.KnowledgeScopeSnapshot) != release.KnowledgeScopeHash {
		return agent, nil, errorsx.InvalidParam("当前机器人生产版本快照校验失败")
	}
	snapshot := dto.AIAgentReleaseConfigSnapshot{}
	if err := json.Unmarshal([]byte(release.AgentConfigSnapshot), &snapshot); err != nil || snapshot.SchemaVersion != 1 {
		return agent, nil, errorsx.InvalidParam("当前机器人生产版本配置无效")
	}
	knowledgeScope := dto.AIAgentReleaseKnowledgeScopeSnapshot{}
	if err := json.Unmarshal([]byte(release.KnowledgeScopeSnapshot), &knowledgeScope); err != nil || knowledgeScope.SchemaVersion != 1 {
		return agent, nil, errorsx.InvalidParam("当前机器人生产版本知识范围无效")
	}
	if knowledgeScope.TenantID != release.TenantID || knowledgeScope.ProductID != release.ProductID {
		return agent, nil, errorsx.Forbidden("当前机器人生产版本知识范围与发布范围不匹配")
	}
	agent.AIConfigID = snapshot.AIConfigID
	agent.LLMModelName = snapshot.LLMModelName
	agent.ServiceMode = snapshot.ServiceMode
	agent.SystemPrompt = snapshot.SystemPrompt
	agent.WelcomeMessage = snapshot.WelcomeMessage
	agent.ReplyTimeoutSeconds = snapshot.ReplyTimeoutSeconds
	agent.TeamIDs = utils.JoinInt64s(snapshot.TeamIDs)
	agent.HandoffMode = snapshot.HandoffMode
	agent.FallbackMode = snapshot.FallbackMode
	agent.FallbackMessage = snapshot.FallbackMessage
	knowledgeIDs := append([]int64(nil), snapshot.KnowledgeIDs...)
	for _, binding := range knowledgeScope.Bindings {
		knowledgeIDs = appendUniqueReleaseInt64(knowledgeIDs, binding.KnowledgeBaseID)
	}
	agent.KnowledgeIDs = utils.JoinInt64s(sortedReleaseInt64s(knowledgeIDs))
	agent.SkillIDs = utils.JoinInt64s(snapshot.SkillIDs)
	if raw, err := json.Marshal(snapshot.AllowedMCPTools); err == nil {
		agent.AllowedMCPTools = string(raw)
	} else {
		return agent, nil, errorsx.InvalidParam("当前机器人生产版本的外部工具快照无效")
	}
	if raw, err := json.Marshal(snapshot.AllowedGraphTools); err == nil {
		agent.AllowedGraphTools = string(raw)
	} else {
		return agent, nil, errorsx.InvalidParam("当前机器人生产版本的流程工具快照无效")
	}
	agent.WorkflowID = release.WorkflowID
	agent.WorkflowVersionID = release.WorkflowVersionID
	return agent, release, nil
}

func (s *aiAgentReleaseService) Get(id int64, operator *dto.AuthPrincipal) *models.AIAgentRelease {
	item := repositories.AIAgentReleaseRepository.Get(sqls.DB(), id)
	if item == nil || operator == nil {
		return nil
	}
	if operator.IsPlatform() && operator.TenantID <= 0 {
		return item
	}
	if operator.TenantID <= 0 || item.TenantID != operator.TenantID {
		return nil
	}
	return item
}

func (s *aiAgentReleaseService) ListByAgent(agentID int64, operator *dto.AuthPrincipal) ([]models.AIAgentRelease, error) {
	agent := AIAgentService.GetForOperator(agentID, operator)
	if agent == nil {
		return nil, errorsx.InvalidParam("客服机器人不存在")
	}
	return repositories.AIAgentReleaseRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", agent.TenantID).
		Eq("agent_id", agent.ID).
		NotEq("status", enums.StatusDeleted).
		Desc("release_no")), nil
}

func (s *aiAgentReleaseService) CreateCandidate(agentID int64, operator *dto.AuthPrincipal) (*models.AIAgentRelease, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	var release *models.AIAgentRelease
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		agent := repositories.AIAgentRepository.GetForUpdate(ctx.Tx, agentID)
		if agent == nil || agent.Status == enums.StatusDeleted || !canManageAIAgent(agent, operator) {
			return errorsx.InvalidParam("客服机器人不存在")
		}
		if agent.Source == TenantDefaultAIAgentSource {
			return errorsx.Forbidden("tenant default AI agent releases are managed by the platform")
		}
		workflow, version, definition, err := s.resolveReleaseWorkflow(ctx.Tx, agent)
		if err != nil {
			return err
		}
		agentSnapshot, agentHash, err := buildAgentReleaseConfigSnapshotForDefinition(agent, &definition)
		if err != nil {
			return err
		}
		knowledgeSnapshot, knowledgeHash, err := s.buildKnowledgeScopeSnapshot(ctx.Tx, agent)
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
		if len(existing) > 0 {
			if existing[0].DeploymentStatus == models.AIAgentReleaseDeploymentInactive {
				release = &existing[0]
				return nil
			}
			return errorsx.InvalidParam("当前草稿与已部署的生产版本一致，无需重复生成")
		}
		release = &models.AIAgentRelease{
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
			ReviewStatus:           enums.AIAgentReviewStatusUnreviewed,
			DeploymentStatus:       models.AIAgentReleaseDeploymentInactive,
			Status:                 enums.StatusOk,
			AuditFields:            utils.BuildAuditFields(operator),
		}
		return repositories.AIAgentReleaseRepository.Create(ctx.Tx, release)
	})
	if err != nil {
		return nil, err
	}
	return release, nil
}

func (s *aiAgentReleaseService) SubmitReview(releaseID int64, operator *dto.AuthPrincipal) error {
	item := s.Get(releaseID, operator)
	if item == nil {
		return errorsx.InvalidParam("机器人发布版本不存在")
	}
	if item.ReviewStatus != enums.AIAgentReviewStatusUnreviewed && item.ReviewStatus != enums.AIAgentReviewStatusRejected {
		return errorsx.InvalidParam("只有未审核或已退回的发布版本可以提交审核")
	}
	updated, err := repositories.AIAgentReleaseRepository.ConditionalUpdates(sqls.DB(), item.ID, string(item.ReviewStatus), map[string]any{
		"review_status":    enums.AIAgentReviewStatusPending,
		"review_comment":   "",
		"reviewed_at":      nil,
		"reviewed_by_id":   0,
		"reviewed_by_name": "",
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	})
	if err != nil {
		return err
	}
	if !updated {
		return errorsx.InvalidParam("发布版本状态已变化，请刷新后重试")
	}
	return nil
}

func (s *aiAgentReleaseService) Review(releaseID int64, approved bool, comment string, operator *dto.AuthPrincipal) error {
	item := s.Get(releaseID, operator)
	if item == nil {
		return errorsx.InvalidParam("机器人发布版本不存在")
	}
	if item.ReviewStatus != enums.AIAgentReviewStatusPending {
		return errorsx.InvalidParam("只有待审核的发布版本可以审核")
	}
	comment = strings.TrimSpace(comment)
	if !approved && comment == "" {
		return errorsx.InvalidParam("退回时必须填写原因")
	}
	status := enums.AIAgentReviewStatusRejected
	if approved {
		status = enums.AIAgentReviewStatusApproved
	}
	now := time.Now()
	updated, err := repositories.AIAgentReleaseRepository.ConditionalUpdates(sqls.DB(), item.ID, string(enums.AIAgentReviewStatusPending), map[string]any{
		"review_status":    status,
		"review_comment":   comment,
		"reviewed_at":      &now,
		"reviewed_by_id":   operator.UserID,
		"reviewed_by_name": operator.Username,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       now,
	})
	if err != nil {
		return err
	}
	if !updated {
		return errorsx.InvalidParam("发布版本状态已变化，请刷新后重试")
	}
	return nil
}

func (s *aiAgentReleaseService) Deploy(releaseID int64, operator *dto.AuthPrincipal) error {
	return s.deploy(releaseID, 0, operator)
}

func (s *aiAgentReleaseService) Rollback(releaseID int64, operator *dto.AuthPrincipal) error {
	item := s.Get(releaseID, operator)
	if item == nil {
		return errorsx.InvalidParam("机器人发布版本不存在")
	}
	agent := AIAgentService.GetForOperator(item.AgentID, operator)
	if agent == nil || agent.ActiveReleaseID <= 0 || agent.ActiveReleaseID == item.ID {
		return errorsx.InvalidParam("该版本已在生产运行，或当前没有可回滚的生产版本")
	}
	return s.deploy(releaseID, agent.ActiveReleaseID, operator)
}

func (s *aiAgentReleaseService) deploy(releaseID, rollbackFromReleaseID int64, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		release := repositories.AIAgentReleaseRepository.GetForUpdate(ctx.Tx, releaseID)
		if release == nil || release.Status != enums.StatusOk || !canManageRelease(release, operator) {
			return errorsx.InvalidParam("机器人发布版本不存在")
		}
		if release.ReviewStatus != enums.AIAgentReviewStatusApproved {
			return errorsx.InvalidParam("只有审核通过的发布版本可以部署")
		}
		agent := repositories.AIAgentRepository.GetForUpdate(ctx.Tx, release.AgentID)
		if agent == nil || agent.TenantID != release.TenantID || agent.ProductID != release.ProductID {
			return errorsx.Forbidden("发布版本与客服机器人范围不匹配")
		}
		workflow := repositories.AIWorkflowRepository.Get(ctx.Tx, release.WorkflowID)
		version := repositories.AIWorkflowVersionRepository.Get(ctx.Tx, release.WorkflowVersionID)
		if workflow == nil || version == nil || workflow.Status == enums.StatusDeleted || version.Status != enums.StatusOk || version.WorkflowID != workflow.ID {
			return errorsx.InvalidParam("发布版本绑定的工作流版本不存在")
		}
		if workflow.Scope == models.AIWorkflowScopeTenant && workflow.TenantID != agent.TenantID {
			return errorsx.Forbidden("发布版本绑定的工作流属于其他租户")
		}
		if IsPlatformBuiltInWorkflow(workflow) && (workflow.CurrentStableVersionID != version.ID || version.ReleaseChannel != models.AIWorkflowReleaseChannelStable) {
			return errorsx.InvalidParam("平台注册流程的历史版本不能再次部署")
		}
		if version.DefinitionHash != release.WorkflowDefinitionHash || hashString(version.Definition) != release.WorkflowDefinitionHash {
			return errorsx.InvalidParam("发布版本的工作流快照与固定版本不一致")
		}
		if err := validateReleaseCapabilityBoundary(release, version); err != nil {
			return err
		}
		if version.ReleaseChannel == models.AIWorkflowReleaseChannelRevoked {
			return errorsx.InvalidParam("已撤销的工作流版本不能部署")
		}
		now := time.Now()
		active := repositories.AIAgentReleaseRepository.Find(ctx.Tx, sqls.NewCnd().
			Eq("agent_id", agent.ID).
			Eq("deployment_status", models.AIAgentReleaseDeploymentActive).
			NotEq("id", release.ID))
		for i := range active {
			status := models.AIAgentReleaseDeploymentRetired
			if rollbackFromReleaseID > 0 && active[i].ID == rollbackFromReleaseID {
				status = models.AIAgentReleaseDeploymentRolledBack
			}
			if err := repositories.AIAgentReleaseRepository.Updates(ctx.Tx, active[i].ID, map[string]any{
				"deployment_status": status,
				"update_user_id":    operator.UserID,
				"update_user_name":  operator.Username,
				"updated_at":        now,
			}); err != nil {
				return err
			}
		}
		if err := repositories.AIAgentReleaseRepository.Updates(ctx.Tx, release.ID, map[string]any{
			"deployment_status":        models.AIAgentReleaseDeploymentActive,
			"deployed_at":              &now,
			"deployed_by_id":           operator.UserID,
			"deployed_by_name":         operator.Username,
			"rollback_from_release_id": rollbackFromReleaseID,
			"update_user_id":           operator.UserID,
			"update_user_name":         operator.Username,
			"updated_at":               now,
		}); err != nil {
			return err
		}
		return repositories.AIAgentRepository.Updates(ctx.Tx, agent.ID, map[string]any{
			"active_release_id": release.ID,
			"status":            enums.StatusOk,
			"review_status":     enums.AIAgentReviewStatusApproved,
			"review_comment":    release.ReviewComment,
			"reviewed_at":       release.ReviewedAt,
			"reviewed_by_id":    release.ReviewedByID,
			"reviewed_by_name":  release.ReviewedByName,
			"update_user_id":    operator.UserID,
			"update_user_name":  operator.Username,
			"updated_at":        now,
		})
	})
}

func validateReleaseCapabilityBoundary(release *models.AIAgentRelease, version *models.AIWorkflowVersion) error {
	if release == nil || version == nil ||
		hashString(release.AgentConfigSnapshot) != release.AgentConfigHash ||
		hashString(release.KnowledgeScopeSnapshot) != release.KnowledgeScopeHash {
		return errorsx.InvalidParam("发布版本快照校验失败")
	}
	definition := dsl.Definition{}
	if err := json.Unmarshal([]byte(version.Definition), &definition); err != nil || !AIWorkflowService.ValidateDefinition(definition).Valid {
		return errorsx.InvalidParam("发布版本的工作流定义无效")
	}
	snapshot := dto.AIAgentReleaseConfigSnapshot{}
	if err := json.Unmarshal([]byte(release.AgentConfigSnapshot), &snapshot); err != nil || snapshot.SchemaVersion != 1 {
		return errorsx.InvalidParam("发布版本配置快照无效")
	}
	normalizeAgentReleaseConfigSnapshot(&snapshot, workflowcapability.FromDefinition(definition))
	normalized, normalizedHash, err := marshalSnapshot(snapshot)
	if err != nil {
		return err
	}
	if normalized != release.AgentConfigSnapshot || normalizedHash != release.AgentConfigHash {
		return errorsx.InvalidParam("发布版本配置超出工作流能力边界，请重新生成候选版本")
	}
	return nil
}

func (s *aiAgentReleaseService) resolveReleaseWorkflow(db *gorm.DB, agent *models.AIAgent) (*models.AIWorkflow, *models.AIWorkflowVersion, dsl.Definition, error) {
	if agent.WorkflowID <= 0 || agent.WorkflowVersionID <= 0 {
		return nil, nil, dsl.Definition{}, errorsx.InvalidParam("生成发布版本前，请先选择并发布工作流版本")
	}
	workflow := repositories.AIWorkflowRepository.GetForUpdate(db, agent.WorkflowID)
	version := repositories.AIWorkflowVersionRepository.Get(db, agent.WorkflowVersionID)
	if workflow == nil || version == nil || workflow.Status == enums.StatusDeleted || version.Status != enums.StatusOk || version.WorkflowID != workflow.ID {
		return nil, nil, dsl.Definition{}, errorsx.InvalidParam("客服机器人绑定的工作流无效")
	}
	if workflow.Scope == models.AIWorkflowScopeTenant && workflow.TenantID != agent.TenantID {
		return nil, nil, dsl.Definition{}, errorsx.Forbidden("工作流属于其他租户")
	}
	if workflow.Scope != models.AIWorkflowScopePlatform && workflow.Scope != models.AIWorkflowScopeTenant {
		return nil, nil, dsl.Definition{}, errorsx.InvalidParam("工作流范围无效")
	}
	if workflow.CurrentStableVersionID != version.ID {
		return nil, nil, dsl.Definition{}, errorsx.InvalidParam("只能基于工作流当前稳定版生成发布版本")
	}
	if version.ReleaseChannel == models.AIWorkflowReleaseChannelDeprecated || version.ReleaseChannel == models.AIWorkflowReleaseChannelRevoked {
		return nil, nil, dsl.Definition{}, errorsx.InvalidParam("当前工作流版本不能用于生成新发布版本")
	}
	if hashString(version.Definition) != version.DefinitionHash {
		return nil, nil, dsl.Definition{}, errorsx.InvalidParam("工作流定义指纹校验失败")
	}
	definition := dsl.Definition{}
	if err := json.Unmarshal([]byte(version.Definition), &definition); err != nil || !AIWorkflowService.ValidateDefinition(definition).Valid {
		return nil, nil, dsl.Definition{}, errorsx.InvalidParam("工作流定义无效")
	}
	return workflow, version, definition, nil
}

func buildAgentReleaseConfigSnapshot(agent *models.AIAgent) (string, string, error) {
	return buildAgentReleaseConfigSnapshotForDefinition(agent, nil)
}

func buildAgentReleaseConfigSnapshotForDefinition(agent *models.AIAgent, definition *dsl.Definition) (string, string, error) {
	var allowedMCPTools any = []any{}
	if raw := strings.TrimSpace(agent.AllowedMCPTools); raw != "" {
		if err := json.Unmarshal([]byte(raw), &allowedMCPTools); err != nil {
			return "", "", errorsx.InvalidParam("客服机器人的外部工具配置无效")
		}
	}
	graphTools := make([]string, 0)
	if raw := strings.TrimSpace(agent.AllowedGraphTools); raw != "" {
		if err := json.Unmarshal([]byte(raw), &graphTools); err != nil {
			return "", "", errorsx.InvalidParam("客服机器人的流程工具配置无效")
		}
	}
	slices.Sort(graphTools)
	graphTools = slices.Compact(graphTools)
	canonicalizeReleaseJSONList(allowedMCPTools)
	snapshot := dto.AIAgentReleaseConfigSnapshot{
		SchemaVersion:       1,
		DraftRevision:       agent.DraftRevision,
		AIConfigID:          agent.AIConfigID,
		LLMModelName:        strings.TrimSpace(agent.LLMModelName),
		ServiceMode:         agent.ServiceMode,
		SystemPrompt:        agent.SystemPrompt,
		WelcomeMessage:      agent.WelcomeMessage,
		ReplyTimeoutSeconds: agent.ReplyTimeoutSeconds,
		TeamIDs:             sortedReleaseInt64s(utils.SplitInt64s(agent.TeamIDs)),
		HandoffMode:         agent.HandoffMode,
		FallbackMode:        agent.FallbackMode,
		FallbackMessage:     agent.FallbackMessage,
		KnowledgeIDs:        sortedReleaseInt64s(utils.SplitInt64s(agent.KnowledgeIDs)),
		SkillIDs:            sortedReleaseInt64s(utils.SplitInt64s(agent.SkillIDs)),
		AllowedMCPTools:     allowedMCPTools,
		AllowedGraphTools:   graphTools,
	}
	if definition != nil {
		normalizeAgentReleaseConfigSnapshot(&snapshot, workflowcapability.FromDefinition(*definition))
	}
	return marshalSnapshot(snapshot)
}

func normalizeAgentReleaseConfigSnapshot(snapshot *dto.AIAgentReleaseConfigSnapshot, capabilities workflowcapability.Set) {
	if snapshot == nil {
		return
	}
	if !capabilities.HumanHandoff {
		snapshot.ServiceMode = enums.IMConversationServiceModeAIOnly
		snapshot.TeamIDs = []int64{}
	}
	snapshot.FallbackMessage = capabilities.SanitizeFallbackMessage(snapshot.FallbackMessage)
}

func (s *aiAgentReleaseService) buildKnowledgeScopeSnapshot(db *gorm.DB, agent *models.AIAgent) (string, string, error) {
	snapshot := dto.AIAgentReleaseKnowledgeScopeSnapshot{
		SchemaVersion: 1,
		TenantID:      agent.TenantID,
		ProductID:     agent.ProductID,
		Resolution:    "product_binding_then_agent_supplement",
		Bindings:      make([]dto.AIAgentReleaseKnowledgeBindingSnapshot, 0),
		Revisions:     make([]dto.AIAgentReleaseKnowledgeRevisionSnapshot, 0),
	}
	knowledgeBaseIDs := utils.SplitInt64s(agent.KnowledgeIDs)
	if agent.ProductID > 0 {
		product := repositories.ProductRepository.GetByTenant(db, agent.ProductID, agent.TenantID)
		if product == nil || product.Status == enums.StatusDeleted {
			return "", "", errorsx.InvalidParam("发布版本关联的产品不存在")
		}
		bindings := repositories.ProductKnowledgeBindingRepository.Find(db, sqls.NewCnd().
			Eq("tenant_id", agent.TenantID).
			Eq("product_id", agent.ProductID).
			Eq("status", enums.StatusOk).
			Asc("sort_no").Asc("id"))
		if len(bindings) == 0 {
			return "", "", errorsx.InvalidParam("产品没有生效中的知识库挂载")
		}
		for i := range bindings {
			binding := bindings[i]
			knowledgeBase := repositories.KnowledgeBaseRepository.Get(db, binding.KnowledgeBaseID)
			if knowledgeBase == nil || knowledgeBase.TenantID != agent.TenantID || knowledgeBase.Status == enums.StatusDeleted {
				return "", "", errorsx.Forbidden("产品知识库挂载跨越租户范围")
			}
			knowledgeBaseIDs = appendUniqueReleaseInt64(knowledgeBaseIDs, knowledgeBase.ID)
			snapshot.Bindings = append(snapshot.Bindings, dto.AIAgentReleaseKnowledgeBindingSnapshot{
				BindingID:       binding.ID,
				ProductModelID:  binding.ProductModelID,
				KnowledgeBaseID: binding.KnowledgeBaseID,
				ScopeType:       binding.ScopeType,
				Locale:          binding.Locale,
				RegionCode:      binding.RegionCode,
			})
		}
	}
	slices.Sort(knowledgeBaseIDs)
	if len(knowledgeBaseIDs) == 0 {
		return "", "", errorsx.InvalidParam("客服机器人没有可解析的知识范围")
	}
	revisions := repositories.KnowledgeRevisionRepository.Find(db, sqls.NewCnd().
		Eq("tenant_id", agent.TenantID).
		In("knowledge_base_id", knowledgeBaseIDs).
		Eq("review_status", "published").
		Asc("knowledge_base_id").Asc("id"))
	for i := range revisions {
		revision := revisions[i]
		snapshot.Revisions = append(snapshot.Revisions, dto.AIAgentReleaseKnowledgeRevisionSnapshot{
			RevisionID:        revision.ID,
			KnowledgeBaseID:   revision.KnowledgeBaseID,
			ContentHash:       revision.ContentHash,
			Language:          revision.Language,
			Visibility:        revision.Visibility,
			IndexGenerationID: revision.IndexGenerationID,
		})
	}
	return marshalSnapshot(snapshot)
}

func canManageRelease(item *models.AIAgentRelease, operator *dto.AuthPrincipal) bool {
	if item == nil || operator == nil {
		return false
	}
	return (operator.IsPlatform() && operator.TenantID <= 0) || (operator.TenantID > 0 && item.TenantID == operator.TenantID)
}

func appendUniqueReleaseInt64(items []int64, value int64) []int64 {
	if value <= 0 {
		return items
	}
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func sortedReleaseInt64s(items []int64) []int64 {
	ret := append([]int64(nil), items...)
	slices.Sort(ret)
	return slices.Compact(ret)
}

func canonicalizeReleaseJSONList(value any) {
	items, ok := value.([]any)
	if !ok {
		return
	}
	slices.SortFunc(items, func(a, b any) int {
		aRaw, _ := json.Marshal(a)
		bRaw, _ := json.Marshal(b)
		return strings.Compare(string(aRaw), string(bRaw))
	})
}

func marshalSnapshot(value any) (string, string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", "", err
	}
	return string(raw), hashString(string(raw)), nil
}

func hashString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
