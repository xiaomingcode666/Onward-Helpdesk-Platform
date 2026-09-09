"use client"

import Link from "next/link"
import { useCallback, useEffect, useMemo, useState, type CSSProperties, type ReactNode } from "react"
import {
  ActivityIcon,
  Building2Icon,
  CheckCircle2Icon,
  ChevronRightIcon,
  CircleAlertIcon,
  Clock3Icon,
  DatabaseIcon,
  GaugeIcon,
  RefreshCwIcon,
  ServerIcon,
  ShieldCheckIcon,
  TicketCheckIcon,
} from "lucide-react"
import { Skeleton } from "antd"
import {
  ChartContainer,
  ContentModule,
  FilterTabs,
  PageShell,
  RailopsButton,
  StatusTag,
  type StatusTagTone,
} from "@railops/ui"

import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import {
  formatBytes,
  formatCompactNumber,
  formatDateTime,
  formatDuration,
  formatNumber,
} from "@/components/platform/platform-live-ui"
import { getAuditActionLabel } from "@/lib/audit-i18n"
import {
  fetchPlatformOpsOverview,
  fetchPlatformOverviewEvents,
  fetchPlatformOverviewMetrics,
  fetchPlatformOverviewTopTenants,
  fetchPlatformOverviewUsageTrend,
  type PlatformOverview,
  type PlatformOpsOverview,
} from "@/lib/api/platform"
import { cn } from "@/lib/utils"
import { useI18n } from "@/i18n/provider"

type I18nT = ReturnType<typeof useI18n>

let platformT: (key: string, values?: Record<string, string | number>) => string = (key) => key
type OverviewUnavailableSection =
  | "tenants"
  | "tickets"
  | "knowledge"
  | "aiUsage"
  | "meetings"
  | "topTenants"
  | "dailyUsage"
  | "recentEvents"

function formatPercent(value: number) {
  if (!Number.isFinite(value)) return "0%"
  return `${Math.round(value)}%`
}

function overviewSectionUnavailable(data: PlatformOverview, section: OverviewUnavailableSection) {
  return data.unavailableSections.includes(section)
}

function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback
}

function overviewLabel(t: I18nT, key: string, values?: Record<string, string | number>) {
  return t(`platformExtract.overview.labels.${key}`, values)
}

function overviewSectionLabel(t: I18nT, section: string) {
  const translated = t(`platformExtract.overview.sections.${section}`)
  return translated === `platformExtract.overview.sections.${section}` ? section : translated
}

function formatUnavailableSections(t: I18nT, sections: string[]) {
  return sections.map((section) => overviewSectionLabel(t, section)).join(", ")
}

type PanelLoadingVariant = "bars" | "chart" | "list" | "metrics" | "ops"
type PlatformOverviewSection = "overview" | "tenant" | "service" | "ops"

function PanelLoading({
  className,
  label,
  variant = "list",
}: {
  className?: string
  label: string
  variant?: PanelLoadingVariant
}) {
  if (variant === "metrics") {
    return (
      <div className={cn("grid grid-cols-2 gap-2.5 p-3", className)} role="status" aria-busy="true" aria-label={label}>
        <span className="sr-only">{label}</span>
        {Array.from({ length: 4 }, (_, index) => (
          <div key={index} className="flex min-h-20 flex-col items-center justify-center rounded bg-muted px-3 py-3">
            <Skeleton.Node active style={{ width: 48, height: 24 }} />
            <Skeleton.Node active className="mt-2" style={{ width: 80, height: 12 }} />
          </div>
        ))}
      </div>
    )
  }

  if (variant === "chart") {
    return (
      <div className={cn("space-y-3 px-3 pb-3 pt-4 sm:px-4", className)} role="status" aria-busy="true" aria-label={label}>
        <span className="sr-only">{label}</span>
        <Skeleton.Node active className="w-full" style={{ width: "100%", height: 210 }} />
        <div className="flex justify-center gap-4">
          <Skeleton.Node active style={{ width: 64, height: 12 }} />
          <Skeleton.Node active style={{ width: 80, height: 12 }} />
          <Skeleton.Node active style={{ width: 96, height: 12 }} />
        </div>
      </div>
    )
  }

  if (variant === "ops") {
    return (
      <div className={cn("space-y-3 p-3", className)} role="status" aria-busy="true" aria-label={label}>
        <span className="sr-only">{label}</span>
        <div className="grid gap-4 lg:grid-cols-[1fr_1fr]">
          <div className="space-y-4">
            <Skeleton.Node active className="w-full" style={{ width: "100%", height: 40 }} />
            <Skeleton.Node active className="w-full" style={{ width: "100%", height: 40 }} />
          </div>
          <div className="grid grid-cols-2 gap-2">
            {Array.from({ length: 4 }, (_, index) => <Skeleton.Node key={index} active className="rounded" style={{ width: "100%", height: 70 }} />)}
          </div>
        </div>
        <div className="grid grid-cols-2 gap-px sm:grid-cols-4">
          {Array.from({ length: 4 }, (_, index) => <Skeleton.Node key={index} active className="rounded-none" style={{ width: "100%", height: 56 }} />)}
        </div>
      </div>
    )
  }

  if (variant === "bars") {
    return (
      <div className={cn("space-y-2.5 p-3", className)} role="status" aria-busy="true" aria-label={label}>
        <span className="sr-only">{label}</span>
        {Array.from({ length: 5 }, (_, index) => (
        <div key={index} className="grid gap-2 sm:grid-cols-[minmax(104px,0.72fr)_1fr_1fr] sm:items-center">
            <div className="space-y-2">
              <Skeleton.Node active style={{ width: 96, height: 16 }} />
              <Skeleton.Node active style={{ width: 64, height: 12 }} />
            </div>
            <Skeleton.Node active className="w-full" style={{ width: "100%", height: 32 }} />
            <Skeleton.Node active className="w-full" style={{ width: "100%", height: 32 }} />
          </div>
        ))}
      </div>
    )
  }

  return (
    <div className={cn("divide-y divide-border", className)} role="status" aria-busy="true" aria-label={label}>
      <span className="sr-only">{label}</span>
      {Array.from({ length: 5 }, (_, index) => (
        <div key={index} className="grid gap-2 px-4 py-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
          <div className="flex items-start gap-3">
            <Skeleton.Node active className="mt-1.5 shrink-0 rounded-full" style={{ width: 8, height: 8 }} />
            <div className="min-w-0 flex-1 space-y-2">
              <Skeleton.Node active style={{ width: 128, height: 16 }} />
              <Skeleton.Node active style={{ width: 176, height: 12 }} />
            </div>
          </div>
          <Skeleton.Node active className="sm:ml-auto" style={{ width: 80, height: 20 }} />
        </div>
      ))}
    </div>
  )
}

function PanelError({ message, onRetry }: { message: string; onRetry: () => void }) {
  const t = useI18n()
  return (
    <div className="rhd-railops-error-state grid min-h-64 place-items-center px-6 py-8 text-center">
      <div>
        <CircleAlertIcon className="mx-auto size-7 text-destructive" />
        <p className="mt-3 text-sm font-medium text-foreground">{message}</p>
        <RailopsButton className="mt-4" size="small" onClick={onRetry}>
          <RefreshCwIcon className="size-3.5" />
          {overviewLabel(t, "retry")}
        </RailopsButton>
      </div>
    </div>
  )
}

function UnavailableBlock({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div className={cn("grid min-h-64 place-items-center px-6 text-center text-sm text-muted-foreground", className)}>
      {children}
    </div>
  )
}

type OverviewDataPanelProps = {
  data: PlatformOverview | null
  error: string
  initialLoading: boolean
  loading: boolean
  onRetry: () => void
}

function OverviewPanel({
  title,
  action,
  children,
  className,
  loading = false,
  loadingLabel,
}: {
  title: string
  action?: ReactNode
  children: ReactNode
  className?: string
  loading?: boolean
  loadingLabel?: string
}) {
  const t = useI18n()
  const extra = loading || action ? (
    <div className="rhd-railops-platform-overview-panel-extra">
      {loading ? <RefreshCwIcon className="size-3.5 animate-spin text-muted-foreground" aria-label={loadingLabel || overviewLabel(t, "refreshingTitle", { title })} /> : null}
      {action}
    </div>
  ) : undefined

  return (
    <ContentModule
      title={title}
      extra={extra}
      className={cn("rhd-railops-platform-overview-panel", className)}
    >
      <div aria-busy={loading || undefined}>{children}</div>
    </ContentModule>
  )
}

