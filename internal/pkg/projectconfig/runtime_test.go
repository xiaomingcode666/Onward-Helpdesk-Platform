package projectconfig

import (
	"testing"
	"time"
)

func runtimeTestDocument() Document {
	return Document{SchemaVersion: 2, TenantID: 1, Environment: "development", Projects: []Project{{Key: "support", Name: "Support"}}, Intake: IntakePolicy{Rules: []Rule{}}, SecretRefs: []string{}, Runtime: &Runtime{
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
		"invalid timezone":           func(d *Document) { d.Runtime.Timezone = "Invalid/City" },
		"empty weekdays":             func(d *Document) { d.Runtime.Calendars[0].WorkDays = []int{} },
		"unknown calendar":           func(d *Document) { d.Runtime.Targets[0].CalendarKey = "missing" },
		"duplicate targets":          func(d *Document) { d.Runtime.Targets = append(d.Runtime.Targets, d.Runtime.Targets[0]) },
		"default locale unavailable": func(d *Document) { d.Runtime.Locale = "en" },
		"unknown locale":             func(d *Document) { d.Runtime.Locales = []string{"not-supported"} },
		"inconsistent channel": func(d *Document) {
			d.Intake.Rules = []Rule{{ProjectKey: "support", Channel: "phone", TicketType: "incident", RequiredFields: []string{}}}
		},
		"plaintext secret":              func(d *Document) { d.Runtime.Mail.PasswordRef = "plaintext-password" },
		"undeclared secret":             func(d *Document) { d.Runtime.Mail.PasswordRef = "secret://smtp" },
		"missing mail credentials":      func(d *Document) { d.Runtime.Mail.Enabled = true },
		"retention order":               func(d *Document) { d.Runtime.Retention.ArchiveAfterDays = 365 },
		"negative minutes":              func(d *Document) { d.Runtime.Targets[0].ResponseMinutes = -1 },
		"unusable integration metadata": func(d *Document) { d.Runtime.Integrations = []Integration{{Provider: "test", MetadataJSON: "[]"}} },
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
