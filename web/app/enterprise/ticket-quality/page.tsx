"use client"

import Link from "next/link"
import { useCallback, useEffect, useMemo, useState } from "react"
import { useRouter } from "next/navigation"
import dayjs from "dayjs"
import { ClipboardCheckIcon, ExternalLinkIcon, EyeIcon, PlayIcon, RefreshCwIcon, SparklesIcon } from "lucide-react"
import { DatePicker, Empty, Input, Select, Tag, type TableColumnsType } from "antd"
import {
  ContentModule,
  DataTable,
  PageShell,
  RailopsButton,
  StatCard,
  StatGrid,
  StandardModal,
  StatusTag,
  type StatusTagTone,
} from "@railops/ui"
import { toast } from "sonner"

import { useAuth } from "@/components/auth-provider"
import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import { translateCurrentMessage } from "@/i18n/messages"
import { useAppLocale } from "@/i18n/provider"
import { TicketQualityPanel } from "@/app/enterprise/tickets/_components/ticket-quality-panel"
import { TicketQualityDisputeQueue } from "./dispute-queue"
import {
  fetchTicketQualitySamples,
  fetchTicketQualityAnalysis,
  type TicketQualityAnalysis,
  type TicketQualityAnalysisBucket,
  generateTicketQualitySamples,
  startTicketQualitySample,
  type TicketQualityReason,
  type TicketQualitySample,
} from "@/lib/api/ticket-quality"

function t(key: string, values?: Record<string, string | number>) {
  return translateCurrentMessage(`ticketQuality.${key}`, values)
}

const PAGE_SIZE = 10
const reasonKeys: TicketQualityReason[] = ["random", "high_priority", "new_agent", "complaint", "reopened", "risk"]
const qualityStatuses = ["pending", "in_progress", "completed"] as const
const qualityCategoryCodes = new Set(["user_case", "incident", "major_incident", "problem", "known_error", "service_request"])
const qualityChannelCodes = new Set(["enterprise", "phone", "im", "web", "email", "wechat_mp", "wxwork_kf", "whatsapp"])

const ticketStatuses = new Set([
  "draft", "pending", "pending_acceptance", "pending_dispatch", "accepted", "assigned",
  "pending_assignee_accept", "in_progress", "processing", "video_support", "supplier_support",
  "escalated", "waiting_customer", "resolved", "pending_customer_confirm", "closed",
  "quality_review", "reopened", "done", "cancelled",
])

function reasonLabel(reason: string) {
  return reasonKeys.includes(reason) ? t(`reasons.${reason}`) : reason
}

function statusTone(status: string): StatusTagTone {
  if (status === "completed") return "success"
  if (status === "in_progress") return "blue"
  return "warning"
}

function statusLabel(status: string) {
  return qualityStatuses.includes(status as (typeof qualityStatuses)[number]) ? t(`status.${status}`) : status
}

function reviewResultLabel(result: string) {
  if (result === "pass") return t("review.result.pass")
  if (result === "needs_improvement") return t("review.result.needsImprovement")
  return t("review.result.fail")
}

function reviewResultTone(result: string): StatusTagTone {
  if (result === "pass") return "success"
  if (result === "needs_improvement") return "warning"
  return "error"
}

function ticketStatusLabel(status: string) {
  return ticketStatuses.has(status) ? t(`ticketStatus.${status}`) : status || "-"
}

function priorityLabel(priority: string) {
  const normalized = priority.trim().toLowerCase()
  const key = ({ critical: "p1", high: "p2", medium: "p3", low: "p4" } as Record<string, string>)[normalized] ?? normalized
  return ["p1", "p2", "p3", "p4"].includes(key) ? t(`priority.${key}`) : priority || "-"
}

function formatDate(value: string, locale: string) {
  if (!value) return "-"
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString(locale, { hour12: false })
}

