"use client"

import { useCallback, useEffect, useRef, useState } from "react"
import { Settings2Icon } from "lucide-react"
import { IconButton, RailopsButton, StandardModal } from "@railops/ui"
import { useAuth } from "@/components/auth-provider"
import { useAppLocale, useI18n } from "@/i18n/provider"
import { applyProjectConfiguration, getProjectConfiguration, listRetentionApprovals, previewProjectConfigurationImpact, reviewRetentionApproval, saveProjectConfiguration, submitRetentionApproval, validateProjectConfiguration, type ConfigurationImpact, type ConfigurationReport, type ConfigurationVersion, type ConfigurationView, type ProjectConfiguration, type RetentionApproval } from "@/lib/api/project-configuration"
import { upgradeProjectConfiguration } from "@/lib/api/project-configuration"
import { editableProjectConfiguration as editableDocument, restoreProjectConfiguration, withManagedMailSecrets as documentWithManagedMailSecrets } from "@/lib/project-configuration-editing"
import { ProjectRuntimeFields } from "./project-runtime-fields"
import { displayTicketWorkflowVersionNote } from "@/app/enterprise/workflow/ticket-status-workflow-i18n"

const environments: Record<string, string> = {
  development: "environmentDevelopment",
  integration: "environmentIntegration",
  staging: "environmentStaging",
  production: "environmentProduction",
}

