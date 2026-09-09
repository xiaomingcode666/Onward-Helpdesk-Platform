import { request } from "@/lib/api/client"
import { readStoredLocale } from "@/i18n/config"

import {
	persistCustomerSession,
	type ImAsset,
	type ImConversation,
	type ImMessage,
	type PageResult,
} from "@/lib/api/im"

export type CustomerEntryKind =
  | "invalid"
  | "revoked"
  | "needRegister"
  | "guestSessionReady"

export type CustomerEntryContextSummary = {
  id?: number
  code?: string
  name?: string
  modelCode?: string
  category?: string
  deviceNo?: string
  serialNo?: string
  modelName?: string
  regionCode?: string
  status?: string
}

export type ResolvedServiceCode = {
  id: number
  serviceCode: string
  entryUrl?: string
  qrUrl?: string
  qrImageUrl?: string
  mode: string
  status: string
}

export type CustomerServiceCodeResolveResult = {
  valid?: boolean
  entryState?: CustomerEntryKind | string
  reason?: string
  serviceCode?: string | ResolvedServiceCode
  revoked?: boolean
  revokedAt?: string
  needRegister?: boolean
  tenant?: CustomerEntryContextSummary | null
  product?: CustomerEntryContextSummary | null
  productModel?: CustomerEntryContextSummary | null
  device?: CustomerEntryContextSummary | null
  entrySession?: CustomerEntrySessionLike | null
  session?: CustomerEntrySessionLike | null
}

export type CustomerEntrySessionLike = Record<string, unknown> & {
  id?: number
  entrySessionId?: number
  token?: string
  status?: string
}

export type CustomerEntryState =
  | {
      kind: "invalid"
      reason: string
      serviceCode?: string
    }
  | {
      kind: "revoked"
      serviceCode?: string
      revokedAt?: string
    }
  | {
      kind: "needRegister"
      serviceCode: string
	  tenant?: CustomerEntryContextSummary | null
      product?: CustomerEntryContextSummary | null
	  device?: CustomerEntryContextSummary | null
    }
  | {
      kind: "accessError"
      serviceCode: string
      reason: string
      tenant?: CustomerEntryContextSummary | null
      product?: CustomerEntryContextSummary | null
      device?: CustomerEntryContextSummary | null
    }
  | {
      kind: "guestSessionReady"
      serviceCode: string
      session?: CustomerEntrySessionLike
      product?: CustomerEntryContextSummary | null
      device?: CustomerEntryContextSummary | null
    }

export type CreateEntrySessionPayload = {
  serviceCode?: string
  deviceNo?: string
  visitorId?: string
  visitorToken?: string
  locale?: string
  customerUserId?: number
}

export type CustomerEntryChat = {
  entrySessionId: number
  visitorId: string
	visitorToken: string
  locale: string
}

export type CustomerEntrySessionExchangeResponse = {
	customerSessionToken: string
	expiresAt: string
	identityKey: string
	customer: {
		id: number
		name: string
	}
}

export type CustomerEntrySessionResponse = {
  entrySessionId: number
  tenant?: CustomerEntryContextSummary | null
  product?: CustomerEntryContextSummary | null
  model?: CustomerEntryContextSummary | null
  device?: CustomerEntryContextSummary | null
  serviceCode?: string
  expiresAt?: string
  chat: CustomerEntryChat
  privacyConsent: CustomerPrivacyConsent
  session: Record<string, unknown>
}

export type CustomerPrivacyConsent = {
  required: boolean
  accepted: boolean
  analytics: boolean
  marketing: boolean
  policyVersion: string
  consentedAt?: string
}

export type ConfirmCustomerPrivacyConsentPayload = {
  entrySessionId: number
  policyVersion: string
  required: true
  analytics: boolean
  marketing: boolean
}

export type ConfirmDeviceBindingPayload = {
  entrySessionId: number
  visitorId: string
  visitorToken: string
  customerUserId?: number
  customerOrgId?: number
  bindingRole?: string
}

export type CustomerDeviceBindingResponse = {
  id: number
  entrySessionId: number
  tenantId: number
  deviceId: number
  customerUserId: number
  customerOrgId: number
  bindingRole: string
  status: number
}

