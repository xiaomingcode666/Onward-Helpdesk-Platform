import type { ReactNode } from "react"

import { EnterpriseShell } from "@/components/layout/enterprise-shell"

export default function EnterpriseLayout({ children }: { children: ReactNode }) {
  return <EnterpriseShell>{children}</EnterpriseShell>
}
