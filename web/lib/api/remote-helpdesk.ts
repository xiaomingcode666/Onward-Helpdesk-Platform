import { request } from "@/lib/api/client"
import type { PageResult } from "@/lib/api/admin"
import { fetchConversations, type AdminConversation } from "@/lib/api/admin"
import { fetchTickets, type TicketItem } from "@/lib/api/ticket"

export type ProductTicket = TicketItem
export type ProductConversation = AdminConversation

export type AdminProduct = {
  id: number
  tenantId: number
  productLineId: number
  code: string
  name: string
  category: string
  ownerMemberId: number
  defaultLocale: string
  status: number
  createdAt: string
  updatedAt: string
}

export type AdminProductModel = {
  id: number
  tenantId: number
  productId: number
  modelCode: string
  name: string
  versionPolicy: string
  regionScopeJson: string
  status: number
  createdAt: string
  updatedAt: string
}

export type AdminDevice = {
  id: number
  tenantId: number
  deviceNo: string
  productId: number
  productModelId: number
  serialNo: string
  customerOrgId: number
  externalDeviceId: string
  installLocationJson: string
  regionCode: string
  source: string
  status: number
  metadataJson: string
  createdAt: string
  updatedAt: string
}

export type AdminProductServiceProfile = {
  id: number
  tenantId: number
  productId: number
  supportLocalesJson: string
  supportRegionsJson: string
  warrantyPolicyJson: string
  safetyLevel: string
  defaultFlowTemplateId: number
  defaultKnowledgeBaseId: number
  meetingEnabled: boolean
  servicePolicyJson: string
  status: number
  createdAt: string
  updatedAt: string
}

export type AdminTenantIntegrationConfig = {
  id: number
  tenantId: number
  provider: string
  baseUrl: string
  appId: string
  enabled: boolean
  status: number
  metadataJson: string
  maskedAppSecret: string
  createdAt: string
  updatedAt: string
}

export type AdminProductAIUsageCredential = {
  id: number
  tenantId: number
  productId: number
  sub2apiAccount: string
  quotaPolicyJson: string
  status: number
  lastSyncedAt?: string
  maskedApiKey: string
  createdAt: string
  updatedAt: string
}

export type CreateAdminTenantIntegrationConfigPayload = {
  tenantId: number
  provider: string
  baseUrl: string
  appId: string
  appKey: string
  appSecret: string
  enabled: boolean
  metadataJson: string
}

export type UpdateAdminTenantIntegrationConfigPayload = CreateAdminTenantIntegrationConfigPayload & { id: number }
export type CreateAdminProductAIUsageCredentialPayload = { tenantId: number; productId: number; sub2apiAccount: string; apiKey: string; quotaPolicyJson: string }
export type UpdateAdminProductAIUsageCredentialPayload = CreateAdminProductAIUsageCredentialPayload & { id: number }
export type MeetingPreviewPayload = { tenantId: number; productId: number; businessType: string; businessId: number }
export type MeetingPreview = { roomName: string; joinUrl: string; expiresAt: string }

export type AdminServiceCodeBatch = {
  id: number
  tenantId: number
  batchNo: string
  mode: string
  productId: number
  productModelId: number
  quantity: number
  labelTemplateId: number
  status: number
  generatedCount: number
  exportedCount: number
  createdAt: string
  updatedAt: string
}

export type AdminServiceCode = {
  id: number
  tenantId: number
  batchId: number
  serviceCode: string
  mode: string
  deviceId: number
  productId: number
  productModelId: number
  status: string
  activatedAt: string
  revokedAt: string
  metadataJson: string
  createdAt: string
  updatedAt: string
}

export type CreateAdminProductPayload = {
  tenantId: number
  productLineId: number
  code: string
  name: string
  category: string
  ownerMemberId: number
  defaultLocale: string
}

export type UpdateAdminProductPayload = CreateAdminProductPayload & {
  id: number
}

export type CreateAdminProductModelPayload = {
  tenantId: number
  productId: number
  modelCode: string
  name: string
  versionPolicy: string
  regionScopeJson: string
}

export type UpdateAdminProductModelPayload = CreateAdminProductModelPayload & {
  id: number
}

