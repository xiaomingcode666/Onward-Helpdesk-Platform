"use client"

import { useEffect, useMemo, useState, type ReactNode } from "react"
import { Input, Skeleton } from "antd"
import type { TableColumnsType } from "antd"
import {
  ActivityIcon,
  CalendarDaysIcon,
  CircleDollarSignIcon,
  GaugeIcon,
  EyeIcon,
  KeyRoundIcon,
  PencilIcon,
  RefreshCwIcon,
  RotateCcwIcon,
  SaveIcon,
  TrendingUpIcon,
  WalletCardsIcon,
} from "lucide-react"
import { toast } from "sonner"

import { EnterpriseAIFeatureGuard } from "@/components/enterprise/ai-feature-guard"
import { useAuth } from "@/components/auth-provider"
import { useConfirm } from "@/components/confirm-provider"
import { CanUseButton } from "@/components/layout/permission-guard"
import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import { ProjectDialog } from "@/components/project-dialog"
import {
  RailopsDateRangeFilter,
  RAILOPS_DEFAULT_TIMEZONE,
  buildRailopsDateRangeItems,
  getRailopsDateRange,
  type RailopsDateRangeValue,
} from "@/components/railops/date-range-filter"
import { formatCurrency, formatDateTime, formatNumber } from "@/components/platform/platform-live-ui"
import { useI18n } from "@/i18n/provider"
import { getEnterpriseAIKeyDisplayName, getEnterpriseAIKeyProductMeta } from "@/lib/enterprise-ai-key-display"
import {
  getEnterpriseAIModelsWorkspace,
  resetEnterpriseAIKeyQuota,
  updateEnterpriseAIKeyQuota,
  type EnterpriseAIKey,
  type EnterpriseAIModelWorkspace,
  type EnterpriseAIModelsQuery,
  type EnterpriseAISubscription,
} from "@/lib/api/enterprise-models"
import { ContentModule, DataTable, EmptyState, FilterTabs, IconButton, PageShell, RailopsButton, SearchField, SelectField, StatusTag, TableToolbar, type RailopsTabItem, type StatusTagTone } from "@railops/ui"

type I18nT = ReturnType<typeof useI18n>
type UsageTab = "trend" | "keys"
type KeyStatusFilter = "all" | "active" | "inactive" | "pending"
type UsageKpiTone = "blue" | "green" | "amber" | "slate"

function statusLabel(t: I18nT, status?: string) {
  const normalized = (status || "").toLowerCase()
  if (normalized === "active" || normalized === "success" || normalized === "ok") return t("enterpriseModels.state.active")
  if (normalized === "inactive" || normalized === "disabled") return t("enterpriseModels.state.inactive")
  if (normalized === "pending" || normalized === "unknown") return t("enterpriseModels.state.pending")
  return status || t("enterpriseModels.state.unknown")
}

function keyStatusFilterLabel(t: I18nT, status: KeyStatusFilter) {
  if (status === "all") return t("enterpriseModels.apiKeys.statusFilters.all")
  if (status === "active") return t("enterpriseModels.apiKeys.statusFilters.active")
  if (status === "inactive") return t("enterpriseModels.apiKeys.statusFilters.inactive")
  return t("enterpriseModels.apiKeys.statusFilters.pending")
}

export default function EnterpriseModelsPage() {
  return (
    <EnterpriseAIFeatureGuard title="Usage">
      <EnterpriseModelsPageContent />
    </EnterpriseAIFeatureGuard>
  )
}

