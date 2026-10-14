package services

import (
	"fmt"
	"regexp"
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/i18nx"
)

const (
	NotificationTemplateChannelInApp = "in_app"
	NotificationTemplateChannelEmail = "email"

	NotificationDeliveryAttemptStatusSent    = "sent"
	NotificationDeliveryAttemptStatusFailed  = "failed"
	NotificationDeliveryAttemptStatusBlocked = "blocked"
	NotificationDeliveryAttemptStatusSkipped = "skipped"

	defaultNotificationTemplateLanguage = "zh-CN"

	// NotificationTemplateCodeGeneric 是通用兜底模板，任何通知类型都能用它渲染。
	NotificationTemplateCodeGeneric = "notification_generic"

	// 以下 code 用于工单状态变化时更新已有通知的文案。
	NotificationTemplateCodeTicketCreatedAssigned     = "ticket_created_assigned"
	NotificationTemplateCodeTicketAssignedTransferred = "ticket_assigned_transferred"
	NotificationTemplateCodeTicketAssignedAccepted    = "ticket_assigned_accepted"
	NotificationTemplateCodeTicketAssignedCancelled   = "ticket_assigned_cancelled"
	NotificationTemplateCodeTicketAssignedRecovered   = "ticket_assigned_recovered"

	notificationTemplateMissingPrefix = "通知缺少内置模板"
)

