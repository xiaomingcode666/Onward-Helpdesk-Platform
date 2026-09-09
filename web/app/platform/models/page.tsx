"use client"

import { useCallback, useEffect, useMemo, useState } from "react"
import {
  Building2Icon,
  CircleDollarSignIcon,
  RefreshCwIcon,
  ServerCogIcon,
  Settings2Icon,
} from "lucide-react"
import { toast } from "sonner"

import { Input, type TableColumnsType } from "antd"
import {
  ContentModule,
  DataTable,
  DetailDrawer,
  IconButton,
  PageShell,
  RailopsButton,
  SearchField,
  SelectField,
  StatusTag,
  UnderlineTabs,
  type RailopsTabItem,
} from "@railops/ui"
import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import { ProjectDialog } from "@/components/project-dialog"
import {
  RailopsDateRangeFilter,
  RAILOPS_DEFAULT_TIMEZONE,
  getRailopsDateRange,
  type RailopsDateRangeValue,
} from "@/components/railops/date-range-filter"
import { ModuleLoading } from "@/components/shared/loading-states"

import {
  formatCurrency,
  formatDateTime,
  formatNumber,
  PlatformEmpty,
  PlatformError,
} from "@/components/platform/platform-live-ui"
import { cn } from "@/lib/utils"
import {
  fetchPlatformAIModelWorkspace,
  fetchPlatformAITenantBilling,
  fetchPlatformAITenantUsage,
  provisionPlatformAITenant,
  rechargePlatformAITenant,
  savePlatformAIProvider,
  type PlatformAIModelWorkspace,
  type PlatformAITenantBilling,
  type PlatformAIRechargeRecord,
  type PlatformAIUsageDashboardModel,
  type PlatformAIUsageDetailResponse,
  type PlatformAITenantWorkspaceItem,
} from "@/lib/api/platform"
import { useAppLocale, useI18n } from "@/i18n/provider"

type ProviderDraft = {
  host: string
  adminApiKey: string
  defaultLlmModel: string
}

type TenantProvisionDraft = {
  concurrency: number
  balance: number
  rpmLimit: number
  defaultKeyExpiresAt: string
}

type RechargeDraft = {
  amount: number
  notes: string
}

type UsageFilters = {
  startDate: string
  endDate: string
  timezone: string
  modelSource: string
  page: number
  pageSize: number
  sortBy: string
  sortOrder: string
}

type UsageRowView = {
  requestTime: string
  model: string
  reasoningEffort: string
  endpoint: string
  requestType: string
  inputTokens: number | null
  outputTokens: number | null
  cacheReadTokens: number | null
  totalTokens: number | null
  standardCost: number | null
  actualCost: number | null
  rateMultiplier: string
  durationMs: number | null
  firstTokenMs: number | null
  requestId: string
}

function buildDefaultUsageFilters(): UsageFilters {
  const timezone = RAILOPS_DEFAULT_TIMEZONE
  return {
    ...getRailopsDateRange("7d", timezone),
    timezone,
    modelSource: "requested",
    page: 1,
    pageSize: 20,
    sortBy: "created_at",
    sortOrder: "desc",
  }
}

function asRecord(value: unknown) {
  return value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : {}
}

function pickString(source: Record<string, unknown>, keys: string[], fallback = "") {
  for (const key of keys) {
    const value = source[key]
    if (value === undefined || value === null) continue
    const text = String(value).trim()
    if (text) return text
  }
  return fallback
}

function pickNumber(source: Record<string, unknown>, keys: string[]) {
  for (const key of keys) {
    const value = source[key]
    if (value === undefined || value === null || value === "") continue
    const numberValue = Number(value)
    if (!Number.isNaN(numberValue)) return numberValue
  }
  return null
}

function parseUsageRows(payload: unknown): UsageRowView[] {
  const root = asRecord(payload)
  const candidate: unknown[] =
    [payload, asRecord(root.data).items, asRecord(root.data).rows, asRecord(root.data).list, root.items, root.rows, root.list].find(
      (item): item is unknown[] => Array.isArray(item)
    ) ?? []

  return candidate.map((item) => {
    const row = asRecord(item)
    const standardCost = pickNumber(row, ["standard_cost", "total_cost", "cost", "standardCost"])
    const actualCost = pickNumber(row, ["actual_cost", "actualCost"])
    const derivedMultiplier =
      standardCost !== null && actualCost !== null && standardCost > 0
        ? `${(actualCost / standardCost).toFixed(2)}x`
        : ""

    return {
      requestTime: pickString(row, ["created_at", "createdAt", "request_time", "requestTime", "timestamp", "time"]),
      model: pickString(row, ["model", "model_name", "modelName"]),
      reasoningEffort: pickString(row, ["reasoning_effort", "reasoningEffort"]),
      endpoint: pickString(row, ["endpoint", "path", "route"]),
      requestType: pickString(row, ["request_type", "requestType", "type", "method"]),
      inputTokens: pickNumber(row, ["input_tokens", "inputTokens"]),
      outputTokens: pickNumber(row, ["output_tokens", "outputTokens"]),
      cacheReadTokens: pickNumber(row, ["cache_read_tokens", "cacheReadTokens"]),
      totalTokens: pickNumber(row, ["total_tokens", "totalTokens"]),
      standardCost,
      actualCost,
      rateMultiplier: pickString(row, ["rate_multiplier", "rateMultiplier", "multiplier"], derivedMultiplier),
      durationMs: pickNumber(row, ["duration_ms", "durationMs"]),
      firstTokenMs: pickNumber(row, ["first_token_ms", "firstTokenMs"]),
      requestId: pickString(row, ["request_id", "requestId", "id"]),
    }
  })
}

function statusToneByText(value: string) {
  const text = value.toLowerCase()
  if (["active", "ok", "success", "healthy", "ready", "enabled", "启用"].includes(text)) return "success" as const
  if (["provisioning_user", "provisioning_key", "provisioning", "pending", "syncing", "info"].includes(text)) {
    return "blue" as const
  }
  if (["warn", "warning", "attention", "degraded", "disabled", "failed", "error", "unknown", "key_unknown"].includes(text)) {
    return "warning" as const
  }
  if (["frozen", "blocked", "bad", "danger", "deleted", "已删除"].includes(text)) return "error" as const
  return "neutral" as const
}

type I18nT = ReturnType<typeof useI18n>

function accountStatusLabel(t: I18nT, status?: string) {
  switch ((status || "").toLowerCase()) {
    case "active":
    case "enabled":
      return t("platformExtract.models.status.enabled")
    case "disabled":
      return t("platformExtract.models.status.disabled")
    case "frozen":
      return t("platformExtract.models.status.frozen")
    case "pending":
      return t("platformExtract.models.status.pending")
    case "failed":
    case "error":
      return t("platformExtract.models.status.error")
    default:
      return status || t("platformExtract.models.status.notCreated")
  }
}

function operationLabel(t: I18nT, operation?: string) {
  if ((operation || "").toLowerCase() === "add") return t("platformExtract.models.operation.recharge")
  return operation || "-"
}

