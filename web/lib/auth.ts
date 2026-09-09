import {
  getPortalFromPath,
  getSessionPortal,
  isLoginPortal,
  LOGIN_PORTALS,
  type LoginPortal,
} from "@/lib/login-portals"
import type { CustomerTheme } from "@/lib/customer-theme"

export type AuthUser = {
  id: number
  username: string
  nickname: string
  avatar: string
  email?: string
  mobile?: string
  locale?: string
  timezone?: string
  status: number
  roles: string[]
}

export type AuthSession = {
  accessToken: string
  expiresAt?: string
  user: AuthUser
  permissions: string[]
  roles: string[]
  featureFlags?: Record<string, boolean>
  tenantId: number
  domainType?: "platform" | "enterprise" | "customer" | "partner" | "service_account"
  subjectType?: string
  subjectId?: number
  partnerAccountId?: number
  supportGrantId?: number
  supportMode?: "platform_tenant" | "customer_portal" | "employee_portal" | string
  impersonatedBy?: string
  locale?: string
  tenantDefaultLocale?: string
  customerDefaultLocale?: string
  timezone?: string
  availablePortals?: Array<{
    domainType: "platform" | "enterprise" | "customer" | "partner" | "service_account"
    label: string
    tenantId: number
    subjectType: string
    subjectId: number
    partnerAccountId?: number
    default?: boolean
  }>
  requiresPortalChoice?: boolean
  membership?: {
    paid: boolean
    planId?: number
    planCode?: string
    planName?: string
    planType?: string
  }
  tenantBranding?: {
    brandName: string
    logoUrl: string
    customDomain?: string
    customerTheme?: CustomerTheme | string
  }
  externalPortals?: Array<{
    provider: string
    name: string
    url: string
    embedMode: "external" | "iframe" | string
  }>
}

const SESSION_STORAGE_KEY = "remote-helpdesk-session"
const LEGACY_SESSION_STORAGE_KEY = "agent-desk-session"
const PORTAL_SESSION_STORAGE_PREFIX = `${SESSION_STORAGE_KEY}:`
const LEGACY_PORTAL_SESSION_STORAGE_PREFIX = `${LEGACY_SESSION_STORAGE_KEY}:`
const PLATFORM_RETURN_SESSION_KEY = "remote-helpdesk-platform-return-session"
const LEGACY_PLATFORM_RETURN_SESSION_KEY = "agent-desk-platform-return-session"
export const AUTH_SESSION_EXPIRED_EVENT = "remote-helpdesk-auth-expired"
export const AUTH_SESSION_CHANGED_EVENT = "remote-helpdesk-auth-session-changed"
const LEGACY_AUTH_SESSION_EXPIRED_EVENT = "agent-desk-auth-expired"
const LEGACY_AUTH_SESSION_CHANGED_EVENT = "agent-desk-auth-session-changed"
const authExpiryDispatchedPortals = new Set<LoginPortal>()

function hasWindow() {
  return typeof window !== "undefined"
}

function portalSessionStorageKey(portal: LoginPortal, legacy = false) {
  const prefix = legacy
    ? LEGACY_PORTAL_SESSION_STORAGE_PREFIX
    : PORTAL_SESSION_STORAGE_PREFIX
  return `${prefix}${portal}`
}

function dispatchAuthEvent(primaryEvent: string, legacyEvent: string) {
  window.dispatchEvent(new Event(primaryEvent))
  window.dispatchEvent(new Event(legacyEvent))
}

function sessionStoragePortal(session: AuthSession): LoginPortal {
  return getSessionPortal(session.domainType)
}

function currentLocationPortal(): LoginPortal {
  const currentPath = `${window.location.pathname}${window.location.search}`
  if (
    window.location.pathname.startsWith("/dashboard/login") ||
    window.location.pathname.startsWith("/platform/login") ||
    window.location.pathname.startsWith("/customer/login")
  ) {
    const params = new URLSearchParams(window.location.search)
    const portal = params.get("portal")
    if (isLoginPortal(portal)) {
      return portal
    }
    const nextPath = params.get("next")
    if (nextPath) {
      return getPortalFromPath(nextPath)
    }
  }
  return getPortalFromPath(currentPath)
}

