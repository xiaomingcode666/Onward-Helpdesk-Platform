package services

import (
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
	"github.com/stretchr/testify/require"
)

const (
	qualityTestReviewerID = int64(9)
	qualityTestAgentID    = int64(21)
	qualityTestManagerID  = int64(31)
	qualityTestOtherID    = int64(41)
)

func qualityTestReviewer() *dto.AuthPrincipal {
	return &dto.AuthPrincipal{TenantID: 1, UserID: qualityTestReviewerID, Permissions: []string{"ticket.view", "ticket.progress", "ticket.update", constants.PermissionTicketQualityDisputeReview.Code}}
}

func qualityTestAgent() *dto.AuthPrincipal {
	return &dto.AuthPrincipal{TenantID: 1, UserID: qualityTestAgentID, Permissions: []string{"ticket.view", "ticket.progress", constants.PermissionTicketQualityDisputeReview.Code}}
}

func qualityTestManager(permissions ...string) *dto.AuthPrincipal {
	return &dto.AuthPrincipal{TenantID: 1, UserID: qualityTestManagerID, Permissions: append([]string{"ticket.view"}, permissions...)}
}

func qualityTestTicket(t *testing.T, ticketNo string) models.Ticket {
	t.Helper()
	db := setupSLATenantTestDB(t)
	require.NoError(t, db.AutoMigrate(models.Models...))
	ticket := models.Ticket{TenantID: 1, TicketNo: ticketNo, Title: "质检异议", Status: enums.TicketStatusClosed, CurrentAssigneeID: qualityTestAgentID, CurrentTeamID: 7}
	require.NoError(t, db.Create(&ticket).Error)
	return ticket
}

// needsImprovementAnswers scores closure and follow_up at zero: 14/18.
func needsImprovementAnswers(t *testing.T) map[string]int {
	t.Helper()
	card, err := GetActiveTicketQualityScorecard(1)
	require.NoError(t, err)
	answers := make(map[string]int, len(card.Items))
	for _, item := range card.Items {
		answers[item.Code] = 2
	}
	answers["closure"] = 0
	answers["follow_up"] = 0
	return answers
}

func createNeedsImprovementReview(t *testing.T, ticket models.Ticket) *models.TicketQualityReview {
	t.Helper()
	review, err := CreateTicketQualityReviewWithMetadata(ticket.ID, needsImprovementAnswers(t), "", TicketQualityReviewMetadata{
		Evidence: "工单进度第 2 条未告知客户结果", DefectCodes: []string{"closure", "follow_up"}, CoachingAction: "复盘关闭前客户确认流程",
	}, qualityTestReviewer())
	require.NoError(t, err)
	return review
}

func TestTicketQualityReviewRequiresDefectAndCoachingWhenNotPassing(t *testing.T) {
	ticket := qualityTestTicket(t, "Q-RULES")
	answers := needsImprovementAnswers(t)
	reviewer := qualityTestReviewer()

	for name, metadata := range map[string]TicketQualityReviewMetadata{
		"missing defect":       {Evidence: "证据", CoachingAction: "复盘"},
		"unknown defect":       {Evidence: "证据", DefectCodes: []string{"tone"}, CoachingAction: "复盘"},
		"no coaching":          {Evidence: "证据", DefectCodes: []string{"closure"}},
		"placeholder coaching": {Evidence: "证据", DefectCodes: []string{"closure"}, CoachingAction: "无需辅导"},
		"reviewer dispute":     {Evidence: "证据", DefectCodes: []string{"closure"}, CoachingAction: "复盘", DisputeStatus: "open"},
	} {
		_, err := CreateTicketQualityReviewWithMetadata(ticket.ID, answers, "", metadata, reviewer)
		require.Error(t, err, name)
	}

	review := createNeedsImprovementReview(t, ticket)
	require.Equal(t, "needs_improvement", review.Outcome)
	require.Equal(t, "needs_improvement", review.FinalOutcome)
	require.Equal(t, qualityTestAgentID, review.ReviewedAgentID)
	require.Equal(t, int64(7), review.ReviewedTeamID)
	require.Equal(t, "none", review.DisputeStatus)

	passing := needsImprovementAnswers(t)
	passing["closure"], passing["follow_up"] = 2, 2
	passed, err := CreateTicketQualityReviewWithMetadata(ticket.ID, passing, "", TicketQualityReviewMetadata{Evidence: "全部符合"}, reviewer)
	require.NoError(t, err)
	require.Equal(t, "无需辅导", passed.CoachingAction)
}

