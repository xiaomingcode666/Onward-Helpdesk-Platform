"use client"

import { CheckCircle2Icon } from "lucide-react"

import { useI18n } from "@/i18n/provider"

export default function Page() {
  const t = useI18n()

  return (
    <main className="flex min-h-svh items-center justify-center bg-[var(--railops-layout-background)] px-6 text-[var(--railops-text)]">
      <section className="grid max-w-sm justify-items-center gap-4 rounded-lg bg-[var(--railops-surface)] p-6 text-center shadow-[var(--railops-card-shadow)]">
        <div className="flex size-12 items-center justify-center rounded-full bg-[var(--railops-success-bg)] text-[#047857]">
          <CheckCircle2Icon className="size-7" />
        </div>
        <div className="grid gap-2">
          <h1 className="text-base font-semibold text-[var(--railops-text)]">{t("supportChat.closedTitle")}</h1>
          <p className="text-xs leading-6 text-[var(--railops-text-secondary)]">
            {t("supportChat.closedDescription")}
          </p>
        </div>
      </section>
    </main>
  )
}
