"use client"

import { translateCurrentMessage } from "@/i18n/messages"
import {
  CalendarPlusIcon,
  CaptionsIcon,
  CheckIcon,
  CircleCheckIcon,
  CircleDashedIcon,
  Clock3Icon,
  CopyIcon,
  EyeIcon,
  FileTextIcon,
  ImageIcon,
  LanguagesIcon,
  Loader2Icon,
  RefreshCwIcon,
  ScanLineIcon,
  TicketCheckIcon,
  UsersIcon,
  VideoIcon,
} from "lucide-react"
import { Button as AntButton, Input, Skeleton } from "antd"
import type { TableColumnsType } from "antd"
import Link from "next/link"
import { useRouter, useSearchParams } from "next/navigation"
import { useCallback, useEffect, useMemo, useState, type KeyboardEvent } from "react"
import { toast } from "sonner"
import {
  ActiveFilterTags,
  DataTable,
  DEFAULT_PAGE_SIZE,
  DetailDrawer,
  FilterTabs,
  PageShell,
  RailopsButton,
  SearchField,
  StandardModal,
  StatusTag,
  TableToolbar,
  UnderlineTabs,
} from "@railops/ui"
import type { RailopsTabItem, StatusTagTone, TableFilterField, TableFilterValues } from "@railops/ui"

import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import { buildEnterpriseMeetingRoomPath } from "@/components/meeting/meeting-room-context"
import { MeetingParticipantList } from "@/components/meeting/meeting-participant-list"
import { EmptyState, ErrorState } from "@/components/shared/error-states"
import { fetchTickets } from "@/lib/api/enterprise-tickets"
import {
  createMeeting,
  endMeeting,
  fetchMeetingAnnotations,
  fetchMeetingTranscripts,
  fetchPersonalMeetings,
} from "@/lib/api/meetings"
import type {
  MeetingARAnnotation,
  MeetingListItem,
  MeetingListResponse,
  MeetingTranscriptSegment,
  TicketListItem,
} from "@/lib/api/types"
import { readSession } from "@/lib/auth"
import { publishMeetingSync, subscribeMeetingSync } from "@/lib/meeting-sync"
import { buildEnterpriseTicketWorkbenchPath } from "@/lib/ticket-workbench-route"
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


type MeetingFilter = "all" | "waiting" | "active" | "finished"
type MeetingCreateMode = "now" | "scheduled"

type MeetingTranscriptView = MeetingTranscriptSegment

const MEETING_PAGE_SIZE = DEFAULT_PAGE_SIZE

function isSystemMeetingTranscript(item: MeetingTranscriptSegment) {
  return item.provider?.toLowerCase() === "system" || item.ingestSource?.toLowerCase() === "system"
}
type DeviceRouteFilter = {
  id: number
  label: string
}

const FILTERS: Array<{ value: MeetingFilter; label: string }> = [
  { value: "all", label: ee("meetingCenter.text001") },
  { value: "waiting", label: ee("meetingCenter.text002") },
  { value: "active", label: ee("meetingCenter.text003") },
  { value: "finished", label: ee("meetingCenter.text004") },
]

function parseMeetingRouteFilter(value: string | null): MeetingFilter {
  return FILTERS.some((item) => item.value === value) ? value as MeetingFilter : "all"
}

const MEETING_READY_TICKET_STATUSES = new Set([
  "accepted",
  "in_progress",
  "processing",
  "video_support",
  "supplier_support",
])

function parseDeviceRouteFilter(searchParams: Pick<URLSearchParams, "get">): DeviceRouteFilter | null {
  const id = Number(searchParams.get("device_id") || searchParams.get("deviceId") || 0)
  if (!Number.isSafeInteger(id) || id <= 0) {
    return null
  }
  const deviceNo = (searchParams.get("device_no") || searchParams.get("deviceNo") || "").trim()
  return { id, label: deviceNo || ee("meetingCenter.text005", { value0: id }) }
}

