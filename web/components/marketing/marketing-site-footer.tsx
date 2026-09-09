import Link from "next/link"
import { ArrowUpRightIcon, BookOpenTextIcon, DownloadIcon, PlayCircleIcon } from "lucide-react"

import { AppLogoMark } from "@/components/brand-logo"
import { DemoRequestDialog } from "@/components/marketing/demo-request-dialog"

const xiaohongshuUrl = "https://www.xiaohongshu.com/user/profile/5d98d6bc000000000100080e"

export function MarketingSiteFooter() {
  return (
    <footer className="bg-[#07111b] text-slate-300">
      <div className="mx-auto grid max-w-7xl gap-10 px-5 py-14 sm:px-8 md:grid-cols-[1.3fr_0.7fr_0.7fr_1fr] lg:px-10">
        <div>
          <div className="flex items-center gap-3"><AppLogoMark alt="RemoteHelpDesk" className="size-9 border-0 bg-white" imageClassName="p-0.5" /><strong className="text-base text-white">RemoteHelpDesk</strong></div>
          <p className="mt-5 max-w-sm text-sm leading-6 text-slate-400">连接客户入口、AI 知识、人工服务、工单调度与远程协作的工业设备智能售后平台。</p>
          <DemoRequestDialog label="申请产品 Demo" className="mt-6 inline-flex h-10 items-center rounded-md bg-blue-600 px-4 text-sm font-semibold text-white transition hover:bg-blue-700" />
        </div>
        <div>
          <h2 className="text-sm font-semibold text-white">网站</h2>
          <div className="mt-4 grid gap-3 text-sm text-slate-400"><Link href="/solutions" className="hover:text-white">解决方案</Link><Link href="/pricing" className="hover:text-white">价格</Link><Link href="/cases" className="hover:text-white">客户案例</Link><Link href="/about" className="hover:text-white">关于我们</Link></div>
        </div>
        <div>
          <h2 className="text-sm font-semibold text-white">入口</h2>
          <div className="mt-4 grid gap-3 text-sm text-slate-400"><Link href="/dashboard/login" className="hover:text-white">企业登录</Link><Link href="/mobile" className="hover:text-white">客户入口</Link><Link href="/legal/privacy" className="hover:text-white">隐私政策</Link><Link href="/legal/terms" className="hover:text-white">服务条款</Link></div>
        </div>
        <div>
          <h2 className="text-sm font-semibold text-white">内容与工具</h2>
          <div className="mt-4 grid gap-3 text-sm text-slate-400">
            <a href={xiaohongshuUrl} target="_blank" rel="noreferrer" className="flex items-center gap-2 hover:text-white"><BookOpenTextIcon className="size-4" />小红书 · AI民工<ArrowUpRightIcon className="size-3.5" /></a>
            <a href="/resources/remotehelpdesk-demo-45s.mp4" className="flex items-center gap-2 hover:text-white"><PlayCircleIcon className="size-4" />45 秒实机演示</a>
            <a href="/resources/remotehelpdesk-aftersales-template.xlsx" download className="flex items-center gap-2 hover:text-white"><DownloadIcon className="size-4" />售后自测与实施模板</a>
          </div>
        </div>
      </div>
      <div className="border-t border-white/10"><div className="mx-auto flex max-w-7xl flex-col gap-2 px-5 py-5 text-xs text-slate-500 sm:flex-row sm:items-center sm:justify-between sm:px-8 lg:px-10"><span>© 2026 RemoteHelpDesk</span><span>企业 AI × 海外设备售后</span></div></div>
    </footer>
  )
}
