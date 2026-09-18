import { apiGet, apiPut } from "@/lib/api/client"
import type { ApiResponse } from "@/lib/api/types"

export interface KnowledgeAccessGrant {
  id: number
  subject_type: "user" | "team"
  subject_id: number
  subject_name: string
  access_level: "view" | "operate"
  note: string
}

export type KnowledgeAccessGrantInput = Pick<
  KnowledgeAccessGrant,
  "subject_type" | "subject_id" | "access_level" | "note"
>

export function getKnowledgeAccessGrants(
  knowledgeBaseId: number,
): Promise<ApiResponse<KnowledgeAccessGrant[]>> {
  return apiGet<KnowledgeAccessGrant[]>(`/knowledge-bases/${knowledgeBaseId}/access-grants`)
}

export function replaceKnowledgeAccessGrants(
  knowledgeBaseId: number,
  grants: KnowledgeAccessGrantInput[],
): Promise<ApiResponse<KnowledgeAccessGrant[]>> {
  return apiPut<KnowledgeAccessGrant[]>(`/knowledge-bases/${knowledgeBaseId}/access-grants`, {
    grants,
  })
}
