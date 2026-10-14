"use client"

import { tm } from "@/components/marketing/marketing-i18n"
import { useI18n } from "@/i18n/provider"
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

const plans = () => [
  {
    icon: Settings2Icon,
    stage: tm("marketing.pricingPage.t011"),
    name: tm("marketing.pricingPage.t012"),
    price: tm("marketing.pricingPage.t013"),
    description: tm("marketing.pricingPage.t014"),
    items: [tm("marketing.pricingPage.t015"), tm("marketing.pricingPage.t016"), tm("marketing.pricingPage.t017"), tm("marketing.pricingPage.t018")],
    action: tm("marketing.pricingPage.t019"),
    recommended: false,
  },
  {
    icon: CloudCogIcon,
    stage: tm("marketing.pricingPage.t020"),
    name: tm("marketing.pricingPage.t021"),
    price: tm("marketing.pricingPage.t022"),
    description: tm("marketing.pricingPage.t023"),
    items: [tm("marketing.pricingPage.t024"), tm("marketing.pricingPage.t025"), tm("marketing.pricingPage.t026"), tm("marketing.pricingPage.t027")],
    action: tm("marketing.pricingPage.t028"),
    recommended: true,
  },
  {
    icon: ServerCogIcon,
    stage: tm("marketing.pricingPage.t029"),
    name: tm("marketing.pricingPage.t030"),
    price: tm("marketing.pricingPage.t031"),
    description: tm("marketing.pricingPage.t032"),
    items: [tm("marketing.pricingPage.t033"), tm("marketing.pricingPage.t034"), tm("marketing.pricingPage.t035"), tm("marketing.pricingPage.t036")],
    action: tm("marketing.pricingPage.t037"),
    recommended: false,
  },
] as const

const comparisonRows = () => [
  { label: tm("marketing.pricingPage.t038"), values: [tm("marketing.pricingPage.t039"), tm("marketing.pricingPage.t040"), tm("marketing.pricingPage.t041")] },
  { label: tm("marketing.pricingPage.t042"), values: [true, true, true] },
  { label: tm("marketing.pricingPage.t043"), values: [tm("marketing.pricingPage.t044"), tm("marketing.pricingPage.t045"), tm("marketing.pricingPage.t045")] },
  { label: tm("marketing.pricingPage.t046"), values: [false, true, true] },
  { label: tm("marketing.pricingPage.t047"), values: [false, true, true] },
  { label: tm("marketing.pricingPage.t048"), values: [false, false, true] },
  { label: tm("marketing.pricingPage.t049"), values: [true, true, true] },
] as const

type ComparisonValue = ReturnType<typeof comparisonRows>[number]["values"][number]

function ComparisonValue({ value }: { value: ComparisonValue }) {
  if (value === true) {
    return (
      <span className="inline-flex size-6 items-center justify-center rounded-full bg-emerald-50 text-emerald-700" aria-label={tm("marketing.pricingPage.t001")}>
        <CheckIcon className="size-3.5" strokeWidth={2.5} />
      </span>
    )
  }

  if (value === false) {
    return (
      <span className="inline-flex size-6 items-center justify-center text-slate-300" aria-label={tm("marketing.pricingPage.t002")}>
        <MinusIcon className="size-4" />
      </span>
    )
  }

  return <span className="text-sm font-medium text-slate-700">{value}</span>
}

export function PricingPage() {
  useI18n()
  return (
    <main className="min-h-screen bg-white text-slate-900">
      <MarketingSiteHeader />

      <section className="border-b border-slate-200 bg-[#f3f7f5]">
        <div className="mx-auto max-w-7xl px-5 pb-16 pt-12 sm:px-8 sm:pb-20 sm:pt-16 lg:px-10">
          <div className="mx-auto max-w-3xl text-center">
            <p className="text-sm font-semibold text-emerald-700">{tm("marketing.pricingPage.t004")}</p>
            <h1 className="mt-4 text-4xl font-semibold leading-tight text-slate-950 sm:text-5xl">
              {tm("marketing.pricingPage.t050")}
            </h1>
            <p className="mt-5 text-base leading-7 text-slate-600 sm:text-lg">
              {tm("marketing.pricingPage.t051")}
            </p>
          </div>

          <div id="main-content" className="mt-10 grid items-stretch gap-5 lg:grid-cols-3">
            {plans().map((plan) => {
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
                      <span className="rounded-full bg-emerald-50 px-3 py-1 text-xs font-semibold text-emerald-700">{tm("marketing.pricingPage.t005")}</span>
                    ) : null}
                  </div>

                  <h2 className="mt-7 text-2xl font-semibold text-slate-950">{plan.name}</h2>
                  <p className="mt-3 min-h-[52px] text-sm leading-6 text-slate-600">{plan.description}</p>

                  <div className="mt-6 border-y border-slate-100 py-5">
                    <p className="text-xs text-slate-500">{tm("marketing.pricingPage.t006")}</p>
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
            {tm("marketing.pricingPage.t052")}
          </p>
        </div>
      </section>

      <section className="py-16 sm:py-20">
        <div className="mx-auto max-w-6xl px-5 sm:px-8 lg:px-10">
          <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <p className="text-sm font-semibold text-emerald-700">{tm("marketing.pricingPage.t007")}</p>
              <h2 className="mt-3 text-3xl font-semibold text-slate-950">{tm("marketing.pricingPage.t008")}</h2>
            </div>
            <p className="max-w-md text-sm leading-6 text-slate-600">
              {tm("marketing.pricingPage.t053")}
            </p>
          </div>

          <div className="mt-8 overflow-x-auto rounded-lg border border-slate-200">
            <div className="min-w-[720px]">
              <div className="grid grid-cols-[1.2fr_repeat(3,1fr)] border-b border-slate-200 bg-slate-50 px-5 py-4 text-sm font-semibold text-slate-950">
                <span>{tm("marketing.pricingPage.t009")}</span>
                {plans().map((plan) => <span key={plan.name} className="text-center">{plan.name}</span>)}
              </div>
              {comparisonRows().map((row) => (
                <div key={row.label} className="grid min-h-14 grid-cols-[1.2fr_repeat(3,1fr)] items-center border-b border-slate-100 px-5 last:border-b-0">
                  <span className="text-sm text-slate-600">{row.label}</span>
                  {row.values.map((value, index) => (
                    <span key={`${row.label}-${plans()[index].name}`} className="flex justify-center text-center">
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
            <h2 className="text-2xl font-semibold text-slate-950 sm:text-3xl">{tm("marketing.pricingPage.t010")}</h2>
            <p className="mt-3 max-w-2xl text-sm leading-6 text-slate-600">
              {tm("marketing.pricingPage.t054")}
            </p>
          </div>
          <DemoRequestDialog
            label={tm("marketing.pricingPage.t003")}
            className="inline-flex h-11 shrink-0 items-center justify-center gap-2 rounded-md bg-slate-950 px-5 text-sm font-semibold text-white transition hover:bg-slate-800"
          />
        </div>
      </section>

      <MarketingSiteFooter />
    </main>
  )
}
