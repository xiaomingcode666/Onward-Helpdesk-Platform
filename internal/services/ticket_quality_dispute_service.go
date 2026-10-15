package services

import (
	"encoding/json"
	"slices"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	TicketQualityDisputeOpen       = "open"
	TicketQualityDisputeUpheld     = "upheld"
	TicketQualityDisputeOverturned = "overturned"

	// TicketQualityDisputeWindow is how long the reviewed agent may object to a review.
	TicketQualityDisputeWindow = 7 * 24 * time.Hour

	ticketQualityListLimit = 200
)

var ticketQualityOutcomes = []string{"pass", "needs_improvement", "fail"}

type TicketQualityScoreItem struct {
	Code  string `json:"code"`
	Title string `json:"title"`
	Score int    `json:"score"`
	Max   int    `json:"max"`
}

type TicketQualityDisputeDetail struct {
	models.TicketQualityDispute
	RaisedByName  string `json:"raised_by_name"`
	DecidedByName string `json:"decided_by_name"`
}

// TicketQualityReviewView is a review as one viewer may see it. Viewers without
// personal-quality access only get the score and final outcome.
type TicketQualityReviewView struct {
	models.TicketQualityReview
	TicketNo          string                      `json:"ticket_no"`
	TicketTitle       string                      `json:"ticket_title"`
	ScorecardVersion  string                      `json:"scorecard_version"`
	ScoreItems        []TicketQualityScoreItem    `json:"score_items"`
	ReviewerName      string                      `json:"reviewer_name"`
	ReviewedAgentName string                      `json:"reviewed_agent_name"`
	Dispute           *TicketQualityDisputeDetail `json:"dispute,omitempty"`
	DisputeDeadline   *time.Time                  `json:"dispute_deadline,omitempty"`
	DetailRestricted  bool                        `json:"detail_restricted"`
	CanDispute        bool                        `json:"can_dispute"`
}

type TicketQualityDisputeView struct {
	TicketQualityDisputeDetail
	TicketID    int64                   `json:"ticket_id"`
	TicketNo    string                  `json:"ticket_no"`
	TicketTitle string                  `json:"ticket_title"`
	Review      TicketQualityReviewView `json:"review"`
}

func canViewTicketQualityDetail(review models.TicketQualityReview, operator *dto.AuthPrincipal) bool {
	return (review.ReviewedAgentID > 0 && operator.UserID == review.ReviewedAgentID) ||
		operator.UserID == review.ReviewerID ||
		operator.HasPermission(constants.PermissionTicketQualityViewAgent.Code)
}

