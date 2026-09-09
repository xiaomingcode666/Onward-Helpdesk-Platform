"use client"

import {
  VideoIcon,
  ClipboardListIcon,
  HeadphonesIcon,
  HistoryIcon,
  Loader2Icon,
  MessageCircleIcon,
  PackageCheckIcon,
  RefreshCwIcon,
  ShieldAlertIcon,
  SmartphoneIcon,
  UserCircleIcon,
  WrenchIcon,
  ChevronLeftIcon,
  AlertCircleIcon,
  InfoIcon,
	StarIcon,
	RotateCcwIcon,
} from "lucide-react"

import { useState } from "react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import {
	confirmCustomerEntryTicket,
	fetchCustomerEntryMeetingJoin,
	reopenCustomerEntryTicket,
	submitCustomerEntryTicketFeedback,
	type CustomerEntryContext,
	type CustomerEntryChat,
	type CustomerEntryKnowledgeEntry,
	type CustomerEntryState,
	type CustomerEntryTicket,
} from "@/lib/api/customer-entry"
import { CustomerEntryChatPanel } from "@/components/remote-helpdesk/customer-entry-chat"
import { MobileLanguageSwitch } from "@/components/remote-helpdesk/mobile-language-switch"
import { cn } from "@/lib/utils"
import { VoiceInput } from "@/app/c/[serviceCode]/components/voice-input"
import { DiagnosisFlow } from "@/app/c/[serviceCode]/components/diagnosis-flow"
import { GuideViewer } from "@/app/c/[serviceCode]/components/guide-viewer"
import { PrivacyConsent, type CookieConsent } from "@/app/c/[serviceCode]/components/privacy-consent"
import { ManualViewer } from "@/app/c/[serviceCode]/components/manual-viewer"
import { HistoryList } from "@/app/c/[serviceCode]/components/history-list"
import { ModuleLoading } from "@/components/shared/loading-states"
import { normalizeSelectableLocale } from "@/i18n/config"
import { useAppLocale, useI18n } from "@/i18n/provider"
import { renderMobileTicketTitle } from "@/components/remote-helpdesk/mobile-ticket-labels"

export type MobileServiceTab = "chat" | "tickets" | "video" | "devices" | "my"

type MobileServiceShellProps = {
  serviceCode: string
  state: CustomerEntryState | null
  activeTab: MobileServiceTab
  loading?: boolean
  sessionStarting?: boolean
  registering?: boolean
	authenticated?: boolean
  error?: string
  deviceNo?: string
  onDeviceNoChange: (value: string) => void
  onRegisterDevice: () => void
  onRetry: () => void
  onTabChange: (tab: MobileServiceTab) => void
  // State routing props
  currentMobileState?: string
  onNavigate?: (state: string, extraParams?: Record<string, string>) => void
  detailId?: string
  // Sub-state loading/error
  subStateLoading?: boolean
  subStateError?: string
  onSubStateRetry?: () => void
  customerContext?: CustomerEntryContext | null
  contextLoading?: boolean
  contextError?: string
  onContextRetry?: () => void
  onPrivacyConsentAgree?: (consent: CookieConsent) => Promise<void>
  privacyConsentError?: string
}

const tabs: {
  value: MobileServiceTab
  labelKey: string
  icon: typeof MessageCircleIcon
}[] = [
  { value: "chat", labelKey: "portalExtract.customerMobile.tabs.chat", icon: MessageCircleIcon },
  { value: "tickets", labelKey: "portalExtract.customerMobile.tabs.tickets", icon: ClipboardListIcon },
  { value: "video", labelKey: "portalExtract.customerMobile.tabs.video", icon: VideoIcon },
  { value: "devices", labelKey: "portalExtract.customerMobile.tabs.devices", icon: WrenchIcon },
  { value: "my", labelKey: "portalExtract.customerMobile.tabs.my", icon: UserCircleIcon },
]

type I18nT = ReturnType<typeof useI18n>

function getProductName(state: CustomerEntryState | null, t: I18nT) {
  if (!state || state.kind === "invalid" || state.kind === "revoked") {
    return t("portalExtract.customerMobile.common.fallbackService")
  }
  return state.product?.name || state.product?.code || t("portalExtract.customerMobile.common.fallbackService")
}

function getDeviceLabel(state: CustomerEntryState | null, t: I18nT) {
	if (!state || state.kind === "invalid" || state.kind === "revoked") {
		return t("portalExtract.customerMobile.serviceShell.pendingDevice")
	}
  return (
    state.device?.deviceNo ||
    state.device?.serialNo ||
    state.device?.name ||
    t("portalExtract.customerMobile.serviceShell.productConsult")
  )
}

function formatShortDate(value?: string) {
  if (!value) {
    return "-"
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return date.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  })
}

function ticketStatusLabel(status: string, t: I18nT) {
  const labels: Record<string, string> = {
    pending_acceptance: t("portalExtract.customerMobile.status.pendingAcceptance"),
    accepted: t("portalExtract.customerMobile.status.accepted"),
    pending_dispatch: t("portalExtract.customerMobile.status.pendingDispatch"),
    pending_assignee_accept: t("portalExtract.customerMobile.status.pendingAssigneeAccept"),
    in_progress: t("portalExtract.customerMobile.status.processing"),
		processing: t("portalExtract.customerMobile.status.processing"),
		video_support: t("portalExtract.customerMobile.serviceShell.videoCollaboration"),
		supplier_support: t("portalExtract.customerMobile.status.processing"),
		resolved: t("portalExtract.customerMobile.status.pendingConfirmation"),
		pending_customer_confirm: t("portalExtract.customerMobile.status.pendingConfirmation"),
		closed: t("portalExtract.customerMobile.status.closed"),
		reopened: t("portalExtract.customerMobile.status.reopened"),
		cancelled: t("portalExtract.customerMobile.status.cancelled"),
    done: t("portalExtract.customerMobile.status.done"),
  }
  return labels[status] ?? status.replaceAll("_", " ")
}

function ticketStatusVariant(status: string) {
	if (status === "done" || status === "closed" || status === "cancelled") {
    return "outline" as const
  }
  if (status === "pending_acceptance" || status === "pending_dispatch" || status === "pending_assignee_accept") {
    return "secondary" as const
  }
  return "default" as const
}

function ticketPriorityLabel(priority: string, t: I18nT) {
	return ({
		critical: t("portalExtract.customerMobile.priority.critical"),
		high: t("portalExtract.customerMobile.priority.high"),
		medium: t("portalExtract.customerMobile.priority.medium"),
		low: t("portalExtract.customerMobile.priority.low"),
	} as Record<string, string>)[priority] || priority || "-"
}

function knowledgeEntriesByType(
  entries: CustomerEntryKnowledgeEntry[] | undefined,
  types: string[]
) {
  const expected = new Set(types)
  return (entries ?? []).filter((entry) =>
    expected.has(entry.type.toLowerCase())
  )
}

function guideStepsFromKnowledge(entries: CustomerEntryKnowledgeEntry[] | undefined, t: I18nT) {
  return knowledgeEntriesByType(entries, ["guide", "troubleshooting"]).map(
    (entry, index) => ({
      id: entry.id,
      order: index + 1,
      title: entry.title,
      description:
        entry.content?.trim() ||
        t("portalExtract.serviceCode.guideViewer.emptyTitle"),
    })
  )
}

