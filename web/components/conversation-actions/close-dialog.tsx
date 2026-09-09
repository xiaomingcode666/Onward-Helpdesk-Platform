"use client"

import { useEffect, useMemo, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Resolver, useForm } from "react-hook-form"
import { z } from "zod/v4"
import { CircleXIcon } from "lucide-react"
import { toast } from "sonner"

import { closeConversation } from "@/lib/api/admin"
import { StandardModal } from "@railops/ui"
import { Button } from "@/components/ui/button"
import {
  Field,
  FieldContent,
  FieldError,
  FieldLabel,
} from "@/components/ui/field"
import { Textarea } from "@/components/ui/textarea"
import { useI18n } from "@/i18n/provider"

type ConversationCloseDialogProps = {
  open: boolean
  conversationId: number | null
  onOpenChange: (open: boolean) => void
  onSuccess?: () => Promise<void> | void
}

type CloseForm = {
  closeReason: string
}

const emptyForm: CloseForm = {
  closeReason: "",
}

export function ConversationCloseDialog({
  open,
  conversationId,
  onOpenChange,
  onSuccess,
}: ConversationCloseDialogProps) {
  const t = useI18n()
  return (
    <StandardModal
      open={open}
      onCancel={() => onOpenChange(false)}
      destroyOnHidden
      title={t("conversationAction.closeTitle")}
      width={512}
      footer={null}
      styles={{ body: { padding: 0 } }}
    >
      {open ? (
        <ConversationCloseDialogBody
          key={conversationId ? `close-${conversationId}` : "close"}
          conversationId={conversationId}
          onOpenChange={onOpenChange}
          onSuccess={onSuccess}
        />
      ) : null}
    </StandardModal>
  )
}

type ConversationCloseDialogBodyProps = {
  conversationId: number | null
  onOpenChange: (open: boolean) => void
  onSuccess?: () => Promise<void> | void
}

function ConversationCloseDialogBody({
  conversationId,
  onOpenChange,
  onSuccess,
}: ConversationCloseDialogBodyProps) {
  const t = useI18n()
  const [saving, setSaving] = useState(false)

  const closeSchema = useMemo(
    () =>
      z.object({
        closeReason: z.string().trim().min(1, t("conversationAction.closeReasonRequired")),
      }),
    [t]
  )
  const closeResolver = useMemo(
    () => zodResolver(closeSchema as never) as Resolver<CloseForm>,
    [closeSchema]
  )

  const form = useForm<CloseForm>({
    resolver: closeResolver,
    defaultValues: emptyForm,
  })
  const {
    handleSubmit,
    reset,
    register,
    formState: { errors },
  } = form

  useEffect(() => {
    reset(emptyForm)
  }, [conversationId, reset])

  async function onFormSubmit(values: CloseForm) {
    if (!conversationId) {
      toast.error(t("conversationAction.conversationMissing"))
      return
    }

    setSaving(true)
    try {
      await closeConversation(conversationId, values.closeReason.trim())
      toast.success(t("conversationAction.closed", { id: conversationId }))
      reset(emptyForm)
      onOpenChange(false)
      await onSuccess?.()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("conversationAction.closeFailed"))
    } finally {
      setSaving(false)
    }
  }

  return (
    <form onSubmit={handleSubmit(onFormSubmit)}>
        <div className="space-y-4 p-6">
          <Field data-invalid={!!errors.closeReason}>
            <FieldLabel htmlFor="conversation-close-reason">
              {t("conversationAction.closeReason")}
            </FieldLabel>
            <FieldContent>
              <Textarea
                id="conversation-close-reason"
                rows={4}
                placeholder={t("conversationAction.closeReasonPlaceholder")}
                aria-invalid={!!errors.closeReason}
                {...register("closeReason")}
              />
              <FieldError errors={[errors.closeReason]} />
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
            <CircleXIcon />
            {saving ? t("conversationAction.closing") : t("conversationAction.confirmClose")}
          </Button>
        </div>
      </form>
  )
}
