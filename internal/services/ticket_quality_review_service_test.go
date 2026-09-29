package services

import (
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/stretchr/testify/require"
)

func TestTicketQualityReviewUsesVersionedScorecardAndPreservesSnapshot(t *testing.T) {
	db := setupSLATenantTestDB(t)
	require.NoError(t, db.AutoMigrate(models.Models...))
	ticket := models.Ticket{TenantID: 1, TicketNo: "Q-100", Title: "质量检查", Status: enums.TicketStatusClosed}
	require.NoError(t, db.Create(&ticket).Error)
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 9, Permissions: []string{"ticket.view", "ticket.progress", "ticket.update"}}

	scorecard, err := GetActiveTicketQualityScorecard(1)
	require.NoError(t, err)
	require.Equal(t, "v1", scorecard.Version)
	require.Len(t, scorecard.Items, 9)

	answers := map[string]int{}
	for _, item := range scorecard.Items {
		answers[item.Code] = 2
	}
	review, err := CreateTicketQualityReview(ticket.ID, answers, "全部符合", operator)
	require.NoError(t, err)
	require.Equal(t, 18, review.TotalScore)
	require.Equal(t, "pass", review.Result)
	require.Equal(t, scorecard.ID, review.ScorecardVersionID)

	newVersion, err := CreateTicketQualityScorecardVersion("v2", "客服质量评估表 v2", scorecard.Items, operator)
	require.NoError(t, err)
	require.NoError(t, PublishTicketQualityScorecardVersion(newVersion.ID, operator))
	active, err := GetActiveTicketQualityScorecard(1)
	require.NoError(t, err)
	require.Equal(t, newVersion.ID, active.ID)

	history, err := ListTicketQualityReviews(ticket.ID, operator)
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Equal(t, scorecard.ID, history[0].ScorecardVersionID)
	require.Contains(t, history[0].ScoreSnapshotJSON, "identity_confirmation")
}

func TestTicketQualityReviewRejectsInvalidScore(t *testing.T) {
	db := setupSLATenantTestDB(t)
	require.NoError(t, db.AutoMigrate(models.Models...))
	ticket := models.Ticket{TenantID: 1, TicketNo: "Q-101", Title: "质量检查", Status: enums.TicketStatusClosed}
	require.NoError(t, db.Create(&ticket).Error)
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 9, Permissions: []string{"ticket.view", "ticket.progress"}}
	_, err := CreateTicketQualityReview(ticket.ID, map[string]int{"identity_confirmation": 3}, "", operator)
	require.Error(t, err)
}

func TestTicketQualityAutoAssessmentScoresRecordedFactsAndFlagsManualReview(t *testing.T) {
	db := setupSLATenantTestDB(t)
	require.NoError(t, db.AutoMigrate(models.Models...))
	now := time.Now().UTC()
	ticket := models.Ticket{
		TenantID: 1, TicketNo: "Q-AUTO-1", Title: "自动质检", Description: "设备无法启动",
		Status: enums.TicketStatusClosed, CustomerID: 88, ContextStatus: "complete", MissingContextJSON: "[]",
		CurrentTeamID: 5, CurrentAssigneeID: 9, FirstRespondedAt: &now, LastCustomerUpdateAt: &now,
		ResolvedAt: &now, CaseResolution: "更换电源模块",
		TicketGovernance: models.TicketGovernance{CaseType: "incident", PriorityLevel: "p2"},
	}
	require.NoError(t, db.Create(&ticket).Error)
	require.NoError(t, db.Create(&models.TicketProgress{TenantID: 1, TicketID: ticket.ID, EventType: enums.TicketProgressEventProgress, Content: "已确认问题", VisibleToCustomer: true, CreatedAt: now.Add(-time.Hour)}).Error)
	require.NoError(t, db.Create(&models.TicketProgress{TenantID: 1, TicketID: ticket.ID, EventType: enums.TicketProgressEventProgress, Content: "已恢复服务", VisibleToCustomer: true, CreatedAt: now}).Error)
	require.NoError(t, db.Create(&models.TicketRepairRecord{TenantID: 1, TicketID: ticket.ID, Conclusion: "修复完成", Solution: "更换电源模块", AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}).Error)

	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 9, Permissions: []string{"ticket.view", "ticket.progress"}}
	assessment, err := BuildTicketQualityAutoAssessment(ticket.ID, operator)
	require.NoError(t, err)
	require.Equal(t, "rules-v1", assessment.Version)
	require.Equal(t, 17, assessment.TotalScore)
	require.Equal(t, 18, assessment.MaxScore)
	require.Equal(t, "pass", assessment.Result)
	require.Equal(t, 1, assessment.ReviewRequiredCount)
	require.Equal(t, 1, assessment.Answers["data_protection"])
	require.Empty(t, assessment.DefectCodes)

	var protection TicketQualityAutoAssessmentItem
	for _, item := range assessment.Items {
		if item.Code == "data_protection" {
			protection = item
		}
	}
	require.True(t, protection.RequiresReview)
	require.Equal(t, "data_protection_review", protection.EvidenceKey)
}

