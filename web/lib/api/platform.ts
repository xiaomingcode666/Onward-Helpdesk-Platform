import { request } from "@/lib/api/client"
import type { AuthSession } from "@/lib/auth"
import type { CustomerTheme } from "@/lib/customer-theme"
import {
  normalizePlatformOverview,
  type PlatformOverviewSection,
} from "@/lib/api/platform-overview-normalizer"

export interface PlatformTenantSummary {
  total: number
  active: number
  trial: number
  frozen: number
  expiringSoon: number
}

export interface PlatformTicketSummary {
  total: number
  open: number
  createdToday: number
  slaRisk: number
}

export interface PlatformDailyUsage {
  date: string
  aiRequests: number
  aiTokens: number
  costAmount: number
}

export interface PlatformTopTenant {
  tenantId: number
  tenantName: string
  planName: string
  devices: number
  aiRequests: number
  aiTokens: number
}

export interface PlatformRecentEvent {
  id: number
  tenantId: number
  tenantName: string
  action: string
  targetType: string
  riskLevel: string
  status: string
  occurredAt: string
}

export interface PlatformOverview {
  generatedAt: string
  tenants: PlatformTenantSummary
  products: number
  devices: number
  tickets: PlatformTicketSummary
  knowledge: {
    documents: number
    indexed: number
    pending: number
  }
  aiUsage: {
    requests: number
    tokens: number
    costAmount: number
  }
  meetings: {
    monthly: number
    active: number
  }
  topTenants: PlatformTopTenant[]
  dailyUsage: PlatformDailyUsage[]
  recentEvents: PlatformRecentEvent[]
  unavailableSections: string[]
}

export interface PlatformTenantItem {
  id: number
  name: string
  code: string
  serviceScene: "equipment_after_sales" | "knowledge_support"
  brandName: string
  logoAssetId: number
  logoUrl: string
  customDomain: string
  customerTheme: CustomerTheme | string
  serverConsoleName: string
  serverConsoleUrl: string
  serverConsoleMode: "external" | "iframe" | string
  serverConsoleEnabled: boolean
  industry: string
  countryRegion: string
  defaultLocale: string
  customerDefaultLocale: string
  supportedLocales: string[]
  timezone: string
  supportedTimezones: string[]
  dataRegion: string
  status: number
  aiEnabled?: boolean
  trialEndsAt: string
  frozenReason: string
  decommissionedAt: string
  decommissionReason: string
  purgeAfterDays: number
  purgedAt: string
  dataExportCompletedAt: string
  planCode: string
  planName: string
  subscriptionStatus: number
  adminUserId: number
  adminUsername: string
  adminDisplayName: string
  adminEmail: string
  deviceCount: number
  memberCount: number
  monthlyAiRequests: number
  monthlyAiTokens: number
  monthlyAiCost: number
  lastMeteredAt: string
  createdAt: string
  updatedAt: string
}

export interface PlatformTenantListResponse {
  tenants: PlatformTenantItem[]
  totalCount: number
  summary: PlatformTenantSummary
}

export interface PlatformGlobalizationRegion {
  dataRegion: string
  tenantCount: number
  countryRegions: string[]
  locales: string[]
  timezones: string[]
}

export interface PlatformGlobalizationResponse {
  generatedAt: string
  totalCount: number
  summary: PlatformTenantSummary
  configuredRegionCount: number
  localeCount: number
  timezoneCount: number
  regions: PlatformGlobalizationRegion[]
  tenants: PlatformTenantItem[]
  page: number
  pageSize: number
  totalPages: number
  hasMore: boolean
}

export interface PlatformTenantAdminPasswordResult {
  tenantId: number
  userId: number
  username: string
  displayName: string
}

export interface PlatformTenantListQuery {
  search?: string
  status?: string
  lifecycle?: "all" | "active" | "trial" | "expiring" | "frozen" | "decommissioned"
  billing?: "paid" | "free"
  page?: number
  limit?: number
}

