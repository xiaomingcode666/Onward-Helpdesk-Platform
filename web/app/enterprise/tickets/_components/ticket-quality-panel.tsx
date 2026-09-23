"use client"

import { useCallback, useEffect, useMemo, useState } from "react"
import { ClipboardCheckIcon, Loader2Icon, SaveIcon } from "lucide-react"
import {
  createTicketQualityReview,
  fetchActiveTicketQualityScorecard,
  fetchTicketQualityReviews,
  type TicketQualityReview,
  type TicketQualityScorecard,
} from "@/lib/api/enterprise-tickets"

type Props = { ticketID: number }

function resultLabel(result: string) {
  if (result === "pass") return "通过"
  if (result === "needs_improvement") return "需改进"
  return "不通过"
}

export function TicketQualityPanel({ ticketID }: Props) {
  const [scorecard, setScorecard] = useState<TicketQualityScorecard | null>(null)
  const [reviews, setReviews] = useState<TicketQualityReview[]>([])
  const [answers, setAnswers] = useState<Record<string, number>>({})
  const [remark, setRemark] = useState("")
  const [defectCodes, setDefectCodes] = useState("")
  const [evidence, setEvidence] = useState("")
  const [disputeStatus, setDisputeStatus] = useState("none")
  const [disputeNote, setDisputeNote] = useState("")
  const [coachingAction, setCoachingAction] = useState("无需辅导")
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState("")
  const [saved, setSaved] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const [scorecardResponse, reviewsResponse] = await Promise.all([
        fetchActiveTicketQualityScorecard(),
        fetchTicketQualityReviews(ticketID),
      ])
      setScorecard(scorecardResponse.data)
      setReviews(reviewsResponse.data ?? [])
      const initial: Record<string, number> = {}
      for (const item of scorecardResponse.data.items) initial[item.code] = 0
      setAnswers(initial)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "质量评估加载失败")
    } finally {
      setLoading(false)
    }
  }, [ticketID])

  useEffect(() => { void load() }, [load])

  const total = useMemo(() => Object.values(answers).reduce((sum, value) => sum + value, 0), [answers])
  const max = scorecard?.items.reduce((sum, item) => sum + item.max, 0) ?? 18
  const result = total >= 15 ? "通过" : total >= 10 ? "需改进" : "不通过"

  async function submit() {
    if (!scorecard) return
    if (!evidence.trim()) {
      setError("请填写证据")
      return
    }
    setSaving(true)
    setError("")
    setSaved(false)
    try {
      await createTicketQualityReview(ticketID, {
        answers,
        remark,
        defect_codes: defectCodes.split(",").map((value) => value.trim()).filter(Boolean),
        evidence,
        dispute_status: disputeStatus,
        dispute_note: disputeNote,
        outcome: result === "通过" ? "pass" : result === "需改进" ? "needs_improvement" : "fail",
        coaching_action: coachingAction,
      })
      setSaved(true)
      await load()
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "质量评估提交失败")
    } finally {
      setSaving(false)
    }
  }

  return (
    <section className="rounded-lg border border-border bg-card p-4" data-testid="ticket-quality-panel">
      <div className="mb-3 flex items-center gap-2">
        <ClipboardCheckIcon className="size-4" />
        <div>
          <h3 className="text-sm font-semibold">质量评估</h3>
          <p className="text-xs text-muted-foreground">统一评分表，当前版本 {scorecard?.version ?? "-"}</p>
        </div>
      </div>
      {loading ? <div className="flex items-center gap-2 text-sm text-muted-foreground"><Loader2Icon className="size-4 animate-spin" />加载中...</div> : null}
      {!loading && error ? <p className="text-sm text-destructive">{error}</p> : null}
      {!loading && scorecard ? (
        <>
          <div className="space-y-2">
            {scorecard.items.map((item) => (
              <label key={item.code} className="flex items-center justify-between gap-3 rounded-md bg-muted/45 px-3 py-2 text-sm">
                <span>{item.title}</span>
                <select
                  className="h-8 rounded-md border border-border bg-background px-2 text-sm"
                  value={answers[item.code] ?? 0}
                  onChange={(event) => setAnswers((current) => ({ ...current, [item.code]: Number(event.target.value) }))}
                  aria-label={item.title}
                >
                  <option value={0}>0</option><option value={1}>1</option><option value={2}>2</option>
                </select>
              </label>
            ))}
          </div>
          <textarea
            className="mt-3 min-h-20 w-full rounded-md border border-border bg-background px-3 py-2 text-sm"
            placeholder="评估备注（可选）"
            value={remark}
            onChange={(event) => setRemark(event.target.value)}
          />
          <input
            className="mt-3 h-9 w-full rounded-md border border-border bg-background px-3 text-sm"
            placeholder="缺陷码，多个用逗号分隔（可选）"
            value={defectCodes}
            onChange={(event) => setDefectCodes(event.target.value)}
          />
          <textarea
            className="mt-3 min-h-20 w-full rounded-md border border-border bg-background px-3 py-2 text-sm"
            placeholder="证据（必填：对话、工单记录或附件依据）"
            value={evidence}
            onChange={(event) => setEvidence(event.target.value)}
          />
          <div className="mt-3 grid gap-2 sm:grid-cols-2">
            <label className="flex flex-col gap-1 text-xs text-muted-foreground">
              争议状态
              <select className="h-9 rounded-md border border-border bg-background px-2 text-sm text-foreground" value={disputeStatus} onChange={(event) => setDisputeStatus(event.target.value)}>
                <option value="none">无争议</option><option value="open">待处理</option><option value="resolved">已解决</option>
              </select>
            </label>
            <input className="mt-5 h-9 rounded-md border border-border bg-background px-3 text-sm" placeholder="辅导动作" value={coachingAction} onChange={(event) => setCoachingAction(event.target.value)} />
          </div>
          {disputeStatus !== "none" ? <textarea className="mt-3 min-h-16 w-full rounded-md border border-border bg-background px-3 py-2 text-sm" placeholder="争议说明" value={disputeNote} onChange={(event) => setDisputeNote(event.target.value)} /> : null}
          <div className="mt-3 flex items-center justify-between gap-3">
            <span className="text-sm font-medium">总分 {total}/{max} · {result}</span>
            <button type="button" className="inline-flex h-8 items-center gap-1.5 rounded-md bg-primary px-3 text-sm text-primary-foreground disabled:opacity-50" onClick={() => void submit()} disabled={saving}>
              {saving ? <Loader2Icon className="size-3.5 animate-spin" /> : <SaveIcon className="size-3.5" />}
              提交评估
            </button>
          </div>
          {saved ? <p className="mt-2 text-xs text-emerald-600">已保存</p> : null}
          {reviews.length > 0 ? (
            <div className="mt-4 border-t border-border pt-3">
              <div className="mb-2 text-xs font-semibold text-muted-foreground">历史评估</div>
              <div className="space-y-1.5">
                {reviews.map((review) => <div key={review.id} className="flex justify-between gap-3 text-xs"><span>{new Date(review.reviewed_at).toLocaleString()} · {resultLabel(review.result)}</span><span className="font-medium">{review.total_score}/{review.max_score} · {review.remark || "无备注"}</span></div>)}
              </div>
            </div>
          ) : null}
        </>
      ) : null}
    </section>
  )
}
