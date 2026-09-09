type TranslateFn = (key: string, values?: Record<string, string | number>) => string

export function renderMobileTicketTitle(title: string | undefined, t: TranslateFn, fallback = "") {
  const trimmed = (title ?? "").trim()
  if (!trimmed) {
    return fallback
  }
  switch (trimmed) {
    case "会话工单":
    case "Conversation ticket":
      return t("portalExtract.customerMobile.ticket.defaultConversationTitle")
    default:
      return title ?? fallback
  }
}
