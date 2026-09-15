"use client"

import { useEffect, useRef, useState } from "react"
import { Input } from "antd"
import { useAuth } from "@/components/auth-provider"
import { FormField, RailopsButton, SelectField, StandardModal, StatusTag } from "@railops/ui"
import { fetchTicketAggregate } from "@/lib/api/enterprise-tickets"
import type { TicketAggregateDTO } from "@/lib/api/types"
import { caseStatusLabel } from "@/lib/ticket-case-labels"
import { formatDateTime } from "@/lib/utils"
import { caseTypes, searchDuplicateTickets, type DuplicateCandidate, governanceLabel as g, fetchGovernance, saveGovernance, type CaseType, type PriorityFacts, type GovernanceView, type GovernanceCommand, type PriorityPolicy, fetchClassificationPolicy, previewPriorityChange, type PriorityPreview } from "@/lib/ticket-governance"

export function TicketClassificationFields({ value, onChange, disabled = false, policy, keepPriority, priorityOverride = "", priorityReason = "", onPriorityChange, onPriorityReasonChange }: { value: string; onChange: (value: CaseType) => void; disabled?: boolean; policy?: PriorityPolicy; keepPriority?: string; priorityOverride?: string; priorityReason?: string; onPriorityChange?: (value: string) => void; onPriorityReasonChange?: (value: string) => void }) {
  const { session } = useAuth()
  const canAdjust = Boolean(session?.permissions?.includes("ticket.view") && (session.permissions.includes("ticket.update") || session.roles?.some(role => ["tenant_owner", "tenant_admin", "service_manager"].includes(role))))
  const [currentPolicy, setCurrentPolicy] = useState<PriorityPolicy | undefined>(policy)
  useEffect(() => { let active = true; if (!policy) void fetchClassificationPolicy().then(r => { if (active && r.success && r.data) setCurrentPolicy(r.data) }).catch(() => {}); return () => { active = false } }, [policy])
  const defaults = policy?.defaults || currentPolicy?.defaults
  const defaultLevel = defaults?.[value as CaseType]
  const level = keepPriority || priorityOverride || defaultLevel
  return <div className="space-y-3" data-testid="classification-fields">
    <SelectField label={g("classification")} required selectProps={{ value: value || undefined, disabled, options: caseTypes.map(t => ({ value: t, label: g(`type.${t}`) })), onChange: v => onChange(v as CaseType) }} />
    {onPriorityChange ? <>
      <SelectField label={g("defaultPriority")} selectProps={{ value: level, disabled: disabled || !canAdjust || !defaultLevel, options: ["p1", "p2", "p3", "p4"].map(p => ({ value: p, label: g(`level.${p}`) })), onChange: p => onPriorityChange(p === defaultLevel ? "" : p) }} />
      {!canAdjust ? <p className="text-xs text-muted-foreground">{g("createPriorityReadonly")}</p> : null}
      {priorityOverride ? <FormField label={g("changeReason")} required><Input.TextArea aria-label={g("changeReason")} value={priorityReason} disabled={disabled} rows={2} maxLength={2000} onChange={e => onPriorityReasonChange?.(e.target.value)} /></FormField> : null}
    </> : null}
  </div>
}