func TestTicketQualityReviewDetailIsLimitedToAgentReviewerAndAuthorizedViewers(t *testing.T) {
	ticket := qualityTestTicket(t, "Q-VIEW")
	createNeedsImprovementReview(t, ticket)

	for name, operator := range map[string]*dto.AuthPrincipal{
		"reviewed agent": qualityTestAgent(),
		"reviewer":       qualityTestReviewer(),
		"authorized":     qualityTestManager(constants.PermissionTicketQualityViewAgent.Code),
	} {
		views, err := ListTicketQualityReviews(ticket.ID, operator)
		require.NoError(t, err, name)
		require.Len(t, views, 1, name)
		require.False(t, views[0].DetailRestricted, name)
		require.Equal(t, []string{"closure", "follow_up"}, views[0].DefectCodes, name)
		require.NotEmpty(t, views[0].Evidence, name)
		require.NotEmpty(t, views[0].CoachingAction, name)
		require.Len(t, views[0].ScoreItems, 9, name)
	}

	views, err := ListTicketQualityReviews(ticket.ID, qualityTestManager())
	require.NoError(t, err)
	require.Len(t, views, 1)
	view := views[0]
	require.True(t, view.DetailRestricted)
	require.Equal(t, "needs_improvement", view.FinalOutcome)
	require.Empty(t, view.Evidence)
	require.Empty(t, view.CoachingAction)
	require.Empty(t, view.DefectCodes)
	require.Empty(t, view.ScoreItems)
	require.Empty(t, view.ScoreSnapshotJSON)
	require.Zero(t, view.ReviewedAgentID)
	require.Nil(t, view.Dispute)

	mine, err := ListMyTicketQualityReviews(qualityTestAgent())
	require.NoError(t, err)
	require.Len(t, mine, 1)
	require.True(t, mine[0].CanDispute)
	others, err := ListMyTicketQualityReviews(qualityTestManager())
	require.NoError(t, err)
	require.Empty(t, others)
}

