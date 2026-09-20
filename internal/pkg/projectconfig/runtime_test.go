package projectconfig

import (
	"testing"
	"time"
)

func runtimeTestDocument() Document {
	return Document{SchemaVersion: 2, TenantID: 1, Environment: "development", Projects: []Project{{Key: "support", Name: "Support"}}, SecretRefs: []string{}, Runtime: &Runtime{
		Timezone: "UTC", Locale: "zh-CN", Locales: []string{"zh-CN"}, ServiceScene: "knowledge_support",
		Calendars: []Calendar{{Key: "office", Timezone: "UTC", WorkDays: []int{1, 2, 3, 4, 5}, Start: "09:00", End: "18:00", Holidays: []string{}}},
		Targets:   []Target{{ProjectKey: "*", Profile: "standard", Priority: "p2", CalendarKey: "office", ResponseMinutes: 30}}, Channels: []Channel{{Name: "manual", Enabled: true}}, Mail: Mail{RetryPolicy: "retry_3_10m"}, Integrations: []Integration{}, Retention: Retention{DataRegion: "global", Days: 365, ArchiveAfterDays: 180}, AutoClose: AutoClose{Days: 7}}}
}

func TestRuntimeValidation(t *testing.T) {
	d := runtimeTestDocument()
	if r := Validate(d, 1, "development", nil); !r.Valid {
		t.Fatalf("valid document: %+v", r)
	}
	tests := map[string]func(*Document){
		"invalid timezone":              func(d *Document) { d.Runtime.Timezone = "Invalid/City" },
		"empty weekdays":                func(d *Document) { d.Runtime.Calendars[0].WorkDays = []int{} },
		"unknown calendar":              func(d *Document) { d.Runtime.Targets[0].CalendarKey = "missing" },
		"duplicate targets":             func(d *Document) { d.Runtime.Targets = append(d.Runtime.Targets, d.Runtime.Targets[0]) },
		"default locale unavailable":    func(d *Document) { d.Runtime.Locale = "en" },
		"unknown locale":                func(d *Document) { d.Runtime.Locales = []string{"not-supported"} },
		"plaintext secret":              func(d *Document) { d.Runtime.Mail.PasswordRef = "plaintext-password" },
		"undeclared secret":             func(d *Document) { d.Runtime.Mail.PasswordRef = "secret://smtp" },
		"missing mail credentials":      func(d *Document) { d.Runtime.Mail.Enabled = true },
		"retention order":               func(d *Document) { d.Runtime.Retention.ArchiveAfterDays = 365 },
		"negative minutes":              func(d *Document) { d.Runtime.Targets[0].ResponseMinutes = -1 },
		"unusable integration metadata": func(d *Document) { d.Runtime.Integrations = []Integration{{Provider: "test", MetadataJSON: "[]"}} },
		"reversed holiday range":        func(d *Document) { d.Runtime.Calendars[0].Holidays = []string{"2026-10-07..2026-10-01"} },
		"malformed holiday":             func(d *Document) { d.Runtime.Calendars[0].Holidays = []string{"2026/10/01"} },
		"duplicate profile targets": func(d *Document) {
			first := d.Runtime.Targets[0]
			first.Priority = ""
			d.Runtime.Targets = []Target{first, first}
		},
		"unsupported project profile": func(d *Document) { d.Projects[0].Profile = "gold" },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			d := runtimeTestDocument()
			change(&d)
			if Validate(d, 1, "development", nil).Valid {
				t.Fatal("invalid configuration passed")
			}
		})
	}
	d = runtimeTestDocument()
	d.Runtime.Calendars[0].Start = "00:00"
	d.Runtime.Calendars[0].End = "24:00"
	if r := Validate(d, 1, "development", nil); !r.Valid {
		t.Fatalf("24 hour calendar rejected: %+v", r)
	}
	// 时限规则不再要求工单优先级：留空表示该项目所有优先级共用一组时限。
	d = runtimeTestDocument()
	d.Runtime.Targets[0].Priority = ""
	if r := Validate(d, 1, "development", nil); !r.Valid {
		t.Fatalf("priority-free target rejected: %+v", r)
	}
}

