package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	workflowbuiltin "remotehelpdesk/internal/ai/workflow/builtin"
	workflowcapability "remotehelpdesk/internal/ai/workflow/capability"
	"remotehelpdesk/internal/ai/workflow/dsl"
	workflowregistry "remotehelpdesk/internal/ai/workflow/registry"
	workflowvalidator "remotehelpdesk/internal/ai/workflow/validator"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx/params"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var AIWorkflowService = newAIWorkflowService()

var loadPlatformBuiltInWorkflowManifests = workflowbuiltin.Load

const (
	PlatformDefaultAfterSalesWorkflowCode = "aftersales_customer_service_default"
	PlatformDeviceAIOnlyWorkflowCode      = "aftersales_device_diagnosis_ai_only"
	PlatformKnowledgeSupportWorkflowCode  = "knowledge_customer_service_default"
	PlatformDispatchOnlyWorkflowCode      = "aftersales_customer_service_dispatch_only"
	retiredBasicAIWorkflowCode            = "aftersales_customer_service_ai_only"
)

type platformWorkflowSpec struct {
	code          string
	name          string
	description   string
	definition    dsl.Definition
	initialChange string
	upgradeChange string
}

func newAIWorkflowService() *aiWorkflowService {
	return &aiWorkflowService{
		registry: workflowregistry.DefaultRegistry(),
	}
}

type aiWorkflowService struct {
	registry *workflowregistry.Registry
}

type AIWorkflowRunAuditItem struct {
	Run      models.AIWorkflowRun
	Workflow *models.AIWorkflow
	Version  *models.AIWorkflowVersion
	Agent    *models.AIAgent
	Skill    dto.AIWorkflowSkillAudit
}

type workflowSkillRuntimeTrace struct {
	MiddlewareEnabled bool
	VisibleIDs        []int64
	SelectedID        int64
	SelectedName      string
	SelectedDesc      string
	RouteReason       string
	RouteTrace        string
	ExposedToolCodes  []string
	InvokedToolCodes  []string
	score             int
}

func (s *aiWorkflowService) Get(id int64) *models.AIWorkflow {
	if id <= 0 {
		return nil
	}
	return repositories.AIWorkflowRepository.Get(sqls.DB(), id)
}

func (s *aiWorkflowService) GetForOperator(id int64, operator *dto.AuthPrincipal) *models.AIWorkflow {
	item := s.Get(id)
	if item == nil || operator == nil {
		return nil
	}
	if operator.IsPlatform() && operator.TenantID <= 0 {
		return item
	}
	if item.Scope == models.AIWorkflowScopePlatform && item.TenantID == 0 {
		return item
	}
	if operator.TenantID > 0 && item.Scope == models.AIWorkflowScopeTenant && item.TenantID == operator.TenantID {
		return item
	}
	return nil
}

func (s *aiWorkflowService) GetVersion(id int64) *models.AIWorkflowVersion {
	if id <= 0 {
		return nil
	}
	return repositories.AIWorkflowVersionRepository.Get(sqls.DB(), id)
}

func (s *aiWorkflowService) GetVersionForOperator(id int64, operator *dto.AuthPrincipal) *models.AIWorkflowVersion {
	item := s.GetVersion(id)
	if item == nil || s.GetForOperator(item.WorkflowID, operator) == nil {
		return nil
	}
	return item
}

func (s *aiWorkflowService) Find(cnd *sqls.Cnd) []models.AIWorkflow {
	return repositories.AIWorkflowRepository.Find(sqls.DB(), cnd)
}

func (s *aiWorkflowService) FindPageByCnd(cnd *sqls.Cnd) (list []models.AIWorkflow, paging *sqls.Paging) {
	return repositories.AIWorkflowRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *aiWorkflowService) FindVersionPageByParams(params *params.QueryParams) (list []models.AIWorkflowVersion, paging *sqls.Paging) {
	return repositories.AIWorkflowVersionRepository.FindPageByParams(sqls.DB(), params)
}

func (s *aiWorkflowService) FindVersionPageByCnd(cnd *sqls.Cnd) (list []models.AIWorkflowVersion, paging *sqls.Paging) {
	return repositories.AIWorkflowVersionRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *aiWorkflowService) FindRunPageByCnd(cnd *sqls.Cnd) (list []models.AIWorkflowRun, paging *sqls.Paging) {
	return repositories.AIWorkflowRunRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *aiWorkflowService) BuildRunAuditItems(list []models.AIWorkflowRun) []AIWorkflowRunAuditItem {
	ret := make([]AIWorkflowRunAuditItem, 0, len(list))
	if len(list) == 0 {
		return ret
	}
	workflowIDs := make([]int64, 0, len(list))
	versionIDs := make([]int64, 0, len(list))
	agentIDs := make([]int64, 0, len(list))
	runIDs := make([]int64, 0, len(list))
	skillIDs := make([]int64, 0)
	skillTraceByRunID := make(map[int64]workflowSkillRuntimeTrace, len(list))
	for _, item := range list {
		workflowIDs = appendNonZeroInt64(workflowIDs, item.WorkflowID)
		versionIDs = appendNonZeroInt64(versionIDs, item.WorkflowVersionID)
		agentIDs = appendNonZeroInt64(agentIDs, item.AIAgentID)
		runIDs = appendNonZeroInt64(runIDs, item.ID)
		trace := parseWorkflowSkillRuntimeTrace(item.TraceData)
		skillTraceByRunID[item.ID] = trace
		for _, skillID := range trace.VisibleIDs {
			skillIDs = appendNonZeroInt64(skillIDs, skillID)
		}
		skillIDs = appendNonZeroInt64(skillIDs, trace.SelectedID)
	}
	var workflows []models.AIWorkflow
	if len(workflowIDs) > 0 {
		workflows = repositories.AIWorkflowRepository.Find(sqls.DB(), sqls.NewCnd().In("id", workflowIDs))
	}
	var versions []models.AIWorkflowVersion
	if len(versionIDs) > 0 {
		versions = repositories.AIWorkflowVersionRepository.Find(sqls.DB(), sqls.NewCnd().In("id", versionIDs))
	}
	var agents []models.AIAgent
	if len(agentIDs) > 0 {
		agents = repositories.AIAgentRepository.Find(sqls.DB(), sqls.NewCnd().In("id", agentIDs))
	}
	var skillLogs []models.SkillRunLog
	if len(runIDs) > 0 {
		skillLogs = repositories.SkillRunLogRepository.Find(sqls.DB(), sqls.NewCnd().In("workflow_run_id", runIDs).Desc("id"))
	}
	skillLogByRunID := make(map[int64]*models.SkillRunLog, len(skillLogs))
	for i := range skillLogs {
		item := skillLogs[i]
		if _, exists := skillLogByRunID[item.WorkflowRunID]; exists {
			continue
		}
		skillLogByRunID[item.WorkflowRunID] = &item
		skillIDs = appendNonZeroInt64(skillIDs, item.SkillDefinitionID)
	}
	skillByID := repositories.SkillDefinitionRepository.GetByIDs(sqls.DB(), skillIDs)
	workflowByID := make(map[int64]*models.AIWorkflow, len(workflows))
	for i := range workflows {
		item := workflows[i]
		workflowByID[item.ID] = &item
	}
	versionByID := make(map[int64]*models.AIWorkflowVersion, len(versions))
	for i := range versions {
		item := versions[i]
		versionByID[item.ID] = &item
	}
	agentByID := make(map[int64]*models.AIAgent, len(agents))
	for i := range agents {
		item := agents[i]
		agentByID[item.ID] = &item
	}
	for _, run := range list {
		ret = append(ret, AIWorkflowRunAuditItem{
			Run:      run,
			Workflow: workflowByID[run.WorkflowID],
			Version:  versionByID[run.WorkflowVersionID],
			Agent:    agentByID[run.AIAgentID],
			Skill:    buildWorkflowSkillAudit(run, skillTraceByRunID[run.ID], skillLogByRunID[run.ID], skillByID),
		})
	}
	return ret
}