export interface PlatformTenantCreatePayload {
  name: string
  serviceScene?: "equipment_after_sales" | "knowledge_support"
  brandName?: string
  logoAssetId?: number
  logoUrl?: string
  customDomain?: string
  customerTheme?: CustomerTheme
  serverConsoleName?: string
  serverConsoleUrl?: string
  serverConsoleMode?: "external" | "iframe"
  serverConsoleEnabled?: boolean
  industry?: string
  countryRegion?: string
  defaultLocale?: string
  customerDefaultLocale?: string
  supportedLocales?: string[]
  timezone?: string
  supportedTimezones?: string[]
  dataRegion?: string
  aiEnabled?: boolean
  trialEndsAt?: string
  planId?: number
  adminUsername: string
  adminNickname: string
  adminEmail?: string
  adminMobile?: string
  adminPassword: string
}

export interface PlatformTenantUpdatePayload {
  id: number
  name?: string
  serviceScene?: "equipment_after_sales" | "knowledge_support"
  brandName?: string
  logoAssetId?: number
  logoUrl?: string
  customDomain?: string
  customerTheme?: CustomerTheme
  serverConsoleName?: string
  serverConsoleUrl?: string
  serverConsoleMode?: "external" | "iframe"
  serverConsoleEnabled?: boolean
  industry?: string
  countryRegion?: string
  defaultLocale?: string
  customerDefaultLocale?: string
  supportedLocales?: string[]
  timezone?: string
  supportedTimezones?: string[]
  dataRegion?: string
  status?: number
  aiEnabled?: boolean
  trialEndsAt?: string
}

export interface PlatformUsageOverview {
  generatedAt: string
  periodStart: string
  periodEnd: string
  summary: {
    tenantCount: number
    aiRequests: number
    aiTokens: number
    costAmount: number
    atRiskTenantCount: number
    overLimitCount: number
  }
  sharedTranslationUsage: {
    configured: boolean
    apiKeyMasked: string
    fingerprint: string
    apiKeyId: string
    requests: number
    tokens: number
    costAmount: number
    lastMeteredAt: string
  }
  dailyUsage: PlatformDailyUsage[]
}

export interface PlatformOpsOverview {
  generatedAt: string
  uptimeSeconds: number
  runtime: {
    goVersion: string
    goroutines: number
    heapAllocBytes: number
    heapSysBytes: number
    gcCount: number
  }
  database: {
    status: "healthy" | "unhealthy"
    latencyMs: number
    openConnections: number
    inUse: number
    idle: number
    maxOpen: number
    error: string
  }
  workloads: {
    activeMeetings: number
    openTickets: number
    pendingKnowledge: number
    unhealthyConnectors: number
  }
  pipeline: {
    lastUsageEventAt: string
    lastAggregateAt: string
    runningSyncs: number
    failedSyncs24h: number
  }
  knowledgeIndex: {
    pendingTasks: number
    runningTasks: number
    failedTasks24h: number
    lastFinishedAt: string
  }
  sub2api: {
    tenantAccounts: number
    healthyAccounts: number
    attentionAccounts: number
    productCredentials: number
    staleCredentials24h: number
    lastUsageEventAt: string
    lastCredentialSyncAt: string
  }
  jitsi: {
    activeMeetings: number
    meetings24h: number
    participantMinutes24h: number
    onlineParticipants: number
    staleParticipants: number
    staleMeetings: number
    heartbeatStatus: "idle" | "healthy" | "degraded"
    heartbeatTimeoutSeconds: number
    lastMeetingAt: string
    lastHeartbeatAt: string
    serviceStatus: "healthy" | "unhealthy" | "unconfigured"
    serviceUrl: string
    probeUrl: string
    probeHttpStatus: number
    probeLatencyMs: number
    probeCheckedAt: string
    probeError: string
  }
  integrations: {
    speechProvider: string
    speechConfigured: boolean
    jigasiEnabled: boolean
    translationProvider: string
    translationConfigured: boolean
    arProvider: string
    arConfigured: boolean
    smtpConfigured: boolean
  }
  access: {
    totalConnectors: number
    activeConnectors: number
    healthyConnectors: number
    degradedConnectors: number
    unhealthyConnectors: number
    failedCalls24h: number
    lastTestedAt: string
  }
  notifications: {
    pendingNotifications: number
    unreadNotifications: number
    failedDeliveries24h: number
    sentDeliveries24h: number
    lastDeliveredAt: string
  }
  syncRuns: Array<{
    id: number
    tenantId: number
    tenantName: string
    syncType: string
    status: string
    recordsProcessed: number
    errorMessage: string
    startedAt: string
    endedAt: string
  }>
  knowledgeTasks: Array<{
    id: number
    tenantId: number
    tenantName: string
    productId: number
    productName: string
    subjectType: string
    subjectId: number
    providerType: string
    action: string
    status: string
    retryCount: number
    errorSummary: string
    updatedAt: string
  }>
}

