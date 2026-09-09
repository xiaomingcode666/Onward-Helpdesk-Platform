"use client"

import { useCallback, useEffect, useState } from "react"
import {
  Building2Icon,
  ChevronDownIcon,
  ChevronRightIcon,
  CircleDollarSignIcon,
  CpuIcon,
  GaugeIcon,
  KeyRoundIcon,
  RefreshCwIcon,
  TriangleAlertIcon,
  UsersIcon,
} from "lucide-react"

import { type TableColumnsType } from "antd"
import {
  ChartContainer,
  ContentModule,
  DataTable,
  IconButton,
  PageShell,
  SearchField,
  SelectField,
  StatCard,
  StatusTag,
  TablePagination,
  type StatusTagTone,
} from "@railops/ui"

import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import { ModuleLoading } from "@/components/shared/loading-states"
import {
  PlatformEmpty,
  PlatformError,
  formatCompactNumber,
  formatCurrency,
  formatNumber,
} from "@/components/platform/platform-live-ui"
import {
  fetchPlatformSub2APIUserKeys,
  fetchPlatformSub2APIUsers,
  fetchPlatformUsage,
  type PlatformSub2APIUserKey,
  type PlatformSub2APIUserKeyListResponse,
  type PlatformSub2APIUserListItem,
  type PlatformSub2APIUserListResponse,
  type PlatformUsageOverview,
} from "@/lib/api/platform"
import { useAppLocale, useI18n } from "@/i18n/provider"
import { cn } from "@/lib/utils"

const SUB2API_ACCOUNT_PAGE_SIZE = 20

function fingerprintLabel(value?: string) {
  if (!value) return "-"
  if (value.length <= 24) return value
  return `${value.slice(0, 12)}...${value.slice(-8)}`
}

function formatFullDateTime(value?: string, locale = "zh-CN", neverLabel = "-") {
  if (!value) return neverLabel
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(date)
}

function formatBalance(value: number, locale = "zh-CN") {
  return new Intl.NumberFormat(locale, {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: 2,
    maximumFractionDigits: 4,
  }).format(value)
}

function sub2APIStatus(t: ReturnType<typeof useI18n>, status: string) {
  switch (status.toLowerCase()) {
    case "active":
      return { label: t("platformExtract.common.status.active"), tone: "success" as const }
    case "disabled":
    case "inactive":
      return { label: t("platformExtract.common.disabled"), tone: "neutral" as const }
    case "banned":
    case "blocked":
      return { label: t("platformExtract.common.restricted"), tone: "danger" as const }
    default:
      return { label: status || t("platformExtract.common.unknown"), tone: "warning" as const }
  }
}

const statusPillToneMap: Record<"success" | "neutral" | "danger" | "warning", StatusTagTone> = {
  success: "success",
  neutral: "neutral",
  danger: "error",
  warning: "warning",
}

