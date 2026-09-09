"use client"

import { AlertTriangleIcon } from "lucide-react"

import { RailopsButton } from "@railops/ui"
import { useI18n } from "@/i18n/provider"

export default function DashboardRouteError({
  error,
  reset,
}: {
  error: Error & { digest?: string }
  reset: () => void
}) {
  const t = useI18n()
  return (
    <div className="grid min-h-[60vh] place-items-center px-4" role="alert">
      <div className="max-w-md rounded-lg border border-border/60 bg-background p-8 text-center shadow-sm">
        <AlertTriangleIcon className="mx-auto size-10 text-destructive" aria-hidden="true" />
        <h2 className="mt-4 text-base font-semibold text-foreground">{t("errorStates.loadFailedTitle")}</h2>
        <p className="mt-2 break-words text-sm text-muted-foreground">
          {error.message || "Unknown error"}
        </p>
        <RailopsButton className="mt-6" onClick={() => reset()}>
          {t("common.retry")}
        </RailopsButton>
      </div>
    </div>
  )
}
