// ============================================================
// Enterprise Product Aggregated API
// ============================================================
// Wraps the enterprise-domain product endpoints with
// aggregated product profile, listing, and detail queries.
//
// Endpoints (proxied via /api/enterprise/v1):
//   GET  /product-count        — visible product count
//   GET  /products             — list products
//   GET  /products/:id         — product detail
//   GET  /products/:id/profile — product service profile
//   GET  /products/:id/devices — product devices
//   GET  /products/:id/tickets — product ticket context
//   POST /products             — create product
//   PATCH /products/:id        — update product
// ============================================================

import { translateCurrentMessage } from "@/i18n/messages"
import { apiDelete, apiGet, apiPatch, apiPost, apiPut, buildFilterQuery, listQueryToParams, requestBlob } from "@/lib/api/client"
import type {
  ApiResponse,
  EnterpriseListResponse,
  ListQuery,
  ProductListItem,
  CreateProductPayload,
  UpdateProductPayload,
} from "@/lib/api/types"

export interface ProductDetail extends ProductListItem {
  product_line_id: number
  owner_member_id: number
  meeting_enabled: boolean
  safety_level: string
  knowledge_base_id: number
}

export interface ProductListQuery extends ListQuery {
  status?: string
  product_line?: string
  search?: string
}

export type EnterpriseProductCount = {
  total: number
}

/**
 * Product service profile — aggregated profile returned by GET /products/:id/profile
 * Backend returns nested structure: { product, serviceProfile, totalTicketCount, totalKnowledgeEntryCount }
 */
export interface ProductServiceProfile {
  product: import("./types").ProductListItem
  serviceProfile: {
    product_id: number
    support_locales: string[]
    support_regions: string[]
    warranty_policy: {
      duration_months: number
      terms: string
    }
    safety_level: string
    meeting_enabled: boolean
    knowledge_base_id: number | null
    knowledge_base_name?: string
    knowledge_base_description?: string
    ragflow_dataset_id?: string | null
    status: string
  }
  totalTicketCount: number
  totalKnowledgeEntryCount: number
}

export type CreateProductKnowledgeBasePayload = {
  name: string
  description?: string
  ragflow_dataset_id?: string
  support_locales?: string[]
  support_regions?: string[]
}

export interface ProductResources {
  product_id: number
  status: "ready" | "pending" | "failed" | "partial_failed" | "missing" | string
  ai_key: {
    status: string
    key_id: string
    key_name: string
    key_preview: string
    quota_limit: number
    quota_used: number
    currency: string
    provision_error?: string
    last_synced_at?: string
    provision_status: string
  }
  knowledge: {
    status: string
    knowledge_base_id: number
    knowledge_base_name: string
    dataset_id: string
    provider: string
    provision_error?: string
  }
  agent: {
    status: string
    agent_id: number
    agent_name: string
    review_status: string
    enabled: boolean
  }
  job?: {
    id: number
    status: string
    requested_quota_limit: number
    knowledge_status: string
    ai_key_status: string
    agent_status: string
    error_summary?: string
    retry_count: number
    max_retries: number
    next_attempt_at?: string
    last_attempt_at?: string
    started_at?: string
    finished_at?: string
    updated_at?: string
  }
}

// ---- API functions ----

export async function listProducts(
  query?: ProductListQuery
): Promise<ApiResponse<ProductListItem[]>> {
  const params: Record<string, unknown> = {
    ...listQueryToParams(query || {}),
  }
  if (query?.search) {
    const filter = buildFilterQuery({ title: `~${query.search}` })
    if (filter) {
      params["filter"] = params["filter"]
        ? `${params["filter"]},${filter}`
        : filter
    }
  }
  return apiGet<ProductListItem[]>("/products", params)
}

