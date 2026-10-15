package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"

	"github.com/mlogclub/simple/sqls"
)

type TicketQualityScorecardItem struct {
	Code  string `json:"code"`
	Title string `json:"title"`
	Max   int    `json:"max"`
}

type TicketQualityScorecard struct {
	ID          int64                        `json:"id"`
	TenantID    int64                        `json:"tenant_id"`
	Version     string                       `json:"version"`
	Name        string                       `json:"name"`
	Status      string                       `json:"status"`
	Items       []TicketQualityScorecardItem `json:"items"`
	PublishedAt *time.Time                   `json:"published_at,omitempty"`
}

type TicketQualityReviewMetadata struct {
	SampleID       int64    `json:"sample_id"`
	DefectCodes    []string `json:"defect_codes"`
	Evidence       string   `json:"evidence"`
	DisputeStatus  string   `json:"dispute_status"`
	DisputeNote    string   `json:"dispute_note"`
	Outcome        string   `json:"outcome"`
	CoachingAction string   `json:"coaching_action"`
}

type TicketQualityAutoAssessmentItem struct {
	Code           string `json:"code"`
	Title          string `json:"title"`
	Score          int    `json:"score"`
	Max            int    `json:"max"`
	Confidence     string `json:"confidence"`
	EvidenceKey    string `json:"evidence_key"`
	RequiresReview bool   `json:"requires_review"`
}

type TicketQualityAutoAssessment struct {
	Source              string                            `json:"source"`
	Version             string                            `json:"version"`
	ScorecardVersion    string                            `json:"scorecard_version"`
	Answers             map[string]int                    `json:"answers"`
	Items               []TicketQualityAutoAssessmentItem `json:"items"`
	DefectCodes         []string                          `json:"defect_codes"`
	TotalScore          int                               `json:"total_score"`
	MaxScore            int                               `json:"max_score"`
	Result              string                            `json:"result"`
	ReviewRequiredCount int                               `json:"review_required_count"`
}

const ticketQualityNoCoaching = "无需辅导"

func ticketQualityResult(total, max int) string {
	passScore := (max*5 + 5) / 6
	improvementScore := (max*5 + 8) / 9
	if total >= passScore {
		return "pass"
	}
	if total >= improvementScore {
		return "needs_improvement"
	}
	return "fail"
}

var defaultTicketQualityScorecardItems = []TicketQualityScorecardItem{
	{Code: "identity_confirmation", Title: "客户身份确认", Max: 2},
	{Code: "information_completeness", Title: "资料完整性", Max: 2},
	{Code: "issue_classification", Title: "问题分类", Max: 2},
	{Code: "priority", Title: "优先级判断", Max: 2},
	{Code: "communication", Title: "沟通表达", Max: 2},
	{Code: "data_protection", Title: "资料保护", Max: 2},
	{Code: "handoff", Title: "转派处理", Max: 2},
	{Code: "follow_up", Title: "跟进情况", Max: 2},
	{Code: "closure", Title: "关闭情况", Max: 2},
}

func DefaultTicketQualityScorecardItems() []TicketQualityScorecardItem {
	items := make([]TicketQualityScorecardItem, len(defaultTicketQualityScorecardItems))
	copy(items, defaultTicketQualityScorecardItems)
	return items
}

func GetActiveTicketQualityScorecard(tenantID int64) (*TicketQualityScorecard, error) {
	if tenantID <= 0 {
		return nil, errors.New("tenant is required")
	}
	var row models.TicketQualityScorecardVersion
	err := sqls.DB().Where("tenant_id = ? AND status = ?", tenantID, "active").Order("id DESC").First(&row).Error
	if err != nil {
		items := DefaultTicketQualityScorecardItems()
		created, createErr := createScorecardVersion(tenantID, "v1", "基础质量评估表", items, 0, "active")
		if createErr != nil {
			return nil, createErr
		}
		row = *created
	}
	return buildTicketQualityScorecard(&row)
}

