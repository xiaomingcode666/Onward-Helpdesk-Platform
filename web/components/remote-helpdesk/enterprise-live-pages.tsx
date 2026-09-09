"use client"

import { translateCurrentMessage } from "@/i18n/messages"
import {
  CheckCheckIcon,
  CheckIcon,
  ExternalLinkIcon,
  MailIcon,
  RefreshCwIcon,
  SendIcon,
  Settings2Icon,
  ShieldCheckIcon,
  UserRoundIcon,
  UsersRoundIcon,
} from "lucide-react"
import { Drawer as AntDrawer, Input, Skeleton } from "antd"
import type { TableColumnsType } from "antd"
import {
  ActiveFilterTags,
  DataTable,
  DEFAULT_PAGE_SIZE,
  FilterTabs,
  IconButton,
  PageShell,
  RailopsButton,
  SearchField,
  StatusTag,
  TableFilters,
  TableToolbar,
} from "@railops/ui"
import type { StatusTagTone, TableFilterField, TableFilterValues } from "@railops/ui"
import Link from "next/link"
import { useRouter, useSearchParams } from "next/navigation"
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react"
import { toast } from "sonner"

import { useNotifications } from "@/components/notification-provider"
import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import { EmptyState, ErrorState } from "@/components/shared/error-states"
import {
  Popover,
  PopoverContent,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
} from "@/components/ui/popover"
import { Switch } from "@/components/ui/switch"
import {
  fetchEnterpriseNotifications,
  fetchNotificationRecipientSetting,
  markAllEnterpriseNotificationsRead,
  markEnterpriseNotificationRead,
  sendNotificationRecipientTestMail,
  updateNotificationRecipientSetting,
} from "@/lib/api/enterprise-notifications"
import { readSession } from "@/lib/auth"
import type {
  EnterpriseNotification,
  EnterpriseNotificationListResponse,
  NotificationRecipientSetting,
} from "@/lib/api/types"
import { buildEnterpriseTicketWorkbenchPath, normalizeEnterpriseActionUrl } from "@/lib/ticket-workbench-route"
import { cn } from "@/lib/utils"
function ee(key: string, values?: Record<string, unknown>) {
  if (!values) {
    return translateCurrentMessage(`enterpriseExtract.${key}`)
  }
  return translateCurrentMessage(
    `enterpriseExtract.${key}`,
    Object.fromEntries(
      Object.entries(values).map(([name, value]) => [
        name,
        typeof value === "string" || typeof value === "number" ? value : String(value ?? ""),
      ]),
    ),
  )
}


const livePanelClass = "overflow-hidden rounded-md bg-[var(--railops-surface)] shadow-[var(--railops-card-shadow)]"
const livePanelHeaderClass = "border-b border-[var(--railops-border-light)] p-3"
const liveMutedTextClass = "text-[var(--railops-text-secondary)]"
const liveTitleClass = "text-[var(--railops-text)]"
const liveSelectedClass = "border-[#bfdbfe] bg-[var(--railops-primary-bg)] text-[var(--railops-primary-hover)]"
const liveIdleClass = "border-[var(--railops-border)] bg-[var(--railops-surface)] text-[var(--railops-text-secondary)]"
const livePrimaryClass = "border-[var(--railops-primary)] bg-[var(--railops-primary)] text-white"

function toneClass(tone?: string) {
  switch (tone) {
    case "success":
      return "border-[#bbf7d0] bg-[var(--railops-success-bg)] text-[#047857]"
    case "amber":
      return "border-[#fde68a] bg-[var(--railops-warning-bg)] text-[#b45309]"
    case "red":
      return "border-[#fecaca] bg-[var(--railops-error-bg)] text-[var(--railops-error)]"
    case "blue":
      return "border-[#bfdbfe] bg-[var(--railops-primary-bg)] text-[var(--railops-primary-hover)]"
    default:
      return "border-[var(--railops-border-light)] bg-[var(--railops-surface-muted)] text-[var(--railops-text-secondary)]"
  }
}