function PanelLink({ href, children }: { href: string; children: ReactNode }) {
  return (
    <Link
      href={href}
      className="inline-flex h-7 items-center gap-1 text-xs font-medium text-muted-foreground transition hover:text-primary"
    >
      {children}
      <ChevronRightIcon className="size-3.5" />
    </Link>
  )
}

type KpiTone = "blue" | "success" | "amber" | "slate"

const kpiToneMap: Record<KpiTone, StatusTagTone> = {
  blue: "blue",
  success: "success",
  amber: "warning",
  slate: "neutral",
}

function KpiCard({
  label,
  value,
  detail,
  icon,
  tone,
  loading = false,
}: {
  label: string
  value?: string
  detail?: string
  icon: ReactNode
  tone: KpiTone
  loading?: boolean
}) {
  const t = useI18n()
  return (
    <div
      className={cn("rhd-railops-platform-kpi", `tone-${tone}`)}
      role={loading ? "status" : undefined}
      aria-busy={loading || undefined}
      aria-label={loading ? overviewLabel(t, "loadingNamed", { label }) : undefined}
    >
      <div className="flex items-center justify-between gap-2">
        <span className="rhd-railops-platform-kpi-label">{label}</span>
        <span className="rhd-railops-platform-kpi-icon">{icon}</span>
      </div>
      {loading ? (
        <>
          <Skeleton.Node active className="mt-2" style={{ width: 64, height: 22 }} />
          <Skeleton.Node active className="mt-2" style={{ width: 96, height: 12 }} />
        </>
      ) : (
        <>
          <div className="rhd-railops-platform-kpi-value" title={value}>
            {value}
          </div>
          {detail ? <StatusTag tone={kpiToneMap[tone]} className="rhd-railops-platform-kpi-detail">{detail}</StatusTag> : null}
        </>
      )}
    </div>
  )
}

function KpiLoadingGrid() {
  const t = useI18n()
  const items = [
    { label: overviewLabel(t, "pendingTickets"), icon: <TicketCheckIcon className="size-3.5" />, tone: "blue" as const },
    { label: overviewLabel(t, "slaRisk"), icon: <Clock3Icon className="size-3.5" />, tone: "blue" as const },
    { label: overviewLabel(t, "knowledgePending"), icon: <GaugeIcon className="size-3.5" />, tone: "blue" as const },
    { label: overviewLabel(t, "abnormalAccess"), icon: <CircleAlertIcon className="size-3.5" />, tone: "slate" as const },
  ]
  return (
    <section className="rhd-railops-platform-kpi-grid" aria-label={overviewLabel(t, "processingItems")}>
      {items.map((item) => <KpiCard key={item.label} {...item} loading />)}
    </section>
  )
}

function KpiErrorGrid({ message, onRetry }: { message: string; onRetry: () => void }) {
  const t = useI18n()
  return (
    <section className="rhd-railops-error-state rounded-md p-4" aria-label={overviewLabel(t, "processingItems")}>
      <div className="flex min-h-24 flex-col items-center justify-center text-center">
        <p className="text-sm font-medium text-foreground">{message}</p>
        <RailopsButton className="mt-3" size="small" onClick={onRetry}>
          <RefreshCwIcon className="size-3.5" />
          {overviewLabel(t, "retry")}
        </RailopsButton>
      </div>
    </section>
  )
}

function ScaleBar({
  label,
  value,
  max,
  color,
}: {
  label: string
  value: number
  max: number
  color: string
}) {
  const width = value <= 0 ? 0 : Math.max(4, (value / Math.max(max, 1)) * 100)
  return (
    <div className="min-w-0">
      <div className="mb-1 flex items-center justify-between gap-2 text-rhd-2xs text-muted-foreground">
        <span>{label}</span>
        <span className="font-medium tabular-nums text-foreground">{formatCompactNumber(value)}</span>
      </div>
      <div className="h-2 overflow-hidden rounded-sm bg-muted">
        <div className={cn("h-full rounded-sm", color)} style={{ width: `${width}%` }} />
      </div>
    </div>
  )
}

function TenantScalePanel({ data, error, initialLoading, loading, onRetry }: OverviewDataPanelProps) {
  if (initialLoading) {
    return (
      <OverviewPanel
        title={platformT("platformExtract.overview.labels.tenantScale")}
        action={<PanelLink href="/platform/tenants">{platformT("platformExtract.overview.labels.tenantManagement")}</PanelLink>}
      >
        <PanelLoading variant="bars" label={platformT("platformExtract.overview.labels.tenantScaleLoading")} />
      </OverviewPanel>
    )
  }

  if (!data) {
    return (
      <OverviewPanel title={platformT("platformExtract.overview.labels.tenantScale")} action={<PanelLink href="/platform/tenants">{platformT("platformExtract.overview.labels.tenantManagement")}</PanelLink>}>
        {error ? <PanelError message={error} onRetry={onRetry} /> : <UnavailableBlock>{platformT("platformExtract.overview.labels.tenantDataUnavailable")}</UnavailableBlock>}
      </OverviewPanel>
    )
  }

  const tenants = data.topTenants.slice(0, 5)
  const maxDevices = Math.max(...tenants.map((tenant) => tenant.devices), 1)
  const maxRequests = Math.max(...tenants.map((tenant) => tenant.aiRequests), 1)
  const unavailable = overviewSectionUnavailable(data, "topTenants")

  return (
    <OverviewPanel
      title={platformT("platformExtract.overview.labels.tenantScale")}
      action={<PanelLink href="/platform/tenants">{platformT("platformExtract.overview.labels.tenantManagement")}</PanelLink>}
      loading={loading}
      loadingLabel={platformT("platformExtract.overview.labels.tenantScaleRefresh")}
    >
      {unavailable ? (
        <UnavailableBlock>{platformT("platformExtract.overview.labels.tenantDataUnavailable")}</UnavailableBlock>
      ) : tenants.length ? (
        <div className="space-y-2.5 p-3">
          {tenants.map((tenant) => (
            <div key={tenant.tenantId} className="grid min-w-0 gap-2 sm:grid-cols-[minmax(104px,0.72fr)_1fr_1fr] sm:items-center">
              <div className="min-w-0">
                <div className="truncate text-xs font-medium text-foreground" title={tenant.tenantName}>{tenant.tenantName}</div>
                <div className="mt-0.5 truncate text-rhd-2xs text-muted-foreground">{tenant.planName || platformT("platformExtract.overview.labels.planUnconfigured")}</div>
              </div>
              <ScaleBar label={platformT("platformExtract.overview.labels.device")} value={tenant.devices} max={maxDevices} color="bg-primary" />
              <ScaleBar label={platformT("platformExtract.overview.labels.modelRequests")} value={tenant.aiRequests} max={maxRequests} color="bg-primary" />
            </div>
          ))}
          <div className="flex items-center gap-4 border-t border-border pt-2.5 text-rhd-2xs text-muted-foreground">
            <span className="flex items-center gap-1.5"><i className="size-2 rounded-sm bg-primary" />{platformT("platformExtract.overview.labels.deviceCount")}</span>
            <span className="flex items-center gap-1.5"><i className="size-2 rounded-sm bg-primary" />{platformT("platformExtract.overview.labels.monthlyModelRequests")}</span>
          </div>
        </div>
      ) : (
        <UnavailableBlock>{platformT("platformExtract.overview.labels.noTenantScale")}</UnavailableBlock>
      )}
    </OverviewPanel>
  )
}

function MetricCell({ label, value, tone = "slate" }: { label: string; value: string; tone?: "slate" | "destructive" | "amber" | "success" | "blue" }) {
  const valueClass = {
    slate: "text-foreground",
    destructive: "text-destructive",
    amber: "text-amber-600",
    success: "text-[#047857]",
    blue: "text-primary",
  }
  return (
    <div className="flex min-h-20 flex-col items-center justify-center rounded bg-muted px-3 py-3 text-center">
      <strong className={cn("text-xl font-semibold leading-6 tabular-nums", valueClass[tone])}>{value}</strong>
      <span className="mt-1.5 text-rhd-xs text-muted-foreground">{label}</span>
    </div>
  )
}

