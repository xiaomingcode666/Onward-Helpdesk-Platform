import { apiGet, apiPost, apiPut } from "@/lib/api/client"

export type EnterpriseAIModelsQuery = {
  startDate?: string
  endDate?: string
  granularity?: string
  timezone?: string
  page?: number
  pageSize?: number
  sortBy?: string
  sortOrder?: string
}

export type EnterpriseAIModelAccount = {
  tenantId: number
  sub2apiAccountId: string
  accountName: string
  loginEmail: string
  accountStatus: string
  provisionStatus: string
  balance: number
  concurrency: number
  rpmLimit: number
  defaultKeyId: string
  defaultKeyName: string
  defaultKeyQuota: number
  defaultKeyQuotaUsed: number
  defaultKeyStatus: string
  defaultLlmModel: string
  accessTokenExpiresAt: string
  lastSyncedAt: string
}

export type EnterpriseAISub2APIUser = {
  id: number
  email: string
  username: string
  role: string
  balance: number
  frozenBalance: number
  concurrency: number
  currentConcurrency: number
  status: string
  totalRecharged: number
  rpmLimit: number
  lastActiveAt: string
  updatedAt: string
}

export type EnterpriseAIKeyGroup = {
  id: number
  name: string
  platform: string
  rateMultiplier: number
  status: string
  rpmLimit: number
}

export type EnterpriseAIKey = {
  id: number
  userId: number
  keyPreview: string
  name: string
  productId?: number
  productName?: string
  productCode?: string
  groupId: number
  group?: EnterpriseAIKeyGroup
  status: string
  quota: number
  quotaUsed: number
  quotaRemaining: number
  quotaUsagePercent: number
  expiresAt: string
  lastUsedAt: string
  lastUsedIp: string
  currentConcurrency: number
  rateLimit5h: number
  rateLimit1d: number
  rateLimit7d: number
  usage5h: number
  usage1d: number
  usage7d: number
  createdAt: string
  updatedAt: string
}

export type EnterpriseAIKeyPaging = {
  total: number
  page: number
  pageSize: number
  pages: number
}

export type EnterpriseAIUsageTrendPoint = {
  date: string
  requests: number
  inputTokens: number
  outputTokens: number
  cacheCreationTokens: number
  cacheReadTokens: number
  totalTokens: number
  cost: number
  actualCost: number
}

export type EnterpriseAIUsagePlatformStats = {
  platform: string
  totalRequests: number
  totalTokens: number
  totalActualCost: number
  todayRequests: number
  todayTokens: number
  todayActualCost: number
}

export type EnterpriseAIUsageStats = {
  totalApiKeys: number
  activeApiKeys: number
  totalRequests: number
  totalInputTokens: number
  totalOutputTokens: number
  totalCacheCreationTokens: number
  totalCacheReadTokens: number
  totalTokens: number
  totalCost: number
  totalActualCost: number
  todayRequests: number
  todayInputTokens: number
  todayOutputTokens: number
  todayCacheCreationTokens: number
  todayCacheReadTokens: number
  todayTokens: number
  todayCost: number
  todayActualCost: number
  averageDurationMs: number
  rpm: number
  tpm: number
  byPlatform: EnterpriseAIUsagePlatformStats[]
}

export type EnterpriseAISubscriptionGroup = {
  id: number
  name: string
  description: string
  platform: string
  rateMultiplier: number
  isExclusive: boolean
  status: string
  subscriptionType: string
  dailyLimitUsd: number
  weeklyLimitUsd: number
  monthlyLimitUsd: number
  rpmLimit: number
}

export type EnterpriseAISubscription = {
  id: number
  userId: number
  groupId: number
  startsAt: string
  expiresAt: string
  status: string
  durationDays: number
  remainingDays: number
  progressPercent: number
  dailyWindowStart: string
  weeklyWindowStart: string
  monthlyWindowStart: string
  dailyUsageUsd: number
  weeklyUsageUsd: number
  monthlyUsageUsd: number
  group?: EnterpriseAISubscriptionGroup
  createdAt: string
  updatedAt: string
}

export type EnterpriseAIModelWorkspace = {
  generatedAt: string
  account: EnterpriseAIModelAccount
  user: EnterpriseAISub2APIUser
  stats: EnterpriseAIUsageStats
  trend: EnterpriseAIUsageTrendPoint[]
  keys: EnterpriseAIKey[]
  keyPaging: EnterpriseAIKeyPaging
  subscriptions: EnterpriseAISubscription[]
  activeSubscription?: EnterpriseAISubscription
}

export type EnterpriseAICapability = {
  type: "llm" | "embedding"
  available: boolean
  modelName: string
  models: string[]
  dimension: number
}

export type EnterpriseAICapabilities = {
  generatedAt: string
  capabilities: EnterpriseAICapability[]
  defaultCredential?: {
    keyName: string
    keyStatus: string
    provisionStatus: string
    ready: boolean
  }
}

export function getEnterpriseAICapabilities() {
  return apiGet<EnterpriseAICapabilities>("/models/capabilities")
}

export function updateEnterpriseAIDefaultLLMModel(modelName: string) {
  return apiPut<EnterpriseAICapabilities>("/models/default-llm-model", { modelName })
}

export function getEnterpriseAIModelsWorkspace(query?: EnterpriseAIModelsQuery) {
  return apiGet<EnterpriseAIModelWorkspace>("/models/workspace", normalizeEnterpriseAIModelsQuery(query))
}

export function updateEnterpriseAIKeyQuota(keyId: number, quota: number, query?: EnterpriseAIModelsQuery) {
  return apiPut<EnterpriseAIModelWorkspace>(`/models/keys/${keyId}/quota${queryString(query)}`, { quota })
}

export function resetEnterpriseAIKeyQuota(keyId: number, query?: EnterpriseAIModelsQuery) {
  return apiPost<EnterpriseAIModelWorkspace>(`/models/keys/${keyId}/reset-quota${queryString(query)}`)
}

function normalizeEnterpriseAIModelsQuery(query?: EnterpriseAIModelsQuery) {
  return {
    startDate: query?.startDate,
    endDate: query?.endDate,
    granularity: query?.granularity || "day",
    timezone: query?.timezone || "Asia/Shanghai",
    page: query?.page || 1,
    pageSize: query?.pageSize || 20,
    sortBy: query?.sortBy || "created_at",
    sortOrder: query?.sortOrder || "desc",
  }
}

function queryString(query?: EnterpriseAIModelsQuery) {
  const normalized = normalizeEnterpriseAIModelsQuery(query)
  const params = new URLSearchParams()
  Object.entries(normalized).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== "") {
      params.set(key, String(value))
    }
  })
  const encoded = params.toString()
  return encoded ? `?${encoded}` : ""
}
