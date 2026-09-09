// ============================================================
// Knowledge Base API
// ============================================================
// GET    /api/enterprise/v1/knowledge/entries              — list entries
// GET    /api/enterprise/v1/knowledge/entries/{id}         — get entry
// POST   /api/enterprise/v1/knowledge/entries              — create entry
// PATCH  /api/enterprise/v1/knowledge/entries/{id}         — update entry
// POST   /api/enterprise/v1/knowledge/entries/{id}/_submit — submit for review
// GET    /api/enterprise/v1/knowledge/entries/{id}/versions — version history
// ============================================================

import { apiDelete, apiGet, apiPost, apiPatch, listQueryToParams, buildFilterQuery } from "@/lib/api/client"
import type { ProductKnowledgeDocumentFile } from "@/lib/api/enterprise-products"
import type { ApiResponse, EnterpriseListResponse, ListQuery } from "@/lib/api/types"

// ---- Types ----

export type KnowledgeEntryStatus = "draft" | "review" | "published" | "deprecated"

export interface KnowledgeEntry {
  id: number
  knowledge_base_id: number
  title: string
  content: string
  category: string
  status: KnowledgeEntryStatus
  tags: string[]
  related_products: { id: number; name: string; code: string }[]
  fault_codes: string[]
  languages: string[]
  hit_rate: number
  completeness_score: number
  quality_score: number
  value_score: number
  entry_score: number
  review_eligible: boolean
  quality_flags: string[]
  author_name: string
  reviewer_name?: string
  created_at: string
  updated_at: string
  published_at?: string
}

export interface KnowledgeEntryListItem {
  id: number
  knowledge_base_id: number
  title: string
  type: "document" | "faq" | "guide" | "manual"
  category: string
  status: KnowledgeEntryStatus
  tags: string[]
  languages: string[]
  product_scope: string
  relevance_score: number
  quality_score: number
  value_score: number
  entry_score: number
  review_eligible: boolean
  quality_flags: string[]
  hit_rate: number
  author_name: string
  updated_at: string
}

export interface KnowledgeListResponse {
  items: KnowledgeEntryListItem[]
  total: number
  page: number
  page_size: number
  total_pages: number
  has_more: boolean
}

export interface KnowledgeStats {
  total_entries: number
  published_count: number
  review_count: number
  draft_count: number
  deprecated_count: number
  avg_hit_rate: number
}

export interface KnowledgeVersion {
  version: number
  entry_id: number
  title: string
  content: string
  language: string
  change_note: string
  updated_by: string
  updated_at: string
}

export interface KnowledgeQuality {
  completeness_score: number
  hit_rate: number
  positive_feedback: number
  negative_feedback: number
  total_views: number
}

export interface KnowledgeTranslation {
  language: string
  title: string
  content: string
  status: KnowledgeEntryStatus
  updated_at: string
}

export interface KnowledgeCandidate {
  id: number
  product_id: number
  product_model_id: number
  source_type: string
  source_id: string
  ticket_id: number
  title: string
  suggestion: string
  root_cause_summary: string
  solution_summary: string
  knowledge_base_id: number
  knowledge_entry_id: number
  quality_score: number
  value_score: number
  candidate_score: number
  score_breakdown: {
    evidence_completeness: number
    outcome_confidence: number
    context_completeness: number
    content_usability: number
    recurrence_value: number
    impact_value: number
    severity_value: number
    knowledge_gap_value: number
  }
  quality_flags: string[]
  score_version: string
  scored_at: string
  similarity_hash: string
  duplicate_group_id: string
  merged_to_candidate_id: number
  deduplication_override: boolean
  recurrence_count: number
  affected_device_count: number
  requires_reassessment: boolean
  review_eligible: boolean
  review_status: string
  review_remark: string
  reviewed_at: string
  created_at: string
}

export interface ApproveKnowledgeCandidateInput {
  title?: string
  content?: string
  language?: string
  category?: string
  knowledge_base_name?: string
  visibility?: "public" | "internal"
  publish?: boolean
}

export interface EnrichKnowledgeCandidateInput {
  title?: string
  suggestion?: string
  root_cause_summary?: string
  solution_summary?: string
}

export interface KnowledgeIndexTask {
  id: number
  subject_type: string
  subject_id: number
  knowledge_base_id: number
  product_id: number
  provider_type: string
  action: string
  input_version: string
  status: string
  external_job_id: string
  retry_count: number
  max_retries: number
  error_summary: string
  updated_at: string
}

