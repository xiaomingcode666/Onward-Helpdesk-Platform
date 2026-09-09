"use client"

import {
  ArrowRightIcon,
  BellRingIcon,
  CalendarClockIcon,
  CheckIcon,
  Loader2Icon,
  TicketCheckIcon,
  XIcon,
  type LucideIcon,
} from "lucide-react"
import { useRouter } from "next/navigation"
import { useCallback, useEffect, useRef, useState } from "react"
import { toast } from "sonner"

import { useAuth } from "@/components/auth-provider"
import { useNotifications } from "@/components/notification-provider"
import { Button } from "@/components/ui/button"
import { translateCurrentMessage } from "@/i18n/messages"
import { useAppLocale, useI18n } from "@/i18n/provider"
import {
  fetchEnterpriseReminders,
  type EnterpriseMeetingReminder,
  type EnterpriseTicketReminder,
} from "@/lib/api/enterprise-reminders"
import type { NotificationItem } from "@/lib/api/notification"
import { readSession } from "@/lib/auth"
import { localizeNotificationItem } from "@/lib/notification-i18n"
import {
  buildEnterpriseTicketWorkbenchPath,
  normalizeEnterpriseActionUrl,
  normalizeWorkbenchId,
} from "@/lib/ticket-workbench-route"
import { cn, formatDateTime } from "@/lib/utils"

const REMINDER_POLL_INTERVAL_MS = 60 * 1000
const REMINDER_INITIAL_LOOKBACK_MS = 60 * 1000
const REMINDER_SEEN_TTL_MS = 12 * 60 * 60 * 1000
const ASSIGNMENT_NOTIFICATION_SEEN_TTL_MS = 30 * 24 * 60 * 60 * 1000
const ASSIGNMENT_BACKLOG_REMINDER_KEY = "ticket-assignment-backlog"

type ReminderKind = "meeting" | "ticket"

type ReminderCard = {
  key: string
  kind: ReminderKind
  ticketId?: number
  title: string
  eyebrow: string
  description: string
  meta: string
  actionUrl: string
  icon: LucideIcon
  tone: string
  notification?: NotificationItem
}

type SeenReminderStore = Record<string, number>

type ReminderBroadcastMessage = {
  senderId: string
  keys: string[]
  action?: "claim" | "dismiss"
}

type I18nT = ReturnType<typeof useI18n>

const rp = (t: I18nT, key: string, values?: Record<string, string | number>) =>
  t(`meetingRoomExtract.reminders.${key}`, values)

function seenStorageKey(userId: number, tenantId: number) {
  return `rhd:enterprise-reminders:seen:${tenantId}:${userId}`
}

function pruneSeenStore(store: SeenReminderStore, now: number) {
  const next: SeenReminderStore = {}
  for (const [key, expiresAt] of Object.entries(store)) {
    if (expiresAt > now) {
      next[key] = expiresAt
    }
  }
  return next
}

function readSeenReminders(key: string, now: number): SeenReminderStore {
  try {
    const raw = window.localStorage.getItem(key)
    if (!raw) return {}
    return pruneSeenStore(JSON.parse(raw) as SeenReminderStore, now)
  } catch {
    try {
      window.localStorage.removeItem(key)
    } catch {
      // Storage can be unavailable in restricted browser contexts.
    }
    return {}
  }
}

function writeSeenReminders(key: string, store: SeenReminderStore) {
  try {
    window.localStorage.setItem(key, JSON.stringify(store))
  } catch {
    // Best-effort local de-duplication; polling should continue even if storage is unavailable.
  }
}

function forgetSeenReminder(storeKey: string, reminderKey: string, now: number) {
  const store = readSeenReminders(storeKey, now)
  delete store[reminderKey]
  writeSeenReminders(storeKey, store)
}

function meetingStartLabel(minutes: number) {
  if (minutes <= 0) return rp(translateCurrentMessage, "startsImminently")
  if (minutes < 60) return rp(translateCurrentMessage, "startsInMinutes", { minutes })
  return rp(translateCurrentMessage, "startsWithinHour")
}

