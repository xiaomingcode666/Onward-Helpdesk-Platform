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

const principles = [
  [MessageSquareMoreIcon, "从真实问题出发", "先理解客户怎样发起问题、谁负责处理，以及上下文在哪里丢失。"],
  [WorkflowIcon, "让流程约束动作", "AI 负责理解和建议，状态、权限与派单继续由平台规则负责。"],
  [UsersRoundIcon, "为角色设计入口", "客户、工程师、伙伴与平台管理员只看到符合职责的信息。"],
  [ShieldCheckIcon, "把治理放进底层", "租户隔离、角色权限、凭据边界和操作审计从一开始就存在。"],
] as const

const buildSteps = [
  ["01", "梳理场景", "明确入口与问题"],
  ["02", "配置租户", "设置角色与流程"],
  ["03", "真实试点", "使用真实客户验证"],
  ["04", "持续复盘", "扩展知识与协作"],
] as const

export function AboutPage() {
  return (
    <main className="min-h-screen bg-white text-slate-900">
      <MarketingPageHero
        compact
        eyebrow="关于我们"
        title="让海外设备服务拥有清晰上下文"
        description="RemoteHelpDesk 连接客户入口、企业知识、人工服务、工单调度与远程协作。"
        aside={
          <div className="rounded-lg border border-slate-700 bg-[#0b1d2b] p-6 text-white shadow-[0_18px_48px_rgba(15,23,42,0.14)]">
            <p className="text-xs font-semibold text-blue-200">我们关注的问题</p>
            <p className="mt-3 text-xl font-semibold leading-snug">跨越语言、时区与组织之后，客户问题怎样仍被完整解决？</p>
            <div className="mt-5 grid gap-2.5 border-t border-white/10 pt-4 text-xs leading-5 text-slate-300 sm:grid-cols-2">
              {["统一客户入口", "AI 与人工衔接", "外部协作受控", "服务记录复用"].map((item) => (
                <span key={item} className="flex items-center gap-2"><CheckIcon className="size-3.5 shrink-0 text-emerald-300" />{item}</span>
              ))}
            </div>
          </div>
        }
      />

      <section id="main-content" className="scroll-mt-16 py-14 sm:py-16">
        <div className="mx-auto grid max-w-7xl gap-10 px-5 sm:px-8 lg:grid-cols-[0.72fr_1.28fr] lg:px-10">
          <div>
            <p className="text-xs font-semibold text-emerald-700">产品方向</p>
            <h2 className="mt-2 text-2xl font-semibold leading-snug text-slate-950 sm:text-3xl">一套完整的服务运营系统</h2>
            <p className="mt-4 max-w-lg text-sm leading-6 text-slate-600">从 AI 问答到客户确认，每个动作始终围绕同一客户和服务上下文发生。</p>
          </div>

          <div className="grid border-t border-slate-300 sm:grid-cols-2">
            {principles.map(([Icon, title, description], index) => (
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
              <p className="text-xs font-semibold text-emerald-700">建设方法</p>
              <h2 className="mt-2 text-2xl font-semibold text-slate-950 sm:text-3xl">先跑通闭环，再扩大自动化</h2>
            </div>
            <p className="max-w-lg text-sm leading-6 text-slate-600">从一个真实场景开始，入口、责任和验收明确后再扩大范围。</p>
          </div>

          <div className="mt-8 grid border-y border-slate-300 sm:grid-cols-4">
            {buildSteps.map(([number, title, detail], index) => (
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
            <p className="text-xs font-semibold text-emerald-700">公开内容</p>
            <h2 className="mt-2 text-2xl font-semibold leading-snug text-slate-950 sm:text-3xl">系统实拍、架构拆解与工具</h2>
            <p className="mt-4 max-w-lg text-sm leading-6 text-slate-600">通过小红书账号“AI民工”持续分享企业 AI、工单协同和海外设备售后实践。</p>
            <a href={xiaohongshuUrl} target="_blank" rel="noreferrer" className="mt-6 inline-flex h-10 items-center gap-2 rounded-md bg-[#ff2442] px-4 text-sm font-semibold text-white hover:bg-[#e91f3b]">
              关注小红书 · AI民工 <ArrowUpRightIcon className="size-4" />
            </a>
          </div>

          <div className="rounded-lg border border-slate-200 bg-white p-5 shadow-[0_16px_44px_rgba(15,23,42,0.07)] sm:p-6">
            <div className="flex items-start justify-between gap-4">
              <div>
                <p className="text-xs font-semibold text-blue-700">免费资源</p>
                <h3 className="mt-2 text-lg font-semibold text-slate-950">海外设备售后自测与实施模板</h3>
              </div>
              <FileCheck2Icon className="size-6 shrink-0 text-blue-700" />
            </div>
            <p className="mt-3 text-sm leading-6 text-slate-600">包含成熟度自测、选型清单、工单流程、AI 知识库清单和线索诊断。</p>
            <a href="/resources/remotehelpdesk-aftersales-template.xlsx" download className="mt-5 inline-flex h-10 items-center gap-2 rounded-md border border-slate-300 px-4 text-sm font-semibold text-slate-800 hover:border-blue-500 hover:text-blue-700">
              下载 Excel 模板 <DownloadIcon className="size-4" />
            </a>
          </div>
        </div>
      </section>

      <MarketingPageFooter />
    </main>
  )
}