func buildWorkflowSkillAudit(run models.AIWorkflowRun, trace workflowSkillRuntimeTrace, log *models.SkillRunLog, skillByID map[int64]models.SkillDefinition) dto.AIWorkflowSkillAudit {
	audit := dto.AIWorkflowSkillAudit{
		State:                    dto.AIWorkflowSkillStateNotEnabled,
		MiddlewareEnabled:        trace.MiddlewareEnabled,
		CandidateSkills:          make([]dto.AIWorkflowSkillCandidate, 0, len(trace.VisibleIDs)),
		SelectedSkillID:          trace.SelectedID,
		SelectedSkillName:        trace.SelectedName,
		SelectedSkillDescription: trace.SelectedDesc,
		MatchReason:              strings.TrimSpace(trace.RouteReason),
		RouteTrace:               strings.TrimSpace(trace.RouteTrace),
		ExposedToolCodes:         append([]string(nil), trace.ExposedToolCodes...),
		InvokedToolCodes:         append([]string(nil), trace.InvokedToolCodes...),
		SourceMessageID:          run.MessageID,
	}
	for _, id := range trace.VisibleIDs {
		item, exists := skillByID[id]
		if !exists {
			audit.CandidateSkills = append(audit.CandidateSkills, dto.AIWorkflowSkillCandidate{ID: id})
			continue
		}
		audit.CandidateSkills = append(audit.CandidateSkills, dto.AIWorkflowSkillCandidate{
			ID:          item.ID,
			Name:        item.Name,
			Description: item.Description,
		})
	}
	if trace.MiddlewareEnabled || len(audit.CandidateSkills) > 0 {
		audit.State = dto.AIWorkflowSkillStateNotSelected
	}
	if log != nil && log.SkillDefinitionID > 0 {
		audit.SelectedSkillID = log.SkillDefinitionID
		audit.MatchReason = strings.TrimSpace(log.MatchReason)
		audit.SourceMessageID = log.SourceMessageID
		audit.CreatedAt = log.CreatedAt
		if strings.TrimSpace(log.TraceData) != "" {
			audit.RouteTrace = strings.TrimSpace(log.TraceData)
		}
	}
	if audit.SelectedSkillID > 0 {
		audit.State = dto.AIWorkflowSkillStateSelected
		if item, exists := skillByID[audit.SelectedSkillID]; exists {
			audit.SelectedSkillName = item.Name
			audit.SelectedSkillDescription = item.Description
		}
	}
	return audit
}

func parseWorkflowSkillRuntimeTrace(raw string) workflowSkillRuntimeTrace {
	var value any
	if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &value) != nil {
		return workflowSkillRuntimeTrace{}
	}
	ret := workflowSkillRuntimeTrace{}
	walkWorkflowSkillTrace(value, &ret)
	return ret
}

func walkWorkflowSkillTrace(value any, ret *workflowSkillRuntimeTrace) {
	switch current := value.(type) {
	case map[string]any:
		if skill, ok := current["skill"].(map[string]any); ok {
			candidate := workflowSkillRuntimeTrace{
				MiddlewareEnabled: boolValue(skill["middlewareEnabled"]),
				VisibleIDs:        int64SliceValue(skill["visibleIds"]),
				SelectedID:        int64Value(skill["id"]),
				SelectedName:      stringValue(skill["name"]),
				SelectedDesc:      stringValue(skill["description"]),
				RouteReason:       stringValue(skill["routeReason"]),
				RouteTrace:        stringValue(skill["routeTrace"]),
				ExposedToolCodes:  stringSliceValue(skill["filteredToolCodes"]),
			}
			candidate.InvokedToolCodes = invokedToolCodes(current["tools"])
			candidate.score = len(candidate.VisibleIDs)
			if candidate.MiddlewareEnabled {
				candidate.score += 100
			}
			if candidate.SelectedID > 0 {
				candidate.score += 1000
			}
			if candidate.score > ret.score {
				*ret = candidate
			}
		}
		for _, child := range current {
			walkWorkflowSkillTrace(child, ret)
		}
	case []any:
		for _, child := range current {
			walkWorkflowSkillTrace(child, ret)
		}
	}
}

func invokedToolCodes(value any) []string {
	tools, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	items, ok := tools["items"].([]any)
	if !ok {
		return nil
	}
	ret := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		code := strings.TrimSpace(stringValue(item["toolCode"]))
		if code == "" {
			continue
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		ret = append(ret, code)
	}
	return ret
}

func stringSliceValue(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	ret := make([]string, 0, len(items))
	for _, item := range items {
		if text := strings.TrimSpace(stringValue(item)); text != "" {
			ret = append(ret, text)
		}
	}
	return ret
}

func int64SliceValue(value any) []int64 {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	ret := make([]int64, 0, len(items))
	for _, item := range items {
		if id := int64Value(item); id > 0 {
			ret = append(ret, id)
		}
	}
	return ret
}

func int64Value(value any) int64 {
	switch current := value.(type) {
	case float64:
		return int64(current)
	case int64:
		return current
	case json.Number:
		ret, _ := current.Int64()
		return ret
	case string:
		ret, _ := strconv.ParseInt(strings.TrimSpace(current), 10, 64)
		return ret
	default:
		return 0
	}
}

func stringValue(value any) string {
	ret, _ := value.(string)
	return ret
}

func boolValue(value any) bool {
	ret, _ := value.(bool)
	return ret
}

func (s *aiWorkflowService) GetRunDetail(id int64) (*models.AIWorkflowRun, []models.AIWorkflowNodeRun) {
	if id <= 0 {
		return nil, nil
	}
	run := repositories.AIWorkflowRunRepository.Get(sqls.DB(), id)
	if run == nil {
		return nil, nil
	}
	nodes := repositories.AIWorkflowNodeRunRepository.Find(sqls.DB(), sqls.NewCnd().Eq("workflow_run_id", id).Asc("id"))
	return run, nodes
}

func (s *aiWorkflowService) BuildRunHumanHandlingAudit(run *models.AIWorkflowRun) *dto.AIWorkflowHumanHandlingAudit {
	if run == nil || run.ConversationID <= 0 || run.MessageID <= 0 {
		return nil
	}
	conversation := repositories.ConversationRepository.Get(sqls.DB(), run.ConversationID)
	if conversation == nil {
		return nil
	}
	nextRun := repositories.AIWorkflowRunRepository.FindNextByConversationIDAndMessageID(sqls.DB(), run.ConversationID, run.MessageID)
	nextMessageID := int64(0)
	if nextRun != nil {
		nextMessageID = nextRun.MessageID
	}
	firstReply := repositories.MessageRepository.FindFirstValidByConversationIDAndSenderTypeBetweenMessageIDs(
		sqls.DB(),
		run.ConversationID,
		enums.IMSenderTypeAgent,
		run.MessageID,
		nextMessageID,
	)
	linkedTicket := findHumanHandledTicketByConversationID(run.ConversationID)
	handoffOccurred := workflowRunContainsHandoff(run, nextRun, conversation.HandoffAt)
	if !handoffOccurred && firstReply == nil && linkedTicket == nil {
		return nil
	}

	ret := &dto.AIWorkflowHumanHandlingAudit{
		HandoffOccurred:        handoffOccurred,
		HandoffReason:          strings.TrimSpace(conversation.HandoffReason),
		HandledByHuman:         firstReply != nil || linkedTicket != nil,
		ConversationStatus:     conversation.Status,
		ConversationStatusName: enums.GetIMConversationStatusLabel(conversation.Status),
	}
	if handoffOccurred {
		if conversation.HandoffAt != nil {
			ret.HandoffAt = *conversation.HandoffAt
		}
	}
	if firstReply != nil {
		ret.HandlerUserID = firstReply.SenderID
		ret.HandlerName = workflowRunHandlerName(firstReply.SenderID)
		ret.FirstHumanReplyMessageID = firstReply.ID
		ret.FirstHumanReplyAt = firstReply.CreatedAt
		if firstReply.SentAt != nil {
			ret.FirstHumanReplyAt = *firstReply.SentAt
		}
	} else if linkedTicket != nil {
		ret.HandlerUserID = linkedTicket.CurrentAssigneeID
		ret.HandlerName = workflowRunHandlerName(linkedTicket.CurrentAssigneeID)
		ret.FirstHumanReplyAt = firstNonZeroTicketHumanHandledAt(linkedTicket)
	}
	return ret
}

func findHumanHandledTicketByConversationID(conversationID int64) *models.Ticket {
	if conversationID <= 0 {
		return nil
	}
	ticket := repositories.TicketRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("conversation_id", conversationID).
		Desc("id"))
	if ticket == nil || ticket.CurrentAssigneeID <= 0 {
		return nil
	}
	if ticket.AcceptedAt != nil || ticket.HandledAt != nil || ticket.ResolvedAt != nil {
		return ticket
	}
	switch enums.NormalizeTicketStatus(string(ticket.Status)) {
	case enums.TicketStatusAccepted,
		enums.TicketStatusProcessing,
		enums.TicketStatusVideoSupport,
		enums.TicketStatusSupplierSupport,
		enums.TicketStatusResolved,
		enums.TicketStatusPendingCustomerConfirm,
		enums.TicketStatusQualityReview,
		enums.TicketStatusClosed:
		return ticket
	default:
		return nil
	}
}

func firstNonZeroTicketHumanHandledAt(ticket *models.Ticket) time.Time {
	if ticket == nil {
		return time.Time{}
	}
	for _, value := range []*time.Time{ticket.AcceptedAt, ticket.ResolvedAt, ticket.HandledAt} {
		if value != nil && !value.IsZero() {
			return *value
		}
	}
	return ticket.UpdatedAt
}

func workflowRunContainsHandoff(run, nextRun *models.AIWorkflowRun, handoffAt *time.Time) bool {
	if run == nil || handoffAt == nil || handoffAt.Before(run.StartedAt) {
		return false
	}
	return nextRun == nil || handoffAt.Before(nextRun.StartedAt)
}

