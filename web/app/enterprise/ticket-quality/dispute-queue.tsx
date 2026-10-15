"use client"

import { useCallback, useEffect, useState } from "react"
import { Loader2Icon, RefreshCwIcon } from "lucide-react"
import { Empty } from "antd"
import { ContentModule, RailopsButton, StandardModal } from "@railops/ui"
import { toast } from "sonner"

import { translateCurrentMessage } from "@/i18n/messages"
import { useAppLocale } from "@/i18n/provider"
import { TicketQualityReviewDetail, effectiveQualityOutcome, qualityResultLabel } from "@/app/enterprise/tickets/_components/ticket-quality-review-detail"
import { decideTicketQualityDispute, fetchTicketQualityDisputes, type TicketQualityDisputeView } from "@/lib/api/ticket-quality"

function t(key: string, values?: Record<string, string | number>) {
  return translateCurrentMessage(`ticketQuality.disputes.${key}`, values)
}

const outcomes = ["pass", "needs_improvement", "fail"] as const

export function TicketQualityDisputeQueue() {
  const { locale } = useAppLocale()
  const [items, setItems] = useState<TicketQualityDisputeView[]>([])
  const [loading, setLoading] = useState(true)
  const [target, setTarget] = useState<TicketQualityDisputeView | null>(null)
  const [decision, setDecision] = useState<"upheld" | "overturned">("upheld")
  const [outcome, setOutcome] = useState("")
  const [note, setNote] = useState("")
  const [submitting, setSubmitting] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const response = await fetchTicketQualityDisputes("open")
      if (!response.success) throw new Error(response.error?.message || t("loadFailed"))
      setItems(response.data ?? [])
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : t("loadFailed"))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void load() }, [load])

  function open(item: TicketQualityDisputeView) {
    setTarget(item)
    setDecision("upheld")
    setOutcome("")
    setNote("")
  }

  async function submit() {
    if (!target) return
    const current = effectiveQualityOutcome(target.review)
    if (!note.trim()) {
      toast.error(t("noteRequired"))
      return
    }
    if (decision === "overturned" && (!outcome || outcome === current)) {
      toast.error(t("outcomeRequired"))
      return
    }
    setSubmitting(true)
    try {
      const response = await decideTicketQualityDispute(target.id, { decision, outcome: decision === "overturned" ? outcome : undefined, note: note.trim() })
      if (!response.success) throw new Error(response.error?.message || t("failed"))
      toast.success(t("decided"))
      setTarget(null)
      await load()
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : t("failed"))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <ContentModule>
      <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="text-sm font-semibold">{t("title")}</div>
          <div className="mt-0.5 text-xs text-slate-500">{t("description")}</div>
        </div>
        <RailopsButton size="small" onClick={() => void load()} disabled={loading}><RefreshCwIcon className={loading ? "size-3.5 animate-spin" : "size-3.5"} /></RailopsButton>
      </div>
      {loading ? (
        <div className="flex min-h-20 items-center justify-center"><Loader2Icon className="size-4 animate-spin text-muted-foreground" /></div>
      ) : items.length === 0 ? (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("empty")} />
      ) : (
        <div className="divide-y divide-border rounded-md border border-border" data-testid="ticket-quality-dispute-queue">
          {items.map((item) => (
            <div key={item.id} className="flex flex-wrap items-center justify-between gap-3 px-3 py-2.5 text-sm">
              <div className="min-w-0">
                <div className="font-medium">{item.ticket_no} · {item.ticket_title}</div>
                <div className="mt-0.5 text-xs text-muted-foreground">
                  {item.raised_by_name || `#${item.raised_by}`} · {t("raisedAt")} {new Date(item.raised_at).toLocaleString(locale, { hour12: false })} · {qualityResultLabel(effectiveQualityOutcome(item.review))}
                </div>
                <div className="mt-1 line-clamp-2 text-xs">{item.reason}</div>
              </div>
              <RailopsButton size="small" onClick={() => open(item)}>{t("review")}</RailopsButton>
            </div>
          ))}
        </div>
      )}

      <StandardModal
        width={760}
        title={target ? t("modalTitle", { ticket: target.ticket_no }) : ""}
        open={Boolean(target)}
        onCancel={() => setTarget(null)}
        footer={<RailopsButton onClick={() => void submit()} disabled={submitting}>{submitting ? <Loader2Icon className="size-3.5 animate-spin" /> : null}{t("submit")}</RailopsButton>}
      >
        {target ? (
          <div className="space-y-3">
            <div className="rounded-md border border-border"><TicketQualityReviewDetail review={target.review} locale={locale} /></div>
            <fieldset className="space-y-2">
              <legend className="text-xs text-muted-foreground">{t("decision")}</legend>
              <div className="flex gap-4 text-sm">
                {(["upheld", "overturned"] as const).map((value) => (
                  <label key={value} className="flex items-center gap-1.5"><input type="radio" name="dispute-decision" checked={decision === value} onChange={() => setDecision(value)} />{t(value)}</label>
                ))}
              </div>
            </fieldset>
            {decision === "overturned" ? (
              <label className="block text-xs text-muted-foreground">
                {t("outcome")}
                <select aria-label={t("outcome")} className="mt-1 h-9 w-full rounded-md border border-border bg-background px-2 text-sm text-foreground" value={outcome} onChange={(event) => setOutcome(event.target.value)}>
                  <option value="" />
                  {outcomes.filter((value) => value !== effectiveQualityOutcome(target.review)).map((value) => <option key={value} value={value}>{qualityResultLabel(value)}</option>)}
                </select>
              </label>
            ) : null}
            <label className="block text-xs text-muted-foreground">
              {t("note")}
              <textarea aria-label={t("note")} className="mt-1 min-h-24 w-full rounded-md border border-border bg-background px-3 py-2 text-sm text-foreground" maxLength={2000} value={note} onChange={(event) => setNote(event.target.value)} placeholder={t("notePlaceholder")} />
            </label>
          </div>
        ) : null}
      </StandardModal>
    </ContentModule>
  )
}
