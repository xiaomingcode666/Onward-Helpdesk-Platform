"use client"

import { useCallback, useEffect, useRef, useState } from "react"
import { Input } from "antd"
import { RailopsButton } from "@railops/ui"
import { apiGet, apiPost } from "@/lib/api/client"
import { readSession } from "@/lib/auth"

type Incoming = { id: number; from: string; subject: string; body: string; received_at: string; attachment_count: number }
type Reply = { id: number; recipient: string; body: string; status: string; created_at: string }
type History = { incoming: Incoming[]; replies: Reply[] }
const labels: Record<string, string> = { queued: "等待发送", sending: "正在发送", sent: "已提交邮件服务器", failed: "发送失败，请检查邮箱配置后重新发送", unknown: "发送结果不确定，请先到邮箱核对，避免重复发送" }

export function TicketEmailPanel({ ticketID }: { ticketID: string | number }) {
  const [history, setHistory] = useState<History>({ incoming: [], replies: [] })
  const [body, setBody] = useState("")
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
  const request = useRef<{ key: string; body: string } | null>(null)
  const canReply = readSession()?.permissions?.includes("ticket.progress") ?? false
  const load = useCallback(async () => {
    try {
      const result = await apiGet<History>(`/tickets/${ticketID}/emails`)
      if (!result.success || !result.data) { setError(result.error?.message || "邮件记录读取失败"); return }
      setHistory(result.data)
    } catch { setError("邮件记录读取失败，请刷新重试") }
  }, [ticketID])
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
        setError(result.error?.message || "提交失败，请重试"); return
      }
      request.current = null; setBody(""); await load()
    } catch { setError("提交结果不确定，请点击发送重试；系统会避免重复提交") }
    finally { setBusy(false) }
  }
  return <section className="space-y-3 rounded-lg border p-4" aria-label="邮件往来">
    <div className="flex items-center justify-between"><h3 className="font-medium">邮件往来</h3><RailopsButton onClick={() => void load()}>刷新</RailopsButton></div>
    <p className="text-xs text-muted-foreground">回复会发给最初来信的客户。客户在邮箱点击“回复”后，内容会自动回到本工单。附件暂不导入。</p>
    <div className="max-h-80 space-y-3 overflow-y-auto">
      {[...history.incoming.map(m => ({ key: `in-${m.id}`, date: m.received_at, title: `客户来信 · ${m.from}`, body: m.body, note: m.attachment_count ? `有 ${m.attachment_count} 个附件，请到原邮箱查看` : "" })), ...history.replies.map(m => ({ key: `out-${m.id}`, date: m.created_at, title: `客服回信 · ${m.recipient}`, body: m.body, note: labels[m.status] || m.status }))].sort((a, b) => a.date.localeCompare(b.date)).map(m => <article key={m.key} className="rounded border p-3 text-sm"><p className="font-medium">{m.title}</p><time className="text-xs text-muted-foreground">{new Date(m.date).toLocaleString()}</time><p className="mt-2 whitespace-pre-wrap break-words">{m.body}</p>{m.note && <p className="mt-2 text-xs">{m.note}</p>}</article>)}
    </div>
    {error && <p role="alert" className="text-sm text-red-600">{error}</p>}
    {canReply && <><Input.TextArea aria-label="邮件回复内容" placeholder="输入给客户的回复" value={body} maxLength={16000} rows={3} disabled={busy || !!request.current} onChange={e => setBody(e.target.value)} /><RailopsButton disabled={busy || !body.trim()} onClick={() => void send()}>{busy ? "正在提交…" : "发送邮件回复"}</RailopsButton></>}
  </section>
}
