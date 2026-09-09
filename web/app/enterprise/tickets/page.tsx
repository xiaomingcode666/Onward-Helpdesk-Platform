"use client"

import { translateCurrentMessage } from "@/i18n/messages"
import { useCallback, useEffect, useMemo, useState } from "react"
import Link from "next/link"
import { useRouter, useSearchParams } from "next/navigation"
import { CheckCircle2Icon, CopyIcon, EyeIcon, Loader2Icon, PlusIcon, RefreshCwIcon, UserRoundCheckIcon, XIcon } from "lucide-react"
import { Input, Segmented, type TableColumnsType } from "antd"
import {
  DataTable,
  FilterTabs,
  FormField,
  IconButton,
  PageShell,
  RailopsButton,
  SearchField,
  SelectField,
  StandardModal,
  StatusTag,
  TablePagination,
  type RailopsTabItem,
  type StatusTagTone,
} from "@railops/ui"

import {
  EnterpriseTicketDetailContent,
  sourceLabel,
  ticketStatusLabel,
} from "@/app/enterprise/tickets/_components/ticket-detail-content"
import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import { useAuth } from "@/components/auth-provider"
import { CanUseButton } from "@/components/layout/permission-guard"
import { ModuleLoading } from "@/components/shared/loading-states"
import {
  createTicket,
  createTicketCustomerInvitationDraft,
  fetchTicketAggregate,
  fetchTicketCustomerOptions,
  fetchTickets,
  fetchTicketSummary,
  type TicketCustomerInvitationDraftResult,
  type TicketCustomerOption,
} from "@/lib/api/enterprise-tickets"
import type { TicketAggregateDTO, TicketListItem, TicketPriority } from "@/lib/api/types"
import { customerDisplayName } from "@/lib/customer-identity"
import { isProcessingTicketStatus, isTerminalTicketStatus } from "@/lib/ticket-lifecycle"
import { toast } from "sonner"
import { IntakeFields, IntakePolicyButton, IntakeCompletion, intakeLabel } from "./_components/ticket-intake"
import { EMPTY_INTAKE, phoneIntakePayload, type IntakeDraft, type TicketIntakePolicy } from "@/lib/ticket-intake"
import { fetchTicketIntakePolicy } from "@/lib/api/enterprise-tickets"
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


const PAGE_SIZE = 10

type TicketFilterKey =
  | "all"
  | "pending"
  | "processing"
  | "awaiting_customer"
  | "sla_risk"
  | "urgent"
  | "done"

type TicketSummary = {
  total: number
  pending: number
  processing: number
  awaitingCustomer: number
  slaRisk: number
  urgent: number
  done: number
}

type DeviceRouteFilter = {
  id: number
  label: string
}

const EMPTY_SUMMARY: TicketSummary = {
  total: 0,
  pending: 0,
  processing: 0,
  awaitingCustomer: 0,
  slaRisk: 0,
  urgent: 0,
  done: 0,
}

type TicketCustomerMode = "existing" | "invite"

type TicketDraft = {
	channel: "enterprise" | "phone"
	intake: IntakeDraft
  title: string
  description: string
  priority: TicketPriority
  customerMode: TicketCustomerMode
  customerId: number
  inviteDisplayName: string
  inviteEmail: string
  inviteCustomerOrg: string
}

const EMPTY_TICKET_DRAFT: TicketDraft = {
  channel: "enterprise",
  intake: EMPTY_INTAKE,
  title: "",
  description: "",
  priority: "medium",
  customerMode: "existing",
  customerId: 0,
  inviteDisplayName: "",
  inviteEmail: "",
  inviteCustomerOrg: "",
}

function parseDeviceRouteFilter(searchParams: Pick<URLSearchParams, "get">): DeviceRouteFilter | null {
  const id = Number(searchParams.get("device_id") || searchParams.get("deviceId") || 0)
  if (!Number.isSafeInteger(id) || id <= 0) {
    return null
  }
  const deviceNo = (searchParams.get("device_no") || searchParams.get("deviceNo") || "").trim()
  return { id, label: deviceNo || ee("tickets.text001", { value0: id }) }
}

function parseTicketTime(value?: string) {
  if (!value) {
    return 0
  }
  const timestamp = Date.parse(value.replace(" ", "T"))
  return Number.isFinite(timestamp) ? timestamp : 0
}

function formatTicketTime(value?: string) {
  const timestamp = parseTicketTime(value)
  if (!timestamp) {
    return "-"
  }
  return new Date(timestamp).toLocaleString("zh-CN", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  })
}

