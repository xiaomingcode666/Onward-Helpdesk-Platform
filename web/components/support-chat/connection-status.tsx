"use client"

import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"
import { useI18n } from "@/i18n/provider"

type SupportChatConnectionStatusProps = {
  status: "connecting" | "connected" | "disconnected"
}

export function SupportChatConnectionStatus({ status }: SupportChatConnectionStatusProps) {
  const t = useI18n()
  const toneClass =
    status === "connected"
      ? "border-primary/20 bg-primary/10 text-primary"
      : status === "connecting"
        ? "border-amber-200 bg-amber-50 text-foreground dark:border-amber-900/70 dark:bg-amber-950/50"
        : "border-border bg-muted text-muted-foreground"

  return (
    <Badge
      variant="outline"
      className={cn("h-6 gap-2 px-2.5 text-rhd-xs font-medium shadow-sm", toneClass)}
    >
      <span
        className={cn(
          "inline-block size-2 rounded-full",
          status === "connected"
            ? "bg-primary shadow-[0_0_0_4px_rgba(37,99,235,0.14)]"
            : status === "connecting"
              ? "bg-amber-500 shadow-[0_0_0_4px_rgba(245,158,11,0.16)]"
              : "bg-muted-foreground shadow-[0_0_0_4px_rgba(148,163,184,0.14)]"
        )}
      />
      <span>{t(`supportChat.${status}`)}</span>
    </Badge>
  )
}