export async function listAllProducts(
  query: Omit<ProductListQuery, "page" | "page_size"> = {}
): Promise<ProductListItem[]> {
  const products: ProductListItem[] = []
  for (let page = 1; page <= 100; page += 1) {
    const response = await listProducts({ ...query, page, page_size: 100 })
    if (!response.success || !response.data) {
      throw new Error(response.error?.message || translateCurrentMessage("miscTailExtract.libMisc.enterpriseProducts.productCatalogLoadFailed"))
    }
    products.push(...response.data)
    if (response.data.length < 100) break
  }
  return products
}

export async function fetchProductCount(): Promise<ApiResponse<EnterpriseProductCount>> {
  return apiGet<EnterpriseProductCount>("/product-count")
}

export async function getProductDetail(
  id: number
): Promise<ApiResponse<ProductDetail>> {
  return apiGet<ProductDetail>(`/products/${id}`)
}

export async function getProductProfile(
  productId: number
): Promise<ApiResponse<ProductServiceProfile>> {
  return apiGet<ProductServiceProfile>(`/products/${productId}/profile`)
}

export async function getProductResources(
  productId: number
): Promise<ApiResponse<ProductResources>> {
  return apiGet<ProductResources>(`/products/${productId}/resources`)
}

export async function retryProductResources(
  productId: number
): Promise<ApiResponse<ProductResources>> {
  return apiPost<ProductResources>(`/products/${productId}/resources/_retry`)
}

export async function updateProductAIQuota(
  productId: number,
  quotaLimit: number
): Promise<ApiResponse<ProductResources>> {
  return apiPatch<ProductResources>(`/products/${productId}/ai-quota`, {
    quota_limit: quotaLimit,
  })
}

export async function resetProductAIQuota(
  productId: number
): Promise<ApiResponse<ProductResources>> {
  return apiPost<ProductResources>(`/products/${productId}/ai-quota/_reset`)
}

export async function createProductKnowledgeBase(
  productId: number,
  payload: CreateProductKnowledgeBasePayload
): Promise<ApiResponse<ProductServiceProfile>> {
  return apiPost<ProductServiceProfile>(`/products/${productId}/knowledge-base`, payload)
}

export async function createEnterpriseProduct(
  payload: CreateProductPayload
): Promise<ApiResponse<ProductListItem>> {
  return apiPost<ProductListItem>("/products", payload)
}

export async function updateEnterpriseProduct(
  id: number,
  payload: UpdateProductPayload
): Promise<ApiResponse<ProductListItem>> {
  return apiPatch<ProductListItem>(`/products/${id}`, payload)
}

// ============================================================
// Types for product sub-resources
// ============================================================

export interface ProductModule {
  id: number
  tenant_id: number
  product_id: number
  module_code: string
  name: string
  type: string
  status: string
  default_supplier_id: number
  is_safety_critical: boolean
  model_ids: number[]
  model_names: string[]
  created_at: string
  updated_at: string
}

export interface ProductModel {
  id: number
  tenant_id: number
  product_id: number
  model_code: string
  name: string
  version_policy: string
  region_scope_json: string
  status: string
  device_count: number
  ticket_count: number
  created_at: string
  updated_at: string
}

export interface FaultStat {
  part: string
  fault_type: string
  model_name: string
  count: number
  percentage: number
  trend: "up" | "down" | "stable"
  severity: "critical" | "high" | "medium" | "low" | string
}

export interface ProductFaultStats {
  range: "30d" | "90d" | "180d" | string
  generated_at: string
  data_status: "ready" | "empty" | "building" | "failed" | string
  total_faults: number
  affected_devices: number
  stats: FaultStat[]
}

export interface ProductFaultStatsRebuildJob {
  id: number
  product_id: number
  status: string
  range_days: number
  processed_count: number
  error_summary: string
  started_at: string
  finished_at: string
  created_at: string
}

export interface ProductKnowledgeLink {
  id: number
  knowledge_base_id?: number
  ragflow_dataset_id?: string | null
  title: string
  type: "manual" | "knowledge_article" | "video" | "guide"
  language: string
  version: string
  visibility: "public" | "internal" | "team"
  url: string
  updated_at: string
}

