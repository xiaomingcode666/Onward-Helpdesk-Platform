export type MeetingSyncEvent = {
  type: "meeting-updated" | "meeting-ended"
  meetingId?: string
  status?: string
  timestamp: number
}

const CHANNEL_NAME = "remotehelpdesk:meeting-sync"
const LOCAL_EVENT_NAME = "remotehelpdesk:meeting-sync-local"

export function publishMeetingSync(event: Omit<MeetingSyncEvent, "timestamp">) {
  if (typeof window === "undefined") return
  const payload: MeetingSyncEvent = { ...event, timestamp: Date.now() }
  window.dispatchEvent(new CustomEvent<MeetingSyncEvent>(LOCAL_EVENT_NAME, { detail: payload }))
  if (typeof BroadcastChannel !== "undefined") {
    const channel = new BroadcastChannel(CHANNEL_NAME)
    channel.postMessage(payload)
    channel.close()
  }
}

export function subscribeMeetingSync(listener: (event: MeetingSyncEvent) => void) {
  if (typeof window === "undefined") return () => undefined
  const handleLocal = (event: Event) => listener((event as CustomEvent<MeetingSyncEvent>).detail)
  window.addEventListener(LOCAL_EVENT_NAME, handleLocal)
  const channel = typeof BroadcastChannel !== "undefined" ? new BroadcastChannel(CHANNEL_NAME) : null
  if (channel) channel.onmessage = (event: MessageEvent<MeetingSyncEvent>) => listener(event.data)
  return () => {
    window.removeEventListener(LOCAL_EVENT_NAME, handleLocal)
    channel?.close()
  }
}
