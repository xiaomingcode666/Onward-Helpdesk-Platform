import {
  CheckIcon,
  CloudCogIcon,
  MinusIcon,
  ServerCogIcon,
  Settings2Icon,
} from "lucide-react"

import { DemoRequestDialog } from "@/components/marketing/demo-request-dialog"
import { MarketingSiteFooter } from "@/components/marketing/marketing-site-footer"
import { MarketingSiteHeader } from "@/components/marketing/marketing-site-header"

const plans = [
  {
    icon: Settings2Icon,
    stage: "验证上线",
    name: "场景试点",
    price: "按试点范围报价",
    description: "用一条真实服务链路，快速验证客户入口、AI 问答和工单闭环。",
    items: ["1 个租户与品牌门户", "知识问答与人工接管", "1 条工单工作流", "试点配置与验收"],
    action: "咨询试点方案",
    recommended: false,
  },
  {
    icon: CloudCogIcon,
    stage: "规模运营",
    name: "团队运营",
    price: "按团队规模报价",
    description: "面向日常售后运营，连接客户、工程师、服务商与完整服务流程。",
    items: ["多团队与角色权限", "产品、设备与服务码", "派单、SLA 与通知", "视频协作与运营视图"],
    action: "咨询团队方案",
    recommended: true,
  },
  {
    icon: ServerCogIcon,
    stage: "独立部署",
    name: "私有化交付",
    price: "按部署范围报价",
    description: "在完整运营能力之上，提供独立运行环境、部署实施和技术交接。",
    items: ["独立环境与企业域名", "监控、备份与恢复", "集成与数据迁移", "交付培训与运维文档"],
    action: "咨询私有化方案",
    recommended: false,
  },
] as const

const comparisonRows = [
  { label: "适用阶段", values: ["首次验证", "持续运营", "独立运行"] },
  { label: "客户入口与 AI", values: [true, true, true] },
  { label: "工单与人工协同", values: ["单流程", "完整能力", "完整能力"] },
  { label: "产品与设备管理", values: [false, true, true] },
  { label: "供应商与伙伴协作", values: [false, true, true] },
  { label: "专属部署环境", values: [false, false, true] },
  { label: "实施与验收", values: [true, true, true] },
] as const

type ComparisonValue = (typeof comparisonRows)[number]["values"][number]

function ComparisonValue({ value }: { value: ComparisonValue }) {
  if (value === true) {
    return (
      <span className="inline-flex size-6 items-center justify-center rounded-full bg-emerald-50 text-emerald-700" aria-label="包含">
        <CheckIcon className="size-3.5" strokeWidth={2.5} />
      </span>
    )
  }

  if (value === false) {
    return (
      <span className="inline-flex size-6 items-center justify-center text-slate-300" aria-label="不包含">
        <MinusIcon className="size-4" />
      </span>
    )
  }

  return <span className="text-sm font-medium text-slate-700">{value}</span>
}

