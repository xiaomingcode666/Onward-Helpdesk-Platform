"use client"

import type { ReactNode } from "react"
import { LockIcon, MessageSquareWarningIcon } from "lucide-react"
import { StatusTag, type StatusTagTone } from "@railops/ui"

import { translateCurrentMessage } from "@/i18n/messages"
import type { TicketQualityReview } from "@/lib/api/enterprise-tickets"
import { localizeTicketQualityEvidence } from "@/lib/ticket-quality-i18n"

export function qualityReviewText(key: string, values?: Record<string, string | number>) {
  return translateCurrentMessage(`ticketQuality.review.${key}`, values)
}

export function qualityResultTone(result: string): StatusTagTone {
  if (result === "pass") return "success"
  if (result === "needs_improvement") return "warning"
  return "error"
}

export function qualityResultLabel(result: string) {
  if (result === "pass") return qualityReviewText("result.pass")
  if (result === "needs_improvement") return qualityReviewText("result.needsImprovement")
  return qualityReviewText("result.fail")
}

export function qualityCriterionLabel(code: string, fallback: string) {
  const value = qualityReviewText(`criteria.${code}`)
  return value.includes(`criteria.${code}`) ? fallback : value
}

/** The outcome that counts: an overturned dispute replaces the original one. */
export function effectiveQualityOutcome(review: Pick<TicketQualityReview, "final_outcome" | "outcome" | "result">) {
  return review.final_outcome || review.outcome || review.result
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="min-w-0">
      <div className="text-[11px] text-muted-foreground">{label}</div>
      <div className="mt-0.5 whitespace-pre-wrap break-words leading-5 text-xs">{children}</div>
    </div>
  )
}

function disputeStatusLabel(status: string) {
  if (status === "upheld") return qualityReviewText("disputeFlow.upheld")
  if (status === "overturned") return qualityReviewText("disputeFlow.overturned")
  return qualityReviewText("disputeFlow.open")
}

export function TicketQualityReviewDetail({ review, locale, actions }: { review: TicketQualityReview; locale: string; actions?: ReactNode }) {
  const finalOutcome = effectiveQualityOutcome(review)
  const originalOutcome = review.outcome || review.result
  const formatTime = (value?: string) => (value ? new Date(value).toLocaleString(locale, { hour12: false }) : "-")
  const itemTitles = new Map(review.score_items?.map((item) => [item.code, item.title]) ?? [])
  return (
    <div className="space-y-3 px-3 py-3 text-xs" data-testid="ticket-quality-review-detail">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-base font-semibold tabular-nums">{review.total_score}/{review.max_score}</span>
          <StatusTag tone={qualityResultTone(finalOutcome)}>{qualityResultLabel(finalOutcome)}</StatusTag>
          {finalOutcome !== originalOutcome ? <span className="text-muted-foreground">{qualityReviewText("detail.originalOutcome")}: {qualityResultLabel(originalOutcome)}</span> : null}
        </div>
        <span className="text-muted-foreground">{formatTime(review.reviewed_at)}</span>
      </div>

      {review.detail_restricted ? (
        <p className="flex items-start gap-1.5 text-muted-foreground"><LockIcon className="mt-0.5 size-3.5 shrink-0" />{qualityReviewText("detail.restricted")}</p>
      ) : (
        <>
          <div className="grid gap-2 sm:grid-cols-2">
            <Field label={qualityReviewText("detail.reviewer")}>{review.reviewer_name || `#${review.reviewer_id}`}</Field>
            <Field label={qualityReviewText("detail.agent")}>{review.reviewed_agent_name || (review.reviewed_agent_id ? `#${review.reviewed_agent_id}` : "-")}</Field>
          </div>

          {review.score_items?.length ? (
            <div>
              <div className="text-[11px] text-muted-foreground">{qualityReviewText("detail.scoreItems")}</div>
              <div className="mt-1 grid gap-1 sm:grid-cols-2">
                {review.score_items.map((item) => {
                  const lost = item.max - item.score
                  return (
                    <div key={item.code} className={`flex items-center justify-between gap-2 rounded border px-2 py-1 ${lost > 0 ? "border-amber-200 bg-amber-50 text-amber-800" : "border-border"}`} data-deducted={lost > 0 ? "true" : undefined}>
                      <span className="min-w-0 truncate">{qualityCriterionLabel(item.code, item.title)}</span>
                      <span className="shrink-0 tabular-nums">{item.score}/{item.max}{lost > 0 ? ` · ${qualityReviewText("detail.deducted", { points: lost })}` : ""}</span>
                    </div>
                  )
                })}
              </div>
            </div>
          ) : null}

          <Field label={qualityReviewText("detail.defects")}>
            {review.defect_codes?.length ? review.defect_codes.map((code) => qualityCriterionLabel(code, itemTitles.get(code) ?? code)).join("、") : qualityReviewText("detail.none")}
          </Field>
          <Field label={qualityReviewText("detail.evidence")}>{localizeTicketQualityEvidence(review.evidence, locale) || "-"}</Field>
          <Field label={qualityReviewText("detail.coaching")}>{review.coaching_action || "-"}</Field>
          {review.remark ? <Field label={qualityReviewText("detail.remark")}>{review.remark}</Field> : null}

          {review.dispute ? (
            <div className="space-y-2 rounded-md border border-border bg-muted/30 p-2.5">
              <div className="flex items-center gap-1.5 font-medium"><MessageSquareWarningIcon className="size-3.5" />{qualityReviewText("disputeFlow.title")} · {disputeStatusLabel(review.dispute.status)}</div>
              <Field label={`${qualityReviewText("disputeFlow.reason")} · ${review.dispute.raised_by_name || `#${review.dispute.raised_by}`} · ${formatTime(review.dispute.raised_at)}`}>{review.dispute.reason}</Field>
              {review.dispute.status !== "open" ? (
                <Field label={`${qualityReviewText("disputeFlow.decision")} · ${review.dispute.decided_by_name || `#${review.dispute.decided_by}`} · ${formatTime(review.dispute.decided_at)}`}>
                  {review.dispute.decision_note}
                  {review.dispute.status === "overturned" ? ` → ${qualityResultLabel(review.dispute.decided_outcome)}` : ""}
                </Field>
              ) : null}
            </div>
          ) : null}
        </>
      )}
      {actions}
    </div>
  )
}
