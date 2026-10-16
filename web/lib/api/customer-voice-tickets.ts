import { customerPortalRequest } from "@/lib/api/customer-portal-client"

export type CustomerVoiceTranscription = {
  text: string
  language: string
  provider: string
  durationSeconds: number
}

export type CustomerVoiceTicketResult = {
  ticketId: number
  ticketNo: string
}

export function transcribeCustomerVoice(wav: File) {
  const formData = new FormData()
  formData.set("file", wav)
  return customerPortalRequest<CustomerVoiceTranscription>("/voice-tickets/_transcribe", { method: "POST", body: formData })
}

// Creates a standalone ticket (not tied to any conversation) from the reviewed
// transcript and attaches the original recording.
export function submitCustomerVoiceTicket(body: { transcript: string; requestKey: string; audio: File }) {
  const formData = new FormData()
  formData.set("transcript", body.transcript)
  formData.set("requestKey", body.requestKey)
  formData.set("audio", body.audio)
  return customerPortalRequest<CustomerVoiceTicketResult>("/voice-tickets", { method: "POST", body: formData })
}
