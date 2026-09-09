import Link from "next/link"
import {
  ArrowRightIcon,
  BotMessageSquareIcon,
  DownloadIcon,
  ExternalLinkIcon,
  FileCheck2Icon,
  HeadphonesIcon,
  NetworkIcon,
  PlayCircleIcon,
  TicketCheckIcon,
  UsersRoundIcon,
} from "lucide-react"

import { DemoRequestDialog } from "@/components/marketing/demo-request-dialog"
import { MarketingSiteFooter } from "@/components/marketing/marketing-site-footer"
import { MarketingSiteHeader } from "@/components/marketing/marketing-site-header"
import { ProductShowcaseScroller } from "@/components/marketing/product-showcase-scroller"
import { ServiceArchitecture } from "@/components/marketing/service-architecture"

const xiaohongshuUrl = "https://www.xiaohongshu.com/user/profile/5d98d6bc000000000100080e"

const serviceSignals = [
  [UsersRoundIcon, "客户入口"],
  [BotMessageSquareIcon, "AI 问诊"],
  [TicketCheckIcon, "工单协同"],
  [HeadphonesIcon, "远程服务"],
] as const

function SectionLead({ eyebrow, title }: { eyebrow: string; title: string }) {
  return (
    <div>
      <p className="text-xs font-semibold uppercase text-emerald-700">{eyebrow}</p>
      <h2 className="mt-3 max-w-3xl text-3xl font-semibold leading-tight text-slate-950 sm:text-4xl">{title}</h2>
    </div>
  )
}