export interface PlatformOpsRuntime {
  generatedAt: string
  uptimeSeconds: number
  runtime: PlatformOpsOverview["runtime"]
}

export interface PlatformOpsDatabase {
  generatedAt: string
  database: PlatformOpsOverview["database"]
}

export interface PlatformInfrastructureDependency {
  key: "database" | "redis" | "vector" | "object_storage"
  name: string
  provider: string
  status: "healthy" | "degraded" | "unhealthy" | "unconfigured"
  endpoint: string
  latencyMs: number
  storedBytes: number
  allocatedBytes: number
  capacityBytes: number
  sharePercent: number
  capacityPercent: number
  itemCount: number
  itemUnit: string
  secondaryValue: number
  secondaryLabel: string
  estimated: boolean
  measurement: string
  error: string
}

export interface PlatformOpsInfrastructure {
  generatedAt: string
  totalMeasuredBytes: number
  dependencies: PlatformInfrastructureDependency[]
  databaseTables: Array<{
    name: string
    sizeBytes: number
    estimatedRows: number
    sharePercent: number
  }>
}

export interface PlatformOpsJitsi {
  generatedAt: string
  jitsi: PlatformOpsOverview["jitsi"]
}

export interface PlatformOpsSpeech {
  generatedAt: string
  speech: {
    provider: string
    configured: boolean
    jigasiEnabled: boolean
  }
}

export interface PlatformOpsTranslation {
  generatedAt: string
  translation: {
    provider: string
    configured: boolean
  }
}

export interface PlatformOpsAR {
  generatedAt: string
  ar: {
    provider: string
    configured: boolean
  }
}

export interface PlatformOpsSub2API {
  generatedAt: string
  sub2api: PlatformOpsOverview["sub2api"]
}

export interface PlatformOpsAccess {
  generatedAt: string
  access: PlatformOpsOverview["access"]
}

export interface PlatformOpsPipeline {
  generatedAt: string
  pipeline: PlatformOpsOverview["pipeline"]
}

export interface PlatformOpsKnowledgeQueue {
  generatedAt: string
  knowledgeIndex: PlatformOpsOverview["knowledgeIndex"]
}

export interface PlatformOpsNotificationQueue {
  generatedAt: string
  notifications: PlatformOpsOverview["notifications"]
}

export type PlatformIntegrationKind = "jitsi" | "speech" | "translation" | "ar" | "sub2api" | "smtp"

export interface PlatformIntegrationSettings {
  canUpdate: boolean
  jitsi: {
    url: string
    jitsiDomain: string
    requireAuth: boolean
    appId: string
    appSecretConfigured: boolean
    webhookSecretConfigured: boolean
    tokenTtlMinutes: number
    timeout: string
    maxRetries: number
    updatedAt: string
  }
  speech: {
    provider: "disabled" | "mock" | "xfyun" | "aliyun"
    configured: boolean
    xfyunAppId: string
    xfyunApiKeyConfigured: boolean
    xfyunEndpoint: string
    xfyunDomain: string
    aliyunApiKeyConfigured: boolean
    aliyunWorkspaceId: string
    aliyunEndpoint: string
    aliyunModel: string
    jigasiEnabled: boolean
    jigasiSharedSecretConfigured: boolean
    jigasiMaxParticipantStreams: number
    jigasiMaxConcurrentStreams: number
    translationEnabled: boolean
    translationConfigured: boolean
    translationEndpoint: string
    translationTargetLanguage: string
  }
  ar: {
    provider: "disabled" | "http" | string
    configured: boolean
    endpoint: string
    apiKeyConfigured: boolean
    healthPath: string
    updatedAt: string
  }
  sub2api: {
    host: string
    adminApiKeyConfigured: boolean
    adminApiKeyMasked: string
    adminApiKeyFingerprint: string
    translationApiKeyConfigured: boolean
    translationApiKeyMasked: string
    translationApiKeyFingerprint: string
    defaultLlmModel: string
    updatedAt: string
  }
  smtp: {
    smtpHost: string
    smtpPort: number
    username: string
    passwordConfigured: boolean
    fromAddress: string
    fromName: string
    useTls: boolean
    configured: boolean
    updatedAt: string
  }
}

