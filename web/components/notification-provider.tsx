"use client"

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react"
import { useRouter } from "next/navigation"
import { toast } from "sonner"

import { useAuth } from "@/components/auth-provider"
import {
  createNotificationWebSocket,
  fetchNotifications,
  fetchNotificationUnreadCount,
  markNotificationRead,
  type NotificationItem,
} from "@/lib/api/notification"
import { readSession } from "@/lib/auth"
import {
  createRealtimeConnectionManager,
  type RealtimeConnectionStatus,
} from "@/lib/realtime-connection"
import { useAppLocale, useI18n } from "@/i18n/provider"
import { localizeNotificationItem } from "@/lib/notification-i18n"
import { normalizeEnterpriseActionUrl } from "@/lib/ticket-workbench-route"

type NotificationRealtimeEnvelope = {
  eventId?: string
  type?: string
  data?: {
    notification?: NotificationItem
  }
}

const TICKET_ASSIGNMENT_SYNC_INTERVAL_MS = 60 * 1000
const TICKET_ASSIGNMENT_PAGE_SIZE = 100
const TICKET_ASSIGNMENT_POPUP_LIMIT = 20

type NotificationSyncMessage = {
  senderId: string
  type: "changed"
}

type NotificationContextValue = {
  unreadCount: number
  notificationRevision: number
  realtimeStatus: RealtimeConnectionStatus
  ticketAssignmentNotifications: NotificationItem[]
  unreadTicketAssignmentIds: number[] | null
  hasMoreUnreadTicketAssignments: boolean
  unreadTicketAssignmentTotal: number
  dismissTicketAssignmentNotifications: (ids: number[]) => void
  notifyNotificationStateChange: () => void
  refreshUnreadCount: () => Promise<void>
  acknowledgeNotification: (notification: NotificationItem) => Promise<boolean>
  markReadAndNavigate: (notification: NotificationItem, fallbackActionUrl?: string) => Promise<void>
}

const NotificationContext = createContext<NotificationContextValue | null>(null)