function LoadingState() {
  const t = useI18n()
  return (
    <section className="space-y-4 px-5 py-4" aria-busy="true" aria-label={t("portalExtract.customerMobile.serviceShell.sessionLoading")}>
      <div className="rounded-lg border border-[var(--railops-border-light)] bg-[var(--railops-surface)] p-4 shadow-[var(--railops-card-shadow)]">
        <Skeleton className="h-4 w-24" />
        <Skeleton className="mt-3 h-20 w-full" />
      </div>
      <div className="grid grid-cols-2 gap-2">
        <Skeleton className="h-28 rounded-lg" />
        <Skeleton className="h-28 rounded-lg" />
      </div>
    </section>
  )
}

function InvalidState({
  state,
  error,
  onRetry,
}: {
  state: Extract<CustomerEntryState, { kind: "invalid" }>
  error?: string
  onRetry: () => void
}) {
  const t = useI18n()
  const message = error || state.reason

  return (
    <section className="flex min-h-[340px] flex-col justify-center px-5">
      <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-5">
        <ShieldAlertIcon className="size-9 text-destructive" />
        <h2 className="mt-4 text-xl font-semibold">{t("portalExtract.customerMobile.serviceShell.invalidServiceCode")}</h2>
        {message ? <p className="mt-2 text-sm leading-6 text-muted-foreground">{message}</p> : null}
        {state.serviceCode ? (
          <p className="mt-3 rounded-md bg-background px-3 py-2 font-mono text-xs text-muted-foreground">
            {state.serviceCode}
          </p>
        ) : null}
        <Button type="button" className="mt-5 w-full" onClick={onRetry}>
          <RefreshCwIcon />
          {t("portalExtract.customerMobile.common.retry")}
        </Button>
      </div>
    </section>
  )
}

function RevokedState({
  state,
  onRetry,
}: {
  state: Extract<CustomerEntryState, { kind: "revoked" }>
  onRetry: () => void
}) {
  const t = useI18n()
  return (
    <section className="flex min-h-[340px] flex-col justify-center px-5">
	      <div className="rounded-lg border border-border bg-muted/30 p-5">
	        <ShieldAlertIcon className="size-9 text-muted-foreground" />
		        <h2 className="mt-4 text-xl font-semibold">{t("portalExtract.customerMobile.serviceShell.revokedEntry")}</h2>
	        <div className="mt-4 space-y-2 rounded-md bg-background p-3 text-sm">
          <div className="flex items-center justify-between gap-4">
	            <span className="text-muted-foreground">{t("portalExtract.customerMobile.common.serviceCode")}</span>
            <span className="font-mono">{state.serviceCode || "-"}</span>
          </div>
          <div className="flex items-center justify-between gap-4">
	            <span className="text-muted-foreground">{t("portalExtract.customerMobile.serviceShell.revokedAt")}</span>
            <span>{state.revokedAt || "-"}</span>
          </div>
        </div>
        <Button type="button" variant="outline" className="mt-5 w-full" onClick={onRetry}>
          <RefreshCwIcon />
	          {t("portalExtract.customerMobile.common.reload")}
        </Button>
      </div>
    </section>
  )
}

function AccessErrorState({
  state,
  onRetry,
}: {
  state: Extract<CustomerEntryState, { kind: "accessError" }>
  onRetry: () => void
}) {
  const t = useI18n()
  return (
    <section className="flex min-h-[340px] flex-col justify-center px-5">
      <div className="rounded-lg border border-border bg-muted/30 p-5">
        <ShieldAlertIcon className="size-9 text-primary" />
	        <h2 className="mt-4 text-xl font-semibold">{t("portalExtract.customerMobile.serviceShell.accessUnavailable")}</h2>
        <p className="mt-2 text-sm leading-6 text-muted-foreground">{state.reason}</p>
        <Button type="button" className="mt-5 w-full" onClick={onRetry}>
          <RefreshCwIcon />
	          {t("portalExtract.customerMobile.common.reload")}
        </Button>
      </div>
    </section>
  )
}

function RegisterState({
  state,
  deviceNo,
	registering,
	authenticated,
  sessionStarting,
  error,
  onDeviceNoChange,
  onRegisterDevice,
}: {
  state: Extract<CustomerEntryState, { kind: "needRegister" }>
  deviceNo?: string
	registering?: boolean
	authenticated?: boolean
  sessionStarting?: boolean
  error?: string
  onDeviceNoChange: (value: string) => void
  onRegisterDevice: () => void
}) {
  const t = useI18n()
  const busy = Boolean(registering || sessionStarting)

  return (
    <section className="px-5 py-6 lg:grid lg:grid-cols-[minmax(0,1.08fr)_minmax(360px,0.92fr)] lg:gap-12 lg:px-10 lg:py-12">
      <div className="lg:pt-2">
		<div className="flex size-11 items-center justify-center rounded-lg bg-primary/10 text-primary">
		  <PackageCheckIcon className="size-6" />
		</div>
		<h2 className="mt-5 text-2xl font-semibold">
			  {state.device
					? t("portalExtract.customerMobile.serviceShell.confirmDeviceBeforeStart")
					: t("portalExtract.customerMobile.serviceShell.registerDeviceBeforeStart")}
		</h2>
		<div className="mt-6 divide-y rounded-lg border bg-muted/20 px-4 text-sm">
			  <div className="flex items-center justify-between gap-4 py-3"><span className="text-muted-foreground">{t("portalExtract.customerMobile.common.customerOrg")}</span><span className="truncate font-medium">{state.tenant?.name || t("portalExtract.customerMobile.serviceShell.deviceRecognized")}</span></div>
			  <div className="flex items-center justify-between gap-4 py-3"><span className="text-muted-foreground">{t("portalExtract.customerMobile.common.device")}</span><span className="truncate font-mono">{state.device?.deviceNo || state.device?.serialNo || t("portalExtract.customerMobile.common.firstRegistration")}</span></div>
			  <div className="flex items-center justify-between gap-4 py-3"><span className="text-muted-foreground">{t("portalExtract.customerMobile.common.product")}</span><span className="truncate font-medium">{state.product?.name || state.product?.code || t("portalExtract.customerMobile.serviceShell.deviceRecognized")}</span></div>
		</div>
	  </div>

	  <div className="mt-7 border-t pt-6 lg:mt-0 lg:self-start lg:rounded-lg lg:border lg:bg-card lg:p-6">
			<h3 className="text-base font-semibold">{state.device ? t("portalExtract.customerMobile.serviceShell.confirmDevice") : t("portalExtract.customerMobile.serviceShell.registerDevice")}</h3>
		{!state.device ? <div className="mt-5 space-y-2">
			  <Label htmlFor="customer-entry-device-no">{t("portalExtract.customerMobile.common.deviceOrSerial")}</Label>
          <Input
            id="customer-entry-device-no"
            value={deviceNo}
            disabled={busy}
	            placeholder="INV-00031"
            onChange={(event) => onDeviceNoChange(event.target.value)}
		  />
		</div> : null}
        {error ? (
          <p className="mt-3 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error}
          </p>
        ) : null}
        <Button
          type="button"
          className="mt-5 w-full"
		  disabled={busy || (!state.device && !deviceNo?.trim())}
          onClick={onRegisterDevice}
        >
          {busy ? <Loader2Icon className="animate-spin" /> : <SmartphoneIcon />}
			  {state.device
				? authenticated ? t("portalExtract.customerMobile.serviceShell.confirmAndStart") : t("portalExtract.customerMobile.serviceShell.loginToConfirm")
				: authenticated ? t("portalExtract.customerMobile.serviceShell.registerAndStart") : t("portalExtract.customerMobile.serviceShell.loginToRegister")}
        </Button>
      </div>
    </section>
  )
}

