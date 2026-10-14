"use client"

import { tm } from "@/components/marketing/marketing-i18n"
import { useI18n } from "@/i18n/provider"
import { ArrowRightIcon } from "lucide-react"
import type { ReactNode } from "react"

import { DemoRequestDialog } from "@/components/marketing/demo-request-dialog"
import { MarketingSiteFooter } from "@/components/marketing/marketing-site-footer"
import { MarketingSiteHeader } from "@/components/marketing/marketing-site-header"

export function MarketingPageHero({ eyebrow, title, description, aside, compact = false }: { eyebrow: string; title: string; description: string; aside?: ReactNode; compact?: boolean }) {
  useI18n()
  return (
    <>
      <MarketingSiteHeader />
      <section className="relative overflow-hidden border-b border-slate-200 bg-[#f3f7f6]">
        <div className="pointer-events-none absolute inset-0 opacity-[0.35] [background-image:linear-gradient(rgba(15,118,110,0.08)_1px,transparent_1px),linear-gradient(90deg,rgba(15,118,110,0.08)_1px,transparent_1px)] [background-size:48px_48px]" />
        <div className={`relative mx-auto grid max-w-7xl px-5 sm:px-8 lg:grid-cols-[1.05fr_0.95fr] lg:items-center lg:px-10 ${compact ? "min-h-[400px] gap-8 py-12 sm:py-14" : "min-h-[500px] gap-12 py-20"}`}>
          <div>
            <p className="text-sm font-semibold text-emerald-700">{eyebrow}</p>
            <h1 className={`mt-4 max-w-3xl font-semibold leading-tight text-slate-950 ${compact ? "text-3xl sm:text-4xl" : "text-4xl sm:text-5xl"}`}>{title}</h1>
            <p className={`max-w-2xl leading-7 text-slate-600 ${compact ? "mt-4 text-sm sm:text-base" : "mt-6 text-base sm:text-lg"}`}>{description}</p>
            <div className={`${compact ? "mt-6" : "mt-8"} flex flex-wrap gap-3`}><DemoRequestDialog label={tm("marketing.marketingPageShell.t001")} className="inline-flex h-11 items-center rounded-md bg-blue-600 px-5 text-sm font-semibold text-white transition hover:bg-blue-700" /><a href="#main-content" className="inline-flex h-11 items-center gap-2 rounded-md border border-slate-300 bg-white px-5 text-sm font-semibold text-slate-800 transition hover:border-slate-500">{tm("marketing.marketingPageShell.t003")} <ArrowRightIcon className="size-4" /></a></div>
          </div>
          {aside ? <div>{aside}</div> : null}
        </div>
      </section>
    </>
  )
}

export function MarketingCta({ title, description }: { title: string; description: string }) {
  useI18n()
  return (
    <section className="bg-[#0a1a28] py-16 text-white sm:py-20">
      <div className="mx-auto flex max-w-7xl flex-col gap-8 px-5 sm:px-8 md:flex-row md:items-end md:justify-between lg:px-10">
        <div><p className="text-sm font-semibold text-emerald-300">RemoteHelpDesk</p><h2 className="mt-3 max-w-3xl text-3xl font-semibold leading-tight sm:text-4xl">{title}</h2><p className="mt-4 max-w-2xl text-sm leading-6 text-slate-300">{description}</p></div>
        <DemoRequestDialog label={tm("marketing.marketingPageShell.t002")} className="inline-flex h-11 shrink-0 items-center rounded-md bg-blue-600 px-5 text-sm font-semibold text-white transition hover:bg-blue-700" />
      </div>
    </section>
  )
}

export function MarketingPageFooter() {
  useI18n()
  return <MarketingSiteFooter />
}
