import {
  ArrowRightIcon,
  CheckCircle2Icon,
  ClipboardCheckIcon,
  HeadphonesIcon,
  MessageSquareTextIcon,
  RotateCcwIcon,
  StarIcon,
  TicketCheckIcon,
  UsersRoundIcon,
  VideoIcon,
  WrenchIcon,
} from "lucide-react"
import Link from "next/link"

import { buildEnterpriseMeetingRoomPath } from "@/components/meeting/meeting-room-context"
import { useI18n } from "@/i18n/provider"
import { cn } from "@/lib/utils"

const customerEntryEventI18nPrefix = "customerEntryExtract.event."
type ConversationServiceEventT = ReturnType<typeof useI18n>
const cee = (t: ConversationServiceEventT, key: string, values?: Record<string, string | number>) => t(`${customerEntryEventI18nPrefix}${key}`, values)

export type ConversationServiceEventKind =
  | "consultation"
  | "handoff"
  | "ticket"
  | "supplier"
  | "video"
  | "repair"
  | "completed"
  | "feedback"
  | "reopened"

export type ConversationServicePhase =
  | "consultation"
  | "handoff"
  | "supplier"
  | "video-completion"

export type ConversationServiceEventData = {
  kind: ConversationServiceEventKind
  eventType: string
  title?: string
  titleKey?: string
  description: string
  phase?: ConversationServicePhase
  rating?: number
  collaborationId?: string
  partnerCompanyName?: string
  productModuleName?: string
  meetingId?: string
  ticketId?: string
  ticketNo?: string
  scheduledAt?: string
  hasDeviceContext?: boolean
}

type ConversationEventMessage = {
  senderType: string
  content: string
  payload?: string
}

function parsePayload(value?: string): Record<string, unknown> {
  if (!value?.trim()) return {}
  try {
    const parsed = JSON.parse(value)
    return parsed && typeof parsed === "object" && !Array.isArray(parsed)
      ? parsed as Record<string, unknown>
      : {}
  } catch {
    return {}
  }
}

function payloadText(payload: Record<string, unknown>, key: string) {
  const value = payload[key]
  if (typeof value === "string") return value.trim()
  if (typeof value === "number" && Number.isFinite(value)) return String(value)
  return ""
}

function payloadBoolean(payload: Record<string, unknown>, key: string) {
  const value = payload[key]
  return typeof value === "boolean" ? value : undefined
}

function contentQueryText(content: string, keys: string[]) {
  const matches = content.matchAll(/(?:https?:\/\/|\/)[^\s"'<>）)\]]+/g)
  for (const match of matches) {
    const rawURL = match[0].replace(/[),.，。）]+$/g, "")
    try {
      const url = new URL(rawURL, "https://remotehelpdesk.local")
      for (const key of keys) {
        const value = url.searchParams.get(key)?.trim()
        if (value) return value
      }
    } catch {
      // Ignore non-URL text fragments and keep scanning.
    }
  }
  return ""
}

function eventText(payload: Record<string, unknown>, content: string, keys: string[]) {
  for (const key of keys) {
    const value = payloadText(payload, key)
    if (value) return value
  }
  return contentQueryText(content, keys)
}

function videoEventDescription(content: string) {
  return content
    .replace(/\s*\[[^\]]+\]\((?:https?:\/\/|\/)[^)]+\)\s*/g, "\n")
    .replace(/\n{3,}/g, "\n\n")
    .trim()
}

function eventRoomName(content: string) {
  return content.match(/会议室[:：]\s*([^，,\n]+)/)?.[1]?.trim() || ""
}

function eventDuration(content: string) {
  return content.match(/时长[:：]\s*([^，,\n]+)/)?.[1]?.trim() || ""
}

function localizeEventDuration(t: ConversationServiceEventT, duration: string) {
  const normalized = duration.replace(/\s+/g, " ").trim()
  const minuteSecond = normalized.match(/^(\d+)\s*分(?:\s*(\d+)\s*秒)?$/)
  if (minuteSecond) {
    const minutes = Number(minuteSecond[1])
    const seconds = Number(minuteSecond[2] || 0)
    return seconds > 0
      ? cee(t, "durationMinutesSeconds", { minutes, seconds })
      : cee(t, "durationMinutes", { minutes })
  }
  const secondOnly = normalized.match(/^(\d+)\s*秒$/)
  if (secondOnly) {
    return cee(t, "durationSeconds", { seconds: Number(secondOnly[1]) })
  }
  return duration
}