function Sub2APIKeyDetails({
  loading,
  error,
  result,
}: {
  loading: boolean
  error?: string
  result?: PlatformSub2APIUserKeyListResponse
}) {
  const t = useI18n()
  const { locale } = useAppLocale()
  if (loading) {
    return (
      <div className="border-l-4 border-[var(--railops-primary)] bg-[var(--railops-primary-bg)] px-4 py-4">
        <ModuleLoading variant="table" count={3} label={t("platformExtract.usage.apiKeyLoading")} />
      </div>
    )
  }
  if (error) {
    return <div className="m-3 rounded-md bg-[var(--railops-error-bg)] px-3 py-2 text-xs text-[var(--railops-error)]">{error}</div>
  }
  if (!result || result.items.length === 0) {
    return <div className="grid min-h-28 place-items-center text-xs text-[var(--railops-text-secondary)]">{t("platformExtract.usage.apiKey.emptyForAccount")}</div>
  }
  const sub2APIKeyColumns: TableColumnsType<PlatformSub2APIUserKey> = [
    {
      title: t("platformExtract.usage.apiKeyColumn.nameAndKey"),
      key: "name",
      render: (_, key) => (
        <div>
          <div className="font-medium text-[var(--railops-text)]">{key.name || `Key ${key.id}`}</div>
        <code className="mt-0.5 block text-rhd-2xs text-[var(--railops-text-secondary)]">{key.keyPreview || t("platformExtract.usage.apiKey.hidden")}</code>
      </div>
    ),
  },
  {
      title: t("platformExtract.usage.apiKeyColumn.group"),
      key: "group",
      render: (_, key) => <span className="text-[var(--railops-text-secondary)]">{key.groupName || t("platformExtract.common.groupFallback", { id: key.groupId })}</span>,
    },
    {
      title: t("platformExtract.usage.apiKeyColumn.status"),
      key: "status",
      render: (_, key) => {
        const status = sub2APIStatus(t, key.status)
        return <StatusTag tone={statusPillToneMap[status.tone]} className="rhd-railops-platform-status">{status.label}</StatusTag>
      },
    },
    {
      title: t("platformExtract.usage.apiKeyColumn.quotaUsage"),
      key: "quota",
      render: (_, key) => (
        <span className="tabular-nums text-[var(--railops-text-secondary)]">
          {key.quota > 0 ? `${formatNumber(key.quotaUsed, locale)} / ${formatNumber(key.quota, locale)}` : t("platformExtract.usage.apiKey.unlimitedQuota")}
        </span>
      ),
    },
    {
      title: t("platformExtract.usage.apiKeyColumn.lastUsedAt"),
      key: "lastUsedAt",
      render: (_, key) => <span className="tabular-nums text-[var(--railops-text-secondary)]">{formatFullDateTime(key.lastUsedAt, locale, t("platformExtract.common.never"))}</span>,
    },
    {
      title: t("platformExtract.usage.apiKeyColumn.createdAt"),
      key: "createdAt",
      render: (_, key) => <span className="tabular-nums text-[var(--railops-text-secondary)]">{formatFullDateTime(key.createdAt, locale, t("platformExtract.common.never"))}</span>,
    },
  ]

  return (
    <div className="border-l-4 border-[var(--railops-primary)] bg-[var(--railops-primary-bg)] px-4 py-4">
      <div className="mb-3 flex items-center gap-2 text-xs font-semibold text-[var(--railops-text)]">
        <KeyRoundIcon className="size-3.5 text-[var(--railops-primary-hover)]" />
        {t("platformExtract.usage.apiKey.total", { count: formatNumber(result.total, locale) })}
      </div>
      <div className="rounded-md bg-[var(--railops-surface)] shadow-[var(--railops-card-shadow)]">
        <DataTable<PlatformSub2APIUserKey>
          className="w-full min-w-[860px] text-xs"
          size="small"
          rowKey={(record) => record.id}
          dataSource={result.items}
          emptyDescription={t("platformExtract.usage.apiKey.emptyForAccount")}
          columns={sub2APIKeyColumns}
          scroll={{ x: 860 }}
        />
      </div>
    </div>
  )
}

