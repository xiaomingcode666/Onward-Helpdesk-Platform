import type { AuthSession } from "@/lib/auth"

export const LOGIN_PORTALS = [
  "platform",
  "enterprise",
  "partner",
  "customer",
] as const

export type LoginPortal = (typeof LOGIN_PORTALS)[number]

const DEFAULT_PORTAL_PATH: Record<LoginPortal, string> = {
  platform: "/platform",
  enterprise: "/enterprise",
  partner: "/partner",
  customer: "/customer",
}

const PORTAL_DESTINATIONS: Record<
  LoginPortal,
  ReadonlyArray<{ path: string; permission?: string | readonly string[]; feature?: string | readonly string[] }>
> = {
  platform: [
    { path: "/platform", permission: "tenant.view" },
    { path: "/platform/usage", permission: ["report.view", "finance.view"] },
    { path: "/platform/models", permission: ["aiConfig.view", "finance.view"] },
    { path: "/platform/ops", permission: "tenant.view" },
    // { path: "/platform/globalization", permission: "tenant.view" },
    { path: "/platform/staff", permission: "user.view" },
    { path: "/platform/tenants", permission: "tenant.view" },
    { path: "/platform/permissions", permission: "role.view" },
    { path: "/platform/audit", permission: "session.view" },
  ],
  enterprise: [
    { path: "/enterprise", permission: "ticket.view" },
    { path: "/enterprise/tickets", permission: "ticket.view" },
    { path: "/enterprise/products", permission: "product.view" },
    { path: "/enterprise/devices", permission: "device.view" },
    { path: "/enterprise/knowledge", permission: "knowledgeBase.view", feature: "ai" },
    { path: "/enterprise/ai", permission: "aiAgent.view", feature: "ai" },
    { path: "/enterprise/video", permission: "meeting.view" },
    { path: "/enterprise/meeting-room", permission: "meeting.view" },
    { path: "/enterprise/access", permission: "tenantIntegrationConfig.view" },
    { path: "/enterprise/workflow", permission: "aiWorkflow.view" },
    { path: "/enterprise/reports", permission: "report.view" },
    { path: "/enterprise/people", permission: "user.view" },
    { path: "/enterprise/customer-users", permission: "customer.view" },
    { path: "/enterprise/permissions", permission: "role.view" },
    { path: "/enterprise/audit", permission: "session.view" },
    { path: "/enterprise/usage", permission: "productAIUsageCredential.view", feature: "ai" },
    { path: "/enterprise/notifications", permission: "notification.view" },
  ],
  partner: [
    { path: "/partner", permission: "ticket.view" },
    { path: "/partner/ticket-detail", permission: "ticket.view" },
    { path: "/partner/tickets", permission: "ticket.view" },
    { path: "/partner/conversations", permission: "ticket.view" },
    { path: "/partner/video", permission: "meeting.view" },
    { path: "/partner/meeting-room", permission: "meeting.view" },
    { path: "/partner/people", permission: "partnerMember.view" },
  ],
  customer: [
    { path: "/customer/onboarding" },
    { path: "/customer", permission: "conversation.view" },
    { path: "/customer/chat", permission: "conversation.view" },
    { path: "/customer/tickets", permission: "ticket.view" },
    { path: "/customer/meeting", permission: "meeting.view" },
    { path: "/customer/devices", permission: "device.view" },
    { path: "/customer/my" },
  ],
}

export function isLoginPortal(value: string | null | undefined): value is LoginPortal {
  return LOGIN_PORTALS.includes(value as LoginPortal)
}

export function sanitizeInternalPath(value: string | null | undefined) {
  const path = value?.trim() ?? ""
  if (!path.startsWith("/") || path.startsWith("//") || path.includes("\\")) {
    return null
  }
  return path
}

export function getPortalDefaultPath(portal: LoginPortal) {
  return DEFAULT_PORTAL_PATH[portal]
}

function isPathUnder(path: string, root: string) {
  return path === root || path.startsWith(`${root}/`) || path.startsWith(`${root}?`)
}

export function getPortalFromPath(value: string | null | undefined): LoginPortal {
  const path = sanitizeInternalPath(value)
  if (path && isPathUnder(path, "/platform")) return "platform"
  if (path && isPathUnder(path, "/partner")) return "partner"
  if (
    path &&
    (isPathUnder(path, "/customer") ||
      isPathUnder(path, "/c") ||
      isPathUnder(path, "/mobile"))
  ) {
    return "customer"
  }
  return "enterprise"
}

export function resolveLoginPortal(
  portalValue: string | null | undefined,
  nextPath: string | null | undefined
): LoginPortal {
  return isLoginPortal(portalValue) ? portalValue : getPortalFromPath(nextPath)
}

export function getPortalRedirectPath(
  portal: LoginPortal,
  nextPath: string | null | undefined
) {
  const safeNextPath = sanitizeInternalPath(nextPath)
  if (safeNextPath && getPortalFromPath(safeNextPath) === portal) {
    return safeNextPath
  }
  return getPortalDefaultPath(portal)
}

export function getSessionPortal(
  domainType: AuthSession["domainType"]
): LoginPortal {
  if (domainType === "platform") return "platform"
  if (domainType === "partner") return "partner"
  if (domainType === "customer") return "customer"
  return "enterprise"
}

function normalizeCustomerLoginDestination(path: string) {
  const parsed = new URL(path, "http://customer.portal.local")
  if (parsed.pathname !== "/customer/chat" || !parsed.searchParams.has("conversationId")) {
    return path
  }
  parsed.searchParams.delete("conversationId")
  const search = parsed.searchParams.toString()
  return `${parsed.pathname}${search ? `?${search}` : ""}${parsed.hash}`
}

function hasPortalPermission(
  required: string | readonly string[] | undefined,
  permissionSet: ReadonlySet<string>
) {
  if (!required) {
    return true
  }
  return typeof required === "string"
    ? permissionSet.has(required)
    : required.some((permission) => permissionSet.has(permission))
}

function hasPortalFeature(
  required: string | readonly string[] | undefined,
  featureFlags?: Record<string, boolean>
) {
  if (!required || !featureFlags) {
    return true
  }
  return typeof required === "string"
    ? featureFlags[required] !== false
    : required.some((feature) => featureFlags[feature] !== false)
}

export function resolveSessionDestination(
  session: Pick<AuthSession, "domainType"> & Partial<Pick<AuthSession, "permissions" | "featureFlags">>,
  nextPath: string | null | undefined
) {
  const portal = getSessionPortal(session.domainType)
  const redirectPath = getPortalRedirectPath(portal, nextPath)
  const candidate = portal === "customer"
    ? normalizeCustomerLoginDestination(redirectPath)
    : redirectPath
  if (!session.permissions) {
    return candidate
  }
  const permissionSet = new Set(session.permissions)
  const destinations = PORTAL_DESTINATIONS[portal]
  const matched = [...destinations]
    .sort((left, right) => right.path.length - left.path.length)
    .find(({ path }) => isPathUnder(candidate, path))
  if (
    hasPortalPermission(matched?.permission, permissionSet) &&
    hasPortalFeature(matched?.feature, session.featureFlags)
  ) {
    return candidate
  }
  return (
    destinations.find(({ permission, feature }) =>
      hasPortalPermission(permission, permissionSet) &&
      hasPortalFeature(feature, session.featureFlags)
    )?.path ??
    getPortalDefaultPath(portal)
  )
}
