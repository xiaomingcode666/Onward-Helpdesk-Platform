import type { CustomerEntryContext } from "@/lib/api/customer-entry"

const TERMINAL_TICKET_STATUSES = new Set(["cancelled", "closed"])

export function selectCustomerRealtimeConversationId(
  context: CustomerEntryContext | null | undefined,
) {
  if (!context) return 0

  const activeMeeting = context.meetings.find((meeting) => meeting.status === "active")
  if (activeMeeting) {
    const meetingTicket = context.tickets.find(
      (ticket) => ticket.id === activeMeeting.ticketId,
    )
    const meetingConversationId = meetingTicket?.conversationId ?? 0
    if (meetingConversationId > 0) {
      return meetingConversationId
    }
  }

  const activeConversation = context.conversations.find(
    (conversation) => !conversation.endedAt,
  )
  const activeConversationId = activeConversation?.id ?? 0
  if (activeConversationId > 0) {
    return activeConversationId
  }

  const activeTicket = context.tickets.find(
    (ticket) =>
      ticket.conversationId > 0 &&
      !TERMINAL_TICKET_STATUSES.has(ticket.status.toLowerCase()),
  )
  if (activeTicket) {
    return activeTicket.conversationId
  }

  return context.conversations[0]?.id ?? context.tickets[0]?.conversationId ?? 0
}