export type CreateAdminDevicePayload = {
  tenantId: number
  deviceNo: string
  productId: number
  productModelId: number
  serialNo: string
  customerOrgId: number
  externalDeviceId: string
  installLocationJson: string
  regionCode: string
  source: string
  metadataJson: string
}

export type UpdateAdminDevicePayload = CreateAdminDevicePayload & {
  id: number
}

export type CreateAdminProductServiceProfilePayload = {
  tenantId: number
  productId: number
  supportLocalesJson: string
  supportRegionsJson: string
  warrantyPolicyJson: string
  safetyLevel: string
  defaultFlowTemplateId: number
  defaultKnowledgeBaseId: number
  meetingEnabled: boolean
  servicePolicyJson: string
}

export type UpdateAdminProductServiceProfilePayload =
  CreateAdminProductServiceProfilePayload & {
    id: number
  }

export type CreateAdminServiceCodeBatchPayload = {
  tenantId: number
  batchNo: string
  mode: string
  productId: number
  productModelId: number
  quantity: number
  labelTemplateId: number
}

export type AdminProductListAllQuery = {
  tenantId?: string | number
}

export type AdminProductModelListAllQuery = {
  tenantId?: string | number
  productId?: string | number
}

function toQueryString(query?: Record<string, string | number | undefined>) {
  if (!query) {
    return ""
  }
  const params = new URLSearchParams()
  Object.entries(query).forEach(([key, value]) => {
    if (value === undefined || value === "") {
      return
    }
    params.set(key, String(value))
  })
  const output = params.toString()
  return output ? `?${output}` : ""
}

export function fetchProducts(query?: Record<string, string | number | undefined>) {
  return request<PageResult<AdminProduct>>(
    `/api/dashboard/product/list${toQueryString(query)}`
  )
}

export function fetchProductListAll(query?: AdminProductListAllQuery) {
  return request<AdminProduct[]>(
    `/api/dashboard/product/list_all${toQueryString(query)}`
  )
}

export function fetchProduct(id: number) {
  return request<AdminProduct>(`/api/dashboard/product/${id}`)
}

