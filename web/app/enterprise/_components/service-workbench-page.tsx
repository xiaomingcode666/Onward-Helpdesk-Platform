"use client"

import {
  AlertTriangleIcon,
  ArrowRightIcon,
  CalendarClockIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  Clock3Icon,
  HeadphonesIcon,
  KeyRoundIcon,
  Loader2Icon,
  RefreshCwIcon,
  ShieldCheckIcon,
  TicketCheckIcon,
  TrendingUpIcon,
  TriangleAlertIcon,
  UsersRoundIcon,
  VideoIcon,
} from "lucide-react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import type { CSSProperties, MouseEvent as ReactMouseEvent, ReactNode } from "react"
import { useCallback, useEffect, useMemo, useState } from "react"
import { Skeleton } from "antd"
import { ContentModule, FilterTabs, PageShell, RailopsButton, StatusTag, type StatusTagTone } from "@railops/ui"

import { useAuth } from "@/components/auth-provider"
import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import { EmptyState, ErrorState } from "@/components/shared/error-states"
import { useAppLocale, useI18n } from "@/i18n/provider"
import { getEnterpriseAIKeyDisplayName, getEnterpriseAIKeyProductMeta } from "@/lib/enterprise-ai-key-display"
import {
  fetchReportTopFailures,
  fetchReportsOverview,
  type DashboardOverview,
  type ProductFailureRank,
} from "@/lib/api/enterprise-reports"
import {
  getEnterpriseAIModelsWorkspace,
  type EnterpriseAIKey,
  type EnterpriseAIModelWorkspace,
  type EnterpriseAIUsageTrendPoint,
} from "@/lib/api/enterprise-models"
import {
  fetchEnterpriseWorkbenchCollaboration,
  fetchEnterpriseWorkbenchCore,
  fetchEnterpriseWorkbenchQueue,
  fetchEnterpriseWorkbenchResources,
} from "@/lib/api/enterprise-workbench"
import { fetchPersonalMeetings } from "@/lib/api/meetings"
import type {
  EnterpriseWorkbenchAlert,
  EnterpriseWorkbenchCollaboration,
  EnterpriseWorkbenchCore,
  EnterpriseWorkbenchQueue,
  EnterpriseWorkbenchProductLoad,
  EnterpriseWorkbenchQueueCard,
  EnterpriseWorkbenchResources,
  MeetingListItem,
  TicketListItem,
} from "@/lib/api/types"
import { cn } from "@/lib/utils"
import { buildEnterpriseProductPath } from "@/lib/enterprise-detail-route"

const WORKBENCH_QUEUE_PAGE_SIZE = 8
const UPCOMING_MEETING_LIMIT = 8
const API_KEY_USAGE_PANEL_LIMIT = 3
const API_KEY_USAGE_TIMEOUT_MS = 15000

type ToneVariant = "soft" | "text" | "border"
type Translate = ReturnType<typeof useI18n>
type WorkbenchView = "overview" | "queue" | "insights" | "resources"

const WORKBENCH_VIEW_ROUTES: Record<WorkbenchView, string> = {
  overview: "/enterprise",
  queue: "/enterprise/workbench/tasks",
  insights: "/enterprise/workbench/insights",
  resources: "/enterprise/workbench/resources",
}

function toneClass(tone?: string, variant: ToneVariant = "soft") {
  const normalized = tone === "critical" ? "red" : tone === "warning" ? "amber" : tone === "green" ? "success" : tone === "success" ? "success" : tone
  const palette: Record<string, Record<ToneVariant, string>> = {
    red: {
      soft: "border-destructive/20 bg-destructive/10 text-destructive",
      text: "text-destructive",
      border: "border-destructive/30",
    },
    amber: {
      soft: "border-amber-200 bg-amber-50 text-foreground",
      text: "text-foreground",
      border: "border-amber-300",
    },
    success: {
      soft: "border-[#bbf7d0] bg-[var(--railops-success-bg)] text-[#047857]",
      text: "text-[#047857]",
      border: "border-[#86efac]",
    },
    blue: {
      soft: "border-primary/20 bg-primary/10 text-primary",
      text: "text-primary",
      border: "border-primary/30",
    },
    info: {
      soft: "border-primary/20 bg-primary/10 text-primary",
      text: "text-primary",
      border: "border-primary/30",
    },
    slate: {
      soft: "border-border bg-muted text-muted-foreground",
      text: "text-muted-foreground",
      border: "border-border",
    },
  }
  return (palette[normalized || "slate"] || palette.slate)[variant]
}

function toneBarClass(tone?: string) {
  const normalized = tone === "critical" ? "red" : tone === "warning" ? "amber" : tone === "green" ? "success" : tone === "success" ? "success" : tone
  if (normalized === "red") return "bg-destructive"
  if (normalized === "amber") return "bg-amber-500"
  if (normalized === "success") return "bg-[var(--railops-success)]"
  if (normalized === "blue" || normalized === "info") return "bg-primary"
  return "bg-muted-foreground"
}

function statusTone(tone?: string): StatusTagTone {
  const normalized = tone === "critical" ? "red" : tone === "warning" ? "amber" : tone === "green" ? "success" : tone === "success" ? "success" : tone
  if (normalized === "red") return "error"
  if (normalized === "amber") return "warning"
  if (normalized === "success") return "success"
  if (normalized === "blue" || normalized === "info") return "blue"
  return "neutral"
}

function statusLabel(status: string | undefined, t: Translate) {
  const keyByStatus: Record<string, string> = {
    pending: "pending",
    pending_acceptance: "pending",
    accepted: "accepted",
    pending_dispatch: "pendingDispatch",
    assigned: "pendingDispatch",
    pending_assignee_accept: "pendingAssigneeAccept",
    in_progress: "processing",
    processing: "processing",
    video_support: "videoSupport",
    supplier_support: "supplierSupport",
    waiting_customer: "waitingCustomer",
    resolved: "waitingCustomer",
    pending_customer_confirm: "waitingCustomer",
    closed: "closed",
    done: "closed",
    reopened: "reopened",
    cancelled: "cancelled",
  }
  const key = status ? keyByStatus[status] : ""
  if (key) return t(`enterpriseWorkbench.status.${key}`)
  return status || "-"
}

function meetingStatusLabel(status: string | undefined, t: Translate) {
  switch (status) {
    case "scheduled":
      return t("enterpriseWorkbench.status.scheduled")
    case "waiting":
      return t("enterpriseWorkbench.status.waiting")
    case "active":
      return t("enterpriseWorkbench.status.active")
    case "ended":
    case "finished":
      return t("enterpriseWorkbench.status.ended")
    default:
      return status || "-"
  }
}

function meetingStatusTone(status: string | undefined): StatusTagTone {
  if (status === "active") return "success"
  if (status === "scheduled") return "blue"
  if (status === "waiting") return "warning"
  return "neutral"
}

function priorityLabel(priority?: string) {
  switch (priority) {
    case "critical":
      return "P0"
    case "high":
      return "P1"
    case "medium":
      return "P2"
    default:
      return "P3"
  }
}

function priorityTone(priority?: string) {
  if (priority === "critical") return "red"
  if (priority === "high") return "amber"
  if (priority === "medium") return "blue"
  return "slate"
}

function dateText(value: string | undefined, locale: string) {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString(locale, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  })
}

function meetingScheduleValue(meeting: MeetingListItem) {
  return meeting.scheduled_at || meeting.started_at || meeting.created_at
}

function meetingScheduleTimestamp(meeting: MeetingListItem) {
  const value = meetingScheduleValue(meeting)
  const date = value ? new Date(value) : null
  const time = date && !Number.isNaN(date.getTime()) ? date.getTime() : 0
  return time || Number.MAX_SAFE_INTEGER
}

function formatMeetingClock(value: string | undefined, locale: string) {
  if (!value) return "--:--"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleTimeString(locale, {
    hour: "2-digit",
    minute: "2-digit",
  })
}

function meetingCalendarParts(value: string | undefined, locale: string) {
  if (!value) {
    return {
      key: "unscheduled",
      monthDay: "--",
      weekday: "--",
    }
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return {
      key: value,
      monthDay: value,
      weekday: "--",
    }
  }
  return {
    key: formatDateOnly(date),
    monthDay: date.toLocaleDateString(locale, { month: "2-digit", day: "2-digit" }),
    weekday: date.toLocaleDateString(locale, { weekday: "short" }),
  }
}

function formatAmount(value?: number, currency?: string) {
  const amount = Number(value || 0)
  const unit = currency || "USD"
  if (amount >= 1000) return `${amount.toFixed(0)} ${unit}`
  return `${amount.toFixed(2)} ${unit}`
}

function formatPercent(value?: number) {
  return `${Math.round(Number(value || 0))}%`
}

function formatInteger(value?: number | null) {
  return typeof value === "number" && Number.isFinite(value) ? value.toLocaleString() : "—"
}

function formatDecimalPercent(value?: number | null) {
  return typeof value === "number" && Number.isFinite(value) ? `${value.toFixed(1)}%` : "—"
}

function formatHours(value?: number | null) {
  if (typeof value !== "number" || !Number.isFinite(value)) return "—"
  return `${value.toFixed(1)}h`
}

function padDatePart(value: number) {
  return String(value).padStart(2, "0")
}

function formatDateOnly(date: Date) {
  return `${date.getFullYear()}-${padDatePart(date.getMonth() + 1)}-${padDatePart(date.getDate())}`
}

function formatMonthDay(value: string | undefined, locale: string) {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleDateString(locale, {
    month: "2-digit",
    day: "2-digit",
  })
}

function currentMonthReportRange() {
  const now = new Date()
  const start = new Date(now)
  start.setDate(1)
  return {
    start: formatDateOnly(start),
    end: formatDateOnly(now),
  }
}

function ticketOwnerText(ticket: TicketListItem, t: Translate, hasDeviceConcept = true) {
  if (ticket.assignee_name) return ticket.assignee_name
  if (ticket.team_name && ticket.team_name !== ticket.product_name) return ticket.team_name
  if (ticket.current_team_id > 0) return t(hasDeviceConcept ? "enterpriseWorkbench.ticket.productTeamPending" : "enterpriseWorkbench.ticket.supportTeamPending")
  return t("enterpriseWorkbench.ticket.unassigned")
}

function productTeamBadgeText(item: EnterpriseWorkbenchProductLoad, t: Translate) {
  if (!item.team_name) return t("enterpriseWorkbench.product.noTeam")
  if (item.team_name === item.product_name) return t("enterpriseWorkbench.product.repairTeam")
  return item.team_name
}

function queueTitle(queue: EnterpriseWorkbenchQueueCard, t: Translate, hasDeviceConcept = true) {
  const key = queue.key === "unassigned" && !hasDeviceConcept
    ? "enterpriseWorkbench.queue.unassigned.generalTitle"
    : `enterpriseWorkbench.queue.${queue.key}.title`
  const translated = t(key)
  return translated === key ? queue.title : translated
}

function alertTitle(alert: EnterpriseWorkbenchAlert, t: Translate) {
  const key = `enterpriseWorkbench.alerts.${alert.key}`
  const translated = t(key)
  return translated === key ? alert.title : translated
}

type TicketSummaryItem = {
  href?: string
  icon: ReactNode
  label: string
  value: string | number
  tone: string
}

function TicketSummaryPanel({
  error = "",
  items,
  loading = false,
  onRetry,
}: {
  error?: string
  items: TicketSummaryItem[]
  loading?: boolean
  onRetry?: () => void
}) {
  const t = useI18n()
  return (
    <InsightShell
      actionHref="/enterprise/tickets"
      icon={<TicketCheckIcon className="size-4" />}
      title={t("enterpriseWorkbench.ticketSummary.title")}
    >
      {loading ? (
        <div
          className="grid grid-cols-2 gap-px overflow-hidden rounded-md border border-border/70 bg-border/40"
          role="status"
          aria-busy="true"
          aria-label={t("enterpriseWorkbench.ticketSummary.loading")}
        >
          {Array.from({ length: 4 }, (_, index) => (
            <div key={index} className="bg-card px-2 py-2.5">
              <Skeleton.Node active style={{ width: 64, height: 20 }} />
              <Skeleton.Node active className="mt-2" style={{ width: 96, height: 12 }} />
            </div>
          ))}
        </div>
      ) : error ? (
        <ModuleError message={t("enterpriseWorkbench.errors.ticketSummaryUnavailable", { error })} onRetry={onRetry || (() => undefined)} />
      ) : (
        <div className="grid grid-cols-2 gap-px overflow-hidden rounded-md border border-border/70 bg-border/40">
          {items.map((item) => (
          <Link
            key={item.label}
            href={item.href || "/enterprise/tickets"}
            className="group relative min-w-0 bg-card px-2 py-2.5 no-underline transition-colors hover:bg-muted/60"
          >
            <span className={cn("absolute inset-y-2 left-0 w-0.5 rounded-r", toneBarClass(item.tone))} />
            <span className="flex min-w-0 items-center gap-1.5">
              <strong className="text-lg font-semibold text-foreground">{item.value}</strong>
              <span className="truncate text-rhd-xs font-medium text-muted-foreground">{item.label}</span>
            </span>
          </Link>
          ))}
        </div>
      )}
    </InsightShell>
  )
}

