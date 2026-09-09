import type {
  PlatformDailyUsage,
  PlatformOverview,
  PlatformRecentEvent,
  PlatformTopTenant,
} from "./platform"

type JsonRecord = Record<string, unknown>

export type PlatformOverviewSection =
  | "tenants"
  | "tickets"
  | "knowledge"
  | "aiUsage"
  | "meetings"
  | "topTenants"
  | "dailyUsage"
  | "recentEvents"

const legacyOverviewSectionMap: Record<string, PlatformOverviewSection> = {
  "\u79df\u6237\u7edf\u8ba1": "tenants",
  "\u5de5\u5355\u7edf\u8ba1": "tickets",
  "\u77e5\u8bc6\u7edf\u8ba1": "knowledge",
  "\u6a21\u578b\u7528\u91cf": "aiUsage",
  "\u4f1a\u8bae\u7edf\u8ba1": "meetings",
  "\u79df\u6237\u89c4\u6a21": "topTenants",
  "\u8c03\u7528\u8d8b\u52bf": "dailyUsage",
  "\u64cd\u4f5c\u65e5\u5fd7": "recentEvents",
}

function asRecord(value: unknown): JsonRecord | null {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? value as JsonRecord
    : null
}

function asNumber(value: unknown) {
  const parsed = typeof value === "number" ? value : Number(value)
  return Number.isFinite(parsed) ? parsed : 0
}

function asString(value: unknown) {
  return typeof value === "string" ? value : ""
}

function normalizeExpectedSections(expectedSections?: readonly string[]) {
  return expectedSections?.map((section) => legacyOverviewSectionMap[section] ?? section)
}

function readObjectSection(
  root: JsonRecord,
  key: string,
  label: PlatformOverviewSection,
  requiredFields: string[],
  unavailableSections: string[],
  expectedSections?: readonly string[],
) {
  const normalizedExpectedSections = normalizeExpectedSections(expectedSections)
  if (normalizedExpectedSections && !normalizedExpectedSections.includes(label)) return {}
  const section = asRecord(root[key])
  if (!section || requiredFields.some((field) => !(field in section))) {
    unavailableSections.push(label)
  }
  return section ?? {}
}

function readArraySection(
  root: JsonRecord,
  key: string,
  label: PlatformOverviewSection,
  unavailableSections: string[],
  expectedSections?: readonly string[],
) {
  const normalizedExpectedSections = normalizeExpectedSections(expectedSections)
  if (normalizedExpectedSections && !normalizedExpectedSections.includes(label)) return []
  const value = root[key]
  if (!Array.isArray(value)) {
    unavailableSections.push(label)
    return []
  }
  return value
}

function normalizeTopTenant(value: unknown): PlatformTopTenant | null {
  const item = asRecord(value)
  if (!item) return null
  return {
    tenantId: asNumber(item.tenantId),
    tenantName: asString(item.tenantName),
    planName: asString(item.planName),
    devices: asNumber(item.devices),
    aiRequests: asNumber(item.aiRequests),
    aiTokens: asNumber(item.aiTokens),
  }
}

function normalizeDailyUsage(value: unknown): PlatformDailyUsage | null {
  const item = asRecord(value)
  if (!item || typeof item.date !== "string") return null
  return {
    date: item.date,
    aiRequests: asNumber(item.aiRequests),
    aiTokens: asNumber(item.aiTokens),
    costAmount: asNumber(item.costAmount),
  }
}

function normalizeRecentEvent(value: unknown): PlatformRecentEvent | null {
  const item = asRecord(value)
  if (!item) return null
  return {
    id: asNumber(item.id),
    tenantId: asNumber(item.tenantId),
    tenantName: asString(item.tenantName),
    action: asString(item.action),
    targetType: asString(item.targetType),
    riskLevel: asString(item.riskLevel),
    status: asString(item.status),
    occurredAt: asString(item.occurredAt),
  }
}

export function normalizePlatformOverview(
  payload: unknown,
  expectedSections?: readonly string[],
): PlatformOverview {
  const root = asRecord(payload) ?? {}
  const unavailableSections: string[] = []
  const tenants = readObjectSection(
    root,
    "tenants",
    "tenants",
    ["total", "active", "trial", "frozen", "expiringSoon"],
    unavailableSections,
    expectedSections,
  )
  const tickets = readObjectSection(
    root,
    "tickets",
    "tickets",
    ["total", "open", "createdToday", "slaRisk"],
    unavailableSections,
    expectedSections,
  )
  const knowledge = readObjectSection(
    root,
    "knowledge",
    "knowledge",
    ["documents", "indexed", "pending"],
    unavailableSections,
    expectedSections,
  )
  const aiUsage = readObjectSection(
    root,
    "aiUsage",
    "aiUsage",
    ["requests", "tokens", "costAmount"],
    unavailableSections,
    expectedSections,
  )
  const meetings = readObjectSection(
    root,
    "meetings",
    "meetings",
    ["monthly", "active"],
    unavailableSections,
    expectedSections,
  )
  const topTenants = readArraySection(root, "topTenants", "topTenants", unavailableSections, expectedSections)
    .map(normalizeTopTenant)
    .filter((item): item is PlatformTopTenant => item !== null)
  const dailyUsage = readArraySection(root, "dailyUsage", "dailyUsage", unavailableSections, expectedSections)
    .map(normalizeDailyUsage)
    .filter((item): item is PlatformDailyUsage => item !== null)
  const recentEvents = readArraySection(root, "recentEvents", "recentEvents", unavailableSections, expectedSections)
    .map(normalizeRecentEvent)
    .filter((item): item is PlatformRecentEvent => item !== null)

  return {
    generatedAt: asString(root.generatedAt),
    tenants: {
      total: asNumber(tenants.total),
      active: asNumber(tenants.active),
      trial: asNumber(tenants.trial),
      frozen: asNumber(tenants.frozen),
      expiringSoon: asNumber(tenants.expiringSoon),
    },
    products: asNumber(root.products),
    devices: asNumber(root.devices),
    tickets: {
      total: asNumber(tickets.total),
      open: asNumber(tickets.open),
      createdToday: asNumber(tickets.createdToday),
      slaRisk: asNumber(tickets.slaRisk),
    },
    knowledge: {
      documents: asNumber(knowledge.documents),
      indexed: asNumber(knowledge.indexed),
      pending: asNumber(knowledge.pending),
    },
    aiUsage: {
      requests: asNumber(aiUsage.requests),
      tokens: asNumber(aiUsage.tokens),
      costAmount: asNumber(aiUsage.costAmount),
    },
    meetings: {
      monthly: asNumber(meetings.monthly),
      active: asNumber(meetings.active),
    },
    topTenants,
    dailyUsage,
    recentEvents,
    unavailableSections,
  }
}