export interface ProductManualLink {
  id: number
  product_id: number
  product_model_id: number
  product_model_name: string
  knowledge_base_id: number
  knowledge_base_name: string
  ragflow_dataset_id?: string | null
  knowledge_entry_id: number
  entry_review_status: string
  title: string
  link_type: "manual" | "knowledge_article" | "video" | "guide" | "faq" | string
  language: string
  version: string
  visibility: "public" | "internal" | "team" | string
  publish_status: "draft" | "review" | "published" | "deprecated" | string
  sort_no: number
  index_status: string
  status: string
  created_at: string
  updated_at: string
}

export interface ProductManualFile {
  id: number
  product_id: number
  asset_id: number
  asset_token: string
  title: string
  language: string
  version: string
  visibility: "public" | "internal" | string
  filename: string
  file_size: number
  mime_type: string
  provider: string
  url: string
  knowledge_base_id: number
  knowledge_document_id: number
  rag_sync_status: string
  rag_sync_error: string
  synced_at: string
  status: string
  uploaded_at: string
  uploaded_by: string
  created_at: string
  updated_at: string
}

export interface ProductKnowledgeDocumentFile {
  id: number
  knowledge_entry_id: number
  product_id: number
  knowledge_base_id: number
  knowledge_link_id: number
  source_asset_id: number
  source_type: "uploaded_document" | "product_manual" | "knowledge_entry" | string
  source_reference_id: number
  title: string
  filename: string
  file_size: number
  mime_type: string
  provider: string
  url: string
  index_status: string
  review_status: string
  deletable: boolean
  index_error: string
  indexed_at: string
  uploaded_at: string
  uploaded_by: string
  created_at: string
  updated_at: string
}

export interface ProductKnowledgeDocumentQuery extends ListQuery {
  limit?: number
}

export type ProductManualFileMutation = {
  title: string
  language: string
  version: string
  visibility: "public" | "internal" | string
}

export interface ProductDevice {
  id: number
  device_no: string
  serial_no: string
  model_name: string
  region_code: string
  status: string
  last_active_at: string
}

export interface ProductServiceCode {
  id: number
  service_code: string
  entry_url?: string
  qr_url?: string
  qr_image_url?: string
  batch_no: string
  mode: string
  status: string
  bound_device: string | null
  created_at: string
}

export interface ProductTicketBrief {
  id: number
  ticket_no: string
  title: string
  status: string
  priority: string
  created_at: string
}

export interface ProductConversationBrief {
  id: number
  channel: string
  customer_name: string
  service_mode: string
  status: string
  last_message_at: string
  summary: string
}

export interface ProductRepairRecord {
  id: number
  ticket_id: number
  ticket_no: string
  device_no: string
  fault_type: string
  resolution: string
  technician: string
  completed_at: string
}

export interface ProductKnowledgeCoverageDetail {
  total_entries?: number
  product_specific?: number
  model_specific?: number
  knowledge_base_id?: number | null
  ragflow_dataset_id?: string | null
  coverage_score: number
  linked_entries: number
  pending_candidates: number
  gaps: string[]
  links: ProductKnowledgeLink[]
}

export interface ProductQualitySignal {
  id: number
  signal_type: string
  severity: string
  title: string
  description?: string
  source?: string
  source_type?: string
  source_id?: string
  metric_value: number
  sample_count: number
  owner_user_id?: number
  detected_at: string
  acknowledged_at?: string
  resolved_at?: string
  status: string
}

export interface ProductUsageMetric {
  metric: string
  current_month: number
  last_month: number
  unit: string
}

export type CreateProductModelPayload = {
  modelCode?: string
  model_code?: string
  name: string
  versionPolicy?: string
  version_policy?: string
  regionScopeJson?: string
  region_scope_json?: string
}

export type UpdateProductModelPayload = Partial<CreateProductModelPayload>