// ---- Device Info Panel ----

function DeviceInfoPanel({
  state,
  onNavigate,
}: {
  state: Extract<CustomerEntryState, { kind: "guestSessionReady" }>
  onNavigate?: (state: string, extraParams?: Record<string, string>) => void
}) {
  const t = useI18n()
  return (
    <section className="space-y-4 px-5 py-4">
      <div className="rounded-lg border bg-card p-4">
        <div className="flex items-center gap-3">
          <div className="flex size-12 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <InfoIcon className="size-6" />
          </div>
	          <div>
		            <h2 className="font-semibold">{t("portalExtract.customerMobile.serviceShell.deviceRecognized")}</h2>
	          </div>
        </div>
        <div className="mt-4 space-y-3 text-sm">
          <InfoRow
	            label={t("portalExtract.customerMobile.common.deviceNo")}
            value={state.device?.deviceNo || "-"}
          />
          <InfoRow
	            label={t("portalExtract.customerMobile.common.serialNo")}
            value={state.device?.serialNo || "-"}
          />
          <InfoRow
	            label={t("portalExtract.customerMobile.common.product")}
            value={
              state.product?.name || state.product?.code || "-"
            }
          />
          <InfoRow
	            label={t("portalExtract.customerMobile.common.region")}
            value={state.device?.regionCode || "-"}
          />
        </div>
        <Button
          type="button"
          className="mt-5 w-full"
          onClick={() => onNavigate?.("chat")}
        >
          <MessageCircleIcon className="size-4" />
	          {t("portalExtract.customerMobile.serviceShell.startConsult")}
        </Button>
      </div>
    </section>
  )
}

// ---- Chat Panel ----

function ChatPanel({
  state,
  sessionStarting,
  error,
  onNavigate,
  serviceCode,
  customerContext,
  contextLoading,
  contextError,
  onContextRetry,
}: {
  state: Extract<CustomerEntryState, { kind: "guestSessionReady" }>
  sessionStarting?: boolean
  error?: string
  onNavigate?: (state: string, extraParams?: Record<string, string>) => void
  serviceCode?: string
  customerContext?: CustomerEntryContext | null
  contextLoading?: boolean
  contextError?: string
  onContextRetry?: () => void
}) {
  const t = useI18n()
  if (sessionStarting && !state.session) {
    return (
      <section className="flex min-h-[calc(100svh-248px)] flex-col px-5 py-4">
        <div className="space-y-3">
          <div className="rounded-lg border bg-card px-3 py-2.5">
            <div className="flex items-center justify-between gap-3">
              <div className="min-w-0">
                <h2 className="text-sm font-semibold">{t("portalExtract.customerMobile.serviceShell.serviceSession")}</h2>
                <p className="mt-0.5 truncate text-xs text-muted-foreground">
                  {state.product?.name || state.product?.code || t("portalExtract.customerMobile.status.processing")}
                </p>
              </div>
              <Badge variant="outline">{t("portalExtract.customerMobile.status.processing")}</Badge>
            </div>
          </div>
          <ModuleLoading label={t("portalExtract.customerMobile.serviceShell.sessionLoading")} variant="list" count={4} />
        </div>
      </section>
    )
  }

	const latestConversation = customerContext?.conversations[0]
	const linkedTicket = latestConversation
		? customerContext?.tickets.find(
			(ticket) => ticket.conversationId === latestConversation.id
		)
		: undefined
	const featuredTicket = linkedTicket ?? customerContext?.tickets[0]
	const showingRecentTicket = Boolean(featuredTicket && !linkedTicket)
	const serviceConversationSummary = latestConversation?.endedAt
			? featuredTicket?.status === "cancelled"
			? t("portalExtract.customerMobile.status.cancelled")
			: t("portalExtract.customerMobile.common.ended")
		: latestConversation?.summary
	const activeMeeting = customerContext?.meetings.find(
		(meeting) => meeting.status === "active"
	)
  const entryChat = (state.session as { chat?: CustomerEntryChat } | undefined)
    ?.chat

  return (
    <section className="flex min-h-[calc(100svh-248px)] flex-col px-5 py-4">
      <div className="flex-1 space-y-3">
        <div className="rounded-lg border bg-card px-3 py-2.5">
          <div className="flex items-center justify-between gap-3">
            <div className="min-w-0">
              <h2 className="text-sm font-semibold">{t("portalExtract.customerMobile.serviceShell.serviceSession")}</h2>
              <p className="mt-0.5 truncate text-xs text-muted-foreground">
                {serviceConversationSummary ||
                  state.product?.name ||
                  state.product?.code ||
                  t("portalExtract.customerMobile.serviceShell.serviceContext")}
              </p>
            </div>
            <div className="flex shrink-0 items-center gap-1.5">
              <Badge variant="secondary">
                {t("portalExtract.customerMobile.serviceShell.ticketCount", { count: customerContext?.tickets.length ?? 0 })}
              </Badge>
              <Badge variant={contextLoading ? "outline" : "secondary"}>
                {contextLoading
                  ? t("portalExtract.customerMobile.serviceShell.sync")
                  : t("portalExtract.customerMobile.serviceShell.knowledgeCount", { count: customerContext?.knowledgeEntries.length ?? 0 })}
              </Badge>
            </div>
          </div>

			{featuredTicket ? (
				<button
					type="button"
					className="mt-2 flex w-full items-center justify-between gap-3 border-t pt-2 text-left text-xs"
					onClick={() =>
						onNavigate?.("ticketDetail", { id: String(featuredTicket.id) })
					}
				>
					<span className="min-w-0">
						<span className="block truncate">
							{showingRecentTicket
								? t("portalExtract.customerMobile.serviceShell.recentTicket")
								: t("portalExtract.customerMobile.serviceShell.currentTicket")}
							<span className="font-mono">{featuredTicket.ticketNo}</span>
							</span>
							<span className="mt-0.5 block truncate text-muted-foreground">
								{renderMobileTicketTitle(featuredTicket.title, t)}
							</span>
					</span>
					<Badge variant={ticketStatusVariant(featuredTicket.status)}>
						{ticketStatusLabel(featuredTicket.status, t)}
					</Badge>
				</button>
          ) : null}

          {contextError ? (
            <div className="mt-3 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
              <p>{contextError}</p>
              {onContextRetry ? (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="mt-2"
                  onClick={onContextRetry}
                >
                  <RefreshCwIcon className="size-3" />
                  {t("portalExtract.customerMobile.common.retry")}
                </Button>
              ) : null}
            </div>
          ) : null}
        </div>

		{activeMeeting && onNavigate ? (
			<div className="flex items-center justify-between gap-3 rounded-md border border-border bg-muted px-3 py-2.5 text-foreground">
				<div className="flex min-w-0 items-center gap-2.5">
					<span className="flex size-8 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary">
						<VideoIcon className="size-4" />
					</span>
					<div className="min-w-0">
						<p className="text-sm font-medium">{t("portalExtract.customerMobile.serviceShell.meetingStarted")}</p>
						<p className="truncate text-xs text-muted-foreground">
							{getCustomerMeetingTitle(activeMeeting.title, t)} · {t("portalExtract.customerMobile.status.processing")}
						</p>
					</div>
				</div>
				<Button
					type="button"
					size="sm"
					className="shrink-0"
					onClick={() => onNavigate("ticketVideo")}
				>
					{t("portalExtract.customerMobile.serviceShell.enterVideo")}
				</Button>
			</div>
		) : null}

        {entryChat?.entrySessionId && entryChat.visitorToken ? (
          <CustomerEntryChatPanel
            chat={entryChat}
            onContextChanged={onContextRetry}
          />
        ) : (
          <p className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {t("portalExtract.customerMobile.serviceShell.sessionMissing")}
          </p>
        )}

        {onNavigate && serviceCode && (
          <div className="mt-3 grid grid-cols-2 gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="h-auto py-2 text-xs"
              onClick={() => onNavigate("diagnosis")}
            >
              <ClipboardListIcon className="mr-1 size-3" />
              {t("portalExtract.serviceCode.diagnosis.start")}
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="h-auto py-2 text-xs"
              onClick={() => onNavigate("guide")}
            >
              <WrenchIcon className="mr-1 size-3" />
              {t("portalExtract.customerMobile.serviceShell.guide")}
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="h-auto py-2 text-xs"
              onClick={() => onNavigate("history")}
            >
              <HistoryIcon className="mr-1 size-3" />
              {t("portalExtract.serviceCode.historyList.title")}
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="h-auto py-2 text-xs"
              onClick={() => onNavigate("ticketVideo")}
            >
              <VideoIcon className="mr-1 size-3" />
								{activeMeeting
									? t("portalExtract.customerMobile.serviceShell.enterVideo")
									: t("portalExtract.customerMobile.serviceShell.videoCollaboration")}
            </Button>
          </div>
        )}
      </div>

      {error ? (
        <p className="mt-3 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </p>
      ) : null}
    </section>
  )
}