function TicketPanel({ data, error, initialLoading, loading, onRetry }: OverviewDataPanelProps) {
  if (initialLoading) {
    return (
      <OverviewPanel title={platformT("platformExtract.overview.labels.ticketOverview")} action={<PanelLink href="/platform/tenants">{platformT("platformExtract.overview.labels.viewTenants")}</PanelLink>}>
        <PanelLoading variant="metrics" label={platformT("platformExtract.overview.labels.ticketLoading")} />
      </OverviewPanel>
    )
  }

  if (!data) {
    return (
      <OverviewPanel title={platformT("platformExtract.overview.labels.ticketOverview")} action={<PanelLink href="/platform/tenants">{platformT("platformExtract.overview.labels.viewTenants")}</PanelLink>}>
        {error ? <PanelError message={error} onRetry={onRetry} /> : <UnavailableBlock className="min-h-[228px]">{platformT("platformExtract.overview.labels.ticketDataUnavailable")}</UnavailableBlock>}
      </OverviewPanel>
    )
  }

  const unavailable = overviewSectionUnavailable(data, "tickets")
  const completed = Math.max(data.tickets.total - data.tickets.open, 0)
  return (
    <OverviewPanel
      title={platformT("platformExtract.overview.labels.ticketOverview")}
      action={<PanelLink href="/platform/tenants">{platformT("platformExtract.overview.labels.viewTenants")}</PanelLink>}
      loading={loading}
      loadingLabel={platformT("platformExtract.overview.labels.ticketRefresh")}
    >
      {unavailable ? (
        <UnavailableBlock className="min-h-[228px]">{platformT("platformExtract.overview.labels.ticketDataUnavailable")}</UnavailableBlock>
      ) : (
        <div className="grid grid-cols-2 gap-2.5 p-3">
          <MetricCell label={platformT("platformExtract.overview.labels.openTickets")} value={formatNumber(data.tickets.open)} tone={data.tickets.open > 0 ? "amber" : "success"} />
          <MetricCell label={platformT("platformExtract.overview.labels.todayAdded")} value={formatNumber(data.tickets.createdToday)} tone="blue" />
          <MetricCell label={platformT("platformExtract.overview.labels.completed")} value={formatNumber(completed)} tone="success" />
          <MetricCell label={platformT("platformExtract.overview.labels.slaRisk")} value={formatNumber(data.tickets.slaRisk)} tone={data.tickets.slaRisk > 0 ? "destructive" : "success"} />
        </div>
      )}
    </OverviewPanel>
  )
}

function KnowledgePanel({ data, error, initialLoading, loading, onRetry }: OverviewDataPanelProps) {
  if (initialLoading) {
    return (
      <OverviewPanel title={platformT("platformExtract.overview.labels.knowledgeDiagnosis")} action={<PanelLink href="/platform/models">{platformT("platformExtract.overview.labels.modelResources")}</PanelLink>}>
        <PanelLoading variant="metrics" label={platformT("platformExtract.overview.labels.knowledgeLoading")} />
      </OverviewPanel>
    )
  }

  if (!data) {
    return (
      <OverviewPanel title={platformT("platformExtract.overview.labels.knowledgeDiagnosis")} action={<PanelLink href="/platform/models">{platformT("platformExtract.overview.labels.modelResources")}</PanelLink>}>
        {error ? <PanelError message={error} onRetry={onRetry} /> : <UnavailableBlock className="min-h-[268px]">{platformT("platformExtract.overview.labels.knowledgeDataUnavailable")}</UnavailableBlock>}
      </OverviewPanel>
    )
  }

  const knowledgeUnavailable = overviewSectionUnavailable(data, "knowledge")
  const aiUsageUnavailable = overviewSectionUnavailable(data, "aiUsage")
  const indexedRate = data.knowledge.documents > 0
    ? (data.knowledge.indexed / data.knowledge.documents) * 100
    : 0

  return (
    <OverviewPanel
      title={platformT("platformExtract.overview.labels.knowledgeDiagnosis")}
      action={<PanelLink href="/platform/models">{platformT("platformExtract.overview.labels.modelResources")}</PanelLink>}
      loading={loading}
      loadingLabel={platformT("platformExtract.overview.labels.knowledgeRefresh")}
    >
      {knowledgeUnavailable ? (
        <UnavailableBlock className="min-h-[268px]">{platformT("platformExtract.overview.labels.knowledgeDataUnavailable")}</UnavailableBlock>
      ) : (
        <>
          <div className="grid grid-cols-2 gap-2.5 p-3">
            <MetricCell label={platformT("platformExtract.overview.labels.knowledgeDocs")} value={formatNumber(data.knowledge.documents)} tone="blue" />
            <MetricCell label={platformT("platformExtract.overview.labels.indexed")} value={formatNumber(data.knowledge.indexed)} tone="success" />
            <MetricCell label={platformT("platformExtract.overview.labels.modelRequests")} value={aiUsageUnavailable ? "-" : formatCompactNumber(data.aiUsage.requests)} tone="blue" />
            <MetricCell label={platformT("platformExtract.overview.labels.pending")} value={formatNumber(data.knowledge.pending)} tone={data.knowledge.pending > 0 ? "amber" : "success"} />
          </div>
          <div className="border-t border-border px-4 py-3">
            <div className="mb-2 flex items-center justify-between text-rhd-xs">
              <span className="text-muted-foreground">{platformT("platformExtract.overview.labels.knowledgeCoverage")}</span>
              <span className="font-semibold tabular-nums text-foreground">{formatPercent(indexedRate)}</span>
            </div>
            <div className="h-2 overflow-hidden rounded-sm bg-muted">
              <div className="h-full rounded-sm bg-primary" style={{ width: `${Math.min(indexedRate, 100)}%` }} />
            </div>
          </div>
        </>
      )}
    </OverviewPanel>
  )
}

function UsageTrendPanel({ data, error, initialLoading, loading, onRetry }: OverviewDataPanelProps) {
  if (initialLoading) {
    return (
      <OverviewPanel title={platformT("platformExtract.overview.labels.usageTrend")} action={<PanelLink href="/platform/usage">{platformT("platformExtract.overview.labels.usageTrend")}</PanelLink>}>
        <PanelLoading variant="chart" label={platformT("platformExtract.overview.labels.usageLoading")} />
      </OverviewPanel>
    )
  }

  if (!data) {
    return (
      <OverviewPanel title={platformT("platformExtract.overview.labels.usageTrend")} action={<PanelLink href="/platform/usage">{platformT("platformExtract.overview.labels.usageTrend")}</PanelLink>}>
        {error ? <PanelError message={error} onRetry={onRetry} /> : <UnavailableBlock className="min-h-72">{platformT("platformExtract.overview.labels.usageDataUnavailable")}</UnavailableBlock>}
      </OverviewPanel>
    )
  }

  const unavailable = overviewSectionUnavailable(data, "dailyUsage")
  const chartData = data.dailyUsage.map((item) => ({
    ...item,
    day: item.date.slice(5),
  }))

  return (
    <OverviewPanel
      title={platformT("platformExtract.overview.labels.usageTrend")}
      action={<PanelLink href="/platform/usage">{platformT("platformExtract.overview.labels.usageTrend")}</PanelLink>}
      loading={loading}
      loadingLabel={platformT("platformExtract.overview.labels.usageRefresh")}
    >
      {unavailable ? (
        <UnavailableBlock className="min-h-72">{platformT("platformExtract.overview.labels.usageDataUnavailable")}</UnavailableBlock>
      ) : chartData.length ? (
        <div className="px-2 pb-3 pt-3 sm:px-3">
          <ChartContainer
            height={210}
            option={{
              xAxis: {
                type: "category",
                data: chartData.map((item) => item.day),
                axisLine: { show: false },
                axisTick: { show: false },
                axisLabel: { margin: 10 },
              },
              yAxis: {
                type: "value",
                axisLine: { show: false },
                axisTick: { show: false },
                axisLabel: { formatter: (value: number | string) => formatCompactNumber(Number(value)) },
              },
              series: [
                {
                  type: "line",
                  smooth: true,
                  symbol: "none",
                  data: chartData.map((item) => item.aiRequests),
                  lineStyle: { color: "#3B82F6", width: 2 },
                  areaStyle: {
                    color: {
                      type: "linear",
                      x: 0,
                      y: 0,
                      x2: 0,
                      y2: 1,
                      colorStops: [
                        { offset: 0, color: "rgba(59, 130, 246, 0.24)" },
                        { offset: 1, color: "rgba(59, 130, 246, 0.02)" },
                      ],
                    },
                  },
                },
              ],
            }}
          />
          <div className="flex flex-wrap items-center justify-center gap-4 text-rhd-2xs text-muted-foreground">
            <span className="flex items-center gap-1.5"><i className="size-2 rounded-sm bg-primary" />{platformT("platformExtract.overview.labels.modelRequests")}</span>
            <span>{platformT("platformExtract.overview.labels.totalRequests", { count: formatCompactNumber(data.aiUsage.requests) })}</span>
            <span>{formatCompactNumber(data.aiUsage.tokens)} Token</span>
          </div>
        </div>
      ) : (
        <UnavailableBlock className="min-h-72">{platformT("platformExtract.overview.labels.noUsageTrend")}</UnavailableBlock>
      )}
    </OverviewPanel>
  )
}

