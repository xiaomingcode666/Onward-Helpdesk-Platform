import { expireSession, readSession } from "@/lib/auth"
import { translateCurrentMessage } from "@/i18n/messages"
import type { ApiResponse, ListQuery } from "@/lib/api/types"

const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL?.trim() || ""

type JsonResult<T> = {
  errorCode: number
  message: string
  data: T
  success: boolean
}

type RequestOptions = RequestInit & {
  skipAuth?: boolean
  baseUrl?: string
  onResponse?: (response: Response) => void
	onDownloadProgress?: (loadedBytes: number, totalBytes: number) => void
  tenantId?: number
  idempotencyKey?: string
  /** 超时毫秒数;0 或省略使用默认值 */
  timeoutMs?: number
}

const DEFAULT_REQUEST_TIMEOUT_MS = 20000
const DEFAULT_DOWNLOAD_TIMEOUT_MS = 60000

/**
 * 为请求创建超时 AbortSignal,与调用方传入的 signal 合并。
 * timeoutMs 为 0 时不启用超时。
 */
function createTimeoutSignal(timeoutMs: number | undefined, externalSignal?: AbortSignal | null) {
  const ms = timeoutMs !== undefined && timeoutMs > 0 ? timeoutMs : 0
  if (!ms) return { signal: externalSignal ?? null }
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(new Error("timeout")), ms)
  if (externalSignal) {
    if (typeof AbortSignal.any === "function") {
      return { signal: AbortSignal.any([externalSignal, controller.signal]), timer }
    }
    if (externalSignal.aborted) {
      controller.abort()
    } else {
      externalSignal.addEventListener("abort", () => controller.abort(), { once: true })
    }
  }
  return { signal: controller.signal, timer }
}

function isAbortError(error: unknown) {
  return error instanceof Error && error.name === "AbortError"
}

const AUTH_ERROR_CODES = new Set([3000, 3002])

function isAuthenticationFailure(httpStatus: number, errorCode?: number) {
  return httpStatus === 401 || (errorCode !== undefined && AUTH_ERROR_CODES.has(errorCode))
}

export type BlobResponse = {
  blob: Blob
  filename: string
}

async function parseResult<T>(response: Response, expireStoredAuth: boolean) {
  const responseText = await response.text()
  let payload: JsonResult<T> | null = null
  if (responseText.trim()) {
    try {
      payload = JSON.parse(responseText) as JsonResult<T>
    } catch {
      payload = null
    }
  }
  if (expireStoredAuth && response.status === 401) {
    expireSession()
  }
  if (!payload) {
    throw new Error(
      responseText.trim() ||
        response.statusText ||
        translateCurrentMessage("api.requestFailed")
    )
  }
  if (!response.ok || !payload.success) {
    if (expireStoredAuth && isAuthenticationFailure(response.status, payload.errorCode)) {
      expireSession()
    }
    const error = new Error(payload.message || translateCurrentMessage("api.requestFailed"))
    ;(error as Error & { errorCode?: number }).errorCode = payload.errorCode
    throw error
  }
  return payload.data
}

function buildRequestHeaders(
  headers: HeadersInit | undefined,
  skipAuth?: boolean,
  body?: BodyInit | null,
  tenantId?: number,
  idempotencyKey?: string,
) {
  const session = readSession()
  const authHeaders = new Headers(headers)

  if (!skipAuth && session?.accessToken) {
    authHeaders.set("Authorization", `Bearer ${session.accessToken}`)
  }

  // X-Tenant-Id: prefer explicit param, then session
  const sessionTenantId =
    session?.tenantId ?? ((session as Record<string, unknown>)?.tenant_id as number | undefined)
  const tid = tenantId || sessionTenantId
  if (tid) {
    authHeaders.set("X-Tenant-Id", String(tid))
  }

  // Idempotency-Key for POST/PATCH
  if (idempotencyKey) {
    authHeaders.set("Idempotency-Key", idempotencyKey)
  }

  if (
    !authHeaders.has("Content-Type") &&
    body &&
    !(typeof FormData !== "undefined" && body instanceof FormData)
  ) {
    authHeaders.set("Content-Type", "application/json")
  }
  return authHeaders
}

function parseFilename(contentDisposition: string | null) {
  if (!contentDisposition) {
    return ""
  }
  const utf8Match = contentDisposition.match(/filename\*=UTF-8''([^;]+)/i)
  if (utf8Match?.[1]) {
    return decodeURIComponent(utf8Match[1])
  }
  const match = contentDisposition.match(/filename="?([^";]+)"?/i)
  return match?.[1] ? decodeURIComponent(match[1]) : ""
}

