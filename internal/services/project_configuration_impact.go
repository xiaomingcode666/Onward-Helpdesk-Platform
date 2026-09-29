package services

import (
	"time"

	"github.com/mlogclub/simple/sqls"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/projectconfig"
)

// ProjectConfigurationImpactRequest is intentionally small for the first
// version of configuration impact analysis. It is a read-only calculation.
type ProjectConfigurationImpactRequest struct {
	Document projectconfig.Document `json:"document"`
	Limit    int                    `json:"limit"`
}

type ProjectConfigurationImpactItem struct {
	TicketID     int64      `json:"ticket_id"`
	TicketNo     string     `json:"ticket_no"`
	Title        string     `json:"title"`
	Metric       string     `json:"metric"`
	OldDueAt     *time.Time `json:"old_due_at,omitempty"`
	NewDueAt     *time.Time `json:"new_due_at,omitempty"`
	DeltaMinutes int        `json:"delta_minutes"`
	OldBreached  bool       `json:"old_breached"`
	NewBreached  bool       `json:"new_breached"`
	Reason       string     `json:"reason"`
}

type ProjectConfigurationImpactSummary struct {
	Scanned       int `json:"scanned"`
	Affected      int `json:"affected"`
	NewlyAtRisk   int `json:"newly_at_risk"`
	NewlyBreached int `json:"newly_breached"`
	Shortened     int `json:"shortened"`
	Extended      int `json:"extended"`
}

type ProjectConfigurationImpact struct {
	Summary ProjectConfigurationImpactSummary `json:"summary"`
	Items   []ProjectConfigurationImpactItem  `json:"items"`
}

// PreviewProjectConfigurationImpact compares the active runtime document with
// a candidate document for a bounded, recent sample of tickets. It never
// mutates tickets, SLA metrics, or configuration state.
func PreviewProjectConfigurationImpact(tenantID int64, candidate projectconfig.Document, limit int) (*ProjectConfigurationImpact, error) {
	result := &ProjectConfigurationImpact{Items: []ProjectConfigurationImpactItem{}}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if candidate.Runtime == nil {
		return result, nil
	}
	candidate.Runtime.ProjectProfiles = projectconfig.BuildProjectProfiles(candidate.Projects)
	oldRuntime, _, err := projectRuntimeDB(sqls.DB(), tenantID, 0)
	if err != nil {
		return nil, err
	}
	if oldRuntime == nil {
		return result, nil
	}
	var tickets []models.Ticket
	cutoff := time.Now().UTC().AddDate(0, 0, -90)
	if err := sqls.DB().Where("tenant_id = ? AND created_at >= ?", tenantID, cutoff).
		Order("created_at DESC").Limit(limit).Find(&tickets).Error; err != nil {
		return nil, err
	}
	now := time.Now()
	affectedTickets := make(map[int64]struct{})
	for _, ticket := range tickets {
		oldTarget, oldCalendar, oldOK := projectTicketTargetForTicket(oldRuntime, ticket.ProjectKey, ticket.ServiceProfile)
		newTarget, newCalendar, newOK := projectTicketTargetForTicket(candidate.Runtime, ticket.ProjectKey, ticket.ServiceProfile)
		if !oldOK && !newOK {
			continue
		}
		for _, metric := range []struct {
			name       string
			oldMinutes int
			newMinutes int
		}{
			{"first_response", targetMinutes(oldTarget, "first_response"), targetMinutes(newTarget, "first_response")},
			{"acknowledgement", targetMinutes(oldTarget, "acknowledgement"), targetMinutes(newTarget, "acknowledgement")},
			{"resolve", targetMinutes(oldTarget, "resolve"), targetMinutes(newTarget, "resolve")},
		} {
			if metric.oldMinutes <= 0 && metric.newMinutes <= 0 {
				continue
			}
			var oldDue, newDue *time.Time
			if oldOK && metric.oldMinutes > 0 {
				due := oldCalendar.AddMinutes(ticket.CreatedAt, metric.oldMinutes)
				oldDue = &due
			}
			if newOK && metric.newMinutes > 0 {
				due := newCalendar.AddMinutes(ticket.CreatedAt, metric.newMinutes)
				newDue = &due
			}
			if oldDue != nil && newDue != nil && oldDue.Equal(*newDue) {
				continue
			}
			oldBreached := oldDue != nil && ticketMetricActual(ticket, metric.name) == nil && now.After(*oldDue)
			newBreached := newDue != nil && ticketMetricActual(ticket, metric.name) == nil && now.After(*newDue)
			delta := 0
			if oldDue != nil && newDue != nil {
				delta = int(newDue.Sub(*oldDue) / time.Minute)
			}
			reason := "SLA时限或工作日历变化"
			if oldCalendar.Key != newCalendar.Key {
				reason = "工作日历变化"
			} else if metric.oldMinutes != metric.newMinutes {
				reason = "SLA时限变化"
			}
			result.Items = append(result.Items, ProjectConfigurationImpactItem{TicketID: ticket.ID, TicketNo: ticket.TicketNo, Title: ticket.Title, Metric: metric.name, OldDueAt: oldDue, NewDueAt: newDue, DeltaMinutes: delta, OldBreached: oldBreached, NewBreached: newBreached, Reason: reason})
			if _, seen := affectedTickets[ticket.ID]; !seen {
				affectedTickets[ticket.ID] = struct{}{}
				result.Summary.Affected++
			}
			if !oldBreached && newBreached {
				result.Summary.NewlyBreached++
			}
			if delta < 0 {
				result.Summary.Shortened++
			} else if delta > 0 {
				result.Summary.Extended++
			}
		}
	}
	result.Summary.Scanned = len(tickets)
	result.Summary.NewlyAtRisk = result.Summary.NewlyBreached
	return result, nil
}

func targetMinutes(target projectconfig.Target, metric string) int {
	switch metric {
	case "first_response":
		return target.ResponseMinutes
	case "acknowledgement":
		return target.AssignmentMinutes
	default:
		return target.ResolutionMinutes
	}
}

func ticketMetricActual(ticket models.Ticket, metric string) *time.Time {
	switch metric {
	case "first_response":
		return ticket.FirstRespondedAt
	case "acknowledgement":
		return ticket.AcknowledgedAt
	default:
		return ticket.ResolvedAt
	}
}
