"use client"

import { BellIcon, CheckCheckIcon, ListIcon, RefreshCwIcon } from "lucide-react"
import { useRouter } from "next/navigation"
import { useCallback, useEffect, useState } from "react"
import { toast } from "sonner"

import { useNotifications } from "@/components/notification-provider"
import { relativeNotificationTime } from "@/components/remote-helpdesk/enterprise-live-pages"
import { Button } from "@/components/ui/button"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { Skeleton } from "@/components/ui/skeleton"
import {
  fetchEnterpriseNotifications,
  markAllEnterpriseNotificationsRead,
  markEnterpriseNotificationRead,
} from "@/lib/api/enterprise-notifications"
import {
  fetchNotifications,
  markAllNotificationsRead,
  type NotificationItem,
} from "@/lib/api/notification"
import { fetchPlatformOverviewEvents, type PlatformRecentEvent } from "@/lib/api/platform"
import type { EnterpriseNotification } from "@/lib/api/types"
import {
  getAuditActionLabel,
  getAuditStatusLabel,
  getAuditTargetLabel,
  isAuditAlertEvent,
} from "@/lib/audit-i18n"
import type { RealtimeConnectionStatus } from "@/lib/realtime-connection"
import { normalizeEnterpriseActionUrl } from "@/lib/ticket-workbench-route"
import { cn } from "@/lib/utils"
import { useI18n } from "@/i18n/provider"

export function NotificationBell({ domain }: { domain: string }) {
  const t = useI18n()
  if (domain === "enterprise") {
    return <EnterpriseNotificationBell />
  }
  if (domain === "platform") {
    return <PlatformNotificationBell />
  }
  return (
    <button
      type="button"
      className="grid size-8 place-items-center rounded-md border border-[var(--railops-border)] bg-[var(--railops-surface)] text-[var(--railops-text-secondary)] shadow-[var(--railops-card-shadow)] transition hover:border-[#bfdbfe] hover:bg-[var(--railops-primary-bg)] hover:text-[var(--railops-primary-hover)]"
      aria-label={t("railopsExtract.notification.title")}
      title={t("railopsExtract.notification.title")}
    >
      <BellIcon className="size-3.5" />
    </button>
  )
}

function NotificationLoading() {
  const t = useI18n()
  return (
    <div className="divide-y divide-border/70" role="status" aria-label={t("railopsExtract.notification.loading")} aria-busy="true">
      {Array.from({ length: 4 }, (_, index) => (
        <div key={index} className="flex min-h-16 items-start gap-3 px-4 py-3">
          <Skeleton className="mt-1 size-2 shrink-0 rounded-full" />
          <div className="min-w-0 flex-1 space-y-2">
            <Skeleton className="h-3.5 w-2/3" />
            <Skeleton className="h-3 w-full" />
          </div>
          <Skeleton className="h-3 w-12" />
        </div>
      ))}
    </div>
  )
}

function realtimeDotClass(status: RealtimeConnectionStatus) {
  if (status === "connected") return "bg-[var(--railops-success)]"
  if (status === "connecting") return "bg-[var(--railops-warning)]"
  return "bg-[var(--railops-text-tertiary)]"
}

function NotificationTrigger({
  realtimeStatus,
  unread,
}: {
  realtimeStatus: RealtimeConnectionStatus
  unread: number
}) {
  const t = useI18n()
  return (
    <PopoverTrigger
      className="relative grid size-8 place-items-center rounded-md border border-[var(--railops-border)] bg-[var(--railops-surface)] text-[var(--railops-text-secondary)] shadow-[var(--railops-card-shadow)] transition hover:border-[#bfdbfe] hover:bg-[var(--railops-primary-bg)] hover:text-[var(--railops-primary-hover)]"
      aria-label={t("remoteShell.notifications")}
      title={`${t("remoteShell.notifications")} · ${t(`realtime.${realtimeStatus}`)}`}
      data-realtime-status={realtimeStatus}
    >
      <BellIcon className="size-3.5" />
      <span
        className={cn(
          "absolute bottom-1 right-1 size-1.5 rounded-full border border-white",
          realtimeDotClass(realtimeStatus),
          realtimeStatus === "connecting" && "animate-pulse"
        )}
      />
          {unread > 0 ? (
        <span className="absolute -right-1 -top-1 grid h-4 min-w-4 place-items-center rounded-full bg-[var(--railops-error)] px-1 text-rhd-2xs font-semibold leading-none text-white">
          {unread > 99 ? "99+" : unread}
        </span>
      ) : null}
    </PopoverTrigger>
  )
}