function AttentionBanner({ alerts, className }: { alerts: EnterpriseWorkbenchAlert[]; className?: string }) {
  const t = useI18n()
  if (!alerts.length) return null
  const alert = alerts[0]
  const hasCritical = alert.severity === "critical"
  return (
    <Link
      href={alert.action_url || "/enterprise/tickets"}
      className={cn(
        "group flex min-w-0 items-center gap-2 rounded-md border px-3 py-2 no-underline transition-colors",
        hasCritical
          ? "border-destructive/20 bg-destructive/10 hover:bg-destructive/15"
          : "border-border bg-muted hover:bg-muted/80",
        className,
      )}
      aria-label={t("enterpriseWorkbench.attention.aria")}
    >
      <AlertTriangleIcon className={cn("size-4 shrink-0", hasCritical ? "text-destructive" : "text-primary")} />
      <span className={cn("shrink-0 text-xs font-semibold", hasCritical ? "text-destructive" : "text-foreground")}>
        {t("enterpriseWorkbench.attention.label")}
      </span>
      <span className="min-w-0 flex-1 truncate text-xs text-foreground">
        <strong className="font-semibold">{t("enterpriseWorkbench.attention.count", { count: alert.count })}</strong> · {alertTitle(alert, t)}
      </span>
      {alerts.length > 1 ? (
        <span className="shrink-0 text-rhd-xs text-muted-foreground">{t("enterpriseWorkbench.attention.more", { count: alerts.length - 1 })}</span>
      ) : null}
      <ArrowRightIcon className="size-3.5 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5" />
    </Link>
  )
}

function InsightShell({
  actionHref,
  children,
  icon,
  title,
}: {
  actionHref?: string
  children: ReactNode
  icon: ReactNode
  title: string
}) {
  const t = useI18n()
  return (
    <ContentModule
      className="rhd-railops-workbench-insight"
      title={(
        <span className="rhd-railops-workbench-insight-title">
          <span>
            {icon}
          </span>
          {title}
        </span>
      )}
      extra={actionHref ? (
        <Link href={actionHref} className="shrink-0 text-xs font-medium text-primary no-underline">
          {t("enterpriseWorkbench.common.details")}
        </Link>
      ) : null}
    >
      {children}
    </ContentModule>
  )
}

function qualityMetrics(overview: DashboardOverview | null, t: Translate, aiEnabled: boolean) {
  const slaAtRisk = overview?.sla_at_risk ?? 0
  const inProgress = overview?.in_progress_tickets ?? 0
  const aiResolveRate = typeof overview?.ai_resolve_rate === "number" ? overview.ai_resolve_rate : null
  const metrics = [
    {
      href: "/enterprise/tickets?sla_risk=true",
      label: t("enterpriseWorkbench.quality.slaRisk"),
      score: slaAtRisk > 0 ? Math.max(8, 100 - slaAtRisk * 18) : 100,
      value: formatInteger(overview?.sla_at_risk),
      status: slaAtRisk > 0 ? t("enterpriseWorkbench.quality.needsAction") : t("enterpriseWorkbench.quality.normal"),
      tone: slaAtRisk > 0 ? "red" : "green",
    },
    {
      href: "/enterprise/tickets",
      label: t("enterpriseWorkbench.quality.newToday"),
      score: 100,
      value: formatInteger(overview?.new_today),
      status: t("enterpriseWorkbench.quality.normal"),
      tone: "green",
    },
    {
      href: "/enterprise/tickets?status=processing",
      label: t("enterpriseWorkbench.quality.processing"),
      score: inProgress > 0 ? Math.max(16, 100 - inProgress * 8) : 100,
      value: formatInteger(overview?.in_progress_tickets),
      status: inProgress > 0 ? t("enterpriseWorkbench.quality.watch") : t("enterpriseWorkbench.quality.normal"),
      tone: inProgress > 0 ? "amber" : "green",
    },
  ]
  if (!aiEnabled) return metrics

  return [
    metrics[0],
    {
      href: "/enterprise/ai",
      label: t("enterpriseWorkbench.quality.aiResolveRate"),
      score: aiResolveRate === null ? 60 : Math.max(45, Math.min(100, aiResolveRate)),
      value: formatDecimalPercent(overview?.ai_resolve_rate),
      status: t("enterpriseWorkbench.quality.tracking"),
      tone: "blue",
    },
    metrics[1],
    metrics[2],
  ]
}

function ServiceQualityPanel({ aiEnabled, overview }: { aiEnabled: boolean; overview: DashboardOverview | null }) {
  const t = useI18n()
  const metrics = qualityMetrics(overview, t, aiEnabled)
  const healthScore = Math.round(metrics.reduce((sum, item) => sum + item.score, 0) / Math.max(1, metrics.length))
  const healthTone = healthScore >= 85 ? "success" : healthScore >= 70 ? "amber" : "red"
  return (
    <InsightShell
      actionHref="/enterprise/reports"
      icon={<TrendingUpIcon className="size-4" />}
      title={t("enterpriseWorkbench.quality.title")}
    >
      <div className="rhd-railops-service-quality-panel">
        <Link href="/enterprise/reports" className="rhd-railops-service-quality-ring no-underline" style={{
          "--rhd-quality-score": `${healthScore}%`,
          "--rhd-quality-tone": healthTone === "success" ? "var(--railops-success)" : healthTone === "amber" ? "var(--railops-warning)" : "var(--railops-error)",
        } as CSSProperties}>
          <span>
            <strong>{healthScore}</strong>
            <small>{t("enterpriseWorkbench.quality.healthScore")}</small>
          </span>
        </Link>
        <div className="rhd-railops-service-quality-list">
          {metrics.map((item) => (
            <Link key={item.label} href={item.href} className="rhd-railops-service-quality-row no-underline">
              <span className="rhd-railops-service-quality-row-head">
                <span className="truncate">{item.label}</span>
                <StatusTag tone={statusTone(item.tone)}>{item.status}</StatusTag>
              </span>
              <span className="rhd-railops-service-quality-row-body">
                <strong>{item.value}</strong>
                <i aria-hidden="true">
                  <b className={cn(`tone-${item.tone}`)} style={{ width: `${Math.min(100, Math.max(4, item.score))}%` }} />
                </i>
              </span>
            </Link>
          ))}
        </div>
      </div>
    </InsightShell>
  )
}

function FailureDistributionPanel({ items }: { items: ProductFailureRank[] }) {
  const t = useI18n()
  const maxCount = Math.max(...items.map((item) => item.failure_count || 0), 1)
  return (
    <InsightShell
      actionHref="/enterprise/reports"
      icon={<TriangleAlertIcon className="size-4" />}
      title={t("enterpriseWorkbench.failures.title")}
    >
      {items.length > 0 ? (
        <div className="divide-y divide-border/70">
          {items.slice(0, 2).map((item) => {
            const tone = item.escalation_rate >= 50 ? "red" : item.escalation_rate >= 20 ? "amber" : "blue"
            return (
              <Link
                key={`${item.product_id}-${item.product_name}`}
                href={item.product_id ? buildEnterpriseProductPath(item.product_id) : "/enterprise/reports"}
                className="block px-2 py-2.5 no-underline transition-colors hover:bg-muted/60"
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <div className="truncate text-sm font-semibold text-foreground" title={item.product_name || undefined}>
                      {item.product_name || t("enterpriseWorkbench.failures.productFallback", { id: item.product_id || t("enterpriseWorkbench.common.unbound") })}
                    </div>
                    <div className="mt-1 text-xs text-muted-foreground">
                      {t("enterpriseWorkbench.failures.metrics", {
                        escalation: formatDecimalPercent(item.escalation_rate),
                        hours: formatHours(item.avg_resolution_hours),
                      })}
                    </div>
                  </div>
                  <StatusTag tone={statusTone(tone)}>
                    {t("enterpriseWorkbench.failures.count", { count: formatInteger(item.failure_count) })}
                  </StatusTag>
                </div>
                <div className="mt-2 h-1 overflow-hidden rounded-full bg-muted">
                  <div
                    className={cn("h-full rounded-full", tone === "red" ? "bg-destructive" : tone === "amber" ? "bg-amber-500" : "bg-primary")}
                    style={{ width: `${Math.min(100, Math.max(8, ((item.failure_count || 0) / maxCount) * 100))}%` }}
                  />
                </div>
              </Link>
            )
          })}
        </div>
      ) : (
        <p className="rounded-md border border-dashed border-border bg-muted p-6 text-center text-sm text-muted-foreground">
          {t("enterpriseWorkbench.failures.empty")}
        </p>
      )}
    </InsightShell>
  )
}

function topUsageKeys(keys: EnterpriseAIKey[]) {
  return [...keys]
    .sort((left, right) => (
      (right.quotaUsed || 0) - (left.quotaUsed || 0)
      || (right.usage7d || 0) - (left.usage7d || 0)
      || (right.usage1d || 0) - (left.usage1d || 0)
      || (right.usage5h || 0) - (left.usage5h || 0)
      || right.id - left.id
    ))
    .slice(0, 3)
}

function keyUsagePercent(value?: number) {
  return Math.min(100, Math.max(0, Number(value || 0)))
}

function keyUsageColorClass(value?: number, variant: "bar" | "text" = "bar") {
  const percent = keyUsagePercent(value)
  if (percent < 40) return variant === "bar" ? "bg-[var(--railops-success)]" : "text-[#047857]"
  if (percent < 80) return variant === "bar" ? "bg-amber-500" : "text-foreground"
  return variant === "bar" ? "bg-destructive" : "text-destructive"
}

function withTimeout<T>(promise: Promise<T>, timeoutMs: number, message: string): Promise<T> {
  let timer: number | undefined
  const timeout = new Promise<never>((_, reject) => {
    timer = window.setTimeout(() => reject(new Error(message)), timeoutMs)
  })
  return Promise.race([promise, timeout]).finally(() => {
    if (timer) window.clearTimeout(timer)
  })
}

function isPermissionDeniedMessage(error: string) {
  const message = error.trim().toLowerCase()
  return Boolean(message)
    && (
      message.includes("无权限")
      || message.includes("没有权限")
      || message.includes("do not have permission")
      || message.includes("permission denied")
      || message.includes("forbidden")
    )
}