func ListTicketQualityReviews(ticketID int64, operator *dto.AuthPrincipal) ([]TicketQualityReviewView, error) {
	if operator == nil || operator.TenantID <= 0 || !operator.HasPermission(constants.PermissionTicketView.Code) {
		return nil, errorsx.Forbidden("无工单查看权限")
	}
	var ticket models.Ticket
	if err := sqls.DB().Where("id = ? AND tenant_id = ?", ticketID, operator.TenantID).First(&ticket).Error; err != nil {
		return nil, err
	}
	var rows []models.TicketQualityReview
	if err := sqls.DB().Where("tenant_id = ? AND ticket_id = ?", operator.TenantID, ticketID).Order("reviewed_at DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return buildTicketQualityReviewViews(sqls.DB(), rows, operator, false)
}

// ListMyTicketQualityReviews returns the reviews of tickets the operator handled.
func ListMyTicketQualityReviews(operator *dto.AuthPrincipal) ([]TicketQualityReviewView, error) {
	if operator == nil || operator.TenantID <= 0 || operator.UserID <= 0 {
		return nil, errorsx.Forbidden("请登录后查看")
	}
	var rows []models.TicketQualityReview
	if err := sqls.DB().Where("tenant_id = ? AND reviewed_agent_id = ?", operator.TenantID, operator.UserID).Order("reviewed_at DESC, id DESC").Limit(ticketQualityListLimit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return buildTicketQualityReviewViews(sqls.DB(), rows, operator, false)
}

func RaiseTicketQualityDispute(reviewID int64, reason string, operator *dto.AuthPrincipal) (*models.TicketQualityDispute, error) {
	if operator == nil || operator.TenantID <= 0 || operator.UserID <= 0 {
		return nil, errorsx.Forbidden("请登录后操作")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" || len([]rune(reason)) > 2000 {
		return nil, errorsx.InvalidParam("请填写异议理由（最多 2000 字）")
	}
	var dispute *models.TicketQualityDispute
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		var review models.TicketQualityReview
		if err := ctx.Tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND tenant_id = ?", reviewID, operator.TenantID).First(&review).Error; err != nil {
			return errorsx.InvalidParam("质检记录不存在")
		}
		if review.ReviewedAgentID <= 0 || review.ReviewedAgentID != operator.UserID {
			return errorsx.Forbidden("只有被质检的客服本人可以提出异议")
		}
		if time.Now().UTC().After(review.ReviewedAt.Add(TicketQualityDisputeWindow)) {
			return errorsx.InvalidParam("已超过 7 天异议期限")
		}
		if review.DisputeStatus != "" && review.DisputeStatus != "none" {
			return errorsx.InvalidParam("每条质检记录只能提出一次异议")
		}
		var existing int64
		if err := ctx.Tx.Model(&models.TicketQualityDispute{}).Where("tenant_id = ? AND review_id = ?", operator.TenantID, review.ID).Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return errorsx.InvalidParam("每条质检记录只能提出一次异议")
		}
		dispute = &models.TicketQualityDispute{TenantID: operator.TenantID, ReviewID: review.ID, RaisedBy: operator.UserID, Reason: reason, RaisedAt: time.Now().UTC(), Status: TicketQualityDisputeOpen}
		if err := ctx.Tx.Create(dispute).Error; err != nil {
			return err
		}
		return ctx.Tx.Model(&models.TicketQualityReview{}).Where("id = ? AND tenant_id = ?", review.ID, operator.TenantID).Update("dispute_status", TicketQualityDisputeOpen).Error
	})
	return dispute, err
}

func ListTicketQualityDisputes(status string, operator *dto.AuthPrincipal) ([]TicketQualityDisputeView, error) {
	if operator == nil || operator.TenantID <= 0 || !operator.HasPermission(constants.PermissionTicketQualityDisputeReview.Code) {
		return nil, errorsx.Forbidden("无质检异议复核权限")
	}
	status = strings.TrimSpace(status)
	query := sqls.DB().Where("tenant_id = ?", operator.TenantID)
	if status != "" {
		if !slices.Contains([]string{TicketQualityDisputeOpen, TicketQualityDisputeUpheld, TicketQualityDisputeOverturned}, status) {
			return nil, errorsx.InvalidParam("异议状态无效")
		}
		query = query.Where("status = ?", status)
	}
	var disputes []models.TicketQualityDispute
	if err := query.Order("raised_at DESC, id DESC").Limit(ticketQualityListLimit).Find(&disputes).Error; err != nil {
		return nil, err
	}
	result := make([]TicketQualityDisputeView, 0, len(disputes))
	if len(disputes) == 0 {
		return result, nil
	}
	reviewIDs := make([]int64, 0, len(disputes))
	for _, dispute := range disputes {
		reviewIDs = append(reviewIDs, dispute.ReviewID)
	}
	var reviews []models.TicketQualityReview
	if err := sqls.DB().Where("tenant_id = ? AND id IN ?", operator.TenantID, reviewIDs).Find(&reviews).Error; err != nil {
		return nil, err
	}
	// Deciding a dispute requires seeing the disputed review in full.
	views, err := buildTicketQualityReviewViews(sqls.DB(), reviews, operator, true)
	if err != nil {
		return nil, err
	}
	byReview := make(map[int64]TicketQualityReviewView, len(views))
	for _, view := range views {
		byReview[view.ID] = view
	}
	for _, dispute := range disputes {
		review, ok := byReview[dispute.ReviewID]
		if !ok {
			continue
		}
		detail := review.Dispute
		if detail == nil {
			detail = &TicketQualityDisputeDetail{TicketQualityDispute: dispute}
		}
		result = append(result, TicketQualityDisputeView{TicketQualityDisputeDetail: *detail, TicketID: review.TicketID, TicketNo: review.TicketNo, TicketTitle: review.TicketTitle, Review: review})
	}
	return result, nil
}

