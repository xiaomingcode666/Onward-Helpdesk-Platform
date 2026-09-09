"use client"

import { useSearchParams } from "next/navigation"

import { RouteAlias } from "@/components/layout/route-alias"
import { useI18n } from "@/i18n/provider"
import { normalizeEnterpriseActionUrl } from "@/lib/ticket-workbench-route"

export default function EnterpriseTicketDetailRedirectClient({ ticketId }: { ticketId?: string }) {
  const searchParams = useSearchParams()
  const t = useI18n()
  const target = normalizeEnterpriseActionUrl({
    action_url: ticketId ? `/enterprise/tickets/${ticketId}` : "",
    biz_type: "ticket",
    biz_id: searchParams.get("biz_id") ?? searchParams.get("bizId") ?? searchParams.get("ticket_id") ?? searchParams.get("ticketId"),
  })

  return (
    <RouteAlias
      to={target}
      title={t("enterprise.ticketRedirect.opening")}
    />
  )
}
