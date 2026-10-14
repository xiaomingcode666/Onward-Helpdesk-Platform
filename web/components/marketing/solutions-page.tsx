"use client"

import { tm } from "@/components/marketing/marketing-i18n"
import { useI18n } from "@/i18n/provider"
import {
  BotMessageSquareIcon,
  BoxesIcon,
  CheckIcon,
  CircleUserRoundIcon,
  Globe2Icon,
  NetworkIcon,
  ShieldCheckIcon,
  TicketCheckIcon,
  VideoIcon,
} from "lucide-react"

import { MarketingPageFooter, MarketingPageHero } from "@/components/marketing/marketing-page-shell"
import { ServiceArchitecture } from "@/components/marketing/service-architecture"

const solutionSections = () => [
  {
    number: "01",
    eyebrow: tm("marketing.solutionsPage.t012"),
    title: tm("marketing.solutionsPage.t013"),
    description: tm("marketing.solutionsPage.t014"),
    points: [tm("marketing.solutionsPage.t015"), tm("marketing.solutionsPage.t016"), tm("marketing.solutionsPage.t017")],
    icon: BoxesIcon,
    tone: "border-emerald-200 bg-emerald-50 text-emerald-800",
  },
  {
    number: "02",
    eyebrow: tm("marketing.solutionsPage.t018"),
    title: tm("marketing.solutionsPage.t019"),
    description: tm("marketing.solutionsPage.t020"),
    points: [tm("marketing.solutionsPage.t021"), tm("marketing.solutionsPage.t022"), tm("marketing.solutionsPage.t023")],
    icon: BotMessageSquareIcon,
    tone: "border-sky-200 bg-sky-50 text-sky-800",
  },
  {
    number: "03",
    eyebrow: tm("marketing.solutionsPage.t024"),
    title: tm("marketing.solutionsPage.t025"),
    description: tm("marketing.solutionsPage.t026"),
    points: [tm("marketing.solutionsPage.t027"), tm("marketing.solutionsPage.t028"), tm("marketing.solutionsPage.t029")],
    icon: NetworkIcon,
    tone: "border-amber-200 bg-amber-50 text-amber-800",
  },
] as const

const governanceItems = () => [
  [CircleUserRoundIcon, tm("marketing.solutionsPage.t030"), tm("marketing.solutionsPage.t031")],
  [TicketCheckIcon, tm("marketing.solutionsPage.t032"), tm("marketing.solutionsPage.t033")],
  [VideoIcon, tm("marketing.solutionsPage.t034"), tm("marketing.solutionsPage.t035")],
  [ShieldCheckIcon, tm("marketing.solutionsPage.t036"), tm("marketing.solutionsPage.t037")],
] as const

