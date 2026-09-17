package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/projectconfig"
)

// 三档时限落到工单上：关键服务走 7×24 日历，增强/标准走工作日历。
func TestProjectRuntimeSLAFollowsTicketServiceProfile(t *testing.T) {
	_, op, doc := setupProjectRuntime(t)
	continuous := doc.Runtime.Calendars[0]
	office := projectconfig.Calendar{Key: "office", Timezone: "UTC", WorkDays: []int{1, 2, 3, 4, 5}, Start: "09:00", End: "18:00", Holidays: []string{}}
	doc.Runtime.Calendars = []projectconfig.Calendar{continuous, office}
	doc.Runtime.Targets = []projectconfig.Target{
		{Profile: "standard", CalendarKey: "office", ResponseMinutes: 120, AssignmentMinutes: 240, ResolutionMinutes: 1440},
		{Profile: "enhanced", CalendarKey: "office", ResponseMinutes: 30, AssignmentMinutes: 30, ResolutionMinutes: 480},
		{Profile: "mission_critical", CalendarKey: continuous.Key, ResponseMinutes: 15, AssignmentMinutes: 15, ResolutionMinutes: 240},
	}
	doc.Projects = []projectconfig.Project{
		{Key: "pump", Name: "冷却水泵", Profile: "mission_critical"},
		{Key: "desk", Name: "桌面设备", Profile: "enhanced"},
	}
	report := projectconfig.Validate(doc, doc.TenantID, projectconfig.Environment(), projectconfig.RuntimeSecretCheck)
	require.True(t, report.Valid, "fixture document issues: %+v", report.Issues)
	activateRuntime(t, doc, op)

	// 关键服务：7×24 日历，解决时限 240 分钟按自然时间推进。
	critical := models.Ticket{TenantID: 1, TicketNo: "PROFILE-CRIT", ProjectKey: "pump", Channel: "manual", Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: time.Now().Add(-time.Minute)}}
	require.NoError(t, TicketService.Create(&critical))
	require.NotNil(t, critical.SLADueAt)
	require.WithinDuration(t, critical.CreatedAt.Add(240*time.Minute), *critical.SLADueAt, time.Second)

	// 增强服务：工作日历 09:00-18:00，周一 08:00 建单，480 分钟应落到当天 17:00。
	start := time.Date(2026, time.September, 14, 8, 0, 0, 0, time.UTC)
	enhanced := models.Ticket{TenantID: 1, TicketNo: "PROFILE-ENH", ProjectKey: "desk", Channel: "manual", Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: start}}
	require.NoError(t, TicketService.Create(&enhanced))
	require.NotNil(t, enhanced.SLADueAt)
	require.True(t, enhanced.SLADueAt.Equal(time.Date(2026, time.September, 14, 17, 0, 0, 0, time.UTC)),
		"enhanced deadline = %s", enhanced.SLADueAt.Format(time.RFC3339))

	// 没有服务项目的工单按标准档：1440 分钟 = 周一 540、周二 540、周三再 360 分钟。
	standard := models.Ticket{TenantID: 1, TicketNo: "PROFILE-STD", Channel: "manual", Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: start}}
	require.NoError(t, TicketService.Create(&standard))
	require.NotNil(t, standard.SLADueAt)
	require.True(t, standard.SLADueAt.Equal(time.Date(2026, time.September, 16, 15, 0, 0, 0, time.UTC)),
		"standard deadline = %s", standard.SLADueAt.Format(time.RFC3339))
}