export type CreateProductModulePayload = {
  moduleCode?: string
  module_code?: string
  name: string
  defaultSupplierId?: number
  default_supplier_id?: number
  isSafetyCritical?: boolean
  is_safety_critical?: boolean
}

export type UpdateProductModulePayload = Partial<CreateProductModulePayload>

export type LinkProductManualPayload = {
  knowledge_entry_id?: number
  knowledge_base_id?: number
  product_model_id?: number
  link_type?: "manual" | "knowledge_article" | "video" | "guide" | "faq" | string
  language?: string
  version?: string
  visibility?: "public" | "internal" | "team" | string
  sort_no?: number
}

export type UpdateProductManualPayload = Partial<
  Pick<LinkProductManualPayload, "product_model_id" | "language" | "version" | "visibility" | "sort_no">
>

export type ProductModuleModelsResponse = {
  module_id: number
  model_ids: number[]
}

export type CreateQualitySignalPayload = {
  productModelId?: number
  product_model_id?: number
  signalType?: string
  signal_type?: string
  severity?: string
  title?: string
  description?: string
  triggerCondition?: string
  trigger_condition?: string
  metricValue?: number
  metric_value?: number
  sampleCount?: number
  sample_count?: number
  source?: string
  sourceType?: string
  source_type?: string
  sourceId?: string
  source_id?: string
  ownerUserId?: number
  owner_user_id?: number
}

export type UpdateQualitySignalPayload = Partial<CreateQualitySignalPayload>

// ============================================================
// Product sub-resource API functions
// ============================================================

export async function getProductModules(
  productId: number
): Promise<ApiResponse<ProductModule[]>> {
  return apiGet<ProductModule[]>(`/products/${productId}/modules`)
}

export async function listProductModels(
  productId: number
): Promise<ApiResponse<ProductModel[]>> {
  return apiGet<ProductModel[]>(`/products/${productId}/models`)
}

export async function createProductModel(
  productId: number,
  payload: CreateProductModelPayload
): Promise<ApiResponse<ProductModel>> {
  return apiPost<ProductModel>(`/products/${productId}/models`, {
    model_code: payload.model_code ?? payload.modelCode ?? "",
    name: payload.name,
    version_policy: payload.version_policy ?? payload.versionPolicy ?? "",
    region_scope_json: payload.region_scope_json ?? payload.regionScopeJson ?? "[]",
  })
}

export async function updateProductModel(
  productId: number,
  modelId: number,
  payload: UpdateProductModelPayload
): Promise<ApiResponse<ProductModel>> {
  return apiPatch<ProductModel>(`/products/${productId}/models/${modelId}`, {
    ...(payload.model_code !== undefined || payload.modelCode !== undefined
      ? { model_code: payload.model_code ?? payload.modelCode ?? "" }
      : {}),
    ...(payload.name !== undefined ? { name: payload.name } : {}),
    ...(payload.version_policy !== undefined || payload.versionPolicy !== undefined
      ? { version_policy: payload.version_policy ?? payload.versionPolicy ?? "" }
      : {}),
    ...(payload.region_scope_json !== undefined || payload.regionScopeJson !== undefined
      ? { region_scope_json: payload.region_scope_json ?? payload.regionScopeJson ?? "[]" }
      : {}),
  })
}

export async function enableProductModel(
  productId: number,
  modelId: number
): Promise<ApiResponse<{ success: boolean }>> {
  return apiPost<{ success: boolean }>(`/products/${productId}/models/${modelId}/_enable`)
}

export async function disableProductModel(
  productId: number,
  modelId: number
): Promise<ApiResponse<{ success: boolean }>> {
  return apiPost<{ success: boolean }>(`/products/${productId}/models/${modelId}/_disable`)
}