func TestTicketQualitySampleListIncludesLatestReviewSummary(t *testing.T) {
	db := setupSLATenantTestDB(t)
	require.NoError(t, db.AutoMigrate(models.Models...))
	now := time.Now().UTC()
	ticket := models.Ticket{TenantID: 1, TicketNo: "Q-LIST-1", Title: "结果列表", Status: enums.TicketStatusClosed}
	require.NoError(t, db.Create(&ticket).Error)
	require.NoError(t, db.Create(&models.TicketQualitySample{TenantID: 1, TicketID: ticket.ID, PeriodKey: "2026-09-28", ReasonsJSON: "[\"random\"]", Status: ticketQualitySampleCompleted, SampledAt: now, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}).Error)
	require.NoError(t, db.Create(&models.TicketQualityReview{TenantID: 1, TicketID: ticket.ID, ScorecardVersionID: 1, ScoreSnapshotJSON: "{}", TotalScore: 16, MaxScore: 18, Result: "pass", Outcome: "pass", ReviewedAt: now}).Error)

	page, err := TicketQualitySampleService.List(1, ticketQualitySampleQuery{Page: 1, PageSize: 10, PeriodKey: "2026-09-28"})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.NotNil(t, page.Items[0].LatestReview)
	require.Equal(t, 16, page.Items[0].LatestReview.TotalScore)
	require.Equal(t, "pass", page.Items[0].LatestReview.Result)
}

func TestTicketQualityAutoAssessmentKeepsCustomScorecardItemsReviewable(t *testing.T) {
	db := setupSLATenantTestDB(t)
	require.NoError(t, db.AutoMigrate(models.Models...))
	ticket := models.Ticket{TenantID: 1, TicketNo: "Q-AUTO-CUSTOM", Title: "自定义评分项", Status: enums.TicketStatusClosed}
	require.NoError(t, db.Create(&ticket).Error)
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 9, Permissions: []string{"ticket.view", "ticket.progress", "ticket.update"}}
	items := append(DefaultTicketQualityScorecardItems(), TicketQualityScorecardItem{Code: "custom_check", Title: "自定义检查", Max: 2})
	version, err := CreateTicketQualityScorecardVersion("v-custom", "自定义评分表", items, operator)
	require.NoError(t, err)
	require.NoError(t, PublishTicketQualityScorecardVersion(version.ID, operator))

	assessment, err := BuildTicketQualityAutoAssessment(ticket.ID, operator)
	require.NoError(t, err)
	require.Equal(t, 1, assessment.Answers["custom_check"])
	require.Equal(t, 20, assessment.MaxScore)
	require.Contains(t, assessment.Items, TicketQualityAutoAssessmentItem{Code: "custom_check", Title: "自定义检查", Score: 1, Max: 2, Confidence: "low", EvidenceKey: "manual_only", RequiresReview: true})
}

func TestTicketQualityReviewRejectsOutcomeThatDoesNotMatchScore(t *testing.T) {
	db := setupSLATenantTestDB(t)
	require.NoError(t, db.AutoMigrate(models.Models...))
	ticket := models.Ticket{TenantID: 1, TicketNo: "Q-OUTCOME", Title: "结果一致性", Status: enums.TicketStatusClosed}
	require.NoError(t, db.Create(&ticket).Error)
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 9, Permissions: []string{"ticket.view", "ticket.progress"}}
	scorecard, err := GetActiveTicketQualityScorecard(1)
	require.NoError(t, err)
	answers := make(map[string]int, len(scorecard.Items))
	for _, item := range scorecard.Items {
		answers[item.Code] = 0
	}
	_, err = CreateTicketQualityReviewWithMetadata(ticket.ID, answers, "", TicketQualityReviewMetadata{
		Evidence: "工单记录", DisputeStatus: "none", CoachingAction: "重新培训", Outcome: "pass",
	}, operator)
	require.Error(t, err)
}

