"use client"

import Link from "next/link"
import { useCallback, useEffect, useMemo, useState } from "react"
import { useRouter } from "next/navigation"
import { CheckCircle2Icon, ClipboardCheckIcon, ExternalLinkIcon, PlayIcon, RefreshCwIcon } from "lucide-react"
import { Input, Select, Tag, type TableColumnsType } from "antd"
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

import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import { translateCurrentMessage } from "@/i18n/messages"
import { TicketQualityPanel } from "@/app/enterprise/tickets/_components/ticket-quality-panel"
import {
  completeTicketQualitySample,
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

function reasonLabel(reason: string) {
  return t(`reasons.${reason}`)
}

function statusTone(status: string): StatusTagTone {
  if (status === "completed") return "success"
  if (status === "in_progress") return "blue"
  return "warning"
}

function statusLabel(status: string) {
  return t(`status.${status}`)
}

function formatDate(value: string) {
  if (!value) return "-"
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

type TicketQualityContentProps = { embedded?: boolean }

export function TicketQualityContent({ embedded = false }: TicketQualityContentProps) {
  const [periodKey, setPeriodKey] = useState(() => new Date().toISOString().slice(0, 10))
  const [status, setStatus] = useState("")
  const [reason, setReason] = useState("")
  const [search, setSearch] = useState("")
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(false)
  const [data, setData] = useState<Awaited<ReturnType<typeof fetchTicketQualitySamples>>["data"] | null>(null)
  const [reviewTarget, setReviewTarget] = useState<TicketQualitySample | null>(null)
  const [note, setNote] = useState("")
  const [analysis, setAnalysis] = useState<TicketQualityAnalysis | null>(null)
  const [analysisFrom, setAnalysisFrom] = useState("")
  const [analysisTo, setAnalysisTo] = useState("")
  const [analysisProject, setAnalysisProject] = useState("")
  const [analysisTeamID, setAnalysisTeamID] = useState("")
  const [analysisAgentID, setAnalysisAgentID] = useState("")
  const [analysisCategory, setAnalysisCategory] = useState("")
  const [analysisChannel, setAnalysisChannel] = useState("")
  const [analysisLoading, setAnalysisLoading] = useState(false)
  const breadcrumbItems = useRouteBreadcrumbItems()

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
        from: analysisFrom || undefined, to: analysisTo || undefined, project: analysisProject || undefined,
        team_id: analysisTeamID ? Number(analysisTeamID) : undefined, agent_id: analysisAgentID ? Number(analysisAgentID) : undefined,
        category: analysisCategory || undefined, channel: analysisChannel || undefined,
      })
      if (!response.success) throw new Error(response.error?.message || t("errors.load"))
      setAnalysis(response.data)
    } catch (error) { toast.error(error instanceof Error ? error.message : t("errors.load")) } finally { setAnalysisLoading(false) }
  }, [analysisAgentID, analysisCategory, analysisChannel, analysisFrom, analysisProject, analysisTeamID, analysisTo])

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

  const complete = async () => {
    if (!reviewTarget) return
    try {
      const response = await completeTicketQualitySample(reviewTarget.id, note)
      if (!response.success) throw new Error(response.error?.message || t("errors.update"))
      setReviewTarget(null)
      setNote("")
      await load()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("errors.update"))
    }
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
    { title: t("columns.priority"), dataIndex: "priority", key: "priority" },
    { title: t("columns.ticketStatus"), dataIndex: "ticket_status", key: "ticket_status" },
    { title: t("columns.assignee"), key: "assignee", render: (_, item) => item.assignee_name || "-" },
    { title: t("columns.sampledAt"), key: "sampled_at", render: (_, item) => formatDate(item.sampled_at) },
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
          {item.status === "pending" ? <RailopsButton size="small" onClick={() => void start(item)}><PlayIcon className="size-3.5" />{t("actions.start")}</RailopsButton> : null}
          {item.status === "in_progress" ? <RailopsButton size="small" onClick={() => setReviewTarget(item)}><CheckCircle2Icon className="size-3.5" />{t("actions.review")}</RailopsButton> : null}
        </div>
      ),
    },
  ], [start])

  const summary = data?.summary ?? { total: 0, pending: 0, in_progress: 0, completed: 0 }

  const actions = <div className="flex gap-2"><RailopsButton onClick={() => void generate()} disabled={loading}><ClipboardCheckIcon className="size-4" />{t("actions.generate")}</RailopsButton><RailopsButton onClick={refresh} disabled={loading}><RefreshCwIcon className={loading ? "size-4 animate-spin" : "size-4"} />{t("actions.refresh")}</RailopsButton></div>
  const analysisCards: Array<[string, TicketQualityAnalysisBucket | undefined]> = [
    ["总体", analysis?.summary],
    ["Project", analysis?.by_project?.[0]],
    ["Team", analysis?.by_team?.[0]],
    [analysis?.can_view_agents ? "Agent" : "Agent（受限）", analysis?.by_agent?.[0]],
    ["Category", analysis?.by_category?.[0]],
  ]
  const analysisLists: Array<[string, TicketQualityAnalysisBucket[] | undefined]> = [
    ["按 Project", analysis?.by_project],
    ["按 Team", analysis?.by_team],
    ["按 Category", analysis?.by_category],
    ["按 Channel", analysis?.by_channel],
    ...(analysis?.can_view_agents ? [["按 Agent", analysis.by_agent ?? []] as [string, TicketQualityAnalysisBucket[]]] : []),
  ]
  const content = (
    <>
      {embedded ? <ContentModule><div className="flex items-center justify-between gap-3"><div><div className="text-sm font-semibold">{t("title")}</div><div className="text-xs text-slate-500">{t("description")}</div></div>{actions}</div></ContentModule> : null}
      <ContentModule>
        <div className="flex flex-wrap items-end gap-3">
          <label className="flex flex-col gap-1 text-xs text-slate-500">开始日期<Input type="date" value={analysisFrom} onChange={(event) => setAnalysisFrom(event.target.value)} /></label>
          <label className="flex flex-col gap-1 text-xs text-slate-500">结束日期<Input type="date" value={analysisTo} onChange={(event) => setAnalysisTo(event.target.value)} /></label>
          <label className="flex flex-col gap-1 text-xs text-slate-500">Project<Input value={analysisProject} onChange={(event) => setAnalysisProject(event.target.value)} placeholder="项目 key" /></label>
          <label className="flex flex-col gap-1 text-xs text-slate-500">Team ID<Input value={analysisTeamID} onChange={(event) => setAnalysisTeamID(event.target.value.replace(/\D/g, ""))} /></label>
          <label className="flex flex-col gap-1 text-xs text-slate-500">Agent ID<Input value={analysisAgentID} onChange={(event) => setAnalysisAgentID(event.target.value.replace(/\D/g, ""))} /></label>
          <label className="flex flex-col gap-1 text-xs text-slate-500">Category<Input value={analysisCategory} onChange={(event) => setAnalysisCategory(event.target.value)} /></label>
          <label className="flex flex-col gap-1 text-xs text-slate-500">Channel<Input value={analysisChannel} onChange={(event) => setAnalysisChannel(event.target.value)} /></label>
          <RailopsButton onClick={refreshAnalysis} disabled={analysisLoading}><RefreshCwIcon className={analysisLoading ? "size-4 animate-spin" : "size-4"} />分析</RailopsButton>
        </div>
        <div className="mt-4 grid gap-3 md:grid-cols-2 xl:grid-cols-5">
          {analysisCards.map(([label, bucket]) => <div key={label} className="rounded-md border border-border p-3"><div className="text-xs text-slate-500">{label}</div><div className="mt-1 text-lg font-semibold">{bucket ? `${bucket.average.toFixed(1)} 分` : "-"}</div><div className="text-xs text-slate-500">{bucket ? `${bucket.reviews} 次评估 · 通过率 ${bucket.pass_rate.toFixed(0)}%` : "暂无数据"}</div></div>)}
        </div>
        <div className="mt-4 grid gap-4 xl:grid-cols-2">
          {analysisLists.map(([title, values]) => <div key={title} className="rounded-md border border-border p-3"><div className="mb-2 text-sm font-semibold">{title}</div><div className="space-y-1.5">{values?.slice(0, 8).map((item) => <div key={item.value} className="flex justify-between gap-3 text-xs"><span className="truncate">{item.label}</span><span className="shrink-0">{item.average.toFixed(1)} 分 · {item.reviews} 次 · {item.pass_rate.toFixed(0)}%</span></div>)}</div></div>)}
        </div>
      </ContentModule>

      <StatGrid>
        <StatCard label={t("summary.total")} value={summary.total} tone="blue" />
        <StatCard label={t("summary.pending")} value={summary.pending} tone="warning" />
        <StatCard label={t("summary.inProgress")} value={summary.in_progress} tone="blue" />
        <StatCard label={t("summary.completed")} value={summary.completed} tone="success" />
      </StatGrid>

      <ContentModule>
        <div className="flex flex-wrap items-end gap-3">
          <label className="flex flex-col gap-1 text-xs text-slate-500">{t("filters.period")}<Input type="date" value={periodKey} onChange={(event) => { setPage(1); setPeriodKey(event.target.value) }} /></label>
          <label className="flex flex-col gap-1 text-xs text-slate-500">{t("filters.status")}<Select className="min-w-40" value={status || undefined} allowClear placeholder={t("filters.all")} onChange={(value) => { setPage(1); setStatus(value || "") }} options={["pending", "in_progress", "completed"].map((value) => ({ value, label: statusLabel(value) }))} /></label>
          <label className="flex flex-col gap-1 text-xs text-slate-500">{t("filters.reason")}<Select className="min-w-44" value={reason || undefined} allowClear placeholder={t("filters.all")} onChange={(value) => { setPage(1); setReason(value || "") }} options={reasonKeys.map((value) => ({ value, label: reasonLabel(value) }))} /></label>
          <label className="flex min-w-64 flex-1 flex-col gap-1 text-xs text-slate-500">{t("filters.search")}<Input value={search} allowClear placeholder={t("filters.searchPlaceholder")} onChange={(event) => { setPage(1); setSearch(event.target.value) }} /></label>
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

      <StandardModal width={760} title={t("completeModal.title")} open={Boolean(reviewTarget)} onCancel={() => setReviewTarget(null)} onOk={() => void complete()} okText={t("actions.save")} cancelText={t("actions.cancel")}>
        {reviewTarget ? <TicketQualityPanel ticketID={reviewTarget.ticket_id} /> : null}
        <Input.TextArea rows={5} value={note} onChange={(event) => setNote(event.target.value)} placeholder={t("completeModal.placeholder")} />
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