// ---- Tickets Panel ----

function TicketsPanel({
  onNavigate,
  customerContext,
  contextLoading,
  contextError,
  onContextRetry,
}: {
  onNavigate?: (state: string, extraParams?: Record<string, string>) => void
  customerContext?: CustomerEntryContext | null
  contextLoading?: boolean
  contextError?: string
  onContextRetry?: () => void
}) {
  const t = useI18n()
  const tickets = customerContext?.tickets ?? []
  const repairs = customerContext?.repairHistory ?? []

  return (
    <section className="space-y-3 px-5 py-4">
      {contextError ? (
        <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive">
          <p>{contextError}</p>
          {onContextRetry ? (
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="mt-3"
              onClick={onContextRetry}
            >
              <RefreshCwIcon className="size-3" />
              {t("portalExtract.customerMobile.common.retry")}
            </Button>
          ) : null}
        </div>
      ) : null}

      {contextLoading ? (
        <ModuleLoading label={t("portalExtract.customerMobile.serviceShell.ticketsLoading")} variant="list" count={3} />
      ) : null}

      {tickets.length === 0 && !contextLoading ? (
        <div className="rounded-lg border bg-card p-6 text-center">
          <ClipboardListIcon className="mx-auto size-9 text-muted-foreground" />
          <h2 className="mt-3 font-semibold">{t("portalExtract.customerMobile.serviceShell.noTickets")}</h2>
        </div>
      ) : (
        tickets.map((ticket) => (
          <button
            key={ticket.id}
            type="button"
            className="block w-full rounded-lg border bg-card p-4 text-left"
            onClick={() =>
              onNavigate?.("ticketDetail", { id: String(ticket.id) })
            }
          >
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <h2 className="truncate font-semibold">{renderMobileTicketTitle(ticket.title, t)}</h2>
                <p className="mt-1 font-mono text-xs text-muted-foreground">
                  {ticket.ticketNo}
                </p>
              </div>
              <Badge variant={ticketStatusVariant(ticket.status)}>
                {ticketStatusLabel(ticket.status, t)}
              </Badge>
            </div>
            <div className="mt-3 flex items-center gap-3 text-xs text-muted-foreground">
              <span>{ticket.deviceNo || "-"}</span>
              <span>{formatShortDate(ticket.createdAt)}</span>
              <span>{ticket.priority}</span>
            </div>
          </button>
        ))
      )}

      {repairs.length > 0 ? (
        <div className="rounded-lg border bg-card p-4">
          <HistoryIcon className="size-5 text-muted-foreground" />
          <h3 className="mt-3 font-medium">{t("portalExtract.customerMobile.serviceShell.recentRepair")}</h3>
          <p className="mt-1 text-sm leading-6 text-muted-foreground">
            {repairs[0].summary || repairs[0].solution || repairs[0].rootCause}
          </p>
          {onNavigate ? (
            <Button
              type="button"
              variant="outline"
              className="mt-3 w-full"
              onClick={() => onNavigate("history")}
            >
              <HistoryIcon className="size-4" />
              {t("portalExtract.customerMobile.serviceShell.actionHistory")}
            </Button>
          ) : null}
        </div>
      ) : null}
    </section>
  )
}

