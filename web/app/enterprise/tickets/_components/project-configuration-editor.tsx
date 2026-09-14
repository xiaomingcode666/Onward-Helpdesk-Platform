"use client"

import { useRef, useState } from "react"
import { Checkbox, Input, Select } from "antd"
import { Settings2Icon, Trash2Icon } from "lucide-react"
import { FormField, IconButton, RailopsButton, StandardModal } from "@railops/ui"
import { useAuth } from "@/components/auth-provider"
import { translateCurrentMessage } from "@/i18n/messages"
import { applyProjectConfiguration, getProjectConfiguration, saveProjectConfiguration, validateProjectConfiguration, type ConfigurationReport, type ConfigurationVersion, type ConfigurationView, type ProjectConfiguration } from "@/lib/api/project-configuration"
import type { TicketIntakePolicy } from "@/lib/ticket-intake"
import { upgradeProjectConfiguration } from "@/lib/api/project-configuration"
import { editableProjectConfiguration as editableDocument, restoreProjectConfiguration } from "@/lib/project-configuration-editing"
import { ProjectRuntimeFields } from "./project-runtime-fields"

const label = (key: string) => translateCurrentMessage(`ticketIntake.${key}`)
const environments: Record<string, string> = { development: "开发", integration: "集成测试", staging: "预发布", production: "生产" }

