const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL?.trim() ||
  (process.env.NODE_ENV === "development" ? "http://127.0.0.1:8083" : "")

const ACCESS_TOKEN_PROTOCOL_PREFIX = "rhd.access."
const CUSTOMER_SESSION_PROTOCOL_PREFIX = "rhd.customer."
const CREDENTIAL_PROTOCOL = "rhd.credential"

function createCredentialWebSocket(url: string, prefix: string, credential: string) {
  const normalizedCredential = credential.trim()
  if (!normalizedCredential) {
    throw new Error("WebSocket credential is required")
  }
  return new WebSocket(url, [CREDENTIAL_PROTOCOL, `${prefix}${normalizedCredential}`])
}

export function createAccessTokenWebSocket(url: string, accessToken: string) {
  return createCredentialWebSocket(url, ACCESS_TOKEN_PROTOCOL_PREFIX, accessToken)
}

export function createCustomerSessionWebSocket(url: string, customerSessionToken: string) {
  return createCredentialWebSocket(url, CUSTOMER_SESSION_PROTOCOL_PREFIX, customerSessionToken)
}

function sameOriginWebSocketBaseUrl() {
  if (typeof window === "undefined") {
    return ""
  }
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:"
  return `${protocol}//${window.location.host}`
}

function shouldUseBrowserSameOrigin(apiBaseUrl: string) {
  if (typeof window === "undefined" || process.env.NODE_ENV === "development" || !apiBaseUrl) {
    return false
  }
  try {
    const target = new URL(apiBaseUrl, window.location.href)
    const host = target.hostname.toLowerCase()
    return target.host === window.location.host ||
      host === "localhost" ||
      host === "127.0.0.1" ||
      host === "0.0.0.0" ||
      host === "::1"
  } catch {
    return true
  }
}

export function createWebSocketBaseUrl() {
  if (shouldUseBrowserSameOrigin(API_BASE_URL)) {
    return sameOriginWebSocketBaseUrl()
  }

  if (API_BASE_URL) {
    return API_BASE_URL.replace(/^http/, "ws").replace(/\/$/, "")
  }

  return sameOriginWebSocketBaseUrl()
}
