import { clearSession, readSession, writeSession, type AuthSession } from "@/lib/auth"
import { request } from "@/lib/api/client"

export type LoginRequest = {
  username: string
  password: string
  domainType?: "platform" | "enterprise" | "partner" | "customer"
  portalChoiceConfirmed?: boolean
}

export type CustomerRegistrationMethod = "invite" | "service_code" | "visitor"

export type CustomerRegistrationContext = {
  method: CustomerRegistrationMethod
  domainType?: "customer" | "partner"
  email?: string
  displayName?: string
  expiresAt?: string
  registered?: boolean
  tenant?: { id: number; name: string }
  customerOrg?: { id: number; name: string }
  partner?: { id: number; name: string }
  product?: { id: number; code: string; name: string; category: string }
  device?: { id: number; deviceNo: string; serialNo: string; regionCode: string }
}

export type CustomerRegistrationRequest = {
  method: CustomerRegistrationMethod
  credential: string
  username: string
  displayName?: string
  email: string
  password: string
  verificationCode?: string
}

export type AuthVerificationPurpose = "enterprise_register" | "customer_register" | "password_reset"

export type EnterpriseRegistrationRequest = {
  tenantName: string
  industry?: string
  countryRegion?: string
  username: string
  displayName?: string
  email: string
  mobile?: string
  password: string
  verificationCode: string
}

export type PasswordResetRequest = {
  email: string
  verificationCode: string
  password: string
}

export type PortalInvitationDomain = "customer" | "partner"

export type PortalInvitationRegisterRequest = {
  domainType: PortalInvitationDomain
  credential: string
  username: string
  displayName?: string
  email: string
  password: string
}

export type PortalInvitationBindRequest = {
  domainType: PortalInvitationDomain
  credential: string
  username: string
  password: string
}

export type UpdateOwnProfilePayload = {
  nickname: string
  avatar?: string
  mobile?: string | null
  email?: string | null
  locale: string
  timezone?: string
}

function mergeAuthSession(profile: AuthSession) {
  const current = readSession()
  const nextDomainType = profile.domainType || current?.domainType
  const nextTenantId = profile.tenantId || current?.tenantId || 0
  const membershipApplies = Boolean(nextTenantId) && (nextDomainType === "enterprise" || nextDomainType === "partner")
  const next: AuthSession = {
    ...(current || profile),
    ...profile,
    accessToken: profile.accessToken || current?.accessToken || "",
    expiresAt: profile.expiresAt || current?.expiresAt,
    tenantId: nextTenantId,
    domainType: nextDomainType,
    subjectType: profile.subjectType || current?.subjectType,
    subjectId: profile.subjectId || current?.subjectId,
    partnerAccountId: profile.partnerAccountId || current?.partnerAccountId,
    supportGrantId: profile.supportGrantId || current?.supportGrantId,
    supportMode: profile.supportMode || current?.supportMode,
    impersonatedBy: profile.impersonatedBy || current?.impersonatedBy,
    featureFlags: profile.featureFlags || current?.featureFlags,
    locale: profile.locale || profile.user?.locale || current?.locale,
    timezone: profile.timezone || profile.user?.timezone || current?.timezone,
    availablePortals: profile.availablePortals || current?.availablePortals,
    requiresPortalChoice: profile.requiresPortalChoice ?? current?.requiresPortalChoice,
    membership: membershipApplies ? (profile.membership || current?.membership) : undefined,
  }
  writeSession(next)
  return next
}

export async function loginWithPassword(payload: LoginRequest) {
  const data = await request<AuthSession>("/api/auth/login", {
    method: "POST",
    body: JSON.stringify(payload),
    skipAuth: true,
  })
  writeSession(data)
  return data
}

export function sendAuthVerificationCode(purpose: AuthVerificationPurpose, email: string) {
  return request<void>("/api/auth/verification-code/send", {
    method: "POST",
    body: JSON.stringify({ purpose, email }),
    skipAuth: true,
  })
}

export function verifyCustomerRegistration(method: CustomerRegistrationMethod, credential: string) {
  return request<CustomerRegistrationContext>("/api/auth/customer-registration/verify", {
    method: "POST",
    body: JSON.stringify({ method, credential }),
    skipAuth: true,
  })
}

export async function registerCustomerAccount(payload: CustomerRegistrationRequest) {
  const data = await request<AuthSession>("/api/auth/customer-registration/register", {
    method: "POST",
    body: JSON.stringify(payload),
    skipAuth: true,
  })
  writeSession(data)
  return data
}

export function verifyPortalInvitation(domainType: PortalInvitationDomain, credential: string) {
  return request<CustomerRegistrationContext>("/api/auth/portal-invitation/verify", {
    method: "POST",
    body: JSON.stringify({ domainType, credential }),
    skipAuth: true,
  })
}

export async function registerPortalInvitation(payload: PortalInvitationRegisterRequest) {
  const data = await request<AuthSession>("/api/auth/portal-invitation/register", {
    method: "POST",
    body: JSON.stringify(payload),
    skipAuth: true,
  })
  writeSession(data)
  return data
}

export async function bindPortalInvitation(payload: PortalInvitationBindRequest) {
  const data = await request<AuthSession>("/api/auth/portal-invitation/bind", {
    method: "POST",
    body: JSON.stringify(payload),
    skipAuth: true,
  })
  writeSession(data)
  return data
}

export async function registerEnterpriseAccount(payload: EnterpriseRegistrationRequest) {
  const data = await request<AuthSession>("/api/auth/enterprise-registration/register", {
    method: "POST",
    body: JSON.stringify(payload),
    skipAuth: true,
  })
  writeSession(data)
  return data
}

export function resetPassword(payload: PasswordResetRequest) {
  return request<void>("/api/auth/password-reset", {
    method: "POST",
    body: JSON.stringify(payload),
    skipAuth: true,
  })
}

export async function exchangeWxWorkTicket(ticket: string) {
  const data = await request<AuthSession>("/api/auth/wxwork_exchange", {
    method: "POST",
    body: JSON.stringify({ ticket }),
    skipAuth: true,
  })
  writeSession(data)
  return data
}

export async function exchangeOIDCTicket(ticket: string) {
  const data = await request<AuthSession>("/api/auth/oidc_exchange", {
    method: "POST",
    body: JSON.stringify({ ticket }),
    skipAuth: true,
  })
  writeSession(data)
  return data
}

export async function fetchProfile() {
  return request<AuthSession>("/api/auth/profile")
}

export async function updateOwnProfile(payload: UpdateOwnProfilePayload) {
  const data = await request<AuthSession>("/api/auth/profile/update", {
    method: "POST",
    body: JSON.stringify(payload),
  })
  return mergeAuthSession(data)
}

export async function logout() {
  try {
    await request("/api/auth/logout", {
      method: "POST",
    })
  } finally {
    clearSession()
  }
}