export async function createProductModule(
  productId: number,
  payload: CreateProductModulePayload
): Promise<ApiResponse<ProductModule>> {
  return apiPost<ProductModule>(`/products/${productId}/modules`, {
    module_code: payload.module_code ?? payload.moduleCode ?? "",
    name: payload.name,
    default_supplier_id: payload.default_supplier_id ?? payload.defaultSupplierId ?? 0,
    is_safety_critical: payload.is_safety_critical ?? payload.isSafetyCritical ?? false,
  })
}

export async function updateProductModule(
  productId: number,
  moduleId: number,
  payload: UpdateProductModulePayload
): Promise<ApiResponse<ProductModule>> {
  return apiPatch<ProductModule>(`/products/${productId}/modules/${moduleId}`, {
    ...(payload.module_code !== undefined || payload.moduleCode !== undefined
      ? { module_code: payload.module_code ?? payload.moduleCode ?? "" }
      : {}),
    ...(payload.name !== undefined ? { name: payload.name } : {}),
    ...(payload.default_supplier_id !== undefined || payload.defaultSupplierId !== undefined
      ? { default_supplier_id: payload.default_supplier_id ?? payload.defaultSupplierId ?? 0 }
      : {}),
    ...(payload.is_safety_critical !== undefined || payload.isSafetyCritical !== undefined
      ? { is_safety_critical: payload.is_safety_critical ?? payload.isSafetyCritical ?? false }
      : {}),
  })
}

export async function enableProductModule(
  productId: number,
  moduleId: number
): Promise<ApiResponse<{ success: boolean }>> {
  return apiPost<{ success: boolean }>(`/products/${productId}/modules/${moduleId}/_enable`)
}

export async function disableProductModule(
  productId: number,
  moduleId: number
): Promise<ApiResponse<{ success: boolean }>> {
  return apiPost<{ success: boolean }>(`/products/${productId}/modules/${moduleId}/_disable`)
}

export async function getProductModuleModels(
  productId: number,
  moduleId: number
): Promise<ApiResponse<ProductModuleModelsResponse>> {
  return apiGet<ProductModuleModelsResponse>(`/products/${productId}/modules/${moduleId}/models`)
}

export async function replaceProductModuleModels(
  productId: number,
  moduleId: number,
  modelIds: number[]
): Promise<ApiResponse<ProductModuleModelsResponse>> {
  return apiPut<ProductModuleModelsResponse>(`/products/${productId}/modules/${moduleId}/models`, {
    model_ids: modelIds,
  })
}

export async function getProductFaultStats(
  productId: number,
  range: "30d" | "90d" | "180d" = "90d"
): Promise<ApiResponse<ProductFaultStats>> {
  return apiGet<ProductFaultStats>(`/products/${productId}/fault-stats`, { range })
}

export async function rebuildProductFaultStats(
  productId: number,
  range: "30d" | "90d" | "180d" = "90d"
): Promise<ApiResponse<ProductFaultStatsRebuildJob>> {
  return apiPost<ProductFaultStatsRebuildJob>(`/products/${productId}/fault-stats/_rebuild?range=${range}`)
}

export async function getProductKnowledgeLinks(
  productId: number,
  query?: { limit?: number }
): Promise<ApiResponse<ProductManualLink[]>> {
  return apiGet<ProductManualLink[]>(`/products/${productId}/knowledge-links`, query)
}

export async function getProductManuals(
  productId: number
): Promise<ApiResponse<ProductManualLink[]>> {
  return apiGet<ProductManualLink[]>(`/products/${productId}/manuals`)
}

export async function getProductManualFiles(
  productId: number
): Promise<ApiResponse<ProductManualFile[]>> {
  return apiGet<ProductManualFile[]>(`/products/${productId}/manual-files`)
}

export async function getProductManualFileContent(
  productId: number,
  manualFileId: number,
  signal?: AbortSignal
) {
  return requestBlob(
    `/api/enterprise/v1/products/${productId}/manual-files/${manualFileId}/content`,
    { signal }
  )
}

