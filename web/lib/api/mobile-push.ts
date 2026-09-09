import { apiPost } from "@/lib/api/client"
import { customerPortalRequest } from "@/lib/api/customer-portal-client"
import { readSession } from "@/lib/auth"

export type MobilePushToken = {
  id: number
  platform: "android" | "ios"
  deviceId?: string
  appVersion?: string
  lastSeenAt: string
}

export async function registerMobilePushToken(payload: {
  platform: "android" | "ios"
  token: string
  deviceId?: string
  appVersion?: string
}) {
  if (readSession()?.domainType === "customer") {
    return customerPortalRequest<MobilePushToken>("/notifications/push-tokens", {
      method: "POST",
      body: JSON.stringify(payload),
    })
  }
  const result = await apiPost<MobilePushToken>("/notifications/push-tokens", payload)
  if (!result.success) throw new Error(result.error?.message || "mobile push token registration failed")
  return result.data
}

export async function revokeMobilePushToken(id: number) {
  if (readSession()?.domainType === "customer") {
    return customerPortalRequest<void>(`/notifications/push-tokens/${id}/_revoke`, {
      method: "POST",
    })
  }
  const result = await apiPost<void>(`/notifications/push-tokens/${id}/_revoke`, {})
  if (!result.success) throw new Error(result.error?.message || "mobile push token revoke failed")
}
