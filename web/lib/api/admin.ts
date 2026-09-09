import { readSession } from "@/lib/auth"
import { getRequestTenantId, request, requestBlob } from "@/lib/api/client"
import { createAccessTokenWebSocket, createWebSocketBaseUrl } from "@/lib/api/websocket"
import { translateCurrentMessage } from "@/i18n/messages"

export type Paging = {
  page: number
  limit: number
  total: number
}

export type PageResult<T> = {
  results: T[]
  page: Paging
}

export type CursorResult<T> = {
  results?: T[] | null
  cursor: string
  hasMore: boolean
}

export type AdminUser = {
  id: number
  username: string
  nickname: string
  avatar: string
  mobile?: string
  email?: string
  locale?: string
  timezone?: string
  status: number
  isSystem: boolean
  lastLoginAt?: string
  lastLoginIp?: string
  roles?: AdminRole[]
  permissions?: string[]
}

export type UpdateAdminUserPayload = {
  id: number
  nickname: string
  avatar: string
  mobile: string | null
  email: string | null
  remark: string
}

export type CreateAdminUserPayload = {
  username: string
  nickname: string
  avatar: string
  mobile: string | null
  email: string | null
  remark: string
  roleIds: number[]
}

export type CreateUserResult = {
  user: AdminUser
  password: string
}

export type ResetPasswordResult = {
  password: string
}

export type AdminRole = {
  id: number
  name: string
  code: string
  status: number
  isSystem: boolean
  sortNo: number
  permissions?: string[]
}

export type CreateAdminRolePayload = {
  name: string
  code: string
  remark: string
}

export type AdminPermission = {
  id: number
  name: string
  code: string
  type: string
  groupName: string
  method: string
  apiPath: string
  status: number
  sortNo: number
}

export type ConversationTag = {
  id: number
  name: string
  color: string
}

export type ConversationParticipant = {
  id: number
  participantType: string
  participantId: number
  externalParticipantId?: string
  joinedAt?: string
  leftAt?: string
  status: number
}

export type AdminConversation = {
  id: number
  channelId: number
  customerId: number
  customerName: string
  tenantId?: number
  productId?: number
  productModelId?: number
  deviceId?: number
  serviceCodeId?: number
  status: number
  serviceMode: number
  humanHandoffEnabled?: boolean
  priority: number
  currentAssigneeId: number
  currentAssigneeName?: string
  lastMessageId: number
  lastMessageAt?: string
  lastActiveAt?: string
  lastMessageSummary?: string
  customerUnreadCount: number
  agentUnreadCount: number
  customerLastReadMessageId: number
  customerLastReadAt?: string
  agentLastReadMessageId: number
  agentLastReadAt?: string
  closedAt?: string
  closedBy: number
  closedByName?: string
  closeReason?: string
  participants?: ConversationParticipant[]
}

export type AdminConversationDetail = AdminConversation & {
  participants?: ConversationParticipant[]
}

export type AdminMessage = {
  id: number
  conversationId: number
  workflowRunId?: number
  clientMsgId?: string
  senderType: string
  senderId: number
  senderName?: string
  senderAvatar?: string
  messageType: string
  content: string
  payload?: string
  sendStatus: number
  sentAt?: string
  deliveredAt?: string
  readAt?: string
  customerRead: boolean
  customerReadAt?: string
  agentRead: boolean
  agentReadAt?: string
  recalledAt?: string
  quotedMessageId?: number
}

export type AdminQuickReply = {
  id: number
  groupName: string
  title: string
  content: string
  status: number
  sortNo: number
  createdBy: number
}

export type AdminChannel = {
  id: number
  channelType: string
  channelId: string
  aiAgentId: number
  aiAgentName?: string
  name: string
  configJson: string
  status: number
  remark: string
}

export type WxWorkKFAccount = {
  openKfId: string
  name: string
  avatar: string
  managePrivilege: boolean
}

export type CreateAdminChannelPayload = {
  channelType: string
  aiAgentId: number
  name: string
  configJson: string
  status: number
  remark: string
}

export type UpdateAdminChannelPayload = CreateAdminChannelPayload & {
  id: number
}

export type ResetChannelUserTokenSecretResult = {
  userTokenSecret: string
}

export type AIAgent = {
  id: number
  tenantId: number
  productId: number
  productName?: string
  productCode?: string
  productDefaultKnowledgeBaseId?: number
  source?: string
  reviewStatus?: "unreviewed" | "pending" | "approved" | "rejected" | string
  reviewStatusName?: string
  reviewComment?: string
  reviewedAt?: string
  reviewedByName?: string
  name: string
  description: string
  status: number
  statusName: string
  aiConfigId: number
  aiConfigName?: string
  llmModelName: string
  serviceMode: number
  serviceModeName: string
  systemPrompt: string
  welcomeMessage: string
  replyTimeoutSeconds: number
  teams: { id: number; name: string }[]
  handoffMode: number
  handoffModeName: string
  fallbackMode: number
  fallbackModeName: string
  fallbackMessage: string
  knowledgeIds: number[]
  knowledgeBaseNames: string[]
  skillIds: number[]
  skills: { id: number; name: string }[]
  directTools: {
    toolCode: string
    serverCode: string
    toolName: string
    title: string
    description: string
    arguments?: Record<string, string>
  }[]
  graphTools: string[]
  workflowId: number
  workflowVersionId: number
  activeReleaseId: number
  draftRevision: number
  workflowPublished: boolean
  workflowState: string
  workflowStateText: string
  sortNo: number
  createdAt: string
  updatedAt: string
  createUserName: string
  updateUserName: string
}

export type AIAgentCapability = {
  type: "llm" | "embedding"
  available: boolean
  modelName: string
  models: string[]
  dimension: number
}

export type AIAgentCapabilities = {
  generatedAt: string
  capabilities: AIAgentCapability[]
}

export type CreateAIAgentPayload = {
  name: string
  description: string
  aiConfigId: number
  llmModelName: string
  serviceMode: number
  systemPrompt: string
  welcomeMessage: string
  replyTimeoutSeconds: number
  teamIds: number[]
  handoffMode: number
  fallbackMode: number
  fallbackMessage: string
  knowledgeIds: number[]
  skillIds: number[]
  directTools: {
    toolCode: string
    serverCode: string
    toolName: string
    title: string
    description: string
    arguments?: Record<string, string>
  }[]
  graphTools: string[]
}

export type UpdateAIAgentPayload = CreateAIAgentPayload & {
  id: number
}

export type AIWorkflowPosition = {
  x: number
  y: number
}

export type AIWorkflowVariableType =
  | "string"
  | "number"
  | "integer"
  | "boolean"
  | "object"
  | "array<string>"
  | "array<int>"
  | "array<object>"
  | "any"

export type AIWorkflowVariableSelector = {
  nodeId: string
  field: string
}

export type AIWorkflowVariableSpec = {
  name: string
  type: AIWorkflowVariableType
  required?: boolean
  description: string
}

export type AIWorkflowDefinition = {
  schemaVersion: number
  entryNodeId: string
  modelPolicy?: {
    credentialChain?: Array<"tenant_default" | "product">
  }
  runtimeVariables?: AIWorkflowRuntimeVariable[]
  nodes: {
    id: string
    type: string
    name: string
    position: AIWorkflowPosition
    config: Record<string, unknown>
    inputs?: Record<string, AIWorkflowVariableSelector>
    errorTargetNodeId?: string
  }[]
  edges: {
    id: string
    source: string
    target: string
  }[]
}

export type AIWorkflow = {
  id: number
  tenantId: number
  code: string
  scope: "platform" | "tenant" | string
  name: string
  description: string
  agentId: number
  status: number
  draftDefinition: AIWorkflowDefinition
  publishedVersionId: number
  currentStableVersionId: number
  sourceWorkflowId: number
  sourceVersionId: number
  locked: boolean
  humanHandoffEnabled: boolean
  sortNo: number
  createdAt: string
  updatedAt: string
  createUserName: string
  updateUserName: string
}