export async function uploadProductManualFile(
  productId: number,
  file: File,
  metadata?: Partial<ProductManualFileMutation>
): Promise<ApiResponse<ProductManualFile>> {
  const formData = new FormData()
  formData.set("file", file)
  if (metadata?.title) formData.set("title", metadata.title)
  if (metadata?.language) formData.set("language", metadata.language)
  if (metadata?.version) formData.set("version", metadata.version)
  if (metadata?.visibility) formData.set("visibility", metadata.visibility)
  return apiPost<ProductManualFile>(`/products/${productId}/manual-files/_upload`, formData)
}

export async function updateProductManualFile(
  productId: number,
  manualFileId: number,
  metadata: ProductManualFileMutation
): Promise<ApiResponse<ProductManualFile>> {
  return apiPatch<ProductManualFile>(`/products/${productId}/manual-files/${manualFileId}`, metadata)
}

export async function deleteProductManualFile(
  productId: number,
  manualFileId: number
): Promise<ApiResponse<{ success: boolean }>> {
  return apiDelete<{ success: boolean }>(`/products/${productId}/manual-files/${manualFileId}`)
}

export async function getProductKnowledgeDocuments(
  productId: number,
  query?: ProductKnowledgeDocumentQuery
): Promise<ApiResponse<EnterpriseListResponse<ProductKnowledgeDocumentFile>>> {
  const params: Record<string, unknown> = {
    ...listQueryToParams(query || {}),
  }
  if (query?.limit && query.limit > 0 && params.page_size === undefined) {
    params.limit = query.limit
  }
  return apiGet<EnterpriseListResponse<ProductKnowledgeDocumentFile>>(`/products/${productId}/knowledge-documents`, params)
}

export async function uploadProductKnowledgeDocument(
  productId: number,
  file: File
): Promise<ApiResponse<ProductKnowledgeDocumentFile>> {
  const formData = new FormData()
  formData.append("file", file)
	return apiPost<ProductKnowledgeDocumentFile>(`/products/${productId}/knowledge-documents/_upload`, formData)
}

export async function reprocessProductKnowledgeDocument(
  productId: number,
  documentId: number
): Promise<ApiResponse<ProductKnowledgeDocumentFile>> {
  return apiPost<ProductKnowledgeDocumentFile>(`/products/${productId}/knowledge-documents/${documentId}/_reprocess`)
}

export async function deleteProductKnowledgeDocument(
  productId: number,
  documentId: number
): Promise<ApiResponse<{ success: boolean }>> {
  return apiDelete<{ success: boolean }>(`/products/${productId}/knowledge-documents/${documentId}`)
}

export async function linkProductManual(
  productId: number,
  payload: LinkProductManualPayload
): Promise<ApiResponse<ProductManualLink>> {
  return apiPost<ProductManualLink>(`/products/${productId}/manuals/_link`, {
    knowledge_entry_id: payload.knowledge_entry_id ?? 0,
    knowledge_base_id: payload.knowledge_base_id ?? 0,
    product_model_id: payload.product_model_id ?? 0,
    link_type: payload.link_type ?? "manual",
    language: payload.language ?? "",
    version: payload.version ?? "",
    visibility: payload.visibility ?? "internal",
    sort_no: payload.sort_no ?? 0,
  })
}

export async function updateProductManual(
  productId: number,
  linkId: number,
  payload: UpdateProductManualPayload
): Promise<ApiResponse<ProductManualLink>> {
  return apiPatch<ProductManualLink>(`/products/${productId}/manuals/${linkId}`, payload)
}

export async function deleteProductManual(
  productId: number,
  linkId: number
): Promise<ApiResponse<{ success: boolean }>> {
  return apiDelete<{ success: boolean }>(`/products/${productId}/manuals/${linkId}`)
}

export async function publishProductManual(
  productId: number,
  linkId: number
): Promise<ApiResponse<ProductManualLink>> {
  return apiPost<ProductManualLink>(`/products/${productId}/manuals/${linkId}/_publish`)
}

