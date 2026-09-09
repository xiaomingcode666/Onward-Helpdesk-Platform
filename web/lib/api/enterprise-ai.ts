import { apiGet, request } from "@/lib/api/client"
import type {
  AIAgent,
  AIWorkflow,
  AIWorkflowDefinition,
  AIWorkflowTestRunResult,
  AIWorkflowVersion,
  PageResult,
} from "@/lib/api/admin"

export type AIAgentRelease = {
  id: number
  tenantId: number
  productId: number
  agentId: number
  releaseNo: number
  workflowId: number
  workflowVersionId: number
  workflowDefinitionHash: string
  agentConfigHash: string
  knowledgeScopeHash: string
  reviewStatus: "unreviewed" | "pending" | "approved" | "rejected" | string
  reviewStatusName: string
  reviewComment: string
  reviewedAt: string
  reviewedByName: string
  deploymentStatus: "inactive" | "active" | "retired" | "rolled_back" | string
  deployedAt: string
  deployedByName: string
  rollbackFromReleaseId: number
  status: number
  createdAt: string
  updatedAt: string
}

export type ProductAgentProvisionFailure = {
  productId: number
  productName: string
  reason: string
}

export type ProductAgentProvisionResult = {
  total: number
  created: number
  existing: number
  failed: number
  failures: ProductAgentProvisionFailure[]
}

export type EnterpriseAIAgentSummary = {
  total: number
  active: number
  notDeployed: number
  productTotal: number
  productAgentTotal: number
  missingProductAgents: number
  tenantDefaultReady: boolean
  productAgentProductIds: number[]
}

export type EnterpriseAIAgentListQuery = {
  page?: number
  limit?: number
  search?: string
  productId?: number
  productScoped?: number
  workflowId?: number
  reviewStatus?: string
  runtimeStatus?: string
  source?: string
}

export function fetchEnterpriseAIAgentSummary() {
  return apiGet<EnterpriseAIAgentSummary>("/ai-agents/summary")
}

export function fetchEnterpriseAIAgents(query?: EnterpriseAIAgentListQuery) {
  return apiGet<PageResult<AIAgent>>("/ai-agents", query)
}

export function provisionProductAIAgents() {
  return request<ProductAgentProvisionResult>(
    "/api/enterprise/v1/ai-agents/_provision-products",
    { method: "POST" },
  )
}

export function ensureProductAIAgent(productId: number) {
  return request<{ id: number; productId: number }>("/api/enterprise/v1/ai-agents/_ensure-product", {
    method: "POST",
    body: JSON.stringify({ productId }),
  })
}

export function ensureTenantDefaultAIAgent() {
  return request<{ id: number; productId: 0 }>("/api/enterprise/v1/ai-agents/_ensure-tenant-default", {
    method: "POST",
  })
}

export function submitProductAIAgentReview(id: number) {
  return request<void>(`/api/enterprise/v1/ai-agents/${id}/_submit-review`, {
    method: "POST",
  })
}

export function approveProductAIAgent(id: number, comment = "") {
  return request<void>(`/api/enterprise/v1/ai-agents/${id}/_approve`, {
    method: "POST",
    body: JSON.stringify({ comment }),
  })
}

export function rejectProductAIAgent(id: number, comment: string) {
  return request<void>(`/api/enterprise/v1/ai-agents/${id}/_reject`, {
    method: "POST",
    body: JSON.stringify({ comment }),
  })
}

export function fetchAIAgentReleases(agentId: number) {
  return request<AIAgentRelease[]>(`/api/enterprise/v1/ai-agents/${agentId}/releases`)
}

export function createAIAgentRelease(agentId: number) {
  return request<AIAgentRelease>(`/api/enterprise/v1/ai-agents/${agentId}/releases`, {
    method: "POST",
  })
}

export function fetchAIAgentRelease(releaseId: number) {
  return request<AIAgentRelease>(`/api/enterprise/v1/ai-agent-releases/${releaseId}`)
}

export function submitAIAgentReleaseReview(releaseId: number) {
  return request<void>(`/api/enterprise/v1/ai-agent-releases/${releaseId}/_submit-review`, {
    method: "POST",
  })
}

export function approveAIAgentRelease(releaseId: number, comment = "") {
  return request<void>(`/api/enterprise/v1/ai-agent-releases/${releaseId}/_approve`, {
    method: "POST",
    body: JSON.stringify({ comment }),
  })
}

export function rejectAIAgentRelease(releaseId: number, comment: string) {
  return request<void>(`/api/enterprise/v1/ai-agent-releases/${releaseId}/_reject`, {
    method: "POST",
    body: JSON.stringify({ comment }),
  })
}