export function EnterpriseMeetingCenter() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const deviceFilter = useMemo(() => parseDeviceRouteFilter(searchParams), [searchParams])
  const deviceFilterId = deviceFilter?.id ?? 0
  const deviceFilterFields: TableFilterField[] = useMemo(() => [{ key: "device", label: ee("meetingCenter.text006") }], [])
  const deviceFilterValues: TableFilterValues = deviceFilter ? { device: deviceFilter.label } : {}
  const [data, setData] = useState<MeetingListResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [filter, setFilter] = useState<MeetingFilter>(() => parseMeetingRouteFilter(searchParams.get("status")))
  const [query, setQuery] = useState("")
  const [debouncedQuery, setDebouncedQuery] = useState("")
  const [meetingPage, setMeetingPage] = useState(1)
  const [selectedId, setSelectedId] = useState("")
  const [endingId, setEndingId] = useState("")
  const [copiedId, setCopiedId] = useState("")
  const [createOpen, setCreateOpen] = useState(false)
  const [ticketLoading, setTicketLoading] = useState(false)
  const [ticketError, setTicketError] = useState("")
  const [ticketQuery, setTicketQuery] = useState("")
  const [tickets, setTickets] = useState<TicketListItem[]>([])
  const [selectedTicketId, setSelectedTicketId] = useState(0)
  const [creatingMeeting, setCreatingMeeting] = useState(false)
  const [createMode, setCreateMode] = useState<MeetingCreateMode>("now")
  const [scheduledAt, setScheduledAt] = useState(defaultMeetingScheduleLocalValue)
  const [minimumScheduledAt, setMinimumScheduledAt] = useState(defaultMeetingMinimumLocalValue)
  const [queryMeetingId, setQueryMeetingId] = useState("")
  const [meetingsLoaded, setMeetingsLoaded] = useState(false)

  const load = useCallback(async (page = 1) => {
    setLoading(true)
    setError("")
    try {
      const result = await fetchPersonalMeetings(filter, page, MEETING_PAGE_SIZE, debouncedQuery, deviceFilterId || undefined)
      if (result.success && result.data) {
        setData(result.data)
        setMeetingsLoaded(true)
      } else {
        const message = result.error?.message || ee("meetingCenter.text007")
        setError(message)
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : ee("meetingCenter.text007")
      setError(message)
    } finally {
      setLoading(false)
    }
  }, [debouncedQuery, deviceFilterId, filter])

  useEffect(() => {
    const timer = window.setTimeout(() => setDebouncedQuery(query.trim()), 300)
    return () => window.clearTimeout(timer)
  }, [query])

  useEffect(() => {
    setMeetingPage(1)
    const timer = window.setTimeout(() => void load(1), 0)
    return () => window.clearTimeout(timer)
  }, [load])

  useEffect(() => {
    const refresh = () => void load(meetingPage)
    const handleVisibility = () => {
      if (document.visibilityState === "visible") refresh()
    }
    window.addEventListener("focus", refresh)
    document.addEventListener("visibilitychange", handleVisibility)
    const unsubscribe = subscribeMeetingSync(refresh)
    return () => {
      window.removeEventListener("focus", refresh)
      document.removeEventListener("visibilitychange", handleVisibility)
      unsubscribe()
    }
  }, [load, meetingPage])

  useEffect(() => {
    const syncQueryMeetingId = () => {
      const params = new URLSearchParams(window.location.search)
      setQueryMeetingId(params.get("meeting_id") || params.get("meetingId") || "")
    }
    syncQueryMeetingId()
    window.addEventListener("popstate", syncQueryMeetingId)
    return () => window.removeEventListener("popstate", syncQueryMeetingId)
  }, [])

  const meetings = useMemo(() => data?.items ?? [], [data])
	const counts = useMemo(() => ({
		waiting: Number(data?.summary.waiting ?? meetings.filter((item) => meetingStage(item) === "waiting").length),
		active: Number(data?.summary.active ?? meetings.filter((item) => meetingStage(item) === "active").length),
		finished: Number(data?.summary.ended ?? meetings.filter((item) => meetingStage(item) === "finished").length),
		participantsOnline: Number(data?.summary.participants_online || 0),
		total: Number(data?.summary.mine || data?.total || meetings.length),
	}), [data, meetings])
  const filterTabs = useMemo(() => FILTERS.map((item) => ({
    ...item,
    count: meetingFilterCount(item.value, counts),
  })), [counts])

  const filteredMeetings = meetings
  const hasMeetings = filteredMeetings.length > 0
  const meetingCenterInitialLoading = loading && !meetingsLoaded
  const meetingCenterRefreshing = loading && meetingsLoaded
  const meetingCenterBlockingError = Boolean(error) && !loading && !hasMeetings
  const meetingCenterRefreshError = Boolean(error) && !loading && hasMeetings

  const selectedMeeting = filteredMeetings.find((item) => item.id === selectedId) ?? null
  const selectedMeetingID = selectedMeeting?.id ?? ""
  const selectedMeetingStatus = selectedMeeting?.status ?? ""

  useEffect(() => {
    if (!selectedMeetingID || selectedMeetingStatus === "ended" || selectedMeetingStatus === "finished") {
      return
    }
    const timer = window.setInterval(() => void load(meetingPage), 3_000)
    return () => window.clearInterval(timer)
  }, [load, meetingPage, selectedMeetingID, selectedMeetingStatus])

  useEffect(() => {
    if (!queryMeetingId || selectedId === queryMeetingId) return
    if (!meetings.some((item) => item.id === queryMeetingId)) return
    setSelectedId(queryMeetingId)
  }, [meetings, queryMeetingId, selectedId])

  const selectableTickets = useMemo(() => {
    const keyword = ticketQuery.trim().toLocaleLowerCase()
    return tickets.filter((ticket) => {
      if (!MEETING_READY_TICKET_STATUSES.has(ticket.status)) return false
      if (!keyword) return true
      return [ticket.ticket_no, ticket.title, ticket.device_no, ticket.customer_name, ticket.product_name]
        .some((value) => value?.toLocaleLowerCase().includes(keyword))
    })
  }, [ticketQuery, tickets])
  const visibleSelectedTicketId = selectableTickets.some((ticket) => ticket.id === selectedTicketId)
    ? selectedTicketId
    : selectableTickets[0]?.id ?? 0

  const loadAssignableTickets = useCallback(async (keyword = "", replace = false) => {
    const userId = readSession()?.user?.id
    if (!userId) {
      setTicketError(ee("meetingCenter.text008"))
      setTicketLoading(false)
      return
    }
    if (replace) setTicketLoading(true)
    setTicketError("")
    const result = await fetchTickets({
      page: 1,
      page_size: 100,
      sort: "-updated_at",
      assignee_id: userId,
      ...(keyword.trim() ? { search: keyword.trim() } : {}),
    })
    if (result.success && result.data) {
      const assigned = result.data.items.filter((ticket) => ticket.assignee_id === userId)
      setTickets((current) => {
        if (replace) return assigned
        return Array.from(new Map([...current, ...assigned].map((ticket) => [ticket.id, ticket])).values())
      })
      if (replace) {
        const firstReady = assigned.find((ticket) => MEETING_READY_TICKET_STATUSES.has(ticket.status))
        setSelectedTicketId(firstReady?.id ?? 0)
      }
    } else if (replace) {
      setTickets([])
      setTicketError(result.error?.message || ee("meetingCenter.text009"))
    }
    if (replace) setTicketLoading(false)
  }, [])

  const openCreateMeeting = useCallback(async () => {
    setCreateOpen(true)
    setTicketLoading(true)
    setTicketError("")
    setTicketQuery("")
    setSelectedTicketId(0)
    setCreateMode("now")
    setScheduledAt(defaultMeetingScheduleLocalValue())
    setMinimumScheduledAt(defaultMeetingMinimumLocalValue())
    await loadAssignableTickets("", true)
  }, [loadAssignableTickets])

  useEffect(() => {
    const keyword = ticketQuery.trim()
    if (!createOpen || !keyword) return
    const timer = window.setTimeout(() => void loadAssignableTickets(keyword), 300)
    return () => window.clearTimeout(timer)
  }, [createOpen, loadAssignableTickets, ticketQuery])

  const selectMeeting = useCallback((meetingId: string) => {
    setSelectedId(meetingId)
  }, [])

  const createAndJoinMeeting = useCallback(async () => {
	const ticket = tickets.find((item) => item.id === visibleSelectedTicketId)
    if (!ticket || creatingMeeting) return
    const isScheduled = createMode === "scheduled"
    const scheduledAtISO = isScheduled ? localDatetimeValueToISOString(scheduledAt) : ""
    if (isScheduled && !scheduledAtISO) {
      toast.error(ee("meetingCenter.text010"))
      return
    }
    if (isScheduled && new Date(scheduledAtISO).getTime() <= Date.now() + 60_000) {
      toast.error(ee("meetingCenter.text011"))
      return
    }
    const popup = isScheduled ? null : window.open("about:blank", "_blank")
    setCreatingMeeting(true)
    try {
      const result = await createMeeting({
        ticket_id: ticket.id,
        title: ticket.title,
        scheduled_at: scheduledAtISO || undefined,
      })
      if (!result.success || !result.data) {
        popup?.close()
        toast.error(result.error?.message || ee("meetingCenter.text012"))
        return
      }
      const meetingId = result.data.meetingId || result.data.meeting_id
      if (!meetingId) {
        popup?.close()
        toast.error(ee("meetingCenter.text013"))
        return
      }
      setCreateOpen(false)
      setSelectedId(String(meetingId))
      setMeetingPage(1)
      publishMeetingSync({ type: "meeting-updated", meetingId: String(meetingId), status: isScheduled ? "scheduled" : "waiting" })
      await load(1)
      if (isScheduled) {
        toast.success(ee("meetingCenter.text014"))
        return
      }
      const roomPath = buildEnterpriseMeetingRoomPath(meetingId, ticket.id)
      if (popup) {
        popup.opener = null
        popup.location.href = roomPath
      } else {
        window.location.assign(roomPath)
      }
    } finally {
      setCreatingMeeting(false)
    }
	}, [createMode, creatingMeeting, load, scheduledAt, tickets, visibleSelectedTicketId])

  const joinMeeting = useCallback((meeting: MeetingListItem) => {
    window.open(
      buildEnterpriseMeetingRoomPath(meeting.id, meeting.ticket_id),
      "_blank",
      "noopener,noreferrer",
    )
  }, [])

  const copyInvite = useCallback(async (meeting: MeetingListItem) => {
    try {
      await navigator.clipboard.writeText(
        `${window.location.origin}/customer/meeting?meetingId=${encodeURIComponent(meeting.id)}`,
      )
      setCopiedId(meeting.id)
		toast.success(ee("meetingCenter.text015"))
      window.setTimeout(() => setCopiedId(""), 1800)
    } catch {
      toast.error(ee("meetingCenter.text016"))
    }
  }, [])

  const closeMeeting = useCallback(async (meeting: MeetingListItem) => {
    if (endingId) return
    const action = meetingStage(meeting) === "waiting" ? ee("meetingCenter.text017") : ee("meetingCenter.text018")
    if (!window.confirm(ee("meetingCenter.text019", { value0: action }))) return
    setEndingId(meeting.id)
    const result = await endMeeting(meeting.id)
    if (result.success) {
      toast.success(ee("meetingCenter.text020"))
		publishMeetingSync({ type: "meeting-ended", meetingId: meeting.id, status: "ended" })
      setMeetingPage(1)
					await load(1)
    } else {
      toast.error(result.error?.message || ee("meetingCenter.text021"))
    }
    setEndingId("")
  }, [endingId, load])

  const clearDeviceFilter = useCallback(() => {
    router.replace("/enterprise/video", { scroll: false })
  }, [router])

  return (
    <PageShell
      title={ee("meetingCenter.text022")}
      breadcrumb={useRouteBreadcrumbItems()}
      className="rhd-railops-video-page"
      actions={(
		  <>
			<RailopsButton onClick={() => void load(meetingPage)} disabled={loading}>
              <RefreshCwIcon className={cn("size-4", loading && "animate-spin")} />{ee("meetingCenter.text023")}</RailopsButton>
            <RailopsButton variant="primary" onClick={() => void openCreateMeeting()}>
              <CalendarPlusIcon className="size-4" />{ee("meetingCenter.text024")}</RailopsButton>
          </>
      )}
    >

      <section className="grid overflow-hidden rounded-md border border-border bg-card sm:grid-cols-2 xl:grid-cols-4">
        <MeetingMetric icon={CircleDashedIcon} label={ee("meetingCenter.text002")} value={counts.waiting} meta={ee("meetingCenter.text025")} />
        <MeetingMetric icon={VideoIcon} label={ee("meetingCenter.text003")} value={counts.active} meta={ee("meetingCenter.text026")} tone="primary" />
			<MeetingMetric icon={CircleCheckIcon} label={ee("meetingCenter.text004")} value={Number(data?.summary.ended ?? counts.finished)} meta={ee("meetingCenter.text027")} />
		<MeetingMetric icon={UsersIcon} label={ee("meetingCenter.text028")} value={counts.participantsOnline} meta={ee("meetingCenter.text029")} />
      </section>

      <section className="overflow-hidden rounded-md border border-border bg-card p-4 shadow-sm">
        <TableToolbar
          tabs={(
            <FilterTabs
              ariaLabel={ee("meetingCenter.text030")}
              items={filterTabs}
              value={filter}
              onChange={(value) => {
                setFilter(value as MeetingFilter)
                setSelectedId("")
                setMeetingPage(1)
              }}
            />
          )}
          search={(
            <SearchField
              allowClear
              aria-label={ee("meetingCenter.text031")}
              className="rhd-railops-search-compact"
              placeholder={ee("meetingCenter.text032")}
              value={query}
              onChange={(event) => {
                setQuery(event.target.value)
                setSelectedId("")
                setMeetingPage(1)
              }}
            />
          )}
          activeFilters={deviceFilter ? (
            <ActiveFilterTags
              fields={deviceFilterFields}
              value={deviceFilterValues}
              onChange={(next) => {
                if (!next.device) clearDeviceFilter()
              }}
            />
          ) : undefined}
        />

        {meetingCenterRefreshing ? (
          <div className="border-b border-border bg-muted/40 px-4 py-2 text-xs text-muted-foreground" role="status" aria-busy="true">{ee("meetingCenter.text033")}</div>
        ) : null}
        {meetingCenterRefreshError ? (
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border bg-muted/40 px-4 py-2 text-sm text-muted-foreground">
            <span>{ee("meetingCenter.text034")}</span>
            <RailopsButton
              size="small"
              onClick={() => {
                setMeetingPage(1)
                void load(1)
              }}
              disabled={loading}
            >{ee("meetingCenter.text035")}</RailopsButton>
          </div>
        ) : null}
        {meetingCenterInitialLoading ? (
          <MeetingCenterSkeleton />
        ) : meetingCenterBlockingError ? (
          <ErrorState
            title={ee("meetingCenter.text034")}
            description={error}
            action={{
              label: ee("meetingCenter.text035"),
              onClick: () => {
                setMeetingPage(1)
                void load(1)
              },
            }}
            className="m-4"
          />
			) : filteredMeetings.length === 0 ? (
				<EmptyState
						title={query ? ee("meetingCenter.text036") : deviceFilter ? ee("meetingCenter.text037") : ee("meetingCenter.text038")}
					action={
						!query && !deviceFilter ? { label: ee("meetingCenter.text024"), onClick: openCreateMeeting } : undefined
					}
            className="m-4"
          />
        ) : (
          <div
            className="grid min-h-[610px] grid-cols-[minmax(0,1fr)]"
            aria-busy={meetingCenterRefreshing}
          >
            <MeetingTable
              meetings={filteredMeetings}
              selectedId={selectedMeeting?.id ?? ""}
              onSelect={selectMeeting}
              total={Number(data?.total ?? data?.summary.mine ?? meetings.length)}
              current={Number(data?.page || meetingPage)}
              onPageChange={(nextPage) => {
                setMeetingPage(nextPage)
                setSelectedId("")
                void load(nextPage)
              }}
            />
          </div>
        )}
      </section>

      <DetailDrawer
        destroyOnHidden
        open={Boolean(selectedMeeting)}
        rootClassName="rhd-railops-video-detail-drawer-root"
        title={(
          <div className="rhd-railops-video-detail-drawer-title">
            <span>{selectedMeeting?.title || selectedMeeting?.room_name || ee("meetingCenter.text039")}</span>
            {selectedMeeting ? <MeetingStatus status={meetingStage(selectedMeeting)} rawStatus={selectedMeeting.status} compact /> : null}
          </div>
        )}
        onClose={() => setSelectedId("")}
      >
        {selectedMeeting ? (
          <MeetingDetail
            key={selectedMeeting.id}
            meeting={selectedMeeting}
            copied={copiedId === selectedMeeting.id}
            ending={endingId === selectedMeeting.id}
            onCopyInvite={copyInvite}
            onEnd={closeMeeting}
            onJoin={joinMeeting}
          />
        ) : null}
      </DetailDrawer>

      <CreateMeetingDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        tickets={selectableTickets}
        totalReady={tickets.filter((ticket) => MEETING_READY_TICKET_STATUSES.has(ticket.status)).length}
        loading={ticketLoading}
        error={ticketError}
        query={ticketQuery}
        onQueryChange={setTicketQuery}
        selectedTicketId={visibleSelectedTicketId}
        onSelectTicket={setSelectedTicketId}
        mode={createMode}
        onModeChange={setCreateMode}
        scheduledAt={scheduledAt}
        minimumScheduledAt={minimumScheduledAt}
        onScheduledAtChange={setScheduledAt}
        creating={creatingMeeting}
        onCreate={createAndJoinMeeting}
      />
    </PageShell>
  )
}

