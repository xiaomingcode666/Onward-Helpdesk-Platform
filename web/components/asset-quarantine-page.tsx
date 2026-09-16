"use client"

import { useCallback, useEffect, useState } from "react"
import { RefreshCwIcon, ShieldCheckIcon } from "lucide-react"
import { toast } from "sonner"

import { DashboardPage, DashboardToolbar } from "@/components/dashboard-page"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import { useI18n } from "@/i18n/provider"
import { request as apiRequest } from "@/lib/api/client"

type Attachment = {
  id: number; filename: string; fileSize: number; source: string; scanStatus: string
  scanReason: string; receiveComplete: boolean; createdAt: string; uploadedBy: string; sha256: string
}
type ScanAttempt = {
  id: number; status: string; reason: string; detail: string; createdAt: string
  engineVersion: string; databaseVersion: string; scanPerformed: boolean
}

export function AssetQuarantinePage({ domain = "enterprise" }: { domain?: "enterprise" | "platform" }) {
  const t = useI18n()
  const base = domain === "enterprise" ? "/api/enterprise/v1/asset-quarantine" : "/api/platform/asset-quarantine"
  const [items, setItems] = useState<Attachment[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [status, setStatus] = useState("quarantined")
  const [search, setSearch] = useState("")
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
  const [rescanning, setRescanning] = useState<number | null>(null)
  const [selected, setSelected] = useState<Attachment | null>(null)
  const [attempts, setAttempts] = useState<ScanAttempt[]>([])
  const [attemptsBusy, setAttemptsBusy] = useState(false)
  const [attemptsError, setAttemptsError] = useState("")
  const [historyMore, setHistoryMore] = useState(false)

  const load = useCallback(async (signal?: AbortSignal) => {
    setBusy(true)
    setError("")
    try {
      const query = new URLSearchParams({ page: String(page), status, search, page_size: "20" })
      const result = await apiRequest<{ items: Attachment[]; total: number }>(`${base}?${query}`, { signal })
      if (!signal?.aborted) { setItems(result.items); setTotal(result.total) }
    } catch (err) {
      if (!signal?.aborted) { setItems([]); setTotal(0); setError(err instanceof Error ? err.message : String(err)) }
    } finally { if (!signal?.aborted) setBusy(false) }
  }, [base, page, status, search])

  useEffect(() => {
    const controller = new AbortController()
    const timer = setTimeout(() => { void load(controller.signal) }, 200)
    return () => { clearTimeout(timer); controller.abort() }
  }, [load])

  async function showAttempts(item: Attachment, before?: number) {
    setSelected(item)
    if (!before) setAttempts([])
    setAttemptsBusy(true)
    setAttemptsError("")
    try {
      const result = await apiRequest<ScanAttempt[]>(`${base}/${item.id}/attempts${before ? `?before=${before}` : ""}`)
      setAttempts(previous => before ? [...previous, ...result] : result)
      setHistoryMore(result.length === 50)
    } catch (err) { setAttemptsError(err instanceof Error ? err.message : String(err)) }
    finally { setAttemptsBusy(false) }
  }

  async function rescan(item: Attachment) {
    setRescanning(item.id)
    try {
      await apiRequest(`${base}/${item.id}/rescan`, { method: "POST", timeoutMs: 60000 })
      toast.success(t("assetQuarantine.released"))
    } catch (err) { toast.error(err instanceof Error ? err.message : String(err)) }
    finally {
      setRescanning(null)
      await load()
      if (selected?.id === item.id) await showAttempts(item)
    }
  }

  return (
    <DashboardPage>
      <DashboardToolbar actions={<Button variant="outline" disabled={busy} onClick={() => void load()}><RefreshCwIcon />{t("assetQuarantine.refresh")}</Button>}>
        <div><h1 className="flex items-center gap-2 text-xl font-semibold"><ShieldCheckIcon />{t("assetQuarantine.title")}</h1><p className="mt-1 text-sm text-muted-foreground">{t("assetQuarantine.description")}</p></div>
      </DashboardToolbar>
      <div className="flex flex-wrap gap-3">
        <Input className="max-w-sm" aria-label={t("assetQuarantine.search")} placeholder={t("assetQuarantine.search")} value={search} onChange={event => { setSearch(event.target.value); setPage(1) }} />
        <select className="h-9 rounded-md border bg-background px-3 text-sm" aria-label={t("assetQuarantine.status")} value={status} onChange={event => { setStatus(event.target.value); setPage(1); setSelected(null) }}>
          {["quarantined", "pending", "unscanned", "clean", "all"].map(value => <option key={value} value={value}>{t(`assetQuarantine.statuses.${value}`)}</option>)}
        </select>
      </div>
      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
      <div className="overflow-x-auto rounded-md border" aria-busy={busy}>
        <table className="w-full text-left text-sm">
          <thead className="bg-muted/50"><tr>{["file", "source", "uploadedBy", "time", "status", "reason", "actions"].map(key => <th key={key} className="whitespace-nowrap p-3 font-medium">{t(`assetQuarantine.${key}`)}</th>)}</tr></thead>
          <tbody>
            {items.map(item => <tr key={item.id} className="border-t align-top">
              <td className="max-w-xs break-words p-3"><p>{item.filename}</p><p className="text-xs text-muted-foreground">#{item.id} · {(item.fileSize / 1024).toFixed(1)} KB</p></td>
              <td className="p-3">{item.source || "—"}</td><td className="p-3">{item.uploadedBy || "—"}</td><td className="whitespace-nowrap p-3">{new Date(item.createdAt).toLocaleString()}</td>
              <td className="p-3"><Badge variant={item.scanStatus === "quarantined" ? "destructive" : "outline"}>{t(`assetQuarantine.statuses.${item.scanStatus || "unscanned"}`)}</Badge></td>
              <td className="min-w-44 p-3"><p>{item.scanReason ? t(`assetQuarantine.reasons.${item.scanReason}`) : "—"}</p>{!item.receiveComplete && item.scanStatus === "quarantined" && <p className="mt-1 text-xs text-muted-foreground">{t("assetQuarantine.incomplete")}</p>}</td>
              <td className="p-3"><div className="flex gap-2"><Button size="sm" variant="outline" disabled={attemptsBusy} onClick={() => void showAttempts(item)}>{t("assetQuarantine.history")}</Button>{["quarantined", "unscanned", ""].includes(item.scanStatus) && <Button size="sm" disabled={rescanning !== null || (item.scanStatus === "quarantined" && !item.receiveComplete)} onClick={() => void rescan(item)}>{rescanning === item.id ? t("assetQuarantine.scanning") : t("assetQuarantine.rescan")}</Button>}</div></td>
            </tr>)}
            {!items.length && <tr><td colSpan={7} className="p-8 text-center text-muted-foreground">{busy ? t("assetQuarantine.loading") : t("assetQuarantine.empty")}</td></tr>}
          </tbody>
        </table>
      </div>
      <div className="flex items-center justify-between text-sm"><span>{t("assetQuarantine.total", { total })}</span><div className="flex items-center gap-3"><Button variant="outline" disabled={page === 1 || busy} onClick={() => setPage(value => value - 1)}>{t("assetQuarantine.previous")}</Button><span>{page}</span><Button variant="outline" disabled={page * 20 >= total || busy} onClick={() => setPage(value => value + 1)}>{t("assetQuarantine.next")}</Button></div></div>
      {selected && <section className="space-y-3 rounded-md border p-4" aria-label={t("assetQuarantine.history")}><h2 className="font-semibold">{t("assetQuarantine.history")} · {selected.filename}</h2><p className="break-all text-xs text-muted-foreground">SHA-256: {selected.sha256 || "—"}</p>
        {attemptsError && <p role="alert" className="text-sm text-destructive">{attemptsError}</p>}
        {!attempts.length && <p className="text-sm text-muted-foreground">{attemptsBusy ? t("assetQuarantine.loading") : t("assetQuarantine.noHistory")}</p>}
        {attempts.map(attempt => <div key={attempt.id} className="border-t pt-3 text-sm"><p>{new Date(attempt.createdAt).toLocaleString()} · {t(`assetQuarantine.statuses.${attempt.status}`)}{attempt.reason ? ` · ${t(`assetQuarantine.reasons.${attempt.reason}`)}` : ""}</p><p className="break-words text-muted-foreground">{attempt.detail}</p><p className="text-xs text-muted-foreground">{attempt.scanPerformed ? `ClamAV ${attempt.engineVersion} · ${t("assetQuarantine.database")} ${attempt.databaseVersion}` : t("assetQuarantine.notPerformed")}</p></div>)}
        {historyMore && <Button variant="outline" disabled={attemptsBusy} onClick={() => void showAttempts(selected, attempts.at(-1)?.id)}>{t("assetQuarantine.more")}</Button>}
      </section>}
    </DashboardPage>
  )
}