export function NotificationProvider({ children }: { children: ReactNode }) {
  const t = useI18n()
  const { locale } = useAppLocale()
  const { ready, session } = useAuth()
  const router = useRouter()
  const [unreadCount, setUnreadCount] = useState(0)
  const [notificationRevision, setNotificationRevision] = useState(0)
  const [realtimeStatus, setRealtimeStatus] =
    useState<RealtimeConnectionStatus>("disconnected")
  const [ticketAssignmentNotifications, setTicketAssignmentNotifications] =
    useState<NotificationItem[]>([])
  const [unreadTicketAssignmentIds, setUnreadTicketAssignmentIds] =
    useState<number[] | null>(null)
  const [hasMoreUnreadTicketAssignments, setHasMoreUnreadTicketAssignments] =
    useState(false)
  const [unreadTicketAssignmentTotal, setUnreadTicketAssignmentTotal] =
    useState(0)
  const currentUserIdRef = useRef(readSession()?.user?.id ?? 0)
  const currentNotificationScopeRef = useRef("")
  const unreadCountLoadIdRef = useRef(0)
  const unreadTicketAssignmentLoadIdRef = useRef(0)
  const unreadTicketAssignmentIdsRef = useRef(new Set<number>())
  const unreadTicketAssignmentTotalRef = useRef(0)
  const notificationAcknowledgementsRef = useRef(
    new Map<number, Promise<boolean>>()
  )
  const notificationSyncChannelRef = useRef<BroadcastChannel | null>(null)
  const notificationSyncChannelNameRef = useRef("")
  const notificationTabIdRef = useRef("")
  const hasConnectedRef = useRef(false)
  const hasAuthenticatedSession = ready && Boolean(session?.accessToken)

  const refreshUnreadCount = useCallback(async () => {
    const authSession = readSession()
    if (!hasAuthenticatedSession || !authSession?.accessToken) {
      setUnreadCount(0)
      return
    }
    const requestId = unreadCountLoadIdRef.current + 1
    unreadCountLoadIdRef.current = requestId
    const requestScope = `${authSession.domainType}:${authSession.tenantId ?? 0}:${authSession.user?.id ?? 0}`
    const result = await fetchNotificationUnreadCount()
    if (
      unreadCountLoadIdRef.current !== requestId ||
      currentNotificationScopeRef.current !== requestScope
    ) {
      return
    }
    setUnreadCount(result.unreadCount)
  }, [hasAuthenticatedSession])

  const enqueueTicketAssignmentNotifications = useCallback((notifications: NotificationItem[]) => {
    if (notifications.length === 0) return
    setTicketAssignmentNotifications((current) => {
      const seen = new Set<number>()
      return [...notifications, ...current]
        .filter((item) => {
          if (item.id <= 0 || seen.has(item.id)) return false
          seen.add(item.id)
          return true
        })
        .slice(0, TICKET_ASSIGNMENT_POPUP_LIMIT)
    })
  }, [])

  const dismissTicketAssignmentNotifications = useCallback((ids: number[]) => {
    if (ids.length === 0) return
    const dismissed = new Set(ids)
    setTicketAssignmentNotifications((current) => current.filter((item) => !dismissed.has(item.id)))
  }, [])

  const loadUnreadTicketAssignments = useCallback(async () => {
    const authSession = readSession()
    if (
      !hasAuthenticatedSession ||
      !authSession?.accessToken ||
      authSession.domainType !== "enterprise"
    ) {
      return
    }
    const requestId = unreadTicketAssignmentLoadIdRef.current + 1
    unreadTicketAssignmentLoadIdRef.current = requestId
    const requestScope = `${authSession.domainType}:${authSession.tenantId ?? 0}:${authSession.user?.id ?? 0}`
    const notifications: NotificationItem[] = []
    let loadedCount = 0
    let page = 1
    let total = 0
    while (true) {
      const result = await fetchNotifications({
        page,
        limit: TICKET_ASSIGNMENT_PAGE_SIZE,
        readStatus: "unread",
        type: "ticket_assigned",
      })
      if (
        unreadTicketAssignmentLoadIdRef.current !== requestId ||
        currentNotificationScopeRef.current !== requestScope
      ) {
        return
      }
      const pageItems = result.results || []
      loadedCount += pageItems.length
      total = result.page.total
      notifications.push(
        ...pageItems.filter(
          (item) => item.recipientUserId === currentUserIdRef.current
        )
      )
      if (pageItems.length === 0 || loadedCount >= total) break
      page += 1
    }
    const seenIds = new Set<number>()
    const uniqueNotifications = notifications.filter((item) => {
      if (item.id <= 0 || seenIds.has(item.id)) return false
      seenIds.add(item.id)
      return true
    })
    unreadTicketAssignmentIdsRef.current = seenIds
    unreadTicketAssignmentTotalRef.current = total
    setUnreadTicketAssignmentIds(uniqueNotifications.map((item) => item.id))
    setUnreadTicketAssignmentTotal(total)
    setHasMoreUnreadTicketAssignments(total > TICKET_ASSIGNMENT_POPUP_LIMIT)
    enqueueTicketAssignmentNotifications(
      uniqueNotifications.slice(0, TICKET_ASSIGNMENT_POPUP_LIMIT)
    )
  }, [enqueueTicketAssignmentNotifications, hasAuthenticatedSession])

  const broadcastNotificationStateChange = useCallback((
    channelName = notificationSyncChannelNameRef.current
  ) => {
    if (!channelName || !notificationTabIdRef.current) return
    const message = {
      senderId: notificationTabIdRef.current,
      type: "changed",
    } satisfies NotificationSyncMessage
    if (
      channelName === notificationSyncChannelNameRef.current &&
      notificationSyncChannelRef.current
    ) {
      notificationSyncChannelRef.current.postMessage(message)
      return
    }
    if (typeof BroadcastChannel === "undefined") return
    const channel = new BroadcastChannel(channelName)
    channel.postMessage(message)
    window.setTimeout(() => channel.close(), 0)
  }, [])

  const notifyNotificationStateChange = useCallback(() => {
    setNotificationRevision((current) => current + 1)
    void loadUnreadTicketAssignments().catch(() => undefined)
    broadcastNotificationStateChange()
  }, [broadcastNotificationStateChange, loadUnreadTicketAssignments])

  const acknowledgeNotification = useCallback(
    (notification: NotificationItem) => {
      if (notification.readAt) return Promise.resolve(true)
      const pending = notificationAcknowledgementsRef.current.get(notification.id)
      if (pending) return pending

      const acknowledgementScope = currentNotificationScopeRef.current
      const notificationChannelName = notificationSyncChannelNameRef.current
      const acknowledgement = markNotificationRead(notification.id)
        .then(() => {
          if (currentNotificationScopeRef.current !== acknowledgementScope) {
            broadcastNotificationStateChange(notificationChannelName)
            return true
          }
          unreadTicketAssignmentLoadIdRef.current += 1
          if (notification.notificationType === "ticket_assigned") {
            unreadTicketAssignmentIdsRef.current.delete(notification.id)
            setUnreadTicketAssignmentIds((current) =>
              current?.filter((id) => id !== notification.id) ?? current
            )
            const nextTotal = Math.max(
              0,
              unreadTicketAssignmentTotalRef.current - 1
            )
            unreadTicketAssignmentTotalRef.current = nextTotal
            setUnreadTicketAssignmentTotal(nextTotal)
            dismissTicketAssignmentNotifications([notification.id])
          }
          setNotificationRevision((current) => current + 1)
          void refreshUnreadCount().catch(() => undefined)
          broadcastNotificationStateChange()
          return true
        })
        .catch(() => false)
      notificationAcknowledgementsRef.current.set(notification.id, acknowledgement)
      void acknowledgement.then(() => {
        if (
          notificationAcknowledgementsRef.current.get(notification.id) ===
          acknowledgement
        ) {
          notificationAcknowledgementsRef.current.delete(notification.id)
        }
      })
      return acknowledgement
    },
    [
      dismissTicketAssignmentNotifications,
      broadcastNotificationStateChange,
      refreshUnreadCount,
    ]
  )

  const markReadAndNavigate = useCallback(
    async (notification: NotificationItem, fallbackActionUrl?: string) => {
      void acknowledgeNotification(notification)
      const actionUrl =
        normalizeEnterpriseActionUrl({
          actionUrl: notification.actionUrl,
          notificationType: notification.notificationType,
          bizType: notification.bizType,
          bizId: notification.bizId,
        }) || normalizeEnterpriseActionUrl(fallbackActionUrl)
      if (actionUrl) {
        router.push(actionUrl)
      }
    },
    [acknowledgeNotification, router]
  )

  useEffect(() => {
    if (!hasAuthenticatedSession) {
      currentUserIdRef.current = 0
      currentNotificationScopeRef.current = ""
      unreadCountLoadIdRef.current += 1
      unreadTicketAssignmentLoadIdRef.current += 1
      unreadTicketAssignmentIdsRef.current.clear()
      unreadTicketAssignmentTotalRef.current = 0
      notificationAcknowledgementsRef.current.clear()
      setUnreadCount(0)
      setTicketAssignmentNotifications([])
      setUnreadTicketAssignmentIds(null)
      setHasMoreUnreadTicketAssignments(false)
      setUnreadTicketAssignmentTotal(0)
      return
    }
    const storedSession = readSession()
    const nextUserId = session?.user?.id ?? storedSession?.user?.id ?? 0
    const nextTenantId = session?.tenantId ?? storedSession?.tenantId ?? 0
    const nextDomainType = session?.domainType ?? storedSession?.domainType ?? ""
    const nextScope = `${nextDomainType}:${nextTenantId}:${nextUserId}`
    if (
      currentNotificationScopeRef.current &&
      currentNotificationScopeRef.current !== nextScope
    ) {
      unreadCountLoadIdRef.current += 1
      unreadTicketAssignmentLoadIdRef.current += 1
      unreadTicketAssignmentIdsRef.current.clear()
      unreadTicketAssignmentTotalRef.current = 0
      notificationAcknowledgementsRef.current.clear()
      setTicketAssignmentNotifications([])
      setUnreadTicketAssignmentIds(null)
      setHasMoreUnreadTicketAssignments(false)
      setUnreadTicketAssignmentTotal(0)
      setUnreadCount(0)
      hasConnectedRef.current = false
    }
    currentNotificationScopeRef.current = nextScope
    currentUserIdRef.current = nextUserId
    const timer = window.setTimeout(() => {
      void refreshUnreadCount().catch(() => undefined)
      void loadUnreadTicketAssignments().catch(() => undefined)
    }, 0)
    const assignmentSyncInterval = nextDomainType === "enterprise"
      ? window.setInterval(() => {
          void refreshUnreadCount().catch(() => undefined)
          void loadUnreadTicketAssignments().catch(() => undefined)
        }, TICKET_ASSIGNMENT_SYNC_INTERVAL_MS)
      : null
    return () => {
      window.clearTimeout(timer)
      if (assignmentSyncInterval !== null) {
        window.clearInterval(assignmentSyncInterval)
      }
    }
  }, [
    hasAuthenticatedSession,
    loadUnreadTicketAssignments,
    refreshUnreadCount,
    session?.domainType,
    session?.tenantId,
    session?.user?.id,
  ])

  useEffect(() => {
    if (
      !hasAuthenticatedSession ||
      typeof BroadcastChannel === "undefined"
    ) {
      notificationSyncChannelRef.current = null
      return
    }
    const authSession = readSession()
    if (!authSession?.accessToken) return
    if (!notificationTabIdRef.current) {
      notificationTabIdRef.current = typeof globalThis.crypto?.randomUUID === "function"
        ? globalThis.crypto.randomUUID()
        : `notification-tab-${Date.now()}-${Math.random().toString(36).slice(2)}`
    }
    const scope = `${authSession.domainType}:${authSession.tenantId ?? 0}:${authSession.user?.id ?? 0}`
    const channel = new BroadcastChannel(`rhd:notifications:${scope}`)
    notificationSyncChannelNameRef.current = channel.name
    notificationSyncChannelRef.current = channel
    channel.onmessage = (event: MessageEvent<NotificationSyncMessage>) => {
      const message = event.data
      if (
        !message ||
        message.type !== "changed" ||
        message.senderId === notificationTabIdRef.current
      ) {
        return
      }
      setNotificationRevision((current) => current + 1)
      void refreshUnreadCount().catch(() => undefined)
      void loadUnreadTicketAssignments().catch(() => undefined)
    }
    return () => {
      if (notificationSyncChannelRef.current === channel) {
        notificationSyncChannelRef.current = null
        notificationSyncChannelNameRef.current = ""
      }
      channel.close()
    }
  }, [
    hasAuthenticatedSession,
    loadUnreadTicketAssignments,
    refreshUnreadCount,
    session?.accessToken,
    session?.domainType,
    session?.tenantId,
    session?.user?.id,
  ])

  useEffect(() => {
    if (!hasAuthenticatedSession) {
      hasConnectedRef.current = false
      setRealtimeStatus("disconnected")
      return
    }
    const realtime = createRealtimeConnectionManager({
      createSocket: createNotificationWebSocket,
      canReconnect: () => Boolean(readSession()?.accessToken),
      onStatusChange: setRealtimeStatus,
      onOpen: () => {
        void refreshUnreadCount().catch(() => undefined)
        if (hasConnectedRef.current) {
          void loadUnreadTicketAssignments().catch(() => undefined)
          setNotificationRevision((current) => current + 1)
        }
        hasConnectedRef.current = true
      },
      onMessage: (event, socket) => {
        try {
          const envelope = JSON.parse(event.data) as NotificationRealtimeEnvelope
          const eventType = envelope.type ?? ""
          const eventId = envelope.eventId?.trim() ?? ""
          if (
            eventType === "" ||
            eventType === "connected" ||
            eventType === "pong" ||
            eventType === "subscribed" ||
            eventType === "unsubscribed"
          ) {
            return
          }
          if (eventId && socket.readyState === WebSocket.OPEN) {
            socket.send(JSON.stringify({ type: "ack", eventId }))
          }
          if (eventType !== "notification.created") {
            if (eventType === "resyncRequired") {
              void refreshUnreadCount().catch(() => undefined)
              void loadUnreadTicketAssignments().catch(() => undefined)
              setNotificationRevision((current) => current + 1)
            }
            return
          }
          const notification = envelope.data?.notification
          if (!notification || notification.recipientUserId !== currentUserIdRef.current) {
            return
          }
          const localizedNotification = localizeNotificationItem(notification, locale)
          void refreshUnreadCount().catch(() => undefined)
          setNotificationRevision((current) => current + 1)
          if (
            localizedNotification.notificationType === "ticket_assigned" &&
            readSession()?.domainType === "enterprise"
          ) {
            unreadTicketAssignmentLoadIdRef.current += 1
            if (!unreadTicketAssignmentIdsRef.current.has(localizedNotification.id)) {
              unreadTicketAssignmentIdsRef.current.add(localizedNotification.id)
              unreadTicketAssignmentTotalRef.current += 1
              setUnreadTicketAssignmentTotal(unreadTicketAssignmentTotalRef.current)
              setHasMoreUnreadTicketAssignments(
                unreadTicketAssignmentTotalRef.current > TICKET_ASSIGNMENT_POPUP_LIMIT
              )
            }
            setUnreadTicketAssignmentIds((current) => {
              const ids = current ?? []
              return ids.includes(localizedNotification.id)
                ? ids
                : [localizedNotification.id, ...ids]
            })
            enqueueTicketAssignmentNotifications([localizedNotification])
            void loadUnreadTicketAssignments().catch(() => undefined)
            return
          }
          toast(localizedNotification.title || t("notification.new"), {
            description: localizedNotification.content,
            action: {
              label: t("notification.view"),
              onClick: () => {
                void markReadAndNavigate(localizedNotification).catch((error) => {
                  toast.error(error instanceof Error ? error.message : t("notification.openFailed"))
                })
              },
            },
          })
        } catch {
          // ignore invalid realtime payload
        }
      },
      onConnectError: (error) => {
        toast.error(error instanceof Error ? error.message : t("notification.connectFailed"))
      },
    })

    realtime.connect()
    return () => {
      realtime.disconnect()
    }
  }, [
    enqueueTicketAssignmentNotifications,
    hasAuthenticatedSession,
    loadUnreadTicketAssignments,
    locale,
    markReadAndNavigate,
    refreshUnreadCount,
    session?.accessToken,
    session?.domainType,
    session?.tenantId,
    session?.user?.id,
    t,
  ])

  const value = useMemo<NotificationContextValue>(
    () => ({
      unreadCount,
      notificationRevision,
      realtimeStatus,
      ticketAssignmentNotifications,
      unreadTicketAssignmentIds,
      hasMoreUnreadTicketAssignments,
      unreadTicketAssignmentTotal,
      dismissTicketAssignmentNotifications,
      notifyNotificationStateChange,
      refreshUnreadCount,
      acknowledgeNotification,
      markReadAndNavigate,
    }),
    [
      acknowledgeNotification,
      dismissTicketAssignmentNotifications,
      hasMoreUnreadTicketAssignments,
      markReadAndNavigate,
      notificationRevision,
      notifyNotificationStateChange,
      realtimeStatus,
      refreshUnreadCount,
      ticketAssignmentNotifications,
      unreadTicketAssignmentTotal,
      unreadTicketAssignmentIds,
      unreadCount,
    ]
  )

  return (
    <NotificationContext.Provider value={value}>
      {children}
    </NotificationContext.Provider>
  )
}

export function useNotifications() {
  const context = useContext(NotificationContext)
  if (!context) {
    throw new Error("useNotifications must be used within NotificationProvider")
  }
  return context
}
