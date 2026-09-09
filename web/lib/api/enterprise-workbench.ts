import { apiGet } from "@/lib/api/client"
import type {
  ApiResponse,
  EnterpriseWorkbenchCollaboration,
  EnterpriseWorkbenchCore,
  EnterpriseWorkbenchQueue,
  EnterpriseWorkbenchResources,
} from "@/lib/api/types"

export async function fetchEnterpriseWorkbenchCore(): Promise<ApiResponse<EnterpriseWorkbenchCore>> {
  return apiGet<EnterpriseWorkbenchCore>("/workbench/core")
}

export async function fetchEnterpriseWorkbenchCollaboration(): Promise<ApiResponse<EnterpriseWorkbenchCollaboration>> {
  return apiGet<EnterpriseWorkbenchCollaboration>("/workbench/collaboration")
}

export async function fetchEnterpriseWorkbenchResources(): Promise<ApiResponse<EnterpriseWorkbenchResources>> {
  return apiGet<EnterpriseWorkbenchResources>("/workbench/resources")
}

export async function fetchEnterpriseWorkbenchQueue(
  query: number | { page?: number; pageSize?: number; page_size?: number; limit?: number; queueKey?: string; queue_key?: string } = 50,
): Promise<ApiResponse<EnterpriseWorkbenchQueue>> {
  if (typeof query === "number") {
    return apiGet<EnterpriseWorkbenchQueue>("/workbench/queue", { limit: query })
  }
  return apiGet<EnterpriseWorkbenchQueue>("/workbench/queue", {
    page: query.page,
    page_size: query.page_size ?? query.pageSize,
    queue_key: query.queue_key ?? query.queueKey,
    limit: query.limit,
  })
}