func TestRuntimeCalendarBoundaries(t *testing.T) {
	c := runtimeTestDocument().Runtime.Calendars[0]
	parse := func(s string) time.Time {
		v, e := time.Parse(time.RFC3339, s)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	for _, tc := range []struct {
		start   string
		minutes int
		end     string
	}{
		{"2026-09-14T08:00:00Z", 30, "2026-09-14T09:30:00Z"},
		{"2026-09-11T17:30:00Z", 60, "2026-09-14T09:30:00Z"},
		{"2026-09-14T18:00:00Z", 60, "2026-09-15T10:00:00Z"},
	} {
		from, want := parse(tc.start), parse(tc.end)
		if got := c.AddMinutes(from, tc.minutes); !got.Equal(want) {
			t.Fatalf("deadline %s != %s", got, want)
		}
		if got := c.WorkingMinutes(from, want); got != tc.minutes {
			t.Fatalf("elapsed %d != %d", got, tc.minutes)
		}
	}
	c.Holidays = []string{"2026-09-14"}
	if got := c.AddMinutes(parse("2026-09-11T17:30:00Z"), 60); !got.Equal(parse("2026-09-15T09:30:00Z")) {
		t.Fatal(got)
	}
	c = Calendar{Timezone: "America/New_York", WorkDays: []int{0, 1, 2, 3, 4, 5, 6}, Start: "00:00", End: "24:00", Holidays: []string{}}
	from := parse("2026-03-08T05:00:00Z")
	to := parse("2026-03-09T04:00:00Z")
	if got := c.WorkingMinutes(from, to); got != 23*60 {
		t.Fatalf("DST elapsed %d", got)
	}
	if got := c.AddMinutes(from, 23*60); !got.Equal(to) {
		t.Fatalf("DST deadline %s", got)
	}
}

func TestRuntimeCalendarHolidayRangeAndAllDay(t *testing.T) {
	parse := func(s string) time.Time {
		v, e := time.Parse(time.RFC3339, s)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	if got, ok := NormalizeHolidayEntry(" 2026-10-01..2026-10-07 "); !ok || got != "2026-10-01..2026-10-07" {
		t.Fatalf("normalize range %q ok=%v", got, ok)
	}
	if got, ok := NormalizeHolidayEntry("2026-10-01"); !ok || got != "2026-10-01" {
		t.Fatalf("normalize day %q ok=%v", got, ok)
	}
	for _, invalid := range []string{"", "2026-10-07..2026-10-01", "2026/10/01", "2026-10-01.."} {
		if _, ok := NormalizeHolidayEntry(invalid); ok {
			t.Fatalf("invalid holiday accepted: %q", invalid)
		}
	}

	// 长假区间：9/30 17:30 起算 60 分钟，必须跳过 10/1-10/7，落到 10/8 09:30。
	office := Calendar{Timezone: "UTC", WorkDays: []int{0, 1, 2, 3, 4, 5, 6}, Start: "09:00", End: "18:00", Holidays: []string{"2026-10-01..2026-10-07"}}
	if got := office.AddMinutes(parse("2026-09-30T17:30:00Z"), 60); !got.Equal(parse("2026-10-08T09:30:00Z")) {
		t.Fatalf("holiday range deadline %s", got)
	}
	if got := office.WorkingMinutes(parse("2026-09-30T00:00:00Z"), parse("2026-10-08T00:00:00Z")); got != 9*60 {
		t.Fatalf("holiday range elapsed %d", got)
	}

	// 24x7：全天候日历不因为跨零点而少算。
	allDay := Calendar{Timezone: "Asia/Shanghai", WorkDays: []int{0, 1, 2, 3, 4, 5, 6}, Start: "00:00", End: "24:00", Holidays: []string{}}
	if got := allDay.AddMinutes(parse("2026-09-16T23:30:00+08:00"), 60); !got.Equal(parse("2026-09-17T00:30:00+08:00")) {
		t.Fatalf("24x7 deadline %s", got)
	}
	if got := allDay.WorkingMinutes(parse("2026-09-16T00:00:00+08:00"), parse("2026-09-17T00:00:00+08:00")); got != 24*60 {
		t.Fatalf("24x7 elapsed %d", got)
	}
}

func TestRetentionPolicyAsCodeEffectiveDays(t *testing.T) {
	d := runtimeTestDocument()
	d.Runtime.Retention.TicketDays = 730
	d.Runtime.Retention.AttachmentDays = 365
	d.Runtime.Retention.EventDays = 90
	d.Runtime.Retention.AuditDays = 180
	d.Runtime.Retention.MetricDays = 60
	d.Runtime.Retention.LogDays = 30
	d.Runtime.Retention.ReportDays = 120
	d.Runtime.Retention.BackupDays = 45

	if r := Validate(d, 1, "development", nil); !r.Valid {
		t.Fatalf("retention with 8 category days rejected: %+v", r)
	}

	ret := d.Runtime.Retention
	if ret.EffectiveDays("ticket") != 730 {
		t.Fatalf("expected ticket 730, got %d", ret.EffectiveDays("ticket"))
	}
	if ret.EffectiveDays("attachment") != 365 {
		t.Fatalf("expected attachment 365, got %d", ret.EffectiveDays("attachment"))
	}
	if ret.EffectiveDays("event") != 90 {
		t.Fatalf("expected event 90, got %d", ret.EffectiveDays("event"))
	}
	if ret.EffectiveDays("audit") != 180 {
		t.Fatalf("expected audit 180, got %d", ret.EffectiveDays("audit"))
	}
	if ret.EffectiveDays("metric") != 60 {
		t.Fatalf("expected metric 60, got %d", ret.EffectiveDays("metric"))
	}
	if ret.EffectiveDays("log") != 30 {
		t.Fatalf("expected log 30, got %d", ret.EffectiveDays("log"))
	}
	if ret.EffectiveDays("report") != 120 {
		t.Fatalf("expected report 120, got %d", ret.EffectiveDays("report"))
	}
	if ret.EffectiveDays("backup") != 45 {
		t.Fatalf("expected backup 45, got %d", ret.EffectiveDays("backup"))
	}

	// Fallback test
	emptyRet := Retention{Days: 365}
	if emptyRet.EffectiveDays("ticket") != 365 {
		t.Fatalf("expected fallback 365, got %d", emptyRet.EffectiveDays("ticket"))
	}
	if emptyRet.EffectiveDays("unknown") != 365 {
		t.Fatalf("expected fallback 365, got %d", emptyRet.EffectiveDays("unknown"))
	}
}
