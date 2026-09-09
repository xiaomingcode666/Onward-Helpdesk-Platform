import { notFound } from "next/navigation"

import { PartnerMeetingRoomPage } from "@/components/partner/partner-portal-pages"
import { isStaticExportRouteParam, STATIC_EXPORT_PARAM } from "@/lib/static-export-route"

export function generateStaticParams() {
  return [{ meetingId: STATIC_EXPORT_PARAM }]
}

type PageProps = {
  params: Promise<{ meetingId: string }>
}

export default async function Page({ params }: PageProps) {
  const { meetingId } = await params
  if (isStaticExportRouteParam(meetingId)) {
    notFound()
  }
  return <PartnerMeetingRoomPage />
}
