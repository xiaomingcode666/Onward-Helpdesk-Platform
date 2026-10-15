import { normalizeLocale } from "@/i18n/config"
import { translateMessage } from "@/i18n/messages"

type LocalizableNotification = {
  title: string
  content: string
  notificationType: string
}

type LocalizableEnterpriseNotification = {
  title: string
  content: string
  notification_type: string
}

const TICKET_ASSIGNED_TITLE = "\u5de5\u5355\u6307\u6d3e\u63d0\u9192"
const CONVERSATION_TRANSFERRED_TITLE = "\u4f1a\u8bdd\u8f6c\u63a5\u63d0\u9192"
const CONVERSATION_AUTO_ASSIGNED_TITLE = "\u4f1a\u8bdd\u81ea\u52a8\u5206\u914d\u63d0\u9192"
const CONVERSATION_ASSIGNED_TITLE = "\u4f1a\u8bdd\u5206\u914d\u63d0\u9192"

const TICKET_ASSIGNED_PATTERN = /^\u5de5\u5355 (.+) \u5df2\u6307\u6d3e\u7ed9\u4f60$/
const CONVERSATION_ASSIGNED_PATTERN = /^\u4f1a\u8bdd #([0-9]+) \u5df2\u5206\u914d\u7ed9\u4f60$/
const ASSIGNMENT_REASON_PREFIX = "\u6307\u6d3e\u539f\u56e0: "
const CONVERSATION_ASSIGNMENT_REASON_PREFIX = "\u5206\u914d\u539f\u56e0: "
const TRANSFER_REASON_PREFIX = "\u8f6c\u63a5\u539f\u56e0: "
const SLA_TITLE_PATTERN = /^\u5de5\u5355\s+(.+?)\s+SLA\s+\u544a\u8b66$/
const SLA_CONTENT_PATTERN = /^(.+?)\u76ee\u6807\s*(\d+)\s*\u5206\u949f[\uff0c,]\s*(\u5f53\u524d\u6709\u6548\u8ba1\u65f6\u4e0d\u8db3\s*1\s*\u5206\u949f|\u5f53\u524d\u6709\u6548\u8ba1\u65f6\s*(\d+)\s*\u5206\u949f)[\uff0c,]\s*\u9884\u8ba1\u5269\u4f59\u7ea6\s*(\d+)\s*\u5206\u949f[\u3002.]?$/
const SLA_TARGET_MISSING_PATTERN = /^(.+?)\u76ee\u6807\u672a\u914d\u7f6e[\uff0c,]\s*(\u5f53\u524d\u6709\u6548\u8ba1\u65f6\u4e0d\u8db3\s*1\s*\u5206\u949f|\u5f53\u524d\u6709\u6548\u8ba1\u65f6\s*\d+\s*\u5206\u949f)[\u3002.]?$/
const SLA_REACHED_PATTERN = /^(.+?)\u76ee\u6807\s*(\d+)\s*\u5206\u949f[\uff0c,]\s*\u76ee\u6807\u65f6\u95f4\u5df2\u5230[\u3002.]?$/
const SLA_OVERDUE_PATTERN = /^(.+?)\u76ee\u6807\s*(\d+)\s*\u5206\u949f[\uff0c,]\s*\u5f53\u524d\u5df2\u8d85\u65f6\s*(\d+)\s*\u5206\u949f[\u3002.]?$/
const SLA_VIOLATION_KEYS: Record<string, string> = {
  "\u9996\u6b21\u54cd\u5e94": "notification.slaViolationFRT",
  "\u6d3e\u5de5": "notification.slaViolationAssignment",
  "\u89e3\u51b3\u65f6\u957f": "notification.slaViolationResolution",
  "\u5904\u7406\u65f6\u957f": "notification.slaViolationProcessing",
}

export function localizeNotificationItem<T extends LocalizableNotification>(
  notification: T,
  locale: string
): T {
  const normalizedLocale = normalizeLocale(locale)
  if (normalizedLocale !== "en-US") {
    return notification
  }
  if (notification.notificationType === "ticket_assigned") {
    return {
      ...notification,
      title: localizeNotificationTitle(notification.title, normalizedLocale),
      content: localizeTicketAssignedContent(notification.content, normalizedLocale),
    }
  }
  if (notification.notificationType === "conversation_assigned") {
    return {
      ...notification,
      title: localizeNotificationTitle(notification.title, normalizedLocale),
      content: localizeConversationAssignedContent(notification.content, normalizedLocale),
    }
  }
  if (notification.notificationType === "sla_warning") {
    return {
      ...notification,
      title: localizeSLAWarningTitle(notification.title, normalizedLocale),
      content: localizeSLAContent(notification.content, normalizedLocale),
    }
  }
  return notification
}

export function localizeEnterpriseNotification<T extends LocalizableEnterpriseNotification>(
  notification: T,
  locale: string,
): T {
  const normalizedLocale = normalizeLocale(locale)
  if (normalizedLocale !== "en-US" || notification.notification_type !== "sla_warning") {
    return notification
  }
  return {
    ...notification,
    title: localizeSLAWarningTitle(notification.title, normalizedLocale),
    content: localizeSLAContent(notification.content, normalizedLocale),
  }
}

