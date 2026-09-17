package services

import (
	"testing"

	"remotehelpdesk/internal/pkg/projectconfig"
)

func TestProjectTicketTargetFollowsServiceProfile(t *testing.T) {
	office := projectconfig.Calendar{Key: "office", Timezone: "Asia/Shanghai", WorkDays: []int{1, 2, 3, 4, 5}, Start: "09:00", End: "18:00", Holidays: []string{}}
	allday := projectconfig.Calendar{Key: "allday", Timezone: "Asia/Shanghai", WorkDays: []int{0, 1, 2, 3, 4, 5, 6}, Start: "00:00", End: "24:00", Holidays: []string{}}
	standard := projectconfig.Target{Profile: "standard", CalendarKey: "office", ResponseMinutes: 120, AssignmentMinutes: 240, ResolutionMinutes: 1440}
	enhanced := projectconfig.Target{Profile: "enhanced", CalendarKey: "office", ResponseMinutes: 30, AssignmentMinutes: 30, ResolutionMinutes: 480}
	critical := projectconfig.Target{Profile: "mission_critical", CalendarKey: "allday", ResponseMinutes: 15, AssignmentMinutes: 15, ResolutionMinutes: 240}
	runtime := &projectconfig.Runtime{
		Calendars:       []projectconfig.Calendar{office, allday},
		Targets:         []projectconfig.Target{standard, enhanced, critical},
		ProjectProfiles: map[string]string{"pump": "mission_critical", "printer": "enhanced", "desk": "standard"},
	}

	// 关键服务项目用 7×24 日历。
	target, calendar, ok := projectTicketTarget(runtime, "pump")
	if !ok || target.ResolutionMinutes != 240 || calendar.Key != "allday" {
		t.Fatalf("mission critical target = %+v calendar=%s ok=%v", target, calendar.Key, ok)
	}
	// 增强服务项目用工作日历。
	if target, calendar, ok = projectTicketTarget(runtime, "printer"); !ok || target.ResolutionMinutes != 480 || calendar.Key != "office" {
		t.Fatalf("enhanced target = %+v calendar=%s ok=%v", target, calendar.Key, ok)
	}
	// 标准服务项目。
	if target, _, ok = projectTicketTarget(runtime, "desk"); !ok || target.ResolutionMinutes != 1440 {
		t.Fatalf("standard target = %+v ok=%v", target, ok)
	}
	// 没声明档次的项目、以及没有项目的工单，都按标准档。
	for _, project := range []string{"unknown", ""} {
		if target, _, ok = projectTicketTarget(runtime, project); !ok || target.ResolutionMinutes != 1440 {
			t.Fatalf("fallback target for %q = %+v ok=%v", project, target, ok)
		}
	}
	// 该档次没有配置时限规则时回退标准档。
	onlyStandard := &projectconfig.Runtime{Calendars: []projectconfig.Calendar{office}, Targets: []projectconfig.Target{standard}, ProjectProfiles: map[string]string{"pump": "mission_critical"}}
	if target, _, ok = projectTicketTarget(onlyStandard, "pump"); !ok || target.ResolutionMinutes != 1440 {
		t.Fatalf("profile fallback = %+v ok=%v", target, ok)
	}
	// 规则引用的日历不存在时不命中。
	missing := &projectconfig.Runtime{Calendars: []projectconfig.Calendar{}, Targets: []projectconfig.Target{critical}, ProjectProfiles: map[string]string{"pump": "mission_critical"}}
	if _, _, ok := projectTicketTarget(missing, "pump"); ok {
		t.Fatal("missing calendar must not match")
	}
}

// 旧 SLA 策略表存的是兼容键 p0-p4，比业务优先级 P1-P4 整体低一档。
func TestLegacyPriorityProfileMapping(t *testing.T) {
	for legacy, want := range map[string]string{
		"p0": "mission_critical", // 业务 P1
		"p1": "mission_critical", // 业务 P2
		"p2": "enhanced",         // 业务 P3
		"p3": "standard",         // 业务 P4
		"p4": "standard",         // 越界的历史值，按最低档处理
		"":   "standard",
	} {
		if got := legacyPriorityProfile(legacy); got != want {
			t.Fatalf("legacy %q -> %q, want %q", legacy, got, want)
		}
	}
	for profile, want := range map[string]string{
		"mission_critical": "p0",
		"enhanced":         "p2",
		"standard":         "p3",
	} {
		if got := legacyPriorityCodeForProfile(profile); got != want {
			t.Fatalf("profile %q -> %q, want %q", profile, got, want)
		}
	}
}