function Sub2APIAccountsPanel({
  data,
  accountInitialLoading,
  loading,
  error,
  expandedUserId,
  keyResults,
  keyErrors,
  keyLoadingUserId,
  search,
  status,
  onSearchChange,
  onStatusChange,
  onToggleUser,
  onPageChange,
  onPageSizeChange,
}: {
  data: PlatformSub2APIUserListResponse | null
  accountInitialLoading: boolean
  loading: boolean
  error: string
  expandedUserId: number | null
  keyResults: Record<number, PlatformSub2APIUserKeyListResponse>
  keyErrors: Record<number, string>
  keyLoadingUserId: number | null
  search: string
  status: string
  onSearchChange: (value: string) => void
  onStatusChange: (value: string) => void
  onToggleUser: (user: PlatformSub2APIUserListItem) => void
  onPageChange: (page: number) => void
  onPageSizeChange: (pageSize: number) => void
}) {
  const t = useI18n()
  const { locale } = useAppLocale()
  const hasAccountData = data !== null

  const accountColumns: TableColumnsType<PlatformSub2APIUserListItem> = [
    {
      title: t("platformExtract.usage.accountColumn.account"),
      key: "account",
      render: (_, user) => {
        const expanded = expandedUserId === user.id
        return (
          <div className="flex items-center gap-2">
            {expanded ? <ChevronDownIcon className="size-4 shrink-0 text-[var(--railops-primary-hover)]" /> : <ChevronRightIcon className="size-4 shrink-0 text-[var(--railops-text-secondary)]" />}
            <div className="min-w-0">
              <div className="truncate font-medium text-[var(--railops-text)]">{user.username || user.email || t("platformExtract.common.userFallback", { id: user.id })}</div>
              <div className="mt-0.5 truncate text-xs text-[var(--railops-text-secondary)]">{user.email || `Sub2API ID ${user.id}`} · {user.role}</div>
            </div>
          </div>
        )
      },
    },
    {
      title: t("platformExtract.usage.accountColumn.balance"),
      key: "balance",
      render: (_, user) => (
        <div>
          <div className="text-sm font-semibold tabular-nums text-[var(--railops-text)]">{formatBalance(user.balance, locale)}</div>
          <div className="mt-0.5 text-rhd-xs text-[var(--railops-text-secondary)]">{t("platformExtract.usage.balance.available")}</div>
        </div>
      ),
    },
    {
      title: t("platformExtract.usage.apiKeyColumn.status"),
      key: "status",
      render: (_, user) => {
        const accountStatus = sub2APIStatus(t, user.status)
        return <StatusTag tone={statusPillToneMap[accountStatus.tone]} className="rhd-railops-platform-status">{accountStatus.label}</StatusTag>
      },
    },
    {
      title: t("platformExtract.usage.accountColumn.lastActiveAt"),
      key: "lastActiveAt",
      render: (_, user) => <span className="text-xs tabular-nums text-[var(--railops-text-secondary)]">{formatFullDateTime(user.lastActiveAt, locale, t("platformExtract.common.never"))}</span>,
    },
    {
      title: t("platformExtract.usage.accountColumn.lastUsedAt"),
      key: "lastUsedAt",
      render: (_, user) => <span className="text-xs tabular-nums text-[var(--railops-text-secondary)]">{formatFullDateTime(user.lastUsedAt, locale, t("platformExtract.common.never"))}</span>,
    },
    {
      title: t("platformExtract.usage.apiKeyColumn.createdAt"),
      key: "createdAt",
      render: (_, user) => <span className="text-xs tabular-nums text-[var(--railops-text-secondary)]">{formatFullDateTime(user.createdAt, locale, t("platformExtract.common.never"))}</span>,
    },
    {
      title: t("platformExtract.usage.accountColumn.apiKeys"),
      key: "keys",
      align: "right",
      render: (_, user) => {
        const keyResult = keyResults[user.id]
        return <span className="text-xs font-medium text-[var(--railops-primary-hover)]">{keyResult ? t("platformExtract.usage.apiKey.countUnit", { count: formatNumber(keyResult.total, locale) }) : t("platformExtract.common.view")}</span>
      },
    },
  ]

  return (
    <ContentModule title={t("platformExtract.usage.sub2apiAccountsTitle")} className="rhd-railops-platform-panel">
      <div className="flex flex-col gap-3 border-b border-[var(--railops-border-light)] p-3 lg:flex-row lg:items-center lg:justify-between">
        <div className="flex min-w-0 flex-1 flex-col gap-2 sm:flex-row">
          <SearchField
            allowClear
            className="rhd-railops-search-wide"
            placeholder={t("platformExtract.usage.accountSearchPlaceholder")}
            value={search}
            onChange={(event) => onSearchChange(event.target.value)}
          />
          <SelectField
            style={{ marginBottom: 0 }}
            selectProps={{
              "aria-label": t("platformExtract.usage.accountStatusAria"),
              value: status,
              onChange: (value) => onStatusChange((value as string) ?? ""),
              options: [
                { value: "", label: t("platformExtract.common.all") },
                { value: "active", label: t("platformExtract.common.status.active") },
                { value: "disabled", label: t("platformExtract.common.disabled") },
                { value: "banned", label: t("platformExtract.common.restricted") },
              ],
              style: { width: 132 },
            }}
          />
        </div>
        <div className="flex shrink-0 items-center gap-2 text-xs text-[var(--railops-text-secondary)]">
          <UsersIcon className="size-3.5" />
          {data ? t("platformExtract.usage.accountTotal", { count: formatNumber(data.total, locale) }) : t("platformExtract.common.waitingForSync")}
        </div>
      </div>

      {accountInitialLoading ? (
        <ModuleLoading variant="table" count={5} label={t("platformExtract.usage.accountsLoading")} />
      ) : error && !hasAccountData ? (
        <div className="m-4 rounded-md bg-[var(--railops-error-bg)] px-3 py-3 text-xs text-[var(--railops-error)]">{error}</div>
      ) : !hasAccountData || data.items.length === 0 ? (
        <PlatformEmpty title={t("platformExtract.usage.noSub2apiAccounts")} />
      ) : (
        <>
          {error ? <div className="border-b border-[var(--railops-border-light)] bg-[var(--railops-surface-muted)] px-3 py-2 text-xs text-[var(--railops-text-secondary)]">{error}</div> : null}
          <DataTable<PlatformSub2APIUserListItem>
            className="w-full min-w-[1120px]"
            size="small"
            rowKey={(record) => record.id}
            dataSource={data.items}
            loading={loading}
            emptyDescription={t("platformExtract.usage.noSub2apiAccounts")}
            columns={accountColumns}
            scroll={{ x: 1120 }}
            expandable={{
              expandedRowRender: (record) => (
                <Sub2APIKeyDetails
                  loading={keyLoadingUserId === record.id}
                  error={keyErrors[record.id]}
                  result={keyResults[record.id]}
                />
              ),
              expandedRowKeys: expandedUserId !== null ? [expandedUserId] : [],
              onExpand: (_expanded, record) => onToggleUser(record),
              showExpandColumn: false,
            }}
            onRow={(record) => ({
              className: cn("cursor-pointer", expandedUserId === record.id ? "bg-[var(--railops-primary-bg)]" : ""),
              tabIndex: 0,
              onClick: () => onToggleUser(record),
              onKeyDown: (event) => {
                if (event.key === "Enter" || event.key === " ") {
                  event.preventDefault()
                  onToggleUser(record)
                }
              },
            })}
          />
          <div className="border-t border-[var(--railops-border-light)] px-4 py-3">
            <TablePagination
              current={data.page}
              pageSize={data.pageSize || SUB2API_ACCOUNT_PAGE_SIZE}
              total={data.total}
              showSizeChanger
              pageSizeOptions={[10, 20, 50]}
              onChange={(page, size) => {
                onPageChange(page)
                if (size !== (data.pageSize || SUB2API_ACCOUNT_PAGE_SIZE)) {
                  onPageSizeChange(size)
                }
              }}
            />
          </div>
        </>
      )}
    </ContentModule>
  )
}