function recordStatusLabel(t: I18nT, status?: string) {
  switch ((status || "").toLowerCase()) {
    case "success":
    case "completed":
    case "ok":
      return t("platformExtract.models.recordStatus.completed")
    case "pending":
    case "processing":
      return t("platformExtract.models.recordStatus.processing")
    case "failed":
    case "error":
      return t("platformExtract.models.recordStatus.failed")
    default:
      return status || "-"
  }
}

function fingerprintLabel(value?: string) {
  if (!value) return "-"
  if (value.length <= 24) return value
  return `${value.slice(0, 12)}...${value.slice(-8)}`
}

function tenantDerivedEmail(item: PlatformAITenantWorkspaceItem) {
  return `${item.tenant.code.trim().toLowerCase()}@remotedesk.com`
}

function tenantIsAssigned(item: PlatformAITenantWorkspaceItem) {
  return Boolean(item.account?.defaultKeyId || item.account?.provisionStatus === "active")
}

function tenantAssignmentLabel(t: I18nT, item: PlatformAITenantWorkspaceItem) {
  if (!tenantIsAssigned(item)) return t("platformExtract.models.assignment.unassigned")
  return item.needsAttention
    ? t("platformExtract.models.assignment.attention")
    : t("platformExtract.models.assignment.assigned")
}

function tenantAssignmentTone(item: PlatformAITenantWorkspaceItem) {
  if (!tenantIsAssigned(item)) return "neutral" as const
  return item.needsAttention ? "warning" as const : "success" as const
}

function renderNumber(value: number | null) {
  return value === null ? "-" : formatNumber(value)
}

function renderCurrency(value: number | null) {
  return value === null ? "-" : formatCurrency(value)
}

function formatDateTimeInputValue(value?: string) {
  if (!value) return ""
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value.slice(0, 16)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, "0")
  const day = String(date.getDate()).padStart(2, "0")
  const hours = String(date.getHours()).padStart(2, "0")
  const minutes = String(date.getMinutes()).padStart(2, "0")
  return `${year}-${month}-${day}T${hours}:${minutes}`
}

function CompactStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex min-h-16 min-w-0 items-center justify-between gap-3 bg-card px-4 py-3">
      <span className="truncate text-xs font-medium text-muted-foreground">{label}</span>
      <span className="shrink-0 text-lg font-semibold text-foreground">{value}</span>
    </div>
  )
}

function ModelWorkspaceLoadingContent({ t }: { t: I18nT }) {
  return (
    <ContentModule title={t("platformExtract.models.modules.tenants")} className="rhd-railops-platform-panel">
      <div className="p-4">
        <ModuleLoading variant="table" count={7} label={t("platformExtract.models.loading.modelTenants")} />
      </div>
    </ContentModule>
  )
}