function APIKeyUsagePanel({
  error,
  loading,
  onRetry,
  workspace,
}: {
  error: string
  loading: boolean
  onRetry: () => void
  workspace: EnterpriseAIModelWorkspace | null
}) {
  const t = useI18n()
  const keys = topUsageKeys(workspace?.keys || [])

  if (!loading && isPermissionDeniedMessage(error)) return null

  return (
    <InsightShell
      actionHref="/enterprise/models"
      icon={<KeyRoundIcon className="size-4" />}
      title={t("enterpriseWorkbench.apiKeys.title")}
    >
      {loading ? (
        <div role="status" aria-busy="true" aria-label={t("enterpriseWorkbench.apiKeys.loading")}>
          <div className="mb-2 flex items-center gap-2 rounded-md border border-dashed border-border bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
            <Loader2Icon className="size-3.5 animate-spin text-primary" />
            <span>{t("enterpriseWorkbench.apiKeys.loading")}</span>
          </div>
          <div className="divide-y divide-border/70">
            {Array.from({ length: 3 }, (_, index) => (
              <div key={index} className="grid gap-2 px-2 py-3 sm:grid-cols-[minmax(0,1fr)_minmax(180px,1fr)] sm:items-center">
                <Skeleton.Node active style={{ width: 112, height: 16 }} />
                <div className="space-y-2">
                  <div className="flex justify-between gap-3">
                    <Skeleton.Node active style={{ width: 56, height: 12 }} />
                    <Skeleton.Node active style={{ width: 36, height: 12 }} />
                  </div>
                  <Skeleton.Node active className="w-full" style={{ width: "100%", height: 6 }} />
                </div>
              </div>
            ))}
          </div>
        </div>
      ) : error ? (
        <ModuleError message={t("enterpriseWorkbench.errors.apiKeysUnavailable", { error })} onRetry={onRetry} />
      ) : keys.length > 0 ? (
        <div className="divide-y divide-border/70">
          {keys.map((key) => {
            const usagePercent = keyUsagePercent(key.quotaUsagePercent)
            const keyDisplayName = getEnterpriseAIKeyDisplayName(key)
            const keyProductMeta = getEnterpriseAIKeyProductMeta(key)
            const keyTitle = [keyDisplayName, keyProductMeta].filter(Boolean).join(" · ")
            return (
              <Link
                key={key.id}
                href="/enterprise/models"
                className="grid gap-2 px-2 py-3 no-underline transition-colors hover:bg-muted/60 sm:grid-cols-[minmax(0,1fr)_minmax(180px,1fr)] sm:items-center"
              >
                <span className="min-w-0">
                  <strong className="block truncate text-xs font-semibold text-foreground" title={keyTitle || undefined}>
                    {keyDisplayName}
                  </strong>
                  {keyProductMeta ? <span className="mt-0.5 block truncate text-rhd-xs text-muted-foreground">{keyProductMeta}</span> : null}
                </span>
                <span className="min-w-0">
                  <span className="flex items-center justify-between gap-2 text-rhd-xs text-muted-foreground">
                    <span>{t("enterpriseWorkbench.apiKeys.usage")}</span>
                    <strong className={keyUsageColorClass(usagePercent, "text")}>{Math.round(usagePercent)}%</strong>
                  </span>
                  <span className="mt-1.5 block h-1.5 overflow-hidden rounded-full bg-muted">
                    <span
                      className={cn("block h-full rounded-full", keyUsageColorClass(usagePercent))}
                      style={{ width: `${usagePercent}%` }}
                    />
                  </span>
                </span>
              </Link>
            )
          })}
        </div>
      ) : (
        <p className="rounded-md border border-dashed border-border bg-muted p-6 text-center text-sm text-muted-foreground">
          {t("enterpriseWorkbench.apiKeys.empty")}
        </p>
      )}
    </InsightShell>
  )
}

function ReportPanelState({
  children,
  error,
  hidePermissionDenied = false,
  loading,
  onRetry,
  title,
}: {
  children: ReactNode
  error: string
  hidePermissionDenied?: boolean
  loading: boolean
  onRetry: () => void
  title: string
}) {
  const t = useI18n()
  if (loading) return <PanelLoading title={title} label={t("common.loadingData")} rows={4} />
  if (hidePermissionDenied && isPermissionDeniedMessage(error)) return null
  if (error) {
    return (
      <section className="rounded-lg border border-border bg-card">
        <div className="border-b border-border px-4 py-3 text-sm font-semibold text-foreground">{title}</div>
        <ModuleError message={t("enterpriseWorkbench.errors.sectionUnavailable", { title, error })} onRetry={onRetry} />
      </section>
    )
  }
  return children
}

function PanelLoading({
  label,
  rows = 3,
  title,
}: {
  label: string
  rows?: number
  title: string
}) {
  return (
    <section className="h-full rounded-lg border border-border bg-card" role="status" aria-busy="true" aria-label={label}>
      <div className="flex items-center justify-between gap-3 border-b border-border px-4 py-3">
        <h2 className="truncate text-sm font-semibold text-foreground">{title}</h2>
        <Loader2Icon className="size-3.5 shrink-0 animate-spin text-muted-foreground" aria-hidden="true" />
      </div>
      <PanelSkeleton rows={rows} status={false} />
    </section>
  )
}

function WorkbenchReportInsights({
  aiEnabled,
  failures,
  failuresError,
  failuresLoading,
  keyWorkspace,
  keyWorkspaceError,
  keyWorkspaceLoading,
  onRetryFailures,
  onRetryKeyWorkspace,
  ticketSummaryError,
  ticketSummaryItems,
  ticketSummaryLoading,
  onRetryTicketSummary,
}: {
  aiEnabled: boolean
  failures: ProductFailureRank[]
  failuresError: string
  failuresLoading: boolean
  keyWorkspace: EnterpriseAIModelWorkspace | null
  keyWorkspaceError: string
  keyWorkspaceLoading: boolean
  onRetryFailures: () => void
  onRetryKeyWorkspace: () => void
  ticketSummaryError: string
  ticketSummaryItems: TicketSummaryItem[]
  ticketSummaryLoading: boolean
  onRetryTicketSummary: () => void
}) {
  const t = useI18n()
  return (
    <section
      className="rhd-railops-workbench-insight-grid"
      aria-label={t("enterpriseWorkbench.reportInsights.aria")}
    >
      <ReportPanelState
        error={failuresError}
        loading={failuresLoading}
        onRetry={onRetryFailures}
        title={t("enterpriseWorkbench.failures.title")}
      >
        <FailureDistributionPanel items={failures} />
      </ReportPanelState>
      <TicketSummaryPanel
        error={ticketSummaryError}
        items={ticketSummaryItems}
        loading={ticketSummaryLoading}
        onRetry={onRetryTicketSummary}
      />
      {aiEnabled ? (
        <div className="rhd-railops-workbench-api-key-panel h-full">
          <APIKeyUsagePanel
            error={keyWorkspaceError}
            loading={keyWorkspaceLoading}
            onRetry={onRetryKeyWorkspace}
            workspace={keyWorkspace}
          />
        </div>
      ) : null}
    </section>
  )
}

type OverviewWidgetProps = {
  detail: string
  href: string
  icon: ReactNode
  label: string
  loading?: boolean
  tone: string
  value: string | number
}

function OverviewWidget({
  detail,
  href,
  icon,
  label,
  loading = false,
  tone,
  value,
}: OverviewWidgetProps) {
  return (
    <Link href={href} className="rhd-railops-enterprise-overview-widget no-underline">
      <span className={cn("rhd-railops-enterprise-overview-widget-icon", `tone-${tone}`)}>
        {icon}
      </span>
      <span className="min-w-0">
        <span className="block truncate text-rhd-xs font-medium text-muted-foreground">{label}</span>
        {loading ? (
          <Skeleton.Node active className="mt-2" style={{ width: 64, height: 24 }} />
        ) : (
          <strong className="mt-1 block truncate text-xl font-semibold text-foreground">{value}</strong>
        )}
        <small className="mt-1 block truncate text-rhd-xs text-muted-foreground">{detail}</small>
      </span>
    </Link>
  )
}

function DailyUsageTrendWidget({
  error,
  loading,
  onRetry,
  trend,
  usage,
}: {
  error: string
  loading: boolean
  onRetry: () => void
  trend: EnterpriseAIUsageTrendPoint[]
  usage?: EnterpriseWorkbenchResources["usage"]
}) {
  const t = useI18n()
  const { locale } = useAppLocale()
  const points = trend.slice(-7)
  const maxValue = Math.max(...points.map((item) => item.totalTokens || item.requests || item.actualCost || 0), 1)
  const totalRequests = points.reduce((sum, item) => sum + Number(item.requests || 0), 0)
  const totalTokens = points.reduce((sum, item) => sum + Number(item.totalTokens || 0), 0)
  const totalCost = points.reduce((sum, item) => sum + Number(item.actualCost || item.cost || 0), 0)
  const latest = points[points.length - 1]
  const usageTone = (usage?.usage_percent ?? 0) >= 90 ? "error" : (usage?.usage_percent ?? 0) >= 75 ? "warning" : "success"
  const [hoveredIndex, setHoveredIndex] = useState<number | null>(null)

  if (!loading && isPermissionDeniedMessage(error)) return null

  return (
    <section className="rhd-railops-enterprise-overview-trend">
      <header>
        <div className="min-w-0">
          <h2>{t("enterpriseWorkbench.overview.dailyUsageTrend")}</h2>
          <p>{points.length ? t("enterpriseWorkbench.overview.dailyUsageTrendMeta", {
            requests: formatInteger(totalRequests),
            tokens: formatInteger(totalTokens),
          }) : t("enterpriseWorkbench.overview.dailyUsageTrendEmpty")}</p>
        </div>
        <Link href="/enterprise/usage" className="shrink-0 text-xs font-medium text-primary no-underline">
          {t("enterpriseWorkbench.overview.usageCenter")}
        </Link>
      </header>
      {loading ? (
        <div className="rhd-railops-enterprise-overview-trend-loading" role="status" aria-busy="true" aria-label={t("common.loadingData")}>
          {Array.from({ length: 7 }, (_, index) => (
            <Skeleton.Node key={index} active style={{ height: "100%", minHeight: 40 }} />
          ))}
        </div>
      ) : error ? (
        <ModuleError message={t("enterpriseWorkbench.errors.sectionUnavailable", { title: t("enterpriseWorkbench.overview.dailyUsageTrend"), error })} onRetry={onRetry} />
      ) : points.length ? (
        <Link href="/enterprise/usage" className="rhd-railops-enterprise-overview-trend-body no-underline">
          <div className="rhd-railops-enterprise-overview-trend-chart" aria-hidden="true">
            {points.map((item, index) => {
              const value = item.totalTokens || item.requests || item.actualCost || 0
              const height = Math.max(12, Math.round((value / maxValue) * 100))
              return (
                <span
                  key={item.date}
                  className={cn(
                    "rhd-railops-enterprise-overview-trend-bar",
                    index === hoveredIndex && "is-hovered",
                    index === 0 && "is-first",
                    index === points.length - 1 && "is-last",
                  )}
                  onMouseEnter={() => setHoveredIndex(index)}
                  onMouseLeave={() => setHoveredIndex((current) => (current === index ? null : current))}
                >
                  <span style={{ height: `${height}%` }} />
                  {index === hoveredIndex ? (
                    <span className="rhd-railops-enterprise-overview-trend-tooltip" role="tooltip">
                      <strong>{formatMonthDay(item.date, locale)}</strong>
                      <span>{t("enterpriseWorkbench.overview.dailyUsageTrendHover", {
                        requests: formatInteger(item.requests),
                        tokens: formatInteger(item.totalTokens),
                        cost: formatAmount(item.actualCost || item.cost, usage?.currency || "USD"),
                      })}</span>
                    </span>
                  ) : null}
                </span>
              )
            })}
          </div>
          <div className="rhd-railops-enterprise-overview-trend-axis">
            <span>{formatMonthDay(points[0]?.date, locale)}</span>
            <span>{formatMonthDay(latest?.date, locale)}</span>
          </div>
          <div className="rhd-railops-enterprise-overview-trend-footer">
            <span>
              <strong>{t("enterpriseWorkbench.overview.dailyUsageToday", { requests: formatInteger(latest?.requests) })}</strong>
              <small>{t("enterpriseWorkbench.overview.dailyUsageCost", { cost: formatAmount(totalCost, usage?.currency || "USD") })}</small>
            </span>
            <StatusTag tone={usageTone}>{usage ? formatPercent(usage.usage_percent) : t("enterpriseWorkbench.page.notLoaded")}</StatusTag>
          </div>
        </Link>
      ) : (
        <Link href="/enterprise/usage" className="rhd-railops-enterprise-overview-trend-empty no-underline">
          <TrendingUpIcon className="size-5 text-primary" />
          <span>{t("enterpriseWorkbench.overview.dailyUsageTrendEmpty")}</span>
          <StatusTag tone={usageTone}>{usage ? formatPercent(usage.usage_percent) : t("enterpriseWorkbench.page.notLoaded")}</StatusTag>
        </Link>
      )}
    </section>
  )
}