export function deployAIAgentRelease(releaseId: number) {
  return request<void>(`/api/enterprise/v1/ai-agent-releases/${releaseId}/_deploy`, {
    method: "POST",
  })
}

export function rollbackAIAgentRelease(releaseId: number) {
  return request<void>(`/api/enterprise/v1/ai-agent-releases/${releaseId}/_rollback`, {
    method: "POST",
  })
}

export function fetchEnterpriseAIWorkflows(query?: Record<string, string | number | undefined>) {
  const search = new URLSearchParams()
  Object.entries(query ?? {}).forEach(([key, value]) => {
    if (value !== undefined && value !== "") search.set(key, String(value))
  })
  const suffix = search.size > 0 ? `?${search.toString()}` : ""
  return request<PageResult<AIWorkflow>>(`/api/enterprise/v1/ai-workflows${suffix}`)
}

export type EnterpriseAIWorkflowSummary = {
  total: number
  adopted: number
  attention: number
  platformTemplates: AIWorkflow[]
  stableVersionByWorkflow: Record<string, number>
}

export type EnterpriseAIWorkflowAdoption = {
  adoption: Record<string, number>
  stableAdoption: Record<string, number>
  stableVersionByWorkflow: Record<string, number>
}

export function fetchEnterpriseAIWorkflowSummary() {
  return request<EnterpriseAIWorkflowSummary>("/api/enterprise/v1/ai-workflows/summary")
}

export function fetchEnterpriseAIWorkflowAdoption(workflowIds: number[]) {
  const search = new URLSearchParams()
  if (workflowIds.length > 0) search.set("workflow_ids", workflowIds.join(","))
  const suffix = search.size > 0 ? `?${search.toString()}` : ""
  return request<EnterpriseAIWorkflowAdoption>(`/api/enterprise/v1/ai-workflows/adoption${suffix}`)
}

export function fetchEnterpriseAIWorkflow(id: number) {
  return request<AIWorkflow>(`/api/enterprise/v1/ai-workflows/${id}`)
}

export function fetchEnterpriseAIWorkflowVersions(
  id: number,
  query?: Record<string, string | number | undefined>,
) {
  const search = new URLSearchParams()
  Object.entries(query ?? {}).forEach(([key, value]) => {
    if (value !== undefined && value !== "") search.set(key, String(value))
  })
  const suffix = search.size > 0 ? `?${search.toString()}` : ""
  return request<PageResult<AIWorkflowVersion>>(
    `/api/enterprise/v1/ai-workflows/${id}/versions${suffix}`,
  )
}

export function createEnterpriseAIWorkflowTemplate(payload: {
  name: string
  description?: string
  sourceVersionId: number
}) {
  return request<AIWorkflow>("/api/enterprise/v1/ai-workflows", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updateEnterpriseAIWorkflowDraft(
  id: number,
  payload: { name: string; description: string; definition: AIWorkflowDefinition },
) {
  return request<AIWorkflow>(`/api/enterprise/v1/ai-workflows/${id}/draft`, {
    method: "PATCH",
    body: JSON.stringify(payload),
  })
}

export function deleteEnterpriseAIWorkflow(id: number) {
  return request<void>(`/api/enterprise/v1/ai-workflows/${id}`, {
    method: "DELETE",
  })
}

export function publishEnterpriseAIWorkflow(
  id: number,
  payload: { name: string; description: string; definition: AIWorkflowDefinition },
) {
  return request<AIWorkflowVersion>(`/api/enterprise/v1/ai-workflows/${id}/_publish`, {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function rollbackEnterpriseAIWorkflowVersion(workflowId: number, versionId: number) {
  return request<AIWorkflowVersion>(`/api/enterprise/v1/ai-workflows/${workflowId}/versions/${versionId}/_rollback`, {
    method: "POST",
  })
}

export function testEnterpriseAIWorkflow(
  id: number,
  payload: {
    aiAgentId: number
    definition: AIWorkflowDefinition
    userMessage: string
    autoConfirm: boolean
    branchOverrides: Record<string, string>
    runtimeContext: {
      productModelId?: number
      deviceId?: number
      serviceCodeId?: number
      customerEntrySessionId?: number
      deviceBound?: boolean
      serviceMode?: "ai_only" | "human_only" | "ai_first"
    }
  },
) {
  return request<AIWorkflowTestRunResult>(`/api/enterprise/v1/ai-workflows/${id}/_test`, {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function bindAIAgentWorkflowVersion(agentId: number, workflowVersionId: number) {
  return request<void>(`/api/enterprise/v1/ai-agents/${agentId}/workflow-binding`, {
    method: "PATCH",
    body: JSON.stringify({ workflowVersionId }),
  })
}
