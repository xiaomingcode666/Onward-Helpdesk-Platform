"use client"

import { translateCurrentMessage } from "@/i18n/messages"
import {
  AlertTriangleIcon,
  CheckCircle2Icon,
  CircleDotIcon,
  ClipboardListIcon,
  FileTextIcon,
  MessageSquareMoreIcon,
  PackageIcon,
  StarIcon,
  UserCheckIcon,
  type LucideIcon,
} from "lucide-react"
import { useState, type ReactNode } from "react"

import { StatusTag, UnderlineTabs, type RailopsTabItem } from "@railops/ui"
import type { TicketAggregateDTO, TicketFlowStepDTO } from "@/lib/api/types"
import { isProcessingTicketStatus, isTerminalTicketStatus } from "@/lib/ticket-lifecycle"
import { cn, formatDateTime } from "@/lib/utils"
import { intakeLabel } from "./ticket-intake"
function ee(key: string, values?: Record<string, unknown>) {
  if (!values) {
    return translateCurrentMessage(`enterpriseExtract.${key}`)
  }
  return translateCurrentMessage(
    `enterpriseExtract.${key}`,
    Object.fromEntries(
      Object.entries(values).map(([name, value]) => [
        name,
        typeof value === "string" || typeof value === "number" ? value : String(value ?? ""),
      ]),
    ),
  )
}

const CUSTOMER_ENTRY_EVENT_I18N_PREFIX = "customerEntryExtract.event."

function cee(key: string, values?: Record<string, string | number>) {
  return translateCurrentMessage(`${CUSTOMER_ENTRY_EVENT_I18N_PREFIX}${key}`, values)
}


type DetailVariant = "full" | "compact"
type DetailTabKey = "overview" | "progress" | "context" | "records"

type TicketDetailContentProps = {
  aggregate: TicketAggregateDTO
  variant?: DetailVariant
  showDeviceContext?: boolean
}

function buildDetailTabs(): RailopsTabItem[] {
  return [
    { label: ee("ticketDetail.text001"), value: "overview" },
    { label: ee("ticketDetail.text002"), value: "progress" },
    { label: ee("ticketDetail.text003"), value: "context" },
    { label: ee("ticketDetail.text004"), value: "records" },
  ]
}

export function ticketStatusLabel(status: string) {
  const map: Record<string, string> = {
    draft: ee("ticketDetail.text005"),
    pending_acceptance: ee("ticketDetail.text006"),
    pending_dispatch: ee("ticketDetail.text007"),
	accepted: ee("ticketDetail.text008"),
	assigned: ee("ticketDetail.text009"),
	pending_assignee_accept: ee("ticketDetail.text009"),
    in_progress: ee("ticketDetail.text010"),
	processing: ee("ticketDetail.text010"),
	video_support: ee("ticketDetail.text011"),
	supplier_support: ee("ticketDetail.text012"),
	resolved: ee("ticketDetail.text013"),
	pending_customer_confirm: ee("ticketDetail.text013"),
	closed: ee("ticketDetail.text014"),
	quality_review: ee("ticketDetail.text015"),
	reopened: ee("ticketDetail.text016"),
    done: ee("ticketDetail.text017"),
    cancelled: ee("ticketDetail.text018"),
  }
  return map[status] ?? status
}

export function ticketPriorityLabel(priority: string) {
  const map: Record<string, string> = {
    critical: ee("ticketDetail.text019"),
    high: ee("ticketDetail.text020"),
    medium: ee("ticketDetail.text021"),
    low: ee("ticketDetail.text022"),
  }
  return map[priority] ?? priority
}

export function ticketStatusClassName(status: string) {
  if (status === "closed" || status === "done") {
    return "border-border bg-muted text-muted-foreground"
  }
  if (isProcessingTicketStatus(status)) {
    return "border-primary/20 bg-primary/10 text-primary"
  }
  if (status === "cancelled") {
    return "border-border bg-muted text-muted-foreground"
  }
  return "border-border bg-muted text-muted-foreground"
}

export function priorityClassName(priority: string) {
  if (priority === "critical") {
    return "border-destructive/20 bg-destructive/10 text-destructive"
  }
  if (priority === "high") {
    return "border-amber-200 bg-amber-50 text-amber-800"
  }
  if (priority === "medium") {
    return "border-primary/20 bg-primary/10 text-primary"
  }
  return "border-border bg-muted text-muted-foreground"
}

