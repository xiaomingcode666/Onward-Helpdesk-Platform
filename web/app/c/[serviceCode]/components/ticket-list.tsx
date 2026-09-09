"use client"

import {
  ClipboardListIcon,
  ClockIcon,
  ExternalLinkIcon,
} from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { useAppLocale, useI18n } from "@/i18n/provider"
import type { CustomerEntryTicket } from "@/lib/api/customer-entry"

type TicketListProps = {
  tickets?: CustomerEntryTicket[]
  onViewTicket?: (ticket: CustomerEntryTicket) => void
}

function getStatusBadge(s: string) {
  const map: Record<string, "default" | "secondary" | "outline"> = {
    pending: "secondary",
    pending_acceptance: "secondary",
    pending_dispatch: "secondary",
    pending_assignee_accept: "secondary",
    in_progress: "default",
    processing: "default",
    resolved: "secondary",
    pending_customer_confirm: "secondary",
    closed: "outline",
    cancelled: "outline",
    done: "outline",
  }
  return map[s] || "outline"
}

function getStatusLabel(status: string, t: ReturnType<typeof useI18n>) {
  const labels: Record<string, string> = {
    pending: t("portalExtract.serviceCode.ticketList.status.pending"),
    pending_acceptance: t("portalExtract.serviceCode.ticketList.status.pendingAcceptance"),
    pending_dispatch: t("portalExtract.serviceCode.ticketList.status.pendingDispatch"),
    pending_assignee_accept: t("portalExtract.serviceCode.ticketList.status.pendingAssigneeAccept"),
    accepted: t("portalExtract.serviceCode.ticketList.status.accepted"),
    in_progress: t("portalExtract.serviceCode.ticketList.status.processing"),
    processing: t("portalExtract.serviceCode.ticketList.status.processing"),
    resolved: t("portalExtract.serviceCode.ticketList.status.pendingCustomerConfirm"),
    pending_customer_confirm: t("portalExtract.serviceCode.ticketList.status.pendingCustomerConfirm"),
    closed: t("portalExtract.serviceCode.ticketList.status.closed"),
    cancelled: t("portalExtract.serviceCode.ticketList.status.cancelled"),
    done: t("portalExtract.serviceCode.ticketList.status.done"),
  }
  return labels[status] ?? status.replaceAll("_", " ")
}

export function TicketList({ tickets = [], onViewTicket }: TicketListProps) {
  const { locale, t } = useAppLocale()

  return (
    <div className="space-y-3 p-4">
      {tickets.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center py-8">
            <ClipboardListIcon className="size-10 text-muted-foreground" />
            <p className="mt-2 text-sm text-muted-foreground">
              {t("portalExtract.serviceCode.ticketList.empty")}
            </p>
          </CardContent>
        </Card>
      ) : (
        tickets.map((ticket) => (
          <Card key={ticket.id} className="relative">
            <CardHeader className="pb-2">
              <div className="flex items-start justify-between gap-2">
                <CardTitle className="truncate text-sm font-medium">
                  {ticket.title}
                </CardTitle>
                <Badge variant={getStatusBadge(ticket.status)} className="shrink-0 text-rhd-2xs">
                  {getStatusLabel(ticket.status, t)}
                </Badge>
              </div>
              <p className="font-mono text-xs text-muted-foreground">
                {ticket.ticketNo}
              </p>
            </CardHeader>
            <CardContent className="pb-3">
              <div className="flex items-center gap-3 text-xs text-muted-foreground">
                <span className="flex items-center gap-1">
                  <ClockIcon className="size-3" />
                  {new Date(ticket.createdAt).toLocaleDateString(locale)}
                </span>
                <span>{ticket.deviceNo}</span>
              </div>
            </CardContent>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="absolute right-2 top-2"
              onClick={() => onViewTicket?.(ticket)}
              aria-label={t("portalExtract.serviceCode.ticketList.viewTicket")}
              title={t("portalExtract.serviceCode.ticketList.viewTicket")}
            >
              <ExternalLinkIcon className="size-4" />
            </Button>
          </Card>
        ))
      )}
    </div>
  )
}
