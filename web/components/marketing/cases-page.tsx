"use client"

import { tm } from "@/components/marketing/marketing-i18n"
import { useI18n } from "@/i18n/provider"
import Link from "next/link"
import {
  ArrowRightIcon,
  BotMessageSquareIcon,
  BoxesIcon,
  CheckIcon,
  ExternalLinkIcon,
  NetworkIcon,
  PlayCircleIcon,
} from "lucide-react"

import { MarketingPageFooter, MarketingPageHero } from "@/components/marketing/marketing-page-shell"

const xiaohongshuUrl = "https://www.xiaohongshu.com/user/profile/5d98d6bc000000000100080e"

const cases = () => [
  {
    icon: BotMessageSquareIcon,
    label: tm("marketing.casesPage.t014"),
    title: tm("marketing.casesPage.t015"),
    start: tm("marketing.casesPage.t016"),
    steps: [tm("marketing.casesPage.t017"), tm("marketing.casesPage.t018"), tm("marketing.casesPage.t019"), tm("marketing.casesPage.t020")],
    acceptance: [tm("marketing.casesPage.t021"), tm("marketing.casesPage.t022"), tm("marketing.casesPage.t023")],
    tone: "bg-sky-50 text-sky-800",
  },
  {
    icon: BoxesIcon,
    label: tm("marketing.casesPage.t024"),
    title: tm("marketing.casesPage.t025"),
    start: tm("marketing.casesPage.t026"),
    steps: [tm("marketing.casesPage.t027"), tm("marketing.casesPage.t028"), tm("marketing.casesPage.t029"), tm("marketing.casesPage.t030")],
    acceptance: [tm("marketing.casesPage.t031"), tm("marketing.casesPage.t032"), tm("marketing.casesPage.t033")],
    tone: "bg-emerald-50 text-emerald-800",
  },
  {
    icon: NetworkIcon,
    label: tm("marketing.casesPage.t034"),
    title: tm("marketing.casesPage.t035"),
    start: tm("marketing.casesPage.t036"),
    steps: [tm("marketing.casesPage.t037"), tm("marketing.casesPage.t038"), tm("marketing.casesPage.t039"), tm("marketing.casesPage.t040")],
    acceptance: [tm("marketing.casesPage.t041"), tm("marketing.casesPage.t042"), tm("marketing.casesPage.t043")],
    tone: "bg-amber-50 text-amber-800",
  },
] as const

const contentTopics = () => [
  tm("marketing.casesPage.t044"),
  tm("marketing.casesPage.t045"),
  tm("marketing.casesPage.t046"),
  tm("marketing.casesPage.t047"),
  tm("marketing.casesPage.t048"),
  tm("marketing.casesPage.t049"),
] as const

