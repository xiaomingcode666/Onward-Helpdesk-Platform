package services

import (
	"testing"
	"time"

	"github.com/mlogclub/simple/sqls"
	"github.com/stretchr/testify/require"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/projectconfig"
)

func TestTicketServiceMetricRefreshWarnsBreachesAndEscalatesP1(t *testing.T) {
	_, op, doc := setupProjectRuntime(t)
	office := projectconfig.Calendar{Key: "office", Timezone: "UTC", WorkDays: []int{1, 2, 3, 4, 5}, Start: "09:00", End: "18:00", Holidays: []string{}}
	doc.Runtime.Calendars = []projectconfig.Calendar{office}
	doc.Runtime.Targets = []projectconfig.Target{
		{Profile: "standard", CalendarKey: "office", ResponseMinutes: 30, AssignmentMinutes: 60, ResolutionMinutes: 480},
	}
	report := projectconfig.Validate(doc, doc.TenantID, projectconfig.Environment(), projectconfig.RuntimeSecretCheck)
	require.True(t, report.Valid, "fixture document issues: %+v", report.Issues)
	activateRuntime(t, doc, op)

	now := time.Date(2026, time.September, 18, 14, 0, 0, 0, time.UTC)
	createdAt := now.Add(-70 * time.Minute)
	ticket := models.Ticket{
		TenantID:      1,
		TicketNo:      "METRIC-P1",
		Channel:       "manual",
		Status:        enums.TicketStatusPending,
		AuditFields:   models.AuditFields{CreatedAt: createdAt, UpdatedAt: createdAt},
	}
	ticket.PriorityLevel = "p1"
	require.NoError(t, TicketService.Create(&ticket))

	require.NoError(t, TicketServiceMetricService.RefreshTicket(ticket, now))
	metrics := TicketServiceMetricService.ListForTicket(ticket.ID)
	require.NotEmpty(t, metrics)

	byType := make(map[string]models.TicketServiceMetric)
	for _, metric := range metrics {
		if _, exists := byType[metric.MetricType]; !exists {
			byType[metric.MetricType] = metric
		}
	}
	require.Equal(t, ServiceMetricStatusBreached, byType[ServiceMetricFirstResponse].Status)
	require.Equal(t, ServiceMetricStatusBreached, byType[ServiceMetricAcknowledgement].Status)
	require.True(t, byType[ServiceMetricFirstResponse].Escalated)
	var firstResponseMetricCount int64
	require.NoError(t, sqls.DB().Model(&models.TicketServiceMetric{}).
		Where("ticket_id = ? AND metric_type = ?", ticket.ID, ServiceMetricFirstResponse).
		Count(&firstResponseMetricCount).Error)
	require.EqualValues(t, 1, firstResponseMetricCount)
	require.NoError(t, TicketServiceMetricService.RefreshTicket(ticket, now))

	var escalationCount int64
	require.NoError(t, sqls.DB().Model(&TicketEscalation{}).
		Where("tenant_id = ? AND ticket_id = ? AND trigger_type = ?", "1", formatID(ticket.ID), "sla_violation").
		Count(&escalationCount).Error)
	require.EqualValues(t, 1, escalationCount)
	serviceReport := TicketServiceMetricService.ReportByTenant(1)
	require.NotEmpty(t, serviceReport)
	var firstResponseReport *TicketServiceMetricReportItem
	for i := range serviceReport {
		if serviceReport[i].MetricType == ServiceMetricFirstResponse {
			firstResponseReport = &serviceReport[i]
		}
	}
	require.NotNil(t, firstResponseReport)
	require.EqualValues(t, 1, firstResponseReport.Breached)

	respondedAt := now
	require.NoError(t, sqls.DB().Model(&models.Ticket{}).Where("id = ?", ticket.ID).
		Updates(map[string]any{"first_responded_at": respondedAt, "acknowledged_at": respondedAt}).Error)
	updatedTicket := TicketService.Get(ticket.ID)
	require.NoError(t, TicketServiceMetricService.RefreshTicket(*updatedTicket, now))
	require.NoError(t, TicketServiceMetricService.RefreshTicket(*updatedTicket, now))
	metrics = TicketServiceMetricService.ListForTicket(ticket.ID)
	for _, metric := range metrics {
		if metric.MetricType == ServiceMetricFirstResponse || metric.MetricType == ServiceMetricAcknowledgement {
			require.Equal(t, ServiceMetricStatusMet, metric.Status)
			require.NotNil(t, metric.ActualAt)
		}
	}
	require.NoError(t, sqls.DB().Model(&models.TicketServiceMetric{}).
		Where("ticket_id = ? AND metric_type = ?", ticket.ID, ServiceMetricFirstResponse).
		Count(&firstResponseMetricCount).Error)
	require.EqualValues(t, 1, firstResponseMetricCount)
}