func workflowRunHandlerName(userID int64) string {
	if userID <= 0 {
		return ""
	}
	if profile := AgentProfileService.GetByUserID(userID); profile != nil {
		if name := strings.TrimSpace(profile.DisplayName); name != "" {
			return name
		}
	}
	if user := UserService.Get(userID); user != nil {
		if name := strings.TrimSpace(user.Nickname); name != "" {
			return name
		}
		return strings.TrimSpace(user.Username)
	}
	return ""
}

func appendNonZeroInt64(list []int64, value int64) []int64 {
	if value <= 0 {
		return list
	}
	for _, item := range list {
		if item == value {
			return list
		}
	}
	return append(list, value)
}

func (s *aiWorkflowService) GetByAgentID(agentID int64) *models.AIWorkflow {
	if agentID <= 0 {
		return nil
	}
	if agent := repositories.AIAgentRepository.Get(sqls.DB(), agentID); agent != nil && agent.WorkflowID > 0 {
		if workflow := repositories.AIWorkflowRepository.Get(sqls.DB(), agent.WorkflowID); workflow != nil && workflow.Status != enums.StatusDeleted {
			return workflow
		}
	}
	return repositories.AIWorkflowRepository.Take(sqls.DB(), "agent_id = ? AND status <> ?", agentID, enums.StatusDeleted)
}

func (s *aiWorkflowService) GetOrCreateAgentWorkflow(agentID int64, operator *dto.AuthPrincipal) (*models.AIWorkflow, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if agentID <= 0 {
		return nil, errorsx.InvalidParam("agent id is required")
	}
	if agent := AIAgentService.GetForOperator(agentID, operator); agent == nil || agent.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParamI18n("error.e0002")
	}
	if item := s.GetByAgentID(agentID); item != nil {
		return item, nil
	}
	var item *models.AIWorkflow
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if current := repositories.AIWorkflowRepository.Take(ctx.Tx, "agent_id = ? AND status <> ?", agentID, enums.StatusDeleted); current != nil {
			item = current
			return nil
		}
		agent := repositories.AIAgentRepository.Get(ctx.Tx, agentID)
		if agent == nil || agent.Status == enums.StatusDeleted {
			return errorsx.InvalidParamI18n("error.e0002")
		}
		if agent.ProductID > 0 {
			profile := repositories.ProductServiceProfileRepository.GetByProductID(ctx.Tx, agent.ProductID)
			if profile == nil || profile.TenantID != agent.TenantID || profile.Status == enums.StatusDeleted {
				return errorsx.InvalidParam("product service profile not found")
			}
			if err := s.BindProductAgentWorkflowDB(ctx.Tx, agent, profile); err != nil {
				return err
			}
			item = repositories.AIWorkflowRepository.Get(ctx.Tx, agent.WorkflowID)
			if item == nil {
				return errorsx.InvalidParam("bound workflow does not exist")
			}
			return nil
		}
		workflow, stableVersion, err := s.EnsurePlatformDefaultWorkflowDB(ctx.Tx)
		if err != nil {
			return err
		}
		if err := repositories.AIAgentRepository.Updates(ctx.Tx, agent.ID, map[string]any{
			"workflow_id":         workflow.ID,
			"workflow_version_id": stableVersion.ID,
			"draft_revision":      agent.DraftRevision + 1,
			"update_user_id":      operator.UserID,
			"update_user_name":    operator.Username,
			"updated_at":          time.Now(),
		}); err != nil {
			return err
		}
		item = workflow
		return nil
	})
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (s *aiWorkflowService) ListNodeSpecs() []workflowregistry.NodeSpec {
	return s.registry.List()
}

func (s *aiWorkflowService) DefaultAgentWorkflowDefinition() dsl.Definition {
	return mustPlatformBuiltInWorkflowDefinition(PlatformDefaultAfterSalesWorkflowCode)
}

func (s *aiWorkflowService) DeviceAIOnlyWorkflowDefinition() dsl.Definition {
	return mustPlatformBuiltInWorkflowDefinition(PlatformDeviceAIOnlyWorkflowCode)
}

func (s *aiWorkflowService) KnowledgeSupportWorkflowDefinition() dsl.Definition {
	return mustPlatformBuiltInWorkflowDefinition(PlatformKnowledgeSupportWorkflowCode)
}

func (s *aiWorkflowService) DispatchOnlyWorkflowDefinition() dsl.Definition {
	return mustPlatformBuiltInWorkflowDefinition(PlatformDispatchOnlyWorkflowCode)
}

func PlatformBuiltInWorkflowCodes() []string {
	return []string{
		PlatformDefaultAfterSalesWorkflowCode,
		PlatformDeviceAIOnlyWorkflowCode,
		PlatformKnowledgeSupportWorkflowCode,
		PlatformDispatchOnlyWorkflowCode,
	}
}

func IsPlatformBuiltInWorkflow(item *models.AIWorkflow) bool {
	return item != nil && item.TenantID == 0 && item.AgentID == 0 && item.Scope == models.AIWorkflowScopePlatform && IsPlatformBuiltInWorkflowCode(item.Code)
}

func IsPlatformBuiltInWorkflowCode(code string) bool {
	switch strings.TrimSpace(code) {
	case PlatformDefaultAfterSalesWorkflowCode, PlatformDeviceAIOnlyWorkflowCode, PlatformKnowledgeSupportWorkflowCode, PlatformDispatchOnlyWorkflowCode:
		return true
	default:
		return false
	}
}

// IsReusableTemplateForOperator identifies the workflow by ownership and
// mutability rather than by a business workflow code. New platform templates
// and tenant templates therefore use the same binding path.
func (s *aiWorkflowService) IsReusableTemplateForOperator(item *models.AIWorkflow, operator *dto.AuthPrincipal) bool {
	if item == nil || operator == nil || item.Status == enums.StatusDeleted || item.AgentID != 0 {
		return false
	}
	if item.Scope == models.AIWorkflowScopePlatform {
		return item.TenantID == 0 && item.Locked
	}
	return item.Scope == models.AIWorkflowScopeTenant && operator.TenantID > 0 && item.TenantID == operator.TenantID
}

// AgentAllowsHumanHandoff resolves the immutable production version and fails
// closed when no governed workflow is available.
func (s *aiWorkflowService) AgentAllowsHumanHandoff(agent *models.AIAgent) bool {
	if agent == nil {
		return false
	}
	if agent.ServiceMode == enums.IMConversationServiceModeAIOnly {
		return false
	}
	if agent.ActiveReleaseID <= 0 && agent.WorkflowVersionID <= 0 {
		return true
	}
	capabilities, ok := s.agentProductionWorkflowCapabilities(agent)
	return ok && capabilities.HumanHandoff
}

// AgentAllowsTicketCreation uses the same immutable production workflow
// boundary as human handoff. Legacy agents without a governed workflow do not
// expose a customer-side ticket shortcut.
func (s *aiWorkflowService) AgentAllowsTicketCreation(agent *models.AIAgent) bool {
	if agent == nil || agent.ServiceMode == enums.IMConversationServiceModeAIOnly {
		return false
	}
	capabilities, ok := s.agentProductionWorkflowCapabilities(agent)
	return ok && capabilities.TicketCreation
}

func (s *aiWorkflowService) agentProductionWorkflowCapabilities(agent *models.AIAgent) (workflowcapability.Set, bool) {
	if agent == nil {
		return workflowcapability.Set{}, false
	}
	workflowVersionID := agent.WorkflowVersionID
	workflowID := agent.WorkflowID
	usingActiveRelease := false
	if agent.ActiveReleaseID > 0 {
		release := repositories.AIAgentReleaseRepository.Get(sqls.DB(), agent.ActiveReleaseID)
		if release == nil || release.Status != enums.StatusOk || release.AgentID != agent.ID || release.TenantID != agent.TenantID || release.ProductID != agent.ProductID || release.ReviewStatus != enums.AIAgentReviewStatusApproved || release.DeploymentStatus != models.AIAgentReleaseDeploymentActive {
			return workflowcapability.Set{}, false
		}
		workflowVersionID = release.WorkflowVersionID
		workflowID = release.WorkflowID
		usingActiveRelease = true
	}
	if workflowVersionID <= 0 {
		return workflowcapability.Set{}, false
	}
	version := repositories.AIWorkflowVersionRepository.Get(sqls.DB(), workflowVersionID)
	if version == nil || version.Status != enums.StatusOk || workflowID <= 0 || version.WorkflowID != workflowID {
		return workflowcapability.Set{}, false
	}
	workflow := repositories.AIWorkflowRepository.Get(sqls.DB(), workflowID)
	if workflow == nil || workflow.Status == enums.StatusDeleted {
		return workflowcapability.Set{}, false
	}
	if IsPlatformBuiltInWorkflow(workflow) && version.ReleaseChannel != models.AIWorkflowReleaseChannelStable {
		return workflowcapability.Set{}, false
	}
	if !usingActiveRelease && workflow.CurrentStableVersionID != version.ID {
		return workflowcapability.Set{}, false
	}
	capabilities, err := workflowVersionCapabilitySet(version)
	if err != nil {
		return workflowcapability.Set{}, false
	}
	return capabilities, true
}