export function createProduct(payload: CreateAdminProductPayload) {
  return request<AdminProduct>("/api/dashboard/product/create", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updateProduct(payload: UpdateAdminProductPayload) {
  return request<void>("/api/dashboard/product/update", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updateProductStatus(id: number, status: number) {
  return request<void>("/api/dashboard/product/update_status", {
    method: "POST",
    body: JSON.stringify({ id, status }),
  })
}

export function deleteProduct(id: number) {
  return request<void>("/api/dashboard/product/delete", {
    method: "POST",
    body: JSON.stringify({ id }),
  })
}

export function fetchProductModels(
  query?: Record<string, string | number | undefined>
) {
  return request<PageResult<AdminProductModel>>(
    `/api/dashboard/product-model/list${toQueryString(query)}`
  )
}

export function fetchProductModelListAll(
  query?: AdminProductModelListAllQuery
) {
  return request<AdminProductModel[]>(
    `/api/dashboard/product-model/list_all${toQueryString(query)}`
  )
}

export function fetchProductModel(id: number) {
  return request<AdminProductModel>(`/api/dashboard/product-model/${id}`)
}

export function createProductModel(payload: CreateAdminProductModelPayload) {
  return request<AdminProductModel>("/api/dashboard/product-model/create", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updateProductModel(payload: UpdateAdminProductModelPayload) {
  return request<void>("/api/dashboard/product-model/update", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updateProductModelStatus(id: number, status: number) {
  return request<void>("/api/dashboard/product-model/update_status", {
    method: "POST",
    body: JSON.stringify({ id, status }),
  })
}

export function deleteProductModel(id: number) {
  return request<void>("/api/dashboard/product-model/delete", {
    method: "POST",
    body: JSON.stringify({ id }),
  })
}

export function fetchDevices(query?: Record<string, string | number | undefined>) {
  return request<PageResult<AdminDevice>>(
    `/api/dashboard/device/list${toQueryString(query)}`
  )
}

export function fetchProductDevices(
  productId: number,
  query?: Record<string, string | number | undefined>
) {
  return fetchDevices({ ...query, productId })
}

export function fetchDevice(id: number) {
  return request<AdminDevice>(`/api/dashboard/device/${id}`)
}

export function createDevice(payload: CreateAdminDevicePayload) {
  return request<AdminDevice>("/api/dashboard/device/create", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updateDevice(payload: UpdateAdminDevicePayload) {
  return request<void>("/api/dashboard/device/update", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updateDeviceStatus(id: number, status: number) {
  return request<void>("/api/dashboard/device/update_status", {
    method: "POST",
    body: JSON.stringify({ id, status }),
  })
}

export function deleteDevice(id: number) {
  return request<void>("/api/dashboard/device/delete", {
    method: "POST",
    body: JSON.stringify({ id }),
  })
}

export function fetchProductServiceProfiles(
  query?: Record<string, string | number | undefined>
) {
  return request<PageResult<AdminProductServiceProfile>>(
    `/api/dashboard/product-service-profile/list${toQueryString(query)}`
  )
}

export function fetchProductServiceProfileByProduct(productId: number) {
  return fetchProductServiceProfiles({ productId, page: 1, limit: 1 })
}

export function fetchProductServiceProfile(id: number) {
  return request<AdminProductServiceProfile>(
    `/api/dashboard/product-service-profile/${id}`
  )
}

export function createProductServiceProfile(
  payload: CreateAdminProductServiceProfilePayload
) {
  return request<AdminProductServiceProfile>(
    "/api/dashboard/product-service-profile/create",
    {
      method: "POST",
      body: JSON.stringify(payload),
    }
  )
}

export function updateProductServiceProfile(
  payload: UpdateAdminProductServiceProfilePayload
) {
  return request<void>("/api/dashboard/product-service-profile/update", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updateProductServiceProfileStatus(id: number, status: number) {
  return request<void>("/api/dashboard/product-service-profile/update_status", {
    method: "POST",
    body: JSON.stringify({ id, status }),
  })
}

export function deleteProductServiceProfile(id: number) {
  return request<void>("/api/dashboard/product-service-profile/delete", {
    method: "POST",
    body: JSON.stringify({ id }),
  })
}

export function fetchTenantIntegrationConfigs(query?: Record<string, string | number | undefined>) {
  return request<PageResult<AdminTenantIntegrationConfig>>(`/api/dashboard/tenant-integration-config/list${toQueryString(query)}`)
}
export function fetchTenantIntegrationConfig(id: number) { return request<AdminTenantIntegrationConfig>(`/api/dashboard/tenant-integration-config/${id}`) }
export function createTenantIntegrationConfig(payload: CreateAdminTenantIntegrationConfigPayload) { return request<AdminTenantIntegrationConfig>("/api/dashboard/tenant-integration-config/create", { method: "POST", body: JSON.stringify(payload) }) }
export function updateTenantIntegrationConfig(payload: UpdateAdminTenantIntegrationConfigPayload) { return request<void>("/api/dashboard/tenant-integration-config/update", { method: "POST", body: JSON.stringify(payload) }) }
export function updateTenantIntegrationConfigStatus(id: number, status: number) { return request<void>("/api/dashboard/tenant-integration-config/update_status", { method: "POST", body: JSON.stringify({ id, status }) }) }
export function deleteTenantIntegrationConfig(id: number) { return request<void>("/api/dashboard/tenant-integration-config/delete", { method: "POST", body: JSON.stringify({ id }) }) }

export function fetchProductAIUsageCredentials(query?: Record<string, string | number | undefined>) {
  return request<PageResult<AdminProductAIUsageCredential>>(`/api/dashboard/product-ai-usage-credential/list${toQueryString(query)}`)
}
export function fetchProductAIUsageCredential(id: number) { return request<AdminProductAIUsageCredential>(`/api/dashboard/product-ai-usage-credential/${id}`) }
export function createProductAIUsageCredential(payload: CreateAdminProductAIUsageCredentialPayload) { return request<AdminProductAIUsageCredential>("/api/dashboard/product-ai-usage-credential/create", { method: "POST", body: JSON.stringify(payload) }) }
export function updateProductAIUsageCredential(payload: UpdateAdminProductAIUsageCredentialPayload) { return request<void>("/api/dashboard/product-ai-usage-credential/update", { method: "POST", body: JSON.stringify(payload) }) }
export function updateProductAIUsageCredentialStatus(id: number, status: number) { return request<void>("/api/dashboard/product-ai-usage-credential/update_status", { method: "POST", body: JSON.stringify({ id, status }) }) }
export function deleteProductAIUsageCredential(id: number) { return request<void>("/api/dashboard/product-ai-usage-credential/delete", { method: "POST", body: JSON.stringify({ id }) }) }
export function createMeetingPreview(payload: MeetingPreviewPayload) { return request<MeetingPreview>("/api/dashboard/meeting/preview-create", { method: "POST", body: JSON.stringify(payload) }) }

export function fetchServiceCodeBatches(
  query?: Record<string, string | number | undefined>
) {
  return request<PageResult<AdminServiceCodeBatch>>(
    `/api/dashboard/service-code-batch/list${toQueryString(query)}`
  )
}

export function fetchServiceCodeBatch(id: number) {
  return request<AdminServiceCodeBatch>(
    `/api/dashboard/service-code-batch/${id}`
  )
}

export function createServiceCodeBatch(
  payload: CreateAdminServiceCodeBatchPayload
) {
  return request<AdminServiceCodeBatch>("/api/dashboard/service-code-batch/create", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updateServiceCodeBatchStatus(id: number, status: number) {
  return request<void>("/api/dashboard/service-code-batch/update_status", {
    method: "POST",
    body: JSON.stringify({ id, status }),
  })
}

export function deleteServiceCodeBatch(id: number) {
  return request<void>("/api/dashboard/service-code-batch/delete", {
    method: "POST",
    body: JSON.stringify({ id }),
  })
}

export function generateServiceCodes(batchId: number, count: number) {
  return request<AdminServiceCode[]>("/api/dashboard/service-code/generate", {
    method: "POST",
    body: JSON.stringify({ batchId, count }),
  })
}

export function fetchServiceCodes(
  query?: Record<string, string | number | undefined>
) {
  return request<PageResult<AdminServiceCode>>(
    `/api/dashboard/service-code/list${toQueryString(query)}`
  )
}

export function fetchProductServiceCodes(
  productId: number,
  query?: Record<string, string | number | undefined>
) {
  return fetchServiceCodes({ ...query, productId })
}

export function fetchProductTickets(
  productId: number,
  query?: Record<string, string | number | undefined>
) {
  return fetchTickets({ ...query, productId })
}

export function fetchProductRepairHistory(
  productId: number,
  query?: Record<string, string | number | undefined>
) {
  return request<PageResult<TicketItem>>(
    `/api/dashboard/ticket/repair-history/list${toQueryString({
      ...query,
      productId,
    })}`
  )
}

export function fetchProductConversations(
  productId: number,
  query?: Record<string, string | number | undefined>
) {
  return fetchConversations({ ...query, productId }) as Promise<
    PageResult<AdminConversation>
  >
}

export type ProductKnowledgeCoverage = {
  productId: number
  defaultKnowledgeBaseId: number
  bindingCount: number
  status: number
}

export async function fetchProductKnowledgeCoverage(productId: number) {
  const [profilePage, bindingPage] = await Promise.all([
    fetchProductServiceProfileByProduct(productId),
    request<PageResult<{ id: number; status: number }>>(
      `/api/dashboard/product-knowledge-binding/list${toQueryString({
        productId,
        page: 1,
        limit: 1,
      })}`
    ),
  ])
  const profile = profilePage.results[0]
  return {
    productId,
    defaultKnowledgeBaseId: profile?.defaultKnowledgeBaseId ?? 0,
    bindingCount: bindingPage.page.total,
    status: profile?.status ?? 0,
  } satisfies ProductKnowledgeCoverage
}

export function fetchServiceCode(id: number) {
  return request<AdminServiceCode>(`/api/dashboard/service-code/${id}`)
}

export function revokeServiceCode(id: number) {
  return request<void>("/api/dashboard/service-code/revoke", {
    method: "POST",
    body: JSON.stringify({ id }),
  })
}
