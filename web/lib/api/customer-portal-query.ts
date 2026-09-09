"use client"

export type CustomerPortalQueryValue = string | number | boolean | null | undefined

export function buildCustomerPortalQueryString(query?: Record<string, CustomerPortalQueryValue>) {
  if (!query) {
    return ""
  }
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) {
    if (value === undefined || value === null || value === "") {
      continue
    }
    params.set(key, String(value))
  }
  const search = params.toString()
  return search ? `?${search}` : ""
}