function CreateMeetingDialog({
  open,
  onOpenChange,
  tickets,
  totalReady,
  loading,
  error,
  query,
  onQueryChange,
  selectedTicketId,
  onSelectTicket,
  mode,
  onModeChange,
  scheduledAt,
  minimumScheduledAt,
  onScheduledAtChange,
  creating,
  onCreate,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  tickets: TicketListItem[]
  totalReady: number
  loading: boolean
  error: string
  query: string
  onQueryChange: (value: string) => void
  selectedTicketId: number
  onSelectTicket: (id: number) => void
  mode: MeetingCreateMode
  onModeChange: (mode: MeetingCreateMode) => void
  scheduledAt: string
  minimumScheduledAt: string
  onScheduledAtChange: (value: string) => void
  creating: boolean
  onCreate: () => void | Promise<void>
}) {
  return (
    <StandardModal
      open={open}
      onCancel={() => onOpenChange(false)}
      width={672}
      styles={{
        body: { display: "flex", flexDirection: "column", maxHeight: "min(760px, calc(100vh - 2rem))" },
      }}
      title={ee("meetingCenter.text040")}
      footer={(
        <div className="flex items-center justify-end gap-2">
          <span className="mr-auto self-center text-xs text-muted-foreground">{ee("meetingCenter.text041")}{totalReady}{ee("meetingCenter.text042")}</span>
          <RailopsButton onClick={() => onOpenChange(false)} disabled={creating}>{ee("meetingCenter.text043")}</RailopsButton>
          <RailopsButton variant="primary" onClick={() => void onCreate()} disabled={!selectedTicketId || loading || creating}>
            {creating ? <Loader2Icon className="size-4 animate-spin" /> : mode === "scheduled" ? <CalendarPlusIcon className="size-4" /> : <VideoIcon className="size-4" />}
            {mode === "scheduled" ? ee("meetingCenter.text044") : ee("meetingCenter.text045")}
          </RailopsButton>
        </div>
      )}
    >
        <div className="grid grid-cols-2 gap-2">
          <RailopsButton
            variant={mode === "now" ? "primary" : "default"}
            onClick={() => onModeChange("now")}
          >
            <VideoIcon className="size-4" />{ee("meetingCenter.text046")}</RailopsButton>
          <RailopsButton
            variant={mode === "scheduled" ? "primary" : "default"}
            onClick={() => onModeChange("scheduled")}
          >
            <CalendarPlusIcon className="size-4" />{ee("meetingCenter.text044")}</RailopsButton>
        </div>

        <div>
          <SearchField
            allowClear
            className="rhd-railops-search-compact"
            aria-label={ee("meetingCenter.text047")}
            placeholder={ee("meetingCenter.text048")}
            value={query}
            onChange={(event) => onQueryChange(event.target.value)}
          />
        </div>

        {mode === "scheduled" ? (
          <label className="block space-y-2 text-sm font-medium text-foreground">
            <span>{ee("meetingCenter.text049")}</span>
            <Input
              type="datetime-local"
              value={scheduledAt}
              min={minimumScheduledAt}
              onChange={(event) => onScheduledAtChange(event.target.value)}
            />
          </label>
        ) : null}

        <div className="min-h-0 flex-1 overflow-y-auto border-y border-border">
          {loading ? (
            <div className="space-y-2 py-3">
              {[1, 2, 3].map((item) => (
                <Skeleton.Node key={item} active className="w-full" style={{ width: "100%", height: 80 }} />
              ))}
            </div>
          ) : error ? (
            <div className="px-3 py-8 text-center text-sm text-destructive">{error}</div>
          ) : tickets.length === 0 ? (
            <div className="px-4 py-10 text-center">
              <div className="text-sm font-medium text-foreground">{query ? ee("meetingCenter.text050") : ee("meetingCenter.text051")}</div>
            </div>
          ) : (
            <div className="divide-y divide-border">
              {tickets.map((ticket) => {
                const selected = ticket.id === selectedTicketId
                return (
                  <button
                    key={ticket.id}
                    type="button"
                    onClick={() => onSelectTicket(ticket.id)}
                    aria-pressed={selected}
                    className={cn(
                      "grid w-full grid-cols-[minmax(0,1fr)_auto] gap-3 px-3 py-3 text-left transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring",
                      selected && "bg-muted",
                    )}
                  >
                    <span className="min-w-0">
                      <span className="flex min-w-0 items-center gap-2">
                        <strong className="truncate text-sm text-foreground">{ticket.title}</strong>
                        <StatusTag tone="neutral" className="shrink-0">{ticketPriorityText(ticket.priority)}</StatusTag>
                      </span>
                      <span className="mt-1 flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-1 text-xs text-muted-foreground">
                        <span>{ticket.ticket_no}</span>
                        <span>·</span>
                        <span className="truncate">{ticket.device_no || ticket.product_name || ee("meetingCenter.text052")}</span>
                        <span>·</span>
                        <span className="truncate">{ticket.customer_name || ee("meetingCenter.text053")}</span>
                      </span>
                    </span>
                    <span className="flex items-center gap-2 self-center">
                      <span className="text-xs text-muted-foreground">{ticketStatusText(ticket.status)}</span>
                      <span className={cn(
                        "flex size-5 items-center justify-center rounded-full border",
                        selected ? "border-primary bg-primary text-primary-foreground" : "border-border text-transparent",
                      )}>
                        <CheckIcon className="size-3" />
                      </span>
                    </span>
                  </button>
                )
              })}
            </div>
          )}
        </div>

    </StandardModal>
  )
}