export function CasesPage() {
  useI18n()
  return (
    <main className="min-h-screen bg-white text-slate-900">
      <MarketingPageHero
        compact
        eyebrow={tm("marketing.casesPage.t001")}
        title={tm("marketing.casesPage.t002")}
        description={tm("marketing.casesPage.t003")}
        aside={
          <div className="overflow-hidden rounded-lg border border-slate-200 bg-[#10263a] shadow-[0_18px_48px_rgba(15,23,42,0.14)]">
            <video controls preload="metadata" poster="/resources/remotehelpdesk-demo-cover.jpg" className="aspect-video max-h-[260px] w-full bg-slate-950 object-cover object-top">
              <source src="/resources/remotehelpdesk-demo-45s.mp4" type="video/mp4" />
              {tm("marketing.casesPage.t050")}
            </video>
            <div className="flex items-center justify-between border-t border-white/10 px-4 py-3 text-xs text-slate-300">
              <span>{tm("marketing.casesPage.t004")}</span>
              <span className="flex items-center gap-1.5 text-blue-200"><PlayCircleIcon className="size-4" />{tm("marketing.casesPage.t005")}</span>
            </div>
          </div>
        }
      />

      <section id="main-content" className="scroll-mt-16 py-14 sm:py-16">
        <div className="mx-auto max-w-7xl px-5 sm:px-8 lg:px-10">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <p className="text-xs font-semibold text-emerald-700">{tm("marketing.casesPage.t006")}</p>
              <h2 className="mt-2 text-2xl font-semibold text-slate-950 sm:text-3xl">{tm("marketing.casesPage.t007")}</h2>
            </div>
            <p className="max-w-md text-sm leading-6 text-slate-600">{tm("marketing.casesPage.t008")}</p>
          </div>

          <div className="mt-8 grid gap-5 lg:grid-cols-3">
            {cases().map((item, index) => {
              const Icon = item.icon
              return (
                <article key={item.label} className="flex flex-col rounded-lg border border-slate-200 bg-white p-6 shadow-[0_12px_36px_rgba(15,23,42,0.05)]">
                  <div className="flex items-center justify-between gap-4">
                    <span className={`grid size-10 place-items-center rounded-md ${item.tone}`}><Icon className="size-5" /></span>
                    <span className="font-mono text-[11px] text-slate-400">0{index + 1}</span>
                  </div>
                  <p className="mt-5 text-xs font-semibold text-emerald-700">{item.label}</p>
                  <h3 className="mt-2 text-xl font-semibold leading-snug text-slate-950">{item.title}</h3>
                  <p className="mt-4 text-sm leading-6 text-slate-600">{item.start}</p>

                  <ol className="mt-5 grid gap-2 border-t border-slate-100 pt-5">
                    {item.steps.map((step, stepIndex) => (
                      <li key={step} className="flex items-center gap-3 text-sm text-slate-700">
                        <span className="font-mono text-[10px] text-slate-400">{String(stepIndex + 1).padStart(2, "0")}</span>
                        {step}
                      </li>
                    ))}
                  </ol>

                  <ul className="mt-5 grid gap-2 border-t border-slate-100 pt-5">
                    {item.acceptance.map((point) => (
                      <li key={point} className="flex items-center gap-2 text-xs text-slate-600">
                        <CheckIcon className="size-3.5 shrink-0 text-emerald-600" />{point}
                      </li>
                    ))}
                  </ul>
                </article>
              )
            })}
          </div>
        </div>
      </section>

      <section className="border-y border-slate-200 bg-[#f7f9f8] py-14 sm:py-16">
        <div className="mx-auto grid max-w-7xl gap-10 px-5 sm:px-8 lg:grid-cols-[0.8fr_1.2fr] lg:items-center lg:px-10">
          <div>
            <p className="text-xs font-semibold text-emerald-700">{tm("marketing.casesPage.t009")}</p>
            <h2 className="mt-2 text-2xl font-semibold leading-snug text-slate-950 sm:text-3xl">{tm("marketing.casesPage.t010")}</h2>
            <p className="mt-4 max-w-lg text-sm leading-6 text-slate-600">{tm("marketing.casesPage.t011")}</p>
            <a href={xiaohongshuUrl} target="_blank" rel="noreferrer" className="mt-6 inline-flex h-10 items-center gap-2 rounded-md bg-[#ff2442] px-4 text-sm font-semibold text-white transition hover:bg-[#e91f3b]">
              {tm("marketing.casesPage.t051")} <ExternalLinkIcon className="size-4" />
            </a>
          </div>

          <div className="grid border-t border-slate-300 sm:grid-cols-2">
            {contentTopics().map((title, index) => (
              <div key={title} className={`flex min-h-20 items-center gap-4 border-b border-slate-300 py-4 sm:px-5 ${index % 2 === 0 ? "sm:border-r sm:pl-0" : "sm:pr-0"}`}>
                <span className="font-mono text-[10px] text-slate-400">{String(index + 1).padStart(2, "0")}</span>
                <h3 className="text-sm font-semibold text-slate-800">{title}</h3>
              </div>
            ))}
          </div>
        </div>
      </section>

      <section className="py-12 sm:py-14">
        <div className="mx-auto flex max-w-7xl flex-col gap-5 px-5 sm:px-8 md:flex-row md:items-center md:justify-between lg:px-10">
          <div>
            <h2 className="text-xl font-semibold text-slate-950 sm:text-2xl">{tm("marketing.casesPage.t012")}</h2>
            <p className="mt-2 text-sm leading-6 text-slate-600">{tm("marketing.casesPage.t013")}</p>
          </div>
          <Link href="/solutions" className="inline-flex h-10 w-fit items-center gap-2 rounded-md border border-slate-300 px-4 text-sm font-semibold text-slate-800 hover:border-blue-500 hover:text-blue-700">
            {tm("marketing.casesPage.t052")} <ArrowRightIcon className="size-4" />
          </Link>
        </div>
      </section>

      <MarketingPageFooter />
    </main>
  )
}
