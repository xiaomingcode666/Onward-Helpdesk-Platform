package services

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/projectconfig"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

const (
	ServiceMetricAcknowledgement = "acknowledgement"
	ServiceMetricFirstResponse   = "first_response"
	ServiceMetricUpdateCadence   = "update_cadence"
	ServiceMetricEscalation      = "escalation"
	ServiceMetricRestore         = "restore"
	ServiceMetricResolve         = "resolve"

	ServiceMetricStatusRunning   = "running"
	ServiceMetricStatusWarning   = "warning"
	ServiceMetricStatusBreached  = "breached"
	ServiceMetricStatusMet       = "met"
	ServiceMetricStatusEscalated = "escalated"
)

var TicketServiceMetricService = newTicketServiceMetricService()

func newTicketServiceMetricService() *ticketServiceMetricService {
	return &ticketServiceMetricService{}
}

type ticketServiceMetricService struct{}

type ticketMetricSpec struct {
	metricType    string
	targetMinutes int
	startedAt     time.Time
	actualAt      *time.Time
}

type TicketServiceMetricReportItem struct {
	MetricType string `json:"metric_type"`
	Running    int64  `json:"running"`
	Warning    int64  `json:"warning"`
	Breached   int64  `json:"breached"`
	Met        int64  `json:"met"`
	Escalated  int64  `json:"escalated"`
	Total      int64  `json:"total"`
}

func (s *ticketServiceMetricService) RefreshTicket(ticket models.Ticket, now time.Time) error {
	if ticket.ID <= 0 || ticket.TenantID <= 0 {
		return nil
	}
	db := sqls.DB()
	target, calendar, ok := s.targetForTicket(db, ticket)
	if !ok {
		return nil
	}

	updateMinutes := max(1, target.ResolutionMinutes/2)
	escalationAt := s.firstEscalationAt(db, ticket)
	metrics := []ticketMetricSpec{
		{ServiceMetricAcknowledgement, target.AssignmentMinutes, ticket.CreatedAt, ticket.AcknowledgedAt},
		{ServiceMetricFirstResponse, target.ResponseMinutes, ticket.CreatedAt, ticket.FirstRespondedAt},
		{ServiceMetricUpdateCadence, updateMinutes, s.updateCadenceStart(ticket), s.updateCadenceActual(ticket)},
		{ServiceMetricEscalation, max(1, target.ResponseMinutes), ticket.CreatedAt, escalationAt},
		{ServiceMetricRestore, target.ResolutionMinutes, ticket.CreatedAt, firstTime(ticket.RestoredAt, ticket.ResolvedAt)},
		{ServiceMetricResolve, target.ResolutionMinutes, ticket.CreatedAt, ticket.ResolvedAt},
	}
	for _, spec := range metrics {
		if err := s.upsertMetric(db, ticket, calendar, spec, now); err != nil {
			return err
		}
	}
	return s.escalatePriorityRisk(db, ticket, now)
}

func (s *ticketServiceMetricService) ScanActiveTickets(limit int) (int, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var total int64
	if err := sqls.DB().Model(&models.Ticket{}).Where(ticketCaseOpenSQL).Count(&total).Error; err != nil {
		return 0, err
	}
	offset := 0
	if total > int64(limit) {
		pages := int((total + int64(limit) - 1) / int64(limit))
		offset = (int(time.Now().Unix()/60) % pages) * limit
	}
	var tickets []models.Ticket
	if err := sqls.DB().Where(ticketCaseOpenSQL).Order("id ASC").Offset(offset).Limit(limit).Find(&tickets).Error; err != nil {
		return 0, err
	}
	now := time.Now()
	processed := 0
	for _, ticket := range tickets {
		if err := s.RefreshTicket(ticket, now); err != nil {
			slog.Warn("refresh ticket service metrics failed", "ticketId", ticket.ID, "error", err)
			continue
		}
		processed++
	}
	return processed, nil
}