func workflowVersionCapabilitySet(version *models.AIWorkflowVersion) (workflowcapability.Set, error) {
	if version == nil {
		return workflowcapability.Set{}, errorsx.InvalidParam("workflow version is required")
	}
	definition := dsl.Definition{}
	if err := json.Unmarshal([]byte(version.Definition), &definition); err != nil {
		return workflowcapability.Set{}, errorsx.InvalidParam("workflow version definition is invalid")
	}
	if result := AIWorkflowService.ValidateDefinition(definition); !result.Valid {
		return workflowcapability.Set{}, errorsx.InvalidParam("workflow version definition is invalid")
	}
	return workflowcapability.FromDefinition(definition), nil
}

func workflowAgentRestrictionUpdates(agent *models.AIAgent, version *models.AIWorkflowVersion) (map[string]any, error) {
	capabilities, err := workflowVersionCapabilitySet(version)
	if err != nil {
		return nil, err
	}
	updates := make(map[string]any)
	if agent == nil || capabilities.HumanHandoff {
		return updates, nil
	}
	if agent.ServiceMode != enums.IMConversationServiceModeAIOnly {
		updates["service_mode"] = enums.IMConversationServiceModeAIOnly
	}
	safeFallback := capabilities.SanitizeFallbackMessage(agent.FallbackMessage)
	if safeFallback != strings.TrimSpace(agent.FallbackMessage) {
		updates["fallback_message"] = safeFallback
	}
	return updates, nil
}

func applyWorkflowAgentRestrictions(agent *models.AIAgent, updates map[string]any) {
	if agent == nil {
		return
	}
	if value, ok := updates["service_mode"].(enums.IMConversationServiceMode); ok {
		agent.ServiceMode = value
	}
	if value, ok := updates["fallback_message"].(string); ok {
		agent.FallbackMessage = value
	}
}

// EnsurePlatformDefaultWorkflowDB materializes the built-in workflow as one
// immutable, shared platform version. It is idempotent and safe for startup,
// migrations, and product provisioning.
func (s *aiWorkflowService) EnsurePlatformDefaultWorkflowDB(db *gorm.DB) (*models.AIWorkflow, *models.AIWorkflowVersion, error) {
	return s.ensurePlatformBuiltInWorkflowCodeDB(db, PlatformDefaultAfterSalesWorkflowCode)
}

// EnsurePlatformDeviceAIOnlyWorkflowDB materializes the AI diagnosis workflow.
// Bound devices receive model-aware diagnosis; unbound entries receive basic
// product service. Neither path exposes tickets or human handoff.
func (s *aiWorkflowService) EnsurePlatformDeviceAIOnlyWorkflowDB(db *gorm.DB) (*models.AIWorkflow, *models.AIWorkflowVersion, error) {
	return s.ensurePlatformBuiltInWorkflowCodeDB(db, PlatformDeviceAIOnlyWorkflowCode)
}

// EnsurePlatformKnowledgeSupportWorkflowDB materializes the tenant-scoped
// knowledge consultation workflow. It deliberately has no product or device
// branches and uses only the tenant default model credential and knowledge.
func (s *aiWorkflowService) EnsurePlatformKnowledgeSupportWorkflowDB(db *gorm.DB) (*models.AIWorkflow, *models.AIWorkflowVersion, error) {
	return s.ensurePlatformBuiltInWorkflowCodeDB(db, PlatformKnowledgeSupportWorkflowCode)
}

func (s *aiWorkflowService) EnsureTenantDefaultWorkflowDB(db *gorm.DB, tenant *models.Tenant) (*models.AIWorkflow, *models.AIWorkflowVersion, error) {
	if tenant != nil && tenant.IsKnowledgeSupportScene() {
		return s.EnsurePlatformKnowledgeSupportWorkflowDB(db)
	}
	return s.EnsurePlatformDeviceAIOnlyWorkflowDB(db)
}

func TenantDefaultWorkflowCode(tenant *models.Tenant) string {
	if tenant != nil && tenant.IsKnowledgeSupportScene() {
		return PlatformKnowledgeSupportWorkflowCode
	}
	return PlatformDeviceAIOnlyWorkflowCode
}

// EnsurePlatformDispatchOnlyWorkflowDB materializes the rule-driven dispatch
// workflow. It routes customers into tickets, handoff, and follow-up actions
// without requiring a model-backed answer path.
func (s *aiWorkflowService) EnsurePlatformDispatchOnlyWorkflowDB(db *gorm.DB) (*models.AIWorkflow, *models.AIWorkflowVersion, error) {
	return s.ensurePlatformBuiltInWorkflowCodeDB(db, PlatformDispatchOnlyWorkflowCode)
}

// EnsurePlatformBuiltInWorkflowsDB materializes every registered platform
// template. Migrations and startup code use this registry-level entry point so
// adding a template does not require another sequence of feature-specific calls.
func (s *aiWorkflowService) EnsurePlatformBuiltInWorkflowsDB(db *gorm.DB) error {
	specs, err := platformBuiltInWorkflowSpecs()
	if err != nil {
		return err
	}
	for _, spec := range specs {
		if _, _, err := s.ensurePlatformBuiltInWorkflowDB(db, spec); err != nil {
			return err
		}
	}
	return s.RetireRemovedPlatformBuiltInWorkflowsDB(db)
}

// RetireRemovedPlatformBuiltInWorkflowsDB prevents an old application binary,
// a backup restore, or manual data repair from reviving a platform template
// that is no longer shipped in the embedded manifest registry.
func (s *aiWorkflowService) RetireRemovedPlatformBuiltInWorkflowsDB(db *gorm.DB) error {
	if db == nil {
		return errorsx.InvalidParam("database is required")
	}
	replacementWorkflow, replacementVersion, err := s.EnsurePlatformDeviceAIOnlyWorkflowDB(db)
	if err != nil {
		return err
	}
	if replacementWorkflow == nil || replacementVersion == nil || replacementVersion.WorkflowID != replacementWorkflow.ID || replacementVersion.Status != enums.StatusOk {
		return fmt.Errorf("replacement platform workflow %s has no valid stable version", PlatformDeviceAIOnlyWorkflowCode)
	}
	for _, code := range []string{retiredBasicAIWorkflowCode} {
		workflow := repositories.AIWorkflowRepository.GetAnyByScopeCode(db, 0, models.AIWorkflowScopePlatform, code)
		if workflow == nil || workflow.AgentID != 0 {
			continue
		}
		if workflow.Status == enums.StatusDeleted {
			continue
		}
		now := time.Now()
		if err := repositories.AIWorkflowRepository.RebindBuiltInReferences(db, workflow.ID, replacementWorkflow.ID, replacementVersion.ID, now); err != nil {
			return fmt.Errorf("rebind retired platform workflow %s: %w", workflow.Code, err)
		}
		references, err := repositories.AIWorkflowRepository.FindRetirementReferences(db, workflow.ID)
		if err != nil {
			return err
		}
		if details := formatBuiltInWorkflowRetirementReferences(references); len(details) > 0 {
			return fmt.Errorf("cannot retire platform workflow %s (id=%d): live references remain after rebind: %s", workflow.Code, workflow.ID, strings.Join(details, "; "))
		}
		if err := repositories.AIWorkflowRepository.RetireBuiltIn(db, workflow.ID, now); err != nil {
			return err
		}
	}
	return nil
}

func formatBuiltInWorkflowRetirementReferences(references *repositories.AIWorkflowRetirementReferences) []string {
	if references == nil {
		return nil
	}
	details := make([]string, 0, 4)
	if len(references.Agents) > 0 {
		items := make([]string, 0, len(references.Agents))
		for i := range references.Agents {
			item := references.Agents[i]
			items = append(items, fmt.Sprintf("id=%d tenant=%d product=%d", item.ID, item.TenantID, item.ProductID))
		}
		details = append(details, "agent drafts ["+strings.Join(items, ", ")+"]")
	}
	if len(references.Releases) > 0 {
		items := make([]string, 0, len(references.Releases))
		for i := range references.Releases {
			item := references.Releases[i]
			items = append(items, fmt.Sprintf("id=%d tenant=%d product=%d agent=%d", item.ID, item.TenantID, item.ProductID, item.AgentID))
		}
		details = append(details, "active releases ["+strings.Join(items, ", ")+"]")
	}
	if len(references.Profiles) > 0 {
		items := make([]string, 0, len(references.Profiles))
		for i := range references.Profiles {
			item := references.Profiles[i]
			items = append(items, fmt.Sprintf("id=%d tenant=%d product=%d", item.ID, item.TenantID, item.ProductID))
		}
		details = append(details, "product service profiles ["+strings.Join(items, ", ")+"]")
	}
	if len(references.Forks) > 0 {
		items := make([]string, 0, len(references.Forks))
		for i := range references.Forks {
			item := references.Forks[i]
			items = append(items, fmt.Sprintf("id=%d tenant=%d", item.ID, item.TenantID))
		}
		details = append(details, "enterprise workflow forks ["+strings.Join(items, ", ")+"]")
	}
	return details
}

