export type WorkbenchRouteCandidate = {
  key: string
  conversationId?: number
  ticketId?: number
}

export type WorkbenchTicketCandidate = {
  id: number
  conversation_id?: number
}

export function normalizeWorkbenchId(value: unknown): number {
  const id = Number(value)
  return Number.isSafeInteger(id) && id > 0 ? id : 0
}

export function buildEnterpriseTicketWorkbenchPath(ticketId: unknown): string {
  const normalizedTicketId = normalizeWorkbenchId(ticketId)
  return normalizedTicketId > 0
    ? `/enterprise/ticket-workbench?ticket_id=${encodeURIComponent(String(normalizedTicketId))}`
    : "/enterprise/ticket-workbench"
}

export function buildEnterpriseConversationWorkbenchPath(conversationId: unknown): string {
  const normalizedConversationId = normalizeWorkbenchId(conversationId)
  return normalizedConversationId > 0
    ? `/enterprise/ticket-workbench?conversationId=${encodeURIComponent(String(normalizedConversationId))}`
    : "/enterprise/ticket-workbench"
}

export type EnterpriseActionRouteInput = {
  actionUrl?: string
  action_url?: string
  notificationType?: string
  notification_type?: string
  bizType?: string
  biz_type?: string
  bizId?: unknown
  biz_id?: unknown
}

export type EnterpriseTicketWorkbenchRouteInput = EnterpriseActionRouteInput & {
  id?: unknown
  ticketId?: unknown
  ticket_id?: unknown
}

export function normalizeEnterpriseActionUrl(input: EnterpriseActionRouteInput | string | null | undefined): string {
  const actionUrl = typeof input === "string" ? input : input?.actionUrl ?? input?.action_url ?? ""
  const bizType = typeof input === "string" || !input
    ? ""
    : (input.bizType ?? input.biz_type ?? "").trim().toLowerCase()
  const notificationType = typeof input === "string" || !input
    ? ""
    : (input.notificationType ?? input.notification_type ?? "").trim().toLowerCase()
  const bizId = typeof input === "string" || !input ? 0 : input.bizId ?? input.biz_id
  const normalizedBizId = normalizeWorkbenchId(bizId)
  if ((bizType === "ticket" || notificationType === "ticket_assigned") && normalizedBizId > 0) {
    return buildEnterpriseTicketWorkbenchPath(normalizedBizId)
  }
  const normalizedLegacyTicketUrl = normalizeLegacyEnterpriseTicketUrl(actionUrl)
  if (normalizedLegacyTicketUrl) {
    if (normalizedLegacyTicketUrl === "/enterprise/ticket-workbench" && bizType === "ticket") {
      return buildEnterpriseTicketWorkbenchPath(bizId)
    }
    return normalizedLegacyTicketUrl
  }

  const trimmedActionUrl = actionUrl.trim()
  if (trimmedActionUrl) {
    return trimmedActionUrl
  }

  if (typeof input === "string" || !input) {
    return ""
  }
  if (bizType === "ticket") {
    return buildEnterpriseTicketWorkbenchPath(bizId)
  }
  if (bizType === "conversation") {
    return buildEnterpriseConversationWorkbenchPath(bizId)
  }
  return ""
}

export function buildEnterpriseTicketWorkbenchPathFromItem(
  input: EnterpriseTicketWorkbenchRouteInput | null | undefined,
): string {
  if (!input) {
    return "/enterprise/ticket-workbench"
  }

  const ticketId = firstWorkbenchId(input.id, input.ticketId, input.ticket_id, input.bizId, input.biz_id)
  const actionUrl = normalizeEnterpriseActionUrl({
    ...input,
    biz_type: input.bizType ?? input.biz_type ?? "ticket",
    biz_id: input.bizId ?? input.biz_id ?? ticketId,
  })
  if (actionUrl === "/enterprise/ticket-workbench" || actionUrl.startsWith("/enterprise/ticket-workbench?")) {
    return actionUrl
  }
  return buildEnterpriseTicketWorkbenchPath(ticketId)
}

function normalizeLegacyEnterpriseTicketUrl(actionUrl: string): string {
  const trimmed = actionUrl.trim()
  if (!trimmed) {
    return ""
  }
  let parsed: URL
  try {
    parsed = new URL(trimmed, "https://remotehelpdesk.local")
  } catch {
    return ""
  }
  const match = parsed.pathname.match(/^\/enterprise\/tickets\/([^/?#]+)\/?$/)
  if (!match) {
    return ""
  }
  return buildEnterpriseTicketWorkbenchPath(match[1])
}

function firstWorkbenchId(...values: unknown[]): number {
  for (const value of values) {
    const id = normalizeWorkbenchId(value)
    if (id > 0) {
      return id
    }
  }
  return 0
}

export function findWorkbenchRouteItem<T extends WorkbenchRouteCandidate>(
  items: T[],
  route: { conversationId?: number; ticketId?: number },
): T | null {
  if (route.ticketId && route.ticketId > 0) {
    return items.find((item) => item.ticketId === route.ticketId) ?? null
  }
  if (route.conversationId && route.conversationId > 0) {
    return items.find((item) => item.conversationId === route.conversationId) ?? null
  }
  return null
}

export function indexWorkbenchTicketsByConversation<T extends WorkbenchTicketCandidate>(
  items: T[],
  preferredTicketId: unknown,
): Map<number, T> {
  const preferredId = normalizeWorkbenchId(preferredTicketId)
  const result = new Map<number, T>()
  for (const item of items) {
    const conversationId = normalizeWorkbenchId(item.conversation_id)
    if (conversationId <= 0) continue
    const current = result.get(conversationId)
    if (!current || (item.id === preferredId && current.id !== preferredId)) {
      result.set(conversationId, item)
    }
  }
  return result
}
