package services

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/projectconfig"
	"remotehelpdesk/internal/pkg/ticketpolicy"
)

func RequireProjectRuntimeOperator(op *dto.AuthPrincipal) error {
	if op != nil && (op.HasRole(EnterpriseRoleOwner) || op.HasRole(EnterpriseRoleAdmin) || op.HasPermission(constants.PermissionTenantUpdate.Code)) {
		return nil
	}
	return errorsx.Forbidden("运营配置涉及公司设置和密钥接入，请由公司管理员操作")
}

func projectRuntimeDB(db *gorm.DB, tenantID int64, versionID int64) (*projectconfig.Runtime, int64, error) {
	if tenantID <= 0 {
		return nil, 0, nil
	}
	if !db.Migrator().HasTable(&models.ProjectConfigurationState{}) {
		return nil, 0, nil
	}
	if versionID == 0 {
		state, err := projectState(db, tenantID)
		if err != nil {
			return nil, 0, err
		}
		versionID = state.ActiveVersionID
	}
	if versionID == 0 {
		return nil, 0, nil
	}
	var v models.ProjectConfigurationVersion
	if err := db.Where("id = ? AND tenant_id = ? AND environment = ?", versionID, tenantID, projectconfig.Environment()).First(&v).Error; err != nil {
		return nil, 0, err
	}
	p, err := decodeProjectVersion(v)
	if err != nil {
		return nil, 0, err
	}
	if p.Document.Runtime != nil {
		p.Document.Runtime.ProjectProfiles = projectconfig.BuildProjectProfiles(p.Document.Projects)
	}
	return p.Document.Runtime, versionID, nil
}

// Existing settings screens must not act as a second configuration writer.
func requireLegacyProjectSettingsDB(db *gorm.DB, tenantID int64) error {
	r, _, err := projectRuntimeDB(db, tenantID, 0)
	if err != nil {
		return err
	}
	if r != nil {
		return errorsx.InvalidParam("此设置已由项目配置版本管理，请到企业工单的配置管理中修改草稿、检查并应用")
	}
	return nil
}

// Serialize legacy writes with activation on the same company row. The final
// check must run inside this transaction, not just before preparing a form.
func legacyProjectSettingsWrite(tenantID int64, write func(*gorm.DB) error) error {
	return sqls.DB().Transaction(func(db *gorm.DB) error {
		if err := lockProjectSettingsTenantDB(db, tenantID); err != nil {
			return err
		}
		if err := requireLegacyProjectSettingsDB(db, tenantID); err != nil {
			return err
		}
		return write(db)
	})
}
func lockProjectSettingsTenantDB(db *gorm.DB, tenantID int64) error {
	if !db.Migrator().HasTable(&models.Tenant{}) {
		return nil
	}
	var tenant models.Tenant
	return db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", tenantID).Limit(1).Find(&tenant).Error
}

