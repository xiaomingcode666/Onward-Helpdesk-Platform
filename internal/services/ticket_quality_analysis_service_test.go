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

func TestAnalyzeTicketQualityReturnsLocaleNeutralDimensionCodes(t *testing.T) {
	db := setupSLATenantTestDB(t)
	require.NoError(t, db.AutoMigrate(models.Models...))
	ticket := models.Ticket{TenantID: 1, TicketNo: "QA-EMPTY", Title: "缺少维度", Status: enums.TicketStatusClosed}
	require.NoError(t, db.Create(&ticket).Error)
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 9, Roles: []string{EnterpriseRoleOwner}, Permissions: []string{"ticket.view", "ticket.progress"}}
	card, err := GetActiveTicketQualityScorecard(1)
	require.NoError(t, err)
	answers := make(map[string]int, len(card.Items))
	for _, item := range card.Items {
		answers[item.Code] = 2
	}
	_, err = CreateTicketQualityReview(ticket.ID, answers, "", operator)
	require.NoError(t, err)

	report, err := AnalyzeTicketQuality(1, TicketQualityAnalysisFilter{}, operator)
	require.NoError(t, err)
	require.Equal(t, qualityAnalysisUnsetValue, report.ByProject[0].Value)
	require.Equal(t, qualityAnalysisUnsetValue, report.ByProject[0].Label)
	require.Equal(t, qualityAnalysisUngroupedValue, report.ByTeam[0].Value)
	require.Equal(t, qualityAnalysisUnassignedValue, report.ByAgent[0].Value)
	require.Equal(t, qualityAnalysisUncategorizedValue, report.ByCategory[0].Value)
	require.Equal(t, qualityAnalysisUnsetValue, report.ByChannel[0].Value)
}