export interface PlatformSystemPolicySettings {
  canUpdate: boolean
  configured: boolean
  updatedAt: string
  tenantQuota: {
    freeTenantDocumentLimit: number
    paidTenantDocumentLimit: number
  }
  uploadLimit: {
    knowledgeDocumentMaxMB: number
  }
}

export interface PlatformIntegrationTestResult {
  integration: PlatformIntegrationKind
  success: boolean
  message: string
  checkedAt: string
  latencyMs: number
  details?: Record<string, unknown>
}

export type PlatformSpeechIntegrationInput = {
  provider: "disabled" | "mock" | "xfyun" | "aliyun"
  xfyunAppId?: string
  xfyunApiKey?: string
  xfyunEndpoint?: string
  xfyunDomain?: string
  aliyunApiKey?: string
  aliyunWorkspaceId?: string
  aliyunEndpoint?: string
  aliyunModel?: string
  xfyunTranslationEnabled?: boolean
  xfyunTranslationApiSecret?: string
  xfyunTranslationEndpoint?: string
  xfyunTranslationTargetLanguage?: string
  mockSegmentDurationMs?: number
  jigasiEnabled?: boolean
  jigasiSharedSecret?: string
  jigasiMaxParticipantStreams?: number
  jigasiMaxConcurrentStreams?: number
}

export type PlatformIntegrationMutation = {
  integration: PlatformIntegrationKind
  jitsi?: {
    url: string
    jitsiDomain: string
    requireAuth: boolean
    appId: string
    appSecret?: string
    webhookSecret?: string
    tokenTtlMinutes: number
    timeout: string
    maxRetries: number
  }
  speech?: PlatformSpeechIntegrationInput
  ar?: {
    provider: string
    endpoint: string
    apiKey?: string
    healthPath: string
  }
  sub2api?: {
    host: string
    adminApiKey?: string
    translationApiKey?: string
    defaultLlmModel: string
  }
  smtp?: {
    smtpHost: string
    smtpPort: number
    username: string
    password?: string
    fromAddress: string
    fromName: string
    useTls: boolean
  }
}

export interface PlatformAIProviderState {
  host: string
  defaultLlmModel: string
  adminApiKeyConfigured: boolean
  adminApiKeyFingerprint: string
  updatedAt: string
}

export interface PlatformAIConfigItem {
  id: number
  name: string
  provider: string
  baseUrl: string
  modelType: string
  modelName: string
  dimension: number
  maxContextTokens: number
  maxOutputTokens: number
  timeoutMs: number
  maxRetryCount: number
  rpmLimit: number
  tpmLimit: number
  status: number
  sortNo: number
  remark: string
  createdAt: string
  updatedAt: string
}

export interface PlatformAITenantAccount {
  id: number
  tenantId: number
  sub2apiAccountId: string
  accountName: string
  dashboardUrl: string
  loginEmail: string
  accountStatus: string
  provisionStatus: string
  concurrency: number
  balance: number
  rpmLimit: number
  defaultKeyId: string
  defaultKeyName: string
  defaultKeyGroupId: number
  defaultKeyQuota: number
  defaultKeyQuotaUsed: number
  defaultKeyStatus: string
  defaultKeyExpiresAt: string
  defaultKey?: string
  defaultLlmModel: string
  accessTokenExpiresAt: string
  lastSyncedAt: string
  lastProvisionedAt: string
  provisionMessage: string
  status: number
}