var notificationTemplateVariablePattern = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_]+)\s*\}\}`)

var NotificationTemplateService = newNotificationTemplateService()

func newNotificationTemplateService() *notificationTemplateService {
	return &notificationTemplateService{}
}

type notificationTemplateService struct{}

// ApplyToNotification 使用代码内置模板按接收人语言渲染通知。
// 代码模板随版本发布并通过代码审查，因此无需数据库批准状态或后台配置。
func (s *notificationTemplateService) ApplyToNotification(item *models.Notification, recipientUserID int64, variables map[string]string) error {
	if item == nil || item.TenantID <= 0 {
		return nil
	}
	code := normalizeNotificationTemplateCode(item.TemplateCode)
	if code == "" {
		code = normalizeNotificationTemplateCode(item.NotificationType)
	}
	if code == "" {
		code = NotificationTemplateCodeGeneric
	}
	language := s.RecipientLanguage(item.TenantID, recipientUserID)
	item.Language = language
	tpl := s.ResolveApproved(item.TenantID, code, NotificationTemplateChannelInApp, language)
	if tpl == nil && code != NotificationTemplateCodeGeneric {
		tpl = s.ResolveApproved(item.TenantID, NotificationTemplateCodeGeneric, NotificationTemplateChannelInApp, language)
	}
	if tpl == nil {
		detail := "缺少内置站内信模板：code=" + code + "，语言=" + language
		NotificationDeliveryAttemptService.RecordBlockedWithReason(item, NotificationTemplateChannelInApp, "no_builtin_template", detail)
		return errorsx.InvalidParam(notificationTemplateMissingPrefix + "：" + detail + "，已阻断发送")
	}
	data := notificationTemplateRenderData(item, language)
	for key, value := range renderNotificationParams(item.TemplateParams, language) {
		data[key] = value
	}
	for key, value := range variables {
		if _, isText := item.TemplateParams[strings.TrimSpace(key)].(i18nx.Text); isText {
			continue
		}
		data[strings.TrimSpace(key)] = value
	}
	title := strings.TrimSpace(RenderNotificationTemplateText(tpl.TitleTemplate, data))
	content := strings.TrimSpace(RenderNotificationTemplateText(tpl.ContentTemplate, data))
	if title == "" {
		title = item.Title
	}
	if content == "" {
		content = item.Content
	}
	item.TemplateID = 0
	item.TemplateCode = tpl.Code
	item.Language = tpl.Language
	if finding := ScanNotificationSensitiveContent(title + "\n" + content); finding != nil {
		NotificationDeliveryAttemptService.RecordBlocked(item, NotificationTemplateChannelInApp, finding, tpl.Code, tpl.Language)
		return errorsx.InvalidParam(notificationSensitiveBlockedPrefix + "：" + finding.Label + "，已阻断发送")
	}
	item.Title = title
	item.Content = content
	return nil
}

// ResolveApproved 从代码模板目录选择模板，并按接收人语言回退到默认语言。
// 方法名保留为 ResolveApproved，表示只有随代码发布的模板可用于发送。
func (s *notificationTemplateService) ResolveApproved(_ int64, code, channel, language string) *models.NotificationTemplate {
	seed := resolveNotificationTemplateSeed(code, channel, language)
	if seed == nil {
		return nil
	}
	return &models.NotificationTemplate{
		Code:            seed.Code,
		Name:            seed.Name,
		Channel:         seed.Channel,
		Language:        seed.Language,
		TitleTemplate:   seed.Title,
		ContentTemplate: seed.Body,
		ApprovalStatus:  "approved",
	}
}

func resolveNotificationTemplateSeed(code, channel, language string) *notificationTemplateSeed {
	code = normalizeNotificationTemplateCode(code)
	channel = normalizeNotificationTemplateChannel(channel)
	if code == "" || channel == "" {
		return nil
	}
	language = normalizeNotificationTemplateLanguage(language)
	catalog := defaultNotificationTemplateSeeds()
	for i := range catalog {
		candidate := &catalog[i]
		if candidate.Code == code && candidate.Channel == channel && candidate.Language == language {
			return candidate
		}
	}
	for i := range catalog {
		candidate := &catalog[i]
		if candidate.Code == code && candidate.Channel == channel && candidate.Language == defaultNotificationTemplateLanguage {
			return candidate
		}
	}
	return nil
}

// RecipientLanguage 读取接收人语言，缺省按默认语言处理。
func (s *notificationTemplateService) RecipientLanguage(_ int64, userID int64) string {
	if userID <= 0 {
		return defaultNotificationTemplateLanguage
	}
	user := repositories.UserRepository.Get(sqls.DB(), userID)
	if user == nil {
		return defaultNotificationTemplateLanguage
	}
	return normalizeNotificationTemplateLanguage(user.Locale)
}

func notificationTemplateRenderData(item *models.Notification, language string) map[string]string {
	data := map[string]string{
		"Title":            item.Title,
		"Content":          item.Content,
		"RecipientName":    item.RecipientName,
		"ActionURL":        item.ActionURL,
		"NotificationType": item.NotificationType,
		"Category":         item.Category,
		"Level":            item.Level,
		"Language":         language,
	}
	if item.BizID > 0 {
		data["BizID"] = fmt.Sprintf("%d", item.BizID)
	}
	return data
}

func normalizeNotificationTemplateCode(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeNotificationTemplateChannel(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeNotificationTemplateLanguage(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return defaultNotificationTemplateLanguage
	}
	switch {
	case strings.HasPrefix(strings.ToLower(normalized), "zh"):
		return "zh-CN"
	case strings.HasPrefix(strings.ToLower(normalized), "en"):
		return "en-US"
	case strings.HasPrefix(strings.ToLower(normalized), "es"):
		return "es-ES"
	default:
		return normalized
	}
}

// RenderNotificationTemplateText 用 {{变量}} 占位符渲染模板，未提供的变量保持原样以便排查。
func RenderNotificationTemplateText(text string, data map[string]string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return notificationTemplateVariablePattern.ReplaceAllStringFunc(text, func(match string) string {
		parts := notificationTemplateVariablePattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		key := strings.TrimSpace(parts[1])
		if value, ok := data[key]; ok {
			return value
		}
		if value, ok := data[strings.ToLower(key)]; ok {
			return value
		}
		return match
	})
}

// buildNotificationTemplateParams 保存渲染模板所用的参数：系统文本以 Text 保存，其余值原样保存。
func buildNotificationTemplateParams(req request.CreateNotificationRequest, title, content string) i18nx.Params {
	params := i18nx.Params{"Title": title, "Content": content}
	if !req.TitleText.IsZero() {
		params["Title"] = req.TitleText
	}
	if !req.ContentText.IsZero() {
		params["Content"] = req.ContentText
	}
	for key, value := range req.TemplateVariables {
		params[strings.TrimSpace(key)] = value
	}
	for key, value := range req.TextVariables {
		if !value.IsZero() {
			params[strings.TrimSpace(key)] = value
		}
	}
	return params
}

// renderNotificationParams 按语言把模板参数渲染为字符串。
func renderNotificationParams(params i18nx.Params, language string) map[string]string {
	data := make(map[string]string, len(params))
	for key, value := range params {
		data[key] = i18nx.RenderValue(value, language)
	}
	return data
}

// LocalizedContent 按读者当前语言重新渲染站内信标题和内容；没有模板参数的历史通知返回存储值。
func (s *notificationTemplateService) LocalizedContent(item *models.Notification, locale string) (string, string) {
	if item == nil {
		return "", ""
	}
	if len(item.TemplateParams) == 0 || strings.TrimSpace(item.TemplateCode) == "" {
		return item.Title, item.Content
	}
	tpl := s.ResolveApproved(item.TenantID, item.TemplateCode, NotificationTemplateChannelInApp, locale)
	if tpl == nil {
		return item.Title, item.Content
	}
	data := notificationTemplateRenderData(item, tpl.Language)
	for key, value := range renderNotificationParams(item.TemplateParams, tpl.Language) {
		data[key] = value
	}
	title := strings.TrimSpace(RenderNotificationTemplateText(tpl.TitleTemplate, data))
	content := strings.TrimSpace(RenderNotificationTemplateText(tpl.ContentTemplate, data))
	if title == "" {
		title = item.Title
	}
	if content == "" {
		content = item.Content
	}
	return title, content
}
