import { apiGet, apiPost } from "@/lib/api/client"
import type { ApiResponse } from "@/lib/api/types"

export type NotificationTemplateApprovalStatus = "draft" | "approved" | "retired"

export interface NotificationTemplate {
  id: number
  tenant_id: number
  code: string
  name: string
  channel: string
  language: string
  title_template: string
  content_template: string
  variables: string[]
  approval_status: NotificationTemplateApprovalStatus
  approved_by: number
  approved_at: string
  source: "platform" | "tenant"
  editable: boolean
  updated_at: string
}

export interface NotificationTemplateListResponse {
  items: NotificationTemplate[]
  total: number
  page: number
  page_size: number
}

export interface NotificationTemplatePayload {
  code: string
  name: string
  channel: string
  language: string
  titleTemplate: string
  contentTemplate: string
  variables?: string[]
}

export interface NotificationTemplatePreview {
  title: string
  content: string
  blocked: boolean
  rule_code: string
  rule_label: string
}

export interface NotificationDeliveryAttempt {
  id: number
  tenant_id: number
  delivery_id: number
  notification_id: number
  attempt_no: number
  channel: string
  status: string
  reason: string
  detail: string
  template_code: string
  language: string
  recipient_id: string
  created_at: string
}

export interface NotificationDeliveryAttemptListResponse {
  items: NotificationDeliveryAttempt[]
  total: number
  page: number
  page_size: number
}

export interface NotificationTemplateQuery {
  code?: string
  channel?: string
  language?: string
  approval_status?: string
  keyword?: string
  page?: number
  page_size?: number
}

export async function fetchNotificationTemplates(
  query?: NotificationTemplateQuery
): Promise<ApiResponse<NotificationTemplateListResponse>> {
  return apiGet<NotificationTemplateListResponse>("/notifications/templates", query ? { ...query } : undefined)
}

export async function createNotificationTemplate(
  payload: NotificationTemplatePayload
): Promise<ApiResponse<NotificationTemplate>> {
  return apiPost<NotificationTemplate>("/notifications/templates", payload)
}

export async function updateNotificationTemplate(
  id: number,
  payload: NotificationTemplatePayload
): Promise<ApiResponse<NotificationTemplate>> {
  return apiPost<NotificationTemplate>(`/notifications/templates/${id}/update`, payload)
}

export async function approveNotificationTemplate(id: number): Promise<ApiResponse<NotificationTemplate>> {
  return apiPost<NotificationTemplate>(`/notifications/templates/${id}/_approve`)
}

export async function retireNotificationTemplate(id: number): Promise<ApiResponse<NotificationTemplate>> {
  return apiPost<NotificationTemplate>(`/notifications/templates/${id}/_retire`)
}

export async function deleteNotificationTemplate(id: number): Promise<ApiResponse<void>> {
  return apiPost<void>(`/notifications/templates/${id}/_delete`)
}

export async function previewNotificationTemplate(
  payload: NotificationTemplatePayload
): Promise<ApiResponse<NotificationTemplatePreview>> {
  return apiPost<NotificationTemplatePreview>("/notifications/templates/_preview", payload)
}

export async function seedNotificationTemplates(): Promise<ApiResponse<{ created: number }>> {
  return apiPost<{ created: number }>("/notifications/templates/_seed-defaults")
}

export async function fetchNotificationDeliveryAttempts(
  query?: { channel?: string; status?: string; page?: number; page_size?: number }
): Promise<ApiResponse<NotificationDeliveryAttemptListResponse>> {
  return apiGet<NotificationDeliveryAttemptListResponse>("/notifications/delivery-attempts", query ? { ...query } : undefined)
}