func BuildTicketQualityAutoAssessment(ticketID int64, operator *dto.AuthPrincipal) (*TicketQualityAutoAssessment, error) {
	if operator == nil || operator.TenantID <= 0 || !operator.HasPermission("ticket.view") {
		return nil, errorsx.Forbidden("无质量评估查看权限")
	}
	var ticket models.Ticket
	if err := sqls.DB().Where("id = ? AND tenant_id = ?", ticketID, operator.TenantID).First(&ticket).Error; err != nil {
		return nil, err
	}
	scorecard, err := GetActiveTicketQualityScorecard(operator.TenantID)
	if err != nil {
		return nil, err
	}

	var progresses []models.TicketProgress
	if err := sqls.DB().Where("tenant_id = ? AND ticket_id = ?", operator.TenantID, ticket.ID).Find(&progresses).Error; err != nil {
		return nil, err
	}
	var repairs []models.TicketRepairRecord
	if err := sqls.DB().Where("tenant_id = ? AND ticket_id = ?", operator.TenantID, ticket.ID).Find(&repairs).Error; err != nil {
		return nil, err
	}

	visibleProgressCount := 0
	for i := range progresses {
		if progresses[i].VisibleToCustomer {
			visibleProgressCount++
		}
	}
	hasRepairConclusion := false
	for i := range repairs {
		if strings.TrimSpace(repairs[i].Conclusion) != "" || strings.TrimSpace(repairs[i].Solution) != "" || strings.TrimSpace(repairs[i].RepairMethod) != "" {
			hasRepairConclusion = true
			break
		}
	}
	missingContextCount := 0
	var missingContext []string
	if json.Unmarshal([]byte(ticket.MissingContextJSON), &missingContext) == nil {
		missingContextCount = len(missingContext)
	}

	assessment := &TicketQualityAutoAssessment{
		Source:           "rules",
		Version:          "rules-v1",
		ScorecardVersion: scorecard.Version,
		Answers:          make(map[string]int, len(scorecard.Items)),
		Items:            make([]TicketQualityAutoAssessmentItem, 0, len(scorecard.Items)),
		DefectCodes:      []string{},
	}
	itemByCode := make(map[string]TicketQualityScorecardItem, len(scorecard.Items))
	for _, item := range scorecard.Items {
		itemByCode[item.Code] = item
		assessment.MaxScore += item.Max
	}
	add := func(code string, score int, confidence, evidenceKey string, requiresReview bool) {
		item, ok := itemByCode[code]
		if !ok {
			return
		}
		if score < 0 {
			score = 0
		}
		if score > item.Max {
			score = item.Max
		}
		assessment.Answers[code] = score
		assessment.TotalScore += score
		assessment.Items = append(assessment.Items, TicketQualityAutoAssessmentItem{Code: code, Title: item.Title, Score: score, Max: item.Max, Confidence: confidence, EvidenceKey: evidenceKey, RequiresReview: requiresReview})
		if requiresReview {
			assessment.ReviewRequiredCount++
		}
		if score == 0 {
			assessment.DefectCodes = append(assessment.DefectCodes, code)
		}
	}

	identityFacts := 0
	if ticket.CustomerID > 0 {
		identityFacts = 2
	} else {
		if strings.TrimSpace(ticket.CallerName) != "" {
			identityFacts++
		}
		if strings.TrimSpace(ticket.CallerPhone) != "" {
			identityFacts++
		}
	}
	if identityFacts >= 2 {
		add("identity_confirmation", 2, "high", "identity_complete", false)
	} else if identityFacts == 1 {
		add("identity_confirmation", 1, "medium", "identity_partial", true)
	} else {
		add("identity_confirmation", 0, "high", "identity_missing", false)
	}

	contextStatus := strings.ToLower(strings.TrimSpace(ticket.ContextStatus))
	if missingContextCount > 0 {
		add("information_completeness", 0, "high", "information_missing", false)
	} else if contextStatus == "complete" || contextStatus == "completed" || contextStatus == "ready" || contextStatus == "sufficient" {
		add("information_completeness", 2, "high", "information_complete", false)
	} else {
		add("information_completeness", 1, "low", "information_unverified", true)
	}

	if strings.TrimSpace(ticket.CaseType) != "" {
		add("issue_classification", 2, "high", "classification_complete", false)
	} else if strings.TrimSpace(ticket.TicketType) != "" {
		add("issue_classification", 1, "medium", "classification_partial", true)
	} else {
		add("issue_classification", 0, "high", "classification_missing", false)
	}

	priorityValid := ticket.PriorityLevel == "p1" || ticket.PriorityLevel == "p2" || ticket.PriorityLevel == "p3" || ticket.PriorityLevel == "p4"
	if priorityValid && !ticket.PriorityReviewRequired {
		add("priority", 2, "high", "priority_complete", false)
	} else if priorityValid || strings.TrimSpace(ticket.PriorityCode) != "" {
		add("priority", 1, "medium", "priority_review", true)
	} else {
		add("priority", 0, "high", "priority_missing", false)
	}

	if ticket.FirstRespondedAt != nil && visibleProgressCount > 0 {
		add("communication", 2, "medium", "communication_recorded", false)
	} else if ticket.FirstRespondedAt != nil || visibleProgressCount > 0 {
		add("communication", 1, "medium", "communication_partial", true)
	} else {
		add("communication", 0, "medium", "communication_missing", true)
	}

	add("data_protection", 1, "low", "data_protection_review", true)

	if ticket.CurrentTeamID > 0 && ticket.CurrentAssigneeID > 0 && ticket.DispatchAttempts <= 1 {
		add("handoff", 2, "high", "handoff_complete", false)
	} else if ticket.CurrentTeamID > 0 || ticket.CurrentAssigneeID > 0 {
		add("handoff", 1, "medium", "handoff_partial", true)
	} else {
		add("handoff", 0, "high", "handoff_missing", false)
	}

	if ticket.LastCustomerUpdateAt != nil && visibleProgressCount >= 2 {
		add("follow_up", 2, "high", "follow_up_complete", false)
	} else if ticket.LastCustomerUpdateAt != nil || visibleProgressCount > 0 {
		add("follow_up", 1, "medium", "follow_up_partial", true)
	} else {
		add("follow_up", 0, "medium", "follow_up_missing", true)
	}

	closed := ticket.Status == enums.TicketStatusClosed || ticket.Status == enums.TicketStatusDone || ticket.Status == enums.TicketStatusResolved
	hasResolution := strings.TrimSpace(ticket.CaseResolution) != "" || hasRepairConclusion
	if closed && ticket.ResolvedAt != nil && hasResolution {
		add("closure", 2, "high", "closure_complete", false)
	} else if closed || ticket.ResolvedAt != nil || hasResolution {
		add("closure", 1, "medium", "closure_partial", true)
	} else {
		add("closure", 0, "high", "closure_missing", false)
	}
	for _, item := range scorecard.Items {
		if _, scored := assessment.Answers[item.Code]; !scored {
			add(item.Code, item.Max/2, "low", "manual_only", true)
		}
	}

	assessment.Result = ticketQualityResult(assessment.TotalScore, assessment.MaxScore)
	return assessment, nil
}