func (s *aiWorkflowService) ensurePlatformBuiltInWorkflowCodeDB(db *gorm.DB, code string) (*models.AIWorkflow, *models.AIWorkflowVersion, error) {
	spec, ok, err := platformBuiltInWorkflowSpec(code)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return nil, nil, errorsx.InvalidParam("platform workflow template is not registered")
	}
	return s.ensurePlatformBuiltInWorkflowDB(db, spec)
}

func platformBuiltInWorkflowSpec(code string) (platformWorkflowSpec, bool, error) {
	specs, err := platformBuiltInWorkflowSpecs()
	if err != nil {
		return platformWorkflowSpec{}, false, err
	}
	for _, spec := range specs {
		if spec.code == code {
			return spec, true, nil
		}
	}
	return platformWorkflowSpec{}, false, nil
}

func platformBuiltInWorkflowSpecs() ([]platformWorkflowSpec, error) {
	manifests, err := loadPlatformBuiltInWorkflowManifests()
	if err != nil {
		return nil, errorsx.InvalidParam("platform workflow manifests are invalid: " + err.Error())
	}
	ret := make([]platformWorkflowSpec, 0, len(manifests))
	for _, item := range manifests {
		if !IsPlatformBuiltInWorkflowCode(item.Code) {
			return nil, errorsx.InvalidParam("unregistered platform workflow manifest: " + item.Code)
		}
		ret = append(ret, platformWorkflowSpec{
			code:          item.Code,
			name:          item.Name,
			description:   item.Description,
			definition:    item.Definition,
			initialChange: item.InitialChange,
			upgradeChange: item.UpgradeChange,
		})
	}
	if len(ret) != len(PlatformBuiltInWorkflowCodes()) {
		return nil, errorsx.InvalidParam("platform workflow manifest registry is incomplete")
	}
	return ret, nil
}

func mustPlatformBuiltInWorkflowDefinition(code string) dsl.Definition {
	spec, ok, err := platformBuiltInWorkflowSpec(code)
	if err != nil {
		panic(err)
	}
	if !ok {
		panic("platform workflow template is not registered: " + code)
	}
	return spec.definition
}

func (s *aiWorkflowService) ensurePlatformBuiltInWorkflowDB(db *gorm.DB, spec platformWorkflowSpec) (*models.AIWorkflow, *models.AIWorkflowVersion, error) {
	if db == nil {
		return nil, nil, errorsx.InvalidParam("database is required")
	}
	definition := spec.definition
	if result := s.ValidateDefinition(definition); !result.Valid {
		return nil, nil, errorsx.InvalidParam("platform built-in workflow definition is invalid")
	}
	definitionJSON, err := marshalDefinition(definition)
	if err != nil {
		return nil, nil, err
	}
	definitionHash := hashDefinition(definitionJSON)
	now := time.Now()
	workflow := repositories.AIWorkflowRepository.GetAnyByScopeCode(
		db,
		0,
		models.AIWorkflowScopePlatform,
		spec.code,
	)
	if workflow == nil {
		workflow = &models.AIWorkflow{
			TenantID:        0,
			Code:            spec.code,
			Scope:           models.AIWorkflowScopePlatform,
			Name:            spec.name,
			Description:     spec.description,
			Status:          enums.StatusOk,
			DraftDefinition: definitionJSON,
			Locked:          true,
			AuditFields: models.AuditFields{
				CreatedAt:      now,
				CreateUserName: "system",
				UpdatedAt:      now,
				UpdateUserName: "system",
			},
		}
		if err := repositories.AIWorkflowRepository.Create(db, workflow); err != nil {
			workflow = repositories.AIWorkflowRepository.GetAnyByScopeCode(db, 0, models.AIWorkflowScopePlatform, spec.code)
			if workflow == nil {
				return nil, nil, err
			}
		}
	}

	var stableVersion *models.AIWorkflowVersion
	if workflow.CurrentStableVersionID > 0 {
		stableVersion = repositories.AIWorkflowVersionRepository.Get(db, workflow.CurrentStableVersionID)
	}
	if stableVersion == nil {
		stableVersion = repositories.AIWorkflowVersionRepository.GetStable(db, workflow.ID)
	}
	if stableVersion == nil || stableVersion.DefinitionHash != definitionHash {
		matchingVersions := repositories.AIWorkflowVersionRepository.Find(db, sqls.NewCnd().
			Eq("workflow_id", workflow.ID).
			Eq("definition_hash", definitionHash).
			Eq("release_channel", models.AIWorkflowReleaseChannelStable).
			Eq("status", enums.StatusOk).
			Desc("version"))
		if len(matchingVersions) > 0 {
			stableVersion = &matchingVersions[0]
		} else {
			changeSummary := spec.initialChange
			if stableVersion != nil {
				changeSummary = strings.TrimSpace(spec.upgradeChange)
				if changeSummary == "" {
					changeSummary = "同步平台内置会话流程与 Agent 运行时变量契约"
				}
			}
			stableVersion = &models.AIWorkflowVersion{
				WorkflowID:          workflow.ID,
				Version:             repositories.AIWorkflowVersionRepository.MaxVersionByWorkflowID(db, workflow.ID) + 1,
				Status:              enums.StatusOk,
				Definition:          definitionJSON,
				DefinitionHash:      definitionHash,
				ModelPolicySnapshot: marshalModelPolicySnapshot(definition.ModelPolicy),
				ReleaseChannel:      models.AIWorkflowReleaseChannelStable,
				SchemaVersion:       definition.SchemaVersion,
				ChangeSummary:       changeSummary,
				PublishedAt:         &now,
				PublishedByName:     "system",
				AuditFields: models.AuditFields{
					CreatedAt:      now,
					CreateUserName: "system",
					UpdatedAt:      now,
					UpdateUserName: "system",
				},
			}
			if err := repositories.AIWorkflowVersionRepository.Create(db, stableVersion); err != nil {
				return nil, nil, err
			}
		}
	}
	if workflow.Status != enums.StatusOk || workflow.CurrentStableVersionID != stableVersion.ID || workflow.PublishedVersionID != stableVersion.ID || workflow.DraftDefinition != definitionJSON || !workflow.Locked || workflow.Name != spec.name || workflow.Description != spec.description {
		if err := repositories.AIWorkflowRepository.Updates(db, workflow.ID, map[string]any{
			"tenant_id":                 0,
			"code":                      spec.code,
			"scope":                     models.AIWorkflowScopePlatform,
			"name":                      spec.name,
			"description":               spec.description,
			"status":                    enums.StatusOk,
			"locked":                    true,
			"draft_definition":          definitionJSON,
			"current_stable_version_id": stableVersion.ID,
			"published_version_id":      stableVersion.ID,
			"updated_at":                now,
			"update_user_name":          "system",
		}); err != nil {
			return nil, nil, err
		}
		workflow = repositories.AIWorkflowRepository.Get(db, workflow.ID)
	}
	return workflow, stableVersion, nil
}