export function sourceLabel(source: string) {
  const map: Record<string, string> = {
    manual: ee("ticketDetail.text023"),
    conversation: ee("ticketDetail.text024"),
  }
  return map[source] ?? (source || "-")
}

export function channelLabel(channel: string) {
  const map: Record<string, string> = {
    im: ee("ticketDetail.text025"),
    web: "Web",
    email: ee("ticketDetail.text026"),
    phone: ee("ticketDetail.text027"),
  }
  return map[channel] ?? (channel || "-")
}

function isDoneStatus(status: string) {
  return isTerminalTicketStatus(status)
}

function slaLabel(deadline: string, status: string) {
  if (isDoneStatus(status)) {
    return ticketStatusLabel(status)
  }
  if (!deadline) {
    return ee("ticketDetail.text028")
  }
  const target = new Date(deadline).getTime()
  if (!Number.isFinite(target)) {
    return ee("ticketDetail.text028")
  }
  const diff = target - Date.now()
  if (diff <= 0) {
    return ee("ticketDetail.text029")
  }
  const mins = Math.floor(diff / 60000)
  if (mins < 60) {
    return ee("ticketDetail.text030", { value0: mins })
  }
  const hours = Math.floor(mins / 60)
  if (hours < 48) {
    return ee("ticketDetail.text031", { value0: hours })
  }
  return formatDateTime(deadline)
}

function serviceSummary(aggregate: TicketAggregateDTO) {
  return (
    aggregate.ticket.description ||
    aggregate.conversation_snapshot?.summary ||
    ee("ticketDetail.text032")
  )
}

function engineerConclusion(aggregate: TicketAggregateDTO) {
  const repair = aggregate.repair
  if (repair.resolution) {
    return repair.resolution
  }
  if (aggregate.diagnosis_snapshot?.recommended_actions?.[0]) {
    return aggregate.diagnosis_snapshot.recommended_actions[0]
  }
  if (isDoneStatus(aggregate.ticket.status)) {
    return ee("ticketDetail.text033")
  }
  return ee("ticketDetail.text034")
}

function stepLabel(name: string) {
  const map: Record<string, string> = {
    Accept: ee("ticketDetail.text035"),
    Dispatch: ee("ticketDetail.text036"),
    Process: ee("ticketDetail.text002"),
    Repair: ee("ticketDetail.text037"),
    Close: ee("ticketDetail.text038"),
  }
  return map[name] ?? name
}

function progressStateClass(status: string) {
  if (status === "done") {
    return "border-border bg-muted text-muted-foreground"
  }
  if (status === "current") {
    return "border-primary/20 bg-primary/10 text-primary"
  }
  return "border-border bg-muted text-muted-foreground"
}

const CHINESE_DURATION_PATTERN = /^\s*(?:(\d+)\s*分)?\s*(?:(\d+)\s*秒)?\s*$/

function timelineContent(content: string, showDeviceContext = true) {
  const localized = localizeGeneratedTimelineContent(content, showDeviceContext)
  if (localized !== content) return localized

  const map: Record<string, string> = {
    "Ticket created": ee("ticketDetail.text039"),
    "Created ticket": ee("ticketDetail.text039"),
  }
  return map[content] ?? content
}