function isStoredSession(value: unknown): value is AuthSession {
  if (!value || typeof value !== "object") {
    return false
  }
  const session = value as Partial<AuthSession>
  return typeof session.accessToken === "string" && session.accessToken.length > 0
}

function readSessionFromStorage(key: string, expectedPortal?: LoginPortal): AuthSession | null {
  const raw = window.localStorage.getItem(key)
  if (!raw) {
    return null
  }
  try {
    const session = JSON.parse(raw) as unknown
    if (!isStoredSession(session)) {
      window.localStorage.removeItem(key)
      return null
    }
    if (expectedPortal && sessionStoragePortal(session) !== expectedPortal) {
      window.localStorage.removeItem(key)
      return null
    }
    return session
  } catch {
    window.localStorage.removeItem(key)
    return null
  }
}

function pruneMismatchedPortalSessions() {
  for (const portal of LOGIN_PORTALS) {
    readSessionFromStorage(portalSessionStorageKey(portal), portal)
    readSessionFromStorage(portalSessionStorageKey(portal, true), portal)
  }
}

export function readSession(): AuthSession | null {
  if (!hasWindow()) {
    return null
  }

  const portal = currentLocationPortal()
  pruneMismatchedPortalSessions()
  const scoped = readSessionFromStorage(portalSessionStorageKey(portal), portal)
  if (scoped) {
    return scoped
  }

  const legacyScoped = readSessionFromStorage(portalSessionStorageKey(portal, true), portal)
  if (legacyScoped) {
    window.localStorage.setItem(portalSessionStorageKey(portal), JSON.stringify(legacyScoped))
    return legacyScoped
  }

  const stored =
    readSessionFromStorage(SESSION_STORAGE_KEY) ||
    readSessionFromStorage(LEGACY_SESSION_STORAGE_KEY)
  if (!stored) {
    return null
  }
  if (getSessionPortal(stored.domainType) !== portal) {
    return null
  }
  const serialized = JSON.stringify(stored)
  window.localStorage.setItem(SESSION_STORAGE_KEY, serialized)
  window.localStorage.setItem(portalSessionStorageKey(portal), serialized)
  return stored
}

export function writeSession(session: AuthSession) {
  if (!hasWindow()) {
    return
  }
  const serialized = JSON.stringify(session)
  const portal = sessionStoragePortal(session)
  window.localStorage.setItem(SESSION_STORAGE_KEY, serialized)
  window.localStorage.setItem(portalSessionStorageKey(portal), serialized)
  window.localStorage.setItem(LEGACY_SESSION_STORAGE_KEY, serialized)
  window.localStorage.setItem(portalSessionStorageKey(portal, true), serialized)
  pruneMismatchedPortalSessions()
  authExpiryDispatchedPortals.delete(portal)
  dispatchAuthEvent(AUTH_SESSION_CHANGED_EVENT, LEGACY_AUTH_SESSION_CHANGED_EVENT)
}

export function clearPortalSession(portal: LoginPortal) {
  if (!hasWindow()) {
    return
  }
  window.localStorage.removeItem(portalSessionStorageKey(portal))
  window.localStorage.removeItem(portalSessionStorageKey(portal, true))
  const stored = readSessionFromStorage(SESSION_STORAGE_KEY)
  if (stored && getSessionPortal(stored.domainType) === portal) {
    window.localStorage.removeItem(SESSION_STORAGE_KEY)
  }
  const legacyStored = readSessionFromStorage(LEGACY_SESSION_STORAGE_KEY)
  if (legacyStored && getSessionPortal(legacyStored.domainType) === portal) {
    window.localStorage.removeItem(LEGACY_SESSION_STORAGE_KEY)
  }
  dispatchAuthEvent(AUTH_SESSION_CHANGED_EVENT, LEGACY_AUTH_SESSION_CHANGED_EVENT)
}

