// ============================================================
// Enterprise Device API
// ============================================================
// GET  /api/enterprise/v1/devices              — list devices
// GET  /api/enterprise/v1/devices/{id}         — device detail
// POST /api/enterprise/v1/devices              — create device
// POST /api/enterprise/v1/devices/{id}         — update device
// POST /api/enterprise/v1/devices/_batch-import — batch import devices
// POST /api/enterprise/v1/devices/_transfer    — transfer ownership
// ============================================================

import { apiGet, apiPost, listQueryToParams, buildFilterQuery } from "@/lib/api/client"
import type {
  ApiResponse,
  BatchImportDevicePayload,
  EnterpriseListResponse,
  ListQuery,
  DeviceListItem,
  DeviceDetailDTO,
  DeviceBatchImportResult,
  CreateDevicePayload,
  UpdateDevicePayload,
  TransferDevicePayload,
} from "@/lib/api/types"

export interface DeviceListQuery extends ListQuery {
  status?: string
  product_id?: number
  search?: string
}

export async function fetchDevices(query?: DeviceListQuery): Promise<ApiResponse<EnterpriseListResponse<DeviceListItem>>> {
  const params: Record<string, unknown> = {
    ...listQueryToParams(query || {}),
  }
  if (query?.status) params["status"] = query.status
  if (query?.product_id) params["product_id"] = query.product_id
  if (query?.search) {
    params["search"] = query.search
    const filter = buildFilterQuery({ device_no: `~${query.search}` })
    if (filter) params["filter"] = filter
  }
  return apiGet<EnterpriseListResponse<DeviceListItem>>("/devices", params)
}

export async function fetchDevice(id: number): Promise<ApiResponse<DeviceDetailDTO>> {
  return apiGet<DeviceDetailDTO>(`/devices/${id}`)
}

export async function createDevice(payload: CreateDevicePayload): Promise<ApiResponse<DeviceListItem>> {
  return apiPost<DeviceListItem>("/devices", payload)
}

export async function updateDevice(id: number, payload: UpdateDevicePayload): Promise<ApiResponse<DeviceDetailDTO>> {
  return apiPost<DeviceDetailDTO>(`/devices/${id}`, payload)
}

export async function batchImportDevices(
  devices: BatchImportDevicePayload[]
): Promise<ApiResponse<DeviceBatchImportResult>> {
  return apiPost<DeviceBatchImportResult>("/devices/_batch-import", { devices })
}

export async function transferDevice(payload: TransferDevicePayload): Promise<ApiResponse<void>> {
  return apiPost<void>("/devices/_transfer", payload)
}
