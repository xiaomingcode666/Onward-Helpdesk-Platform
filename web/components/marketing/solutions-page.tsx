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

const solutionSections = [
  {
    number: "01",
    eyebrow: "设备制造商与出海企业",
    title: "设备身份进入每一次服务",
    description: "从服务码和设备档案开始，关联安装区域、型号、保修、历史工单与现场信息。",
    points: ["产品、型号与设备分层管理", "客户资产和服务范围可追溯", "维修记录回到设备生命周期"],
    icon: BoxesIcon,
    tone: "border-emerald-200 bg-emerald-50 text-emerald-800",
  },
  {
    number: "02",
    eyebrow: "知识服务与技术支持团队",
    title: "没有设备，也能完成服务闭环",
    description: "客户直接创建会话并使用企业知识。需要人工时进入维护组，工单无需填写产品或设备。",
    points: ["按租户隐藏产品和设备入口", "知识引用、追问和主动转人工", "会话上下文进入无设备工单"],
    icon: BotMessageSquareIcon,
    tone: "border-sky-200 bg-sky-50 text-sky-800",
  },
  {
    number: "03",
    eyebrow: "区域工程师、伙伴与供应商",
    title: "外部资源参与，但不越过边界",
    description: "总部保留业务规则与客户关系，伙伴和供应商按邀请参与指定工单和远程任务。",
    points: ["按区域、技能和班次组织", "外部人员只看授权事项", "负责人、状态和处理记录完整"],
    icon: NetworkIcon,
    tone: "border-amber-200 bg-amber-50 text-amber-800",
  },
] as const

const governanceItems = [
  [CircleUserRoundIcon, "客户体验", "品牌、语言、会话与进度保持一致。"],
  [TicketCheckIcon, "服务责任", "状态、SLA、负责人和操作记录持续可见。"],
  [VideoIcon, "协作上下文", "会议和外部协作继续关联原始问题。"],
  [ShieldCheckIcon, "治理边界", "租户、权限、凭据和关键动作受到约束。"],
] as const

export function SolutionsPage() {
  return (
    <main className="min-h-screen bg-white text-slate-900">
      <MarketingPageHero
        compact
        eyebrow="解决方案"
        title="按真实服务模式组合能力"
        description="设备售后、知识问答和多方服务网络可以独立使用，也可以共享同一套服务上下文。"
        aside={
          <div className="rounded-lg border border-slate-200 bg-white p-5 shadow-[0_18px_48px_rgba(15,23,42,0.1)]">
            <div className="flex items-center justify-between border-b border-slate-200 pb-3">
              <strong className="text-sm text-slate-950">服务模式</strong>
              <span className="text-xs text-blue-700">Tenant ready</span>
            </div>
            <div className="mt-3 divide-y divide-slate-100">
              {[
                [BoxesIcon, "设备售后", "设备、服务码与维修史"],
                [BotMessageSquareIcon, "知识问答", "知识、会话与人工接管"],
                [Globe2Icon, "协作网络", "总部、伙伴与供应商"],
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
              <p className="text-xs font-semibold text-emerald-700">服务模式</p>
              <h2 className="mt-2 text-2xl font-semibold text-slate-950 sm:text-3xl">选择适合企业的服务起点</h2>
            </div>
            <p className="max-w-md text-sm leading-6 text-slate-600">业务扩展时可以继续启用设备、知识或外部协作能力。</p>
          </div>

          <div className="mt-8 grid gap-5 lg:grid-cols-3">
            {solutionSections.map((item) => {
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
              <p className="text-xs font-semibold text-emerald-700">共同底座</p>
              <h2 className="mt-2 text-2xl font-semibold text-slate-950 sm:text-3xl">不同模式，共用同一业务架构</h2>
            </div>
            <p className="max-w-lg text-sm leading-6 text-slate-600">租户边界、身份权限、会话、工单、知识和审计不会因入口不同而分裂。</p>
          </div>
          <div className="mt-8"><ServiceArchitecture /></div>
        </div>
      </section>

      <section className="py-14 sm:py-16">
        <div className="mx-auto grid max-w-7xl gap-10 px-5 sm:px-8 lg:grid-cols-[0.72fr_1.28fr] lg:px-10">
          <div>
            <ShieldCheckIcon className="size-6 text-blue-700" />
            <h2 className="mt-4 text-2xl font-semibold leading-snug text-slate-950 sm:text-3xl">客户入口简单，企业治理完整</h2>
            <p className="mt-3 text-sm leading-6 text-slate-600">用户只看到当前任务，企业仍掌握完整责任链和数据边界。</p>
          </div>

          <div className="grid border-t border-slate-300 sm:grid-cols-2">
            {governanceItems.map(([Icon, title, description], index) => {
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
