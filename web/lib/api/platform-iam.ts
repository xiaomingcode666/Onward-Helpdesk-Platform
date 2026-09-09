import { request, requestBlob } from "@/lib/api/client"
import type { AuthSession } from "@/lib/auth"

export interface IAMPageInfo {
  page: number
  limit: number
  total: number
}

export interface IAMPageResult<T> {
  results: T[]
  page: IAMPageInfo
}

export interface PlatformStaff {
  id: number
  userId: number
  username: string
  displayName: string
  email: string
  mobile: string
  teamCode: string
  jobTitle: string
  supportLevel: string
  employmentType: string
  status: number
  createdAt: string
  updatedAt: string
}

export interface IAMRole {
  id: number
  tenantId: number
  domainType: string
  code: string
  name: string
  description: string
  isBuiltin: boolean
  status: number
  sortNo: number
  permissions?: string[]
  createdAt: string
  updatedAt: string
}

export interface IAMAuditLog {
  id: number
  tenantId: number
  domainType: string
  actorUserId: number
  actorSubjectType: string
  actorSubjectId: number
  targetType: string
  targetId: string
  action: string
  summary?: string
  metadata?: Record<string, string>
  requestId: string
  supportGrantId: number
  ipAddress: string
  userAgent: string
  riskLevel: string
  status: string
  occurredAt: string
}

export interface PlatformAuditRetentionSettings {
  retentionDays: number
  defaultRetentionDays: number
  maxRetentionDays: number
  minRetentionDays: number
  cutoffAt: string
  configured: boolean
}

export interface EnterpriseIAMMember {
  id: number
  tenant_id: number
  user_id: number
  username: string
  email: string
  mobile: string
  display_name: string
  member_no: string
  department_id: number
  department_name: string
  job_title: string
  member_type: string
  roles: string[]
  role_names?: string[]
  skill_tags_json: string
  languages_json: string
  service_regions_json: string
  dispatch_enabled: boolean
  product_groups: EnterpriseIAMProductGroup[]
  status: number
  joined_at: string
  updated_at: string
}

export interface EnterpriseIAMProductGroup {
  team_id: number
  team_name: string
  product_id: number
  product_name: string
}

export interface EnterpriseIAMCustomerUser {
  id: number
  tenant_id: number
  customer_org_id: number
  customer_org_name: string
  user_id: number
  display_name: string
  email: string
  phone: string
  locale: string
  timezone: string
  roles: string[]
  status: number
  last_seen_at: string
  updated_at: string
}

export interface EnterpriseIAMPartner {
  id: number
  tenant_id: number
  partner_no: string
  name: string
  partner_type: string
  country_region: string
  contact_name: string
  account_count: number
  contract_count: number
  status: number
  updated_at: string
}

export interface EnterpriseIAMDepartment {
  id: number
  tenant_id: number
  parent_id: number
  department_code: string
  name: string
  path: string
  depth: number
  manager_member_id: number
  manager_name: string
  region_code: string
  member_count: number
  status: number
  updated_at: string
}

export interface IAMListQuery {
  search?: string
  status?: string
  action?: string
  riskLevel?: string
  targetType?: string
  source?: string
  period?: string
  actorId?: number
  page?: number
  limit?: number
}

export interface PlatformStaffInvitePayload {
  username: string
  nickname: string
  email?: string
  mobile?: string
  password: string
  roleCodes: string[]
  teamCode?: string
  jobTitle?: string
  supportLevel?: string
  employmentType?: string
}

export interface PlatformStaffUpdatePayload {
  platformStaffId: number
  nickname?: string
  email?: string
  mobile?: string
  teamCode?: string
  jobTitle?: string
  supportLevel?: string
  employmentType?: string
  status?: number
}

export interface EnterpriseMemberInvitePayload {
  username: string
  displayName: string
  password: string
  email?: string
  mobile?: string
  departmentId?: number
  jobTitle?: string
  memberType?: string
  roleCodes: string[]
  dispatchEnabled?: boolean
}