func CreateTicketQualityScorecardVersion(version, name string, items []TicketQualityScorecardItem, operator *dto.AuthPrincipal) (*TicketQualityScorecard, error) {
	if operator == nil || operator.TenantID <= 0 || !operator.HasPermission("ticket.update") {
		return nil, errorsx.Forbidden("无质量评估配置权限")
	}
	version = strings.TrimSpace(version)
	name = strings.TrimSpace(name)
	if version == "" || name == "" || len(items) == 0 {
		return nil, errorsx.InvalidParam("评分表版本、名称和评分项不能为空")
	}
	if err := validateScorecardItems(items); err != nil {
		return nil, err
	}
	row, err := createScorecardVersion(operator.TenantID, version, name, items, operator.UserID, "draft")
	if err != nil {
		return nil, err
	}
	return buildTicketQualityScorecard(row)
}

func PublishTicketQualityScorecardVersion(id int64, operator *dto.AuthPrincipal) error {
	if operator == nil || operator.TenantID <= 0 || !operator.HasPermission("ticket.update") {
		return errorsx.Forbidden("无质量评估配置权限")
	}
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		var row models.TicketQualityScorecardVersion
		if err := ctx.Tx.Where("id = ? AND tenant_id = ?", id, operator.TenantID).First(&row).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		if err := ctx.Tx.Model(&models.TicketQualityScorecardVersion{}).Where("tenant_id = ? AND status = ?", operator.TenantID, "active").Updates(map[string]any{"status": "retired", "updated_at": now}).Error; err != nil {
			return err
		}
		return ctx.Tx.Model(&row).Updates(map[string]any{"status": "active", "published_at": now, "updated_at": now}).Error
	})
}