export type CustomerEntryDevice = {
  id: number
  deviceNo: string
  serialNo: string
  productName: string
  productCode: string
  modelName: string
  regionCode: string
  status: string
  lastServiceAt: string
}

export type CustomerEntryTicket = {
  id: number
	conversationId: number
  ticketNo: string
  title: string
  status: string
  priority: string
  createdAt: string
  deviceNo: string
	repairSummary: string
	feedback?: {
		id: number
		rating: number
		tags: string[]
		comment: string
		submittedAt: string
	}
	canConfirm: boolean
	canReopen: boolean
	canRate: boolean
}

export type CustomerEntryConversation = {
  id: number
  summary: string
  startedAt: string
  endedAt?: string
  agentType: "ai" | "human" | string
  deviceNo: string
}

export type CustomerEntryRepairHistory = {
  id: number
  ticketId: number
  ticketNo: string
  deviceNo: string
  serviceType: string
  summary: string
  rootCause: string
  solution: string
  occurredAt: string
}

export type CustomerEntryKnowledgeEntry = {
  id: number
  title: string
  type: string
  language: string
  version: string
  content?: string
  contentType?: string
  updatedAt: string
}

export type CustomerEntryManualFile = {
  id: number
  title: string
  filename: string
  fileSize: number
  mimeType: string
  url: string
  uploadedAt: string
}

export type CustomerEntryMeeting = {
  id: string
  title: string
  status: "waiting" | "active" | "finished" | string
  scheduledAt: string
  startedAt: string
  endedAt: string
  initiator: string
  ticketId: number
}

export type CustomerEntryMeetingJoinConfig = {
	domain: string
	roomName: string
	jwt: string
	jitsiUrl: string
	meetingId: string
}

export type CustomerEntryContext = {
  entrySessionId: number
  serviceCode?: string
  tenant?: CustomerEntryContextSummary | null
  product?: CustomerEntryContextSummary | null
  model?: CustomerEntryContextSummary | null
  device?: CustomerEntryContextSummary | null
  devices: CustomerEntryDevice[]
  tickets: CustomerEntryTicket[]
  conversations: CustomerEntryConversation[]
  repairHistory: CustomerEntryRepairHistory[]
  knowledgeEntries: CustomerEntryKnowledgeEntry[]
  manualFiles: CustomerEntryManualFile[]
  meetings: CustomerEntryMeeting[]
}