function ProgressSignal({ label, value, detail, tone = "blue" }: { label: string; value: number; detail: string; tone?: "blue" | "success" | "amber" }) {
  const fillClass = {
    blue: "bg-primary",
    success: "bg-[var(--railops-success)]",
    amber: "bg-amber-500",
  }
  return (
    <div>
      <div className="flex items-center justify-between gap-3 text-rhd-xs">
        <span className="font-medium text-muted-foreground">{label}</span>
        <span className="font-semibold tabular-nums text-foreground">{formatPercent(value)}</span>
      </div>
      <div className="mt-2 h-2 overflow-hidden rounded-sm bg-muted">
        <div className={cn("h-full rounded-sm", fillClass[tone])} style={{ width: `${Math.max(0, Math.min(value, 100))}%` }} />
      </div>
      <p className="mt-1.5 truncate text-rhd-2xs text-muted-foreground" title={detail}>{detail}</p>
    </div>
  )
}

function OpsHealthPanel({
  error,
  initialLoading,
  loading,
  onRetry,
  ops,
}: {
  error: string
  initialLoading: boolean
  loading: boolean
  onRetry: () => void
  ops: PlatformOpsOverview | null
}) {
  if (initialLoading) {
    return (
      <OverviewPanel title={platformT("platformExtract.overview.labels.systemOpsStatus")} action={<PanelLink href="/platform/ops">{platformT("platformExtract.overview.labels.operationsCenter")}</PanelLink>}>
        <PanelLoading variant="ops" label={platformT("platformExtract.overview.labels.opsHealthLoading")} />
      </OverviewPanel>
    )
  }

  if (!ops) {
    return (
      <OverviewPanel title={platformT("platformExtract.overview.labels.systemOpsStatus")} action={<PanelLink href="/platform/ops">{platformT("platformExtract.overview.labels.operationsCenter")}</PanelLink>}>
        {error ? <PanelError message={error} onRetry={onRetry} /> : <UnavailableBlock className="min-h-[292px]">{platformT("platformExtract.overview.labels.noOpsStatus")}</UnavailableBlock>}
      </OverviewPanel>
    )
  }

  const heapRatio = ops.runtime.heapSysBytes > 0 ? (ops.runtime.heapAllocBytes / ops.runtime.heapSysBytes) * 100 : 0
  const connectionRatio = ops.database.maxOpen > 0 ? (ops.database.openConnections / ops.database.maxOpen) * 100 : 0
  const pendingWorkload = ops.workloads.openTickets + ops.workloads.pendingKnowledge + ops.workloads.unhealthyConnectors

  return (
    <OverviewPanel
      title={platformT("platformExtract.overview.labels.systemOpsStatus")}
      action={<PanelLink href="/platform/ops">{platformT("platformExtract.overview.labels.operationsCenter")}</PanelLink>}
      loading={loading}
      loadingLabel={platformT("platformExtract.overview.labels.opsHealthRefresh")}
    >
      <div className="grid gap-4 p-3 lg:grid-cols-[1fr_1fr]">
        <div className="space-y-4">
          <ProgressSignal
            label={platformT("platformExtract.overview.labels.goHeapMemory")}
            value={heapRatio}
            detail={`${formatBytes(ops.runtime.heapAllocBytes)} / ${formatBytes(ops.runtime.heapSysBytes)}`}
            tone={heapRatio >= 80 ? "amber" : "blue"}
          />
          <ProgressSignal
            label={platformT("platformExtract.overview.labels.databasePool")}
            value={connectionRatio}
            detail={platformT("platformExtract.overview.labels.databasePoolDetail", {
              latency: formatNumber(ops.database.latencyMs),
              max: formatNumber(ops.database.maxOpen),
              open: formatNumber(ops.database.openConnections),
            })}
            tone={ops.database.status === "healthy" ? "blue" : "amber"}
          />
        </div>
        <div className="grid grid-cols-2 gap-2">
          <OpsTile icon={<Clock3Icon />} label={platformT("platformExtract.overview.labels.uptime")} value={formatDuration(ops.uptimeSeconds)} />
          <OpsTile icon={<ServerIcon />} label="Goroutine" value={formatNumber(ops.runtime.goroutines)} />
          <OpsTile icon={<DatabaseIcon />} label={platformT("platformExtract.overview.labels.database")} value={ops.database.status === "healthy" ? platformT("platformExtract.overview.labels.normal") : platformT("platformExtract.overview.labels.abnormal")} healthy={ops.database.status === "healthy"} />
          <OpsTile icon={<CircleAlertIcon />} label={platformT("platformExtract.overview.labels.pendingWorkload")} value={formatNumber(pendingWorkload)} healthy={pendingWorkload === 0} />
        </div>
      </div>
      <div className="grid grid-cols-2 divide-x divide-border border-t border-border sm:grid-cols-4">
        <CompactHealth label={platformT("platformExtract.overview.labels.openTickets")} value={ops.workloads.openTickets} />
        <CompactHealth label={platformT("platformExtract.overview.labels.knowledgeQueue")} value={ops.workloads.pendingKnowledge} />
        <CompactHealth label={platformT("platformExtract.overview.labels.abnormalAccess")} value={ops.workloads.unhealthyConnectors} danger={ops.workloads.unhealthyConnectors > 0} />
        <CompactHealth label={platformT("platformExtract.overview.labels.activeMeetings")} value={ops.workloads.activeMeetings} />
      </div>
    </OverviewPanel>
  )
}

function OpsTile({ icon, label, value, healthy }: { icon: ReactNode; label: string; value: string; healthy?: boolean }) {
  return (
    <div className="min-w-0 rounded border border-border bg-muted px-3 py-3">
      <div className="flex items-center gap-1.5 text-rhd-2xs text-muted-foreground">
        <span className="[&>svg]:size-3.5">{icon}</span>
        <span>{label}</span>
        {healthy !== undefined ? <i className={cn("ml-auto size-1.5 rounded-full", healthy ? "bg-[var(--railops-success)]" : "bg-destructive")} /> : null}
      </div>
      <div className="mt-2 truncate text-sm font-semibold tabular-nums text-foreground" title={value}>{value}</div>
    </div>
  )
}

function CompactHealth({ label, value, danger = false }: { label: string; value: number; danger?: boolean }) {
  return (
    <div className="px-3 py-3 text-center">
      <div className={cn("text-base font-semibold tabular-nums", danger ? "text-destructive" : "text-foreground")}>{formatNumber(value)}</div>
      <div className="mt-0.5 text-rhd-2xs text-muted-foreground">{label}</div>
    </div>
  )
}

type ExecutiveTone = "blue" | "success" | "amber" | "red" | "slate"

const executiveToneMap: Record<ExecutiveTone, StatusTagTone> = {
  blue: "blue",
  success: "success",
  amber: "warning",
  red: "error",
  slate: "neutral",
}

function latestUsagePoint(data: PlatformOverview | null) {
  if (!data?.dailyUsage.length) return null
  return data.dailyUsage[data.dailyUsage.length - 1]
}

function previousUsagePoint(data: PlatformOverview | null) {
  if (!data || data.dailyUsage.length < 2) return null
  return data.dailyUsage[data.dailyUsage.length - 2]
}