function MeetingMetric({
  icon: Icon,
  label,
  value,
  meta,
  tone = "slate",
}: {
  icon: typeof VideoIcon
  label: string
  value: number
  meta: string
  tone?: "slate" | "primary"
}) {
  return (
    <div className="flex min-h-20 items-center gap-2.5 border-b border-border px-3 py-2.5 last:border-b-0 sm:[&:nth-child(odd)]:border-r xl:border-b-0 xl:border-r xl:last:border-r-0">
      <span className={cn(
        "flex size-8 shrink-0 items-center justify-center rounded-md border",
        tone === "primary"
          ? "border-primary/20 bg-primary/10 text-primary"
          : "border-border bg-muted text-muted-foreground",
      )}>
        <Icon className="size-4" />
      </span>
      <div className="min-w-0">
        <div className="flex items-baseline gap-2">
          <strong className="text-lg font-semibold text-foreground">{value}</strong>
          <span className="text-xs font-medium text-foreground">{label}</span>
        </div>
        <p className="text-xs text-muted-foreground">{meta}</p>
      </div>
    </div>
  )
}

function MeetingTable({
  current,
  meetings,
  onPageChange,
  selectedId,
  onSelect,
  total,
}: {
  current: number
  meetings: MeetingListItem[]
  onPageChange?: (page: number, pageSize: number) => void
  selectedId: string
  onSelect: (id: string) => void
  total: number
}) {
  const columns = useMemo<TableColumnsType<MeetingListItem>>(() => [
    {
      title: ee("meetingCenter.text054"),
      dataIndex: "title",
      width: 300,
      render: (_value, meeting) => (
        <div className="flex min-w-0 items-center gap-2">
          <span className="grid size-7 shrink-0 place-items-center rounded-md bg-primary/10 text-primary">
            <VideoIcon className="size-3.5" />
          </span>
          <span className="min-w-0">
            <strong className="block truncate text-sm font-semibold text-foreground" title={meeting.title || meeting.room_name}>
              {meeting.title || meeting.room_name}
            </strong>
            <span className="mt-0.5 block truncate text-rhd-xs text-muted-foreground" title={meeting.room_name}>{ee("meetingCenter.text055")}{meeting.room_name || "-"}
            </span>
          </span>
        </div>
      ),
    },
    {
      title: ee("meetingCenter.text056"),
      dataIndex: "status",
      width: 96,
      align: "center",
      render: (_value, meeting) => <MeetingStatus status={meetingStage(meeting)} rawStatus={meeting.status} compact />,
    },
    {
      title: ee("meetingCenter.text057"),
      dataIndex: "ticket_no",
      width: 210,
      render: (_value, meeting) => (
        <div className="min-w-0">
          <div className="truncate text-xs font-medium text-foreground" title={meeting.ticket_no || undefined}>
            {meeting.ticket_no || ee("meetingCenter.text058")}
          </div>
          <div className="mt-1 truncate text-rhd-xs text-muted-foreground" title={meeting.device_no || meeting.product_name || undefined}>
            {meeting.device_no || meeting.product_name || ee("meetingCenter.text052")}
          </div>
        </div>
      ),
    },
    {
      title: ee("meetingCenter.text059"),
      dataIndex: "customer_name",
      width: 190,
      render: (_value, meeting) => (
        <div className="min-w-0">
          <div className="truncate text-xs font-medium text-foreground" title={meeting.customer_name || undefined}>
            {meeting.customer_name || ee("meetingCenter.text053")}
          </div>
          <div className="mt-1 truncate text-rhd-xs text-muted-foreground" title={meeting.created_by || undefined}>{ee("meetingCenter.text060")}{meeting.created_by || "-"}
          </div>
        </div>
      ),
    },
    {
      title: ee("meetingCenter.text061"),
      dataIndex: "scheduled_at",
      width: 150,
      align: "right",
      render: (_value, meeting) => (
        <div className="text-right">
          <div className="text-xs font-semibold tabular-nums text-foreground">{meetingListTime(meeting)}</div>
          <div className="mt-1 text-rhd-xs tabular-nums text-muted-foreground">
            {formatDateTime(meeting.scheduled_at || meeting.started_at || meeting.ended_at || meeting.created_at)}
          </div>
        </div>
      ),
    },
    {
      title: ee("meetingCenter.text062"),
      dataIndex: "participant_count",
      width: 76,
      align: "center",
      render: (value: MeetingListItem["participant_count"]) => (
        <span className="text-xs font-semibold tabular-nums text-foreground">{Number(value || 0)}</span>
      ),
    },
    {
      title: ee("meetingCenter.text063"),
      key: "action",
      width: 86,
      align: "center",
      render: (_value, meeting) => (
        <AntButton
          aria-label={ee("meetingCenter.text064")}
          className="rhd-railops-row-action"
          icon={<EyeIcon className="size-4" />}
          size="small"
          title={ee("meetingCenter.text065")}
          type="text"
          onClick={(event) => {
            event.stopPropagation()
            onSelect(meeting.id)
          }}
        />
      ),
    },
  ], [onSelect])

  return (
    <div className="rhd-railops-video-table-pane">
        <DataTable<MeetingListItem>
          className="rhd-railops-video-table"
          columns={columns}
          current={current}
          dataSource={meetings}
          footerNote={ee("meetingCenter.text066", { value0: total })}
          pagination={false}
          rowKey="id"
          scroll={{ x: 1108, y: 590 }}
          size="middle"
          total={total}
          onPageChange={onPageChange}
          rowClassName={(record) => cn("rhd-railops-video-table-row", selectedId === record.id && "is-selected")}
          onRow={(record) => ({
            onClick: () => onSelect(record.id),
            onKeyDown: (event: KeyboardEvent<HTMLTableRowElement>) => {
              if (event.key !== "Enter" && event.key !== " ") return
              event.preventDefault()
              onSelect(record.id)
            },
            role: "button",
            tabIndex: 0,
            "aria-pressed": selectedId === record.id,
          })}
        />
    </div>
  )
}

