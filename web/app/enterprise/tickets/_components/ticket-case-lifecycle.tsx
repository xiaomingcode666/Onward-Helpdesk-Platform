"use client"

import { useEffect, useRef, useState } from "react"
import { Input } from "antd"
import { FormField, RailopsButton, StandardModal, StatusTag } from "@railops/ui"
import { toast } from "sonner"
import { advanceTicketLifecycle, fetchTicketAggregate } from "@/lib/api/enterprise-tickets"
import type { TicketAggregateDTO } from "@/lib/api/types"
import { caseLabel, caseStatusLabel } from "@/lib/ticket-case-labels"
import { formatDateTime } from "@/lib/utils"

const lifecycleActions = ["triage", "wait", "resume", "restore", "resolve", "request_closure", "close", "cancel", "reopen"] as const
type Action = typeof lifecycleActions[number]

export function TicketCaseLifecycle({ aggregate, onSaved, active = true }: {
  aggregate: TicketAggregateDTO
  onSaved: (value: TicketAggregateDTO) => void | Promise<void>
  active?: boolean
}) {
  const lifecycle = aggregate.case_lifecycle
  const [action, setAction] = useState<Action | null>(null)
  const [reason, setReason] = useState("")
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)
  const inFlight = useRef(false)
  const mutationCompleted = useRef(false)
  const available = useRef(false)
  const [submitted, setSubmitted] = useState(false)
  const [expectedStatus, setExpectedStatus] = useState("")
  const [expectedRevision, setExpectedRevision] = useState(0)
  const [idempotencyKey, setIdempotencyKey] = useState("")
  const [actionsOpen, setActionsOpen] = useState(false)
  const needsReason = action !== "triage"
  const staleAction = Boolean(action && !submitted && lifecycle && (
    lifecycle.status !== expectedStatus || lifecycle.revision !== expectedRevision ||
    !(lifecycle.allowed_actions ?? []).includes(action)
  ))

  useEffect(() => {
    available.current = active
    return () => { available.current = false }
  }, [active])

  if (!lifecycle || !active) return null

  function openAction(next: Action) {
    if (!lifecycle || !available.current || inFlight.current) return
    setAction(next)
    setReason("")
    setError("")
    setExpectedStatus(lifecycle.status)
    setExpectedRevision(lifecycle.revision)
    setIdempotencyKey(crypto.randomUUID())
    mutationCompleted.current = false
    setSubmitted(false)
  }

  async function save() {
    if (!action || !available.current || inFlight.current || staleAction) return
    if (needsReason && !reason.trim()) { setError(caseLabel("requiredReason")); return }
    inFlight.current = true
    setBusy(true)
    setSubmitted(true)
    setError("")
    try {
      const id = aggregate.ticket.id
      if (!mutationCompleted.current) {
        const result = await advanceTicketLifecycle(id, {
          action, reason: reason.trim(), expected_status: expectedStatus,
          expected_revision: expectedRevision, idempotency_key: idempotencyKey,
        })
        if (!result.success) throw new Error(result.error?.message || caseLabel("failed"))
        // A refresh failure must not submit another close/reopen.
        mutationCompleted.current = true
      }
      if (!available.current) return
      const refreshed = await fetchTicketAggregate(id)
      if (!available.current) return
      if (!refreshed.success || !refreshed.data) throw new Error(refreshed.error?.message || caseLabel("failed"))
      if (refreshed.data.ticket.id !== id) throw new Error(caseLabel("invalidTicket"))
      await onSaved(refreshed.data)
      if (!available.current) return
      setAction(null)
      toast.success(caseLabel("saved"))
    } catch (cause) {
      if (available.current) setError(cause instanceof Error ? cause.message : caseLabel("failed"))
    } finally {
      inFlight.current = false
      if (available.current) setBusy(false)
    }
  }

  return (
    <section className="space-y-3 rounded-md border border-border bg-card p-4" data-testid="ticket-case-lifecycle">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="text-sm font-semibold">{caseLabel("title")}</h3>
        <StatusTag tone="blue" data-testid="ticket-case-status">{caseStatusLabel(lifecycle.status)}</StatusTag>
      </div>
      <dl className="grid gap-2 text-sm">
        <div><dt className="text-xs text-muted-foreground">{caseLabel("engineer")}</dt><dd>{aggregate.assignment.assignee_name || "—"}</dd></div>
        {aggregate.assignment.accepted_at ? <div><dt className="text-xs text-muted-foreground">{caseLabel("acceptedAt")}</dt><dd>{formatDateTime(aggregate.assignment.accepted_at)}</dd></div> : null}
        {lifecycle.restored_at ? <div><dt className="text-xs text-muted-foreground">{caseLabel("restoredAt")}</dt><dd>{formatDateTime(lifecycle.restored_at)}</dd></div> : null}
        {lifecycle.waiting_reason ? <div><dt className="text-xs text-muted-foreground">{caseLabel("waitingReason")}</dt><dd className="whitespace-pre-wrap">{lifecycle.waiting_reason}</dd></div> : null}
      </dl>
      {lifecycle.legacy_record ? <p className="text-xs text-amber-700" data-testid="ticket-case-legacy">{caseLabel("legacy")}</p> : null}
      {lifecycle.workflow_version_id ? <p className="text-xs text-muted-foreground">工单流程版本 #{lifecycle.workflow_version_id}</p> : null}
      {lifecycle.workflow_error ? <p role="alert" className="text-sm text-destructive">{lifecycle.workflow_error}</p> : null}
      <div className="flex flex-wrap items-center gap-2">
        {lifecycle.allowed_actions?.length ? <RailopsButton size="small" data-testid="case-update-status" onClick={() => setActionsOpen((value) => !value)} aria-expanded={actionsOpen}>
          {caseLabel("updateStatus")}
        </RailopsButton> : null}
      </div>
      {actionsOpen ? <div className="flex flex-wrap gap-2 rounded-md border border-border bg-muted/20 p-2" data-testid="case-status-actions">
        {lifecycleActions.filter((item) => (lifecycle.allowed_actions ?? []).includes(item)).map((item) => (
          <RailopsButton key={item} size="small" data-testid={`case-action-${item}`} onClick={() => { setActionsOpen(false); openAction(item) }}>{caseLabel(`action.${item}`)}</RailopsButton>
        ))}
        {!lifecycle.allowed_actions?.length ? <span className="text-sm text-muted-foreground">{caseLabel("noActions")}</span> : null}
      </div> : null}
      <StandardModal
        open={action !== null}
        title={caseLabel(`action.${action || "triage"}`)}
        onCancel={() => { if (!busy) setAction(null) }}
        footer={<>
          <RailopsButton disabled={busy} onClick={() => setAction(null)}>{caseLabel("cancel")}</RailopsButton>
          <RailopsButton
            variant="primary"
            aria-label={caseLabel("confirm")}
            aria-busy={busy}
            loading={busy}
            disabled={busy || staleAction}
            onClick={() => void save()}
          >{caseLabel("confirm")}</RailopsButton>
        </>}
      >
        <div className="space-y-4">
          <FormField label={caseLabel("reason")} required={needsReason}>
            <Input.TextArea aria-label={caseLabel("reason")} value={reason} rows={4} maxLength={2000} disabled={busy || submitted || staleAction} onChange={(event) => setReason(event.target.value)} placeholder={caseLabel(needsReason ? "reasonHint" : "optionalReason")} />
          </FormField>
          {staleAction ? <p role="alert" className="text-sm text-destructive">{caseLabel("staleAction")}</p> : null}
          {error ? <p role="alert" className="text-sm text-destructive">{error}</p> : null}
          {error && submitted ? <p className="text-xs text-muted-foreground">{caseLabel("retryHint")}</p> : null}
        </div>
      </StandardModal>
    </section>
  )
}