function platformOpsHealthScore(ops: PlatformOpsOverview | null) {
  if (!ops) return 0
  let score = 100
  if (ops.database.status !== "healthy") score -= 28
  if (ops.jitsi.heartbeatStatus === "degraded") score -= 12
  score -= Math.min(24, ops.workloads.unhealthyConnectors * 8)
  score -= Math.min(18, ops.pipeline.failedSyncs24h * 6)
  score -= Math.min(18, ops.knowledgeIndex.failedTasks24h * 6)
  return Math.max(0, Math.round(score))
}

function ExecutiveMetric({
  detail,
  href,
  icon,
  label,
  loading = false,
  tone,
  value,
}: {
  detail: string
  href: string
  icon: ReactNode
  label: string
  loading?: boolean
  tone: ExecutiveTone
  value: string
}) {
  const body = (
    <div className={cn("rhd-railops-platform-exec-metric", `tone-${tone}`)}>
      <div className="flex min-w-0 items-center justify-between gap-2">
        <span className="rhd-railops-platform-exec-metric-label">{label}</span>
        <span className="rhd-railops-platform-exec-metric-icon">{icon}</span>
      </div>
      {loading ? (
        <>
          <Skeleton.Node active className="mt-3" style={{ width: 80, height: 24 }} />
          <Skeleton.Node active className="mt-2" style={{ width: 112, height: 12 }} />
        </>
      ) : (
        <>
          <strong className="rhd-railops-platform-exec-metric-value">{value}</strong>
          <StatusTag tone={executiveToneMap[tone]} className="rhd-railops-platform-exec-metric-detail">{detail}</StatusTag>
        </>
      )}
    </div>
  )
  return (
    <Link href={href} className="block min-w-0 no-underline">
      {body}
    </Link>
  )
}

function LifecycleRow({ label, value, total, tone }: { label: string; value: number; total: number; tone: ExecutiveTone }) {
  const width = total > 0 ? Math.max(4, Math.min(100, (value / total) * 100)) : 0
  return (
    <div className="rhd-railops-platform-lifecycle-row">
      <span>{label}</span>
      <strong>{formatNumber(value)}</strong>
      <i aria-hidden="true">
        <b className={`tone-${tone}`} style={{ width: `${width}%` }} />
      </i>
    </div>
  )
}

