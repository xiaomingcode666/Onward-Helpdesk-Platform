import { apiGet, apiPost } from "@/lib/api/client"
import type {
  ApiResponse,
  EnterpriseNotificationListResponse,
  NotificationMailSetting,
  NotificationRecipientSetting,
  UpdateNotificationMailSettingPayload,
  UpdateNotificationRecipientSettingPayload,
} from "@/lib/api/types"

export interface EnterpriseNotificationQuery {
  read_status?: "read" | "unread" | "all"
  category?: string
  search?: string
  scope?: "personal" | "tenant"
  page?: number
  page_size?: number
  limit?: number
}

export async function fetchEnterpriseNotifications(
  query?: EnterpriseNotificationQuery
): Promise<ApiResponse<EnterpriseNotificationListResponse>> {
  return apiGet<EnterpriseNotificationListResponse>("/notifications", query ? { ...query } : undefined)
}

export async function markEnterpriseNotificationRead(id: number): Promise<ApiResponse<void>> {
  return apiPost<void>(`/notifications/${id}/_read`)
}

export async function markAllEnterpriseNotificationsRead(): Promise<ApiResponse<void>> {
  return apiPost<void>("/notifications/_mark_all_read")
}

export async function fetchNotificationMailSettings(): Promise<ApiResponse<NotificationMailSetting>> {
  return apiGet<NotificationMailSetting>("/notifications/mail-settings")
}

export async function updateNotificationMailSettings(
  payload: UpdateNotificationMailSettingPayload
): Promise<ApiResponse<NotificationMailSetting>> {
  return apiPost<NotificationMailSetting>("/notifications/mail-settings", payload)
}

export async function sendNotificationTestMail(to: string): Promise<ApiResponse<void>> {
  return apiPost<void>("/notifications/mail-settings/_test", { to })
}

export async function fetchNotificationRecipientSetting(): Promise<ApiResponse<NotificationRecipientSetting>> {
  return apiGet<NotificationRecipientSetting>("/notifications/preferences")
}

export async function updateNotificationRecipientSetting(
  payload: UpdateNotificationRecipientSettingPayload
): Promise<ApiResponse<NotificationRecipientSetting>> {
  return apiPost<NotificationRecipientSetting>("/notifications/preferences", payload)
}

export async function sendNotificationRecipientTestMail(): Promise<ApiResponse<void>> {
  return apiPost<void>("/notifications/preferences/_test")
}
