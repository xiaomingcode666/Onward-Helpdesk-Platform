"use client"

import { useState, useMemo, useCallback } from "react"
import {
  HistoryIcon,
  MessageSquareIcon,
  TicketIcon,
  FilterIcon,
  ChevronRightIcon,
  Loader2Icon,
  InboxIcon,
  ClockIcon,
  UserIcon,
} from "lucide-react"

import { SelectField, UnderlineTabs, type RailopsTabItem } from "@railops/ui"

import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { Separator } from "@/components/ui/separator"

import { useAppLocale, useI18n } from "@/i18n/provider"
import type {
  CustomerEntryConversation,
  CustomerEntryTicket,
} from "@/lib/api/customer-entry"

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type HistoryState = "loading" | "empty" | "list"

type HistoryTab = "conversations" | "tickets"

type Conversation = CustomerEntryConversation
type Ticket = CustomerEntryTicket

interface HistoryListProps {
  conversations?: Conversation[]
  tickets?: Ticket[]
  loading?: boolean
  error?: string
  onRetry?: () => void
  /** Called when a conversation item is clicked */
  onViewConversation?: (conv: Conversation) => void
  /** Called when a ticket item is clicked */
  onViewTicket?: (ticket: Ticket) => void
  /** Device filter options */
  devices?: string[]
}

// ---------------------------------------------------------------------------
// Helper components
// ---------------------------------------------------------------------------

function getStatusBadgeVariant(s: string) {
  const map: Record<string, "default" | "secondary" | "outline"> = {
    pending: "secondary",
    pending_acceptance: "secondary",
    pending_dispatch: "secondary",
    pending_assignee_accept: "secondary",
    in_progress: "default",
    processing: "default",
    resolved: "secondary",
    pending_customer_confirm: "secondary",
    closed: "outline",
    cancelled: "outline",
    done: "outline",
  }
  return map[s] || "outline"
}

function getTicketStatusLabel(status: string, t: ReturnType<typeof useI18n>) {
  if (status === "pending" || status === "pending_acceptance" || status === "pending_dispatch") {
    return t("portalExtract.serviceCode.historyList.ticketPending")
  }
  if (status === "pending_assignee_accept") {
    return t("portalExtract.serviceCode.historyList.ticketPendingAssigneeAccept")
  }
  if (status === "in_progress" || status === "processing" || status === "accepted") {
    return t("portalExtract.serviceCode.historyList.ticketInProgress")
  }
  if (status === "resolved" || status === "pending_customer_confirm") {
    return t("portalExtract.serviceCode.historyList.ticketPendingConfirmation")
  }
  if (status === "closed" || status === "cancelled" || status === "done") {
    return t("portalExtract.serviceCode.historyList.ticketDone")
  }
  return status.replaceAll("_", " ")
}

const formatDate = (
  iso: string,
  locale: string,
  t: ReturnType<typeof useI18n>,
) => {
  const d = new Date(iso)
  const now = new Date()
  const diffMs = now.getTime() - d.getTime()
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24))

  if (diffDays === 0) {
    return d.toLocaleTimeString(locale, { hour: "2-digit", minute: "2-digit" })
  }
  if (diffDays === 1) return t("portalExtract.serviceCode.historyList.yesterday")
  if (diffDays < 7) {
    return t("portalExtract.serviceCode.historyList.daysAgo", { count: diffDays })
  }
  return d.toLocaleDateString(locale, { month: "short", day: "numeric" })
}

// ---------------------------------------------------------------------------
// Main component
// ---------------------------------------------------------------------------

