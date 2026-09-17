"use client"

import { useRef, useState } from "react"
import { Settings2Icon } from "lucide-react"
import { IconButton, RailopsButton, StandardModal } from "@railops/ui"
import { useAuth } from "@/components/auth-provider"
import { applyProjectConfiguration, getProjectConfiguration, saveProjectConfiguration, validateProjectConfiguration, type ConfigurationReport, type ConfigurationVersion, type ConfigurationView, type ProjectConfiguration } from "@/lib/api/project-configuration"
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
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
  const [message, setMessage] = useState("")
  const request = useRef({ signature: "", key: "" })

  if (!session?.permissions?.includes("ticket.update")) return null

  function edit(next: ProjectConfiguration) {
    setDocument(next); setDraft(null); setReport(null); setMessage(""); setError("")
  }
  async function load() {
    setOpen(true); setBusy(true); setError(""); setMessage(""); setView(null); setDocument(null); setDraft(null); setReport(null); setNote("")
    try {
      const result = await getProjectConfiguration()
      if (!result.success || !result.data) throw new Error(result.error?.message || "配置加载失败")
      setView(result.data); setDocument(editableDocument(result.data.document))
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
      setView({ ...view, versions: [result.data, ...view.versions.filter(v => v.id !== result.data.id)] })
      setMessage(`草稿 V${result.data.id} 已保存，尚未生效。`)
    } catch (e) { setError(e instanceof Error ? e.message : "草稿保存失败") }
    finally { setBusy(false) }
  }
  async function apply() {
    if (!draft || !view) return
    setBusy(true); setError(""); setMessage("")
    try {
      const result = await applyProjectConfiguration(draft.id)
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
      {view?.deployment_managed
        ? <RailopsButton disabled={busy || !draft || !report?.valid} onClick={() => draft && exportVersion(draft)}>导出已检查的草稿</RailopsButton>
        : <RailopsButton disabled={busy || !draft || !report?.valid} onClick={() => void apply()}>应用已检查的草稿</RailopsButton>}
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
          <details><summary className="cursor-pointer font-medium">历史与草稿</summary>
            <div className="mt-3 grid gap-3">{view.versions.map(version => <div key={version.id} className="rounded border p-3 text-sm">
              <div>V{version.id} · {version.id === view.active_version_id ? "当前生效" : version.activation ? "曾经生效" : version.created_by === 0 ? "迁移基线" : "未应用草稿"} · {new Date(version.created_at).toLocaleString()}</div>
              <div className="my-2 break-words">{version.note}</div>
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