func (s *aiWorkflowService) BindProductAgentWorkflowDB(db *gorm.DB, agent *models.AIAgent, profile *models.ProductServiceProfile) error {
	if db == nil || agent == nil || profile == nil || agent.TenantID <= 0 || agent.ProductID <= 0 {
		return errorsx.InvalidParam("product agent workflow context is invalid")
	}
	if profile.TenantID != agent.TenantID || profile.ProductID != agent.ProductID {
		return errorsx.Forbidden("product service profile does not belong to the AI Agent")
	}
	if agent.WorkflowVersionID > 0 {
		version := repositories.AIWorkflowVersionRepository.Get(db, agent.WorkflowVersionID)
		if version == nil || version.Status != enums.StatusOk {
			return errorsx.InvalidParam("bound workflow version does not exist")
		}
		workflow := repositories.AIWorkflowRepository.Get(db, version.WorkflowID)
		if workflow == nil || workflow.Status == enums.StatusDeleted {
			return errorsx.InvalidParam("bound workflow does not exist")
		}
		versionChanged := workflow.CurrentStableVersionID != version.ID
		if versionChanged {
			version = repositories.AIWorkflowVersionRepository.Get(db, workflow.CurrentStableVersionID)
			if version == nil || version.Status != enums.StatusOk || version.ReleaseChannel != models.AIWorkflowReleaseChannelStable {
				return errorsx.InvalidParam("bound workflow has no current stable version")
			}
			agent.WorkflowVersionID = version.ID
		}
		restrictions, err := workflowAgentRestrictionUpdates(agent, version)
		if err != nil {
			return err
		}
		if agent.WorkflowID == workflow.ID && !versionChanged && len(restrictions) == 0 {
			return nil
		}
		agent.WorkflowID = workflow.ID
		agent.WorkflowVersionID = version.ID
		applyWorkflowAgentRestrictions(agent, restrictions)
		restrictions["workflow_id"] = workflow.ID
		restrictions["workflow_version_id"] = version.ID
		return repositories.AIAgentRepository.Updates(db, agent.ID, restrictions)
	}

	workflow, stableVersion, err := s.EnsurePlatformDefaultWorkflowDB(db)
	if err != nil {
		return err
	}
	if profile.DefaultFlowTemplateID > 0 {
		workflow = repositories.AIWorkflowRepository.Get(db, profile.DefaultFlowTemplateID)
		if workflow == nil || workflow.Status == enums.StatusDeleted {
			return errorsx.InvalidParam("product default workflow template does not exist")
		}
		if workflow.Scope == models.AIWorkflowScopeTenant && workflow.TenantID != agent.TenantID {
			return errorsx.Forbidden("product default workflow template belongs to another tenant")
		}
		if workflow.Scope != models.AIWorkflowScopePlatform && workflow.Scope != models.AIWorkflowScopeTenant {
			return errorsx.InvalidParam("product default workflow template scope is invalid")
		}
		stableVersion = repositories.AIWorkflowVersionRepository.Get(db, workflow.CurrentStableVersionID)
		if stableVersion == nil {
			stableVersion = repositories.AIWorkflowVersionRepository.GetStable(db, workflow.ID)
		}
	}
	if stableVersion == nil || stableVersion.Status != enums.StatusOk || stableVersion.ReleaseChannel != models.AIWorkflowReleaseChannelStable {
		return errorsx.InvalidParam("product default workflow has no bindable stable version")
	}
	agent.WorkflowID = workflow.ID
	agent.WorkflowVersionID = stableVersion.ID
	restrictions, err := workflowAgentRestrictionUpdates(agent, stableVersion)
	if err != nil {
		return err
	}
	applyWorkflowAgentRestrictions(agent, restrictions)
	if agent.DraftRevision <= 0 {
		agent.DraftRevision = 1
	}
	updates := map[string]any{
		"workflow_id":         workflow.ID,
		"workflow_version_id": stableVersion.ID,
		"draft_revision":      agent.DraftRevision,
	}
	for key, value := range restrictions {
		updates[key] = value
	}
	return repositories.AIAgentRepository.Updates(db, agent.ID, updates)
}

func (s *aiWorkflowService) ValidateDefinition(def dsl.Definition) workflowvalidator.Result {
	return workflowvalidator.ValidateDefinition(def, s.registry)
}

func (s *aiWorkflowService) CreateWorkflow(req request.CreateAIWorkflowRequest, operator *dto.AuthPrincipal) (*models.AIWorkflow, error) {
	return s.SaveAgentWorkflow(req, operator)
}

// CreateReusableTemplateFromVersion creates a tenant-owned editable draft from
// an immutable version. Product and Agent capability identifiers are never
// copied into the template by this operation.
func (s *aiWorkflowService) CreateReusableTemplateFromVersion(req request.CreateAIWorkflowTemplateRequest, operator *dto.AuthPrincipal) (*models.AIWorkflow, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if operator.TenantID <= 0 {
		return nil, errorsx.InvalidParam("enterprise tenant is required")
	}
	if !TenantCapabilityService.AIEnabled(operator.TenantID) {
		return nil, errorsx.Forbidden("tenant AI capability is disabled")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errorsx.InvalidParam("workflow template name is required")
	}
	sourceVersion := s.GetVersionForOperator(req.SourceVersionID, operator)
	if sourceVersion == nil || sourceVersion.Status != enums.StatusOk {
		return nil, errorsx.InvalidParam("source workflow version does not exist")
	}
	sourceWorkflow := s.GetForOperator(sourceVersion.WorkflowID, operator)
	if !s.IsReusableTemplateForOperator(sourceWorkflow, operator) {
		return nil, errorsx.InvalidParam("source workflow is not a reusable template")
	}
	if !TenantCapabilityService.CanUseWorkflowTemplate(operator.TenantID, sourceWorkflow) {
		return nil, errorsx.Forbidden("tenant AI capability is disabled")
	}
	var definition dsl.Definition
	if err := json.Unmarshal([]byte(sourceVersion.Definition), &definition); err != nil {
		return nil, errorsx.InvalidParam("source workflow definition is invalid")
	}
	if result := s.ValidateDefinition(definition); !result.Valid {
		return nil, errorsx.InvalidParam("source workflow definition is invalid")
	}
	now := time.Now()
	item := &models.AIWorkflow{
		TenantID:         operator.TenantID,
		Code:             fmt.Sprintf("tenant_%d_%d", operator.TenantID, now.UnixNano()),
		Scope:            models.AIWorkflowScopeTenant,
		Name:             name,
		Description:      strings.TrimSpace(req.Description),
		AgentID:          0,
		SourceWorkflowID: sourceWorkflow.ID,
		SourceVersionID:  sourceVersion.ID,
		Status:           enums.StatusOk,
		DraftDefinition:  sourceVersion.Definition,
		Locked:           false,
		AuditFields:      utils.BuildAuditFields(operator),
	}
	if err := repositories.AIWorkflowRepository.Create(sqls.DB(), item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *aiWorkflowService) UpdateReusableTemplateDraft(id int64, req request.UpdateAIWorkflowTemplateDraftRequest, operator *dto.AuthPrincipal) (*models.AIWorkflow, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	current := s.GetForOperator(id, operator)
	if current == nil || current.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParamI18n("error.e0002")
	}
	if current.Scope != models.AIWorkflowScopeTenant || current.TenantID != operator.TenantID || current.AgentID != 0 || current.Locked {
		return nil, errorsx.Forbidden("only reusable tenant workflow templates can be edited")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errorsx.InvalidParam("workflow template name is required")
	}
	if result := s.ValidateDefinition(req.Definition); !result.Valid {
		return nil, errorsx.InvalidParam("workflow definition is invalid")
	}
	definition, err := marshalDefinition(req.Definition)
	if err != nil {
		return nil, err
	}
	if err := repositories.AIWorkflowRepository.Updates(sqls.DB(), current.ID, map[string]any{
		"name":             name,
		"description":      strings.TrimSpace(req.Description),
		"draft_definition": definition,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	}); err != nil {
		return nil, err
	}
	return s.Get(current.ID), nil
}

func (s *aiWorkflowService) BindAgentWorkflowVersion(agentID, versionID int64, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	agent := AIAgentService.GetForOperator(agentID, operator)
	if agent == nil || agent.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("AI Agent does not exist")
	}
	if agent.Source == TenantDefaultAIAgentSource {
		return errorsx.Forbidden("tenant default AI agent workflow is managed by the platform")
	}
	version := s.GetVersionForOperator(versionID, operator)
	if version == nil || version.Status != enums.StatusOk || version.ReleaseChannel != models.AIWorkflowReleaseChannelStable {
		return errorsx.InvalidParam("only an enabled stable workflow version can be bound")
	}
	workflow := s.GetForOperator(version.WorkflowID, operator)
	if !s.IsReusableTemplateForOperator(workflow, operator) {
		return errorsx.InvalidParam("workflow version is not a reusable template version")
	}
	if workflow.Scope == models.AIWorkflowScopeTenant && workflow.TenantID != agent.TenantID {
		return errorsx.Forbidden("workflow template belongs to another tenant")
	}
	if workflow.Scope != models.AIWorkflowScopePlatform && workflow.Scope != models.AIWorkflowScopeTenant {
		return errorsx.InvalidParam("workflow template scope is invalid")
	}
	if workflow.CurrentStableVersionID != version.ID {
		return errorsx.InvalidParam("only the current stable workflow version can be bound")
	}
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		current := repositories.AIAgentRepository.GetForUpdate(ctx.Tx, agent.ID)
		if current == nil || current.TenantID != agent.TenantID || current.Status == enums.StatusDeleted {
			return errorsx.InvalidParam("AI Agent does not exist")
		}
		lockedWorkflow := repositories.AIWorkflowRepository.GetForUpdate(ctx.Tx, workflow.ID)
		if !s.IsReusableTemplateForOperator(lockedWorkflow, operator) {
			return errorsx.InvalidParam("workflow version is not a reusable template version")
		}
		if lockedWorkflow.Scope == models.AIWorkflowScopeTenant && lockedWorkflow.TenantID != current.TenantID {
			return errorsx.Forbidden("workflow template belongs to another tenant")
		}
		lockedVersion := repositories.AIWorkflowVersionRepository.Get(ctx.Tx, version.ID)
		if lockedVersion == nil || lockedVersion.Status != enums.StatusOk || lockedVersion.WorkflowID != lockedWorkflow.ID || lockedVersion.ReleaseChannel != models.AIWorkflowReleaseChannelStable || lockedWorkflow.CurrentStableVersionID != lockedVersion.ID {
			return errorsx.InvalidParam("only an enabled stable workflow version can be bound")
		}
		restrictions, err := workflowAgentRestrictionUpdates(current, lockedVersion)
		if err != nil {
			return err
		}
		if current.WorkflowID == lockedWorkflow.ID && current.WorkflowVersionID == lockedVersion.ID && len(restrictions) == 0 {
			return nil
		}
		updates := map[string]any{
			"workflow_id":         lockedWorkflow.ID,
			"workflow_version_id": lockedVersion.ID,
			"draft_revision":      current.DraftRevision + 1,
			"review_status":       enums.AIAgentReviewStatusUnreviewed,
			"review_comment":      "",
			"reviewed_at":         nil,
			"reviewed_by_id":      0,
			"reviewed_by_name":    "",
			"update_user_id":      operator.UserID,
			"update_user_name":    operator.Username,
			"updated_at":          time.Now(),
		}
		for key, value := range restrictions {
			updates[key] = value
		}
		return repositories.AIAgentRepository.Updates(ctx.Tx, current.ID, updates)
	})
}