function dateText(value?: string) {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

function NotificationRows({
  items,
  onRead,
}: {
  items: EnterpriseNotification[]
	onRead?: (item: EnterpriseNotification) => void
}) {
	if (items.length === 0) {
		return <EmptyState title={ee("enterpriseLive.text024")} className="rounded-none border-0" />
	}
  return (
    <div className="remote-helpdesk-scrollbar max-h-[480px] divide-y divide-border overflow-y-auto">
      {items.map((item) => {
        const actionUrl = normalizeEnterpriseActionUrl(item)
        return (
        <article key={item.id} className="flex items-center gap-3 px-4 py-3 hover:bg-muted/50">
          <Popover>
            <PopoverTrigger
              render={(
                <button
                  type="button"
                  className="min-w-0 flex-1 rounded-md text-left outline-none transition hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring/20"
                />
              )}
            >
              <p className="truncate px-2 py-1.5 text-sm text-foreground">
                <span className="font-semibold text-foreground">{item.title || item.notification_type}</span>
                {item.content?.trim() ? (
                  <span className="text-muted-foreground">
                    {" · "}
                    {item.content.replace(/\s+/g, " ").trim()}
                  </span>
                ) : null}
              </p>
            </PopoverTrigger>
            <PopoverContent side="bottom" align="start" sideOffset={8} className="w-80 gap-3 p-3">
              <PopoverHeader className="gap-1">
                <div className="flex items-center gap-2">
                  <PopoverTitle className="text-foreground">{item.title || item.notification_type}</PopoverTitle>
                  <span className={cn("rounded border px-2 py-0.5 text-rhd-xs", toneClass(item.read_at ? "slate" : "blue"))}>
                    {item.read_at ? ee("enterpriseLive.text025") : ee("enterpriseLive.text026")}
                  </span>
                </div>
                <div className="text-xs text-muted-foreground">
                  {relativeNotificationTime(item.created_at)} · {dateText(item.created_at)}
                </div>
              </PopoverHeader>
              <div className="space-y-2 text-sm text-muted-foreground">
                {item.content?.trim() ? <p className="leading-6 text-foreground">{item.content.trim()}</p> : null}
                <div className="flex flex-wrap gap-2 text-xs text-muted-foreground">
                  <span className="rounded border border-border bg-muted px-2 py-1">
                    {notificationCategoryLabel(item.category, notificationTypeLabel(item.notification_type))}
                  </span>
                  {item.biz_type ? (
                    <span className="rounded border border-border bg-muted px-2 py-1">{notificationTypeLabel(item.biz_type)}</span>
                  ) : null}
                  {item.delivery_status && item.delivery_status !== "sent" ? (
                    <span className={cn("rounded border px-2 py-1", toneClass(item.delivery_status === "failed" ? "red" : "amber"))}>{deliveryStatusLabel(item.delivery_status)}</span>
                  ) : null}
                </div>
              </div>
            </PopoverContent>
          </Popover>
          <div className="flex shrink-0 items-center gap-2 text-xs text-muted-foreground">
            <span className={cn("rounded border px-2 py-0.5", toneClass(item.read_at ? "slate" : "blue"))}>
              {item.read_at ? ee("enterpriseLive.text025") : ee("enterpriseLive.text026")}
            </span>
            {item.delivery_status && item.delivery_status !== "sent" ? (
              <span className={cn("rounded border px-2 py-0.5", toneClass(item.delivery_status === "failed" ? "red" : "amber"))}>{deliveryStatusLabel(item.delivery_status)}</span>
            ) : null}
            <span className="hidden text-muted-foreground/70 md:inline">{dateText(item.created_at)}</span>
            {actionUrl ? (
              <Link href={actionUrl}>
                <RailopsButton size="small" variant="text">
                  <ExternalLinkIcon className="size-3.5" />
                </RailopsButton>
              </Link>
            ) : null}
            {!item.read_at && onRead ? (
              <RailopsButton size="small" variant="text" onClick={() => onRead(item)}>
                <CheckCheckIcon className="size-3.5" />
              </RailopsButton>
            ) : null}
          </div>
        </article>
        )
      })}
    </div>
  )
}

const NOTIFICATION_CATEGORIES = [
  { value: "all", label: ee("enterpriseLive.text034") },
  { value: "ticket", label: ee("enterpriseLive.text021") },
  { value: "conversation", label: ee("enterpriseLive.text150") },
  { value: "sla", label: "SLA" },
  { value: "approval", label: ee("enterpriseLive.text038") },
  { value: "quota", label: ee("enterpriseLive.text039") },
  { value: "knowledge", label: ee("enterpriseLive.text040") },
  { value: "video", label: ee("enterpriseLive.text041") },
  { value: "system", label: ee("enterpriseLive.text042") },
] as const

const NOTIFICATION_CATEGORY_LABELS: Record<string, string> = Object.fromEntries(
  NOTIFICATION_CATEGORIES.map((item) => [item.value, item.label])
)

const NOTIFICATION_TYPE_LABELS: Record<string, string> = {
  conversation: ee("enterpriseLive.text150"),
  conversation_assigned: ee("enterpriseLive.text164"),
  conversation_message: ee("enterpriseLive.text163"),
  customer: ee("enterpriseLive.text151"),
  data_breach_overdue: ee("enterpriseLive.text175"),
  data_breach_urgent: ee("enterpriseLive.text175"),
  device_alarm_raised: ee("enterpriseLive.text169"),
  dsar_expiring: ee("enterpriseLive.text176"),
  dsar_overdue: ee("enterpriseLive.text176"),
  email: ee("enterpriseLive.text052"),
  knowledge: ee("enterpriseLive.text040"),
  knowledge_candidate_created: ee("enterpriseLive.text171"),
  meeting: ee("enterpriseLive.text041"),
  "meeting.invite": ee("enterpriseLive.text177"),
  meeting_ended: ee("enterpriseLive.text170"),
  product_created: ee("enterpriseLive.text168"),
  push: ee("enterpriseLive.text152"),
  quota_exceeded: ee("enterpriseLive.text172"),
  security_alert: ee("enterpriseLive.text174"),
  sla: "SLA",
  sla_warning: ee("enterpriseLive.text173"),
  system: ee("enterpriseLive.text042"),
  ticket: ee("enterpriseLive.text021"),
  ticket_assigned: ee("enterpriseLive.text166"),
  ticket_closed: ee("enterpriseLive.text167"),
  ticket_created: ee("enterpriseLive.text165"),
  video: ee("enterpriseLive.text041"),
}

const EXTERNAL_CHANNEL_STATUS_LABELS: Record<string, string> = {
  email_failed: ee("enterpriseLive.text048"),
  email_pending: ee("enterpriseLive.text047"),
  email_retrying: ee("enterpriseLive.text153"),
  email_sent: ee("enterpriseLive.text046"),
  email_skipped: ee("enterpriseLive.text049"),
  email_unavailable: ee("enterpriseLive.text154"),
  push_unavailable: ee("enterpriseLive.text155"),
}

const DELIVERY_STATUS_LABELS: Record<string, string> = {
  bounced: ee("enterpriseLive.text156"),
  delivered: ee("enterpriseLive.text157"),
  failed: ee("enterpriseLive.text027"),
  pending: ee("enterpriseLive.text158"),
  processing: ee("enterpriseLive.text159"),
  sent: ee("enterpriseLive.text160"),
  skipped: ee("enterpriseLive.text161"),
  waiting_retry: ee("enterpriseLive.text153"),
}

function notificationCategoryLabel(category?: string, fallback?: string) {
  const value = category?.trim()
  if (!value) return fallback || ee("enterpriseLive.text042")
  return NOTIFICATION_CATEGORY_LABELS[value] || NOTIFICATION_TYPE_LABELS[value] || fallback || value
}

function notificationTypeLabel(value?: string) {
  const key = value?.trim()
  if (!key) return "-"
  return NOTIFICATION_TYPE_LABELS[key] || key
}

function deliveryStatusLabel(value?: string) {
  const key = value?.trim()
  if (!key) return "-"
  return DELIVERY_STATUS_LABELS[key] || EXTERNAL_CHANNEL_STATUS_LABELS[key] || key
}

function externalChannelStatusLabel(value?: string) {
  const key = value?.trim()
  if (!key) return "-"
  return EXTERNAL_CHANNEL_STATUS_LABELS[key] || DELIVERY_STATUS_LABELS[key] || key
}

function notificationDeliveryDetailStatus(item: EnterpriseNotification, email: ReturnType<typeof emailStatusMeta>) {
  if (item.channels?.includes("email")) return email.label
  if (item.channels?.includes("push")) {
    return item.external_channel_status ? externalChannelStatusLabel(item.external_channel_status) : ee("enterpriseLive.text162")
  }
  if (item.external_channel_status) return externalChannelStatusLabel(item.external_channel_status)
  if (item.delivery_status) return deliveryStatusLabel(item.delivery_status)
  return ee("enterpriseLive.text162")
}

function notificationLevelMeta(level?: string) {
  switch (level) {
    case "urgent":
      return { label: ee("enterpriseLive.text043"), dot: "bg-destructive", badge: "border-destructive/20 bg-destructive/10 text-destructive" }
    case "warning":
      return { label: ee("enterpriseLive.text044"), dot: "bg-amber-500", badge: "border-amber-200 bg-amber-50 text-foreground" }
    default:
      return { label: ee("enterpriseLive.text045"), dot: "bg-primary", badge: "border-border bg-muted text-muted-foreground" }
  }
}

function emailStatusMeta(status?: string) {
  switch (status) {
    case "sent":
      return { label: ee("enterpriseLive.text046"), badge: "border-primary/20 bg-primary/10 text-primary" }
    case "pending":
      return { label: ee("enterpriseLive.text047"), badge: "border-amber-200 bg-amber-50 text-foreground" }
    case "failed":
      return { label: ee("enterpriseLive.text048"), badge: "border-destructive/20 bg-destructive/10 text-destructive" }
    case "skipped":
      return { label: ee("enterpriseLive.text049"), badge: "border-border bg-muted text-muted-foreground" }
    default:
      return { label: ee("enterpriseLive.text050"), badge: "border-border bg-muted text-muted-foreground" }
  }
}

function channelLabel(channel: string) {
  if (channel === "in_app") return ee("enterpriseLive.text051")
  if (channel === "email") return ee("enterpriseLive.text052")
  if (channel === "sms") return ee("enterpriseLive.text053")
  if (channel === "wxwork") return ee("enterpriseLive.text054")
  if (channel === "push") return ee("enterpriseLive.text152")
  return channel
}

export function relativeNotificationTime(value?: string) {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const diff = Date.now() - date.getTime()
  if (diff < 60000) return ee("enterpriseLive.text055")
  if (diff < 3600000) return ee("enterpriseLive.text056", { value0: Math.floor(diff / 60000) })
  if (diff < 86400000) return ee("enterpriseLive.text057", { value0: Math.floor(diff / 3600000) })
  if (diff < 172800000) return ee("enterpriseLive.text058")
  return ee("enterpriseLive.text059", { value0: Math.floor(diff / 86400000) })
}

export function EnterpriseNotificationsLivePage() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const {
    notifyNotificationStateChange,
    notificationRevision,
    refreshUnreadCount,
  } = useNotifications()
  const handledNotificationRevision = useRef(0)
  const session = readSession()
  const routeCategory = NOTIFICATION_CATEGORIES.some(
    (item) => item.value === searchParams.get("category")
  )
    ? searchParams.get("category") || "all"
    : "all"
  const routeReadStatus = searchParams.get("read_status")
  const initialReadStatus = routeReadStatus === "unread" || routeReadStatus === "read"
    ? routeReadStatus
    : "all"
  const [scope, setScope] = useState<"personal" | "tenant">("personal")
  const [category, setCategory] = useState(routeCategory)
  const [readStatus, setReadStatus] = useState<"all" | "unread" | "read">(initialReadStatus)
  const [page, setPage] = useState(1)
  const [searchInput, setSearchInput] = useState("")
  const [search, setSearch] = useState("")
  const [data, setData] = useState<EnterpriseNotificationListResponse | null>(null)
  const [notificationListLoaded, setNotificationListLoaded] = useState(false)
  const [selectedNotification, setSelectedNotification] = useState<EnterpriseNotification | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [recipientSetting, setRecipientSetting] = useState<NotificationRecipientSetting | null>(null)
  const [recipientEmail, setRecipientEmail] = useState("")
  const [useProfileEmail, setUseProfileEmail] = useState(true)
  const [emailEnabled, setEmailEnabled] = useState(true)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [markingAll, setMarkingAll] = useState(false)
  const canUpdateNotifications = session?.permissions.includes("notification.update") ?? false
  const canViewAudit = session?.permissions.includes("session.view") ?? false

  const load = useCallback(async () => {
    setLoading(true)
    setError("")
    const res = await fetchEnterpriseNotifications({
      category,
      read_status: readStatus,
      search,
      scope,
      page,
      page_size: DEFAULT_PAGE_SIZE,
    })
    if (res.success && res.data) {
      setData(res.data)
      setNotificationListLoaded(true)
      if (!res.data.can_view_tenant_scope && scope !== res.data.scope) {
        setScope(res.data.scope)
      }
    } else {
      setError(res.error?.message || ee("enterpriseLive.text060"))
    }
    setLoading(false)
  }, [category, page, readStatus, scope, search, setData, setError, setLoading, setScope])

  const loadRecipientSetting = useCallback(async () => {
    const res = await fetchNotificationRecipientSetting()
    if (res.success && res.data) {
      setRecipientSetting(res.data)
      setRecipientEmail(res.data.override_email || res.data.profile_email)
      setUseProfileEmail(res.data.use_profile_email)
      setEmailEnabled(res.data.email_enabled)
    }
  }, [])

  useEffect(() => {
    const timer = window.setTimeout(() => void load(), 0)
    return () => window.clearTimeout(timer)
  }, [load])
  useEffect(() => {
    if (notificationRevision <= handledNotificationRevision.current) return
    handledNotificationRevision.current = notificationRevision
    const timer = window.setTimeout(() => void load(), 0)
    return () => window.clearTimeout(timer)
  }, [load, notificationRevision])
  useEffect(() => {
    const timer = window.setTimeout(() => void loadRecipientSetting(), 0)
    return () => window.clearTimeout(timer)
  }, [loadRecipientSetting])
  useEffect(() => {
    setPage(1)
    setSelectedNotification(null)
    setCategory(routeCategory)
    setReadStatus(initialReadStatus)
  }, [initialReadStatus, routeCategory])
  useEffect(() => {
    const timer = setTimeout(() => {
      setPage(1)
      setSearch(searchInput.trim())
    }, 300)
    return () => clearTimeout(timer)
  }, [searchInput])

  async function markAllRead() {
    if (!canUpdateNotifications) return
    const unreadCount = data?.summary.markable_unread ?? 0
    setMarkingAll(true)
    const res = await markAllEnterpriseNotificationsRead()
    setMarkingAll(false)
    if (res.success) {
      notifyNotificationStateChange()
      toast.success(unreadCount > 0 ? ee("enterpriseLive.text061", { value0: unreadCount }) : ee("enterpriseLive.text062"))
      await Promise.all([
        load(),
        refreshUnreadCount().catch(() => undefined),
      ])
    } else {
      toast.error(res.error?.message || ee("enterpriseLive.text063"))
    }
  }

  async function handleAction(item: EnterpriseNotification) {
    if (!item.read_at && item.can_mark_read && canUpdateNotifications) {
      const readResult = await markEnterpriseNotificationRead(item.id)
      if (readResult.success) {
        notifyNotificationStateChange()
        await refreshUnreadCount().catch(() => undefined)
      } else {
        toast.error(readResult.error?.message || ee("enterpriseLive.text064"))
      }
    }
    const actionUrl = normalizeEnterpriseActionUrl(item)
    if (actionUrl) {
      router.push(actionUrl)
    } else {
      void load()
    }
  }

  async function handleMarkRead(item: EnterpriseNotification) {
    if (item.read_at || !item.can_mark_read || !canUpdateNotifications) return false
    const res = await markEnterpriseNotificationRead(item.id)
    if (!res.success) {
      toast.error(res.error?.message || ee("enterpriseLive.text064"))
      return false
    }
    notifyNotificationStateChange()
    await Promise.all([
      load(),
      refreshUnreadCount().catch(() => undefined),
    ])
    setSelectedNotification((current) => {
      if (!current || current.id !== item.id) return current
      return {
        ...current,
        read_at: new Date().toISOString(),
        unread_count: 0,
      }
    })
    return true
  }

  async function saveRecipientSetting() {
    if (!canUpdateNotifications) return
    if (!useProfileEmail && !recipientEmail.trim()) {
      toast.error(ee("enterpriseLive.text065"))
      return
    }
    setSaving(true)
    const res = await updateNotificationRecipientSetting({
      email: recipientEmail.trim(),
      useProfileEmail,
      emailEnabled,
    })
    setSaving(false)
    if (res.success && res.data) {
      setRecipientSetting(res.data)
      setRecipientEmail(res.data.override_email || res.data.profile_email)
      toast.success(ee("enterpriseLive.text066"))
    } else {
      toast.error(res.error?.message || ee("enterpriseLive.text067"))
    }
  }

  async function sendRecipientTest() {
    if (!canUpdateNotifications) return
    if (!recipientSetting?.effective_email || !recipientSetting.email_enabled) {
      toast.error(ee("enterpriseLive.text068"))
      return
    }
    setTesting(true)
    const res = await sendNotificationRecipientTestMail()
    setTesting(false)
    if (res.success) {
      toast.success(ee("enterpriseLive.text069", { value0: recipientSetting.effective_email }))
    } else {
      toast.error(res.error?.message || ee("enterpriseLive.text070"))
    }
  }

  const summary = data?.summary
  const visibleItems = data?.items ?? []
  const notificationListInitialLoading = loading && !notificationListLoaded
  const notificationListBlockingError = Boolean(error) && !notificationListLoaded

  const notificationFilterFields = useMemo<TableFilterField[]>(() => [
    {
      key: "read_status",
      label: ee("enterpriseLive.text071"),
      type: "select",
      options: [
        { label: ee("enterpriseLive.text072"), value: "all" },
        { label: data?.scope === "tenant" ? ee("enterpriseLive.text073") : ee("enterpriseLive.text074"), value: "unread" },
        { label: data?.scope === "tenant" ? ee("enterpriseLive.text075") : ee("enterpriseLive.text076"), value: "read" },
      ],
    },
    {
      key: "category",
      label: ee("enterpriseLive.text077"),
      type: "select",
      options: NOTIFICATION_CATEGORIES.filter((item) => item.value !== "all").map((item) => ({
        label: item.label,
        value: item.value,
      })),
    },
  ], [data?.scope])
  const notificationFilterValues: TableFilterValues = {
    read_status: readStatus === "all" ? undefined : readStatus,
    category: category === "all" ? undefined : category,
  }
  const applyNotificationFilters = (next: TableFilterValues) => {
    setPage(1)
    setSelectedNotification(null)
    setReadStatus((next.read_status as "all" | "unread" | "read") ?? "all")
    setCategory((next.category as string) ?? "all")
  }
  return (
    <PageShell
      title={ee("enterpriseLive.text078")}
      breadcrumb={useRouteBreadcrumbItems()}
      className="rhd-railops-notifications-page"
      actions={(
        <>
          <IconButton
            icon={<RefreshCwIcon className={cn("size-4", loading && "animate-spin")} />}
            tooltip={ee("enterpriseLive.text079")}
            aria-label={ee("enterpriseLive.text079")}
            onClick={() => void load()}
            disabled={loading}
          />
          <RailopsButton
            variant="primary"
            onClick={() => void markAllRead()}
            disabled={markingAll || !canUpdateNotifications || (summary?.markable_unread ?? 0) === 0}
            title={ee("enterpriseLive.text080")}
          >
            <CheckCheckIcon className="size-4" />
            {markingAll ? ee("enterpriseLive.text081") : ee("enterpriseLive.text082", { value0: summary?.markable_unread ? ` (${summary.markable_unread})` : "" })}
          </RailopsButton>
          <Popover>
            <PopoverTrigger render={<RailopsButton size="small" aria-label={ee("enterpriseLive.text083")} title={ee("enterpriseLive.text083")} />}>
              <Settings2Icon className="size-4" />
            </PopoverTrigger>
              <PopoverContent align="end" sideOffset={8} className="rhd-railops-notification-settings-popover w-[min(720px,calc(100vw-24px))] gap-0 overflow-hidden p-0">
                <PopoverHeader className="border-b border-border px-3 py-2.5">
                  <PopoverTitle className="text-sm font-semibold text-foreground">{ee("enterpriseLive.text083")}</PopoverTitle>
                  <div className="text-xs text-muted-foreground">{ee("enterpriseLive.text084")}</div>
                </PopoverHeader>
                <div className="rhd-railops-notification-settings-grid">
                  <section className="rhd-railops-notification-settings-section">
                    <div className="rhd-railops-notification-settings-title">
                      <span className="grid size-8 shrink-0 place-items-center rounded-md bg-primary/10 text-primary">
                        <MailIcon className="size-4" />
                      </span>
                      <span className="min-w-0">
                        <strong>{ee("enterpriseLive.text085")}</strong>
                        <small>{ee("enterpriseLive.text086")}</small>
                      </span>
                      <StatusTag tone={emailEnabled && recipientSetting?.effective_email ? "blue" : "neutral"}>
                        {emailEnabled ? ee("enterpriseLive.text087") : ee("enterpriseLive.text088")}
                      </StatusTag>
                    </div>

                    <div className="rhd-railops-notification-setting-row">
                      <span>
                        <strong>{ee("enterpriseLive.text089")}</strong>
                        <small>{ee("enterpriseLive.text090")}</small>
                      </span>
                      <Switch disabled={!canUpdateNotifications} checked={emailEnabled} onCheckedChange={setEmailEnabled} aria-label={ee("enterpriseLive.text089")} />
                    </div>

                    <div className="rhd-railops-notification-email-options">
                      <button
                        type="button"
                        aria-pressed={useProfileEmail}
                        className={cn("rhd-railops-notification-email-option", useProfileEmail && "is-active")}
                        onClick={() => setUseProfileEmail(true)}
                      >
                        <span className={cn("rhd-railops-notification-radio", useProfileEmail && "is-active")}><CheckIcon className="size-3" /></span>
                        <span>
                          <strong>{ee("enterpriseLive.text091")}</strong>
                          <small>{recipientSetting?.profile_email || ee("enterpriseLive.text092")}</small>
                        </span>
                      </button>
                      <button
                        type="button"
                        aria-pressed={!useProfileEmail}
                        className={cn("rhd-railops-notification-email-option", !useProfileEmail && "is-active")}
                        onClick={() => setUseProfileEmail(false)}
                      >
                        <span className={cn("rhd-railops-notification-radio", !useProfileEmail && "is-active")}><CheckIcon className="size-3" /></span>
                        <span>
                          <strong>{ee("enterpriseLive.text093")}</strong>
                          <small>{ee("enterpriseLive.text094")}</small>
                        </span>
                      </button>
                    </div>

                    {!useProfileEmail ? (
                      <MailSettingField label={ee("enterpriseLive.text095")}>
                        <Input type="email" autoComplete="email" disabled={!canUpdateNotifications} value={recipientEmail} onChange={(event) => setRecipientEmail(event.target.value)} placeholder="name@example.com" />
                      </MailSettingField>
                    ) : null}

                    <div className="rhd-railops-notification-settings-actions">
                      <RailopsButton onClick={() => void sendRecipientTest()} disabled={testing || !canUpdateNotifications || !recipientSetting?.email_enabled || !recipientSetting.effective_email}>
                        <SendIcon className="size-4" />{testing ? ee("enterpriseLive.text096") : ee("enterpriseLive.text097")}
                      </RailopsButton>
                      <RailopsButton variant="primary" onClick={() => void saveRecipientSetting()} disabled={saving || !canUpdateNotifications}>
                        <MailIcon className="size-4" />{saving ? ee("enterpriseLive.text098") : ee("enterpriseLive.text099")}
                      </RailopsButton>
                    </div>
                  </section>

                  <aside className="rhd-railops-notification-settings-section is-side">
                    <h3>{ee("enterpriseLive.text100")}</h3>
                    <dl>
                      <ChannelStatusTerm label={ee("enterpriseLive.text101")} value={recipientSetting?.effective_email || ee("enterpriseLive.text102")} danger={!recipientSetting?.effective_email} />
                      <ChannelStatusTerm label={ee("enterpriseLive.text103")} value={recipientSetting?.use_profile_email ? ee("enterpriseLive.text104") : ee("enterpriseLive.text105")} />
                      <ChannelStatusTerm label={ee("enterpriseLive.text106")} value={recipientSetting?.email_enabled ? ee("enterpriseLive.text107") : ee("enterpriseLive.text009")} />
                      <ChannelStatusTerm label={ee("enterpriseLive.text108")} value={ee("enterpriseLive.text109")} />
                    </dl>
                    {canViewAudit ? (
                      <Link href="/enterprise/audit" className="block w-full">
                        <RailopsButton variant="text" className="w-full" style={{ justifyContent: "flex-start" }}>
                          <ShieldCheckIcon className="size-4" />{ee("enterpriseLive.text110")}</RailopsButton>
                      </Link>
                    ) : null}
                  </aside>
                </div>
              </PopoverContent>
            </Popover>
          </>
        )}
      >

      <section className="min-w-0 rounded-md border border-border bg-card p-4 shadow-sm">
        <TableToolbar
          tabs={data?.can_view_tenant_scope ? (
            <FilterTabs
              ariaLabel={ee("enterpriseLive.text111")}
              items={[
                { label: ee("enterpriseLive.text112"), value: "personal", icon: <UserRoundIcon className="size-3.5" /> },
                { label: ee("enterpriseLive.text113"), value: "tenant", icon: <UsersRoundIcon className="size-3.5" /> },
              ]}
              value={scope}
              onChange={(value) => {
                setPage(1)
                setScope(value as "personal" | "tenant")
                setSelectedNotification(null)
              }}
            />
          ) : undefined}
          search={(
            <SearchField
              allowClear
              aria-label={ee("enterpriseLive.text114")}
              className="rhd-railops-search-compact"
              placeholder={ee("enterpriseLive.text115")}
              value={searchInput}
              onChange={(event) => {
                setSearchInput(event.target.value)
                setSelectedNotification(null)
              }}
            />
          )}
          filters={(
            <TableFilters
              fields={notificationFilterFields}
              value={notificationFilterValues}
              onChange={applyNotificationFilters}
              onReset={() => applyNotificationFilters({})}
            />
          )}
          activeFilters={(
            <ActiveFilterTags
              fields={notificationFilterFields}
              value={notificationFilterValues}
              onChange={applyNotificationFilters}
              onReset={() => applyNotificationFilters({})}
            />
          )}
        />

        {notificationListInitialLoading ? <ListSkeleton /> : notificationListBlockingError ? (
          <ErrorState title={ee("enterpriseLive.text036")} description={error} action={{ label: ee("enterpriseLive.text037"), onClick: load }} className="rounded-none border-0" />
        ) : visibleItems.length === 0 ? (
          <EmptyState
            title={ee("enterpriseLive.text116", { value0: NOTIFICATION_CATEGORY_LABELS[category] && category !== "all" ? NOTIFICATION_CATEGORY_LABELS[category] : "" })}
            className="rounded-none border-0"
          />
        ) : (
          <NotificationTable
            items={visibleItems}
            selectedId={selectedNotification?.id ?? 0}
            onAction={handleAction}
            onMarkRead={handleMarkRead}
            onSelect={setSelectedNotification}
            canUpdate={canUpdateNotifications}
            current={data?.page ?? page}
            total={data?.total ?? 0}
            footerNote={ee("enterpriseLive.text117", { value0: data?.total ?? 0, value1: data?.scope === "tenant" ? ee("enterpriseLive.text148") : ee("enterpriseLive.text149") })}
            onPageChange={(nextPage) => {
              setSelectedNotification(null)
              setPage(nextPage)
            }}
          />
        )}

        <NotificationDetailDrawer
          item={selectedNotification}
          open={Boolean(selectedNotification)}
          canUpdate={canUpdateNotifications}
          onAction={handleAction}
          onClose={() => setSelectedNotification(null)}
          onMarkRead={handleMarkRead}
        />
      </section>
    </PageShell>
  )
}

