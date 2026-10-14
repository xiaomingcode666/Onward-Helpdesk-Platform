"use client"

import { tm } from "@/components/marketing/marketing-i18n"
import { useI18n } from "@/i18n/provider"
import {
  ArrowUpRightIcon,
  CheckIcon,
  DownloadIcon,
  FileCheck2Icon,
  MessageSquareMoreIcon,
  ShieldCheckIcon,
  UsersRoundIcon,
  WorkflowIcon,
} from "lucide-react"

import { MarketingPageFooter, MarketingPageHero } from "@/components/marketing/marketing-page-shell"

const xiaohongshuUrl = "https://www.xiaohongshu.com/user/profile/5d98d6bc000000000100080e"

const principles = () => [
  [MessageSquareMoreIcon, tm("marketing.aboutPage.t018"), tm("marketing.aboutPage.t019")],
  [WorkflowIcon, tm("marketing.aboutPage.t020"), tm("marketing.aboutPage.t021")],
  [UsersRoundIcon, tm("marketing.aboutPage.t022"), tm("marketing.aboutPage.t023")],
  [ShieldCheckIcon, tm("marketing.aboutPage.t024"), tm("marketing.aboutPage.t025")],
] as const

const buildSteps = () => [
  ["01", tm("marketing.aboutPage.t026"), tm("marketing.aboutPage.t027")],
  ["02", tm("marketing.aboutPage.t028"), tm("marketing.aboutPage.t029")],
  ["03", tm("marketing.aboutPage.t030"), tm("marketing.aboutPage.t031")],
  ["04", tm("marketing.aboutPage.t032"), tm("marketing.aboutPage.t033")],
] as const

export function AboutPage() {
  useI18n()
  return (
    <main className="min-h-screen bg-white text-slate-900">
      <MarketingPageHero
        compact
        eyebrow={tm("marketing.aboutPage.t001")}
        title={tm("marketing.aboutPage.t002")}
        description={tm("marketing.aboutPage.t003")}
        aside={
          <div className="rounded-lg border border-slate-700 bg-[#0b1d2b] p-6 text-white shadow-[0_18px_48px_rgba(15,23,42,0.14)]">
            <p className="text-xs font-semibold text-blue-200">{tm("marketing.aboutPage.t004")}</p>
            <p className="mt-3 text-xl font-semibold leading-snug">{tm("marketing.aboutPage.t005")}</p>
            <div className="mt-5 grid gap-2.5 border-t border-white/10 pt-4 text-xs leading-5 text-slate-300 sm:grid-cols-2">
              {[tm("marketing.aboutPage.t034"), tm("marketing.aboutPage.t035"), tm("marketing.aboutPage.t036"), tm("marketing.aboutPage.t037")].map((item) => (
                <span key={item} className="flex items-center gap-2"><CheckIcon className="size-3.5 shrink-0 text-emerald-300" />{item}</span>
              ))}
            </div>
          </div>
        }
      />

      <section id="main-content" className="scroll-mt-16 py-14 sm:py-16">
        <div className="mx-auto grid max-w-7xl gap-10 px-5 sm:px-8 lg:grid-cols-[0.72fr_1.28fr] lg:px-10">
          <div>
            <p className="text-xs font-semibold text-emerald-700">{tm("marketing.aboutPage.t006")}</p>
            <h2 className="mt-2 text-2xl font-semibold leading-snug text-slate-950 sm:text-3xl">{tm("marketing.aboutPage.t007")}</h2>
            <p className="mt-4 max-w-lg text-sm leading-6 text-slate-600">{tm("marketing.aboutPage.t008")}</p>
          </div>

          <div className="grid border-t border-slate-300 sm:grid-cols-2">
            {principles().map(([Icon, title, description], index) => (
              <article key={title} className={`border-b border-slate-300 py-5 sm:px-5 ${index % 2 === 0 ? "sm:border-r sm:pl-0" : "sm:pr-0"}`}>
                <span className="grid size-9 place-items-center rounded-md bg-slate-100 text-slate-700"><Icon className="size-4.5" /></span>
                <h3 className="mt-4 text-sm font-semibold text-slate-950">{title}</h3>
                <p className="mt-2 text-sm leading-6 text-slate-600">{description}</p>
              </article>
            ))}
          </div>
        </div>
      </section>

      <section className="border-y border-slate-200 bg-[#f7f9f8] py-14 sm:py-16">
        <div className="mx-auto max-w-7xl px-5 sm:px-8 lg:px-10">
          <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <p className="text-xs font-semibold text-emerald-700">{tm("marketing.aboutPage.t009")}</p>
              <h2 className="mt-2 text-2xl font-semibold text-slate-950 sm:text-3xl">{tm("marketing.aboutPage.t010")}</h2>
            </div>
            <p className="max-w-lg text-sm leading-6 text-slate-600">{tm("marketing.aboutPage.t011")}</p>
          </div>

          <div className="mt-8 grid border-y border-slate-300 sm:grid-cols-4">
            {buildSteps().map(([number, title, detail], index) => (
              <div key={number} className={`py-5 sm:px-5 ${index > 0 ? "sm:border-l sm:border-slate-300" : "sm:pl-0"}`}>
                <span className="font-mono text-[11px] font-semibold text-blue-700">{number}</span>
                <h3 className="mt-3 text-sm font-semibold text-slate-950">{title}</h3>
                <p className="mt-1 text-xs text-slate-500">{detail}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      <section className="py-14 sm:py-16">
        <div className="mx-auto grid max-w-7xl gap-10 px-5 sm:px-8 lg:grid-cols-[0.85fr_1.15fr] lg:items-center lg:px-10">
          <div>
            <p className="text-xs font-semibold text-emerald-700">{tm("marketing.aboutPage.t012")}</p>
            <h2 className="mt-2 text-2xl font-semibold leading-snug text-slate-950 sm:text-3xl">{tm("marketing.aboutPage.t013")}</h2>
            <p className="mt-4 max-w-lg text-sm leading-6 text-slate-600">{tm("marketing.aboutPage.t014")}</p>
            <a href={xiaohongshuUrl} target="_blank" rel="noreferrer" className="mt-6 inline-flex h-10 items-center gap-2 rounded-md bg-[#ff2442] px-4 text-sm font-semibold text-white hover:bg-[#e91f3b]">
              {tm("marketing.aboutPage.t038")} <ArrowUpRightIcon className="size-4" />
            </a>
          </div>

          <div className="rounded-lg border border-slate-200 bg-white p-5 shadow-[0_16px_44px_rgba(15,23,42,0.07)] sm:p-6">
            <div className="flex items-start justify-between gap-4">
              <div>
                <p className="text-xs font-semibold text-blue-700">{tm("marketing.aboutPage.t015")}</p>
                <h3 className="mt-2 text-lg font-semibold text-slate-950">{tm("marketing.aboutPage.t016")}</h3>
              </div>
              <FileCheck2Icon className="size-6 shrink-0 text-blue-700" />
            </div>
            <p className="mt-3 text-sm leading-6 text-slate-600">{tm("marketing.aboutPage.t017")}</p>
            <a href="/resources/remotehelpdesk-aftersales-template.xlsx" download className="mt-5 inline-flex h-10 items-center gap-2 rounded-md border border-slate-300 px-4 text-sm font-semibold text-slate-800 hover:border-blue-500 hover:text-blue-700">
              {tm("marketing.aboutPage.t039")} <DownloadIcon className="size-4" />
            </a>
          </div>
        </div>
      </section>

      <MarketingPageFooter />
    </main>
  )
}