export async function request<T>(
  path: string,
  options: RequestOptions = {}
): Promise<T> {
  const { headers, skipAuth, baseUrl, onResponse, tenantId, idempotencyKey, timeoutMs, ...rest } = options
  delete (rest as RequestOptions).baseUrl
  delete (rest as RequestOptions).onResponse
  delete (rest as RequestOptions).tenantId
  delete (rest as RequestOptions).idempotencyKey
  const authHeaders = buildRequestHeaders(headers, skipAuth, rest.body, tenantId, idempotencyKey)

  const { signal, timer } = createTimeoutSignal(
    timeoutMs ?? DEFAULT_REQUEST_TIMEOUT_MS,
    rest.signal ?? null
  )
  const requestBaseUrl = baseUrl !== undefined ? baseUrl : API_BASE_URL
  try {
    const response = await fetch(`${requestBaseUrl}${path}`, {
      ...rest,
      headers: authHeaders,
      cache: "no-store",
      signal,
    })
    onResponse?.(response)

    return parseResult<T>(response, !skipAuth)
  } catch (error) {
    if (isAbortError(error)) {
      throw new Error(translateCurrentMessage("api.requestTimeout"))
    }
    throw error
  } finally {
    if (timer !== undefined) clearTimeout(timer)
  }
}

// ---- New API helper functions (ApiResponse<T> envelope) ----

export function getRequestTenantId(): number {
  const session = readSession()
  if (session?.tenantId) return session.tenantId
  // Default to system tenant for platform users
  if (session?.roles?.includes("super_admin")) return 1
  return 1
}

function generateIdempotencyKey(): string {
  if (typeof globalThis.crypto?.randomUUID === "function") {
    return globalThis.crypto.randomUUID()
  }
  return `ik_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`
}

async function scopedApiFetch<T>(
	prefix: string,
  method: string,
  path: string,
  body?: unknown,
  params?: Record<string, unknown>,
): Promise<ApiResponse<T>> {
  const session = readSession()
  const tenantId = getRequestTenantId()
  const url = path.startsWith("http") ? path : `${prefix}${path}`
  const qs = params && Object.keys(params).length > 0
    ? `?${new URLSearchParams(
        Object.entries(params).reduce<Record<string, string>>((acc, [k, v]) => {
          if (v !== undefined && v !== null && v !== "") acc[k] = String(v)
          return acc
        }, {}),
      ).toString()}`
    : ""

  const isFormDataBody = typeof FormData !== "undefined" && body instanceof FormData
  const headers: Record<string, string> = {}
  if (!isFormDataBody) {
    headers["Content-Type"] = "application/json"
  }
  if (session?.accessToken) {
    headers["Authorization"] = `Bearer ${session.accessToken}`
  }
  if (tenantId) {
    headers["X-Tenant-Id"] = String(tenantId)
  }
  if ((method === "POST" || method === "PATCH") && !body) {
    headers["Idempotency-Key"] = generateIdempotencyKey()
  }

  const { signal, timer } = createTimeoutSignal(DEFAULT_REQUEST_TIMEOUT_MS)
  try {
    const response = await fetch(`${url}${qs}`, {
      method,
      headers,
      body:
        body === undefined
          ? undefined
          : isFormDataBody
            ? body
            : JSON.stringify(body),
      cache: "no-store",
      signal,
    })

    if (response.status === 401) {
      expireSession()
    }

    const responseText = await response.text()
    let json: (ApiResponse<T> & { errorCode?: number; message?: string }) | null = null
    if (responseText.trim()) {
      try {
        json = JSON.parse(responseText) as ApiResponse<T> & {
          errorCode?: number
          message?: string
        }
      } catch {
        json = null
      }
    }

    if (!json) {
      return {
        success: false,
        data: null as unknown as T,
        error: {
          code: `HTTP_${response.status}`,
          message: responseText.trim() || response.statusText || "Request failed",
        },
      }
    }

    if (!response.ok || !json.success) {
      if (isAuthenticationFailure(response.status, json.errorCode)) {
        expireSession()
      }
      return {
        success: false,
        data: null as unknown as T,
        error: json.error || {
          code: json.errorCode !== undefined ? `ERROR_${json.errorCode}` : `HTTP_${response.status}`,
          message: json.message || response.statusText || "Request failed",
        },
        requestId: json.requestId,
        timestamp: json.timestamp,
      }
    }

    return json
  } catch (err) {
    return {
      success: false,
      data: null as unknown as T,
      error: {
        code: isAbortError(err) ? "REQUEST_TIMEOUT" : "NETWORK_ERROR",
        message: isAbortError(err)
          ? translateCurrentMessage("api.requestTimeout")
          : err instanceof Error
            ? err.message
            : "Network request failed",
      },
    }
  } finally {
    if (timer !== undefined) clearTimeout(timer)
  }
}

export async function apiGet<T>(url: string, params?: Record<string, unknown>): Promise<ApiResponse<T>> {
  return scopedApiFetch<T>("/api/enterprise/v1", "GET", url, undefined, params)
}

export async function apiPost<T>(url: string, body?: unknown): Promise<ApiResponse<T>> {
  return scopedApiFetch<T>("/api/enterprise/v1", "POST", url, body)
}

export async function apiPatch<T>(url: string, body?: unknown): Promise<ApiResponse<T>> {
  return scopedApiFetch<T>("/api/enterprise/v1", "PATCH", url, body)
}

export async function apiPut<T>(url: string, body?: unknown): Promise<ApiResponse<T>> {
  return scopedApiFetch<T>("/api/enterprise/v1", "PUT", url, body)
}

