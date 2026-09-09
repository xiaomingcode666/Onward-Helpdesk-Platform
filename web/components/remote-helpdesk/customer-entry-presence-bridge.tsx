"use client"

import { useEffect } from "react"

import {
  exchangeCustomerEntrySession,
  type CustomerEntryChat,
} from "@/lib/api/customer-entry"
import { createCustomerSessionWebSocket } from "@/lib/api/websocket"
import { createRealtimeConnectionManager } from "@/lib/realtime-connection"

type CustomerEntryPresenceBridgeProps = {
  chat: CustomerEntryChat
  conversationId: number
}

type CustomerSessionRefreshEnvelope = {
  type?: string
  data?: {
    customerSessionToken?: string
  }
}

export function CustomerEntryPresenceBridge({
  chat,
  conversationId,
}: CustomerEntryPresenceBridgeProps) {
  useEffect(() => {
    if (conversationId <= 0) return

    let active = true
    let realtimeToken = ""
    let realtime: ReturnType<typeof createRealtimeConnectionManager> | null = null
    let retryTimer: number | null = null
    let retryAttempt = 0

    const scheduleRetry = () => {
      if (!active || retryTimer !== null) return
      const delay = Math.min(500 * 2 ** retryAttempt, 5_000)
      retryAttempt += 1
      retryTimer = window.setTimeout(() => {
        retryTimer = null
        void connect()
      }, delay)
    }

    async function connect() {
      try {
        const session = await exchangeCustomerEntrySession({
          entrySessionId: chat.entrySessionId,
          visitorId: chat.visitorId,
          visitorToken: chat.visitorToken,
        })
        if (!active) return

        realtimeToken = session.customerSessionToken
        retryAttempt = 0
        const protocol = window.location.protocol === "https:" ? "wss:" : "ws:"
        realtime = createRealtimeConnectionManager({
          createSocket: () => createCustomerSessionWebSocket(
            `${protocol}//${window.location.host}/api/ws/open`,
            realtimeToken,
          ),
          canReconnect: () => active,
          onOpen: (socket) => {
            socket.send(
              JSON.stringify({
                type: "subscribe",
                topics: [`conversation:${conversationId}`],
              }),
            )
          },
          onMessage: (event) => {
            try {
              const envelope = JSON.parse(String(event.data)) as CustomerSessionRefreshEnvelope
              const refreshedToken = envelope.data?.customerSessionToken?.trim()
              if (envelope.type === "customer_session.refresh" && refreshedToken) {
                realtimeToken = refreshedToken
              }
            } catch {
              // Presence only needs session refresh events; other realtime payloads are ignored.
            }
          },
          reconnectBaseDelayMs: 500,
          reconnectMaxDelayMs: 5_000,
        })
        realtime.connect()
      } catch {
        scheduleRetry()
      }
    }

    void connect()
    return () => {
      active = false
      if (retryTimer !== null) {
        window.clearTimeout(retryTimer)
      }
      realtime?.disconnect()
    }
  }, [chat.entrySessionId, chat.visitorId, chat.visitorToken, conversationId])

  return null
}
