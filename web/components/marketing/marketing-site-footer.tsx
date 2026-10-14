"use client"

import { tm } from "@/components/marketing/marketing-i18n"
import { useI18n } from "@/i18n/provider"
import Link from "next/link"
import { ArrowUpRightIcon, BookOpenTextIcon, DownloadIcon, PlayCircleIcon } from "lucide-react"

import { AppLogoMark } from "@/components/brand-logo"
import { DemoRequestDialog } from "@/components/marketing/demo-request-dialog"

const xiaohongshuUrl = "https://www.xiaohongshu.com/user/profile/5d98d6bc000000000100080e"

export function MarketingSiteFooter() {
  useI18n()
  return (
    <footer className="bg-[#07111b] text-slate-300">
      <div className="mx-auto grid max-w-7xl gap-10 px-5 py-14 sm:px-8 md:grid-cols-[1.3fr_0.7fr_0.7fr_1fr] lg:px-10">
        <div>
          <div className="flex items-center gap-3"><AppLogoMark alt="RemoteHelpDesk" className="size-9 border-0 bg-white" imageClassName="p-0.5" /><strong className="text-base text-white">RemoteHelpDesk</strong></div>
          <p className="mt-5 max-w-sm text-sm leading-6 text-slate-400">{tm("marketing.marketingSiteFooter.t002")}</p>
          <DemoRequestDialog label={tm("marketing.marketingSiteFooter.t001")} className="mt-6 inline-flex h-10 items-center rounded-md bg-blue-600 px-4 text-sm font-semibold text-white transition hover:bg-blue-700" />
        </div>
        <div>
          <h2 className="text-sm font-semibold text-white">{tm("marketing.marketingSiteFooter.t003")}</h2>
          <div className="mt-4 grid gap-3 text-sm text-slate-400"><Link href="/solutions" className="hover:text-white">{tm("marketing.marketingSiteFooter.t004")}</Link><Link href="/pricing" className="hover:text-white">{tm("marketing.marketingSiteFooter.t005")}</Link><Link href="/cases" className="hover:text-white">{tm("marketing.marketingSiteFooter.t006")}</Link><Link href="/about" className="hover:text-white">{tm("marketing.marketingSiteFooter.t007")}</Link></div>
        </div>
        <div>
          <h2 className="text-sm font-semibold text-white">{tm("marketing.marketingSiteFooter.t008")}</h2>
          <div className="mt-4 grid gap-3 text-sm text-slate-400"><Link href="/dashboard/login" className="hover:text-white">{tm("marketing.marketingSiteFooter.t009")}</Link><Link href="/mobile" className="hover:text-white">{tm("marketing.marketingSiteFooter.t010")}</Link><Link href="/legal/privacy" className="hover:text-white">{tm("marketing.marketingSiteFooter.t011")}</Link><Link href="/legal/terms" className="hover:text-white">{tm("marketing.marketingSiteFooter.t012")}</Link></div>
        </div>
        <div>
          <h2 className="text-sm font-semibold text-white">{tm("marketing.marketingSiteFooter.t013")}</h2>
          <div className="mt-4 grid gap-3 text-sm text-slate-400">
            <a href={xiaohongshuUrl} target="_blank" rel="noreferrer" className="flex items-center gap-2 hover:text-white"><BookOpenTextIcon className="size-4" />{tm("marketing.marketingSiteFooter.t014")}<ArrowUpRightIcon className="size-3.5" /></a>
            <a href="/resources/remotehelpdesk-demo-45s.mp4" className="flex items-center gap-2 hover:text-white"><PlayCircleIcon className="size-4" />{tm("marketing.marketingSiteFooter.t015")}</a>
            <a href="/resources/remotehelpdesk-aftersales-template.xlsx" download className="flex items-center gap-2 hover:text-white"><DownloadIcon className="size-4" />{tm("marketing.marketingSiteFooter.t016")}</a>
          </div>
        </div>
      </div>
      <div className="border-t border-white/10"><div className="mx-auto flex max-w-7xl flex-col gap-2 px-5 py-5 text-xs text-slate-500 sm:flex-row sm:items-center sm:justify-between sm:px-8 lg:px-10"><span>© 2026 RemoteHelpDesk</span><span>{tm("marketing.marketingSiteFooter.t017")}</span></div></div>
    </footer>
  )
}