export type AIWorkflowRuntimeVariable = {
  key: string
  type: string
  source: string
  required?: boolean
  description?: string
}

export type AIWorkflowVersion = {
  id: number
  workflowId: number
  version: number
  status: number
  definition: AIWorkflowDefinition
  definitionHash: string
  releaseChannel: "stable" | "preview" | "deprecated" | "revoked" | string
  schemaVersion: number
  changeSummary: string
  sourceVersionId: number
  publishedAt: string
  publishedById: number
  publishedByName: string
  createdAt: string
  updatedAt: string
}

export type AIWorkflowNodeSpec = {
  type: string
  title: string
  description: string
  riskLevel: "low" | "medium" | "high"
  interruptible: boolean
  requiresConfirmationPredecessor: boolean
  configSchema?: unknown
  inputSchema?: AIWorkflowVariableSpec[]
  outputSchema?: AIWorkflowVariableSpec[]
  defaultInputs?: Record<string, AIWorkflowVariableSelector>
}

export type AIWorkflowValidationResult = {
  valid: boolean
  errors: {
    field: string
    message: string
  }[]
}

export type CreateAIWorkflowPayload = {
  name: string
  description: string
  agentId: number
  definition: AIWorkflowDefinition
}

export type CreateAdminQuickReplyPayload = {
  groupName: string
  title: string
  content: string
  status: number
  sortNo: number
}

export type UpdateAdminQuickReplyPayload = CreateAdminQuickReplyPayload & {
  id: number
}

export type SkillDefinition = {
  id: number
  tenantId: number
  name: string
  description: string
  instruction: string
  examples: string[]
  toolWhitelist: string[]
  status: number
  statusName: string
  remark: string
  createdAt: string
  updatedAt: string
  createUserName: string
  updateUserName: string
}

export type CreateSkillDefinitionPayload = {
  name: string
  description: string
  instruction: string
  examples: string[]
  toolWhitelist: string[]
  remark: string
}

export type UpdateSkillDefinitionPayload = CreateSkillDefinitionPayload & {
  id: number
}

export type SkillDebugRunPayload = {
  aiAgentId: number
  conversationId?: number
  skillDefinitionId: number
  userMessage: string
}

export type SkillDebugResumePayload = {
  aiAgentId: number
  conversationId?: number
  checkPointId: string
  userMessage: string
}

export type SkillDebugRunResult = {
  skillDefinitionId: number
  skillName: string
  replyText: string
  planReason: string
  skillRouteTrace: string
  toolWhitelist: string[]
  exposedToolCodes: string[]
  invokedToolCodes: string[]
  toolSearchTrace: string
  graphToolTrace: string
  graphToolCode: string
  interruptType: string
  checkPointId: string
  interrupted: boolean
  traceData: string
  errorMessage: string
  conversationId: number
  aiAgentId: number
}

export type MCPConnectionResult = {
  serverCode: string
  endpoint: string
  protocol: string
  serverName: string
  version: string
}

export type MCPServerInfo = {
  code: string
  enabled: boolean
  endpoint: string
  timeoutMs: number
}

export type MCPToolInfo = {
  name: string
  title: string
  description: string
  inputSchema: unknown
  outputSchema?: unknown
}

export type MCPToolSourceType = "mcp" | "graph" | "builtin"

export type MCPToolCatalogItem = {
  toolCode: string
  serverCode: string
  toolName: string
  sourceType: MCPToolSourceType
  autoInjected: boolean
  title: string
  description: string
  inputSchema: unknown
  outputSchema?: unknown
}

export type MCPToolResultContent = {
  type: string
  text?: string
  data?: unknown
}

export type MCPToolCallResult = {
  serverCode: string
  toolName: string
  isError: boolean
  content: MCPToolResultContent[]
  structuredContent?: unknown
}

export type AIWorkflowNodeRun = {
  id: number
  workflowRunId: number
  nodeId: string
  nodeType: string
  attempt: number
  idempotencyKey: string
  status: number
  statusName: string
  inputPreview: string
  outputPreview: string
  errorMessage: string
  startedAt: string
  endedAt: string
  durationMs: number
}

export type AIWorkflowSkillCandidate = {
  id: number
  name: string
  description: string
}

export type AIWorkflowSkillAudit = {
  state: "selected" | "not_selected" | "not_enabled" | string
  middlewareEnabled: boolean
  candidateSkills: AIWorkflowSkillCandidate[]
  selectedSkillId: number
  selectedSkillName: string
  selectedSkillDescription: string
  matchReason: string
  routeTrace: string
  exposedToolCodes: string[]
  invokedToolCodes: string[]
  sourceMessageId: number
  createdAt: string
}

export type AIWorkflowRun = {
  id: number
  tenantId: number
  productId: number
  workflowId: number
  workflowVersionId: number
  definitionHash: string
  runtimeEngine: string
  workflowVersion: number
  workflowName: string
  conversationId: number
  aiAgentId: number
  agentReleaseId: number
  aiAgentName: string
  messageId: number
  status: number
  statusName: string
  startedAt: string
  endedAt: string
  durationMs: number
  interruptType: string
  interruptNodeId: string
  errorMessage: string
  configSnapshot: string
  traceData: string
  createdAt: string
  updatedAt: string
  definition?: AIWorkflowDefinition
  nodes?: AIWorkflowNodeRun[]
  skillAudit?: AIWorkflowSkillAudit
  humanHandling?: AIWorkflowHumanHandling
}

export type AIWorkflowTestNodeResult = {
  nodeId: string
  nodeType: string
  status: "completed" | "failed" | "interrupted" | "running" | string
  inputPreview: string
  outputPreview: string
  errorMessage: string
  durationMs: number
}

export type AIWorkflowTestRunResult = {
  workflowId: number
  aiAgentId: number
  aiAgentName: string
  runtimeEngine: string
  status: "completed" | "failed" | "interrupted" | "error" | string
  replyText: string
  failedNodeId: string
  interruptNodeId: string
  errorMessage: string
  durationMs: number
  sideEffects: "suppressed" | string
  modelCredentialScope: "tenant_default" | "product" | string
  modelApiKeyId: string
  modelCredentialFallback: boolean
  nodePath: string[]
  nodes: AIWorkflowTestNodeResult[]
}

export type AIWorkflowHumanHandling = {
  handoffOccurred: boolean
  handoffAt: string
  handoffReason: string
  handledByHuman: boolean
  handlerUserId: number
  handlerName: string
  firstHumanReplyMessageId: number
  firstHumanReplyAt: string
  conversationStatus: number
  conversationStatusName: string
}

export type AdminAgentProfile = {
  id: number
  tenantId: number
  userId: number
  teamId: number
  teamIds?: number[]
  teamName?: string
  username?: string
  nickname?: string
  agentCode: string
  displayName: string
  avatar: string
  serviceStatus: number
  maxConcurrentCount: number
  priorityLevel: number
  autoAssignEnabled: boolean
  teamDispatchEnabled: boolean
  dispatchWeight?: number
  receiveOfflineMessage: boolean
  lastOnlineAt?: string
  lastStatusAt?: string
  remark: string
}

export type AdminDispatchCandidate = AdminAgentProfile & {
  assignmentMode: string
  teamEligible: boolean
  workStatusConfirmed: boolean
  reachable: boolean
  capacityAvailable: boolean
  activeConversationCount: number
  openTicketCount: number
  workload: number
  loadRate: number
  capacityRemaining: number
}

export type CreateAdminAgentProfilePayload = {
  userId: number
  teamId: number
  agentCode: string
  displayName: string
  avatar: string
  serviceStatus: number
  maxConcurrentCount: number
  priorityLevel: number
  autoAssignEnabled: boolean
  receiveOfflineMessage: boolean
  lastOnlineAt?: string
  lastStatusAt?: string
  remark: string
}

