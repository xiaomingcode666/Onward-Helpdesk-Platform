"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { MenuIcon, XIcon } from "lucide-react"
import { useState } from "react"

import { AppLogoMark } from "@/components/brand-logo"
import { DemoRequestDialog } from "@/components/marketing/demo-request-dialog"
import { cn } from "@/lib/utils"

const navigation = [
  { href: "/", label: "首页" },
  { href: "/solutions", label: "解决方案" },
  { href: "/pricing", label: "价格" },
  { href: "/cases", label: "客户案例" },
  { href: "/about", label: "关于我们" },
]

type MarketingSiteHeaderProps = {
  tone?: "dark" | "light"
}

export function MarketingSiteHeader({ tone = "light" }: MarketingSiteHeaderProps) {
  const pathname = usePathname()
  const [mobileOpen, setMobileOpen] = useState(false)
  const dark = tone === "dark"

  return (
    <header className={cn("sticky top-0 z-50 border-b", dark ? "border-white/15 bg-[#07131f]/90 text-white backdrop-blur-xl" : "border-slate-200 bg-white/95 text-slate-950 shadow-[0_1px_0_rgba(15,23,42,0.03)] backdrop-blur-xl")}>
      <div className="mx-auto flex h-[72px] w-full max-w-7xl items-center justify-between px-5 sm:px-8 lg:px-10">
        <Link href="/" className="flex items-center gap-3" aria-label="RemoteHelpDesk 首页">
          <AppLogoMark alt="RemoteHelpDesk" className="size-9 border-0 bg-white" imageClassName="p-0.5" priority />
          <span className="grid leading-tight">
            <strong className="text-base font-semibold">RemoteHelpDesk</strong>
            <small className={cn("text-xs", dark ? "text-slate-300" : "text-slate-500")}>工业设备智能售后平台</small>
          </span>
        </Link>

        <nav className="hidden items-center gap-8 text-sm lg:flex" aria-label="主导航">
          {navigation.map((item) => {
            const active = pathname === item.href
            return (
              <Link
                key={item.href}
                href={item.href}
                className={cn(
                  "relative py-2 font-medium transition after:absolute after:inset-x-0 after:-bottom-1 after:h-0.5 after:origin-center after:scale-x-0 after:bg-emerald-500 after:transition",
                  dark ? "text-slate-200 hover:text-white" : "text-slate-600 hover:text-slate-950",
                  active && "text-current after:scale-x-100"
                )}
              >
                {item.label}
              </Link>
            )
          })}
        </nav>

        <div className="hidden items-center gap-3 sm:flex">
          <Link href="/dashboard/login" className={cn("px-3 py-2 text-sm font-medium transition", dark ? "text-slate-200 hover:text-white" : "text-slate-700 hover:text-slate-950")}>登录</Link>
          <DemoRequestDialog label="免费试用" className="inline-flex h-10 items-center rounded-md bg-blue-600 px-4 text-sm font-semibold text-white transition hover:bg-blue-700" />
        </div>

        <button
          type="button"
          className={cn("grid size-10 place-items-center rounded-md border sm:hidden", dark ? "border-white/25 text-white" : "border-slate-200 text-slate-800")}
          aria-label={mobileOpen ? "关闭导航" : "打开导航"}
          title={mobileOpen ? "关闭导航" : "打开导航"}
          onClick={() => setMobileOpen((current) => !current)}
        >
          {mobileOpen ? <XIcon className="size-5" /> : <MenuIcon className="size-5" />}
        </button>
      </div>

      {mobileOpen ? (
        <div className={cn("absolute inset-x-0 top-full border-b px-5 py-5 shadow-xl sm:hidden", dark ? "border-white/10 bg-slate-950" : "border-slate-200 bg-white")}>
          <nav className="grid gap-1" aria-label="移动导航">
            {navigation.map((item) => <Link key={item.href} href={item.href} onClick={() => setMobileOpen(false)} className={cn("rounded-md px-3 py-3 text-sm font-medium", pathname === item.href ? "bg-emerald-500/15 text-emerald-600" : dark ? "text-slate-200" : "text-slate-700")}>{item.label}</Link>)}
          </nav>
          <div className="mt-4 grid grid-cols-2 gap-3 border-t border-current/10 pt-4">
            <Link href="/dashboard/login" className={cn("inline-flex h-10 items-center justify-center rounded-md border text-sm font-medium", dark ? "border-white/25 text-white" : "border-slate-300 text-slate-800")}>登录</Link>
            <DemoRequestDialog label="免费试用" className="inline-flex h-10 items-center justify-center rounded-md bg-blue-600 px-4 text-sm font-semibold text-white transition hover:bg-blue-700" />
          </div>
        </div>
      ) : null}
    </header>
  )
}
