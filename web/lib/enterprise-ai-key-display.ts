import type { EnterpriseAIKey } from "@/lib/api/enterprise-models"

export function getEnterpriseAIKeyDisplayName(key: EnterpriseAIKey, fallback?: string) {
  return key.productName || key.name || fallback || `Key #${key.id}`
}

export function getEnterpriseAIKeyProductMeta(key: EnterpriseAIKey) {
  if (!key.productName) return ""
  return [key.productCode, key.name && key.name !== key.productName ? key.name : ""].filter(Boolean).join(" · ")
}