function levelDotClass(level?: string) {
  if (level === "urgent") return "bg-destructive"
  if (level === "warning") return "bg-amber-500"
  return "bg-primary"
}

function NotificationPopoverHeader({
  onMarkAll,
  markAllDisabled = false,
  onRefresh,
  realtimeStatus,
  title,
}: {
  onMarkAll: () => void
  markAllDisabled?: boolean
  onRefresh: () => void
  realtimeStatus: RealtimeConnectionStatus
  title?: string
}) {
  const t = useI18n()
  const resolvedTitle = title ?? t("railopsExtract.notification.messageCenter")
  return (
    <div className="flex items-center justify-between border-b border-[var(--railops-border-light)] px-4 py-3">
      <div className="min-w-0">
        <span className="block text-sm font-semibold text-[var(--railops-text)]">{resolvedTitle}</span>
        <span className="mt-0.5 flex items-center gap-1.5 text-rhd-xs text-[var(--railops-text-secondary)]">
          <span className={cn("size-1.5 rounded-full", realtimeDotClass(realtimeStatus))} />
          {t(`realtime.${realtimeStatus}`)}
        </span>
      </div>
      <div className="flex gap-1">
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={t("railopsExtract.notification.refresh")}
          title={t("railopsExtract.notification.refresh")}
          onClick={onRefresh}
        >
          <RefreshCwIcon />
        </Button>
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={t("railopsExtract.notification.markAllRead")}
          title={t("railopsExtract.notification.markAllRead")}
          onClick={onMarkAll}
          disabled={markAllDisabled}
        >
          <CheckCheckIcon />
        </Button>
      </div>
    </div>
  )
}