function TicketDetailPanel({
	ticket,
	chat,
	loading,
	onChanged,
}: {
	ticket?: CustomerEntryTicket
	chat?: CustomerEntryChat | null
	loading?: boolean
	onChanged?: () => void
}) {
	const t = useI18n()
	const [rating, setRating] = useState(0)
	const [comment, setComment] = useState("")
	const [reopenReason, setReopenReason] = useState("")
	const [submitting, setSubmitting] = useState(false)
	const [actionError, setActionError] = useState("")
	const showResolutionActions = ticket?.canRate || ticket?.canConfirm
	const showReopenReason = Boolean(ticket?.canReopen)

	if (loading) {
		return (
			<ModuleLoading label={t("portalExtract.customerMobile.serviceShell.ticketDetailLoading")} variant="detail" count={2} />
		)
	}

  if (!ticket) {
    return (
      <div className="rounded-lg border bg-card p-6 text-center">
        <ClipboardListIcon className="mx-auto size-9 text-muted-foreground" />
        <h2 className="mt-3 font-semibold">{t("portalExtract.customerMobile.serviceShell.ticketNotFound")}</h2>
      </div>
    )
  }

	async function handleConfirm() {
		if (!ticket || !chat || submitting) return
		if (ticket.canRate && rating === 0) {
			setActionError(t("portalExtract.customerMobile.ticket.selectRatingFirst"))
			return
		}
		setSubmitting(true)
		setActionError("")
		try {
			if (ticket.canRate) {
				await submitCustomerEntryTicketFeedback(
					chat.entrySessionId,
					chat.visitorId,
					chat.visitorToken,
					ticket.id,
					{ rating, comment }
				)
			}
			if (ticket.canConfirm) {
				await confirmCustomerEntryTicket(
					chat.entrySessionId,
					chat.visitorId,
					chat.visitorToken,
					ticket.id
				)
			}
			onChanged?.()
		} catch (error) {
			setActionError(error instanceof Error ? error.message : t("portalExtract.customerMobile.ticket.submitFailed"))
		} finally {
			setSubmitting(false)
		}
	}

	async function handleReopen() {
		const reason = reopenReason.trim()
		if (!ticket || !chat || submitting || !reason) return
		setSubmitting(true)
		setActionError("")
		try {
			await reopenCustomerEntryTicket(
				chat.entrySessionId,
				chat.visitorId,
				chat.visitorToken,
				ticket.id,
				reason
			)
			onChanged?.()
		} catch (error) {
			setActionError(error instanceof Error ? error.message : t("portalExtract.customerMobile.ticket.reopenFailed"))
		} finally {
			setSubmitting(false)
		}
	}

  return (
    <div className="rounded-lg border bg-card p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="font-semibold">{renderMobileTicketTitle(ticket.title, t)}</h2>
          <p className="mt-1 font-mono text-xs text-muted-foreground">
            {ticket.ticketNo}
          </p>
        </div>
        <Badge variant={ticketStatusVariant(ticket.status)}>
          {ticketStatusLabel(ticket.status, t)}
        </Badge>
      </div>
      <div className="mt-4 space-y-3 text-sm">
        <InfoRow label={t("portalExtract.customerMobile.common.device")} value={ticket.deviceNo || "-"} />
		<InfoRow label={t("portalExtract.customerMobile.ticket.priority")} value={ticketPriorityLabel(ticket.priority, t)} />
        <InfoRow label={t("portalExtract.customerMobile.ticket.createdAt")} value={formatShortDate(ticket.createdAt)} />
      </div>
		{ticket.repairSummary ? (
			<div className="mt-4 rounded-md bg-muted/50 p-3 text-sm">
				<p className="font-medium">{t("portalExtract.customerMobile.ticket.repairSummary")}</p>
				<p className="mt-1 leading-6 text-muted-foreground">{ticket.repairSummary}</p>
			</div>
		) : null}
		{ticket.feedback ? (
			<div className="mt-4 rounded-md border p-3 text-sm">
				<div className="flex items-center justify-between gap-3">
					<span className="text-xs font-medium text-muted-foreground">
						{ticket.canRate ? t("portalExtract.customerMobile.ticket.previousFeedback") : t("portalExtract.customerMobile.ticket.currentFeedback")}
					</span>
					<div className="flex items-center gap-1" aria-label={`${ticket.canRate ? t("portalExtract.customerMobile.ticket.previousFeedback") : t("portalExtract.customerMobile.ticket.currentFeedback")} ${t("portalExtract.customerMobile.common.star", { value: ticket.feedback.rating })}`}>
						{[1, 2, 3, 4, 5].map((value) => (
							<StarIcon key={value} className={cn("size-4", value <= ticket.feedback!.rating ? "fill-amber-400 text-amber-400" : "text-muted-foreground")} />
						))}
					</div>
				</div>
				{ticket.feedback.comment ? <p className="mt-2 text-muted-foreground">{ticket.feedback.comment}</p> : null}
			</div>
		) : null}
			{ticket.canRate || ticket.canConfirm || ticket.canReopen ? (
				<div className="mt-4 space-y-3 border-t pt-4">
					{ticket.canConfirm ? (
						<div className="rounded-md bg-primary/5 px-3 py-2">
							<div className="flex items-center justify-between gap-2">
								<span className="text-sm font-semibold">{t("portalExtract.customerMobile.ticket.endTicketTitle")}</span>
								<span className="rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-medium text-amber-700">
									{t("portalExtract.customerMobile.ticket.endAvailable")}
								</span>
							</div>
							<p className="mt-1 text-xs leading-5 text-muted-foreground">
								{ticket.canRate ? t("portalExtract.customerMobile.ticket.endTicketHintWithFeedback") : t("portalExtract.customerMobile.ticket.endTicketHint")}
							</p>
						</div>
					) : null}
					{ticket.canRate ? (
						<div className="flex items-center justify-between gap-3">
							<span className="text-sm font-medium">{t("portalExtract.customerMobile.ticket.serviceFeedback")}</span>
						<div className="flex items-center gap-1">
							{[1, 2, 3, 4, 5].map((value) => (
								<Button
									key={value}
									type="button"
									variant="ghost"
									size="icon"
									className="size-8"
									aria-label={t("portalExtract.customerMobile.common.star", { value })}
									onClick={() => setRating(value)}
								>
									<StarIcon className={cn("size-5", value <= rating ? "fill-amber-400 text-amber-400" : "text-muted-foreground")} />
								</Button>
							))}
						</div>
					</div>
				) : null}
				{showResolutionActions ? (
					<Input
						value={comment}
						onChange={(event) => setComment(event.target.value)}
						placeholder={t("portalExtract.customerMobile.ticket.feedbackPlaceholder")}
					/>
				) : null}
				{showReopenReason ? (
					<Input
						value={reopenReason}
						onChange={(event) => setReopenReason(event.target.value)}
						placeholder={t("portalExtract.customerMobile.ticket.reopenPlaceholder")}
					/>
				) : null}
				{actionError ? <p className="text-sm text-destructive">{actionError}</p> : null}
				{ticket.canConfirm || ticket.canRate ? (
					<Button
						type="button"
						className="w-full"
						onClick={handleConfirm}
						disabled={!chat || submitting || (ticket.canRate && rating === 0)}
					>
							{submitting ? <Loader2Icon className="animate-spin" /> : <StarIcon />}
							{ticket.canConfirm
								? ticket.canRate
									? t("portalExtract.customerMobile.ticket.rateAndEndTicket")
									: t("portalExtract.customerMobile.ticket.endTicketNow")
								: t("portalExtract.customerMobile.ticket.submitFeedback")}
						</Button>
					) : null}
				{ticket.canReopen ? (
					<Button type="button" variant="outline" className="w-full" onClick={handleReopen} disabled={!chat || submitting || !reopenReason.trim()}>
						<RotateCcwIcon />
						{t("portalExtract.customerMobile.ticket.reopenTicket")}
					</Button>
				) : null}
			</div>
		) : null}
    </div>
  )
}

// ---- Meeting Panel ----

function getCustomerMeetingTitle(title: string, t: I18nT) {
	const value = title.trim()
	return !value || value === "Remote support" ? t("portalExtract.customerMobile.meeting.remoteSupport") : value
}