func (s *ticketServiceMetricService) ListForTicket(ticketID int64) []models.TicketServiceMetric {
	items := make([]models.TicketServiceMetric, 0)
	if ticketID <= 0 {
		return items
	}
	sqls.DB().Where("ticket_id = ?", ticketID).Order("metric_type ASC, id DESC").Find(&items)
	return items
}

func (s *ticketServiceMetricService) RecordCustomerReplyForConversation(db *gorm.DB, conversationID int64, now time.Time) error {
	if db == nil || conversationID <= 0 {
		return nil
	}
	var ticket models.Ticket
	if err := db.Where("conversation_id = ?", conversationID).Where(ticketCaseOpenSQL).First(&ticket).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	updates := map[string]any{
		"last_customer_update_at": now,
		"updated_at":              now,
	}
	if ticket.FirstRespondedAt == nil {
		updates["first_responded_at"] = now
	}
	return db.Model(&models.Ticket{}).Where("id = ?", ticket.ID).Updates(updates).Error
}

func (s *ticketServiceMetricService) ReportByTenant(tenantID int64) []TicketServiceMetricReportItem {
	result := make([]TicketServiceMetricReportItem, 0)
	if tenantID <= 0 {
		return result
	}
	type metricReportRow struct {
		MetricType string
		Running    int64
		Warning    int64
		Breached   int64
		Met        int64
		Escalated  int64
		Total      int64
	}
	rows := make([]metricReportRow, 0)
	err := sqls.DB().Raw(`
		SELECT metric_type,
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS running,
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS warning,
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS breached,
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS met,
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS escalated,
			COUNT(1) AS total
		FROM t_ticket_service_metric
		WHERE tenant_id = ?
			AND id IN (
				SELECT MAX(id)
				FROM t_ticket_service_metric
				WHERE tenant_id = ?
				GROUP BY ticket_id, metric_type
			)
		GROUP BY metric_type`,
		ServiceMetricStatusRunning,
		ServiceMetricStatusWarning,
		ServiceMetricStatusBreached,
		ServiceMetricStatusMet,
		ServiceMetricStatusEscalated,
		tenantID,
		tenantID,
	).Scan(&rows).Error
	if err != nil {
		slog.Warn("service metric report failed", "tenantId", tenantID, "error", err)
		return result
	}
	for _, row := range rows {
		result = append(result, TicketServiceMetricReportItem{
			MetricType: row.MetricType,
			Running:    row.Running,
			Warning:    row.Warning,
			Breached:   row.Breached,
			Met:        row.Met,
			Escalated:  row.Escalated,
			Total:      row.Total,
		})
	}
	return result
}

func (s *ticketServiceMetricService) targetForTicket(db *gorm.DB, ticket models.Ticket) (projectconfig.Target, projectconfig.Calendar, bool) {
	r, _, err := projectRuntimeDB(db, ticket.TenantID, ticket.ProjectConfigVersionID)
	if err != nil || r == nil {
		return projectconfig.Target{}, projectconfig.Calendar{}, false
	}
	return projectTicketTargetForTicket(r, ticket.ProjectKey, ticket.ServiceProfile)
}

func (s *ticketServiceMetricService) upsertMetric(
	db *gorm.DB,
	ticket models.Ticket,
	calendar projectconfig.Calendar,
	spec ticketMetricSpec,
	now time.Time,
) error {
	start := spec.startedAt
	if start.IsZero() {
		start = ticket.CreatedAt
	}
	if start.IsZero() {
		start = now
	}
	targetAt := calendar.AddMinutes(start, max(1, spec.targetMinutes))
	status := ServiceMetricStatusRunning
	var actualAt *time.Time
	var breachedAt *time.Time
	if spec.metricType == ServiceMetricEscalation && spec.actualAt != nil {
		status = ServiceMetricStatusEscalated
	} else if spec.actualAt != nil {
		status = ServiceMetricStatusMet
		actualAt = spec.actualAt
	} else if now.After(targetAt) {
		status = ServiceMetricStatusBreached
		breachedAt = &now
	} else if metricWarningThreshold(start, targetAt, now) {
		status = ServiceMetricStatusWarning
	}

	if spec.metricType == ServiceMetricUpdateCadence {
		return s.upsertUpdateCadence(db, ticket, calendar, spec.targetMinutes, targetAt, actualAt, start, now)
	} else {
		item := models.TicketServiceMetric{
			TenantID:        ticket.TenantID,
			TicketID:        ticket.ID,
			MetricType:      spec.metricType,
			Status:          status,
			StartedAt:       start,
			TargetAt:        &targetAt,
			ActualAt:        actualAt,
			BreachedAt:      breachedAt,
			ConfigVersionID: ticket.ProjectConfigVersionID,
		}
		return s.upsertLatest(db, item, now)
	}
}

