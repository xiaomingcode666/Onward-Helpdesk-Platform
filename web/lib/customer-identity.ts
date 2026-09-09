import { translateCurrentMessage } from "@/i18n/messages"

export function customerDisplayName(
  value?: string | null,
  fallback = translateCurrentMessage("miscTailExtract.libMisc.customerIdentity.unlinkedCustomer"),
) {
  const name = String(value || "").trim()
  if (!name || name.startsWith("访客") || name.includes("@")) {
    return fallback
  }
  return name
}