export function HistoryList({
  conversations = [],
  tickets = [],
  loading = false,
  error = "",
  onRetry,
  onViewConversation,
  onViewTicket,
  devices,
}: HistoryListProps) {
  const { locale, t } = useAppLocale()

  // ---- State ----
  const [activeTab, setActiveTab] = useState<HistoryTab>("conversations")
  const historyTabs: RailopsTabItem[] = [
    { value: "conversations", label: t("portalExtract.serviceCode.historyList.conversations"), icon: <MessageSquareIcon className="size-3.5" /> },
    { value: "tickets", label: t("portalExtract.serviceCode.historyList.tickets"), icon: <TicketIcon className="size-3.5" /> },
  ]
  const [deviceFilter, setDeviceFilter] = useState<string>("all")
  const state: HistoryState = loading ? "loading" : "list"

  // Derived unique devices for filter
  const deviceOptions = useMemo(() => {
    if (devices) return devices
    const allDevices = new Set<string>()
    conversations.forEach((c) => {
      if (c.deviceNo) allDevices.add(c.deviceNo)
    })
    tickets.forEach((t) => {
      if (t.deviceNo) allDevices.add(t.deviceNo)
    })
    return Array.from(allDevices).sort()
  }, [conversations, devices, tickets])

  // Filtered data
  const filteredConversations = useMemo(
    () =>
      deviceFilter === "all"
        ? conversations
        : conversations.filter((c) => c.deviceNo === deviceFilter),
    [conversations, deviceFilter]
  )

  const filteredTickets = useMemo(
    () =>
      deviceFilter === "all"
        ? tickets
        : tickets.filter((t) => t.deviceNo === deviceFilter),
    [deviceFilter, tickets]
  )

  // ---- Device filter change ----
  const handleDeviceFilterChange = useCallback((value: string | null) => {
    if (!value) return
    setDeviceFilter(value)
  }, [])

  // ---- Render skeleton ----
  const renderSkeleton = () => (
    <div className="space-y-3 p-4">
      {Array.from({ length: 3 }).map((_, i) => (
        <Card key={i}>
          <CardContent className="p-4">
            <div className="flex items-start gap-3">
              <Skeleton className="size-10 shrink-0 rounded-lg" />
              <div className="min-w-0 flex-1 space-y-2">
                <Skeleton className="h-4 w-3/4" />
                <Skeleton className="h-3 w-1/2" />
                <Skeleton className="h-3 w-1/3" />
              </div>
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  )

  // ---- Render empty ----
  const renderEmpty = () => (
    <div className="flex flex-col items-center gap-3 px-4 py-12">
      <div className="flex size-16 items-center justify-center rounded-full bg-muted">
        <InboxIcon className="size-8 text-muted-foreground" />
      </div>
      <p className="text-sm font-medium text-muted-foreground">
        {t("portalExtract.serviceCode.historyList.emptyTitle")}
      </p>
    </div>
  )

  // ---- Render conversation item ----
  const renderConversation = (conv: Conversation) => (
    <button
      key={conv.id}
      type="button"
      onClick={() => onViewConversation?.(conv)}
      className="w-full text-left"
    >
      <Card className="transition-colors hover:bg-muted/50 active:bg-muted">
        <CardContent className="flex items-start gap-3 p-4">
          {/* Agent type icon */}
            <div
            className={`flex size-10 shrink-0 items-center justify-center rounded-lg ${
              conv.agentType === "ai"
                ? "bg-blue-100 text-blue-600"
                : "bg-primary/10 text-primary"
            }`}
          >
            {conv.agentType === "ai" ? (
              <MessageSquareIcon className="size-5" />
            ) : (
              <UserIcon className="size-5" />
            )}
          </div>

          <div className="min-w-0 flex-1">
            {/* Summary + badge */}
            <div className="flex items-start gap-2">
              <p className="flex-1 truncate text-sm font-medium leading-snug">
                {conv.summary}
              </p>
              <Badge
                variant={conv.agentType === "ai" ? "secondary" : "default"}
                className="shrink-0 text-rhd-2xs"
              >
                {conv.agentType === "ai"
                  ? t("portalExtract.serviceCode.historyList.ai")
                  : t("portalExtract.serviceCode.historyList.human")}
              </Badge>
            </div>

            {/* Metadata */}
            <div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs text-muted-foreground">
              <span className="flex items-center gap-1">
                <ClockIcon className="size-3" />
                {formatDate(conv.startedAt, locale, t)}
              </span>
              <span>{conv.deviceNo}</span>
              {conv.endedAt && (
                <span>
                  {Math.round(
                    (new Date(conv.endedAt).getTime() -
                      new Date(conv.startedAt).getTime()) /
                      60000
                  )}
                  {t("portalExtract.serviceCode.historyList.minutes")}
                </span>
              )}
            </div>
          </div>

          {/* Arrow */}
          <ChevronRightIcon className="mt-1 size-4 shrink-0 text-muted-foreground/50" />
        </CardContent>
      </Card>
    </button>
  )

  // ---- Render ticket item ----
  const renderTicket = (ticket: Ticket) => (
    <button
      key={ticket.id}
      type="button"
      onClick={() => onViewTicket?.(ticket)}
      className="w-full text-left"
    >
      <Card className="transition-colors hover:bg-muted/50 active:bg-muted">
        <CardContent className="flex items-start gap-3 p-4">
          {/* Ticket icon */}
          <div className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-amber-100 text-amber-600">
            <TicketIcon className="size-5" />
          </div>

          <div className="min-w-0 flex-1">
            {/* Title + status */}
            <div className="flex items-start gap-2">
              <p className="flex-1 truncate text-sm font-medium leading-snug">
                {ticket.title}
              </p>
              <Badge
                variant={getStatusBadgeVariant(ticket.status)}
                className="shrink-0 text-rhd-2xs"
              >
                {getTicketStatusLabel(ticket.status, t)}
              </Badge>
            </div>

            {/* Metadata */}
            <div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs text-muted-foreground">
              <span className="font-mono">{ticket.ticketNo}</span>
              <span>{ticket.deviceNo}</span>
              <span className="flex items-center gap-1">
                <ClockIcon className="size-3" />
                {formatDate(ticket.createdAt, locale, t)}
              </span>
            </div>
          </div>

          {/* Arrow */}
          <ChevronRightIcon className="mt-1 size-4 shrink-0 text-muted-foreground/50" />
        </CardContent>
      </Card>
    </button>
  )

  // ---- Main render ----

  return (
    <div className="flex flex-col">
      {/* Header */}
      <div className="flex items-center gap-2 border-b px-4 py-3">
        <HistoryIcon className="size-5 text-primary" />
        <h2 className="text-sm font-medium">
          {t("portalExtract.serviceCode.historyList.title")}
        </h2>
      </div>

      {/* Device filter */}
      <div className="flex items-center gap-3 px-4 py-2">
        <FilterIcon className="size-4 text-muted-foreground" />
        <SelectField
          style={{ marginBottom: 0 }}
          selectProps={{
            placeholder: t("portalExtract.serviceCode.historyList.filterDevice"),
            value: deviceFilter,
            onChange: handleDeviceFilterChange,
            options: [
              { value: "all", label: t("portalExtract.serviceCode.historyList.allDevices") },
              ...deviceOptions.map((d) => ({ value: d, label: d })),
            ],
            style: { flex: 1, minWidth: 0 },
          }}
        />
      </div>

      <Separator />

      {error ? (
        <div className="mx-4 mt-3 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
          <p>{error}</p>
          {onRetry ? (
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="mt-2"
              onClick={onRetry}
            >
              {t("portalExtract.serviceCode.historyList.retry")}
            </Button>
          ) : null}
        </div>
      ) : null}

      {/* Tab switcher */}
      <div className="px-4 pt-2">
        <UnderlineTabs
          ariaLabel={t("portalExtract.serviceCode.historyList.tabsAria")}
          items={historyTabs}
          value={activeTab}
          onChange={(value) => setActiveTab(value as HistoryTab)}
        />
      </div>

      {/* List */}
      <div className="flex-1 overflow-y-auto pb-4">
        {/* Loading */}
        {state === "loading" && renderSkeleton()}

        {/* List / empty for conversations tab */}
        {state !== "loading" && activeTab === "conversations" && (
          <>
            {filteredConversations.length === 0 ? (
              renderEmpty()
            ) : (
              <div className="space-y-2 p-4">
                {filteredConversations.map(renderConversation)}
              </div>
            )}
          </>
        )}

        {/* List / empty for tickets tab */}
        {state !== "loading" && activeTab === "tickets" && (
          <>
            {filteredTickets.length === 0 ? (
              renderEmpty()
            ) : (
              <div className="space-y-2 p-4">
                {filteredTickets.map(renderTicket)}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}
