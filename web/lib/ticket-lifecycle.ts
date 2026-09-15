const terminalTicketStatuses = new Set(["closed", "done", "cancelled"])
const processingTicketStatuses = new Set([
  "in_progress",
  "processing",
  "video_support",
  "supplier_support",
])

export function isTerminalTicketStatus(status: string) {
  return terminalTicketStatuses.has(status)
}

export function isProcessingTicketStatus(status: string) {
  return processingTicketStatuses.has(status)
}

export const caseStatuses = ["new", "acknowledged", "in_triage", "assigned", "waiting", "restored", "resolved", "closure_pending", "closed", "cancelled"] as const

export function isCaseStatus(status: string) {
  return (caseStatuses as readonly string[]).includes(status)
}

/** Legacy technical statuses remain available for dispatch actions. Display the recorded case phase first. */
export function displayTicketStatus(ticket: { status: string; case_status?: string }) {
  return ticket.case_status && isCaseStatus(ticket.case_status) ? ticket.case_status : ticket.status
}