export function PricingPage() {
  return (
    <main className="min-h-screen bg-white text-slate-900">
      <MarketingSiteHeader />

      <section className="border-b border-slate-200 bg-[#f3f7f5]">
        <div className="mx-auto max-w-7xl px-5 pb-16 pt-12 sm:px-8 sm:pb-20 sm:pt-16 lg:px-10">
          <div className="mx-auto max-w-3xl text-center">
            <p className="text-sm font-semibold text-emerald-700">价格与交付</p>
            <h1 className="mt-4 text-4xl font-semibold leading-tight text-slate-950 sm:text-5xl">
              三种方案，匹配不同服务阶段
            </h1>
            <p className="mt-5 text-base leading-7 text-slate-600 sm:text-lg">
              从小范围验证，到团队规模化运营，再到企业独立部署。
            </p>
          </div>

          <div id="main-content" className="mt-10 grid items-stretch gap-5 lg:grid-cols-3">
            {plans.map((plan) => {
              const Icon = plan.icon
              return (
                <article
                  key={plan.name}
                  className={`relative flex min-h-[472px] flex-col overflow-hidden rounded-lg border bg-white p-6 sm:p-7 ${
                    plan.recommended
                      ? "border-emerald-300 shadow-[0_24px_64px_rgba(15,118,110,0.14)] lg:-translate-y-2"
                      : "border-slate-200 shadow-[0_16px_44px_rgba(15,23,42,0.06)]"
                  }`}
                >
                  <div className={`absolute inset-x-0 top-0 h-1 ${plan.recommended ? "bg-emerald-500" : "bg-slate-200"}`} />
                  <div className="flex items-center justify-between gap-4">
                    <span className="inline-flex items-center gap-2 text-xs font-semibold text-slate-500">
                      <Icon className={`size-5 ${plan.recommended ? "text-emerald-700" : "text-slate-600"}`} />
                      {plan.stage}
                    </span>
                    {plan.recommended ? (
                      <span className="rounded-full bg-emerald-50 px-3 py-1 text-xs font-semibold text-emerald-700">推荐</span>
                    ) : null}
                  </div>

                  <h2 className="mt-7 text-2xl font-semibold text-slate-950">{plan.name}</h2>
                  <p className="mt-3 min-h-[52px] text-sm leading-6 text-slate-600">{plan.description}</p>

                  <div className="mt-6 border-y border-slate-100 py-5">
                    <p className="text-xs text-slate-500">方案价格</p>
                    <p className="mt-1 text-xl font-semibold text-slate-950">{plan.price}</p>
                  </div>

                  <ul className="mt-6 grid gap-3">
                    {plan.items.map((item) => (
                      <li key={item} className="flex items-start gap-3 text-sm text-slate-700">
                        <span className="mt-0.5 inline-flex size-5 shrink-0 items-center justify-center rounded-full bg-emerald-50 text-emerald-700">
                          <CheckIcon className="size-3" strokeWidth={2.5} />
                        </span>
                        {item}
                      </li>
                    ))}
                  </ul>

                  <DemoRequestDialog
                    label={plan.action}
                    className={`mt-auto inline-flex h-11 w-full items-center justify-center rounded-md px-4 text-sm font-semibold transition ${
                      plan.recommended
                        ? "bg-slate-950 text-white hover:bg-slate-800"
                        : "border border-slate-300 text-slate-800 hover:border-slate-500 hover:bg-slate-50"
                    }`}
                  />
                </article>
              )
            })}
          </div>

          <p className="mt-6 text-center text-xs leading-5 text-slate-500">
            最终报价根据使用规模、启用能力、实施范围和持续支持确定。
          </p>
        </div>
      </section>

      <section className="py-16 sm:py-20">
        <div className="mx-auto max-w-6xl px-5 sm:px-8 lg:px-10">
          <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <p className="text-sm font-semibold text-emerald-700">方案对比</p>
              <h2 className="mt-3 text-3xl font-semibold text-slate-950">快速找到合适的起点</h2>
            </div>
            <p className="max-w-md text-sm leading-6 text-slate-600">
              不确定从哪里开始，可以先用场景试点验证一条完整链路，再平滑扩展。
            </p>
          </div>

          <div className="mt-8 overflow-x-auto rounded-lg border border-slate-200">
            <div className="min-w-[720px]">
              <div className="grid grid-cols-[1.2fr_repeat(3,1fr)] border-b border-slate-200 bg-slate-50 px-5 py-4 text-sm font-semibold text-slate-950">
                <span>能力范围</span>
                {plans.map((plan) => <span key={plan.name} className="text-center">{plan.name}</span>)}
              </div>
              {comparisonRows.map((row) => (
                <div key={row.label} className="grid min-h-14 grid-cols-[1.2fr_repeat(3,1fr)] items-center border-b border-slate-100 px-5 last:border-b-0">
                  <span className="text-sm text-slate-600">{row.label}</span>
                  {row.values.map((value, index) => (
                    <span key={`${row.label}-${plans[index].name}`} className="flex justify-center text-center">
                      <ComparisonValue value={value} />
                    </span>
                  ))}
                </div>
              ))}
            </div>
          </div>
        </div>
      </section>

      <section className="border-y border-slate-200 bg-[#f7f9f8] py-14 sm:py-16">
        <div className="mx-auto flex max-w-6xl flex-col gap-7 px-5 sm:px-8 md:flex-row md:items-center md:justify-between lg:px-10">
          <div>
            <h2 className="text-2xl font-semibold text-slate-950 sm:text-3xl">先说清场景，再给出可解释的报价</h2>
            <p className="mt-3 max-w-2xl text-sm leading-6 text-slate-600">
              告诉我们服务地区、团队规模和最想解决的问题，即可获得对应方案。
            </p>
          </div>
          <DemoRequestDialog
            label="获取方案与报价"
            className="inline-flex h-11 shrink-0 items-center justify-center gap-2 rounded-md bg-slate-950 px-5 text-sm font-semibold text-white transition hover:bg-slate-800"
          />
        </div>
      </section>

      <MarketingSiteFooter />
    </main>
  )
}
