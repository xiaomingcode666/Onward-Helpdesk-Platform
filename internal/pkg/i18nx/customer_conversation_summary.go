package i18nx

import (
	"regexp"
	"strings"
)

var (
	conversationSummaryFeedbackPattern = regexp.MustCompile(`^(?:客户已提交服务评价|Customer submitted (?:service feedback|a service rating)|El cliente envio una valoracion del servicio)[:：]\s*([1-5])/5(?:\s*·\s*(.+))?$`)
	conversationSummaryTicketPattern   = regexp.MustCompile(`^已生成服务工单\s+([^，,\s]+)`)
	conversationSummaryRoomPattern     = regexp.MustCompile(`会议室[:：]\s*([^，,]+)`)
	conversationSummaryDurationPattern = regexp.MustCompile(`时长[:：]\s*(.+)$`)
)

var conversationSummaryLocales = []string{LocaleZhCN, LocaleEnUS, LocaleEsES}

// LocalizeCustomerConversationSummary translates only platform-generated summary templates.
// Customer, engineer, supplier, and AI-authored business content remains unchanged.
func LocalizeCustomerConversationSummary(locale, summary string, hasDeviceConcept bool) string {
	value := strings.TrimSpace(summary)
	if value == "" {
		return summary
	}

	switch {
	case matchesConversationSummaryCopy(value, "conversation.welcome.knowledgeSupport"):
		return Getf(locale, "conversation.welcome.knowledgeSupport")
	case matchesConversationSummaryCopy(value, "conversation.welcome.generalService"):
		return Getf(locale, "conversation.welcome.generalService")
	case matchesConversationSummaryCopy(value, "conversation.handoff.waiting"):
		return Getf(locale, "conversation.handoff.waiting")
	case matchesConversationSummaryCopy(value, "conversation.handoff.customerRequested"):
		return Getf(locale, "conversation.handoff.customerRequested")
	case matchesConversationSummaryCopy(value, "conversation.handoff.offHours"):
		return Getf(locale, "conversation.handoff.offHours")
	case isConversationSummaryOffHoursQueued(value):
		if hasDeviceConcept {
			return Getf(locale, "conversation.summary.handoff.offHoursQueuedProduct")
		}
		return Getf(locale, "conversation.handoff.offHoursQueued")
	case matchesConversationSummaryCopy(value, "conversation.handoff.aiHold") || isConversationSummaryAIHold(value):
		if hasDeviceConcept {
			return Getf(locale, "conversation.handoff.aiHold")
		}
		return Getf(locale, "conversation.summary.handoff.aiHoldSupport")
	}

	if match := conversationSummaryFeedbackPattern.FindStringSubmatch(value); len(match) > 0 {
		localized := Getf(locale, "conversation.summary.feedbackSubmitted", match[1])
		if len(match) > 2 && strings.TrimSpace(match[2]) != "" {
			return localized + " · " + strings.TrimSpace(match[2])
		}
		return localized
	}

	if match := conversationSummaryTicketPattern.FindStringSubmatch(value); len(match) > 0 {
		return Getf(locale, "conversation.summary.ticketCreatedWithTicketNo", match[1])
	}
	if strings.HasPrefix(value, "已生成服务工单") {
		return Getf(locale, "conversation.summary.ticketCreated")
	}

	if detail, ok := trimConversationSummaryPrefix(value, "已邀请供应商协作：", "已邀请供应商协作:"); ok {
		parts := strings.FieldsFunc(detail, func(r rune) bool {
			return r == '·' || r == '/' || r == '／'
		})
		if len(parts) == 0 {
			return summary
		}
		company := strings.TrimSpace(parts[0])
		if len(parts) > 1 {
			modules := make([]string, 0, len(parts)-1)
			for _, part := range parts[1:] {
				if trimmed := strings.TrimSpace(part); trimmed != "" {
					modules = append(modules, trimmed)
				}
			}
			if len(modules) > 0 {
				return Getf(locale, "conversation.summary.supplierInvitedWithModule", company, strings.Join(modules, " · "))
			}
		}
		return Getf(locale, "conversation.summary.supplierInvitedWithCompany", company)
	}

	if strings.HasSuffix(value, "已加入协作会话") {
		company := strings.TrimSpace(strings.TrimSuffix(value, "已加入协作会话"))
		if company != "" {
			return Getf(locale, "conversation.summary.supplierJoinedWithCompany", company)
		}
	}
	if resolution, ok := trimConversationSummaryPrefix(value, "供应商处理完成：", "供应商处理完成:"); ok {
		return Getf(locale, "conversation.summary.supplierResolvedWithResolution", resolution)
	}

	roomName := conversationSummaryMatch(value, conversationSummaryRoomPattern)
	duration := localizeConversationSummaryDuration(locale, conversationSummaryMatch(value, conversationSummaryDurationPattern))
	if strings.HasPrefix(value, "视频协作已结束") {
		if roomName != "" && duration != "" {
			return Getf(locale, "conversation.summary.videoEndedWithRoomDuration", roomName, duration)
		}
		if duration != "" {
			return Getf(locale, "conversation.summary.videoEndedWithDuration", duration)
		}
		return Getf(locale, "conversation.summary.videoEnded")
	}
	if strings.HasPrefix(value, "发起视频协作") {
		if roomName != "" {
			return Getf(locale, "conversation.summary.videoStartedWithRoom", roomName)
		}
		return Getf(locale, "conversation.summary.videoStarted")
	}
	if strings.HasPrefix(value, "工程师已发起视频协作") {
		return Getf(locale, "conversation.summary.engineerVideoStarted")
	}
	if strings.HasPrefix(value, "工程师已预定视频协作") {
		return Getf(locale, "conversation.summary.engineerVideoScheduled")
	}

	for source, key := range map[string]string{
		"[图片]": "conversation.summary.image",
		"[语音]": "conversation.summary.audio",
		"[附件]": "conversation.summary.attachment",
	} {
		if value == source {
			return Getf(locale, key)
		}
		if strings.HasPrefix(value, source+" ") {
			return Getf(locale, key) + " " + strings.TrimSpace(strings.TrimPrefix(value, source))
		}
	}

	if value == "维修结论已提交，等待客户确认设备状态。" || value == "维修结论已提交，等待客户确认服务结果。" {
		key := "conversation.summary.repairSubmittedWaiting"
		if !hasDeviceConcept {
			key = "conversation.summary.repairSubmittedWaitingGeneral"
		}
		return Getf(locale, key)
	}
	if result, ok := trimConversationSummaryPrefix(value, "维修结论已提交：", "维修结论已提交:"); ok {
		return Getf(locale, "conversation.summary.repairSubmittedWithSummary", result)
	}
	if value == "该消息已撤回" || matchesConversationSummaryCopy(value, "conversation.summary.recalled") {
		return Getf(locale, "conversation.summary.recalled")
	}

	return summary
}

