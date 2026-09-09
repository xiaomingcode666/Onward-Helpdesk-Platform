import { request } from "@/lib/api/client"

export type PublicConfig = {
  language: string
  wxworkEnabled: boolean
  oidcEnabled: boolean
}

export async function fetchPublicConfig() {
  return request<PublicConfig>("/api/config", {
    method: "GET",
    skipAuth: true,
  })
}