function toQueryString(query: Record<string, string | number | undefined>) {
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

function serviceCodeValue(
  result: CustomerServiceCodeResolveResult | null | undefined
) {
  const value = result?.serviceCode
  if (typeof value === "string") {
    return value.trim()
  }
  return value?.serviceCode?.trim() ?? ""
}

function serviceCodeStatus(
  result: CustomerServiceCodeResolveResult | null | undefined
) {
  const value = result?.serviceCode
  return typeof value === "object" ? value.status : ""
}

function normalizeReason(reason: string | undefined, fallback: string) {
  return reason?.trim() || fallback
}

export function buildCustomerEntryState(
  result: CustomerServiceCodeResolveResult | null | undefined
): CustomerEntryState {
  if (!result) {
    return {
      kind: "invalid",
      reason: "missing",
    }
  }

  const code = serviceCodeValue(result)
  const entryState = result.entryState
  const status = serviceCodeStatus(result)
  const isRevoked =
    result.revoked ||
    entryState === "revoked" ||
    status === "revoked" ||
    status === "disabled"

  if (isRevoked) {
    return {
      kind: "revoked",
      ...(code ? { serviceCode: code } : {}),
      ...(result.revokedAt ? { revokedAt: result.revokedAt } : {}),
    }
  }

  if (result.valid === false || entryState === "invalid") {
    return {
      kind: "invalid",
      ...(code ? { serviceCode: code } : {}),
      reason: normalizeReason(result.reason, "notFound"),
    }
  }

	if (result.needRegister || entryState === "needRegister") {
		return {
			kind: "needRegister",
			serviceCode: code,
			...(result.tenant ? { tenant: result.tenant } : {}),
			...(result.product ? { product: result.product } : {}),
			...(result.device ? { device: result.device } : {}),
		}
	}

	// A resolved service code is only a discovery result. It never creates a
	// guest service identity; a signed-in account must bind the device first.
	return {
		kind: "needRegister",
		serviceCode: code,
		...(result.tenant ? { tenant: result.tenant } : {}),
		...(result.product ? { product: result.product } : {}),
		...(result.device ? { device: result.device } : {}),
	}
}

export function resolveServiceCode(serviceCode: string) {
  return request<CustomerServiceCodeResolveResult>(
    `/api/customer/service-code/resolve${toQueryString({ serviceCode })}`,
    { skipAuth: true }
  )
}

export function createEntrySession(payload: CreateEntrySessionPayload) {
  return request<CustomerEntrySessionResponse>(
    "/api/customer/entry-session/create",
    {
      method: "POST",
      body: JSON.stringify(payload),
      skipAuth: true,
    }
  )
}

export function fetchCustomerPrivacyConsent(
  entrySessionId: number,
  visitorId: string,
  visitorToken: string
) {
  return request<CustomerPrivacyConsent>(
    `/api/customer/entry-session/privacy-consent${toQueryString({ entrySessionId })}`,
    {
      skipAuth: true,
      headers: {
        "X-Customer-Entry-Visitor-Id": visitorId,
        "X-Customer-Entry-Visitor-Token": visitorToken,
      },
    }
  )
}

export function confirmCustomerPrivacyConsent(
  payload: ConfirmCustomerPrivacyConsentPayload,
  visitorId: string,
  visitorToken: string
) {
  return request<CustomerPrivacyConsent>(
    "/api/customer/entry-session/privacy-consent",
    {
      method: "POST",
      body: JSON.stringify(payload),
      skipAuth: true,
      headers: {
        "X-Customer-Entry-Visitor-Id": visitorId,
        "X-Customer-Entry-Visitor-Token": visitorToken,
      },
    }
  )
}

export function confirmDeviceBinding(payload: ConfirmDeviceBindingPayload) {
  return request<CustomerDeviceBindingResponse>(
    "/api/customer/device-binding/confirm",
    {
      method: "POST",
      body: JSON.stringify(payload),
      skipAuth: true,
    }
  )
}

export function fetchEntrySessionContext(
  entrySessionId: number,
  visitorId: string,
  visitorToken: string
) {
  return request<CustomerEntryContext>(
    `/api/customer/entry-session/context${toQueryString({ entrySessionId })}`,
    {
      skipAuth: true,
      headers: {
        "X-Customer-Entry-Visitor-Id": visitorId,
        "X-Customer-Entry-Visitor-Token": visitorToken,
      },
    }
  )
}

function customerEntryTicketOptions(visitorId: string, visitorToken: string, init: RequestInit) {
	return {
		...init,
		skipAuth: true,
		headers: {
			"X-Customer-Entry-Visitor-Id": visitorId,
			"X-Customer-Entry-Visitor-Token": visitorToken,
			...(init.headers as Record<string, string> | undefined),
		},
	}
}

export function submitCustomerEntryTicketFeedback(
	entrySessionId: number,
	visitorId: string,
	visitorToken: string,
	ticketId: number,
	payload: { rating: number; tags?: string[]; comment?: string }
) {
	return request<CustomerEntryTicket["feedback"]>(
		`/api/customer/entry-session/tickets/${ticketId}/feedback${toQueryString({ entrySessionId })}`,
		customerEntryTicketOptions(visitorId, visitorToken, {
			method: "POST",
			body: JSON.stringify(payload),
		})
	)
}

export function confirmCustomerEntryTicket(
	entrySessionId: number,
	visitorId: string,
	visitorToken: string,
	ticketId: number
) {
	return request<CustomerEntryTicket>(
		`/api/customer/entry-session/tickets/${ticketId}/_confirm${toQueryString({ entrySessionId })}`,
		customerEntryTicketOptions(visitorId, visitorToken, { method: "POST", body: "{}" })
	)
}

export function reopenCustomerEntryTicket(
	entrySessionId: number,
	visitorId: string,
	visitorToken: string,
	ticketId: number,
	reason: string
) {
	return request<CustomerEntryTicket>(
		`/api/customer/entry-session/tickets/${ticketId}/_reopen${toQueryString({ entrySessionId })}`,
		customerEntryTicketOptions(visitorId, visitorToken, {
			method: "POST",
			body: JSON.stringify({ reason }),
		})
	)
}

export function fetchCustomerEntryMeetingJoin(
	entrySessionId: number,
	visitorId: string,
	visitorToken: string,
	meetingId: string
) {
	return request<CustomerEntryMeetingJoinConfig>(
		`/api/customer/entry-session/meetings/${encodeURIComponent(meetingId)}/join${toQueryString({ entrySessionId })}`,
		customerEntryTicketOptions(visitorId, visitorToken, {})
	)
}

function entrySessionAuthOptions(token: string, init?: RequestInit) {
	return {
		...init,
		skipAuth: true,
		headers: {
			Authorization: `Bearer ${token}`,
			...(init?.headers as Record<string, string> | undefined),
		},
	}
}

export function exchangeCustomerEntrySession(payload: {
	entrySessionId: number
	visitorId: string
	visitorToken: string
}) {
	return request<CustomerEntrySessionExchangeResponse>(
		"/api/customer/entry-session/exchange",
		{
			method: "POST",
			body: JSON.stringify(payload),
			skipAuth: true,
		}
	).then((result) => {
		persistCustomerSession(result)
		return result
	})
}

export function createCustomerEntryConversation(
	token: string,
	entrySessionId: number
) {
	return request<ImConversation>(
		"/api/conversation/create_or_match",
		entrySessionAuthOptions(token, {
			method: "POST",
			body: JSON.stringify({ customerEntrySessionId: entrySessionId, locale: readStoredLocale() }),
		})
	)
}

export function fetchCustomerEntryConversation(
	token: string,
	conversationId: number
) {
	return request<ImConversation>(
		`/api/conversation/${conversationId}`,
		entrySessionAuthOptions(token)
	)
}

export function fetchCustomerEntryMessages(
	token: string,
	conversationId: number
) {
	return request<PageResult<ImMessage>>(
		`/api/message/list${toQueryString({ conversationId, limit: 50 })}`,
		entrySessionAuthOptions(token)
	)
}

export function sendCustomerEntryMessage(
	token: string,
	payload: {
		conversationId: number
		content: string
		clientMsgId: string
		messageType?: string
		payload?: string
	}
) {
	return request<ImMessage>(
		"/api/message/send",
		entrySessionAuthOptions(token, {
			method: "POST",
			body: JSON.stringify({
				...payload,
				messageType: payload.messageType || "text",
			}),
		})
	)
}

function uploadCustomerEntryAsset(
	token: string,
	conversationId: number,
	file: File,
	endpoint: string
) {
	const formData = new FormData()
	formData.set("conversationId", String(conversationId))
	formData.set("file", file)
	return request<ImAsset>(
		endpoint,
		entrySessionAuthOptions(token, {
			method: "POST",
			body: formData,
		})
	)
}

export function uploadCustomerEntryImage(token: string, conversationId: number, file: File) {
	return uploadCustomerEntryAsset(token, conversationId, file, "/api/message/upload_image")
}

export function uploadCustomerEntryAttachment(token: string, conversationId: number, file: File) {
	return uploadCustomerEntryAsset(token, conversationId, file, "/api/message/upload_attachment")
}

export function uploadCustomerEntryAudio(token: string, conversationId: number, file: File) {
	return uploadCustomerEntryAsset(token, conversationId, file, "/api/message/upload_audio")
}

export function requestCustomerEntryHumanSupport(
	token: string,
	conversationId: number,
	reason: string
) {
	return request<void>(
		"/api/conversation/request_human",
		entrySessionAuthOptions(token, {
			method: "POST",
			body: JSON.stringify({ conversationId, reason, locale: readStoredLocale() }),
		})
	)
}