func CreateTicketQualityReview(ticketID int64, answers map[string]int, remark string, operator *dto.AuthPrincipal) (*models.TicketQualityReview, error) {
	return CreateTicketQualityReviewWithMetadata(ticketID, answers, remark, TicketQualityReviewMetadata{
		Evidence:       "legacy review evidence",
		CoachingAction: ticketQualityNoCoaching,
	}, operator)
}

func CreateTicketQualityReviewWithMetadata(ticketID int64, answers map[string]int, remark string, metadata TicketQualityReviewMetadata, operator *dto.AuthPrincipal) (*models.TicketQualityReview, error) {
	if operator == nil || operator.TenantID <= 0 || !operator.HasPermission("ticket.progress") {
		return nil, errorsx.Forbidden("无质量评估权限")
	}
	var ticket models.Ticket
	if err := sqls.DB().Where("id = ? AND tenant_id = ?", ticketID, operator.TenantID).First(&ticket).Error; err != nil {
		return nil, err
	}
	scorecard, err := GetActiveTicketQualityScorecard(operator.TenantID)
	if err != nil {
		return nil, err
	}
	total := 0
	for _, item := range scorecard.Items {
		score, ok := answers[item.Code]
		if !ok || score < 0 || score > item.Max {
			return nil, errorsx.InvalidParam("评分项必须全部填写，分值范围为 0 到 2")
		}
		total += score
	}
	maxScore := 0
	for _, item := range scorecard.Items {
		maxScore += item.Max
	}
	result := ticketQualityResult(total, maxScore)
	// Disputes belong to the reviewed agent after the review; a reviewer cannot pre-record one.
	if status := strings.ToLower(strings.TrimSpace(metadata.DisputeStatus)); (status != "" && status != "none") || strings.TrimSpace(metadata.DisputeNote) != "" {
		return nil, errorsx.InvalidParam("异议由被质检客服在质检后提出，提交质检时不能填写")
	}
	metadata.Evidence = strings.TrimSpace(metadata.Evidence)
	metadata.CoachingAction = strings.TrimSpace(metadata.CoachingAction)
	if metadata.Evidence == "" {
		return nil, errorsx.InvalidParam("证据不能为空")
	}
	if len([]rune(metadata.Evidence)) > 5000 || len([]rune(metadata.CoachingAction)) > 2000 {
		return nil, errorsx.InvalidParam("证据或辅导措施过长")
	}
	defectCodes := normalizeQualityDefectCodes(metadata.DefectCodes)
	scorecardCodes := make(map[string]bool, len(scorecard.Items))
	for _, item := range scorecard.Items {
		scorecardCodes[item.Code] = true
	}
	for _, code := range defectCodes {
		if !scorecardCodes[code] {
			return nil, errorsx.InvalidParam("问题类型必须从当前评分表的评分项中选择")
		}
	}
	outcome := strings.TrimSpace(metadata.Outcome)
	if outcome == "" {
		outcome = result
	}
	if outcome != result {
		return nil, errorsx.InvalidParam("评估结果与评分不一致")
	}
	if outcome != "pass" {
		if len(defectCodes) == 0 {
			return nil, errorsx.InvalidParam("结论不是合格时，至少选择一个问题类型")
		}
		if metadata.CoachingAction == "" || metadata.CoachingAction == ticketQualityNoCoaching {
			return nil, errorsx.InvalidParam("结论不是合格时，必须填写具体的辅导或改进措施")
		}
	} else if metadata.CoachingAction == "" {
		metadata.CoachingAction = ticketQualityNoCoaching
	}
	defectJSON, _ := json.Marshal(defectCodes)
	snapshot, _ := json.Marshal(map[string]any{"version": scorecard.Version, "items": scorecard.Items, "answers": answers})
	now := time.Now().UTC()
	review := &models.TicketQualityReview{TenantID: operator.TenantID, TicketID: ticket.ID, ScorecardVersionID: scorecard.ID, ScoreSnapshotJSON: string(snapshot), TotalScore: total, MaxScore: maxScore, Result: result, Outcome: outcome, DefectCodesJSON: string(defectJSON), Evidence: metadata.Evidence, DisputeStatus: "none", CoachingAction: metadata.CoachingAction, Remark: strings.TrimSpace(remark), ReviewerID: operator.UserID, ReviewedAt: now, ReviewedAgentID: ticket.CurrentAssigneeID, ReviewedTeamID: ticket.CurrentTeamID, FinalOutcome: outcome}
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		var sample models.TicketQualitySample
		if metadata.SampleID > 0 {
			if err := ctx.Tx.Where("id = ? AND tenant_id = ? AND ticket_id = ?", metadata.SampleID, operator.TenantID, ticket.ID).First(&sample).Error; err != nil {
				return errorsx.InvalidParam("质量抽检记录不存在")
			}
		}
		if err := ctx.Tx.Create(review).Error; err != nil {
			return err
		}
		if metadata.SampleID > 0 && sample.Status != ticketQualitySampleCompleted {
			updates := map[string]any{"status": ticketQualitySampleCompleted, "reviewer_id": operator.UserID, "completed_at": now, "updated_at": now}
			if err := ctx.Tx.Model(&models.TicketQualitySample{}).Where("id = ? AND tenant_id = ?", sample.ID, operator.TenantID).Updates(updates).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return review, nil
}

func normalizeQualityDefectCodes(codes []string) []string {
	seen := make(map[string]struct{}, len(codes))
	result := make([]string, 0, len(codes))
	for _, code := range codes {
		code = strings.TrimSpace(strings.ToLower(code))
		if code == "" || len(code) > 64 {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		result = append(result, code)
	}
	return result
}

func validateScorecardItems(items []TicketQualityScorecardItem) error {
	seen := map[string]bool{}
	for _, item := range items {
		if strings.TrimSpace(item.Code) == "" || strings.TrimSpace(item.Title) == "" || item.Max != 2 || seen[item.Code] {
			return errorsx.InvalidParam("评分项必须有唯一编码、名称且满分为 2")
		}
		seen[item.Code] = true
	}
	return nil
}

func createScorecardVersion(tenantID int64, version, name string, items []TicketQualityScorecardItem, userID int64, status string) (*models.TicketQualityScorecardVersion, error) {
	if err := validateScorecardItems(items); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	row := &models.TicketQualityScorecardVersion{TenantID: tenantID, Version: version, Name: name, ItemsJSON: string(raw), Status: status, CreatedBy: userID, CreatedAt: now, UpdatedAt: now}
	if status == "active" {
		row.PublishedAt = &now
	}
	if err := sqls.DB().Create(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

func buildTicketQualityScorecard(row *models.TicketQualityScorecardVersion) (*TicketQualityScorecard, error) {
	var items []TicketQualityScorecardItem
	if err := json.Unmarshal([]byte(row.ItemsJSON), &items); err != nil {
		return nil, fmt.Errorf("decode scorecard items: %w", err)
	}
	return &TicketQualityScorecard{ID: row.ID, TenantID: row.TenantID, Version: row.Version, Name: row.Name, Status: row.Status, Items: items, PublishedAt: row.PublishedAt}, nil
}