function localizeGeneratedTimelineContent(value: string, showDeviceContext = true) {
  const trimmed = value.trim()
  if (!trimmed) return value

  if (trimmed === "创建工单" || trimmed === "已创建工单") {
    return ee("ticketDetail.text039")
  }
  if (trimmed === "受理工单") {
    return ee("ticketDetail.text035")
  }
  if (trimmed === "关闭工单") {
    return ee("ticketDetail.text038")
  }

  if (/^工单随会话自动分配/.test(trimmed)) {
    const reason = extractTimelineReason(trimmed)
    return reason
      ? cee("ticketAutoAssignedWithReason", { reason: localizeTimelineReason(reason) })
      : cee("ticketAutoAssigned")
  }

  if (trimmed === "技术支持组成员接管并受理") {
    return cee("ticketSupportTakeoverAccepted")
  }
  if (trimmed === "同产品维修组成员接管并受理") {
    return cee(showDeviceContext ? "ticketSameProductTakeoverAccepted" : "ticketSupportTakeoverAccepted")
  }
  if (trimmed === "技术支持组成员从待派单池转派给自己并受理") {
    return cee("ticketSupportPoolTakeoverAccepted")
  }
  if (trimmed === "产品组成员从待派单池转派给自己并受理") {
    return cee(showDeviceContext ? "ticketPoolTakeoverAccepted" : "ticketSupportPoolTakeoverAccepted")
  }
  if (trimmed === "原处理人接单超时，技术支持组成员接管并受理") {
    return cee("ticketExpiredSupportTakeoverAccepted")
  }
  if (trimmed === "原处理人接单超时，产品组成员接管并受理") {
    return cee(showDeviceContext ? "ticketExpiredTakeoverAccepted" : "ticketExpiredSupportTakeoverAccepted")
  }

  const ticketCreated = trimmed.match(/^已生成服务工单(?:\s+([^，,]+))?[，,].*同步至工单[。.]?$/)
  if (ticketCreated) {
    const ticketNo = ticketCreated[1]?.trim()
    return ticketNo
      ? cee(showDeviceContext ? "ticketCreatedDescriptionWithTicketNo" : "ticketCreatedDescriptionWithTicketNoGeneral", { ticketNo })
      : cee(showDeviceContext ? "ticketCreatedDescription" : "ticketCreatedDescriptionGeneral")
  }

  const videoStarted = trimmed.match(/^发起视频协作[，,]\s*会议室[:：]\s*(.+)$/)
  if (videoStarted) {
    return cee("videoStartedWithRoom", { roomName: videoStarted[1].trim() })
  }
  if (/^工程师已发起视频协作[，,]/.test(trimmed)) {
    return cee("videoStartedDescription")
  }

  const scheduledVideo = trimmed.match(/^预定视频协作[，,]\s*会议室[:：]\s*(.+)$/)
  if (scheduledVideo) {
    return cee("videoScheduledWithRoom", { roomName: scheduledVideo[1].trim() })
  }
  if (/^工程师已预定视频协作[，,]/.test(trimmed)) {
    return cee("videoScheduledDescription")
  }

  const videoEnded = trimmed.match(/^视频协作已结束(?:[，,]\s*会议室[:：]\s*([^，,]+))?(?:[，,]\s*时长[:：]\s*(.+))?[。.]?$/)
  if (videoEnded) {
    const roomName = videoEnded[1]?.trim()
    const duration = localizeEventDuration(videoEnded[2])
    if (roomName && duration) {
      return cee("videoEndedDescriptionWithRoomDuration", { roomName, duration })
    }
    if (duration) {
      return cee("videoEndedDescriptionWithDuration", { duration })
    }
    return cee("videoEndedDescription")
  }

  const supplierInvited = trimmed.match(/^(?:已邀请供应商协作|升级供应商协作)[:：]\s*(.+?)(?:\s*[·/]\s*(.+))?$/)
  if (supplierInvited) {
    const company = supplierInvited[1].trim()
    const moduleName = supplierInvited[2]?.trim()
    return moduleName
      ? cee("supplierInvitedDescriptionWithModule", { company, module: moduleName })
      : cee("supplierInvitedDescriptionWithCompany", { company })
  }

  if (trimmed === "供应商已接单") {
    return cee("supplierAccepted")
  }

  const supplierJoined = trimmed.match(/^(.+?)已加入协作会话[，,].*供应商可共同沟通[。.]?$/)
  if (supplierJoined) {
    return cee("supplierJoinedDescriptionWithCompany", { company: supplierJoined[1].trim() })
  }
  if (/^供应商已加入协作会话/.test(trimmed)) {
    return cee("supplierJoinedDescription")
  }

  const supplierResolved = trimmed.match(/^供应商处理完成[:：]\s*(.+)$/)
  if (supplierResolved) {
    return cee("supplierResolvedDescriptionWithResolution", { resolution: supplierResolved[1].trim() })
  }

  const feedback = trimmed.match(/^客户已提交服务评价[:：]\s*([1-5])\/5(?:\s*[·,，]\s*(.+))?$/)
  if (feedback) {
    const rating = Number(feedback[1])
    const comment = feedback[2]?.trim()
    return comment
      ? cee("feedbackSubmittedDescriptionWithComment", { rating, comment })
      : cee("feedbackSubmittedDescription", { rating })
  }
  if (/^客户已完成服务评价/.test(trimmed)) {
    return cee("feedbackSubmittedDescriptionNoRating")
  }

  const repairConclusion = trimmed.match(/^维修结论已提交[:：]\s*(.+)$/)
  if (repairConclusion) {
    return cee("repairConclusionDescriptionWithSummary", { summary: repairConclusion[1].trim() })
  }
  if (trimmed === "已填写维修记录") {
    return cee("repairRecordFilled")
  }

  return value
}

