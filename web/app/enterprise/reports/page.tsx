"use client"

import { translateCurrentMessage } from "@/i18n/messages"
import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react"
import {
  AlertCircleIcon,
  DownloadIcon,
  RefreshCwIcon,
  TriangleAlertIcon,
  TrophyIcon,
} from "lucide-react"
import type { TableColumnsType } from "antd"
import {
  ContentModule,
  DataTable,
  FilterTabs,
  IconButton,
  PageShell,
  RailopsButton,
  SearchField,
  StatCard,
  StatGrid,
  StatusTag,
  type StatCardTone,
  type StatusTagTone,
} from "@railops/ui"

import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import { ModuleLoading } from "@/components/shared/loading-states"
import {
  fetchReportTeamPerformance,
  fetchReportSupplierPerformance,
  fetchReportTopFailures,
  fetchReportsOverview,
  type AgentPerformance,
  type DashboardOverview,
  type ProductFailureRank,
  type SupplierPerformance,
} from "@/lib/api/enterprise-reports"
import { fetchTickets, type TicketListResponse } from "@/lib/api/enterprise-tickets"
import type { TicketListItem } from "@/lib/api/types"
import { cn } from "@/lib/utils"

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

type ReportRole = "admin" | "engineer" | "supplier"
type ReportSectionTab = "overview" | "insights" | "details"
type TicketFilter = "all" | "processing" | "done"
type Tone = "good" | "info" | "warn" | "bad" | "slate"

type DateRangeKey = "current_month" | "last_30_days" | "last_month"
type CsvCell = boolean | number | string | null | undefined
type CsvRow = CsvCell[]

type ReportState = {
  overview: DashboardOverview | null
  topFailures: ProductFailureRank[]
  teamPerformance: AgentPerformance[]
  supplierPerformance: SupplierPerformance[]
  tickets: TicketListResponse | null
}

type MetricItem = {
  label: string
  meta: string
  tone?: Tone
  value: string
}

type ProcurementMetricItem = {
  label: string
  value: string
}

const roleTabs: Array<{ key: ReportRole; labelKey: string }> = [
  { key: "admin", labelKey: "reports.roleTabs.admin" },
  { key: "engineer", labelKey: "reports.roleTabs.engineer" },
  { key: "supplier", labelKey: "reports.roleTabs.supplier" },
]

const sectionTabs: Array<{ key: ReportSectionTab; labelKey: string }> = [
  { key: "overview", labelKey: "reports.sectionTabs.overview" },
  { key: "insights", labelKey: "reports.sectionTabs.insights" },
  { key: "details", labelKey: "reports.sectionTabs.details" },
]

const tableFilterTabs: Array<{ key: TicketFilter; labelKey: string }> = [
  { key: "all", labelKey: "reports.tableFilterTabs.all" },
  { key: "processing", labelKey: "reports.tableFilterTabs.processing" },
  { key: "done", labelKey: "reports.tableFilterTabs.done" },
]

const periodTabs: Array<{ key: DateRangeKey; labelKey: string }> = [
  { key: "current_month", labelKey: "reports.periodTabs.currentMonth" },
  { key: "last_30_days", labelKey: "reports.periodTabs.last30Days" },
  { key: "last_month", labelKey: "reports.periodTabs.lastMonth" },
]

const toneStatusTag: Record<Tone, StatusTagTone> = {
  bad: "error",
  good: "success",
  info: "blue",
  slate: "neutral",
  warn: "warning",
}

const toneStatTone: Record<Tone, StatCardTone> = {
  bad: "error",
  good: "success",
  info: "blue",
  slate: "neutral",
  warn: "warning",
}

function pad(value: number) {
  return String(value).padStart(2, "0")
}