function EnterpriseOverviewPanel({
  activeQueueKey,
  aiEnabled,
  hasDeviceConcept,
  collaboration,
  collaborationLoading,
  core,
  coreLoading,
  keyWorkspace,
  keyWorkspaceError,
  keyWorkspaceLoading,
  onRetryKeyWorkspace,
  onRetryOverview,
  onRetryMeetingSchedule,
  onSelectQueue,
  overview,
  overviewError,
  overviewLoading,
  queues,
  resources,
  resourcesLoading,
  upcomingMeetingError,
  upcomingMeetingLoading,
  upcomingMeetings,
  upcomingMeetingTotal,
}: {
  activeQueueKey: string
  aiEnabled: boolean
  hasDeviceConcept: boolean
  collaboration: EnterpriseWorkbenchCollaboration | null
  collaborationLoading: boolean
  core: EnterpriseWorkbenchCore | null
  coreLoading: boolean
  keyWorkspace: EnterpriseAIModelWorkspace | null
  keyWorkspaceError: string
  keyWorkspaceLoading: boolean
  onRetryKeyWorkspace: () => void
  onRetryOverview: () => void
  onRetryMeetingSchedule: () => void
  onSelectQueue: (key: string) => void
  overview: DashboardOverview | null
  overviewError: string
  overviewLoading: boolean
  queues: EnterpriseWorkbenchQueueCard[]
  resources: EnterpriseWorkbenchResources | null
  resourcesLoading: boolean
  upcomingMeetingError: string
  upcomingMeetingLoading: boolean
  upcomingMeetings: MeetingListItem[]
  upcomingMeetingTotal: number
}) {
  const t = useI18n()
  const summary = core?.summary
  const usage = resources?.usage
  const activeMeetings = collaboration?.active_meetings ?? summary?.active_meetings ?? 0
  const activeConversations = collaboration?.active_conversations ?? summary?.active_conversations ?? 0
  const totalProducts = resources?.total_products ?? summary?.total_products ?? overview?.total_products ?? 0
  const totalDevices = resources?.total_devices ?? summary?.total_devices ?? overview?.total_devices ?? 0
  const widgetLoading = coreLoading && !core

  return (
    <section className="rhd-railops-enterprise-overview space-y-4" role="tabpanel" aria-label={t("enterpriseWorkbench.views.overview")}>
      <ContentModule className="rhd-railops-enterprise-overview-hero">
        <div className="rhd-railops-enterprise-overview-widget-grid">
          <OverviewWidget
            detail={t("enterpriseWorkbench.overview.openTicketsMeta", {
              pending: summary?.pending_tickets ?? 0,
              processing: summary?.processing_tickets ?? 0,
            })}
            href="/enterprise/tickets"
            icon={<TicketCheckIcon className="size-4" />}
            label={t("enterpriseWorkbench.overview.openTickets")}
            loading={widgetLoading}
            tone="blue"
            value={summary?.open_tickets ?? overview?.pending_tickets ?? "—"}
          />
          <OverviewWidget
            detail={t("enterpriseWorkbench.overview.slaRiskMeta", { breached: summary?.sla_breached_tickets ?? 0 })}
            href="/enterprise/tickets?sla_risk=true"
            icon={<AlertTriangleIcon className="size-4" />}
            label={t("enterpriseWorkbench.overview.slaRisk")}
            loading={widgetLoading}
            tone={(summary?.sla_breached_tickets ?? 0) > 0 ? "red" : (summary?.sla_risk_tickets ?? 0) > 0 ? "amber" : "success"}
            value={summary?.sla_risk_tickets ?? overview?.sla_at_risk ?? "—"}
          />
          <OverviewWidget
            detail={t("enterpriseWorkbench.overview.collaborationMeta", {
              conversations: activeConversations,
              meetings: activeMeetings,
            })}
            href="/enterprise/video"
            icon={<VideoIcon className="size-4" />}
            label={t("enterpriseWorkbench.overview.collaboration")}
            loading={collaborationLoading && !collaboration}
            tone="blue"
            value={activeConversations + activeMeetings}
          />
          {hasDeviceConcept ? (
            <OverviewWidget
              detail={t("enterpriseWorkbench.overview.assetsMeta", { devices: totalDevices })}
              href="/enterprise/products"
              icon={<ShieldCheckIcon className="size-4" />}
              label={t("enterpriseWorkbench.overview.assets")}
              loading={(resourcesLoading && !resources) || widgetLoading}
              tone="success"
              value={totalProducts}
            />
          ) : null}
          {aiEnabled ? (
            <>
              <OverviewWidget
                detail={usage ? t("enterpriseWorkbench.overview.quotaUsed", { percent: formatPercent(usage.usage_percent) }) : t("enterpriseWorkbench.page.notLoaded")}
                href="/enterprise/usage"
                icon={<KeyRoundIcon className="size-4" />}
                label={t("enterpriseWorkbench.overview.quota")}
                loading={resourcesLoading && !resources}
                tone={(usage?.usage_percent ?? 0) >= 90 ? "red" : (usage?.usage_percent ?? 0) >= 75 ? "amber" : "success"}
                value={usage ? formatAmount(usage.quota_remaining, usage.currency) : "—"}
              />
              <OverviewWidget
                detail={t("enterpriseWorkbench.overview.aiSessions", { count: overview?.ai_sessions ?? 0 })}
                href="/enterprise/reports"
                icon={<TrendingUpIcon className="size-4" />}
                label={t("enterpriseWorkbench.overview.selfService")}
                loading={overviewLoading && !overview}
                tone="blue"
                value={formatDecimalPercent(overview?.ai_resolve_rate)}
              />
            </>
          ) : null}
        </div>
      </ContentModule>

      <div className="rhd-railops-enterprise-overview-analytics">
        <ReportPanelState
          error={overviewError}
          hidePermissionDenied
          loading={overviewLoading}
          onRetry={onRetryOverview}
          title={t("enterpriseWorkbench.quality.title")}
        >
          <ServiceQualityPanel aiEnabled={aiEnabled} overview={overview} />
        </ReportPanelState>
        <QueueDistributionPie
          hasDeviceConcept={hasDeviceConcept}
          loading={coreLoading && !core}
          onSelect={onSelectQueue}
          queues={queues}
          selectedKey={activeQueueKey}
        />
        {aiEnabled ? (
          <DailyUsageTrendWidget
            error={keyWorkspaceError}
            loading={keyWorkspaceLoading && !keyWorkspace}
            onRetry={onRetryKeyWorkspace}
            trend={keyWorkspace?.trend || []}
            usage={usage}
          />
        ) : null}
      </div>

      <UpcomingMeetingSchedulePanel
        error={upcomingMeetingError}
        loading={upcomingMeetingLoading}
        meetings={upcomingMeetings}
        onRetry={onRetryMeetingSchedule}
        total={upcomingMeetingTotal}
      />
    </section>
  )
}