func matchesConversationSummaryCopy(value, key string) bool {
	for _, locale := range conversationSummaryLocales {
		if value == Getf(locale, key) {
			return true
		}
	}
	return false
}

func isConversationSummaryOffHoursQueued(value string) bool {
	if matchesConversationSummaryCopy(value, "conversation.handoff.offHoursQueued") ||
		matchesConversationSummaryCopy(value, "conversation.summary.handoff.offHoursQueuedProduct") {
		return true
	}
	return value == "你的请求已进入产品维修组待认领，组内工程师可以查看并接单；若当前无人值守，排班恢复后会继续处理。" ||
		value == "Your request is now visible to the product repair team for pickup. If nobody is on duty, it will be handled when the next shift starts."
}

func isConversationSummaryAIHold(value string) bool {
	return matchesConversationSummaryCopy(value, "conversation.summary.handoff.aiHoldSupport")
}

func trimConversationSummaryPrefix(value string, prefixes ...string) (string, bool) {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(value, prefix)), true
		}
	}
	return "", false
}

func conversationSummaryMatch(value string, pattern *regexp.Regexp) string {
	match := pattern.FindStringSubmatch(value)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func localizeConversationSummaryDuration(locale, duration string) string {
	fields := strings.Fields(strings.TrimSpace(duration))
	if len(fields) == 2 && fields[1] == "秒" {
		return Getf(locale, "conversation.summary.durationSeconds", fields[0])
	}
	if len(fields) == 2 && fields[1] == "分" {
		return Getf(locale, "conversation.summary.durationMinutes", fields[0])
	}
	return duration
}