export async function deprecateProductManual(
  productId: number,
  linkId: number
): Promise<ApiResponse<ProductManualLink>> {
  return apiPost<ProductManualLink>(`/products/${productId}/manuals/${linkId}/_deprecate`)
}

export async function reindexProductManual(
  productId: number,
  linkId: number
): Promise<ApiResponse<ProductManualLink>> {
  return apiPost<ProductManualLink>(`/products/${productId}/manuals/${linkId}/_reindex`)
}

export interface ProductResourceQuery {
  page?: number
  page_size?: number
  search?: string
}

export interface ProductResourcePage<T> {
  items: T[]
  total: number
  page: number
  page_size: number
  total_pages: number
  has_more: boolean
}

function productResourceParams(query?: ProductResourceQuery): Record<string, unknown> | undefined {
  return query ? { ...query } : undefined
}

export async function getProductDevices(
  productId: number,
  query?: ProductResourceQuery
): Promise<ApiResponse<ProductResourcePage<ProductDevice>>> {
  return apiGet<ProductResourcePage<ProductDevice>>(`/products/${productId}/devices`, productResourceParams(query))
}

export async function getProductServiceCodes(
  productId: number,
  query?: ProductResourceQuery
): Promise<ApiResponse<ProductResourcePage<ProductServiceCode>>> {
  return apiGet<ProductResourcePage<ProductServiceCode>>(`/products/${productId}/service-codes`, productResourceParams(query))
}

export async function getProductTickets(
  productId: number,
  query?: ProductResourceQuery
): Promise<ApiResponse<ProductResourcePage<ProductTicketBrief>>> {
  return apiGet<ProductResourcePage<ProductTicketBrief>>(`/products/${productId}/tickets`, productResourceParams(query))
}

export async function getProductConversations(
  productId: number,
  query?: ProductResourceQuery
): Promise<ApiResponse<ProductResourcePage<ProductConversationBrief>>> {
  return apiGet<ProductResourcePage<ProductConversationBrief>>(`/products/${productId}/conversations`, productResourceParams(query))
}

export async function getProductRepairHistory(
  productId: number,
  query?: ProductResourceQuery
): Promise<ApiResponse<ProductResourcePage<ProductRepairRecord>>> {
  return apiGet<ProductResourcePage<ProductRepairRecord>>(`/products/${productId}/repair-history`, productResourceParams(query))
}

export async function getProductKnowledgeCoverage(
  productId: number
): Promise<ApiResponse<ProductKnowledgeCoverageDetail>> {
  return apiGet<ProductKnowledgeCoverageDetail>(`/products/${productId}/knowledge-coverage`)
}

export async function getProductQualitySignals(
  productId: number
): Promise<ApiResponse<ProductQualitySignal[]>> {
  return apiGet<ProductQualitySignal[]>(`/products/${productId}/quality-signals`)
}

export async function createQualitySignal(
  productId: number,
  payload: CreateQualitySignalPayload
): Promise<ApiResponse<ProductQualitySignal>> {
  return apiPost<ProductQualitySignal>(`/products/${productId}/quality-signals`, payload)
}

export async function updateQualitySignal(
  productId: number,
  signalId: number,
  payload: UpdateQualitySignalPayload
): Promise<ApiResponse<ProductQualitySignal>> {
  return apiPatch<ProductQualitySignal>(`/products/${productId}/quality-signals/${signalId}`, payload)
}

export async function resolveQualitySignal(
  productId: number,
  signalId: number,
  resolution?: string
): Promise<ApiResponse<ProductQualitySignal>> {
  return apiPost<ProductQualitySignal>(
    `/products/${productId}/quality-signals/${signalId}/_resolve`,
    { resolution: resolution || "" }
  )
}

export async function getProductUsage(
  productId: number
): Promise<ApiResponse<ProductUsageMetric[]>> {
  return apiGet<ProductUsageMetric[]>(`/products/${productId}/usage`)
}
