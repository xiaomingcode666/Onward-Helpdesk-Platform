import { setSupportChatRuntimeConfig } from "@/lib/sdk/runtime-config"
import type { SupportChatRuntimeConfig } from "@/lib/sdk/config-types"

type HostBridgeOptions = {
  onInit?: () => void
  onOpen?: () => void
  onMinimize?: () => void
  onMaximizedChange?: (isMaximized: boolean) => void
}

type WidgetProtocol = "remote-helpdesk" | "agent-desk"

const PRIMARY_PROTOCOL: WidgetProtocol = "remote-helpdesk"
const LEGACY_PROTOCOL: WidgetProtocol = "agent-desk"
let parentProtocol: WidgetProtocol = PRIMARY_PROTOCOL

function messageType(protocol: WidgetProtocol, action: string) {
  return `${protocol}:${action}`
}

export function bindSupportHostBridge(options: HostBridgeOptions = {}) {
  if (typeof window === "undefined") {
    return () => undefined
  }

  if (window.parent && window.parent !== window) {
    window.parent.postMessage({ type: messageType(PRIMARY_PROTOCOL, "ready") }, "*")
    window.parent.postMessage({ type: messageType(LEGACY_PROTOCOL, "ready") }, "*")
  }

  const handleMessage = (event: MessageEvent) => {
    const data = event.data as
      | {
          type?: string
          payload?: SupportChatRuntimeConfig | { isMaximized?: boolean }
        }
      | undefined
    if (!data?.type) {
      return
    }

    if (
      (data.type === messageType(PRIMARY_PROTOCOL, "init") ||
        data.type === messageType(LEGACY_PROTOCOL, "init")) &&
      data.payload
    ) {
      parentProtocol = data.type.startsWith(`${LEGACY_PROTOCOL}:`)
        ? LEGACY_PROTOCOL
        : PRIMARY_PROTOCOL
      setSupportChatRuntimeConfig(data.payload as SupportChatRuntimeConfig)
      options.onInit?.()
      return
    }

    if (data.type === messageType(PRIMARY_PROTOCOL, "open") || data.type === messageType(LEGACY_PROTOCOL, "open")) {
      options.onOpen?.()
      return
    }

    if (data.type === messageType(PRIMARY_PROTOCOL, "minimize") || data.type === messageType(LEGACY_PROTOCOL, "minimize")) {
      options.onMinimize?.()
      return
    }

    if (data.type === messageType(PRIMARY_PROTOCOL, "maximized") || data.type === messageType(LEGACY_PROTOCOL, "maximized")) {
      const payload = data.payload as { isMaximized?: boolean } | undefined
      options.onMaximizedChange?.(Boolean(payload?.isMaximized))
    }
  }

  window.addEventListener("message", handleMessage)
  return () => window.removeEventListener("message", handleMessage)
}

function postToParent(action: string, protocol: WidgetProtocol = parentProtocol) {
  if (typeof window === "undefined") {
    return
  }
  if (window.parent && window.parent !== window) {
    window.parent.postMessage({ type: messageType(protocol, action) }, "*")
  }
}

export function requestSupportHostMinimize() {
  postToParent("request-minimize")
}

export function requestSupportHostClose() {
  postToParent("request-close")
}

export function requestSupportHostToggleMaximize() {
  postToParent("request-toggle-maximize")
}