function PlatformNotificationBell() {
  const t = useI18n()
  const router = useRouter()
  const {
    notifyNotificationStateChange,
    markReadAndNavigate,
    notificationRevision,
    realtimeStatus,
    refreshUnreadCount,
    unreadCount,
  } = useNotifications()
  const [open, setOpen] = useState(false)
  const [items, setItems] = useState<NotificationItem[]>([])
  const [alerts, setAlerts] = useState<PlatformRecentEvent[]>([])
  const [loading, setLoading] = useState(false)
  const [notificationsLoaded, setNotificationsLoaded] = useState(false)

  const refresh = useCallback(async () => {
    setLoading(true)
    try {
      const [notificationResult, eventsResult] = await Promise.allSettled([
        fetchNotifications({ limit: 8 }),
        fetchPlatformOverviewEvents(8),
      ])
      if (notificationResult.status === "fulfilled") {
        setItems(notificationResult.value.results ?? [])
      }
      if (eventsResult.status === "fulfilled") {
        setAlerts(eventsResult.value.recentEvents.filter(isAuditAlertEvent).slice(0, 5))
      }
      setNotificationsLoaded(true)
      await refreshUnreadCount().catch(() => undefined)
    } finally {
      setLoading(false)
    }
  }, [refreshUnreadCount])

  useEffect(() => {
    void refresh().catch(() => undefined)
  }, [notificationRevision, refresh])

  const handleMarkAll = useCallback(async () => {
    try {
      await markAllNotificationsRead()
      notifyNotificationStateChange()
      await refresh()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("railopsExtract.notification.allReadFailed"))
    }
  }, [notifyNotificationStateChange, refresh])

  const handleClickItem = useCallback(async (item: NotificationItem) => {
    setOpen(false)
    try {
      await markReadAndNavigate(item)
      await refresh()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("railopsExtract.notification.openFailed"))
    }
  }, [markReadAndNavigate, refresh])

  const openPlatformAudit = useCallback(() => {
    setOpen(false)
    router.push("/platform/audit")
  }, [router])

  const urgentAlertCount = alerts.filter((item) => {
    const risk = item.riskLevel.toLowerCase()
    return risk === "critical" || risk === "high" || risk === "danger"
  }).length

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <NotificationTrigger realtimeStatus={realtimeStatus} unread={unreadCount + urgentAlertCount} />
      <PopoverContent align="end" className="w-[min(380px,calc(100vw-24px))] p-0">
        <NotificationPopoverHeader
          onMarkAll={() => void handleMarkAll()}
          onRefresh={() => void refresh().catch(() => undefined)}
          realtimeStatus={realtimeStatus}
          title={t("railopsExtract.notification.withAlerts")}
        />
        <div className="max-h-[360px] overflow-y-auto">
          {loading && !notificationsLoaded ? (
            <NotificationLoading />
          ) : items.length === 0 && alerts.length === 0 ? (
            <div className="px-4 py-8 text-center text-xs text-muted-foreground">{t("railopsExtract.notification.empty")}</div>
          ) : (
            <div>
              {alerts.length > 0 ? (
                <section aria-label={t("railopsExtract.notification.platformAlerts")}>
                  <div className="border-b border-border bg-muted/40 px-4 py-2 text-rhd-xs font-semibold text-muted-foreground">
                    {t("railopsExtract.notification.platformAlerts")}
                  </div>
                  <div className="divide-y divide-border/70">
                    {alerts.map((item) => (
                      <button
                        key={item.id}
                        type="button"
                        className="flex w-full items-start gap-2.5 px-4 py-3 text-left transition hover:bg-muted/60"
                        onClick={openPlatformAudit}
                      >
                        <span className="mt-1.5 size-2 shrink-0 rounded-full bg-destructive" />
                        <span className="min-w-0 flex-1">
                          <span className="block truncate text-sm font-semibold text-foreground">
                            {getAuditActionLabel(item.action)}
                          </span>
                          <span className="mt-0.5 block truncate text-xs text-muted-foreground">
                            {item.tenantName || t("railopsExtract.notification.global")} · {getAuditTargetLabel(item.targetType)} · {getAuditStatusLabel(item.status)}
                          </span>
                          <span className="mt-0.5 block text-rhd-xs text-muted-foreground">{relativeNotificationTime(item.occurredAt)}</span>
                        </span>
                      </button>
                    ))}
                  </div>
                </section>
              ) : null}
              {items.length > 0 ? (
                <section aria-label={t("railopsExtract.notification.accountSection")}>
                  <div className="border-y border-border bg-muted/40 px-4 py-2 text-rhd-xs font-semibold text-muted-foreground">
                    {t("railopsExtract.notification.accountSection")}
                  </div>
                  <div className="divide-y divide-border/70">
                    {items.map((item) => {
                      const isUnread = !item.readAt
                      return (
                        <button
                          key={item.id}
                          type="button"
                          className={cn(
                            "flex w-full items-start gap-2.5 px-4 py-3 text-left transition hover:bg-muted/60",
                            isUnread && "bg-primary/5"
                          )}
                          onClick={() => void handleClickItem(item)}
                        >
                          <span className="mt-1.5 size-2 shrink-0 rounded-full bg-primary" />
                          <span className="min-w-0 flex-1">
                            <span className={cn("block truncate text-sm", isUnread ? "font-semibold text-foreground" : "font-medium text-muted-foreground")}>
                              {item.title || item.notificationType || t("railopsExtract.notification.notification")}
                            </span>
                            <span className="mt-0.5 block truncate text-xs text-muted-foreground">{item.content}</span>
                            <span className="mt-0.5 block text-rhd-xs text-muted-foreground">{relativeNotificationTime(item.createdAt)}</span>
                          </span>
                        </button>
                      )
                    })}
                  </div>
                </section>
              ) : null}
            </div>
          )}
        </div>
        <div className="border-t border-border p-2">
          <Button variant="ghost" size="sm" className="w-full justify-center text-xs" onClick={openPlatformAudit}>
            <ListIcon />
            {t("railopsExtract.notification.viewAudit")}
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  )
}

