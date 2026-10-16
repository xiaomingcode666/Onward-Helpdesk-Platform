import { request, requestBlob } from "@/lib/api/client"
import { readSession } from "@/lib/auth"
import { readStoredLocale } from "@/i18n/config"
import { translateCurrentMessage } from "@/i18n/messages"
import type { SupportChatRuntimeConfig } from "@/lib/sdk/config-types"
import { readSupportChatRuntimeConfig } from "@/lib/sdk/runtime-config"
import { generateUUID } from "@/lib/utils"

export type Paging = {
  page: number
  limit: number
  total: number
}

export type PageResult<T> = {
  results?: T[] | null
  page: Paging
  cursor?: string
  hasMore?: boolean
}

export type ImConversationTag = {
  id: number
  name: string
  color: string
}

export type ImConversationParticipant = {
  id?: number
  participantType: string
  participantId?: number
  externalParticipantId?: string
  joinedAt?: string
  leftAt?: string
  status: number
}

export type ImConversation = {
  id: number
  channelId: number
  customerName: string
  customerEntrySessionId?: number
  productId?: number
  productModelId?: number
  deviceId?: number
  status: number
  serviceMode: number
  humanHandoffEnabled?: boolean
  priority: number
  currentAssigneeId?: number
  currentAssigneeName?: string
  currentTeamId?: number
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
  tags?: ImConversationTag[]
  participants?: ImConversationParticipant[]
}

export type ImConversationDetail = ImConversation

