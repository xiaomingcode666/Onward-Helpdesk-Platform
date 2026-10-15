"use client"

import { useCallback, useEffect, useRef, useState } from "react"
import { Input } from "antd"
import { RailopsButton } from "@railops/ui"
import { useI18n } from "@/i18n/provider"
import { apiGet, apiPost } from "@/lib/api/client"
import { readSession } from "@/lib/auth"
import { formatDateTime } from "@/lib/utils"

type Incoming = { id: number; from: string; subject: string; body: string; received_at: string; attachment_count: number }
type Reply = { id: number; recipient: string; body: string; status: string; created_at: string }
type History = { incoming: Incoming[]; replies: Reply[] }
const replyStatuses = new Set(["queued", "sending", "sent", "failed", "unknown"])

export function TicketEmailPanel({ ticketID }: { ticketID: string | number }) {
  const t = useI18n()
  const [history, setHistory] = useState<History>({ incoming: [], replies: [] })
  const [body, setBody] = useState("")
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
  const request = useRef<{ key: string; body: string } | null>(null)
  const canReply = readSession()?.permissions?.includes("ticket.progress") ?? false
  const load = useCallback(async () => {
    try {
      const result = await apiGet<History>(`/tickets/${ticketID}/emails`)
      if (!result.success || !result.data) { setError(result.error?.message || t("ticketEmail.loadFailed")); return }
      setHistory(result.data)
    } catch { setError(t("ticketEmail.loadFailedRetry")) }
  }, [t, ticketID])
  useEffect(() => {
    const timer = window.setTimeout(() => void load(), 0)
    const refresh = window.setInterval(() => void load(), 5000)
    return () => { window.clearTimeout(timer); window.clearInterval(refresh) }
  }, [load])
  const send = async () => {
    if (!body.trim() || busy) return
    if (!request.current) request.current = { key: crypto.randomUUID(), body: body.trim() }
    setBusy(true); setError("")
    try {
      const result = await apiPost<Reply>(`/tickets/${ticketID}/email-replies`, { body: request.current.body, request_key: request.current.key })
      if (!result.success) {
        if (result.error?.code !== "NETWORK_ERROR" && result.error?.code !== "REQUEST_TIMEOUT") request.current = null
        setError(result.error?.message || t("ticketEmail.sendFailed")); return
      }
      request.current = null; setBody(""); await load()
    } catch { setError(t("ticketEmail.sendUncertain")) }
    finally { setBusy(false) }
  }
  const replyStatusLabel = (status: string) => replyStatuses.has(status) ? t(`ticketEmail.status.${status}`) : status
  return <section className="space-y-3 rounded-lg border p-4" aria-label={t("ticketEmail.title")}>
    <div className="flex items-center justify-between"><h3 className="font-medium">{t("ticketEmail.title")}</h3><RailopsButton onClick={() => void load()}>{t("ticketEmail.refresh")}</RailopsButton></div>
    <p className="text-xs text-muted-foreground">{t("ticketEmail.hint")}</p>
    <div className="max-h-80 space-y-3 overflow-y-auto">
      {[...history.incoming.map(m => ({ key: `in-${m.id}`, date: m.received_at, title: t("ticketEmail.incomingTitle", { from: m.from }), body: m.body, note: m.attachment_count ? t("ticketEmail.attachments", { count: m.attachment_count }) : "" })), ...history.replies.map(m => ({ key: `out-${m.id}`, date: m.created_at, title: t("ticketEmail.replyTitle", { recipient: m.recipient }), body: m.body, note: replyStatusLabel(m.status) }))].sort((a, b) => a.date.localeCompare(b.date)).map(m => <article key={m.key} className="rounded border p-3 text-sm"><p className="font-medium">{m.title}</p><time className="text-xs text-muted-foreground">{formatDateTime(m.date)}</time><p className="mt-2 whitespace-pre-wrap break-words">{m.body}</p>{m.note && <p className="mt-2 text-xs">{m.note}</p>}</article>)}
    </div>
    {error && <p role="alert" className="text-sm text-red-600">{error}</p>}
    {canReply && <><Input.TextArea aria-label={t("ticketEmail.replyLabel")} placeholder={t("ticketEmail.replyPlaceholder")} value={body} maxLength={16000} rows={3} disabled={busy || !!request.current} onChange={e => setBody(e.target.value)} /><RailopsButton disabled={busy || !body.trim()} onClick={() => void send()}>{busy ? t("ticketEmail.sending") : t("ticketEmail.send")}</RailopsButton></>}
  </section>
}
