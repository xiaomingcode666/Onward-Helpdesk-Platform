import { request } from "@/lib/api/client"

export type DemoRequestPayload = {
  company: string
  contactName: string
  email: string
  mobile: string
  countryRegion: string
  requirements: string
  website: string
}

export function submitDemoRequest(payload: DemoRequestPayload) {
  return request<void>("/api/marketing/demo-request", {
    method: "POST",
    body: JSON.stringify(payload),
    skipAuth: true,
    timeoutMs: 15000,
  })
}