function getCustomerMeetingStatus(status: string, t: I18nT) {
	return ({
		active: t("portalExtract.customerMobile.status.processing"),
		waiting: t("portalExtract.customerMobile.status.waiting"),
		scheduled: t("portalExtract.customerMobile.status.scheduled"),
		ended: t("portalExtract.customerMobile.status.ended"),
		finished: t("portalExtract.customerMobile.status.ended"),
	} as Record<string, string>)[status] || status
}

function MeetingPanel({
  customerContext,
  contextLoading,
	chat,
}: {
  customerContext?: CustomerEntryContext | null
  contextLoading?: boolean
	chat?: CustomerEntryChat | null
}) {
  const t = useI18n()
  const meetings = customerContext?.meetings ?? []
	const [joiningId, setJoiningId] = useState("")
	const [joinError, setJoinError] = useState("")

	async function handleJoin(meetingId: string) {
		if (!chat || joiningId) return
		const popup = window.open("about:blank", "_blank")
		setJoiningId(meetingId)
		setJoinError("")
		try {
			const config = await fetchCustomerEntryMeetingJoin(
				chat.entrySessionId,
				chat.visitorId,
				chat.visitorToken,
				meetingId
			)
			const base = config.jitsiUrl || config.domain
				if (!base || !config.roomName) throw new Error(t("portalExtract.customerMobile.meeting.joinUrlUnavailable"))
			const token = config.jwt ? `?jwt=${encodeURIComponent(config.jwt)}` : ""
			const url = `${base.replace(/\/$/, "")}/${encodeURIComponent(config.roomName)}${token}`
			if (popup) popup.location.href = url
			else window.open(url, "_blank", "noopener,noreferrer")
		} catch (error) {
			popup?.close()
				setJoinError(error instanceof Error ? error.message : t("portalExtract.customerMobile.meeting.joinFailed"))
		} finally {
			setJoiningId("")
		}
	}

  return (
    <section className="space-y-3 px-5 py-4">
      {contextLoading ? (
	        <ModuleLoading label={t("portalExtract.customerMobile.serviceShell.videoLoading")} variant="list" count={2} />
      ) : null}

      {meetings.length === 0 && !contextLoading ? (
        <div className="rounded-lg border bg-card p-6 text-center">
          <VideoIcon className="mx-auto size-9 text-muted-foreground" />
	          <h2 className="mt-3 font-semibold">{t("portalExtract.customerMobile.serviceShell.noVideo")}</h2>
        </div>
      ) : (
        meetings.map((meeting) => (
          <div key={meeting.id} className="rounded-lg border bg-card p-4">
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <h2 className="truncate font-semibold">{getCustomerMeetingTitle(meeting.title, t)}</h2>
                <p className="mt-1 text-xs text-muted-foreground">
                  {formatShortDate(meeting.startedAt || meeting.scheduledAt)}
                </p>
              </div>
              <Badge
                variant={meeting.status === "active" ? "default" : "secondary"}
              >
                {getCustomerMeetingStatus(meeting.status, t)}
              </Badge>
            </div>
            <Button
              type="button"
              className="mt-4 w-full"
              variant="outline"
				disabled={meeting.status !== "active" || !chat || Boolean(joiningId)}
				onClick={() => void handleJoin(meeting.id)}
            >
				{joiningId === meeting.id ? <Loader2Icon className="animate-spin" /> : <VideoIcon />}
					{meeting.status === "active"
						? t("portalExtract.customerMobile.meeting.join")
						: t("portalExtract.customerMobile.serviceShell.archivedMeeting")}
            </Button>
          </div>
        ))
      )}
		{joinError ? <p className="text-sm text-destructive">{joinError}</p> : null}
    </section>
  )
}

// ---- Device Panel ----

function DevicesPanel({
  state,
  customerContext,
  contextLoading,
}: {
  state: Extract<CustomerEntryState, { kind: "guestSessionReady" }>
  customerContext?: CustomerEntryContext | null
  contextLoading?: boolean
}) {
  const t = useI18n()
  const device = customerContext?.devices[0]

  return (
    <section className="space-y-3 px-5 py-4">
      {contextLoading ? (
        <ModuleLoading label={t("portalExtract.customerMobile.common.deviceArchive")} variant="detail" count={2} />
      ) : null}
      <div className="rounded-lg border bg-card p-4">
        <h2 className="font-semibold">{t("portalExtract.customerMobile.common.deviceArchive")}</h2>
        <div className="mt-4 space-y-3 text-sm">
          <InfoRow
            label={t("portalExtract.customerMobile.common.deviceNo")}
            value={device?.deviceNo || state.device?.deviceNo || "-"}
          />
          <InfoRow
            label={t("portalExtract.customerMobile.common.serialNo")}
            value={device?.serialNo || state.device?.serialNo || "-"}
          />
          <InfoRow
            label={t("portalExtract.customerMobile.common.region")}
            value={device?.regionCode || state.device?.regionCode || "-"}
          />
          <InfoRow
            label={t("portalExtract.customerMobile.common.product")}
            value={
              device?.productName ||
              state.product?.name ||
              state.product?.code ||
              "-"
            }
          />
          <InfoRow
            label={t("portalExtract.customerMobile.ticket.status")}
            value={device?.status || state.device?.status || "-"}
          />
          <InfoRow
            label={t("portalExtract.customerMobile.device.lastService")}
            value={formatShortDate(device?.lastServiceAt)}
          />
        </div>
      </div>
    </section>
  )
}

// ---- My Panel ----

function MyPanel({
  serviceCode,
  customerContext,
  contextLoading,
}: {
  serviceCode: string
  customerContext?: CustomerEntryContext | null
  contextLoading?: boolean
}) {
  const t = useI18n()
  const { locale } = useAppLocale()
  const currentLocale = normalizeSelectableLocale(locale)

  return (
    <section className="space-y-3 px-5 py-4">
      {contextLoading ? (
        <ModuleLoading label={t("portalExtract.customerMobile.common.accountInfoLoading")} variant="list" count={2} />
      ) : null}
      <div className="rounded-lg border bg-card p-4">
        <UserCircleIcon className="size-6 text-primary" />
        <h2 className="mt-3 font-semibold">{t("portalExtract.customerMobile.common.myService")}</h2>
        <div className="mt-4 space-y-3 text-sm">
          <InfoRow
            label={t("portalExtract.customerMobile.common.serviceCode")}
            value={customerContext?.serviceCode || serviceCode || "-"}
          />
          <InfoRow label={t("portalExtract.serviceCode.profile.identity")} value={t("portalExtract.customerMobile.common.customerAccount")} />
          <InfoRow
            label={t("portalExtract.customerMobile.common.entrySession")}
            value={
              customerContext?.entrySessionId
                ? String(customerContext.entrySessionId)
                : "-"
            }
          />
        </div>
      </div>
      <div className="rounded-lg border bg-card p-4">
        <h2 className="font-semibold">{t("account.languageSettings")}</h2>
        <div className="mt-4 flex items-center justify-between gap-3">
          <div className="min-w-0 text-sm">
            <p className="text-muted-foreground">{t("account.locale")}</p>
            <p className="mt-1 font-medium">{t(`locale.${currentLocale}`)}</p>
          </div>
          <MobileLanguageSwitch />
        </div>
      </div>
    </section>
  )
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-4">
      <span className="text-muted-foreground">{label}</span>
      <span className="min-w-0 truncate text-right font-medium">
        {value}
      </span>
    </div>
  )
}

