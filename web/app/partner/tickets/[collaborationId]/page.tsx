import { PartnerTicketDetailPage } from "@/components/partner/partner-portal-pages"
import { isStaticExportRouteParam, STATIC_EXPORT_PARAM } from "@/lib/static-export-route"
import { notFound } from "next/navigation"

export function generateStaticParams() {
  return [{ collaborationId: STATIC_EXPORT_PARAM }]
}

export default async function PartnerTicketDetailRoute({
  params,
}: {
  params: Promise<{ collaborationId?: string }>
}) {
  const { collaborationId } = await params
  if (isStaticExportRouteParam(collaborationId)) {
    notFound()
  }
  return <PartnerTicketDetailPage />
}
