"use client"

import { Loader2Icon } from "lucide-react"
import { usePathname, useRouter } from "next/navigation"
import type { CSSProperties, ReactNode } from "react"
import { useEffect } from "react"

import { AppSidebar } from "@/components/app-sidebar"
import { useAuth } from "@/components/auth-provider"
import { NotificationProvider } from "@/components/notification-provider"
import { SiteHeader } from "@/components/site-header"
import { useI18n } from "@/i18n/provider"
import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar"
import { Skeleton } from "@/components/ui/skeleton"

function DashboardAuthLoadingShell({ label, description }: { label: string; description: string }) {
  return (
    <div className="flex min-h-screen bg-background text-foreground" aria-busy="true" aria-live="polite">
      <aside className="hidden w-64 shrink-0 bg-muted/30 p-3 lg:block" aria-hidden="true">
        <Skeleton className="h-10 w-40 rounded-md" />
        <div className="mt-6 space-y-2">
          {Array.from({ length: 8 }, (_, index) => (
            <Skeleton key={index} className="h-9 w-full rounded-md" />
          ))}
        </div>
      </aside>
      <main className="min-w-0 flex-1">
        <header className="flex h-12 items-center gap-3 border-b border-border/70 px-4">
          <Skeleton className="h-6 w-44 max-w-[55vw]" />
          <div className="ml-auto flex items-center gap-2">
            <Skeleton className="hidden h-8 w-24 rounded-md sm:block" />
            <Skeleton className="size-8 rounded-full" />
          </div>
        </header>
        <div className="grid gap-4 p-4 lg:p-5">
          <div className="flex min-h-14 items-center justify-between gap-4 border-b border-border/70 pb-3">
            <div className="min-w-0">
              <div className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
                <Loader2Icon className="size-4 animate-spin" aria-hidden="true" />
                <span>{label}</span>
              </div>
              <p className="mt-1 text-xs text-muted-foreground">{description}</p>
            </div>
            <Skeleton className="hidden h-9 w-28 rounded-md sm:block" />
          </div>
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            {Array.from({ length: 4 }, (_, index) => (
              <Skeleton key={index} className="h-24 rounded-md" />
            ))}
          </div>
          <div className="grid gap-4 xl:grid-cols-[minmax(0,1.2fr)_minmax(280px,0.8fr)]">
            <Skeleton className="h-80 rounded-md" />
            <Skeleton className="h-80 rounded-md" />
          </div>
        </div>
      </main>
    </div>
  )
}

export default function DashboardLayout({
  children,
}: {
  children: ReactNode
}) {
  const t = useI18n()
  const { ready, session } = useAuth()
  const pathname = usePathname()
  const router = useRouter()
  const isLoginRoute = pathname?.startsWith("/dashboard/login") ?? false

  useEffect(() => {
    if (ready && !session && !isLoginRoute) {
      router.replace("/dashboard/login")
    }
  }, [isLoginRoute, ready, router, session])

  if (isLoginRoute) {
    return <>{children}</>
  }

  if (!ready || !session) {
    return (
      <DashboardAuthLoadingShell
        label={t("auth.checkingSession")}
        description={t("auth.syncingProfile")}
      />
    )
  }

  return (
    <SidebarProvider
      className="h-svh min-h-0 overflow-hidden"
      style={
        {
          "--sidebar-width": "calc(var(--spacing) * 54)",
          "--header-height": "calc(var(--spacing) * 12)",
        } as CSSProperties
      }
    >
      <NotificationProvider>
        <AppSidebar variant="inset" />
        <SidebarInset className="overflow-hidden bg-background">
          <SiteHeader />
          <div className="@container/main flex min-h-0 flex-1 flex-col overflow-auto">
            {children}
          </div>
        </SidebarInset>
      </NotificationProvider>
    </SidebarProvider>
  )
}
