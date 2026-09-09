import { Suspense } from "react"

import { RouteLoadingPage } from "@/components/shared/loading-states"
import EnterpriseMeetingRoomPage from "../meetings/[meetingId]/room/room-client-page"

export default function Page() {
  return (
    <Suspense fallback={<RouteLoadingPage />}>
      <EnterpriseMeetingRoomPage />
    </Suspense>
  )
}