export function ProjectConfigurationEditor() {
  const { session } = useAuth()
  const { locale } = useAppLocale()
  const translate = useI18n()
  const pc = (key: string, values?: Record<string, string | number>) => translate(`projectConfiguration.${key}`, values)
  const versionNote = (key: string, values?: Record<string, string | number>) => translate(`ticketStatusWorkflow.versionNotes.${key}`, values)
  const [open, setOpen] = useState(false)
  const [view, setView] = useState<ConfigurationView | null>(null)
  const [document, setDocument] = useState<ProjectConfiguration | null>(null)
  const [mailSecrets, setMailSecrets] = useState({ smtpPassword: "", imapPassword: "" })
  const [note, setNote] = useState("")
  const [draft, setDraft] = useState<ConfigurationVersion | null>(null)
  const [report, setReport] = useState<ConfigurationReport | null>(null)
  const [impact, setImpact] = useState<ConfigurationImpact | null>(null)
  const [impactSignature, setImpactSignature] = useState("")
  const [busy, setBusy] = useState(false)
  const [loadingOlder, setLoadingOlder] = useState(false)
  const [error, setError] = useState("")
  const [message, setMessage] = useState("")
  const [approvals, setApprovals] = useState<RetentionApproval[]>([])
  const request = useRef({ signature: "", key: "" })
  const historySentinel = useRef<HTMLDivElement | null>(null)

  function edit(next: ProjectConfiguration) {
    setDocument(next); setDraft(null); setReport(null); setImpact(null); setImpactSignature(""); setMessage(""); setError("")
  }
  function changeMailSecrets(next: { smtpPassword: string; imapPassword: string }) {
    setMailSecrets(next); setDraft(null); setReport(null); setImpact(null); setImpactSignature("")
  }
  async function load() {
    setOpen(true); setBusy(true); setError(""); setMessage(""); setView(null); setDocument(null); setDraft(null); setReport(null); setImpact(null); setImpactSignature(""); setNote(""); setApprovals([]); setMailSecrets({ smtpPassword: "", imapPassword: "" })
    try {
      const result = await getProjectConfiguration()
      if (!result.success || !result.data) throw new Error(result.error?.message || pc("loadFailed"))
      setView(result.data); setDocument(editableDocument(result.data.document))
      const productionVersions = result.data.versions.filter(v => v.document.environment === "production")
      const approvalResults = await Promise.all(productionVersions.map(v => listRetentionApprovals(v.id)))
      setApprovals(approvalResults.flatMap(a => a.success && a.data ? a.data : []))
    } catch (e) { setError(e instanceof Error ? e.message : pc("loadFailed")) }
    finally { setBusy(false) }
  }
  async function validate() {
    if (!document) return
    setBusy(true); setError(""); setMessage("")
    try {
      const result = await validateProjectConfiguration(documentWithManagedMailSecrets(document, mailSecrets))
      if (!result.success || !result.data) throw new Error(result.error?.message || pc("checkFailed"))
      setReport(result.data)
    } catch (e) { setError(e instanceof Error ? e.message : pc("checkFailed")) }
    finally { setBusy(false) }
  }
  async function preview() {
    if (!document || !draft || !report?.valid) return
    setBusy(true); setError(""); setMessage("")
    try {
      const result = await previewProjectConfigurationImpact(documentWithManagedMailSecrets(document, mailSecrets))
      if (!result.success || !result.data) throw new Error(result.error?.message || pc("impactFailed"))
      setImpact(result.data)
      setImpactSignature(JSON.stringify(document))
      setMessage(pc("impactDone", { scanned: result.data.summary.scanned, affected: result.data.summary.affected }))
    } catch (e) { setError(e instanceof Error ? e.message : pc("impactFailed")) }
    finally { setBusy(false) }
  }
  async function save() {
    if (!document || !view) return
    setBusy(true); setError(""); setMessage("")
    try {
      // 修改说明不再让用户填写：备注由系统生成，历史列表仍然看得出这版是怎么来的。
      const effectiveNote = note.trim() || (view.active_version_id ? versionNote("adjustedOperationalConfig", { version: view.active_version_id }) : versionNote("adoptExistingSettings"))
      // Include write-only credential input in the idempotency signature. The
      // values never leave this request except through the dedicated password
      // fields, but changing one must create a new request key.
      const signature = JSON.stringify([document, effectiveNote, view.active_version_id, mailSecrets])
      if (request.current.signature !== signature) request.current = { signature, key: crypto.randomUUID() }
      const result = await saveProjectConfiguration(documentWithManagedMailSecrets(document, mailSecrets), view.active_version_id, effectiveNote, request.current.key, mailSecrets)
      if (!result.success || !result.data) throw new Error(result.error?.message || pc("draftSaveFailed"))
      setDraft(result.data)
      const a = await listRetentionApprovals(result.data.id); if (a.success && a.data) setApprovals(a.data)
      setView({ ...view, versions: [result.data, ...view.versions.filter(v => v.id !== result.data.id)] })
      setMessage(pc("draftSaved", { id: result.data.id }))
    } catch (e) { setError(e instanceof Error ? e.message : pc("draftSaveFailed")) }
    finally { setBusy(false) }
  }
  async function submitRetention(version: ConfigurationVersion) {
    setBusy(true); setError(""); setMessage("")
    try { const result = await submitRetentionApproval(version.id); if (!result.success || !result.data) throw new Error(result.error?.message || pc("submitFailed")); setApprovals(result.data); setMessage(pc("retentionSubmitted", { id: version.id })) }
    catch (e) { setError(e instanceof Error ? e.message : pc("submitFailed")) } finally { setBusy(false) }
  }
  async function reviewRetention(approval: RetentionApproval, approved: boolean) {
    const comment = window.prompt(approved ? pc("approveComment") : pc("rejectReason")) ?? ""
    if (!approved && !comment.trim()) return
    setBusy(true); setError("")
    try { const result = await reviewRetentionApproval(approval.id, approved, comment); if (!result.success) throw new Error(result.error?.message || pc("reviewFailed")); const current = await listRetentionApprovals(approval.version_id); if (current.success && current.data) setApprovals(current.data); setMessage(approved ? pc("approvalApproved", { project: approval.project_key }) : pc("approvalRejected", { project: approval.project_key })) }
    catch (e) { setError(e instanceof Error ? e.message : pc("reviewFailed")) } finally { setBusy(false) }
  }
  async function apply() {
    if (!document || !view || !draft) return
    if (!impact || impactSignature !== JSON.stringify(document)) { setError(pc("configurationChanged")); return }
    const targetDraft = draft
    setBusy(true); setError(""); setMessage("")
    try {
      const result = await applyProjectConfiguration(targetDraft.id)
      if (!result.success || !result.data) throw new Error(result.error?.message || pc("applyFailed"))
      // Re-read the active pointer: a retried old application must not present
      // itself as current if another version was applied in the meantime.
      const current = await getProjectConfiguration()
      if (!current.success || !current.data) throw new Error(pc("refreshFailed"))
      setView(current.data); setDocument(editableDocument(current.data.document)); setDraft(null); setReport(null); setNote(""); setMailSecrets({ smtpPassword: "", imapPassword: "" })
      setMessage(pc("activeVersionApplied", { id: current.data.active_version_id }))
    } catch (e) { setError(e instanceof Error ? e.message : pc("applyFailed")) }
    finally { setBusy(false) }
  }
  function copyVersion(version: ConfigurationVersion) {
    const copy = restoreProjectConfiguration(version.document, view?.document)
    edit(copy); setNote(versionNote("adjustedFromPrefix", { version: version.id, rest: version.note }).slice(0, 500))
    const retainedRuntime = !version.document.runtime && copy.runtime ? pc("retainedRuntime") : ""
    setMessage(`${pc("loadedDraft", { id: version.id })}${retainedRuntime}`)
  }
  async function upgrade() {
    setBusy(true); setError("")
    try {
      const result = await upgradeProjectConfiguration()
      if (!result.success || !result.data) throw new Error(result.error?.message || pc("loadFailed"))
      edit(editableDocument(result.data)); setNote(versionNote("adoptExistingSettings"))
      setMessage(pc("loadedExisting"))
    } catch (e) { setError(e instanceof Error ? e.message : pc("loadFailed")) }
    finally { setBusy(false) }
  }
  const loadOlder = useCallback(async () => {
    if (!view?.next_before_id || busy || loadingOlder) return
    const beforeID = view.next_before_id
    setLoadingOlder(true); setError("")
    try {
      const result = await getProjectConfiguration(beforeID)
      if (!result.success || !result.data) throw new Error(result.error?.message || translate("projectConfiguration.historyLoadFailed"))
      const productionVersions = result.data.versions.filter(v => v.document.environment === "production")
      const approvalResults = await Promise.all(productionVersions.map(v => listRetentionApprovals(v.id)))
      const newApprovals = approvalResults.flatMap(a => a.success && a.data ? a.data : [])
      setApprovals(current => [...current.filter(a => !productionVersions.some(v => v.id === a.version_id)), ...newApprovals])
      setView(current => {
        if (!current) return current
        const newVersions = result.data!.versions.filter(v => !current.versions.some(existing => existing.id === v.id))
        return { ...current, next_before_id: result.data!.next_before_id, versions: [...current.versions, ...newVersions] }
      })
    } catch (e) { setError(e instanceof Error ? e.message : translate("projectConfiguration.historyLoadFailed")) }
    finally { setLoadingOlder(false) }
  }, [busy, loadingOlder, translate, view])
  useEffect(() => {
    const sentinel = historySentinel.current
    if (!sentinel || !view?.next_before_id) return
    const observer = new IntersectionObserver(entries => {
      if (entries.some(entry => entry.isIntersecting)) void loadOlder()
    }, { threshold: 0.1 })
    observer.observe(sentinel)
    return () => observer.disconnect()
  }, [loadOlder, view?.next_before_id])

  if (!session?.permissions?.includes("ticket.update")) return null
  function exportVersion(version: ConfigurationVersion) {
    const blob = new Blob([JSON.stringify({ version_id: version.id, digest: version.digest, document: version.document }, null, 2)], { type: "application/json" })
    const url = URL.createObjectURL(blob)
    const a = window.document.createElement("a")
    a.href = url; a.download = `project-configuration-v${version.id}.json`; a.click(); URL.revokeObjectURL(url)
  }
  return <>
    <IconButton icon={<Settings2Icon />} tooltip={pc("operationsConfig")} aria-label={pc("operationsConfig")} onClick={() => void load()} />
    <StandardModal open={open} width={880} title={pc("editorTitle")} styles={{ body: { maxHeight: "calc(100dvh - 200px)", overflowY: "auto" } }} onCancel={() => !busy && setOpen(false)} footer={<div className="flex flex-wrap justify-end gap-2">
      <RailopsButton disabled={busy} onClick={() => void load()}>{pc("reload")}</RailopsButton>
      <RailopsButton disabled={busy || !document} onClick={() => void save()}>{pc("saveDraft")}</RailopsButton>
      <RailopsButton disabled={busy || !document} onClick={() => void validate()}>{pc("validateConfig")}</RailopsButton>
      <RailopsButton disabled={busy || !document || !draft || !report?.valid} onClick={() => void preview()}>{pc("impactPreview")}</RailopsButton>
      {view?.deployment_managed
        ? <RailopsButton disabled={busy || !document || !draft || !report?.valid || !impact || impactSignature !== JSON.stringify(document)} onClick={() => draft && exportVersion(draft)}>{pc("exportDraft")}</RailopsButton>
        : <RailopsButton disabled={busy || !document || !draft || !report?.valid || !impact || impactSignature !== JSON.stringify(document)} onClick={() => void apply()}>{pc("applyDraft")}</RailopsButton>}
    </div>}>
      <div className="grid gap-4">
        {error && <div role="alert" className="text-sm text-destructive">{error}</div>}
        {message && <div role="status" className="rounded border p-3 text-sm">{message}</div>}
          {view && document && <>
          <div className="text-sm text-muted-foreground">{pc(environments[document.environment]) || document.environment} · {view.active_version_id ? pc("activeVersion", { id: view.active_version_id }) : pc("legacyConfig")}</div>
          <p className="text-sm">{pc("draftIntro")}</p>
          {view.deployment_managed && <p className="rounded border p-3 text-sm">{pc("deploymentManaged")}</p>}
          <fieldset disabled={busy} className="grid min-w-0 gap-4">
            {!document.runtime && <RailopsButton onClick={() => void upgrade()}>{pc("loadExisting")}</RailopsButton>}
            <ProjectRuntimeFields document={document} onChange={edit} disabled={busy} mailSecrets={mailSecrets} onMailSecretsChange={changeMailSecrets} />
          </fieldset>
          {report && <div role="status" className={`rounded border p-3 text-sm ${report.valid ? "text-green-700" : "text-destructive"}`}>
            {report.valid ? pc("validationPassed") : report.issues.map((issue, i) => <div key={i}>{issue.path}: {issue.message}</div>)}
          </div>}
          {impact && <div role="status" className="rounded border p-3 text-sm">
            <div className="font-medium">{pc("impactTitle")}</div>
            <div className="mt-1">{pc("impactSummary", { scanned: impact.summary.scanned, affected: impact.summary.affected, shortened: impact.summary.shortened, extended: impact.summary.extended, newlyBreached: impact.summary.newly_breached })}</div>
            {impact.items.length > 0 && <div className="mt-2 grid gap-1">{impact.items.slice(0, 20).map(item => <div key={`${item.ticket_id}-${item.metric}`} className="text-muted-foreground">{item.ticket_no || item.ticket_id} · {item.metric} · {item.delta_minutes > 0 ? pc("later", { minutes: item.delta_minutes }) : pc("earlier", { minutes: Math.abs(item.delta_minutes) })} · {item.reason}</div>)}</div>}
          </div>}
          <details><summary className="cursor-pointer font-medium">{pc("historyDrafts")}</summary>
            <div className="mt-3 grid gap-3">{view.versions.map(version => <div key={version.id} className="rounded border p-3 text-sm">
              <div>V{version.id} · {version.id === view.active_version_id ? pc("active") : version.activation ? pc("previouslyActive") : version.created_by === 0 ? pc("migrationBaseline") : pc("unappliedDraft")} · {new Date(version.created_at).toLocaleString()}</div>
              <div className="my-2 break-words">{displayTicketWorkflowVersionNote(version.note, locale)}</div>
              {version.document.environment === "production" && <div className="my-2 rounded bg-muted/40 p-2">
                <div className="mb-2 font-medium">{pc("retentionApproval")}</div>
                {approvals.filter(a => a.version_id === version.id).length === 0
                  ? <span className="text-muted-foreground">{pc("notSubmitted")}</span>
                  : approvals.filter(a => a.version_id === version.id).map(a => <div key={a.id} className="flex flex-wrap items-center gap-2"><span>{a.project_key}: {a.status}</span>{a.reviewed_by_name && <span className="text-muted-foreground">({a.reviewed_by_name})</span>}{a.can_approve && <><RailopsButton disabled={busy} onClick={() => void reviewRetention(a, true)}>{pc("approve")}</RailopsButton><RailopsButton disabled={busy} onClick={() => void reviewRetention(a, false)}>{pc("reject")}</RailopsButton></>}</div>)}
                {!version.activation && version.id !== view.active_version_id && <RailopsButton disabled={busy} onClick={() => void submitRetention(version)}>{pc("submitRetention")}</RailopsButton>}
              </div>}
              {version.document.runtime && <details><summary>{pc("viewRuntimeSnapshot")}</summary><pre className="max-h-64 overflow-auto whitespace-pre-wrap break-all">{JSON.stringify(version.document.runtime, null, 2)}</pre></details>}
              <div className="text-muted-foreground">{pc("createdBy")}: {version.created_by_name || version.created_by || pc("systemMigration")}{version.activation ? `; ${pc("appliedBy")}: ${version.activation.applied_by_name || version.activation.applied_by}; ${pc("appliedAt")}: ${new Date(version.activation.applied_at).toLocaleString()}` : ""}</div>
              <div className="mt-2 flex gap-2"><RailopsButton disabled={busy} onClick={() => copyVersion(version)}>{pc("loadAsDraft")}</RailopsButton>{version.activation && <RailopsButton disabled={busy} onClick={() => exportVersion(version)}>{pc("exportDeployment")}</RailopsButton>}</div>
            </div>)}</div>
            {view.next_before_id > 0 && <div ref={historySentinel} className="flex min-h-8 items-center justify-center text-sm text-muted-foreground">{loadingOlder ? pc("loadingEarlier") : pc("loadEarlier")}</div>}
          </details>
        </>}
      </div>
    </StandardModal>
  </>
}
