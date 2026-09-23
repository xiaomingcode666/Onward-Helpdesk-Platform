package services

import (
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/stretchr/testify/require"
)

func TestAnalyzeTicketQualityFiltersTenantDimensionsAndMasksAgents(t *testing.T) {
	db := setupSLATenantTestDB(t)
	require.NoError(t, db.AutoMigrate(models.Models...))
	first := models.Ticket{TenantID: 1, TicketNo: "QA-A", Title: "A", ProjectKey: "project-a", Channel: "email", Status: enums.TicketStatusClosed, TicketGovernance: models.TicketGovernance{CaseType: "billing"}}
	second := models.Ticket{TenantID: 1, TicketNo: "QA-B", Title: "B", ProjectKey: "project-b", Channel: "web", Status: enums.TicketStatusClosed, TicketGovernance: models.TicketGovernance{CaseType: "technical"}}
	require.NoError(t, db.Create(&first).Error)
	require.NoError(t, db.Create(&second).Error)
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 9, Permissions: []string{"ticket.view", "ticket.progress"}}
	answers := map[string]int{}
	card, err := GetActiveTicketQualityScorecard(1)
	require.NoError(t, err)
	for _, item := range card.Items {
		answers[item.Code] = 2
	}
	_, err = CreateTicketQualityReview(first.ID, answers, "", operator)
	require.NoError(t, err)
	answers["closure"] = 1
	_, err = CreateTicketQualityReview(second.ID, answers, "", operator)
	require.NoError(t, err)

	report, err := AnalyzeTicketQuality(1, TicketQualityAnalysisFilter{Project: "project-a"}, operator)
	require.NoError(t, err)
	require.False(t, report.CanViewAgents)
	require.Equal(t, int64(1), report.Summary.Reviews)
	require.Len(t, report.ByProject, 1)
	require.Empty(t, report.ByAgent)
}