// DecideTicketQualityDispute keeps the original score and outcome; an overturn
// only changes the review's final outcome.
func DecideTicketQualityDispute(disputeID int64, decision, outcome, note string, operator *dto.AuthPrincipal) (*models.TicketQualityDispute, error) {
	if operator == nil || operator.TenantID <= 0 || operator.UserID <= 0 || !operator.HasPermission(constants.PermissionTicketQualityDisputeReview.Code) {
		return nil, errorsx.Forbidden("无质检异议复核权限")
	}
	decision = strings.TrimSpace(decision)
	outcome = strings.TrimSpace(outcome)
	note = strings.TrimSpace(note)
	if decision != TicketQualityDisputeUpheld && decision != TicketQualityDisputeOverturned {
		return nil, errorsx.InvalidParam("复核结论必须是维持或改判")
	}
	if note == "" || len([]rune(note)) > 2000 {
		return nil, errorsx.InvalidParam("请填写复核意见（最多 2000 字）")
	}
	var dispute models.TicketQualityDispute
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := ctx.Tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND tenant_id = ?", disputeID, operator.TenantID).First(&dispute).Error; err != nil {
			return errorsx.InvalidParam("异议不存在")
		}
		if dispute.Status != TicketQualityDisputeOpen {
			return errorsx.InvalidParam("该异议已复核")
		}
		var review models.TicketQualityReview
		if err := ctx.Tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND tenant_id = ?", dispute.ReviewID, operator.TenantID).First(&review).Error; err != nil {
			return errorsx.InvalidParam("质检记录不存在")
		}
		if operator.UserID == review.ReviewerID || operator.UserID == review.ReviewedAgentID {
			return errorsx.Forbidden("原质检人和被质检客服不能复核该异议")
		}
		final := effectiveTicketQualityOutcome(review)
		if decision == TicketQualityDisputeOverturned {
			if !slices.Contains(ticketQualityOutcomes, outcome) || outcome == final {
				return errorsx.InvalidParam("改判必须给出与当前不同的结论")
			}
			final = outcome
		}
		now := time.Now().UTC()
		updated := ctx.Tx.Model(&models.TicketQualityDispute{}).Where("id = ? AND tenant_id = ? AND status = ?", dispute.ID, operator.TenantID, TicketQualityDisputeOpen).
			Updates(map[string]any{"status": decision, "decided_by": operator.UserID, "decision_note": note, "decided_outcome": final, "decided_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return errorsx.InvalidParam("该异议已复核")
		}
		if err := ctx.Tx.Model(&models.TicketQualityReview{}).Where("id = ? AND tenant_id = ?", review.ID, operator.TenantID).
			Updates(map[string]any{"final_outcome": final, "dispute_status": "resolved"}).Error; err != nil {
			return err
		}
		dispute.Status, dispute.DecidedBy, dispute.DecisionNote, dispute.DecidedOutcome, dispute.DecidedAt = decision, operator.UserID, note, final, &now
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &dispute, nil
}