export default function PlatformModelsPage() {
  const t = useI18n()
  const { locale } = useAppLocale()
  const [tenantDetailTab, setTenantDetailTab] = useState("config")
  const breadcrumbItems = useRouteBreadcrumbItems()
  const tenantDetailTabs = useMemo<RailopsTabItem[]>(
    () => [
      { value: "config", label: t("platformExtract.models.modules.tenantConfig") },
      { value: "model-usage", label: t("platformExtract.models.usage.modelUsage") },
      { value: "call-details", label: t("platformExtract.models.usage.callDetails") },
      { value: "recharge", label: t("platformExtract.models.tabs.recharge") },
    ],
    [t]
  )

  const [workspace, setWorkspace] = useState<PlatformAIModelWorkspace | null>(null)
  const [workspaceLoading, setWorkspaceLoading] = useState(true)
  const [workspaceLoaded, setWorkspaceLoaded] = useState(false)
  const [workspaceError, setWorkspaceError] = useState("")

  const [providerDraft, setProviderDraft] = useState<ProviderDraft>({
    host: "",
    adminApiKey: "",
    defaultLlmModel: "",
  })
  const [tenantDraft, setTenantDraft] = useState<TenantProvisionDraft>({
    concurrency: 50,
    balance: 20,
    rpmLimit: 0,
    defaultKeyExpiresAt: "",
  })
  const [rechargeDraft, setRechargeDraft] = useState<RechargeDraft>({
    amount: 1,
    notes: "",
  })
  const [usageFilters, setUsageFilters] = useState<UsageFilters>(buildDefaultUsageFilters())
  const [usageRange, setUsageRange] = useState<RailopsDateRangeValue>("7d")
  const [selectedTenantId, setSelectedTenantId] = useState<number | null>(null)
  const [tenantSearch, setTenantSearch] = useState("")
  const [providerDialogOpen, setProviderDialogOpen] = useState(false)
  const [rechargeDialogOpen, setRechargeDialogOpen] = useState(false)

  const [savingProvider, setSavingProvider] = useState(false)
  const [provisioningTenant, setProvisioningTenant] = useState(false)
  const [rechargingTenant, setRechargingTenant] = useState(false)

  const [usage, setUsage] = useState<PlatformAIUsageDetailResponse | null>(null)
  const [usageLoading, setUsageLoading] = useState(false)
  const [usageError, setUsageError] = useState("")
  const [usageLoadedTenantId, setUsageLoadedTenantId] = useState<number | null>(null)
  const [billing, setBilling] = useState<PlatformAITenantBilling | null>(null)
  const [billingLoading, setBillingLoading] = useState(false)
  const [billingError, setBillingError] = useState("")
  const [billingLoadedTenantId, setBillingLoadedTenantId] = useState<number | null>(null)

  const refreshWorkspace = useCallback(async () => {
    setWorkspaceLoading(true)
    setWorkspaceError("")
    try {
      const next = await fetchPlatformAIModelWorkspace()
      setWorkspace(next)
      setWorkspaceLoaded(true)
      setProviderDraft({
        host: next.provider.host || "",
        adminApiKey: "",
        defaultLlmModel: next.provider.defaultLlmModel || "",
      })
      setSelectedTenantId((current) => {
        if (current && next.tenants.some((item) => item.tenant.id === current)) {
          return current
        }
        return null
      })
      return next
    } catch (error) {
      const message = error instanceof Error ? error.message : t("platformExtract.models.errors.workspaceLoadFailed")
      setWorkspaceError(message)
      return null
    } finally {
      setWorkspaceLoading(false)
    }
  }, [t])

  const refreshUsage = useCallback(async (tenantId: number, filters: UsageFilters) => {
    setUsageLoading(true)
    setUsageError("")
    try {
      const next = await fetchPlatformAITenantUsage({
        tenantId,
        startDate: filters.startDate,
        endDate: filters.endDate,
        timezone: filters.timezone,
        modelSource: filters.modelSource,
        page: filters.page,
        pageSize: filters.pageSize,
        sortBy: filters.sortBy,
        sortOrder: filters.sortOrder,
      })
      setUsage(next)
      setUsageLoadedTenantId(tenantId)
    } catch (error) {
      setUsage(null)
      setUsageLoadedTenantId(null)
      setUsageError(error instanceof Error ? error.message : t("platformExtract.models.errors.usageLoadFailed"))
    } finally {
      setUsageLoading(false)
    }
  }, [t])

  const refreshBilling = useCallback(async (tenantId: number, timezone: string) => {
    setBillingLoading(true)
    setBillingError("")
    try {
      const next = await fetchPlatformAITenantBilling({
        tenantId,
        timezone,
      })
      setBilling(next)
      setBillingLoadedTenantId(tenantId)
      return next
    } catch (error) {
      const message = error instanceof Error ? error.message : t("platformExtract.models.errors.billingLoadFailed")
      setBillingError(message)
      setBilling(null)
      setBillingLoadedTenantId(null)
      return null
    } finally {
      setBillingLoading(false)
    }
  }, [t])

  useEffect(() => {
    void refreshWorkspace()
  }, [refreshWorkspace])

  const selectedTenant = useMemo(() => {
    if (!workspace || !selectedTenantId) return null
    return workspace.tenants.find((item) => item.tenant.id === selectedTenantId) ?? null
  }, [selectedTenantId, workspace])
  const selectedTenantAccountId = selectedTenant?.account?.id ?? 0

  const filteredTenants = useMemo(() => {
    if (!workspace) return []
    const q = tenantSearch.trim().toLowerCase()
    if (!q) return workspace.tenants
    return workspace.tenants.filter((item) => {
      return (
        item.tenant.code.toLowerCase().includes(q) ||
        item.tenant.name.toLowerCase().includes(q) ||
        tenantDerivedEmail(item).toLowerCase().includes(q) ||
        (item.account?.loginEmail || "").toLowerCase().includes(q) ||
        (item.account?.accountName || "").toLowerCase().includes(q)
      )
    })
  }, [tenantSearch, workspace])

  const usageRows = useMemo(() => parseUsageRows(usage?.usage), [usage])

  useEffect(() => {
    if (!workspace || !selectedTenantId || !selectedTenant || selectedTenant.needsAttention) {
      setUsage(null)
      setUsageError("")
      setUsageLoadedTenantId(null)
      return
    }
    void refreshUsage(selectedTenantId, usageFilters)
  }, [refreshUsage, selectedTenant, selectedTenantId, usageFilters, workspace])

  useEffect(() => {
    if (!selectedTenantId || !selectedTenantAccountId || !selectedTenant || selectedTenant.needsAttention) {
      setBilling(null)
      setBillingError("")
      setBillingLoadedTenantId(null)
      return
    }
    void refreshBilling(selectedTenantId, usageFilters.timezone)
  }, [refreshBilling, selectedTenant, selectedTenantAccountId, selectedTenantId, usageFilters.timezone])

  useEffect(() => {
    if (!selectedTenant) return
    setTenantDraft({
      concurrency: selectedTenant.account?.concurrency || 50,
      balance: selectedTenant.account?.balance || 20,
      rpmLimit: selectedTenant.account?.rpmLimit || 0,
      defaultKeyExpiresAt: formatDateTimeInputValue(selectedTenant.account?.defaultKeyExpiresAt),
    })
  }, [selectedTenant])

  const handleSaveProvider = useCallback(async () => {
    const host = providerDraft.host.trim()
    if (!host) {
      toast.error(t("platformExtract.models.toast.hostRequired"))
      return
    }
    setSavingProvider(true)
    try {
      const next = await savePlatformAIProvider({
        host,
        adminApiKey: providerDraft.adminApiKey.trim() || undefined,
        defaultLlmModel: providerDraft.defaultLlmModel.trim() || undefined,
      })
      setWorkspace(next)
      setWorkspaceLoaded(true)
      setProviderDraft({
        host: next.provider.host || host,
        adminApiKey: "",
        defaultLlmModel: next.provider.defaultLlmModel || providerDraft.defaultLlmModel.trim(),
      })
      setProviderDialogOpen(false)
      toast.success(t("platformExtract.models.toast.providerSaved"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("platformExtract.models.toast.providerSaveFailed"))
    } finally {
      setSavingProvider(false)
    }
  }, [providerDraft.adminApiKey, providerDraft.defaultLlmModel, providerDraft.host, t])

  const handleProvisionTenant = useCallback(async () => {
    if (!selectedTenantId) {
      toast.error(t("platformExtract.models.toast.selectTenant"))
      return
    }
    setProvisioningTenant(true)
    try {
      const next = await provisionPlatformAITenant({
        tenantId: selectedTenantId,
        concurrency: tenantDraft.concurrency,
        balance: tenantDraft.balance,
        rpmLimit: tenantDraft.rpmLimit,
        defaultKeyExpiresAt: tenantDraft.defaultKeyExpiresAt || undefined,
      })
      setWorkspace(next)
      setWorkspaceLoaded(true)
      void refreshBilling(selectedTenantId, usageFilters.timezone)
      toast.success(t("platformExtract.models.toast.tenantProvisioned"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("platformExtract.models.toast.tenantProvisionFailed"))
    } finally {
      setProvisioningTenant(false)
    }
  }, [
    refreshBilling,
    selectedTenantId,
    tenantDraft.balance,
    tenantDraft.concurrency,
    tenantDraft.defaultKeyExpiresAt,
    tenantDraft.rpmLimit,
    usageFilters.timezone,
    t,
  ])

  const handleRechargeTenant = useCallback(async () => {
    if (!selectedTenantId) {
      toast.error(t("platformExtract.models.toast.selectTenant"))
      return
    }
    if (!selectedTenant?.account?.sub2apiAccountId) {
      toast.error(t("platformExtract.models.toast.provisionFirst"))
      return
    }
    if (rechargeDraft.amount <= 0) {
      toast.error(t("platformExtract.models.toast.rechargeAmountRequired"))
      return
    }
    setRechargingTenant(true)
    try {
      const next = await rechargePlatformAITenant({
        tenantId: selectedTenantId,
        amount: rechargeDraft.amount,
        notes: rechargeDraft.notes.trim() || undefined,
        timezone: usageFilters.timezone,
      })
      setBilling(next)
      setBillingLoadedTenantId(selectedTenantId)
      setRechargeDialogOpen(false)
      setRechargeDraft({ amount: 1, notes: "" })
      void refreshWorkspace()
      toast.success(t("platformExtract.models.toast.rechargeSuccess"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("platformExtract.models.toast.rechargeFailed"))
    } finally {
      setRechargingTenant(false)
    }
  }, [rechargeDraft.amount, rechargeDraft.notes, refreshWorkspace, selectedTenant?.account?.sub2apiAccountId, selectedTenantId, usageFilters.timezone, t])

  const currentBalance = billing?.remoteUser?.balance ?? selectedTenant?.account?.balance ?? null
  const currentFrozenBalance = billing?.remoteUser?.frozenBalance ?? null
  const currentTotalRecharged = billing?.remoteUser?.totalRecharged ?? null
  const rechargeRecords = billing?.rechargeRecords ?? []
  const providerConfigured = workspace?.provider.adminApiKeyConfigured ?? false
  const hasWorkspace = workspace !== null
  const hasUsageLoaded = usageLoadedTenantId === selectedTenantId
  const hasBillingLoaded = billingLoadedTenantId === selectedTenantId
  const initialWorkspaceLoading = workspaceLoading && !workspaceLoaded
  const usageInitialLoading = usageLoading && !hasUsageLoaded
  const billingInitialLoading = billingLoading && !hasBillingLoaded

  function handleUsageRangeChange(value: RailopsDateRangeValue) {
    setUsageRange(value)
    if (value === "custom") return
    setUsageFilters((current) => ({
      ...current,
      ...getRailopsDateRange(value, current.timezone),
      page: 1,
    }))
  }

  function updateUsageDateField(field: "startDate" | "endDate", value: string) {
    setUsageRange("custom")
    setUsageFilters((current) => ({ ...current, [field]: value, page: 1 }))
  }

  const usageModelsColumns: TableColumnsType<PlatformAIUsageDashboardModel> = [
    {
      title: t("platformExtract.models.columns.model"),
      dataIndex: "model",
      key: "model",
      render: (_value, model) => <div className="font-semibold text-foreground">{model.model}</div>,
    },
    {
      title: t("platformExtract.models.columns.requests"),
      dataIndex: "requests",
      key: "requests",
      render: (_value, model) => formatNumber(model.requests),
    },
    {
      title: t("platformExtract.models.columns.input"),
      dataIndex: "inputTokens",
      key: "inputTokens",
      render: (_value, model) => formatNumber(model.inputTokens),
    },
    {
      title: t("platformExtract.models.columns.output"),
      dataIndex: "outputTokens",
      key: "outputTokens",
      render: (_value, model) => formatNumber(model.outputTokens),
    },
    {
      title: t("platformExtract.models.columns.cacheRead"),
      dataIndex: "cacheReadTokens",
      key: "cacheReadTokens",
      render: (_value, model) => formatNumber(model.cacheReadTokens),
    },
    {
      title: t("platformExtract.models.columns.totalTokens"),
      dataIndex: "totalTokens",
      key: "totalTokens",
      render: (_value, model) => formatNumber(model.totalTokens),
    },
    {
      title: t("platformExtract.models.columns.standardCost"),
      dataIndex: "cost",
      key: "cost",
      render: (_value, model) => formatCurrency(model.cost),
    },
    {
      title: t("platformExtract.models.columns.actualCost"),
      dataIndex: "actualCost",
      key: "actualCost",
      render: (_value, model) => formatCurrency(model.actualCost),
    },
  ]

  const usageRowsColumns: TableColumnsType<UsageRowView> = [
    {
      title: t("platformExtract.models.columns.requestTime"),
      key: "requestTime",
      render: (_value, row) => (
        <span className="whitespace-nowrap">{row.requestTime ? formatDateTime(row.requestTime, locale) : "-"}</span>
      ),
    },
    {
      title: t("platformExtract.models.columns.modelReasoning"),
      key: "model",
      render: (_value, row) => (
        <div>
          <div className="font-semibold text-foreground">{row.model || "-"}</div>
          <div className="mt-1 text-xs text-muted-foreground">{row.reasoningEffort || "-"}</div>
        </div>
      ),
    },
    {
      title: t("platformExtract.models.columns.endpointType"),
      key: "endpoint",
      render: (_value, row) => (
        <div>
          <div className="font-medium text-foreground">{row.endpoint || "-"}</div>
          <div className="mt-1 text-xs text-muted-foreground">{row.requestType || "-"}</div>
        </div>
      ),
    },
    {
      title: t("platformExtract.models.columns.tokens"),
      key: "tokens",
      render: (_value, row) => (
        <div>
          <div className="text-xs text-muted-foreground">{t("platformExtract.models.columns.input")} {renderNumber(row.inputTokens)}</div>
          <div className="text-xs text-muted-foreground">{t("platformExtract.models.columns.output")} {renderNumber(row.outputTokens)}</div>
          <div className="text-xs text-muted-foreground">{t("platformExtract.models.columns.cache")} {renderNumber(row.cacheReadTokens)}</div>
          <div className="mt-1 text-xs text-muted-foreground">{t("platformExtract.models.columns.total")} {renderNumber(row.totalTokens)}</div>
        </div>
      ),
    },
    {
      title: t("platformExtract.models.columns.cost"),
      key: "cost",
      render: (_value, row) => (
        <div>
          <div className="text-xs text-muted-foreground">{t("platformExtract.models.columns.standard")} {renderCurrency(row.standardCost)}</div>
          <div className="text-xs text-muted-foreground">{t("platformExtract.models.columns.actual")} {renderCurrency(row.actualCost)}</div>
          <div className="mt-1 text-xs text-muted-foreground">{t("platformExtract.models.columns.multiplier")} {row.rateMultiplier || "-"}</div>
        </div>
      ),
    },
    {
      title: t("platformExtract.models.columns.duration"),
      key: "duration",
      render: (_value, row) => (
        <div>
          <div className="text-xs text-muted-foreground">{t("platformExtract.models.columns.totalDuration")} {renderNumber(row.durationMs)} ms</div>
          <div className="text-xs text-muted-foreground">{t("platformExtract.models.columns.firstToken")} {renderNumber(row.firstTokenMs)} ms</div>
        </div>
      ),
    },
    {
      title: t("platformExtract.models.columns.requestId"),
      key: "requestId",
      render: (_value, row) => (
        <span className="font-mono text-xs text-muted-foreground">{row.requestId || "-"}</span>
      ),
    },
  ]

  const rechargeColumns: TableColumnsType<PlatformAIRechargeRecord> = [
    {
      title: t("platformExtract.models.columns.time"),
      key: "occurredAt",
      render: (_value, record) => (
        <span className="whitespace-nowrap">{record.occurredAt ? formatDateTime(record.occurredAt, locale) : "-"}</span>
      ),
    },
    {
      title: t("platformExtract.models.columns.operation"),
      dataIndex: "operation",
      key: "operation",
      render: (_value, record) => operationLabel(t, record.operation),
    },
    {
      title: t("platformExtract.models.columns.amount"),
      dataIndex: "amount",
      key: "amount",
      render: (_value, record) => <span className="font-semibold text-foreground">{formatCurrency(record.amount)}</span>,
    },
    {
      title: t("platformExtract.models.columns.balanceChange"),
      key: "balance",
      render: (_value, record) => (
        <div>
          <div className="text-xs text-muted-foreground">{t("platformExtract.models.columns.before")} {formatCurrency(record.balanceBefore)}</div>
          <div className="text-xs text-muted-foreground">{t("platformExtract.models.columns.after")} {formatCurrency(record.balanceAfter)}</div>
        </div>
      ),
    },
    {
      title: t("platformExtract.models.columns.notes"),
      dataIndex: "notes",
      key: "notes",
      render: (_value, record) => <span className="text-muted-foreground">{record.notes || "-"}</span>,
    },
    {
      title: t("platformExtract.models.columns.operator"),
      key: "operatorName",
      render: (_value, record) => (
        <span className="text-muted-foreground">{record.operatorName || `#${record.operatorId || "-"}`}</span>
      ),
    },
    {
      title: t("platformExtract.models.columns.status"),
      key: "status",
      render: (_value, record) => (
        <StatusTag tone={statusToneByText(record.status)} className="rhd-railops-platform-status">
          {recordStatusLabel(t, record.status)}
        </StatusTag>
      ),
    },
  ]

  const tenantColumns: TableColumnsType<PlatformAITenantWorkspaceItem> = [
    {
      title: t("platformExtract.models.groups.tenant"),
      children: [
        {
          title: t("platformExtract.models.modules.tenants"),
          key: "tenant",
          fixed: "left",
          width: 220,
          render: (_value, item) => (
            <div className="flex min-w-0 items-center gap-3">
              <span className="inline-flex size-8 shrink-0 items-center justify-center rounded-md border border-primary/15 bg-primary/5 text-primary">
                <Building2Icon className="size-4" />
              </span>
              <div className="min-w-0">
                <div className="truncate font-semibold text-foreground" title={item.tenant.name}>{item.tenant.name}</div>
                <div className="mt-1 font-mono text-xs text-muted-foreground">{item.tenant.code}</div>
              </div>
            </div>
          ),
        },
        {
          title: t("platformExtract.models.tenantConfig.authAccount"),
          key: "authAccount",
          width: 230,
          render: (_value, item) => (
            <span className="inline-flex max-w-full whitespace-nowrap rounded-md bg-muted/60 px-2 py-1 font-mono text-xs text-foreground">
              {item.account?.loginEmail || tenantDerivedEmail(item)}
            </span>
          ),
        },
      ],
    },
    {
      title: t("platformExtract.models.groups.service"),
      children: [
        {
          title: t("platformExtract.models.tenantConfig.account"),
          key: "accountStatus",
          width: 110,
          render: (_value, item) => (
            <StatusTag tone={statusToneByText(item.account?.accountStatus || "")} className="rhd-railops-platform-status">
              {accountStatusLabel(t, item.account?.accountStatus)}
            </StatusTag>
          ),
        },
        {
          title: t("platformExtract.models.tenantConfig.tokenExpiresAt"),
          key: "tokenExpiresAt",
          width: 160,
          render: (_value, item) => (
            <span className="whitespace-nowrap text-xs text-foreground">
              {item.account?.accessTokenExpiresAt ? formatDateTime(item.account.accessTokenExpiresAt, locale) : "-"}
            </span>
          ),
        },
      ],
    },
    {
      title: t("platformExtract.models.groups.limits"),
      children: [
        {
          title: t("platformExtract.models.tenantConfig.concurrency"),
          key: "concurrency",
          width: 110,
          align: "right",
          render: (_value, item) => item.account ? <span className="font-semibold text-foreground">{formatNumber(item.account.concurrency)}</span> : "-",
        },
        {
          title: t("platformExtract.models.tenantConfig.balance"),
          key: "balance",
          width: 120,
          align: "right",
          render: (_value, item) => item.account ? <span className="font-semibold text-foreground">{formatCurrency(item.account.balance)}</span> : "-",
        },
        {
          title: t("platformExtract.models.tenantConfig.rpmLimit"),
          key: "rpmLimit",
          width: 150,
          align: "right",
          render: (_value, item) => item.account ? <span className="font-semibold text-foreground">{formatNumber(item.account.rpmLimit)}</span> : "-",
        },
      ],
    },
    {
      title: t("platformExtract.models.groups.model"),
      children: [
        {
          title: t("platformExtract.models.tenantConfig.modelKeyExpiresAt"),
          key: "defaultKeyExpiresAt",
          width: 170,
          render: (_value, item) => (
            <span className="whitespace-nowrap text-xs text-foreground">
              {item.account?.defaultKeyExpiresAt ? formatDateTime(item.account.defaultKeyExpiresAt, locale) : "-"}
            </span>
          ),
        },
        {
          title: t("platformExtract.models.providerDialog.defaultModel"),
          key: "defaultLlmModel",
          width: 160,
          render: (_value, item) => item.account?.defaultLlmModel ? (
            <code className="inline-flex max-w-full truncate rounded-md border border-primary/15 bg-primary/5 px-2 py-1 text-xs text-primary" title={item.account.defaultLlmModel}>
              {item.account.defaultLlmModel}
            </code>
          ) : <span className="text-muted-foreground">-</span>,
        },
      ],
    },
    {
      title: t("platformExtract.models.columns.status"),
      key: "assignment",
      width: 110,
      render: (_value, item) => (
        <StatusTag tone={tenantAssignmentTone(item)} className="rhd-railops-platform-status">
          {tenantAssignmentLabel(t, item)}
        </StatusTag>
      ),
    },
    {
      title: t("platformExtract.models.columns.operation"),
      key: "actions",
      fixed: "right",
      width: 72,
      align: "center",
      render: (_value, item) => (
        <IconButton
          icon={<Settings2Icon className="size-4" />}
          tooltip={t("platformExtract.models.modules.tenantConfig")}
          aria-label={`${t("platformExtract.models.modules.tenantConfig")}：${item.tenant.name}`}
          onClick={(event) => {
            event.stopPropagation()
            setSelectedTenantId(item.tenant.id)
            setTenantDetailTab("config")
          }}
        />
      ),
    },
  ]

  const usageControls = (
    <div className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_220px]">
      <RailopsDateRangeFilter
        ariaLabel={t("platformExtract.models.usage.dateRangeAria")}
        className="rhd-railops-platform-date-filter"
        disabled={!selectedTenantId || Boolean(selectedTenant?.needsAttention)}
        endDate={usageFilters.endDate}
        endLabel={t("platformExtract.models.usage.endDate")}
        loading={usageLoading}
        startDate={usageFilters.startDate}
        startLabel={t("platformExtract.models.usage.startDate")}
        summary={`${usageFilters.startDate || "-"} ~ ${usageFilters.endDate || "-"}`}
        title={t("platformExtract.models.usage.dateRangeTitle")}
        value={usageRange}
        onChange={handleUsageRangeChange}
        onEndDateChange={(value) => updateUsageDateField("endDate", value)}
        onStartDateChange={(value) => updateUsageDateField("startDate", value)}
        actions={(
          <RailopsButton
            size="small"
            onClick={() => {
              if (selectedTenantId) {
                void refreshUsage(selectedTenantId, usageFilters)
              }
            }}
            disabled={usageLoading || !selectedTenantId || selectedTenant?.needsAttention}
          >
            <RefreshCwIcon className={usageLoading ? "animate-spin" : ""} />
            {t("platformExtract.models.refresh")}
          </RailopsButton>
        )}
      />
      <SelectField
        label={t("platformExtract.models.usage.modelSource")}
        style={{ marginBottom: 0 }}
        selectProps={{
          value: usageFilters.modelSource,
          onChange: (value) =>
            setUsageFilters((current) => ({ ...current, modelSource: value || current.modelSource, page: 1 })),
          placeholder: t("platformExtract.models.usage.selectModelSource"),
          options: [
            { value: "requested", label: t("platformExtract.models.usage.requestedModel") },
            { value: "all", label: t("platformExtract.models.usage.allModels") },
          ],
          style: { width: "100%" },
        }}
      />
    </div>
  )

  return (
    <PageShell
      title={t("platformExtract.models.title")}
      breadcrumb={breadcrumbItems}
      className="min-h-[calc(100vh-97px)]"
      actions={(
        <>
          <IconButton
            icon={<RefreshCwIcon className={cn("size-4", workspaceLoading && "animate-spin")} />}
            tooltip={t("platformExtract.models.refresh")}
            aria-label={t("platformExtract.models.refresh")}
            onClick={() => void refreshWorkspace()}
            disabled={workspaceLoading}
          />
          <RailopsButton
            onClick={() => setProviderDialogOpen(true)}
            disabled={!hasWorkspace}
          >
            <Settings2Icon className="size-4" />
            {t("platformExtract.models.providerSettings")}
          </RailopsButton>
        </>
      )}
    >

      {workspaceError && workspace ? (
        <div className="mb-4 rounded-md border border-border bg-muted px-3 py-2 text-sm text-muted-foreground">
          {workspaceError}
        </div>
      ) : null}

      {initialWorkspaceLoading ? (
        <ModelWorkspaceLoadingContent t={t} />
      ) : workspaceError && !workspace ? (
        <PlatformError message={workspaceError} onRetry={() => void refreshWorkspace()} />
      ) : workspace ? (
        <div className="min-w-0 space-y-4">
          <ContentModule
            className="rhd-railops-platform-panel w-full"
          >
            <div className="min-w-0">
              <div className="flex justify-end border-b border-border/70 bg-muted/20 px-4 py-3">
                <SearchField
                  allowClear
                  className="w-full sm:w-80"
                  placeholder={t("platformExtract.models.searchTenants")}
                  value={tenantSearch}
                  onChange={(e) => setTenantSearch(e.target.value)}
                />
              </div>
              {filteredTenants.length === 0 ? (
                <div className="p-4">
                  <PlatformEmpty title={t("platformExtract.models.empty.noMatchedTenants")} />
                </div>
              ) : (
                <div className="overflow-hidden border-x border-border">
                  <DataTable<PlatformAITenantWorkspaceItem>
                    className="rhd-railops-platform-models-tenants-table"
                    size="small"
                    rowKey={(item) => item.tenant.id}
                    dataSource={filteredTenants}
                    emptyDescription={t("platformExtract.models.empty.noMatchedTenants")}
                    scroll={{ x: 2012 }}
                    columns={tenantColumns}
                    rowClassName={(item) => cn(
                      "cursor-pointer transition-colors",
                      selectedTenantId === item.tenant.id && "rhd-railops-platform-models-tenant-row-selected"
                    )}
                    onRow={(item) => ({
                      onClick: () => {
                        setSelectedTenantId(item.tenant.id)
                        setTenantDetailTab("config")
                      },
                    })}
                  />
                </div>
              )}
            </div>
          </ContentModule>

          <DetailDrawer
            open={Boolean(selectedTenant)}
            onClose={() => {
              setSelectedTenantId(null)
              setTenantDetailTab("config")
              setRechargeDialogOpen(false)
            }}
            title={selectedTenant?.tenant.name || t("platformExtract.models.modules.tenantConfig")}
            width="min(980px, calc(100vw - 16px))"
          >
            <div className="space-y-4">
              <div className="-mx-4 border-b border-border bg-muted/40 px-4 sm:-mx-6 sm:px-6">
                <UnderlineTabs
                  ariaLabel={t("platformExtract.models.usage.ariaSection")}
                  items={tenantDetailTabs}
                  value={tenantDetailTab}
                  onChange={setTenantDetailTab}
                />
              </div>

              {tenantDetailTab === "config" ? (
                <ContentModule
                  className="rhd-railops-platform-panel"
                  title={t("platformExtract.models.modules.tenantConfig")}
                  note={selectedTenant?.tenant.code}
                  extra={selectedTenant ? (
                    <StatusTag tone={tenantAssignmentTone(selectedTenant)} className="rhd-railops-platform-status">
                      {tenantAssignmentLabel(t, selectedTenant)}
                    </StatusTag>
                  ) : null}
                >
                  <div className="space-y-4 p-4">
                    {selectedTenant ? (
                      <>
                        <dl className="grid gap-x-6 gap-y-4 text-sm sm:grid-cols-2">
                          <div>
                            <dt className="text-xs text-muted-foreground">{t("platformExtract.models.tenantConfig.authAccount")}</dt>
                            <dd className="mt-1 font-mono font-medium text-foreground">
                              {selectedTenant.account?.loginEmail || tenantDerivedEmail(selectedTenant)}
                            </dd>
                          </div>
                          <div>
                            <dt className="text-xs text-muted-foreground">{t("platformExtract.models.tenantConfig.account")}</dt>
                            <dd className="mt-1">
                              <StatusTag tone={statusToneByText(selectedTenant.account?.accountStatus || "")} className="rhd-railops-platform-status">
                                {accountStatusLabel(t, selectedTenant.account?.accountStatus)}
                              </StatusTag>
                            </dd>
                          </div>
                          <div>
                            <dt className="text-xs text-muted-foreground">{t("platformExtract.models.tenantConfig.tokenExpiresAt")}</dt>
                            <dd className="mt-1 font-medium text-foreground">
                              {selectedTenant.account?.accessTokenExpiresAt ? formatDateTime(selectedTenant.account.accessTokenExpiresAt, locale) : "-"}
                            </dd>
                          </div>
                          <div>
                            <dt className="text-xs text-muted-foreground">{t("platformExtract.models.tenantConfig.concurrency")}</dt>
                            <dd className="mt-1 font-medium text-foreground">{selectedTenant.account ? formatNumber(selectedTenant.account.concurrency) : "-"}</dd>
                          </div>
                          <div>
                            <dt className="text-xs text-muted-foreground">{t("platformExtract.models.tenantConfig.balance")}</dt>
                            <dd className="mt-1 font-medium text-foreground">{selectedTenant.account ? formatCurrency(selectedTenant.account.balance) : "-"}</dd>
                          </div>
                          <div>
                            <dt className="text-xs text-muted-foreground">{t("platformExtract.models.tenantConfig.rpmLimit")}</dt>
                            <dd className="mt-1 font-medium text-foreground">{selectedTenant.account ? formatNumber(selectedTenant.account.rpmLimit) : "-"}</dd>
                          </div>
                          <div>
                            <dt className="text-xs text-muted-foreground">{t("platformExtract.models.tenantConfig.modelKeyExpiresAt")}</dt>
                            <dd className="mt-1 font-medium text-foreground">
                              {selectedTenant.account?.defaultKeyExpiresAt ? formatDateTime(selectedTenant.account.defaultKeyExpiresAt, locale) : "-"}
                            </dd>
                          </div>
                        </dl>

                        <form
                          className="grid gap-4 border-t border-border pt-4"
                          onSubmit={(event) => {
                            event.preventDefault()
                            void handleProvisionTenant()
                          }}
                        >
                          <div className="grid gap-3 sm:grid-cols-2">
                            <div className="space-y-2">
                              <label className="text-xs font-medium text-muted-foreground">{t("platformExtract.models.tenantConfig.concurrency")}</label>
                              <Input
                                type="number"
                                min={1}
                                value={tenantDraft.concurrency}
                                onChange={(e) => setTenantDraft((current) => ({ ...current, concurrency: Number(e.target.value) || 0 }))}
                              />
                            </div>
                            <div className="space-y-2">
                              <label className="text-xs font-medium text-muted-foreground">{t("platformExtract.models.tenantConfig.balance")}</label>
                              <Input
                                type="number"
                                min={0}
                                step="0.01"
                                value={tenantDraft.balance}
                                onChange={(e) => setTenantDraft((current) => ({ ...current, balance: Number(e.target.value) || 0 }))}
                              />
                            </div>
                            <div className="space-y-2">
                              <label className="text-xs font-medium text-muted-foreground">{t("platformExtract.models.tenantConfig.rpmLimit")}</label>
                              <Input
                                type="number"
                                min={0}
                                value={tenantDraft.rpmLimit}
                                onChange={(e) => setTenantDraft((current) => ({ ...current, rpmLimit: Number(e.target.value) || 0 }))}
                              />
                            </div>
                            <div className="space-y-2">
                              <label className="text-xs font-medium text-muted-foreground">{t("platformExtract.models.tenantConfig.modelKeyExpiresAt")}</label>
                              <Input
                                type="datetime-local"
                                value={tenantDraft.defaultKeyExpiresAt}
                                onChange={(e) => setTenantDraft((current) => ({ ...current, defaultKeyExpiresAt: e.target.value }))}
                              />
                            </div>
                          </div>
                          <div className="flex flex-wrap gap-2">
                            <RailopsButton variant="primary" htmlType="submit" disabled={provisioningTenant}>
                              <RefreshCwIcon className={provisioningTenant ? "animate-spin" : ""} />
                              {selectedTenant.needsAttention
                                ? t("platformExtract.models.actions.repairModelService")
                                : selectedTenant.account?.defaultKeyId
                                  ? t("platformExtract.models.actions.updateServiceConfig")
                                  : t("platformExtract.models.actions.provisionModelService")}
                            </RailopsButton>
                          </div>
                        </form>
                      </>
                    ) : (
                      <PlatformEmpty title={t("platformExtract.models.empty.selectTenant")} />
                    )}
                  </div>
                </ContentModule>
              ) : null}

              {tenantDetailTab === "model-usage" ? (
                <ContentModule
                  className="rhd-railops-platform-panel"
                  title={t("platformExtract.models.usage.modelUsage")}
                  note={selectedTenant?.tenant.name}
                >
                  <div className="space-y-4 p-4">
                    {usageControls}
                    {usageInitialLoading ? (
                      <ModuleLoading variant="metrics" count={4} label={t("platformExtract.models.loading.usageStats")} />
                    ) : (
                      <section
                        className="grid grid-cols-2 gap-px overflow-hidden border border-border bg-muted xl:grid-cols-4"
                        aria-label={t("platformExtract.models.usage.statsAria")}
                      >
                        <CompactStat label={t("platformExtract.models.usage.requests")} value={formatNumber(usage?.stats.totalRequests || 0)} />
                        <CompactStat label={t("platformExtract.models.usage.tokens")} value={formatNumber(usage?.stats.totalTokens || 0)} />
                        <CompactStat label={t("platformExtract.models.usage.cost")} value={renderCurrency(usage?.stats.totalActualCost ?? null)} />
                        <CompactStat
                          label={t("platformExtract.models.usage.averageDuration")}
                          value={usage ? `${formatNumber(Math.round(usage.stats.averageDurationMs))} ms` : "0 ms"}
                        />
                      </section>
                    )}
                    {usageError ? (
                      <div className="rounded-md border border-border bg-muted px-3 py-2 text-sm text-muted-foreground">{usageError}</div>
                    ) : null}
                    <DataTable<PlatformAIUsageDashboardModel>
                      className="rhd-railops-platform-models-usage-models-table"
                      size="small"
                      rowKey="model"
                      dataSource={usage?.models ?? []}
                      emptyDescription={t("platformExtract.models.empty.noData")}
                      scroll={{ x: 980 }}
                      columns={usageModelsColumns}
                    />
                  </div>
                </ContentModule>
              ) : null}

              {tenantDetailTab === "call-details" ? (
                <ContentModule
                  className="rhd-railops-platform-panel"
                  title={t("platformExtract.models.usage.callDetails")}
                  note={selectedTenant?.tenant.name}
                >
                  <div className="space-y-4 p-4">
                    {usageControls}
                    {usageError ? (
                      <div className="rounded-md border border-border bg-muted px-3 py-2 text-sm text-muted-foreground">{usageError}</div>
                    ) : null}
                    {usageLoading ? (
                      <ModuleLoading variant="table" count={5} label={t("platformExtract.models.loading.callDetails")} />
                    ) : usageRows.length > 0 ? (
                      <DataTable<UsageRowView>
                        className="rhd-railops-platform-models-usage-rows-table"
                        size="small"
                        rowKey={(row, index) => `${row.requestId}-${index}`}
                        dataSource={usageRows}
                        emptyDescription={t("platformExtract.models.empty.noRecords")}
                        scroll={{ x: 1240 }}
                        columns={usageRowsColumns}
                      />
                    ) : (
                      <PlatformEmpty title={t("platformExtract.models.empty.noRecords")} />
                    )}
                  </div>
                </ContentModule>
              ) : null}

              {tenantDetailTab === "recharge" ? (
                <ContentModule
                  className="rhd-railops-platform-panel"
                  title={t("platformExtract.models.tabs.recharge")}
                  note={selectedTenant?.tenant.name}
                >
                  <div className="space-y-4 p-4">
                    {selectedTenant ? (
                      billingInitialLoading ? (
                        <>
                          <ModuleLoading variant="metrics" count={3} label={t("platformExtract.models.loading.balance")} />
                          <ModuleLoading variant="table" count={4} label={t("platformExtract.models.loading.rechargeRecords")} />
                        </>
                      ) : (
                        <>
                          <div className="grid gap-px overflow-hidden border border-border bg-muted md:grid-cols-[repeat(3,minmax(0,1fr))_auto]">
                            <CompactStat label={t("platformExtract.models.balance.current")} value={currentBalance === null ? "-" : formatCurrency(currentBalance)} />
                            <CompactStat label={t("platformExtract.models.balance.frozen")} value={currentFrozenBalance === null ? "-" : formatCurrency(currentFrozenBalance)} />
                            <CompactStat
                              label={t("platformExtract.models.balance.totalRecharged")}
                              value={currentTotalRecharged === null ? "-" : formatCurrency(currentTotalRecharged)}
                            />
                            <div className="flex items-center justify-end bg-card p-3">
                              <RailopsButton
                                onClick={() => setRechargeDialogOpen(true)}
                                disabled={!selectedTenant.account?.sub2apiAccountId || selectedTenant.needsAttention || rechargingTenant}
                              >
                                <CircleDollarSignIcon />
                                {t("platformExtract.models.operation.recharge")}
                              </RailopsButton>
                            </div>
                          </div>
                          {billingError ? (
                            <div className="rounded-md border border-border bg-muted px-3 py-2 text-sm text-muted-foreground">{billingError}</div>
                          ) : null}
                          <div className="flex items-center justify-end">
                            <RailopsButton
                              size="small"
                              onClick={() => selectedTenantId && void refreshBilling(selectedTenantId, usageFilters.timezone)}
                              disabled={billingLoading || !selectedTenant.account?.sub2apiAccountId || selectedTenant.needsAttention}
                            >
                              <RefreshCwIcon className={billingLoading ? "animate-spin" : ""} />
                              {t("platformExtract.models.actions.sync")}
                            </RailopsButton>
                          </div>
                          {rechargeRecords.length > 0 ? (
                            <DataTable<PlatformAIRechargeRecord>
                              className="rhd-railops-platform-models-recharge-table"
                              size="small"
                              rowKey="id"
                              dataSource={rechargeRecords}
                              emptyDescription={t("platformExtract.models.empty.noRechargeRecords")}
                              scroll={{ x: 880 }}
                              columns={rechargeColumns}
                            />
                          ) : (
                            <PlatformEmpty title={t("platformExtract.models.empty.noRechargeRecords")} />
                          )}
                        </>
                      )
                    ) : (
                      <PlatformEmpty title={t("platformExtract.models.empty.selectTenant")} />
                    )}
                  </div>
                </ContentModule>
              ) : null}
            </div>
          </DetailDrawer>
        </div>
      ) : null}

      {workspace ? (
        <ProjectDialog
          open={providerDialogOpen}
          onOpenChange={setProviderDialogOpen}
          title={
            <span className="inline-flex items-center gap-2">
              <ServerCogIcon className="size-4 text-muted-foreground" />
              {t("platformExtract.models.providerDialog.title")}
            </span>
          }
          size="lg"
          footer={
            <>
              <RailopsButton onClick={() => setProviderDialogOpen(false)}>
                {t("platformExtract.models.cancel")}
              </RailopsButton>
              <RailopsButton variant="primary" onClick={() => void handleSaveProvider()} disabled={savingProvider}>
                <RefreshCwIcon className={savingProvider ? "animate-spin" : ""} />
                {t("platformExtract.models.providerDialog.save")}
              </RailopsButton>
            </>
          }
        >
          <div className="grid gap-4 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,0.8fr)]">
            <div className="space-y-4">
              <div className="space-y-2">
                <label className="text-xs font-medium text-muted-foreground">{t("platformExtract.models.providerDialog.host")}</label>
                <Input
                  value={providerDraft.host}
                  onChange={(e) => setProviderDraft((current) => ({ ...current, host: e.target.value }))}
                  placeholder={t("platformExtract.models.providerDialog.hostPlaceholder")}
                />
              </div>
              <div className="space-y-2">
                <label className="text-xs font-medium text-muted-foreground">{t("platformExtract.models.providerDialog.adminCredential")}</label>
                <Input
                  type="password"
                  value={providerDraft.adminApiKey}
                  onChange={(e) => setProviderDraft((current) => ({ ...current, adminApiKey: e.target.value }))}
                  placeholder={t("platformExtract.models.providerDialog.adminCredentialPlaceholder")}
                />
              </div>
              <div className="space-y-2">
                <label className="text-xs font-medium text-muted-foreground">{t("platformExtract.models.providerDialog.defaultModel")}</label>
                <Input
                  value={providerDraft.defaultLlmModel}
                  onChange={(e) => setProviderDraft((current) => ({ ...current, defaultLlmModel: e.target.value }))}
                  placeholder="gpt-5.4-mini"
                />
              </div>
              <div className="rounded-md border border-border bg-muted px-3 py-2 text-xs leading-5 text-muted-foreground">
                {t("platformExtract.models.providerDialog.credentialHint")}
              </div>
            </div>

            <div className="grid gap-3 border border-border bg-muted/50 p-4 text-sm text-muted-foreground">
              <div className="flex items-center justify-between gap-3">
                <span>{t("platformExtract.models.providerDialog.host")}</span>
                <StatusTag tone={workspace.provider.host ? "success" : "warning"} className="rhd-railops-platform-status">
                  {workspace.provider.host ? t("platformExtract.models.providerDialog.configured") : t("platformExtract.models.providerDialog.notConfigured")}
                </StatusTag>
              </div>
              <div className="flex items-center justify-between gap-3">
                <span>{t("platformExtract.models.providerDialog.managementCredential")}</span>
                <StatusTag tone={providerConfigured ? "success" : "warning"} className="rhd-railops-platform-status">
                  {providerConfigured ? t("platformExtract.models.providerDialog.encryptedSaved") : t("platformExtract.models.providerDialog.notSaved")}
                </StatusTag>
              </div>
              <div className="flex items-center justify-between gap-3">
                <span>{t("platformExtract.models.providerDialog.credentialFingerprint")}</span>
                <span
                  className="min-w-0 font-mono text-xs font-medium text-foreground"
                  title={workspace.provider.adminApiKeyFingerprint || undefined}
                >
                  {fingerprintLabel(workspace.provider.adminApiKeyFingerprint)}
                </span>
              </div>
              <div className="flex items-center justify-between gap-3">
                <span>{t("platformExtract.models.providerDialog.updatedAt")}</span>
                <span className="font-medium text-foreground">
                  {workspace.provider.updatedAt ? formatDateTime(workspace.provider.updatedAt, locale) : "-"}
                </span>
              </div>
            </div>
          </div>
        </ProjectDialog>
      ) : null}

      {workspace && selectedTenant ? (
        <ProjectDialog
          open={rechargeDialogOpen}
          onOpenChange={setRechargeDialogOpen}
          title={
            <span className="inline-flex items-center gap-2">
              <CircleDollarSignIcon className="size-4 text-primary" />
              {t("platformExtract.models.rechargeDialog.title")}
            </span>
          }
          size="md"
          footer={
            <>
              <RailopsButton onClick={() => setRechargeDialogOpen(false)}>
                {t("platformExtract.models.cancel")}
              </RailopsButton>
              <RailopsButton variant="primary" onClick={() => void handleRechargeTenant()} disabled={rechargingTenant}>
                <RefreshCwIcon className={rechargingTenant ? "animate-spin" : ""} />
                {t("platformExtract.models.rechargeDialog.confirm")}
              </RailopsButton>
            </>
          }
        >
          <form
            className="space-y-4"
            onSubmit={(event) => {
              event.preventDefault()
              void handleRechargeTenant()
            }}
          >
            <div className="border border-border bg-muted px-3 py-2 text-sm text-muted-foreground">
              {selectedTenant.tenant.name} · {selectedTenant.account?.loginEmail || tenantDerivedEmail(selectedTenant)}
            </div>
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="rounded-lg border border-border bg-muted/50 p-3">
                <div className="text-xs text-muted-foreground">{t("platformExtract.models.balance.current")}</div>
                <div className="mt-1 text-lg font-semibold text-foreground">
                  {currentBalance === null ? "-" : formatCurrency(currentBalance)}
                </div>
              </div>
              <div className="rounded-lg border border-border bg-muted/50 p-3">
                <div className="text-xs text-muted-foreground">{t("platformExtract.models.rechargeDialog.remoteUserId")}</div>
                <div className="mt-1 text-lg font-semibold text-foreground">
                  {billing?.remoteUser?.id || selectedTenant.account?.sub2apiAccountId || "-"}
                </div>
              </div>
            </div>
            <div className="space-y-2">
              <label className="text-xs font-medium text-muted-foreground">{t("platformExtract.models.rechargeDialog.amount")}</label>
              <Input
                type="number"
                min={0.01}
                step="0.01"
                value={rechargeDraft.amount}
                onChange={(event) => setRechargeDraft((current) => ({ ...current, amount: Number(event.target.value) || 0 }))}
                placeholder="1"
              />
            </div>
            <div className="space-y-2">
              <label className="text-xs font-medium text-muted-foreground">{t("platformExtract.models.columns.notes")}</label>
              <Input
                value={rechargeDraft.notes}
                onChange={(event) => setRechargeDraft((current) => ({ ...current, notes: event.target.value }))}
                placeholder={t("platformExtract.models.rechargeDialog.notesPlaceholder")}
              />
            </div>
          </form>
        </ProjectDialog>
      ) : null}
    </PageShell>
  )
}