func (s *aiWorkflowService) SaveAgentWorkflow(req request.SaveAIWorkflowRequest, operator *dto.AuthPrincipal) (*models.AIWorkflow, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	agent := AIAgentService.GetForOperator(req.AgentID, operator)
	if agent == nil || agent.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParamI18n("error.e0002")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = defaultAgentWorkflowName(agent.Name)
	}
	definition, err := marshalDefinition(req.Definition)
	if err != nil {
		return nil, err
	}
	current := s.GetByAgentID(req.AgentID)
	if current == nil || current.Locked || current.AgentID == 0 {
		sourceWorkflowID := int64(0)
		sourceVersionID := int64(0)
		if current != nil {
			sourceWorkflowID = current.ID
			sourceVersionID = agent.WorkflowVersionID
		}
		item := &models.AIWorkflow{
			TenantID:         agent.TenantID,
			Code:             "agent_" + strconv.FormatInt(agent.ID, 10) + "_custom",
			Scope:            models.AIWorkflowScopeTenant,
			Name:             name,
			Description:      strings.TrimSpace(req.Description),
			AgentID:          req.AgentID,
			SourceWorkflowID: sourceWorkflowID,
			SourceVersionID:  sourceVersionID,
			Status:           enums.StatusOk,
			DraftDefinition:  definition,
			AuditFields:      utils.BuildAuditFields(operator),
		}
		if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			if err := repositories.AIWorkflowRepository.Create(ctx.Tx, item); err != nil {
				return err
			}
			return repositories.AIAgentRepository.Updates(ctx.Tx, agent.ID, map[string]any{
				"workflow_id":      item.ID,
				"draft_revision":   agent.DraftRevision + 1,
				"update_user_id":   operator.UserID,
				"update_user_name": operator.Username,
				"updated_at":       time.Now(),
			})
		}); err != nil {
			return nil, err
		}
		return item, nil
	}
	if current.TenantID != agent.TenantID || (current.AgentID > 0 && current.AgentID != agent.ID) {
		return nil, errorsx.Forbidden("workflow does not belong to the AI Agent tenant")
	}
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := repositories.AIWorkflowRepository.Updates(ctx.Tx, current.ID, map[string]interface{}{
			"name":             name,
			"description":      strings.TrimSpace(req.Description),
			"agent_id":         req.AgentID,
			"draft_definition": definition,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
			"updated_at":       time.Now(),
		}); err != nil {
			return err
		}
		return repositories.AIAgentRepository.Updates(ctx.Tx, agent.ID, map[string]any{
			"draft_revision":   agent.DraftRevision + 1,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
			"updated_at":       time.Now(),
		})
	}); err != nil {
		return nil, err
	}
	return s.Get(current.ID), nil
}

func (s *aiWorkflowService) UpdateWorkflow(req request.UpdateAIWorkflowRequest, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	current := s.GetForOperator(req.ID, operator)
	if current == nil {
		return errorsx.InvalidParamI18n("error.e0002")
	}
	if current.Locked && (!operator.IsPlatform() || operator.TenantID > 0) {
		return errorsx.Forbidden("platform workflows are read-only for enterprise operators")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return errorsx.InvalidParam("workflow name is required")
	}
	if req.AgentID <= 0 {
		return errorsx.InvalidParam("agent id is required")
	}
	definition, err := marshalDefinition(req.Definition)
	if err != nil {
		return err
	}
	return repositories.AIWorkflowRepository.Updates(sqls.DB(), req.ID, map[string]interface{}{
		"name":             name,
		"description":      strings.TrimSpace(req.Description),
		"agent_id":         req.AgentID,
		"draft_definition": definition,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	})
}

func (s *aiWorkflowService) DeleteWorkflow(id int64, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	current := s.GetForOperator(id, operator)
	if current == nil {
		return errorsx.InvalidParamI18n("error.e0002")
	}
	if current.Locked || current.Scope == models.AIWorkflowScopePlatform {
		return errorsx.Forbidden("platform workflows cannot be deleted through the enterprise API")
	}
	return sqls.DB().Transaction(func(tx *gorm.DB) error {
		locked := repositories.AIWorkflowRepository.GetForUpdate(tx, id)
		if locked == nil || locked.Status == enums.StatusDeleted {
			return errorsx.InvalidParamI18n("error.e0002")
		}
		if locked.Scope != models.AIWorkflowScopeTenant || locked.TenantID != operator.TenantID || locked.Locked {
			return errorsx.Forbidden("only editable enterprise workflow templates can be retired")
		}
		if repositories.AIAgentRepository.Take(tx, "workflow_id = ? AND status <> ?", id, enums.StatusDeleted) != nil {
			return errorsx.Forbidden("workflow is still bound to an AI Agent draft")
		}
		if repositories.AIAgentReleaseRepository.ExistsActiveByWorkflowID(tx, id) {
			return errorsx.Forbidden("workflow is still referenced by an active AI Agent release")
		}
		if repositories.ProductServiceProfileRepository.ExistsByDefaultFlowTemplateID(tx, id) {
			return errorsx.Forbidden("workflow is still selected as a product default workflow")
		}
		now := time.Now()
		return repositories.AIWorkflowRepository.Updates(tx, id, map[string]interface{}{
			"status":           enums.StatusDeleted,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
			"updated_at":       now,
		})
	})
}

func (s *aiWorkflowService) PublishWorkflow(req request.PublishAIWorkflowRequest, operator *dto.AuthPrincipal) (*models.AIWorkflowVersion, error) {
	if req.AgentID > 0 {
		return s.PublishAgentWorkflow(req, operator)
	}
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	workflow := s.Get(req.WorkflowID)
	if workflow == nil || workflow.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParamI18n("error.e0002")
	}
	if IsPlatformBuiltInWorkflow(workflow) {
		return nil, errorsx.Forbidden("platform built-in workflows are managed by embedded manifests")
	}
	if workflow.Scope == models.AIWorkflowScopePlatform {
		if !operator.IsPlatform() || operator.TenantID > 0 {
			return nil, errorsx.Forbidden("only platform operators can publish platform workflows")
		}
	} else if workflow.TenantID <= 0 || workflow.TenantID != operator.TenantID {
		return nil, errorsx.Forbidden("workflow belongs to another tenant")
	}
	result := s.ValidateDefinition(req.Definition)
	if !result.Valid {
		return nil, errorsx.InvalidParam("workflow definition is invalid")
	}
	definition, err := marshalDefinition(req.Definition)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	definitionHash := hashDefinition(definition)
	var version *models.AIWorkflowVersion
	err = sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		locked := repositories.AIWorkflowRepository.GetForUpdate(ctx.Tx, req.WorkflowID)
		if locked == nil || locked.Status == enums.StatusDeleted {
			return errorsx.InvalidParamI18n("error.e0002")
		}
		if locked.Scope == models.AIWorkflowScopePlatform {
			if !operator.IsPlatform() || operator.TenantID > 0 {
				return errorsx.Forbidden("only platform operators can publish platform workflows")
			}
		} else if locked.TenantID <= 0 || locked.TenantID != operator.TenantID {
			return errorsx.Forbidden("workflow belongs to another tenant")
		}
		if locked.CurrentStableVersionID > 0 {
			stable := repositories.AIWorkflowVersionRepository.Get(ctx.Tx, locked.CurrentStableVersionID)
			if stable != nil && stable.Status == enums.StatusOk && stable.DefinitionHash == definitionHash {
				version = stable
				return repositories.AIWorkflowRepository.Updates(ctx.Tx, req.WorkflowID, map[string]interface{}{
					"draft_definition": definition,
					"update_user_id":   operator.UserID,
					"update_user_name": operator.Username,
					"updated_at":       now,
				})
			}
		}
		nextVersion := repositories.AIWorkflowVersionRepository.MaxVersionByWorkflowID(ctx.Tx, req.WorkflowID) + 1
		version = &models.AIWorkflowVersion{
			WorkflowID:          req.WorkflowID,
			Version:             nextVersion,
			Status:              enums.StatusOk,
			Definition:          definition,
			DefinitionHash:      definitionHash,
			ModelPolicySnapshot: marshalModelPolicySnapshot(req.Definition.ModelPolicy),
			ReleaseChannel:      models.AIWorkflowReleaseChannelStable,
			SchemaVersion:       req.Definition.SchemaVersion,
			PublishedAt:         &now,
			PublishedByID:       operator.UserID,
			PublishedByName:     operator.Username,
			AuditFields:         utils.BuildAuditFields(operator),
		}
		if err := repositories.AIWorkflowVersionRepository.Create(ctx.Tx, version); err != nil {
			return err
		}
		return repositories.AIWorkflowRepository.Updates(ctx.Tx, req.WorkflowID, map[string]interface{}{
			"draft_definition":          definition,
			"published_version_id":      version.ID,
			"current_stable_version_id": version.ID,
			"update_user_id":            operator.UserID,
			"update_user_name":          operator.Username,
			"updated_at":                now,
		})
	})
	if err != nil {
		return nil, err
	}
	return version, nil
}