function formatSla(deadline: string, breached: boolean, status: string) {
  if (isTerminalTicketStatus(status)) {
    return ee("tickets.text002")
  }
  if (breached) {
    return ee("tickets.text003")
  }
  const target = parseTicketTime(deadline)
  if (!target) {
    return ee("tickets.text004")
  }
  const diff = target - Date.now()
  if (diff <= 0) {
    return ee("tickets.text005")
  }
  const mins = Math.floor(diff / 60000)
  if (mins < 60) {
    return ee("tickets.text006", { value0: mins })
  }
  const hours = Math.floor(mins / 60)
  if (hours < 72) {
    return ee("tickets.text007", { value0: hours })
  }
  return formatTicketTime(deadline)
}

function priorityLabel(priority: string) {
  if (priority === "critical") return ee("tickets.text008")
  if (priority === "high") return ee("tickets.text009")
  if (priority === "medium") return ee("tickets.text010")
  return ee("tickets.text011")
}

function priorityTagTone(priority: string): StatusTagTone {
  if (priority === "critical") return "error"
  if (priority === "high") return "warning"
  if (priority === "medium") return "blue"
  return "neutral"
}

function ticketStatusTagTone(status: string): StatusTagTone {
  if (status === "draft") return "neutral"
  if (isTerminalTicketStatus(status)) return "neutral"
  if (isProcessingTicketStatus(status)) return "success"
  if (status === "pending_customer_confirm" || status === "resolved") return "blue"
  return "warning"
}

function slaTagTone(ticket: TicketListItem): StatusTagTone {
  if (isTerminalTicketStatus(ticket.status)) return "neutral"
  if (ticket.sla_breached || ticket.priority === "critical") return "error"
  return "success"
}

function ticketOwner(ticket: TicketListItem) {
  if (ticket.assignee_name) {
    return ticket.assignee_name
  }
  if (ticket.team_name) {
    return ee("tickets.text012", { value0: ticket.team_name })
  }
  return ee("tickets.text013")
}

function filterToQuery(filter: TicketFilterKey) {
  switch (filter) {
    case "pending":
      return { status: "pending" }
    case "processing":
      return { status: "processing" }
    case "awaiting_customer":
      return { status: "awaiting_customer" }
    case "sla_risk":
      return { sla_risk: true }
    case "urgent":
      return { priority: "critical" }
    case "done":
      return { status: "done" }
    default:
      return {}
  }
}