function extractTimelineReason(value: string) {
  const match = value.match(/原因[:：]\s*(.+)$/)
  return match?.[1]?.trim() ?? ""
}

function localizeTimelineReason(value: string) {
  const normalized = value.trim()
  if (normalized === "自动分配") return cee("ticketReasonAutoAssign")
  if (normalized === "创建工单时指定负责人") return cee("ticketReasonInitialAssignee")
  if (normalized === "手动触发自动分配") return cee("ticketReasonManualAutoAssign")
  return normalized
}

function localizeEventDuration(value: string | undefined) {
  const duration = value?.trim() ?? ""
  if (!duration) return ""

  const match = duration.match(CHINESE_DURATION_PATTERN)
  if (!match || (!match[1] && !match[2])) {
    return duration
  }

  const minutes = Number(match[1] ?? 0)
  const seconds = Number(match[2] ?? 0)
  if (minutes > 0 && seconds > 0) {
    return cee("durationMinutesSeconds", { minutes, seconds })
  }
  if (minutes > 0) {
    return cee("durationMinutes", { minutes })
  }
  return cee("durationSeconds", { seconds })
}

function isTicketCreatedTimelineItem(item: TicketAggregateDTO["timeline"][number]) {
  return (
    item.type === "created" ||
    item.type === "ticket_created" ||
    item.content === "Ticket created" ||
    item.content === "Created ticket"
  )
}

function fieldValue(value?: string | number | null) {
  if (value === undefined || value === null || value === "") {
    return "-"
  }
  return String(value)
}

function dispatchFailureReasonLabel(reason?: string | null) {
  const normalized = String(reason || "").trim()
  const map: Record<string, string> = {
    accept_timeout_redispatching: ee("ticketDetail.text040"),
    all_candidates_at_capacity: ee("ticketDetail.text041"),
    candidate_became_unavailable: ee("ticketDetail.text042"),
    missing_supervisor: ee("ticketDetail.text043"),
    no_active_schedule_team: ee("ticketDetail.text044"),
    no_alternative_after_accept_timeout: ee("ticketDetail.text045"),
    no_matched_profile: ee("ticketDetail.text046"),
    no_product_repair_team: ee("ticketDetail.text047"),
    no_profile_for_enabled_user: ee("ticketDetail.text048"),
    no_reachable_user: ee("ticketDetail.text049"),
  }
  return map[normalized] ?? normalized
}

