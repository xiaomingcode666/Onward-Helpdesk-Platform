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

