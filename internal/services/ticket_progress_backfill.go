package services

import (
	"regexp"
	"strings"
	"sync"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/i18nx"

	"gorm.io/gorm"
)

// 历史工单进度只保存了中文文本。它们都由系统按固定模板生成，模板就是当前的简体中文资源，
// 回填时用资源模板反向识别并还原为可翻译文本；识别不了的内容视为用户录入，保持原样。

// legacyProgressTemplateKeys 是需要识别的历史进度模板，按从具体到宽泛排序。
var legacyProgressTemplateKeys = []string{
	"caseProgress.transitionWithReason", "caseProgress.transition",
	"ticketProgress.statusChangedWithRemark", "ticketProgress.statusChanged",
	"ticketProgress.reassigned", "ticketProgress.assigned",
	"ticketProgress.dispatchedAwaitingAccept",
	"supportReadiness.progressWithReason", "supportReadiness.progress",
	"ticketProgress.meetingEndedWithRoom", "ticketProgress.meetingEnded",
	"ticketProgress.meetingScheduled", "ticketProgress.meetingStarted",
	"ticketProgress.closedWithResolution", "ticketProgress.cancelled",
	"ticketProgress.knowledgeCandidateRejectedWithRemark", "ticketProgress.supervisorTakeoverUndispatchable",
	"ticketProgress.supplierEscalated", "ticketProgress.supplierTimeoutEscalated", "ticketProgress.supplierInternalAssigned",
	"ticketProgress.supplierMemberJoined", "ticketProgress.supplierMemberLeft", "ticketProgress.supplierResolved",
}

// legacyProgressExactKeys 是不带参数的系统进度，中文规范值与历史文本逐字相同。
var legacyProgressExactKeys = []string{
	"ticketProgress.created", "ticketProgress.draftCreated", "ticketProgress.invitationAccepted",
	"ticketProgress.conversationRestoredPending", "ticketProgress.returnedToPool.product", "ticketProgress.returnedToPool.support",
	"ticketProgress.autoAssignedWithConversation", "ticketProgress.repairRecorded", "ticketProgress.supplierClosedWithRepair",
	"ticketProgress.accepted", "ticketProgress.takeover.product", "ticketProgress.takeover.support",
	"ticketProgress.takeoverExpiredAcceptance.product", "ticketProgress.takeoverExpiredAcceptance.support",
	"ticketProgress.takeoverUnansweredCustomer.product", "ticketProgress.takeoverUnansweredCustomer.support",
	"ticketProgress.poolSelfAssign.product", "ticketProgress.poolSelfAssign.support",
	"ticketProgress.escalated", "ticketProgress.closed", "ticketProgress.reopened",
	"ticketProgress.teamDisabledReturnedToPool", "ticketProgress.assigneeUnavailableRedispatch", "ticketProgress.acceptTimeoutRedispatch",
	"ticketProgress.repeatedAcceptTimeoutEscalated", "ticketProgress.supplierAuthorizationExpired", "ticketProgress.supplierAccepted",
	"ticketProgress.knowledgeCandidateRejected", "ticketProgress.meetingCreationFailed",
	"caseProgress.public.acknowledged", "caseProgress.public.in_triage", "caseProgress.public.waiting", "caseProgress.public.assigned",
	"caseProgress.public.restored", "caseProgress.public.resolved", "caseProgress.public.closure_pending",
}

// legacyProgressEnglishKeys 是早期以英文写入的系统进度。
var legacyProgressEnglishKeys = map[string]string{
	"Created ticket": "ticketProgress.created",
	"Draft ticket created; awaiting customer invitation acceptance": "ticketProgress.draftCreated",
	"Customer accepted invitation; draft ticket submitted":          "ticketProgress.invitationAccepted",
}

var legacyDurationKeys = []string{"duration.hoursMinutes", "duration.hours", "duration.minutesSeconds", "duration.minutes", "duration.seconds"}