export function SolutionsPage() {
  useI18n()
  return (
    <main className="min-h-screen bg-white text-slate-900">
      <MarketingPageHero
        compact
        eyebrow={tm("marketing.solutionsPage.t001")}
        title={tm("marketing.solutionsPage.t002")}
        description={tm("marketing.solutionsPage.t003")}
        aside={
          <div className="rounded-lg border border-slate-200 bg-white p-5 shadow-[0_18px_48px_rgba(15,23,42,0.1)]">
            <div className="flex items-center justify-between border-b border-slate-200 pb-3">
              <strong className="text-sm text-slate-950">{tm("marketing.solutionsPage.t004")}</strong>
              <span className="text-xs text-blue-700">Tenant ready</span>
            </div>
            <div className="mt-3 divide-y divide-slate-100">
              {[
                [BoxesIcon, tm("marketing.solutionsPage.t038"), tm("marketing.solutionsPage.t039")],
                [BotMessageSquareIcon, tm("marketing.solutionsPage.t040"), tm("marketing.solutionsPage.t041")],
                [Globe2Icon, tm("marketing.solutionsPage.t042"), tm("marketing.solutionsPage.t043")],
              ].map(([Icon, title, detail]) => {
                const ItemIcon = Icon as typeof BoxesIcon
                return (
                  <div key={title as string} className="grid grid-cols-[36px_1fr] items-center gap-3 py-3">
                    <span className="grid size-8 place-items-center rounded-md bg-slate-100 text-slate-700"><ItemIcon className="size-4" /></span>
                    <div><p className="text-sm font-semibold text-slate-950">{title as string}</p><p className="mt-0.5 text-xs text-slate-500">{detail as string}</p></div>
                  </div>
                )
              })}
            </div>
          </div>
        }
      />

      <section id="main-content" className="scroll-mt-16 py-14 sm:py-16">
        <div className="mx-auto max-w-7xl px-5 sm:px-8 lg:px-10">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <p className="text-xs font-semibold text-emerald-700">{tm("marketing.solutionsPage.t004")}</p>
              <h2 className="mt-2 text-2xl font-semibold text-slate-950 sm:text-3xl">{tm("marketing.solutionsPage.t005")}</h2>
            </div>
            <p className="max-w-md text-sm leading-6 text-slate-600">{tm("marketing.solutionsPage.t006")}</p>
          </div>

          <div className="mt-8 grid gap-5 lg:grid-cols-3">
            {solutionSections().map((item) => {
              const Icon = item.icon
              return (
                <article key={item.number} className="flex flex-col rounded-lg border border-slate-200 bg-white p-6 shadow-[0_12px_36px_rgba(15,23,42,0.05)]">
                  <div className="flex items-center justify-between gap-4">
                    <span className={`grid size-10 place-items-center rounded-md border ${item.tone}`}><Icon className="size-5" /></span>
                    <span className="font-mono text-[11px] font-semibold text-slate-400">{item.number}</span>
                  </div>
                  <p className="mt-5 text-xs font-semibold text-emerald-700">{item.eyebrow}</p>
                  <h3 className="mt-2 text-xl font-semibold leading-snug text-slate-950">{item.title}</h3>
                  <p className="mt-4 text-sm leading-6 text-slate-600">{item.description}</p>
                  <ul className="mt-5 grid gap-3 border-t border-slate-100 pt-5">
                    {item.points.map((point) => (
                      <li key={point} className="flex items-start gap-2 text-sm text-slate-700"><CheckIcon className="mt-0.5 size-4 shrink-0 text-emerald-600" />{point}</li>
                    ))}
                  </ul>
                </article>
              )
            })}
          </div>
        </div>
      </section>

      <section className="border-y border-slate-200 bg-[#f7f9f8] py-14 sm:py-16">
        <div className="mx-auto max-w-7xl px-5 sm:px-8 lg:px-10">
          <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <p className="text-xs font-semibold text-emerald-700">{tm("marketing.solutionsPage.t007")}</p>
              <h2 className="mt-2 text-2xl font-semibold text-slate-950 sm:text-3xl">{tm("marketing.solutionsPage.t008")}</h2>
            </div>
            <p className="max-w-lg text-sm leading-6 text-slate-600">{tm("marketing.solutionsPage.t009")}</p>
          </div>
          <div className="mt-8"><ServiceArchitecture /></div>
        </div>
      </section>

      <section className="py-14 sm:py-16">
        <div className="mx-auto grid max-w-7xl gap-10 px-5 sm:px-8 lg:grid-cols-[0.72fr_1.28fr] lg:px-10">
          <div>
            <ShieldCheckIcon className="size-6 text-blue-700" />
            <h2 className="mt-4 text-2xl font-semibold leading-snug text-slate-950 sm:text-3xl">{tm("marketing.solutionsPage.t010")}</h2>
            <p className="mt-3 text-sm leading-6 text-slate-600">{tm("marketing.solutionsPage.t011")}</p>
          </div>

          <div className="grid border-t border-slate-300 sm:grid-cols-2">
            {governanceItems().map(([Icon, title, description], index) => {
              const ItemIcon = Icon
              return (
                <div key={title} className={`border-b border-slate-300 py-5 sm:px-5 ${index % 2 === 0 ? "sm:border-r sm:pl-0" : "sm:pr-0"}`}>
                  <ItemIcon className="size-4.5 text-slate-700" />
                  <h3 className="mt-3 text-sm font-semibold text-slate-950">{title}</h3>
                  <p className="mt-1.5 text-sm leading-6 text-slate-600">{description}</p>
                </div>
              )
            })}
          </div>
        </div>
      </section>

      <MarketingPageFooter />
    </main>
  )
}