function ReadyContent({
  state,
  activeTab,
  serviceCode,
  sessionStarting,
  error,
  onNavigate,
  customerContext,
  contextLoading,
  contextError,
  onContextRetry,
}: {
  state: Extract<CustomerEntryState, { kind: "guestSessionReady" }>
  activeTab: MobileServiceTab
  serviceCode: string
  sessionStarting?: boolean
  error?: string
  onNavigate?: (state: string, extraParams?: Record<string, string>) => void
  customerContext?: CustomerEntryContext | null
  contextLoading?: boolean
  contextError?: string
  onContextRetry?: () => void
}) {
	const entryChat = state.session
		? (state.session as { chat?: CustomerEntryChat }).chat
		: null
  switch (activeTab) {
    case "tickets":
      return (
        <TicketsPanel
          onNavigate={onNavigate}
          customerContext={customerContext}
          contextLoading={contextLoading}
          contextError={contextError}
          onContextRetry={onContextRetry}
        />
      )
    case "video":
      return (
        <MeetingPanel
          customerContext={customerContext}
          contextLoading={contextLoading}
		  chat={entryChat}
        />
      )
    case "devices":
      return (
        <DevicesPanel
          state={state}
          customerContext={customerContext}
          contextLoading={contextLoading}
        />
      )
    case "my":
      return (
        <MyPanel
          serviceCode={serviceCode}
          customerContext={customerContext}
          contextLoading={contextLoading}
        />
      )
    default:
      return (
        <ChatPanel
          state={state}
          sessionStarting={sessionStarting}
          error={error}
          onNavigate={onNavigate}
          serviceCode={serviceCode}
          customerContext={customerContext}
          contextLoading={contextLoading}
          contextError={contextError}
          onContextRetry={onContextRetry}
        />
      )
  }
}

// ---- Sub-state panels (voice, diagnosis, guide, manual, history, etc.) ----

function SubStateContent({
  currentMobileState,
  detailId,
  subStateLoading,
  subStateError,
  onSubStateRetry,
  customerContext,
  contextLoading,
  contextError,
  onContextRetry,
	entryChat,
}: {
  currentMobileState: string
  detailId?: string
  subStateLoading?: boolean
  subStateError?: string
  onSubStateRetry?: () => void
  customerContext?: CustomerEntryContext | null
  contextLoading?: boolean
  contextError?: string
  onContextRetry?: () => void
	entryChat?: CustomerEntryChat | null
}) {
  const t = useI18n()
  if (subStateLoading) {
    return (
      <div className="flex-1 px-5 py-4">
        <ModuleLoading label={t("portalExtract.customerMobile.serviceShell.contentLoading")} variant="detail" count={2} />
      </div>
    )
  }

  switch (currentMobileState) {
    case "voice":
      return (
        <div className="flex-1">
          <VoiceInput />
        </div>
      )
    case "diagnosis":
      return (
        <div className="flex-1">
          <DiagnosisFlow />
        </div>
      )
    case "guide":
      return (
        <div className="flex-1">
          <GuideViewer steps={guideStepsFromKnowledge(customerContext?.knowledgeEntries, t)} />
        </div>
      )
    case "manual":
      return (
        <div className="flex-1">
          <ManualViewer
            toc={(customerContext?.manualFiles ?? []).map((file) => ({
              id: String(file.id),
              label: file.title || file.filename,
              url: file.url,
            }))}
          />
        </div>
      )
    case "history":
      return (
        <div className="flex-1">
          <HistoryList
            conversations={customerContext?.conversations}
            tickets={customerContext?.tickets}
            devices={customerContext?.devices.map((device) => device.deviceNo)}
            loading={contextLoading}
            error={contextError}
            onRetry={onContextRetry}
          />
        </div>
      )
    case "ticketVideo":
      return (
        <div className="flex-1">
          <MeetingPanel
            customerContext={customerContext}
            contextLoading={contextLoading}
			chat={entryChat}
          />
        </div>
      )
    case "ticketDetail":
      return (
        <div className="flex-1 px-5 py-4">
          <TicketDetailPanel
            ticket={customerContext?.tickets.find(
              (ticket) => String(ticket.id) === detailId
            )}
				chat={entryChat}
				loading={contextLoading}
				onChanged={onContextRetry}
          />
        </div>
      )
    default:
      return (
        <div className="flex flex-col items-center justify-center flex-1 gap-3 px-6 py-12 text-center">
          <div className="rounded-full bg-muted p-4">
            <Loader2Icon className="size-8 text-muted-foreground" />
          </div>
          <p className="text-sm text-muted-foreground">{t("portalExtract.customerMobile.serviceShell.unavailable")}</p>
          {subStateError && (
            <div className="mt-2 flex items-center gap-2 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
              <AlertCircleIcon className="size-4 shrink-0" />
              <span>{subStateError}</span>
            </div>
          )}
          {subStateError && onSubStateRetry && (
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="mt-2"
              onClick={onSubStateRetry}
            >
              <RefreshCwIcon className="size-3" />
              {t("portalExtract.customerMobile.common.retry")}
            </Button>
          )}
        </div>
      )
  }
}

