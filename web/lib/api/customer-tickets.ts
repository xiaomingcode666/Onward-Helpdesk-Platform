"use client"

import { buildCustomerPortalQueryString } from "@/lib/api/customer-portal-query"
import { customerPortalRequest } from "@/lib/api/customer-portal-client"
import type {
  CustomerPortalListQuery,
  CustomerPortalPage,
  CustomerPortalTicket,
} from "@/lib/api/customer-portal-types"

export function fetchCustomerTickets() {
  return customerPortalRequest<CustomerPortalTicket[]>("/tickets")
}

export function fetchCustomerTicketsPage(query?: CustomerPortalListQuery) {
  return customerPortalRequest<CustomerPortalPage<CustomerPortalTicket>>(`/tickets/page${buildCustomerPortalQueryString(query)}`)
}

export function fetchCustomerTicketDetail(ticketId: number) {
	return customerPortalRequest<CustomerPortalTicket>(`/tickets/${ticketId}`)
}

export function submitCustomerTicketFeedback(
	ticketId: number,
	payload: { rating: number; tags?: string[]; comment?: string }
) {
	return customerPortalRequest<CustomerPortalTicket["feedback"]>(`/tickets/${ticketId}/feedback`, {
		method: "POST",
		body: JSON.stringify(payload),
	})
}

export function confirmCustomerTicket(ticketId: number) {
	return customerPortalRequest<CustomerPortalTicket>(`/tickets/${ticketId}/_confirm`, {
		method: "POST",
		body: "{}",
	})
}

export function reopenCustomerTicket(ticketId: number, reason: string) {
	return customerPortalRequest<CustomerPortalTicket>(`/tickets/${ticketId}/_reopen`, {
		method: "POST",
		body: JSON.stringify({ reason }),
	})
}
