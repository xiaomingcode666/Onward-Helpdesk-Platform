"use client"

import Image from "next/image"
import { BadgeCheckIcon, BrainCircuitIcon, Clock3Icon, RadioIcon, ShieldCheckIcon, WrenchIcon } from "lucide-react"

import { AppLogoMark } from "@/components/brand-logo"
import { useI18n } from "@/i18n/provider"
import { cn } from "@/lib/utils"

type PlatformLoginShowcaseProps = {
  className?: string
}

const serviceSignals = [
  { icon: RadioIcon, labelKey: "auth.platformVisual.networkStatus", valueKey: "auth.platformVisual.networkRegions" },
  { icon: BrainCircuitIcon, labelKey: "auth.platformVisual.aiFirstReply", valueKey: "auth.platformVisual.aiAnswer" },
  { icon: WrenchIcon, labelKey: "auth.platformVisual.escalation", valueKey: "auth.platformVisual.humanHandling" },
] as const

const serviceTimeline = [
  { icon: BadgeCheckIcon, labelKey: "auth.platformVisual.aiFirstReply" },
  { icon: BrainCircuitIcon, labelKey: "auth.platformVisual.aiAnswer" },
  { icon: WrenchIcon, labelKey: "auth.platformVisual.humanHandling" },
] as const

export function PlatformLoginShowcase({ className }: PlatformLoginShowcaseProps) {
  const t = useI18n()

  return (
    <aside className={cn("rhd-railops-login-showcase relative isolate flex min-h-svh flex-col overflow-hidden bg-[#0f172a] px-8 py-8 text-white xl:px-12", className)}>
      <Image
        src="/images/login-service-operations.jpg"
        alt=""
        fill
        priority
        sizes="(min-width: 1024px) 68vw, 100vw"
        className="pointer-events-none object-cover object-center"
      />
      <div className="pointer-events-none absolute inset-0 bg-[#0f172a]/74" />

      <header className="relative z-10 flex items-start justify-between gap-4">
        <div className="flex items-center gap-3">
          <AppLogoMark alt={t("remoteTopbar.brandTitle")} className="size-9 ring-white/20" imageClassName="p-0.5" priority />
          <div>
            <p className="text-rhd-md font-semibold text-white">{t("remoteTopbar.brandTitle")}</p>
            <p className="mt-0.5 text-xs text-blue-50/65">{t("remoteTopbar.brandSub")}</p>
          </div>
        </div>
        <span className="inline-flex h-7 items-center gap-2 rounded-md border border-white/15 bg-white/10 px-2.5 text-rhd-xs font-medium text-blue-50/85">
          <ShieldCheckIcon className="size-3.5 text-blue-200" />
          {t("auth.platformVisual.globalService")}
        </span>
      </header>

      <div className="relative z-10 flex flex-1 flex-col justify-center py-10">
        <div className="max-w-[680px]">
          <p className="inline-flex h-7 items-center gap-2 rounded-md border border-blue-100/20 bg-blue-50/10 px-2.5 text-rhd-xs font-semibold text-blue-100">
            <RadioIcon className="size-3.5" />
            {t("auth.platformVisual.eyebrow")}
          </p>
          <h1 className="mt-5 max-w-[640px] text-rhd-5xl font-semibold leading-[1.25] text-white xl:text-rhd-5xl">
            {t("auth.platformVisual.heading")}
          </h1>
        </div>

        <div className="mt-8 grid max-w-[680px] gap-3 xl:grid-cols-3">
          {serviceSignals.map((item) => {
            const Icon = item.icon
            return (
              <div key={item.labelKey} className="rounded-lg border border-white/14 bg-white/10 p-3 backdrop-blur-sm">
                <div className="flex items-center gap-2 text-rhd-xs font-medium text-blue-50/72">
                  <Icon className="size-3.5 text-blue-200" />
                  {t(item.labelKey)}
                </div>
                <p className="mt-2 text-rhd-md font-semibold text-white">{t(item.valueKey)}</p>
              </div>
            )
          })}
        </div>
      </div>

      <div className="relative z-10 rounded-lg border border-white/14 bg-white/10 p-4 backdrop-blur-sm">
        <div className="mb-3 flex items-center justify-between gap-3">
          <p className="text-rhd-md font-semibold text-white">{t("auth.platformVisual.aiTitle")}</p>
          <span className="inline-flex items-center gap-1.5 text-rhd-xs font-medium text-blue-50/68">
            <Clock3Icon className="size-3.5" />
            24 / 7
          </span>
        </div>
        <div className="grid gap-2 sm:grid-cols-3">
          {serviceTimeline.map((item, index) => {
            const Icon = item.icon
            return (
              <div key={item.labelKey} className="flex items-center gap-2 rounded-md bg-white/10 px-3 py-2 text-rhd-sm font-semibold text-blue-50">
                <span className="grid size-6 shrink-0 place-items-center rounded-md bg-white text-[#2563eb]">
                  <Icon className="size-3.5" />
                </span>
                <span className="min-w-0 truncate">{index + 1}. {t(item.labelKey)}</span>
              </div>
            )
          })}
        </div>
        <div className="mt-3 flex items-center gap-2 text-rhd-xs text-blue-50/62">
          <ShieldCheckIcon className="size-3.5 text-blue-200" />
          {t("auth.secureAccess")}
        </div>
      </div>
    </aside>
  )
}
