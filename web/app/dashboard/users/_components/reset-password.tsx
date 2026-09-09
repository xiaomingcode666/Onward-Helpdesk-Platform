"use client"

import { useState } from "react"
import { CopyIcon } from "lucide-react"
import { toast } from "sonner"

import { type AdminUser } from "@/lib/api/admin"
import { useI18n } from "@/i18n/provider"
import { StandardModal } from "@railops/ui"
import { Button } from "@/components/ui/button"

type ResetPasswordDialogsProps = {
  open: boolean
  saving: boolean
  item: AdminUser | null
  password: string
  onOpenChange: (open: boolean) => void
  onConfirm: () => Promise<void>
}

export function ResetPasswordDialogs({
  open,
  saving,
  item,
  password,
  onOpenChange,
  onConfirm,
}: ResetPasswordDialogsProps) {
  const t = useI18n()
  const [copying, setCopying] = useState(false)
  const showingResult = password.trim().length > 0

  async function handleCopy() {
    if (!password || copying) {
      return
    }

    setCopying(true)
    try {
      await navigator.clipboard.writeText(password)
      toast.success(t("user.copied"))
    } catch {
      toast.error(t("user.copyFailed"))
    } finally {
      setCopying(false)
    }
  }

  return (
    <>
      <StandardModal
        open={open && !showingResult}
        onCancel={() => {
          if (saving) {
            return
          }
          onOpenChange(false)
        }}
        title={t("user.confirmResetTitle")}
        footer={
          <>
            <Button onClick={() => void onConfirm()} disabled={saving}>
              {saving ? t("user.resetting") : t("user.confirmReset")}
            </Button>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={saving}
            >
              {t("user.cancel")}
            </Button>
          </>
        }
      >
        <div className="rounded-md border bg-muted/35 px-3 py-2 text-sm">
          <span className="text-muted-foreground">{t("user.username")}</span>
          <span className="ml-2 font-medium">{item?.username || "-"}</span>
        </div>
      </StandardModal>
      <StandardModal
        open={open && showingResult}
        onCancel={() => onOpenChange(false)}
        title={t("user.resetSuccessTitle")}
        footer={
          <>
            <Button type="button" variant="outline" onClick={() => void handleCopy()} disabled={copying}>
              <CopyIcon />
              {copying ? t("user.copying") : t("user.copyPassword")}
            </Button>
            <Button type="button" onClick={() => onOpenChange(false)}>
              {t("user.close")}
            </Button>
          </>
        }
      >
        <div className="rounded-md border bg-muted/35 p-4">
          <div className="text-xs text-muted-foreground">{t("user.newPassword")}</div>
          <div className="mt-2 break-all font-mono text-base">{password}</div>
        </div>
      </StandardModal>
    </>
  )
}
