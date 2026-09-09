import EnterpriseTicketDetailRedirectClient from "@/app/enterprise/tickets/[ticketId]/redirect-client"
import { isStaticExportRouteParam, STATIC_EXPORT_PARAM } from "@/lib/static-export-route"
import { notFound } from "next/navigation"

export function generateStaticParams() {
  return [{ ticketId: STATIC_EXPORT_PARAM }]
}

export default async function EnterpriseTicketDetailRedirectPage({
  params,
}: {
  params: Promise<{ ticketId?: string }>
}) {
  const { ticketId } = await params
  if (isStaticExportRouteParam(ticketId)) {
    notFound()
  }
  return <EnterpriseTicketDetailRedirectClient ticketId={ticketId} />
}