func TestTicketQualityDisputeIsRaisedByAgentAndDecidedByIndependentReviewer(t *testing.T) {
	ticket := qualityTestTicket(t, "Q-DISPUTE")
	review := createNeedsImprovementReview(t, ticket)

	_, err := RaiseTicketQualityDispute(review.ID, "不是我处理的", qualityTestManager(constants.PermissionTicketQualityViewAgent.Code))
	require.Error(t, err, "only the reviewed agent can dispute")
	_, err = RaiseTicketQualityDispute(review.ID, " ", qualityTestAgent())
	require.Error(t, err, "reason is required")
	dispute, err := RaiseTicketQualityDispute(review.ID, "客户已在电话中确认关闭，录音见通话记录", qualityTestAgent())
	require.NoError(t, err)
	require.Equal(t, TicketQualityDisputeOpen, dispute.Status)
	_, err = RaiseTicketQualityDispute(review.ID, "再次申诉", qualityTestAgent())
	require.Error(t, err, "a review can only be disputed once")

	open, err := ListTicketQualityDisputes(TicketQualityDisputeOpen, qualityTestManager(constants.PermissionTicketQualityDisputeReview.Code))
	require.NoError(t, err)
	require.Len(t, open, 1)
	require.Equal(t, ticket.TicketNo, open[0].TicketNo)
	_, err = ListTicketQualityDisputes(TicketQualityDisputeOpen, qualityTestManager())
	require.Error(t, err)

	decider := qualityTestManager(constants.PermissionTicketQualityDisputeReview.Code)
	_, err = DecideTicketQualityDispute(dispute.ID, TicketQualityDisputeOverturned, "pass", "录音证实已确认", qualityTestReviewer())
	require.Error(t, err, "original reviewer cannot decide")
	_, err = DecideTicketQualityDispute(dispute.ID, TicketQualityDisputeOverturned, "pass", "录音证实已确认", qualityTestAgent())
	require.Error(t, err, "reviewed agent cannot decide")
	_, err = DecideTicketQualityDispute(dispute.ID, TicketQualityDisputeOverturned, "pass", "录音证实已确认", qualityTestManager())
	require.Error(t, err, "decider needs the review permission")
	_, err = DecideTicketQualityDispute(dispute.ID, TicketQualityDisputeOverturned, "needs_improvement", "结论不变", decider)
	require.Error(t, err, "overturn must change the outcome")
	_, err = DecideTicketQualityDispute(dispute.ID, TicketQualityDisputeOverturned, "pass", " ", decider)
	require.Error(t, err, "decision note is required")

	decided, err := DecideTicketQualityDispute(dispute.ID, TicketQualityDisputeOverturned, "pass", "录音证实客户已确认关闭", decider)
	require.NoError(t, err)
	require.Equal(t, TicketQualityDisputeOverturned, decided.Status)
	_, err = DecideTicketQualityDispute(dispute.ID, TicketQualityDisputeUpheld, "", "重复处理", decider)
	require.Error(t, err, "a dispute is decided once")

	views, err := ListTicketQualityReviews(ticket.ID, qualityTestAgent())
	require.NoError(t, err)
	require.Equal(t, "needs_improvement", views[0].Outcome, "original outcome is preserved")
	require.Equal(t, 14, views[0].TotalScore, "original score is preserved")
	require.Equal(t, "pass", views[0].FinalOutcome)
	require.Equal(t, "resolved", views[0].DisputeStatus)
	require.NotNil(t, views[0].Dispute)
	require.Equal(t, "录音证实客户已确认关闭", views[0].Dispute.DecisionNote)
	require.False(t, views[0].CanDispute)

	report, err := AnalyzeTicketQuality(1, TicketQualityAnalysisFilter{}, qualityTestManager(constants.PermissionTicketQualityViewAgent.Code))
	require.NoError(t, err)
	require.Equal(t, float64(100), report.Summary.PassRate, "pass rate follows the final outcome")
}

func TestTicketQualityDisputeWindowCloses(t *testing.T) {
	ticket := qualityTestTicket(t, "Q-WINDOW")
	review := createNeedsImprovementReview(t, ticket)
	expired := time.Now().UTC().Add(-TicketQualityDisputeWindow - time.Hour)
	require.NoError(t, sqls.DB().Model(&models.TicketQualityReview{}).Where("id = ?", review.ID).Update("reviewed_at", expired).Error)

	_, err := RaiseTicketQualityDispute(review.ID, "超期申诉", qualityTestAgent())
	require.Error(t, err)
	mine, err := ListMyTicketQualityReviews(qualityTestAgent())
	require.NoError(t, err)
	require.False(t, mine[0].CanDispute)
}

func TestAnalyzeTicketQualityProtectsAgentDimensionAndUsesReviewSnapshot(t *testing.T) {
	ticket := qualityTestTicket(t, "Q-SNAPSHOT")
	createNeedsImprovementReview(t, ticket)
	require.NoError(t, sqls.DB().Model(&models.Ticket{}).Where("id = ?", ticket.ID).Updates(map[string]any{"current_assignee_id": qualityTestOtherID, "current_team_id": 8}).Error)

	_, err := AnalyzeTicketQuality(1, TicketQualityAnalysisFilter{AgentID: qualityTestAgentID}, qualityTestManager())
	require.Error(t, err, "agent filter requires the personal-quality permission")

	report, err := AnalyzeTicketQuality(1, TicketQualityAnalysisFilter{AgentID: qualityTestAgentID}, qualityTestManager(constants.PermissionTicketQualityViewAgent.Code))
	require.NoError(t, err)
	require.True(t, report.CanViewAgents)
	require.Equal(t, int64(1), report.Summary.Reviews, "the review stays with the agent who was reviewed")
	require.Len(t, report.ByAgent, 1)
	require.Equal(t, "agent:21", report.ByAgent[0].Value)
	require.Equal(t, "team:7", report.ByTeam[0].Value)
}
