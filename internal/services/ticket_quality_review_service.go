package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
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
	DefectCodes    []string `json:"defect_codes"`
	Evidence       string   `json:"evidence"`
	DisputeStatus  string   `json:"dispute_status"`
	DisputeNote    string   `json:"dispute_note"`
	Outcome        string   `json:"outcome"`
	CoachingAction string   `json:"coaching_action"`
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
		DisputeStatus:  "none",
		Evidence:       "legacy review evidence",
		CoachingAction: "无需辅导",
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
	maxScore := len(scorecard.Items) * 2
	result := "fail"
	if total >= 15 {
		result = "pass"
	} else if total >= 10 {
		result = "needs_improvement"
	}
	metadata.DisputeStatus = strings.ToLower(strings.TrimSpace(metadata.DisputeStatus))
	if metadata.DisputeStatus == "" {
		metadata.DisputeStatus = "none"
	}
	if metadata.DisputeStatus != "none" && metadata.DisputeStatus != "open" && metadata.DisputeStatus != "resolved" {
		return nil, errorsx.InvalidParam("争议状态必须为 none、open 或 resolved")
	}
	metadata.Evidence = strings.TrimSpace(metadata.Evidence)
	metadata.DisputeNote = strings.TrimSpace(metadata.DisputeNote)
	metadata.CoachingAction = strings.TrimSpace(metadata.CoachingAction)
	if metadata.Evidence == "" {
		return nil, errorsx.InvalidParam("证据不能为空")
	}
	if metadata.CoachingAction == "" {
		return nil, errorsx.InvalidParam("辅导动作不能为空")
	}
	if len([]rune(metadata.Evidence)) > 5000 || len([]rune(metadata.DisputeNote)) > 2000 || len([]rune(metadata.CoachingAction)) > 2000 {
		return nil, errorsx.InvalidParam("证据、争议说明或辅导动作过长")
	}
	defectCodes := normalizeQualityDefectCodes(metadata.DefectCodes)
	if len(defectCodes) > 0 && metadata.Evidence == "" {
		return nil, errorsx.InvalidParam("填写缺陷码时必须提供证据")
	}
	outcome := strings.TrimSpace(metadata.Outcome)
	if outcome == "" {
		outcome = result
	}
	if outcome != "pass" && outcome != "needs_improvement" && outcome != "fail" {
		return nil, errorsx.InvalidParam("结果必须为 pass、needs_improvement 或 fail")
	}
	defectJSON, _ := json.Marshal(defectCodes)
	snapshot, _ := json.Marshal(map[string]any{"version": scorecard.Version, "items": scorecard.Items, "answers": answers})
	now := time.Now().UTC()
	review := &models.TicketQualityReview{TenantID: operator.TenantID, TicketID: ticket.ID, ScorecardVersionID: scorecard.ID, ScoreSnapshotJSON: string(snapshot), TotalScore: total, MaxScore: maxScore, Result: result, Outcome: outcome, DefectCodesJSON: string(defectJSON), Evidence: metadata.Evidence, DisputeStatus: metadata.DisputeStatus, DisputeNote: metadata.DisputeNote, CoachingAction: metadata.CoachingAction, Remark: strings.TrimSpace(remark), ReviewerID: operator.UserID, ReviewedAt: now}
	if err := sqls.DB().Create(review).Error; err != nil {
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

func ListTicketQualityReviews(ticketID int64, operator *dto.AuthPrincipal) ([]models.TicketQualityReview, error) {
	if operator == nil || operator.TenantID <= 0 || !operator.HasPermission("ticket.view") {
		return nil, errorsx.Forbidden("无工单查看权限")
	}
	var ticket models.Ticket
	if err := sqls.DB().Where("id = ? AND tenant_id = ?", ticketID, operator.TenantID).First(&ticket).Error; err != nil {
		return nil, err
	}
	var rows []models.TicketQualityReview
	err := sqls.DB().Where("tenant_id = ? AND ticket_id = ?", operator.TenantID, ticketID).Order("reviewed_at DESC, id DESC").Find(&rows).Error
	return rows, err
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