function feedbackComment(description: string, rating?: number) {
  const value = description.trim()
  if (!value) return ""
  const legacy = value.match(/^客户已提交服务评价[:：]\s*\d+\/5(?:\s*·\s*(.+))?$/)
  if (legacy) return legacy[1]?.trim() || ""
  if (rating && value === `${rating}/5`) return ""
  return value
}

function ticketNoFromContent(content: string) {
  return content.match(/服务工单\s+([A-Za-z0-9#_-]+)/)?.[1]?.trim() || ""
}

function repairSummaryFromContent(content: string) {
  const value = content.trim()
  if (value === "维修结论已提交，等待客户确认设备状态。" || value === "维修结论已提交，等待客户确认服务结果。") return ""
  return value.match(/^维修结论已提交[:：]\s*(.+)$/)?.[1]?.trim() || ""
}

const HANDOFF_CUSTOMER_REQUESTED_DESCRIPTIONS = new Set([
  "客户主动请求转接技术工程师，系统正在安排人工支持。",
  "The customer requested a technical engineer. Human support is being arranged.",
  "El cliente solicito un ingeniero tecnico. Se esta organizando el soporte humano.",
])

const HANDOFF_OFF_HOURS_DESCRIPTIONS = new Set([
  "当前不在人工客服服务时间内。你可以继续描述问题，我会尽力协助；也可以在服务时间恢复后再次申请人工客服。",
  "Human support is currently outside service hours. You can keep describing the issue and I will do my best to help. You can also request a human agent again when service hours resume.",
  "El soporte humano esta fuera del horario de servicio. Puede seguir describiendo el problema o volver a solicitar soporte cuando comience el siguiente turno.",
])

const HANDOFF_OFF_HOURS_QUEUED_DESCRIPTIONS = new Set([
  "你的请求已进入产品维修组待认领，组内工程师可以查看并接单；若当前无人值守，排班恢复后会继续处理。",
  "你的请求已进入技术支持组待认领，组内工程师可以查看并接单；若当前无人值守，排班恢复后会继续处理。",
  "Your request is now visible to the product repair team for pickup. If nobody is on duty, it will be handled when the next shift starts.",
  "Your request is now visible to the assigned technical support team for pickup. If nobody is on duty, it will be handled when the next shift starts.",
  "Tu solicitud ya esta visible para el equipo de soporte tecnico asignado. Si no hay nadie de guardia, se atendera al comenzar el siguiente turno.",
])

const HANDOFF_AI_HOLD_DESCRIPTIONS = new Set([
  "你的问题已记录并通知维修团队。我会继续协助，工程师接入后可以看到本次会话和工单上下文。",
  "Your issue has been recorded and the repair team has been notified. I will keep assisting, and the engineer will receive this conversation and ticket context when they join.",
  "Su problema se ha registrado y se ha notificado al equipo de soporte. Seguire ayudando y el ingeniero recibira el contexto de la conversacion y el ticket cuando se incorpore.",
])

function localizedHandoffDescription(
  t: ConversationServiceEventT,
  description: string,
  hasDeviceConcept: boolean,
) {
  const normalized = description.replace(/\s+/g, " ").trim()
  if (HANDOFF_CUSTOMER_REQUESTED_DESCRIPTIONS.has(normalized)) {
    return cee(t, "handoffCustomerRequestedDescription")
  }
  if (HANDOFF_OFF_HOURS_DESCRIPTIONS.has(normalized)) {
    return cee(t, "handoffOffHoursDescription")
  }
  if (HANDOFF_OFF_HOURS_QUEUED_DESCRIPTIONS.has(normalized)) {
    return cee(t, hasDeviceConcept ? "handoffOffHoursQueuedDescriptionProduct" : "handoffOffHoursQueuedDescription")
  }
  if (HANDOFF_AI_HOLD_DESCRIPTIONS.has(normalized)) {
    return cee(t, hasDeviceConcept ? "handoffAiHoldDescriptionProduct" : "handoffAiHoldDescription")
  }
  return ""
}

function supplierEventContext(content: string) {
  const normalized = content.replace(/\s+/g, " ").trim()
  const invited = normalized.match(/已邀请供应商协作[：:]\s*(.+)$/)
  if (invited?.[1]) {
    const parts = invited[1]
      .split(/\s*[·/／]\s*/)
      .map((part) => part.trim())
      .filter(Boolean)
    return {
      partnerCompanyName: parts[0] ?? "",
      productModuleName: parts.slice(1).join(" · "),
    }
  }
  const joined = normalized.match(/^(.+?)已加入协作会话/)
  if (joined?.[1]) {
    return {
      partnerCompanyName: joined[1].trim(),
      productModuleName: "",
    }
  }
  return {
    partnerCompanyName: "",
    productModuleName: "",
  }
}

export function parseConversationServiceEvent(
  message: ConversationEventMessage,
): ConversationServiceEventData | null {
  const senderType = message.senderType.toLowerCase()
  if (senderType !== "system") return null

  const content = message.content.trim()
  const payload = parsePayload(message.payload)
  const eventType = payloadText(payload, "eventType")
  const source = payloadText(payload, "source")

  if (eventType === "ticket_created") {
    return {
      kind: "ticket",
      eventType,
      titleKey: "ticketCreated",
      description: content,
      phase: "handoff",
      ticketNo: payloadText(payload, "ticketNo") || payloadText(payload, "ticket_no") || ticketNoFromContent(content),
      hasDeviceContext: payloadBoolean(payload, "hasDeviceContext"),
    }
  }

  if (eventType === "ticket_feedback_submitted") {
    const rawRating = Number(payload.rating)
    const rating = Number.isFinite(rawRating) ? Math.min(5, Math.max(1, rawRating)) : undefined
    return {
      kind: "feedback",
      eventType,
      titleKey: "feedbackSubmitted",
      description: payloadText(payload, "comment") || content,
      rating,
    }
  }
  if (eventType === "supplier_collaboration_invited") {
    const supplierContext = supplierEventContext(content)
    return {
      kind: "supplier",
      eventType,
      titleKey: "supplierInvited",
      description: content,
      phase: "supplier",
      collaborationId: eventText(payload, content, ["collaborationId", "collaboration_id"]),
      partnerCompanyName: payloadText(payload, "partnerCompanyName") || payloadText(payload, "partner_company_name") || supplierContext.partnerCompanyName,
      productModuleName: payloadText(payload, "productModuleName") || payloadText(payload, "product_module_name") || supplierContext.productModuleName,
    }
  }
  if (eventType === "supplier_collaboration_accepted") {
    const supplierContext = supplierEventContext(content)
    return {
      kind: "supplier",
      eventType,
      titleKey: "supplierJoined",
      description: content,
      phase: "supplier",
      collaborationId: eventText(payload, content, ["collaborationId", "collaboration_id"]),
      partnerCompanyName: payloadText(payload, "partnerCompanyName") || payloadText(payload, "partner_company_name") || supplierContext.partnerCompanyName,
      productModuleName: payloadText(payload, "productModuleName") || payloadText(payload, "product_module_name") || supplierContext.productModuleName,
    }
  }
  if (eventType === "meeting_started" || eventType === "meeting_scheduled") {
    const scheduled = eventType === "meeting_scheduled"
    return {
      kind: "video",
      eventType,
      titleKey: scheduled ? "videoScheduled" : "videoStarted",
      description: videoEventDescription(content),
      phase: "video-completion",
      meetingId: eventText(payload, content, ["meetingId", "meeting_id"]),
      ticketId: eventText(payload, content, ["ticketId", "ticket_id"]),
      scheduledAt: eventText(payload, content, ["scheduledAt", "scheduled_at"]),
    }
  }
  if (eventType === "meeting_ended") {
    return {
      kind: "video",
      eventType,
      titleKey: "videoRecorded",
      description: videoEventDescription(content),
      phase: "video-completion",
      meetingId: eventText(payload, content, ["meetingId", "meeting_id"]),
      ticketId: eventText(payload, content, ["ticketId", "ticket_id"]),
    }
  }
  if (eventType === "ticket_reopened") {
    return {
      kind: "reopened",
      eventType,
      titleKey: "issueRecurred",
      description: content,
      phase: "handoff",
    }
  }
  if (eventType === "ticket_resolved" || eventType === "repair_completed") {
    return {
      kind: "repair",
      eventType,
      titleKey: "repairConclusion",
      description: content,
      phase: "video-completion",
      ticketNo: payloadText(payload, "ticketNo") || payloadText(payload, "ticket_no"),
      hasDeviceContext: payloadBoolean(payload, "hasDeviceContext"),
    }
  }
  if (eventType === "ticket_closed" || eventType === "conversation_closed") {
    return {
      kind: "completed",
      eventType,
      titleKey: "serviceCompleted",
      description: content,
      phase: "video-completion",
    }
  }
  if (source === "customer_request" || /客户主动请求|转接技术工程师|接入人工/.test(content)) {
    return {
      kind: "handoff",
      eventType: eventType || "handoff_requested",
      titleKey: "humanRequested",
      description: payloadText(payload, "reason") || content,
      phase: "handoff",
    }
  }
  if (/生成.*工单|工单.*已创建|已生成服务工单/.test(content)) {
    return {
      kind: "ticket",
      eventType: eventType || "ticket_created",
      titleKey: "ticketCreated",
      description: content,
      phase: "handoff",
      ticketNo: payloadText(payload, "ticketNo") || payloadText(payload, "ticket_no") || ticketNoFromContent(content),
    }
  }
  if (/工程师.*接入|客服.*接入|会话已分配/.test(content)) {
    return {
      kind: "handoff",
      eventType: eventType || "human_support_started",
      titleKey: "engineerJoined",
      description: content,
      phase: "handoff",
    }
  }
  if (/已邀请供应商|供应商已加入/.test(content)) {
    const supplierContext = supplierEventContext(content)
    return {
      kind: "supplier",
      eventType: eventType || "supplier_collaboration",
      titleKey: /已加入/.test(content) ? "supplierJoined" : "supplierInvited",
      description: content,
      phase: "supplier",
      collaborationId: eventText(payload, content, ["collaborationId", "collaboration_id"]),
      partnerCompanyName: payloadText(payload, "partnerCompanyName") || payloadText(payload, "partner_company_name") || supplierContext.partnerCompanyName,
      productModuleName: payloadText(payload, "productModuleName") || payloadText(payload, "product_module_name") || supplierContext.productModuleName,
    }
  }
  if (/预定视频协作|发起视频协作/.test(content)) {
    const scheduled = /预定视频协作/.test(content)
    return {
      kind: "video",
      eventType: eventType || (scheduled ? "meeting_scheduled" : "meeting_started"),
      titleKey: scheduled ? "videoScheduled" : "videoStarted",
      description: videoEventDescription(content),
      phase: "video-completion",
      meetingId: eventText(payload, content, ["meetingId", "meeting_id"]),
      ticketId: eventText(payload, content, ["ticketId", "ticket_id"]),
      scheduledAt: eventText(payload, content, ["scheduledAt", "scheduled_at"]),
    }
  }
  if (LEGACY_VIDEO_MEETING_ENDED_PATTERN.test(content)) {
    return {
      kind: "video",
      eventType: eventType || "meeting_ended",
      titleKey: "videoRecorded",
      description: videoEventDescription(content),
      phase: "video-completion",
      meetingId: eventText(payload, content, ["meetingId", "meeting_id"]),
      ticketId: eventText(payload, content, ["ticketId", "ticket_id"]),
    }
  }
  if (/维修结论|维修记录/.test(content)) {
    return {
      kind: "repair",
      eventType: eventType || "repair_completed",
      titleKey: "repairConclusion",
      description: content,
      phase: "video-completion",
    }
  }
  return null
}

const eventStyle: Record<ConversationServiceEventKind, {
  icon: typeof HeadphonesIcon
  iconClassName: string
  panelClassName: string
}> = {
  consultation: {
    icon: MessageSquareTextIcon,
    iconClassName: "bg-primary/10 text-primary",
    panelClassName: "border-primary/20 bg-primary/10",
  },
  handoff: {
    icon: HeadphonesIcon,
    iconClassName: "bg-primary/10 text-primary",
    panelClassName: "border-primary/20 bg-primary/10",
  },
  ticket: {
    icon: TicketCheckIcon,
    iconClassName: "bg-primary/10 text-primary",
    panelClassName: "border-primary/20 bg-primary/10",
  },
  supplier: {
    icon: UsersRoundIcon,
    iconClassName: "bg-primary/10 text-primary",
    panelClassName: "border-primary/20 bg-primary/10",
  },
  video: {
    icon: VideoIcon,
    iconClassName: "bg-primary/10 text-primary",
    panelClassName: "border-primary/20 bg-primary/10",
  },
  repair: {
    icon: WrenchIcon,
    iconClassName: "bg-primary/10 text-primary",
    panelClassName: "border-primary/20 bg-primary/10",
  },
  completed: {
    icon: CheckCircle2Icon,
    iconClassName: "bg-primary/10 text-primary",
    panelClassName: "border-primary/20 bg-primary/10",
  },
  feedback: {
    icon: StarIcon,
    iconClassName: "bg-amber-100 text-amber-700",
    panelClassName: "border-amber-200 bg-amber-50/80",
  },
  reopened: {
    icon: RotateCcwIcon,
    iconClassName: "bg-destructive/10 text-destructive",
    panelClassName: "border-destructive/20 bg-destructive/10",
  },
}

export type ConversationServiceEventAudience = "enterprise" | "customer" | "partner"
export type ConversationServiceEventVideoHrefResolver = (
  event: ConversationServiceEventData,
  audience: ConversationServiceEventAudience,
) => string

function defaultVideoActionHref(event: ConversationServiceEventData, audience: ConversationServiceEventAudience) {
  if (
    event.kind !== "video" ||
    (event.eventType !== "meeting_started" && event.eventType !== "meeting_scheduled") ||
    !event.meetingId
  ) {
    return ""
  }
  const meetingId = encodeURIComponent(event.meetingId)
  if (audience === "customer") {
    return `/customer/meeting?meetingId=${meetingId}`
  }
  if (audience === "partner") {
    return `/partner/video?meeting_id=${meetingId}`
  }
  const ticketId = Number(event.ticketId)
  return buildEnterpriseMeetingRoomPath(event.meetingId, Number.isSafeInteger(ticketId) && ticketId > 0 ? ticketId : undefined)
}

function videoActionLabel(t: ConversationServiceEventT, audience: ConversationServiceEventAudience) {
  if (audience === "customer") return cee(t, "videoActionCustomer")
  if (audience === "partner") return cee(t, "videoActionPartner")
  return cee(t, "videoActionEnterprise")
}

const LEGACY_VIDEO_MEETING_ENDED_PATTERN = /(?:\u89c6\u9891\u4f1a\u8bae\u5df2\u7ed3\u675f|视频排查已记录)/
const LEGACY_AFTERSALES_COMMAND_TITLE_PATTERN = /\u552e\u540e\u5de5\u5355\u6307\u6325\u53f0/g
const LEGACY_CONVERSATION_WORKBENCH_TITLE_PATTERN = /\u4f1a\u8bdd\u5de5\u4f5c\u53f0/g
const LEGACY_SERVICE_WORKBENCH_TITLE_PATTERN = /\u670d\u52a1\u5de5\u4f5c\u53f0/g
const LEGACY_VIDEO_MEETING_TITLE_PATTERN = /\u89c6\u9891\u4f1a\u8bae/g

function normalizeEnterpriseServiceEventCopy(t: ConversationServiceEventT, value: string) {
  return value
    .replaceAll(LEGACY_AFTERSALES_COMMAND_TITLE_PATTERN, cee(t, "normalizeWorkbenchCenter"))
    .replaceAll(LEGACY_CONVERSATION_WORKBENCH_TITLE_PATTERN, cee(t, "normalizeConversationHandling"))
    .replaceAll(LEGACY_SERVICE_WORKBENCH_TITLE_PATTERN, cee(t, "normalizeServiceHandling"))
    .replaceAll(LEGACY_VIDEO_MEETING_TITLE_PATTERN, cee(t, "normalizeVideoCollaboration"))
}

function serviceEventDescription(
  t: ConversationServiceEventT,
  event: ConversationServiceEventData,
  audience: ConversationServiceEventAudience,
  hasDeviceConcept: boolean,
) {
  if (event.kind === "handoff") {
    const localizedDescription = localizedHandoffDescription(t, event.description, hasDeviceConcept)
    if (localizedDescription) return localizedDescription
  }
  if (event.eventType === "ticket_created") {
    if (event.hasDeviceContext === false) {
      return event.ticketNo
        ? cee(t, "ticketCreatedDescriptionWithTicketNoGeneral", { ticketNo: event.ticketNo })
        : cee(t, "ticketCreatedDescriptionGeneral")
    }
    return event.ticketNo
      ? cee(t, "ticketCreatedDescriptionWithTicketNo", { ticketNo: event.ticketNo })
      : cee(t, "ticketCreatedDescription")
  }
  if (event.eventType === "supplier_collaboration_invited" || event.titleKey === "supplierInvited") {
    if (event.partnerCompanyName && event.productModuleName) {
      return cee(t, "supplierInvitedDescriptionWithModule", {
        company: event.partnerCompanyName,
        module: event.productModuleName,
      })
    }
    if (event.partnerCompanyName) {
      return cee(t, "supplierInvitedDescriptionWithCompany", { company: event.partnerCompanyName })
    }
    return cee(t, "supplierInvitedDescription")
  }
  if (event.eventType === "supplier_collaboration_accepted" || event.titleKey === "supplierJoined") {
    return event.partnerCompanyName
      ? cee(t, "supplierJoinedDescriptionWithCompany", { company: event.partnerCompanyName })
      : cee(t, "supplierJoinedDescription")
  }
  if (event.eventType === "meeting_started") {
    return cee(t, "videoStartedDescription")
  }
  if (event.eventType === "meeting_scheduled") {
    return event.scheduledAt
      ? cee(t, "videoScheduledDescriptionWithTime", { time: event.scheduledAt })
      : cee(t, "videoScheduledDescription")
  }
  if (event.eventType === "meeting_ended") {
    const roomName = eventRoomName(event.description)
    const duration = eventDuration(event.description)
    const durationText = duration ? localizeEventDuration(t, duration) : ""
    if (roomName && durationText) {
      return cee(t, "videoEndedDescriptionWithRoomDuration", { roomName, duration: durationText })
    }
    if (durationText) {
      return cee(t, "videoEndedDescriptionWithDuration", { duration: durationText })
    }
    return cee(t, "videoEndedDescription")
  }
  if (event.eventType === "ticket_feedback_submitted") {
    const comment = feedbackComment(event.description, event.rating)
    if (event.rating && comment) {
      return cee(t, "feedbackSubmittedDescriptionWithComment", { rating: event.rating, comment })
    }
    if (event.rating) {
      return cee(t, "feedbackSubmittedDescription", { rating: event.rating })
    }
    return cee(t, "feedbackSubmittedDescriptionNoRating")
  }
  if (event.eventType === "ticket_resolved" || event.eventType === "repair_completed") {
    const summary = repairSummaryFromContent(event.description)
    return summary
      ? cee(t, "repairConclusionDescriptionWithSummary", { summary })
      : cee(t, event.hasDeviceContext === false ? "repairConclusionDescriptionWaitingGeneral" : "repairConclusionDescriptionWaiting")
  }
  return audience === "enterprise"
    ? normalizeEnterpriseServiceEventCopy(t, event.description)
    : event.description
}

export function ConversationServiceEvent({
  event,
  time,
  className,
  audience = "enterprise",
  density = "default",
  hasDeviceConcept = true,
  resolveVideoActionHref,
}: {
  event: ConversationServiceEventData
  time?: string
  className?: string
  audience?: ConversationServiceEventAudience
  density?: "default" | "compact"
  hasDeviceConcept?: boolean
  resolveVideoActionHref?: ConversationServiceEventVideoHrefResolver
}) {
  const t = useI18n()
  const style = eventStyle[event.kind]
  const EventIcon = style.icon
  const actionHref = resolveVideoActionHref?.(event, audience) ?? defaultVideoActionHref(event, audience)
  const description = serviceEventDescription(t, event, audience, hasDeviceConcept)
  const compact = density === "compact"
  return (
    <div
      className={cn("flex items-center gap-2 py-1", compact && "py-0.5", className)}
      data-service-event={event.eventType}
      data-sender-type="system"
      role="status"
    >
      <span className="h-px min-w-3 flex-1 bg-border" aria-hidden="true" />
      <div className={cn("flex max-w-[92%] items-start gap-2 rounded-md border shadow-sm", compact ? "px-2.5 py-1.5" : "px-3 py-2", style.panelClassName)}>
        <span className={cn("mt-0.5 flex shrink-0 items-center justify-center rounded-full", compact ? "size-6" : "size-7", style.iconClassName)}>
          <EventIcon className={compact ? "size-3" : "size-3.5"} />
        </span>
        <div className="min-w-0 text-left">
          <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5">
            <strong className={cn("font-semibold text-foreground", compact ? "text-rhd-xs" : "text-xs")}>{event.titleKey ? cee(t, event.titleKey) : (event.title ?? "")}</strong>
            {time ? <time className={cn("text-muted-foreground", compact ? "text-rhd-2xs" : "text-rhd-2xs")}>{time}</time> : null}
          </div>
          {event.rating ? (
            <div className={cn("mt-1 flex items-center gap-0.5", compact && "mt-0.5")} aria-label={cee(t, "starRating", { rating: event.rating })}>
              {Array.from({ length: 5 }, (_, index) => (
                <StarIcon
                  key={index}
                  className={cn(
                    compact ? "size-3" : "size-3.5",
                    index < event.rating! ? "fill-amber-400 text-amber-400" : "text-amber-200",
                  )}
                />
              ))}
            </div>
          ) : null}
          {description ? (
            <p className={cn("whitespace-pre-wrap break-words text-muted-foreground", compact ? "mt-0.5 text-rhd-xs leading-[1.125rem]" : "mt-1 text-xs leading-5")}>
              {description}
            </p>
          ) : null}
          {actionHref ? (
            <Link
              href={actionHref}
              className={cn("inline-flex items-center gap-1.5 rounded-md border border-primary/20 bg-card font-medium text-primary shadow-sm transition hover:bg-primary/10", compact ? "mt-1.5 h-7 px-2 text-rhd-xs" : "mt-2 h-8 px-2.5 text-xs")}
            >
              <VideoIcon className={compact ? "size-3" : "size-3.5"} />
              {videoActionLabel(t, audience)}
              <ArrowRightIcon className={compact ? "size-3" : "size-3.5"} />
            </Link>
          ) : null}
        </div>
      </div>
      <span className="h-px min-w-3 flex-1 bg-border" aria-hidden="true" />
    </div>
  )
}

const phaseKeyMap: Record<ConversationServicePhase, string> = {
  consultation: "phaseConsultation",
  handoff: "phaseHandoff",
  supplier: "phaseSupplier",
  "video-completion": "phaseVideoCompletion",
}

export function ConversationPhaseDivider({
  phase,
  density = "default",
}: {
  phase: ConversationServicePhase
  density?: "default" | "compact"
}) {
  const t = useI18n()
  const PhaseIcon = phase === "consultation"
    ? MessageSquareTextIcon
    : phase === "handoff"
      ? HeadphonesIcon
      : phase === "supplier"
        ? UsersRoundIcon
        : ClipboardCheckIcon
  const compact = density === "compact"

  return (
    <div className={cn("flex items-center gap-3", compact ? "py-1.5" : "py-2")} data-service-phase={phase}>
      <span className={cn("h-px min-w-5 flex-1 bg-slate-200", compact && "min-w-4")} aria-hidden="true" />
      <div className={cn("flex shrink-0 items-center gap-2 font-medium text-slate-400", compact ? "text-rhd-xs" : "text-xs")}>
        <PhaseIcon className={compact ? "size-3" : "size-3.5"} />
        <span>{cee(t, phaseKeyMap[phase])}</span>
      </div>
      <span className={cn("h-px min-w-5 flex-1 bg-slate-200", compact && "min-w-4")} aria-hidden="true" />
    </div>
  )
}