func (s *ticketServiceMetricService) upsertLatest(db *gorm.DB, next models.TicketServiceMetric, now time.Time) error {
	var current models.TicketServiceMetric
	err := db.Where(
		"tenant_id = ? AND ticket_id = ? AND metric_type = ?",
		next.TenantID, next.TicketID, next.MetricType,
	).Order("id DESC").First(&current).Error
	if err == nil {
		if current.ActualAt != nil && (next.Status == ServiceMetricStatusMet || next.Status == ServiceMetricStatusEscalated) {
			return nil
		}
		if current.ActualAt == nil {
			return db.Model(&current).Updates(map[string]any{
				"status":            next.Status,
				"target_at":         next.TargetAt,
				"actual_at":         next.ActualAt,
				"breached_at":       next.BreachedAt,
				"config_version_id": next.ConfigVersionID,
				"updated_at":        now,
			}).Error
		}
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	next.UpdatedAt = now
	next.CreatedAt = now
	return db.Create(&next).Error
}

func (s *ticketServiceMetricService) upsertUpdateCadence(
	db *gorm.DB,
	ticket models.Ticket,
	calendar projectconfig.Calendar,
	targetMinutes int,
	targetAt time.Time,
	actualAt *time.Time,
	start time.Time,
	now time.Time,
) error {
	var current models.TicketServiceMetric
	err := db.Where(
		"tenant_id = ? AND ticket_id = ? AND metric_type = ? AND actual_at IS NULL",
		ticket.TenantID, ticket.ID, ServiceMetricUpdateCadence,
	).Order("id DESC").First(&current).Error
	if err == nil {
		if actualAt != nil && actualAt.After(current.StartedAt) {
			if err := db.Model(&current).Updates(map[string]any{
				"status":     ServiceMetricStatusMet,
				"actual_at":  actualAt,
				"updated_at": now,
			}).Error; err != nil {
				return err
			}
			nextStatus := ServiceMetricStatusRunning
			if ticketCaseClosed(ticket) {
				nextStatus = ServiceMetricStatusMet
			}
			nextTargetAt := calendar.AddMinutes(*actualAt, max(1, targetMinutes))
			next := models.TicketServiceMetric{
				TenantID: ticket.TenantID, TicketID: ticket.ID, MetricType: ServiceMetricUpdateCadence,
				Status: nextStatus, StartedAt: *actualAt, TargetAt: &nextTargetAt, ActualAt: s.updateCadenceActual(ticket),
				ConfigVersionID: ticket.ProjectConfigVersionID, CreatedAt: now, UpdatedAt: now,
			}
			return db.Create(&next).Error
		}
		return nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	status := ServiceMetricStatusRunning
	if actualAt != nil {
		status = ServiceMetricStatusMet
	}
	return db.Create(&models.TicketServiceMetric{
		TenantID: ticket.TenantID, TicketID: ticket.ID, MetricType: ServiceMetricUpdateCadence,
		Status: status, StartedAt: start, TargetAt: &targetAt, ActualAt: actualAt,
		ConfigVersionID: ticket.ProjectConfigVersionID, CreatedAt: now, UpdatedAt: now,
	}).Error
}

func (s *ticketServiceMetricService) escalatePriorityRisk(db *gorm.DB, ticket models.Ticket, now time.Time) error {
	priority := ticketEffectivePriority(ticket)
	if priority != "p1" && priority != "p2" {
		return nil
	}
	var atRisk []models.TicketServiceMetric
	if err := db.Where(
		"tenant_id = ? AND ticket_id = ? AND status IN ? AND escalated = ?",
		ticket.TenantID, ticket.ID, []string{ServiceMetricStatusWarning, ServiceMetricStatusBreached}, false,
	).Find(&atRisk).Error; err != nil {
		return err
	}
	if len(atRisk) == 0 {
		return nil
	}
	var existingCount int64
	if err := db.Model(&TicketEscalation{}).
		Where("tenant_id = ? AND ticket_id = ? AND escalation_rule_id = ? AND status IN ?",
			formatID(ticket.TenantID), formatID(ticket.ID), "priority-near-breach",
			[]string{"pending", "accepted"}).
		Count(&existingCount).Error; err != nil {
		return err
	}
	if existingCount > 0 {
		return db.Model(&models.TicketServiceMetric{}).
			Where("tenant_id = ? AND ticket_id = ? AND status IN ? AND escalated = ?",
				ticket.TenantID, ticket.ID, []string{ServiceMetricStatusWarning, ServiceMetricStatusBreached}, false).
			Updates(map[string]any{"escalated": true, "updated_at": now}).Error
	}
	labels := make([]string, 0, len(atRisk))
	for i := range atRisk {
		item := atRisk[i]
		labels = append(labels, serviceMetricLabel(item.MetricType))
	}
	reason := fmt.Sprintf("%s 目标即将或已经违约，自动升级", joinMetricLabels(labels))
	if _, err := EscalationService.EscalateTicket(
		formatID(ticket.ID), formatID(ticket.TenantID), "priority-near-breach",
		0, 1, "sla_violation", reason,
	); err != nil {
		return err
	}
	if err := db.Model(&models.TicketServiceMetric{}).
		Where("tenant_id = ? AND ticket_id = ? AND status IN ? AND escalated = ?",
			ticket.TenantID, ticket.ID, []string{ServiceMetricStatusWarning, ServiceMetricStatusBreached}, false).
		Updates(map[string]any{"escalated": true, "updated_at": now}).Error; err != nil {
		return err
	}
	return nil
}

func joinMetricLabels(labels []string) string {
	if len(labels) == 0 {
		return "服务目标"
	}
	return strings.Join(labels, "、")
}

func (s *ticketServiceMetricService) updateCadenceStart(ticket models.Ticket) time.Time {
	if ticket.LastCustomerUpdateAt != nil {
		return *ticket.LastCustomerUpdateAt
	}
	return ticket.CreatedAt
}

func (s *ticketServiceMetricService) firstEscalationAt(db *gorm.DB, ticket models.Ticket) *time.Time {
	var item TicketEscalation
	if err := db.Where("tenant_id = ? AND ticket_id = ?", formatID(ticket.TenantID), formatID(ticket.ID)).
		Order("escalated_at ASC").First(&item).Error; err != nil {
		return nil
	}
	return &item.EscalatedAt
}

func (s *ticketServiceMetricService) updateCadenceActual(ticket models.Ticket) *time.Time {
	if ticketCaseClosed(ticket) {
		return ticket.ResolvedAt
	}
	return nil
}

func metricWarningThreshold(start, targetAt, now time.Time) bool {
	total := targetAt.Sub(start)
	if total <= 0 {
		return false
	}
	return now.Sub(start) >= total*4/5
}

func firstTime(values ...*time.Time) *time.Time {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func serviceMetricLabel(metricType string) string {
	switch metricType {
	case ServiceMetricAcknowledgement:
		return "确认"
	case ServiceMetricFirstResponse:
		return "首次响应"
	case ServiceMetricUpdateCadence:
		return "更新节奏"
	case ServiceMetricEscalation:
		return "升级"
	case ServiceMetricRestore:
		return "恢复"
	case ServiceMetricResolve:
		return "解决"
	default:
		return metricType
	}
}