export type ImMessage = {
  id: number
  conversationId: number
  workflowRunId?: number
  clientMsgId?: string
  senderType: string
  senderId?: number
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

export type ImAsset = {
  id: number
  assetId: string
  provider?: string
  storageKey?: string
  filename: string
  fileSize: number
  mimeType: string
  status: number
  url?: string
  createdAt: string
  updatedAt: string
  createUserId: number
  createUserName: string
  updateUserId: number
  updateUserName: string
}

export type ImWidgetConfig = {
  channelId?: string
  channelType?: string
  userToken?: string
  title?: string
  subtitle?: string
  themeColor?: string
  position?: "left" | "right"
  width?: string
}

export type ImCustomerSessionCustomer = {
  id: number
  name: string
}

export type ImCustomerSessionExchangeResponse = {
  customerSessionToken: string
  expiresAt: string
  identityKey: string
  customer: ImCustomerSessionCustomer
}

export type ImCustomerSession = ImCustomerSessionExchangeResponse & {
  channelId: string
}

const GUEST_STORAGE_KEY = "cs_ai_agent_im_guest_id"
const CUSTOMER_SESSION_STORAGE_KEY = "cs_ai_agent_customer_session"
const CUSTOMER_SESSION_TOKEN_HEADER = "X-Customer-Session-Token"
const CUSTOMER_SESSION_EXPIRES_HEADER = "X-Customer-Session-Expires-At"
const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL?.trim() || ""
const OPEN_IM_CHANNEL_ID =
  process.env.NEXT_PUBLIC_OPEN_IM_CHANNEL_ID?.trim() || ""
const MEDIA_FETCH_TIMEOUT_MS = 90_000

export type ImRuntimeConfig = Pick<
  SupportChatRuntimeConfig,
  "channelId" | "baseUrl" | "apiBaseUrl" | "externalId" | "externalName" | "userToken"
>

let entryUserTokenExchangeKey = ""

function buildGuestId() {
  return `guest_${generateUUID()}`
}

export function getGuestId() {
  if (typeof window === "undefined") {
    return ""
  }
  const existing = window.localStorage.getItem(GUEST_STORAGE_KEY)?.trim()
  if (existing) {
    return existing
  }
  const guestId = buildGuestId()
  window.localStorage.setItem(GUEST_STORAGE_KEY, guestId)
  return guestId
}

function getRuntimeImConfig(): ImRuntimeConfig {
	const widgetConfig = readSupportChatRuntimeConfig()
	const baseUrl = (widgetConfig.apiBaseUrl || widgetConfig.baseUrl || API_BASE_URL)
		.trim()
		.replace(/\/$/, "")
	return {
		baseUrl,
		apiBaseUrl: widgetConfig.apiBaseUrl?.trim().replace(/\/$/, "") || undefined,
		channelId: (widgetConfig.channelId || OPEN_IM_CHANNEL_ID).trim(),
		externalId: (widgetConfig.externalId || "").trim(),
		externalName: (widgetConfig.externalName || "").trim(),
		userToken: (widgetConfig.userToken || "").trim(),
	}
}

function resolveRuntimeImConfig(runtimeConfig?: Partial<ImRuntimeConfig>): ImRuntimeConfig {
	const config = getRuntimeImConfig()
	if (!runtimeConfig) {
		return config
	}
	const baseUrl = String(runtimeConfig.baseUrl ?? config.baseUrl ?? "").trim().replace(/\/$/, "")
	const apiBaseUrl = runtimeConfig.apiBaseUrl ?? config.apiBaseUrl
	return {
		...config,
		...runtimeConfig,
		channelId: String(runtimeConfig.channelId ?? config.channelId ?? "").trim(),
		baseUrl,
		apiBaseUrl: apiBaseUrl ? apiBaseUrl.trim().replace(/\/$/, "") : undefined,
		externalId: runtimeConfig.externalId ?? config.externalId,
		externalName: runtimeConfig.externalName ?? config.externalName,
		userToken: runtimeConfig.userToken ?? config.userToken,
	}
}

function requireChannelId(config: ImRuntimeConfig) {
	const channelId = String(config.channelId || "").trim()
	if (!channelId) {
		throw new Error(translateCurrentMessage("api.channelNotConfigured"))
	}
	return channelId
}

function parseExpiresAt(value: string) {
  const normalized = value.trim().replace(" ", "T")
  const timestamp = Date.parse(normalized)
  return Number.isFinite(timestamp) ? timestamp : 0
}

function isCustomerSessionValid(
  session: ImCustomerSession | null,
  channelId?: string,
  identityKey?: string
) {
  if (!session?.customerSessionToken || !session.expiresAt) {
    return false
  }
	if (channelId && session.channelId && session.channelId !== channelId) {
    return false
  }
  if (identityKey && session.identityKey !== identityKey) {
    return false
  }
  return parseExpiresAt(session.expiresAt) > Date.now() + 5000
}

export function readCustomerSession(): ImCustomerSession | null {
  if (typeof window === "undefined") {
    return null
  }
  const raw = window.sessionStorage.getItem(CUSTOMER_SESSION_STORAGE_KEY)
  if (!raw) {
    return null
  }
  try {
    return JSON.parse(raw) as ImCustomerSession
  } catch {
    window.sessionStorage.removeItem(CUSTOMER_SESSION_STORAGE_KEY)
    return null
  }
}

function writeCustomerSession(session: ImCustomerSession) {
  if (typeof window === "undefined") {
    return
  }
	window.sessionStorage.setItem(CUSTOMER_SESSION_STORAGE_KEY, JSON.stringify(session))
}

export function persistCustomerSession(
	session: ImCustomerSessionExchangeResponse,
	channelId = "",
) {
	if (!session.customerSessionToken?.trim() || !session.expiresAt?.trim()) {
		return
	}
	writeCustomerSession({
		...session,
		channelId: channelId.trim(),
	})
}

export function getCustomerSessionToken() {
  const config = resolveRuntimeImConfig()
  const session = readCustomerSession()
  return isCustomerSessionValid(session, config.channelId)
    ? session?.customerSessionToken ?? ""
    : ""
}

export function applyCustomerSessionRefresh(payload?: {
  customerSessionToken?: string
  expiresAt?: string
}) {
  const token = payload?.customerSessionToken?.trim()
  const expiresAt = payload?.expiresAt?.trim()
  if (!token || !expiresAt) {
    return
  }
  const current = readCustomerSession()
  if (!current) {
    return
  }
  writeCustomerSession({
    ...current,
    customerSessionToken: token,
    expiresAt,
  })
}

function applyCustomerSessionHeaders(response: Response) {
  applyCustomerSessionRefresh({
    customerSessionToken: response.headers.get(CUSTOMER_SESSION_TOKEN_HEADER) ?? "",
    expiresAt: response.headers.get(CUSTOMER_SESSION_EXPIRES_HEADER) ?? "",
  })
}

function createChannelHeaders(runtimeConfig?: Partial<ImRuntimeConfig>) {
	const config = resolveRuntimeImConfig(runtimeConfig)
	const headers: Record<string, string> = {}
	if (config.channelId) {
		headers["X-Channel-Id"] = config.channelId
	}
	return headers
}

function createExchangeHeaders(runtimeConfig?: Partial<ImRuntimeConfig>) {
	const config = resolveRuntimeImConfig(runtimeConfig)
	const headers: Record<string, string> = createChannelHeaders(config)
	if (config.userToken) {
		headers.Authorization = `Bearer ${config.userToken}`
	} else {
    headers["X-External-Id"] = config.externalId || getGuestId()
    if (config.externalName) {
      headers["X-External-Name"] = encodeURIComponent(config.externalName)
    }
  }
  return {
    ...headers,
  }
}

function createImHeaders(runtimeConfig?: Partial<ImRuntimeConfig>) {
	const config = resolveRuntimeImConfig(runtimeConfig)
	requireChannelId(config)
	const session = readCustomerSession()
	const sessionToken = isCustomerSessionValid(session, config.channelId)
	    ? session?.customerSessionToken ?? ""
    : ""
  if (!sessionToken) {
    throw new Error(translateCurrentMessage("api.customerSessionNotReady"))
  }
  return {
    ...createChannelHeaders(config),
    Authorization: `Bearer ${sessionToken}`,
  }
}

type ImRequestInit = RequestInit & { idempotencyKey?: string }

function createRequestOptions(
  init?: ImRequestInit,
  runtimeConfig?: Partial<ImRuntimeConfig>
): ImRequestInit & {
  baseUrl?: string
  skipAuth?: boolean
  onResponse?: (response: Response) => void
} {
  const config = resolveRuntimeImConfig(runtimeConfig)
  const accountSession = readSession()
  if (accountSession?.domainType === "customer" && accountSession.accessToken) {
    return {
      ...init,
      baseUrl: config.baseUrl,
    }
  }
  return {
    ...init,
    skipAuth: true,
    headers: {
      ...createImHeaders(config),
      ...(init?.headers as Record<string, string> | undefined),
    },
    onResponse: applyCustomerSessionHeaders,
    baseUrl: config.baseUrl,
  }
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

export async function exchangeCustomerSession(runtimeConfig?: Partial<ImRuntimeConfig>) {
	const config = resolveRuntimeImConfig(runtimeConfig)
	requireChannelId(config)
	const result = await request<ImCustomerSessionExchangeResponse>(
		"/api/customer/session_exchange",
		{
      method: "POST",
      skipAuth: true,
      baseUrl: config.baseUrl,
      headers: createExchangeHeaders(config),
    }
  )
  const session = {
    ...result,
    channelId: config.channelId,
  }
	persistCustomerSession(result, config.channelId)
  if (config.userToken) {
    entryUserTokenExchangeKey = `${config.channelId}:${config.userToken}`
  }
  return session
}

export async function ensureCustomerSession(runtimeConfig?: Partial<ImRuntimeConfig>) {
	const config = resolveRuntimeImConfig(runtimeConfig)
	requireChannelId(config)
	const cached = readCustomerSession()
	if (config.userToken) {
		const exchangeKey = `${config.channelId}:${config.userToken}`
    if (
      entryUserTokenExchangeKey === exchangeKey &&
      isCustomerSessionValid(cached, config.channelId)
    ) {
      return cached
    }
    return exchangeCustomerSession(config)
  }

  const externalId = config.externalId || getGuestId()
  const identityKey = `guest:${externalId}`
  if (isCustomerSessionValid(cached, config.channelId, identityKey)) {
    return cached
  }
  return exchangeCustomerSession(config)
}

export function fetchImConversationDetail(id: number) {
  return request<ImConversationDetail>(`/api/conversation/${id}`, {
    ...createRequestOptions(),
  })
}

export function fetchImMessages(
  query?: Record<string, string | number | undefined>
) {
  return request<PageResult<ImMessage>>(
    `/api/message/list${toQueryString(query)}`,
    createRequestOptions()
  )
}

export function createOrMatchImConversation(payload?: {
  deviceId?: number
  serviceCodeId?: number
  customerEntrySessionId?: number
  locale?: string
  general?: boolean
  forceNew?: boolean
}, idempotencyKey = globalThis.crypto?.randomUUID?.() ?? `conversation-create-${Date.now()}-${Math.random().toString(36).slice(2)}`) {
  return request<ImConversation>("/api/conversation/create_or_match", {
    ...createRequestOptions({
      method: "POST",
      body: JSON.stringify({ ...payload, locale: payload?.locale ?? readStoredLocale() }),
      idempotencyKey,
    }),
  })
}

export function fetchImWidgetConfig() {
  return request<ImWidgetConfig>(
    `/api/channel/config${toQueryString({
        channelId: getRuntimeImConfig().channelId,
      })}`,
    {
      skipAuth: true,
      baseUrl: getRuntimeImConfig().baseUrl,
      headers: createChannelHeaders(),
    }
  )
}

export function closeImConversation(conversationId: number) {
  return request<void>("/api/conversation/close", {
    ...createRequestOptions({
      method: "POST",
      body: JSON.stringify({ conversationId }),
    }),
  })
}

export function requestImHumanSupport(conversationId: number, reason = "") {
  return request<void>("/api/conversation/request_human", {
    ...createRequestOptions({
      method: "POST",
      body: JSON.stringify({ conversationId, reason, locale: readStoredLocale() }),
    }),
  })
}

export function sendImMessage(payload: {
  conversationId: number
  messageType: string
  content: string
  payload?: string
  clientMsgId?: string
}) {
  return request<ImMessage>("/api/message/send", {
    ...createRequestOptions({
      method: "POST",
      body: JSON.stringify(payload),
    }),
  })
}

export function markImMessageRead(conversationId: number, messageId = 0) {
  return request<void>("/api/message/read", {
    ...createRequestOptions({
      method: "POST",
      body: JSON.stringify({ conversationId, messageId }),
    }),
  })
}

export function uploadImImage(conversationId: number, file: File) {
  const formData = new FormData()
  formData.set("conversationId", String(conversationId))
  formData.set("file", file)
  return request<ImAsset>("/api/message/upload_image", {
    ...createRequestOptions({
      method: "POST",
      body: formData,
    }),
  })
}

export function uploadImAttachment(conversationId: number, file: File) {
  const formData = new FormData()
  formData.set("conversationId", String(conversationId))
  formData.set("file", file)
  return request<ImAsset>("/api/message/upload_attachment", {
    ...createRequestOptions({
      method: "POST",
      body: formData,
    }),
  })
}

export function uploadImAudio(conversationId: number, file: File) {
  const formData = new FormData()
  formData.set("conversationId", String(conversationId))
  formData.set("file", file)
  return request<ImAsset>("/api/message/upload_audio", {
    ...createRequestOptions({
      method: "POST",
      body: formData,
    }),
  })
}

export async function fetchImMediaAsset(
	assetId: string,
	signal?: AbortSignal,
	onProgress?: (loadedBytes: number, totalBytes: number) => void,
) {
  const normalizedAssetId = assetId.trim()
  if (!normalizedAssetId) {
    throw new Error(translateCurrentMessage("supportChat.mediaLoadFailed"))
  }
  const controller = new AbortController()
  let timedOut = false
  const abortFromCaller = () => controller.abort()
  if (signal?.aborted) {
    controller.abort()
  } else {
    signal?.addEventListener("abort", abortFromCaller, { once: true })
  }
  const timeoutId = setTimeout(() => {
    timedOut = true
    controller.abort()
  }, MEDIA_FETCH_TIMEOUT_MS)

  try {
    const session = readSession()
		const encodedAssetId = encodeURIComponent(normalizedAssetId)
		const result = session?.accessToken && session.domainType !== "customer"
			? await requestBlob(`/api/media/${encodedAssetId}`, {
				signal: controller.signal,
				onDownloadProgress: onProgress,
			})
			: await requestBlob(
				`/api/message/media/${encodedAssetId}`,
				{
					...createRequestOptions({ signal: controller.signal }),
					onDownloadProgress: onProgress,
				},
			)
    return result.blob
  } catch (error) {
    if (timedOut) {
      throw new Error(translateCurrentMessage("supportChat.mediaLoadFailed"))
    }
    throw error
  } finally {
    clearTimeout(timeoutId)
    signal?.removeEventListener("abort", abortFromCaller)
  }
}

/** Build a native-media URL so the browser can issue Range requests itself. */
export function buildImMediaURL(assetId: string) {
  const session = readSession()
  const encodedAssetId = encodeURIComponent(assetId.trim())
  const customerPath = `/api/message/media/${encodedAssetId}`
  const isCustomerPortal = !session || session.domainType === "customer"
  if (isCustomerPortal) {
    const customerSessionToken = getCustomerSessionToken()
    return customerSessionToken
      ? `${customerPath}?customerSessionToken=${encodeURIComponent(customerSessionToken)}`
      : customerPath
  }
  const operatorPath = `/api/media/${encodedAssetId}`
  return session.accessToken
    ? `${operatorPath}?accessToken=${encodeURIComponent(session.accessToken)}`
    : operatorPath
}
