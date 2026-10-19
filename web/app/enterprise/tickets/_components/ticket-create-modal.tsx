"use client"

import { useEffect, useState } from "react"
import { CheckCircle2Icon, CopyIcon } from "lucide-react"
import { Input, Segmented } from "antd"
import { FormField, IconButton, RailopsButton, SelectField, StandardModal, StatusTag, type StatusTagTone } from "@railops/ui"
import { toast } from "sonner"

import { translateCurrentMessage } from "@/i18n/messages"
import { useI18n } from "@/i18n/provider"
import {
  createTicket,
  createTicketCustomerInvitationDraft,
  fetchTicketCustomerOptions,
  type TicketCustomerInvitationDraftResult,
  type TicketCustomerOption,
} from "@/lib/api/enterprise-tickets"
import type { TicketListItem, TicketPriority } from "@/lib/api/types"
import { type CaseType } from "@/lib/ticket-governance"
import { TicketClassificationFields } from "./ticket-governance"

function ee(key: string) {
  return translateCurrentMessage(`enterpriseExtract.${key}`)
}

function serviceProfileLabel(profile: string) {
  if (profile === "enhanced") return ee("tickets.text105")
  if (profile === "mission_critical") return ee("tickets.text106")
  if (profile === "standard") return ee("tickets.text104")
  return ee("tickets.text107")
}

function serviceProfileTone(profile: string): StatusTagTone {
  if (profile === "mission_critical") return "error"
  if (profile === "enhanced") return "warning"
  if (profile === "standard") return "neutral"
  return "disabled"
}

type TicketCustomerMode = "existing" | "invite"

type TicketDraft = {
  priorityOverride: string
  priorityReason: string
  caseType: CaseType
  parentId: number
  relationReason: string
  channel: "enterprise" | "phone"
  title: string
  description: string
  priority: TicketPriority
  customerMode: TicketCustomerMode
  customerId: number
  inviteDisplayName: string
  inviteEmail: string
  inviteCustomerOrg: string
}

const EMPTY_TICKET_DRAFT: TicketDraft = {
  priorityOverride: "",
  priorityReason: "",
  caseType: "user_case",
  parentId: 0,
  relationReason: "",
  channel: "enterprise",
  title: "",
  description: "",
  priority: "medium",
  customerMode: "existing",
  customerId: 0,
  inviteDisplayName: "",
  inviteEmail: "",
  inviteCustomerOrg: "",
}

type TicketCreateModalProps = {
  open: boolean
  parentTicketId?: number
  onOpenChange: (open: boolean) => void
  onCreated?: (ticket: TicketListItem) => void | Promise<void>
}

