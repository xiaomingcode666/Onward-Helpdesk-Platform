package services

import (
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"remotehelpdesk/internal/models"
	"time"
)

type projectSLAWork struct {
	ticket models.Ticket
	policy SLAPolicy
}

func projectSLAWorkItems() ([]projectSLAWork, error) {
	var tickets []models.Ticket
	if err := sqls.DB().Where("project_config_version_id > 0").Where(ticketCaseOpenSQL).Find(&tickets).Error; err != nil {
		return nil, err
	}
	items := []projectSLAWork{}
	for _, ticket := range tickets {
		r, _, err := projectRuntimeDB(sqls.DB(), ticket.TenantID, ticket.ProjectConfigVersionID)
		if err != nil {
			return nil, err
		}
		if r == nil {
			continue
		}
		t, c, ok := projectTicketTarget(r, ticket.ProjectKey)
		if !ok {
			continue
		}
		items = append(items, projectSLAWork{ticket, SLAPolicy{TenantID: formatID(ticket.TenantID), Priority: t.Priority, FRTMinutes: t.ResponseMinutes, AssignmentMinutes: t.AssignmentMinutes, ResolutionMinutes: t.ResolutionMinutes, runtimeCalendar: &c}})
	}
	return items, nil
}
func (p SLAPolicy) deadline(start time.Time, minutes int, paused time.Duration) time.Time {
	if p.runtimeCalendar != nil {
		return p.runtimeCalendar.AddMinutes(start, minutes+int(paused/time.Minute))
	}
	return start.Add(time.Duration(minutes) * time.Minute).Add(paused)
}
func (p SLAPolicy) elapsed(start, end time.Time, paused time.Duration) int {
	if p.runtimeCalendar != nil {
		return max(0, p.runtimeCalendar.WorkingMinutes(start, end)-int(paused/time.Minute))
	}
	return effectiveSLAMinutes(start, end, paused)
}
func (p SLAPolicy) paused(ticket models.Ticket, now time.Time) (time.Duration, error) {
	return p.pausedDB(sqls.DB(), ticket, now)
}
func (p SLAPolicy) pausedDB(db *gorm.DB, ticket models.Ticket, now time.Time) (time.Duration, error) {
	var records []SLAPauseRecord
	if err := db.Where("tenant_id = ? AND ticket_id = ?", formatID(ticket.TenantID), formatID(ticket.ID)).Order("paused_at ASC").Find(&records).Error; err != nil {
		return 0, err
	}
	if p.runtimeCalendar == nil {
		// Use the caller's transaction; borrowing a global connection here can
		// deadlock a saturated pool. Keep legacy stored-duration semantics.
		var seconds int64
		for _, r := range records {
			seconds += r.Duration
			if r.ResumedAt == nil && now.After(r.PausedAt) {
				seconds += int64(now.Sub(r.PausedAt) / time.Second)
			}
		}
		return time.Duration(seconds) * time.Second, nil
	}
	minutes := 0
	lastEnd := ticket.CreatedAt
	for _, r := range records {
		end := now
		if r.ResumedAt != nil && r.ResumedAt.Before(end) {
			end = *r.ResumedAt
		}
		start := r.PausedAt
		if start.Before(lastEnd) {
			start = lastEnd
		}
		if end.After(start) {
			minutes += p.runtimeCalendar.WorkingMinutes(start, end)
			lastEnd = end
		}
	}
	return time.Duration(minutes) * time.Minute, nil
}
