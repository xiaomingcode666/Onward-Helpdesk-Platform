import { Suspense } from "react"

import { PartnerTicketDetailAliasPage } from "@/components/partner/partner-portal-pages"
import { RouteLoadingPage } from "@/components/shared/loading-states"

export default function PartnerTicketDetailRoute() {
  return (
    <Suspense fallback={<RouteLoadingPage />}>
      <PartnerTicketDetailAliasPage />
    </Suspense>
  )
}