function PlatformExecutiveOverview({
  events,
  eventsLoading,
  metrics,
  metricsLoading,
  ops,
  opsLoading,
  t,
  tenantScale,
  tenantScaleLoading,
  usageTrend,
  usageTrendLoading,
}: {
  events: PlatformOverview | null
  eventsLoading: boolean
  metrics: PlatformOverview | null
  metricsLoading: boolean
  ops: PlatformOpsOverview | null
  opsLoading: boolean
  t: ReturnType<typeof useI18n>
  tenantScale: PlatformOverview | null
  tenantScaleLoading: boolean
  usageTrend: PlatformOverview | null
  usageTrendLoading: boolean
}) {
  const latestUsage = latestUsagePoint(usageTrend)
  const previousUsage = previousUsagePoint(usageTrend)
  const todayRequests = latestUsage?.aiRequests ?? 0
  const requestDelta = previousUsage ? todayRequests - previousUsage.aiRequests : 0
  const activeTenantSample = tenantScale?.topTenants.filter((tenant) => tenant.aiRequests > 0).length ?? 0
  const tenantTotal = metrics?.tenants.total ?? tenantScale?.topTenants.length ?? 0
  const activeTenants = metrics?.tenants.active ?? 0
  const trialTenants = metrics?.tenants.trial ?? 0
  const frozenTenants = metrics?.tenants.frozen ?? 0
  const expiringTenants = metrics?.tenants.expiringSoon ?? 0
  const tenantActiveRate = tenantTotal > 0 ? (activeTenants / tenantTotal) * 100 : 0
  const opsScore = platformOpsHealthScore(ops)
  const opsTone: ExecutiveTone = opsScore >= 86 ? "success" : opsScore >= 70 ? "amber" : "red"
  const pendingWorkload = (metrics?.tickets.open ?? 0) + (metrics?.knowledge.pending ?? 0) + (ops?.workloads.unhealthyConnectors ?? 0)
  const trendMax = Math.max(...(usageTrend?.dailyUsage.map((item) => item.aiRequests) ?? [0]), 1)
  const recentEvents = events?.recentEvents.slice(0, 3) ?? []
  const topActiveTenants = tenantScale?.topTenants.filter((tenant) => tenant.aiRequests > 0).slice(0, 3) ?? []

  return (
    <section className="rhd-railops-platform-executive-overview" aria-label={platformT("platformExtract.overview.labels.platformOverview")}>
      <div className="rhd-railops-platform-exec-metric-grid">
        <ExecutiveMetric
          detail={previousUsage
            ? platformT("platformExtract.overview.labels.requestDelta", { delta: `${requestDelta >= 0 ? "+" : ""}${formatCompactNumber(requestDelta)}` })
            : platformT("platformExtract.overview.labels.activeTenantCount", { count: activeTenantSample })}
          href="/platform/usage"
          icon={<ActivityIcon className="size-3.5" />}
          label={platformT("platformExtract.overview.labels.todayPlatformActivity")}
          loading={usageTrendLoading && !usageTrend}
          tone={todayRequests > 0 ? "blue" : "slate"}
          value={usageTrend ? formatCompactNumber(todayRequests) : "-"}
        />
        <ExecutiveMetric
          detail={platformT("platformExtract.overview.labels.activeTenantDetail", {
            active: formatNumber(activeTenants),
            trial: formatNumber(trialTenants),
          })}
          href="/platform/tenants"
          icon={<Building2Icon className="size-3.5" />}
          label={platformT("platformExtract.overview.labels.tenantStructure")}
          loading={metricsLoading && !metrics}
          tone={frozenTenants > 0 ? "amber" : "success"}
          value={metrics ? formatNumber(tenantTotal) : "-"}
        />
        <ExecutiveMetric
          detail={ops?.database.status === "healthy" ? platformT("platformExtract.models.status.valid") : platformT("platformExtract.overview.labels.noOpsStatus")}
          href="/platform/ops"
          icon={<ShieldCheckIcon className="size-3.5" />}
          label={platformT("platformExtract.overview.labels.opsHealth")}
          loading={opsLoading && !ops}
          tone={ops ? opsTone : "slate"}
          value={ops ? `${opsScore}%` : "-"}
        />
        <ExecutiveMetric
          detail={platformT("platformExtract.overview.labels.ticketKnowledgeDetail", {
            knowledge: formatNumber(metrics?.knowledge.pending ?? 0),
            tickets: formatNumber(metrics?.tickets.open ?? 0),
          })}
          href="/platform/ops"
          icon={<CircleAlertIcon className="size-3.5" />}
          label={platformT("platformExtract.overview.labels.processingItems")}
          loading={(metricsLoading && !metrics) || (opsLoading && !ops)}
          tone={pendingWorkload > 0 ? "amber" : "success"}
          value={formatNumber(pendingWorkload)}
        />
      </div>

      <div className="rhd-railops-platform-exec-panel-grid">
        <OverviewPanel title={platformT("platformExtract.overview.labels.tenantStructure")} action={<PanelLink href="/platform/tenants">{platformT("platformExtract.overview.labels.tenantManagement")}</PanelLink>} loading={metricsLoading && Boolean(metrics)} loadingLabel={platformT("platformExtract.overview.labels.tenantScaleRefresh")}>
          {metricsLoading && !metrics ? (
            <PanelLoading variant="metrics" label={platformT("platformExtract.overview.labels.tenantScaleLoading")} />
          ) : metrics ? (
            <div className="rhd-railops-platform-lifecycle">
              <div className="rhd-railops-platform-lifecycle-ring" style={{
                "--rhd-platform-tenant-rate": `${Math.min(100, Math.max(0, tenantActiveRate))}%`,
              } as CSSProperties}>
                <span>
                  <strong>{formatPercent(tenantActiveRate)}</strong>
                  <small>{platformT("platformExtract.models.status.enabled")}</small>
                </span>
              </div>
              <div className="rhd-railops-platform-lifecycle-list">
                <LifecycleRow label={platformT("platformExtract.models.status.enabled")} value={activeTenants} total={tenantTotal} tone="success" />
                <LifecycleRow label={platformT("platformExtract.tenants.trialing")} value={trialTenants} total={tenantTotal} tone="blue" />
                <LifecycleRow label={platformT("platformExtract.overview.labels.within30Days")} value={expiringTenants} total={tenantTotal} tone={expiringTenants > 0 ? "amber" : "slate"} />
                <LifecycleRow label={platformT("platformExtract.tenants.frozen")} value={frozenTenants} total={tenantTotal} tone={frozenTenants > 0 ? "red" : "slate"} />
              </div>
            </div>
          ) : (
            <UnavailableBlock>{platformT("platformExtract.overview.labels.noTenantStructure")}</UnavailableBlock>
          )}
        </OverviewPanel>

        <OverviewPanel title={platformT("platformExtract.overview.labels.systemOpsStatus")} action={<PanelLink href="/platform/ops">{platformT("platformExtract.overview.labels.operationsCenter")}</PanelLink>} loading={opsLoading && Boolean(ops)} loadingLabel={platformT("platformExtract.overview.labels.opsHealthRefresh")}>
          {opsLoading && !ops ? (
            <PanelLoading variant="ops" label={platformT("platformExtract.overview.labels.opsHealthLoading")} />
          ) : ops ? (
            <div className="rhd-railops-platform-ops-snapshot">
              <div className="rhd-railops-platform-ops-ring" style={{
                "--rhd-platform-ops-score": `${opsScore}%`,
                "--rhd-platform-ops-tone": opsTone === "success" ? "var(--railops-success)" : opsTone === "amber" ? "var(--railops-warning)" : "var(--railops-error)",
              } as CSSProperties}>
                <span>
                  <strong>{opsScore}</strong>
                  <small>{platformT("platformExtract.overview.labels.healthScore")}</small>
                </span>
              </div>
              <div className="rhd-railops-platform-ops-snapshot-list">
                <MetricCell label={platformT("platformExtract.overview.labels.databaseLatency")} value={`${formatNumber(ops.database.latencyMs)}ms`} tone={ops.database.status === "healthy" ? "success" : "destructive"} />
                <MetricCell label={platformT("platformExtract.overview.labels.abnormalAccess")} value={formatNumber(ops.workloads.unhealthyConnectors)} tone={ops.workloads.unhealthyConnectors > 0 ? "amber" : "success"} />
                <MetricCell label={platformT("platformExtract.overview.labels.syncFailed")} value={formatNumber(ops.pipeline.failedSyncs24h)} tone={ops.pipeline.failedSyncs24h > 0 ? "amber" : "success"} />
                <MetricCell label={platformT("platformExtract.overview.labels.activeMeetings")} value={formatNumber(ops.workloads.activeMeetings)} tone="blue" />
              </div>
            </div>
          ) : (
            <UnavailableBlock>{platformT("platformExtract.overview.labels.noOpsStatus")}</UnavailableBlock>
          )}
        </OverviewPanel>

        <OverviewPanel title={platformT("platformExtract.overview.labels.todayPlatformActivity")} action={<PanelLink href="/platform/usage">{platformT("platformExtract.overview.labels.usageTrend")}</PanelLink>} loading={(usageTrendLoading && Boolean(usageTrend)) || (tenantScaleLoading && Boolean(tenantScale))} loadingLabel={platformT("platformExtract.overview.labels.usageRefresh")}>
          {usageTrendLoading && !usageTrend ? (
            <PanelLoading variant="chart" label={platformT("platformExtract.overview.labels.usageLoading")} />
          ) : usageTrend?.dailyUsage.length ? (
            <div className="rhd-railops-platform-activity-snapshot">
              <div className="rhd-railops-platform-activity-bars" aria-hidden="true">
                {usageTrend.dailyUsage.map((item) => (
                  <span key={item.date}>
                    <b style={{ height: `${Math.max(4, (item.aiRequests / trendMax) * 100)}%` }} />
                    <small>{item.date.slice(5)}</small>
                  </span>
                ))}
              </div>
              <div className="rhd-railops-platform-activity-list">
                {topActiveTenants.length ? topActiveTenants.map((tenant) => (
                  <Link key={tenant.tenantId} href="/platform/tenants" className="rhd-railops-platform-activity-row no-underline">
                    <span className="truncate">{tenant.tenantName}</span>
                    <strong>{formatCompactNumber(tenant.aiRequests)}</strong>
                  </Link>
                )) : (
                  <div className="rhd-railops-platform-activity-row">
                    <span>{platformT("platformExtract.overview.labels.noPlatformActivity")}</span>
                    <strong>0</strong>
                  </div>
                )}
              </div>
            </div>
          ) : (
            <UnavailableBlock>{platformT("platformExtract.overview.labels.noPlatformActivity")}</UnavailableBlock>
          )}
        </OverviewPanel>
      </div>

      <div className="rhd-railops-platform-exec-bottom-grid">
        <OverviewPanel title={platformT("platformExtract.overview.labels.recentPlatformEvents")} action={<PanelLink href="/platform/audit">{platformT("platformExtract.overview.labels.auditLog")}</PanelLink>} loading={eventsLoading && Boolean(events)} loadingLabel={platformT("platformExtract.overview.labels.eventRefresh")}>
          {eventsLoading && !events ? (
            <PanelLoading label={platformT("platformExtract.overview.labels.eventLoading")} />
          ) : recentEvents.length ? (
            <div className="divide-y divide-border">
              {recentEvents.map((event) => (
                <div key={event.id} className="rhd-railops-platform-event-row">
                  <span className="mt-1.5 size-2 shrink-0 rounded-full bg-primary" />
                  <div className="min-w-0">
                    <strong>{getAuditActionLabel(event.action)}</strong>
                    <small>{event.tenantName || platformT("platformExtract.overview.labels.currentPlatform")} · {formatDateTime(event.occurredAt)}</small>
                  </div>
                  <StatusTag tone="neutral">{platformT("platformExtract.overview.labels.statusRecorded")}</StatusTag>
                </div>
              ))}
            </div>
          ) : (
            <UnavailableBlock>{platformT("platformExtract.overview.labels.noPlatformEvents")}</UnavailableBlock>
          )}
        </OverviewPanel>
        <OverviewPanel title={platformT("platformExtract.overview.labels.resourceBaseline")} action={<PanelLink href="/platform/models">{platformT("platformExtract.overview.labels.modelAndData")}</PanelLink>} loading={metricsLoading && Boolean(metrics)} loadingLabel={platformT("platformExtract.overview.labels.resourceRefresh")}>
          {metricsLoading && !metrics ? (
            <PanelLoading variant="metrics" label={platformT("platformExtract.overview.labels.resourceLoading")} />
          ) : metrics ? (
            <div className="grid grid-cols-2 gap-2.5 p-3 sm:grid-cols-4">
              <MetricCell label={platformT("platformExtract.overview.labels.product")} value={formatNumber(metrics.products)} tone="blue" />
              <MetricCell label={platformT("platformExtract.overview.labels.device")} value={formatNumber(metrics.devices)} tone="success" />
              <MetricCell label={platformT("platformExtract.overview.labels.meeting")} value={formatNumber(metrics.meetings.active)} tone="blue" />
              <MetricCell label={platformT("platformExtract.overview.labels.modelToken")} value={formatCompactNumber(metrics.aiUsage.tokens)} tone="slate" />
            </div>
          ) : (
            <UnavailableBlock>{platformT("platformExtract.overview.labels.noResourceBaseline")}</UnavailableBlock>
          )}
        </OverviewPanel>
      </div>
    </section>
  )
}

