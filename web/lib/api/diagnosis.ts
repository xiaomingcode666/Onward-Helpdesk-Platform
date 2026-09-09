// ============================================================
// Enterprise / Customer Diagnosis API
// ============================================================
// Enterprise:
//   POST /api/enterprise/v1/diagnosis/start   — start diagnosis
//   POST /api/enterprise/v1/diagnosis/{id}/step  — submit step result
//   GET  /api/enterprise/v1/diagnosis/{id}/summary  — get summary
// ============================================================

import { apiGet, apiPatch, apiPost } from "@/lib/api/client"
import type {
  ApiResponse,
  DiagnosisStepResponseDTO,
  StartDiagnosisPayload,
  StartDiagnosisResponseDTO,
  StepDiagnosisPayload,
  DiagnosisSummaryDTO,
} from "@/lib/api/types"

export type FaultTreeNodeType = "symptom" | "check" | "action" | "diagnosis"
export type FaultTreeRiskLevel = "low" | "medium" | "high" | "critical"
export type FaultTreeStatus = "draft" | "published" | "archived"

export interface EnterpriseFaultTreeNode {
  id: string
  tenant_id: number
  product_id: number
  parent_id: string
  title: string
  description: string
  node_type: FaultTreeNodeType
  fault_pattern: "persistent" | "intermittent" | "conditional"
  trigger_conditions: string
  is_composite: boolean
  risk_level: FaultTreeRiskLevel
  order_index: number
  status: FaultTreeStatus
  created_at: string
  updated_at: string
}

export interface CreateFaultTreeNodePayload {
  product_id: number
  parent_id?: string
  title: string
  description?: string
  node_type: FaultTreeNodeType
  fault_pattern?: EnterpriseFaultTreeNode["fault_pattern"]
  trigger_conditions?: string
  is_composite?: boolean
  risk_level?: FaultTreeRiskLevel
  order_index?: number
}

export type UpdateFaultTreeNodePayload = Partial<Omit<CreateFaultTreeNodePayload, "product_id">> & {
  status?: FaultTreeStatus
}

function normalizeStartDiagnosisPayload(payload: StartDiagnosisPayload) {
  return {
    ...payload,
    ticket_id: payload.ticket_id != null ? String(payload.ticket_id) : undefined,
    device_id: payload.device_id != null ? String(payload.device_id) : undefined,
    product_id: String(payload.product_id),
  }
}

export async function startDiagnosis(payload: StartDiagnosisPayload): Promise<ApiResponse<StartDiagnosisResponseDTO>> {
  return apiPost<StartDiagnosisResponseDTO>("/diagnosis/start", normalizeStartDiagnosisPayload(payload))
}

export async function submitDiagnosisStep(diagnosisId: number | string, payload: StepDiagnosisPayload): Promise<ApiResponse<DiagnosisStepResponseDTO>> {
  return apiPost<DiagnosisStepResponseDTO>(`/diagnosis/${diagnosisId}/step`, payload)
}

export async function fetchDiagnosisSummary(diagnosisId: number | string): Promise<ApiResponse<DiagnosisSummaryDTO>> {
  return apiGet<DiagnosisSummaryDTO>(`/diagnosis/${diagnosisId}/summary`)
}

export async function listFaultTreeNodes(productId: number): Promise<ApiResponse<EnterpriseFaultTreeNode[]>> {
  return apiGet<EnterpriseFaultTreeNode[]>("/diagnosis/fault-tree-nodes", { product_id: productId })
}

export async function createFaultTreeNode(payload: CreateFaultTreeNodePayload): Promise<ApiResponse<EnterpriseFaultTreeNode>> {
  return apiPost<EnterpriseFaultTreeNode>("/diagnosis/fault-tree-nodes", payload)
}

export async function updateFaultTreeNode(
  nodeId: string,
  payload: UpdateFaultTreeNodePayload,
): Promise<ApiResponse<EnterpriseFaultTreeNode>> {
  return apiPatch<EnterpriseFaultTreeNode>(`/diagnosis/fault-tree-nodes/${nodeId}`, payload)
}