function TicketQueueTable({ tickets, hasDeviceConcept }: { tickets: TicketListItem[]; hasDeviceConcept: boolean }) {
  const t = useI18n()
  const { locale } = useAppLocale()
  return (
    <div className="rhd-railops-workbench-ticket-table-wrap">
      <table className="rhd-railops-workbench-ticket-table">
        <thead>
          <tr>
            <th>{t("enterpriseWorkbench.ticketTable.ticket")}</th>
            <th>{t(hasDeviceConcept ? "enterpriseWorkbench.ticketTable.customerDevice" : "enterpriseWorkbench.ticketTable.customer")}</th>
            <th>{t(hasDeviceConcept ? "enterpriseWorkbench.ticketTable.productOwner" : "enterpriseWorkbench.ticketTable.handling")}</th>
            <th>{t("enterpriseWorkbench.ticketTable.priority")}</th>
            <th>{t("enterpriseWorkbench.ticketTable.status")}</th>
            <th>{t("enterpriseWorkbench.ticketTable.updatedAt")}</th>
            <th>{t("enterpriseWorkbench.ticketTable.action")}</th>
          </tr>
        </thead>
        <tbody>
          {tickets.map((ticket) => (
            <tr key={ticket.id}>
              <td>
                <Link href={`/enterprise/ticket-workbench?ticket_id=${ticket.id}`} className="block min-w-0 no-underline">
                  <strong className="block truncate text-xs font-semibold text-primary">{ticket.ticket_no}</strong>
                  <span className="mt-1 block truncate text-xs font-medium text-foreground" title={ticket.title}>{ticket.title}</span>
                </Link>
              </td>
              <td>
                <span className="block truncate text-xs font-medium text-foreground">{ticket.customer_name || t("enterpriseWorkbench.common.unbound")}</span>
                {hasDeviceConcept ? <span className="mt-1 block truncate text-rhd-xs text-muted-foreground">{ticket.device_no || t("enterpriseWorkbench.ticket.unboundProduct")}</span> : null}
              </td>
              <td>
                <span className="block truncate text-xs font-medium text-foreground">{hasDeviceConcept ? ticket.product_name || t("enterpriseWorkbench.ticket.unboundProduct") : ticket.team_name || t("enterpriseWorkbench.common.unbound")}</span>
                <span className="mt-1 block truncate text-rhd-xs text-muted-foreground">{ticketOwnerText(ticket, t, hasDeviceConcept)}</span>
              </td>
              <td>
                <span className={cn("rounded border px-1.5 py-0.5 text-rhd-xs font-medium", toneClass(priorityTone(ticket.priority)))}>
                  {priorityLabel(ticket.priority)}
                </span>
              </td>
              <td>
                <div className="flex min-w-0 items-center gap-2">
                  <StatusTag tone={ticket.sla_breached ? "error" : statusTone(priorityTone(ticket.priority))}>
                    {statusLabel(ticket.status, t)}
                  </StatusTag>
                  {ticket.sla_breached ? <span className="text-rhd-xs font-medium text-destructive">{t("enterpriseWorkbench.ticket.overdue")}</span> : null}
                </div>
              </td>
              <td>
                <span className="text-xs text-muted-foreground">{dateText(ticket.updated_at, locale)}</span>
              </td>
              <td>
                <Link href={`/enterprise/ticket-workbench?ticket_id=${ticket.id}`} className="inline-flex items-center gap-1 text-xs font-medium text-primary no-underline">
                  {t("enterpriseWorkbench.ticketTable.view")}
                  <ArrowRightIcon className="size-3.5" />
                </Link>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

const queueOrder = ["sla_risk", "urgent", "unassigned", "pending", "processing", "awaiting_customer"]

function queueToneColor(tone?: string, key?: string) {
  const normalized = tone === "critical" ? "red" : tone === "warning" ? "amber" : tone === "green" ? "success" : tone === "success" ? "success" : tone
  if (normalized === "red" || key === "sla_risk") return "var(--railops-error)"
  if (normalized === "amber" || key === "urgent") return "var(--railops-warning)"
  if (normalized === "success") return "var(--railops-success)"
  if (normalized === "blue" || normalized === "info") return "var(--railops-primary)"
  return "var(--railops-text-tertiary)"
}

function QueueHierarchyTree({
  hasDeviceConcept,
  loading,
  onSelect,
  queues,
  selectedKey,
}: {
  hasDeviceConcept: boolean
  loading: boolean
  onSelect: (key: string) => void
  queues: EnterpriseWorkbenchQueueCard[]
  selectedKey: string
}) {
  const t = useI18n()
  const total = queues.reduce((sum, queue) => sum + queue.count, 0)
  const groups = [
    {
      key: "risk",
      keys: ["sla_risk", "urgent"],
      title: t("enterpriseWorkbench.queueTree.risk"),
    },
    {
      key: "dispatch",
      keys: ["unassigned", "pending", "processing"],
      title: t("enterpriseWorkbench.queueTree.dispatch"),
    },
    {
      key: "closure",
      keys: ["awaiting_customer"],
      title: t("enterpriseWorkbench.queueTree.closure"),
    },
  ].map((group) => ({
    ...group,
    queues: group.keys.map((key) => queues.find((item) => item.key === key)).filter(Boolean) as EnterpriseWorkbenchQueueCard[],
  })).filter((group) => group.queues.length > 0)

  return (
    <section className="rhd-railops-workbench-queue-tree" aria-label={t("enterpriseWorkbench.queueTree.aria")}>
      <header>
        <h3>{t("enterpriseWorkbench.queueTree.title")}</h3>
        <span>{t("enterpriseWorkbench.queueTree.total", { count: total })}</span>
      </header>
      {loading ? (
        <div className="space-y-2 p-3" role="status" aria-busy="true" aria-label={t("common.loadingData")}>
          {Array.from({ length: 5 }, (_, index) => <Skeleton.Node key={index} active className="w-full" style={{ width: "100%", height: 28 }} />)}
        </div>
      ) : (
        <div className="rhd-railops-workbench-queue-tree-body" role="tree">
          {groups.map((group) => {
            const groupCount = group.queues.reduce((sum, queue) => sum + queue.count, 0)
            const groupPercent = total > 0 ? Math.round((groupCount / total) * 100) : 0
            return (
              <div key={group.key} className="rhd-railops-workbench-queue-tree-group">
                <div
                  className="rhd-railops-workbench-queue-tree-group-title"
                  title={t("enterpriseWorkbench.queuePie.hover", { name: group.title, count: groupCount, percent: groupPercent })}
                >
                  <span>{group.title}</span>
                  <strong>{groupCount}</strong>
                </div>
                <div className="rhd-railops-workbench-queue-tree-children">
                  {group.queues.map((queue) => {
                    const active = queue.key === selectedKey
                    const percent = total > 0 ? Math.round((queue.count / total) * 100) : 0
                    return (
                      <button
                        key={queue.key}
                        type="button"
                        role="treeitem"
                        aria-selected={active}
                        onClick={() => onSelect(queue.key)}
                        className={cn("rhd-railops-workbench-queue-tree-item", active && "is-active")}
                        title={t("enterpriseWorkbench.queuePie.hover", { name: queueTitle(queue, t, hasDeviceConcept), count: queue.count, percent })}
                      >
                        <span className="rhd-railops-workbench-queue-tree-node" style={{ background: queueToneColor(queue.tone, queue.key) }} />
                        <span className="min-w-0 flex-1 truncate">{queueTitle(queue, t, hasDeviceConcept)}</span>
                        <StatusTag tone={statusTone(queue.tone)}>{queue.count}</StatusTag>
                      </button>
                    )
                  })}
                </div>
              </div>
            )
          })}
        </div>
      )}
    </section>
  )
}

function QueueDistributionPie({
  hasDeviceConcept,
  loading,
  queues,
  onSelect,
  selectedKey = "",
}: {
  hasDeviceConcept: boolean
  loading: boolean
  queues: EnterpriseWorkbenchQueueCard[]
  onSelect?: (key: string) => void
  selectedKey?: string
}) {
  const t = useI18n()
  const total = queues.reduce((sum, queue) => sum + queue.count, 0)
  const [hoveredKey, setHoveredKey] = useState<string | null>(null)

  let cursor = 0
  const segments = queues.map((queue) => {
    const start = cursor
    const size = total > 0 ? (queue.count / total) * 360 : 0
    cursor += size
    return { key: queue.key, start, end: cursor, color: queueToneColor(queue.tone, queue.key) }
  })

  // Hovered segment gets a lighter shade so the ring shows which part the value belongs to.
  const pieStyle: CSSProperties = {
    background: total > 0
      ? `conic-gradient(${segments.map((segment) => {
          const color = segment.key === hoveredKey ? `color-mix(in oklab, ${segment.color} 55%, white)` : segment.color
          return `${color} ${segment.start}deg ${segment.end}deg`
        }).join(", ")})`
      : "var(--railops-surface-muted)",
  }

  const handlePieMove = (event: ReactMouseEvent<HTMLDivElement>) => {
    if (total <= 0) return
    const rect = event.currentTarget.getBoundingClientRect()
    const dx = event.clientX - rect.left - rect.width / 2
    const dy = event.clientY - rect.top - rect.height / 2
    const distance = Math.hypot(dx, dy)
    // Inside the center hole → no segment hovered.
    if (distance < rect.width / 2 - 22) {
      setHoveredKey(null)
      return
    }
    // conic-gradient starts at 12 o'clock and runs clockwise.
    let deg = (Math.atan2(dy, dx) * 180) / Math.PI + 90
    if (deg < 0) deg += 360
    const hit = segments.find((segment) => deg >= segment.start && deg < segment.end)
    setHoveredKey(hit ? hit.key : null)
  }

  const hoveredQueue = hoveredKey ? queues.find((queue) => queue.key === hoveredKey) : null
  const hoveredPercent = hoveredQueue && total > 0 ? Math.round((hoveredQueue.count / total) * 100) : 0

  return (
    <section className="rhd-railops-workbench-queue-pie" aria-label={t("enterpriseWorkbench.queuePie.aria")}>
      <header>
        <h3>{t("enterpriseWorkbench.queuePie.title")}</h3>
        <span>{t("enterpriseWorkbench.queuePie.total", { count: total })}</span>
      </header>
      {loading ? (
        <div className="rhd-railops-workbench-queue-pie-loading" role="status" aria-busy="true" aria-label={t("common.loadingData")}>
          <Skeleton.Node active style={{ width: 112, height: 112 }} />
          <div className="min-w-0 flex-1 space-y-2">
            {Array.from({ length: 4 }, (_, index) => <Skeleton.Node key={index} active className="w-full" style={{ width: "100%", height: 16 }} />)}
          </div>
        </div>
      ) : (
        <div className="rhd-railops-workbench-queue-pie-body">
          <div
            className={cn("rhd-railops-workbench-queue-pie-chart", hoveredKey && "is-hovered")}
            style={pieStyle}
            onMouseMove={handlePieMove}
            onMouseLeave={() => setHoveredKey(null)}
            onClick={() => {
              if (hoveredKey && onSelect) onSelect(hoveredKey)
            }}
            aria-label={hoveredQueue
              ? t("enterpriseWorkbench.queuePie.hover", { name: queueTitle(hoveredQueue, t, hasDeviceConcept), count: hoveredQueue.count, percent: hoveredPercent })
              : t("enterpriseWorkbench.queuePie.total", { count: total })}
          >
            <span>
              {hoveredQueue ? (
                <>
                  <strong>{formatInteger(hoveredQueue.count)}</strong>
                  <small>{t("enterpriseWorkbench.queuePie.hover", { name: queueTitle(hoveredQueue, t, hasDeviceConcept), count: hoveredQueue.count, percent: hoveredPercent })}</small>
                </>
              ) : (
                <>
                  <strong>{total}</strong>
                  <small>{t("enterpriseWorkbench.queuePie.center")}</small>
                </>
              )}
            </span>
          </div>
          <div className="rhd-railops-workbench-queue-pie-legend">
            {queues.map((queue) => (
              <div
                key={queue.key}
                className={cn(
                  "rhd-railops-workbench-queue-pie-legend-row",
                  queue.key === hoveredKey && "is-hovered",
                  queue.key === selectedKey && "is-active",
                )}
              >
                <span className="rhd-railops-workbench-queue-pie-dot" style={{ background: queueToneColor(queue.tone, queue.key) }} />
                <span className="min-w-0 flex-1 truncate">{queueTitle(queue, t, hasDeviceConcept)}</span>
                <strong>{queue.count}</strong>
              </div>
            ))}
          </div>
        </div>
      )}
    </section>
  )
}

function QueueWorkspace({
  coreLoading,
  error,
  loading,
  onPageChange,
  onRetry,
  onSelect,
  pagination,
  queues,
  refreshing,
  selectedKey,
  tickets,
  hasDeviceConcept,
}: {
  coreLoading: boolean
  error: string
  loading: boolean
  onPageChange: (page: number) => void
  onRetry: () => void
  onSelect: (key: string) => void
  pagination?: EnterpriseWorkbenchQueue["tickets_pagination"] | null
  queues: EnterpriseWorkbenchQueueCard[]
  refreshing: boolean
  selectedKey: string
  tickets: TicketListItem[]
  hasDeviceConcept: boolean
}) {
  const t = useI18n()
  const sortedQueues = [...queues].sort((a, b) => queueOrder.indexOf(a.key) - queueOrder.indexOf(b.key))
  const activeQueue = sortedQueues.find((queue) => queue.key === selectedKey) || sortedQueues[0]
  const items = tickets
  const activePage = pagination?.page ?? 1
  const totalPages = pagination?.total_pages ?? 0
  const total = pagination?.total ?? activeQueue?.count ?? items.length
  const canPrev = activePage > 1 && !loading
  const canNext = Boolean(pagination?.has_more) && !loading

  return (
    <section className="min-w-0 overflow-hidden rounded-lg border border-border bg-card">
      <div className="flex items-center justify-between gap-3 border-b border-border px-4 py-3.5">
        <div className="min-w-0">
          <div className="flex min-w-0 items-center gap-2">
            <h2 className="text-sm font-semibold text-foreground">{t("enterpriseWorkbench.queue.title")}</h2>
            {refreshing ? (
              <RefreshCwIcon className="size-3.5 shrink-0 animate-spin text-muted-foreground" aria-label={t("enterpriseWorkbench.queue.refreshing")} />
            ) : null}
          </div>
        </div>
        <Link href="/enterprise/tickets" className="shrink-0 text-xs font-medium text-primary no-underline">
          {t("enterpriseWorkbench.queue.allTickets")}
        </Link>
      </div>
      <div className="rhd-railops-workbench-queue-content-grid">
        <div className="rhd-railops-workbench-queue-visual-grid">
          <QueueHierarchyTree
            hasDeviceConcept={hasDeviceConcept}
            loading={coreLoading}
            onSelect={onSelect}
            queues={sortedQueues}
            selectedKey={activeQueue?.key || selectedKey}
          />
        </div>
        <div className="rhd-railops-workbench-queue-table-panel">
          <div className="min-w-0 flex-1">
            {loading ? (
              <PanelSkeleton rows={5} label={t("common.loadingData")} />
            ) : error ? (
              <ModuleError message={t("enterpriseWorkbench.errors.todayTasksUnavailable", { error })} onRetry={onRetry} />
            ) : items.length > 0 ? (
              <>
                <div className="rhd-railops-workbench-ticket-table-title">
                  <div className="min-w-0">
                    <h3>{activeQueue ? queueTitle(activeQueue, t, hasDeviceConcept) : t("enterpriseWorkbench.queue.title")}</h3>
                    <p>{activeQueue?.description || t("enterpriseWorkbench.queue.tableSubtitle")}</p>
                  </div>
                  <StatusTag tone={statusTone(activeQueue?.tone)}>{t("enterpriseWorkbench.queue.tableCount", { count: total })}</StatusTag>
                </div>
                <TicketQueueTable tickets={items} hasDeviceConcept={hasDeviceConcept} />
              </>
            ) : (
              <div className="grid min-h-56 place-items-center p-6 text-center">
                <div>
                  <span className="mx-auto grid size-10 place-items-center rounded-full border border-border bg-muted text-muted-foreground">
                    <ShieldCheckIcon className="size-5" />
                  </span>
                  <p className="mt-3 text-sm font-medium text-foreground">{t("enterpriseWorkbench.queue.emptyTitle")}</p>
                </div>
              </div>
            )}
          </div>
          {pagination ? (
            <div className="flex flex-wrap items-center justify-between gap-3 border-t border-border/70 bg-muted/60 px-4 py-3">
              <span className="text-xs text-muted-foreground">
                {t("enterpriseWorkbench.queue.pageSummary", {
                  page: activePage,
                  totalPages: Math.max(totalPages, 1),
                  total,
                })}
              </span>
              <div className="flex items-center gap-2">
                <RailopsButton size="small" onClick={() => onPageChange(activePage - 1)} disabled={!canPrev}>
                  <ChevronLeftIcon className="size-3.5" />
                  {t("enterpriseWorkbench.common.previous")}
                </RailopsButton>
                <RailopsButton size="small" onClick={() => onPageChange(activePage + 1)} disabled={!canNext}>
                  {t("enterpriseWorkbench.common.next")}
                  <ChevronRightIcon className="size-3.5" />
                </RailopsButton>
              </div>
            </div>
          ) : activeQueue && activeQueue.count > items.length ? (
            <div className="border-t border-border/70 bg-muted/60 px-4 py-3 text-right">
              <Link href={activeQueue.action_url || "/enterprise/tickets"} className="text-xs font-medium text-primary no-underline">
                {t("enterpriseWorkbench.queue.viewRemaining", { count: activeQueue.count - items.length })}
              </Link>
            </div>
          ) : null}
        </div>
      </div>
    </section>
  )
}

function ModuleError({ message, onRetry }: { message: string; onRetry: () => void }) {
  const t = useI18n()
  return (
    <div className="rhd-railops-module-error flex min-h-24 flex-col items-center justify-center gap-2 p-4 text-center">
      <p className="text-xs text-destructive">{message}</p>
      <RailopsButton size="small" onClick={onRetry}>
        <RefreshCwIcon className="size-3.5" />
        {t("enterpriseWorkbench.common.retry")}
      </RailopsButton>
    </div>
  )
}

function PanelSkeleton({ label, rows = 3, status = true }: { label?: string; rows?: number; status?: boolean }) {
  const t = useI18n()
  const resolvedLabel = label || t("common.loadingData")
  const statusProps = status
    ? { role: "status", "aria-busy": "true" as const, "aria-label": resolvedLabel }
    : {}
  return (
    <div className="space-y-3 p-4" {...statusProps}>
      {Array.from({ length: rows }, (_, index) => (
        <Skeleton.Node key={index} active className="w-full" style={{ width: "100%", height: 48 }} />
      ))}
    </div>
  )
}

function ProductLoadPanel({
  error,
  loading,
  onRetry,
  products,
}: {
  error: string
  loading: boolean
  onRetry: () => void
  products: EnterpriseWorkbenchProductLoad[]
}) {
  const t = useI18n()
  return (
    <section className="rounded-lg border border-border bg-card">
      <div className="flex items-center justify-between border-b border-border p-4">
        <h2 className="text-sm font-semibold text-foreground">{t("enterpriseWorkbench.product.title")}</h2>
        <Link href="/enterprise/products" className="text-xs font-medium text-primary no-underline">
          {t("enterpriseWorkbench.common.viewAll")}
        </Link>
      </div>
      <div>
        {loading ? (
          <PanelSkeleton rows={3} label={t("common.loadingData")} />
        ) : error ? (
          <ModuleError message={t("enterpriseWorkbench.errors.productLoadUnavailable", { error })} onRetry={onRetry} />
        ) : products.length > 0 ? (
          <div className="rhd-railops-workbench-product-table-wrap">
            <table className="rhd-railops-workbench-product-table">
              <thead>
                <tr>
                  <th>{t("enterpriseWorkbench.product.columns.product")}</th>
                  <th>{t("enterpriseWorkbench.product.columns.team")}</th>
                  <th className="is-number">{t("enterpriseWorkbench.product.columns.open")}</th>
                  <th className="is-number">{t("enterpriseWorkbench.product.columns.unassigned")}</th>
                  <th className="is-number">{t("enterpriseWorkbench.product.columns.pending")}</th>
                  <th className="is-number">{t("enterpriseWorkbench.product.columns.sla")}</th>
                  <th>{t("enterpriseWorkbench.product.columns.usage")}</th>
                  <th className="is-money">{t("enterpriseWorkbench.product.columns.remaining")}</th>
                  <th className="is-action">{t("enterpriseWorkbench.product.columns.actions")}</th>
                </tr>
              </thead>
              <tbody>
                {products.map((item) => {
                  const usagePercent = Math.min(100, Math.max(0, item.usage_percent || 0))
                  const productName = item.product_name || t("enterpriseWorkbench.product.fallback", { id: item.product_id })
                  return (
                    <tr key={item.product_id}>
                      <td>
                        <span className="rhd-railops-workbench-product-name" title={productName}>
                          {productName}
                        </span>
                      </td>
                      <td>
                        <span className={cn("rhd-railops-workbench-product-team", toneClass(item.tone))}>
                          {productTeamBadgeText(item, t)}
                        </span>
                      </td>
                      <td className="is-number">{item.open_tickets}</td>
                      <td className="is-number">{item.unassigned_tickets}</td>
                      <td className="is-number">{item.pending_tickets}</td>
                      <td className="is-number">{item.sla_risk_tickets}</td>
                      <td>
                        <div className="rhd-railops-workbench-product-usage">
                          <span>{formatPercent(item.usage_percent)}</span>
                          <i aria-hidden="true">
                            <b
                              className={cn(
                                item.usage_percent >= 95
                                  ? "is-danger"
                                  : item.usage_percent >= 80
                                    ? "is-warning"
                                    : "is-normal",
                              )}
                              style={{ width: `${usagePercent}%` }}
                            />
                          </i>
                        </div>
                      </td>
                      <td className="is-money">{formatAmount(item.quota_remaining, item.currency)}</td>
                      <td className="is-action">
                        <Link
                          href={buildEnterpriseProductPath(item.product_id)}
                          className="text-xs font-medium text-primary no-underline"
                        >
                          {t("enterpriseWorkbench.product.viewProduct")}
                        </Link>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        ) : (
          <EmptyState title={t("enterpriseWorkbench.product.empty")} className="rounded-none border-0" />
        )}
      </div>
    </section>
  )
}

function WorkFocusPanel({
  collaboration,
  collaborationError,
  collaborationLoading,
  onRetryCollaboration,
}: {
  collaboration: EnterpriseWorkbenchCollaboration | null
  collaborationError: string
  collaborationLoading: boolean
  onRetryCollaboration: () => void
}) {
  const t = useI18n()
  const items = [
    {
      href: "/enterprise/ticket-workbench",
      icon: <HeadphonesIcon className="size-4" />,
      label: t("enterpriseWorkbench.collaboration.conversations"),
      error: collaborationError,
      loading: collaborationLoading,
      value: collaboration?.active_conversations,
    },
    {
      href: "/enterprise/video",
      icon: <UsersRoundIcon className="size-4" />,
      label: t("enterpriseWorkbench.collaboration.meetings"),
      error: collaborationError,
      loading: collaborationLoading,
      value: collaboration?.active_meetings,
    },
  ]

  return (
    <section className="overflow-hidden rounded-lg border border-border bg-card" aria-busy={collaborationLoading ? "true" : undefined}>
      <div className="border-b border-border px-4 py-3.5">
        <h2 className="text-sm font-semibold text-foreground">{t("enterpriseWorkbench.collaboration.title")}</h2>
      </div>
      <div className="grid grid-cols-2 divide-x divide-border/70">
        {items.map((item) => (
          <Link key={item.label} href={item.href} className="min-w-0 px-3 py-4 text-center no-underline transition-colors hover:bg-muted/60">
            <span className="mx-auto grid size-7 place-items-center text-muted-foreground">{item.icon}</span>
            {item.loading ? (
              <Skeleton.Node active className="mx-auto mt-1" style={{ width: 32, height: 24 }} />
            ) : (
              <strong className={cn("mt-1 block text-lg font-semibold", item.error ? "text-destructive" : "text-foreground")}>
                {item.error ? "—" : item.value ?? 0}
              </strong>
            )}
            <span className="mt-0.5 block truncate text-rhd-xs text-muted-foreground">{item.label}</span>
          </Link>
        ))}
      </div>
      {collaborationError ? (
        <div className="flex flex-wrap gap-2 border-t border-border bg-muted/40 px-3 py-2">
          <button type="button" className="text-xs font-medium text-destructive" onClick={onRetryCollaboration}>
            {t("enterpriseWorkbench.collaboration.retry")}
          </button>
        </div>
      ) : null}
    </section>
  )
}

function MeetingPanel({
  error,
  loading,
  meetings,
  onRetry,
}: {
  error: string
  loading: boolean
  meetings: MeetingListItem[]
  onRetry: () => void
}) {
  const t = useI18n()
  return (
    <section className="rounded-lg border border-border bg-card">
      <div className="flex items-center justify-between border-b border-border p-4">
        <h2 className="text-sm font-semibold text-foreground">{t("enterpriseWorkbench.meetings.title")}</h2>
        <Link href="/enterprise/video" className="text-xs font-medium text-primary no-underline">
          {t("enterpriseWorkbench.common.viewAll")}
        </Link>
      </div>
      <div className="divide-y divide-border/70">
        {loading ? (
          <PanelSkeleton rows={3} label={t("common.loadingData")} />
        ) : error ? (
          <ModuleError message={t("enterpriseWorkbench.errors.meetingsUnavailable", { error })} onRetry={onRetry} />
        ) : meetings.length > 0 ? (
          meetings.slice(0, 3).map((meeting) => (
            <article key={meeting.id} className="p-4">
              <div className="flex items-center justify-between gap-3">
                <strong className="truncate text-sm text-foreground">{meeting.title || meeting.room_name}</strong>
                <StatusTag tone="neutral" className={toneClass(meeting.status === "active" ? "success" : "slate")}>
                  {meetingStatusLabel(meeting.status, t)}
                </StatusTag>
              </div>
              <p className="mt-1 truncate text-xs text-muted-foreground">
                {meeting.ticket_no || t("enterpriseWorkbench.meetings.unlinkedTicket")} · {meeting.product_name || t("enterpriseWorkbench.ticket.unboundProduct")} · {t("enterpriseWorkbench.meetings.participants", { count: meeting.participant_count })}
              </p>
            </article>
          ))
        ) : (
          <p className="p-4 text-sm text-muted-foreground">{t("enterpriseWorkbench.meetings.empty")}</p>
        )}
      </div>
    </section>
  )
}

function UpcomingMeetingSchedulePanel({
  error,
  loading,
  meetings,
  onRetry,
  total,
}: {
  error: string
  loading: boolean
  meetings: MeetingListItem[]
  onRetry: () => void
  total: number
}) {
  const t = useI18n()
  const { locale } = useAppLocale()
  const groups = useMemo(() => {
    const grouped = new Map<string, {
      key: string
      monthDay: string
      weekday: string
      items: MeetingListItem[]
    }>()
    const ordered = [...meetings]
      .filter((meeting) => meeting.status === "waiting" || meeting.status === "scheduled")
      .sort((a, b) => meetingScheduleTimestamp(a) - meetingScheduleTimestamp(b))

    ordered.forEach((meeting) => {
      const parts = meetingCalendarParts(meetingScheduleValue(meeting), locale)
      const current = grouped.get(parts.key) || { ...parts, items: [] }
      current.items.push(meeting)
      grouped.set(parts.key, current)
    })
    return Array.from(grouped.values())
  }, [locale, meetings])

  return (
    <section className="rhd-railops-upcoming-meeting-panel rounded-lg border border-border bg-card">
      <div className="rhd-railops-upcoming-meeting-header">
        <div className="min-w-0">
          <div className="flex min-w-0 items-center gap-2">
            <span className="rhd-railops-upcoming-meeting-icon">
              <CalendarClockIcon className="size-4" />
            </span>
            <h2 className="truncate text-sm font-semibold text-foreground">{t("enterpriseWorkbench.meetingSchedule.title")}</h2>
          </div>
          <p className="mt-1 truncate text-rhd-xs text-muted-foreground">{t("enterpriseWorkbench.meetingSchedule.subtitle")}</p>
        </div>
        <div className="rhd-railops-upcoming-meeting-actions">
          <StatusTag tone="blue">{t("enterpriseWorkbench.meetingSchedule.count", { count: total })}</StatusTag>
          <Link href="/enterprise/video?status=waiting" className="text-xs font-medium text-primary no-underline">
            {t("enterpriseWorkbench.common.viewAll")}
          </Link>
        </div>
      </div>
      <div className="rhd-railops-upcoming-meeting-body">
        {loading ? (
          <PanelSkeleton rows={4} label={t("enterpriseWorkbench.meetingSchedule.loading")} />
        ) : error ? (
          <ModuleError message={t("enterpriseWorkbench.errors.meetingScheduleUnavailable", { error })} onRetry={onRetry} />
        ) : groups.length > 0 ? (
          <div
            className="rhd-railops-upcoming-calendar"
            role="table"
            aria-label={t("enterpriseWorkbench.meetingSchedule.calendarAria")}
          >
            <div
              className="rhd-railops-upcoming-calendar-grid"
              style={{ "--rhd-upcoming-days": groups.length } as CSSProperties}
            >
              {groups.map((group, index) => (
                <section key={group.key} className={cn("rhd-railops-upcoming-calendar-column", index === 0 && "is-active")} role="cell">
                  <header className="rhd-railops-upcoming-calendar-column-head">
                    <span>{group.weekday}</span>
                    <strong>{group.monthDay}</strong>
                    <em>{t("enterpriseWorkbench.meetingSchedule.dayCount", { count: group.items.length })}</em>
                  </header>
                  <div className="rhd-railops-upcoming-calendar-events">
                    {group.items.map((meeting) => {
                      const scheduleValue = meetingScheduleValue(meeting)
                      return (
                        <Link
                          key={meeting.id}
                          href={`/enterprise/video?meeting_id=${encodeURIComponent(meeting.id)}`}
                          className="rhd-railops-upcoming-calendar-event no-underline"
                        >
                          <span className="rhd-railops-upcoming-calendar-event-time">{formatMeetingClock(scheduleValue, locale)}</span>
                          <span className="rhd-railops-upcoming-calendar-event-title">
                            <VideoIcon className="size-3.5" />
                            <strong>{meeting.title || meeting.room_name}</strong>
                          </span>
                          <span className="rhd-railops-upcoming-calendar-event-meta">
                            {meeting.ticket_no || t("enterpriseWorkbench.meetings.unlinkedTicket")}
                          </span>
                          <span className="rhd-railops-upcoming-calendar-event-meta">
                            {meeting.device_no || meeting.product_name || t("enterpriseWorkbench.ticket.unboundProduct")} · {meeting.customer_name || t("enterpriseWorkbench.common.unbound")}
                          </span>
                          <StatusTag tone={meetingStatusTone(meeting.status)}>{meetingStatusLabel(meeting.status, t)}</StatusTag>
                        </Link>
                      )
                    })}
                  </div>
                </section>
              ))}
            </div>
          </div>
        ) : (
          <EmptyState title={t("enterpriseWorkbench.meetingSchedule.empty")} className="rounded-none border-0" />
        )}
      </div>
    </section>
  )
}

export function EnterpriseServiceWorkbenchPage({ view = "overview" }: { view?: WorkbenchView } = {}) {
  const t = useI18n()
  const { ready, session } = useAuth()
  const router = useRouter()
  const workbenchView = view
  const tenantAIEnabled = ready && session !== null && session.featureFlags?.ai !== false
  const hasDeviceConcept = session?.featureFlags?.device !== false
  const [core, setCore] = useState<EnterpriseWorkbenchCore | null>(null)
  const [collaboration, setCollaboration] = useState<EnterpriseWorkbenchCollaboration | null>(null)
  const [queue, setQueue] = useState<EnterpriseWorkbenchQueue | null>(null)
  const [resources, setResources] = useState<EnterpriseWorkbenchResources | null>(null)
  const [upcomingMeetings, setUpcomingMeetings] = useState<MeetingListItem[]>([])
  const [upcomingMeetingTotal, setUpcomingMeetingTotal] = useState(0)
  const [reportOverview, setReportOverview] = useState<DashboardOverview | null>(null)
  const [reportFailures, setReportFailures] = useState<ProductFailureRank[]>([])
  const [keyWorkspace, setKeyWorkspace] = useState<EnterpriseAIModelWorkspace | null>(null)
  const [selectedQueueKey, setSelectedQueueKey] = useState("")
  const [queuePage, setQueuePage] = useState(1)
  const [loadedQueueKey, setLoadedQueueKey] = useState("")
  const [coreLoading, setCoreLoading] = useState(true)
  const [coreLoaded, setCoreLoaded] = useState(false)
  const [coreError, setCoreError] = useState("")
  const [collaborationLoading, setCollaborationLoading] = useState(true)
  const [collaborationLoaded, setCollaborationLoaded] = useState(false)
  const [collaborationError, setCollaborationError] = useState("")
  const [queueLoading, setQueueLoading] = useState(true)
  const [queueLoaded, setQueueLoaded] = useState(false)
  const [queueError, setQueueError] = useState("")
  const [resourcesLoading, setResourcesLoading] = useState(true)
  const [resourcesLoaded, setResourcesLoaded] = useState(false)
  const [resourcesError, setResourcesError] = useState("")
  const [upcomingMeetingLoading, setUpcomingMeetingLoading] = useState(true)
  const [upcomingMeetingLoaded, setUpcomingMeetingLoaded] = useState(false)
  const [upcomingMeetingError, setUpcomingMeetingError] = useState("")
  const [reportOverviewLoading, setReportOverviewLoading] = useState(true)
  const [reportOverviewLoaded, setReportOverviewLoaded] = useState(false)
  const [reportOverviewError, setReportOverviewError] = useState("")
  const [reportFailuresLoading, setReportFailuresLoading] = useState(true)
  const [reportFailuresLoaded, setReportFailuresLoaded] = useState(false)
  const [reportFailuresError, setReportFailuresError] = useState("")
  const [keyWorkspaceLoading, setKeyWorkspaceLoading] = useState(true)
  const [keyWorkspaceLoaded, setKeyWorkspaceLoaded] = useState(false)
  const [keyWorkspaceError, setKeyWorkspaceError] = useState("")

  const loadCollaboration = useCallback(async () => {
    setCollaborationLoading(true)
    setCollaborationError("")
    try {
      const res = await fetchEnterpriseWorkbenchCollaboration()
      if (res.success && res.data) {
        setCollaboration(res.data)
        setCollaborationLoaded(true)
      } else {
        setCollaborationError(res.error?.message || t("enterpriseWorkbench.errors.collaborationLoadFailed"))
      }
    } catch (error) {
      setCollaborationError(error instanceof Error ? error.message : t("enterpriseWorkbench.errors.collaborationLoadFailed"))
    } finally {
      setCollaborationLoading(false)
    }
  }, [t])

  const loadResources = useCallback(async () => {
    setResourcesLoading(true)
    setResourcesError("")
    try {
      const res = await fetchEnterpriseWorkbenchResources()
      if (res.success && res.data) {
        setResources(res.data)
        setResourcesLoaded(true)
      } else {
        setResourcesError(res.error?.message || t("enterpriseWorkbench.errors.resourcesLoadFailed"))
      }
    } catch (error) {
      setResourcesError(error instanceof Error ? error.message : t("enterpriseWorkbench.errors.resourcesLoadFailed"))
    } finally {
      setResourcesLoading(false)
    }
  }, [t])

  const loadUpcomingMeetings = useCallback(async () => {
    setUpcomingMeetingLoading(true)
    setUpcomingMeetingError("")
    try {
      const res = await fetchPersonalMeetings("waiting", 1, UPCOMING_MEETING_LIMIT)
      if (res.success && res.data) {
        const items = [...(res.data.items || [])].sort((a, b) => meetingScheduleTimestamp(a) - meetingScheduleTimestamp(b))
        setUpcomingMeetings(items)
        setUpcomingMeetingTotal(Number(res.data.total ?? res.data.summary.waiting ?? items.length))
        setUpcomingMeetingLoaded(true)
      } else {
        setUpcomingMeetingError(res.error?.message || t("enterpriseWorkbench.errors.meetingScheduleLoadFailed"))
      }
    } catch (error) {
      setUpcomingMeetingError(error instanceof Error ? error.message : t("enterpriseWorkbench.errors.meetingScheduleLoadFailed"))
    } finally {
      setUpcomingMeetingLoading(false)
    }
  }, [t])

  const loadReportOverview = useCallback(async () => {
    setReportOverviewLoading(true)
    setReportOverviewError("")
    try {
      const res = await fetchReportsOverview()
      if (res.success && res.data) {
        setReportOverview(res.data)
        setReportOverviewLoaded(true)
      } else {
        setReportOverviewError(res.error?.message || t("enterpriseWorkbench.errors.qualityLoadFailed"))
      }
    } catch (error) {
      setReportOverviewError(error instanceof Error ? error.message : t("enterpriseWorkbench.errors.qualityLoadFailed"))
    } finally {
      setReportOverviewLoading(false)
    }
  }, [t])

  const loadReportFailures = useCallback(async () => {
    setReportFailuresLoading(true)
    setReportFailuresError("")
    const range = currentMonthReportRange()
    try {
      const res = await fetchReportTopFailures({ limit: 3, start: range.start, end: range.end })
      if (res.success) {
        setReportFailures(res.data ?? [])
        setReportFailuresLoaded(true)
      } else {
        setReportFailuresError(res.error?.message || t("enterpriseWorkbench.errors.failuresLoadFailed"))
      }
    } catch (error) {
      setReportFailuresError(error instanceof Error ? error.message : t("enterpriseWorkbench.errors.failuresLoadFailed"))
    } finally {
      setReportFailuresLoading(false)
    }
  }, [t])

  const loadKeyWorkspace = useCallback(async () => {
    if (!tenantAIEnabled) {
      setKeyWorkspace(null)
      setKeyWorkspaceError("")
      setKeyWorkspaceLoading(false)
      setKeyWorkspaceLoaded(true)
      return
    }
    setKeyWorkspaceLoading(true)
    setKeyWorkspaceError("")
    try {
      const res = await withTimeout(
        getEnterpriseAIModelsWorkspace({
          page: 1,
          pageSize: API_KEY_USAGE_PANEL_LIMIT,
          sortBy: "quota_used",
          sortOrder: "desc",
        }),
        API_KEY_USAGE_TIMEOUT_MS,
        t("enterpriseWorkbench.errors.apiKeysLoadFailed"),
      )
      if (res.success && res.data) {
        setKeyWorkspace(res.data)
        setKeyWorkspaceLoaded(true)
      } else {
        setKeyWorkspaceError(res.error?.message || t("enterpriseWorkbench.errors.apiKeysLoadFailed"))
      }
    } catch (error) {
      setKeyWorkspaceError(error instanceof Error ? error.message : t("enterpriseWorkbench.errors.apiKeysLoadFailed"))
    } finally {
      setKeyWorkspaceLoading(false)
    }
  }, [t, tenantAIEnabled])

  const load = useCallback(async () => {
    setCoreLoading(true)
    setCoreError("")
    try {
      const res = await fetchEnterpriseWorkbenchCore()
      if (res.success && res.data) {
        setCore(res.data)
        setCoreLoaded(true)
      } else {
        setCoreError(res.error?.message || t("enterpriseWorkbench.errors.coreLoadFailed"))
      }
    } catch (error) {
      setCoreError(error instanceof Error ? error.message : t("enterpriseWorkbench.errors.coreLoadFailed"))
    } finally {
      setCoreLoading(false)
    }
  }, [t])

  useEffect(() => {
    const timer = window.setTimeout(() => {
      void load()
      if (workbenchView === "overview") {
        void loadCollaboration()
        void loadResources()
        void loadReportOverview()
        if (tenantAIEnabled) void loadKeyWorkspace()
        void loadUpcomingMeetings()
      }
      if (workbenchView === "insights") {
        void loadReportOverview()
        void loadReportFailures()
        if (tenantAIEnabled) void loadKeyWorkspace()
        void loadUpcomingMeetings()
      }
      if (workbenchView === "resources") {
        void loadCollaboration()
        void loadResources()
        void loadUpcomingMeetings()
      }
    }, 0)
    return () => window.clearTimeout(timer)
  }, [load, loadCollaboration, loadKeyWorkspace, loadReportFailures, loadReportOverview, loadResources, loadUpcomingMeetings, tenantAIEnabled, workbenchView])

  const unassignedHref = core?.scope.restricted
    ? "/enterprise/ticket-workbench"
    : "/enterprise/org"

  const alerts = useMemo(
    () => (core?.alerts || []).map((item) => (
      item.key === "unassigned" ? { ...item, action_url: unassignedHref } : item
    )),
    [core?.alerts, unassignedHref],
  )

  const queues = useMemo(
    () => (core?.queues || []).map((item) => (
      item.key === "unassigned" ? { ...item, action_url: unassignedHref } : item
    )),
    [core?.queues, unassignedHref],
  )

  const actionAlerts = useMemo(
    () => alerts.filter((item) => item.count > 0 && (item.severity === "critical" || item.severity === "warning")),
    [alerts],
  )

  const kpiItems = useMemo(() => {
    if (!core) return []
    const { summary } = core
    if (core.scope.restricted) {
      return [
        {
          href: "/enterprise/tickets?status=pending",
          icon: <Clock3Icon className="size-4" />,
          label: t("enterpriseWorkbench.kpi.pendingResponse"),
          tone: summary.pending_tickets > 0 ? "amber" : "green",
          value: summary.pending_tickets,
        },
        {
          href: "/enterprise/tickets?status=processing",
          icon: <TicketCheckIcon className="size-4" />,
          label: t("enterpriseWorkbench.kpi.processing"),
          tone: "blue",
          value: summary.processing_tickets,
        },
        {
          href: "/enterprise/tickets?sla_risk=true",
          icon: <AlertTriangleIcon className="size-4" />,
          label: t("enterpriseWorkbench.kpi.slaRisk"),
          tone: summary.sla_breached_tickets > 0 ? "red" : summary.sla_risk_tickets > 0 ? "amber" : "green",
          value: summary.sla_risk_tickets,
        },
        {
          href: "/enterprise/tickets?status=awaiting_customer",
          icon: <UsersRoundIcon className="size-4" />,
          label: t("enterpriseWorkbench.kpi.awaitingCustomer"),
          tone: "green",
          value: summary.awaiting_customer_tickets,
        },
      ]
    }
    return [
      {
        href: "/enterprise/tickets",
        icon: <TicketCheckIcon className="size-4" />,
        label: t("enterpriseWorkbench.kpi.openTickets"),
        tone: "blue",
        value: summary.open_tickets,
      },
      {
        href: "/enterprise/tickets?sla_risk=true",
        icon: <AlertTriangleIcon className="size-4" />,
        label: t("enterpriseWorkbench.kpi.slaRisk"),
        tone: summary.sla_breached_tickets > 0 ? "red" : summary.sla_risk_tickets > 0 ? "amber" : "green",
        value: summary.sla_risk_tickets,
      },
      {
        href: unassignedHref,
        icon: <UsersRoundIcon className="size-4" />,
        label: t("enterpriseWorkbench.kpi.unassigned"),
        tone: summary.unassigned_tickets > 0 ? "amber" : "green",
        value: summary.unassigned_tickets,
      },
      {
        href: "/enterprise/tickets?status=closed",
        icon: <ShieldCheckIcon className="size-4" />,
        label: t("enterpriseWorkbench.kpi.closedToday"),
        tone: "green",
        value: summary.closed_today_tickets,
      },
    ]
  }, [core, t, unassignedHref])

  const defaultQueueKey = core?.summary.sla_risk_tickets
    ? "sla_risk"
    : core?.summary.urgent_tickets
      ? "urgent"
      : core?.scope.restricted
        ? "pending"
        : core?.summary.unassigned_tickets
          ? "unassigned"
          : "pending"

  const activeQueueKey = selectedQueueKey || defaultQueueKey
  const showingActiveQueue = loadedQueueKey === activeQueueKey
  const queueTickets = showingActiveQueue ? queue?.tickets ?? [] : []
  const queuePagination = showingActiveQueue ? queue?.tickets_pagination ?? null : null
  const activeQueueCount = queuePagination?.total ?? queues.find((item) => item.key === activeQueueKey)?.count ?? queueTickets.length
  const initialCoreLoading = coreLoading && !coreLoaded
  const canShowAdminModules = !core || !core.scope.restricted
  const keyWorkspaceRefreshing = tenantAIEnabled && keyWorkspaceLoading
  const pageRefreshing = coreLoading
    || (workbenchView === "overview" && (collaborationLoading || resourcesLoading || reportOverviewLoading || keyWorkspaceRefreshing || upcomingMeetingLoading))
    || (workbenchView === "queue" && queueLoading)
    || (workbenchView === "insights" && (reportOverviewLoading || reportFailuresLoading || keyWorkspaceRefreshing || upcomingMeetingLoading))
    || (workbenchView === "resources" && (collaborationLoading || resourcesLoading || upcomingMeetingLoading))
  const overviewCount = kpiItems.reduce((sum, item) => {
    const value = typeof item.value === "number" ? item.value : Number(item.value)
    return Number.isFinite(value) ? sum + value : sum
  }, 0)
  const resourceCount = (resources?.products.length ?? 0)
    + (collaboration?.active_conversations ?? 0)
    + (collaboration?.active_meetings ?? 0)
    + upcomingMeetingTotal
  const workbenchTabs = [
    { label: t("enterpriseWorkbench.views.overview"), value: "overview" },
    { count: activeQueueCount, label: t("enterpriseWorkbench.views.tasks"), value: "queue" },
    { count: overviewCount || undefined, label: t("enterpriseWorkbench.views.insights"), value: "insights" },
    { count: resourceCount || undefined, label: t("enterpriseWorkbench.views.resources"), value: "resources" },
  ]

  const loadQueue = useCallback(async (page = queuePage, queueKey = activeQueueKey) => {
    setQueueLoading(true)
    setQueueError("")
    try {
      const res = await fetchEnterpriseWorkbenchQueue({
        page,
        pageSize: WORKBENCH_QUEUE_PAGE_SIZE,
        queueKey,
      })
      if (res.success && res.data) {
        setQueue(res.data)
        setLoadedQueueKey(queueKey)
        setQueueLoaded(true)
      } else {
        setQueueError(res.error?.message || t("enterpriseWorkbench.errors.todayTasksLoadFailed"))
      }
    } catch (error) {
      setQueueError(error instanceof Error ? error.message : t("enterpriseWorkbench.errors.todayTasksLoadFailed"))
    } finally {
      setQueueLoading(false)
    }
  }, [activeQueueKey, queuePage, t])

  useEffect(() => {
    if (workbenchView !== "queue") return undefined
    const timer = window.setTimeout(() => void loadQueue(queuePage, activeQueueKey), 0)
    return () => window.clearTimeout(timer)
  }, [activeQueueKey, loadQueue, queuePage, workbenchView])

  const handleSelectQueue = useCallback((key: string) => {
    setSelectedQueueKey(key)
    setQueuePage(1)
  }, [])

  const refreshAll = useCallback(() => {
    void load()
    if (workbenchView === "overview") {
      void loadCollaboration()
      void loadResources()
      void loadReportOverview()
      if (tenantAIEnabled) void loadKeyWorkspace()
      void loadUpcomingMeetings()
    }
    if (workbenchView === "queue") {
      void loadQueue(queuePage, activeQueueKey)
    }
    if (workbenchView === "insights") {
      void loadReportOverview()
      void loadReportFailures()
      if (tenantAIEnabled) void loadKeyWorkspace()
      void loadUpcomingMeetings()
    }
    if (workbenchView === "resources") {
      void loadCollaboration()
      void loadResources()
      void loadUpcomingMeetings()
    }
  }, [activeQueueKey, load, loadCollaboration, loadKeyWorkspace, loadQueue, loadReportFailures, loadReportOverview, loadResources, loadUpcomingMeetings, queuePage, tenantAIEnabled, workbenchView])

  return (
    <PageShell
      title={t("enterpriseWorkbench.page.title")}
      breadcrumb={useRouteBreadcrumbItems()}
      className="rhd-railops-workbench-page"
      actions={
        <>
          <RailopsButton size="small" onClick={refreshAll} disabled={pageRefreshing}>
            <RefreshCwIcon className={cn("size-3.5", pageRefreshing && "animate-spin")} />
            {t("enterpriseWorkbench.common.refresh")}
          </RailopsButton>
          <Link href="/enterprise/ticket-workbench">
            <RailopsButton size="small">
              {t("enterpriseWorkbench.page.openWorkbench")}
              <ArrowRightIcon className="size-3.5" />
            </RailopsButton>
          </Link>
        </>
      }
    >
      <div className="rhd-railops-workbench-switchbar">
        <div className="rhd-railops-dashboard-tabs rhd-railops-workbench-tabs">
          <FilterTabs
            ariaLabel={t("enterpriseWorkbench.views.aria")}
            items={workbenchTabs}
            value={workbenchView}
            onChange={(value) => router.push(WORKBENCH_VIEW_ROUTES[value as WorkbenchView])}
          />
        </div>
        {initialCoreLoading ? (
          <div className="rhd-railops-workbench-attention-slot" role="status" aria-busy="true" aria-label={t("common.loadingData")}>
            <Skeleton.Node active className="w-full" style={{ width: "100%", height: 36 }} />
          </div>
        ) : actionAlerts.length > 0 ? (
          <div className="rhd-railops-workbench-attention-slot">
            <AttentionBanner alerts={actionAlerts} className="w-full" />
          </div>
        ) : null}
      </div>
      {coreError && !core ? (
        <ErrorState
          title={t("enterpriseWorkbench.errors.coreStateTitle")}
          description={coreError}
          action={{ label: t("enterpriseWorkbench.common.retry"), onClick: load }}
          className="min-h-0 bg-card"
        />
      ) : null}

      {workbenchView === "overview" ? (
        <EnterpriseOverviewPanel
          activeQueueKey={activeQueueKey}
          aiEnabled={tenantAIEnabled}
          hasDeviceConcept={hasDeviceConcept}
          collaboration={collaboration}
          collaborationLoading={collaborationLoading && !collaborationLoaded}
          core={core}
          coreLoading={initialCoreLoading}
          keyWorkspace={keyWorkspace}
          keyWorkspaceError={keyWorkspaceError}
          keyWorkspaceLoading={keyWorkspaceLoading && !keyWorkspaceLoaded}
          onRetryKeyWorkspace={loadKeyWorkspace}
          onRetryMeetingSchedule={loadUpcomingMeetings}
          onRetryOverview={loadReportOverview}
          onSelectQueue={handleSelectQueue}
          overview={reportOverview}
          overviewError={reportOverviewError}
          overviewLoading={reportOverviewLoading && !reportOverviewLoaded}
          queues={queues}
          resources={resources}
          resourcesLoading={resourcesLoading && !resourcesLoaded}
          upcomingMeetingError={upcomingMeetingError}
          upcomingMeetingLoading={upcomingMeetingLoading && !upcomingMeetingLoaded}
          upcomingMeetings={upcomingMeetings}
          upcomingMeetingTotal={upcomingMeetingTotal}
        />
      ) : null}

      {workbenchView === "queue" ? (
        <section className="rhd-railops-workbench-main-grid" role="tabpanel" aria-label={t("enterpriseWorkbench.queue.title")}>
          <QueueWorkspace
            coreLoading={initialCoreLoading}
            error={queueError}
            loading={queueLoading && (!queueLoaded || !showingActiveQueue)}
            onPageChange={setQueuePage}
            onRetry={() => void loadQueue(queuePage, activeQueueKey)}
            queues={queues}
            pagination={queuePagination}
            refreshing={queueLoading && Boolean(queue) && showingActiveQueue}
            selectedKey={activeQueueKey}
            tickets={queueTickets}
            hasDeviceConcept={hasDeviceConcept}
            onSelect={handleSelectQueue}
          />
        </section>
      ) : null}

      {workbenchView === "insights" ? (
        <section className="rhd-railops-workbench-tab-panel space-y-4" role="tabpanel" aria-label={t("enterpriseWorkbench.views.insights")}>
          {canShowAdminModules ? (
            <WorkbenchReportInsights
              aiEnabled={tenantAIEnabled}
              failures={reportFailures}
              failuresError={reportFailuresError}
              failuresLoading={reportFailuresLoading && !reportFailuresLoaded}
              keyWorkspace={keyWorkspace}
              keyWorkspaceError={keyWorkspaceError}
              keyWorkspaceLoading={keyWorkspaceLoading && !keyWorkspaceLoaded}
              onRetryFailures={loadReportFailures}
              onRetryKeyWorkspace={loadKeyWorkspace}
              onRetryTicketSummary={load}
              ticketSummaryError={coreError && core === null ? coreError : ""}
              ticketSummaryItems={kpiItems}
              ticketSummaryLoading={initialCoreLoading}
            />
          ) : (
            <TicketSummaryPanel
              error={coreError && core === null ? coreError : ""}
              items={kpiItems}
              loading={initialCoreLoading}
              onRetry={load}
            />
          )}
          <UpcomingMeetingSchedulePanel
            error={upcomingMeetingError}
            loading={upcomingMeetingLoading && !upcomingMeetingLoaded}
            meetings={upcomingMeetings}
            onRetry={loadUpcomingMeetings}
            total={upcomingMeetingTotal}
          />
        </section>
      ) : null}

      {workbenchView === "resources" ? (
        <section className="rhd-railops-workbench-resource-grid" role="tabpanel" aria-label={t("enterpriseWorkbench.views.resources")}>
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-[320px_minmax(0,1fr)]">
            <div className="space-y-4">
              <WorkFocusPanel
                collaboration={collaboration}
                collaborationError={collaborationError}
                collaborationLoading={collaborationLoading && !collaborationLoaded}
                onRetryCollaboration={loadCollaboration}
              />
              <MeetingPanel
                error={collaborationError}
                loading={collaborationLoading && !collaborationLoaded}
                meetings={collaboration?.meetings || []}
                onRetry={loadCollaboration}
              />
            </div>
            <div className="space-y-4">
              <UpcomingMeetingSchedulePanel
                error={upcomingMeetingError}
                loading={upcomingMeetingLoading && !upcomingMeetingLoaded}
                meetings={upcomingMeetings}
                onRetry={loadUpcomingMeetings}
                total={upcomingMeetingTotal}
              />
              {canShowAdminModules && hasDeviceConcept ? (
                <ProductLoadPanel
                  error={resourcesError}
                  loading={resourcesLoading && !resourcesLoaded}
                  onRetry={loadResources}
                  products={resources?.products || []}
                />
              ) : (
                <TicketSummaryPanel
                  error={coreError && core === null ? coreError : ""}
                  items={kpiItems}
                  loading={initialCoreLoading}
                  onRetry={load}
                />
              )}
            </div>
          </div>
        </section>
      ) : null}
    </PageShell>
  )
}
