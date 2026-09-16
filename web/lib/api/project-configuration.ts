import { apiGet, apiPost } from "./client"
import type { TicketIntakePolicy } from "../ticket-intake"

export type ProjectConfiguration = {
  ticket_workflows?: Record<string, { transitions: Record<string, string[]> }>
  schema_version: number
  tenant_id: number
  environment: string
  projects: Array<{ key: string; name: string }>
  intake: TicketIntakePolicy
  secret_refs: string[]
  runtime?: ProjectRuntime
}
export type ProjectRuntime = {
  timezone: string; locale: string; locales: string[]; service_scene: string
  calendars: Array<{ key: string; timezone: string; work_days: number[]; start: string; end: string; holidays: string[] }>
  targets: Array<{ project_key: string; profile: string; priority: string; calendar_key: string; response_minutes: number; assignment_minutes: number; resolution_minutes: number }>
  channels: Array<{ name: string; enabled: boolean }>
  mail: { enabled: boolean; host: string; port: number; username: string; password_ref: string; from_address: string; from_name: string; use_tls: boolean; reply_to: string; retry_policy: string; imap?: { enabled: boolean; host: string; port: number; username: string; password_ref: string; project_key: string; ticket_type: string } }
  integrations: Array<{ provider: string; enabled: boolean; base_url: string; app_id: string; secret_ref: string; key_ref: string; metadata_json: string }>
  retention: { data_region: string; days: number; archive_after_days: number; auto_delete: boolean; legal_hold: boolean; gdpr_region: boolean; ccpa_region: boolean }
  auto_close: { enabled: boolean; days: number }
}
export type ConfigurationVersion = {
  id: number
  base_version_id: number
  digest: string
  note: string
  created_by: number
  created_by_name?: string
  created_at: string
  document: ProjectConfiguration
  activation?: { applied_by: number; applied_by_name?: string; applied_at: string; previous_version_id: number }
}
export type ConfigurationView = {
	deployment_managed: boolean
	next_before_id: number
  active_version_id: number
  document: ProjectConfiguration
  versions: ConfigurationVersion[]
}
export type ConfigurationReport = { valid: boolean; issues: Array<{ kind: string; path: string; message: string }> }
export const getProjectConfiguration = (beforeId?: number) => apiGet<ConfigurationView>("/ticket-settings/configuration", beforeId ? { before_id: beforeId } : undefined)
export const upgradeProjectConfiguration = () => apiGet<ProjectConfiguration>("/ticket-settings/configuration/upgrade")
export const validateProjectConfiguration = (document: ProjectConfiguration) => apiPost<ConfigurationReport>("/ticket-settings/configuration/validate", document)
export const saveProjectConfiguration = (document: ProjectConfiguration, baseVersion: number, note: string, requestKey: string) =>
  apiPost<ConfigurationVersion>("/ticket-settings/configuration/drafts", { document, base_version_id: baseVersion, note, request_key: requestKey })
export const applyProjectConfiguration = (id: number) => apiPost<ConfigurationVersion>(`/ticket-settings/configuration/${id}/apply`, {})