function analysisBucketLabel(dimension: string, item: TicketQualityAnalysisBucket) {
  const missingKey = ({
    __unset__: "unset",
    __ungrouped__: "ungrouped",
    __unassigned__: "unassigned",
    __uncategorized__: "uncategorized",
  } as Record<string, string>)[item.value]
  if (missingKey) return t(`analysis.values.${missingKey}`)
  if (dimension === "category" && qualityCategoryCodes.has(item.value)) return t(`analysis.values.categories.${item.value}`)
  if (dimension === "channel" && qualityChannelCodes.has(item.value)) return t(`analysis.values.channels.${item.value}`)
  return item.label || item.value || "-"
}

type TicketQualityContentProps = { embedded?: boolean }

export function TicketQualityContent({ embedded = false }: TicketQualityContentProps) {
  const { locale } = useAppLocale()
  const [periodKey, setPeriodKey] = useState(() => dayjs().format("YYYY-MM-DD"))
  const [status, setStatus] = useState("")
  const [reason, setReason] = useState("")
  const [search, setSearch] = useState("")
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(false)
  const [data, setData] = useState<Awaited<ReturnType<typeof fetchTicketQualitySamples>>["data"] | null>(null)
  const [reviewTarget, setReviewTarget] = useState<TicketQualitySample | null>(null)
  const [analysis, setAnalysis] = useState<TicketQualityAnalysis | null>(null)
  const [analysisFrom, setAnalysisFrom] = useState("")
  const [analysisTo, setAnalysisTo] = useState("")
  const [analysisAgentID, setAnalysisAgentID] = useState("")
  const [analysisCategory, setAnalysisCategory] = useState("")
  const [analysisChannel, setAnalysisChannel] = useState("")
  const [analysisLoading, setAnalysisLoading] = useState(false)
  const breadcrumbItems = useRouteBreadcrumbItems()
  const { session } = useAuth()
  // Personal results are only filterable and reviewable with the dedicated permissions.
  const canViewAgentQuality = Boolean(session?.permissions?.includes("ticketQuality.viewAgent"))
  const canReviewDisputes = Boolean(session?.permissions?.includes("ticketQuality.disputeReview"))

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const response = await fetchTicketQualitySamples({
        page,
        page_size: PAGE_SIZE,
        period_key: periodKey,
        status: status || undefined,
        reason: reason || undefined,
        search: search.trim() || undefined,
      })
      if (!response.success) throw new Error(response.error?.message || t("errors.load"))
      setData(response.data)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("errors.load"))
    } finally {
      setLoading(false)
    }
  }, [page, periodKey, reason, search, status])

  useEffect(() => {
    void load()
  }, [load])

  const loadAnalysis = useCallback(async () => {
    setAnalysisLoading(true)
    try {
      const response = await fetchTicketQualityAnalysis({
        from: analysisFrom || undefined, to: analysisTo || undefined,
        agent_id: analysisAgentID ? Number(analysisAgentID) : undefined,
        category: analysisCategory || undefined, channel: analysisChannel || undefined,
      })
      if (!response.success) throw new Error(response.error?.message || t("errors.load"))
      setAnalysis(response.data)
    } catch (error) { toast.error(error instanceof Error ? error.message : t("errors.load")) } finally { setAnalysisLoading(false) }
  }, [analysisAgentID, analysisCategory, analysisChannel, analysisFrom, analysisTo])

  const refresh = () => void load()
  const refreshAnalysis = () => void loadAnalysis()

  const generate = async () => {
    try {
      const response = await generateTicketQualitySamples({ period_key: periodKey, random_count: 10 })
      if (!response.success) throw new Error(response.error?.message || t("errors.generate"))
      toast.success(t("messages.generated", { count: response.data.created }))
      await load()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("errors.generate"))
    }
  }

  const start = useCallback(async (item: TicketQualitySample) => {
    try {
      const response = await startTicketQualitySample(item.id)
      if (!response.success) throw new Error(response.error?.message || t("errors.update"))
      setReviewTarget({ ...item, status: "in_progress" })
      await load()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("errors.update"))
    }
  }, [load])

  const handleReviewSubmitted = async () => {
    setReviewTarget(null)
    await load()
  }

  const columns = useMemo<TableColumnsType<TicketQualitySample>>(() => [
    {
      title: t("columns.ticket"),
      key: "ticket",
      render: (_, item) => (
        <div className="flex min-w-0 flex-col gap-0.5">
          <Link className="font-medium text-blue-700 hover:underline" href={`/enterprise/ticket-workbench?ticket_id=${item.ticket_id}`}>
            {item.ticket_no || `#${item.ticket_id}`} <ExternalLinkIcon className="inline size-3" />
          </Link>
          <span className="max-w-[26rem] truncate text-xs text-slate-500">{item.title || "-"}</span>
        </div>
      ),
    },
    {
      title: t("columns.reasons"),
      key: "reasons",
      render: (_, item) => <div className="flex flex-wrap gap-1">{item.reasons.map((value) => <Tag key={value}>{reasonLabel(value)}</Tag>)}</div>,
    },
    { title: t("columns.priority"), key: "priority", render: (_, item) => priorityLabel(item.priority) },
    { title: t("columns.ticketStatus"), key: "ticket_status", render: (_, item) => ticketStatusLabel(item.ticket_status) },
    { title: t("columns.assignee"), key: "assignee", render: (_, item) => item.assignee_name || "-" },
    { title: t("columns.sampledAt"), key: "sampled_at", render: (_, item) => formatDate(item.sampled_at, locale) },
    {
      title: t("columns.reviewResult"),
      key: "review_result",
      render: (_, item) => item.latest_review ? (
        <div className="flex flex-col gap-0.5">
          <span className="font-medium tabular-nums">{item.latest_review.total_score}/{item.latest_review.max_score}</span>
          <StatusTag tone={reviewResultTone(item.latest_review.final_outcome || item.latest_review.outcome || item.latest_review.result)}>{reviewResultLabel(item.latest_review.final_outcome || item.latest_review.outcome || item.latest_review.result)}</StatusTag>
        </div>
      ) : <span className="text-slate-400">{t("review.notEvaluated")}</span>,
    },
    {
      title: t("columns.qaStatus"),
      key: "status",
      render: (_, item) => <StatusTag tone={statusTone(item.status)}>{statusLabel(item.status)}</StatusTag>,
    },
    {
      title: t("columns.actions"),
      key: "actions",
      render: (_, item) => (
        <div className="flex gap-1">
          {item.latest_review ? <RailopsButton size="small" onClick={() => setReviewTarget(item)}><EyeIcon className="size-3.5" />{t("actions.viewResult")}</RailopsButton> : null}
          {!item.latest_review && item.status === "pending" ? <RailopsButton size="small" onClick={() => void start(item)}><PlayIcon className="size-3.5" />{t("actions.start")}</RailopsButton> : null}
          {!item.latest_review && item.status !== "pending" ? <RailopsButton size="small" onClick={() => setReviewTarget(item)}><SparklesIcon className="size-3.5" />{t("actions.autoReview")}</RailopsButton> : null}
        </div>
      ),
    },
  ], [locale, start])

  const summary = data?.summary ?? { total: 0, pending: 0, in_progress: 0, completed: 0 }

  const actions = <div className="flex gap-2"><RailopsButton onClick={() => void generate()} disabled={loading}><ClipboardCheckIcon className="size-4" />{t("actions.generate")}</RailopsButton><RailopsButton onClick={refresh} disabled={loading}><RefreshCwIcon className={loading ? "size-4 animate-spin" : "size-4"} />{t("actions.refresh")}</RailopsButton></div>
  const analysisCards: Array<[string, string, TicketQualityAnalysisBucket | undefined]> = [
    ["overall", t("analysis.cards.overall"), analysis?.summary],
    ["project", t("analysis.cards.project"), analysis?.by_project?.[0]],
    ["team", t("analysis.cards.team"), analysis?.by_team?.[0]],
    ["agent", t(analysis?.can_view_agents ? "analysis.cards.agent" : "analysis.cards.agentRestricted"), analysis?.by_agent?.[0]],
    ["category", t("analysis.cards.category"), analysis?.by_category?.[0]],
  ]
  const analysisLists: Array<[string, string, TicketQualityAnalysisBucket[] | undefined]> = [
    ["project", t("analysis.groups.project"), analysis?.by_project],
    ["team", t("analysis.groups.team"), analysis?.by_team],
    ["category", t("analysis.groups.category"), analysis?.by_category],
    ["channel", t("analysis.groups.channel"), analysis?.by_channel],
    ...(analysis?.can_view_agents ? [["agent", t("analysis.groups.agent"), analysis.by_agent ?? []] as [string, string, TicketQualityAnalysisBucket[]]] : []),
  ]
  const hasAnalysis = Boolean(analysis && analysis.summary.reviews > 0)
  const content = (
    <>
      {embedded ? <ContentModule><div className="flex items-center justify-between gap-3"><div><div className="text-sm font-semibold">{t("title")}</div><div className="text-xs text-slate-500">{t("description")}</div></div>{actions}</div></ContentModule> : null}
      <ContentModule>
        <div className="mb-4">
          <div className="text-sm font-semibold">{t("analysis.title")}</div>
          <div className="mt-0.5 text-xs text-slate-500">{t("analysis.description")}</div>
        </div>
        <div className="grid items-end gap-3 md:grid-cols-2 xl:grid-cols-4 2xl:grid-cols-8">
          <label className="flex min-w-0 flex-col gap-1 text-xs text-slate-500">{t("analysis.filters.startDate")}<DatePicker className="ticket-quality-date-picker h-9 w-full" value={analysisFrom ? dayjs(analysisFrom) : null} format="YYYY-MM-DD" placeholder={t("analysis.filters.datePlaceholder")} onChange={(value) => setAnalysisFrom(value?.format("YYYY-MM-DD") ?? "")} /></label>
          <label className="flex min-w-0 flex-col gap-1 text-xs text-slate-500">{t("analysis.filters.endDate")}<DatePicker className="ticket-quality-date-picker h-9 w-full" value={analysisTo ? dayjs(analysisTo) : null} format="YYYY-MM-DD" placeholder={t("analysis.filters.datePlaceholder")} onChange={(value) => setAnalysisTo(value?.format("YYYY-MM-DD") ?? "")} /></label>
          {canViewAgentQuality ? <label className="flex min-w-0 flex-col gap-1 text-xs text-slate-500">{t("analysis.filters.agent")}<Input className="h-9 w-full" value={analysisAgentID} onChange={(event) => setAnalysisAgentID(event.target.value.replace(/\D/g, ""))} placeholder={t("analysis.filters.idPlaceholder")} /></label> : null}
          <label className="flex min-w-0 flex-col gap-1 text-xs text-slate-500">{t("analysis.filters.category")}<Input className="h-9 w-full" value={analysisCategory} onChange={(event) => setAnalysisCategory(event.target.value)} placeholder={t("analysis.filters.optionalPlaceholder")} /></label>
          <label className="flex min-w-0 flex-col gap-1 text-xs text-slate-500">{t("analysis.filters.channel")}<Input className="h-9 w-full" value={analysisChannel} onChange={(event) => setAnalysisChannel(event.target.value)} placeholder={t("analysis.filters.optionalPlaceholder")} /></label>
          <RailopsButton className="h-9 w-full" onClick={refreshAnalysis} disabled={analysisLoading}><RefreshCwIcon className={analysisLoading ? "size-4 animate-spin" : "size-4"} />{t("analysis.actions.analyze")}</RailopsButton>
        </div>
        {!hasAnalysis ? (
          <div className="mt-4 border-t border-border py-8"><Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={analysis ? t("analysis.empty") : t("analysis.notRun")} /></div>
        ) : (
          <>
            <div className="mt-4 grid gap-3 md:grid-cols-2 xl:grid-cols-5">
              {analysisCards.map(([dimension, label, bucket]) => <div key={dimension} className="rounded-md border border-border p-3"><div className="text-xs text-slate-500">{label}</div>{bucket && dimension !== "overall" ? <div className="mt-1 truncate text-sm font-medium">{analysisBucketLabel(dimension, bucket)}</div> : null}<div className="mt-1 text-lg font-semibold">{bucket ? t("analysis.metrics.score", { value: bucket.average.toFixed(1) }) : "-"}</div><div className="text-xs text-slate-500">{bucket ? t("analysis.metrics.summary", { reviews: bucket.reviews, rate: bucket.pass_rate.toFixed(0) }) : t("analysis.empty")}</div></div>)}
            </div>
            <div className="mt-4 grid gap-4 xl:grid-cols-2">
              {analysisLists.filter(([, , values]) => Boolean(values?.length)).map(([dimension, title, values]) => <div key={dimension} className="rounded-md border border-border p-3"><div className="mb-2 text-sm font-semibold">{title}</div><div className="space-y-1.5">{values?.slice(0, 8).map((item) => <div key={item.value} className="flex justify-between gap-3 text-xs"><span className="truncate">{analysisBucketLabel(dimension, item)}</span><span className="shrink-0">{t("analysis.metrics.row", { value: item.average.toFixed(1), reviews: item.reviews, rate: item.pass_rate.toFixed(0) })}</span></div>)}</div></div>)}
            </div>
          </>
        )}
      </ContentModule>

      {canReviewDisputes ? <TicketQualityDisputeQueue /> : null}

      <StatGrid>
        <StatCard label={t("summary.total")} value={summary.total} tone="blue" />
        <StatCard label={t("summary.pending")} value={summary.pending} tone="warning" />
        <StatCard label={t("summary.inProgress")} value={summary.in_progress} tone="blue" />
        <StatCard label={t("summary.completed")} value={summary.completed} tone="success" />
      </StatGrid>

      <ContentModule>
        <div className="grid items-end gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <label className="flex min-w-0 flex-col gap-1 text-xs text-slate-500">{t("filters.period")}<DatePicker className="ticket-quality-date-picker h-9 w-full" allowClear={false} value={dayjs(periodKey)} format="YYYY-MM-DD" placeholder={t("analysis.filters.datePlaceholder")} onChange={(value) => { if (!value) return; setPage(1); setPeriodKey(value.format("YYYY-MM-DD")) }} /></label>
          <label className="flex min-w-0 flex-col gap-1 text-xs text-slate-500">{t("filters.status")}<Select className="h-9 w-full" value={status || undefined} allowClear placeholder={t("filters.all")} onChange={(value) => { setPage(1); setStatus(value || "") }} options={["pending", "in_progress", "completed"].map((value) => ({ value, label: statusLabel(value) }))} /></label>
          <label className="flex min-w-0 flex-col gap-1 text-xs text-slate-500">{t("filters.reason")}<Select className="h-9 w-full" value={reason || undefined} allowClear placeholder={t("filters.all")} onChange={(value) => { setPage(1); setReason(value || "") }} options={reasonKeys.map((value) => ({ value, label: reasonLabel(value) }))} /></label>
          <label className="flex min-w-0 flex-col gap-1 text-xs text-slate-500">{t("filters.search")}<Input className="h-9 w-full" value={search} allowClear placeholder={t("filters.searchPlaceholder")} onChange={(event) => { setPage(1); setSearch(event.target.value) }} /></label>
        </div>
      </ContentModule>

      <ContentModule>
        <DataTable<TicketQualitySample>
          rowKey="id"
          columns={columns}
          dataSource={data?.items ?? []}
          loading={loading}
          total={data?.total ?? 0}
          current={page}
          pageSize={PAGE_SIZE}
          onPageChange={(nextPage) => setPage(nextPage)}
          emptyDescription={t("empty")}
        />
      </ContentModule>

      <StandardModal width={760} title={reviewTarget?.latest_review ? t("completeModal.resultTitle") : t("completeModal.title")} open={Boolean(reviewTarget)} onCancel={() => setReviewTarget(null)} footer={null}>
        {reviewTarget ? <TicketQualityPanel key={reviewTarget.ticket_id} ticketID={reviewTarget.ticket_id} sampleID={reviewTarget.id} onSubmitted={handleReviewSubmitted} /> : null}
      </StandardModal>
    </>
  )
  if (embedded) return content
  return <PageShell title={t("title")} description={t("description")} breadcrumb={breadcrumbItems} actions={actions}>{content}</PageShell>
}

export default function TicketQualityPage() {
  const router = useRouter()
  useEffect(() => {
    router.replace("/enterprise/reports?section=quality")
  }, [router])
  return null
}