export function EnterpriseTicketDetailContent({
  aggregate,
  variant = "full",
  showDeviceContext = true,
}: TicketDetailContentProps) {
  const [activeTab, setActiveTab] = useState<DetailTabKey>("overview")
  const detailTabs = buildDetailTabs()
  const compact = variant === "compact"
  const ticket = aggregate.ticket
  const customer = aggregate.customer
  const device = aggregate.device_context
  const assignment = aggregate.assignment
  const repair = aggregate.repair
  const repairParts = repair.parts ?? []
  const conversation = aggregate.conversation_snapshot
  const feedback = aggregate.feedback
  const assets = aggregate.assets ?? []
  const timeline = aggregate.timeline ?? []
  const progressRecords = timeline.filter((item) => !isTicketCreatedTimelineItem(item))

  const fields: Array<[string, string]> = [
    [ee("ticketDetail.text050"), ticket.ticket_no],
    [ee("ticketDetail.text051"), customer.name],
    ...(showDeviceContext ? [
      [ee("ticketDetail.text052"), device.device_no || device.serial_no],
      [ee("ticketDetail.text053"), device.product_name || device.product_code],
    ] as Array<[string, string]> : []),
    [ee("ticketDetail.text054"), sourceLabel(ticket.source)],
    ...(ticket.source_record_id ? [
      [intakeLabel("channel"), channelLabel(ticket.channel)],
      [intakeLabel("source_record_id"), ticket.source_record_id],
      [intakeLabel("project_key"), ticket.project_key || "-"],
      [intakeLabel("ticket_type"), ticket.ticket_type || "-"],
      [intakeLabel("caller_name"), ticket.caller_name || "-"],
      [intakeLabel("caller_phone"), ticket.caller_phone || "-"],
      [intakeLabel("received_at"), ticket.received_at ? formatDateTime(ticket.received_at) : "-"],
      [ticket.context_status === "context_incomplete" ? intakeLabel("incomplete") : intakeLabel("complete"), (ticket.missing_context ?? []).map(intakeLabel).join(", ") || "-"],
    ] as Array<[string, string]> : []),
    [ee("ticketDetail.text055"), assignment.assignee_name || ee("ticketDetail.text056")],
    ["SLA", slaLabel(ticket.sla_deadline, ticket.status)],
    [ee("ticketDetail.text057"), ticketStatusLabel(ticket.status)],
  ]

  const assignmentSection = (
    <DetailSection
      icon={UserCheckIcon}
      title={ee("ticketDetail.text058")}
    >
      <div className="space-y-2">
        <RecordRow
          label={ee("ticketDetail.text059")}
          value={assignment.team_name || ee("ticketDetail.text060")}
          meta={assignment.dispatch_attempts > 0 ? ee("ticketDetail.text061", { value0: assignment.dispatch_attempts }) : undefined}
        />
        <RecordRow
          label={ee("ticketDetail.text055")}
          value={assignment.assignee_name || ee("ticketDetail.text056")}
          meta={
            assignment.accepted_at
              ? ee("ticketDetail.text062", { value0: formatDateTime(assignment.accepted_at) })
              : assignment.accept_deadline_at
                ? ee("ticketDetail.text063", { value0: formatDateTime(assignment.accept_deadline_at) })
                : undefined
          }
        />
        {assignment.dispatch_deferred_until || assignment.last_dispatch_failure_reason ? (
          <RecordRow
            label={ee("ticketDetail.text064")}
            value={dispatchFailureReasonLabel(assignment.last_dispatch_failure_reason) || ee("ticketDetail.text065")}
            meta={assignment.dispatch_deferred_until ? formatDateTime(assignment.dispatch_deferred_until) : undefined}
          />
        ) : null}
      </div>
    </DetailSection>
  )

  const conversationSection = (
    <DetailSection
      icon={MessageSquareMoreIcon}
      title={ee("ticketDetail.text066")}
    >
      {conversation ? (
        <div className="space-y-2 text-xs leading-5 text-muted-foreground">
          <p className="line-clamp-4">{conversation.summary || ee("ticketDetail.text067")}</p>
          <div className="flex flex-wrap gap-2">
            <span>{conversation.message_count}{ee("ticketDetail.text068")}</span>
            <span>{conversation.ai_served ? ee("ticketDetail.text069") : ee("ticketDetail.text070")}</span>
            <span>{formatDateTime(conversation.last_message_at)}</span>
          </div>
        </div>
      ) : (
        <EmptyDetail text={ee("ticketDetail.text071")} />
      )}
    </DetailSection>
  )

  const assetsSection = (
    <DetailSection
      icon={FileTextIcon}
      title={ee("ticketDetail.text072")}
    >
      {assets.length > 0 ? (
        <div className="space-y-2">
          {assets.slice(0, 4).map((asset) => (
            <RecordRow
              key={asset.id}
              label={asset.file_type || ee("ticketDetail.text073")}
              value={asset.file_name}
              meta={`${Math.max(1, Math.round(asset.file_size / 1024))} KB`}
            />
          ))}
        </div>
      ) : aggregate.meeting.title ? (
        <RecordRow
          label={ee("ticketDetail.text074")}
          value={aggregate.meeting.title}
          meta={aggregate.meeting.status || "waiting"}
        />
      ) : (
        <EmptyDetail text={ee("ticketDetail.text075")} />
      )}
    </DetailSection>
  )

  const partsSection = (
    <DetailSection
      icon={PackageIcon}
      title={ee("ticketDetail.text076")}
    >
      {repairParts.length > 0 ? (
        <div className="space-y-2">
          {repairParts.slice(0, 4).map((part, index) => (
            <RecordRow
              key={`${part.name}-${index}`}
              label={ee("ticketDetail.text077")}
              value={part.name}
              meta={ee("ticketDetail.text078", { value0: part.quantity })}
            />
          ))}
        </div>
      ) : repair.repair_id > 0 ? (
        <div className="space-y-2">
          <RecordRow
            label={ee("ticketDetail.text079")}
            value={repair.resolution || ee("ticketDetail.text080")}
            meta={repair.cost_hours ? `${repair.cost_hours}h` : ee("ticketDetail.text081")}
          />
        </div>
      ) : (
        <EmptyDetail text={ee("ticketDetail.text082")} />
      )}
    </DetailSection>
  )

  const feedbackSection = (
    <DetailSection
      icon={StarIcon}
      title={ee("ticketDetail.text083")}
    >
      {feedback ? (
        <div className="rounded-md bg-muted px-3 py-2 text-foreground">
          <div className="text-sm font-semibold">{feedback.rating} / 5</div>
          <p className="mt-1 text-xs leading-5">{feedback.comment || ee("ticketDetail.text084")}</p>
          {feedback.tags?.length ? (
            <div className="mt-2 flex flex-wrap gap-1">
              {feedback.tags.map((tag) => (
                <StatusTag key={tag} tone="neutral">{tag}</StatusTag>
              ))}
            </div>
          ) : null}
        </div>
      ) : (
        <div className="rounded-md bg-muted/50 px-3 py-2">
          <div className="text-sm font-semibold">
            {isTerminalTicketStatus(ticket.status) ? ee("ticketDetail.text085") : ee("ticketDetail.text086")}
          </div>
          <p className="mt-1 text-xs leading-5 text-muted-foreground">
            {isTerminalTicketStatus(ticket.status)
              ? ee("ticketDetail.text087")
              : ee("ticketDetail.text088")}
          </p>
        </div>
      )}
    </DetailSection>
  )

  return (
    <div className={cn("rhd-railops-ticket-detail-content", compact && "is-compact")}>
      <section className="rhd-railops-ticket-detail-hero rounded-lg border border-border bg-card p-4 lg:p-5">
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
              <span className="font-mono">{ticket.ticket_no}</span>
              <StatusTag
                tone="neutral"
                className={cn("h-5 px-1.5 text-rhd-xs", priorityClassName(ticket.priority))}
              >
                {ticketPriorityLabel(ticket.priority)}
              </StatusTag>
              <span>{formatDateTime(ticket.updated_at || ticket.created_at)}</span>
            </div>
            <h2 className="mt-2 line-clamp-2 text-base font-semibold leading-6 lg:text-lg">
              {ticket.title}
            </h2>
            {!compact ? (
              <p className="mt-3 line-clamp-2 max-w-5xl text-sm leading-6 text-muted-foreground">
                {serviceSummary(aggregate)}
              </p>
            ) : null}
          </div>
          <div className="flex shrink-0 flex-col items-end gap-2">
            <StatusTag
              tone="neutral"
              className={cn("h-6 px-2 text-xs", ticketStatusClassName(ticket.status))}
            >
              {ticketStatusLabel(ticket.status)}
            </StatusTag>
          </div>
        </div>
      </section>

      <section className="rhd-railops-ticket-detail-fields grid grid-cols-2 gap-3 text-xs lg:grid-cols-4">
        {fields.map(([label, value]) => (
          <InfoTile key={label} label={label} value={fieldValue(value)} />
        ))}
      </section>

      <div className="rhd-railops-ticket-detail-tabs">
        <UnderlineTabs
          ariaLabel={ee("ticketDetail.text089")}
          items={detailTabs}
          value={activeTab}
          onChange={(value) => setActiveTab(value as DetailTabKey)}
        />
      </div>

      <div className="rhd-railops-ticket-detail-tab-panel" role="tabpanel">
        {activeTab === "overview" ? (
          <div className="rhd-railops-ticket-detail-tab-grid grid gap-3 xl:grid-cols-[minmax(0,1.3fr)_minmax(22rem,0.9fr)]">
            <div className="space-y-3">
              <DetailSection
                icon={CheckCircle2Icon}
                title={ee("ticketDetail.text090")}
              >
                <p className="text-xs leading-5 text-muted-foreground">
                  {engineerConclusion(aggregate)}
                </p>
                <div className="mt-2 flex flex-wrap gap-1.5">
                  {[sourceLabel(ticket.source), channelLabel(ticket.channel), ticket.category]
                    .filter(Boolean)
                    .map((tag) => (
                      <span
                        key={tag}
                        className="rounded-md bg-muted px-2 py-1 text-rhd-xs text-muted-foreground"
                      >
                        {tag}
                      </span>
                    ))}
                </div>
              </DetailSection>

              {aggregate.diagnosis_snapshot ? (
                <DetailSection
                  icon={AlertTriangleIcon}
                  title={ee("ticketDetail.text091")}
                >
                  <div className="space-y-2 text-xs leading-5 text-muted-foreground">
                    <div className="flex flex-wrap gap-2">
                      <StatusTag tone="neutral">{aggregate.diagnosis_snapshot.fault_category || ee("ticketDetail.text092")}</StatusTag>
                      <StatusTag tone="neutral">{ee("ticketDetail.text093")}{Math.round(aggregate.diagnosis_snapshot.confidence * 100)}%
                      </StatusTag>
                      <StatusTag tone="neutral">
                        {ticketPriorityLabel(aggregate.diagnosis_snapshot.triage_level)}
                      </StatusTag>
                    </div>
                    {aggregate.diagnosis_snapshot.handoff_reason ? (
                      <p>{aggregate.diagnosis_snapshot.handoff_reason}</p>
                    ) : null}
                  </div>
                </DetailSection>
              ) : null}
            </div>
            <div className="space-y-3">
              {assignmentSection}
            </div>
          </div>
        ) : null}

        {activeTab === "progress" ? (
          <div className="rhd-railops-ticket-detail-tab-grid grid gap-3 xl:grid-cols-2">
            <DetailSection
              icon={ClipboardListIcon}
              title={ee("ticketDetail.text094")}
            >
              <div className="rhd-railops-ticket-detail-scroll-body">
                <TimelineList items={timeline} fallbackSteps={aggregate.flow.steps} showDeviceContext={showDeviceContext} />
              </div>
            </DetailSection>

            <DetailSection
              icon={UserCheckIcon}
              title={ee("ticketDetail.text095")}
            >
              <div className="rhd-railops-ticket-detail-scroll-body">
                {progressRecords.length > 0 ? (
                  <div className="space-y-2">
                    {progressRecords.map((item) => (
                      <RecordRow
                        key={`${item.type}-${item.id}`}
                        label={formatDateTime(item.timestamp)}
                        value={timelineContent(item.content, showDeviceContext)}
                        meta={item.actor || ee("ticketDetail.text096")}
                      />
                    ))}
                  </div>
                ) : (
                  <EmptyDetail text={ee("ticketDetail.text097")} />
                )}
              </div>
            </DetailSection>
          </div>
        ) : null}

        {activeTab === "context" ? (
          <div className="rhd-railops-ticket-detail-tab-grid grid gap-3 xl:grid-cols-3">
            <DetailSection
              icon={showDeviceContext ? PackageIcon : UserCheckIcon}
              title={ee(showDeviceContext ? "ticketDetail.text098" : "ticketDetail.text104")}
            >
              <div className="space-y-2">
                <RecordRow
                  label={ee("ticketDetail.text051")}
                  value={customer.name || ee("ticketDetail.text099")}
                  meta={customer.email || customer.contact}
                />
                {showDeviceContext ? (
                  <>
                    <RecordRow
                      label={ee("ticketDetail.text052")}
                      value={device.device_no || device.serial_no || ee("ticketDetail.text100")}
                      meta={device.model_name || device.product_code}
                    />
                    <RecordRow
                      label={ee("ticketDetail.text053")}
                      value={device.product_name || device.product_code || ee("ticketDetail.text101")}
                      meta={ticket.category || undefined}
                    />
                  </>
                ) : null}
              </div>
            </DetailSection>
            {conversationSection}
            {assignmentSection}
          </div>
        ) : null}

        {activeTab === "records" ? (
          <div className="rhd-railops-ticket-detail-tab-grid grid gap-3 xl:grid-cols-3">
            {assetsSection}
            {partsSection}
            {feedbackSection}
          </div>
        ) : null}
      </div>
    </div>
  )
}