export type UpdateAdminAgentProfilePayload =
  CreateAdminAgentProfilePayload & {
    id: number
  }

export type AdminAgentTeam = {
  id: number
  tenantId: number
  parentId: number
  productId: number
  productName?: string
  departmentId: number
  teamType: string
  systemManaged: boolean
  name: string
  leaderUserId: number
  leaderUsername?: string
  leaderNickname?: string
  assignmentMode: string
  scheduleEnforced: boolean
  scheduleVersion: number
  status: number
  description: string
  remark: string
}

export type CreateAdminAgentTeamPayload = {
  name: string
  leaderUserId: number
  assignmentMode?: string
  status: number
  description: string
  remark: string
}

export type UpdateAdminAgentTeamPayload = CreateAdminAgentTeamPayload & {
  id: number
}

export type AdminAgentTeamSchedule = {
  id: number
  tenantId: number
  teamId: number
  teamName?: string
  userId: number
  userName?: string
  userAvatar?: string
  repeatType: "once" | "weekly"
  dayType: "work" | "rest"
  weekday: number
  startMinute: number
  endMinute: number
  startTime?: string
  endTime?: string
  timezone: string
  effectiveFrom?: string
  effectiveUntil?: string
  publishStatus: "draft" | "published" | "archived"
  version: number
  startAt: string
  endAt: string
  remark: string
}

export type AdminAgentScheduleException = {
  id: number
  tenantId: number
  userId: number
  userName?: string
  userAvatar?: string
  requestKey?: string
  exceptionType: "leave"
  startAt: string
  endAt: string
  approvalStatus: "pending" | "approved" | "rejected" | "cancelled"
  reason?: string
  reviewNote?: string
  requestedAt: string
  reviewedAt?: string
  reviewerUserId?: number
  reviewerUserName?: string
}

export type AdminAgentTeamMemberAvailability = {
  userId: number
  memberId: number
  profileId: number
  username?: string
  nickname?: string
  displayName: string
  avatar?: string
  agentCode?: string
  dispatchEnabled: boolean
  dispatchWeight: number
  autoAssignEnabled: boolean
  serviceStatus: number
  maxConcurrentCount: number
  workStatus: string
  workStatusNote?: string
  workStatusConfirmed: boolean
  availableNow: boolean
  unavailableReason?: string
  workdays: number[]
  startTime: string
  endTime: string
  timezone: string
  activeLeave?: AdminAgentScheduleException
  pendingLeave?: AdminAgentScheduleException
}

export type CreateAdminAgentTeamSchedulePayload = {
  teamId: number
  userId?: number
  repeatType?: "once" | "weekly"
  dayType?: "work" | "rest"
  weekday?: number
  startTime?: string
  endTime?: string
  startAt: string
  endAt: string
  timezone?: string
  effectiveFrom?: string
  effectiveUntil?: string
  remark: string
}

export type UpdateAdminAgentTeamSchedulePayload =
  CreateAdminAgentTeamSchedulePayload & {
    id: number
  }

export type BatchAdminAgentTeamSchedulePayload = {
  teamIds: number[]
  startDate: string
  endDate: string
  weekdays: number[]
  startTime: string
  endTime: string
  remark: string
}

export type AdminAgentTeamScheduleBatchPreviewItem = {
  teamId: number
  teamName: string
  date: string
  weekday: number
  startAt: string
  endAt: string
  remark: string
  conflict: boolean
  conflictReason: string
}

export type AdminAgentTeamScheduleBatchPreview = {
  total: number
  conflict: boolean
  items: AdminAgentTeamScheduleBatchPreviewItem[]
}

export type AdminAgentTeamScheduleBatchGenerateResult = {
  created: number
}

export type AdminAgentTeamScheduleDraftResult = {
  teamId: number
  version: number
  count: number
}

export type AdminAgentTeamScheduleTemplate = {
  tenantId: number
  workdays: number[]
  startTime: string
  endTime: string
  timezone: string
}

export type AdminAgentTeamSchedulePublishResult = {
  teamId: number
  version: number
  published: number
  missingWeekdays: number[]
}

function rejectDeprecatedAgentTeamScheduleWrite<T>(): Promise<T> {
  return Promise.reject(
    new Error(translateCurrentMessage("miscTailExtract.libMisc.admin.agentTeamScheduleDeprecated"))
  )
}

function toQueryString(query?: Record<string, string | number | undefined>) {
  if (!query) {
    return ""
  }

  const params = new URLSearchParams()
  Object.entries(query).forEach(([key, value]) => {
    if (value === undefined || value === "") {
      return
    }
    params.set(key, String(value))
  })
  const output = params.toString()
  return output ? `?${output}` : ""
}

export function createAdminWebSocketUrl() {
  return `${createWebSocketBaseUrl()}/api/ws/dashboard`
}

export function createAdminWebSocket() {
  const session = readSession()
  if (!session?.accessToken) {
    throw new Error(translateCurrentMessage("api.authExpired"))
  }
  return createAccessTokenWebSocket(createAdminWebSocketUrl(), session.accessToken)
}

export function fetchChannels(
  query?: Record<string, string | number | undefined>
) {
  return request<PageResult<AdminChannel>>(
    `/api/dashboard/channel/list${toQueryString(query)}`
  )
}

export function fetchChannel(id: number) {
  return request<AdminChannel>(`/api/dashboard/channel/${id}`)
}

export function fetchWxWorkKFAccounts() {
  return request<WxWorkKFAccount[]>("/api/dashboard/channel/wxwork/kf/accounts")
}