export interface PlatformAISub2APIUser {
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
  lastUsedAt: string
  updatedAt: string
}

export interface PlatformSub2APIUserListItem {
  id: number
  email: string
  username: string
  role: string
  balance: number
  frozenBalance: number
  concurrency: number
  currentConcurrency: number
  status: string
  lastActiveAt: string
  lastUsedAt: string
  createdAt: string
}

export interface PlatformSub2APIUserListResponse {
  generatedAt: string
  items: PlatformSub2APIUserListItem[]
  total: number
  page: number
  pageSize: number
  pages: number
}

export interface PlatformSub2APIUserKey {
  id: number
  userId: number
  keyPreview: string
  name: string
  groupId: number
  groupName: string
  status: string
  lastUsedAt: string
  lastUsedIp: string
  quota: number
  quotaUsed: number
  expiresAt: string
  createdAt: string
  currentConcurrency: number
}

export interface PlatformSub2APIUserKeyListResponse {
  generatedAt: string
  userId: number
  items: PlatformSub2APIUserKey[]
  total: number
  page: number
  pageSize: number
  pages: number
}

export interface PlatformAIRechargeRecord {
  id: number
  tenantId: number
  sub2apiAccountId: string
  remoteUserId: number
  operation: string
  amount: number
  balanceBefore: number
  balanceAfter: number
  notes: string
  status: string
  message: string
  occurredAt: string
  createdAt: string
  operatorId: number
  operatorName: string
}

export interface PlatformAITenantWorkspaceItem {
  tenant: PlatformTenantItem
  account?: PlatformAITenantAccount
  hasAccessToken: boolean
  needsAttention: boolean
}

export interface PlatformAIModelSummary {
  totalTenants: number
  configuredTenants: number
  activeTenants: number
  attentionTenants: number
  activeKeys: number
  modelCount: number
}

export interface PlatformAIModelWorkspace {
  generatedAt: string
  provider: PlatformAIProviderState
  models: PlatformAIConfigItem[]
  tenants: PlatformAITenantWorkspaceItem[]
  summary: PlatformAIModelSummary
}

export interface PlatformAITenantBilling {
  generatedAt: string
  tenant: PlatformAITenantWorkspaceItem
  remoteUser?: PlatformAISub2APIUser
  rechargeRecords: PlatformAIRechargeRecord[]
}

export interface PlatformAIUsageDashboardModel {
  model: string
  requests: number
  inputTokens: number
  outputTokens: number
  cacheCreationTokens: number
  cacheReadTokens: number
  totalTokens: number
  cost: number
  actualCost: number
}

export interface PlatformAIUsageEndpoint {
  endpoint: string
  requests: number
  totalTokens: number
  cost: number
  actualCost: number
}

export interface PlatformAIUsageStats {
  totalRequests: number
  totalInputTokens: number
  totalOutputTokens: number
  totalCacheTokens: number
  totalCacheCreationTokens: number
  totalCacheReadTokens: number
  totalTokens: number
  totalCost: number
  totalActualCost: number
  averageDurationMs: number
  currency: string
  endpoints: PlatformAIUsageEndpoint[]
}

export interface PlatformAIUsageDetailResponse {
  generatedAt: string
  tenant: PlatformAITenantWorkspaceItem
  stats: PlatformAIUsageStats
  models: PlatformAIUsageDashboardModel[]
  usage: unknown
}

function queryString(query?: Record<string, string | number | undefined>) {
  const params = new URLSearchParams()
  Object.entries(query ?? {}).forEach(([key, value]) => {
    if (value !== undefined && value !== "") params.set(key, String(value))
  })
  const value = params.toString()
  return value ? `?${value}` : ""
}

const platformOverviewMetricsSections = [
  "tenants",
  "tickets",
  "knowledge",
  "aiUsage",
  "meetings",
] satisfies PlatformOverviewSection[]

