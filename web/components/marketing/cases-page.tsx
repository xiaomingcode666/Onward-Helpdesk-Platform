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

const cases = [
  {
    icon: BotMessageSquareIcon,
    label: "知识问答型服务",
    title: "无需设备，直接问答与转人工",
    start: "企业以知识支持为主，不需要产品树和设备绑定。",
    steps: ["品牌门户发起会话", "企业知识回答与引用", "客户主动转人工", "无设备工单闭环"],
    acceptance: ["隐藏产品和设备", "完整保留会话", "支持工程师独立建单"],
    tone: "bg-sky-50 text-sky-800",
  },
  {
    icon: BoxesIcon,
    label: "设备售后型服务",
    title: "设备身份贯穿维修全流程",
    start: "海外报修入口分散，客户、型号和历史记录需要人工确认。",
    steps: ["服务码识别设备", "AI 收集故障信息", "会话转入工单", "维修结果归档"],
    acceptance: ["服务对象可追溯", "自动携带设备上下文", "结案进入维修记录"],
    tone: "bg-emerald-50 text-emerald-800",
  },
  {
    icon: NetworkIcon,
    label: "多方协作型服务",
    title: "总部治理，外部资源协同执行",
    start: "服务跨区域、跨语言，外部资源需要参与但不能越过数据边界。",
    steps: ["总部受理与派单", "伙伴或供应商接单", "远程协作处理", "客户确认结果"],
    acceptance: ["外部权限受限", "责任人与处理组清晰", "协作过程完整留痕"],
    tone: "bg-amber-50 text-amber-800",
  },
] as const

const contentTopics = [
  "海外设备售后系统实拍",
  "海外售后技术架构拆解",
  "AI 接待与人工接管",
  "完整服务链路管理",
  "售后运营关键指标",
  "重复故障与知识沉淀",
] as const

export function CasesPage() {
  return (
    <main className="min-h-screen bg-white text-slate-900">
      <MarketingPageHero
        compact
        eyebrow="客户案例"
        title="从真实服务场景，看系统如何落地"
        description="用入口、角色、处理路径和验收结果，呈现不同企业的售后服务方式。"
        aside={
          <div className="overflow-hidden rounded-lg border border-slate-200 bg-[#10263a] shadow-[0_18px_48px_rgba(15,23,42,0.14)]">
            <video controls preload="metadata" poster="/resources/remotehelpdesk-demo-cover.jpg" className="aspect-video max-h-[260px] w-full bg-slate-950 object-cover object-top">
              <source src="/resources/remotehelpdesk-demo-45s.mp4" type="video/mp4" />
              您的浏览器不支持视频播放。
            </video>
            <div className="flex items-center justify-between border-t border-white/10 px-4 py-3 text-xs text-slate-300">
              <span>RemoteHelpDesk 实机演示</span>
              <span className="flex items-center gap-1.5 text-blue-200"><PlayCircleIcon className="size-4" />46 秒</span>
            </div>
          </div>
        }
      />

      <section id="main-content" className="scroll-mt-16 py-14 sm:py-16">
        <div className="mx-auto max-w-7xl px-5 sm:px-8 lg:px-10">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <p className="text-xs font-semibold text-emerald-700">典型场景</p>
              <h2 className="mt-2 text-2xl font-semibold text-slate-950 sm:text-3xl">三种常见服务模式</h2>
            </div>
            <p className="max-w-md text-sm leading-6 text-slate-600">每种模式都从客户入口开始，以明确的服务结果结束。</p>
          </div>

          <div className="mt-8 grid gap-5 lg:grid-cols-3">
            {cases.map((item, index) => {
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
            <p className="text-xs font-semibold text-emerald-700">持续更新</p>
            <h2 className="mt-2 text-2xl font-semibold leading-snug text-slate-950 sm:text-3xl">系统实拍与售后实践</h2>
            <p className="mt-4 max-w-lg text-sm leading-6 text-slate-600">在小红书账号“AI民工”查看产品实拍、架构拆解和海外售后方法。</p>
            <a href={xiaohongshuUrl} target="_blank" rel="noreferrer" className="mt-6 inline-flex h-10 items-center gap-2 rounded-md bg-[#ff2442] px-4 text-sm font-semibold text-white transition hover:bg-[#e91f3b]">
              打开小红书主页 <ExternalLinkIcon className="size-4" />
            </a>
          </div>

          <div className="grid border-t border-slate-300 sm:grid-cols-2">
            {contentTopics.map((title, index) => (
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
            <h2 className="text-xl font-semibold text-slate-950 sm:text-2xl">用自己的业务场景验证</h2>
            <p className="mt-2 text-sm leading-6 text-slate-600">选择一种服务模式，再用真实问题准备针对性演示。</p>
          </div>
          <Link href="/solutions" className="inline-flex h-10 w-fit items-center gap-2 rounded-md border border-slate-300 px-4 text-sm font-semibold text-slate-800 hover:border-blue-500 hover:text-blue-700">
            查看解决方案 <ArrowRightIcon className="size-4" />
          </Link>
        </div>
      </section>

      <MarketingPageFooter />
    </main>
  )
}