function DetailSection({
  icon: Icon,
  title,
  children,
}: {
  icon: LucideIcon
  title: string
  children: ReactNode
}) {
  return (
    <section className="rhd-railops-ticket-detail-section rounded-lg border border-border bg-card p-4">
      <div className="mb-2 flex items-start gap-2">
        <span className="rhd-railops-ticket-detail-section-icon">
          <Icon className="size-3.5 shrink-0" />
        </span>
        <div className="min-w-0">
          <div className="truncate text-sm font-semibold">{title}</div>
        </div>
      </div>
      {children}
    </section>
  )
}

function InfoTile({ label, value }: { label: string; value: string }) {
  return (
    <div className="rhd-railops-ticket-detail-info-tile min-w-0 rounded-md bg-muted/50 px-3 py-2.5">
      <div className="text-rhd-xs text-muted-foreground">{label}</div>
      <div className="mt-1 truncate text-sm font-medium text-foreground" title={value}>
        {value}
      </div>
    </div>
  )
}

function RecordRow({
  label,
  value,
  meta,
}: {
  label: string
  value: string
  meta?: string
}) {
  return (
    <div className="rhd-railops-ticket-detail-record flex min-w-0 items-start justify-between gap-3 rounded-md bg-muted/45 px-3 py-2.5 text-sm">
      <div className="min-w-0">
        <div className="text-rhd-xs text-muted-foreground">{label}</div>
        <div className="mt-1 break-words font-medium leading-5 text-foreground" title={value}>
          {value || "-"}
        </div>
      </div>
      {meta ? (
        <div className="max-w-28 shrink-0 truncate text-right text-rhd-xs text-muted-foreground">
          {meta}
        </div>
      ) : null}
    </div>
  )
}