function meetingReminderToCard(item: EnterpriseMeetingReminder): ReminderCard {
  const ticketId = normalizeWorkbenchId(item.ticket_id)
  return {
    key: item.id || `meeting:${item.meeting_id}:${item.scheduled_at}`,
    kind: "meeting",
    title: item.title || rp(translateCurrentMessage, "meetingTitleFallback"),
    eyebrow: meetingStartLabel(item.starts_in_minutes),
    description: item.ticket_no ? `${item.ticket_no} · ${item.room_name}` : item.room_name || rp(translateCurrentMessage, "meetingDescriptionFallback"),
    meta: rp(translateCurrentMessage, "scheduledAt", { date: formatDateTime(item.scheduled_at) }),
    actionUrl: ticketId > 0
      ? buildEnterpriseTicketWorkbenchPath(ticketId)
      : item.action_url || `/enterprise/video?meeting_id=${encodeURIComponent(item.meeting_id)}`,
    icon: CalendarClockIcon,
    tone: "border-primary/20 bg-primary/10 text-primary",
  }
}

function ticketReminderToCard(item: EnterpriseTicketReminder): ReminderCard {
  const ticketId = normalizeWorkbenchId(item.ticket_id)
  return {
    key: item.id || `ticket:${item.ticket_id}:${item.created_at}`,
    kind: "ticket",
    ticketId: ticketId || undefined,
    title: item.title || rp(translateCurrentMessage, "newTicketTitleFallback"),
    eyebrow: rp(translateCurrentMessage, "productGroupNewTicket"),
    description: `${item.ticket_no || `#${item.ticket_id}`} · ${item.product_name || item.team_name || rp(translateCurrentMessage, "myProductGroup")}`,
    meta: rp(translateCurrentMessage, "createdAt", { date: formatDateTime(item.created_at) }),
    actionUrl: ticketId > 0 ? buildEnterpriseTicketWorkbenchPath(ticketId) : item.action_url || "/enterprise/ticket-workbench",
    icon: TicketCheckIcon,
    tone: "border-border bg-muted text-muted-foreground",
  }
}

function ticketAssignmentNotificationToCard(
  item: NotificationItem,
  locale: string,
  t: I18nT,
): ReminderCard {
  const notification = localizeNotificationItem(item, locale)
  const ticketId = normalizeWorkbenchId(notification.bizId)
  return {
    key: `notification:${notification.id}`,
    kind: "ticket",
    ticketId: ticketId || undefined,
    title: notification.title || t("notification.ticketAssignedTitle"),
    eyebrow: rp(t, "ticketAssignedToMe"),
    description: notification.content,
    meta: rp(t, "assignedAt", { date: formatDateTime(notification.createdAt) }),
    actionUrl: ticketId > 0
      ? buildEnterpriseTicketWorkbenchPath(ticketId)
      : normalizeEnterpriseActionUrl({
          actionUrl: notification.actionUrl,
          notificationType: notification.notificationType,
          bizType: notification.bizType,
          bizId: notification.bizId,
        }) || "/enterprise/notifications",
    icon: TicketCheckIcon,
    tone: "border-primary/20 bg-primary/10 text-primary",
    notification,
  }
}

function reminderPriority(card: ReminderCard) {
  if (card.notification) return 3
  if (card.kind === "meeting") return 2
  return 1
}

function mergeReminderCards(current: ReminderCard[], incoming: ReminderCard[]) {
  const combined = [...incoming, ...current]
  const assignedTicketIds = new Set(
    combined
      .filter((card) => card.notification && card.ticketId)
      .map((card) => card.ticketId as number)
  )
  const keys = new Set<string>()
  const assignmentTicketIds = new Set<number>()
  return combined
    .filter(
      (card) =>
        !(
          card.kind === "ticket" &&
          !card.notification &&
          card.ticketId &&
          assignedTicketIds.has(card.ticketId)
        )
    )
    .filter((card) => {
      if (card.notification && card.ticketId) {
        if (assignmentTicketIds.has(card.ticketId)) return false
        assignmentTicketIds.add(card.ticketId)
      }
      if (keys.has(card.key)) return false
      keys.add(card.key)
      return true
    })
    .sort((left, right) => reminderPriority(right) - reminderPriority(left))
    .slice(0, 50)
}

function initialSince() {
  return new Date(Date.now() - REMINDER_INITIAL_LOOKBACK_MS).toISOString()
}

