"use client"

import { useCallback, useEffect, useState } from "react"
import { Loader2Icon, MessageSquareWarningIcon } from "lucide-react"
import { Empty } from "antd"
import { ContentModule, PageShell, RailopsButton, StandardModal } from "@railops/ui"
import { toast } from "sonner"

import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import { translateCurrentMessage } from "@/i18n/messages"
import { useAppLocale } from "@/i18n/provider"
import { TicketQualityReviewDetail, qualityReviewText } from "@/app/enterprise/tickets/_components/ticket-quality-review-detail"
import type { TicketQualityReview } from "@/lib/api/enterprise-tickets"
import { fetchMyTicketQualityReviews, raiseTicketQualityDispute } from "@/lib/api/ticket-quality"

function t(key: string, values?: Record<string, string | number>) {
  return translateCurrentMessage(`ticketQuality.mine.${key}`, values)
}

export default function MyTicketQualityPage() {
  const { locale } = useAppLocale()
  const breadcrumbItems = useRouteBreadcrumbItems()
  const [reviews, setReviews] = useState<TicketQualityReview[]>([])
  const [loading, setLoading] = useState(true)
  const [target, setTarget] = useState<TicketQualityReview | null>(null)
  const [reason, setReason] = useState("")
  const [submitting, setSubmitting] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const response = await fetchMyTicketQualityReviews()
      if (!response.success) throw new Error(response.error?.message || t("loadFailed"))
      setReviews(response.data ?? [])
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : t("loadFailed"))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void load() }, [load])

  async function submitDispute() {
    if (!target) return
    if (!reason.trim()) {
      toast.error(qualityReviewText("disputeFlow.reasonRequired"))
      return
    }
    setSubmitting(true)
    try {
      const response = await raiseTicketQualityDispute(target.id, reason.trim())
      if (!response.success) throw new Error(response.error?.message || qualityReviewText("disputeFlow.failed"))
      toast.success(qualityReviewText("disputeFlow.submitted"))
      setTarget(null)
      setReason("")
      await load()
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : qualityReviewText("disputeFlow.failed"))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <PageShell title={t("title")} description={t("description")} breadcrumb={breadcrumbItems}>
      <ContentModule>
        {loading ? (
          <div className="flex min-h-40 items-center justify-center gap-2 text-sm text-muted-foreground"><Loader2Icon className="size-4 animate-spin" /></div>
        ) : reviews.length === 0 ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("empty")} />
        ) : (
          <div className="divide-y divide-border rounded-md border border-border" data-testid="my-quality-reviews">
            {reviews.map((review) => (
              <div key={review.id}>
                <div className="px-3 pt-3 text-sm font-medium">{review.ticket_no} · {review.ticket_title}</div>
                <TicketQualityReviewDetail
                  review={review}
                  locale={locale}
                  actions={review.can_dispute ? (
                    <div className="flex flex-wrap items-center justify-between gap-2 border-t border-border pt-2">
                      <span className="text-muted-foreground">{review.dispute_deadline ? qualityReviewText("disputeFlow.deadline", { time: new Date(review.dispute_deadline).toLocaleString(locale, { hour12: false }) }) : null}</span>
                      <RailopsButton size="small" onClick={() => { setTarget(review); setReason("") }}><MessageSquareWarningIcon className="size-3.5" />{qualityReviewText("disputeFlow.raise")}</RailopsButton>
                    </div>
                  ) : null}
                />
              </div>
            ))}
          </div>
        )}
      </ContentModule>

      <StandardModal
        width={560}
        title={target ? `${qualityReviewText("disputeFlow.raise")} · ${target.ticket_no}` : ""}
        open={Boolean(target)}
        onCancel={() => setTarget(null)}
        footer={<RailopsButton onClick={() => void submitDispute()} disabled={submitting}>{submitting ? <Loader2Icon className="size-3.5 animate-spin" /> : null}{qualityReviewText("disputeFlow.submit")}</RailopsButton>}
      >
        <label className="block text-xs text-muted-foreground">
          {qualityReviewText("disputeFlow.reason")}
          <textarea aria-label={qualityReviewText("disputeFlow.reason")} className="mt-1 min-h-28 w-full rounded-md border border-border bg-background px-3 py-2 text-sm text-foreground" maxLength={2000} value={reason} onChange={(event) => setReason(event.target.value)} placeholder={qualityReviewText("disputeFlow.reasonPlaceholder")} />
        </label>
      </StandardModal>
    </PageShell>
  )
}
