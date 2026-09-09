import type { ReactNode } from "react"

import { PartnerShell } from "@/components/layout/partner-shell"

export default function PartnerLayout({ children }: { children: ReactNode }) {
  return <PartnerShell>{children}</PartnerShell>
}