function EventPanel({ data, error, initialLoading, loading, onRetry }: OverviewDataPanelProps) {
  if (initialLoading) {
    return (
      <OverviewPanel title={platformT("platformExtract.overview.labels.latestOperationLog")} action={<PanelLink href="/platform/audit">{platformT("platformExtract.overview.labels.allLogs")}</PanelLink>}>
        <PanelLoading label={platformT("platformExtract.overview.labels.eventLoading")} />
      </OverviewPanel>
    )
  }

  if (!data) {
    return (
      <OverviewPanel title={platformT("platformExtract.overview.labels.latestOperationLog")} action={<PanelLink href="/platform/audit">{platformT("platformExtract.overview.labels.allLogs")}</PanelLink>}>
        {error ? <PanelError message={error} onRetry={onRetry} /> : <UnavailableBlock className="min-h-[292px]">{platformT("platformExtract.overview.labels.opsDataUnavailable")}</UnavailableBlock>}
      </OverviewPanel>
    )
  }

  const events = data.recentEvents
  const unavailable = overviewSectionUnavailable(data, "recentEvents")

  return (
    <OverviewPanel
      title={platformT("platformExtract.overview.labels.latestOperationLog")}
      action={<PanelLink href="/platform/audit">{platformT("platformExtract.overview.labels.allLogs")}</PanelLink>}
      loading={loading}
      loadingLabel={platformT("platformExtract.overview.labels.eventRefresh")}
    >
      {unavailable ? (
        <UnavailableBlock className="min-h-[292px]">{platformT("platformExtract.overview.labels.opsDataUnavailable")}</UnavailableBlock>
      ) : events.length ? (
        <div className="divide-y divide-border">
          {events.slice(0, 6).map((event) => (
              <div key={event.id} className="grid min-w-0 gap-2 px-4 py-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
                <div className="flex min-w-0 items-start gap-3">
                  <span className="mt-1.5 size-2 shrink-0 rounded-full bg-muted-foreground ring-4 ring-background" />
                  <div className="min-w-0">
                    <div className="truncate text-xs font-medium text-foreground">{getAuditActionLabel(event.action)}</div>
                    <div className="mt-1 truncate text-rhd-2xs text-muted-foreground">{event.tenantName || platformT("platformExtract.overview.labels.currentPlatform")} · {event.targetType || platformT("platformExtract.overview.labels.systemTarget")}</div>
                  </div>
                </div>
                <div className="flex items-center justify-between gap-3 pl-5 sm:justify-end sm:pl-0">
                  <StatusTag tone="neutral">{platformT("platformExtract.overview.labels.statusRecorded")}</StatusTag>
                  <time className="w-[74px] text-right text-rhd-2xs tabular-nums text-muted-foreground">{formatDateTime(event.occurredAt)}</time>
                </div>
              </div>
          ))}
        </div>
      ) : (
        <UnavailableBlock className="min-h-[292px]">{platformT("platformExtract.overview.labels.noPlatformEvents")}</UnavailableBlock>
      )}
    </OverviewPanel>
  )
}

