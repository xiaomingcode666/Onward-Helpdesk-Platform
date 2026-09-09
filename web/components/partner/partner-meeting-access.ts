import { translateCurrentMessage } from "@/i18n/messages"

export type PartnerMeetingAccessCollaboration = {
  id: number
  ticket_id: number
  status: string
  authorization_active: boolean
  visibility?: string[]
}

export type PartnerMeetingAccessMeeting = {
  id: string
  collaboration_id?: number
  ticket_id: number
  status: string
}

export type PartnerMeetingAccess = {
  meeting: PartnerMeetingAccessMeeting | null
  state: "hidden" | "loading" | "unavailable" | "none" | "ended" | "cancelled" | "joinable"
  message: string
}

function pt(key: string) {
  return translateCurrentMessage(`partnerExtract.meetingAccess.${key}`)
}

const JOINABLE_MEETING_STATUS_PRIORITY: Record<string, number> = {
  active: 0,
  waiting: 1,
  scheduled: 2,
}

const ENDED_MEETING_STATUSES = new Set(["ended", "finished"])
const CANCELLED_MEETING_STATUSES = new Set(["cancelled", "canceled"])

export function resolvePartnerMeetingAccess({
  collaboration,
  loadError = "",
  meetings,
}: {
  collaboration: PartnerMeetingAccessCollaboration
  loadError?: string
  meetings: readonly PartnerMeetingAccessMeeting[] | null
}): PartnerMeetingAccess {
  if (
    !collaboration.visibility?.includes("meeting") ||
    collaboration.status === "resolved" ||
    !collaboration.authorization_active
  ) {
    return { meeting: null, state: "hidden", message: "" }
  }

  if (loadError) {
    return { meeting: null, state: "unavailable", message: pt("unavailable") }
  }
  if (meetings === null) {
    return { meeting: null, state: "loading", message: pt("loading") }
  }

  const collaborationMeetings = meetings.filter((meeting) =>
    meeting.collaboration_id === collaboration.id &&
    meeting.ticket_id === collaboration.ticket_id,
  )
  const joinableMeeting = collaborationMeetings
    .filter((meeting) => JOINABLE_MEETING_STATUS_PRIORITY[meeting.status] !== undefined)
    .sort((left, right) =>
      JOINABLE_MEETING_STATUS_PRIORITY[left.status] - JOINABLE_MEETING_STATUS_PRIORITY[right.status],
    )[0]

  if (joinableMeeting) {
    return { meeting: joinableMeeting, state: "joinable", message: "" }
  }
  if (collaborationMeetings.some((meeting) => CANCELLED_MEETING_STATUSES.has(meeting.status))) {
    return { meeting: null, state: "cancelled", message: pt("cancelled") }
  }
  if (collaborationMeetings.some((meeting) => ENDED_MEETING_STATUSES.has(meeting.status))) {
    return { meeting: null, state: "ended", message: pt("ended") }
  }
  return { meeting: null, state: "none", message: pt("none") }
}