export interface EnterpriseMemberUpdatePayload {
  displayName?: string
  email?: string
  mobile?: string
  departmentId?: number
  jobTitle?: string
  memberType?: string
  roleCodes?: string[]
  dispatchEnabled?: boolean
  status?: number
}

export interface EnterpriseCustomerAuthorizePayload {
  username: string
  displayName: string
  password: string
  email?: string
  mobile?: string
  customerOrg: string
  roleCodes: string[]
  locale?: string
  timezone?: string
}

export interface EnterpriseCustomerUserUpdatePayload {
  displayName?: string
  email?: string
  mobile?: string
  customerOrgId?: number
  customerOrg?: string
  roleCodes?: string[]
  locale?: string
  timezone?: string
  status?: number
}

export interface EnterpriseCustomerInvitePayload {
  displayName?: string
  email: string
  customerOrg: string
  roleCodes?: string[]
  locale?: string
  timezone?: string
}

export interface EnterpriseCustomerInviteResult {
  grantId: number
  inviteCode: string
  registrationUrl: string
  email: string
  customerOrgId: number
  customerOrgName: string
  expiresAt: string
  emailSent?: boolean
  emailError?: string
}

export interface EnterprisePartnerAdminInvitePayload {
  partnerCompanyId?: number
  partnerNo?: string
  partnerName: string
  partnerType?: string
  countryRegion?: string
  contactName?: string
  username?: string
  displayName?: string
  password?: string
  email: string
  mobile?: string
}

export interface EnterprisePartnerAdminInviteResult {
  partner: EnterpriseIAMPartner
  inviteCode: string
  registrationUrl: string
  email: string
  partnerId: number
  partnerName: string
  expiresAt: string
  emailSent?: boolean
  emailError?: string
}

export interface EnterprisePartnerUpdatePayload {
  partnerNo?: string
  partnerName?: string
  partnerType?: string
  countryRegion?: string
  contactName?: string
  status?: number
}

export interface EnterpriseDepartmentCreatePayload {
  parentId?: number
  name: string
  regionCode?: string
  managerMemberId?: number
}

export interface IAMRoleSavePayload {
  id?: number
  tenantId: number
  domainType: "platform" | "enterprise"
  code: string
  name: string
  description?: string
  isBuiltin?: boolean
  sortNo?: number
  status?: number
}

function queryString(query?: Record<string, string | number | undefined> | IAMListQuery) {
  const params = new URLSearchParams()
  Object.entries(query ?? {}).forEach(([key, value]) => {
    if (value !== undefined && value !== "") {
      params.set(key, String(value))
    }
  })
  const qs = params.toString()
  return qs ? `?${qs}` : ""
}

export function fetchPlatformStaff(query?: IAMListQuery) {
  return request<PlatformStaff[]>(`/api/platform/staff/list${queryString(query)}`)
}

