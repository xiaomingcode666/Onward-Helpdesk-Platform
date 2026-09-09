"use client"

import { useEffect, useMemo, useRef, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, type Resolver, useForm } from "react-hook-form"
import { toast } from "sonner"
import { z } from "zod/v4"

import { StandardModal } from "@railops/ui"
import { OptionCombobox } from "@/components/option-combobox"
import { Button } from "@/components/ui/button"
import { Field, FieldContent, FieldError, FieldLabel } from "@/components/ui/field"
import { Textarea } from "@/components/ui/textarea"
import {
  fetchAgentDispatchCandidates,
  type AdminDispatchCandidate,
} from "@/lib/api/admin"
import { useI18n } from "@/i18n/provider"
import { assignTicket } from "@/lib/api/ticket"

type TFunction = (key: string, values?: Record<string, string | number>) => string

const tr = (t: TFunction, key: string, values?: Record<string, string | number>) =>
  t(`transferExtract.candidateInfo.${key}`, values)

function createSchema(t: TFunction) {
  return z.object({
  toUserId: z.string().trim().min(1, t("ticket.assigneeRequired")),
  reason: z.string().trim(),
  })
}

type FormValues = {
  toUserId: string
  reason: string
}

const emptyForm: FormValues = {
  toUserId: "",
  reason: "",
}

function dispatchCandidateLabel(
  t: TFunction,
  agent: AdminDispatchCandidate,
  fallback: string
) {
  const name = agent.displayName || agent.nickname || agent.username || fallback
  const capacity = agent.maxConcurrentCount > 0
    ? `${agent.workload}/${agent.maxConcurrentCount}`
    : `${agent.workload}/${tr(t, "limitLabel")}`
  return tr(t, "candidateLabel", {
    name,
    loadLabel: tr(t, "load"),
    capacity,
    weightLabel: tr(t, "weight"),
    weight: agent.dispatchWeight || 1,
    reachability: dispatchCandidateReachability(t, agent),
  })
}

function dispatchCandidateReachability(t: TFunction, agent: AdminDispatchCandidate) {
  if (agent.receiveOfflineMessage) {
    return tr(t, "offlineDispatchable")
  }
  if (!agent.lastOnlineAt) {
    return tr(t, "onlineNow")
  }
  const lastOnlineAt = Date.parse(agent.lastOnlineAt)
  if (Number.isNaN(lastOnlineAt)) {
    return tr(t, "onlineNow")
  }
  const minutes = Math.max(0, Math.floor((Date.now() - lastOnlineAt) / 60000))
  if (minutes < 1) {
    return tr(t, "onlineJustNow")
  }
  if (minutes < 60) {
    return tr(t, "onlineMinutesAgo", { minutes })
  }
  return tr(t, "onlineHoursAgo", { hours: Math.floor(minutes / 60) })
}

type TicketAssignDialogProps = {
  open: boolean
  ticketId: number | null
  currentAssigneeId?: number
  onOpenChange: (open: boolean) => void
  onSuccess?: () => Promise<void> | void
}

export function TicketAssignDialog({
  open,
  ticketId,
  currentAssigneeId,
  onOpenChange,
  onSuccess,
}: TicketAssignDialogProps) {
  const t = useI18n()
  return (
    <StandardModal
      open={open}
      onCancel={() => onOpenChange(false)}
      destroyOnHidden
      title={t("ticket.assignTitle")}
      width={512}
      footer={null}
      styles={{ body: { padding: 0 } }}
    >
      {open ? (
        <TicketAssignDialogBody
          key={ticketId ?? "ticket-assign"}
          ticketId={ticketId}
          currentAssigneeId={currentAssigneeId}
          onOpenChange={onOpenChange}
          onSuccess={onSuccess}
        />
      ) : null}
    </StandardModal>
  )
}

