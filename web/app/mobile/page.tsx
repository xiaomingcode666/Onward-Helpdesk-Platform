import { Suspense } from "react"

import { translateCurrentMessage } from "@/i18n/messages"
import { MobileEntryPage } from "./mobile-entry-page"

function MobileEntryFallback() {
  return (
    <main className="flex min-h-svh items-center justify-center bg-[var(--railops-layout-background)] px-6 text-center text-[var(--railops-text)]">
      <p className="text-xs text-[var(--railops-text-secondary)]">{translateCurrentMessage("portalExtract.mobileEntry.opening")}</p>
    </main>
  )
}

export default function MobilePage() {
  return (
    <Suspense fallback={<MobileEntryFallback />}>
      <MobileEntryPage />
    </Suspense>
  )
}