// effectiveTicketQualityOutcome falls back for reviews created before final outcomes existed.
func effectiveTicketQualityOutcome(review models.TicketQualityReview) string {
	for _, value := range []string{review.FinalOutcome, review.Outcome, review.Result} {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func buildTicketQualityReviewViews(db *gorm.DB, rows []models.TicketQualityReview, operator *dto.AuthPrincipal, forceDetail bool) ([]TicketQualityReviewView, error) {
	views := make([]TicketQualityReviewView, 0, len(rows))
	if len(rows) == 0 {
		return views, nil
	}
	ticketIDs := make([]int64, 0, len(rows))
	reviewIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		ticketIDs = append(ticketIDs, row.TicketID)
		reviewIDs = append(reviewIDs, row.ID)
	}
	var tickets []models.Ticket
	if err := db.Where("tenant_id = ? AND id IN ?", operator.TenantID, ticketIDs).Find(&tickets).Error; err != nil {
		return nil, err
	}
	ticketByID := make(map[int64]models.Ticket, len(tickets))
	for _, ticket := range tickets {
		ticketByID[ticket.ID] = ticket
	}
	var disputes []models.TicketQualityDispute
	if err := db.Where("tenant_id = ? AND review_id IN ?", operator.TenantID, reviewIDs).Find(&disputes).Error; err != nil {
		return nil, err
	}
	disputeByReview := make(map[int64]models.TicketQualityDispute, len(disputes))
	for _, dispute := range disputes {
		disputeByReview[dispute.ReviewID] = dispute
	}
	names := ticketQualityUserNames(db)
	now := time.Now().UTC()
	for _, row := range rows {
		ticket := ticketByID[row.TicketID]
		view := TicketQualityReviewView{TicketQualityReview: row, TicketNo: ticket.TicketNo, TicketTitle: ticket.Title}
		view.FinalOutcome = effectiveTicketQualityOutcome(row)
		view.ReviewerName = names(row.ReviewerID)
		if !forceDetail && !canViewTicketQualityDetail(row, operator) {
			view.DetailRestricted = true
			view.ScoreSnapshotJSON, view.Evidence, view.CoachingAction, view.DisputeNote, view.Remark = "", "", "", "", ""
			view.DefectCodes, view.ScoreItems = []string{}, []TicketQualityScoreItem{}
			view.ReviewedAgentID, view.ReviewedTeamID = 0, 0
			views = append(views, view)
			continue
		}
		view.DefectCodes = []string{}
		_ = json.Unmarshal([]byte(row.DefectCodesJSON), &view.DefectCodes)
		view.ScorecardVersion, view.ScoreItems = ticketQualityScoreItems(row.ScoreSnapshotJSON)
		view.ReviewedAgentName = names(row.ReviewedAgentID)
		if dispute, ok := disputeByReview[row.ID]; ok {
			view.Dispute = &TicketQualityDisputeDetail{TicketQualityDispute: dispute, RaisedByName: names(dispute.RaisedBy), DecidedByName: names(dispute.DecidedBy)}
		}
		if row.ReviewedAgentID > 0 {
			deadline := row.ReviewedAt.Add(TicketQualityDisputeWindow)
			view.DisputeDeadline = &deadline
			view.CanDispute = operator.UserID == row.ReviewedAgentID && view.Dispute == nil && (row.DisputeStatus == "" || row.DisputeStatus == "none") && now.Before(deadline)
		}
		views = append(views, view)
	}
	return views, nil
}

func ticketQualityScoreItems(snapshotJSON string) (string, []TicketQualityScoreItem) {
	var snapshot struct {
		Version string                       `json:"version"`
		Items   []TicketQualityScorecardItem `json:"items"`
		Answers map[string]int               `json:"answers"`
	}
	items := []TicketQualityScoreItem{}
	if err := json.Unmarshal([]byte(snapshotJSON), &snapshot); err != nil {
		return "", items
	}
	for _, item := range snapshot.Items {
		items = append(items, TicketQualityScoreItem{Code: item.Code, Title: item.Title, Score: snapshot.Answers[item.Code], Max: item.Max})
	}
	return snapshot.Version, items
}

// ticketQualityUserNames resolves display names lazily and caches them per request.
func ticketQualityUserNames(db *gorm.DB) func(int64) string {
	cache := map[int64]string{}
	return func(id int64) string {
		if id <= 0 {
			return ""
		}
		if name, ok := cache[id]; ok {
			return name
		}
		name := ""
		if user := repositories.UserRepository.Get(db, id); user != nil {
			name = strings.TrimSpace(user.Nickname)
			if name == "" {
				name = user.Username
			}
		}
		cache[id] = name
		return name
	}
}
