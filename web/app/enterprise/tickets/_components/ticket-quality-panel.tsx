"use client"

import { useCallback, useEffect, useMemo, useState } from "react"
import {
  AlertTriangleIcon,
  ChevronDownIcon,
  ChevronUpIcon,
  ClipboardCheckIcon,
  HistoryIcon,
  Loader2Icon,
  RotateCcwIcon,
  SaveIcon,
  SparklesIcon,
} from "lucide-react"
import { RailopsButton, StatusTag } from "@railops/ui"

import { useAppLocale } from "@/i18n/provider"
import {
  createTicketQualityReview,
  fetchActiveTicketQualityScorecard,
  fetchTicketQualityAutoAssessment,
  fetchTicketQualityReviews,
  type TicketQualityAutoAssessment,
  type TicketQualityReview,
  type TicketQualityScorecard,
} from "@/lib/api/enterprise-tickets"
import {
  effectiveQualityOutcome,
  qualityCriterionLabel as criterionLabel,
  qualityResultLabel as resultLabel,
  qualityResultTone as resultTone,
  qualityReviewText as qt,
  TicketQualityReviewDetail,
} from "./ticket-quality-review-detail"

type Props = {
  ticketID: number
  sampleID?: number
  onSubmitted?: (review: TicketQualityReview) => Promise<void> | void
}

function qualityResult(total: number, max: number) {
  const passScore = Math.ceil(max * 5 / 6)
  const improvementScore = Math.ceil(max * 5 / 9)
  if (total >= passScore) return "pass"
  if (total >= improvementScore) return "needs_improvement"
  return "fail"
}