function localizeTicketAssignedContent(content: string, locale: ReturnType<typeof normalizeLocale>) {
  const lines = splitNotificationLines(content)
  if (lines.length === 0) {
    return content
  }
  const match = lines[0].match(TICKET_ASSIGNED_PATTERN)
  if (match?.[1]) {
    lines[0] = translateMessage(locale, "notification.ticketAssignedLine", {
      ticketNo: match[1],
    })
  }
  return lines
    .map((line) =>
      line.startsWith(ASSIGNMENT_REASON_PREFIX)
        ? translateMessage(locale, "notification.assignmentReason", {
            reason: line.slice(ASSIGNMENT_REASON_PREFIX.length),
          })
        : line
    )
    .join("\n")
}

function localizeConversationAssignedContent(content: string, locale: ReturnType<typeof normalizeLocale>) {
  const lines = splitNotificationLines(content)
  if (lines.length === 0) {
    return content
  }
  const match = lines[0].match(CONVERSATION_ASSIGNED_PATTERN)
  if (match?.[1]) {
    lines[0] = translateMessage(locale, "notification.conversationAssignedLine", {
      conversationId: match[1],
    })
  }
  return lines
    .map((line) => {
      if (line.startsWith(CONVERSATION_ASSIGNMENT_REASON_PREFIX)) {
        return translateMessage(locale, "notification.assignmentReason", {
          reason: line.slice(CONVERSATION_ASSIGNMENT_REASON_PREFIX.length),
        })
      }
      if (line.startsWith(TRANSFER_REASON_PREFIX)) {
        return translateMessage(locale, "notification.transferReason", {
          reason: line.slice(TRANSFER_REASON_PREFIX.length),
        })
      }
      return line
    })
    .join("\n")
}

function localizeNotificationTitle(title: string, locale: ReturnType<typeof normalizeLocale>) {
  switch (title.trim()) {
    case TICKET_ASSIGNED_TITLE:
      return translateMessage(locale, "notification.ticketAssignedTitle")
    case CONVERSATION_TRANSFERRED_TITLE:
      return translateMessage(locale, "notification.conversationTransferredTitle")
    case CONVERSATION_AUTO_ASSIGNED_TITLE:
      return translateMessage(locale, "notification.conversationAutoAssignedTitle")
    case CONVERSATION_ASSIGNED_TITLE:
      return translateMessage(locale, "notification.conversationAssignedTitle")
    default:
      return title
  }
}

function localizeSLAWarningTitle(title: string, locale: ReturnType<typeof normalizeLocale>) {
  const match = title.trim().match(SLA_TITLE_PATTERN)
  if (!match?.[1]) return title
  return translateMessage(locale, "notification.slaWarningTitle", { ticketNo: match[1] })
}

function localizeSLAContent(content: string, locale: ReturnType<typeof normalizeLocale>): string {
  const normalized = content.trim()
  if (!normalized) return content
  const lines = normalized.split(/\r?\n/)
  if (lines.length > 1) {
    const localizedFirstLine: string = localizeSLAContent(lines[0], locale)
    return localizedFirstLine === lines[0]
      ? content
      : [localizedFirstLine, ...lines.slice(1)].join("\n")
  }

  const elapsedLabel = (value: string) => {
    const elapsedMinutes = value.match(/\u5f53\u524d\u6709\u6548\u8ba1\u65f6\s*(\d+)/)?.[1]
    return elapsedMinutes
      ? translateMessage(locale, "notification.slaElapsedMinutes", { minutes: elapsedMinutes })
      : translateMessage(locale, "notification.slaElapsedNone")
  }
  const violationLabel = (value: string) => {
    const key = SLA_VIOLATION_KEYS[value.trim()]
    return key ? translateMessage(locale, key) : value
  }

  const remaining = normalized.match(SLA_CONTENT_PATTERN)
  if (remaining) {
    return translateMessage(locale, "notification.slaContentRemaining", {
      label: violationLabel(remaining[1]),
      targetMinutes: remaining[2],
      elapsed: elapsedLabel(remaining[3]),
      remainingMinutes: remaining[5],
    })
  }
  const targetMissing = normalized.match(SLA_TARGET_MISSING_PATTERN)
  if (targetMissing) {
    return translateMessage(locale, "notification.slaContentTargetMissing", {
      label: violationLabel(targetMissing[1]),
      elapsed: elapsedLabel(targetMissing[2]),
    })
  }
  const reached = normalized.match(SLA_REACHED_PATTERN)
  if (reached) {
    return translateMessage(locale, "notification.slaContentReached", {
      label: violationLabel(reached[1]),
      targetMinutes: reached[2],
    })
  }
  const overdue = normalized.match(SLA_OVERDUE_PATTERN)
  if (overdue) {
    return translateMessage(locale, "notification.slaContentOverdue", {
      label: violationLabel(overdue[1]),
      targetMinutes: overdue[2],
      overdueMinutes: overdue[3],
    })
  }
  return content
}

function splitNotificationLines(content: string) {
  const normalized = content.trim()
  if (!normalized) {
    return []
  }
  return normalized.split("\n")
}