export async function apiDelete<T>(url: string): Promise<ApiResponse<T>> {
  return scopedApiFetch<T>("/api/enterprise/v1", "DELETE", url)
}

export async function partnerApiGet<T>(url: string, params?: Record<string, unknown>): Promise<ApiResponse<T>> {
  return scopedApiFetch<T>("/api/partner/v1", "GET", url, undefined, params)
}

export async function partnerApiPost<T>(url: string, body?: unknown): Promise<ApiResponse<T>> {
  return scopedApiFetch<T>("/api/partner/v1", "POST", url, body)
}

export async function partnerApiPut<T>(url: string, body?: unknown): Promise<ApiResponse<T>> {
  return scopedApiFetch<T>("/api/partner/v1", "PUT", url, body)
}

/**
 * Build a filter query string from a Record of filter fields to values.
 * Example: { status: "active", priority: "high|critical" } => "status:active,priority:high|critical"
 */
export function buildFilterQuery(filters?: Record<string, string | string[] | undefined>): string | undefined {
  if (!filters) return undefined
  const parts: string[] = []
  for (const [key, value] of Object.entries(filters)) {
    if (value === undefined || value === "") continue
    if (Array.isArray(value)) {
      if (value.length > 0) parts.push(`${key}:${value.join("|")}`)
    } else {
      parts.push(`${key}:${value}`)
    }
  }
  return parts.length > 0 ? parts.join(",") : undefined
}

/**
 * Convert a ListQuery to Record for use with apiGet params
 */
export function listQueryToParams(query: ListQuery): Record<string, unknown> {
  const params: Record<string, unknown> = {}
  if (query.page && query.page > 1) params["page"] = query.page
  if (query.page_size && query.page_size !== 20) params["page_size"] = query.page_size
  if (query.sort) params["sort"] = query.sort
  if (query.filter) params["filter"] = query.filter
  return params
}

export async function requestBlob(
  path: string,
  options: RequestOptions = {}
): Promise<BlobResponse> {
	const {
		headers,
		skipAuth,
		baseUrl,
		onResponse,
		onDownloadProgress,
		tenantId,
		idempotencyKey,
		timeoutMs,
		...rest
	} = options
  delete (rest as RequestOptions).baseUrl
  delete (rest as RequestOptions).onResponse
  delete (rest as RequestOptions).tenantId
  delete (rest as RequestOptions).idempotencyKey
  const authHeaders = buildRequestHeaders(headers, skipAuth, rest.body, tenantId, idempotencyKey)
  const { signal, timer } = createTimeoutSignal(
    timeoutMs ?? DEFAULT_DOWNLOAD_TIMEOUT_MS,
    rest.signal ?? null
  )
  const requestBaseUrl = baseUrl !== undefined ? baseUrl : API_BASE_URL
  let response: Response
  try {
    response = await fetch(`${requestBaseUrl}${path}`, {
      ...rest,
      headers: authHeaders,
      cache: "no-store",
      signal,
    })
  } catch (error) {
    if (timer !== undefined) clearTimeout(timer)
    if (isAbortError(error)) {
      throw new Error(translateCurrentMessage("api.requestTimeout"))
    }
    throw error
  }
  onResponse?.(response)

  if (!skipAuth && response.status === 401) {
    expireSession()
  }

  const contentType = response.headers.get("Content-Type") ?? ""
  if (contentType.includes("application/json")) {
    if (timer !== undefined) clearTimeout(timer)
    try {
      await parseResult<never>(response, !skipAuth)
    } catch (error) {
      throw error
    }
    throw new Error(translateCurrentMessage("api.requestFailed"))
  }
  if (!response.ok) {
    if (timer !== undefined) clearTimeout(timer)
    throw new Error(response.statusText || translateCurrentMessage("api.requestFailed"))
  }
	let blob: Blob
	try {
		if (!onDownloadProgress || !response.body) {
			blob = await response.blob()
			onDownloadProgress?.(blob.size, blob.size)
		} else {
			const totalHeader = Number(response.headers.get("Content-Length") ?? 0)
			const totalBytes = Number.isFinite(totalHeader) && totalHeader > 0 ? totalHeader : 0
			const reader = response.body.getReader()
			const chunks: ArrayBuffer[] = []
			let loadedBytes = 0
			while (true) {
				const { done, value } = await reader.read()
				if (done) break
				if (!value?.byteLength) continue
				const chunk = new Uint8Array(value.byteLength)
				chunk.set(value)
				chunks.push(chunk.buffer)
				loadedBytes += value.byteLength
				onDownloadProgress(loadedBytes, totalBytes)
			}
			blob = new Blob(chunks, { type: contentType || "application/octet-stream" })
		}
	} catch (error) {
		if (isAbortError(error)) {
			throw new Error(translateCurrentMessage("api.requestTimeout"))
		}
		throw error
	} finally {
		if (timer !== undefined) clearTimeout(timer)
	}
	return {
		blob,
		filename: parseFilename(response.headers.get("Content-Disposition")),
	}
}