// Upgrade is a read-only proposal. No historic document, secret or live setting
// is modified. Unmapped legacy credentials deliberately require operator input.
func UpgradeProjectConfiguration(tenantID int64) (*projectconfig.Document, error) {
	view, err := GetProjectConfiguration(tenantID)
	if err != nil {
		return nil, err
	}
	doc := view.Document
	if doc.Runtime != nil {
		return &doc, nil
	}
	db := sqls.DB()
	var tenant models.Tenant
	if err := db.First(&tenant, tenantID).Error; err != nil {
		return nil, err
	}
	r := &projectconfig.Runtime{Timezone: tenant.Timezone, Locale: tenant.DefaultLocale, Locales: []string{}, ServiceScene: tenant.EffectiveServiceScene(), Calendars: []projectconfig.Calendar{}, Targets: []projectconfig.Target{}, Channels: []projectconfig.Channel{}, Integrations: []projectconfig.Integration{}, Retention: projectconfig.Retention{DataRegion: tenant.DataRegion, Days: 365, ArchiveAfterDays: 180}, AutoClose: projectconfig.AutoClose{Enabled: tenant.TicketAutoCloseEnabled, Days: normalizeAutoCloseDays(tenant.TicketAutoCloseDays)}}
	if r.Timezone == "" {
		r.Timezone = "UTC"
	}
	if r.Locale == "" {
		r.Locale = "en-US"
	}
	if r.Retention.DataRegion == "" {
		r.Retention.DataRegion = "global"
	}
	_ = json.Unmarshal([]byte(tenant.SupportedLocalesJSON), &r.Locales)
	if len(r.Locales) == 0 {
		r.Locales = []string{r.Locale}
	}
	if db.Migrator().HasTable(&models.TenantBranding{}) {
		var branding models.TenantBranding
		if err := db.Where("tenant_id = ?", tenantID).Limit(1).Find(&branding).Error; err != nil {
			return nil, err
		}
		if branding.ID > 0 && branding.DefaultLocale != "" {
			r.Locale = branding.DefaultLocale
		}
		found := false
		for _, l := range r.Locales {
			found = found || l == r.Locale
		}
		if !found {
			r.Locales = append(r.Locales, r.Locale)
		}
	}
	if db.Migrator().HasTable(&ServiceCalendar{}) {
		var rows []ServiceCalendar
		if err := db.Where("tenant_id = ?", formatID(tenantID)).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, c := range rows {
			days := []int{}
			_ = json.Unmarshal([]byte(c.WorkDays), &days)
			r.Calendars = append(r.Calendars, projectconfig.Calendar{Key: c.ID, Timezone: c.Timezone, WorkDays: days, Start: c.WorkHoursStart, End: c.WorkHoursEnd, Holidays: []string{}})
		}
	}
	defaultKey := "continuous"
	for {
		found := false
		for _, c := range r.Calendars {
			found = found || c.Key == defaultKey
		}
		if !found {
			break
		}
		defaultKey += "_"
	}
	// Legacy violation checks counted wall-clock minutes without a calendar.
	r.Calendars = append(r.Calendars, projectconfig.Calendar{Key: defaultKey, Timezone: r.Timezone, WorkDays: []int{0, 1, 2, 3, 4, 5, 6}, Start: "00:00", End: "24:00", Holidays: []string{}})
	if db.Migrator().HasTable(&SLAPolicy{}) {
		var rows []SLAPolicy
		if err := db.Where("tenant_id = ? AND status = 'active'", formatID(tenantID)).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, p := range rows {
			calendar := p.CalendarID
			if calendar == "" {
				calendar = defaultKey
			}
			// 旧策略是按优先级存的，落到新的档次模型：p0/p1 视为关键服务、
			// p2 视为增强、其余视为标准；同一档次取更紧的一组时限。
			r.Targets = mergeTargetByProfile(r.Targets, projectconfig.Target{
				ProjectKey: "*", Profile: legacyPriorityProfile(p.Priority), CalendarKey: calendar,
				ResponseMinutes: p.FRTMinutes, AssignmentMinutes: p.AssignmentMinutes, ResolutionMinutes: p.ResolutionMinutes,
			})
		}
	}
	for _, name := range []string{"manual", "phone", "email", "monitoring_alert", "api", "webhook", "whatsapp", "chatbot_handoff"} {
		r.Channels = append(r.Channels, projectconfig.Channel{Name: name, Enabled: name == "manual"})
	}
	mcfg := config.CurrentOrDefault().Email
	r.Mail = projectconfig.Mail{Enabled: mcfg.SMTPHost != "", Host: mcfg.SMTPHost, Port: mcfg.SMTPPort, Username: mcfg.Username, FromAddress: mcfg.FromAddress, FromName: mcfg.FromName, UseTLS: mcfg.UseTLS, RetryPolicy: "retry_3_10m"}
	if db.Migrator().HasTable(&models.TenantMailSetting{}) {
		var m models.TenantMailSetting
		if err := db.Where("tenant_id = ?", tenantID).Limit(1).Find(&m).Error; err != nil {
			return nil, err
		}
		if m.ID > 0 && m.Status == int(enums.StatusOk) {
			resolved := EmailNotificationService.resolveMailConfig(tenantID)
			r.Mail = projectconfig.Mail{Enabled: resolved.SMTPHost != "", Host: resolved.SMTPHost, Port: resolved.SMTPPort, Username: resolved.Username, FromAddress: resolved.FromAddress, FromName: resolved.FromName, UseTLS: resolved.UseTLS, ReplyTo: m.ReplyTo, RetryPolicy: defaultString(m.RetryPolicy, "retry_3_10m")}
			if m.IMAPEnabled {
				r.Mail.IMAP = &projectconfig.IMAP{Enabled: true, Host: m.IMAPHost, Port: m.IMAPPort, Username: m.IMAPUsername, PasswordRef: "secret://imap-password"}
				doc.SecretRefs = append(doc.SecretRefs, r.Mail.IMAP.PasswordRef)
				for i := range r.Channels {
					if r.Channels[i].Name == "email" {
						r.Channels[i].Enabled = true
					}
				}
			}
		}
	}
	if r.Mail.Enabled {
		r.Mail.PasswordRef = "secret://smtp-password"
		doc.SecretRefs = append(doc.SecretRefs, r.Mail.PasswordRef)
	}
	if db.Migrator().HasTable(&models.TenantIntegrationConfig{}) {
		var rows []models.TenantIntegrationConfig
		if err := db.Where("tenant_id = ? AND status = ?", tenantID, enums.StatusOk).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, i := range rows {
			ref := i.AppSecretRef
			if ref != "" && !projectconfig.ValidSecretReference(ref) {
				ref = "secret://" + i.Provider + "-credential"
				doc.SecretRefs = append(doc.SecretRefs, ref)
			}
			keyRef := ""
			if i.AppKey != "" {
				keyRef = "secret://" + i.Provider + "-key"
				doc.SecretRefs = append(doc.SecretRefs, keyRef)
			}
			if ref != "" {
				doc.SecretRefs = append(doc.SecretRefs, ref)
			}
			r.Integrations = append(r.Integrations, projectconfig.Integration{Provider: i.Provider, Enabled: i.Enabled, BaseURL: i.BaseURL, AppID: i.AppID, SecretRef: ref, KeyRef: keyRef, MetadataJSON: defaultString(i.MetadataJSON, "{}")})
		}
	}
	if db.Migrator().HasTable(&models.DataRegionPolicy{}) {
		var policies []models.DataRegionPolicy
		if err := db.Where("tenant_id = ? AND status = 'active'", formatID(tenantID)).Find(&policies).Error; err != nil {
			return nil, err
		}
		if len(policies) > 1 {
			return nil, errorsx.InvalidParam("当前公司有多条保存策略，请先核对为一条公司级策略后再迁移，系统不会自动丢弃其他策略")
		}
		if len(policies) == 1 {
			p := policies[0]
			r.Retention = projectconfig.Retention{DataRegion: p.DataRegion, Days: p.RetentionDays, ArchiveAfterDays: p.ArchiveAfterDays, AutoDelete: p.AutoDeleteEnabled}
			r.Retention.GDPRRegion = p.GDPRRegion
			r.Retention.CCPARegion = p.CCPARegion
		}
	}
	doc.SchemaVersion = 2
	refs := []string{}
	seen := map[string]bool{}
	for _, ref := range doc.SecretRefs {
		if !seen[ref] {
			refs = append(refs, ref)
			seen[ref] = true
		}
	}
	doc.SecretRefs = refs
	doc.Runtime = r
	return &doc, nil
}