export function createChannel(payload: CreateAdminChannelPayload) {
  return request<AdminChannel>("/api/dashboard/channel/create", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updateChannel(payload: UpdateAdminChannelPayload) {
  return request<void>("/api/dashboard/channel/update", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updateChannelStatus(id: number, status: number) {
  return request<void>("/api/dashboard/channel/update_status", {
    method: "POST",
    body: JSON.stringify({ id, status }),
  })
}

export function resetChannelUserTokenSecret(id: number) {
  return request<ResetChannelUserTokenSecretResult>(
    "/api/dashboard/channel/reset_user_token_secret",
    {
      method: "POST",
      body: JSON.stringify({ id }),
    }
  )
}

export function deleteChannel(id: number) {
  return request<void>("/api/dashboard/channel/delete", {
    method: "POST",
    body: JSON.stringify({ id }),
  })
}

export function fetchAIAgents(
  query?: Record<string, string | number | undefined>
) {
  return request<PageResult<AIAgent>>(
    `/api/dashboard/ai-agent/list${toQueryString(query)}`
  )
}

export function fetchAIAgentsAll(query?: Record<string, string | number | undefined>) {
  return request<AIAgent[]>(
    `/api/dashboard/ai-agent/list_all${toQueryString(query)}`
  )
}

export function fetchAIAgent(id: number) {
  return request<AIAgent>(`/api/dashboard/ai-agent/${id}`)
}

export function fetchAIAgentCapabilities() {
  return request<AIAgentCapabilities>("/api/dashboard/ai-agent/capabilities")
}

export function createAIAgent(payload: CreateAIAgentPayload) {
  return request<AIAgent>("/api/dashboard/ai-agent/create", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updateAIAgent(payload: UpdateAIAgentPayload) {
  return request<void>("/api/dashboard/ai-agent/update", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function deleteAIAgent(id: number) {
  return request<void>("/api/dashboard/ai-agent/delete", {
    method: "POST",
    body: JSON.stringify({ id }),
  })
}

export function updateAIAgentSort(ids: number[]) {
  return request<void>("/api/dashboard/ai-agent/update_sort", {
    method: "POST",
    body: JSON.stringify(ids),
  })
}

export function updateAIAgentStatus(id: number, status: number) {
  return request<void>("/api/dashboard/ai-agent/update_status", {
    method: "POST",
    body: JSON.stringify({ id, status }),
  })
}

export function fetchAIAgentWorkflow(agentId: number) {
  return request<AIWorkflow>(`/api/dashboard/ai-agent/${agentId}/workflow`)
}

export function saveAIAgentWorkflow(payload: CreateAIWorkflowPayload) {
  return request<AIWorkflow>("/api/dashboard/ai-agent/workflow/save", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function fetchAIWorkflowNodeSpecs() {
  return request<AIWorkflowNodeSpec[]>("/api/dashboard/ai-workflow/node-spec/list")
}

export function fetchAIWorkflowDefaultDefinition() {
  return request<AIWorkflowDefinition>("/api/dashboard/ai-workflow/default-definition")
}

export function fetchAIWorkflowVersions(query?: Record<string, string | number | undefined>) {
  return request<PageResult<AIWorkflowVersion>>(
    `/api/dashboard/ai-workflow/version/list${toQueryString(query)}`
  )
}

export function fetchAIWorkflowVersion(id: number) {
  return request<AIWorkflowVersion>(`/api/dashboard/ai-workflow/version/${id}`)
}

export function validateAIWorkflow(definition: AIWorkflowDefinition) {
  return request<AIWorkflowValidationResult>("/api/dashboard/ai-agent/workflow/validate", {
    method: "POST",
    body: JSON.stringify({ definition }),
  })
}

export function publishAIAgentWorkflow(agentId: number, definition: AIWorkflowDefinition) {
  return request<AIWorkflowVersion>("/api/dashboard/ai-agent/workflow/publish", {
    method: "POST",
    body: JSON.stringify({ agentId, definition }),
  })
}

export function fetchUsers(query?: Record<string, string | number | undefined>) {
  return request<PageResult<AdminUser>>(
    `/api/dashboard/user/list${toQueryString(query)}`
  )
}

export function fetchUsersAll(query?: Record<string, string | number | undefined>) {
  return request<AdminUser[]>(
    `/api/dashboard/user/list_all${toQueryString(query)}`
  )
}

export function createUser(payload: CreateAdminUserPayload) {
  return request<CreateUserResult>("/api/dashboard/user/create", {
    method: "POST",
    body: JSON.stringify({
      username: payload.username,
      nickname: payload.nickname,
      avatar: payload.avatar,
      mobile: payload.mobile,
      email: payload.email,
      remark: payload.remark,
      roleIds: payload.roleIds,
    }),
  })
}

export function updateUser(payload: UpdateAdminUserPayload) {
  return request<void>("/api/dashboard/user/update", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function fetchUserDetail(id: number) {
  return request<AdminUser>(`/api/dashboard/user/${id}`)
}

export function updateUserStatus(id: number, status: number) {
  return request<void>("/api/dashboard/user/update_status", {
    method: "POST",
    body: JSON.stringify({ id, status }),
  })
}

export function resetUserPassword(userId: number) {
  return request<ResetPasswordResult>("/api/dashboard/user/reset_password", {
    method: "POST",
    body: JSON.stringify({ userId }),
  })
}

export function changeSelfPassword(password: string) {
  return request<void>("/api/dashboard/user/change_password", {
    method: "POST",
    body: JSON.stringify({ password }),
  })
}

export function assignUserRoles(userId: number, roleIds: number[]) {
  return request<void>("/api/dashboard/user/assign_role", {
    method: "POST",
    body: JSON.stringify({ userId, roleIds }),
  })
}

export function fetchRoles(query?: Record<string, string | number | undefined>) {
  return request<PageResult<AdminRole>>(
    `/api/dashboard/role/list${toQueryString(query)}`
  )
}

export function fetchRoleListAll() {
  return request<AdminRole[]>("/api/dashboard/role/list_all")
}

export function fetchRoleDetail(id: number) {
  return request<AdminRole>(`/api/dashboard/role/${id}`)
}

export function createRole(payload: CreateAdminRolePayload) {
  return request<AdminRole>("/api/dashboard/role/create", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function assignRolePermissions(roleId: number, permissionIds: number[]) {
  return request<void>("/api/dashboard/role/assign_permission", {
    method: "POST",
    body: JSON.stringify({ roleId, permissionIds }),
  })
}

export function updateRoleSort(ids: number[]) {
  return request<void>("/api/dashboard/role/update_sort", {
    method: "POST",
    body: JSON.stringify(ids),
  })
}

export function fetchPermissions(
  query?: Record<string, string | number | undefined>
) {
  return request<PageResult<AdminPermission>>(
    `/api/dashboard/permission/list${toQueryString(query)}`
  )
}

const PERMISSION_CATALOG_PAGE_SIZE = 100

export async function fetchPermissionCatalog(
  query?: Record<string, string | number | undefined>
) {
  const { page: _page, limit: _limit, ...rest } = query ?? {}
  const results: AdminPermission[] = []
  let page = 1

  for (;;) {
    const response = await fetchPermissions({
      ...rest,
      page,
      limit: PERMISSION_CATALOG_PAGE_SIZE,
    })
    results.push(...response.results)

    const total = response.page?.total ?? results.length
    if (results.length >= total || response.results.length < PERMISSION_CATALOG_PAGE_SIZE) {
      return results
    }
    page += 1
  }
}

export function fetchConversations(
  query?: Record<string, string | number | undefined>
) {
  return request<PageResult<AdminConversation>>(
    `/api/dashboard/conversation/list${toQueryString(query)}`
  )
}

export function fetchConversationDetail(id: number) {
  return request<AdminConversationDetail>(`/api/dashboard/conversation/${id}`)
}

export function fetchConversationMessages(
  query?: Record<string, string | number | undefined>
) {
  return request<CursorResult<AdminMessage>>(
    `/api/dashboard/conversation/message_list${toQueryString(query)}`
  )
}

export function assignConversation(
  conversationId: number,
  assigneeId: number,
  reason: string
) {
  return request<void>("/api/dashboard/conversation/assign", {
    method: "POST",
    body: JSON.stringify({ conversationId, assigneeId, reason }),
  })
}

export function dispatchConversation(conversationId: number) {
  return request<void>("/api/dashboard/conversation/dispatch", {
    method: "POST",
    body: JSON.stringify({ conversationId }),
  })
}

export function transferConversation(
  conversationId: number,
  toUserId: number,
  reason: string
) {
  return request<void>("/api/dashboard/conversation/transfer", {
    method: "POST",
    body: JSON.stringify({ conversationId, toUserId, reason }),
  })
}

export function closeConversation(conversationId: number, closeReason: string) {
  return request<void>("/api/dashboard/conversation/close", {
    method: "POST",
    body: JSON.stringify({ conversationId, closeReason }),
  })
}

export function markConversationRead(conversationId: number, messageId = 0) {
  return request<void>("/api/dashboard/conversation/read", {
    method: "POST",
    body: JSON.stringify({ conversationId, messageId }),
  })
}

export function sendConversationMessage(payload: {
  conversationId: number
  messageType: string
  content: string
  payload?: string
  clientMsgId?: string
}) {
  return request<AdminMessage>("/api/dashboard/conversation/send_message", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function recallConversationMessage(messageId: number) {
  return request<AdminMessage>("/api/dashboard/conversation/recall_message", {
    method: "POST",
    body: JSON.stringify({ messageId }),
  })
}

export function fetchQuickReplies(
  query?: Record<string, string | number | undefined>
) {
  return request<PageResult<AdminQuickReply>>(
    `/api/dashboard/quick-reply/list${toQueryString(query)}`
  )
}

export function fetchQuickReplyListAll() {
  return request<AdminQuickReply[]>("/api/dashboard/quick-reply/list_all")
}

export function fetchQuickReply(id: number) {
  return request<AdminQuickReply>(`/api/dashboard/quick-reply/${id}`)
}

export function createQuickReply(payload: CreateAdminQuickReplyPayload) {
  return request<AdminQuickReply>("/api/dashboard/quick-reply/create", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updateQuickReply(payload: UpdateAdminQuickReplyPayload) {
  return request<void>("/api/dashboard/quick-reply/update", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function deleteQuickReply(id: number) {
  return request<void>("/api/dashboard/quick-reply/delete", {
    method: "POST",
    body: JSON.stringify({ id }),
  })
}

export function fetchSkillDefinitions(
  query?: Record<string, string | number | undefined>
) {
  return request<PageResult<SkillDefinition>>(
    `/api/dashboard/skill-definition/list${toQueryString(query)}`
  )
}

export function fetchSkillDefinitionsAll(
  query?: Record<string, string | number | undefined>
) {
  return request<SkillDefinition[]>(
    `/api/dashboard/skill-definition/list_all${toQueryString(query)}`
  )
}

export function fetchSkillDefinition(id: number) {
  return request<SkillDefinition>(`/api/dashboard/skill-definition/${id}`)
}

export function createSkillDefinition(payload: CreateSkillDefinitionPayload) {
  return request<SkillDefinition>("/api/dashboard/skill-definition/create", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updateSkillDefinition(payload: UpdateSkillDefinitionPayload) {
  return request<void>("/api/dashboard/skill-definition/update", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function fetchAIWorkflowRuns(
  query?: Record<string, string | number | undefined>
) {
  return request<PageResult<AIWorkflowRun>>(
    `/api/dashboard/ai-workflow/run/list${toQueryString(query)}`
  )
}

export function fetchAIWorkflowRun(id: number) {
  return request<AIWorkflowRun>(`/api/dashboard/ai-workflow/run/${id}`)
}

export function updateSkillDefinitionStatus(id: number, status: number) {
  return request<void>("/api/dashboard/skill-definition/update_status", {
    method: "POST",
    body: JSON.stringify({ id, status }),
  })
}

export function deleteSkillDefinition(id: number) {
  return request<void>("/api/dashboard/skill-definition/delete", {
    method: "POST",
    body: JSON.stringify({ id }),
  })
}

export function restoreSkillDefinition(id: number) {
  return request<void>("/api/dashboard/skill-definition/restore", {
    method: "POST",
    body: JSON.stringify({ id }),
  })
}

export function debugRunSkillDefinition(payload: SkillDebugRunPayload) {
  return request<SkillDebugRunResult>("/api/dashboard/skill-definition/debug_run", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function debugResumeSkillDefinition(payload: SkillDebugResumePayload) {
  return request<SkillDebugRunResult>("/api/dashboard/skill-definition/debug_resume", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function testMCPConnection(serverCode: string) {
  return request<MCPConnectionResult>("/api/dashboard/mcp/test_connection", {
    method: "POST",
    body: JSON.stringify({ serverCode }),
  })
}

export function listMCPServers() {
  return request<MCPServerInfo[]>("/api/dashboard/mcp/list_servers")
}

export function listMCPTools(serverCode: string) {
  return request<MCPToolInfo[]>("/api/dashboard/mcp/list_tools", {
    method: "POST",
    body: JSON.stringify({ serverCode }),
  })
}

export function fetchMCPCatalog() {
  return request<MCPToolCatalogItem[]>("/api/dashboard/mcp/catalog")
}

export function callMCPTool(payload: {
  serverCode: string
  toolName: string
  arguments: Record<string, unknown>
}) {
  return request<MCPToolCallResult>("/api/dashboard/mcp/call_tool", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function fetchAgentProfiles(
  query?: Record<string, string | number | undefined>
) {
  return request<PageResult<AdminAgentProfile>>(
    `/api/dashboard/agent/list${toQueryString(query)}`,
    { tenantId: getRequestTenantId() },
  )
}

export function fetchAgentProfile(id: number) {
  return request<AdminAgentProfile>(`/api/dashboard/agent/${id}`, {
    tenantId: getRequestTenantId(),
  })
}

export function fetchAgentProfilesAll(
  query?: Record<string, string | number | undefined>
) {
  return request<AdminAgentProfile[]>(
    `/api/dashboard/agent/list_all${toQueryString(query)}`,
    { tenantId: getRequestTenantId() },
  )
}

export function fetchAgentDispatchCandidates(
  query?: Record<string, string | number | undefined>
) {
  return request<AdminDispatchCandidate[]>(
    `/api/dashboard/agent/dispatch-candidates${toQueryString(query)}`,
    { tenantId: getRequestTenantId() },
  )
}

export function createAgentProfile(payload: CreateAdminAgentProfilePayload) {
  return request<AdminAgentProfile>("/api/dashboard/agent/create", {
    method: "POST",
    body: JSON.stringify(payload),
    tenantId: getRequestTenantId(),
  })
}

export function updateAgentProfile(payload: UpdateAdminAgentProfilePayload) {
  return request<void>("/api/dashboard/agent/update", {
    method: "POST",
    body: JSON.stringify(payload),
    tenantId: getRequestTenantId(),
  })
}

export function deleteAgentProfile(id: number) {
  return request<void>("/api/dashboard/agent/delete", {
    method: "POST",
    body: JSON.stringify({ id }),
    tenantId: getRequestTenantId(),
  })
}

export function fetchAgentTeams(query?: Record<string, string | number | undefined>) {
  return request<AdminAgentTeam[]>(
    `/api/dashboard/agent-team/list${toQueryString(query)}`,
    { tenantId: getRequestTenantId() },
  )
}

export function fetchAgentTeamsAll() {
  return request<AdminAgentTeam[]>(
    `/api/dashboard/agent-team/list_all`,
    { tenantId: getRequestTenantId() },
  )
}

export function fetchAgentTeam(id: number) {
  return request<AdminAgentTeam>(`/api/dashboard/agent-team/${id}`, {
    tenantId: getRequestTenantId(),
  })
}

export function createAgentTeam(payload: CreateAdminAgentTeamPayload) {
  return request<AdminAgentTeam>("/api/dashboard/agent-team/create", {
    method: "POST",
    body: JSON.stringify(payload),
    tenantId: getRequestTenantId(),
  })
}

export function updateAgentTeam(payload: UpdateAdminAgentTeamPayload) {
  return request<void>("/api/dashboard/agent-team/update", {
    method: "POST",
    body: JSON.stringify(payload),
    tenantId: getRequestTenantId(),
  })
}

export function upsertAgentTeamMember(payload: {
  teamId: number
  userId: number
  dispatchEnabled: boolean
  dispatchWeight: number
}) {
  return request<{
    id: number
    tenantId: number
    teamId: number
    userId: number
    dispatchEnabled: boolean
    dispatchWeight: number
    status: number
  }>("/api/dashboard/agent-team/member/upsert", {
    method: "POST",
    body: JSON.stringify(payload),
    tenantId: getRequestTenantId(),
  })
}

export type AdminAgentTeamMemberRemoval = {
  teamId: number
  userId: number
  pendingTicketsRecovered: number
  pendingConversationsRecovered: number
  alreadyRemoved: boolean
}

export function removeAgentTeamMember(payload: { teamId: number; userId: number }) {
  return request<AdminAgentTeamMemberRemoval>("/api/dashboard/agent-team/member/delete", {
    method: "POST",
    body: JSON.stringify(payload),
    tenantId: getRequestTenantId(),
  })
}

export function deleteAgentTeam(id: number) {
  return request<void>("/api/dashboard/agent-team/delete", {
    method: "POST",
    body: JSON.stringify({ id }),
    tenantId: getRequestTenantId(),
  })
}

export function fetchAgentTeamSchedules(
  query?: Record<string, string | number | undefined>
) {
  return request<PageResult<AdminAgentTeamSchedule>>(
    `/api/dashboard/agent-team-schedule/list${toQueryString(query)}`,
    { tenantId: getRequestTenantId() },
  )
}

export function fetchAgentTeamScheduleCalendar(
  query: Record<string, string | number | undefined>
) {
  return request<AdminAgentTeamSchedule[]>(
    `/api/dashboard/agent-team-schedule/calendar${toQueryString(query)}`,
    { tenantId: getRequestTenantId() },
  )
}

export function fetchAgentTeamSchedule(id: number) {
  return request<AdminAgentTeamSchedule>(`/api/dashboard/agent-team-schedule/${id}`, {
    tenantId: getRequestTenantId(),
  })
}

export function createAgentTeamSchedule(payload: CreateAdminAgentTeamSchedulePayload) {
  void payload
  return rejectDeprecatedAgentTeamScheduleWrite<AdminAgentTeamSchedule>()
}

export function fetchAgentTeamScheduleTemplate() {
  return request<AdminAgentTeamScheduleTemplate>("/api/dashboard/agent-team-schedule/template", {
    tenantId: getRequestTenantId(),
  })
}

export function fetchAgentTeamScheduleLeaves(teamId: number, approvalStatus = "all") {
  return request<AdminAgentScheduleException[]>(
    `/api/dashboard/agent-team-schedule/leave/list${toQueryString({ teamId, approvalStatus })}`,
    { tenantId: getRequestTenantId() },
  )
}

export function fetchAgentTeamMemberAvailability(teamId: number) {
  return request<AdminAgentTeamMemberAvailability[]>(
    `/api/dashboard/agent-team-schedule/member-availability${toQueryString({ teamId })}`,
    { tenantId: getRequestTenantId() },
  )
}

export function reviewAgentTeamScheduleLeave(payload: {
  id: number
  decision: "approved" | "rejected"
  reviewNote?: string
}) {
  return request<AdminAgentScheduleException>("/api/dashboard/agent-team-schedule/leave/review", {
    method: "POST",
    body: JSON.stringify(payload),
    tenantId: getRequestTenantId(),
  })
}

export function updateAgentTeamScheduleTemplate(payload: Omit<AdminAgentTeamScheduleTemplate, "tenantId">) {
  return request<AdminAgentTeamScheduleTemplate>("/api/dashboard/agent-team-schedule/template/update", {
    method: "POST",
    body: JSON.stringify(payload),
    tenantId: getRequestTenantId(),
  })
}

export function updateAgentTeamSchedule(payload: UpdateAdminAgentTeamSchedulePayload) {
  void payload
  return rejectDeprecatedAgentTeamScheduleWrite<void>()
}

export function deleteAgentTeamSchedule(id: number) {
  void id
  return rejectDeprecatedAgentTeamScheduleWrite<void>()
}

export function prepareAgentTeamScheduleDraft(teamId: number) {
  void teamId
  return rejectDeprecatedAgentTeamScheduleWrite<AdminAgentTeamScheduleDraftResult>()
}

export function publishAgentTeamSchedule(teamId: number, allowCoverageGap: boolean) {
  void teamId
  void allowCoverageGap
  return rejectDeprecatedAgentTeamScheduleWrite<AdminAgentTeamSchedulePublishResult>()
}

export function rollbackAgentTeamSchedule(teamId: number) {
  void teamId
  return rejectDeprecatedAgentTeamScheduleWrite<AdminAgentTeamSchedulePublishResult>()
}

export function disableAgentTeamSchedule(teamId: number) {
  void teamId
  return rejectDeprecatedAgentTeamScheduleWrite<void>()
}

export function previewAgentTeamScheduleBatch(payload: BatchAdminAgentTeamSchedulePayload) {
  void payload
  return rejectDeprecatedAgentTeamScheduleWrite<AdminAgentTeamScheduleBatchPreview>()
}

export function generateAgentTeamScheduleBatch(payload: BatchAdminAgentTeamSchedulePayload) {
  void payload
  return rejectDeprecatedAgentTeamScheduleWrite<AdminAgentTeamScheduleBatchGenerateResult>()
}

export type AIConfig = {
  id: number
  name: string
  provider: string
  baseUrl: string
  hasApiKey: boolean
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
}

export type CreateAIConfigPayload = {
  name: string
  provider: string
  baseUrl: string
  apiKey: string
  modelType: string
  modelName: string
  dimension: number
  maxContextTokens: number
  maxOutputTokens: number
  timeoutMs: number
  maxRetryCount: number
  rpmLimit: number
  tpmLimit: number
  remark: string
}

export type UpdateAIConfigPayload = CreateAIConfigPayload & {
  id: number
}

export function fetchAIConfigs(
  query?: Record<string, string | number | undefined>
) {
  return request<PageResult<AIConfig>>(
    `/api/dashboard/ai-config/list${toQueryString(query)}`
  )
}

export function fetchAIConfig(id: number) {
  return request<AIConfig>(`/api/dashboard/ai-config/${id}`)
}

export function fetchAIConfigsAll(
  query?: Record<string, string | number | undefined>
) {
  return request<AIConfig[]>(
    `/api/dashboard/ai-config/list_all${toQueryString(query)}`
  )
}

export function createAIConfig(payload: CreateAIConfigPayload) {
  return request<AIConfig>("/api/dashboard/ai-config/create", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updateAIConfig(payload: UpdateAIConfigPayload) {
  return request<void>("/api/dashboard/ai-config/update", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function deleteAIConfig(id: number) {
  return request<void>("/api/dashboard/ai-config/delete", {
    method: "POST",
    body: JSON.stringify({ id }),
  })
}

export function updateAIConfigStatus(id: number, status: number) {
  return request<void>("/api/dashboard/ai-config/update_status", {
    method: "POST",
    body: JSON.stringify({ id, status }),
  })
}

export function updateAIConfigSort(ids: number[]) {
  return request<void>("/api/dashboard/ai-config/update_sort", {
    method: "POST",
    body: JSON.stringify(ids),
  })
}

export type SpeechRuntimeProvider = "disabled" | "mock" | "xfyun" | "aliyun"

export type SpeechRuntimeStatus = {
  provider: SpeechRuntimeProvider
  configured: boolean
  enabled: boolean
  canUpdate: boolean
  xfyunCredentialsConfigured: boolean
  xfyunEndpoint: string
  xfyunDomain: string
  xfyunTranslationEnabled: boolean
  xfyunTranslationConfigured: boolean
  xfyunTranslationEndpoint: string
  xfyunTranslationTargetLanguage: string
  aliyunApiKeyConfigured: boolean
  aliyunWorkspaceId: string
  aliyunEndpoint: string
  aliyunModel: string
  mockSegmentDurationMs: number
}

export type UpdateSpeechRuntimePayload = {
  provider: SpeechRuntimeProvider
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
}

export function fetchSpeechRuntime() {
  return request<SpeechRuntimeStatus>("/api/dashboard/ai-config/speech_runtime")
}

export function updateSpeechRuntime(payload: UpdateSpeechRuntimePayload) {
  return request<SpeechRuntimeStatus>("/api/dashboard/ai-config/speech_runtime/update", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export type KnowledgeBase = {
  id: number
  name: string
  description: string
  knowledgeType: string
  knowledgeTypeName: string
  accessScope: "tenant" | "product"
  accessScopeName: string
  status: number
  statusName: string
  defaultTopK: number
  defaultScoreThreshold: number
  defaultRerankLimit: number
  chunkProvider: string
  chunkTargetTokens: number
  chunkMaxTokens: number
  chunkOverlapTokens: number
  answerMode: number
  answerModeName: string
  documentCount: number
  faqCount: number
  remark: string
  createdAt: string
  updatedAt: string
  createUserName: string
  updateUserName: string
}

export type CreateKnowledgeBasePayload = {
  name: string
  description: string
  knowledgeType: string
  accessScope?: "tenant" | "product"
  defaultTopK: number
  defaultScoreThreshold: number
  defaultRerankLimit: number
  chunkProvider: string
  chunkTargetTokens: number
  chunkMaxTokens: number
  chunkOverlapTokens: number
  answerMode: number
  remark: string
}

export type UpdateKnowledgeBasePayload = CreateKnowledgeBasePayload & {
  id: number
}

export type KnowledgeDocument = {
  id: number
  knowledgeBaseId: number
  knowledgeBaseName?: string
  directoryId: number
  directoryName?: string
  directoryPath?: string
  title: string
  contentType: string
  content: string
  status: number
  statusName: string
  indexStatus: string
  indexStatusName: string
  indexedAt?: string | null
  indexError: string
  contentHash: string
  createdAt: string
  updatedAt: string
  createUserName: string
  updateUserName: string
}

export type KnowledgeDocumentListItem = Omit<KnowledgeDocument, "content">

export type KnowledgeFAQ = {
  id: number
  knowledgeBaseId: number
  knowledgeBaseName?: string
  directoryId: number
  directoryName?: string
  directoryPath?: string
  question: string
  answer: string
  similarQuestions: string[]
  status: number
  statusName: string
  indexStatus: string
  indexStatusName: string
  indexedAt?: string | null
  indexError: string
  remark: string
  createdAt: string
  updatedAt: string
  createUserName: string
  updateUserName: string
}

export type KnowledgeSearchResult = {
  knowledgeBaseId: number
  chunkId: number
  documentId: number
  documentTitle: string
  faqId: number
  faqQuestion: string
  chunkNo: number
  title: string
  sectionPath: string
  content: string
  score: number
  rerankScore?: number
}

export type KnowledgeSearchResponse = {
  question: string
  results: KnowledgeSearchResult[]
  hitCount: number
  latencyMs: number
}

export type KnowledgeAnswerResponse = {
  question: string
  rewriteQuestion?: string
  answer: string
  answerStatus: number
  answerStatusName: string
  citations: KnowledgeCitation[]
  hits: KnowledgeSearchResult[]
  hitCount: number
  topScore: number
  latencyMs: number
  retrieveMs: number
  generateMs: number
  promptTokens: number
  completionTokens: number
  modelName: string
  retrieveLogId: number
}

export type KnowledgeCitation = {
  documentId: number
  documentTitle: string
  faqId: number
  faqQuestion: string
  chunkNo: number
  title: string
  sectionPath: string
  snippet: string
  score: number
}

export type KnowledgeRetrieveLog = {
  id: number
  knowledgeBaseId: number
  knowledgeBaseName?: string
  channel: string
  channelName: string
  scene: string
  sceneName: string
  sessionId: string
  conversationId: number
  requestId: string
  question: string
  rewriteQuestion: string
  answer: string
  answerStatus: number
  answerStatusName: string
  hitCount: number
  topScore: number
  chunkProvider: string
  chunkTargetTokens: number
  chunkMaxTokens: number
  chunkOverlapTokens: number
  rerankEnabled: boolean
  rerankLimit: number
  citationCount: number
  usedChunkCount: number
  latencyMs: number
  retrieveMs: number
  generateMs: number
  promptTokens: number
  completionTokens: number
  modelName: string
  traceData: string
  createdAt: string
}

export type KnowledgeRetrieveHit = {
  id: number
  retrieveLogId: number
  knowledgeBaseId: number
  chunkId: number
  documentId: number
  documentTitle: string
  faqId: number
  faqQuestion: string
  chunkNo: number
  title: string
  sectionPath: string
  chunkType: string
  chunkTypeName: string
  provider: string
  rankNo: number
  score: number
  rerankScore: number
  usedInAnswer: boolean
  isCitation: boolean
  snippet: string
  createdAt: string
}

export type KnowledgeRetrieveLogDetail = {
  log: KnowledgeRetrieveLog
  hits: KnowledgeRetrieveHit[]
}

export type KnowledgeRetrieveLogListQuery = {
  knowledgeBaseId: number
  question?: string
  channel?: string
  scene?: string
  answerStatus?: number
  chunkProvider?: string
  rerankEnabled?: number
  page?: number
  limit?: number
}

export type KnowledgeSearchPayload = {
  tenantId?: number
  productId: number
  productModelId?: number
  knowledgeBaseIds: number[]
  locale?: string
  regionCode?: string
  audience?: "customer" | "agent" | "engineer"
  question: string
  topK?: number
  scoreThreshold?: number
  rerankLimit?: number
}

export type KnowledgeAnswerPayload = KnowledgeSearchPayload & {
  answerMode?: number
}

export type CreateKnowledgeDocumentPayload = {
  knowledgeBaseId: number
  directoryId: number
  title: string
  contentType: string
  content: string
}

export type UpdateKnowledgeDocumentPayload = CreateKnowledgeDocumentPayload & {
  id: number
}

export type CreateKnowledgeFAQPayload = {
  knowledgeBaseId: number
  directoryId: number
  question: string
  answer: string
  similarQuestions: string[]
  remark: string
}

export type UpdateKnowledgeFAQPayload = CreateKnowledgeFAQPayload & {
  id: number
}

export type BatchMoveKnowledgeContentPayload = {
  knowledgeBaseId: number
  directoryId: number
  ids: number[]
}

export type KnowledgeDirectory = {
  id: number
  knowledgeBaseId: number
  parentId: number
  name: string
  sortNo: number
  status: number
  statusName: string
  remark: string
  createdAt: string
  updatedAt: string
  createUserName: string
  updateUserName: string
  children: KnowledgeDirectory[]
}

export type CreateKnowledgeDirectoryPayload = {
  knowledgeBaseId: number
  parentId: number
  name: string
  remark: string
}

export type UpdateKnowledgeDirectoryPayload = CreateKnowledgeDirectoryPayload & {
  id: number
}

export type KnowledgeFAQImportMode = "append" | "overwrite"

export type KnowledgeFAQImportError = {
  row: number
  message: string
}

export type KnowledgeFAQImportResult = {
  total: number
  created: number
  updated: number
  skipped: number
  failed: number
  errors: KnowledgeFAQImportError[]
}

export function fetchKnowledgeBases(
  query?: Record<string, string | number | undefined>
) {
  return request<PageResult<KnowledgeBase>>(
    `/api/dashboard/knowledge-base/list${toQueryString(query)}`
  )
}

export function fetchKnowledgeBasesAll(
  query?: Record<string, string | number | undefined>
) {
  return request<KnowledgeBase[]>(
    `/api/dashboard/knowledge-base/list_all${toQueryString(query)}`
  )
}

export function fetchKnowledgeBase(id: number) {
  return request<KnowledgeBase>(`/api/dashboard/knowledge-base/${id}`)
}

export function createKnowledgeBase(payload: CreateKnowledgeBasePayload) {
  return request<KnowledgeBase>("/api/dashboard/knowledge-base/create", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updateKnowledgeBase(payload: UpdateKnowledgeBasePayload) {
  return request<void>("/api/dashboard/knowledge-base/update", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function deleteKnowledgeBase(id: number) {
  return request<void>("/api/dashboard/knowledge-base/delete", {
    method: "POST",
    body: JSON.stringify({ id }),
  })
}

export function updateKnowledgeBaseSort(ids: number[]) {
  return request<void>("/api/dashboard/knowledge-base/update_sort", {
    method: "POST",
    body: JSON.stringify(ids),
  })
}

export function rebuildKnowledgeBaseIndex(id: number) {
  return request<void>("/api/dashboard/knowledge-base/rebuild_index", {
    method: "POST",
    body: JSON.stringify({ id }),
  })
}

export function fetchKnowledgeDirectories(knowledgeBaseId: number) {
  return request<KnowledgeDirectory[]>(
    `/api/dashboard/knowledge-directory/list_all${toQueryString({ knowledgeBaseId })}`
  )
}

export function createKnowledgeDirectory(payload: CreateKnowledgeDirectoryPayload) {
  return request<KnowledgeDirectory>("/api/dashboard/knowledge-directory/create", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updateKnowledgeDirectory(payload: UpdateKnowledgeDirectoryPayload) {
  return request<void>("/api/dashboard/knowledge-directory/update", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function deleteKnowledgeDirectory(id: number) {
  return request<void>("/api/dashboard/knowledge-directory/delete", {
    method: "POST",
    body: JSON.stringify({ id }),
  })
}

export function updateKnowledgeDirectorySort(payload: {
  knowledgeBaseId: number
  parentId: number
  ids: number[]
}) {
  return request<void>("/api/dashboard/knowledge-directory/update_sort", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function fetchKnowledgeDocuments(
  query?: Record<string, string | number | undefined>
) {
  return request<PageResult<KnowledgeDocumentListItem>>(
    `/api/dashboard/knowledge-document/list${toQueryString(query)}`
  )
}

export function fetchKnowledgeDocument(id: number) {
  return request<KnowledgeDocument>(`/api/dashboard/knowledge-document/${id}`)
}

export function createKnowledgeDocument(payload: CreateKnowledgeDocumentPayload) {
  return request<KnowledgeDocument>("/api/dashboard/knowledge-document/create", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updateKnowledgeDocument(payload: UpdateKnowledgeDocumentPayload) {
  return request<void>("/api/dashboard/knowledge-document/update", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function deleteKnowledgeDocument(id: number) {
  return request<void>("/api/dashboard/knowledge-document/delete", {
    method: "POST",
    body: JSON.stringify({ id }),
  })
}

export function batchMoveKnowledgeDocuments(payload: BatchMoveKnowledgeContentPayload) {
  return request<void>("/api/dashboard/knowledge-document/batch_move", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function batchDeleteKnowledgeDocuments(ids: number[]) {
  return request<void>("/api/dashboard/knowledge-document/batch_delete", {
    method: "POST",
    body: JSON.stringify({ ids }),
  })
}

export function buildKnowledgeDocumentIndex(documentId: number) {
  return request<void>("/api/dashboard/knowledge-retrieve/build", {
    method: "POST",
    body: JSON.stringify({ documentId }),
  })
}

export function fetchKnowledgeFAQs(
  query?: Record<string, string | number | undefined>
) {
  return request<PageResult<KnowledgeFAQ>>(
    `/api/dashboard/knowledge-faq/list${toQueryString(query)}`
  )
}

export function fetchKnowledgeFAQ(id: number) {
  return request<KnowledgeFAQ>(`/api/dashboard/knowledge-faq/${id}`)
}

export function createKnowledgeFAQ(payload: CreateKnowledgeFAQPayload) {
  return request<KnowledgeFAQ>("/api/dashboard/knowledge-faq/create", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updateKnowledgeFAQ(payload: UpdateKnowledgeFAQPayload) {
  return request<void>("/api/dashboard/knowledge-faq/update", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function deleteKnowledgeFAQ(id: number) {
  return request<void>("/api/dashboard/knowledge-faq/delete", {
    method: "POST",
    body: JSON.stringify({ id }),
  })
}

export function batchMoveKnowledgeFAQs(payload: BatchMoveKnowledgeContentPayload) {
  return request<void>("/api/dashboard/knowledge-faq/batch_move", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function batchDeleteKnowledgeFAQs(ids: number[]) {
  return request<void>("/api/dashboard/knowledge-faq/batch_delete", {
    method: "POST",
    body: JSON.stringify({ ids }),
  })
}

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement("a")
  link.href = url
  link.download = filename
  link.click()
  URL.revokeObjectURL(url)
}

export async function downloadKnowledgeFAQImportTemplate() {
  const result = await requestBlob("/api/dashboard/knowledge-faq/import_template")
  downloadBlob(result.blob, result.filename || "knowledge-faq-import-template.xlsx")
}

export async function exportKnowledgeFAQs(knowledgeBaseId: number) {
  const result = await requestBlob(
    `/api/dashboard/knowledge-faq/export${toQueryString({ knowledgeBaseId })}`
  )
  downloadBlob(result.blob, result.filename || `knowledge-faq-${knowledgeBaseId}.xlsx`)
}

export function importKnowledgeFAQs(payload: {
  knowledgeBaseId: number
  mode: KnowledgeFAQImportMode
  file: File
}) {
  const formData = new FormData()
  formData.set("knowledgeBaseId", String(payload.knowledgeBaseId))
  formData.set("mode", payload.mode)
  formData.set("file", payload.file)
  return request<KnowledgeFAQImportResult>("/api/dashboard/knowledge-faq/import", {
    method: "POST",
    body: formData,
  })
}

export function buildKnowledgeFAQIndex(faqId: number) {
  return request<void>("/api/dashboard/knowledge-retrieve/build", {
    method: "POST",
    body: JSON.stringify({ faqId }),
  })
}

export function debugKnowledgeSearch(payload: KnowledgeSearchPayload) {
  return request<KnowledgeSearchResponse>("/api/dashboard/knowledge-retrieve/debug/search", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function debugKnowledgeAnswer(payload: KnowledgeAnswerPayload) {
  return request<KnowledgeAnswerResponse>("/api/dashboard/knowledge-retrieve/debug/answer", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function fetchKnowledgeRetrieveLogs(query: KnowledgeRetrieveLogListQuery) {
  return request<PageResult<KnowledgeRetrieveLog>>(
    `/api/dashboard/knowledge-retrieve-log/list${toQueryString(query)}`
  )
}

export function fetchKnowledgeRetrieveLog(id: number) {
  return request<KnowledgeRetrieveLogDetail>(`/api/dashboard/knowledge-retrieve-log/${id}`)
}

export type AdminAsset = {
  id: number
  assetId: string
  provider: string
  filename: string
  fileSize: number
  mimeType: string
  status: number
  url: string
  createdAt: string
  updatedAt: string
  createUserId: number
  createUserName: string
  updateUserId: number
  updateUserName: string
}

export function uploadAsset(file: File, prefix?: string) {
  const formData = new FormData()
  formData.set("file", file)
  if (prefix) {
    formData.set("prefix", prefix)
  }
  return request<AdminAsset>("/api/dashboard/asset/create", {
    method: "POST",
    body: formData,
  })
}

export type Tag = {
  id: number
  parentId: number
  name: string
  remark: string
  sortNo: number
  status: number
  createdAt: string
  updatedAt: string
}

export type TagTree = {
  id: number
  parentId: number
  name: string
  remark: string
  sortNo: number
  status: number
  createdAt: string
  updatedAt: string
  children: TagTree[]
}

export type CreateTagPayload = {
  parentId: number
  name: string
  remark: string
  status: number
}

export type UpdateTagPayload = CreateTagPayload & {
  id: number
}

export function fetchTags(query?: Record<string, string | number | undefined>) {
  return request<PageResult<Tag>>(
    `/api/dashboard/tag/list${toQueryString(query)}`
  )
}

export function fetchTagsAll() {
  return request<TagTree[]>("/api/dashboard/tag/list_all")
}

export function fetchTag(id: number) {
  return request<Tag>(`/api/dashboard/tag/${id}`)
}

export function createTag(payload: CreateTagPayload) {
  return request<Tag>("/api/dashboard/tag/create", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updateTag(payload: UpdateTagPayload) {
  return request<void>("/api/dashboard/tag/update", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function deleteTag(id: number) {
  return request<void>("/api/dashboard/tag/delete", {
    method: "POST",
    body: JSON.stringify({ id }),
  })
}

export function updateTagSort(ids: number[]) {
  return request<void>("/api/dashboard/tag/update_sort", {
    method: "POST",
    body: JSON.stringify(ids),
  })
}

export function updateTagStatus(id: number, status: number) {
  return request<void>("/api/dashboard/tag/update_status", {
    method: "POST",
    body: JSON.stringify({ id, status }),
  })
}
