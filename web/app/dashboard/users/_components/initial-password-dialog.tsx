"use client"

import { useState } from "react"
import { CopyIcon } from "lucide-react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { useI18n } from "@/i18n/provider"
import { StandardModal } from "@railops/ui"

type InitialPasswordDialogProps = {
  open: boolean
  username: string
  password: string
  onOpenChange: (open: boolean) => void
}

export function InitialPasswordDialog({
  open,
  username,
  password,
  onOpenChange,
}: InitialPasswordDialogProps) {
  const t = useI18n()
  const [copying, setCopying] = useState(false)

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
    <StandardModal
      open={open}
      onCancel={() => onOpenChange(false)}
      title={t("user.createdTitle")}
      footer={
        <>
          <Button
            type="button"
            variant="outline"
            onClick={() => void handleCopy()}
            disabled={copying || !password}
          >
            <CopyIcon />
            {copying ? t("user.copying") : t("user.copyPassword")}
          </Button>
          <Button type="button" onClick={() => onOpenChange(false)}>
            {t("user.close")}
          </Button>
        </>
      }
    >
      <div className="rounded-md border bg-muted/35 px-3 py-2 text-sm">
        <span className="text-muted-foreground">{t("user.username")}</span>
        <span className="ml-2 font-medium">{username || "-"}</span>
      </div>
      <div className="rounded-md border bg-muted/35 p-4">
        <div className="text-xs text-muted-foreground">{t("user.initialPassword")}</div>
        <div className="mt-2 break-all font-mono text-base">{password}</div>
      </div>
    </StandardModal>
  )
}
