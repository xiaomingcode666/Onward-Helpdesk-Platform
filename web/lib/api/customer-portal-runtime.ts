import type { ImRuntimeConfig } from "@/lib/api/im"

export function selectInitialCustomerConversationId(
  conversations: Array<{ id: number; status: string }>,
  requestedId: number,
) {
  const requested = conversations.find((item) => item.id === requestedId)
  if (requested) {
    return requested.id
  }
  return conversations.find((item) => item.status !== "closed")?.id ?? 0
}

export function readCustomerPortalRuntimeConfig(): ImRuntimeConfig {
  if (typeof window === "undefined") {
    return {
      channelId: "",
      baseUrl: "",
      apiBaseUrl: "",
    }
  }

  const query = new URLSearchParams(window.location.search)
  const baseUrl =
    query.get("baseUrl")?.trim() ||
    process.env.NEXT_PUBLIC_API_BASE_URL?.trim() ||
    window.location.origin

  return {
    channelId:
      query.get("channelId")?.trim() ||
      process.env.NEXT_PUBLIC_CUSTOMER_PORTAL_CHANNEL_ID?.trim() ||
      process.env.NEXT_PUBLIC_OPEN_IM_CHANNEL_ID?.trim() ||
      "",
    baseUrl: baseUrl.replace(/\/$/, ""),
    apiBaseUrl:
      query.get("apiBaseUrl")?.trim() ||
      process.env.NEXT_PUBLIC_API_BASE_URL?.trim() ||
      undefined,
    externalId: query.get("externalId")?.trim() || undefined,
    externalName: query.get("externalName")?.trim() || undefined,
    userToken: query.get("userToken")?.trim() || undefined,
  }
}
