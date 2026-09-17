"use client"

import { useCallback, useEffect, useRef, useState } from "react"
import { RailopsButton } from "@railops/ui"
import { apiGet, apiPost } from "@/lib/api/client"
import { TicketWorkflowCanvas } from "./ticket-workflow-canvas"

type Workflow = { transitions: Record<string, string[]> }
type Version = { id: number; note: string; published: boolean; workflow: Workflow }
type View = {
  active_version_id: number; deployment_managed: boolean; can_manage: boolean; project_key: string
  projects: Array<{ key: string; name: string }>; workflow: Workflow; allowed: Workflow
  required: Record<string, string[]>; states: string[]; versions: Version[]; next_before_id: number
}
const endpoint = "/ticket-settings/status-workflow"

export function TicketStatusWorkflowCard() {
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
      if (!result.success || !result.data) throw new Error(result.error?.message || "工单流程加载失败")
      if (!Array.isArray(result.data.states) || result.data.states.length !== 10 || !result.data.workflow?.transitions || !result.data.allowed?.transitions || !result.data.required || !Array.isArray(result.data.versions) || !Array.isArray(result.data.projects)) {
        throw new Error("后端尚未提供完整的工单流程配置，请升级并重启后端后重新加载。")
      }
      setView(result.data)
      setWorkflow(result.data.workflow)
    } catch (cause) {
      if (request === generation.current) setError(cause instanceof Error ? cause.message : "工单流程加载失败")
    }
  }, [project])
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
      if (!result.success || !result.data) throw new Error(result.error?.message || "草稿保存失败")
      setDraftId(result.data.id)
      setNotice(`草稿 #${result.data.id} 已保存，尚未生效。`)
    } catch (cause) { setError(cause instanceof Error ? cause.message : "草稿保存失败") }
    finally { inFlight.current = false; setBusy(false) }
  }

  async function publish() {
    if (!draftId || inFlight.current) return
    inFlight.current = true; setBusy(true); setError(""); setNotice("")
    try {
      const result = await apiPost<{ id: number }>(`${endpoint}/publish`, { version_id: draftId })
      if (!result.success) throw new Error(result.error?.message || "发布失败")
      setNotice("已发布。此后创建的工单使用新流程，已有工单保持原流程。")
      await load()
    } catch (cause) { setError(cause instanceof Error ? cause.message : "发布失败") }
    finally { inFlight.current = false; setBusy(false) }
  }

  async function moreHistory() {
    if (!view?.next_before_id || inFlight.current) return
    inFlight.current = true; setBusy(true); setError("")
    try {
      const result = await apiGet<View>(endpoint, { project_key: project, before_id: view.next_before_id })
      if (!result.success || !result.data) throw new Error(result.error?.message || "历史加载失败")
      const next = result.data
      setView({ ...view, versions: [...view.versions, ...next.versions], next_before_id: next.next_before_id })
    } catch (cause) { setError(cause instanceof Error ? cause.message : "历史加载失败") }
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
      <div><h2 className="text-base font-semibold">工单状态流程</h2>
        <p className="mt-1 text-sm text-muted-foreground">点击状态查看下一步；修改时可以拖动节点、连接状态。</p></div>
      {view && <label className="text-sm">适用范围 <select aria-label="流程适用范围" value={project} disabled={busy || editing} className="rounded border bg-background p-2" onChange={e => { setNotice(""); setProject(e.target.value) }}>
        <option value="*">公司默认流程</option>
        {view.projects.map(p => <option key={p.key} value={p.key}>{p.name}</option>)}
      </select></label>}
    </div>
    {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
    {notice && <p role="status" className="text-sm text-emerald-700">{notice}</p>}
    {!view ? <RailopsButton disabled={busy} onClick={() => void load()}>重新加载流程</RailopsButton> : <>
      <p className="text-sm text-muted-foreground">{view.active_version_id ? `当前配置版本 #${view.active_version_id}` : "当前使用系统默认规则"}。修改发布后只影响新工单；项目未单独设置时使用公司默认流程。</p>
      <p className="text-xs text-muted-foreground">分配工程师后自动进入“已分配”；等待结束时回到等待前的状态；重新打开回到“分析中”。必要步骤已锁定，权限和处理说明仍需满足。</p>
      <p className="text-xs text-muted-foreground">设备工单通过处理记录确认解决。这里设置允许的流转范围，工单详情只显示当前能执行的操作。</p>
      {workflow && <TicketWorkflowCanvas key={project} states={view.states} workflow={workflow}
        allowed={view.allowed} required={view.required} editing={editing} busy={busy}
        onChangeTransition={changeTransition} />}
      {view.can_manage ? <div className="space-y-3">
        {editing && <label className="block text-sm">修改原因
          <input aria-label="流程修改原因" className="mt-1 block w-full rounded border bg-background p-2" maxLength={400} value={note} disabled={busy} onChange={e => { setNote(e.target.value); setDraftId(0); setNotice("") }} placeholder="例如：问题解决后先进入待关闭，再确认关闭" />
        </label>}
        <div className="flex flex-wrap gap-2">
          {!editing ? <RailopsButton onClick={() => { setEditing(true); setNote(""); setNotice("") }}>修改流程</RailopsButton> : <>
            <RailopsButton disabled={busy || !note.trim() || Boolean(draftId)} onClick={() => void saveDraft()}>保存草稿</RailopsButton>
            <RailopsButton variant="primary" disabled={busy || !draftId || view.deployment_managed} onClick={() => void publish()}>发布流程</RailopsButton>
            <RailopsButton disabled={busy} onClick={() => { setEditing(false); setWorkflow(view.workflow); setDraftId(0); setNotice("") }}>返回当前流程</RailopsButton>
          </>}
        </div>
        {view.deployment_managed && <p className="text-sm text-muted-foreground">此环境由部署管理。保存草稿后，在企业工单的配置管理中导出对应版本，由运维部署生效。</p>}
        <details><summary className="cursor-pointer text-sm">历史版本与恢复</summary>
          <p className="my-2 text-xs text-muted-foreground">选择历史版本只载入流程规则，填写原因、保存并发布后才会生效；其他公司设置不会被恢复。</p>
          <div className="space-y-2">{view.versions.map(version => <div key={version.id} className="flex flex-wrap items-center justify-between gap-2 rounded border p-2 text-sm">
            <span>#{version.id} · {version.id === view.active_version_id ? "当前生效" : version.published ? "曾发布" : "草稿"} · {version.note}</span>
            <RailopsButton size="small" disabled={busy} onClick={() => { setWorkflow(structuredClone(version.workflow)); setEditing(true); setNote(`恢复版本 #${version.id} 的工单流程`); setDraftId(0); setNotice("已载入历史规则，请保存并发布后生效。"); setError("") }}>载入此流程</RailopsButton>
          </div>)}</div>
          {view.next_before_id > 0 && <RailopsButton disabled={busy} onClick={() => void moreHistory()}>加载更早版本</RailopsButton>}
        </details>
      </div> : <p className="text-sm text-muted-foreground">可查看当前规则；修改和发布请由公司管理员操作。</p>}
    </>}
  </section>
}