function MailSettingField({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block">
      <span className="mb-1 block text-xs font-medium text-muted-foreground">{label}</span>
      {children}
    </label>
  )
}

function ChannelStatusTerm({ label, value, danger = false }: { label: string; value: string; danger?: boolean }) {
  return (
    <div className="flex items-start justify-between gap-3">
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className={cn("min-w-0 break-all text-right text-xs font-medium", danger ? "text-destructive" : "text-foreground")}>{value}</dd>
    </div>
  )
}

function notificationUnread(item: EnterpriseNotification) {
  return item.is_event_summary ? item.unread_count > 0 : !item.read_at
}

function NotificationPill({
  children,
  className,
}: {
  children: ReactNode
  className?: string
}) {
  return (
    <span className={cn("rhd-railops-notification-pill", className)}>
      {children}
    </span>
  )
}

function NotificationTable({
  current = 1,
  footerNote,
  items,
  onAction,
  onMarkRead,
  onPageChange,
  onSelect,
  canUpdate,
  selectedId,
  total,
}: {
  current?: number
  footerNote?: ReactNode
  items: EnterpriseNotification[]
  onAction: (item: EnterpriseNotification) => void
  onMarkRead: (item: EnterpriseNotification) => Promise<boolean>
  onPageChange?: (page: number, pageSize: number) => void
  onSelect: (item: EnterpriseNotification) => void
  canUpdate: boolean
  selectedId: number
  total: number
}) {
  const columns: TableColumnsType<EnterpriseNotification> = [
    {
      title: ee("enterpriseLive.text118"),
      dataIndex: "title",
      width: 330,
      render: (_value, item) => {
        const level = notificationLevelMeta(item.level)
        const unread = notificationUnread(item)
        return (
          <div className="rhd-railops-notification-title-cell">
            <span className={cn("rhd-railops-notification-level-dot", level.dot)} title={level.label} />
            <span className="min-w-0">
              <strong className={cn(unread && "is-unread")} title={item.title || item.notification_type}>
                {item.title || item.notification_type}
              </strong>
              {item.content ? (
                <span title={item.content}>
                  {item.content.replace(/\s+/g, " ").trim()}
                </span>
              ) : null}
            </span>
          </div>
        )
      },
    },
    {
      title: ee("enterpriseLive.text119"),
      dataIndex: "category",
      width: 142,
      render: (_value, item) => {
        const level = notificationLevelMeta(item.level)
        const levelTone: StatusTagTone = item.level === "urgent" ? "error" : item.level === "warning" ? "warning" : "neutral"
        return (
          <div className="rhd-railops-notification-stack">
            <NotificationPill>{notificationCategoryLabel(item.category, notificationTypeLabel(item.notification_type))}</NotificationPill>
            {item.level !== "info" ? <StatusTag tone={levelTone}>{level.label}</StatusTag> : null}
          </div>
        )
      },
    },
    {
      title: ee("enterpriseLive.text120"),
      dataIndex: "recipient_name",
      width: 136,
      render: (_value, item) => (
        <div className="rhd-railops-notification-stack">
          <span className="font-medium text-foreground">
            {item.is_event_summary ? ee("enterpriseLive.text121", { value0: item.recipient_count }) : item.recipient_name || ee("enterpriseLive.text122")}
          </span>
          {item.is_event_summary ? (
            <span className={cn(notificationUnread(item) ? "text-primary" : "text-muted-foreground")}>
              {notificationUnread(item) ? ee("enterpriseLive.text123", { value0: item.unread_count }) : ee("enterpriseLive.text075")}
            </span>
          ) : null}
        </div>
      ),
    },
    {
      title: ee("enterpriseLive.text124"),
      dataIndex: "channels",
      width: 150,
      render: (_value, item) => {
        const email = emailStatusMeta(item.email_status)
        return (
          <div className="rhd-railops-notification-stack">
            <span>{(item.channels ?? []).map(channelLabel).join(" + ") || ee("enterpriseLive.text051")}</span>
            {item.is_event_summary && item.email_failed_count > 0 ? (
              <span className="text-destructive">{ee("enterpriseLive.text125")}{item.email_failed_count}</span>
            ) : item.is_event_summary && item.email_pending_count > 0 ? (
              <span className="text-amber-700">{ee("enterpriseLive.text126")}{item.email_pending_count}</span>
            ) : item.channels?.includes("email") ? (
              <span className={email.badge.split(" ").at(-1)}>{email.label}</span>
            ) : null}
          </div>
        )
      },
    },
    {
      title: ee("enterpriseLive.text127"),
      dataIndex: "read_at",
      width: 116,
      align: "center",
      render: (_value, item) => (
        <div className="rhd-railops-notification-stack is-centered">
          <StatusTag tone={notificationUnread(item) ? "blue" : "neutral"}>
            {notificationUnread(item) ? ee("enterpriseLive.text026") : ee("enterpriseLive.text025")}
          </StatusTag>
          {item.delivery_status && item.delivery_status !== "sent" ? (
            <StatusTag tone={item.delivery_status === "failed" ? "error" : "warning"}>{deliveryStatusLabel(item.delivery_status)}</StatusTag>
          ) : null}
        </div>
      ),
    },
    {
      title: ee("enterpriseLive.text128"),
      dataIndex: "created_at",
      width: 132,
      align: "right",
      render: (_value, item) => (
        <div className="rhd-railops-notification-stack is-right">
          <span className="font-medium text-foreground">{relativeNotificationTime(item.created_at)}</span>
          <span>{dateText(item.created_at)}</span>
        </div>
      ),
    },
    {
      title: ee("enterpriseLive.text129"),
      key: "actions",
      width: 104,
      align: "center",
      render: (_value, item) => {
        const actionUrl = normalizeEnterpriseActionUrl(item)
        return (
          <div className="rhd-railops-notification-actions">
            {notificationUnread(item) && item.can_mark_read && canUpdate ? (
              <IconButton
                icon={<CheckIcon className="size-4" />}
                tooltip={ee("enterpriseLive.text130")}
                aria-label={ee("enterpriseLive.text131", { value0: item.title || item.notification_type })}
                onClick={(event) => {
                  event.stopPropagation()
                  void onMarkRead(item)
                }}
              />
            ) : null}
            {actionUrl ? (
              <IconButton
                icon={<ExternalLinkIcon className="size-3.5" />}
                tooltip={ee("enterpriseLive.text132")}
                aria-label={ee("enterpriseLive.text133", { value0: item.title || item.notification_type })}
                onClick={(event) => {
                  event.stopPropagation()
                  void onAction(item)
                }}
              />
            ) : null}
          </div>
        )
      },
    },
  ]

  return (
    <DataTable<EnterpriseNotification>
      className="rhd-railops-notifications-table"
      columns={columns}
      current={current}
      dataSource={items}
      footerNote={footerNote}
      pagination={false}
      rowKey="id"
      scroll={{ x: 1110 }}
      total={total}
      onPageChange={onPageChange}
      rowClassName={(record) => cn(
        "rhd-railops-notification-row",
        notificationUnread(record) && "is-unread",
        selectedId === record.id && "is-selected",
      )}
      onRow={(record) => ({
        onClick: () => onSelect(record),
        onKeyDown: (event) => {
          if (event.key !== "Enter" && event.key !== " ") return
          event.preventDefault()
          onSelect(record)
        },
        role: "button",
        tabIndex: 0,
      })}
    />
  )
}

