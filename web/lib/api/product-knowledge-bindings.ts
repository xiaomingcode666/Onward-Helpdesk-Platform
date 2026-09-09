import { readSession } from "@/lib/auth"
import { request } from "@/lib/api/client"
import type { PageResult } from "@/lib/api/admin"

export type ProductKnowledgeBindingRecord = {
  id: number
  tenantId: number
  productId: number
  productModelId: number
  knowledgeBaseId: number
  scopeType: string
  locale: string
  regionCode: string
  sortNo: number
  status: number
  createdAt: string
  updatedAt: string
}

export type CreateProductKnowledgeBindingPayload = {
  productId: number
  knowledgeBaseId: number
  productModelId?: number
  scopeType?: string
  locale?: string
  regionCode?: string
  sortNo?: number
}

function resolveTenantId() {
  const session = readSession()
  const sessionTenantId =
    session?.tenantId ?? ((session as Record<string, unknown> | null)?.tenant_id as number | undefined)
  return sessionTenantId || 1
}

function toQueryString(query?: Record<string, string | number | undefined>) {
  if (!query) return ""
  const search = new URLSearchParams()
  Object.entries(query).forEach(([key, value]) => {
    if (value === undefined || value === null || value === "") return
    search.set(key, String(value))
  })
  const raw = search.toString()
  return raw ? `?${raw}` : ""
}

export function fetchProductKnowledgeBindings(
  query?: Record<string, string | number | undefined>
) {
  return request<PageResult<ProductKnowledgeBindingRecord>>(
    `/api/dashboard/product-knowledge-binding/list${toQueryString(query)}`
  )
}

export function createProductKnowledgeBinding(
  payload: CreateProductKnowledgeBindingPayload
) {
  return request<ProductKnowledgeBindingRecord>("/api/dashboard/product-knowledge-binding/create", {
    method: "POST",
    body: JSON.stringify({
      tenantId: resolveTenantId(),
      productId: payload.productId,
      productModelId: payload.productModelId || 0,
      knowledgeBaseId: payload.knowledgeBaseId,
      scopeType: payload.scopeType || "product",
      locale: payload.locale || "",
      regionCode: payload.regionCode || "",
      sortNo: payload.sortNo || 100,
    }),
  })
}