export default function PlatformUsagePage() {
  const [data, setData] = useState<PlatformUsageOverview | null>(null)
  const { locale } = useAppLocale()
  const t = useI18n()
  const [usageLoading, setUsageLoading] = useState(true)
  const [hasLoadedUsage, setHasLoadedUsage] = useState(false)
  const [error, setError] = useState("")
  const [sub2APIData, setSub2APIData] = useState<PlatformSub2APIUserListResponse | null>(null)
  const [sub2APILoading, setSub2APILoading] = useState(true)
  const [sub2APIDataLoaded, setSub2APIDataLoaded] = useState(false)
  const [sub2APIError, setSub2APIError] = useState("")
  const [accountSearch, setAccountSearch] = useState("")
  const [accountStatus, setAccountStatus] = useState("")
  const [appliedAccountSearch, setAppliedAccountSearch] = useState("")
  const [appliedAccountStatus, setAppliedAccountStatus] = useState("")
  const [accountPageSize, setAccountPageSize] = useState(SUB2API_ACCOUNT_PAGE_SIZE)
  const [expandedUserId, setExpandedUserId] = useState<number | null>(null)
  const [keyResults, setKeyResults] = useState<Record<number, PlatformSub2APIUserKeyListResponse>>({})
  const [keyErrors, setKeyErrors] = useState<Record<number, string>>({})
  const [keyLoadingUserId, setKeyLoadingUserId] = useState<number | null>(null)

  const load = useCallback(async () => {
    setUsageLoading(true)
    setError("")
    try {
      setData(await fetchPlatformUsage())
      setHasLoadedUsage(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : t("platformExtract.usage.meteringUnavailable"))
    } finally {
      setUsageLoading(false)
    }
  }, [])

  const loadSub2APIUsers = useCallback(async (page = 1, search = "", status = "", pageSize = accountPageSize) => {
    setSub2APILoading(true)
    setSub2APIError("")
    try {
      const result = await fetchPlatformSub2APIUsers({
        page,
        pageSize,
        search: search.trim(),
        status,
        role: "",
        timezone: "Asia/Shanghai",
        sortBy: "created_at",
        sortOrder: "desc",
      })
      setSub2APIData(result)
      setSub2APIDataLoaded(true)
      setExpandedUserId(null)
    } catch (err) {
      setSub2APIError(err instanceof Error ? err.message : t("platformExtract.usage.sub2apiUnavailable"))
    } finally {
      setSub2APILoading(false)
    }
  }, [accountPageSize])

  useEffect(() => {
    const frame = window.requestAnimationFrame(() => {
      void load()
    })
    return () => window.cancelAnimationFrame(frame)
  }, [load])

  const hasData = data !== null
  const summaryInitialLoading = usageLoading && !hasLoadedUsage
  const trendInitialLoading = usageLoading && !hasLoadedUsage
  const usageRefreshing = usageLoading || sub2APILoading
  const sharedTranslationUsage = data?.sharedTranslationUsage

  // Debounced live search, consistent with the ticket center.
  // Sole account-list loader: also handles the initial load on mount.
  useEffect(() => {
    const timer = window.setTimeout(() => {
      const search = accountSearch.trim()
      setAppliedAccountSearch(search)
      setAppliedAccountStatus(accountStatus)
      void loadSub2APIUsers(1, search, accountStatus, accountPageSize)
    }, 320)
    return () => window.clearTimeout(timer)
  }, [accountSearch, accountStatus, loadSub2APIUsers])

  function toggleSub2APIUser(user: PlatformSub2APIUserListItem) {
    if (expandedUserId === user.id) {
      setExpandedUserId(null)
      return
    }
    setExpandedUserId(user.id)
    if (keyResults[user.id]) return

    setKeyLoadingUserId(user.id)
    setKeyErrors((current) => ({ ...current, [user.id]: "" }))
    void fetchPlatformSub2APIUserKeys(user.id)
      .then((result) => {
        setKeyResults((current) => ({ ...current, [user.id]: result }))
      })
      .catch((err) => {
        setKeyErrors((current) => ({
          ...current,
          [user.id]: err instanceof Error ? err.message : t("platformExtract.usage.apiKeyLoadFailed"),
        }))
      })
      .finally(() => setKeyLoadingUserId((current) => current === user.id ? null : current))
  }

  return (
    <PageShell
      title={t("platformExtract.usage.pageTitle")}
      breadcrumb={useRouteBreadcrumbItems()}
      className="min-h-[calc(100vh-97px)]"
      actions={
        <>
          <IconButton
            icon={<RefreshCwIcon className={usageRefreshing ? "animate-spin" : ""} />}
            tooltip={t("platformExtract.common.refresh")}
            aria-label={t("platformExtract.common.refresh")}
            onClick={() => void Promise.all([load(), loadSub2APIUsers(sub2APIData?.page || 1, appliedAccountSearch, appliedAccountStatus, accountPageSize)])}
            disabled={usageRefreshing}
          />
        </>
      }
    >

      <div className="space-y-4">
        {!usageLoading && error && !hasData ? <PlatformError message={error} onRetry={() => void load()} /> : null}
        {error && hasData ? <div className="rounded-md bg-[var(--railops-surface-muted)] px-3 py-2 text-xs text-[var(--railops-text-secondary)]">{error}</div> : null}

        {summaryInitialLoading ? (
          <section className="grid gap-3 sm:grid-cols-2 xl:grid-cols-5" aria-label={t("platformExtract.usage.metricsAria")}>
            <StatCard label={t("platformExtract.usage.metric.tenants")} value="-" note={t("platformExtract.common.loading")} icon={<Building2Icon className="size-4" />} tone="neutral" className="rhd-railops-platform-metric" />
            <StatCard label={t("platformExtract.usage.metric.aiRequests")} value="-" note={t("platformExtract.common.loading")} icon={<CpuIcon className="size-4" />} tone="blue" className="rhd-railops-platform-metric" />
            <StatCard label={t("platformExtract.usage.metric.aiCost")} value="-" note={t("platformExtract.common.loading")} icon={<CircleDollarSignIcon className="size-4" />} tone="blue" className="rhd-railops-platform-metric" />
            <StatCard label={t("platformExtract.usage.metric.atRisk")} value="-" note={t("platformExtract.common.loading")} icon={<GaugeIcon className="size-4" />} tone="warning" className="rhd-railops-platform-metric" />
            <StatCard label={t("platformExtract.usage.metric.overLimit")} value="-" note={t("platformExtract.common.loading")} icon={<TriangleAlertIcon className="size-4" />} tone="error" className="rhd-railops-platform-metric" />
          </section>
        ) : (
          <section className="grid gap-3 sm:grid-cols-2 xl:grid-cols-5" aria-label={t("platformExtract.usage.metricsAria")}>
            <StatCard label={t("platformExtract.usage.metric.tenants")} value={data ? formatNumber(data.summary.tenantCount, locale) : "-"} note={data ? t("platformExtract.usage.metric.tenantCountNote") : t("platformExtract.common.statusUnavailable")} icon={<Building2Icon className="size-4" />} tone="neutral" className="rhd-railops-platform-metric" />
            <StatCard label={t("platformExtract.usage.metric.aiRequests")} value={data ? formatCompactNumber(data.summary.aiRequests, locale) : "-"} note={data ? `${formatCompactNumber(data.summary.aiTokens, locale)} Token` : t("platformExtract.common.statusUnavailable")} icon={<CpuIcon className="size-4" />} tone="blue" className="rhd-railops-platform-metric" />
            <StatCard label={t("platformExtract.usage.metric.aiCost")} value={data ? formatCurrency(data.summary.costAmount, locale) : "-"} note={data ? t("platformExtract.usage.metric.monthlyCostNote") : t("platformExtract.common.statusUnavailable")} icon={<CircleDollarSignIcon className="size-4" />} tone="blue" className="rhd-railops-platform-metric" />
            <StatCard label={t("platformExtract.usage.metric.atRisk")} value={data ? formatNumber(data.summary.atRiskTenantCount, locale) : "-"} note={data ? t("platformExtract.usage.metric.usageAtRiskNote") : t("platformExtract.common.statusUnavailable")} icon={<GaugeIcon className="size-4" />} tone="warning" className="rhd-railops-platform-metric" />
            <StatCard label={t("platformExtract.usage.metric.overLimit")} value={data ? formatNumber(data.summary.overLimitCount, locale) : "-"} note={data ? t("platformExtract.usage.metric.quotaReachedNote") : t("platformExtract.common.statusUnavailable")} icon={<TriangleAlertIcon className="size-4" />} tone="error" className="rhd-railops-platform-metric" />
          </section>
        )}

        {!summaryInitialLoading ? (
          <ContentModule title={t("platformExtract.usage.sharedTranslationTitle")} className="rhd-railops-platform-panel">
            <section className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4" aria-label={t("platformExtract.usage.sharedTranslationAria")}>
              <StatCard
                label={t("platformExtract.usage.sharedTranslation.key")}
                value={sharedTranslationUsage?.configured ? (sharedTranslationUsage.apiKeyMasked || t("platformExtract.common.enabled")) : t("platformExtract.common.notConfigured")}
                note={sharedTranslationUsage?.fingerprint ? t("platformExtract.usage.sharedTranslation.fingerprint", { fingerprint: fingerprintLabel(sharedTranslationUsage.fingerprint) }) : t("platformExtract.usage.sharedTranslation.notConfiguredNote")}
                icon={<KeyRoundIcon className="size-4" />}
                tone={sharedTranslationUsage?.configured ? "blue" : "neutral"}
                className="rhd-railops-platform-metric"
              />
              <StatCard
                label={t("platformExtract.usage.sharedTranslation.requests")}
                value={sharedTranslationUsage ? formatCompactNumber(sharedTranslationUsage.requests, locale) : "-"}
                note={t("platformExtract.usage.sharedTranslation.monthToDate")}
                icon={<CpuIcon className="size-4" />}
                tone="blue"
                className="rhd-railops-platform-metric"
              />
              <StatCard
                label={t("platformExtract.usage.sharedTranslation.tokens")}
                value={sharedTranslationUsage ? formatCompactNumber(sharedTranslationUsage.tokens, locale) : "-"}
                note={t("platformExtract.usage.sharedTranslation.monthToDate")}
                icon={<GaugeIcon className="size-4" />}
                tone="neutral"
                className="rhd-railops-platform-metric"
              />
              <StatCard
                label={t("platformExtract.usage.sharedTranslation.lastUsed")}
                value={formatFullDateTime(sharedTranslationUsage?.lastMeteredAt, locale, t("platformExtract.common.never"))}
                note={sharedTranslationUsage ? formatCurrency(sharedTranslationUsage.costAmount, locale) : t("platformExtract.common.statusUnavailable")}
                icon={<CircleDollarSignIcon className="size-4" />}
                tone="neutral"
                className="rhd-railops-platform-metric"
              />
            </section>
          </ContentModule>
        ) : null}

        <Sub2APIAccountsPanel
          data={sub2APIData}
          accountInitialLoading={sub2APILoading && !sub2APIDataLoaded}
          loading={sub2APILoading}
          error={sub2APIError}
          expandedUserId={expandedUserId}
          keyResults={keyResults}
          keyErrors={keyErrors}
          keyLoadingUserId={keyLoadingUserId}
          search={accountSearch}
          status={accountStatus}
          onSearchChange={setAccountSearch}
          onStatusChange={setAccountStatus}
          onToggleUser={toggleSub2APIUser}
          onPageChange={(page) => {
            void loadSub2APIUsers(page, appliedAccountSearch, appliedAccountStatus, accountPageSize)
          }}
          onPageSizeChange={(pageSize) => {
            setAccountPageSize(pageSize)
            void loadSub2APIUsers(1, appliedAccountSearch, appliedAccountStatus, pageSize)
          }}
        />

        <ContentModule title={t("platformExtract.usage.trendTitle")} className="rhd-railops-platform-panel">
          {trendInitialLoading ? (
            <ModuleLoading variant="detail" count={3} label={t("platformExtract.usage.trendLoading")} />
          ) : !data || data.dailyUsage.length === 0 ? (
            <PlatformEmpty title={t("platformExtract.usage.noTrend")} />
          ) : (
            <div className="px-4 pb-4 pt-5">
              <ChartContainer
                height={176}
                option={{
                  xAxis: {
                    type: "category",
                    data: data.dailyUsage.map((item) => item.date.slice(5).replace("-", "/")),
                  },
                  yAxis: { type: "value" },
                  series: [
                    {
                      type: "bar",
                      data: data.dailyUsage.map((item) => item.aiRequests),
                      itemStyle: { color: "#3B82F6" },
                      label: {
                        show: true,
                        position: "top",
                        formatter: (params: { value: unknown }) => formatCompactNumber(Number(params.value)),
                        fontSize: 12,
                        color: "#64748B",
                      },
                    },
                  ],
                }}
              />
            </div>
          )}
        </ContentModule>
      </div>

    </PageShell>
  )
}