export function invitePlatformStaff(payload: PlatformStaffInvitePayload) {
  return request<PlatformStaff>("/api/platform/staff/invite", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updatePlatformStaff(payload: PlatformStaffUpdatePayload) {
  return request<PlatformStaff>("/api/platform/staff/update", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function deletePlatformStaff(platformStaffId: number, reason = "Delete platform staff") {
  return request<{ success: boolean }>("/api/platform/staff/delete", {
    method: "POST",
    body: JSON.stringify({ platformStaffId, reason }),
  })
}

export function fetchIAMRoles(query: { tenantId?: number; domainType: string }) {
  return request<IAMRole[]>(
    `/api/platform/permission/role/list${queryString({
      tenantId: query.tenantId ?? 0,
      domainType: query.domainType,
    })}`
  )
}

export function saveIAMRole(payload: IAMRoleSavePayload) {
  const enterprise = payload.domainType === "enterprise"
  return request<IAMRole>(enterprise ? "/api/enterprise/v1/iam/roles/save" : "/api/platform/permission/role/save", {
    method: "POST",
    body: JSON.stringify(payload),
    tenantId: enterprise ? payload.tenantId : undefined,
  })
}

export function deleteIAMRole(roleId: number, reason = "Delete platform role") {
  return request<{ success: boolean }>("/api/platform/permission/role/delete", {
    method: "POST",
    body: JSON.stringify({ roleId, reason }),
  })
}

export function saveIAMRolePolicy(roleId: number, permissionCodes: string[], tenantId: number, domainType: "platform" | "enterprise") {
  const enterprise = domainType === "enterprise"
  return request<IAMRole>(enterprise ? "/api/enterprise/v1/iam/policy/save" : "/api/platform/permission/policy/save", {
    method: "POST",
    body: JSON.stringify({ tenantId, roleId, permissionCodes, effect: "allow" }),
    tenantId: enterprise ? tenantId : undefined,
  })
}

export function fetchPlatformAudit(query?: IAMListQuery) {
  return request<IAMPageResult<IAMAuditLog>>(`/api/platform/audit/list${queryString(query)}`)
}

export function downloadPlatformAudit(query?: IAMListQuery) {
  return requestBlob(`/api/platform/audit/export${queryString(query)}`)
}

export function fetchPlatformAuditRetention() {
  return request<PlatformAuditRetentionSettings>("/api/platform/audit/retention")
}

export function updatePlatformAuditRetention(retentionDays: number) {
  return request<PlatformAuditRetentionSettings>("/api/platform/audit/retention", {
    method: "POST",
    body: JSON.stringify({ retentionDays }),
  })
}

export function fetchEnterpriseIAMMembers(query?: IAMListQuery, tenantId?: number) {
  return request<IAMPageResult<EnterpriseIAMMember>>(
    `/api/enterprise/v1/iam/members${queryString(query)}`,
    { tenantId }
  )
}

export function inviteEnterpriseMember(payload: EnterpriseMemberInvitePayload, tenantId?: number) {
  return request<{ member: EnterpriseIAMMember; initialPassword: string }>(
    "/api/enterprise/v1/iam/members/invite",
    { method: "POST", body: JSON.stringify(payload), tenantId }
  )
}

export function updateEnterpriseMember(memberId: number, payload: EnterpriseMemberUpdatePayload, tenantId?: number) {
  return request<EnterpriseIAMMember>(
    `/api/enterprise/v1/iam/members/${memberId}/update`,
    { method: "POST", body: JSON.stringify(payload), tenantId }
  )
}

export function updateEnterpriseMemberStatus(memberId: number, status: number, tenantId?: number, reason = "Update enterprise member status") {
  return request<EnterpriseIAMMember>(
    `/api/enterprise/v1/iam/members/${memberId}/_status`,
    { method: "POST", body: JSON.stringify({ status, reason }), tenantId }
  )
}

export function startEnterpriseMemberPortalSession(memberId: number, tenantId?: number) {
  return request<AuthSession>(
    `/api/enterprise/v1/iam/members/${memberId}/portal-session`,
    { method: "POST", body: "{}", tenantId }
  )
}

export function fetchEnterpriseIAMCustomerUsers(query?: IAMListQuery, tenantId?: number) {
  return request<IAMPageResult<EnterpriseIAMCustomerUser>>(
    `/api/enterprise/v1/iam/customer-users${queryString(query)}`,
    { tenantId }
  )
}

export function authorizeEnterpriseCustomerUser(payload: EnterpriseCustomerAuthorizePayload, tenantId?: number) {
  return request<{ customerUser: EnterpriseIAMCustomerUser; initialPassword: string }>(
    "/api/enterprise/v1/iam/customer-users/authorize",
    { method: "POST", body: JSON.stringify(payload), tenantId }
  )
}

export function inviteEnterpriseCustomerUser(payload: EnterpriseCustomerInvitePayload, tenantId?: number) {
  return request<EnterpriseCustomerInviteResult>(
    "/api/enterprise/v1/iam/customer-users/invite",
    { method: "POST", body: JSON.stringify(payload), tenantId }
  )
}

export function updateEnterpriseCustomerUser(customerUserId: number, payload: EnterpriseCustomerUserUpdatePayload, tenantId?: number) {
  return request<EnterpriseIAMCustomerUser>(
    `/api/enterprise/v1/iam/customer-users/${customerUserId}/update`,
    { method: "POST", body: JSON.stringify(payload), tenantId }
  )
}

export function updateEnterpriseCustomerUserStatus(customerUserId: number, status: number, tenantId?: number, reason = "Update customer user status") {
  return request<EnterpriseIAMCustomerUser>(
    `/api/enterprise/v1/iam/customer-users/${customerUserId}/_status`,
    { method: "POST", body: JSON.stringify({ status, reason }), tenantId }
  )
}

export function startEnterpriseCustomerPortalSession(customerUserId: number, tenantId?: number) {
  return request<AuthSession>(
    `/api/enterprise/v1/iam/customer-users/${customerUserId}/portal-session`,
    { method: "POST", body: "{}", tenantId }
  )
}

export function fetchEnterpriseIAMPartners(query?: IAMListQuery, tenantId?: number) {
  return request<IAMPageResult<EnterpriseIAMPartner>>(
    `/api/enterprise/v1/iam/partners${queryString(query)}`,
    { tenantId }
  )
}

export function inviteEnterprisePartnerAdmin(payload: EnterprisePartnerAdminInvitePayload, tenantId?: number) {
  return request<EnterprisePartnerAdminInviteResult>(
    "/api/enterprise/v1/iam/partners/invite-admin",
    { method: "POST", body: JSON.stringify(payload), tenantId }
  )
}

export function updateEnterprisePartner(partnerId: number, payload: EnterprisePartnerUpdatePayload, tenantId?: number) {
  return request<EnterpriseIAMPartner>(
    `/api/enterprise/v1/iam/partners/${partnerId}/update`,
    { method: "POST", body: JSON.stringify(payload), tenantId }
  )
}

export function updateEnterprisePartnerStatus(partnerId: number, status: number, tenantId?: number, reason = "Update partner status") {
  return request<EnterpriseIAMPartner>(
    `/api/enterprise/v1/iam/partners/${partnerId}/_status`,
    { method: "POST", body: JSON.stringify({ status, reason }), tenantId }
  )
}

export function fetchEnterpriseIAMDepartments(query?: IAMListQuery, tenantId?: number) {
  return request<IAMPageResult<EnterpriseIAMDepartment>>(
    `/api/enterprise/v1/iam/departments${queryString(query)}`,
    { tenantId }
  )
}

const ENTERPRISE_IAM_CATALOG_PAGE_SIZE = 100

export async function fetchAllEnterpriseIAMDepartments(query?: IAMListQuery, tenantId?: number) {
  const { page: _page, limit: _limit, ...rest } = query ?? {}
  const results: EnterpriseIAMDepartment[] = []
  let page = 1

  for (;;) {
    const response = await fetchEnterpriseIAMDepartments(
      {
        ...rest,
        page,
        limit: ENTERPRISE_IAM_CATALOG_PAGE_SIZE,
      },
      tenantId
    )
    results.push(...response.results)

    const total = response.page?.total ?? results.length
    if (results.length >= total || response.results.length < ENTERPRISE_IAM_CATALOG_PAGE_SIZE) {
      return results
    }
    page += 1
  }
}

export function createEnterpriseIAMDepartment(payload: EnterpriseDepartmentCreatePayload, tenantId?: number) {
  return request<EnterpriseIAMDepartment>("/api/enterprise/v1/iam/departments/create", {
    method: "POST",
    body: JSON.stringify(payload),
    tenantId,
  })
}

export function fetchEnterpriseIAMRoles(tenantId?: number, domainType = "enterprise") {
  return request<IAMRole[]>(`/api/enterprise/v1/iam/roles${queryString({ domainType })}`, { tenantId })
}

export function fetchEnterpriseIAMAudit(query?: IAMListQuery, tenantId?: number) {
  return request<IAMPageResult<IAMAuditLog>>(
    `/api/enterprise/v1/iam/audit${queryString(query)}`,
    { tenantId }
  )
}

export function downloadEnterpriseIAMAudit(query?: IAMListQuery, tenantId?: number) {
  return requestBlob(`/api/enterprise/v1/iam/audit/export${queryString(query)}`, { tenantId })
}
