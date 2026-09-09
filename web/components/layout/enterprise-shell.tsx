"use client"

import type { ReactNode } from "react"

import { DomainShell } from "@/components/layout/domain-shell"
import { EnterpriseReminderPoller } from "@/components/layout/enterprise-reminder-poller"
import { NotificationProvider } from "@/components/notification-provider"
import { getEnterpriseNavigationSections } from "@/lib/navigation-enterprise"

export function EnterpriseShell({ children }: { children: ReactNode }) {
  return (
    <NotificationProvider>
      <DomainShell domain="enterprise" sections={getEnterpriseNavigationSections()}>
        {children}
      </DomainShell>
      <EnterpriseReminderPoller />
    </NotificationProvider>
  )
}