export function clearSession() {
  if (!hasWindow()) {
    return
  }
  window.localStorage.removeItem(SESSION_STORAGE_KEY)
  window.localStorage.removeItem(LEGACY_SESSION_STORAGE_KEY)
  for (const portal of LOGIN_PORTALS) {
    window.localStorage.removeItem(portalSessionStorageKey(portal))
    window.localStorage.removeItem(portalSessionStorageKey(portal, true))
  }
  window.sessionStorage.removeItem(PLATFORM_RETURN_SESSION_KEY)
  window.sessionStorage.removeItem(LEGACY_PLATFORM_RETURN_SESSION_KEY)
  authExpiryDispatchedPortals.clear()
  dispatchAuthEvent(AUTH_SESSION_CHANGED_EVENT, LEGACY_AUTH_SESSION_CHANGED_EVENT)
}

function readDelegatedReturnStack(): AuthSession[] {
  if (!hasWindow()) return []
  const raw =
    window.sessionStorage.getItem(PLATFORM_RETURN_SESSION_KEY) ||
    window.sessionStorage.getItem(LEGACY_PLATFORM_RETURN_SESSION_KEY)
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw) as AuthSession | AuthSession[]
    return Array.isArray(parsed) ? parsed : [parsed]
  } catch {
    window.sessionStorage.removeItem(PLATFORM_RETURN_SESSION_KEY)
    window.sessionStorage.removeItem(LEGACY_PLATFORM_RETURN_SESSION_KEY)
    return []
  }
}

export function stashDelegatedReturnSession(session: AuthSession) {
  if (!hasWindow()) return
  const stack = readDelegatedReturnStack()
  stack.push(session)
  const serialized = JSON.stringify(stack)
  window.sessionStorage.setItem(PLATFORM_RETURN_SESSION_KEY, serialized)
  window.sessionStorage.setItem(LEGACY_PLATFORM_RETURN_SESSION_KEY, serialized)
}

export function restoreDelegatedReturnSession(): AuthSession | null {
  if (!hasWindow()) return null
  const stack = readDelegatedReturnStack()
  const session = stack.pop()
  if (!session) {
    window.sessionStorage.removeItem(PLATFORM_RETURN_SESSION_KEY)
    window.sessionStorage.removeItem(LEGACY_PLATFORM_RETURN_SESSION_KEY)
    return null
  }
  writeSession(session)
  if (stack.length > 0) {
    const serialized = JSON.stringify(stack)
    window.sessionStorage.setItem(PLATFORM_RETURN_SESSION_KEY, serialized)
    window.sessionStorage.setItem(LEGACY_PLATFORM_RETURN_SESSION_KEY, serialized)
  } else {
    window.sessionStorage.removeItem(PLATFORM_RETURN_SESSION_KEY)
    window.sessionStorage.removeItem(LEGACY_PLATFORM_RETURN_SESSION_KEY)
  }
  return session
}

export function stashPlatformReturnSession(session: AuthSession) {
  stashDelegatedReturnSession(session)
}

export function restorePlatformReturnSession(): AuthSession | null {
  return restoreDelegatedReturnSession()
}

function buildLoginRedirectPath() {
  const currentPath = `${window.location.pathname}${window.location.search}`
  if (
    currentPath.startsWith("/dashboard/login") ||
    currentPath.startsWith("/platform/login") ||
    currentPath.startsWith("/customer/login")
  ) {
    return null
  }

  const portal = getPortalFromPath(currentPath)
  const params = new URLSearchParams({ next: currentPath })
  if (portal === "platform") return `/platform/login?${params.toString()}`
  if (portal === "customer") return `/customer/login?${params.toString()}`
  params.set("portal", portal)
  return `/dashboard/login?${params.toString()}`
}

export function expireSession() {
  if (!hasWindow()) {
    return
  }
  const portal = currentLocationPortal()
  if (authExpiryDispatchedPortals.has(portal)) {
    return
  }
  authExpiryDispatchedPortals.add(portal)
  clearPortalSession(portal)
  dispatchAuthEvent(AUTH_SESSION_EXPIRED_EVENT, LEGACY_AUTH_SESSION_EXPIRED_EVENT)
  const loginPath = buildLoginRedirectPath()
  if (loginPath) {
    window.location.replace(loginPath)
  }
}