function EnterpriseNotificationBell() {
  const t = useI18n()
  const router = useRouter()
  const {
    notifyNotificationStateChange,
    notificationRevision,
    realtimeStatus,
    refreshUnreadCount,
  } = useNotifications()
  const [open, setOpen] = useState(false)
  const [items, setItems] = useState<EnterpriseNotification[]>([])
  const [unread, setUnread] = useState(0)
  const [markableUnread, setMarkableUnread] = useState(0)
  const [loading, setLoading] = useState(false)
  const [notificationsLoaded, setNotificationsLoaded] = useState(false)

  const refresh = useCallback(async () => {
    setLoading(true)
    try {
    const res = await fetchEnterpriseNotifications({ scope: "personal", page: 1, page_size: 8 })
      if (res.success && res.data) {
        setItems(res.data.items ?? [])
        setUnread(res.data.summary?.unread ?? 0)
        setMarkableUnread(res.data.summary?.markable_unread ?? 0)
        setNotificationsLoaded(true)
      }
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void refresh().catch(() => undefined)
  }, [notificationRevision, refresh])

  const handleOpenChange = useCallback((next: boolean) => {
    setOpen(next)
    if (next) void refresh().catch(() => undefined)
  }, [refresh])

  const handleMarkAll = useCallback(async () => {
    if (markableUnread <= 0) return
    const res = await markAllEnterpriseNotificationsRead()
    if (res.success) {
      notifyNotificationStateChange()
      await Promise.all([
        refresh(),
        refreshUnreadCount().catch(() => undefined),
      ])
    }
  }, [markableUnread, notifyNotificationStateChange, refresh, refreshUnreadCount])

  const handleClickItem = useCallback(async (item: EnterpriseNotification) => {
    setOpen(false)
    if (!item.read_at && item.can_mark_read) {
      const res = await markEnterpriseNotificationRead(item.id).catch(() => null)
      if (res?.success) {
        notifyNotificationStateChange()
        await refreshUnreadCount().catch(() => undefined)
      }
    }
    const actionUrl = normalizeEnterpriseActionUrl(item)
    if (actionUrl) {
      router.push(actionUrl)
    } else {
      await refresh().catch(() => undefined)
    }
  }, [notifyNotificationStateChange, refresh, refreshUnreadCount, router])

  const goToCenter = useCallback(() => {
    setOpen(false)
    router.push("/enterprise/notifications")
  }, [router])

  return (
    <Popover open={open} onOpenChange={handleOpenChange}>
      <NotificationTrigger realtimeStatus={realtimeStatus} unread={unread} />
      <PopoverContent align="end" className="w-[min(380px,calc(100vw-24px))] p-0">
        <NotificationPopoverHeader
          onMarkAll={() => void handleMarkAll()}
          markAllDisabled={markableUnread <= 0}
          onRefresh={() => void refresh().catch(() => undefined)}
          realtimeStatus={realtimeStatus}
        />
        <div className="max-h-[360px] overflow-y-auto">
          {loading && !notificationsLoaded ? (
            <NotificationLoading />
          ) : items.length === 0 ? (
            <div className="px-4 py-8 text-center text-xs text-muted-foreground">{t("railopsExtract.notification.empty")}</div>
          ) : (
            <div className="divide-y divide-border/70">
              {items.map((item) => {
                const isUnread = !item.read_at
                return (
                  <button
                    key={item.id}
                    type="button"
                    className={cn(
                      "flex w-full items-start gap-2.5 px-4 py-3 text-left transition hover:bg-muted/60",
                      isUnread && "bg-primary/5"
                    )}
                    onClick={() => void handleClickItem(item)}
                  >
                    <span className={cn("mt-1.5 size-2 shrink-0 rounded-full", levelDotClass(item.level))} />
                    <span className="min-w-0 flex-1">
                      <span className={cn("block truncate text-sm", isUnread ? "font-semibold text-foreground" : "font-medium text-muted-foreground")}>
                        {item.title || item.notification_type}
                      </span>
                      <span className="mt-0.5 block truncate text-xs text-muted-foreground">{item.content}</span>
                      <span className="mt-0.5 block truncate text-rhd-xs text-muted-foreground">
                        {[item.recipient_name, relativeNotificationTime(item.created_at)].filter(Boolean).join(" · ")}
                      </span>
                    </span>
                  </button>
                )
              })}
            </div>
          )}
        </div>
        <div className="border-t border-border p-2">
          <Button variant="ghost" size="sm" className="w-full justify-center text-xs" onClick={goToCenter}>
            <ListIcon />
            {t("railopsExtract.notification.viewAll")}
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  )
}
