"use client"

import { usePathname } from "next/navigation"
import type { ReactNode } from "react"

import { DomainShell } from "@/components/layout/domain-shell"
import { NotificationProvider } from "@/components/notification-provider"
import { getPlatformNavigationSections } from "@/lib/navigation-platform"

export function PlatformShell({ children }: { children: ReactNode }) {
  const pathname = usePathname()
  const isLoginRoute = pathname?.startsWith("/platform/login") ?? false

  if (isLoginRoute) {
    return <>{children}</>
  }

  return (
    <NotificationProvider>
      <DomainShell domain="platform" sections={getPlatformNavigationSections()}>
        {children}
      </DomainShell>
    </NotificationProvider>
  )
}
