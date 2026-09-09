"use client"

import { zodResolver } from "@hookform/resolvers/zod"
import { ArrowRightLeftIcon } from "lucide-react"
import { useEffect, useMemo, useState } from "react"
import { Controller, Resolver, useForm } from "react-hook-form"
import { toast } from "sonner"
import { z } from "zod/v4"

import { OptionCombobox } from "@/components/option-combobox"
import { StandardModal } from "@railops/ui"
import { Button } from "@/components/ui/button"
import {
  Field,
  FieldContent,
  FieldError,
  FieldLabel,
} from "@/components/ui/field"
import { Textarea } from "@/components/ui/textarea"
import {
  assignConversation,
  fetchAgentDispatchCandidates,
  transferConversation,
  type AdminDispatchCandidate,
} from "@/lib/api/admin"
import { useI18n } from "@/i18n/provider"

type TFunction = (key: string, values?: Record<string, string | number>) => string

const tr = (t: TFunction, key: string, values?: Record<string, string | number>) =>
  t(`transferExtract.candidateInfo.${key}`, values)

type ConversationTransferDialogProps = {
  open: boolean
  mode: "assign" | "transfer"
  conversationId: number | null
  onOpenChange: (open: boolean) => void
  onSuccess?: () => Promise<void> | void
}

type TransferForm = {
  toUserId: string
  reason: string
}

const emptyForm: TransferForm = {
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

export function ConversationTransferDialog({
  open,
  mode,
  conversationId,
  onOpenChange,
  onSuccess,
}: ConversationTransferDialogProps) {
  const t = useI18n()
  return (
    <StandardModal
      open={open}
      onCancel={() => onOpenChange(false)}
      destroyOnHidden
      title={
        mode === "assign"
          ? t("conversationAction.assignTitle")
          : t("conversationAction.transferTitle")
      }
      width={512}
      footer={null}
      styles={{ body: { padding: 0 } }}
    >
      {open ? (
        <ConversationTransferDialogBody
          key={conversationId ? `transfer-${conversationId}` : "transfer"}
          mode={mode}
          conversationId={conversationId}
          onOpenChange={onOpenChange}
          onSuccess={onSuccess}
        />
      ) : null}
    </StandardModal>
  )
}

type ConversationTransferDialogBodyProps = {
  mode: "assign" | "transfer"
  conversationId: number | null
  onOpenChange: (open: boolean) => void
  onSuccess?: () => Promise<void> | void
}

function ConversationTransferDialogBody({
  mode,
  conversationId,
  onOpenChange,
  onSuccess,
}: ConversationTransferDialogBodyProps) {
  const t = useI18n()
  const [saving, setSaving] = useState(false)
  const [loadingAgents, setLoadingAgents] = useState(false)
  const [agents, setAgents] = useState<AdminDispatchCandidate[]>([])
  const userOptions = agents.map((agent) => ({
    value: String(agent.userId),
    label: dispatchCandidateLabel(t, agent, t("conversationAction.agentFallback", { id: agent.userId })),
  }))

  const transferSchema = useMemo(
    () =>
      z.object({
        toUserId: z.string().trim().min(1, t("conversationAction.targetAgentRequired")),
        reason: z.string().trim(),
      }),
    [t]
  )
  const transferResolver = useMemo(
    () => zodResolver(transferSchema as never) as Resolver<TransferForm>,
    [transferSchema]
  )

  const form = useForm<TransferForm>({
    resolver: transferResolver,
    defaultValues: emptyForm,
  })
  const {
    control,
    handleSubmit,
    reset,
    register,
    formState: { errors },
  } = form

  useEffect(() => {
    reset(emptyForm)
  }, [conversationId, reset])

  useEffect(() => {
    if (!conversationId) {
      setAgents([])
      setLoadingAgents(false)
      return
    }
    let cancelled = false
    setAgents([])
    setLoadingAgents(true)
    fetchAgentDispatchCandidates({
      conversationId,
      manualTransfer: mode === "transfer" ? 1 : undefined,
    })
      .then((data) => {
        if (!cancelled) {
          setAgents(data)
        }
      })
      .catch((error) => {
        if (!cancelled) {
          toast.error(error instanceof Error ? error.message : t("conversationAction.loadAgentsFailed"))
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingAgents(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [conversationId, mode, t])

  async function onFormSubmit(values: TransferForm) {
    if (!conversationId) {
      toast.error(t("conversationAction.conversationMissing"))
      return
    }

    const toUserId = Number(values.toUserId)
    const reason = values.reason.trim()
    if (!Number.isSafeInteger(toUserId) || toUserId <= 0) {
      toast.error(t("conversationAction.targetAgentRequired"))
      return
    }

    setSaving(true)
    try {
      if (mode === "assign") {
        await assignConversation(conversationId, toUserId, reason)
        toast.success(t("conversationAction.assigned", { id: conversationId }))
      } else {
        await transferConversation(conversationId, toUserId, reason)
        toast.success(t("conversationAction.transferred", { id: conversationId }))
      }
      reset(emptyForm)
      onOpenChange(false)
      await onSuccess?.()
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : mode === "assign"
            ? t("conversationAction.assignFailed")
            : t("conversationAction.transferFailed")
      )
    } finally {
      setSaving(false)
    }
  }

  const isAssign = mode === "assign"

  return (
    <form onSubmit={handleSubmit(onFormSubmit)}>
        <div className="space-y-4 p-6">
          <Field data-invalid={!!errors.toUserId}>
            <FieldLabel htmlFor="conversation-transfer-user">
              {t("conversationAction.targetAgent")}
            </FieldLabel>
            <FieldContent>
              <Controller
                control={control}
                name="toUserId"
                render={({ field }) => (
                  <OptionCombobox
                    value={field.value}
                    options={userOptions}
                    placeholder={
                      loadingAgents
                        ? t("conversationAction.loading")
                        : t("conversationAction.selectTargetAgent")
                    }
                    searchPlaceholder={t("conversationAction.searchAgent")}
                    emptyText={t("conversationAction.emptyAgents")}
                    disabled={saving || loadingAgents}
                    onChange={field.onChange}
                  />
                )}
              />
              <FieldError errors={[errors.toUserId]} />
            </FieldContent>
          </Field>
          <Field data-invalid={!!errors.reason}>
            <FieldLabel htmlFor="conversation-transfer-reason">
              {isAssign ? t("conversationAction.assignNote") : t("conversationAction.transferReason")}
            </FieldLabel>
            <FieldContent>
              <Textarea
                id="conversation-transfer-reason"
                rows={4}
                placeholder={
                  isAssign
                    ? t("conversationAction.assignPlaceholder")
                    : t("conversationAction.transferPlaceholder")
                }
                aria-invalid={!!errors.reason}
                {...register("reason")}
              />
              <FieldError errors={[errors.reason]} />
            </FieldContent>
          </Field>
        </div>
        <div className="flex justify-end gap-2 px-6 py-4">
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={saving}
          >
            {t("conversationAction.cancel")}
          </Button>
          <Button type="submit" disabled={saving}>
            <ArrowRightLeftIcon />
            {saving
              ? isAssign
                ? t("conversationAction.assigning")
                : t("conversationAction.transferring")
              : isAssign
                ? t("conversationAction.confirmAssign")
                : t("conversationAction.confirmTransfer")}
          </Button>
        </div>
      </form>
  )
}