export function EnterpriseModelsPageContent() {
  const t = useI18n()
  const { session } = useAuth()
  const confirm = useConfirm()
  const canUpdateKeys = CanUseButton("productAIUsageCredential.update", session?.permissions)
  const [query, setQuery] = useState<EnterpriseAIModelsQuery>(() => defaultQuery())
  const [range, setRange] = useState<RailopsDateRangeValue>("7d")
  const [activeUsageTab, setActiveUsageTab] = useState<UsageTab>("trend")
  const [keyStatus, setKeyStatus] = useState<KeyStatusFilter>("all")
  const [keyword, setKeyword] = useState("")
  const [workspace, setWorkspace] = useState<EnterpriseAIModelWorkspace | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [editingKey, setEditingKey] = useState<EnterpriseAIKey | null>(null)
  const [detailKey, setDetailKey] = useState<EnterpriseAIKey | null>(null)
  const [quotaValue, setQuotaValue] = useState("")
  const [savingQuota, setSavingQuota] = useState(false)
  const [resettingKeyId, setResettingKeyId] = useState<number | null>(null)

  async function load(nextQuery = query) {
    setLoading(true)
    setError("")
    const response = await getEnterpriseAIModelsWorkspace(nextQuery)
    if (!response.success) {
      setLoading(false)
      setError(response.error?.message || t("enterpriseModels.loadFailed"))
      return
    }
    setWorkspace(response.data)
    setLoading(false)
  }

  useEffect(() => {
    void load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const stats = workspace?.stats
  const user = workspace?.user
  const account = workspace?.account
  const activeSubscription = workspace?.activeSubscription
  const waitingForData = !workspace
  const maxTrendCost = useMemo(() => {
    return Math.max(...(workspace?.trend ?? []).map((item) => item.actualCost), 1)
  }, [workspace?.trend])
  const filteredKeys = useMemo(() => {
    const term = keyword.trim().toLowerCase()
    return (workspace?.keys ?? []).filter((key) => {
      const normalizedStatus = (key.status || "").toLowerCase()
      const statusMatched =
        keyStatus === "all" ||
        normalizedStatus === keyStatus ||
        (keyStatus === "inactive" && normalizedStatus === "disabled") ||
        (keyStatus === "pending" && normalizedStatus === "unknown")
      if (!statusMatched) return false
      if (!term) return true
      const haystack = [
        getEnterpriseAIKeyDisplayName(key, t("enterpriseModels.apiKeys.keyPrefix", { id: key.id })),
        getEnterpriseAIKeyProductMeta(key),
        key.keyPreview,
        key.name,
        key.productName,
        key.productCode,
        key.group?.name,
        key.group?.platform,
      ]
        .filter(Boolean)
        .join(" ")
        .toLowerCase()
      return haystack.includes(term)
    })
  }, [keyStatus, keyword, t, workspace?.keys])
  const usageTabItems = useMemo<RailopsTabItem[]>(() => [
    {
      label: t("enterpriseModels.trend.title"),
      value: "trend",
      count: workspace?.trend.length ?? 0,
    },
    {
      label: t("enterpriseModels.apiKeys.title"),
      value: "keys",
      count: filteredKeys.length,
    },
  ], [filteredKeys.length, t, workspace?.trend.length])
  const usageRangeItems = useMemo(() => buildRailopsDateRangeItems({
    "7d": t("enterpriseModels.filters.range7d"),
    "14d": t("enterpriseModels.filters.range14d"),
    "30d": t("enterpriseModels.filters.range30d"),
    custom: t("enterpriseModels.filters.custom"),
  }), [t])
  const editingKeyProductMeta = editingKey ? getEnterpriseAIKeyProductMeta(editingKey) : ""

  const keyColumns = useMemo<TableColumnsType<EnterpriseAIKey>>(() => [
    {
      title: t("enterpriseModels.apiKeys.columns.key"),
      dataIndex: "name",
      key: "key",
      width: 230,
      render: (_, key) => (
        <div className="rhd-railops-table-cell">
          <strong>{getEnterpriseAIKeyDisplayName(key, t("enterpriseModels.apiKeys.keyPrefix", { id: key.id }))}</strong>
        </div>
      ),
    },
    {
      title: t("enterpriseModels.apiKeys.columns.modelGroup"),
      dataIndex: "groupId",
      key: "modelGroup",
      width: 160,
      render: (_, key) => (
        <div className="rhd-railops-table-cell">
          <strong>{key.group?.name || t("enterpriseModels.apiKeys.groupPrefix", { id: key.groupId })}</strong>
          <span>{t("enterpriseModels.apiKeys.groupMeta", { platform: key.group?.platform || "-", multiplier: key.group?.rateMultiplier ?? 1 })}</span>
        </div>
      ),
    },
    {
      title: t("enterpriseModels.apiKeys.columns.quota"),
      dataIndex: "quotaUsagePercent",
      key: "quota",
      width: 210,
      render: (_, key) => <QuotaCell item={key} t={t} />,
    },
    {
      title: t("enterpriseModels.apiKeys.columns.status"),
      dataIndex: "status",
      key: "status",
      width: 88,
      align: "center",
      render: (_, key) => <StatusTag tone={statusTone(key.status)}>{statusLabel(t, key.status)}</StatusTag>,
    },
    {
      title: t("enterpriseModels.apiKeys.columns.lastUsed"),
      dataIndex: "lastUsedAt",
      key: "lastUsedAt",
      width: 136,
      render: (_, key) => (
        <div className="rhd-railops-table-cell">
          <strong>{formatDateTime(key.lastUsedAt)}</strong>
          <span>{key.lastUsedIp || "-"}</span>
        </div>
      ),
    },
    {
      title: t("enterpriseModels.apiKeys.columns.actions"),
      key: "actions",
      width: 96,
      align: "center",
      render: (_, key) => (
        <div className="rhd-railops-usage-row-actions">
          <IconButton
            className="rhd-railops-row-action"
            icon={<EyeIcon className="size-3.5" />}
            tooltip={t("enterpriseModels.apiKeys.detailAction")}
            aria-label={t("enterpriseModels.apiKeys.detailAction")}
            onClick={() => setDetailKey(key)}
          />
          {canUpdateKeys ? (
            <>
              <IconButton
                className="rhd-railops-row-action"
                icon={<PencilIcon className="size-3.5" />}
                tooltip={t("enterpriseModels.apiKeys.editQuota")}
                aria-label={t("enterpriseModels.apiKeys.editQuota")}
                onClick={() => openQuotaDialog(key)}
              />
              <IconButton
                className="rhd-railops-row-action is-danger"
                danger
                icon={<RotateCcwIcon className="size-3.5" />}
                loading={resettingKeyId === key.id}
                tooltip={t("enterpriseModels.apiKeys.resetUsage")}
                aria-label={t("enterpriseModels.apiKeys.resetUsage")}
                onClick={() => void handleResetQuota(key)}
              />
            </>
          ) : null}
        </div>
      ),
    },
  ], [canUpdateKeys, query, resettingKeyId, t])

  function openQuotaDialog(key: EnterpriseAIKey) {
    setEditingKey(key)
    setQuotaValue(String(key.quota ?? 0))
  }

  async function submitQuota() {
    if (!editingKey) return
    const quota = Number(quotaValue)
    if (Number.isNaN(quota) || quota < 0) {
      setError(t("enterpriseModels.invalidQuota"))
      return
    }
    setSavingQuota(true)
    const response = await updateEnterpriseAIKeyQuota(editingKey.id, quota, query)
    setSavingQuota(false)
    if (!response.success) {
      setError(response.error?.message || t("enterpriseModels.quotaUpdateFailed"))
      return
    }
    setWorkspace(response.data)
    setEditingKey(null)
    setError("")
  }

  async function handleResetQuota(key: EnterpriseAIKey) {
    const keyLabel = getEnterpriseAIKeyDisplayName(key, t("enterpriseModels.apiKeys.keyPrefix", { id: key.id })) || key.keyPreview || `Key #${key.id}`
    const accepted = await confirm({
      title: t("enterpriseModels.resetQuotaTitle"),
      description: t("enterpriseModels.resetQuotaDescription", { keyLabel }),
      confirmText: t("enterpriseModels.resetUsage"),
      cancelText: t("enterpriseModels.cancel"),
      variant: "destructive",
    })
    if (!accepted) return

    setResettingKeyId(key.id)
    try {
      const response = await resetEnterpriseAIKeyQuota(key.id, query)
      if (!response.success) {
        throw new Error(response.error?.message || t("enterpriseModels.resetQuotaFailed"))
      }
      setWorkspace(response.data)
      setError("")
      toast.success(t("enterpriseModels.resetQuotaSuccess", { keyLabel }))
    } catch (err) {
      const message = err instanceof Error ? err.message : t("enterpriseModels.resetQuotaFailed")
      setError(message)
      toast.error(message)
    } finally {
      setResettingKeyId(null)
    }
  }

  function handleRangeChange(value: string) {
    const nextRange = value as RailopsDateRangeValue
    setRange(nextRange)
    if (nextRange === "custom") return
    const nextQuery = {
      ...query,
      ...getRailopsDateRange(nextRange),
      page: 1,
    }
    setQuery(nextQuery)
    void load(nextQuery)
  }

  function updateDateQuery(field: "startDate" | "endDate", value: string) {
    setRange("custom")
    setQuery((prev) => ({ ...prev, [field]: value, page: 1 }))
  }

  function applyQuery() {
    const nextQuery = { ...query, page: 1 }
    setQuery(nextQuery)
    void load(nextQuery)
  }

  return (
    <PageShell
      className="rhd-railops-usage-page"
      title={t("enterpriseModels.pageTitle")}
      breadcrumb={useRouteBreadcrumbItems()}
      actions={
        <RailopsButton onClick={() => void load()} disabled={loading}>
          <RefreshCwIcon data-icon="inline-start" className={loading ? "animate-spin" : ""} />
          {t("enterpriseModels.refresh")}
        </RailopsButton>
      }
    >

      {error ? (
        <div className="rhd-railops-refresh-error" role="alert">
          <span>{error}</span>
          {!workspace ? (
            <RailopsButton size="small" onClick={() => void load()} disabled={loading}>
              <RefreshCwIcon data-icon="inline-start" className={loading ? "animate-spin" : ""} />
              {t("enterpriseModels.reload")}
            </RailopsButton>
          ) : null}
        </div>
      ) : null}

      <div className="rhd-railops-usage-kpis">
        <UsageKpi
          label={t("enterpriseModels.metrics.balance")}
          value={formatCurrency(user?.balance ?? account?.balance ?? 0)}
          detail={t("enterpriseModels.metrics.accountDetail", { email: account?.loginEmail || user?.email || "-" })}
          icon={<WalletCardsIcon className="size-4" />}
          loading={waitingForData}
          tone="blue"
        />
        <UsageKpi
          label={t("enterpriseModels.metrics.subscription")}
          value={subscriptionDurationLabel(t, activeSubscription)}
          detail={subscriptionDetailLabel(t, activeSubscription)}
          icon={<CalendarDaysIcon className="size-4" />}
          loading={waitingForData}
          tone="green"
        />
        <UsageKpi
          label={t("enterpriseModels.metrics.todayCost")}
          value={formatCurrency(stats?.todayActualCost ?? 0)}
          detail={t("enterpriseModels.metrics.todayCostDetail", {
            requests: formatNumber(stats?.todayRequests ?? 0),
            tokens: formatNumber(stats?.todayTokens ?? 0),
          })}
          icon={<CircleDollarSignIcon className="size-4" />}
          loading={waitingForData}
          tone="amber"
        />
        <UsageKpi
          label={t("enterpriseModels.metrics.totalCost")}
          value={formatCurrency(stats?.totalActualCost ?? 0)}
          detail={t("enterpriseModels.metrics.totalCostDetail", {
            requests: formatNumber(stats?.totalRequests ?? 0),
            tokens: formatNumber(stats?.totalTokens ?? 0),
          })}
          icon={<TrendingUpIcon className="size-4" />}
          loading={waitingForData}
          tone="blue"
        />
        <UsageKpi
          label={t("enterpriseModels.metrics.apiKey")}
          value={`${formatNumber(stats?.activeApiKeys ?? 0)} / ${formatNumber(stats?.totalApiKeys ?? 0)}`}
          detail={t("enterpriseModels.metrics.defaultKey", { status: account?.defaultKeyStatus || "-" })}
          icon={<KeyRoundIcon className="size-4" />}
          loading={waitingForData}
          tone="slate"
        />
        <UsageKpi
          label={t("enterpriseModels.metrics.throughput")}
          value={`${formatNumber(stats?.rpm ?? 0)} RPM`}
          detail={t("enterpriseModels.metrics.throughputDetail", {
            tpm: formatNumber(stats?.tpm ?? 0),
            concurrency: formatNumber(user?.currentConcurrency ?? 0),
          })}
          icon={<GaugeIcon className="size-4" />}
          loading={waitingForData}
          tone="blue"
        />
      </div>

      <section className="rhd-railops-usage-tabs-shell">
        <div className="rhd-railops-usage-tabs-bar">
          <FilterTabs
            ariaLabel={t("enterpriseModels.pageTitle")}
            items={usageTabItems}
            value={activeUsageTab}
            onChange={(value) => setActiveUsageTab(value as UsageTab)}
          />
          <RailopsDateRangeFilter
            ariaLabel={t("enterpriseModels.filters.range")}
            applyLabel={t("enterpriseModels.query")}
            className="rhd-railops-usage-range-filter"
            endDate={query.endDate || ""}
            endLabel={t("enterpriseModels.filters.endDate")}
            items={usageRangeItems}
            loading={loading}
            startDate={query.startDate || ""}
            startLabel={t("enterpriseModels.filters.startDate")}
            summary={`${query.startDate || "-"} ~ ${query.endDate || "-"}`}
            title={t("enterpriseModels.filters.range")}
            value={range}
            onApply={applyQuery}
            onChange={handleRangeChange}
            onEndDateChange={(value) => updateDateQuery("endDate", value)}
            onStartDateChange={(value) => updateDateQuery("startDate", value)}
          />
        </div>

        {activeUsageTab === "trend" ? (
          <section className="rhd-railops-usage-grid">
            <ContentModule
              className="rhd-railops-usage-trend-module"
              title={t("enterpriseModels.trend.title")}
            >
              {waitingForData ? (
                <TrendLoading />
              ) : (workspace?.trend ?? []).length === 0 ? (
                <EmptyBlock title={t("enterpriseModels.trend.emptyTitle")} />
              ) : (
                <div className="rhd-railops-usage-trend-list">
                  {workspace?.trend.map((item) => (
                    <div key={item.date} className="rhd-railops-usage-trend-row">
                      <div className="rhd-railops-usage-trend-date">
                        <strong>{item.date}</strong>
                        <span>{t("enterpriseModels.units.requests", { count: formatNumber(item.requests) })}</span>
                      </div>
                      <div className="rhd-railops-usage-trend-meter">
                        <div>
                          <span style={{ width: `${Math.max(4, Math.min((item.actualCost / maxTrendCost) * 100, 100))}%` }} />
                        </div>
                        <small>{t("enterpriseModels.units.tokens", { count: formatNumber(item.totalTokens) })}</small>
                      </div>
                      <div className="rhd-railops-usage-trend-cost">
                        <strong>{formatCurrency(item.actualCost)}</strong>
                        <span>{t("enterpriseModels.trend.billing", { value: formatCurrency(item.cost) })}</span>
                      </div>
                    </div>
                  ))}
                </div>
              )}

              {!waitingForData && (stats?.byPlatform ?? []).length > 0 ? (
                <div className="rhd-railops-usage-platform-strip">
                  {stats?.byPlatform.map((item) => (
                    <div key={item.platform} className="rhd-railops-usage-platform-item">
                      <ActivityIcon className="size-3.5" />
                      <strong>{item.platform || t("enterpriseModels.unknown")}</strong>
                      <span>{t("enterpriseModels.units.requests", { count: formatNumber(item.totalRequests) })}</span>
                      <span>{formatCurrency(item.todayActualCost)}</span>
                    </div>
                  ))}
                </div>
              ) : null}
            </ContentModule>

            <ContentModule
              className="rhd-railops-usage-account-module"
              title={t("enterpriseModels.account.title")}
              extra={!waitingForData ? <StatusTag tone={statusTone(user?.status || account?.accountStatus)}>{statusLabel(t, user?.status || account?.accountStatus)}</StatusTag> : null}
            >
              <div className="rhd-railops-usage-account-grid">
                <InfoRow label={t("enterpriseModels.account.user")} value={waitingForData ? <InlineLoading /> : user?.username || "-"} />
                <InfoRow label={t("enterpriseModels.account.loginEmail")} value={waitingForData ? <InlineLoading className="w-32" /> : user?.email || account?.loginEmail || "-"} />
                <InfoRow label={t("enterpriseModels.account.concurrencyLimit")} value={waitingForData ? <InlineLoading /> : formatNumber(user?.concurrency ?? account?.concurrency ?? 0)} />
                <InfoRow label={t("enterpriseModels.account.rpmLimit")} value={waitingForData ? <InlineLoading /> : formatNumber(user?.rpmLimit ?? account?.rpmLimit ?? 0)} />
                <InfoRow label={t("enterpriseModels.account.subscriptionPeriod")} value={waitingForData ? <InlineLoading className="w-28" /> : subscriptionPeriodLabel(t, activeSubscription)} />
                <InfoRow label={t("enterpriseModels.account.tokenExpires")} value={waitingForData ? <InlineLoading className="w-24" /> : formatDateTime(account?.accessTokenExpiresAt)} />
                <InfoRow label={t("enterpriseModels.account.lastSynced")} value={waitingForData ? <InlineLoading className="w-24" /> : formatDateTime(account?.lastSyncedAt)} />
              </div>
            </ContentModule>
          </section>
        ) : (
          <ContentModule
            className="rhd-railops-usage-key-module"
            title={t("enterpriseModels.apiKeys.title")}
            note={t("enterpriseModels.apiKeys.tableNote", { count: formatNumber(filteredKeys.length) })}
            extra={<StatusTag tone="blue">{formatNumber(workspace?.keyPaging?.total ?? workspace?.keys.length ?? 0)}</StatusTag>}
          >
            <TableToolbar
              search={
                <SearchField
                  allowClear
                  className="rhd-railops-usage-search"
                  placeholder={t("enterpriseModels.apiKeys.searchPlaceholder")}
                  value={keyword}
                  onChange={(event) => setKeyword(event.target.value)}
                />
              }
              filters={
                <SelectField
                  style={{ marginBottom: 0 }}
                  selectProps={{
                    "aria-label": t("enterpriseModels.filters.keyStatus"),
                    value: keyStatus,
                    onChange: (value) => setKeyStatus(value as KeyStatusFilter),
                    options: [
                      { value: "all", label: keyStatusFilterLabel(t, "all") },
                      { value: "active", label: keyStatusFilterLabel(t, "active") },
                      { value: "inactive", label: keyStatusFilterLabel(t, "inactive") },
                      { value: "pending", label: keyStatusFilterLabel(t, "pending") },
                    ],
                    style: { width: 140 },
                  }}
                />
              }
              actions={<span className="rhd-railops-table-summary">{formatNumber(filteredKeys.length)} / {formatNumber(workspace?.keyPaging?.total ?? workspace?.keys.length ?? 0)}</span>}
            />

            {waitingForData ? (
              <div className="rhd-railops-table-skeleton">
                {Array.from({ length: 6 }).map((_, index) => <Skeleton.Node key={index} active className="w-full" style={{ width: "100%", height: 48 }} />)}
              </div>
            ) : (
              <div className="rhd-railops-usage-table-wrap">
                <DataTable<EnterpriseAIKey>
                  className="rhd-railops-device-table rhd-railops-usage-table"
                  columns={keyColumns}
                  dataSource={filteredKeys}
                  emptyDescription={t("enterpriseModels.apiKeys.emptyTitle")}
                  pagination={{
                    defaultPageSize: 6,
                    hideOnSinglePage: true,
                    showSizeChanger: false,
                    size: "small",
                  }}
                  rowKey="id"
                  size="small"
                />
              </div>
            )}
          </ContentModule>
        )}
      </section>

      <ProjectDialog
        open={canUpdateKeys && Boolean(editingKey)}
        onOpenChange={(open) => {
          if (!open) setEditingKey(null)
        }}
        title={t("enterpriseModels.quotaDialog.title")}
        footer={
          <>
            <RailopsButton onClick={() => setEditingKey(null)} disabled={savingQuota}>
              {t("enterpriseModels.cancel")}
            </RailopsButton>
            <RailopsButton onClick={() => void submitQuota()} disabled={savingQuota}>
              <SaveIcon data-icon="inline-start" />
              {savingQuota ? t("enterpriseModels.saving") : t("enterpriseModels.save")}
            </RailopsButton>
          </>
        }
      >
        <div className="grid gap-3">
          <div className="rounded-md border border-border bg-muted/40 p-3 text-sm text-muted-foreground">
            <div className="font-medium text-foreground">{editingKey ? getEnterpriseAIKeyDisplayName(editingKey, t("enterpriseModels.apiKeys.keyPrefix", { id: editingKey.id })) : "-"}</div>
            {editingKeyProductMeta ? <div className="mt-1 text-xs">{editingKeyProductMeta}</div> : null}
            <div className="mt-1 font-mono text-xs">{editingKey?.keyPreview || t("enterpriseModels.hidden")}</div>
          </div>
          <label className="grid gap-1 text-sm">
            <span className="font-medium text-foreground">{t("enterpriseModels.quotaDialog.quota")}</span>
            <Input
              type="number"
              min={0}
              step="0.01"
              value={quotaValue}
              onChange={(event) => setQuotaValue(event.target.value)}
              autoFocus
            />
          </label>
          <p className="text-xs text-muted-foreground">
            {t("enterpriseModels.quotaDialog.hint", { value: formatCurrency(editingKey?.quotaUsed ?? 0) })}
          </p>
        </div>
      </ProjectDialog>

      <ProjectDialog
        open={Boolean(detailKey)}
        onOpenChange={(open) => {
          if (!open) setDetailKey(null)
        }}
        title={t("enterpriseModels.apiKeys.detailTitle")}
        size="lg"
      >
        {detailKey ? (
          <div className="rhd-railops-usage-key-detail">
            <section className="rhd-railops-usage-key-detail-hero">
              <div>
                <strong>{getEnterpriseAIKeyDisplayName(detailKey, t("enterpriseModels.apiKeys.keyPrefix", { id: detailKey.id }))}</strong>
                <span>{getEnterpriseAIKeyProductMeta(detailKey) || t("enterpriseModels.apiKeys.noProductScope")}</span>
              </div>
              <StatusTag tone={statusTone(detailKey.status)}>{statusLabel(t, detailKey.status)}</StatusTag>
            </section>

            <div className="rhd-railops-usage-detail-grid">
              <InfoRow label={t("enterpriseModels.apiKeys.columns.key")} value={detailKey.keyPreview || t("enterpriseModels.hidden")} />
              <InfoRow label={t("enterpriseModels.apiKeys.columns.modelGroup")} value={detailKey.group?.name || t("enterpriseModels.apiKeys.groupPrefix", { id: detailKey.groupId })} />
              <InfoRow label={t("enterpriseModels.apiKeys.groupPlatform")} value={detailKey.group?.platform || "-"} />
              <InfoRow label={t("enterpriseModels.apiKeys.rateMultiplier")} value={String(detailKey.group?.rateMultiplier ?? 1)} />
              <InfoRow label={t("enterpriseModels.apiKeys.concurrency")} value={formatNumber(detailKey.currentConcurrency)} />
              <InfoRow label={t("enterpriseModels.apiKeys.lastUsedIp")} value={detailKey.lastUsedIp || "-"} />
              <InfoRow label={t("enterpriseModels.apiKeys.columns.lastUsed")} value={formatDateTime(detailKey.lastUsedAt)} />
            </div>

            <section className="rhd-railops-usage-detail-section">
              <div>
                <strong>{t("enterpriseModels.apiKeys.columns.quota")}</strong>
              </div>
              <QuotaCell item={detailKey} t={t} />
            </section>

            <div className="rhd-railops-usage-detail-columns">
              <section className="rhd-railops-usage-detail-section">
                <div>
                  <strong>{t("enterpriseModels.apiKeys.columns.usageWindow")}</strong>
                </div>
                <MetricStack
                  rows={[
                    { label: "5h", value: formatNumber(detailKey.usage5h) },
                    { label: "1d", value: formatNumber(detailKey.usage1d) },
                    { label: "7d", value: formatNumber(detailKey.usage7d) },
                  ]}
                />
              </section>
              <section className="rhd-railops-usage-detail-section">
                <div>
                  <strong>{t("enterpriseModels.apiKeys.columns.rateLimit")}</strong>
                </div>
                <MetricStack
                  rows={[
                    { label: "5h", value: formatNumber(detailKey.rateLimit5h) },
                    { label: "1d", value: formatNumber(detailKey.rateLimit1d) },
                    { label: "7d", value: formatNumber(detailKey.rateLimit7d) },
                  ]}
                />
              </section>
            </div>
          </div>
        ) : null}
      </ProjectDialog>
    </PageShell>
  )
}

function UsageKpi({
  detail,
  icon,
  label,
  loading,
  tone,
  value,
}: {
  detail: string
  icon: ReactNode
  label: string
  loading: boolean
  tone: UsageKpiTone
  value: string
}) {
  return (
    <div className={`rhd-railops-usage-kpi is-${tone}`}>
      <span className="rhd-railops-usage-kpi-icon">{icon}</span>
      <div>
        <span>{label}</span>
        {loading ? <Skeleton.Node active style={{ width: "5rem", height: 20 }} /> : <strong>{value}</strong>}
        {loading ? <Skeleton.Node active style={{ width: "7rem", height: 12 }} /> : <small>{detail}</small>}
      </div>
    </div>
  )
}

function QuotaCell({ item, t }: { item: EnterpriseAIKey; t: I18nT }) {
  const percent = Math.max(0, Math.min(item.quotaUsagePercent || 0, 100))
  return (
    <div className="rhd-railops-usage-quota-cell">
      <div className="rhd-railops-usage-quota-head">
        <strong>{Math.round(percent)}%</strong>
        <span>{t("enterpriseModels.apiKeys.remaining", { value: formatCurrency(item.quotaRemaining) })}</span>
      </div>
      <div className="rhd-railops-usage-quota-bar">
        <span style={{ width: `${percent}%` }} />
      </div>
      <small>{t("enterpriseModels.apiKeys.quotaUsed", { used: formatCurrency(item.quotaUsed), total: formatCurrency(item.quota) })}</small>
    </div>
  )
}

function MetricStack({ rows }: { rows: { label: string; value: string }[] }) {
  return (
    <div className="rhd-railops-usage-metric-stack">
      {rows.map((row) => (
        <div key={row.label}>
          <span>{row.label}</span>
          <strong>{row.value}</strong>
        </div>
      ))}
    </div>
  )
}

function EmptyBlock({ title }: { title: string }) {
  return (
    <div className="rhd-railops-table-empty">
      <EmptyState description={title} />
    </div>
  )
}

function InlineLoading({ className = "w-20" }: { className?: string }) {
  const width = className === "w-32" ? "8rem" : className === "w-28" ? "7rem" : className === "w-24" ? "6rem" : "5rem"
  return <Skeleton.Node active style={{ width, height: 16 }} aria-label="Loading data" />
}

function TrendLoading() {
  return (
    <div className="rhd-railops-usage-trend-list" aria-label="Loading trend data">
      {Array.from({ length: 4 }).map((_, index) => (
        <div key={index} className="rhd-railops-usage-trend-row">
          <div className="rhd-railops-usage-trend-date">
            <Skeleton.Node active style={{ width: "5rem", height: 16 }} />
            <Skeleton.Node active style={{ width: "4rem", height: 12 }} />
          </div>
          <div className="rhd-railops-usage-trend-meter">
            <Skeleton.Node active style={{ width: "100%", height: 8 }} />
            <Skeleton.Node active style={{ width: "6rem", height: 12 }} />
          </div>
          <div className="rhd-railops-usage-trend-cost">
            <Skeleton.Node active style={{ width: "5rem", height: 16 }} />
            <Skeleton.Node active style={{ width: "6rem", height: 12 }} />
          </div>
        </div>
      ))}
    </div>
  )
}

function defaultQuery(): EnterpriseAIModelsQuery {
  return {
    ...getRailopsDateRange("7d"),
    granularity: "day",
    timezone: RAILOPS_DEFAULT_TIMEZONE,
    page: 1,
    pageSize: 20,
    sortBy: "created_at",
    sortOrder: "desc",
  }
}

function subscriptionDurationLabel(t: I18nT, subscription?: EnterpriseAISubscription) {
  if (!subscription) return t("enterpriseModels.subscription.unlimited")
  if (subscription.durationDays > 0) return t("enterpriseModels.subscription.days", { value: formatNumber(subscription.durationDays) })
  if (subscription.expiresAt) return t("enterpriseModels.subscription.configured")
  return t("enterpriseModels.subscription.unlimited")
}

function subscriptionDetailLabel(t: I18nT, subscription?: EnterpriseAISubscription) {
  if (!subscription) return t("enterpriseModels.subscription.noSubscription")
  const groupName = subscription.group?.name || t("enterpriseModels.subscription.groupPrefix", { id: subscription.groupId })
  const range = subscription.expiresAt
    ? t("enterpriseModels.subscription.range", { start: formatDateOnly(subscription.startsAt), end: formatDateOnly(subscription.expiresAt) })
    : t("enterpriseModels.subscription.noExpiry")
  const remaining =
    subscription.remainingDays > 0
      ? t("enterpriseModels.subscription.remainingDays", { value: formatNumber(subscription.remainingDays) })
      : subscription.status === "active"
        ? t("enterpriseModels.subscription.dueToday")
        : t("enterpriseModels.subscription.expired")
  return t("enterpriseModels.subscription.detail", { groupName, range, remaining })
}

function subscriptionPeriodLabel(t: I18nT, subscription?: EnterpriseAISubscription) {
  if (!subscription) return t("enterpriseModels.subscription.unlimited")
  if (!subscription.expiresAt) {
    const startsAt = formatDateOnly(subscription.startsAt)
    return startsAt === "-" ? t("enterpriseModels.subscription.unlimited") : t("enterpriseModels.subscription.periodFrom", { start: startsAt })
  }
  return t("enterpriseModels.subscription.range", { start: formatDateOnly(subscription.startsAt), end: formatDateOnly(subscription.expiresAt) })
}

function formatDateOnly(value?: string) {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value.split("T")[0] || value
  }
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, "0")
  const day = String(date.getDate()).padStart(2, "0")
  return `${year}-${month}-${day}`
}

function statusTone(status?: string): StatusTagTone {
  const normalized = (status || "").toLowerCase()
  if (normalized === "active" || normalized === "success" || normalized === "ok") return "success"
  if (normalized.includes("fail") || normalized.includes("disabled") || normalized.includes("error")) return "error"
  if (normalized.includes("pending") || normalized.includes("unknown")) return "warning"
  if (normalized.includes("provision")) return "blue"
  return "neutral"
}

function InfoRow({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="rhd-railops-usage-info-row">
      <span>{label}</span>
      <div className="rhd-railops-usage-info-value">{value}</div>
    </div>
  )
}