function MeetingDetail({
  meeting,
  copied,
  ending,
  onCopyInvite,
  onEnd,
  onJoin,
}: {
  meeting: MeetingListItem
  copied: boolean
  ending: boolean
  onCopyInvite: (meeting: MeetingListItem) => void | Promise<void>
  onEnd: (meeting: MeetingListItem) => void | Promise<void>
  onJoin: (meeting: MeetingListItem) => void
}) {
  const stage = meetingStage(meeting)
  return (
    <div className="min-w-0">
      <div className="flex flex-col gap-3 border-b border-border px-4 py-4 sm:px-5 lg:flex-row lg:items-start lg:justify-between">
        <div className="min-w-0">
          <div className="mb-2 flex flex-wrap items-center gap-2">
            <MeetingStatus status={stage} rawStatus={meeting.status} />
            {meeting.ticket_no ? <StatusTag tone="neutral">{meeting.ticket_no}</StatusTag> : null}
          </div>
          <h2 className="break-words text-lg font-semibold text-foreground [overflow-wrap:anywhere]">{meeting.title || meeting.room_name}</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            {meeting.product_name || ee("meetingCenter.text067")} · {meeting.device_no || ee("meetingCenter.text052")} · {meeting.customer_name || ee("meetingCenter.text053")}
          </p>
        </div>
        <div className="flex shrink-0 flex-wrap gap-2">
          {meeting.ticket_id ? (
            <Link href={buildEnterpriseTicketWorkbenchPath(meeting.ticket_id)}>
              <RailopsButton size="small">
                <TicketCheckIcon className="size-4" />{ee("meetingCenter.text068")}</RailopsButton>
            </Link>
          ) : null}
          {stage !== "finished" ? (
            <RailopsButton size="small" onClick={() => void onCopyInvite(meeting)}>
              {copied ? <CheckIcon className="size-4" /> : <CopyIcon className="size-4" />}
			  {copied ? ee("meetingCenter.text069") : ee("meetingCenter.text070")}
            </RailopsButton>
          ) : null}
          {stage !== "finished" ? (
            <RailopsButton size="small" variant="primary" onClick={() => onJoin(meeting)}>
              <VideoIcon className="size-4" />
              {stage === "active" ? ee("meetingCenter.text071") : ee("meetingCenter.text072")}
            </RailopsButton>
          ) : null}
        </div>
      </div>

      {stage === "finished" ? (
		<MeetingReview meeting={meeting} />
      ) : (
        <MeetingPreparation
          meeting={meeting}
          ending={ending}
          onEnd={onEnd}
        />
      )}
    </div>
  )
}

function MeetingPreparation({
  meeting,
  ending,
  onEnd,
}: {
  meeting: MeetingListItem
  ending: boolean
  onEnd: (meeting: MeetingListItem) => void | Promise<void>
}) {
  const isActive = meetingStage(meeting) === "active"
  return (
      <div className="grid gap-6 p-4 sm:p-5 2xl:grid-cols-[minmax(0,1fr)_260px]">
      <div className="min-w-0 space-y-6">
        <section>
          <div className="mb-3 flex items-center justify-between">
              <h3 className="text-sm font-semibold text-foreground">{isActive ? ee("meetingCenter.text073") : ee("meetingCenter.text074")}</h3>
              <span className="text-xs text-muted-foreground">{ee("meetingCenter.text075")}{meeting.created_by || "-"}</span>
          </div>
          <div className="divide-y divide-border border-y border-border">
            <ReadinessRow
              icon={TicketCheckIcon}
              title={ee("meetingCenter.text076")}
              detail={[meeting.ticket_no, meeting.product_name, meeting.device_no].filter(Boolean).join(" · ") || ee("meetingCenter.text077")}
              ready={Boolean(meeting.ticket_no)}
            />
            <ReadinessRow
			  icon={VideoIcon}
			  title={ee("meetingCenter.text078")}
				  detail={isActive ? ee("meetingCenter.text079") : ee("meetingCenter.text080")}
			  ready={isActive}
			/>
			<ReadinessRow
              icon={CaptionsIcon}
              title={ee("meetingCenter.text081")}
              detail={isActive ? ee("meetingCenter.text082") : ee("meetingCenter.text083")}
              ready={isActive}
            />
            <ReadinessRow
              icon={ScanLineIcon}
              title={ee("meetingCenter.text084")}
              detail={isActive ? ee("meetingCenter.text085") : ee("meetingCenter.text086")}
              ready={false}
            />
          </div>
        </section>

        <section>
          <h3 className="mb-3 text-sm font-semibold text-foreground">{ee("meetingCenter.text087")}</h3>
          <dl className="grid gap-x-6 gap-y-4 border-y border-border py-4 sm:grid-cols-2">
            <MeetingFact label={ee("meetingCenter.text088")} value={meeting.room_name} />
            <MeetingFact label={ee("meetingCenter.text089")} value={formatDateTime(meeting.created_at)} />
            <MeetingFact label={ee("meetingCenter.text090")} value={formatDateTime(meeting.started_at)} />
            <MeetingFact label={ee("meetingCenter.text091")} value={ee("meetingCenter.text092", { value0: meeting.participant_count || 0 })} />
          </dl>
        </section>

        <section>
          <div className="mb-3 flex items-center justify-between gap-3">
            <h3 className="text-sm font-semibold text-foreground">{ee("meetingCenter.text093")}</h3>
            <span className="text-xs text-muted-foreground">{meeting.participants?.length ?? 0}{ee("meetingCenter.text094")}</span>
          </div>
          <MeetingParticipantList participants={meeting.participants ?? []} meetingStatus={meeting.status} />
        </section>
      </div>

      <aside className="border-t border-border pt-5 2xl:border-l 2xl:border-t-0 2xl:pl-5 2xl:pt-0">
        <h3 className="text-sm font-semibold text-foreground">{ee("meetingCenter.text095")}</h3>
        <div className="mt-3 space-y-3 text-sm">
          <StatusLine label={ee("meetingCenter.text096")} value={ee("meetingCenter.text097")} good />
		  <StatusLine label={ee("meetingCenter.text098")} value={meeting.customer_name ? ee("meetingCenter.text099") : ee("meetingCenter.text100")} good={Boolean(meeting.customer_name)} />
          <StatusLine label={ee("meetingCenter.text101")} value={isActive ? ee("meetingCenter.text102") : ee("meetingCenter.text103")} good={isActive} />
			  <StatusLine label={ee("meetingCenter.text104")} value={ee("meetingCenter.text105")} />
        </div>
		{meeting.can_end && meeting.status !== "finished" ? (
          <RailopsButton
            danger
            className="mt-6 w-full"
            onClick={() => void onEnd(meeting)}
            disabled={ending}
          >
            {ending ? <Loader2Icon className="size-4 animate-spin" /> : <CircleCheckIcon className="size-4" />}
            {isActive ? ee("meetingCenter.text106") : ee("meetingCenter.text107")}
          </RailopsButton>
        ) : null}
      </aside>
    </div>
  )
}