function TicketAssignDialogBody({
  ticketId,
  currentAssigneeId,
  onOpenChange,
  onSuccess,
}: Omit<TicketAssignDialogProps, "open">) {
  const t = useI18n()
  const [saving, setSaving] = useState(false)
  const [loading, setLoading] = useState(false)
  const [agents, setAgents] = useState<AdminDispatchCandidate[]>([])
  const activeRef = useRef(false)

  const schema = useMemo(() => createSchema(t), [t])
  const resolver = useMemo(
    () => zodResolver(schema) as Resolver<FormValues>,
    [schema],
  )
  const form = useForm<FormValues>({
    resolver,
    defaultValues: emptyForm,
  })

  const {
    control,
    handleSubmit,
    register,
    reset,
    formState: { errors },
  } = form

  useEffect(() => {
    activeRef.current = true
    return () => {
      activeRef.current = false
    }
  }, [])

  useEffect(() => {
    reset({
      toUserId: currentAssigneeId ? String(currentAssigneeId) : "",
      reason: "",
    })
  }, [currentAssigneeId, reset, ticketId])

  useEffect(() => {
    if (!ticketId) {
      setAgents([])
      setLoading(false)
      return
    }
    setLoading(true)
    fetchAgentDispatchCandidates({ ticketId })
      .then((agentData) => {
        if (!activeRef.current) {
          return
        }
        const rows = Array.isArray(agentData) ? agentData : []
        setAgents(rows)
        const current = currentAssigneeId ? String(currentAssigneeId) : ""
        reset({
          toUserId: rows.some((agent) => String(agent.userId) === current) ? current : "",
          reason: "",
        })
      })
      .catch((error) => {
        if (!activeRef.current) {
          return
        }
        toast.error(error instanceof Error ? error.message : t("ticket.loadAssigneesFailed"))
        setAgents([])
      })
      .finally(() => {
        if (activeRef.current) {
          setLoading(false)
        }
      })
  }, [currentAssigneeId, reset, t, ticketId])

  async function onFormSubmit(values: FormValues) {
    if (!ticketId) {
      toast.error(t("ticket.selectTicket"))
      return
    }
    setSaving(true)
    try {
      await assignTicket({
        ticketId,
        toUserId: Number(values.toUserId),
        reason: values.reason.trim() || undefined,
      })
      if (!activeRef.current) {
        return
      }
      toast.success(t("ticket.assigneeUpdated"))
      if (!activeRef.current) {
        return
      }
      onOpenChange(false)
      await onSuccess?.()
    } catch (error) {
      if (!activeRef.current) {
        return
      }
      toast.error(error instanceof Error ? error.message : t("ticket.assignFailed"))
    } finally {
      if (activeRef.current) {
        setSaving(false)
      }
    }
  }

  return (
    <form onSubmit={handleSubmit(onFormSubmit)}>
        <div className="space-y-4 p-6">
          <Field data-invalid={!!errors.toUserId}>
            <FieldLabel>{t("ticket.assignee")}</FieldLabel>
            <FieldContent>
              <Controller
                control={control}
                name="toUserId"
                render={({ field }) => (
                  <OptionCombobox
                    value={field.value}
                    onChange={field.onChange}
                    placeholder={loading ? t("ticket.loading") : t("ticket.selectHandler")}
                    options={agents.map((agent) => ({
                      value: String(agent.userId),
                      label: dispatchCandidateLabel(t, agent, t("ticket.agentFallback", { id: agent.userId })),
                    }))}
                  />
                )}
              />
              <FieldError errors={[errors.toUserId]} />
            </FieldContent>
          </Field>
          <Field data-invalid={!!errors.reason}>
            <FieldLabel>{t("ticket.assignReason")}</FieldLabel>
            <FieldContent>
              <Textarea rows={4} placeholder={t("ticket.assignReasonPlaceholder")} {...register("reason")} />
              <FieldError errors={[errors.reason]} />
            </FieldContent>
          </Field>
        </div>
        <div className="flex justify-end gap-2 px-6 py-4">
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            {t("ticket.cancel")}
          </Button>
          <Button type="submit" disabled={saving}>
            {saving ? t("ticket.submitting") : t("ticket.confirmAssign")}
          </Button>
        </div>
      </form>
  )
}