var (
	placeholderPattern       = regexp.MustCompile(`\{([A-Za-z0-9_]+)\}`)
	legacyStatusCodePattern  = regexp.MustCompile(`^[a-z_]+$`)
	legacyNumberPattern      = regexp.MustCompile(`^\d+$`)
	zhTemplateCacheMu        sync.Mutex
	zhTemplateCache          = map[string]*zhTemplate{}
	legacySupportStatusCodes = []string{SupportStatusReady, SupportStatusRestricted, SupportStatusUnknown}
)

type zhTemplate struct {
	pattern *regexp.Regexp
	names   []string
}

// matchZhTemplate 用资源键的简体中文模板匹配文本，返回各占位符对应的原文。
func matchZhTemplate(key, content string) (map[string]string, bool) {
	zhTemplateCacheMu.Lock()
	tpl, ok := zhTemplateCache[key]
	if !ok {
		format := canonicalText(i18nx.NewText(key, nil))
		names := []string{}
		var builder strings.Builder
		builder.WriteString("^")
		last := 0
		for _, loc := range placeholderPattern.FindAllStringSubmatchIndex(format, -1) {
			builder.WriteString(regexp.QuoteMeta(format[last:loc[0]]))
			builder.WriteString(`([\s\S]+?)`)
			names = append(names, format[loc[2]:loc[3]])
			last = loc[1]
		}
		builder.WriteString(regexp.QuoteMeta(format[last:]))
		builder.WriteString("$")
		tpl = &zhTemplate{pattern: regexp.MustCompile(builder.String()), names: names}
		zhTemplateCache[key] = tpl
	}
	zhTemplateCacheMu.Unlock()
	match := tpl.pattern.FindStringSubmatch(content)
	if match == nil {
		return nil, false
	}
	values := make(map[string]string, len(tpl.names))
	for index, name := range tpl.names {
		values[name] = match[index+1]
	}
	return values, true
}

// legacyProgressText 识别历史系统进度；返回 false 表示内容由用户录入或格式未知。
func legacyProgressText(content string) (i18nx.Text, bool) {
	content = strings.TrimSpace(content)
	if content == "" {
		return i18nx.Text{}, false
	}
	if key, ok := legacyProgressEnglishKeys[content]; ok {
		return i18nx.NewText(key, nil), true
	}
	if values, ok := matchZhTemplate("ticketProgress.withReason", content); ok {
		if base, matched := legacyExactProgressText(values["content"]); matched {
			return withReasonText(base, values["reason"]), true
		}
	}
	if text, ok := legacyExactProgressText(content); ok {
		return text, true
	}
	for _, key := range legacyProgressTemplateKeys {
		values, ok := matchZhTemplate(key, content)
		if !ok {
			continue
		}
		if text, ok := legacyTemplateText(key, values); ok {
			return text, true
		}
	}
	return i18nx.Text{}, false
}

// legacyTemplateText 把模板参数还原为可翻译参数：状态码、系统原因和时长按语言渲染，其余保持原文。
func legacyTemplateText(key string, values map[string]string) (i18nx.Text, bool) {
	params := i18nx.Params{}
	for name, value := range values {
		params[name] = value
	}
	switch key {
	case "caseProgress.transition", "caseProgress.transitionWithReason":
		for _, name := range []string{"from", "to"} {
			if !legacyStatusCodePattern.MatchString(values[name]) {
				return i18nx.Text{}, false
			}
			params[name] = caseStatusText(values[name])
		}
		if reason, ok := values["reason"]; ok {
			params["reason"] = legacySystemTextParam(reason, "caseProgress.acceptReason")
		}
	case "ticketProgress.statusChanged", "ticketProgress.statusChangedWithRemark":
		if !legacyStatusCodePattern.MatchString(values["status"]) {
			return i18nx.Text{}, false
		}
		params["status"] = ticketStatusText(enums.TicketStatus(values["status"]))
	case "ticketProgress.dispatchedAwaitingAccept":
		params["reason"] = systemReasonParam(values["reason"])
	case "ticketProgress.assigned", "ticketProgress.reassigned":
		// 历史分配记录把原因追加在处理人之后，按 withReason 模板的分隔文本拆开。
		assignee, reason, hasReason := strings.Cut(values["assignee"], legacyReasonSeparator())
		params["assignee"] = assignee
		text := i18nx.NewText(key, params)
		if hasReason {
			return withReasonText(text, reason), true
		}
		return text, true
	case "supportReadiness.progress", "supportReadiness.progressWithReason":
		status, ok := legacySupportStatus(values["status"])
		if !ok {
			return i18nx.Text{}, false
		}
		params["status"] = supportStatusText(status)
	case "ticketProgress.meetingEnded", "ticketProgress.meetingEndedWithRoom":
		params["duration"] = legacyDurationParam(values["duration"])
	case "ticketProgress.supervisorTakeoverUndispatchable":
		params["reason"] = dispatchFailureReasonText(values["reason"])
	}
	return i18nx.NewText(key, params), true
}

