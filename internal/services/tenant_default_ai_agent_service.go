package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	workflowcapability "remotehelpdesk/internal/ai/workflow/capability"
	"remotehelpdesk/internal/ai/workflow/dsl"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/i18nx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

const (
	TenantDefaultAIAgentSource       = "tenant_default"
	tenantDefaultKnowledgeBaseRemark = "builtin-tenant-default-service"
)

// 内置数据按资源键展示；存储列保留中文规范值，兼容按名称识别的存量数据和运行时引用。
var (
	tenantDefaultKnowledgeBaseNameText        = i18nx.NewText("builtin.knowledgeBase.tenantDefault.name", nil)
	tenantDefaultKnowledgeBaseDescriptionText = i18nx.NewText("builtin.knowledgeBase.tenantDefault.description", nil)
	tenantDefaultAgentNameText                = i18nx.NewText("builtin.aiAgent.tenantDefault.name", nil)
)

func tenantDefaultKnowledgeBaseName() string {
	return tenantDefaultKnowledgeBaseNameText.Render(i18nx.LocaleZhCN)
}

func tenantDefaultKnowledgeBaseDescription() string {
	return tenantDefaultKnowledgeBaseDescriptionText.Render(i18nx.LocaleZhCN)
}

func tenantDefaultAgentName() string {
	return tenantDefaultAgentNameText.Render(i18nx.LocaleZhCN)
}

var TenantDefaultAIAgentService = &tenantDefaultAIAgentService{}

type tenantDefaultAIAgentService struct{}

// EnsureDB provisions the always-available, no-device reception path for a
// tenant. The first release is activated during tenant initialization. Later
// code-owned updates only refresh the draft and create an immutable candidate;
// they never bypass the normal review and deployment workflow.
func (s *tenantDefaultAIAgentService) EnsureDB(db *gorm.DB, tenantID int64, operator *dto.AuthPrincipal) (*models.AIAgent, error) {
	if db == nil || tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant default AI agent context is invalid")
	}
	for _, table := range []any{
		&models.Tenant{},
		&models.KnowledgeBase{},
		&models.KnowledgeRevision{},
		&models.AIWorkflow{},
		&models.AIWorkflowVersion{},
		&models.AIAgent{},
		&models.AIAgentRelease{},
	} {
		if !db.Migrator().HasTable(table) {
			return nil, nil
		}
	}

	var result *models.AIAgent
	err := db.Transaction(func(tx *gorm.DB) error {
		tenant := repositories.PlatformIAMRepository.LockTenant(tx, tenantID)
		if tenant == nil || tenant.Status == enums.StatusDeleted {
			return errorsx.InvalidParam("tenant not found")
		}
		if !tenant.IsAIEnabled() {
			return errorsx.Forbidden("tenant AI capability is disabled")
		}
		workflow, version, err := AIWorkflowService.EnsureTenantDefaultWorkflowDB(tx, tenant)
		if err != nil {
			return err
		}
		if workflow == nil || version == nil || version.Status != enums.StatusOk || version.WorkflowID != workflow.ID || version.ReleaseChannel != models.AIWorkflowReleaseChannelStable {
			return errorsx.InvalidParam("platform AI diagnosis workflow has no stable version")
		}
		knowledgeBase, err := s.ensureKnowledgeBase(tx, tenant, operator)
		if err != nil {
			return err
		}
		agent, err := s.ensureAgent(tx, tenant, knowledgeBase, workflow, version, operator)
		if err != nil {
			return err
		}
		if err := s.ensureActiveRelease(tx, agent, workflow, version, operator); err != nil {
			return err
		}
		result = repositories.AIAgentRepository.Get(tx, agent.ID)
		return nil
	})
	return result, err
}