func TestTicketQualityReviewCompletesSpecifiedSampleAtomically(t *testing.T) {
	db := setupSLATenantTestDB(t)
	require.NoError(t, db.AutoMigrate(models.Models...))
	now := time.Now().UTC()
	ticket := models.Ticket{TenantID: 1, TicketNo: "Q-SAMPLE", Title: "抽检完成", Status: enums.TicketStatusClosed}
	require.NoError(t, db.Create(&ticket).Error)
	sample := models.TicketQualitySample{TenantID: 1, TicketID: ticket.ID, PeriodKey: "2026-09-28", ReasonsJSON: "[]", Status: ticketQualitySampleInProgress, SampledAt: now, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	require.NoError(t, db.Create(&sample).Error)
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 9, Permissions: []string{"ticket.view", "ticket.progress"}}
	scorecard, err := GetActiveTicketQualityScorecard(1)
	require.NoError(t, err)
	answers := make(map[string]int, len(scorecard.Items))
	for _, item := range scorecard.Items {
		answers[item.Code] = 2
	}
	_, err = CreateTicketQualityReviewWithMetadata(ticket.ID, answers, "", TicketQualityReviewMetadata{
		SampleID: sample.ID, Evidence: "工单记录", DisputeStatus: "none", CoachingAction: "无需辅导", Outcome: "pass",
	}, operator)
	require.NoError(t, err)
	require.NoError(t, db.First(&sample, sample.ID).Error)
	require.Equal(t, ticketQualitySampleCompleted, sample.Status)
	require.Equal(t, operator.UserID, sample.ReviewerID)
	require.NotNil(t, sample.CompletedAt)
}

func TestTicketQualitySampleSummaryUsesCurrentPeriod(t *testing.T) {
	db := setupSLATenantTestDB(t)
	require.NoError(t, db.AutoMigrate(models.Models...))
	now := time.Now().UTC()
	tickets := []models.Ticket{
		{TenantID: 1, TicketNo: "Q-PERIOD-1", Title: "本期", Status: enums.TicketStatusClosed},
		{TenantID: 1, TicketNo: "Q-PERIOD-2", Title: "其他期", Status: enums.TicketStatusClosed},
	}
	require.NoError(t, db.Create(&tickets).Error)
	require.NoError(t, db.Create(&models.TicketQualitySample{TenantID: 1, TicketID: tickets[0].ID, PeriodKey: "2026-09-28", ReasonsJSON: "[]", Status: ticketQualitySampleCompleted, SampledAt: now, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}).Error)
	require.NoError(t, db.Create(&models.TicketQualitySample{TenantID: 1, TicketID: tickets[1].ID, PeriodKey: "2026-09-27", ReasonsJSON: "[]", Status: ticketQualitySamplePending, SampledAt: now, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}).Error)

	page, err := TicketQualitySampleService.List(1, ticketQualitySampleQuery{Page: 1, PageSize: 10, PeriodKey: "2026-09-28"})
	require.NoError(t, err)
	require.Equal(t, int64(1), page.Summary.Total)
	require.Equal(t, int64(1), page.Summary.Completed)
	require.Zero(t, page.Summary.Pending)
}

func TestTicketQualityThresholdScalesWithScorecardMaximum(t *testing.T) {
	db := setupSLATenantTestDB(t)
	require.NoError(t, db.AutoMigrate(models.Models...))
	ticket := models.Ticket{TenantID: 1, TicketNo: "Q-THRESHOLD", Title: "比例阈值", Status: enums.TicketStatusClosed}
	require.NoError(t, db.Create(&ticket).Error)
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 9, Permissions: []string{"ticket.view", "ticket.progress", "ticket.update"}}
	items := append(DefaultTicketQualityScorecardItems(), TicketQualityScorecardItem{Code: "custom_check", Title: "自定义检查", Max: 2})
	version, err := CreateTicketQualityScorecardVersion("v-threshold", "十分制评分项", items, operator)
	require.NoError(t, err)
	require.NoError(t, PublishTicketQualityScorecardVersion(version.ID, operator))
	answers := make(map[string]int, len(items))
	for index, item := range items {
		if index < 8 {
			answers[item.Code] = 2
		} else {
			answers[item.Code] = 0
		}
	}
	review, err := CreateTicketQualityReviewWithMetadata(ticket.ID, answers, "", TicketQualityReviewMetadata{
		Evidence: "工单记录", DisputeStatus: "none", CoachingAction: "重新培训", Outcome: "needs_improvement",
	}, operator)
	require.NoError(t, err)
	require.Equal(t, 20, review.MaxScore)
	require.Equal(t, "needs_improvement", review.Result)
}