function NotificationDetailDrawer({
  canUpdate,
  item,
  onAction,
  onClose,
  onMarkRead,
  open,
}: {
  canUpdate: boolean
  item: EnterpriseNotification | null
  onAction: (item: EnterpriseNotification) => void
  onClose: () => void
  onMarkRead: (item: EnterpriseNotification) => Promise<boolean>
  open: boolean
}) {
  if (!item) return null

  const level = notificationLevelMeta(item.level)
  const email = emailStatusMeta(item.email_status)
  const unread = notificationUnread(item)
  const actionUrl = normalizeEnterpriseActionUrl(item)

  return (
    <AntDrawer
      className="rhd-railops-notification-detail-drawer"
      rootClassName="rhd-railops-notification-detail-drawer-root"
      open={open}
      title={(
        <div className="rhd-railops-notification-detail-title">
          <span>{item.title || item.notification_type}</span>
          <NotificationPill className={toneClass(unread ? "blue" : "slate")}>
            {unread ? ee("enterpriseLive.text026") : ee("enterpriseLive.text025")}
          </NotificationPill>
        </div>
      )}
      width="min(520px, calc(100vw - 24px))"
      onClose={onClose}
      extra={(
        <div className="rhd-railops-notification-drawer-actions">
          {unread && item.can_mark_read && canUpdate ? (
            <RailopsButton size="small" onClick={() => void onMarkRead(item)}>
              <CheckIcon className="size-3.5" />{ee("enterpriseLive.text130")}</RailopsButton>
          ) : null}
          {actionUrl ? (
            <RailopsButton size="small" variant="primary" onClick={() => void onAction(item)}>
              <ExternalLinkIcon className="size-3.5" />{ee("enterpriseLive.text132")}</RailopsButton>
          ) : null}
        </div>
      )}
    >
      <div className="rhd-railops-notification-detail-body">
        <section className="rhd-railops-notification-detail-section">
          <div className="rhd-railops-notification-detail-tags">
            <NotificationPill>{notificationCategoryLabel(item.category, notificationTypeLabel(item.notification_type))}</NotificationPill>
            {item.level !== "info" ? <NotificationPill className={level.badge}>{level.label}</NotificationPill> : null}
            {item.delivery_status && item.delivery_status !== "sent" ? (
              <NotificationPill className={toneClass(item.delivery_status === "failed" ? "red" : "amber")}>
                {deliveryStatusLabel(item.delivery_status)}
              </NotificationPill>
            ) : null}
          </div>
          {item.content ? (
            <p className="rhd-railops-notification-detail-content">{item.content}</p>
          ) : (
            <p className="rhd-railops-notification-detail-content is-muted">{ee("enterpriseLive.text134")}</p>
          )}
        </section>

        <section className="rhd-railops-notification-detail-section">
          <h3>{ee("enterpriseLive.text135")}</h3>
          <dl className="rhd-railops-notification-detail-facts">
            <NotificationFact label={ee("enterpriseLive.text120")} value={item.is_event_summary ? ee("enterpriseLive.text121", { value0: item.recipient_count }) : item.recipient_name || ee("enterpriseLive.text122")} />
            {item.is_event_summary ? (
              <NotificationFact label={ee("enterpriseLive.text136")} value={`${item.unread_count}/${item.recipient_count}`} />
            ) : null}
            <NotificationFact label={ee("enterpriseLive.text137")} value={(item.channels ?? []).map(channelLabel).join(" + ") || ee("enterpriseLive.text051")} />
            <NotificationFact label={ee("enterpriseLive.text138")} value={notificationDeliveryDetailStatus(item, email)} />
            <NotificationFact label={ee("enterpriseLive.text140")} value={dateText(item.created_at)} />
            <NotificationFact label={ee("enterpriseLive.text141")} value={relativeNotificationTime(item.created_at)} />
          </dl>
        </section>

        <section className="rhd-railops-notification-detail-section">
          <h3>{ee("enterpriseLive.text142")}</h3>
          <dl className="rhd-railops-notification-detail-facts">
            <NotificationFact label={ee("enterpriseLive.text143")} value={notificationTypeLabel(item.notification_type)} />
            <NotificationFact label={ee("enterpriseLive.text144")} value={notificationTypeLabel(item.biz_type)} />
            <NotificationFact label={ee("enterpriseLive.text145")} value={item.biz_id ? String(item.biz_id) : "-"} />
            <NotificationFact label={ee("enterpriseLive.text146")} value={externalChannelStatusLabel(item.external_channel_status)} />
          </dl>
        </section>
      </div>
    </AntDrawer>
  )
}

function NotificationFact({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt>{label}</dt>
      <dd>{value}</dd>
    </div>
  )
}

function ListSkeleton({ label = ee("enterpriseLive.text147") }: { label?: string }) {
  return (
    <div className="space-y-2.5 p-3" role="status" aria-busy="true" aria-label={label}>
      {Array.from({ length: 5 }).map((_, index) => (
        <Skeleton.Node key={index} active className="w-full" style={{ width: "100%", height: 48 }} />
      ))}
    </div>
  )
}