export async function fetchPlatformOverviewMetrics() {
  const payload = await request<unknown>("/api/platform/overview/metrics")
  return normalizePlatformOverview(payload, platformOverviewMetricsSections)
}

export async function fetchPlatformOverviewTopTenants(limit = 5) {
  const payload = await request<unknown>(`/api/platform/overview/top-tenants${queryString({ limit })}`)
  return normalizePlatformOverview(payload, ["租户规模"])
}

export async function fetchPlatformOverviewUsageTrend(days = 7) {
  const payload = await request<unknown>(`/api/platform/overview/usage-trend${queryString({ days })}`)
  return normalizePlatformOverview(payload, ["模型用量", "调用趋势"])
}

export async function fetchPlatformOverviewEvents(limit = 8) {
  const payload = await request<unknown>(`/api/platform/overview/events${queryString({ limit })}`)
  return normalizePlatformOverview(payload, ["操作日志"])
}

export function fetchPlatformGlobalization(query?: { page?: number; pageSize?: number }) {
  return request<PlatformGlobalizationResponse>(
    `/api/platform/globalization${queryString(query)}`
  )
}

export function fetchPlatformTenants(query?: PlatformTenantListQuery) {
  return request<PlatformTenantListResponse>(
    `/api/platform/tenant/list${queryString(query as Record<string, string | number | undefined>)}`
  )
}