export default function EnterpriseTicketsPage() {
  const { session } = useAuth()
  const router = useRouter()
  const searchParams = useSearchParams()
  const hasDeviceConcept = session?.featureFlags?.device !== false
  const deviceFilter = useMemo(() => hasDeviceConcept ? parseDeviceRouteFilter(searchParams) : null, [hasDeviceConcept, searchParams])
  const deviceFilterId = deviceFilter?.id ?? 0
  const mineOnly = searchParams.get("mine") === "1" || searchParams.get("mine") === "true"
  const [filter, setFilter] = useState<TicketFilterKey>("all")
  const [search, setSearch] = useState("")
  const [debouncedSearch, setDebouncedSearch] = useState("")
  const [tickets, setTickets] = useState<TicketListItem[]>([])
  const [summary, setSummary] = useState<TicketSummary>(EMPTY_SUMMARY)
  const [summaryLoading, setSummaryLoading] = useState(true)
  const [loading, setLoading] = useState(true)
  const [ticketsLoaded, setTicketsLoaded] = useState(false)
  const [error, setError] = useState("")
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [totalPages, setTotalPages] = useState(1)
  const [detailOpen, setDetailOpen] = useState(false)
  const [detailTicketId, setDetailTicketId] = useState<number | null>(null)
  const [detailData, setDetailData] = useState<TicketAggregateDTO | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailError, setDetailError] = useState("")
  const [createOpen, setCreateOpen] = useState(false)
  const [createSaving, setCreateSaving] = useState(false)
  const [intakePolicy, setIntakePolicy] = useState<TicketIntakePolicy>({ rules: [] })
  const [createKey, setCreateKey] = useState("")
  const [createError, setCreateError] = useState("")
  const [ticketDraft, setTicketDraft] = useState(EMPTY_TICKET_DRAFT)
  const [customerOptions, setCustomerOptions] = useState<TicketCustomerOption[]>([])
  const [customerOptionsLoading, setCustomerOptionsLoading] = useState(false)
  const [customerOptionsError, setCustomerOptionsError] = useState("")
  const [createdInvitation, setCreatedInvitation] = useState<TicketCustomerInvitationDraftResult | null>(null)
  const [invitationLinkCopied, setInvitationLinkCopied] = useState(false)
  const canCreateTicket = CanUseButton("ticket.create", session?.permissions)

  const loadTickets = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const res = await fetchTickets({
        page,
        page_size: PAGE_SIZE,
        sort: "-updated_at",
        search: debouncedSearch || undefined,
        device_id: deviceFilterId || undefined,
        mine: mineOnly || undefined,
        ...filterToQuery(filter),
      })
      if (res.success && res.data) {
        setTickets(res.data.items)
        setTotal(res.data.total)
        setTotalPages(Math.max(1, res.data.total_pages))
        setTicketsLoaded(true)
      } else {
        setError(res.error?.message || ee("tickets.text014"))
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : ee("tickets.text014"))
    } finally {
      setLoading(false)
    }
  }, [page, filter, debouncedSearch, deviceFilterId, mineOnly])

  const loadSummary = useCallback(async () => {
    setSummaryLoading(true)
    try {
      const res = await fetchTicketSummary({
        device_id: deviceFilterId || undefined,
        mine: mineOnly || undefined,
      })
      if (res.success && res.data) {
        setSummary({
          total: res.data.total,
          pending: res.data.pending,
          processing: res.data.processing,
          awaitingCustomer: res.data.awaiting_customer,
          slaRisk: res.data.sla_risk,
          urgent: res.data.urgent,
          done: res.data.done,
        })
      } else {
        setSummary(EMPTY_SUMMARY)
      }
    } catch {
      setSummary(EMPTY_SUMMARY)
    } finally {
      setSummaryLoading(false)
    }
  }, [deviceFilterId, mineOnly])

  useEffect(() => {
    void loadTickets()
  }, [loadTickets])

  useEffect(() => {
    void loadSummary()
  }, [loadSummary])

  useEffect(() => {
    const timer = window.setTimeout(() => setDebouncedSearch(search.trim()), 320)
    return () => window.clearTimeout(timer)
  }, [search])

  useEffect(() => {
    setPage(1)
  }, [filter, debouncedSearch, deviceFilterId, mineOnly])

  const clearDeviceFilter = useCallback(() => {
    const params = new URLSearchParams(searchParams.toString())
    params.delete("device_id")
    params.delete("deviceId")
    params.delete("device_no")
    params.delete("deviceNo")
    const query = params.toString()
    router.replace(query ? `/enterprise/tickets?${query}` : "/enterprise/tickets", { scroll: false })
  }, [router, searchParams])

  const toggleMineOnly = useCallback(() => {
    const params = new URLSearchParams(searchParams.toString())
    if (mineOnly) {
      params.delete("mine")
    } else {
      params.set("mine", "1")
    }
    const query = params.toString()
    router.replace(query ? `/enterprise/tickets?${query}` : "/enterprise/tickets", { scroll: false })
  }, [mineOnly, router, searchParams])

  const openTicketModal = useCallback((ticketId: number) => {
    setDetailTicketId(ticketId)
    setDetailData(null)
    setDetailError("")
    setDetailOpen(true)
  }, [])

  const closeTicketModal = useCallback(() => {
    setDetailOpen(false)
  }, [])

  const closeCreateModal = useCallback(() => {
    if (createSaving) return
    setCreateOpen(false)
    setCreateError("")
    setCustomerOptionsError("")
    setCreatedInvitation(null)
    setInvitationLinkCopied(false)
    setTicketDraft(EMPTY_TICKET_DRAFT)
  }, [createSaving])

  useEffect(() => {
    if (!createOpen) return
    let cancelled = false
    async function loadCustomerOptions() {
      setCustomerOptionsLoading(true)
      setCustomerOptionsError("")
      try {
        const result = await fetchTicketCustomerOptions()
        if (cancelled) return
        if (!result.success) {
          setCustomerOptions([])
          setCustomerOptionsError(result.error?.message || ee("tickets.text098"))
          return
        }
        const items = result.data || []
        setCustomerOptions(items)
        setTicketDraft((current) => ({
          ...current,
          customerMode: items.length === 0 && current.channel !== "phone" ? "invite" : current.customerMode,
        }))
      } catch (error) {
        if (!cancelled) {
          setCustomerOptions([])
          setCustomerOptionsError(error instanceof Error ? error.message : ee("tickets.text098"))
        }
      } finally {
        if (!cancelled) setCustomerOptionsLoading(false)
      }
    }
    void loadCustomerOptions()
    return () => {
      cancelled = true
    }
  }, [createOpen])

  useEffect(() => {
    if (!createOpen) return
    let active = true
    setIntakePolicy({ rules: [] })
    fetchTicketIntakePolicy().then((result) => {
      if (!active) return
      if (result.success && result.data) setIntakePolicy({ rules: result.data.rules ?? [] })
      else setCreateError(result.error?.message || intakeLabel("loadError"))
    }).catch(() => { if (active) setCreateError(intakeLabel("loadError")) })
    return () => { active = false }
  }, [createOpen])

  const submitCreateTicket = useCallback(async () => {
    const title = ticketDraft.title.trim()
    const description = ticketDraft.description.trim()
    if (!title || !description) {
      setCreateError(ee("tickets.text076"))
      return
    }
    if (ticketDraft.channel !== "phone" && ticketDraft.customerMode === "existing" && ticketDraft.customerId <= 0) {
      setCreateError(ee("tickets.text084"))
      return
    }
    if (ticketDraft.customerMode === "invite" && (!ticketDraft.inviteDisplayName.trim() || !ticketDraft.inviteEmail.trim() || !ticketDraft.inviteCustomerOrg.trim())) {
      setCreateError(ee("tickets.text091"))
      return
    }
    setCreateSaving(true)
    setCreateError("")
    try {
      const inviteCustomer = ticketDraft.customerMode === "invite"
      const result = inviteCustomer
        ? await createTicketCustomerInvitationDraft({
            title,
            description,
            priority: ticketDraft.priority,
            displayName: ticketDraft.inviteDisplayName.trim(),
            email: ticketDraft.inviteEmail.trim(),
            customerOrg: ticketDraft.inviteCustomerOrg.trim(),
          })
        : await createTicket({
            source: "manual",
            channel: ticketDraft.channel,
            idempotency_key: createKey,
            ...phoneIntakePayload(ticketDraft.channel, ticketDraft.intake),
            title,
            description,
            priority: ticketDraft.priority,
            ...(ticketDraft.customerId > 0 ? { customer_id: ticketDraft.customerId } : {}),
          })
      if (!result.success || !result.data) {
        setCreateError(result.error?.message || (inviteCustomer ? ee("tickets.text099") : ee("tickets.text075")))
        return
      }
      setPage(1)
      await Promise.all([loadTickets(), loadSummary()])
      if (inviteCustomer) {
        setCreatedInvitation(result.data as TicketCustomerInvitationDraftResult)
        toast.success(ee("tickets.text092"))
        return
      }
      toast.success(ee("tickets.text074"))
      setCreateOpen(false)
      setTicketDraft(EMPTY_TICKET_DRAFT)
    } catch (error) {
      setCreateError(error instanceof Error ? error.message : (ticketDraft.customerMode === "invite" ? ee("tickets.text099") : ee("tickets.text075")))
    } finally {
      setCreateSaving(false)
    }
  }, [loadSummary, loadTickets, ticketDraft, createKey])

  const copyInvitationLink = useCallback(async () => {
    const link = createdInvitation?.invitation.registrationUrl
    if (!link) return
    await navigator.clipboard.writeText(link)
    setInvitationLinkCopied(true)
    toast.success(ee("tickets.text096"))
  }, [createdInvitation])

  useEffect(() => {
    if (!detailOpen || detailTicketId == null) {
      return
    }
    const ticketId = detailTicketId
    let cancelled = false
    async function loadTicketDetail() {
      setDetailLoading(true)
      setDetailError("")
      try {
        const res = await fetchTicketAggregate(ticketId)
        if (cancelled) {
          return
        }
        if (res.success && res.data) {
          setDetailData(res.data)
        } else {
          setDetailData(null)
          setDetailError(res.error?.message || ee("tickets.text015"))
        }
      } catch (err) {
        if (!cancelled) {
          setDetailData(null)
          setDetailError(err instanceof Error ? err.message : ee("tickets.text015"))
        }
      } finally {
        if (!cancelled) {
          setDetailLoading(false)
        }
      }
    }
    void loadTicketDetail()
    return () => {
      cancelled = true
    }
  }, [detailOpen, detailTicketId])

  const filterChips: Array<{ key: TicketFilterKey; label: string; count: number }> = [
    { key: "all", label: ee("tickets.text016"), count: summary.total },
    { key: "pending", label: ee("tickets.text017"), count: summary.pending },
    { key: "processing", label: ee("tickets.text018"), count: summary.processing },
    { key: "awaiting_customer", label: ee("tickets.text019"), count: summary.awaitingCustomer },
    { key: "sla_risk", label: ee("tickets.text020"), count: summary.slaRisk },
    { key: "urgent", label: ee("tickets.text008"), count: summary.urgent },
    { key: "done", label: ee("tickets.text021"), count: summary.done },
  ]

  const detailListTicket = detailTicketId
    ? tickets.find((ticket) => ticket.id === detailTicketId) ?? null
    : null
  const ticketFilterTabs = filterChips.map((chip): RailopsTabItem => ({
    label: chip.label,
    value: chip.key,
    count: chip.count,
  }))
  const ticketColumns = useMemo<TableColumnsType<TicketListItem>>(() => [
    {
      title: ee("tickets.text022"),
      key: "ticketNo",
      width: 154,
      render: (_, ticket) => (
        <button
          type="button"
          className="rhd-railops-ticket-no"
          onClick={(event) => {
            event.stopPropagation()
            openTicketModal(ticket.id)
          }}
        >
          {ticket.ticket_no}
        </button>
      ),
    },
    {
      title: ee("tickets.text023"),
      key: "source",
      width: 112,
      render: (_, ticket) => (
        <span className="rhd-railops-ticket-source">
          {ticket.channel === "phone" ? intakeLabel("phone") : sourceLabel(ticket.source || ticket.channel || "")}
          {ticket.context_status === "context_incomplete" && <span className="block text-xs text-amber-700">{intakeLabel("incomplete")}</span>}
        </span>
      ),
    },
    {
      title: ee("tickets.text024"),
      key: "description",
      width: 220,
      render: (_, ticket) => (
        <div className="rhd-railops-ticket-cell">
          <strong title={ticket.title}>{ticket.title}</strong>
          <span>{ticket.conversation_id ? ee("tickets.text025", { value0: ticket.conversation_id }) : ee("tickets.text026")}</span>
        </div>
      ),
    },
    ...(hasDeviceConcept ? [{
      title: ee("tickets.text027"),
      key: "product",
      width: 150,
      render: (_: unknown, ticket: TicketListItem) => (
        <strong className="rhd-railops-ticket-product" title={ticket.product_name || ee("tickets.text028")}>
          {ticket.product_name || ee("tickets.text028")}
        </strong>
      ),
    }] : []),
    {
      title: hasDeviceConcept ? ee("tickets.text029") : ee("tickets.text077"),
      key: "customer",
      width: 160,
      render: (_, ticket) => (
        <div className="rhd-railops-table-cell">
          <strong>{customerDisplayName(ticket.customer_name)}</strong>
          {hasDeviceConcept ? <span>{ticket.device_no || ee("tickets.text030")}</span> : null}
        </div>
      ),
    },
    {
      title: ee("tickets.text031"),
      key: "status",
      width: 126,
      align: "center",
      render: (_, ticket) => (
        <div className="rhd-railops-centered-cell">
          <StatusTag tone={ticketStatusTagTone(ticket.status)}>
            {ticketStatusLabel(ticket.status)}
          </StatusTag>
          <span>{ticket.dispatch_attempts > 0 ? ee("tickets.text032", { value0: ticket.dispatch_attempts }) : ee("tickets.text033")}</span>
        </div>
      ),
    },
    {
      title: ee("tickets.text034"),
      key: "owner",
      width: 150,
      render: (_, ticket) => (
        <div className="rhd-railops-table-cell">
          <strong>{ticketOwner(ticket)}</strong>
          <span>{ticket.team_name || ee("tickets.text035")}</span>
        </div>
      ),
    },
    {
      title: ee("tickets.text036"),
      key: "risk",
      width: 120,
      align: "center",
      render: (_, ticket) => (
        <div className="rhd-railops-ticket-risk-cell">
          <StatusTag tone={priorityTagTone(ticket.priority)}>{priorityLabel(ticket.priority)}</StatusTag>
          <StatusTag tone={slaTagTone(ticket)}>{formatSla(ticket.sla_deadline, ticket.sla_breached, ticket.status)}</StatusTag>
        </div>
      ),
    },
    {
      title: ee("tickets.text037"),
      key: "updated",
      width: 104,
      align: "center",
      render: (_, ticket) => formatTicketTime(ticket.updated_at || ticket.created_at),
    },
    {
      title: "",
      key: "actions",
      width: 92,
      align: "center",
      render: (_, ticket) => (
        <div className="rhd-railops-ticket-actions">
          <Link
            href={`/enterprise/ticket-workbench?ticket_id=${ticket.id}`}
            className="rhd-railops-ticket-workbench-link"
            onClick={(event) => event.stopPropagation()}
          >{isTerminalTicketStatus(ticket.status) ? ee("tickets.text039") : ee("tickets.text038")}</Link>
          <IconButton
            icon={<EyeIcon className="size-4" />}
            tooltip={ee("tickets.text039")}
            aria-label={ee("tickets.text040")}
            className="rhd-railops-row-action"
            size="small"
            onClick={(event) => {
              event.stopPropagation()
              openTicketModal(ticket.id)
            }}
          />
        </div>
      ),
    },
  ], [hasDeviceConcept, openTicketModal])

  return (
    <PageShell
      title={ee("tickets.text041")}
      breadcrumb={useRouteBreadcrumbItems()}
      className="ticket-guoqi-page ticket-ops-page rhd-railops-ticket-page"
      actions={
        <>
          {canCreateTicket ? (
            <RailopsButton variant="primary" onClick={() => { setCreateKey(crypto.randomUUID()); setCreateOpen(true) }}>
              <PlusIcon className="size-4" />{ee("tickets.text065")}
            </RailopsButton>
          ) : null}
          <RailopsButton
            onClick={() => {
              void loadSummary()
              void loadTickets()
            }}
          >
            <RefreshCwIcon className="size-4" />{ee("tickets.text042")}</RailopsButton>
          <Link href="/enterprise/ticket-workbench">
            <RailopsButton>{ee("tickets.text043")}</RailopsButton>
          </Link>
        </>
      }
    >

      <section className="rhd-railops-ticket-queue-shell" aria-label={ee("tickets.text044")}>
        <div className="rhd-railops-ticket-queue-toolbar">
          <div className="rhd-railops-ticket-filter-row">
            <FilterTabs
              ariaLabel={ee("tickets.text045")}
              items={ticketFilterTabs}
              value={filter}
              onChange={(value) => setFilter(value as TicketFilterKey)}
            />
            <div className="rhd-railops-ticket-context">
              {deviceFilter ? (
                <span className="rhd-railops-ticket-device-filter">{ee("tickets.text046")}{deviceFilter.label}
                  <button type="button" aria-label={ee("tickets.text047", { value0: deviceFilter.label })} onClick={clearDeviceFilter}>
                    <XIcon className="size-3.5" />
                  </button>
                </span>
              ) : null}
              {mineOnly ? <StatusTag tone="blue">{ee("tickets.text048")}</StatusTag> : null}
              {summaryLoading ? (
                <span className="rhd-railops-ticket-meta">
                  <Loader2Icon className="size-3 animate-spin" />{ee("tickets.text049")}</span>
              ) : (
                <span className="rhd-railops-ticket-meta">{ee("tickets.text050")}{summary.total}{ee("tickets.text051")}{summary.urgent} · SLA {summary.slaRisk}
                </span>
              )}
            </div>
          </div>

          <div className="rhd-railops-ticket-tool-row">
            <div className="rhd-railops-ticket-tool-actions">
              <SearchField
                allowClear
                className="rhd-railops-ticket-search"
                placeholder={ee(hasDeviceConcept ? "tickets.text052" : "tickets.text102")}
                value={search}
                onChange={(event) => setSearch(event.target.value)}
              />
              <RailopsButton
                size="small"
                variant={mineOnly ? "primary" : undefined}
                aria-pressed={mineOnly}
                onClick={toggleMineOnly}
              >
                <UserRoundCheckIcon className="size-4" />{ee("tickets.text048")}</RailopsButton>
              <RailopsButton
                size="small"
                disabled={!search && filter === "all" && !deviceFilter && !mineOnly}
                onClick={() => {
                  setSearch("")
                  setFilter("all")
                  if (deviceFilter || mineOnly) {
                    router.replace("/enterprise/tickets", { scroll: false })
                  }
                }}
              >{ee("tickets.text053")}</RailopsButton>
            </div>
          </div>
        </div>

        <div className="rhd-railops-ticket-table-wrap">
          {loading && !ticketsLoaded && (
            <ModuleLoading className="p-4" variant="table" count={6} />
          )}
          {loading && tickets.length > 0 && (
            <div className="rhd-railops-refreshing" role="status" aria-label={ee("tickets.text054")} aria-busy="true">
              <Loader2Icon className="mr-2 inline size-4 animate-spin" />{ee("tickets.text055")}</div>
          )}
          {!loading && error && (
            <div className="rhd-railops-ticket-error">
              <p>{error}</p>
              <RailopsButton size="small" onClick={loadTickets}>{ee("tickets.text056")}</RailopsButton>
            </div>
          )}
          {(!loading || tickets.length > 0) && !error && (
            <DataTable<TicketListItem>
              className="rhd-railops-device-table rhd-railops-ticket-table"
              size="small"
              columns={ticketColumns}
              dataSource={tickets}
              emptyDescription={
                <div className="rhd-railops-ticket-empty">
                  <strong>{ee("tickets.text057")}</strong>
                  <span>{ee("tickets.text058")}</span>
                </div>
              }
              rowKey={(record) => record.id}
              scroll={{ x: 1400 }}
              onRow={(ticket) => ({
                onClick: () => openTicketModal(ticket.id),
              })}
            />
          )}
        </div>

        {loading === false && !error && (
          <TablePagination
            className="rhd-railops-table-footer rhd-railops-ticket-footer"
            current={page}
            pageSize={PAGE_SIZE}
            total={total}
            onChange={setPage}
            footerNote={ee("tickets.text059", { value0: total, value1: page, value2: totalPages })}
          />
        )}
      </section>

      <StandardModal
        centered
        destroyOnHidden
        open={createOpen}
        title={ee("tickets.text065")}
        width={620}
        styles={{ body: { maxHeight: "calc(100dvh - 180px)", overflowY: "auto", overflowX: "hidden" } }}
        onCancel={closeCreateModal}
        footer={
          createdInvitation ? (
            <RailopsButton variant="primary" onClick={closeCreateModal}>{ee("tickets.text097")}</RailopsButton>
          ) : (
            <>
              <RailopsButton onClick={closeCreateModal} disabled={createSaving}>{ee("tickets.text073")}</RailopsButton>
              <RailopsButton variant="primary" onClick={() => void submitCreateTicket()} disabled={createSaving}>
                {createSaving ? ee("tickets.text072") : ee("tickets.text071")}
              </RailopsButton>
            </>
          )
        }
      >
        {createdInvitation ? (
          <div className="grid gap-4">
            <div className="flex items-start gap-3 border border-emerald-200 bg-emerald-50 px-4 py-3 text-emerald-950">
              <CheckCircle2Icon className="mt-0.5 size-5 shrink-0 text-emerald-600" />
              <div className="grid gap-1">
                <strong>{ee("tickets.text092")}</strong>
                <span className="text-sm text-emerald-800">{ee("tickets.text093")}</span>
                <div className="mt-1"><StatusTag tone="neutral">{ee("tickets.text101")} · {createdInvitation.ticket.ticket_no}</StatusTag></div>
              </div>
            </div>
            <FormField label={ee("tickets.text094")}>
              <div className="flex items-center gap-2">
                <Input readOnly value={createdInvitation.invitation.registrationUrl} />
                <IconButton
                  icon={<CopyIcon className="size-4" />}
                  tooltip={invitationLinkCopied ? ee("tickets.text096") : ee("tickets.text095")}
                  aria-label={invitationLinkCopied ? ee("tickets.text096") : ee("tickets.text095")}
                  onClick={() => void copyInvitationLink()}
                />
              </div>
            </FormField>
            {!createdInvitation.invitation.emailSent ? (
              <div className="border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-900">{ee("tickets.text100")}</div>
            ) : null}
          </div>
        ) : (
          <div className="grid gap-4">
            <div className="flex items-end gap-2">
              <div className="min-w-0 flex-1"><SelectField label={intakeLabel("channel")} selectProps={{ value: ticketDraft.channel, options: [{ value: "enterprise", label: intakeLabel("enterprise") }, { value: "phone", label: intakeLabel("phone") }], onChange: (channel) => setTicketDraft((current) => ({ ...current, channel, customerMode: "existing" })) }} /></div>
              {CanUseButton("ticket.update", session?.permissions) && <IntakePolicyButton onSaved={setIntakePolicy} />}
            </div>
            {ticketDraft.channel === "phone" && <IntakeFields value={ticketDraft.intake} onChange={(intake) => setTicketDraft((current) => ({ ...current, intake }))} policy={intakePolicy} customerId={ticketDraft.customerId} showDeviceContext={hasDeviceConcept} />}
            {ticketDraft.channel !== "phone" && <FormField label={ee("tickets.text078")} required>
              <Segmented
                block
                value={ticketDraft.customerMode}
                options={[
                  { value: "existing", label: ee("tickets.text079") },
                  { value: "invite", label: ee("tickets.text080") },
                ]}
                onChange={(value) => {
                  setCreateError("")
                  setTicketDraft((current) => ({ ...current, customerMode: value as TicketCustomerMode }))
                }}
              />
            </FormField>}
            {ticketDraft.customerMode === "existing" ? (
              <div className="grid gap-2">
                <SelectField
                  label={ee("tickets.text077")}
                  required={ticketDraft.channel !== "phone"}
                  selectProps={{
                    allowClear: ticketDraft.channel === "phone",
                    value: ticketDraft.customerId || undefined,
                    loading: customerOptionsLoading,
                    showSearch: true,
                    optionFilterProp: "label",
                    placeholder: customerOptionsLoading ? ee("tickets.text082") : ticketDraft.channel === "phone" ? intakeLabel("unidentified") : ee("tickets.text081"),
                    onChange: (customerId) => setTicketDraft((current) => ({ ...current, customerId: Number(customerId) || 0 })),
                    options: customerOptions.map((customer) => ({
                      value: customer.customer_id,
                      label: [customer.display_name, customer.customer_org_name, customer.email].filter(Boolean).join(" · "),
                    })),
                  }}
                />
                {customerOptionsError ? <span className="text-sm text-destructive">{customerOptionsError}</span> : null}
                {!customerOptionsLoading && !customerOptionsError && customerOptions.length === 0 ? (
                  <span className="text-sm text-muted-foreground">{ticketDraft.channel === "phone" ? intakeLabel("unidentified") : ee("tickets.text083")}</span>
                ) : null}
              </div>
            ) : (
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <FormField label={ee("tickets.text085")} required>
                  <Input
                    value={ticketDraft.inviteDisplayName}
                    maxLength={100}
                    placeholder={ee("tickets.text088")}
                    onChange={(event) => setTicketDraft((current) => ({ ...current, inviteDisplayName: event.target.value }))}
                  />
                </FormField>
                <FormField label={ee("tickets.text086")} required>
                  <Input
                    type="email"
                    value={ticketDraft.inviteEmail}
                    maxLength={200}
                    placeholder={ee("tickets.text089")}
                    onChange={(event) => setTicketDraft((current) => ({ ...current, inviteEmail: event.target.value }))}
                  />
                </FormField>
                <div className="sm:col-span-2">
                  <FormField label={ee("tickets.text087")} required>
                    <Input
                      value={ticketDraft.inviteCustomerOrg}
                      maxLength={200}
                      placeholder={ee("tickets.text090")}
                      onChange={(event) => setTicketDraft((current) => ({ ...current, inviteCustomerOrg: event.target.value }))}
                    />
                  </FormField>
                </div>
              </div>
            )}
            <FormField label={ee("tickets.text066")} required>
              <Input
                value={ticketDraft.title}
                maxLength={200}
                autoFocus
                placeholder={ee("tickets.text069")}
                onChange={(event) => setTicketDraft((current) => ({ ...current, title: event.target.value }))}
              />
            </FormField>
            <FormField label={ee("tickets.text067")} required>
              <Input.TextArea
                value={ticketDraft.description}
                rows={5}
                maxLength={4000}
                showCount
                placeholder={ee("tickets.text070")}
                onChange={(event) => setTicketDraft((current) => ({ ...current, description: event.target.value }))}
              />
            </FormField>
            <SelectField
              label={ee("tickets.text068")}
              selectProps={{
                value: ticketDraft.priority,
                onChange: (priority) => setTicketDraft((current) => ({ ...current, priority: priority as TicketPriority })),
                options: [
                  { value: "critical", label: ee("tickets.text008") },
                  { value: "high", label: ee("tickets.text009") },
                  { value: "medium", label: ee("tickets.text010") },
                  { value: "low", label: ee("tickets.text011") },
                ],
              }}
            />
            {createError ? <div className="border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive">{createError}</div> : null}
          </div>
        )}
      </StandardModal>

      <StandardModal
        centered
        destroyOnHidden
        footer={null}
        open={detailOpen}
        rootClassName="rhd-railops-ticket-modal-root"
        title={(
          <div className="rhd-railops-ticket-modal-title">
            <span>
              {detailData
                ? `${detailData.ticket.ticket_no} · ${detailData.ticket.title}`
                : detailListTicket?.ticket_no || ee("tickets.text060")}
            </span>
            {detailData ? (
              <StatusTag tone={ticketStatusTagTone(detailData.ticket.status)}>
                {ticketStatusLabel(detailData.ticket.status)}
              </StatusTag>
            ) : null}
          </div>
        )}
        width="80vw"
        afterClose={() => {
          setDetailTicketId(null)
          setDetailData(null)
          setDetailError("")
        }}
        onCancel={closeTicketModal}
      >
        <div className="rhd-railops-ticket-modal-body">
          {detailLoading ? (
            <div className="grid gap-4">
              <section className="rounded-md border border-border bg-card p-4">
                <div className="flex items-start justify-between gap-4">
                  <div className="min-w-0">
                    <div className="font-mono text-xs text-muted-foreground">
                      {detailListTicket?.ticket_no || ee("tickets.text061")}
                    </div>
                    <div className="mt-2 line-clamp-2 text-base font-semibold">
                      {detailListTicket?.title || ee("tickets.text060")}
                    </div>
                  </div>
                  {detailListTicket ? (
                    <StatusTag tone={ticketStatusTagTone(detailListTicket.status)}>
                      {ticketStatusLabel(detailListTicket.status)}
                    </StatusTag>
                  ) : null}
                </div>
              </section>
              <ModuleLoading label={ee("tickets.text062")} variant="detail" count={2} />
            </div>
          ) : detailError ? (
            <div className="flex min-h-80 flex-col items-center justify-center gap-3 rounded-md border border-destructive/30 bg-destructive/5 p-6 text-center">
              <p className="text-sm text-destructive">{detailError}</p>
              {detailTicketId ? (
                <RailopsButton
                  size="small"
                  onClick={() => openTicketModal(detailTicketId)}
                >{ee("tickets.text056")}</RailopsButton>
              ) : null}
            </div>
          ) : detailData ? (
            <div className="rhd-railops-ticket-detail-shell">
              <EnterpriseTicketDetailContent aggregate={detailData} showDeviceContext={hasDeviceConcept} />
            </div>
          ) : (
            <div className="flex min-h-80 items-center justify-center rounded-md border border-border bg-card text-sm text-muted-foreground">{ee("tickets.text063")}</div>
          )}
          <div className="rhd-railops-ticket-modal-actions">
            {detailData && <IntakeCompletion aggregate={detailData} showDeviceContext={hasDeviceConcept} onSaved={(value) => { setDetailData(value); void loadTickets() }} />}
            <RailopsButton onClick={closeTicketModal}>{ee("tickets.text064")}</RailopsButton>
            {detailTicketId ? (
              <Link href={`/enterprise/ticket-workbench?ticket_id=${detailTicketId}`}>
                <RailopsButton>{ee("tickets.text043")}</RailopsButton>
              </Link>
            ) : null}
          </div>
        </div>
      </StandardModal>
    </PageShell>
  )
}