// RollbackWorkflowVersion republishes an immutable historical version as a new
// stable version. Existing versions and product bindings are never mutated.
func (s *aiWorkflowService) RollbackWorkflowVersion(workflowID, targetVersionID int64, operator *dto.AuthPrincipal) (*models.AIWorkflowVersion, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	workflow := s.GetForOperator(workflowID, operator)
	if workflow == nil || workflow.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParamI18n("error.e0002")
	}
	if workflow.Scope != models.AIWorkflowScopeTenant || workflow.TenantID != operator.TenantID || workflow.AgentID != 0 || workflow.Locked {
		return nil, errorsx.Forbidden("only editable enterprise workflow templates can be rolled back")
	}
	target := s.GetVersionForOperator(targetVersionID, operator)
	if target == nil || target.WorkflowID != workflow.ID || target.Status != enums.StatusOk || target.ReleaseChannel != models.AIWorkflowReleaseChannelStable {
		return nil, errorsx.InvalidParam("rollback target is not a stable version of this workflow")
	}
	if target.ID == workflow.CurrentStableVersionID {
		return nil, errorsx.InvalidParam("rollback target is already the current stable version")
	}
	var definition dsl.Definition
	if err := json.Unmarshal([]byte(target.Definition), &definition); err != nil {
		return nil, errorsx.InvalidParam("rollback target definition is invalid")
	}
	if result := s.ValidateDefinition(definition); !result.Valid {
		return nil, errorsx.InvalidParam("rollback target definition is invalid")
	}

	now := time.Now()
	var version *models.AIWorkflowVersion
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		nextVersion := repositories.AIWorkflowVersionRepository.MaxVersionByWorkflowID(ctx.Tx, workflow.ID) + 1
		version = &models.AIWorkflowVersion{
			WorkflowID:          workflow.ID,
			Version:             nextVersion,
			Status:              enums.StatusOk,
			Definition:          target.Definition,
			DefinitionHash:      target.DefinitionHash,
			ModelPolicySnapshot: marshalModelPolicySnapshot(definition.ModelPolicy),
			ReleaseChannel:      models.AIWorkflowReleaseChannelStable,
			SchemaVersion:       definition.SchemaVersion,
			ChangeSummary:       fmt.Sprintf("回滚至 V%d", target.Version),
			SourceVersionID:     target.ID,
			PublishedAt:         &now,
			PublishedByID:       operator.UserID,
			PublishedByName:     operator.Username,
			AuditFields:         utils.BuildAuditFields(operator),
		}
		if err := repositories.AIWorkflowVersionRepository.Create(ctx.Tx, version); err != nil {
			return err
		}
		return repositories.AIWorkflowRepository.Updates(ctx.Tx, workflow.ID, map[string]interface{}{
			"draft_definition":          target.Definition,
			"published_version_id":      version.ID,
			"current_stable_version_id": version.ID,
			"update_user_id":            operator.UserID,
			"update_user_name":          operator.Username,
			"updated_at":                now,
		})
	})
	if err != nil {
		return nil, err
	}
	return version, nil
}

func (s *aiWorkflowService) PublishAgentWorkflow(req request.PublishAIWorkflowRequest, operator *dto.AuthPrincipal) (*models.AIWorkflowVersion, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	agent := AIAgentService.GetForOperator(req.AgentID, operator)
	if agent == nil || agent.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParamI18n("error.e0002")
	}
	workflow, err := s.GetOrCreateAgentWorkflow(req.AgentID, operator)
	if err != nil {
		return nil, err
	}
	if workflow.Locked || workflow.AgentID == 0 {
		definitionJSON, marshalErr := marshalDefinition(req.Definition)
		if marshalErr != nil {
			return nil, marshalErr
		}
		boundVersion := repositories.AIWorkflowVersionRepository.Get(sqls.DB(), agent.WorkflowVersionID)
		if boundVersion != nil && boundVersion.WorkflowID == workflow.ID && boundVersion.DefinitionHash == hashDefinition(definitionJSON) {
			return boundVersion, nil
		}
		workflow, err = s.SaveAgentWorkflow(request.SaveAIWorkflowRequest{
			Name:        defaultAgentWorkflowName(agent.Name),
			Description: workflow.Description,
			AgentID:     agent.ID,
			Definition:  req.Definition,
		}, operator)
		if err != nil {
			return nil, err
		}
		agent = AIAgentService.GetForOperator(req.AgentID, operator)
	}
	req.WorkflowID = workflow.ID
	result := s.ValidateDefinition(req.Definition)
	if !result.Valid {
		return nil, errorsx.InvalidParam("workflow definition is invalid")
	}
	definition, err := marshalDefinition(req.Definition)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	var version *models.AIWorkflowVersion
	err = sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		current := repositories.AIWorkflowRepository.Get(ctx.Tx, workflow.ID)
		if current == nil || current.AgentID != req.AgentID || current.TenantID != agent.TenantID || current.Status == enums.StatusDeleted {
			return errorsx.InvalidParamI18n("error.e0002")
		}
		nextVersion := repositories.AIWorkflowVersionRepository.MaxVersionByWorkflowID(ctx.Tx, current.ID) + 1
		version = &models.AIWorkflowVersion{
			WorkflowID:          current.ID,
			Version:             nextVersion,
			Status:              enums.StatusOk,
			Definition:          definition,
			DefinitionHash:      hashDefinition(definition),
			ModelPolicySnapshot: marshalModelPolicySnapshot(req.Definition.ModelPolicy),
			ReleaseChannel:      models.AIWorkflowReleaseChannelStable,
			SchemaVersion:       req.Definition.SchemaVersion,
			PublishedAt:         &now,
			PublishedByID:       operator.UserID,
			PublishedByName:     operator.Username,
			AuditFields:         utils.BuildAuditFields(operator),
		}
		if err := repositories.AIWorkflowVersionRepository.Create(ctx.Tx, version); err != nil {
			return err
		}
		if err := repositories.AIWorkflowRepository.Updates(ctx.Tx, current.ID, map[string]interface{}{
			"draft_definition":          definition,
			"published_version_id":      version.ID,
			"current_stable_version_id": version.ID,
			"update_user_id":            operator.UserID,
			"update_user_name":          operator.Username,
			"updated_at":                now,
		}); err != nil {
			return err
		}
		agentUpdates := map[string]any{
			"workflow_id":         current.ID,
			"workflow_version_id": version.ID,
			"draft_revision":      agent.DraftRevision + 1,
			"update_user_id":      operator.UserID,
			"update_user_name":    operator.Username,
			"updated_at":          now,
		}
		if agent.ProductID > 0 && agent.ActiveReleaseID <= 0 {
			agentUpdates["status"] = enums.StatusDisabled
			agentUpdates["review_status"] = enums.AIAgentReviewStatusUnreviewed
			agentUpdates["review_comment"] = ""
			agentUpdates["reviewed_at"] = nil
			agentUpdates["reviewed_by_id"] = 0
			agentUpdates["reviewed_by_name"] = ""
		}
		return repositories.AIAgentRepository.Updates(ctx.Tx, req.AgentID, agentUpdates)
	})
	if err != nil {
		return nil, err
	}
	return version, nil
}

func defaultAgentWorkflowName(agentName string) string {
	agentName = strings.TrimSpace(agentName)
	if agentName == "" {
		return "会话流程"
	}
	return agentName + " 会话流程"
}

func marshalDefinition(def dsl.Definition) (string, error) {
	buf, err := json.Marshal(def)
	if err != nil {
		return "", errorsx.InvalidParam("invalid workflow definition")
	}
	return string(buf), nil
}

func marshalModelPolicySnapshot(policy *dsl.ModelPolicy) string {
	if policy == nil {
		return "{}"
	}
	buf, err := json.Marshal(policy)
	if err != nil {
		return "{}"
	}
	return string(buf)
}

func hashDefinition(definition string) string {
	sum := sha256.Sum256([]byte(definition))
	return hex.EncodeToString(sum[:])
}