function MeetingReview({ meeting }: { meeting: MeetingListItem }) {
  const [transcripts, setTranscripts] = useState<MeetingTranscriptSegment[]>([])
  const [annotations, setAnnotations] = useState<MeetingARAnnotation[]>([])
  const [loading, setLoading] = useState(true)
  const [recordLoaded, setRecordLoaded] = useState(false)
  const [error, setError] = useState("")
  const [activeTab, setActiveTab] = useState("summary")

  const load = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const [transcriptResult, annotationResult] = await Promise.all([
        fetchMeetingTranscripts(meeting.id),
        fetchMeetingAnnotations(meeting.id),
      ])
      if (transcriptResult.success) setTranscripts(transcriptResult.data ?? [])
      if (annotationResult.success) setAnnotations(annotationResult.data ?? [])
      if (!transcriptResult.success || !annotationResult.success) {
        setError(transcriptResult.error?.message || annotationResult.error?.message || ee("meetingCenter.text108"))
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : ee("meetingCenter.text108"))
    } finally {
      setRecordLoaded(true)
      setLoading(false)
    }
  }, [meeting.id])

  useEffect(() => {
    const timer = window.setTimeout(() => void load(), 0)
    return () => window.clearTimeout(timer)
  }, [load])

  const recordInitialLoading = loading && !recordLoaded
  const recordRefreshing = loading && recordLoaded

  const recordTabs = useMemo<RailopsTabItem[]>(() => {
    const items: RailopsTabItem[] = [
      { value: "summary", label: ee("meetingCenter.text104"), icon: <FileTextIcon /> },
      { value: "participants", label: ee("meetingCenter.text093"), count: meeting.participants?.length ?? 0, icon: <UsersIcon /> },
      { value: "transcript", label: ee("meetingCenter.text101"), count: transcripts.filter((item) => !isSystemMeetingTranscript(item)).length, icon: <FileTextIcon /> },
    ]
    if (annotations.length > 0) {
      items.push({ value: "annotations", label: ee("meetingCenter.text109"), count: annotations.length, icon: <ScanLineIcon /> })
    }
    return items
  }, [meeting.participants?.length, transcripts, annotations.length])

  return (
    <div className="min-w-0">
      <div className="px-4 sm:px-5">
        <UnderlineTabs ariaLabel={ee("meetingCenter.text110")} items={recordTabs} value={activeTab} onChange={setActiveTab} />
      </div>

      {recordRefreshing ? (
        <div className="border-b border-border bg-muted/40 px-4 py-2 text-xs text-muted-foreground sm:px-5" role="status" aria-busy="true">{ee("meetingCenter.text033")}</div>
      ) : null}
      {recordInitialLoading ? (
        <div className="space-y-3 p-5" role="status" aria-busy="true">
          <Skeleton.Node active style={{ width: 144, height: 20 }} />
          <Skeleton.Node active className="w-full" style={{ width: "100%", height: 80 }} />
          <Skeleton.Node active className="w-full" style={{ width: "100%", height: 112 }} />
        </div>
      ) : (
        <>
          {error ? (
            <ErrorState title={ee("meetingCenter.text111")} description={error} action={{ label: ee("meetingCenter.text035"), onClick: load }} className="m-5" />
          ) : null}
          {activeTab === "summary" ? (
            <div className="p-4 sm:p-5">
              <MeetingSummary
                meeting={meeting}
                transcripts={transcripts}
                annotations={annotations}
              />
            </div>
          ) : null}
          {activeTab === "participants" ? (
            <div className="p-4 sm:p-5">
              <div className="mb-3 flex items-center justify-between gap-3">
                <h3 className="text-sm font-semibold text-foreground">{ee("meetingCenter.text112")}</h3>
                <span className="text-xs text-muted-foreground">{ee("meetingCenter.text113")}</span>
              </div>
              <MeetingParticipantList participants={meeting.participants ?? []} meetingStatus={meeting.status} />
            </div>
          ) : null}
          {activeTab === "transcript" ? (
            <TranscriptList transcripts={transcripts} meeting={meeting} />
          ) : null}
          {activeTab === "annotations" ? (
            <AnnotationList annotations={annotations} />
          ) : null}
        </>
      )}
    </div>
  )
}

function MeetingSummary({
  meeting,
  transcripts,
  annotations,
}: {
  meeting: MeetingListItem
  transcripts: MeetingTranscriptView[]
  annotations: MeetingARAnnotation[]
}) {
  const realtimeTranscripts = transcripts.filter((item) => !isSystemMeetingTranscript(item))
  const systemArchives = transcripts.filter(isSystemMeetingTranscript)
  const speakers = new Set(realtimeTranscripts.map((item) => item.speakerName).filter(Boolean)).size
	const subject = meeting.title || meeting.ticket_no || ee("meetingCenter.text114")
  const transcriptSummary = realtimeTranscripts.length > 0
    ? ee("meetingCenter.text115", { value0: realtimeTranscripts.length })
    : systemArchives.length > 0
      ? ee("meetingCenter.text116")
      : ee("meetingCenter.text117")
  return (
    <div className="space-y-6">
      <section>
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h3 className="text-sm font-semibold text-foreground">{ee("meetingCenter.text104")}</h3>
          <StatusTag tone="neutral">
			{realtimeTranscripts.length ? ee("meetingCenter.text118") : systemArchives.length ? ee("meetingCenter.text119") : ee("meetingCenter.text120")}
          </StatusTag>
        </div>
        <p className="mt-3 border-l-2 border-primary pl-4 text-sm leading-6 text-foreground">
			  {ee("meetingCenter.text121", { value0: subject, value1: durationText(meeting.duration_seconds), value2: meeting.participant_count || 0, value3: transcriptSummary, value4: annotations.length })}
        </p>
      </section>

      <dl className="grid gap-4 border-y border-border py-4 sm:grid-cols-2 xl:grid-cols-4">
	        <MeetingFact label={ee("meetingCenter.text122")} value={durationText(meeting.duration_seconds)} />
        <MeetingFact label={ee("meetingCenter.text091")} value={ee("meetingCenter.text092", { value0: meeting.participant_count || 0 })} />
        <MeetingFact label={ee("meetingCenter.text123")} value={ee("meetingCenter.text092", { value0: speakers })} />
        <MeetingFact label={ee("meetingCenter.text124")} value={formatDateTime(meeting.ended_at)} />
      </dl>

      <section>
        <h3 className="text-sm font-semibold text-foreground">{ee("meetingCenter.text125")}</h3>
		<div className="mt-3 grid gap-3 sm:grid-cols-3">
          <ReviewStatus
            icon={CaptionsIcon}
            label={ee("meetingCenter.text126")}
            value={realtimeTranscripts.length ? ee("meetingCenter.text127", { value0: realtimeTranscripts.length }) : systemArchives.length ? ee("meetingCenter.text128") : ee("meetingCenter.text129")}
            ready={realtimeTranscripts.length > 0}
		  />
		  <ReviewStatus
			icon={LanguagesIcon}
			label={ee("meetingCenter.text130")}
			value={translationArchiveLabel(realtimeTranscripts)}
			ready={realtimeTranscripts.some((item) => item.translationStatus === "completed")}
		  />
          <ReviewStatus
            icon={ImageIcon}
            label={ee("meetingCenter.text131")}
            value={annotations.length ? ee("meetingCenter.text127", { value0: annotations.length }) : ee("meetingCenter.text132")}
            ready={annotations.length > 0}
          />
        </div>
      </section>
    </div>
  )
}

