import { SUPPORTED_LOCALES } from "@/i18n/config"
import { translateMessage } from "@/i18n/messages"
import type { EnterpriseAIKey } from "@/lib/api/enterprise-models"

const ENGLISH_KEY_NAME = /^[\x00-\x7F]+$/

function isEnglishKeyName(value: string | undefined) {
  const name = value?.trim() || ""
  return name.length > 0 && ENGLISH_KEY_NAME.test(name)
}

// Older Sub2API accounts were named with the rendered "default key" label of
// whichever locale created them.
function isLegacyDefaultKeyName(value: string | undefined) {
  const name = value?.trim().toLowerCase() || ""
  return name.length > 0 && SUPPORTED_LOCALES.some((locale) =>
    name.includes(translateMessage(locale, "enterpriseModels.apiKeys.legacyDefaultKeyName").toLowerCase())
  )
}

function isCompanyKeyName(value: string | undefined) {
  return isEnglishKeyName(value) && value!.trim().toUpperCase().startsWith("RHD-COMPANY-")
}

export function getEnterpriseAIKeyDisplayName(key: EnterpriseAIKey, fallback?: string) {
  return key.productName || key.name || fallback || `Key #${key.id}`
}

export function getEnterpriseAIKeyProductMeta(key: EnterpriseAIKey) {
  if (!key.productName) return ""
  return [key.productCode, key.name && key.name !== key.productName ? key.name : ""].filter(Boolean).join(" · ")
}

/**
 * A tenant has one shared model key in the enterprise usage view. Older
 * Sub2API accounts can still return legacy product and Chinese-named keys;
 * keep the stable English tenant key when one is available and show only it.
 */
export function selectSingleEnterpriseAIKey(
  keys: EnterpriseAIKey[] | undefined,
  preferredKeyId?: string,
  preferredKeyName?: string,
) {
  const items = keys ?? []
  if (items.length <= 1) return items
  const preferredId = Number(preferredKeyId)
  const normalizedName = preferredKeyName?.trim() || ""
  const selected =
    (preferredId > 0 ? items.find((key) => key.id === preferredId && (isCompanyKeyName(key.name) || isLegacyDefaultKeyName(key.name))) : undefined) ??
    (normalizedName && (isCompanyKeyName(normalizedName) || isLegacyDefaultKeyName(normalizedName)) ? items.find((key) => key.name.trim() === normalizedName) : undefined) ??
    items.find((key) => isCompanyKeyName(key.name)) ??
    items.find((key) => isLegacyDefaultKeyName(key.name)) ??
    items.find((key) => isEnglishKeyName(key.name)) ??
    items.find((key) => key.status.toLowerCase() === "active") ??
    items[0]
  return selected ? [selected] : []
}

export function getSingleEnterpriseAIKeyName(key: EnterpriseAIKey, preferredKeyName?: string, tenantId?: number) {
  if (isEnglishKeyName(key.name)) return key.name.trim()
  if (isEnglishKeyName(preferredKeyName)) return preferredKeyName!.trim()
  return `RHD-COMPANY-ID-${tenantId && tenantId > 0 ? tenantId : "UNKNOWN"}-DEFAULT`
}
