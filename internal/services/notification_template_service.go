package services

import (
	"fmt"
	"regexp"
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
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
	for key, value := range variables {
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