function EmptyDetail({ text }: { text: string }) {
  return (
    <div className="rounded-md border border-dashed border-border px-3 py-3 text-sm text-muted-foreground">
      {text}
    </div>
  )
}

function TimelineList({
  items,
  fallbackSteps,
  showDeviceContext,
}: {
  items: TicketAggregateDTO["timeline"]
  fallbackSteps: TicketFlowStepDTO[]
  showDeviceContext: boolean
}) {
  if (items.length > 0) {
    return (
      <div className="space-y-2">
        {items.map((item, index) => {
          const isLast = index === items.length - 1
          return (
            <div key={`${item.type}-${item.id}-${index}`} className="flex gap-3">
              <div className="flex w-8 shrink-0 flex-col items-center">
                <span
                  className={cn(
                    "flex size-6 items-center justify-center rounded-full border text-rhd-2xs font-semibold",
                    isLast
                      ? "border-primary bg-primary text-primary-foreground"
                    : "border-primary/20 bg-primary/10 text-primary",
                  )}
                >
                  {String(index + 1).padStart(2, "0")}
                </span>
                {!isLast ? <span className="h-7 w-px bg-border" /> : null}
              </div>
              <div className="min-w-0 flex-1 pb-2">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="text-sm font-semibold">
                    {timelineContent(item.content, showDeviceContext)}
                  </span>
                  {isLast ? (
                    <StatusTag tone="neutral" className="h-5 px-1.5 text-rhd-xs">{ee("ticketDetail.text102")}</StatusTag>
                  ) : null}
                </div>
                <div className="mt-0.5 flex flex-wrap gap-2 text-rhd-xs text-muted-foreground">
                  <span>{formatDateTime(item.timestamp)}</span>
                  <span>{item.actor || ee("ticketDetail.text096")}</span>
                </div>
              </div>
            </div>
          )
        })}
      </div>
    )
  }

  return (
    <div className="space-y-2">
      {fallbackSteps.map((step) => (
        <div key={step.name} className="flex items-center justify-between gap-2">
          <div className="flex min-w-0 items-center gap-2">
            <CircleDotIcon className="size-3.5 shrink-0 text-muted-foreground" />
            <span className="truncate text-xs font-medium">{stepLabel(step.name)}</span>
          </div>
          <StatusTag
            tone="neutral"
            className={cn("h-5 px-1.5 text-rhd-xs", progressStateClass(step.status))}
          >
            {step.status === "done" ? ee("ticketDetail.text017") : step.status === "current" ? ee("ticketDetail.text102") : ee("ticketDetail.text103")}
          </StatusTag>
        </div>
      ))}
    </div>
  )
}
