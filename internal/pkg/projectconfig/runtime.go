package projectconfig

import (
	"encoding/json"
	"fmt"
	"net/mail"
	"net/url"
	"slices"
	"strings"
	"time"
)

// Runtime is the company/environment overlay. Projects are service categories;
// targets may override a category using ProjectKey, or use "*" for all of them.
type Runtime struct {
	Timezone     string        `json:"timezone"`
	Locale       string        `json:"locale"`
	Locales      []string      `json:"locales"`
	ServiceScene string        `json:"service_scene"`
	Calendars    []Calendar    `json:"calendars"`
	Targets      []Target      `json:"targets"`
	Channels     []Channel     `json:"channels"`
	Mail         Mail          `json:"mail"`
	Integrations []Integration `json:"integrations"`
	Retention    Retention     `json:"retention"`
	AutoClose    AutoClose     `json:"auto_close"`
}
type Calendar struct {
	Key      string   `json:"key"`
	Timezone string   `json:"timezone"`
	WorkDays []int    `json:"work_days"`
	Start    string   `json:"start"`
	End      string   `json:"end"`
	Holidays []string `json:"holidays"`
}
type Target struct {
	ProjectKey        string `json:"project_key"`
	Profile           string `json:"profile"`
	Priority          string `json:"priority"`
	CalendarKey       string `json:"calendar_key"`
	ResponseMinutes   int    `json:"response_minutes"`
	AssignmentMinutes int    `json:"assignment_minutes"`
	ResolutionMinutes int    `json:"resolution_minutes"`
}
type Channel struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}
type Mail struct {
	Enabled     bool   `json:"enabled"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	PasswordRef string `json:"password_ref"`
	FromAddress string `json:"from_address"`
	FromName    string `json:"from_name"`
	UseTLS      bool   `json:"use_tls"`
	ReplyTo     string `json:"reply_to"`
	RetryPolicy string `json:"retry_policy"`
}
type Integration struct {
	Provider     string `json:"provider"`
	Enabled      bool   `json:"enabled"`
	BaseURL      string `json:"base_url"`
	AppID        string `json:"app_id"`
	SecretRef    string `json:"secret_ref"`
	KeyRef       string `json:"key_ref"`
	MetadataJSON string `json:"metadata_json"`
}
type Retention struct {
	DataRegion       string `json:"data_region"`
	Days             int    `json:"days"`
	ArchiveAfterDays int    `json:"archive_after_days"`
	AutoDelete       bool   `json:"auto_delete"`
	LegalHold        bool   `json:"legal_hold"`
	GDPRRegion       bool   `json:"gdpr_region"`
	CCPARegion       bool   `json:"ccpa_region"`
}
type AutoClose struct {
	Enabled bool `json:"enabled"`
	Days    int  `json:"days"`
}

func validateRuntime(doc Document, add func(string, string, string)) {
	r := doc.Runtime
	bad := func(path, message string) { add("policy", "runtime."+path, message) }
	if _, err := time.LoadLocation(r.Timezone); err != nil {
		bad("timezone", "请选择有效的 IANA 时区")
	}
	allowedLocales := []string{"zh-CN", "en", "en-US", "ar", "ar-SA"}
	if !slices.Contains(r.Locales, r.Locale) {
		bad("locale", "默认语言必须包含在支持语言中")
	}
	for _, locale := range r.Locales {
		if !slices.Contains(allowedLocales, locale) {
			bad("locales", "语言不受当前系统支持")
		}
	}
	refs := map[string]bool{}
	for _, ref := range doc.SecretRefs {
		refs[ref] = true
	}
	checkRef := func(path, ref string) {
		if ref != "" && (!ValidSecretReference(ref) || !refs[ref]) {
			bad(path, "密钥必须使用已声明的 secret://名称 引用")
		}
	}
	calendars := map[string]bool{}
	for i, c := range r.Calendars {
		path := fmt.Sprintf("calendars[%d]", i)
		if strings.TrimSpace(c.Key) == "" || calendars[c.Key] {
			bad(path, "日历标识不能为空或重复")
		}
		calendars[c.Key] = true
		if _, err := time.LoadLocation(c.Timezone); err != nil {
			bad(path, "日历时区无效")
		}
		start, e1 := time.Parse("15:04", c.Start)
		end, e2 := time.Parse("15:04", c.End)
		if c.End == "24:00" {
			end = start.Truncate(24 * time.Hour).Add(24 * time.Hour)
			e2 = nil
		}
		if e1 != nil || e2 != nil || !end.After(start) {
			bad(path, "工作结束时间必须晚于开始时间，使用 HH:MM 格式")
		}
		for _, h := range c.Holidays {
			if _, err := time.Parse("2006-01-02", h); err != nil {
				bad(path, "休息日期必须使用 YYYY-MM-DD")
			}
		}
	}
	projects := map[string]bool{"*": true}
	for _, p := range doc.Projects {
		projects[p.Key] = true
	}
	targets := map[string]bool{}
	for i, t := range r.Targets {
		path := fmt.Sprintf("targets[%d]", i)
		if !projects[t.ProjectKey] || !calendars[t.CalendarKey] {
			bad(path, "服务目标引用的项目或日历不存在")
		}
		key := t.ProjectKey + "/" + t.Priority
		if targets[key] {
			bad(path, "同一项目和优先级只能配置一组服务目标")
		}
		targets[key] = true
		if t.ResponseMinutes+t.AssignmentMinutes+t.ResolutionMinutes == 0 {
			bad(path, "至少填写一项处理时限")
		}
	}
	channels := map[string]bool{}
	for _, c := range r.Channels {
		if _, ok := channels[c.Name]; ok {
			bad("channels", "渠道不能重复")
		}
		channels[c.Name] = c.Enabled
	}
	if r.ServiceScene == "knowledge_support" {
		for _, rule := range doc.Intake.Rules {
			if slices.Contains(rule.RequiredFields, "product_id") || slices.Contains(rule.RequiredFields, "device_id") {
				bad("service_scene", "知识服务不应要求填写产品或设备")
			}
		}
	}
	for _, rule := range doc.Intake.Rules {
		if !channels[rule.Channel] {
			bad("channels", "受理规则使用的渠道必须明确启用")
		}
	}
	m := r.Mail
	if m.ReplyTo != "" {
		if _, err := mail.ParseAddress(m.ReplyTo); err != nil || strings.ContainsAny(m.ReplyTo, "\r\n") {
			bad("mail.reply_to", "回复地址无效")
		}
	}
	if !slices.Contains([]string{"retry_3_10m", "retry_1_5m", "no_retry"}, m.RetryPolicy) {
		bad("mail.retry_policy", "请选择有效的邮件重试策略")
	}
	checkRef("mail.password_ref", m.PasswordRef)
	if m.Enabled {
		if strings.TrimSpace(m.Host) == "" || strings.ContainsAny(m.Host, "/\r\n ") || m.Port < 1 || m.Username == "" || m.PasswordRef == "" {
			bad("mail", "启用发信需填写 SMTP 地址、端口、账号和密钥引用")
		}
		if _, err := mail.ParseAddress(m.FromAddress); err != nil || strings.ContainsAny(m.FromAddress+m.FromName, "\r\n") {
			bad("mail.from_address", "发信地址或名称无效")
		}
		if (doc.Environment == "production" || doc.Environment == "staging") && !m.UseTLS {
			bad("mail.use_tls", "部署环境发信必须启用 TLS")
		}
	}
	providers := map[string]bool{}
	for _, i := range r.Integrations {
		var metadata map[string]any
		if err := json.Unmarshal([]byte(i.MetadataJSON), &metadata); err != nil || metadata == nil {
			bad("integrations.metadata_json", "附加设置必须是 JSON 对象")
		}
		if providers[i.Provider] {
			bad("integrations", "接入服务不能重复")
		}
		providers[i.Provider] = true
		checkRef("integrations.secret_ref", i.SecretRef)
		checkRef("integrations.key_ref", i.KeyRef)
		if i.Enabled {
			u, err := url.Parse(i.BaseURL)
			if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
				bad("integrations", "接入地址必须是无内嵌密码的 HTTP(S) 地址")
			}
		}
	}
	if r.Retention.ArchiveAfterDays >= r.Retention.Days {
		bad("retention.archive_after_days", "归档时间必须早于保存期限")
	}
	if strings.TrimSpace(r.Retention.DataRegion) == "" {
		bad("retention.data_region", "请填写数据区域")
	}
}

// AddMinutes and WorkingMinutes share a calendar, including holidays and DST.
func (c Calendar) AddMinutes(from time.Time, minutes int) time.Time {
	loc, _ := time.LoadLocation(c.Timezone)
	if loc == nil {
		loc = time.UTC
	}
	t := from.In(loc)
	remaining := time.Duration(minutes) * time.Minute
	for remaining > 0 {
		start, end, working := c.bounds(t, loc)
		if !working || !t.Before(end) {
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, loc)
			continue
		}
		if t.Before(start) {
			t = start
		}
		available := end.Sub(t)
		if remaining <= available {
			return t.Add(remaining)
		}
		remaining -= available
		t = end
	}
	return t
}
func (c Calendar) WorkingMinutes(from, to time.Time) int {
	loc, _ := time.LoadLocation(c.Timezone)
	if loc == nil {
		loc = time.UTC
	}
	t := from.In(loc)
	total := time.Duration(0)
	for t.Before(to) {
		start, end, working := c.bounds(t, loc)
		if end.After(to) {
			end = to
		}
		if start.Before(t) {
			start = t
		}
		if working && end.After(start) {
			total += end.Sub(start)
		}
		t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, loc)
	}
	return int(total / time.Minute)
}
func (c Calendar) bounds(t time.Time, loc *time.Location) (time.Time, time.Time, bool) {
	s, _ := time.Parse("15:04", c.Start)
	e, _ := time.Parse("15:04", c.End)
	end := time.Date(t.Year(), t.Month(), t.Day(), e.Hour(), e.Minute(), 0, 0, loc)
	if c.End == "24:00" {
		end = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, loc)
	}
	return time.Date(t.Year(), t.Month(), t.Day(), s.Hour(), s.Minute(), 0, 0, loc), end, slices.Contains(c.WorkDays, int(t.Weekday())) && !slices.Contains(c.Holidays, t.Format("2006-01-02"))
}