function TranscriptList({
  transcripts,
  meeting,
}: {
  transcripts: MeetingTranscriptView[]
  meeting: MeetingListItem
}) {
  const realtimeTranscripts = transcripts.filter((item) => !isSystemMeetingTranscript(item))
  const systemArchives = transcripts.filter(isSystemMeetingTranscript)
  if (realtimeTranscripts.length === 0) {
    return (
      <div className="m-5">
        <EmptyState
          title={ee("meetingCenter.text117")}
        />
        {systemArchives[0]?.text ? (
          <p className="mt-4 rounded-md border border-border bg-muted px-4 py-3 text-sm leading-6 text-muted-foreground">
            {systemArchives[0].text}
          </p>
        ) : null}
      </div>
    )
  }
  return (
    <div className="max-h-[560px] overflow-y-auto">
      <div className="divide-y divide-border">
      {realtimeTranscripts.map((item) => (
        <article key={item.id} className="grid gap-2 px-4 py-3.5 sm:grid-cols-[88px_minmax(0,1fr)] sm:px-5">
          <div className="text-xs text-muted-foreground">
            <div className="font-medium text-foreground">{item.speakerName || ee("meetingCenter.text133")}</div>
            <div className="mt-1 tabular-nums">{transcriptTime(item.startedAtMs, meeting.started_at)}</div>
            <div className="mt-1">{languageLabel(item.language)}</div>
          </div>
          <div className="min-w-0">
            <p className="text-sm leading-6 text-foreground">{item.text}</p>
            {item.translatedText ? (
            <div className="mt-2 flex items-start gap-2 border-l-2 border-primary/20 bg-primary/10 px-3 py-2">
                <LanguagesIcon className="mt-0.5 size-4 shrink-0 text-primary" />
                <div className="min-w-0">
                  <div className="text-rhd-xs font-medium text-primary">{ee("meetingCenter.text134")}{languageLabel(item.translatedLanguage)}</div>
                  <p className="mt-0.5 text-sm leading-6 text-foreground">{item.translatedText}</p>
                </div>
              </div>
			) : null}
			{item.translationStatus === "pending" ? <div className="mt-1.5 text-rhd-xs text-amber-600">{ee("meetingCenter.text135")}</div> : null}
			{item.translationStatus === "failed" ? <div className="mt-1.5 text-rhd-xs text-destructive">{ee("meetingCenter.text136")}</div> : null}
            {item.confidence > 0 ? <div className="mt-1.5 text-rhd-xs text-muted-foreground">{ee("meetingCenter.text137")}{Math.round(item.confidence * 100)}%</div> : null}
          </div>
        </article>
      ))}
      </div>
    </div>
  )
}

function AnnotationList({ annotations }: { annotations: MeetingARAnnotation[] }) {
  if (annotations.length === 0) {
    return (
      <EmptyState
        title={ee("meetingCenter.text138")}
        className="m-5"
      />
    )
  }
  return (
    <div className="max-h-[590px] overflow-y-auto">
	  <RealARFrames annotations={annotations} />
      <div className="divide-y divide-border border-t border-border">
      {annotations.map((annotation) => (
        <article key={annotation.id} className="flex items-start gap-3 px-4 py-3.5 sm:px-5">
          <span className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-md border border-border bg-muted text-muted-foreground">
            <ScanLineIcon className="size-4" />
          </span>
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <strong className="text-sm text-foreground">{annotation.label || ee("meetingCenter.text139")}</strong>
              {annotation.partCode ? <StatusTag tone="neutral">{annotation.partCode}</StatusTag> : null}
              {annotation.confidence > 0 ? <span className="text-xs text-muted-foreground">{ee("meetingCenter.text140")}{Math.round(annotation.confidence * 100)}%</span> : null}
            </div>
            {annotation.note ? <p className="mt-1 text-sm text-muted-foreground">{annotation.note}</p> : null}
            <p className="mt-1 text-xs text-muted-foreground">{ee("meetingCenter.text141")}{annotation.frameAssetId} · {formatDateTime(annotation.createdAt)}</p>
          </div>
        </article>
      ))}
      </div>
    </div>
  )
}

function RealARFrames({ annotations }: { annotations: MeetingARAnnotation[] }) {
  const frames = Array.from(
    annotations.reduce((result, annotation) => {
      if (annotation.frameUrl && !result.has(annotation.frameAssetId)) {
        result.set(annotation.frameAssetId, annotation.frameUrl)
      }
      return result
    }, new Map<number, string>()),
  )
  if (frames.length === 0) return null

  return (
    <div className="space-y-3 bg-card p-3 sm:p-4">
      {frames.map(([frameAssetId, frameUrl]) => {
        const frameAnnotations = annotations.filter((annotation) => annotation.frameAssetId === frameAssetId)
        return (
          <figure key={frameAssetId} className="mx-auto max-w-3xl">
            <div className="relative aspect-video overflow-hidden border border-border bg-black">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img src={frameUrl} alt={ee("meetingCenter.text142", { value0: frameAssetId })} className="size-full object-contain" loading="lazy" />
              {frameAnnotations.map((annotation) => (
                <div
                  key={annotation.id}
                  className="absolute border-2"
                  style={{
                    borderColor: annotation.color,
                    height: `${annotation.bounds.height * 100}%`,
                    left: `${annotation.bounds.x * 100}%`,
                    top: `${annotation.bounds.y * 100}%`,
                    width: `${annotation.bounds.width * 100}%`,
                  }}
                >
                  <span
                    className="absolute left-0 top-0 max-w-[180px] truncate px-1.5 py-0.5 text-rhd-2xs font-semibold text-primary-foreground"
                    style={{ backgroundColor: annotation.color }}
                  >
                    {annotation.label}
                  </span>
                </div>
              ))}
            </div>
            <figcaption className="mt-1.5 text-rhd-xs text-muted-foreground">{ee("meetingCenter.text143")}{frameAssetId} · {frameAnnotations.length}{ee("meetingCenter.text144")}</figcaption>
          </figure>
        )
      })}
    </div>
  )
}

