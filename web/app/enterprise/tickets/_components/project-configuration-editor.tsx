"use client"

import { useRef, useState } from "react"
import { Settings2Icon } from "lucide-react"
import { IconButton, RailopsButton, StandardModal } from "@railops/ui"
import { useAuth } from "@/components/auth-provider"
import { applyProjectConfiguration, getProjectConfiguration, listRetentionApprovals, previewProjectConfigurationImpact, reviewRetentionApproval, saveProjectConfiguration, submitRetentionApproval, validateProjectConfiguration, type ConfigurationImpact, type ConfigurationReport, type ConfigurationVersion, type ConfigurationView, type ProjectConfiguration, type RetentionApproval } from "@/lib/api/project-configuration"
import { upgradeProjectConfiguration } from "@/lib/api/project-configuration"
import { editableProjectConfiguration as editableDocument, restoreProjectConfiguration } from "@/lib/project-configuration-editing"
import { ProjectRuntimeFields } from "./project-runtime-fields"

const environments: Record<string, string> = { development: "开发", integration: "集成测试", staging: "预发布", production: "生产" }

export function ProjectConfigurationEditor() {
  const { session } = useAuth()
  const [open, setOpen] = useState(false)
  const [view, setView] = useState<ConfigurationView | null>(null)
  const [document, setDocument] = useState<ProjectConfiguration | null>(null)
  const [note, setNote] = useState("")
  const [draft, setDraft] = useState<ConfigurationVersion | null>(null)
  const [report, setReport] = useState<ConfigurationReport | null>(null)
  const [impact, setImpact] = useState<ConfigurationImpact | null>(null)
  const [impactSignature, setImpactSignature] = useState("")
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
  const [message, setMessage] = useState("")
  const [approvals, setApprovals] = useState<RetentionApproval[]>([])
  const request = useRef({ signature: "", key: "" })

  if (!session?.permissions?.includes("ticket.update")) return null

  function edit(next: ProjectConfiguration) {
    setDocument(next); setDraft(null); setReport(null); setImpact(null); setImpactSignature(""); setMessage(""); setError("")
  }
  async function load() {
    setOpen(true); setBusy(true); setError(""); setMessage(""); setView(null); setDocument(null); setDraft(null); setReport(null); setImpact(null); setImpactSignature(""); setNote(""); setApprovals([])
    try {
      const result = await getProjectConfiguration()
      if (!result.success || !result.data) throw new Error(result.error?.message || "配置加载失败")
      setView(result.data); setDocument(editableDocument(result.data.document))
      const productionVersions = result.data.versions.filter(v => v.document.environment === "production")
      const approvalResults = await Promise.all(productionVersions.map(v => listRetentionApprovals(v.id)))
      setApprovals(approvalResults.flatMap(a => a.success && a.data ? a.data : []))
    } catch (e) { setError(e instanceof Error ? e.message : "配置加载失败") }
    finally { setBusy(false) }
  }
  async function validate() {
    if (!document) return
    setBusy(true); setError(""); setMessage("")
    try {
      const result = await validateProjectConfiguration(document)
      if (!result.success || !result.data) throw new Error(result.error?.message || "检查失败")
      setReport(result.data)
    } catch (e) { setError(e instanceof Error ? e.message : "检查失败") }
    finally { setBusy(false) }
  }
  async function preview() {
    if (!document || !draft || !report?.valid) return
    setBusy(true); setError(""); setMessage("")
    try {
      const result = await previewProjectConfigurationImpact(document)
      if (!result.success || !result.data) throw new Error(result.error?.message || "影响试算失败")
      setImpact(result.data)
      setImpactSignature(JSON.stringify(document))
      setMessage(`影响试算完成：扫描 ${result.data.summary.scanned} 条工单，发现 ${result.data.summary.affected} 条受影响。`)
    } catch (e) { setError(e instanceof Error ? e.message : "影响试算失败") }
    finally { setBusy(false) }
  }
  async function save() {
    if (!document || !view) return
    setBusy(true); setError(""); setMessage("")
    try {
      // 修改说明不再让用户填写：备注由系统生成，历史列表仍然看得出这版是怎么来的。
      const effectiveNote = note.trim() || (view.active_version_id ? `基于 V${view.active_version_id} 调整运营配置` : "将现有运营设置纳入配置版本管理")
      const signature = JSON.stringify([document, effectiveNote, view.active_version_id])
      if (request.current.signature !== signature) request.current = { signature, key: crypto.randomUUID() }
      const result = await saveProjectConfiguration(document, view.active_version_id, effectiveNote, request.current.key)
      if (!result.success || !result.data) throw new Error(result.error?.message || "草稿保存失败")
      setDraft(result.data)
      const a = await listRetentionApprovals(result.data.id); if (a.success && a.data) setApprovals(a.data)
      setView({ ...view, versions: [result.data, ...view.versions.filter(v => v.id !== result.data.id)] })
      setMessage(`草稿 V${result.data.id} 已保存，尚未生效。`)
    } catch (e) { setError(e instanceof Error ? e.message : "草稿保存失败") }
    finally { setBusy(false) }
  }
  async function submitRetention(version: ConfigurationVersion) {
    setBusy(true); setError(""); setMessage("")
    try { const result = await submitRetentionApproval(version.id); if (!result.success || !result.data) throw new Error(result.error?.message || "提交审批失败"); setApprovals(result.data); setMessage(`V${version.id} 的 Retention Policy 已提交租户所有者审批。`) }
    catch (e) { setError(e instanceof Error ? e.message : "提交审批失败") } finally { setBusy(false) }
  }
  async function reviewRetention(approval: RetentionApproval, approved: boolean) {
    const comment = window.prompt(approved ? "批准意见（可留空）" : "请输入驳回原因") ?? ""
    if (!approved && !comment.trim()) return
    setBusy(true); setError("")
    try { const result = await reviewRetentionApproval(approval.id, approved, comment); if (!result.success) throw new Error(result.error?.message || "审批失败"); const current = await listRetentionApprovals(approval.version_id); if (current.success && current.data) setApprovals(current.data); setMessage(approved ? `项目 ${approval.project_key} 已批准。` : `项目 ${approval.project_key} 已驳回。`) }
    catch (e) { setError(e instanceof Error ? e.message : "审批失败") } finally { setBusy(false) }
  }
  async function apply() {
    if (!document || !view || !draft) return
    if (!impact || impactSignature !== JSON.stringify(document)) { setError("配置已修改，请先重新执行影响试算"); return }
    const targetDraft = draft
    setBusy(true); setError(""); setMessage("")
    try {
      const result = await applyProjectConfiguration(targetDraft.id)
      if (!result.success || !result.data) throw new Error(result.error?.message || "应用失败，原配置继续生效")
      // Re-read the active pointer: a retried old application must not present
      // itself as current if another version was applied in the meantime.
      const current = await getProjectConfiguration()
      if (!current.success || !current.data) throw new Error("应用已提交，但当前版本刷新失败，请重新加载确认")
      setView(current.data); setDocument(editableDocument(current.data.document)); setDraft(null); setReport(null); setNote("")
      setMessage(`当前生效版本为 V${current.data.active_version_id}。已有工单继续使用各自记录的规则版本。`)
    } catch (e) { setError(e instanceof Error ? e.message : "应用失败") }
    finally { setBusy(false) }
  }
  function copyVersion(version: ConfigurationVersion) {
    const copy = restoreProjectConfiguration(version.document, view?.document)
    edit(copy); setNote(`基于 V${version.id} 调整：${version.note}`.slice(0, 500))
    const retainedRuntime = !version.document.runtime && copy.runtime ? "这版未包含运营设置，已保留当前运营设置及其密钥引用。" : ""
    setMessage(`已载入 V${version.id} 的内容，保存为新草稿并检查后才能应用。${retainedRuntime}`)
  }
  async function upgrade() {
    setBusy(true); setError("")
    try {
      const result = await upgradeProjectConfiguration()
      if (!result.success || !result.data) throw new Error(result.error?.message || "现有设置加载失败")
      edit(editableDocument(result.data)); setNote("将现有运营设置纳入配置版本管理")
      setMessage("已载入现有设置，尚未生效。请核对渠道开关，并补齐邮件及接入密钥引用，再保存和检查。")
    } catch (e) { setError(e instanceof Error ? e.message : "加载失败") }
    finally { setBusy(false) }
  }
  async function loadOlder() {
    if (!view?.next_before_id) return
    setBusy(true); setError("")
    try {
      const result = await getProjectConfiguration(view.next_before_id)
      if (!result.success || !result.data) throw new Error(result.error?.message || "历史加载失败")
      setView({ ...view, next_before_id: result.data.next_before_id, versions: [...view.versions, ...result.data.versions.filter(v => !view.versions.some(existing => existing.id === v.id))] })
    } catch (e) { setError(e instanceof Error ? e.message : "历史加载失败") }
    finally { setBusy(false) }
  }
  function exportVersion(version: ConfigurationVersion) {
    const blob = new Blob([JSON.stringify({ version_id: version.id, digest: version.digest, document: version.document }, null, 2)], { type: "application/json" })
    const url = URL.createObjectURL(blob)
    const a = window.document.createElement("a")
    a.href = url; a.download = `project-configuration-v${version.id}.json`; a.click(); URL.revokeObjectURL(url)
  }
  return <>
    <IconButton icon={<Settings2Icon />} tooltip="运营配置" aria-label="运营配置" onClick={() => void load()} />
    <StandardModal open={open} width={880} title="运营配置 · 配置版本" styles={{ body: { maxHeight: "calc(100dvh - 200px)", overflowY: "auto" } }} onCancel={() => !busy && setOpen(false)} footer={<div className="flex flex-wrap justify-end gap-2">
      <RailopsButton disabled={busy} onClick={() => void load()}>重新加载</RailopsButton>
      <RailopsButton disabled={busy || !document} onClick={() => void save()}>保存草稿</RailopsButton>
      <RailopsButton disabled={busy || !document} onClick={() => void validate()}>检查配置</RailopsButton>
      <RailopsButton disabled={busy || !document || !draft || !report?.valid} onClick={() => void preview()}>影响试算</RailopsButton>
      {view?.deployment_managed
        ? <RailopsButton disabled={busy || !document || !draft || !report?.valid || !impact || impactSignature !== JSON.stringify(document)} onClick={() => draft && exportVersion(draft)}>导出已试算的草稿</RailopsButton>
        : <RailopsButton disabled={busy || !document || !draft || !report?.valid || !impact || impactSignature !== JSON.stringify(document)} onClick={() => void apply()}>应用已试算的草稿</RailopsButton>}
    </div>}>
      <div className="grid gap-4">
        {error && <div role="alert" className="text-sm text-destructive">{error}</div>}
        {message && <div role="status" className="rounded border p-3 text-sm">{message}</div>}
        {view && document && <>
          <div className="text-sm text-muted-foreground">当前公司 #{document.tenant_id} · {environments[document.environment] || document.environment}环境 · 当前生效：{view.active_version_id ? `V${view.active_version_id}` : "原有规则（尚未应用版本）"}</div>
          <p className="text-sm">保存草稿不会改变现有规则。检查通过并应用后，新工单才使用新版本。渠道开关只限制录单来源，不会自动接通邮件或其他外部渠道。</p>
          {view.deployment_managed && <p className="rounded border p-3 text-sm">当前环境由部署管理。请保存、检查并导出草稿，交给负责部署的工程师应用；导出不会改变现有规则。部署成功后点击“重新加载”查看生效版本。</p>}
          <fieldset disabled={busy} className="grid min-w-0 gap-4">
            {!document.runtime && <RailopsButton onClick={() => void upgrade()}>载入现有运营设置（升级配置）</RailopsButton>}
            <ProjectRuntimeFields document={document} onChange={edit} disabled={busy} />
          </fieldset>
          {report && <div role="status" className={`rounded border p-3 text-sm ${report.valid ? "text-green-700" : "text-destructive"}`}>
            {report.valid ? "配置检查通过；应用时还会再次检查。" : report.issues.map((issue, i) => <div key={i}>{issue.path}：{issue.message}</div>)}
          </div>}
          {impact && <div role="status" className="rounded border p-3 text-sm">
            <div className="font-medium">历史工单影响试算</div>
            <div className="mt-1">扫描 {impact.summary.scanned} 条，受影响 {impact.summary.affected} 条；截止时间提前 {impact.summary.shortened} 条，延后 {impact.summary.extended} 条，新增逾期 {impact.summary.newly_breached} 条。</div>
            {impact.items.length > 0 && <div className="mt-2 grid gap-1">{impact.items.slice(0, 20).map(item => <div key={`${item.ticket_id}-${item.metric}`} className="text-muted-foreground">{item.ticket_no || item.ticket_id} · {item.metric} · {item.delta_minutes > 0 ? `延后 ${item.delta_minutes} 分钟` : `提前 ${Math.abs(item.delta_minutes)} 分钟`} · {item.reason}</div>)}</div>}
          </div>}
          <details><summary className="cursor-pointer font-medium">历史与草稿</summary>
            <div className="mt-3 grid gap-3">{view.versions.map(version => <div key={version.id} className="rounded border p-3 text-sm">
              <div>V{version.id} · {version.id === view.active_version_id ? "当前生效" : version.activation ? "曾经生效" : version.created_by === 0 ? "迁移基线" : "未应用草稿"} · {new Date(version.created_at).toLocaleString()}</div>
              <div className="my-2 break-words">{version.note}</div>
              {version.document.environment === "production" && <div className="my-2 rounded bg-muted/40 p-2">
                <div className="mb-2 font-medium">Retention 审批</div>
                {approvals.filter(a => a.version_id === version.id).length === 0
                  ? <span className="text-muted-foreground">尚未提交审批</span>
                  : approvals.filter(a => a.version_id === version.id).map(a => <div key={a.id} className="flex flex-wrap items-center gap-2"><span>{a.project_key}: {a.status}</span>{a.reviewed_by_name && <span className="text-muted-foreground">({a.reviewed_by_name})</span>}{a.can_approve && <><RailopsButton disabled={busy} onClick={() => void reviewRetention(a, true)}>批准</RailopsButton><RailopsButton disabled={busy} onClick={() => void reviewRetention(a, false)}>驳回</RailopsButton></>}</div>)}
                {!version.activation && version.id !== view.active_version_id && <RailopsButton disabled={busy} onClick={() => void submitRetention(version)}>提交 Retention 审批</RailopsButton>}
              </div>}
              {version.document.runtime && <details><summary>查看运营设置快照</summary><pre className="max-h-64 overflow-auto whitespace-pre-wrap break-all">{JSON.stringify(version.document.runtime, null, 2)}</pre></details>}
              <div className="text-muted-foreground">记录人：{version.created_by_name || version.created_by || "系统迁移"}{version.activation ? `；应用人：${version.activation.applied_by_name || version.activation.applied_by}；应用时间：${new Date(version.activation.applied_at).toLocaleString()}` : ""}</div>
              <div className="mt-2 flex gap-2"><RailopsButton disabled={busy} onClick={() => copyVersion(version)}>载入为新草稿</RailopsButton>{version.activation && <RailopsButton disabled={busy} onClick={() => exportVersion(version)}>导出部署配置</RailopsButton>}</div>
            </div>)}</div>
            {view.next_before_id > 0 && <RailopsButton disabled={busy} onClick={() => void loadOlder()}>加载更早版本</RailopsButton>}
          </details>
        </>}
      </div>
    </StandardModal>
  </>
}