// legacyReasonSeparator 是 ticketProgress.withReason 中文模板里内容与原因之间的分隔文本。
func legacyReasonSeparator() string {
	format := canonicalText(i18nx.NewText("ticketProgress.withReason", nil))
	start := strings.Index(format, "{content}")
	end := strings.Index(format, "{reason}")
	if start < 0 || end <= start {
		return "\x00"
	}
	return format[start+len("{content}") : end]
}

func legacySystemTextParam(value, key string) any {
	text := i18nx.NewText(key, nil)
	if canonicalText(text) == strings.TrimSpace(value) {
		return text
	}
	return value
}

func legacySupportStatus(label string) (string, bool) {
	for _, status := range legacySupportStatusCodes {
		if canonicalText(supportStatusText(status)) == label {
			return status, true
		}
	}
	return "", false
}

func legacyExactProgressText(content string) (i18nx.Text, bool) {
	for _, key := range legacyProgressExactKeys {
		text := i18nx.NewText(key, nil)
		if canonicalText(text) == content {
			return text, true
		}
	}
	return i18nx.Text{}, false
}

// legacyDurationParam 把历史中文时长识别为可翻译文本，无法识别时原样保留。
func legacyDurationParam(value string) any {
	value = strings.TrimSpace(value)
	if text := i18nx.NewText("duration.lessThanSecond", nil); canonicalText(text) == value {
		return text
	}
	for _, key := range legacyDurationKeys {
		values, ok := matchZhTemplate(key, value)
		if !ok {
			continue
		}
		params := i18nx.Params{}
		valid := true
		for name, number := range values {
			if !legacyNumberPattern.MatchString(number) {
				valid = false
				break
			}
			params[name] = number
		}
		if valid {
			return i18nx.NewText(key, params)
		}
	}
	return value
}

// BackfillTicketProgressDisplayTextDB 为历史系统进度、支持原因和优先级说明补充可翻译文本。
func BackfillTicketProgressDisplayTextDB(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if err := db.AutoMigrate(&models.TicketProgress{}, &models.Ticket{}); err != nil {
		return err
	}
	var rows []models.TicketProgress
	if err := db.Select("id", "content").Where("content_text = ''").FindInBatches(&rows, 500, func(tx *gorm.DB, _ int) error {
		for _, row := range rows {
			text, ok := legacyProgressText(row.Content)
			if !ok {
				continue
			}
			if err := db.Model(&models.TicketProgress{}).Where("id = ?", row.ID).Update("content_text", text).Error; err != nil {
				return err
			}
		}
		return nil
	}).Error; err != nil {
		return err
	}
	for _, key := range []string{"ticketPriority.explanation.auto", "ticketPriority.explanation.manual"} {
		text := i18nx.NewText(key, nil)
		if err := db.Model(&models.Ticket{}).Where("priority_explanation = ? AND priority_explanation_text = ''", canonicalText(text)).
			Update("priority_explanation_text", text).Error; err != nil {
			return err
		}
	}
	for _, key := range []string{"supportReadiness.reason.knowledgeNotMapped", "supportReadiness.reason.knowledgeInactive", "supportReadiness.reason.assigneeNotAssigned"} {
		text := i18nx.NewText(key, nil)
		if err := db.Model(&models.Ticket{}).Where("support_reason = ? AND support_reason_text = ''", canonicalText(text)).
			Update("support_reason_text", text).Error; err != nil {
			return err
		}
	}
	return nil
}