export function EnterpriseReminderPoller() {
  const router = useRouter()
  const t = useI18n()
  const { locale } = useAppLocale()
  const { session: authSession } = useAuth()
  const {
    acknowledgeNotification,
    dismissTicketAssignmentNotifications,
    hasMoreUnreadTicketAssignments,
    ticketAssignmentNotifications,
    unreadTicketAssignmentIds,
    unreadTicketAssignmentTotal,
  } = useNotifications()
  const [open, setOpen] = useState(false)
  const [showBacklogPrompt, setShowBacklogPrompt] = useState(false)
  const [items, setItems] = useState<ReminderCard[]>([])
  const [acknowledgingKeys, setAcknowledgingKeys] = useState<Set<string>>(
    () => new Set()
  )
  const [pageVisible, setPageVisible] = useState(
    () => typeof document === "undefined" || document.visibilityState !== "hidden"
  )
  const hasAssignmentBacklog =
    hasMoreUnreadTicketAssignments ||
    unreadTicketAssignmentTotal > (unreadTicketAssignmentIds?.length ?? 0)
  const sinceRef = useRef(initialSince())
  const pollingRef = useRef(false)
  const pollRequestIdRef = useRef(0)
  const pollerScopeRef = useRef("")
  const reminderChannelRef = useRef<BroadcastChannel | null>(null)
  const reminderChannelNameRef = useRef("")
  const reminderTabIdRef = useRef("")
  const backlogPromptedRef = useRef(false)

  useEffect(() => {
    const nextScope = authSession?.accessToken
      ? `${authSession.domainType ?? ""}:${authSession.tenantId ?? 0}:${authSession.user?.id ?? 0}`
      : ""
    if (pollerScopeRef.current !== nextScope) {
      pollerScopeRef.current = nextScope
      sinceRef.current = initialSince()
      pollingRef.current = false
      pollRequestIdRef.current += 1
      backlogPromptedRef.current = false
      setItems([])
      setAcknowledgingKeys(new Set())
      setShowBacklogPrompt(false)
      setOpen(false)
    }
  }, [authSession?.accessToken, authSession?.domainType, authSession?.tenantId, authSession?.user?.id])

  useEffect(() => {
    if (
      !authSession?.accessToken ||
      authSession.domainType !== "enterprise" ||
      typeof BroadcastChannel === "undefined"
    ) {
      reminderChannelRef.current = null
      return
    }
    if (!reminderTabIdRef.current) {
      reminderTabIdRef.current = typeof globalThis.crypto?.randomUUID === "function"
        ? globalThis.crypto.randomUUID()
        : `tab-${Date.now()}-${Math.random().toString(36).slice(2)}`
    }
    const channel = new BroadcastChannel(
      `rhd:enterprise-reminders:${authSession.tenantId ?? 0}:${authSession.user?.id ?? 0}`
    )
    reminderChannelNameRef.current = channel.name
    reminderChannelRef.current = channel
    channel.onmessage = (event: MessageEvent<ReminderBroadcastMessage>) => {
      const message = event.data
      if (
        !message ||
        typeof message.senderId !== "string" ||
        !Array.isArray(message.keys)
      ) {
        return
      }
      const remoteKeys = new Set(message.keys.filter((key) => typeof key === "string"))
      if (message.action === "dismiss") {
        setItems((current) => current.filter((item) => !remoteKeys.has(item.key)))
        if (remoteKeys.has(ASSIGNMENT_BACKLOG_REMINDER_KEY)) {
          setShowBacklogPrompt(false)
        }
        return
      }
      const senderOrder = message.senderId.localeCompare(reminderTabIdRef.current)
      if (senderOrder >= 0) {
        if (
          senderOrder > 0 &&
          remoteKeys.has(ASSIGNMENT_BACKLOG_REMINDER_KEY) &&
          backlogPromptedRef.current
        ) {
          channel.postMessage({
            senderId: reminderTabIdRef.current,
            keys: [ASSIGNMENT_BACKLOG_REMINDER_KEY],
            action: "claim",
          } satisfies ReminderBroadcastMessage)
        }
        return
      }
      setItems((current) => current.filter((item) => !remoteKeys.has(item.key)))
      if (remoteKeys.has(ASSIGNMENT_BACKLOG_REMINDER_KEY)) {
        backlogPromptedRef.current = true
        setShowBacklogPrompt(false)
      }
    }
    return () => {
      if (reminderChannelRef.current === channel) {
        reminderChannelRef.current = null
        reminderChannelNameRef.current = ""
      }
      channel.close()
    }
  }, [authSession?.accessToken, authSession?.domainType, authSession?.tenantId, authSession?.user?.id])

  const announceCards = useCallback((cards: ReminderCard[]) => {
    const keys = cards.map((card) => card.key)
    if (keys.length === 0) return
    reminderChannelRef.current?.postMessage({
      senderId: reminderTabIdRef.current,
      keys,
      action: "claim",
    } satisfies ReminderBroadcastMessage)
  }, [])

  const announceBacklogPrompt = useCallback(() => {
    reminderChannelRef.current?.postMessage({
      senderId: reminderTabIdRef.current,
      keys: [ASSIGNMENT_BACKLOG_REMINDER_KEY],
      action: "claim",
    } satisfies ReminderBroadcastMessage)
  }, [])

  const announceDismissedCards = useCallback((
    cards: ReminderCard[],
    channelName = reminderChannelNameRef.current
  ) => {
    const keys = cards.map((card) => card.key)
    if (keys.length === 0) return
    const message = {
      senderId: reminderTabIdRef.current,
      keys,
      action: "dismiss",
    } satisfies ReminderBroadcastMessage
    if (
      channelName &&
      channelName === reminderChannelNameRef.current &&
      reminderChannelRef.current
    ) {
      reminderChannelRef.current.postMessage(message)
      return
    }
    if (!channelName || typeof BroadcastChannel === "undefined") return
    const channel = new BroadcastChannel(channelName)
    channel.postMessage(message)
    window.setTimeout(() => channel.close(), 0)
  }, [])

  const rememberCards = useCallback((cards: ReminderCard[], storeKey: string, now: number) => {
    if (cards.length === 0) return
    const store = readSeenReminders(storeKey, now)
    for (const card of cards) {
      const ttl = card.notification
        ? ASSIGNMENT_NOTIFICATION_SEEN_TTL_MS
        : REMINDER_SEEN_TTL_MS
      store[card.key] = now + ttl
    }
    writeSeenReminders(storeKey, store)
  }, [])

  useEffect(() => {
    if (!pageVisible || ticketAssignmentNotifications.length === 0) return
    const session = readSession()
    if (!session?.accessToken || session.domainType !== "enterprise") return
    const userId = session.user?.id || 0
    const tenantId = session.tenantId || 0
    const notificationIds = ticketAssignmentNotifications.map((item) => item.id)
    const now = Date.now()
    const storeKey = seenStorageKey(userId, tenantId)
    const seen = readSeenReminders(storeKey, now)
    const cards = ticketAssignmentNotifications
      .map((item) => ticketAssignmentNotificationToCard(item, locale, t))
      .filter((card) => !seen[card.key])
    dismissTicketAssignmentNotifications(notificationIds)
    if (cards.length === 0) {
      writeSeenReminders(storeKey, seen)
      if (hasAssignmentBacklog && !backlogPromptedRef.current) {
        backlogPromptedRef.current = true
        setShowBacklogPrompt(true)
        announceBacklogPrompt()
        setOpen(true)
      }
      return
    }
    if (hasAssignmentBacklog) {
      backlogPromptedRef.current = true
      setShowBacklogPrompt(true)
      announceBacklogPrompt()
    }
    rememberCards(cards, storeKey, now)
    announceCards(cards)
    setItems((current) => mergeReminderCards(current, cards))
    setOpen(true)
  }, [
    announceCards,
    announceBacklogPrompt,
    dismissTicketAssignmentNotifications,
    hasAssignmentBacklog,
    locale,
    pageVisible,
    rememberCards,
    t,
    ticketAssignmentNotifications,
  ])

  useEffect(() => {
    if (!hasAssignmentBacklog) {
      backlogPromptedRef.current = false
      setShowBacklogPrompt(false)
    }
  }, [hasAssignmentBacklog])

  useEffect(() => {
    if (unreadTicketAssignmentIds === null) return
    const unreadIds = new Set(unreadTicketAssignmentIds)
    setItems((current) =>
      current.filter(
        (item) =>
          !item.notification ||
          unreadIds.has(item.notification.id)
      )
    )
  }, [unreadTicketAssignmentIds])

  const poll = useCallback(async () => {
    if (pollingRef.current || document.visibilityState === "hidden") return
    const session = readSession()
    if (!session?.accessToken || session.domainType !== "enterprise") return
    const requestScope = `${session.domainType}:${session.tenantId || 0}:${session.user?.id || 0}`
    const requestId = pollRequestIdRef.current + 1
    pollRequestIdRef.current = requestId
    pollingRef.current = true
    const checkedAtFallback = new Date().toISOString()
    try {
      const response = await fetchEnterpriseReminders(sinceRef.current)
      if (
        pollRequestIdRef.current !== requestId ||
        pollerScopeRef.current !== requestScope
      ) {
        return
      }
      if (!response.success || !response.data) return
      const checkedAt = response.data.checked_at || checkedAtFallback
      const storeKey = seenStorageKey(session.user?.id || 0, session.tenantId || 0)
      const now = Date.now()
      const seen = readSeenReminders(storeKey, now)
      const cards = [
        ...(response.data.meeting_reminders || []).map(meetingReminderToCard),
        ...(response.data.ticket_reminders || []).map(ticketReminderToCard),
      ].filter((card) => !seen[card.key])
      sinceRef.current = checkedAt
      if (cards.length === 0) {
        writeSeenReminders(storeKey, seen)
        return
      }
      rememberCards(cards, storeKey, now)
      announceCards(cards)
      setItems((current) => mergeReminderCards(current, cards))
      setOpen(true)
    } finally {
      if (pollRequestIdRef.current === requestId) {
        pollingRef.current = false
      }
    }
  }, [announceCards, rememberCards])

  useEffect(() => {
    void poll()
    const interval = window.setInterval(() => {
      void poll()
    }, REMINDER_POLL_INTERVAL_MS)
    const onVisibilityChange = () => {
      const visible = document.visibilityState === "visible"
      setPageVisible(visible)
      if (visible) {
        void poll()
      }
    }
    const onFocus = () => {
      setPageVisible(true)
      void poll()
    }
    window.addEventListener("focus", onFocus)
    document.addEventListener("visibilitychange", onVisibilityChange)
    return () => {
      window.clearInterval(interval)
      window.removeEventListener("focus", onFocus)
      document.removeEventListener("visibilitychange", onVisibilityChange)
    }
  }, [poll])

  function closePanel() {
    setOpen(false)
    setItems([])
    setShowBacklogPrompt(false)
  }

  function dismissReminder(key: string) {
    setItems((current) => current.filter((item) => item.key !== key))
  }

  async function acknowledgeReminder(item: ReminderCard) {
    if (!item.notification) {
      announceDismissedCards([item])
      dismissReminder(item.key)
      return
    }
    const reminderScope = pollerScopeRef.current
    const channelName = reminderChannelNameRef.current
    const session = readSession()
    const storeKey = session?.accessToken && session.domainType === "enterprise"
      ? seenStorageKey(session.user?.id || 0, session.tenantId || 0)
      : ""
    setAcknowledgingKeys((current) => new Set(current).add(item.key))
    try {
      const acknowledged = await acknowledgeNotification(item.notification)
      if (acknowledged) {
        announceDismissedCards([item], channelName)
        if (pollerScopeRef.current === reminderScope) {
          dismissReminder(item.key)
        }
      } else {
        if (storeKey) {
          forgetSeenReminder(storeKey, item.key, Date.now())
        }
        if (pollerScopeRef.current === reminderScope) {
          toast.error(rp(t, "acknowledgeFailed"))
        }
      }
    } finally {
      if (pollerScopeRef.current === reminderScope) {
        setAcknowledgingKeys((current) => {
          const next = new Set(current)
          next.delete(item.key)
          return next
        })
      }
    }
  }

  function openReminder(item: ReminderCard) {
    const reminderScope = pollerScopeRef.current
    const channelName = reminderChannelNameRef.current
    const session = readSession()
    const storeKey = session?.accessToken && session.domainType === "enterprise"
      ? seenStorageKey(session.user?.id || 0, session.tenantId || 0)
      : ""
    dismissReminder(item.key)
    if (item.notification) {
      const acknowledgement = acknowledgeNotification(item.notification)
      router.push(item.actionUrl)
      void acknowledgement.then((acknowledged) => {
        if (acknowledged) {
          announceDismissedCards([item], channelName)
          return
        }
        if (storeKey) {
          forgetSeenReminder(storeKey, item.key, Date.now())
        }
        if (pollerScopeRef.current !== reminderScope) return
        setItems((current) => mergeReminderCards(current, [item]))
        setOpen(true)
        toast.error(rp(t, "openReadFailed"))
      })
      return
    }
    announceDismissedCards([item])
    router.push(item.actionUrl)
  }

  function openNotificationCenter() {
    setOpen(false)
    setShowBacklogPrompt(false)
    router.push("/enterprise/notifications?read_status=unread&category=ticket")
  }

  const nonAssignmentReminderCount = items.filter((item) => !item.notification).length
  const attentionCount = showBacklogPrompt
    ? unreadTicketAssignmentTotal + nonAssignmentReminderCount
    : items.length

  if (!open || (items.length === 0 && !showBacklogPrompt)) {
    return null
  }

  return (
    <aside
      aria-live="polite"
      aria-atomic="false"
      aria-label={rp(t, "reminderPanelAria")}
      className="fixed bottom-[max(1rem,env(safe-area-inset-bottom))] left-[max(1rem,env(safe-area-inset-left))] right-[max(1rem,env(safe-area-inset-right))] z-50 overflow-hidden rounded-lg border border-border bg-card text-card-foreground shadow-2xl sm:left-auto sm:w-[min(420px,calc(100vw-2rem))]"
    >
      <div className="flex min-h-12 items-center justify-between gap-3 border-b border-border bg-muted px-3.5 py-2.5">
        <div className="flex min-w-0 items-center gap-2">
          <span className="grid size-8 shrink-0 place-items-center rounded-md bg-primary text-primary-foreground">
            <BellRingIcon aria-hidden="true" className="size-4" />
          </span>
          <div className="min-w-0">
            <div className="truncate text-sm font-semibold text-foreground">{rp(t, "serviceReminderPanel")}</div>
            <div className="text-xs text-muted-foreground">
              {rp(
                t,
                attentionCount === 1
                  ? "oneItemNeedsAttention"
                  : "itemsNeedAttention",
                { count: attentionCount }
              )}
            </div>
          </div>
        </div>
        <Button variant="ghost" size="icon-lg" className="shrink-0" onClick={closePanel} aria-label={rp(t, "closeReminder")}>
          <XIcon aria-hidden="true" className="size-4" />
        </Button>
      </div>
      <div className="max-h-[min(520px,calc(100dvh_-_9rem_-_env(safe-area-inset-bottom)))] divide-y divide-border overflow-y-auto">
        {items.map((item) => {
          const Icon = item.icon
          const itemAccessibleLabel = [
            item.eyebrow,
            item.title,
            item.description,
            item.meta,
          ].filter(Boolean).join(". ")
          const acknowledging = acknowledgingKeys.has(item.key)
          return (
            <div key={item.key} className="flex min-w-0 items-start transition hover:bg-muted/40">
              <button
                type="button"
                aria-label={itemAccessibleLabel}
                className="grid min-w-0 flex-1 grid-cols-[auto_minmax(0,1fr)_auto] gap-3 px-3.5 py-3 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
                onClick={() => void openReminder(item)}
              >
                <span className={cn("mt-0.5 grid size-9 place-items-center rounded-md border", item.tone)}>
                  <Icon aria-hidden="true" className="size-4" />
                </span>
                <span className="min-w-0">
                  <span className="block text-xs font-semibold text-muted-foreground">{item.eyebrow}</span>
                  <span className="mt-0.5 line-clamp-2 block text-sm font-semibold leading-5 text-foreground">{item.title}</span>
                  <span className="mt-1 line-clamp-2 whitespace-pre-line text-xs text-muted-foreground">{item.description}</span>
                  <span className="mt-0.5 block truncate text-xs text-muted-foreground">{item.meta}</span>
                </span>
                <ArrowRightIcon aria-hidden="true" className="mt-2 size-4 shrink-0 text-muted-foreground/60" />
              </button>
              <Button
                variant="ghost"
                size="icon-lg"
                className="mr-1.5 mt-1.5 shrink-0"
                aria-label={rp(t, "acknowledgeItem", { title: itemAccessibleLabel })}
                aria-busy={acknowledging}
                title={rp(t, "acknowledge")}
                disabled={acknowledging}
                onClick={() => void acknowledgeReminder(item)}
              >
                {acknowledging ? (
                  <Loader2Icon aria-hidden="true" className="size-4 animate-spin" />
                ) : (
                  <CheckIcon aria-hidden="true" className="size-4" />
                )}
              </Button>
            </div>
          )
        })}
      </div>
      {showBacklogPrompt ? (
        <Button
          variant="ghost"
          className="h-auto min-h-10 w-full justify-between rounded-none border-t border-border px-3.5 py-2 text-left"
          onClick={openNotificationCenter}
        >
          <span className="min-w-0 whitespace-normal leading-4">
            {rp(t, "moreUnreadAssignments")}
          </span>
          <ArrowRightIcon aria-hidden="true" className="size-4 shrink-0" />
        </Button>
      ) : null}
    </aside>
  )
}