export function MarketingHomePage() {
  return (
    <main className="min-h-screen bg-white text-slate-900">
      <MarketingSiteHeader />

      <section id="product-interface" className="relative isolate scroll-mt-16 overflow-hidden border-b border-slate-200 bg-[#f3f7f5] pt-10 sm:pt-14">
        <div className="relative mx-auto max-w-5xl px-5 text-center sm:px-8">
          <h1 className="mx-auto max-w-4xl text-4xl font-semibold leading-[1.12] text-slate-950 sm:text-5xl lg:text-[56px]">工业设备智能售后平台</h1>
          <p className="mx-auto mt-4 max-w-2xl text-base leading-7 text-slate-600 sm:text-lg">从客户提问到工程师处理，始终保留同一份服务上下文。</p>
          <div className="mt-6 flex flex-wrap justify-center gap-3">
            <DemoRequestDialog label="申请产品 Demo" className="inline-flex h-11 items-center gap-2 rounded-md bg-blue-600 px-5 text-sm font-semibold text-white transition hover:bg-blue-700" />
            <a href="#architecture" className="inline-flex h-11 items-center gap-2 rounded-md border border-slate-300 bg-white px-5 text-sm font-semibold text-slate-800 transition hover:border-slate-500">查看服务架构 <ArrowRightIcon className="size-4" /></a>
          </div>
        </div>

        <div className="mt-9 sm:mt-10">
          <ProductShowcaseScroller />
        </div>

        <div className="mt-3 border-t border-slate-200 bg-white/80">
          <div className="mx-auto grid w-full max-w-7xl grid-cols-2 px-5 sm:grid-cols-4 sm:px-8 lg:px-10">
            {serviceSignals.map(([Icon, label]) => <div key={label} className="flex min-h-20 items-center justify-center gap-3 border-r border-slate-200 py-4 last:border-r-0 sm:px-5"><Icon className="size-5 shrink-0 text-emerald-600" /><span className="text-sm font-medium text-slate-700">{label}</span></div>)}
          </div>
        </div>
      </section>

      <section id="architecture" className="scroll-mt-16 bg-white py-16 sm:py-20">
        <div className="mx-auto max-w-7xl px-5 sm:px-8 lg:px-10">
          <div className="flex flex-col gap-5 md:flex-row md:items-end md:justify-between">
            <SectionLead eyebrow="Architecture" title="一个上下文，连接四个服务入口" />
            <span className="hidden items-center gap-2 font-mono text-[10px] text-slate-400 md:flex"><NetworkIcon className="size-4 text-emerald-600" /> TENANT-ISOLATED SERVICE CORE</span>
          </div>
          <div className="mt-9"><ServiceArchitecture /></div>
        </div>
      </section>

      <section id="resources" className="scroll-mt-16 border-y border-slate-200 bg-[#edf3f1] py-16 sm:py-20">
        <div className="mx-auto max-w-7xl px-5 sm:px-8 lg:px-10">
          <div className="grid overflow-hidden border-y border-slate-300 bg-white lg:grid-cols-[0.8fr_0.42fr_0.78fr]">
            <div className="flex flex-col justify-center border-b border-slate-200 p-7 sm:p-10 lg:border-b-0 lg:border-r">
              <p className="text-xs font-semibold text-emerald-700">LIVE DEMO</p>
              <h2 className="mt-3 text-3xl font-semibold leading-tight text-slate-950">46 秒，看完整服务闭环</h2>
              <div className="mt-7 flex flex-wrap gap-3">
                <a href="/resources/remotehelpdesk-demo-45s.mp4" className="inline-flex h-11 items-center gap-2 rounded-md bg-slate-950 px-5 text-sm font-semibold text-white hover:bg-slate-800"><PlayCircleIcon className="size-4" />播放演示</a>
                <a href={xiaohongshuUrl} target="_blank" rel="noreferrer" className="inline-flex h-11 items-center gap-2 rounded-md border border-slate-300 px-5 text-sm font-semibold text-slate-800 hover:border-slate-500">小红书 <ExternalLinkIcon className="size-4" /></a>
              </div>
            </div>

            <div className="bg-[#0b1a26] p-4">
              <video controls preload="metadata" poster="/resources/remotehelpdesk-demo-cover.jpg" className="mx-auto aspect-[9/16] h-full max-h-[460px] w-full object-cover"><source src="/resources/remotehelpdesk-demo-45s.mp4" type="video/mp4" />您的浏览器不支持视频播放。</video>
            </div>

            <div className="flex flex-col justify-center border-t border-slate-200 p-7 sm:p-10 lg:border-l lg:border-t-0">
              <span className="grid size-11 place-items-center rounded-md bg-emerald-50 text-emerald-700"><FileCheck2Icon className="size-5" /></span>
              <p className="mt-6 text-xs font-semibold text-emerald-700">FREE TOOLKIT</p>
              <h3 className="mt-2 text-2xl font-semibold text-slate-950">售后自测与实施模板</h3>
              <p className="mt-3 text-sm leading-6 text-slate-600">成熟度、工单流程与知识库清单。</p>
              <a href="/resources/remotehelpdesk-aftersales-template.xlsx" download className="mt-7 inline-flex h-11 w-fit items-center gap-2 rounded-md border border-slate-300 px-5 text-sm font-semibold text-slate-800 hover:border-emerald-600 hover:text-emerald-700">下载 Excel <DownloadIcon className="size-4" /></a>
            </div>
          </div>
        </div>
      </section>

      <section className="border-t border-slate-200 bg-white py-16 sm:py-20">
        <div className="mx-auto flex max-w-7xl flex-col gap-7 px-5 sm:px-8 md:flex-row md:items-center md:justify-between lg:px-10">
          <h2 className="max-w-2xl text-3xl font-semibold leading-tight text-slate-950 sm:text-4xl">带着一个真实问题，看看服务如何跑起来</h2>
          <div className="flex shrink-0 flex-wrap gap-3"><DemoRequestDialog label="申请产品 Demo" className="inline-flex h-11 items-center rounded-md bg-blue-600 px-5 text-sm font-semibold text-white transition hover:bg-blue-700" /><Link href="/pricing" className="inline-flex h-11 items-center gap-2 rounded-md border border-slate-300 px-5 text-sm font-semibold text-slate-800 hover:border-slate-500">查看价格 <ArrowRightIcon className="size-4" /></Link></div>
        </div>
      </section>

      <MarketingSiteFooter />
    </main>
  )
}
