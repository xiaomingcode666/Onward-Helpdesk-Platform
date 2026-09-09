"use client"

import type { ReactNode } from "react"

import { DomainShell } from "@/components/layout/domain-shell"
import { getPartnerNavigationSections } from "@/lib/navigation-partner"

export function PartnerShell({ children }: { children: ReactNode }) {
  return (
    <DomainShell
      domain="partner"
      sections={getPartnerNavigationSections()}
      showDomainTabs={false}
    >
      {children}
    </DomainShell>
  )
}
