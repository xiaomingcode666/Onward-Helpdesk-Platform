"use client"

import { useEffect } from "react"

import { useAuth } from "@/components/auth-provider"
import { heartbeatCustomerPortalPresence } from "@/lib/api/customer-presence"

const CUSTOMER_PORTAL_PRESENCE_HEARTBEAT_MS = 30_000

export function useCustomerPortalPresenceHeartbeat(enabled = true) {
  const { session } = useAuth()
  const heartbeatKey =
    enabled && session?.domainType === "customer" && session.subjectType === "customer_user" && session.user?.id
      ? `${session.tenantId}:${session.subjectType ?? ""}:${session.subjectId ?? session.user.id}`
      : ""

  useEffect(() => {
    if (!heartbeatKey) {
      return
    }

    let stopped = false
    const heartbeat = () => {
      if (stopped || document.visibilityState === "hidden") {
        return
      }
      void heartbeatCustomerPortalPresence().catch(() => undefined)
    }
    const handleVisibilityChange = () => {
      if (document.visibilityState === "visible") {
        heartbeat()
      }
    }

    heartbeat()
    const timer = window.setInterval(heartbeat, CUSTOMER_PORTAL_PRESENCE_HEARTBEAT_MS)
    window.addEventListener("focus", heartbeat)
    document.addEventListener("visibilitychange", handleVisibilityChange)
    return () => {
      stopped = true
      window.clearInterval(timer)
      window.removeEventListener("focus", heartbeat)
      document.removeEventListener("visibilitychange", handleVisibilityChange)
    }
  }, [heartbeatKey])
}