function ReadinessRow({
  icon: Icon,
  title,
  detail,
  ready,
}: {
  icon: typeof VideoIcon
  title: string
  detail: string
  ready: boolean
}) {
  return (
    <div className="grid gap-2 py-3 sm:grid-cols-[32px_120px_minmax(0,1fr)_72px] sm:items-center">
      <span className="flex size-8 items-center justify-center rounded-md bg-muted text-muted-foreground"><Icon className="size-4" /></span>
      <span className="text-sm font-medium text-foreground">{title}</span>
      <span className="text-sm text-muted-foreground">{detail}</span>
      <span className={cn("flex items-center gap-1 text-xs", ready ? "text-primary" : "text-muted-foreground")}>
        {ready ? <CircleCheckIcon className="size-3.5" /> : <Clock3Icon className="size-3.5" />}
        {ready ? ee("meetingCenter.text097") : ee("meetingCenter.text145")}
      </span>
    </div>
  )
}

function ReviewStatus({
  icon: Icon,
  label,
  value,
  ready,
}: {
  icon: typeof VideoIcon
  label: string
  value: string
  ready: boolean
}) {
  return (
    <div className="flex items-center gap-3 rounded-md border border-border p-3">
      <span className={cn(
        "flex size-8 shrink-0 items-center justify-center rounded-md",
        ready ? "bg-primary/10 text-primary" : "bg-muted text-muted-foreground",
      )}>
        <Icon className="size-4" />
      </span>
      <div className="min-w-0">
        <div className="text-sm font-medium text-foreground">{label}</div>
        <div className="truncate text-xs text-muted-foreground">{value}</div>
      </div>
    </div>
  )
}

function MeetingFact({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className="mt-1 truncate text-sm font-medium text-foreground" title={value}>{value || "-"}</dd>
    </div>
  )
}

function StatusLine({ label, value, good = false }: { label: string; value: string; good?: boolean }) {
  return (
    <div className="flex items-center justify-between gap-3 border-b border-border pb-3 last:border-b-0">
      <span className="text-muted-foreground">{label}</span>
      <span className={cn("flex items-center gap-1 text-xs font-medium", good ? "text-primary" : "text-muted-foreground")}>
        {good ? <CircleCheckIcon className="size-3.5" /> : <Clock3Icon className="size-3.5" />}
        {value}
      </span>
    </div>
  )
}

function MeetingStatus({
  status,
  rawStatus,
  compact = false,
}: {
  status: Exclude<MeetingFilter, "all">
  rawStatus?: MeetingListItem["status"]
  compact?: boolean
}) {
  let tone: StatusTagTone = "warning"
  let label = ee("meetingCenter.text002")
  if (rawStatus === "scheduled") {
    tone = "blue"
    label = ee("meetingCenter.text146")
  } else if (status === "active") {
    tone = "blue"
    label = ee("meetingCenter.text003")
  } else if (status === "finished") {
    tone = "neutral"
    label = ee("meetingCenter.text004")
  }
  return <StatusTag tone={tone} className={cn("shrink-0", compact && "rhd-railops-video-status-compact")}>{label}</StatusTag>
}

function MeetingCenterSkeleton() {
  return (
    <div className="rhd-railops-video-table-pane min-h-[610px]">
      <div className="flex items-center justify-between border-b border-border px-4 py-3">
        <Skeleton.Node active style={{ width: 80, height: 20 }} />
        <Skeleton.Node active style={{ width: 112, height: 16 }} />
      </div>
      <div className="space-y-0">
        {[1, 2, 3, 4, 5, 6].map((item) => (
          <div key={item} className="grid grid-cols-[2fr_0.7fr_1.4fr_1.2fr_0.9fr_0.5fr_0.5fr] gap-4 border-b border-border px-4 py-4">
            {[36, 24, 32, 32, 32, 24, 24].map((height, col) => (
              <Skeleton.Node key={col} active className="w-full" style={{ width: "100%", height }} />
            ))}
          </div>
        ))}
      </div>
    </div>
  )
}

function meetingFilterCount(
  filter: MeetingFilter,
	counts: { waiting: number; active: number; finished: number; total: number },
) {
	return filter === "all" ? counts.total : counts[filter]
}

function meetingListTime(meeting: MeetingListItem) {
  const stage = meetingStage(meeting)
  if (stage === "finished") return formatShortDate(meeting.ended_at || meeting.started_at || meeting.created_at)
  if (stage === "active") return durationText(meeting.duration_seconds)
  return formatShortDate(meeting.scheduled_at || meeting.created_at)
}

function ticketStatusText(status: string) {
  switch (status) {
    case "accepted": return ee("meetingCenter.text147")
    case "in_progress":
    case "processing": return ee("meetingCenter.text148")
    case "video_support": return ee("meetingCenter.text149")
    case "supplier_support": return ee("meetingCenter.text150")
    default: return status
  }
}

function ticketPriorityText(priority: TicketListItem["priority"]) {
  switch (priority) {
    case "critical": return "P0"
    case "high": return "P1"
    case "medium": return "P2"
    case "low": return "P3"
  }
}

function meetingStage(meeting: MeetingListItem): Exclude<MeetingFilter, "all"> {
  if (meeting.status === "finished") return "finished"
  if (meeting.status === "waiting" || meeting.status === "scheduled" || Number(meeting.participant_count || 0) === 0) return "waiting"
  return "active"
}

function toMeetingDatetimeLocalValue(date: Date) {
  const localDate = new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
  return localDate.toISOString().slice(0, 16)
}

function defaultMeetingScheduleLocalValue() {
  return toMeetingDatetimeLocalValue(new Date(Date.now() + 30 * 60_000))
}

function defaultMeetingMinimumLocalValue() {
  return toMeetingDatetimeLocalValue(new Date(Date.now() + 60_000))
}

function localDatetimeValueToISOString(value: string) {
  if (!value) return ""
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return ""
  return date.toISOString()
}

function formatDateTime(value?: string) {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date)
}

function formatShortDate(value?: string) {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat("zh-CN", { month: "2-digit", day: "2-digit" }).format(date)
}

function durationText(seconds?: number) {
  const total = Math.max(0, Number(seconds || 0))
	if (total < 1) return ee("meetingCenter.text151")
	if (total < 60) return ee("meetingCenter.text152", { value0: Math.floor(total) })
  const minutes = Math.floor(total / 60)
  if (minutes < 60) return ee("meetingCenter.text153", { value0: minutes })
  const hours = Math.floor(minutes / 60)
  return ee("meetingCenter.text154", { value0: hours, value1: minutes % 60 })
}

function transcriptTime(offsetMs: number, startedAt?: string) {
  if (offsetMs > 0 && offsetMs < 24 * 60 * 60 * 1000) {
    const seconds = Math.floor(offsetMs / 1000)
    return `${String(Math.floor(seconds / 60)).padStart(2, "0")}:${String(seconds % 60).padStart(2, "0")}`
  }
  if (offsetMs > 0) {
    return new Intl.DateTimeFormat("zh-CN", { hour: "2-digit", minute: "2-digit", second: "2-digit" }).format(new Date(offsetMs))
  }
  return formatDateTime(startedAt)
}

function languageLabel(value?: string) {
  switch (value) {
    case "zh-CN":
      return ee("meetingCenter.text155")
    case "en-US":
      return "English"
    default:
      return value || ee("meetingCenter.text156")
  }
}

function translationArchiveLabel(transcripts: MeetingTranscriptView[]) {
  const completed = transcripts.filter((item) => item.translationStatus === "completed").length
  const failed = transcripts.filter((item) => item.translationStatus === "failed").length
  if (completed > 0) return ee("meetingCenter.text157", { value0: completed, value1: failed > 0 ? ee("meetingCenter.text160", { value0: failed }) : "" })
  if (failed > 0) return ee("meetingCenter.text158", { value0: failed })
  if (transcripts.some((item) => item.translationStatus === "pending")) return ee("meetingCenter.text135")
  return ee("meetingCenter.text159")
}