export function TicketCreateModal({ open, parentTicketId = 0, onOpenChange, onCreated }: TicketCreateModalProps) {
  const t = useI18n()
  const [saving, setSaving] = useState(false)
  const [createKey, setCreateKey] = useState(() => crypto.randomUUID())
  const [error, setError] = useState("")
  const [draft, setDraft] = useState<TicketDraft>({ ...EMPTY_TICKET_DRAFT, parentId: parentTicketId })
  const [customerOptions, setCustomerOptions] = useState<TicketCustomerOption[]>([])
  const [customerOptionsLoading, setCustomerOptionsLoading] = useState(false)
  const [customerOptionsError, setCustomerOptionsError] = useState("")
  const [createdInvitation, setCreatedInvitation] = useState<TicketCustomerInvitationDraftResult | null>(null)
  const [invitationLinkCopied, setInvitationLinkCopied] = useState(false)

  useEffect(() => {
    if (!open) return
    setCreateKey(crypto.randomUUID())
    setError("")
    setCreatedInvitation(null)
    setInvitationLinkCopied(false)
    setDraft({ ...EMPTY_TICKET_DRAFT, parentId: parentTicketId })
  }, [open, parentTicketId])

  useEffect(() => {
    if (!open) return
    let cancelled = false
    async function loadCustomerOptions() {
      setCustomerOptionsLoading(true)
      setCustomerOptionsError("")
      try {
        const result = await fetchTicketCustomerOptions()
        if (cancelled) return
        if (!result.success) {
          setCustomerOptions([])
          setCustomerOptionsError(result.error?.message || ee("tickets.text098"))
          return
        }
        const items = result.data || []
        setCustomerOptions(items)
        setDraft((current) => ({
          ...current,
          customerMode: items.length === 0 ? "invite" : current.customerMode,
        }))
      } catch (loadError) {
        if (!cancelled) {
          setCustomerOptions([])
          setCustomerOptionsError(loadError instanceof Error ? loadError.message : ee("tickets.text098"))
        }
      } finally {
        if (!cancelled) setCustomerOptionsLoading(false)
      }
    }
    void loadCustomerOptions()
    return () => {
      cancelled = true
    }
  }, [open])

  function close() {
    if (!saving) onOpenChange(false)
  }

  async function notifyCreated(ticket: TicketListItem) {
    try {
      await onCreated?.(ticket)
    } catch {
      // The ticket is already created; a background refresh failure must not invite a duplicate submission.
    }
  }

  async function submit() {
    const title = draft.title.trim()
    const description = draft.description.trim()
    if (!title || !description) {
      setError(ee("tickets.text076"))
      return
    }
    if (draft.customerMode === "existing" && draft.customerId <= 0) {
      setError(ee("tickets.text084"))
      return
    }
    if (draft.customerMode === "invite" && (!draft.inviteDisplayName.trim() || !draft.inviteEmail.trim() || !draft.inviteCustomerOrg.trim())) {
      setError(ee("tickets.text091"))
      return
    }
    if (draft.priorityOverride && !draft.priorityReason.trim()) {
      setError(t("ticketGovernance.reasonRequired"))
      return
    }
    setSaving(true)
    setError("")
    try {
      const inviteCustomer = draft.customerMode === "invite"
      if (inviteCustomer) {
        const result = await createTicketCustomerInvitationDraft({
          priority_level: draft.priorityOverride || undefined,
          priority_reason: draft.priorityOverride ? draft.priorityReason.trim() : undefined,
          case_type: draft.caseType,
          parent_ticket_id: draft.parentId,
          relation_reason: draft.relationReason,
          title,
          description,
          priority: draft.priority,
          displayName: draft.inviteDisplayName.trim(),
          email: draft.inviteEmail.trim(),
          customerOrg: draft.inviteCustomerOrg.trim(),
        })
        if (!result.success || !result.data) {
          setError(result.error?.message || ee("tickets.text099"))
          return
        }
        await notifyCreated(result.data.ticket)
        setCreatedInvitation(result.data)
        toast.success(ee("tickets.text092"))
        return
      }

      const result = await createTicket({
        priority_level: draft.priorityOverride || undefined,
        priority_reason: draft.priorityOverride ? draft.priorityReason.trim() : undefined,
        case_type: draft.caseType,
        parent_ticket_id: draft.parentId,
        relation_reason: draft.relationReason,
        source: "manual",
        channel: draft.channel,
        idempotency_key: createKey,
        title,
        description,
        priority: draft.priority,
        ...(draft.customerId > 0 ? { customer_id: draft.customerId } : {}),
      })
      if (!result.success || !result.data) {
        setError(result.error?.message || ee("tickets.text075"))
        return
      }
      await notifyCreated(result.data)
      toast.success(ee("tickets.text074"))
      onOpenChange(false)
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : (draft.customerMode === "invite" ? ee("tickets.text099") : ee("tickets.text075")))
    } finally {
      setSaving(false)
    }
  }

  async function copyInvitationLink() {
    const link = createdInvitation?.invitation.registrationUrl
    if (!link) return
    await navigator.clipboard.writeText(link)
    setInvitationLinkCopied(true)
    toast.success(ee("tickets.text096"))
  }

  return (
    <StandardModal
      centered
      destroyOnHidden
      open={open}
      title={ee("tickets.text065")}
      width={620}
      styles={{ body: { maxHeight: "calc(100dvh - 180px)", overflowY: "auto", overflowX: "hidden" } }}
      onCancel={close}
      footer={
        createdInvitation ? (
          <RailopsButton variant="primary" onClick={close}>{ee("tickets.text097")}</RailopsButton>
        ) : (
          <>
            <RailopsButton onClick={close} disabled={saving}>{ee("tickets.text073")}</RailopsButton>
            <RailopsButton variant="primary" onClick={() => void submit()} disabled={saving}>
              {saving ? ee("tickets.text072") : ee("tickets.text071")}
            </RailopsButton>
          </>
        )
      }
    >
      {createdInvitation ? (
        <div className="grid gap-4">
          <div className="flex items-start gap-3 border border-emerald-200 bg-emerald-50 px-4 py-3 text-emerald-950">
            <CheckCircle2Icon className="mt-0.5 size-5 shrink-0 text-emerald-600" />
            <div className="grid gap-1">
              <strong>{ee("tickets.text092")}</strong>
              <span className="text-sm text-emerald-800">{ee("tickets.text093")}</span>
              <div className="mt-1"><StatusTag tone="neutral">{ee("tickets.text101")} · {createdInvitation.ticket.ticket_no}</StatusTag></div>
            </div>
          </div>
          <FormField label={ee("tickets.text094")}>
            <div className="flex items-center gap-2">
              <Input readOnly value={createdInvitation.invitation.registrationUrl} />
              <IconButton
                icon={<CopyIcon className="size-4" />}
                tooltip={invitationLinkCopied ? ee("tickets.text096") : ee("tickets.text095")}
                aria-label={invitationLinkCopied ? ee("tickets.text096") : ee("tickets.text095")}
                onClick={() => void copyInvitationLink()}
              />
            </div>
          </FormField>
          {!createdInvitation.invitation.emailSent ? (
            <div className="border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-900">{ee("tickets.text100")}</div>
          ) : null}
        </div>
      ) : (
        <div className="grid gap-4">
          <FormField label={ee("tickets.text078")} required>
            <Segmented
              block
              value={draft.customerMode}
              options={[
                { value: "existing", label: ee("tickets.text079") },
                { value: "invite", label: ee("tickets.text080") },
              ]}
              onChange={(value) => {
                setError("")
                setDraft((current) => ({ ...current, customerMode: value as TicketCustomerMode }))
              }}
            />
          </FormField>
          {draft.customerMode === "existing" ? (
            <div className="grid gap-2">
              <SelectField
                label={ee("tickets.text077")}
                required
                selectProps={{
                  value: draft.customerId || undefined,
                  loading: customerOptionsLoading,
                  showSearch: true,
                  optionFilterProp: "label",
                  placeholder: customerOptionsLoading ? ee("tickets.text082") : ee("tickets.text081"),
                  onChange: (customerId) => setDraft((current) => ({ ...current, customerId: Number(customerId) || 0 })),
                  options: customerOptions.map((customer) => ({
                    value: customer.customer_id,
                    label: [customer.display_name, customer.customer_org_name, customer.email].filter(Boolean).join(" · "),
                  })),
                }}
              />
              {draft.customerId > 0 ? (
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <span>{ee("tickets.text103")}</span>
                  <StatusTag tone={serviceProfileTone(customerOptions.find((item) => item.customer_id === draft.customerId)?.service_profile || "")}>
                    {serviceProfileLabel(customerOptions.find((item) => item.customer_id === draft.customerId)?.service_profile || "")}
                  </StatusTag>
                </div>
              ) : null}
              {customerOptionsError ? <span className="text-sm text-destructive">{customerOptionsError}</span> : null}
              {!customerOptionsLoading && !customerOptionsError && customerOptions.length === 0 ? (
                <span className="text-sm text-muted-foreground">{ee("tickets.text083")}</span>
              ) : null}
            </div>
          ) : (
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <FormField label={ee("tickets.text085")} required>
                <Input value={draft.inviteDisplayName} maxLength={100} placeholder={ee("tickets.text088")} onChange={(event) => setDraft((current) => ({ ...current, inviteDisplayName: event.target.value }))} />
              </FormField>
              <FormField label={ee("tickets.text086")} required>
                <Input type="email" value={draft.inviteEmail} maxLength={200} placeholder={ee("tickets.text089")} onChange={(event) => setDraft((current) => ({ ...current, inviteEmail: event.target.value }))} />
              </FormField>
              <div className="sm:col-span-2">
                <FormField label={ee("tickets.text087")} required>
                  <Input value={draft.inviteCustomerOrg} maxLength={200} placeholder={ee("tickets.text090")} onChange={(event) => setDraft((current) => ({ ...current, inviteCustomerOrg: event.target.value }))} />
                </FormField>
              </div>
            </div>
          )}
          <FormField label={ee("tickets.text066")} required>
            <Input value={draft.title} maxLength={200} autoFocus placeholder={ee("tickets.text069")} onChange={(event) => setDraft((current) => ({ ...current, title: event.target.value }))} />
          </FormField>
          <FormField label={ee("tickets.text067")} required>
            <Input.TextArea value={draft.description} rows={5} maxLength={4000} showCount placeholder={ee("tickets.text070")} onChange={(event) => setDraft((current) => ({ ...current, description: event.target.value }))} />
          </FormField>
          <TicketClassificationFields
            priorityOverride={draft.priorityOverride}
            priorityReason={draft.priorityReason}
            onPriorityChange={(priorityOverride) => setDraft((current) => ({ ...current, priorityOverride }))}
            onPriorityReasonChange={(priorityReason) => setDraft((current) => ({ ...current, priorityReason }))}
            value={draft.caseType}
            disabled={saving}
            onChange={(caseType) => setDraft((current) => ({ ...current, caseType }))}
          />
          {draft.parentId > 0 ? (
            <FormField label={`${t("ticketGovernance.parentTicket")} #${draft.parentId} · ${t("ticketGovernance.reason")}`} required>
              <Input.TextArea value={draft.relationReason} maxLength={2000} onChange={(event) => setDraft((current) => ({ ...current, relationReason: event.target.value }))} />
            </FormField>
          ) : null}
          {error ? <div className="border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</div> : null}
        </div>
      )}
    </StandardModal>
  )
}
