export type WorkbenchQueueOrderCandidate = {
  key: string
  isDone?: boolean
  ticketId?: number
  updatedAt?: string
  ticket?: {
    id?: number
    created_at?: string
    updated_at?: string
  } | null
}

function timestamp(value?: string | null) {
  if (!value) return null
  const parsed = Date.parse(value)
  return Number.isFinite(parsed) ? parsed : null
}

function queueUpdatedAt(item: WorkbenchQueueOrderCandidate) {
  return timestamp(item.updatedAt) ?? timestamp(item.ticket?.updated_at) ?? timestamp(item.ticket?.created_at)
}

function queueKey(item: WorkbenchQueueOrderCandidate) {
  return String(item.key || "")
}

function ticketID(item: WorkbenchQueueOrderCandidate) {
  return item.ticket?.id || item.ticketId || 0
}

export function compareWorkbenchQueueItems(
  left: WorkbenchQueueOrderCandidate,
  right: WorkbenchQueueOrderCandidate,
) {
  if (Boolean(left.isDone) !== Boolean(right.isDone)) {
    return left.isDone ? 1 : -1
  }

  const leftUpdatedAt = queueUpdatedAt(left) ?? 0
  const rightUpdatedAt = queueUpdatedAt(right) ?? 0
  if (leftUpdatedAt !== rightUpdatedAt) {
    return rightUpdatedAt - leftUpdatedAt
  }
  const leftHasTicket = ticketID(left) > 0
  const rightHasTicket = ticketID(right) > 0
  if (leftHasTicket !== rightHasTicket) {
    return leftHasTicket ? -1 : 1
  }
  if (ticketID(left) !== ticketID(right)) {
    return ticketID(right) - ticketID(left)
  }
  return queueKey(left).localeCompare(queueKey(right))
}

export function sortWorkbenchQueueItems<T extends WorkbenchQueueOrderCandidate>(items: T[]) {
  return [...items].sort(compareWorkbenchQueueItems)
}