export default function PlatformOverviewPage() {
  const t = useI18n()
  platformT = t
  const [activeSection, setActiveSection] = useState<PlatformOverviewSection>("overview")
  const [metrics, setMetrics] = useState<PlatformOverview | null>(null)
  const [tenantScale, setTenantScale] = useState<PlatformOverview | null>(null)
  const [usageTrend, setUsageTrend] = useState<PlatformOverview | null>(null)
  const [events, setEvents] = useState<PlatformOverview | null>(null)
  const [ops, setOps] = useState<PlatformOpsOverview | null>(null)
  const [metricsLoading, setMetricsLoading] = useState(true)
  const [tenantScaleLoading, setTenantScaleLoading] = useState(true)
  const [usageTrendLoading, setUsageTrendLoading] = useState(true)
  const [eventsLoading, setEventsLoading] = useState(true)
  const [opsLoading, setOpsLoading] = useState(true)
  const [metricsLoaded, setMetricsLoaded] = useState(false)
  const [tenantScaleLoaded, setTenantScaleLoaded] = useState(false)
  const [usageTrendLoaded, setUsageTrendLoaded] = useState(false)
  const [eventsLoaded, setEventsLoaded] = useState(false)
  const [opsLoaded, setOpsLoaded] = useState(false)
  const [metricsError, setMetricsError] = useState("")
  const [tenantScaleError, setTenantScaleError] = useState("")
  const [usageTrendError, setUsageTrendError] = useState("")
  const [eventsError, setEventsError] = useState("")
  const [opsError, setOpsError] = useState("")

  const loadMetrics = useCallback(async () => {
    setMetricsLoading(true)
    setMetricsError("")
    try {
      setMetrics(await fetchPlatformOverviewMetrics())
      setMetricsLoaded(true)
    } catch (err) {
      setMetricsError(errorMessage(err, platformT("platformExtract.overview.labels.criticalMetricsUnavailable")))
    } finally {
      setMetricsLoading(false)
    }
  }, [t])

  const loadTenantScale = useCallback(async () => {
    setTenantScaleLoading(true)
    setTenantScaleError("")
    try {
      setTenantScale(await fetchPlatformOverviewTopTenants(5))
      setTenantScaleLoaded(true)
    } catch (err) {
      setTenantScaleError(errorMessage(err, platformT("platformExtract.overview.labels.tenantDataUnavailable")))
    } finally {
      setTenantScaleLoading(false)
    }
  }, [t])

  const loadUsageTrend = useCallback(async () => {
    setUsageTrendLoading(true)
    setUsageTrendError("")
    try {
      setUsageTrend(await fetchPlatformOverviewUsageTrend(7))
      setUsageTrendLoaded(true)
    } catch (err) {
      setUsageTrendError(errorMessage(err, platformT("platformExtract.overview.labels.usageDataUnavailable")))
    } finally {
      setUsageTrendLoading(false)
    }
  }, [t])

  const loadEvents = useCallback(async () => {
    setEventsLoading(true)
    setEventsError("")
    try {
      setEvents(await fetchPlatformOverviewEvents(8))
      setEventsLoaded(true)
    } catch (err) {
      setEventsError(errorMessage(err, platformT("platformExtract.overview.labels.opsDataUnavailable")))
    } finally {
      setEventsLoading(false)
    }
  }, [t])

  const loadOps = useCallback(async () => {
    setOpsLoading(true)
    setOpsError("")
    try {
      const overview = await fetchPlatformOpsOverview()
      setOps(overview)
      setOpsLoaded(true)
    } catch (err) {
      setOpsError(errorMessage(err, platformT("platformExtract.overview.labels.noOpsStatus")))
    } finally {
      setOpsLoading(false)
    }
  }, [t])

  const load = useCallback(async () => {
    await Promise.allSettled([
      loadMetrics(),
      loadTenantScale(),
      loadUsageTrend(),
      loadEvents(),
      loadOps(),
    ])
  }, [loadEvents, loadMetrics, loadOps, loadTenantScale, loadUsageTrend])

  const loading = metricsLoading || tenantScaleLoading || usageTrendLoading || eventsLoading || opsLoading
  const generatedAt = useMemo(() => {
    const timestamps = [
      metrics?.generatedAt,
      tenantScale?.generatedAt,
      usageTrend?.generatedAt,
      events?.generatedAt,
      ops?.generatedAt,
    ].filter((value): value is string => Boolean(value))
    return timestamps.sort().slice(-1)[0] ?? ""
  }, [events, metrics, ops, tenantScale, usageTrend])
  const partialWarning = [
    metrics?.unavailableSections.length ? platformT("platformExtract.overview.labels.missingSummary", {
      prefix: platformT("platformExtract.overview.labels.missingPrefix"),
      sections: formatUnavailableSections(t, metrics.unavailableSections),
    }) : "",
    tenantScale?.unavailableSections.length ? platformT("platformExtract.overview.labels.namedMissingSummary", {
      name: platformT("platformExtract.overview.labels.tenantScale"),
      prefix: platformT("platformExtract.overview.labels.missingPrefix"),
      sections: formatUnavailableSections(t, tenantScale.unavailableSections),
    }) : "",
    usageTrend?.unavailableSections.length ? platformT("platformExtract.overview.labels.namedMissingSummary", {
      name: platformT("platformExtract.overview.labels.usageTrend"),
      prefix: platformT("platformExtract.overview.labels.missingPrefix"),
      sections: formatUnavailableSections(t, usageTrend.unavailableSections),
    }) : "",
    events?.unavailableSections.length ? platformT("platformExtract.overview.labels.namedMissingSummary", {
      name: platformT("platformExtract.overview.labels.opsLog"),
      prefix: platformT("platformExtract.overview.labels.missingPrefix"),
      sections: formatUnavailableSections(t, events.unavailableSections),
    }) : "",
    metricsError && metrics ? metricsError : "",
    tenantScaleError && tenantScale ? tenantScaleError : "",
    usageTrendError && usageTrend ? usageTrendError : "",
    eventsError && events ? eventsError : "",
    opsError && ops ? platformT("platformExtract.overview.labels.opsStatusNotSynced") : "",
  ].filter(Boolean).join(" ")

  useEffect(() => {
    const frame = window.requestAnimationFrame(() => {
      void load()
    })
    return () => window.cancelAnimationFrame(frame)
  }, [load])

  const kpis = useMemo(() => {
    if (!metrics) return []
    const ticketUnavailable = overviewSectionUnavailable(metrics, "tickets")
    const knowledgeUnavailable = overviewSectionUnavailable(metrics, "knowledge")
    return [
      { label: t("platformExtract.overview.labels.pendingTickets"), value: ticketUnavailable ? "-" : formatNumber(metrics.tickets.open), detail: ticketUnavailable ? t("platformExtract.overview.labels.ticketDataUnavailable") : t("platformExtract.overview.labels.todayAddedDetail", { count: formatNumber(metrics.tickets.createdToday) }), icon: <TicketCheckIcon className="size-3.5" />, tone: ticketUnavailable ? "slate" as const : metrics.tickets.open > 0 ? "amber" as const : "blue" as const },
      { label: t("platformExtract.overview.labels.slaRisk"), value: ticketUnavailable ? "-" : formatNumber(metrics.tickets.slaRisk), detail: ticketUnavailable ? t("platformExtract.overview.labels.ticketDataUnavailable") : metrics.tickets.slaRisk > 0 ? t("platformExtract.overview.labels.followUpRequired") : t("platformExtract.overview.labels.noRisk"), icon: <Clock3Icon className="size-3.5" />, tone: ticketUnavailable ? "slate" as const : metrics.tickets.slaRisk > 0 ? "amber" as const : "blue" as const },
      { label: t("platformExtract.overview.labels.knowledgePending"), value: knowledgeUnavailable ? "-" : formatNumber(metrics.knowledge.pending), detail: knowledgeUnavailable ? t("platformExtract.overview.labels.knowledgeDataUnavailable") : metrics.knowledge.pending > 0 ? t("platformExtract.overview.labels.pendingReviewOrIndex") : t("platformExtract.overview.labels.queueEmpty"), icon: <GaugeIcon className="size-3.5" />, tone: knowledgeUnavailable ? "slate" as const : metrics.knowledge.pending > 0 ? "amber" as const : "blue" as const },
      { label: t("platformExtract.overview.labels.abnormalAccess"), value: formatNumber(ops?.workloads.unhealthyConnectors ?? 0), detail: ops ? t("platformExtract.overview.labels.connectorStatus") : t("platformExtract.overview.labels.noOpsStatus"), icon: <CircleAlertIcon className="size-3.5" />, tone: ops?.workloads.unhealthyConnectors ? "amber" as const : "slate" as const, loading: opsLoading && !opsLoaded },
    ]
  }, [metrics, ops, opsLoaded, opsLoading, t])

  const overviewTabs = useMemo(() => {
    const serviceCount = (metrics?.tickets.open ?? 0) + (metrics?.knowledge.pending ?? 0)
    const opsCount = (ops?.workloads.unhealthyConnectors ?? 0) + (events?.recentEvents.length ?? 0)
    return [
      { label: t("platformExtract.overview.labels.overviewTab"), value: "overview" },
      { label: t("platformExtract.overview.labels.tenantUsageTab"), value: "tenant", count: tenantScale?.topTenants.length ?? undefined },
      { label: t("platformExtract.overview.labels.serviceQualityTab"), value: "service", count: metrics ? serviceCount : undefined },
      { label: t("platformExtract.overview.labels.opsLogsTab"), value: "ops", count: ops || events ? opsCount : undefined },
    ]
  }, [events, metrics, ops, tenantScale, t])

  return (
    <PageShell
      title={t("platformExtract.overview.labels.platformOverview")}
      breadcrumb={useRouteBreadcrumbItems()}
      className="rhd-railops-platform-overview-page min-h-[calc(100vh-89px)] pb-2"
      actions={
        <div className="flex flex-wrap items-center gap-2 sm:justify-end">
          {ops?.database.status === "healthy" ? (
            <StatusTag tone="success">
              <CheckCircle2Icon className="size-3.5" />
              {t("platformExtract.models.status.valid")}
            </StatusTag>
          ) : null}
          {generatedAt ? <span className="text-rhd-xs text-muted-foreground">{t("platformExtract.overview.labels.generatedAt")} {formatDateTime(generatedAt)}</span> : null}
          <RailopsButton size="small" onClick={() => void load()} disabled={loading}>
            <RefreshCwIcon className={loading ? "animate-spin" : ""} data-icon="inline-start" />
            {t("platformExtract.common.refresh")}
          </RailopsButton>
        </div>
      }
    >

      <div className="space-y-3">
        <div className="rhd-railops-platform-switchbar">
          <div className="rhd-railops-dashboard-tabs rhd-railops-platform-tabs">
            <FilterTabs
              ariaLabel={t("platformExtract.overview.labels.platformOverview")}
              items={overviewTabs}
              value={activeSection}
              onChange={(value) => setActiveSection(value as PlatformOverviewSection)}
            />
          </div>
          {partialWarning ? (
            <div className="rhd-railops-platform-attention-slot">
              <div className="rhd-railops-platform-overview-warning">
                <StatusTag tone="warning">{t("platformExtract.overview.labels.partialMissing")}</StatusTag>
                <span>{partialWarning}</span>
              </div>
            </div>
          ) : null}
        </div>

        {activeSection !== "overview" ? (
          metricsLoading && !metricsLoaded ? (
            <KpiLoadingGrid />
          ) : metricsError && metrics === null ? (
            <KpiErrorGrid message={metricsError} onRetry={() => void loadMetrics()} />
          ) : (
            <section
              className="rhd-railops-platform-kpi-grid"
              aria-label={t("platformExtract.overview.labels.processingItems")}
              aria-busy={metricsLoading || (opsLoading && !opsLoaded) || undefined}
            >
              {kpis.map((item) => <KpiCard key={item.label} {...item} />)}
            </section>
          )
        ) : null}

        <section className={cn("rhd-railops-platform-tab-panel", activeSection === "overview" && "is-overview")} role="tabpanel" aria-label={overviewTabs.find((item) => item.value === activeSection)?.label}>
          {activeSection === "overview" ? (
            <PlatformExecutiveOverview
              t={t}
              events={events}
              eventsLoading={eventsLoading && !eventsLoaded}
              metrics={metrics}
              metricsLoading={metricsLoading && !metricsLoaded}
              ops={ops}
              opsLoading={opsLoading && !opsLoaded}
              tenantScale={tenantScale}
              tenantScaleLoading={tenantScaleLoading && !tenantScaleLoaded}
              usageTrend={usageTrend}
              usageTrendLoading={usageTrendLoading && !usageTrendLoaded}
            />
          ) : null}
          {activeSection === "tenant" ? (
            <>
              <TenantScalePanel data={tenantScale} error={tenantScaleError} initialLoading={tenantScaleLoading && !tenantScaleLoaded} loading={tenantScaleLoading} onRetry={() => void loadTenantScale()} />
              <UsageTrendPanel data={usageTrend} error={usageTrendError} initialLoading={usageTrendLoading && !usageTrendLoaded} loading={usageTrendLoading} onRetry={() => void loadUsageTrend()} />
            </>
          ) : null}
          {activeSection === "service" ? (
            <>
              <TicketPanel data={metrics} error={metricsError} initialLoading={metricsLoading && !metricsLoaded} loading={metricsLoading} onRetry={() => void loadMetrics()} />
              <KnowledgePanel data={metrics} error={metricsError} initialLoading={metricsLoading && !metricsLoaded} loading={metricsLoading} onRetry={() => void loadMetrics()} />
            </>
          ) : null}
          {activeSection === "ops" ? (
            <>
              <OpsHealthPanel ops={ops} error={opsError} initialLoading={opsLoading && !opsLoaded} loading={opsLoading} onRetry={() => void loadOps()} />
              <EventPanel data={events} error={eventsError} initialLoading={eventsLoading && !eventsLoaded} loading={eventsLoading} onRetry={() => void loadEvents()} />
            </>
          ) : null}
        </section>

        <footer className="flex flex-col gap-1 border-t border-border pt-3 text-rhd-2xs text-muted-foreground sm:flex-row sm:items-center sm:justify-between">
          <span>{t("platformExtract.overview.labels.dataUpdatedAt", { time: generatedAt ? formatDateTime(generatedAt) : "-" })}</span>
          <span>{loading ? t("platformExtract.overview.labels.syncing") : t("platformExtract.overview.labels.synchronized")}</span>
        </footer>
      </div>
    </PageShell>
  )
}