export function createPlatformTenant(payload: PlatformTenantCreatePayload) {
  return request<PlatformTenantItem>("/api/platform/tenant/create", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updatePlatformTenant(payload: PlatformTenantUpdatePayload) {
  return request<PlatformTenantItem>("/api/platform/tenant/update", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export type PlatformTenantLogoUpload = {
  assetId: number
  url: string
  previewUrl: string
}

export function uploadPlatformTenantLogo(file: File) {
  const formData = new FormData()
  formData.set("file", file)
  return request<PlatformTenantLogoUpload>("/api/platform/tenant/logo/upload", {
    method: "POST",
    body: formData,
  })
}

export function freezePlatformTenant(tenantId: number, reason: string) {
  return request<PlatformTenantItem>("/api/platform/tenant/freeze", {
    method: "POST",
    body: JSON.stringify({ tenantId, reason }),
  })
}

export function unfreezePlatformTenant(tenantId: number) {
  return request<PlatformTenantItem>("/api/platform/tenant/unfreeze", {
    method: "POST",
    body: JSON.stringify({ tenantId }),
  })
}

export function enterPlatformTenant(tenantId: number, reason = "平台管理员进入企业端") {
  return request<AuthSession>("/api/platform/tenant/enter", {
    method: "POST",
    body: JSON.stringify({ tenantId, reason }),
  })
}

export function resetPlatformTenantAdminPassword(tenantId: number, password: string) {
  return request<PlatformTenantAdminPasswordResult>("/api/platform/tenant/admin-password", {
    method: "POST",
    body: JSON.stringify({ tenantId, password }),
  })
}

export function deletePlatformTenant(tenantId: number) {
  return request<PlatformTenantItem>("/api/platform/tenant/delete", {
    method: "POST",
    body: JSON.stringify({ tenantId }),
  })
}

export function fetchPlatformUsage() {
  return request<PlatformUsageOverview>("/api/platform/usage/overview")
}

export function fetchPlatformSub2APIUsers(query?: {
  page?: number
  pageSize?: number
  status?: string
  role?: string
  search?: string
  timezone?: string
  sortBy?: string
  sortOrder?: string
}) {
  return request<PlatformSub2APIUserListResponse>(
    `/api/platform/usage/sub2api/users${queryString(query)}`
  )
}

export function fetchPlatformSub2APIUserKeys(userId: number, timezone = "Asia/Shanghai") {
  return request<PlatformSub2APIUserKeyListResponse>(
    `/api/platform/usage/sub2api/users/${encodeURIComponent(String(userId))}/api-keys${queryString({ timezone })}`
  )
}

/** @deprecated Use the resource-specific fetchPlatformOps* functions. */
export function fetchPlatformOpsOverview() {
  return request<PlatformOpsOverview>("/api/platform/ops/overview")
}

export function fetchPlatformOpsRuntime() {
  return request<PlatformOpsRuntime>("/api/platform/ops/runtime")
}

export function fetchPlatformOpsDatabase() {
  return request<PlatformOpsDatabase>("/api/platform/ops/database")
}

export function fetchPlatformOpsInfrastructure() {
  return request<PlatformOpsInfrastructure>("/api/platform/ops/infrastructure")
}

export function fetchPlatformOpsJitsi() {
  return request<PlatformOpsJitsi>("/api/platform/ops/jitsi")
}

export function fetchPlatformOpsSpeech() {
  return request<PlatformOpsSpeech>("/api/platform/ops/speech")
}

export function fetchPlatformOpsTranslation() {
  return request<PlatformOpsTranslation>("/api/platform/ops/translation")
}

export function fetchPlatformOpsAR() {
  return request<PlatformOpsAR>("/api/platform/ops/ar")
}

export function fetchPlatformOpsSub2API() {
  return request<PlatformOpsSub2API>("/api/platform/ops/sub2api")
}

export function fetchPlatformOpsAccess() {
  return request<PlatformOpsAccess>("/api/platform/ops/access")
}

export function fetchPlatformOpsPipeline() {
  return request<PlatformOpsPipeline>("/api/platform/ops/sync-queue")
}

export function fetchPlatformOpsKnowledgeQueue() {
  return request<PlatformOpsKnowledgeQueue>("/api/platform/ops/knowledge-queue")
}

export function fetchPlatformOpsNotificationQueue() {
  return request<PlatformOpsNotificationQueue>("/api/platform/ops/notification-queue")
}

export function fetchPlatformIntegrationSettings() {
  return request<PlatformIntegrationSettings>("/api/platform/integrations/config")
}

export function fetchPlatformSystemPolicy() {
  return request<PlatformSystemPolicySettings>("/api/platform/system-config/policy")
}

export function updatePlatformSystemPolicy(payload: {
  freeTenantDocumentLimit: number
  paidTenantDocumentLimit: number
  knowledgeDocumentMaxMB: number
}) {
  return request<PlatformSystemPolicySettings>("/api/platform/system-config/policy", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updatePlatformIntegration(payload: PlatformIntegrationMutation) {
  return request<PlatformIntegrationSettings>("/api/platform/integrations/update", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function testPlatformIntegration(payload: PlatformIntegrationMutation) {
  return request<PlatformIntegrationTestResult>("/api/platform/integrations/test", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function fetchPlatformAIModelWorkspace() {
  return request<PlatformAIModelWorkspace>("/api/platform/models/workspace")
}

export function savePlatformAIProvider(payload: { host: string; adminApiKey?: string; translationApiKey?: string; defaultLlmModel?: string }) {
  return request<PlatformAIModelWorkspace>("/api/platform/models/provider/save", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function provisionPlatformAITenant(payload: {
  tenantId: number
  concurrency: number
  balance: number
  rpmLimit: number
  defaultKeyExpiresAt?: string
}) {
  return request<PlatformAIModelWorkspace>("/api/platform/models/tenant/provision", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function fetchPlatformAITenantBilling(query: { tenantId: number; timezone?: string }) {
  return request<PlatformAITenantBilling>(
    `/api/platform/models/tenant/billing${queryString(query as Record<string, string | number | undefined>)}`
  )
}

export function rechargePlatformAITenant(payload: {
  tenantId: number
  amount: number
  notes?: string
  timezone?: string
}) {
  return request<PlatformAITenantBilling>("/api/platform/models/tenant/recharge", {
    method: "POST",
    body: JSON.stringify({ ...payload, operation: "add" }),
  })
}

export function fetchPlatformAITenantUsage(query: {
  tenantId: number
  startDate?: string
  endDate?: string
  modelSource?: string
  timezone?: string
  page?: number
  pageSize?: number
  sortBy?: string
  sortOrder?: string
}) {
  return request<PlatformAIUsageDetailResponse>(
    `/api/platform/models/tenant/usage${queryString(query as Record<string, string | number | undefined>)}`
  )
}