export function TicketGovernancePanel({ aggregate, onSaved, active = true }: { aggregate: TicketAggregateDTO; onSaved: (value: TicketAggregateDTO) => void | Promise<void>; active?: boolean }) {
  const id = aggregate.ticket.id
  const [view, setView] = useState<GovernanceView | null>(null)
  const [preview, setPreview] = useState<PriorityPreview | null>(null)
  const [previewError, setPreviewError] = useState("")
  const [previewAttempt, setPreviewAttempt] = useState(0)
  const [loadError, setLoadError] = useState("")
  const [action, setAction] = useState("")
  const [reason, setReason] = useState("")
  const [priority, setPriority] = useState("p2")
  const [caseType, setCaseType] = useState("user_case")
  const [target, setTarget] = useState<number>()
  const [search, setSearch] = useState("")
  const [candidates, setCandidates] = useState<DuplicateCandidate[]>([])
  const [selectedParent, setSelectedParent] = useState<DuplicateCandidate>()
  const [searching, setSearching] = useState(false)
  const [reference, setReference] = useState(0)
  const [revision, setRevision] = useState(0)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
  const [submitted, setSubmitted] = useState(false)
  const mounted = useRef(false)
  const pending = useRef<GovernanceCommand | null>(null)
  const committed = useRef(false)
  const inFlight = useRef(false)
  const searchSequence = useRef(0)
  const terminal = ["closed", "cancelled"].includes(aggregate.ticket.case_status || aggregate.ticket.status)

  useEffect(() => {
    let cancelled = false
    mounted.current = active
    if (active) void fetchGovernance(id).then(r => { if (cancelled || !mounted.current) return; if (r.success && r.data && r.data.ticket_id === id) { setView(r.data); setLoadError("") } else setLoadError(r.error?.message || g("loadFailed")) }).catch(e => { if (!cancelled && mounted.current) setLoadError(e instanceof Error ? e.message : g("loadFailed")) })
    return () => { cancelled = true; mounted.current = false }
  }, [id, active, aggregate.ticket.updated_at])

  const previewPriority = priority
  useEffect(() => {
    let cancelled = false
    setPreview(null); setPreviewError("")
    if (action === "override" && previewPriority) {
      void previewPriorityChange(id, previewPriority).then(r => { if (cancelled) return; if (r.success && r.data) { setPreview(r.data); setPreviewError("") } else setPreviewError(r.error?.message || g("loadFailed")) }).catch(e => { if (!cancelled) setPreviewError(String(e)) })
    }
    return () => { cancelled = true }
  }, [id, action, previewPriority, previewAttempt])
  async function reload() {
    const result = await fetchGovernance(id)
    if (!result.success || !result.data || result.data.ticket_id !== id) throw new Error(result.error?.message || g("loadFailed"))
    if (mounted.current) { setView(result.data); setLoadError("") }
  }
  useEffect(() => {
    if (!active || action !== "duplicate_child" || submitted) return
    let cancelled = false
    const sequence = ++searchSequence.current
    setSearching(true)
    const timer = setTimeout(() => {
      void searchDuplicateTickets(id, search.trim()).then(result => {
        if (cancelled || sequence !== searchSequence.current) return
        if (!result.success) throw new Error(result.error?.message || g("loadFailed"))
        setCandidates(result.data ?? [])
      }).catch(e => {
        if (!cancelled && sequence === searchSequence.current) setError(e instanceof Error ? e.message : g("loadFailed"))
      }).finally(() => {
        if (!cancelled && sequence === searchSequence.current) setSearching(false)
      })
    }, search ? 250 : 0)
    return () => { cancelled = true; clearTimeout(timer) }
  }, [action, id, search, active, submitted])
  function searchParents(value: string) {
    searchSequence.current++
    setSearch(value); setCandidates([]); setTarget(undefined); setSelectedParent(undefined)
    setSearching(true); setError("")
  }
  function open(next: string, ref = 0) {
    if (!view || inFlight.current) return
    pending.current = null; committed.current = false
    setPreview(null); setPreviewError("")
    setAction(next); setReference(ref); setRevision(view.revision); setReason(""); setError(""); setSubmitted(false)
    setCaseType(view.case_type || "user_case"); setPriority(view.priority || "p2"); setTarget(undefined); setSelectedParent(undefined); setCandidates([]); setSearch("")
  }
  async function save() {
    if (!view || inFlight.current || !mounted.current) return
    if (action !== "duplicate_child" && !reason.trim()) { setError(g("reasonRequired")); return }
    if (!submitted && action === "override" && preview?.priority !== priority) { setError(previewError || g("loading")); return }
    if (action === "duplicate_child" && (!target || !selectedParent)) { setError(g("chooseTicket")); return }
    inFlight.current = true; setBusy(true); setSubmitted(true); setError("")
    if (!pending.current) pending.current = { action, expected_revision: revision, operation_key: crypto.randomUUID(), reason: reason.trim(), priority, case_type: caseType, relation_id: reference, target_id: target, target_revision: action === "duplicate_child" ? selectedParent?.revision : undefined }
    try {
      if (!committed.current) { const result = await saveGovernance(id, pending.current); if (!result.success) throw new Error(result.error?.message || g("failed")); committed.current = true }
      await reload()
      const updated = await fetchTicketAggregate(id)
      if (!updated.success || !updated.data || updated.data.ticket.id !== id) throw new Error(updated.error?.message || g("loadFailed"))
      if (!mounted.current) return
      await onSaved(updated.data); if (mounted.current) setAction("")
    } catch (e) { if (mounted.current) setError(e instanceof Error ? e.message : g("failed")) }
    finally { inFlight.current = false; if (mounted.current) setBusy(false) }
  }
  if (!active) return null
  if (!view) return <section className="rounded-md border p-4 text-sm">{loadError || g("loading")} {loadError ? <RailopsButton onClick={() => void reload().catch(e => setLoadError(String(e)))}>{g("retry")}</RailopsButton> : null}</section>
  const stale = !submitted && revision !== view.revision
  const formDisabled = busy || submitted || stale
  const reasonLabel = action === "duplicate_child" ? "duplicateNote" : action === "override" ? "changeReason" : action === "unlink" ? "relationReason" : "reason"
  const children = view.relations.filter(r => r.kind === "parent" && r.source_id === id)
  const ended = children.filter(r => ["closed", "cancelled"].includes(r.status)).length
  const highest = children.filter(r => !["closed", "cancelled"].includes(r.status)).map(r => r.priority).filter(Boolean).sort()[0]
  return <section className="space-y-4 rounded-md border border-border bg-card p-4" data-testid="ticket-governance">
    <div className="flex flex-wrap items-center justify-between gap-2"><h3 className="text-sm font-semibold">{g("title")}</h3><StatusTag tone={view.priority === "p1" ? "error" : "blue"}>{view.priority ? g(`level.${view.priority}`) : "—"}</StatusTag></div>
    <dl className="grid gap-2 text-sm sm:grid-cols-2"><div><dt className="text-muted-foreground">{g("classification")}</dt><dd>{view.case_type ? g(`type.${view.case_type}`) : g("unclassified")}</dd></div><div><dt className="text-muted-foreground">{g("prioritySource")}</dt><dd>{g(view.overridden ? "manualSource" : view.legacy || view.priority !== view.policy.defaults[view.case_type as CaseType] ? "historicalSource" : "classificationSource")}</dd></div></dl>
    <div className="flex flex-wrap gap-2">{!terminal && view.can_manage ? ["classify", "override"].map(a => <RailopsButton key={a} size="small" onClick={() => open(a)}>{g(`action.${a}`)}</RailopsButton>) : null}</div>
    <div className="flex flex-wrap items-center justify-between gap-2 border-t pt-3">
      <h4 className="text-sm font-semibold">{g("relations")}</h4>
      <div className="flex flex-wrap gap-2">
        {view.can_associate_duplicate ? <RailopsButton size="small" onClick={() => open("duplicate_child")}>{g("action.duplicate_child")}</RailopsButton> : null}
        {view.can_manage && !terminal ? <RailopsButton size="small" href={`/enterprise/tickets?parent_ticket_id=${id}`}>{g("createChild")}</RailopsButton> : null}
      </div>
    </div>
    {view.merge?.into ? <p className="rounded-md bg-muted p-3 text-sm">{g("mergedInto")} <a className="underline" href={`/enterprise/ticket-workbench?ticket_id=${view.merge.into.other_id}`}>{view.merge.into.ticket_no} · {view.merge.into.title}</a></p> : null}
    {view.merge?.sources.length ? <div className="space-y-3 border-t pt-3"><h4 className="text-sm font-semibold">{g("mergeSources")} ({view.merge.sources.length})</h4>{view.merge.sources.map(source => <details key={source.ticket_id} className="rounded border p-3 text-sm" open><summary className="cursor-pointer"><a className="underline" href={`/enterprise/ticket-workbench?ticket_id=${source.ticket_id}`}>{source.ticket_no}</a> · {source.title}</summary><p className="mt-2 text-xs text-muted-foreground">{source.author} · {formatDateTime(source.created_at)}</p><p className="whitespace-pre-wrap break-words">{source.description}</p><details className="mt-2"><summary>{g("mergeTimeline")} ({source.timeline.length})</summary>{source.timeline.map((item, index) => <div key={`${item.id}-${index}`} className="border-b py-2"><p className="text-xs text-muted-foreground">{item.actor} · {formatDateTime(item.timestamp)}</p><p className="whitespace-pre-wrap break-words">{item.content}</p></div>)}</details>{source.assets.length ? <div className="mt-2"><p>{g("mergeAssets")}</p>{source.assets.map(asset => <p key={asset.id} className="break-words">{asset.file_name} · {asset.uploaded_by} · {formatDateTime(asset.uploaded_at)}</p>)}</div> : null}</details>)}</div> : null}
    {children.length ? <p className="text-xs text-muted-foreground">{g("visibleChildren")}: {ended}/{children.length} · {g("highest")}: {highest?.toUpperCase() || "—"} · {g("overdue")}: {children.filter(r => r.sla_breached).length}</p> : null}
    {terminal && children.some(child => !["closed", "cancelled"].includes(child.status)) ? <p className="text-sm text-amber-700">{g("reopenedChild")}</p> : null}
    {view.relations.some(r => r.kind === "parent" && r.target_id === id && ["restored", "resolved", "closure_pending", "closed"].includes(r.status)) && !terminal ? <p className="text-sm text-amber-700">{g("parentRestored")}</p> : null}
    {highest && highest < view.priority ? <p className="text-sm text-amber-700">{g("childPriorityWarning")}</p> : null}
    {!view.relations.length ? <p className="text-sm text-muted-foreground">{g("noRelations")}</p> : view.relations.map(r => <div key={r.id} className="space-y-1 border-b pb-2 text-sm"><div className="flex flex-wrap items-center justify-between gap-2"><a className="underline" href={`/enterprise/ticket-workbench?ticket_id=${r.other_id}`}>{r.ticket_no} · {r.title}</a>{r.can_remove ? <RailopsButton size="small" onClick={() => open("unlink", r.id)}>{g("action.unlink")}</RailopsButton> : null}</div><p>{g(`relation.${r.kind === "parent" && r.target_id === id ? "child_of" : r.kind === "causes" && r.target_id === id ? "caused_by" : r.kind === "duplicate" && r.target_id === id ? "duplicates" : r.kind === "merged" && r.target_id === id ? "merged_sources" : r.kind}`)} · {r.priority.toUpperCase()} · {caseStatusLabel(r.status)} · {g("owner")}: {r.owner_id || "—"}</p><p className="text-muted-foreground">{r.reason}</p></div>)}
    {view.proposals.length ? <details className="border-t pt-3 text-sm"><summary className="cursor-pointer">{g("historicalProposals")} ({view.proposals.length})</summary><p className="my-2 text-muted-foreground">{g("historicalProposalHelp")}</p>{view.proposals.map(p => <div key={p.id} className="space-y-1 border-b py-2"><p>#{p.id} · {p.priority.toUpperCase()} · {g(p.status === "pending" ? "historicalPending" : `proposal.${p.status}`)}</p><p>{p.reason}</p>{p.review_reason ? <p>{g("reviewReason")}: {p.review_reason}</p> : null}</div>)}</details> : null}
    <details className="border-t pt-3 text-sm"><summary className="cursor-pointer">{g("history")} ({view.history.length})</summary><div className="mt-3 space-y-3">{view.history.map(h => <div key={h.id} className="border-b pb-2"><p>{g(`action.${h.action}`)} · {formatDateTime(h.created_at)} · {g("operator")} #{h.actor_id}</p><p className="whitespace-pre-wrap">{h.reason}</p><details><summary>{g("decisionDetails")}</summary><DecisionDetails details={h.details} /></details></div>)}</div></details>
    <StandardModal open={Boolean(action)} title={g(`action.${action || "classify"}`)} onCancel={() => { if (!busy) { setAction(""); void reload().catch(() => {}) } }} footer={<><RailopsButton disabled={busy} onClick={() => { setAction(""); void reload().catch(() => {}) }}>{g("cancel")}</RailopsButton><RailopsButton variant="primary" loading={busy} disabled={busy || stale || (action === "duplicate_child" && !submitted && !selectedParent) || (action === "override" && !submitted && preview?.priority !== priority)} onClick={() => void save()}>{committed.current ? g("refresh") : g(action === "duplicate_child" ? "confirmParentLink" : "save")}</RailopsButton></>}>
      <div className="space-y-4">
        {action === "classify" ? <TicketClassificationFields value={caseType} onChange={setCaseType} disabled={formDisabled} policy={view.policy} keepPriority={view.overridden ? view.priority : undefined} /> : null}
        {action === "override" ? <SelectField label={g("newPriority")} selectProps={{ value: priority, disabled: formDisabled, options: ["p1", "p2", "p3", "p4"].map(p => ({ value: p, label: g(`level.${p}`) })), onChange: setPriority }} /> : null}
        {action === "duplicate_child" ? <SelectField label={g("parentTicket")} required selectProps={{
          "aria-label": g("parentTicket"), value: target, disabled: formDisabled, showSearch: true,
          placeholder: g("parentSearchPlaceholder"), filterOption: false, loading: searching,
          onSearch: searchParents, notFoundContent: g(searching ? "loading" : "noParentTickets"),
          options: (selectedParent && !candidates.some(c => c.other_id === selectedParent.other_id) ? [selectedParent, ...candidates] : candidates).map(c => ({ value: c.other_id, label: `${c.ticket_no} · ${c.title}` })),
          onChange: v => { const candidate = candidates.find(c => c.other_id === Number(v)); setTarget(candidate?.other_id); setSelectedParent(candidate); setError("") },
        }} /> : null}
        {action === "override" ? <div className="text-xs text-muted-foreground">{preview?.deadline ? <p>{g("slaDeadline")}: {formatDateTime(preview.deadline)}</p> : null}{preview?.overdue ? <p className="text-amber-700">{g("previewOverdue")}</p> : null}{!preview && !previewError ? <p>{g("loading")}</p> : null}{previewError ? <p role="alert">{previewError} <RailopsButton onClick={() => setPreviewAttempt(n => n + 1)}>{g("retry")}</RailopsButton></p> : null}</div> : null}
        {action !== "duplicate_child" ? <FormField label={g(reasonLabel)} required><Input.TextArea aria-label={g(reasonLabel)} value={reason} disabled={formDisabled} rows={2} maxLength={2000} onChange={e => { setReason(e.target.value); setError("") }} /></FormField> : null}
        {stale ? <p role="alert" className="text-sm text-destructive">{g("stale")}</p> : null}{error ? <p role="alert" className="text-sm text-destructive">{error}</p> : null}{submitted && error ? <p className="text-xs text-muted-foreground">{g("retryHelp")}</p> : null}
      </div>
    </StandardModal>
  </section>
}


function DecisionDetails({ details }: { details: Record<string, unknown> }) {
  const before = (details.before ?? {}) as Record<string, unknown>
  const after = (details.after ?? {}) as Record<string, unknown>
  let facts: Partial<PriorityFacts> = {}
  try { facts = JSON.parse(String(after.PriorityFactsJSON || "{}")) } catch { /* historical record */ }
  if (details.sla_snapshot) {
    const snapshot = details.sla_snapshot as Record<string, unknown>
    const date = (key: string) => snapshot[key] ? formatDateTime(String(snapshot[key])) : "—"
    return <dl className="mt-2 space-y-1 text-xs">
      <div><dt>{g("relationTiming")}</dt><dd>{g("clockUnchanged")}</dd></div>
      <div><dt>{g("originalCreated")}</dt><dd>{date("created_at")}</dd></div>
      <div><dt>{g("slaDeadline")}</dt><dd>{date("previous_sla_deadline")} → {date("sla_deadline")}</dd></div>
      <div><dt>{g("acceptDeadline")}</dt><dd>{date("previous_accept_deadline")} → {date("accept_deadline")}</dd></div>
      <div><dt>{g("timingAtAssociation")}</dt><dd>{g(snapshot.resolution_overdue ? "resolutionOverdue" : "resolutionNotOverdue")} · {g(snapshot.accept_overdue ? "acceptOverdue" : "acceptNotOverdue")}</dd></div>
    </dl>
  }
  return <dl className="mt-2 space-y-1 text-xs">
    <div><dt>{g("classification")}</dt><dd>{before.CaseType ? g(`type.${before.CaseType}`) : "—"} → {after.CaseType ? g(`type.${after.CaseType}`) : "—"}</dd></div>
    <div><dt>{g("newPriority")}</dt><dd>{String(before.PriorityLevel || "—").toUpperCase()} → {String(after.PriorityLevel || "—").toUpperCase()}</dd></div>
    <div><dt>{g("suggested")}</dt><dd>{String(after.PrioritySuggested || "—").toUpperCase()} · {String(after.PriorityExplanation || "")}</dd></div>
    <div><dt>{g("evidence")}</dt><dd>{facts.evidence || "—"}</dd></div>
    {facts.root_cause ? <div><dt>{g("rootCause")}</dt><dd>{facts.root_cause}</dd></div> : null}
    <div><dt>{g("slaDeadline")}</dt><dd>{details.previous_sla_deadline ? formatDateTime(String(details.previous_sla_deadline)) : "—"} → {details.sla_deadline ? formatDateTime(String(details.sla_deadline)) : "—"}</dd></div>
  </dl>
}
