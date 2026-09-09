import EnterpriseMeetingRoomPage from "./room-client-page"
import { notFound } from "next/navigation"
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
  return <EnterpriseMeetingRoomPage initialMeetingId={meetingId} />
}
