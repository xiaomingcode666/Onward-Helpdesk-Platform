package services

import (
	"testing"

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
