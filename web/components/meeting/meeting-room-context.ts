import type { MeetingJoinConfig } from "@/lib/api/types"
import { translateCurrentMessage } from "@/i18n/messages"

type MeetingContext = Pick<MeetingJoinConfig, "subject" | "ticketId" | "ticket_id" | "ticketNo" | "ticket_no">

export function getMeetingTicketId(context?: MeetingContext | null, fallback?: string | number | null): number {
  const value = context?.ticketId ?? context?.ticket_id ?? fallback
  const ticketId = Number(value)
  return Number.isSafeInteger(ticketId) && ticketId > 0 ? ticketId : 0
}

export function getMeetingReturnPath(context?: MeetingContext | null, fallback?: string | number | null): string {
  const ticketId = getMeetingTicketId(context, fallback)
  return ticketId > 0
    ? `/enterprise/ticket-workbench?ticket_id=${encodeURIComponent(String(ticketId))}`
    : "/enterprise/video"
}

export function getMeetingDisplaySubject(
  context: MeetingContext | null | undefined,
  roomName: string,
): string {
  const subject = context?.subject?.trim()
  if (subject) return subject
  const ticketNo = (context?.ticketNo ?? context?.ticket_no)?.trim()
  return ticketNo ? `${ticketNo} · ${translateCurrentMessage("enterpriseExtract.meetingCenter.text054")}` : roomName
}

export function buildEnterpriseMeetingRoomPath(meetingId: string | number, ticketId?: string | number | null): string {
  const params = [`meeting_id=${encodeURIComponent(String(meetingId))}`]
  const normalizedTicketId = getMeetingTicketId(null, ticketId)
  if (normalizedTicketId > 0) {
    params.push(`ticket_id=${normalizedTicketId}`)
  }
  return `/enterprise/meeting-room?${params.join("&")}`
}
