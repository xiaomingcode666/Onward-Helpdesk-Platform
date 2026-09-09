export type RemoteHelpDeskWidgetConfig = {
  channelId: string
  baseUrl?: string
  apiBaseUrl?: string
  widgetBaseUrl?: string
  /** Stable external visitor ID. Uses the browser-local visitor ID when omitted. */
  externalId?: string
  /** Visitor display name, only used when first exchanging for a chat token. */
  externalName?: string
  /** Gets the user JWT issued by the host system before opening support. */
  getUserToken?: () => string | Promise<string>
  title?: string
  subtitle?: string
  language?: string
  position?: "left" | "right"
  themeColor?: string
  width?: string
}

export type SupportChatRuntimeConfig = Omit<RemoteHelpDeskWidgetConfig, "getUserToken"> & {
  /** Used only by /support/chat to exchange for a chat token; not part of the host widget config. */
  userToken?: string
}

export type RemoteHelpDeskWidget = {
  mount: (config?: RemoteHelpDeskWidgetConfig) => void
  destroy: () => void
  open: () => Promise<void>
  close: () => void
  getChatUrl: () => Promise<string>
}

/** @deprecated Use RemoteHelpDeskWidgetConfig. */
export type AgentDeskConfig = RemoteHelpDeskWidgetConfig

/** @deprecated Use RemoteHelpDeskWidget. */
export type AgentDeskWidget = RemoteHelpDeskWidget

declare global {
  interface Window {
    RemoteHelpDeskConfig?: RemoteHelpDeskWidgetConfig
    RemoteHelpDeskWidget?: RemoteHelpDeskWidget
    __REMOTE_HELP_DESK_WIDGET_CONFIG__?: SupportChatRuntimeConfig
    __REMOTE_HELP_DESK_WIDGET_STATE__?: unknown
    /** @deprecated Compatibility alias for integrations using the former SDK globals. */
    AgentDeskConfig?: RemoteHelpDeskWidgetConfig
    /** @deprecated Compatibility alias for integrations using the former SDK globals. */
    AgentDeskWidget?: RemoteHelpDeskWidget
    /** @deprecated Compatibility alias for cached SDK builds. */
    __CS_AI_AGENT_WIDGET_CONFIG__?: SupportChatRuntimeConfig
    /** @deprecated Compatibility alias for cached SDK builds. */
    __CS_AI_AGENT_WIDGET_STATE__?: unknown
  }
}
