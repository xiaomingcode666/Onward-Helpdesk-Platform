"use client"

import { useCallback, useEffect, useRef, useState } from "react"
import { RailopsButton } from "@railops/ui"
import { useAppLocale } from "@/i18n/provider"
import { apiGet, apiPost } from "@/lib/api/client"
import { TicketWorkflowCanvas } from "./ticket-workflow-canvas"
import { displayTicketWorkflowProjectName, displayTicketWorkflowVersionNote, ticketWorkflowCopy } from "./ticket-status-workflow-i18n"

type Workflow = { transitions: Record<string, string[]> }
type Version = { id: number; note: string; published: boolean; workflow: Workflow }
type View = {
  active_version_id: number; deployment_managed: boolean; can_manage: boolean; project_key: string
  projects: Array<{ key: string; name: string }>; workflow: Workflow; allowed: Workflow
  required: Record<string, string[]>; states: string[]; versions: Version[]; next_before_id: number
}
const endpoint = "/ticket-settings/status-workflow"

export function TicketStatusWorkflowCard() {
  const { locale } = useAppLocale()
  const copy = ticketWorkflowCopy(locale)
  const [view, setView] = useState<View | null>(null)
  const [project, setProject] = useState("*")
  const [workflow, setWorkflow] = useState<Workflow | null>(null)
  const [editing, setEditing] = useState(false)
  const [note, setNote] = useState("")
  const [draftId, setDraftId] = useState(0)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
  const [notice, setNotice] = useState("")
  const generation = useRef(0)
  const inFlight = useRef(false)
  const attempt = useRef({ signature: "", key: "" })

  const load = useCallback(async () => {
    const request = ++generation.current
    setError("")
    setView(null)
    setEditing(false)
    setDraftId(0)
    try {
      const result = await apiGet<View>(endpoint, { project_key: project })
      if (request !== generation.current) return
      if (!result.success || !result.data) throw new Error(result.error?.message || copy.loadFailed)
      if (!Array.isArray(result.data.states) || result.data.states.length !== 10 || !result.data.workflow?.transitions || !result.data.allowed?.transitions || !result.data.required || !Array.isArray(result.data.versions) || !Array.isArray(result.data.projects)) {
        throw new Error(copy.incomplete)
      }
      setView(result.data)
      setWorkflow(result.data.workflow)
    } catch (cause) {
      if (request === generation.current) setError(cause instanceof Error ? cause.message : copy.loadFailed)
    }
  }, [copy, project])
  const invalidateLoads = useCallback(() => { generation.current++ }, [])
  useEffect(() => { void load(); return invalidateLoads }, [load, invalidateLoads])

  async function saveDraft() {
    if (!view || !workflow || inFlight.current) return
    inFlight.current = true
    setBusy(true); setError(""); setNotice("")
    const body = { project_key: project, workflow, base_version_id: view.active_version_id, note: note.trim() }
    const signature = JSON.stringify(body)
    if (attempt.current.signature !== signature) attempt.current = { signature, key: crypto.randomUUID() }
    try {
      const result = await apiPost<{ id: number }>(`${endpoint}/drafts`, { ...body, request_key: attempt.current.key })
      if (!result.success || !result.data) throw new Error(result.error?.message || copy.saveFailed)
      setDraftId(result.data.id)
      setNotice(copy.draftSaved(result.data.id))
    } catch (cause) { setError(cause instanceof Error ? cause.message : copy.saveFailed) }
    finally { inFlight.current = false; setBusy(false) }
  }

  async function publish() {
    if (!draftId || inFlight.current) return
    inFlight.current = true; setBusy(true); setError(""); setNotice("")
    try {
      const result = await apiPost<{ id: number }>(`${endpoint}/publish`, { version_id: draftId })
      if (!result.success) throw new Error(result.error?.message || copy.publishFailed)
      setNotice(copy.published)
      await load()
    } catch (cause) { setError(cause instanceof Error ? cause.message : copy.publishFailed) }
    finally { inFlight.current = false; setBusy(false) }
  }

  async function moreHistory() {
    if (!view?.next_before_id || inFlight.current) return
    inFlight.current = true; setBusy(true); setError("")
    try {
      const result = await apiGet<View>(endpoint, { project_key: project, before_id: view.next_before_id })
      if (!result.success || !result.data) throw new Error(result.error?.message || copy.historyFailed)
      const next = result.data
      setView({ ...view, versions: [...view.versions, ...next.versions], next_before_id: next.next_before_id })
    } catch (cause) { setError(cause instanceof Error ? cause.message : copy.historyFailed) }
    finally { inFlight.current = false; setBusy(false) }
  }

  function changeTransition(from: string, to: string, enabled: boolean) {
    if (!editing || busy || !view?.can_manage || !view.allowed.transitions[from]?.includes(to)) return
    if (!enabled && view.required[from]?.includes(to)) return
    setWorkflow(current => current && ({ transitions: { ...current.transitions, [from]: enabled
      ? Array.from(new Set([...(current.transitions[from] ?? []), to]))
      : (current.transitions[from] ?? []).filter(value => value !== to) } }))
    setDraftId(0); setNotice("")
  }

  return <section className="space-y-4 rounded-lg border border-border bg-card p-4" data-testid="ticket-status-workflow">
    <div className="flex flex-wrap items-center justify-between gap-3">
      <div><h2 className="text-base font-semibold">{copy.title}</h2>
        <p className="mt-1 text-sm text-muted-foreground">{copy.instruction}</p></div>
      {view && <label className="text-sm">{copy.scope} <select aria-label={copy.scope} value={project} disabled={busy || editing} className="rounded border bg-background p-2" onChange={e => { setNotice(""); setProject(e.target.value) }}>
        <option value="*">{copy.defaultScope}</option>
        {view.projects.map(p => <option key={p.key} value={p.key}>{displayTicketWorkflowProjectName(p.name, locale)}</option>)}
      </select></label>}
    </div>
    {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
    {notice && <p role="status" className="text-sm text-emerald-700">{notice}</p>}
    {!view ? <RailopsButton disabled={busy} onClick={() => void load()}>{copy.reload}</RailopsButton> : <>
      <p className="text-sm text-muted-foreground">{view.active_version_id ? copy.version(view.active_version_id) : copy.systemDefault}。{copy.versionNote}</p>
      <p className="text-xs text-muted-foreground">{copy.assignmentNote}</p>
      <p className="text-xs text-muted-foreground">{copy.deviceNote}</p>
      {workflow && <TicketWorkflowCanvas key={project} states={view.states} workflow={workflow}
        allowed={view.allowed} required={view.required} editing={editing} busy={busy}
        onChangeTransition={changeTransition} />}
      {view.can_manage ? <div className="space-y-3">
        {editing && <label className="block text-sm">{copy.editReason}
          <input aria-label={copy.editReason} className="mt-1 block w-full rounded border bg-background p-2" maxLength={400} value={note} disabled={busy} onChange={e => { setNote(e.target.value); setDraftId(0); setNotice("") }} placeholder={copy.editReasonPlaceholder} />
        </label>}
        <div className="flex flex-wrap gap-2">
          {!editing ? <RailopsButton onClick={() => { setEditing(true); setNote(""); setNotice("") }}>{copy.edit}</RailopsButton> : <>
            <RailopsButton disabled={busy || !note.trim() || Boolean(draftId)} onClick={() => void saveDraft()}>{copy.saveDraft}</RailopsButton>
            <RailopsButton variant="primary" disabled={busy || !draftId || view.deployment_managed} onClick={() => void publish()}>{copy.publish}</RailopsButton>
            <RailopsButton disabled={busy} onClick={() => { setEditing(false); setWorkflow(view.workflow); setDraftId(0); setNotice("") }}>{copy.returnCurrent}</RailopsButton>
          </>}
        </div>
        {view.deployment_managed && <p className="text-sm text-muted-foreground">{copy.deploymentNote}</p>}
        <details><summary className="cursor-pointer text-sm">{copy.history}</summary>
          <p className="my-2 text-xs text-muted-foreground">{copy.historyNote}</p>
          <div className="space-y-2">{view.versions.map(version => <div key={version.id} className="flex flex-wrap items-center justify-between gap-2 rounded border p-2 text-sm">
            <span>#{version.id} · {version.id === view.active_version_id ? copy.current : version.published ? copy.publishedHistory : copy.draft} · {displayTicketWorkflowVersionNote(version.note, locale)}</span>
            <RailopsButton size="small" disabled={busy} onClick={() => { setWorkflow(structuredClone(version.workflow)); setEditing(true); setNote(copy.loadedVersion(version.id)); setDraftId(0); setNotice(copy.loadedNotice); setError("") }}>{copy.loadVersion}</RailopsButton>
          </div>)}</div>
          {view.next_before_id > 0 && <RailopsButton disabled={busy} onClick={() => void moreHistory()}>{copy.loadEarlier}</RailopsButton>}
        </details>
      </div> : <p className="text-sm text-muted-foreground">{copy.viewOnly}</p>}
    </>}
  </section>
}