export function ProjectConfigurationEditor({ onSaved }: { onSaved?: (policy: TicketIntakePolicy) => void }) {
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
      const signature = JSON.stringify([document, note, view.active_version_id])
      if (request.current.signature !== signature) request.current = { signature, key: crypto.randomUUID() }
      const result = await saveProjectConfiguration(document, view.active_version_id, note, request.current.key)
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
      onSaved?.(current.data.document.intake)
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
    <IconButton icon={<Settings2Icon />} tooltip={label("policy")} aria-label={label("policy")} onClick={() => void load()} />
    <StandardModal open={open} width={880} title="受理规则 · 配置版本" styles={{ body: { maxHeight: "calc(100dvh - 200px)", overflowY: "auto" } }} onCancel={() => !busy && setOpen(false)} footer={<div className="flex flex-wrap justify-end gap-2">
      <RailopsButton disabled={busy} onClick={() => void load()}>重新加载</RailopsButton>
      <RailopsButton disabled={busy || !document || !note.trim()} onClick={() => void save()}>保存草稿</RailopsButton>
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
          <p className="text-sm">保存草稿不会改变现有规则。检查通过并应用后，新工单才使用新版本。渠道规则用于检查录单资料，添加规则不会自动接通邮件或其他外部渠道。</p>
          {view.deployment_managed && <p className="rounded border p-3 text-sm">当前环境由部署管理。请保存、检查并导出草稿，交给负责部署的工程师应用；导出不会改变现有规则。部署成功后点击“重新加载”查看生效版本。</p>}
          <fieldset disabled={busy} className="grid min-w-0 gap-4">
            {!document.runtime && <RailopsButton onClick={() => void upgrade()}>载入现有运营设置（升级配置）</RailopsButton>}
            <ProjectRuntimeFields document={document} onChange={edit} disabled={busy} />
            <FormField label="服务项目（不是产品）">
              <div className="grid gap-2">{document.projects.map((project, i) => <div className="flex flex-wrap gap-2" key={i}>
                <Input aria-label={`项目标识 ${i + 1}`} className="!w-48" maxLength={64} value={project.key} placeholder="项目标识" onChange={e => {
                  const key = e.target.value
                  edit({ ...document, projects: document.projects.map((p, j) => i === j ? { ...p, key } : p), intake: { rules: document.intake.rules.map(r => r.project_key === project.key ? { ...r, project_key: key } : r) } })
                }} />
                <Input aria-label={`项目名称 ${i + 1}`} className="!w-48" maxLength={120} value={project.name} placeholder="方便识别的服务名称" onChange={e => edit({ ...document, projects: document.projects.map((p, j) => i === j ? { ...p, name: e.target.value } : p) })} />
                <IconButton icon={<Trash2Icon />} aria-label={`删除项目 ${i + 1}`} tooltip="删除项目（其规则需另行处理）" onClick={() => edit({ ...document, projects: document.projects.filter((_, j) => j !== i) })} />
              </div>)}</div>
              <RailopsButton onClick={() => edit({ ...document, projects: [...document.projects, { key: "", name: "" }] })}>添加服务项目</RailopsButton>
            </FormField>
            {document.intake.rules.map((rule, index) => {
              const change = (patch: Partial<typeof rule>) => edit({ ...document, intake: { rules: document.intake.rules.map((r, i) => i === index ? { ...r, ...patch } : r) } })
              return <div key={index} className="grid grid-cols-1 gap-3 border-t pt-3 sm:grid-cols-3">
                <FormField label={label("project_key")}><Select disabled={busy} className="w-full" value={rule.project_key || undefined} options={document.projects.filter(p => p.key).map(p => ({ value: p.key, label: p.name || p.key }))} onChange={value => change({ project_key: value })} /></FormField>
                <FormField label={label("channel")}><Select disabled={busy} className="w-full" value={rule.channel} options={["phone", "email", "monitoring_alert", "api", "webhook", "whatsapp", "chatbot_handoff"].map(value => ({ value, label: value === "phone" ? label("phone") : value }))} onChange={value => change({ channel: value })} /></FormField>
                <FormField label={label("ticket_type")}><Input maxLength={64} value={rule.ticket_type} onChange={e => change({ ticket_type: e.target.value })} /></FormField>
                <div className="sm:col-span-3"><FormField label={label("requiredFields")}><Checkbox.Group disabled={busy} value={rule.required_fields} options={["caller_name", "caller_phone", "customer_id", "product_id", "device_id", "service_region"].map(value => ({ value, label: label(value) }))} onChange={value => change({ required_fields: value as string[] })} /></FormField></div>
                <IconButton icon={<Trash2Icon />} aria-label={`删除规则 ${index + 1}`} tooltip={label("removeRule")} onClick={() => edit({ ...document, intake: { rules: document.intake.rules.filter((_, i) => i !== index) } })} />
              </div>
            })}
            <RailopsButton onClick={() => edit({ ...document, intake: { rules: [...document.intake.rules, { project_key: document.projects[0]?.key || "", channel: "phone", ticket_type: "", required_fields: [] }] } })}>{label("addRule")}</RailopsButton>
            <details><summary className="cursor-pointer text-sm">部署所需密钥引用（电话受理无需填写）</summary>
              <p className="my-2 text-sm text-muted-foreground">这里只填运维提供的 secret://名称，每行一个，不要填写密码。系统会检查当前公司和环境是否能读取对应密钥。</p>
              <Input.TextArea aria-label="密钥引用" rows={3} value={document.secret_refs.join("\n")} onChange={e => edit({ ...document, secret_refs: e.target.value.split("\n").filter(Boolean) })} />
            </details>
            <FormField label="修改说明" required><Input.TextArea maxLength={500} rows={2} value={note} placeholder="例如：电话咨询增加联系电话必填" onChange={e => { setNote(e.target.value); setDraft(null) }} /></FormField>
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
              <details className="my-2"><summary className="cursor-pointer">查看这版规则</summary><div className="mt-2 grid gap-1">
                {!version.document.intake?.rules?.length && <div>未配置受理规则</div>}
                {version.document.intake?.rules?.map((rule, i) => <div key={i}>{version.document.projects?.find(p => p.key === rule.project_key)?.name || rule.project_key} / {rule.channel === "phone" ? label("phone") : rule.channel} / {rule.ticket_type}：{rule.required_fields?.map(label).join("、") || "无额外必填资料"}</div>)}
              </div></details>
              <div className="mt-2 flex gap-2"><RailopsButton disabled={busy} onClick={() => copyVersion(version)}>载入为新草稿</RailopsButton>{version.activation && <RailopsButton disabled={busy} onClick={() => exportVersion(version)}>导出部署配置</RailopsButton>}</div>
            </div>)}</div>
            {view.next_before_id > 0 && <RailopsButton disabled={busy} onClick={() => void loadOlder()}>加载更早版本</RailopsButton>}
          </details>
        </>}
      </div>
    </StandardModal>
  </>
}