func applyProjectRuntimeDB(db *gorm.DB, doc projectconfig.Document, op *dto.AuthPrincipal) error {
	if doc.Runtime == nil {
		r, _, err := projectRuntimeDB(db, doc.TenantID, 0)
		if err != nil {
			return err
		}
		if r != nil {
			return errorsx.InvalidParam("运营设置已纳管，不能降级为仅受理规则的版本；请将历史内容另存到版本 2 草稿")
		}
		return nil
	}
	r := doc.Runtime
	var previous models.Tenant
	if err := db.First(&previous, doc.TenantID).Error; err != nil {
		return err
	}
	locales, _ := json.Marshal(r.Locales)
	if err := db.Model(&models.Tenant{}).Where("id = ?", doc.TenantID).Updates(map[string]any{"timezone": r.Timezone, "default_locale": r.Locale, "supported_locales_json": string(locales), "service_scene": r.ServiceScene, "data_region": r.Retention.DataRegion, "ticket_auto_close_enabled": r.AutoClose.Enabled, "ticket_auto_close_days": r.AutoClose.Days}).Error; err != nil {
		return err
	}
	if err := db.Model(&models.TenantBranding{}).Where("tenant_id = ?", doc.TenantID).Update("default_locale", r.Locale).Error; err != nil {
		return err
	}
	if previous.EffectiveServiceScene() != r.ServiceScene {
		if err := EnsureTenantDefaultIAMRolesDB(db, doc.TenantID, op); err != nil {
			return err
		}
		if _, err := ProductSupportOrganizationService.EnsureTenantTechnicalRepairTeamDB(db, doc.TenantID, op); err != nil {
			return err
		}
		if previous.IsAIEnabled() {
			if _, err := TenantDefaultAIAgentService.EnsureDB(db, doc.TenantID, op); err != nil {
				return err
			}
		}
	}
	// Keep the old screens readable. SLA execution uses the immutable version,
	// while these records are only projections and cannot be edited separately.
	if err := db.Model(&SLAPolicy{}).Where("tenant_id = ?", formatID(doc.TenantID)).Update("status", "inactive").Error; err != nil {
		return err
	}
	calendarIDs := map[string]string{}
	if err := db.Model(&ServiceCalendar{}).Where("tenant_id = ?", formatID(doc.TenantID)).Update("status", "inactive").Error; err != nil {
		return err
	}
	for _, c := range r.Calendars {
		days, _ := json.Marshal(c.WorkDays)
		row := ServiceCalendar{ID: uuid.NewString(), TenantID: formatID(doc.TenantID), Name: c.Key, Timezone: c.Timezone, WorkDays: string(days), WorkHoursStart: c.Start, WorkHoursEnd: c.End, Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
		if err := db.Create(&row).Error; err != nil {
			return err
		}
		calendarIDs[c.Key] = row.ID
	}
	for _, target := range r.Targets {
		p := SLAPolicy{ID: uuid.NewString(), TenantID: formatID(doc.TenantID), Name: serviceProfileLabel(target.Profile), Priority: legacyPriorityCodeForProfile(target.Profile), FRTMinutes: target.ResponseMinutes, AssignmentMinutes: target.AssignmentMinutes, ResolutionMinutes: target.ResolutionMinutes, Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
		p.CalendarID = calendarIDs[target.CalendarKey]
		if err := db.Create(&p).Error; err != nil {
			return err
		}
	}
	if err := db.Model(&models.DataRegionPolicy{}).Where("tenant_id = ?", formatID(doc.TenantID)).Update("status", "inactive").Error; err != nil {
		return err
	}
	p := models.DataRegionPolicy{ID: uuid.NewString(), TenantID: formatID(doc.TenantID), DataRegion: r.Retention.DataRegion, RetentionDays: r.Retention.Days, ArchiveAfterDays: r.Retention.ArchiveAfterDays, AutoDeleteEnabled: r.Retention.AutoDelete && !r.Retention.LegalHold, Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	p.GDPRRegion = r.Retention.GDPRRegion
	p.CCPARegion = r.Retention.CCPARegion
	if err := db.Create(&p).Error; err != nil {
		return err
	}
	if err := db.Model(&models.TenantIntegrationConfig{}).Where("tenant_id = ?", doc.TenantID).Update("enabled", false).Error; err != nil {
		return err
	}
	for _, i := range r.Integrations {
		var row models.TenantIntegrationConfig
		if err := db.Where("tenant_id = ? AND provider = ?", doc.TenantID, i.Provider).Limit(1).Find(&row).Error; err != nil {
			return err
		}
		if row.ID == 0 {
			row.TenantID = doc.TenantID
			row.Provider = i.Provider
		}
		row.Enabled = i.Enabled
		row.BaseURL = i.BaseURL
		row.AppID = i.AppID
		row.AppSecretRef = i.SecretRef
		row.AppKey = i.KeyRef
		row.MetadataJSON = i.MetadataJSON
		row.AppSecretFingerprint = ""
		row.Status = enums.StatusOk
		if err := db.Save(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func resolveProjectMailConfig(tenantID int64) (*config.EmailConfig, error) {
	r, _, err := projectRuntimeDB(sqls.DB(), tenantID, 0)
	if err != nil || r == nil {
		return nil, err
	}
	m := r.Mail
	if !m.Enabled {
		return nil, fmt.Errorf("当前项目配置未启用邮件发送")
	}
	secret, err := projectconfig.ReadSecret(os.Getenv("RHD_PROJECT_SECRET_DIR"), tenantID, projectconfig.Environment(), m.PasswordRef)
	if err != nil {
		return nil, fmt.Errorf("邮件密钥不可读取")
	}
	return &config.EmailConfig{SMTPHost: m.Host, SMTPPort: m.Port, Username: m.Username, Password: strings.TrimSpace(string(secret)), FromAddress: m.FromAddress, FromName: m.FromName, UseTLS: m.UseTLS, ReplyTo: m.ReplyTo}, nil
}

func prepareProjectRuntimeTicketDB(db *gorm.DB, ticket *models.Ticket) error {
	r, id, err := projectRuntimeDB(db, ticket.TenantID, ticket.IntakeConfigVersionID)
	if err != nil {
		return err
	}
	if r == nil {
		return nil
	}
	channel := ticket.Channel
	switch channel {
	case "", "enterprise", "dashboard", "web", "im", "widget":
		channel = "manual"
	}
	enabled := false
	for _, c := range r.Channels {
		if c.Name == channel {
			enabled = c.Enabled
		}
	}
	if !enabled {
		return errorsx.InvalidParam("当前项目配置未启用此工单渠道")
	}
	ticket.ProjectConfigVersionID = id
	if ticket.CreatedAt.IsZero() {
		ticket.CreatedAt = time.Now()
	}
	if target, calendar, ok := projectTicketTarget(r, ticket.ProjectKey); ok && target.ResolutionMinutes > 0 {
		deadline := calendar.AddMinutes(ticket.CreatedAt, target.ResolutionMinutes)
		ticket.SLADueAt = &deadline
	}
	return nil
}

// projectTicketTarget 按工单所属服务项目的服务档次挑选时限规则。
// 项目未声明档次、或该档次没有配置时，回退标准档，保证老配置仍然生效。
func projectTicketTarget(r *projectconfig.Runtime, project string) (projectconfig.Target, projectconfig.Calendar, bool) {
	if r == nil {
		return projectconfig.Target{}, projectconfig.Calendar{}, false
	}
	return projectTargetForProfile(r, r.ProjectProfile(project))
}

func projectTargetForProfile(r *projectconfig.Runtime, profile string) (projectconfig.Target, projectconfig.Calendar, bool) {
	if r == nil {
		return projectconfig.Target{}, projectconfig.Calendar{}, false
	}
	wanted := projectconfig.NormalizeServiceProfile(profile)
	var fallback *projectconfig.Target
	for i := range r.Targets {
		candidate := &r.Targets[i]
		candidateProfile := projectconfig.NormalizeServiceProfile(candidate.Profile)
		if candidateProfile == wanted {
			if target, calendar, ok := targetWithCalendar(r, candidate); ok {
				return target, calendar, true
			}
			continue
		}
		if candidateProfile == projectconfig.DefaultServiceProfile && fallback == nil {
			fallback = candidate
		}
	}
	if fallback != nil {
		if target, calendar, ok := targetWithCalendar(r, fallback); ok {
			return target, calendar, true
		}
	}
	return projectconfig.Target{}, projectconfig.Calendar{}, false
}

func targetWithCalendar(r *projectconfig.Runtime, target *projectconfig.Target) (projectconfig.Target, projectconfig.Calendar, bool) {
	for _, c := range r.Calendars {
		if c.Key == target.CalendarKey {
			return *target, c, true
		}
	}
	return projectconfig.Target{}, projectconfig.Calendar{}, false
}

// legacyPriorityProfile 把旧 SLA 策略的优先级折算成服务档次。
// 旧表存的是兼容键 p0-p4，比业务优先级 P1-P4 整体低一档（ticketpolicy.LegacyCode），
// 折算规则：业务 P1/P2 → 关键服务，P3 → 增强，P4 → 标准。
func legacyPriorityProfile(priority string) string {
	switch ticketpolicy.FromLegacy(strings.ToLower(strings.TrimSpace(priority))) {
	case "p1", "p2":
		return "mission_critical"
	case "p3":
		return "enhanced"
	default:
		return projectconfig.DefaultServiceProfile
	}
}

// legacyPriorityCodeForProfile 把服务档次折回旧表的兼容优先级键，供旧界面展示。
func legacyPriorityCodeForProfile(profile string) string {
	switch projectconfig.NormalizeServiceProfile(profile) {
	case "mission_critical":
		return ticketpolicy.LegacyCode("p1")
	case "enhanced":
		return ticketpolicy.LegacyCode("p3")
	default:
		return ticketpolicy.LegacyCode("p4")
	}
}

func serviceProfileLabel(profile string) string {
	switch projectconfig.NormalizeServiceProfile(profile) {
	case "mission_critical":
		return "关键服务"
	case "enhanced":
		return "增强"
	default:
		return "标准"
	}
}

// mergeTargetByProfile 按档次合并：同一档次只保留一组时限，取更紧的那个值。
func mergeTargetByProfile(targets []projectconfig.Target, candidate projectconfig.Target) []projectconfig.Target {
	profile := projectconfig.NormalizeServiceProfile(candidate.Profile)
	for i := range targets {
		if projectconfig.NormalizeServiceProfile(targets[i].Profile) != profile {
			continue
		}
		targets[i].ResponseMinutes = tighterMinutes(targets[i].ResponseMinutes, candidate.ResponseMinutes)
		targets[i].AssignmentMinutes = tighterMinutes(targets[i].AssignmentMinutes, candidate.AssignmentMinutes)
		targets[i].ResolutionMinutes = tighterMinutes(targets[i].ResolutionMinutes, candidate.ResolutionMinutes)
		return targets
	}
	return append(targets, candidate)
}

func tighterMinutes(current, candidate int) int {
	switch {
	case candidate <= 0:
		return current
	case current <= 0:
		return candidate
	case candidate < current:
		return candidate
	default:
		return current
	}
}

// A global cleanup must never override a managed company's retention or hold.
func projectRetentionManagedTenantIDs(db *gorm.DB) ([]int64, error) {
	ids := []int64{}
	if !db.Migrator().HasTable(&models.ProjectConfigurationState{}) {
		return ids, nil
	}
	var states []models.ProjectConfigurationState
	if err := db.Where("environment = ? AND active_version_id > 0", projectconfig.Environment()).Find(&states).Error; err != nil {
		return nil, err
	}
	for _, s := range states {
		r, _, err := projectRuntimeDB(db, s.TenantID, s.ActiveVersionID)
		if err != nil {
			return nil, err
		}
		if r != nil {
			ids = append(ids, s.TenantID)
		}
	}
	return ids, nil
}