export interface KnowledgeUploadQuota {
  document_limit: number
  document_used: number
  document_remaining: number
  tenant_document_limit: number
  tenant_document_used: number
  tenant_document_remaining: number
  product_document_limit: number
  product_document_used: number
  product_document_remaining: number
  total_file_size: number
  single_file_size_limit: number
  unlimited_documents: boolean
}

export interface KnowledgeQuery extends ListQuery {
  search?: string
  category?: string
  status?: string
  product?: string
  product_id?: number
  tag?: string
}

export interface KnowledgeCandidateQuery {
  product_id?: number
  review_status?: string
  page?: number
  page_size?: number
}

export interface KnowledgeCandidatePage {
  items: KnowledgeCandidate[]
  total: number
  page: number
  page_size: number
  total_pages: number
  has_more: boolean
}

export interface KnowledgeCreateInput {
  title: string
  content: string
  category: string
  type?: "document" | "faq"
  tags: string[]
  related_product_ids?: number[]
  fault_codes?: string[]
  language: string
  translations?: {
    language: string
    title: string
    content: string
  }[]
}

export interface KnowledgeUpdateInput {
  title?: string
  content?: string
  category?: string
  type?: "document" | "faq"
  tags?: string[]
  related_product_ids?: number[]
  fault_codes?: string[]
  language?: string
  translations?: {
    language: string
    title: string
    content: string
  }[]
}

// ---- API functions ----

export async function fetchKnowledgeEntries(
  query?: KnowledgeQuery,
): Promise<ApiResponse<KnowledgeListResponse>> {
  const params: Record<string, unknown> = {
    ...listQueryToParams(query || {}),
  }

  const filters: Record<string, string | string[] | undefined> = {}
  if (query?.status) filters["status"] = query.status
  if (query?.category) filters["category"] = query.category
  if (query?.product) filters["product"] = query.product
  if (query?.tag) filters["tag"] = query.tag

  if (query?.search) {
    const existingFilter = buildFilterQuery(filters)
    params["filter"] = existingFilter
      ? `${existingFilter},title:~${query.search}`
      : `title:~${query.search}`
  } else {
    const existingFilter = buildFilterQuery(filters)
    if (existingFilter) params["filter"] = existingFilter
  }

  if (query?.product_id) {
    params["product_id"] = query.product_id
  }

  return apiGet<KnowledgeListResponse>("/knowledge/entries", params)
}

export async function fetchKnowledgeEntry(
  id: string,
): Promise<ApiResponse<KnowledgeEntry>> {
  return apiGet<KnowledgeEntry>(`/knowledge/entries/${id}`)
}

export async function createKnowledgeEntry(
  data: KnowledgeCreateInput,
): Promise<ApiResponse<KnowledgeEntry>> {
  return apiPost<KnowledgeEntry>("/knowledge/entries", data)
}

export async function updateKnowledgeEntry(
  id: string,
  data: KnowledgeUpdateInput,
): Promise<ApiResponse<KnowledgeEntry>> {
  return apiPatch<KnowledgeEntry>(`/knowledge/entries/${id}`, data)
}

export async function submitForReview(
  id: string,
): Promise<ApiResponse<KnowledgeEntry>> {
  return apiPost<KnowledgeEntry>(`/knowledge/entries/${id}/_submit`)
}

export async function publishEntry(
  id: string,
): Promise<ApiResponse<KnowledgeEntry>> {
  return apiPost<KnowledgeEntry>(`/knowledge/entries/${id}/_publish`)
}

export async function deprecateEntry(
  id: string,
): Promise<ApiResponse<KnowledgeEntry>> {
  return apiPost<KnowledgeEntry>(`/knowledge/entries/${id}/_deprecate`)
}

export async function getEntryVersions(
  id: string,
): Promise<ApiResponse<KnowledgeVersion[]>> {
  return apiGet<KnowledgeVersion[]>(`/knowledge/entries/${id}/versions`)
}

export async function fetchKnowledgeStats(): Promise<ApiResponse<KnowledgeStats>> {
  return apiGet<KnowledgeStats>("/knowledge/stats")
}

export async function fetchEntryQuality(
  id: string,
): Promise<ApiResponse<KnowledgeQuality>> {
  return apiGet<KnowledgeQuality>(`/knowledge/entries/${id}/quality`)
}

export async function fetchEntryTranslations(
  id: string,
): Promise<ApiResponse<KnowledgeTranslation[]>> {
  return apiGet<KnowledgeTranslation[]>(`/knowledge/entries/${id}/translations`)
}

