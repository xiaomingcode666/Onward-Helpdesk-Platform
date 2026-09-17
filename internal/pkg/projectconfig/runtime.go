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
	// ProjectProfiles 是「服务项目标识 → 服务档次」的派生索引，由配置加载器
	// 从文档的 projects 派生，不参与 JSON 读写，也不进入版本摘要。
	ProjectProfiles map[string]string `json:"-"`
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
	IMAP        *IMAP  `json:"imap,omitempty"`
}
type IMAP struct {
	Enabled     bool   `json:"enabled"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	PasswordRef string `json:"password_ref"`
	ProjectKey  string `json:"project_key"`
	TicketType  string `json:"ticket_type"`
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

// DefaultServiceProfile 是未声明服务档次时的兜底档次。
const DefaultServiceProfile = "standard"

// ServiceProfiles 列出可选的服务档次，顺序即界面展示顺序。
var ServiceProfiles = []string{"standard", "enhanced", "mission_critical"}

// NormalizeServiceProfile 归一化档次名，无法识别时按标准档处理。
func NormalizeServiceProfile(value string) string {
	profile := strings.ToLower(strings.TrimSpace(value))
	for _, item := range ServiceProfiles {
		if profile == item {
			return item
		}
	}
	return DefaultServiceProfile
}

// IsServiceProfile 判断字符串是否是受支持的服务档次。
func IsServiceProfile(value string) bool {
	profile := strings.ToLower(strings.TrimSpace(value))
	for _, item := range ServiceProfiles {
		if profile == item {
			return true
		}
	}
	return false
}

// ProjectProfile 返回服务项目的服务档次；项目不存在或未声明时按标准档处理。
// 索引由配置加载器写入 Runtime.ProjectProfiles。
func (r *Runtime) ProjectProfile(projectKey string) string {
	if r == nil {
		return DefaultServiceProfile
	}
	key := strings.TrimSpace(projectKey)
	if key == "" {
		return DefaultServiceProfile
	}
	if profile, ok := r.ProjectProfiles[key]; ok {
		return NormalizeServiceProfile(profile)
	}
	return DefaultServiceProfile
}

// BuildProjectProfiles 从文档的服务项目列表派生「项目标识 → 档次」索引。
func BuildProjectProfiles(projects []Project) map[string]string {
	if len(projects) == 0 {
		return nil
	}
	index := make(map[string]string, len(projects))
	for _, project := range projects {
		key := strings.TrimSpace(project.Key)
		if key == "" {
			continue
		}
		index[key] = NormalizeServiceProfile(project.Profile)
	}
	return index
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
			if _, ok := NormalizeHolidayEntry(h); !ok {
				bad(path, "休息日期必须使用 YYYY-MM-DD 或 YYYY-MM-DD..YYYY-MM-DD")
			}
		}
	}
	projects := map[string]bool{"*": true}
	for _, p := range doc.Projects {
		projects[p.Key] = true
		if strings.TrimSpace(p.Profile) != "" && !IsServiceProfile(p.Profile) {
			bad("projects", "服务档次只能是 standard、enhanced 或 mission_critical")
		}
	}
	targets := map[string]bool{}
	for i, t := range r.Targets {
		path := fmt.Sprintf("targets[%d]", i)
		if !calendars[t.CalendarKey] {
			bad(path, "时限规则引用的工作日历不存在")
		}
		profile := NormalizeServiceProfile(t.Profile)
		if targets[profile] {
			bad(path, "每个服务档次只能配置一组时限")
		}
		targets[profile] = true
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
	if m.IMAP != nil {
		i := m.IMAP
		checkRef("mail.imap.password_ref", i.PasswordRef)
		if i.Enabled {
			if strings.TrimSpace(i.Host) == "" || strings.ContainsAny(i.Host, "/\r\n ") || i.Port < 1 || i.Port > 65535 || i.PasswordRef == "" {
				bad("mail.imap", "请填写 IMAP 地址、端口和密钥引用")
			}
			if _, err := mail.ParseAddress(i.Username); err != nil || strings.ContainsAny(i.Username, "\r\n") {
				bad("mail.imap.username", "收件账号必须是邮箱地址")
			}
			if !channels["email"] {
				bad("mail.imap", "启用收件前必须启用 email 渠道")
			}
			if i.ProjectKey != "" && !projects[i.ProjectKey] {
				bad("mail.imap.project_key", "服务项目不存在")
			}
		}
	}
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
	return time.Date(t.Year(), t.Month(), t.Day(), s.Hour(), s.Minute(), 0, 0, loc), end, slices.Contains(c.WorkDays, int(t.Weekday())) && !c.IsHoliday(t)
}

// HolidayRangeSeparator 连接节假日起止日期，例如 "2026-10-01..2026-10-07"。
const HolidayRangeSeparator = ".."

// NormalizeHolidayEntry 校验单条节假日配置并返回规范写法。
// 支持单日 "2026-10-01" 和闭区间 "2026-10-01..2026-10-07" 两种写法；
// 返回 ok=false 表示写法非法，调用方应据此报错。
func NormalizeHolidayEntry(value string) (string, bool) {
	entry := strings.TrimSpace(value)
	if entry == "" {
		return "", false
	}
	start, end, hasRange := strings.Cut(entry, HolidayRangeSeparator)
	if !hasRange {
		day, err := time.Parse("2006-01-02", entry)
		if err != nil {
			return "", false
		}
		return day.Format("2006-01-02"), true
	}
	startDay, err := time.Parse("2006-01-02", strings.TrimSpace(start))
	if err != nil {
		return "", false
	}
	endDay, err := time.Parse("2006-01-02", strings.TrimSpace(end))
	if err != nil || endDay.Before(startDay) {
		return "", false
	}
	return startDay.Format("2006-01-02") + HolidayRangeSeparator + endDay.Format("2006-01-02"), true
}

// IsHoliday 判断某一天是否落在日历配置的休息日里（支持区间写法）。
func (c Calendar) IsHoliday(day time.Time) bool {
	target := day.Format("2006-01-02")
	for _, entry := range c.Holidays {
		start, end, hasRange := strings.Cut(strings.TrimSpace(entry), HolidayRangeSeparator)
		if !hasRange {
			if start == target {
				return true
			}
			continue
		}
		// 字符串比较即按日期先后比较，格式固定为 YYYY-MM-DD。
		if target >= start && target <= end {
			return true
		}
	}
	return false
}
