import { apiGet, apiPost } from "./client"

export type ProjectConfiguration = {
  ticket_workflows?: Record<string, { transitions: Record<string, string[]> }>
  schema_version: number
  tenant_id: number
  environment: string
  projects: Array<{ key: string; name: string; profile?: string }>
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
  retention: { data_region: string; days: number; archive_after_days: number; auto_delete: boolean; legal_hold: boolean; gdpr_region: boolean; ccpa_region: boolean; ticket_days?: number; attachment_days?: number; event_days?: number; audit_days?: number; metric_days?: number; log_days?: number; report_days?: number; backup_days?: number }
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
export type ConfigurationImpactItem = { ticket_id: number; ticket_no: string; title: string; metric: string; old_due_at?: string; new_due_at?: string; delta_minutes: number; old_breached: boolean; new_breached: boolean; reason: string }
export type ConfigurationImpact = { summary: { scanned: number; affected: number; newly_at_risk: number; newly_breached: number; shortened: number; extended: number }; items: ConfigurationImpactItem[] }
export type RetentionApproval = { id: number; project_key: string; version_id: number; policy_digest: string; status: string; submitted_by: number; submitted_by_name?: string; submitted_at: string; reviewed_by: number; reviewed_by_name?: string; reviewed_at?: string; review_comment?: string; can_approve?: boolean }
export const getProjectConfiguration = (beforeId?: number) => apiGet<ConfigurationView>("/ticket-settings/configuration", beforeId ? { before_id: beforeId } : undefined)
export const upgradeProjectConfiguration = () => apiGet<ProjectConfiguration>("/ticket-settings/configuration/upgrade")
export const validateProjectConfiguration = (document: ProjectConfiguration) => apiPost<ConfigurationReport>("/ticket-settings/configuration/validate", document)
export const previewProjectConfigurationImpact = (document: ProjectConfiguration, limit = 100) => apiPost<ConfigurationImpact>("/ticket-settings/configuration/impact-preview", { document, limit })
export const saveProjectConfiguration = (document: ProjectConfiguration, baseVersion: number, note: string, requestKey: string) =>
  apiPost<ConfigurationVersion>("/ticket-settings/configuration/drafts", { document, base_version_id: baseVersion, note, request_key: requestKey })
export const applyProjectConfiguration = (id: number) => apiPost<ConfigurationVersion>(`/ticket-settings/configuration/${id}/apply`, {})
export const listRetentionApprovals = (versionId: number) => apiGet<RetentionApproval[]>("/ticket-settings/configuration/retention", { version_id: versionId })
export const submitRetentionApproval = (versionId: number) => apiPost<RetentionApproval[]>(`/ticket-settings/configuration/${versionId}/retention/submit`, {})
export const reviewRetentionApproval = (approvalId: number, approved: boolean, comment: string) => apiPost<RetentionApproval>(`/ticket-settings/configuration/retention/${approvalId}/review`, { approved, comment })