export async function fetchKnowledgeCandidates(
  query?: KnowledgeCandidateQuery,
): Promise<ApiResponse<KnowledgeCandidate[]>> {
  return apiGet<KnowledgeCandidate[]>("/knowledge/candidates", query ? { ...query } : undefined)
}

export async function fetchKnowledgeCandidatePage(
  query?: KnowledgeCandidateQuery,
): Promise<ApiResponse<KnowledgeCandidatePage>> {
  return apiGet<KnowledgeCandidatePage>("/knowledge/candidates/page", query ? { ...query } : undefined)
}

export async function approveKnowledgeCandidate(
  id: number,
  data?: ApproveKnowledgeCandidateInput,
): Promise<ApiResponse<KnowledgeCandidate>> {
  return apiPost<KnowledgeCandidate>(`/knowledge/candidates/${id}/_approve`, data)
}

export async function enrichKnowledgeCandidate(
  id: number,
  data: EnrichKnowledgeCandidateInput,
): Promise<ApiResponse<KnowledgeCandidate>> {
  return apiPost<KnowledgeCandidate>(`/knowledge/candidates/${id}/_enrich`, data)
}

export async function detachDuplicateKnowledgeCandidate(
  id: number,
): Promise<ApiResponse<KnowledgeCandidate>> {
  return apiPost<KnowledgeCandidate>(`/knowledge/candidates/${id}/_detach-duplicate`)
}

export async function rejectKnowledgeCandidate(
  id: number,
  remark?: string,
): Promise<ApiResponse<KnowledgeCandidate>> {
  return apiPost<KnowledgeCandidate>(`/knowledge/candidates/${id}/_reject`, {
    remark: remark || "",
  })
}

export async function mergeKnowledgeCandidate(
  id: number,
  knowledgeEntryId: number,
): Promise<ApiResponse<KnowledgeCandidate>> {
  return apiPost<KnowledgeCandidate>(`/knowledge/candidates/${id}/_merge`, {
    knowledge_entry_id: knowledgeEntryId,
  })
}

export async function fetchKnowledgeIndexTasks(
  productId?: number,
): Promise<ApiResponse<KnowledgeIndexTask[]>> {
  return apiGet<KnowledgeIndexTask[]>("/knowledge/index-tasks", productId ? { product_id: productId } : {})
}

export async function fetchKnowledgeUploadQuota(productId?: number): Promise<ApiResponse<KnowledgeUploadQuota>> {
  return apiGet<KnowledgeUploadQuota>("/knowledge/upload-quota", productId ? { product_id: productId } : {})
}

export async function fetchTenantKnowledgeDocuments(
  knowledgeBaseId: number,
  query?: { page?: number; page_size?: number; limit?: number }
): Promise<ApiResponse<EnterpriseListResponse<ProductKnowledgeDocumentFile>>> {
  const params: Record<string, unknown> = {
    ...listQueryToParams(query || {}),
  }
  if (query?.limit && query.limit > 0 && params.page_size === undefined) {
    params.limit = query.limit
  }
  return apiGet<EnterpriseListResponse<ProductKnowledgeDocumentFile>>(`/knowledge-bases/${knowledgeBaseId}/documents`, params)
}

export async function uploadTenantKnowledgeDocument(
  knowledgeBaseId: number,
  file: File,
): Promise<ApiResponse<ProductKnowledgeDocumentFile>> {
  const formData = new FormData()
  formData.append("file", file)
  return apiPost<ProductKnowledgeDocumentFile>(`/knowledge-bases/${knowledgeBaseId}/documents/_upload`, formData)
}

export async function reprocessTenantKnowledgeDocument(
  knowledgeBaseId: number,
  documentId: number,
): Promise<ApiResponse<ProductKnowledgeDocumentFile>> {
  return apiPost<ProductKnowledgeDocumentFile>(`/knowledge-bases/${knowledgeBaseId}/documents/${documentId}/_reprocess`)
}

export async function deleteTenantKnowledgeDocument(
  knowledgeBaseId: number,
  documentId: number,
): Promise<ApiResponse<{ success: boolean }>> {
  return apiDelete<{ success: boolean }>(`/knowledge-bases/${knowledgeBaseId}/documents/${documentId}`)
}

export async function retryKnowledgeIndexTask(
  id: number,
): Promise<ApiResponse<{ id: number; status: string }>> {
  return apiPost<{ id: number; status: string }>(`/knowledge/index-tasks/${id}/_retry`)
}