// SyncStaleWorkflowDraftsDB re-provisions tenant default agents whose draft is
// still pinned to a superseded platform workflow version, so a platform upgrade
// yields a deployable candidate instead of leaving every new release stranded.
// It returns the IDs of tenants whose sync failed; one tenant never blocks others.
func (s *tenantDefaultAIAgentService) SyncStaleWorkflowDraftsDB(db *gorm.DB) (map[int64]error, error) {
	if db == nil {
		return nil, errorsx.InvalidParam("database is required")
	}
	for _, table := range []any{&models.Tenant{}, &models.AIWorkflow{}, &models.AIAgent{}} {
		if !db.Migrator().HasTable(table) {
			return nil, nil
		}
	}
	agentTable, err := tableNameOf(db, &models.AIAgent{})
	if err != nil {
		return nil, err
	}
	workflowTable, err := tableNameOf(db, &models.AIWorkflow{})
	if err != nil {
		return nil, err
	}
	var tenantIDs []int64
	if err := db.Table(agentTable+" AS a").
		Joins("JOIN "+workflowTable+" AS w ON w.id = a.workflow_id").
		Where("a.source = ? AND a.product_id = 0 AND a.status <> ?", TenantDefaultAIAgentSource, enums.StatusDeleted).
		Where("w.scope = ? AND w.current_stable_version_id > 0 AND w.current_stable_version_id <> a.workflow_version_id", models.AIWorkflowScopePlatform).
		Distinct("a.tenant_id").
		Order("a.tenant_id ASC").
		Pluck("a.tenant_id", &tenantIDs).Error; err != nil {
		return nil, err
	}
	failures := make(map[int64]error)
	for _, tenantID := range tenantIDs {
		tenant := repositories.PlatformIAMRepository.GetTenant(db, tenantID)
		if tenant == nil || tenant.Status == enums.StatusDeleted || !tenant.IsAIEnabled() {
			continue
		}
		operator := &dto.AuthPrincipal{
			TenantID:   tenantID,
			Username:   "platform-workflow-sync",
			DomainType: models.DomainTypeEnterprise,
			Domain:     models.DomainTypeEnterprise,
		}
		if _, err := s.EnsureDB(db, tenantID, operator); err != nil {
			failures[tenantID] = err
		}
	}
	return failures, nil
}

func tableNameOf(db *gorm.DB, model any) (string, error) {
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(model); err != nil {
		return "", err
	}
	return stmt.Schema.Table, nil
}

func (s *tenantDefaultAIAgentService) EnsureDefaultCredential(tenantID int64, operator *dto.AuthPrincipal) error {
	if tenantID <= 0 {
		return errorsx.InvalidParam("tenant is required")
	}
	if !TenantCapabilityService.AIEnabled(tenantID) {
		return errorsx.Forbidden("tenant AI capability is disabled")
	}
	db := sqls.DB()
	if db == nil || !db.Migrator().HasTable(&models.Sub2APITenantAccount{}) {
		return errorsx.InvalidParam("tenant AI credential storage is unavailable")
	}
	if account := repositories.PlatformIAMRepository.FindSub2APIAccountByTenantID(db, tenantID); account != nil &&
		strings.EqualFold(strings.TrimSpace(account.ProvisionStatus), "active") &&
		strings.EqualFold(strings.TrimSpace(account.DefaultKeyStatus), "active") &&
		strings.TrimSpace(account.DefaultKeyCiphertext) != "" {
		return nil
	}
	_, err := PlatformAIModelService.ProvisionTenant(request.PlatformAITenantProvisionRequest{
		TenantID:    tenantID,
		Concurrency: 50,
		Balance:     20,
		RPMLimit:    0,
	}, operator)
	return err
}