export function MobileServiceShell({
  serviceCode,
  state,
  activeTab,
  loading = false,
  sessionStarting = false,
	registering = false,
	authenticated = false,
  error = "",
  deviceNo = "",
  onDeviceNoChange,
  onRegisterDevice,
  onRetry,
  onTabChange,
  currentMobileState,
  onNavigate,
  detailId,
  subStateLoading = false,
  subStateError = "",
  onSubStateRetry,
  customerContext,
  contextLoading = false,
  contextError = "",
  onContextRetry,
  onPrivacyConsentAgree,
  privacyConsentError = "",
}: MobileServiceShellProps) {
  const t = useI18n()
  const isReady = state?.kind === "guestSessionReady"
  const hasEntryState = Boolean(state)
  const initialLoading = loading && !hasEntryState
  const privacyAccepted = Boolean(
    isReady &&
      (state.session as { privacyConsent?: { accepted?: boolean } } | undefined)
        ?.privacyConsent?.accepted
  )
  const isSubState =
    currentMobileState &&
    ![
      "resolving",
      "device_info",
      "login",
      "privacyConsent",
      "chat",
      "tickets",
      "video",
      "devices",
      "my",
    ].includes(currentMobileState)

  // Show device_info panel for device_info state
  const isDeviceInfoState =
    currentMobileState === "device_info" && isReady

  // For sub-states (voice, diagnosis, guide, ticketDetail, ticketVideo, manual, history),
  // show a simpler layout with back button
  if (isSubState) {
    return (
      <main className="rhd-railops-mobile-service min-h-svh bg-[var(--railops-layout-background)] text-[var(--railops-text)]">
        <div className="mx-auto flex min-h-svh w-full max-w-md flex-col bg-[var(--railops-surface)] shadow-[var(--railops-header-shadow)]">
          <header className="flex items-center gap-2 border-b px-4 py-3">
            <button
              type="button"
              onClick={() => onNavigate?.("chat")}
              className="rounded p-1 hover:bg-muted transition-colors"
              aria-label={t("portalExtract.customerMobile.common.back")}
            >
              <ChevronLeftIcon className="size-5" />
            </button>
            <h1 className="min-w-0 flex-1 truncate text-base font-semibold capitalize">
              {currentMobileState === "voice" && t("portalExtract.customerMobile.subState.voice")}
              {currentMobileState === "diagnosis" && t("portalExtract.serviceCode.diagnosis.start")}
              {currentMobileState === "guide" && t("portalExtract.customerMobile.serviceShell.guide")}
              {currentMobileState === "ticketDetail" && t("portalExtract.customerMobile.subState.ticketDetail")}
              {currentMobileState === "ticketVideo" && t("portalExtract.customerMobile.subState.ticketVideo")}
              {currentMobileState === "manual" && t("portalExtract.serviceCode.manualViewer.title")}
              {currentMobileState === "history" && t("portalExtract.serviceCode.historyList.title")}
            </h1>
            <MobileLanguageSwitch />
          </header>
          <SubStateContent
            currentMobileState={currentMobileState}
            detailId={detailId}
            subStateLoading={subStateLoading}
            subStateError={subStateError}
            onSubStateRetry={onSubStateRetry}
            customerContext={customerContext}
            contextLoading={contextLoading}
            contextError={contextError}
            onContextRetry={onContextRetry}
			entryChat={
				isReady && state.session
					? (state.session as { chat?: CustomerEntryChat }).chat
					: null
			}
          />
        </div>
      </main>
    )
  }

  return (
    <main className="rhd-railops-mobile-service min-h-svh bg-[var(--railops-layout-background)] text-[var(--railops-text)]">
      <div className="mx-auto flex min-h-svh w-full max-w-md flex-col bg-[var(--railops-surface)] shadow-[var(--railops-header-shadow)] lg:max-w-6xl">
        <header className="border-b border-[var(--railops-border-light)] bg-[var(--railops-surface)] px-5 pb-4 pt-[max(16px,env(safe-area-inset-top))] lg:grid lg:grid-cols-[minmax(0,1fr)_420px] lg:items-center lg:gap-10 lg:px-10 lg:py-6">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <p className="text-xs font-medium uppercase tracking-normal text-muted-foreground">
                RemoteHelpDesk
              </p>
              <h1 className="mt-1 truncate text-xl font-semibold">
                {getProductName(state, t)}
              </h1>
            </div>
            <div className="flex shrink-0 items-center gap-2">
              <MobileLanguageSwitch />
              <Badge
                variant={
                  isReady
                    ? "default"
                    : loading
                      ? "secondary"
                      : "outline"
                }
              >
                {isReady
                  ? t("portalExtract.customerMobile.entryStatus.ready")
                  : state?.kind === "accessError"
                    ? t("portalExtract.customerMobile.entryStatus.checkFailed")
                  : state?.kind === "needRegister"
                    ? state.device
                    ? authenticated ? t("portalExtract.customerMobile.entryStatus.pendingConfirm") : t("portalExtract.customerMobile.entryStatus.pendingLogin")
                    : t("portalExtract.customerMobile.entryStatus.pendingRegister")
                  : loading
                    ? t("portalExtract.customerMobile.entryStatus.resolving")
                    : t("portalExtract.customerMobile.entryStatus.pending")}
              </Badge>
            </div>
          </div>
          <div className="mt-4 rounded-lg border bg-muted/40 p-3 lg:mt-0">
            <div className="flex items-center gap-3">
              <div className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
                <HeadphonesIcon className="size-5" />
              </div>
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium">
	                  {getDeviceLabel(state, t)}
                </p>
                <p className="mt-0.5 truncate font-mono text-xs text-muted-foreground">
	                  {serviceCode || t("portalExtract.customerMobile.common.noServiceCode")}
                </p>
              </div>
            </div>
          </div>
        </header>

        <div className="min-h-0 flex-1 overflow-y-auto pb-[calc(76px+env(safe-area-inset-bottom))]">
          {/* Render based on state kind */}
          {initialLoading ? <LoadingState /> : null}

          {!initialLoading && state?.kind === "invalid" ? (
            <InvalidState state={state} error={error} onRetry={onRetry} />
          ) : null}
          {!initialLoading && state?.kind === "revoked" ? (
            <RevokedState state={state} onRetry={onRetry} />
          ) : null}
          {!initialLoading && state?.kind === "accessError" ? (
            <AccessErrorState state={state} onRetry={onRetry} />
          ) : null}
          {!initialLoading && state?.kind === "needRegister" ? (
            <RegisterState
              state={state}
              deviceNo={deviceNo}
              registering={registering}
              authenticated={authenticated}
              sessionStarting={sessionStarting}
              error={error}
              onDeviceNoChange={onDeviceNoChange}
              onRegisterDevice={onRegisterDevice}
            />
          ) : null}

          {/* Device info state */}
          {!initialLoading && isDeviceInfoState ? (
            <DeviceInfoPanel state={state} onNavigate={onNavigate} />
          ) : null}

          {/* Privacy consent */}
          {!initialLoading && currentMobileState === "privacyConsent" ? (
            <div className="flex-1">
              <PrivacyConsent
                onAgree={onPrivacyConsentAgree}
                error={privacyConsentError}
              />
            </div>
          ) : null}

          {/* Ready content (tabs) */}
          {!initialLoading &&
          state?.kind === "guestSessionReady" &&
          !isDeviceInfoState &&
          currentMobileState !== "privacyConsent" ? (
            <ReadyContent
              state={state}
              activeTab={activeTab}
              serviceCode={serviceCode}
              sessionStarting={sessionStarting}
              error={error}
              onNavigate={onNavigate}
              customerContext={customerContext}
              contextLoading={contextLoading}
              contextError={contextError}
              onContextRetry={onContextRetry}
            />
          ) : null}
        </div>

        <nav className={cn("fixed inset-x-0 bottom-0 z-20 mx-auto w-full max-w-md border-t bg-background/95 px-2 pb-[max(8px,env(safe-area-inset-bottom))] pt-2 backdrop-blur", !isReady && "lg:hidden")}>
          <div className="grid grid-cols-5 gap-1">
            {tabs.map((tab) => {
              const Icon = tab.icon
              const selected = activeTab === tab.value
              return (
                <button
                  key={tab.value}
                  type="button"
                  disabled={!isReady || !privacyAccepted}
                  className={cn(
                    "flex h-14 flex-col items-center justify-center gap-1 rounded-lg text-xs font-medium text-muted-foreground transition-colors",
                    selected && "bg-primary/10 text-primary",
                    isReady &&
                      privacyAccepted &&
                      !selected &&
                      "hover:bg-muted hover:text-foreground",
                    (!isReady || !privacyAccepted) && "opacity-50"
                  )}
                  onClick={() => onTabChange(tab.value)}
                >
                  <Icon className="size-5" />
	                  <span>{t(tab.labelKey)}</span>
                </button>
              )
            })}
          </div>
        </nav>
      </div>
    </main>
  )
}
