import type { ChannelSecretStatus, WhatsAppSecretKey, WhatsAppSecretsInput } from "@/lib/api/admin"

export const WHATSAPP_SECRET_KEYS: readonly WhatsAppSecretKey[] = ["metaAccessToken", "metaAppSecret", "webhookVerifyToken"]

export const WHATSAPP_SECRET_REF_FIELD = {
  metaAccessToken: "metaAccessTokenSecretRef",
  metaAppSecret: "metaAppSecretSecretRef",
  webhookVerifyToken: "webhookVerifyTokenSecretRef",
} as const

export type WhatsAppSecretDraft = Record<WhatsAppSecretKey, string>
export type WhatsAppSecretStatuses = Partial<Record<WhatsAppSecretKey, ChannelSecretStatus>>

/** Returns undefined when nothing changed, so stored secrets stay untouched. */
export function buildWhatsAppSecretsPayload(
  draft: WhatsAppSecretDraft,
  cleared: ReadonlySet<WhatsAppSecretKey>,
): WhatsAppSecretsInput | undefined {
  const payload: WhatsAppSecretsInput = {}
  for (const key of WHATSAPP_SECRET_KEYS) {
    const value = draft[key].trim()
    if (value) payload[key] = value
  }
  // A newly typed value replaces the stored one, so it must not also be cleared.
  const clear = WHATSAPP_SECRET_KEYS.filter((key) => cleared.has(key) && !payload[key])
  if (clear.length) payload.clear = clear
  return Object.keys(payload).length ? payload : undefined
}

/** Typed now, stored and not being cleared, or referenced from a server secret file. */
export function isWhatsAppSecretSatisfied(
  key: WhatsAppSecretKey,
  draft: WhatsAppSecretDraft,
  statuses: WhatsAppSecretStatuses | undefined,
  cleared: ReadonlySet<WhatsAppSecretKey>,
  ref: string,
): boolean {
  if (draft[key].trim()) return true
  if (statuses?.[key]?.configured && !cleared.has(key)) return true
  return ref.trim() !== ""
}