func (s *tenantDefaultAIAgentService) ensureKnowledgeBase(db *gorm.DB, tenant *models.Tenant, operator *dto.AuthPrincipal) (*models.KnowledgeBase, error) {
	item := repositories.KnowledgeBaseRepository.FindOne(db, sqls.NewCnd().
		Eq("tenant_id", tenant.ID).
		Eq("remark", tenantDefaultKnowledgeBaseRemark).
		NotEq("status", enums.StatusDeleted).
		Asc("id"))
	if item != nil {
		updates := make(map[string]any)
		if item.Name != tenantDefaultKnowledgeBaseName() {
			updates["name"] = tenantDefaultKnowledgeBaseName()
		}
		if item.Description != tenantDefaultKnowledgeBaseDescription() {
			updates["description"] = tenantDefaultKnowledgeBaseDescription()
		}
		if !item.NameText.Equal(tenantDefaultKnowledgeBaseNameText) {
			updates["name_text"] = tenantDefaultKnowledgeBaseNameText
		}
		if !item.DescriptionText.Equal(tenantDefaultKnowledgeBaseDescriptionText) {
			updates["description_text"] = tenantDefaultKnowledgeBaseDescriptionText
		}
		if item.AccessScope != string(enums.KnowledgeBaseAccessScopeTenant) {
			updates["access_scope"] = string(enums.KnowledgeBaseAccessScopeTenant)
		}
		if item.Status != enums.StatusOk {
			updates["status"] = enums.StatusOk
		}
		if len(updates) == 0 {
			return item, nil
		}
		updates["update_user_id"] = auditUserID(operator)
		updates["update_user_name"] = auditUserName(operator)
		updates["updated_at"] = time.Now()
		if err := repositories.KnowledgeBaseRepository.Updates(db, item.ID, updates); err != nil {
			return nil, err
		}
		return repositories.KnowledgeBaseRepository.Get(db, item.ID), nil
	}
	item = repositories.KnowledgeBaseRepository.FindOne(db, sqls.NewCnd().
		Eq("tenant_id", tenant.ID).
		Eq("name", tenantDefaultKnowledgeBaseName()).
		Eq("access_scope", string(enums.KnowledgeBaseAccessScopeTenant)).
		NotEq("status", enums.StatusDeleted).
		Asc("id"))
	if item != nil {
		if err := repositories.KnowledgeBaseRepository.Updates(db, item.ID, map[string]any{
			"remark":           tenantDefaultKnowledgeBaseRemark,
			"name_text":        tenantDefaultKnowledgeBaseNameText,
			"description_text": tenantDefaultKnowledgeBaseDescriptionText,
			"status":           enums.StatusOk,
			"update_user_id":   auditUserID(operator),
			"update_user_name": auditUserName(operator),
			"updated_at":       time.Now(),
		}); err != nil {
			return nil, err
		}
		return repositories.KnowledgeBaseRepository.Get(db, item.ID), nil
	}
	item = &models.KnowledgeBase{
		TenantID:              tenant.ID,
		Name:                  tenantDefaultKnowledgeBaseName(),
		Description:           tenantDefaultKnowledgeBaseDescription(),
		NameText:              tenantDefaultKnowledgeBaseNameText,
		DescriptionText:       tenantDefaultKnowledgeBaseDescriptionText,
		KnowledgeType:         string(enums.KnowledgeBaseTypeDocument),
		AccessScope:           string(enums.KnowledgeBaseAccessScopeTenant),
		Status:                enums.StatusOk,
		DefaultTopK:           10,
		DefaultScoreThreshold: 0.5,
		DefaultRerankLimit:    5,
		ChunkProvider:         string(enums.KnowledgeChunkProviderStructured),
		ChunkTargetTokens:     300,
		ChunkMaxTokens:        400,
		ChunkOverlapTokens:    40,
		AnswerMode:            2,
		Remark:                tenantDefaultKnowledgeBaseRemark,
		AuditFields:           utils.BuildAuditFields(operator),
	}
	if err := repositories.KnowledgeBaseRepository.Create(db, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *tenantDefaultAIAgentService) ensureAgent(
	db *gorm.DB,
	tenant *models.Tenant,
	knowledgeBase *models.KnowledgeBase,
	workflow *models.AIWorkflow,
	version *models.AIWorkflowVersion,
	operator *dto.AuthPrincipal,
) (*models.AIAgent, error) {
	description := tenantDefaultAgentDescription(tenant)
	descriptionText := tenantDefaultAgentDescriptionText(tenant)
	welcomeMessage := tenantDefaultAgentWelcomeMessage(tenant)
	serviceMode := tenantDefaultAgentServiceMode(tenant)
	teamIDs, handoffMode, err := tenantDefaultAgentDispatchSettingsDB(db, tenant, operator)
	if err != nil {
		return nil, err
	}
	agent := repositories.AIAgentRepository.Take(db,
		"tenant_id = ? AND product_id = 0 AND source = ? AND status <> ?",
		tenant.ID, TenantDefaultAIAgentSource, enums.StatusDeleted)
	if agent == nil {
		agent = &models.AIAgent{
			TenantID:            tenant.ID,
			ProductID:           0,
			Source:              TenantDefaultAIAgentSource,
			Name:                tenantDefaultAgentName(),
			Description:         description,
			NameText:            tenantDefaultAgentNameText,
			DescriptionText:     descriptionText,
			Status:              enums.StatusOk,
			ServiceMode:         serviceMode,
			SystemPrompt:        defaultTenantAgentSystemPrompt(tenant),
			WelcomeMessage:      welcomeMessage,
			ReplyTimeoutSeconds: 180,
			TeamIDs:             teamIDs,
			HandoffMode:         handoffMode,
			FallbackMode:        enums.AIAgentFallbackModeSuggestRetry,
			FallbackMessage:     workflowcapability.SafeKnowledgeFallbackMessage,
			KnowledgeIDs:        fmt.Sprint(knowledgeBase.ID),
			SkillIDs:            "",
			WorkflowID:          workflow.ID,
			WorkflowVersionID:   version.ID,
			DraftRevision:       1,
			ReviewStatus:        enums.AIAgentReviewStatusApproved,
			ReviewComment:       "系统初始化租户默认接待",
			SortNo:              -100,
			AuditFields:         utils.BuildAuditFields(operator),
		}
		if err := repositories.AIAgentRepository.Create(db, agent); err != nil {
			return nil, err
		}
		return agent, nil
	}
	// 展示文本不属于草稿配置，单独校正，避免为每个租户生成待审核的候选版本。
	if !agent.NameText.Equal(tenantDefaultAgentNameText) || !agent.DescriptionText.Equal(descriptionText) {
		if err := repositories.AIAgentRepository.Updates(db, agent.ID, map[string]any{
			"name_text":        tenantDefaultAgentNameText,
			"description_text": descriptionText,
		}); err != nil {
			return nil, err
		}
		agent.NameText = tenantDefaultAgentNameText
		agent.DescriptionText = descriptionText
	}

	knowledgeIDs := utils.SplitInt64s(agent.KnowledgeIDs)
	if !tenantDefaultContainsInt64(knowledgeIDs, knowledgeBase.ID) {
		knowledgeIDs = append(knowledgeIDs, knowledgeBase.ID)
	}
	nextKnowledgeIDs := utils.JoinInt64s(knowledgeIDs)
	updates := make(map[string]any)
	if agent.WorkflowID != workflow.ID {
		updates["workflow_id"] = workflow.ID
	}
	if agent.WorkflowVersionID != version.ID {
		updates["workflow_version_id"] = version.ID
	}
	if agent.Name != tenantDefaultAgentName() {
		updates["name"] = tenantDefaultAgentName()
	}
	if agent.Description != description {
		updates["description"] = description
	}
	if agent.ServiceMode != serviceMode {
		updates["service_mode"] = serviceMode
	}
	if agent.SystemPrompt != defaultTenantAgentSystemPrompt(tenant) {
		updates["system_prompt"] = defaultTenantAgentSystemPrompt(tenant)
	}
	if agent.WelcomeMessage != welcomeMessage {
		updates["welcome_message"] = welcomeMessage
	}
	if agent.TeamIDs != teamIDs {
		updates["team_ids"] = teamIDs
	}
	if agent.HandoffMode != handoffMode {
		updates["handoff_mode"] = handoffMode
	}
	if agent.FallbackMode != enums.AIAgentFallbackModeSuggestRetry {
		updates["fallback_mode"] = enums.AIAgentFallbackModeSuggestRetry
	}
	if agent.FallbackMessage != workflowcapability.SafeKnowledgeFallbackMessage {
		updates["fallback_message"] = workflowcapability.SafeKnowledgeFallbackMessage
	}
	if agent.KnowledgeIDs != nextKnowledgeIDs {
		updates["knowledge_ids"] = nextKnowledgeIDs
	}
	if agent.SkillIDs != "" {
		updates["skill_ids"] = ""
	}
	if strings.TrimSpace(agent.AllowedMCPTools) != "" {
		updates["allowed_mcp_tools"] = ""
	}
	if strings.TrimSpace(agent.AllowedGraphTools) != "" {
		updates["allowed_graph_tools"] = ""
	}
	if agent.Status != enums.StatusOk {
		updates["status"] = enums.StatusOk
	}
	if len(updates) == 0 {
		return agent, nil
	}
	updates["draft_revision"] = agent.DraftRevision + 1
	updates["update_user_id"] = auditUserID(operator)
	updates["update_user_name"] = auditUserName(operator)
	updates["updated_at"] = time.Now()
	if err := repositories.AIAgentRepository.Updates(db, agent.ID, updates); err != nil {
		return nil, err
	}
	return repositories.AIAgentRepository.Get(db, agent.ID), nil
}

func tenantDefaultAgentDispatchSettingsDB(db *gorm.DB, tenant *models.Tenant, operator *dto.AuthPrincipal) (string, enums.AIAgentHandoffMode, error) {
	if tenant == nil || !tenant.IsKnowledgeSupportScene() {
		return "", enums.AIAgentHandoffModeWaitPool, nil
	}
	team, err := ProductSupportOrganizationService.EnsureTenantTechnicalRepairTeamDB(db, tenant.ID, operator)
	if err != nil {
		return "", enums.AIAgentHandoffModeWaitPool, err
	}
	if team == nil || team.ID <= 0 {
		return "", enums.AIAgentHandoffModeWaitPool, fmt.Errorf("default maintenance team was not created")
	}
	return utils.JoinInt64s([]int64{team.ID}), enums.AIAgentHandoffModeDefaultTeamPool, nil
}

func (s *tenantDefaultAIAgentService) ensureActiveRelease(db *gorm.DB, agent *models.AIAgent, workflow *models.AIWorkflow, version *models.AIWorkflowVersion, operator *dto.AuthPrincipal) error {
	definition := dsl.Definition{}
	if err := json.Unmarshal([]byte(version.Definition), &definition); err != nil || !AIWorkflowService.ValidateDefinition(definition).Valid {
		return errorsx.InvalidParam("platform AI diagnosis workflow definition is invalid")
	}
	agentSnapshot, agentHash, err := buildAgentReleaseConfigSnapshotForDefinition(agent, &definition)
	if err != nil {
		return err
	}
	knowledgeSnapshot, knowledgeHash, err := AIAgentReleaseService.buildKnowledgeScopeSnapshot(db, agent)
	if err != nil {
		return err
	}
	if active := repositories.AIAgentReleaseRepository.Get(db, agent.ActiveReleaseID); active != nil &&
		active.AgentID == agent.ID &&
		active.TenantID == agent.TenantID &&
		active.ProductID == agent.ProductID &&
		active.Status == enums.StatusOk &&
		active.DeploymentStatus == models.AIAgentReleaseDeploymentActive &&
		active.ReviewStatus == enums.AIAgentReviewStatusApproved &&
		active.WorkflowID == workflow.ID &&
		active.WorkflowVersionID == version.ID &&
		active.WorkflowDefinitionHash == version.DefinitionHash &&
		active.AgentConfigHash == agentHash &&
		active.KnowledgeScopeHash == knowledgeHash {
		return nil
	}
	now := time.Now()
	isInitialRelease := agent.ActiveReleaseID <= 0
	activateImmediately := isInitialRelease || s.activeReleaseNeedsSafetyReplacement(db, agent, workflow, version)
	managedReviewComment := tenantDefaultManagedReleaseReviewComment(db, agent)
	existing := repositories.AIAgentReleaseRepository.Find(db, sqls.NewCnd().
		Eq("tenant_id", agent.TenantID).
		Eq("agent_id", agent.ID).
		Eq("workflow_version_id", version.ID).
		Eq("workflow_definition_hash", version.DefinitionHash).
		Eq("agent_config_hash", agentHash).
		Eq("knowledge_scope_hash", knowledgeHash).
		NotEq("status", enums.StatusDeleted).
		Desc("release_no"))
	if len(existing) > 0 {
		if activateImmediately {
			return s.activateManagedReleaseDB(db, agent, &existing[0], now, managedReviewComment, operator)
		}
		return nil
	}
	release := &models.AIAgentRelease{
		TenantID:               agent.TenantID,
		ProductID:              0,
		AgentID:                agent.ID,
		ReleaseNo:              repositories.AIAgentReleaseRepository.MaxReleaseNo(db, agent.ID) + 1,
		WorkflowID:             workflow.ID,
		WorkflowVersionID:      version.ID,
		WorkflowDefinitionHash: version.DefinitionHash,
		AgentConfigSnapshot:    agentSnapshot,
		AgentConfigHash:        agentHash,
		KnowledgeScopeSnapshot: knowledgeSnapshot,
		KnowledgeScopeHash:     knowledgeHash,
		ReviewStatus:           enums.AIAgentReviewStatusUnreviewed,
		ReviewComment:          "平台托管配置已同步，需按审核流程逐个部署",
		DeploymentStatus:       models.AIAgentReleaseDeploymentInactive,
		Status:                 enums.StatusOk,
		AuditFields:            utils.BuildAuditFields(operator),
	}
	if activateImmediately {
		release.ReviewStatus = enums.AIAgentReviewStatusApproved
		release.ReviewComment = managedReviewComment
		if isInitialRelease {
			release.ReviewComment = "系统初始化租户默认接待"
		}
		release.ReviewedAt = &now
		release.ReviewedByID = auditUserID(operator)
		release.ReviewedByName = auditUserName(operator)
	}
	if err := repositories.AIAgentReleaseRepository.Create(db, release); err != nil {
		return err
	}
	if !activateImmediately {
		return repositories.AIAgentRepository.Updates(db, agent.ID, map[string]any{
			"review_status":    enums.AIAgentReviewStatusUnreviewed,
			"review_comment":   release.ReviewComment,
			"reviewed_at":      nil,
			"reviewed_by_id":   0,
			"reviewed_by_name": "",
			"update_user_id":   auditUserID(operator),
			"update_user_name": auditUserName(operator),
			"updated_at":       now,
		})
	}
	return s.activateManagedReleaseDB(db, agent, release, now, release.ReviewComment, operator)
}

func (s *tenantDefaultAIAgentService) activeReleaseNeedsSafetyReplacement(db *gorm.DB, agent *models.AIAgent, workflow *models.AIWorkflow, version *models.AIWorkflowVersion) bool {
	if db == nil || agent == nil || workflow == nil || version == nil {
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
		return true
	}
	activeWorkflow := repositories.AIWorkflowRepository.Get(db, active.WorkflowID)
	activeVersion := repositories.AIWorkflowVersionRepository.Get(db, active.WorkflowVersionID)
	if activeWorkflow == nil || activeVersion == nil || activeWorkflow.Status == enums.StatusDeleted || activeVersion.Status != enums.StatusOk || activeVersion.WorkflowID != activeWorkflow.ID {
		return true
	}
	if activeWorkflow.Code != workflow.Code {
		return true
	}
	activeCapabilities, err := workflowVersionCapabilitySet(activeVersion)
	if err != nil {
		return true
	}
	activeDefinition := dsl.Definition{}
	if err := json.Unmarshal([]byte(activeVersion.Definition), &activeDefinition); err != nil {
		return true
	}
	nextDefinition := dsl.Definition{}
	if err := json.Unmarshal([]byte(version.Definition), &nextDefinition); err != nil {
		return true
	}
	tenant := repositories.PlatformIAMRepository.GetTenant(db, agent.TenantID)
	if tenant != nil && tenant.IsKnowledgeSupportScene() {
		activeSnapshot := dto.AIAgentReleaseConfigSnapshot{}
		if err := json.Unmarshal([]byte(active.AgentConfigSnapshot), &activeSnapshot); err != nil {
			return true
		}
		return !activeCapabilities.HumanHandoff ||
			activeCapabilities.TicketCreation ||
			(workflowDefinitionCarriesKnowledgeCitations(nextDefinition) && !workflowDefinitionCarriesKnowledgeCitations(activeDefinition)) ||
			activeSnapshot.ServiceMode != enums.IMConversationServiceModeAIFirst ||
			utils.JoinInt64s(activeSnapshot.TeamIDs) != agent.TeamIDs ||
			activeSnapshot.HandoffMode != agent.HandoffMode
	}
	if activeCapabilities.HumanHandoff || activeCapabilities.TicketCreation {
		return true
	}
	return false
}

func workflowDefinitionCarriesKnowledgeCitations(definition dsl.Definition) bool {
	knowledgeNodes := make(map[string]struct{})
	for _, node := range definition.Nodes {
		if node.Type == "knowledge_retrieve" {
			knowledgeNodes[node.ID] = struct{}{}
		}
	}
	for _, node := range definition.Nodes {
		if node.Type != "send_reply" {
			continue
		}
		selector, ok := node.Inputs["citations"]
		if !ok || selector.Field != "citations" {
			continue
		}
		if _, ok := knowledgeNodes[selector.NodeID]; ok {
			return true
		}
	}
	return false
}

func tenantDefaultManagedReleaseReviewComment(db *gorm.DB, agent *models.AIAgent) string {
	if db != nil && agent != nil {
		if tenant := repositories.PlatformIAMRepository.GetTenant(db, agent.TenantID); tenant != nil && tenant.IsKnowledgeSupportScene() {
			return "平台托管知识服务已同步到 AI 与人工协同流程"
		}
	}
	return "平台托管租户默认接待已同步到安全 AI-only 流程"
}

func (s *tenantDefaultAIAgentService) activateManagedReleaseDB(db *gorm.DB, agent *models.AIAgent, release *models.AIAgentRelease, now time.Time, reviewComment string, operator *dto.AuthPrincipal) error {
	if db == nil || agent == nil || release == nil {
		return errorsx.InvalidParam("tenant default AI agent release is invalid")
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
		reviewComment = "平台托管租户默认接待已同步到安全 AI-only 流程"
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

func defaultTenantAgentSystemPrompt(tenant *models.Tenant) string {
	name := "当前企业"
	if tenant != nil && strings.TrimSpace(tenant.Name) != "" {
		name = strings.TrimSpace(tenant.Name)
	}
	if tenant != nil && tenant.IsKnowledgeSupportScene() {
		return fmt.Sprintf(`你是 %s 的企业知识服务助手。
	优先依据企业已发布的通用知识库回答用户问题；一般问候和服务范围咨询可以自然回应。
	始终使用用户最新消息的主要语言回答；连续追问时保持上下文，不要因为知识不足自动切回中文。
	不得要求用户绑定产品或设备，也不要询问设备型号、序列号、故障码等与当前服务无关的信息。
	涉及制度、流程、服务承诺、费用、时效或其他事实时，只能依据已发布知识；资料不足时明确说明暂时无法确认，并询问完成回答所需的最少信息。
	用户明确要求人工帮助时，允许进入现有转人工流程；只有系统动作实际成功后才能说明已转接或已创建关联工单。
	回答直接、自然、简洁，不重复问候，不使用 Emoji。`, name)
	}
	return fmt.Sprintf(`你是 %s 的企业服务助手。
	先结合企业通用知识库回答咨询；问候、服务范围和一般沟通应自然回应，不要机械要求用户先提供设备信息。
	始终使用用户最新消息的主要语言回答；连续追问时保持上下文，不要因为知识不足自动切回中文。
	只有问题确实涉及具体产品或故障时，才逐步询问产品名称、设备型号、序列号、故障现象、故障码、所在地区和已尝试步骤。
	当前会话未绑定具体设备时，不编造设备参数、故障结论、维修步骤或保修承诺。识别到具体产品后，引导用户进入对应产品服务。
	涉及保修范围、服务时间或服务区域时，只能依据已发布知识或正式服务协议；资料不足时明确说无法确认，并询问国家或地区、合同或保修凭证等必要信息。
	出现高温、焦味、烟雾、漏电、异常声响、漏液、保护装置异常，或设备状态无法确认时，先给出停止运行、断电隔离、避免重复上电和避免自行拆机的通用安全边界；不要声称这是具体产品的维修结论。
	当前入口只提供 AI 问诊，不提供转人工或创建工单；不要向用户承诺这些能力。
	回答直接、自然、简洁，不重复问候，不使用 Emoji。`, name)
}

func tenantDefaultAgentServiceMode(tenant *models.Tenant) enums.IMConversationServiceMode {
	if tenant != nil && tenant.IsKnowledgeSupportScene() {
		return enums.IMConversationServiceModeAIFirst
	}
	return enums.IMConversationServiceModeAIOnly
}

func tenantDefaultAgentDescriptionText(tenant *models.Tenant) i18nx.Text {
	if tenant != nil && tenant.IsKnowledgeSupportScene() {
		return i18nx.NewText("builtin.aiAgent.tenantDefault.description.knowledgeSupport", nil)
	}
	return i18nx.NewText("builtin.aiAgent.tenantDefault.description.generalService", nil)
}

func tenantDefaultAgentDescription(tenant *models.Tenant) string {
	return tenantDefaultAgentDescriptionText(tenant).Render(i18nx.LocaleZhCN)
}

func tenantDefaultAgentWelcomeMessage(tenant *models.Tenant) string {
	return tenantDefaultAgentWelcomeMessageForLocale(tenant, i18nx.DefaultLocale)
}

func tenantDefaultAgentWelcomeMessageForLocale(tenant *models.Tenant, locale string) string {
	if tenant != nil && tenant.IsKnowledgeSupportScene() {
		return i18nx.Getf(locale, "conversation.welcome.knowledgeSupport")
	}
	return i18nx.Getf(locale, "conversation.welcome.generalService")
}

func tenantDefaultContainsInt64(items []int64, target int64) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