export function TicketQualityPanel({ ticketID, sampleID, onSubmitted }: Props) {
  const { locale } = useAppLocale()
  const [scorecard, setScorecard] = useState<TicketQualityScorecard | null>(null)
  const [assessment, setAssessment] = useState<TicketQualityAutoAssessment | null>(null)
  const [reviews, setReviews] = useState<TicketQualityReview[]>([])
  const [answers, setAnswers] = useState<Record<string, number>>({})
  const [remark, setRemark] = useState("")
  const [defectCodes, setDefectCodes] = useState<string[]>([])
  const [evidence, setEvidence] = useState("")
  const [coachingAction, setCoachingAction] = useState("")
  const [showAssessment, setShowAssessment] = useState(true)
  const [scoreDetailsOpen, setScoreDetailsOpen] = useState(false)
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const [historyOpen, setHistoryOpen] = useState(false)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState("")
  const [saved, setSaved] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const [scorecardResponse, reviewsResponse, assessmentResponse] = await Promise.all([
        fetchActiveTicketQualityScorecard(),
        fetchTicketQualityReviews(ticketID),
        fetchTicketQualityAutoAssessment(ticketID),
      ])
      if (!scorecardResponse.success || !reviewsResponse.success || !assessmentResponse.success) throw new Error(qt("loadFailed"))
      const reviewItems = reviewsResponse.data ?? []
      setScorecard(scorecardResponse.data)
      setReviews(reviewItems)
      setAssessment(assessmentResponse.data)
      setAnswers(assessmentResponse.data.answers)
      setDefectCodes(assessmentResponse.data.defect_codes)
      setEvidence("")
      setRemark("")
      setCoachingAction("")
      setShowAssessment(reviewItems.length === 0)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : qt("loadFailed"))
    } finally {
      setLoading(false)
    }
  }, [ticketID])

  useEffect(() => { void load() }, [load])

  const total = useMemo(() => Object.values(answers).reduce((sum, value) => sum + value, 0), [answers])
  const max = scorecard?.items.reduce((sum, item) => sum + item.max, 0) ?? assessment?.max_score ?? 18
  const result = qualityResult(total, max)
  const latestReview = reviews[0]
  const scorecardCodes = useMemo(() => new Set(scorecard?.items.map((item) => item.code) ?? []), [scorecard])
  // Only codes from the active scorecard are accepted by the server.
  const selectedDefects = defectCodes.filter((code) => scorecardCodes.has(code))
  const needsIssues = result !== "pass"
  const toggleDefect = (code: string) => setDefectCodes((current) => (current.includes(code) ? current.filter((value) => value !== code) : [...current, code]))
  const generatedEvidence = useMemo(() => {
    // The translator reads the active catalog globally; consuming locale keeps generated evidence reactive.
    void locale
    return assessment?.items.map((item) => {
      const label = criterionLabel(item.code, item.title)
      return `${label}: ${qt(`evidence.${item.evidence_key}`)}`
    }).join("\n") ?? ""
  }, [assessment, locale])

  async function submit() {
    if (!scorecard) return
    const finalEvidence = evidence.trim() || generatedEvidence
    if (!finalEvidence) {
      setError(qt("evidenceRequired"))
      return
    }
    // A result below pass must say what went wrong and how to improve.
    if (result !== "pass" && selectedDefects.length === 0) {
      setError(qt("defectRequired"))
      return
    }
    if (result !== "pass" && (!coachingAction.trim() || coachingAction.trim() === qt("noCoaching"))) {
      setError(qt("coachingRequired"))
      return
    }
    setSaving(true)
    setError("")
    setSaved(false)
    try {
      const response = await createTicketQualityReview(ticketID, {
        sample_id: sampleID,
        answers,
        remark,
        defect_codes: selectedDefects,
        evidence: finalEvidence,
        outcome: result,
        coaching_action: coachingAction.trim(),
      })
      if (!response.success) throw new Error(response.error?.message || qt("saveFailed"))
      setSaved(true)
      await load()
      await onSubmitted?.(response.data)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : qt("saveFailed"))
    } finally {
      setSaving(false)
    }
  }

  function renderIssuesFields() {
    return (
      <fieldset className="space-y-3 rounded-md border border-border p-3" data-testid="ticket-quality-issues">
        <legend className="px-1 text-sm font-medium">{qt("issuesAndCoaching")}{needsIssues ? <span className="ml-1 text-xs text-red-600">*{qt("required")}</span> : null}</legend>
        <div>
          <div className="text-xs text-muted-foreground">{qt("defectTypes")}</div>
          <div className="mt-1 grid gap-1 sm:grid-cols-3">
            {scorecard?.items.map((item) => (
              <label key={item.code} className="flex items-center gap-2 text-sm">
                <input type="checkbox" checked={selectedDefects.includes(item.code)} onChange={() => toggleDefect(item.code)} />
                {criterionLabel(item.code, item.title)}
              </label>
            ))}
          </div>
        </div>
        <label className="block text-xs text-muted-foreground">{qt("coaching")}<textarea className="mt-1 min-h-16 w-full rounded-md border border-border bg-background px-3 py-2 text-sm text-foreground" value={coachingAction} onChange={(event) => setCoachingAction(event.target.value)} placeholder={needsIssues ? qt("coachingPlaceholder") : qt("noCoaching")} /></label>
      </fieldset>
    )
  }

  if (loading) {
    return <div className="flex min-h-40 items-center justify-center gap-2 text-sm text-muted-foreground"><Loader2Icon className="size-4 animate-spin" />{qt("loading")}</div>
  }

  return (
    <section className="space-y-4" data-testid="ticket-quality-panel">
      <div className="flex flex-wrap items-start justify-between gap-3 border-b border-border pb-3">
        <div className="flex items-start gap-2.5">
          <ClipboardCheckIcon className="mt-0.5 size-4" />
          <div>
            <h3 className="text-sm font-semibold">{qt("title")}</h3>
            <p className="mt-0.5 text-xs text-muted-foreground">{qt("subtitle", { version: scorecard?.version ?? "-" })}</p>
          </div>
        </div>
        {assessment ? <span className="inline-flex items-center gap-1 rounded border border-blue-200 bg-blue-50 px-2 py-1 text-xs text-blue-700"><SparklesIcon className="size-3.5" />{qt("ruleBadge")}</span> : null}
      </div>

      {error ? <div className="flex items-start gap-2 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700"><AlertTriangleIcon className="mt-0.5 size-4 shrink-0" />{error}</div> : null}

      {latestReview && !showAssessment ? (
        <div className="space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-4 rounded-md border border-border bg-muted/30 px-4 py-3">
            <div>
              <div className="text-xs text-muted-foreground">{qt("latestResult")}</div>
              <div className="mt-1 flex items-center gap-2">
                <span className="text-2xl font-semibold tabular-nums">{latestReview.total_score}/{latestReview.max_score}</span>
                <StatusTag tone={resultTone(effectiveQualityOutcome(latestReview))}>{resultLabel(effectiveQualityOutcome(latestReview))}</StatusTag>
              </div>
              <div className="mt-1 text-xs text-muted-foreground">{new Date(latestReview.reviewed_at).toLocaleString(locale, { hour12: false })}</div>
            </div>
            <RailopsButton size="small" onClick={() => setShowAssessment(true)}><RotateCcwIcon className="size-3.5" />{qt("reassess")}</RailopsButton>
          </div>
          <button type="button" className="flex w-full items-center justify-between border-b border-border py-2 text-left text-sm font-medium" onClick={() => setHistoryOpen((value) => !value)}>
            <span className="inline-flex items-center gap-2"><HistoryIcon className="size-4" />{qt("history", { count: reviews.length })}</span>
            {historyOpen ? <ChevronUpIcon className="size-4" /> : <ChevronDownIcon className="size-4" />}
          </button>
          {historyOpen ? <ReviewHistory reviews={reviews} locale={locale} /> : null}
        </div>
      ) : null}

      {showAssessment && scorecard ? (
        <div className="space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-4 rounded-md border border-border bg-muted/30 px-4 py-3">
            <div>
              <div className="text-xs text-muted-foreground">{qt("autoScore")}</div>
              <div className="mt-1 flex items-center gap-2">
                <span className="text-2xl font-semibold tabular-nums">{total}/{max}</span>
                <StatusTag tone={resultTone(result)}>{resultLabel(result)}</StatusTag>
              </div>
            </div>
            <div className="text-right text-xs text-muted-foreground">
              <div>{qt("reviewRequired", { count: assessment?.review_required_count ?? 0 })}</div>
              <div className="mt-1">{qt("autoDescription")}</div>
            </div>
          </div>

          <button type="button" className="flex w-full items-center justify-between border-b border-border py-2 text-left text-sm font-medium" onClick={() => setScoreDetailsOpen((value) => !value)}>
            <span>{qt("scoreDetails")}</span>
            {scoreDetailsOpen ? <ChevronUpIcon className="size-4" /> : <ChevronDownIcon className="size-4" />}
          </button>
          {scoreDetailsOpen ? (
            <div className="divide-y divide-border rounded-md border border-border">
              {scorecard.items.map((item) => {
                const automatic = assessment?.items.find((value) => value.code === item.code)
                return (
                  <div key={item.code} className="grid gap-2 px-3 py-2.5 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2 text-sm font-medium">
                        <span>{criterionLabel(item.code, item.title)}</span>
                        {automatic?.requires_review ? <span className="rounded bg-amber-50 px-1.5 py-0.5 text-[11px] text-amber-700">{qt("needsConfirmation")}</span> : null}
                      </div>
                      <p className="mt-0.5 text-xs text-muted-foreground">{automatic ? qt(`evidence.${automatic.evidence_key}`) : qt("noEvidence")}</p>
                    </div>
                    <div className="inline-grid grid-cols-3 gap-1" aria-label={criterionLabel(item.code, item.title)}>
                      {[0, 1, 2].map((score) => (
                        <button key={score} type="button" className={`size-8 rounded border text-sm tabular-nums transition ${answers[item.code] === score ? "border-primary bg-primary text-primary-foreground" : "border-border bg-background hover:bg-muted"}`} onClick={() => setAnswers((current) => ({ ...current, [item.code]: score }))}>{score}</button>
                      ))}
                    </div>
                  </div>
                )
              })}
            </div>
          ) : null}

          <label className="block text-xs text-muted-foreground">
            {qt("remark")}
            <textarea className="mt-1 min-h-16 w-full rounded-md border border-border bg-background px-3 py-2 text-sm text-foreground" placeholder={qt("remarkPlaceholder")} value={remark} onChange={(event) => setRemark(event.target.value)} />
          </label>

          {needsIssues ? renderIssuesFields() : null}

          <button type="button" className="flex w-full items-center justify-between border-b border-border py-2 text-left text-sm font-medium" onClick={() => setAdvancedOpen((value) => !value)}>
            <span>{qt("advanced")}</span>
            {advancedOpen ? <ChevronUpIcon className="size-4" /> : <ChevronDownIcon className="size-4" />}
          </button>
          {advancedOpen ? (
            <div className="grid gap-3">
              {needsIssues ? null : renderIssuesFields()}
              <label className="text-xs text-muted-foreground">{qt("evidenceLabel")}<textarea className="mt-1 min-h-24 w-full rounded-md border border-border bg-background px-3 py-2 text-sm text-foreground" value={evidence} onChange={(event) => setEvidence(event.target.value)} placeholder={generatedEvidence || qt("evidencePlaceholder")} /></label>
            </div>
          ) : null}

          <div className="flex flex-wrap items-center justify-between gap-3 border-t border-border pt-3">
            <span className="text-xs text-muted-foreground">{saved ? qt("saved") : qt("saveHint")}</span>
            <div className="flex gap-2">
              {latestReview ? <RailopsButton size="small" onClick={() => setShowAssessment(false)}>{qt("cancelReassess")}</RailopsButton> : null}
              <RailopsButton size="small" onClick={() => void submit()} disabled={saving}>{saving ? <Loader2Icon className="size-3.5 animate-spin" /> : <SaveIcon className="size-3.5" />}{qt("confirmSave")}</RailopsButton>
            </div>
          </div>
        </div>
      ) : null}
    </section>
  )
}

function ReviewHistory({ reviews, locale }: { reviews: TicketQualityReview[]; locale: string }) {
  return (
    <div className="divide-y divide-border rounded-md border border-border">
      {reviews.map((review) => <TicketQualityReviewDetail key={review.id} review={review} locale={locale} />)}
    </div>
  )
}