function formatDate(date: Date) {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

function getDateRange(key: DateRangeKey) {
  const now = new Date()
  const start = new Date(now)
  const end = new Date(now)

  if (key === "last_30_days") {
    start.setDate(now.getDate() - 30)
  } else if (key === "last_month") {
    start.setMonth(now.getMonth() - 1, 1)
    end.setDate(0)
  } else {
    start.setDate(1)
  }

  return {
    end: formatDate(end),
    label: ee("reports.common.rangeLabel", { value0: formatDate(start), value1: formatDate(end) }),
    start: formatDate(start),
  }
}

function getRoleLabel(role: ReportRole) {
  return ee(roleTabs.find((tab) => tab.key === role)?.labelKey || "")
}

function getPeriodLabel(periodKey: DateRangeKey) {
  return ee(periodTabs.find((tab) => tab.key === periodKey)?.labelKey || "")
}

function getTableFilterLabel(filter: TicketFilter) {
  return ee(tableFilterTabs.find((tab) => tab.key === filter)?.labelKey || "")
}

function formatInteger(value?: number | null) {
  return typeof value === "number" ? value.toLocaleString() : "—"
}

function formatPercent(value?: number | null, digits = 1) {
  return typeof value === "number" ? `${value.toFixed(digits)}%` : "—"
}

function formatMinutes(value?: number | null) {
  if (typeof value !== "number" || Number.isNaN(value)) return "—"
  if (value >= 60) return ee("reports.common.hours", { value0: (value / 60).toFixed(1) })
  return ee("reports.common.minutes", { value0: value.toFixed(1) })
}

function formatProcurementPercent(value?: number | null) {
  return typeof value === "number" && Number.isFinite(value)
    ? `${value.toFixed(1)}%`
    : ee("reports.common.noData")
}

function formatProcurementCount(value?: number | null) {
  return typeof value === "number" && Number.isFinite(value)
    ? ee("reports.common.countWithUnit", { value0: value.toLocaleString(), value1: ee("reports.common.timesUnit") })
    : ee("reports.common.noData")
}

function formatProcurementMinutes(value?: number | null) {
  if (typeof value !== "number" || !Number.isFinite(value)) return ee("reports.common.noData")
  if (value >= 60) return ee("reports.common.hours", { value0: (value / 60).toFixed(1) })
  return ee("reports.common.minutes", { value0: value.toFixed(1) })
}

function mean(values: number[]) {
  const realValues = values.filter((value) => Number.isFinite(value) && value > 0)
  if (!realValues.length) return null
  return realValues.reduce((sum, value) => sum + value, 0) / realValues.length
}

function getTicketStatusQuery(filter: TicketFilter) {
  if (filter === "processing") return "in_progress"
  if (filter === "done") return "done"
  return undefined
}

function ticketStatusLabel(status: string) {
  const map: Record<string, string> = {
    cancelled: ee("reports.ticketStatus.cancelled"),
    closed: ee("reports.ticketStatus.closed"),
    done: ee("reports.ticketStatus.done"),
    in_progress: ee("reports.ticketStatus.inProgress"),
    pending_acceptance: ee("reports.ticketStatus.pendingAcceptance"),
    pending_dispatch: ee("reports.ticketStatus.pendingDispatch"),
    processing: ee("reports.ticketStatus.processing"),
    resolved: ee("reports.ticketStatus.resolved"),
  }
  return map[status] || status
}

function ticketStatusTone(status: string): Tone {
  if (["done", "closed", "resolved"].includes(status)) return "good"
  if (["pending_acceptance", "pending_dispatch"].includes(status)) return "warn"
  if (status === "cancelled") return "slate"
  return "info"
}

function ToneBadge({ children, tone = "slate" }: { children: ReactNode; tone?: Tone }) {
  return (
    <StatusTag tone={toneStatusTag[tone]}>
      {children}
    </StatusTag>
  )
}

function ReportsLoadingContent({ role }: { role: ReportRole }) {
  return (
    <div className="grid gap-4">
      <ModuleLoading
        label={ee("reports.loading.metrics")}
        variant="metrics"
        count={role === "supplier" ? 3 : 4}
      />
      {role === "admin" ? (
        <ModuleLoading label={ee("reports.loading.procurementMetrics")} variant="metrics" count={6} />
      ) : null}
      <div className="grid gap-4 xl:grid-cols-2">
        <ModuleLoading label={ee("reports.loading.issueInsights")} variant="list" count={3} />
        <ModuleLoading label={ee("reports.loading.performanceInsights")} variant="list" count={3} />
      </div>
      <ModuleLoading label={ee("reports.loading.details")} variant="table" count={5} />
    </div>
  )
}

function EmptyBlock({ message }: { message: string }) {
  return (
    <div className="rounded-lg border border-dashed border-slate-200 bg-slate-50 p-6 text-center text-sm text-slate-500">
      {message}
    </div>
  )
}

function ErrorBlock({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="rounded-lg border border-rose-200 bg-rose-50 p-6 text-center">
      <AlertCircleIcon className="mx-auto size-8 text-rose-600" />
      <p className="mt-2 text-sm font-medium text-rose-700">{message}</p>
      <RailopsButton size="small" className="mt-4" onClick={onRetry}>
        <RefreshCwIcon className="size-4" />
        {ee("reports.actions.retry")}
      </RailopsButton>
    </div>
  )
}

function buildMetrics(role: ReportRole, data: ReportState | null): MetricItem[] {
  const overview = data?.overview
  const team = data?.teamPerformance ?? []
  const assigned = team.reduce((sum, row) => sum + row.tickets_assigned, 0)
  const resolved = team.reduce((sum, row) => sum + row.tickets_resolved, 0)
  const avgResponse = mean(team.map((row) => row.avg_response_minutes))
  const avgHandle = mean(team.map((row) => row.avg_handle_minutes))

  if (role === "engineer") {
    return [
      { label: ee("reports.engineerMetrics.assignedTickets"), value: formatInteger(assigned), meta: ee("reports.engineerMetrics.currentPeriod"), tone: "info" },
      { label: ee("reports.engineerMetrics.resolvedTickets"), value: formatInteger(resolved), meta: ee("reports.engineerMetrics.currentPeriod"), tone: "good" },
      { label: ee("reports.engineerMetrics.avgFirstResponse"), value: formatMinutes(avgResponse), meta: ee("reports.engineerMetrics.responseDerivedFromProgress"), tone: "info" },
      { label: ee("reports.engineerMetrics.avgHandleTime"), value: formatMinutes(avgHandle), meta: ee("reports.engineerMetrics.resolvedTicketScope"), tone: "slate" },
    ]
  }

  if (role === "supplier") {
    const suppliers = data?.supplierPerformance ?? []
    const assigned = suppliers.reduce((sum, row) => sum + row.tickets_assigned, 0)
    const processing = suppliers.reduce((sum, row) => sum + row.tickets_processing, 0)
    const completed = suppliers.reduce((sum, row) => sum + row.tickets_completed, 0)
    const responded = suppliers.reduce((sum, row) => sum + row.tickets_responded, 0)
    const ratingCount = suppliers.reduce((sum, row) => sum + row.rating_count, 0)
    const avgResponse = responded > 0
      ? suppliers.reduce((sum, row) => sum + row.avg_response_minutes * row.tickets_responded, 0) / responded
      : null
    const completionRate = assigned > 0 ? completed / assigned * 100 : null
    const averageRating = ratingCount > 0
      ? suppliers.reduce((sum, row) => sum + row.average_rating * row.rating_count, 0) / ratingCount
      : null
    return [
      { label: ee("reports.supplierMetrics.tickets"), value: formatInteger(assigned), meta: ee("reports.supplierMetrics.supplierCount", { value0: formatInteger(suppliers.length) }), tone: "info" },
      { label: ee("reports.supplierMetrics.processingCount"), value: formatInteger(processing), meta: ee("reports.supplierMetrics.completedCount", { value0: formatInteger(completed) }), tone: processing > 0 ? "warn" : "good" },
      { label: ee("reports.supplierMetrics.avgFirstResponse"), value: formatMinutes(avgResponse), meta: ee("reports.supplierMetrics.respondedCount", { value0: formatInteger(responded) }), tone: "info" },
      { label: ee("reports.supplierMetrics.completionRate"), value: formatPercent(completionRate), meta: averageRating === null ? ee("reports.supplierMetrics.noCustomerRating") : ee("reports.supplierMetrics.averageRating", { value0: averageRating.toFixed(1), value1: formatInteger(ratingCount) }), tone: (completionRate ?? 0) >= 90 ? "good" : "warn" },
    ]
  }

  return [
    { label: ee("reports.adminMetrics.totalTickets"), value: formatInteger(overview?.total_tickets), meta: ee("reports.adminMetrics.todayNew", { value0: formatInteger(overview?.new_today) }), tone: "info" },
    { label: ee("reports.adminMetrics.pendingAndProcessing"), value: formatInteger((overview?.pending_tickets ?? 0) + (overview?.in_progress_tickets ?? 0)), meta: ee("reports.adminMetrics.pendingCount", { value0: formatInteger(overview?.pending_tickets) }), tone: "warn" },
    { label: ee("reports.adminMetrics.slaRisk"), value: formatInteger(overview?.sla_at_risk), meta: ee("reports.adminMetrics.currentTenantRisk"), tone: (overview?.sla_at_risk ?? 0) > 0 ? "bad" : "good" },
    { label: ee("reports.adminMetrics.aiResolveRate"), value: formatPercent(overview?.ai_resolve_rate), meta: ee("reports.adminMetrics.todayAiSessions", { value0: formatInteger(overview?.ai_sessions) }), tone: "good" },
  ]
}

function KpiGrid({ data, role }: { data: ReportState | null; role: ReportRole }) {
  return (
    <section aria-label={ee("reports.aria.kpis")}>
      <StatGrid className="rhd-railops-reports-kpis">
        {buildMetrics(role, data).map((item) => (
          <StatCard
            key={item.label}
            label={item.label}
            value={item.value}
            note={item.meta}
            tone={item.tone ? toneStatTone[item.tone] : undefined}
          />
        ))}
      </StatGrid>
    </section>
  )
}

function ProcurementDecisionMetrics({ overview }: { overview: DashboardOverview | null }) {
  const metrics: ProcurementMetricItem[] = [
    { label: ee("reports.procurement.expertInterventionRate"), value: formatProcurementPercent(overview?.expert_intervention_rate) },
    { label: ee("reports.procurement.avoidedTrips"), value: formatProcurementCount(overview?.avoided_trips) },
    { label: ee("reports.procurement.firstTimeFixRate"), value: formatProcurementPercent(overview?.first_time_fix_rate) },
    { label: ee("reports.procurement.avgDowntime"), value: formatProcurementMinutes(overview?.avg_downtime_minutes) },
    { label: ee("reports.procurement.knowledgeReuseRate"), value: formatProcurementPercent(overview?.knowledge_reuse_rate) },
    { label: ee("reports.procurement.remoteResolutionRate"), value: formatProcurementPercent(overview?.remote_resolution_rate) },
  ]

  const sampleSummary = typeof overview?.outcome_metric_sample_size === "number"
    ? ee("reports.procurement.sampleSummary", { value0: formatInteger(overview.outcome_metric_sample_size), value1: formatProcurementPercent(overview.outcome_metric_coverage_rate) })
    : ee("reports.procurement.noSample")

  return (
    <ContentModule className="rhd-railops-reports-procurement" title={ee("reports.procurement.title")} note={sampleSummary}>
      <dl className="mt-3 grid grid-cols-2 gap-x-3 gap-y-4 lg:grid-cols-3 2xl:grid-cols-6">
        {metrics.map((metric) => (
          <div
            key={metric.label}
            role="group"
            aria-label={metric.label}
            className="min-w-0 border-l border-slate-200 pl-3"
          >
            <dt className="truncate text-xs font-medium text-slate-500">{metric.label}</dt>
            <dd className="mt-1 truncate text-base font-semibold tabular-nums text-slate-950" title={metric.value}>
              {metric.value}
            </dd>
          </div>
        ))}
      </dl>
    </ContentModule>
  )
}

function InsightPanel({
  children,
  icon,
  title,
}: {
  children: ReactNode
  icon: ReactNode
  title: string
}) {
  return (
    <article className="rhd-railops-reports-insight">
      <div className="flex items-start gap-2.5 border-b border-slate-100 pb-3">
        <span className="grid size-8 shrink-0 place-items-center rounded-md border border-blue-100 bg-blue-50 text-blue-700">
          {icon}
        </span>
        <div className="min-w-0">
          <h2 className="truncate text-sm font-semibold text-slate-950">{title}</h2>
        </div>
      </div>
      <div className="mt-2 divide-y divide-slate-100">{children}</div>
    </article>
  )
}

function Insights({ data, role }: { data: ReportState | null; role: ReportRole }) {
  const topFailures = data?.topFailures ?? []
  const ranking = data?.teamPerformance.slice(0, 3) ?? []
  const supplierRanking = data?.supplierPerformance.slice(0, 3) ?? []

  return (
    <section className="rhd-railops-reports-insights" aria-label={ee("reports.aria.insights")}>
      <InsightPanel
        icon={<TriangleAlertIcon className="size-4" />}
        title={ee("reports.insights.issueDistribution")}
      >
        {topFailures.length ? topFailures.map((row) => (
          <div key={`${row.product_id}-${row.product_name}`} className="grid grid-cols-[minmax(0,1fr)_64px_auto] items-center gap-3 py-3">
            <div className="min-w-0">
              <span className="block truncate text-xs font-medium text-slate-500">
                {row.product_name || ee("reports.common.productFallback", { value0: row.product_id || ee("reports.common.unbound") })}
              </span>
              <strong className="mt-1 block truncate text-sm font-semibold text-slate-800">
                {ee("reports.insights.escalationAndDuration", { value0: formatPercent(row.escalation_rate), value1: row.avg_resolution_hours.toFixed(1) })}
              </strong>
            </div>
            <span className="text-xs font-semibold text-slate-500">{ee("reports.common.times", { value0: formatInteger(row.failure_count) })}</span>
            {row.escalation_rate >= 30 ? <ToneBadge tone="bad">{ee("reports.insights.highEscalationRate")}</ToneBadge> : <span />}
          </div>
        )) : (
          <EmptyBlock message={ee("reports.insights.noFailureRanking")} />
        )}
      </InsightPanel>

      <InsightPanel
        icon={<TrophyIcon className="size-4" />}
        title={role === "supplier" ? ee("reports.insights.supplierPerformance") : ee("reports.insights.engineerPerformance")}
      >
        {role === "supplier" ? (supplierRanking.length ? supplierRanking.map((row, index) => (
          <div key={row.supplier_id || row.supplier_name} className="grid grid-cols-[28px_minmax(0,1fr)_92px] items-center gap-3 py-3">
            <span className="grid size-6 place-items-center rounded-full bg-blue-50 text-xs font-bold text-blue-700">
              {index + 1}
            </span>
            <div className="min-w-0">
              <span className="block truncate text-xs font-medium text-slate-500">{row.supplier_name || ee("reports.common.supplierFallback", { value0: row.supplier_id })}</span>
              <strong className="mt-1 block truncate text-sm font-semibold text-slate-800">
                {ee("reports.insights.completionRateWithValue", { value0: formatPercent(row.completion_rate) })}
              </strong>
            </div>
            <span className="truncate text-xs font-medium text-slate-500">
              {ee("reports.insights.ratingValue", { value0: row.rating_count > 0 ? row.average_rating.toFixed(1) : "—" })}
            </span>
          </div>
        )) : (
          <EmptyBlock message={ee("reports.insights.noSupplierPerformance")} />
        )) : ranking.length ? ranking.map((row, index) => (
          <div key={row.agent_id || row.agent_name} className="grid grid-cols-[28px_minmax(0,1fr)_92px] items-center gap-3 py-3">
            <span className="grid size-6 place-items-center rounded-full bg-blue-50 text-xs font-bold text-blue-700">
              {index + 1}
            </span>
            <div className="min-w-0">
              <span className="block truncate text-xs font-medium text-slate-500">{row.agent_name || ee("reports.common.userFallback", { value0: row.agent_id })}</span>
              <strong className="mt-1 block truncate text-sm font-semibold text-slate-800">
                {ee("reports.insights.resolvedTickets", { value0: formatInteger(row.tickets_resolved) })}
              </strong>
            </div>
            <span className="truncate text-xs font-medium text-slate-500">
              {ee("reports.insights.firstResponse", { value0: formatMinutes(row.avg_response_minutes) })}
            </span>
          </div>
        )) : (
          <EmptyBlock message={ee("reports.insights.noEngineerPerformance")} />
        )}
      </InsightPanel>
    </section>
  )
}

function matchesQuery(text: string, query: string) {
  return text.toLowerCase().includes(query.trim().toLowerCase())
}

function filterTeamRows(data: ReportState | null, query: string) {
  const rows = data?.teamPerformance ?? []
  if (!query.trim()) return rows
  return rows.filter((row) => matchesQuery(`${row.agent_name} ${row.agent_id}`, query))
}

function filterTicketRows(data: ReportState | null, query: string) {
  const rows = data?.tickets?.items ?? []
  if (!query.trim()) return rows
  return rows.filter((row) => matchesQuery(
    `${row.ticket_no} ${row.title} ${row.customer_name} ${row.product_name} ${row.device_no} ${row.assignee_name ?? ""}`,
    query
  ))
}

function filterSupplierRows(data: ReportState | null, query: string) {
  const rows = data?.supplierPerformance ?? []
  if (!query.trim()) return rows
  return rows.filter((row) => matchesQuery(`${row.supplier_name} ${row.supplier_no} ${row.supplier_id}`, query))
}

function formatCsvCell(value: CsvCell) {
  const raw = value === null || value === undefined ? "" : String(value)
  const safe = /^[=+\-@]/.test(raw) ? `'${raw}` : raw
  return `"${safe.replace(/"/g, '""')}"`
}

function downloadCsv(filename: string, rows: CsvRow[]) {
  const csv = `\uFEFF${rows.map((row) => row.map(formatCsvCell).join(",")).join("\n")}`
  const blob = new Blob([csv], { type: "text/csv;charset=utf-8" })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement("a")
  anchor.href = url
  anchor.download = filename
  document.body.append(anchor)
  anchor.click()
  anchor.remove()
  window.setTimeout(() => URL.revokeObjectURL(url), 0)
}

function buildReportExportRows({
  data,
  filter,
  periodKey,
  query,
  rangeLabel,
  role,
}: {
  data: ReportState
  filter: TicketFilter
  periodKey: DateRangeKey
  query: string
  rangeLabel: string
  role: ReportRole
}): CsvRow[] {
  const overview = data.overview
  const rows: CsvRow[] = [
    [ee("reports.export.title")],
    [ee("reports.export.scope"), getRoleLabel(role)],
    [ee("reports.export.period"), getPeriodLabel(periodKey), rangeLabel],
    [ee("reports.export.exportTime"), new Date().toLocaleString(undefined, { hour12: false })],
    [ee("reports.export.search"), query.trim() || ee("reports.common.all")],
    [],
    [ee("reports.export.sectionMetrics")],
    [ee("reports.export.columnLabel"), ee("reports.export.columnValue"), ee("reports.export.columnNote")],
    ...buildMetrics(role, data).map((item) => [item.label, item.value, item.meta]),
  ]

  if (role === "admin") {
    rows.push(
      [],
      [ee("reports.export.procurementSection")],
      [ee("reports.export.columnLabel"), ee("reports.export.columnValue")],
      [ee("reports.procurement.expertInterventionRate"), formatProcurementPercent(overview?.expert_intervention_rate)],
      [ee("reports.procurement.avoidedTrips"), formatProcurementCount(overview?.avoided_trips)],
      [ee("reports.procurement.firstTimeFixRate"), formatProcurementPercent(overview?.first_time_fix_rate)],
      [ee("reports.procurement.avgDowntime"), formatProcurementMinutes(overview?.avg_downtime_minutes)],
      [ee("reports.procurement.knowledgeReuseRate"), formatProcurementPercent(overview?.knowledge_reuse_rate)],
      [ee("reports.procurement.remoteResolutionRate"), formatProcurementPercent(overview?.remote_resolution_rate)],
      [ee("reports.procurement.sampleSize"), formatInteger(overview?.outcome_metric_sample_size)],
      [ee("reports.procurement.coverageRate"), formatProcurementPercent(overview?.outcome_metric_coverage_rate)]
    )
  }

  rows.push(
    [],
    [ee("reports.export.issueSection")],
    [ee("reports.export.product"), ee("reports.export.failureCount"), ee("reports.export.escalationRate"), ee("reports.export.avgResolutionHours"), ee("reports.export.trend")],
    ...(data.topFailures.length
      ? data.topFailures.map((row) => [
          row.product_name || ee("reports.common.productFallback", { value0: row.product_id || ee("reports.common.unbound") }),
          row.failure_count,
          formatPercent(row.escalation_rate),
          ee("reports.common.hours", { value0: row.avg_resolution_hours.toFixed(1) }),
          row.trend || "",
        ])
      : [[ee("reports.insights.noFailureRanking")]])
  )

  if (role === "supplier") {
    const supplierRows = filterSupplierRows(data, query)
    rows.push(
      [],
      [ee("reports.export.supplierSection")],
      [ee("reports.export.supplier"), ee("reports.export.supplierNo"), ee("reports.export.assignedTickets"), ee("reports.export.processing"), ee("reports.export.completed"), ee("reports.export.responded"), ee("reports.export.avgFirstResponse"), ee("reports.export.completionRate"), ee("reports.export.averageRating"), ee("reports.export.ratingCount")],
      ...supplierRows.map((row) => [
        row.supplier_name || ee("reports.common.supplierFallback", { value0: row.supplier_id }),
        row.supplier_no || row.supplier_id,
        row.tickets_assigned,
        row.tickets_processing,
        row.tickets_completed,
        row.tickets_responded,
        formatMinutes(row.avg_response_minutes),
        formatPercent(row.completion_rate),
        row.rating_count > 0 ? row.average_rating.toFixed(1) : "",
        row.rating_count,
      ])
    )
    return rows
  }

  if (role === "engineer") {
    const ticketRows = filterTicketRows(data, query)
    rows.push(
      [],
      [ee("reports.export.ticketSection"), ee("reports.export.statusFilter"), getTableFilterLabel(filter)],
      [ee("reports.export.ticketNo"), ee("reports.export.summary"), ee("reports.export.customer"), ee("reports.export.product"), ee("reports.export.device"), ee("reports.export.status"), ee("reports.export.handler"), ee("reports.export.sla"), ee("reports.export.updatedAt")],
      ...ticketRows.map((row) => [
        row.ticket_no,
        row.title,
        row.customer_name || "",
        row.product_name || "",
        row.device_no || "",
        ticketStatusLabel(row.status),
        row.assignee_name || ee("reports.common.unassigned"),
        row.sla_breached ? ee("reports.common.overdue") : ee("reports.common.normal"),
        row.updated_at || "",
      ])
    )
    return rows
  }

  const teamRows = filterTeamRows(data, query)
  rows.push(
    [],
    [ee("reports.export.engineerSection")],
    [ee("reports.export.person"), ee("reports.export.userId"), ee("reports.export.assignedTickets"), ee("reports.export.resolvedTickets"), ee("reports.export.avgFirstResponse"), ee("reports.export.avgHandleTime"), ee("reports.export.satisfaction"), ee("reports.export.meetings")],
    ...teamRows.map((row) => [
      row.agent_name || ee("reports.common.userFallback", { value0: row.agent_id }),
      row.agent_id,
      row.tickets_assigned,
      row.tickets_resolved,
      formatMinutes(row.avg_response_minutes),
      formatMinutes(row.avg_handle_minutes),
      row.satisfaction > 0 ? row.satisfaction.toFixed(1) : "",
      row.meetings_held,
    ])
  )

  return rows
}

function ReportsTable({
  data,
  query,
  role,
}: {
  data: ReportState | null
  query: string
  role: ReportRole
}) {
  const teamRows = useMemo(() => filterTeamRows(data, query), [data, query])

  const ticketRows = useMemo(() => filterTicketRows(data, query), [data, query])

  const supplierRows = useMemo(() => filterSupplierRows(data, query), [data, query])

  if (role === "supplier") {
    return <SupplierPerformanceTable rows={supplierRows} />
  }

  if (role === "engineer") {
    return <TicketTable rows={ticketRows} total={data?.tickets?.total ?? ticketRows.length} />
  }

  return <TeamPerformanceTable rows={teamRows} />
}

function TeamPerformanceTable({ rows }: { rows: AgentPerformance[] }) {
  const columns: TableColumnsType<AgentPerformance> = [
    {
      title: ee("reports.export.person"),
      key: "agent",
      width: 180,
      render: (_, row) => (
        <div className="rhd-railops-reports-name-cell">
          <strong>{row.agent_name || `用户 ${row.agent_id}`}</strong>
          <span>ID {row.agent_id || "—"}</span>
        </div>
      ),
    },
    {
      title: ee("reports.export.assignedTickets"),
      dataIndex: "tickets_assigned",
      width: 96,
      align: "right",
      render: (value: number) => <span className="tabular-nums">{formatInteger(value)}</span>,
    },
    {
      title: ee("reports.export.resolvedTickets"),
      dataIndex: "tickets_resolved",
      width: 96,
      align: "right",
      render: (value: number) => <span className="tabular-nums">{formatInteger(value)}</span>,
    },
    {
      title: ee("reports.export.avgFirstResponse"),
      dataIndex: "avg_response_minutes",
      width: 120,
      render: (value: number) => <span className="tabular-nums">{formatMinutes(value)}</span>,
    },
    {
      title: ee("reports.export.avgHandleTime"),
      dataIndex: "avg_handle_minutes",
      width: 120,
      render: (value: number) => <span className="tabular-nums">{formatMinutes(value)}</span>,
    },
    {
      title: ee("reports.export.satisfaction"),
      dataIndex: "satisfaction",
      width: 112,
      render: (value: number) => value > 0 ? value.toFixed(1) : ee("reports.tables.noRating"),
    },
    {
      title: ee("reports.export.meetings"),
      dataIndex: "meetings_held",
      width: 96,
      align: "right",
      render: (value: number) => <span className="tabular-nums">{formatInteger(value)}</span>,
    },
  ]

  return (
    <ReportTableFrame count={rows.length} minWidth="820px">
      <DataTable<AgentPerformance>
        className="rhd-railops-reports-table"
        size="small"
        columns={columns}
        dataSource={rows}
        emptyDescription={<EmptyBlock message={ee("reports.insights.noEngineerPerformance")} />}
        rowKey={(row) => String(row.agent_id || row.agent_name)}
        scroll={{ x: 820 }}
      />
    </ReportTableFrame>
  )
}

function SupplierPerformanceTable({ rows }: { rows: SupplierPerformance[] }) {
  const columns: TableColumnsType<SupplierPerformance> = [
    {
      title: ee("reports.export.supplier"),
      key: "supplier",
      width: 220,
      render: (_, row) => (
        <div className="rhd-railops-reports-name-cell">
          <strong>{row.supplier_name || ee("reports.common.supplierFallback", { value0: row.supplier_id })}</strong>
          <span>{row.supplier_no || `ID ${row.supplier_id}`}</span>
        </div>
      ),
    },
    {
      title: ee("reports.export.assignedTickets"),
      dataIndex: "tickets_assigned",
      width: 104,
      align: "right",
      render: (value: number) => <span className="tabular-nums">{formatInteger(value)}</span>,
    },
    {
      title: ee("reports.export.processing"),
      dataIndex: "tickets_processing",
      width: 96,
      align: "right",
      render: (value: number) => <span className="tabular-nums">{formatInteger(value)}</span>,
    },
    {
      title: ee("reports.export.completed"),
      dataIndex: "tickets_completed",
      width: 96,
      align: "right",
      render: (value: number) => <span className="tabular-nums">{formatInteger(value)}</span>,
    },
    {
      title: ee("reports.export.avgFirstResponse"),
      key: "response",
      width: 120,
      render: (_, row) => row.tickets_responded > 0 ? formatMinutes(row.avg_response_minutes) : ee("reports.tables.noResponse"),
    },
    {
      title: ee("reports.export.completionRate"),
      dataIndex: "completion_rate",
      width: 104,
      render: (value: number) => <span className="tabular-nums">{formatPercent(value)}</span>,
    },
    {
      title: ee("reports.export.averageRating"),
      key: "rating",
      width: 140,
      render: (_, row) => row.rating_count > 0 ? ee("reports.tables.ratingValue", { value0: row.average_rating.toFixed(1), value1: row.rating_count }) : ee("reports.tables.noRating"),
    },
  ]

  return (
    <ReportTableFrame count={rows.length} minWidth="860px">
      <DataTable<SupplierPerformance>
        className="rhd-railops-reports-table"
        size="small"
        columns={columns}
        dataSource={rows}
        emptyDescription={<EmptyBlock message={ee("reports.insights.noSupplierPerformance")} />}
        rowKey={(row) => String(row.supplier_id || row.supplier_name)}
        scroll={{ x: 860 }}
      />
    </ReportTableFrame>
  )
}

function TicketTable({ rows, total }: { rows: TicketListItem[]; total: number }) {
  const columns: TableColumnsType<TicketListItem> = [
    {
      title: ee("reports.export.ticketNo"),
      dataIndex: "ticket_no",
      width: 140,
      render: (value: string) => <strong className="rhd-railops-reports-primary">{value}</strong>,
    },
    {
      title: ee("reports.export.summary"),
      dataIndex: "title",
      width: 260,
      render: (value: string) => <span className="rhd-railops-reports-ellipsis" title={value}>{value}</span>,
    },
    {
      title: ee("reports.export.customer"),
      dataIndex: "customer_name",
      width: 150,
      render: (value?: string) => <span className="rhd-railops-reports-ellipsis">{value || "—"}</span>,
    },
    {
      title: ee("reports.export.productDevice"),
      key: "product",
      width: 160,
      render: (_, row) => <span className="rhd-railops-reports-ellipsis">{row.product_name || row.device_no || "—"}</span>,
    },
    {
      title: ee("reports.export.status"),
      dataIndex: "status",
      width: 104,
      render: (value: string) => <ToneBadge tone={ticketStatusTone(value)}>{ticketStatusLabel(value)}</ToneBadge>,
    },
    {
      title: ee("reports.export.handler"),
      dataIndex: "assignee_name",
      width: 120,
      render: (value?: string) => <span className="rhd-railops-reports-ellipsis">{value || ee("reports.common.unassigned")}</span>,
    },
    {
      title: ee("reports.export.sla"),
      dataIndex: "sla_breached",
      width: 88,
      render: (value?: boolean) => <ToneBadge tone={value ? "bad" : "good"}>{value ? ee("reports.common.overdue") : ee("reports.common.normal")}</ToneBadge>,
    },
    {
      title: ee("reports.export.updatedAt"),
      dataIndex: "updated_at",
      width: 150,
      render: (value?: string) => value || "—",
    },
  ]

  return (
    <ReportTableFrame count={total} minWidth="980px">
      <DataTable<TicketListItem>
        className="rhd-railops-reports-table"
        size="small"
        columns={columns}
        dataSource={rows}
        emptyDescription={<EmptyBlock message={ee("reports.tables.noTickets")} />}
        rowKey={(record) => record.id}
        scroll={{ x: 980 }}
      />
    </ReportTableFrame>
  )
}

function ReportTableFrame({
  children,
  count,
  minWidth,
}: {
  children: ReactNode
  count: number
  minWidth: string
}) {
  return (
    <>
      <div className="rhd-railops-reports-table-frame">
        <div style={{ minWidth }}>{children}</div>
      </div>
      <div className="rhd-railops-table-summary">
        {ee("reports.common.totalRecords", { value0: formatInteger(count) })}
      </div>
    </>
  )
}

export default function EnterpriseReportsPage() {
  const [role, setRole] = useState<ReportRole>("admin")
  const [sectionTab, setSectionTab] = useState<ReportSectionTab>("overview")
  const [filter, setFilter] = useState<TicketFilter>("all")
  const [periodKey, setPeriodKey] = useState<DateRangeKey>("current_month")
  const [query, setQuery] = useState("")
  const [data, setData] = useState<ReportState | null>(null)
  const [loading, setLoading] = useState(true)
  const [dataLoaded, setDataLoaded] = useState(false)
  const [error, setError] = useState("")

  const range = useMemo(() => getDateRange(periodKey), [periodKey])

  const loadReports = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const ticketStatus = role === "engineer" ? getTicketStatusQuery(filter) : undefined
      const [overviewRes, failuresRes, teamRes, supplierRes, ticketsRes] = await Promise.all([
        fetchReportsOverview({ start: range.start, end: range.end }),
        fetchReportTopFailures({ limit: 5, start: range.start, end: range.end }),
        fetchReportTeamPerformance({ start: range.start, end: range.end }),
        role === "supplier"
          ? fetchReportSupplierPerformance({ start: range.start, end: range.end })
          : Promise.resolve(null),
        role === "engineer"
          ? fetchTickets({
              page: 1,
              page_size: 20,
              search: query || undefined,
              sort: "-updated_at",
              status: ticketStatus,
            })
          : Promise.resolve(null),
      ])

      if (!overviewRes.success) throw new Error(overviewRes.error?.message || ee("reports.errors.overviewLoadFailed"))
      if (!failuresRes.success) throw new Error(failuresRes.error?.message || ee("reports.errors.failuresLoadFailed"))
      if (!teamRes.success) throw new Error(teamRes.error?.message || ee("reports.errors.teamLoadFailed"))
      if (supplierRes && !supplierRes.success) throw new Error(supplierRes.error?.message || ee("reports.errors.supplierLoadFailed"))
      if (ticketsRes && !ticketsRes.success) throw new Error(ticketsRes.error?.message || ee("reports.errors.ticketsLoadFailed"))

      setData({
        overview: overviewRes.data,
        supplierPerformance: supplierRes?.data ?? [],
        teamPerformance: teamRes.data ?? [],
        tickets: ticketsRes?.data ?? null,
        topFailures: failuresRes.data ?? [],
      })
      setDataLoaded(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : ee("reports.errors.dataLoadFailed"))
    } finally {
      setLoading(false)
    }
  }, [filter, query, range.end, range.start, role])

  const handleExport = useCallback(() => {
    if (!data) return
    const rows = buildReportExportRows({
      data,
      filter,
      periodKey,
      query,
      rangeLabel: range.label,
      role,
    })
    downloadCsv(ee("reports.export.fileName", { value0: getRoleLabel(role), value1: range.start, value2: range.end }), rows)
  }, [data, filter, periodKey, query, range.end, range.label, range.start, role])

  useEffect(() => {
    void loadReports()
  }, [loadReports])

  const handleRoleChange = (nextRole: ReportRole) => {
    setRole(nextRole)
    setFilter("all")
    setQuery("")
    setSectionTab("overview")
  }

  const initialLoading = loading && !dataLoaded

  return (
    <PageShell
      title={ee("reports.pageTitle")}
      breadcrumb={useRouteBreadcrumbItems()}
      className="rhd-railops-reports-page"
    >
      <ContentModule className="rhd-railops-reports-filter-module">
          <div className="rhd-railops-reports-control-bar">
            <div className="rhd-railops-reports-control-group">
              <span>{ee("reports.labels.reportScope")}</span>
              <FilterTabs
                ariaLabel={ee("reports.labels.reportScope")}
                items={roleTabs.map((tab) => ({ value: tab.key, label: ee(tab.labelKey) }))}
                value={role}
                onChange={(value) => handleRoleChange(value as ReportRole)}
              />
            </div>
            <div className="rhd-railops-reports-control-group">
              <span>{ee("reports.labels.period")}</span>
              <FilterTabs
                ariaLabel={ee("reports.labels.period")}
                items={periodTabs.map((tab) => ({ value: tab.key, label: ee(tab.labelKey) }))}
                value={periodKey}
                onChange={(value) => setPeriodKey(value as DateRangeKey)}
              />
            </div>
            <div className="rhd-railops-reports-control-meta">
              <span>{range.label}</span>
              <RailopsButton
                size="small"
                className="rhd-railops-reports-export-button"
                onClick={handleExport}
                disabled={!data}
              >
                <DownloadIcon className="size-3.5" />
                {ee("reports.actions.exportCsv")}
              </RailopsButton>
              <IconButton
                icon={<RefreshCwIcon className={cn("size-4", loading && "animate-spin")} />}
                tooltip={ee("reports.actions.refresh")}
                aria-label={ee("reports.actions.refreshReport")}
                onClick={() => void loadReports()}
                disabled={loading}
              />
            </div>
          </div>
      </ContentModule>

      {initialLoading ? (
        <ReportsLoadingContent role={role} />
      ) : error && !data ? (
        <ErrorBlock message={error} onRetry={() => void loadReports()} />
      ) : (
        <div className="relative grid gap-4" aria-busy={loading ? "true" : undefined}>
          {loading ? (
            <div
              className="flex min-h-9 items-center gap-2 border border-blue-100 bg-blue-50 px-3 text-xs font-medium text-blue-700"
              role="status"
              aria-label={ee("reports.labels.refreshing")}
            >
              <RefreshCwIcon className="size-3.5 animate-spin" aria-hidden="true" />
              {ee("reports.labels.refreshingMessage")}
            </div>
          ) : null}
          {error ? (
            <div className="flex flex-col gap-2 border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800 sm:flex-row sm:items-center sm:justify-between">
              <span>{ee("reports.labels.refreshFailed", { value0: error })}</span>
              <RailopsButton size="small" onClick={() => void loadReports()}>
                <RefreshCwIcon className="size-3.5" />
                {ee("reports.actions.retry")}
              </RailopsButton>
            </div>
          ) : null}
          <ContentModule className="rhd-railops-reports-section-tabs">
            <FilterTabs
              ariaLabel={ee("reports.labels.contentTabs")}
              items={sectionTabs.map((tab) => ({ value: tab.key, label: ee(tab.labelKey) }))}
              value={sectionTab}
              onChange={(value) => setSectionTab(value as ReportSectionTab)}
            />
          </ContentModule>

          {sectionTab === "overview" ? (
            <section className="rhd-railops-reports-tab-panel">
              <KpiGrid data={data} role={role} />
              {role === "admin" ? <ProcurementDecisionMetrics overview={data?.overview ?? null} /> : null}
            </section>
          ) : null}

          {sectionTab === "insights" ? (
            <section className="rhd-railops-reports-tab-panel">
              <Insights data={data} role={role} />
            </section>
          ) : null}

          {sectionTab === "details" ? (
            <section className="rhd-railops-reports-tab-panel">
              <ContentModule className="rhd-railops-reports-table-toolbar-module">
                <div className="flex min-w-0 flex-wrap items-center gap-2">
                  {role === "engineer" ? (
                    <FilterTabs
                      ariaLabel={ee("reports.labels.ticketStatusFilter")}
                      items={tableFilterTabs.map((tab) => ({ value: tab.key, label: ee(tab.labelKey) }))}
                      value={filter}
                      onChange={(value) => setFilter(value as TicketFilter)}
                    />
                  ) : (
                    <StatusTag tone="blue">{role === "supplier" ? ee("reports.labels.supplierDetail") : ee("reports.labels.engineerDetail")}</StatusTag>
                  )}
                </div>
                <div className="flex min-w-0 flex-col gap-2 sm:flex-row sm:items-center lg:justify-end">
                  <SearchField
                    className="rhd-railops-reports-search"
                    placeholder={role === "supplier" ? ee("reports.placeholders.searchSupplier") : ee("reports.placeholders.searchEngineer")}
                    value={query}
                    onChange={(event) => setQuery(event.target.value)}
                  />
                </div>
              </ContentModule>

              <ReportsTable data={data} query={query} role={role} />
            </section>
          ) : null}
        </div>
      )}
    </PageShell>
  )
}
